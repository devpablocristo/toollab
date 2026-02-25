package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"toolab-core/internal/app"
	"toolab-core/internal/gen"
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
	case "gen":
		genCmd := flag.NewFlagSet("gen", flag.ExitOnError)
		out := genCmd.String("out", "", "write output to file instead of stdout")
		profile := genCmd.String("profile", "light", "chaos profile: none|light|moderate|aggressive")
		concurrency := genCmd.Int("concurrency", 2, "workload concurrency")
		duration := genCmd.Int("duration", 30, "workload duration in seconds")
		schedule := genCmd.String("schedule", "closed_loop", "schedule mode: closed_loop|open_loop")
		runSeed := genCmd.String("run-seed", "", "run seed (auto-generated if empty)")
		chaosSeed := genCmd.String("chaos-seed", "", "chaos seed (auto-generated if empty)")
		var includeTags, excludeTags stringSlice
		genCmd.Var(&includeTags, "include-tag", "only include operations with this tag (repeatable)")
		genCmd.Var(&excludeTags, "exclude-tag", "exclude operations with this tag (repeatable)")

		if err := genCmd.Parse(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(2)
		}
		args := genCmd.Args()
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "usage: toolab gen <openapi-spec> [flags]")
			os.Exit(2)
		}

		yamlBytes, err := gen.Generate(args[0], gen.Options{
			Profile:     *profile,
			Concurrency: *concurrency,
			DurationS:   *duration,
			Schedule:    *schedule,
			IncludeTags: []string(includeTags),
			ExcludeTags: []string(excludeTags),
			RunSeed:     *runSeed,
			ChaosSeed:   *chaosSeed,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		if *out != "" {
			if err := os.WriteFile(*out, yamlBytes, 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "wrote %s\n", *out)
		} else {
			os.Stdout.Write(yamlBytes)
		}
	default:
		usage()
		os.Exit(2)
	}
}

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(val string) error {
	*s = append(*s, val)
	return nil
}

func usage() {
	fmt.Println("toolab usage:")
	fmt.Println("  toolab run <scenario.yaml> [--out DIR]")
	fmt.Println("  toolab gen <openapi-spec>  [flags]")
}
