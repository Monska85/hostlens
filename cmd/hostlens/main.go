package main

import (
	"fmt"
	"github.com/Monska85/hostlens/internal/cli"
	"os"
)

func main() {
	if e := cli.Main(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
