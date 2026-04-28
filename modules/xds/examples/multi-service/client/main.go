package main

import "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/examples/internal/exampleapp"

func main() {
	exampleapp.RunClient(
		"github.com.codesjoy.yggdrasil.contrib.xds.example.multi-service.client",
		exampleapp.LibraryService,
		exampleapp.GreeterService,
	)
}
