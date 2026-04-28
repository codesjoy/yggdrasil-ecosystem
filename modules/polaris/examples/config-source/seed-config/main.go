package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	polarisgo "github.com/polarismesh/polaris-go"
	polariscfg "github.com/polarismesh/polaris-go/pkg/config"
)

func main() {
	namespace := flag.String("namespace", "default", "Polaris namespace")
	group := flag.String("group", "yggdrasil-polaris-examples", "Polaris config file group")
	fileName := flag.String("file", "config-source.yaml", "Polaris config file name")
	namingAddress := flag.String("address", "127.0.0.1:8091", "Polaris naming server address")
	configAddress := flag.String(
		"config-address",
		"127.0.0.1:8093",
		"Polaris config server address",
	)
	contentPath := flag.String("content", "remote-config.yaml", "YAML file to seed into Polaris")
	flag.Parse()

	content, err := os.ReadFile(*contentPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read content: %v\n", err)
		os.Exit(1)
	}

	cfg := polariscfg.NewDefaultConfiguration([]string{*namingAddress})
	cfg.GetConfigFile().GetConfigConnectorConfig().SetAddresses([]string{*configAddress})

	api, err := polarisgo.NewConfigAPIByConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create Polaris config API: %v\n", err)
		os.Exit(1)
	}

	if err := upsertConfig(api, *namespace, *group, *fileName, string(content)); err != nil {
		fmt.Fprintf(os.Stderr, "seed Polaris config: %v\n", err)
		os.Exit(1)
	}
	if err := api.PublishConfigFile(*namespace, *group, *fileName); err != nil {
		fmt.Fprintf(os.Stderr, "publish Polaris config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("seeded %s/%s/%s\n", *namespace, *group, *fileName)
}

func upsertConfig(
	api polarisgo.ConfigAPI,
	namespace string,
	group string,
	fileName string,
	content string,
) error {
	if api == nil {
		return errors.New("nil Polaris config API")
	}
	if err := api.CreateConfigFile(namespace, group, fileName, content); err == nil {
		return nil
	}
	if err := api.UpdateConfigFile(namespace, group, fileName, content); err != nil {
		return err
	}
	return nil
}
