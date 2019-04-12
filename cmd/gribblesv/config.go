package main

import (
	"encoding"
	"flag"
	"fmt"
	"time"

	"github.com/hashicorp/go-sockaddr"
	"go.uber.org/zap/zapcore"
)

const (
	defaultListenAddr  = "127.0.0.1:4077"
	defaultGracePeriod = time.Second * 30

	defaultBackendName BackendName = "sqlite"

	defaultSQLiteFile     = "gribble.db"
	defaultSQLitePoolSize = 8

	defaultLogLevel = zapcore.InfoLevel
)

func DefaultConfig() *Config {
	return &Config{
		Listen:      newSockAddr(defaultListenAddr),
		GracePeriod: defaultGracePeriod,

		DB: defaultBackendName,
		// SQLite defaults
		SQLiteFile:     defaultSQLiteFile,
		SQLitePoolSize: defaultSQLitePoolSize,
	}
}

type Config struct {
	// Listen is the address the HTTP server should bind to.
	Listen *SockAddr `envi:"HTTP_LISTEN_ADDR"`
	// GracePeriod is how long the HTTP server will wait to finalize requests and shut down.
	GracePeriod time.Duration `envi:"HTTP_GRACE_PERIOD"`

	// GitHubToken is the webhook token used to validate incoming events.
	GitHubToken string `envi:"GITHUB_TOKEN"`

	// DB is any valid database supported by gribble.
	// These are declared under backend.go in the backends map.
	DB BackendName `envi:"BACKEND"`

	// SQLite (go.spiff.io/gribble/internal/sqlite)
	// Driver: sqlite, sqlite-memory
	SQLiteFile     string `envi:"SQLITE_FILE"`      // sqlite
	SQLitePoolSize int    `envi:"SQLITE_POOL_SIZE"` // sqlite, sqlite-memory

	// LogLevel is the initial logging level of the process.
	LogLevel zapcore.Level `envi:"LOG_LEVEL"`
	// LogJSON, if true, will cause the process to emit logs in JSON format.
	LogJSON bool `envi:"LOG_JSON"`
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
