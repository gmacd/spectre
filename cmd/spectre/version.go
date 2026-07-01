package main

import (
	"fmt"
	"runtime/debug"
)

func runVersion() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("spectre (build info unavailable)")
		return
	}

	version := info.Main.Version
	if version == "" {
		version = "(devel)"
	}
	fmt.Printf("spectre %s (%s)\n", version, info.GoVersion)

	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			fmt.Println("commit:", s.Value)
		}
	}
}
