package com

import (
	"errors"
	"strings"
	"time"

	gciwire "go.spiff.io/gribble/internal/gci-wire"
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

func ParseTags(tags string) []string {
	r := strings.Split(tags, ",")
	t := r[:0]
	for _, tag := range r {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		t = append(t, tag)
	}
	return t
}

type Job struct {
	ID     int64
	Runner int64
	State  gciwire.JobState
	Spec   *JobSpec
}

type JobSpec struct {
	GitLab gciwire.JobResponse `json:"gitlab,omitempty"`
}
