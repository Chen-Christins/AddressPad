package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"addresspad/internal/sshcmd"
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
  add            Add a server (--name NAME --host HOST)
  list [--json]  List servers
  import         Import ~/.ssh/config (--dry-run, --overwrite, --ssh-config PATH)
  connect NAME   Connect directly (or use: apad NAME)
  config path    Print the effective configuration path without creating it
  help           Show help

Without a command, open the interactive server list.
In the list: enter connects using OpenSSH; c copies the SSH command.
In the list: a adds, e edits, d deletes (with confirmation).
Run apad help COMMAND for command-specific help.
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
	if len(pos) > 0 && pos[0] == "help" {
		if len(pos) == 1 {
			fmt.Fprint(out, usage)
			return 0
		}
		if len(pos) == 2 {
			if text, ok := commandHelp[pos[1]]; ok {
				fmt.Fprint(out, text)
				return 0
			}
		}
		fmt.Fprintln(errOut, "unknown command for help")
		return 2
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
	path, err := store.ResolvePath(*config)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	c := cli{path: path, in: input, out: out, stderr: errOut, command: sshcmd.Command}
	if len(pos) > 0 {
		switch pos[0] {
		case "add":
			return c.add(pos[1:])
		case "list":
			return c.list(pos[1:])
		case "import":
			return c.importConfig(pos[1:])
		case "connect":
			if len(pos) == 2 && (pos[1] == "--help" || pos[1] == "-h") {
				fmt.Fprint(out, commandHelp["connect"])
				return 0
			}
			if len(pos) != 2 || strings.TrimSpace(pos[1]) == "" {
				fmt.Fprint(errOut, commandHelp["connect"])
				return 2
			}
			return c.connect(pos[1])
		case "config":
			fmt.Fprintln(errOut, "unknown command or arguments; use config path")
			return 2
		default:
			if len(pos) != 1 {
				fmt.Fprintln(errOut, "unknown command or arguments")
				return 2
			}
			return c.connect(pos[0])
		}
	}
	if err == nil {
		err = launch(path, input, out)
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}
