package controlplane

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"gopkg.in/yaml.v3"

	"github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/examples/internal/controlplane/server"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/examples/internal/controlplane/snapshot"
	"github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/examples/internal/controlplane/watcher"
)

type Config struct {
	Server struct {
		Port   uint   `yaml:"port"`
		NodeID string `yaml:"nodeID"`
	} `yaml:"server"`
	XDS struct {
		WatchInterval string `yaml:"watchInterval"`
	} `yaml:"xds"`
}

var snapshotVersion atomic.Uint64

type staticNodeHash string

func (h staticNodeHash) ID(*core.Node) string {
	return string(h)
}

func Run(bootstrapPath, snapshotPath string) error {
	config, err := loadConfig(bootstrapPath)
	if err != nil {
		return err
	}
	nodeID := config.Server.NodeID
	if nodeID == "" {
		nodeID = "xds-example"
	}

	slog.Info(
		"Loaded control plane bootstrap",
		"bootstrap",
		bootstrapPath,
		"snapshot",
		snapshotPath,
		"port",
		config.Server.Port,
		"snapshot_key",
		nodeID,
	)

	snapshotCache := cache.NewSnapshotCache(false, staticNodeHash(nodeID), nil)
	if err := loadAndUpdateSnapshot(snapshotPath, nodeID, snapshotCache); err != nil {
		return err
	}

	watchInterval := parseDuration(config.XDS.WatchInterval, time.Second)
	fw, err := watcher.NewFileWatcher(snapshotPath, func(filePath string) {
		slog.Info("Reloading xDS snapshot", "path", filePath)
		if err := loadAndUpdateSnapshot(filePath, nodeID, snapshotCache); err != nil {
			slog.Error("Reload snapshot failed", "error", err)
		}
	}, watchInterval)
	if err != nil {
		return fmt.Errorf("create file watcher: %w", err)
	}
	defer fw.Close()

	fw.Start()

	xdsServer := server.NewServer(config.Server.Port, snapshotCache)

	serverErr := make(chan error, 1)
	go func() {
		if err := xdsServer.Run(); err != nil {
			serverErr <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("Received shutdown signal", "signal", sig)
	case err := <-serverErr:
		return fmt.Errorf("run xDS server: %w", err)
	}

	xdsServer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	<-ctx.Done()

	return nil
}

//nolint:gosec // Example file paths come from explicit local CLI flags.
func loadConfig(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read bootstrap config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse bootstrap config: %w", err)
	}

	return &config, nil
}

//
//nolint:gosec // Example file paths come from explicit local CLI flags.
func loadAndUpdateSnapshot(
	filePath string,
	nodeID string,
	snapshotCache cache.SnapshotCache,
) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read xDS snapshot file: %w", err)
	}

	var xdsConfig snapshot.XDSConfig
	if err := yaml.Unmarshal(data, &xdsConfig); err != nil {
		return fmt.Errorf("parse xDS snapshot: %w", err)
	}

	version := snapshotVersion.Add(1)
	versionStr := strconv.FormatUint(version, 10)

	builder := snapshot.NewBuilder(versionStr)
	snap, err := builder.BuildSnapshot(&xdsConfig)
	if err != nil {
		return fmt.Errorf("build xDS snapshot: %w", err)
	}

	if err := snap.Consistent(); err != nil {
		return fmt.Errorf("snapshot inconsistent: %w", err)
	}

	if err := snapshotCache.SetSnapshot(context.Background(), nodeID, snap); err != nil {
		return fmt.Errorf("set xDS snapshot: %w", err)
	}

	slog.Info(
		"Updated xDS snapshot",
		"version",
		versionStr,
		"clusters",
		len(xdsConfig.Clusters),
		"endpoints",
		len(xdsConfig.Endpoints),
		"listeners",
		len(xdsConfig.Listeners),
		"routes",
		len(xdsConfig.Routes),
	)

	return nil
}

func parseDuration(input string, fallback time.Duration) time.Duration {
	if input == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(input)
	if err != nil {
		slog.Warn(
			"Invalid duration, using fallback",
			"input",
			input,
			"fallback",
			fallback,
			"error",
			err,
		)
		return fallback
	}

	return parsed
}
