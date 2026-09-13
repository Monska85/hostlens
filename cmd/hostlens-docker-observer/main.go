// Command hostlens-docker-observer is the isolated Docker observer. It is the
// only HostLens executable permitted to open the system-wide Docker control
// socket, and it exposes the fixed, typed, GET-only observation contract over
// local IPC.
package main

import (
	"fmt"
	"os"

	"github.com/Monska85/hostlens/internal/observerapp"
)

func main() {
	if err := observerapp.Main(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
