// Copyright 2022 The codesjoy Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlp

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	otlpmetrichttp "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	defaultGRPCEndpoint       = "localhost:4317"
	defaultHTTPEndpoint       = "localhost:4318"
	defaultBatchTimeout       = 5 * time.Second
	defaultMaxQueueSize       = 2048
	defaultMaxExportBatchSize = 512
	defaultTimeout            = 30 * time.Second
	defaultRetryInitialDelay  = 100 * time.Millisecond
	defaultRetryMaxDelay      = 5 * time.Second
	defaultMaxAttempts        = 5

	defaultExportInterval = 60 * time.Second
	defaultExportTimeout  = 30 * time.Second
)

func buildResourceAttributes(
	serviceName string,
	customAttrs map[string]interface{},
) map[string]any {
	attrs := make(map[string]any, len(customAttrs)+1)
	attrs["service.name"] = serviceName

	for key, value := range customAttrs {
		attrs[key] = value
	}

	return attrs
}

func createGRPCDialOptions(tlsCfg TLSConfig) ([]grpc.DialOption, error) {
	var opts []grpc.DialOption

	switch {
	case tlsCfg.Insecure:
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	case tlsCfg.Enabled:
		tlsConfig, err := createTLSConfig(tlsCfg)
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	default:
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return opts, nil
}

func createHTTPClientTLSOption(tlsCfg TLSConfig) (otlptracehttp.Option, error) {
	if tlsCfg.Insecure {
		return otlptracehttp.WithInsecure(), nil
	}
	if tlsCfg.Enabled {
		tlsConfig, err := createTLSConfig(tlsCfg)
		if err != nil {
			return nil, err
		}
		return otlptracehttp.WithTLSClientConfig(tlsConfig), nil
	}
	return otlptracehttp.WithInsecure(), nil
}

func createHTTPMetricClientTLSOption(tlsCfg TLSConfig) (otlpmetrichttp.Option, error) {
	if tlsCfg.Insecure {
		return otlpmetrichttp.WithInsecure(), nil
	}
	if tlsCfg.Enabled {
		tlsConfig, err := createTLSConfig(tlsCfg)
		if err != nil {
			return nil, err
		}
		return otlpmetrichttp.WithTLSClientConfig(tlsConfig), nil
	}
	return otlpmetrichttp.WithInsecure(), nil
}

func createTLSConfig(tlsCfg TLSConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{} // nolint:gosec

	if tlsCfg.CAFile != "" {
		caCert, err := os.ReadFile(tlsCfg.CAFile)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	if tlsCfg.CertFile != "" && tlsCfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
