// Copyright 2026 Codesjoy
// Licensed under the Apache License, Version 2.0.

package protobufcompat_test

import (
	"testing"

	_ "github.com/codesjoy/yggdrasil-ecosystem/modules/etcd/v3"
	_ "github.com/codesjoy/yggdrasil-ecosystem/modules/polaris/v3"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func TestPolarisAndEtcdDescriptorsCoexist(t *testing.T) {
	tests := []struct {
		path    string
		pkgName string
	}{
		{path: "auth.proto", pkgName: "v1"},
		{path: "etcd/api/authpb/auth.proto", pkgName: "authpb"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			file, err := protoregistry.GlobalFiles.FindFileByPath(test.path)
			if err != nil {
				t.Fatalf("find descriptor: %v", err)
			}
			if got := string(file.Package()); got != test.pkgName {
				t.Fatalf("descriptor package = %q, want %q", got, test.pkgName)
			}
			if got := file.Path(); got != test.path {
				t.Fatalf("descriptor path = %q, want %q", got, test.path)
			}
		})
	}
}
