package main

import (
	"fmt"
	"os"
	"os/exec"
)

// Main entry point for the tools command.
// It dispatches to subcommands like "bootstrap" and "seed"
// which are implemented as separate Go programs under cmd/bootstrap and cmd/seed.
func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]
	args := os.Args[2:]

	switch subcommand {
	case "bootstrap":
		runGoCommand("./cmd/bootstrap", args)
	case "seed":
		runGoCommand("./cmd/seed", args)
	default:
		fmt.Fprintf(os.Stderr, "unknown tools subcommand: %s\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

// runGoCommand executes a "go run" command for the specified target and arguments
func runGoCommand(target string, args []string) {
	cmdArgs := append([]string{"run", target}, args...)
	// #nosec G204,G702 -- target is constrained by switch, and args are intentional pass-through CLI flags.
	cmd := exec.Command("go", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

// printUsage prints the usage instructions for the tools command and its subcommands
func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/tools bootstrap -- -email admin@domain.com -password StrongPass123!")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/tools seed -- --categories=seeds/products_category.json --products=seeds/products_product.json")
}
