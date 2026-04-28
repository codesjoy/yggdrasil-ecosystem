package main

import "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/examples/internal/exampleapp"

func main() {
	exampleapp.RunServer(
		"github.com.codesjoy.yggdrasil.contrib.xds.example.multi-service.server",
		exampleapp.LibraryService,
		"hello from xDS multi-service",
	)
}
