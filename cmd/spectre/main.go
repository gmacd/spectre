// Command spectre is a single-process personal AI chat assistant: a daemon
// (serve) that talks to an OpenAI-compatible LLM backend and persists
// conversation history, plus a CLI client (send) that talks to the daemon.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "serve":
		err = runServe(os.Args[2:])
	case "send":
		err = runSend(os.Args[2:])
	case "version":
		runVersion()
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "spectre:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage: spectre <command> [flags]

Commands:
  serve    run the spectre daemon
  send     send a message to a running daemon
  version  print build information`)
}
