//go:build windows && amd64

package main

import (
	"flag"
	"os"
	"sgfg-ng/internal/filterhelper"
)

func main() {
	pipe := flag.String("pipe", "", "private control pipe")
	parent := flag.Uint("parent", 0, "GUI process ID")
	flag.Parse()
	if flag.NArg() != 0 || uint64(*parent) > 0xffffffff {
		os.Exit(2)
	}
	if err := filterhelper.Run(*pipe, uint32(*parent)); err != nil {
		os.Exit(1)
	}
}
