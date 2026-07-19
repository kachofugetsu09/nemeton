package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/kachofugetsu09/nemeton/internal/api"
	"github.com/kachofugetsu09/nemeton/internal/config"
	"github.com/kachofugetsu09/nemeton/internal/daemon"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		printUsage(stderr)
		return 2
	}
	switch args[0] + " " + args[1] {
	case "daemon start":
		return daemonStart(args[2:], stderr)
	case "daemon status":
		return daemonStatus(ctx, args[2:], stdout, stderr)
	case "project open":
		return projectOpen(ctx, args[2:], stdout, stderr)
	case "project inspect":
		return projectInspect(ctx, args[2:], stdout, stderr)
	case "project relink":
		return projectRelink(ctx, args[2:], stdout, stderr)
	case "project replay":
		return projectReplay(ctx, args[2:], stdout, stderr)
	default:
		printUsage(stderr)
		return 2
	}
}

func daemonStart(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("daemon start", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data-dir", "", "absolute Nemeton data directory")
	if err := flags.Parse(interspersedArgs(args, "data-dir")); err != nil || flags.NArg() != 0 {
		return 2
	}
	configuration, err := config.Resolve(*dataDir)
	if err != nil {
		return printFailure(stderr, false, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := daemon.Run(ctx, configuration); err != nil {
		return printFailure(stderr, false, err)
	}
	return 0
}

func daemonStatus(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags, dataDir, jsonOutput, ok := commonFlags("daemon status", args, stderr)
	if !ok || flags.NArg() != 0 {
		return 2
	}
	configuration, err := config.Resolve(*dataDir)
	if err != nil {
		return printFailure(stderr, *jsonOutput, err)
	}
	health, err := api.NewClient(configuration.SocketPath).Health(ctx)
	if err != nil {
		return printFailure(stderr, *jsonOutput, err)
	}
	if *jsonOutput {
		return printJSON(stdout, stderr, health)
	}
	fmt.Fprintf(stdout, "Status: %s\nData: %s\nWorktrees: %s\n", health.Status, health.DataDir, health.WorktreesRoot)
	return 0
}

func projectOpen(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("project open", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data-dir", "", "absolute Nemeton data directory")
	branch := flags.String("integration-branch", "", "confirmed local integration branch")
	jsonOutput := flags.Bool("json", false, "print machine-readable JSON")
	if err := flags.Parse(interspersedArgs(args, "data-dir", "integration-branch")); err != nil || flags.NArg() > 1 {
		return 2
	}
	path := "."
	if flags.NArg() == 1 {
		path = flags.Arg(0)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return printFailure(stderr, *jsonOutput, fmt.Errorf("resolve repository path: %w", err))
	}
	configuration, err := config.Resolve(*dataDir)
	if err != nil {
		return printFailure(stderr, *jsonOutput, err)
	}
	response, err := api.NewClient(configuration.SocketPath).Open(ctx, api.OpenRequest{Path: absolute, IntegrationBranch: *branch})
	if err != nil {
		return printFailure(stderr, *jsonOutput, err)
	}
	return printProject(stdout, stderr, *jsonOutput, response)
}

func projectInspect(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags, dataDir, jsonOutput, ok := commonFlags("project inspect", args, stderr)
	if !ok || flags.NArg() != 1 {
		return 2
	}
	configuration, err := config.Resolve(*dataDir)
	if err != nil {
		return printFailure(stderr, *jsonOutput, err)
	}
	response, err := api.NewClient(configuration.SocketPath).Inspect(ctx, flags.Arg(0))
	if err != nil {
		return printFailure(stderr, *jsonOutput, err)
	}
	return printProject(stdout, stderr, *jsonOutput, response)
}

func projectRelink(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("project relink", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data-dir", "", "absolute Nemeton data directory")
	branch := flags.String("integration-branch", "", "confirmed local integration branch")
	jsonOutput := flags.Bool("json", false, "print machine-readable JSON")
	if err := flags.Parse(interspersedArgs(args, "data-dir", "integration-branch")); err != nil || flags.NArg() != 2 {
		return 2
	}
	absolute, err := filepath.Abs(flags.Arg(1))
	if err != nil {
		return printFailure(stderr, *jsonOutput, fmt.Errorf("resolve repository path: %w", err))
	}
	configuration, err := config.Resolve(*dataDir)
	if err != nil {
		return printFailure(stderr, *jsonOutput, err)
	}
	response, err := api.NewClient(configuration.SocketPath).Relink(ctx, flags.Arg(0), api.RelinkRequest{Path: absolute, IntegrationBranch: *branch})
	if err != nil {
		return printFailure(stderr, *jsonOutput, err)
	}
	return printProject(stdout, stderr, *jsonOutput, response)
}

func projectReplay(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags, dataDir, jsonOutput, ok := commonFlags("project replay", args, stderr)
	if !ok || flags.NArg() != 1 {
		return 2
	}
	configuration, err := config.Resolve(*dataDir)
	if err != nil {
		return printFailure(stderr, *jsonOutput, err)
	}
	response, err := api.NewClient(configuration.SocketPath).Replay(ctx, flags.Arg(0))
	if err != nil {
		return printFailure(stderr, *jsonOutput, err)
	}
	return printProject(stdout, stderr, *jsonOutput, response)
}

func commonFlags(name string, args []string, stderr io.Writer) (*flag.FlagSet, *string, *bool, bool) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data-dir", "", "absolute Nemeton data directory")
	jsonOutput := flags.Bool("json", false, "print machine-readable JSON")
	if err := flags.Parse(interspersedArgs(args, "data-dir")); err != nil {
		return flags, dataDir, jsonOutput, false
	}
	return flags, dataDir, jsonOutput, true
}

func printProject(stdout, stderr io.Writer, jsonOutput bool, response api.ProjectResponse) int {
	if jsonOutput {
		return printJSON(stdout, stderr, response)
	}
	fmt.Fprintf(stdout, "Project: %s\nReality: %s\nCommit: %s\nDigest: %s\n", response.Project.Project.ID, response.Project.CurrentReality.ID, response.Project.CurrentReality.CommitOID, response.Project.ResultDigest)
	return 0
}

func printFailure(stderr io.Writer, jsonOutput bool, err error) int {
	if jsonOutput {
		var problem *api.Problem
		if errors.As(err, &problem) {
			if printJSON(stderr, stderr, problem) == 0 {
				return 1
			}
		}
	}
	fmt.Fprintln(stderr, err)
	return 1
}

func printJSON(output, stderr io.Writer, value any) int {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, "usage: nemeton <daemon start|daemon status|project open|project inspect|project relink|project replay> [options]")
}

func interspersedArgs(args []string, valueFlags ...string) []string {
	requiresValue := make(map[string]bool, len(valueFlags))
	for _, name := range valueFlags {
		requiresValue["--"+name] = true
	}
	var options []string
	var positionals []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--" {
			positionals = append(positionals, args[index+1:]...)
			break
		}
		name, _, hasInlineValue := strings.Cut(argument, "=")
		if requiresValue[name] {
			options = append(options, argument)
			if !hasInlineValue && index+1 < len(args) {
				index++
				options = append(options, args[index])
			}
			continue
		}
		if argument == "--json" || strings.HasPrefix(argument, "--json=") || strings.HasPrefix(argument, "-") {
			options = append(options, argument)
			continue
		}
		positionals = append(positionals, argument)
	}
	return append(options, positionals...)
}
