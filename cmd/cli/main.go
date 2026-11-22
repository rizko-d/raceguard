package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"raceguard/internal/config"
	"raceguard/internal/guard"
	"raceguard/internal/reporter"
	"raceguard/internal/runner"
)

func main() {
	specPath := flag.String("spec", "", "Path to YAML scenario file (required)")
	jsonOut := flag.String("json", "", "Optional path to write JSON report")
	sarifOut := flag.String("sarif", "", "Optional path to write SARIF 2.1.0 report")
	htmlOut := flag.String("html", "", "Optional path to write static HTML report")
	flag.Parse()

	if *specPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: raceguard-cli -spec examples/voucher-redeem-safe.yaml [-json out.json] [-sarif out.sarif] [-html out.html]")
		os.Exit(2)
	}

	scenario, err := config.LoadScenario(*specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading scenario: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	r := runner.New(guard.DefaultPolicy())
	result, err := r.Execute(ctx, scenario, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Execution blocked or failed: %v\n", err)
		os.Exit(2)
	}

	fmt.Println(reporter.FormatCLI(result))

	if *jsonOut != "" {
		if err := reporter.WriteJSON(*jsonOut, result); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing JSON report: %v\n", err)
			os.Exit(2)
		}
	}
	if *sarifOut != "" {
		if err := reporter.WriteSARIF(*sarifOut, result); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing SARIF report: %v\n", err)
			os.Exit(2)
		}
	}
	if *htmlOut != "" {
		if err := reporter.WriteHTML(*htmlOut, result); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing HTML report: %v\n", err)
			os.Exit(2)
		}
	}

	if result.Verdict == config.VerdictFail {
		os.Exit(1)
	}
}
