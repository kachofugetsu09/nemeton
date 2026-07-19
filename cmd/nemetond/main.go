package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kachofugetsu09/nemeton/internal/config"
	"github.com/kachofugetsu09/nemeton/internal/daemon"
)

func main() {
	flags := flag.NewFlagSet("nemetond", flag.ExitOnError)
	dataDir := flags.String("data-dir", "", "absolute Nemeton data directory")
	webAddress := flags.String("web-address", "", "loopback address for the local Web UI")
	flags.Parse(os.Args[1:])
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: nemetond [--data-dir PATH]")
		os.Exit(2)
	}
	configuration, err := config.Resolve(*dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *webAddress != "" {
		configuration.WebAddress = *webAddress
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := daemon.Run(ctx, configuration); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
