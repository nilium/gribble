// +build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

func shutdownSignals() []os.Signal {
	return []os.Signal{windows.SIGINT, windows.SIGTERM}
}
