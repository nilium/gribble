package main

import (
	"encoding"
	"flag"
	"fmt"
	"time"

	"github.com/hashicorp/go-sockaddr"
)

type Config struct {
	Listen *SockAddr

	// GracePeriod is how long the HTTP server will wait to finalize requests and shut down.
	GracePeriod time.Duration
}

type SockAddr struct {
	sockaddr.SockAddr
}

// newSockAddr returns a new SockAddr for the address a.
// If an error occurs, newSockAddr panics.
//
// This should only be used to create SockAddrs at startup when the SockAddr is known to be valid.
func newSockAddr(a string) *SockAddr {
	addr, err := sockaddr.NewSockAddr(a)
	if err != nil {
		panic(err)
	}
	return &SockAddr{addr}
}

func (a *SockAddr) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

func (a *SockAddr) UnmarshalText(p []byte) error {
	addr, err := sockaddr.NewSockAddr(string(p))
	if err != nil {
		return err
	}
	*a = SockAddr{addr}
	return nil
}

type TextValue interface {
	encoding.TextUnmarshaler
	fmt.Stringer
}

type TextFlag struct {
	def  string
	dest TextValue
}

var _ flag.Value = (*TextFlag)(nil)

func NewTextFlag(dest TextValue) *TextFlag {
	return &TextFlag{dest.String(), dest}
}

func (t *TextFlag) String() string {
	return t.def
}

func (t *TextFlag) Set(s string) error {
	return t.dest.UnmarshalText([]byte(s))
}
