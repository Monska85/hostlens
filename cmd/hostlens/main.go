package main

import (
	"fmt"
	"github.com/Monska85/hostlens/internal/app"
	"os"
)

func main() {
	if e := app.Main(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
