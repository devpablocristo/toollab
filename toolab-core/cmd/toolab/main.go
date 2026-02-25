package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"toolab-core/internal/app"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "run":
		runCmd := flag.NewFlagSet("run", flag.ExitOnError)
		outDir := runCmd.String("out", "golden_runs", "base output directory")
		if err := runCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(2)
		}
		args := runCmd.Args()
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "usage: toolab run <scenario.yaml> [--out DIR]")
			os.Exit(2)
		}

		result, err := app.RunScenario(context.Background(), args[0], *outDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		fmt.Printf("run_dir=%s\n", result.RunDir)
		fmt.Printf("decision_tape_hash=%s\n", result.Bundle.Execution.DecisionTapeHash)
		fmt.Printf("deterministic_fingerprint=%s\n", result.Bundle.DeterministicFingerprint)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println("toolab usage:")
	fmt.Println("  toolab run <scenario.yaml> [--out DIR]")
}
