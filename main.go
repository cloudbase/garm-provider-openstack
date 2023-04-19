package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/cloudbase/garm-provider-openstack/provider"
	"github.com/cloudbase/garm/runner/providers/external/execution"
)

var version = flag.Bool("version", false, "prints version")
var Version string

func main() {
	flag.Parse()
	if *version {
		fmt.Println(Version)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), signals...)
	defer stop()

	executionEnv, err := execution.GetEnvironment()
	if err != nil {
		log.Fatal(err)
	}

	prov, err := provider.NewOpenStackProvider(executionEnv.ProviderConfigFile, executionEnv.ControllerID)
	if err != nil {
		log.Fatal(err)
	}

	result, err := execution.Run(ctx, prov, executionEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run command: %s", err)
		os.Exit(execution.ResolveErrorToExitCode(err))
	}
	if len(result) > 0 {
		fmt.Fprint(os.Stdout, result)
	}
}
