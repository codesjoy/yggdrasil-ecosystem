package main

import "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/examples/internal/exampleapp"

func main() {
	exampleapp.RunServer(
		"github.com.codesjoy.yggdrasil.contrib.xds.example.basic.server",
		exampleapp.SampleService,
		"hello from xDS basic",
	)
}
