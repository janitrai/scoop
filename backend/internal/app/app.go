package app

import (
	"fmt"
	"os"
	"strings"
)

// Run executes the CLI command and returns a process exit code.
func Run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 2
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "help", "--help", "-h":
		printUsage()
		return 0
	case "health":
		return runHealth(args[1:])
	case "ingest":
		return runIngest(args[1:])
	case "validate":
		return runValidate(args[1:])
	case "normalize":
		return runNormalize(args[1:])
	case "embed":
		return runEmbed(args[1:])
	case "dedup":
		return runDedup(args[1:])
	case "process", "run-once":
		return runProcess(args[1:])
	case "serve":
		return runServe(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printUsage()
		return 2
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "scoop CLI")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  scoop <command> [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  health   Verify database connectivity")
	fmt.Fprintln(os.Stderr, "  ingest   Insert one item into ingest ledger tables")
	fmt.Fprintln(os.Stderr, "  validate  Validate news item JSON files against v1 schema")
	fmt.Fprintln(os.Stderr, "  normalize  Convert pending raw arrivals into normalized documents")
	fmt.Fprintln(os.Stderr, "  embed      Generate embeddings for normalized documents")
	fmt.Fprintln(os.Stderr, "  dedup      Assign pending documents into canonical stories")
	fmt.Fprintln(os.Stderr, "  process    Run normalize + embed + dedup in sequence")
	fmt.Fprintln(os.Stderr, "  run-once   Alias for process")
	fmt.Fprintln(os.Stderr, "  serve      Start Echo API server")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Use \"scoop <command> -h\" for command-specific flags.")
}
