package main

import (
	"fmt"
	"os"
	"strings"

	"go-api-starter/cmd/cli/commands"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmdKey, rest := splitCommand(os.Args[1:])
	handler, ok := commands.Lookup(cmdKey)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", cmdKey)
		printUsage()
		os.Exit(1)
	}

	if err := handler(rest); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// splitCommand takes the leading non-flag arguments as the command name
// (e.g. "migrate tenant up") and returns the rest as flags, so callers can
// run `cli migrate tenant up --tenant=acme_corp`.
func splitCommand(args []string) (cmdKey string, rest []string) {
	var parts []string
	i := 0
	for i < len(args) && !strings.HasPrefix(args[i], "--") {
		parts = append(parts, args[i])
		i++
	}
	return strings.Join(parts, " "), args[i:]
}

func printUsage() {
	fmt.Println("usage: cli <command> [flags]")
	fmt.Println("\navailable commands:")
	for _, name := range commands.Names() {
		fmt.Println("  " + name)
	}
}
