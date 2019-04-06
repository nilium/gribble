package com

import (
	"errors"
	"strings"
	"time"

	gciwire "go.spiff.io/gribble/internal/gci-wire"
)

var (
	ErrNil      = errors.New("resource is nil")
	ErrNoToken  = errors.New("runner requires a token")
	ErrNoID     = errors.New("resource ID is not set")
	ErrNotFound = errors.New("resource not found")

	// ErrHasID is returned for resources that cannot be created because their IDs must be
	// assigned by a database.
	ErrHasID = errors.New("cannot preassign an ID to this resource")
)

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
	Created     time.Time
	Updated     time.Time
}

func (r *Runner) CanCreate() error {
	if r == nil {
		return ErrNil
	}
	if r.ID != 0 {
		return ErrHasID
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

type Feature int64

func (f Feature) IsSet(flags Feature) bool {
	return f&flags == flags
}

const (
	FeatureVariables               Feature = 1 << 0
	FeatureImage                   Feature = 1 << 1
	FeatureServices                Feature = 1 << 2
	FeatureArtifacts               Feature = 1 << 3
	FeatureCache                   Feature = 1 << 4
	FeatureShared                  Feature = 1 << 5
	FeatureUploadMultipleArtifacts Feature = 1 << 6
	FeatureUploadRawArtifacts      Feature = 1 << 7
	FeatureSession                 Feature = 1 << 8
	FeatureTerminal                Feature = 1 << 9
	FeatureRefspecs                Feature = 1 << 10
	FeatureMasking                 Feature = 1 << 11
)

func FromFeatureFlags(flags Feature) (f gciwire.FeaturesInfo) {
	bits := [12]*bool{
		&f.Variables,
		&f.Image,
		&f.Services,
		&f.Artifacts,
		&f.Cache,
		&f.Shared,
		&f.UploadMultipleArtifacts,
		&f.UploadRawArtifacts,
		&f.Session,
		&f.Terminal,
		&f.Refspecs,
		&f.Masking,
	}
	for i, b := range bits {
		if flags&(1<<uint(i)) != 0 {
			*b = true
		}
	}
	return f
}

func ToFeatureFlags(info gciwire.FeaturesInfo) (flags Feature) {
	bits := [12]bool{
		info.Variables,
		info.Image,
		info.Services,
		info.Artifacts,
		info.Cache,
		info.Shared,
		info.UploadMultipleArtifacts,
		info.UploadRawArtifacts,
		info.Session,
		info.Terminal,
		info.Refspecs,
		info.Masking,
	}
	for i, b := range bits {
		if b {
			flags |= (1 << uint(i))
		}
	}
	return flags
}
