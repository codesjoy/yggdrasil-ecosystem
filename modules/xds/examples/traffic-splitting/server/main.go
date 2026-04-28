package main

import "github.com/codesjoy/yggdrasil-ecosystem/modules/xds/v3/examples/internal/exampleapp"

func main() {
	exampleapp.RunServer(
		"github.com.codesjoy.yggdrasil.contrib.xds.example.traffic-splitting.server",
		exampleapp.SampleService,
		"hello from xDS traffic splitting",
	)
}
