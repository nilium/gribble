package main

import (
	"context"
	"fmt"
	"time"

	com "go.spiff.io/gribble/internal/common"
	"go.spiff.io/gribble/internal/sqlite"
)

type DB interface {
	Close() error
	Migrate(ctx context.Context) error

	CreateRunner(ctx context.Context, r *com.Runner) error
	SetRunnerUpdatedTime(ctx context.Context, r *com.Runner, t time.Time) error
	GetRunnerByToken(ctx context.Context, token string, getDeleted bool) (*com.Runner, error)
	GetRunnerTags(ctx context.Context, r *com.Runner) error
}

type backendError struct {
	Name BackendName
}

func (be *backendError) Error() string {
	return fmt.Sprintf("unrecognized backend %q", be.Name)
}

type BackendName string

func (b BackendName) String() string {
	return string(b)
}

func (b BackendName) MarshalText() ([]byte, error) {
	if _, ok := backends[b]; !ok {
		return nil, &backendError{b}
	}
	return []byte(b), nil
}

func (b *BackendName) UnmarshalText(p []byte) error {
	name := BackendName(p)
	if _, ok := backends[BackendName(name)]; !ok {
		return &backendError{name}
	}
	*b = BackendName(name)
	return nil
}

type BackendFunc func(ctx context.Context, conf *Config) (DB, error)

var backends = map[BackendName]BackendFunc{
	"sqlite":        newSQLiteBackend,
	"sqlite-memory": newSQLiteMemoryBackend, // For testing only
}

func newSQLiteBackend(ctx context.Context, conf *Config) (DB, error) {
	return sqlite.NewFileDB(ctx, conf.SQLiteFile, conf.SQLitePoolSize)
}

func newSQLiteMemoryBackend(ctx context.Context, conf *Config) (DB, error) {
	return sqlite.NewMemoryDB(ctx, conf.SQLitePoolSize)
}

func NewDatabase(ctx context.Context, conf *Config) (DB, error) {
	fn := backends[conf.DB]
	if fn == nil {
		return nil, &backendError{conf.DB}
	}
	return fn(ctx, conf)
}
