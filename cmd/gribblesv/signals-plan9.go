// +build plan9

package main

import (
	"os"
	"syscall"
)

func shutdownSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}
