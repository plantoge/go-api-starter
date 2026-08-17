package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"go-api-starter/cmd/cli/commands"
	"go-api-starter/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmdKey, rest := splitCommand(os.Args[1:])
	handler, ok := commands.Lookup(cmdKey)
	if !ok {
		fmt.Fprintf(os.Stderr, "perintah tidak dikenal: %q\n\n", cmdKey)
		printUsage()
		os.Exit(1)
	}

	if err := handler(rest); err != nil {
		var ve *config.ValidationError
		if errors.As(err, &ve) {
			fmt.Fprintln(os.Stderr, ve.Multiline())
		} else {
			fmt.Fprintf(os.Stderr, "gagal: %v\n", err)
		}
		os.Exit(1)
	}
}

// splitCommand ambil argumen depan yang bukan flag sebagai nama perintah
// (misal "migrate tenant up"), sisanya dianggap flag. Jadi kita bisa nulis
// `cli migrate tenant up --tenant=acme_corp`.
//
// Yang dicek awalan satu strip "-", bukan cuma "--". Soalnya package flag
// bawaan Go nerima dua-duanya (`-all` atau `--all`), jadi flag berstrip
// satu juga harus bikin pembacaan nama perintah berhenti. Kalau nggak,
// flag itu ikut kebaca jadi bagian nama perintah dan ujungnya malah
// dilaporin "perintah tidak dikenal".
func splitCommand(args []string) (cmdKey string, rest []string) {
	var parts []string
	i := 0
	for i < len(args) && !strings.HasPrefix(args[i], "-") {
		parts = append(parts, args[i])
		i++
	}
	return strings.Join(parts, " "), args[i:]
}

func printUsage() {
	fmt.Println("cara pakai: cli <perintah> [flag]")
	fmt.Println("\nperintah yang tersedia:")
	for _, name := range commands.Names() {
		fmt.Println("  " + name)
	}
}
