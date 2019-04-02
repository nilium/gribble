// +build unix linux solaris darwin netbsd openbsd freebsd aix

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

func shutdownSignals() []os.Signal {
	return []os.Signal{unix.SIGINT, unix.SIGTERM}
}
