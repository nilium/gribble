package main

import (
	"context"
	"time"

	com "go.spiff.io/gribble/internal/common"
)

type DB interface {
	Migrate(ctx context.Context) error

	CreateRunner(ctx context.Context, r *com.Runner) error
	SetRunnerUpdatedTime(ctx context.Context, r *com.Runner, t time.Time) error
	GetRunnerByToken(ctx context.Context, token string, getDeleted bool) (*com.Runner, error)
	GetRunnerTags(ctx context.Context, r *com.Runner) error
}
