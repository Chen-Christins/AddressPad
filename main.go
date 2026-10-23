package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"addresspad/internal/store"
	"addresspad/internal/ui"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

const usage = `AddressPad — terminal SSH address book

Usage: apad [--config PATH] [command]

Options:
  --config PATH  Override ADDRESSPAD_CONFIG and the default configuration path
  --version      Print version, commit and build date
  -h, --help     Show help

Commands:
  config path    Print the effective configuration path without creating it
  help           Show help

Without a command, open the interactive server list.
Connection, forms and import commands are not implemented yet.
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	return runWithUI(args, os.Stdin, out, errOut, ui.Run)
}

func runWithUI(args []string, input io.Reader, out, errOut io.Writer, launch func(string, io.Reader, io.Writer) error) int {
	flags := flag.NewFlagSet("apad", flag.ContinueOnError)
	flags.SetOutput(errOut)
	config := flags.String("config", "", "configuration path")
	showVersion := flags.Bool("version", false, "print version")
	flags.Usage = func() { fmt.Fprint(errOut, usage) }
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Fprintf(out, "apad %s (commit %s, built %s)\n", version, commit, date)
		return 0
	}
	pos := flags.Args()
	if len(pos) == 1 && pos[0] == "help" {
		fmt.Fprint(out, usage)
		return 0
	}
	if len(pos) == 2 && pos[0] == "config" && pos[1] == "path" {
		path, err := store.ResolvePath(*config)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		fmt.Fprintln(out, path)
		return 0
	}
	if len(pos) != 0 {
		fmt.Fprintf(errOut, "unknown command or arguments: %v\nRun apad --help for usage.\n", pos)
		return 2
	}
	path, err := store.ResolvePath(*config)
	if err == nil {
		err = launch(path, input, out)
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}
