package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/provider"
)

// version is set via -ldflags at release time.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with debug support (delve attach)")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/sigmacomputing/sigma",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}
