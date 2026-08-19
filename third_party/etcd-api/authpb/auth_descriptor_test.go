// Copyright 2026 Codesjoy
// Licensed under the Apache License, Version 2.0.

package authpb

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoregistry"
)

func TestAuthDescriptorUsesCanonicalPath(t *testing.T) {
	const path = "etcd/api/authpb/auth.proto"

	file, err := protoregistry.GlobalFiles.FindFileByPath(path)
	if err != nil {
		t.Fatalf("find %q: %v", path, err)
	}
	if got := string(file.Package()); got != "authpb" {
		t.Fatalf("descriptor package = %q, want authpb", got)
	}
	if got := file.Path(); got != path {
		t.Fatalf("descriptor path = %q, want %q", got, path)
	}
}
