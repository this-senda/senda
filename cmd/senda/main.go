// Command senda is the single git-native API client binary. It is pure Go (no
// CGO/webview) so it runs anywhere — servers, CI, containers. The desktop GUI
// ships as a separate `senda-desktop` binary (it links GTK4/WebKit); `senda gui`
// just launches it.
//
//	senda                      interactive terminal UI (default, needs a TTY)
//	senda tui [-collection ...] interactive terminal UI (explicit)
//	senda run [-collection ...] run a collection headlessly (CI-friendly)
//	senda mock [...]            local mock server
//	senda docs [...]            render API docs (Markdown/HTML)
//	senda gui [...]             launch the desktop app
//	senda -h | -v              help / version
package main

import (
	"fmt"
	"os"

	"senda/internal/buildinfo"
	"senda/internal/tui"
)

func main() {
	args := os.Args[1:]

	// Bare `senda`: launch the TUI when attached to a terminal, otherwise print
	// usage. The fallback keeps `senda` in a pipe/CI from dropping into a
	// full-screen UI that has no TTY to drive it.
	if len(args) == 0 {
		if isTerminal(os.Stdin) && isTerminal(os.Stdout) {
			if err := tui.Run(nil); err != nil {
				fatal(err)
			}
			return
		}
		usage(os.Stdout)
		return
	}

	switch args[0] {
	case "run":
		runHeadless(args[1:])
	case "mock":
		runMock(args[1:])
	case "docs":
		runDocsCmd(args[1:])
	case "tui":
		if err := tui.Run(args[1:]); err != nil {
			fatal(err)
		}
	case "gui":
		runGUI(args[1:])
	case "-h", "--help", "help":
		usage(os.Stdout)
	case "-v", "--version", "version":
		bi := buildinfo.Get()
		line := "senda " + bi.Version
		if bi.Commit != "" {
			line += " (" + bi.Commit + ")"
		}
		if bi.Date != "" {
			line += " " + bi.Date
		}
		fmt.Println(line)
	default:
		// A leading flag (e.g. `senda -collection ./api`) is shorthand for the
		// default TUI with its flags.
		if args[0][0] == '-' {
			if err := tui.Run(args); err != nil {
				fatal(err)
			}
			return
		}
		// `senda ./my-api` (code-style): a directory argument opens that
		// collection in the TUI.
		if fi, err := os.Stat(args[0]); err == nil && fi.IsDir() {
			if err := tui.Run(append([]string{"-collection", args[0]}, args[1:]...)); err != nil {
				fatal(err)
			}
			return
		}
		fmt.Fprintf(os.Stderr, "senda: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		os.Exit(2)
	}
}

func usage(w *os.File) {
	fmt.Fprint(w, `senda — fast, git-native API client

Usage:
  senda                       interactive terminal UI (default)
  senda tui [-collection dir] [-env name]
  senda run [-collection dir] [-folder sub] [-env name] [-q] [-data file]
  senda run --docs [-o file] [--docs-format md|html]
  senda mock [-collection dir] [-addr :8787] [-scenario name]
  senda mock init <preset> [-collection dir]
  senda docs [-collection dir] [-folder sub] [-o file] [--docs-format md|html]
  senda gui [args...]         launch the desktop app (senda-desktop)

  senda -h                    this help
  senda -v                    version
`)
}

// isTerminal reports whether f is attached to a character device (a TTY),
// without pulling in a terminal dependency.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
