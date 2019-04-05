package com

import (
	"errors"
	"time"
)

var ErrNil = errors.New("resource is nil")
var ErrNoToken = errors.New("runner requires a token")
var ErrNoID = errors.New("resource ID is not set")
var ErrNotFound = errors.New("resource not found")

type Runner struct {
	ID          int64
	Token       string
	Description string
	Tags        []string
	RunUntagged bool
	Locked      bool
	MaxTimeout  time.Duration // <= 0 -> System limit
	Active      bool
	Deleted     bool
}

func (r *Runner) CanCreate() error {
	if r == nil {
		return ErrNil
	}
	if r.Token == "" {
		return ErrNoToken
	}
	return nil
}
