package main

import (
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/google/go-github/v24/github"
	"github.com/julienschmidt/httprouter"
	com "go.spiff.io/gribble/internal/common"
	gciwire "go.spiff.io/gribble/internal/gci-wire"
	"go.spiff.io/gribble/internal/proc"
)

var (
	errBadRequest = ErrorRep{"bad request"}
)

const (
	defaultTokenLength = 20
	runnerTokenLen     = 48
)

type Server struct {
	mux *httprouter.Router
	db  DB

	toker *randomToken
	rng   io.Reader

	githubToken []byte
}

type ServerConfig struct {
	TokenLength int
	RandSource  io.Reader
	GitHubToken string
}

func (s *ServerConfig) tokenLength() int {
	if s == nil || s.TokenLength <= 0 {
		return defaultTokenLength
	}
	return s.TokenLength
}

func (s *ServerConfig) randReader() io.Reader {
	if s == nil || s.RandSource == nil {
		return rand.Reader
	}
	return s.RandSource
}

func NewServer(conf *ServerConfig, db DB) (*Server, error) {
	rng := conf.randReader()
	tokenLen := conf.tokenLength()
	toker, err := newRandomToken(tokenLen, rng)
	if err != nil {
		return nil, err
	}

	s := &Server{
		mux: httprouter.New(),
		db:  db,

		toker: toker,
		rng:   rng,
	}

	s.mux.POST("/_gitlab/api/v4/runners", HandleJSON(s.RegisterRunner))
	s.mux.POST("/_gitlab/api/v4/jobs/request", HandleJSON(s.RequestJob))
	s.mux.PATCH("/_gitlab/api/v4/jobs/:id/trace", HandleJSON(s.PatchTrace))
	s.mux.PUT("/_gitlab/api/v4/jobs/:id", HandleJSON(s.UpdateJob))

	if token := []byte(conf.GitHubToken); len(token) > 0 {
		if conf.GitHubToken == "DEV" {
			// If token is DEV (uppercase), don't validate payloads by using an empty
			// token. Assigned to nil instead of doing a != for readability's sake.
			// This means you can't have a token that is DEV.
			token = nil
		}
		s.githubToken = token
		s.mux.POST("/v1/events/github", HandleJSON(s.GitHubEvent))
	}

	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.mux.ServeHTTP(w, req)
}

func runnerFetchErrorCode(err error) int {
	switch err {
	case com.ErrNotFound:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

func (s *Server) Token() string {
	tok, _ := s.toker.Token()
	return tok
}

func (s *Server) getRunnerByToken(ctx context.Context, token string, tags bool) (*com.Runner, error) {
	runner, err := s.db.GetRunnerByToken(ctx, token, false)
	if err != nil {
		return nil, err
	}

	now := proc.Now(ctx)
	s.db.SetRunnerUpdatedTime(ctx, runner, now)

	if !tags {
	} else if err = s.db.GetRunnerTags(ctx, runner); err != nil {
		return nil, err
	}
	return runner, err
}

func (s *Server) RegisterRunner(w http.ResponseWriter, req *http.Request, params httprouter.Params) (int, interface{}) {
	var body gciwire.RegisterRunnerRequest
	if err := ReadJSON(req.Body, &body); err != nil {
		return http.StatusBadRequest, errBadRequest
	}

	if s.toker.Compare(body.Token) != nil {
		return http.StatusForbidden, nil
	}

	token, err := genToken(runnerTokenLen, s.rng)
	if err != nil {
		return http.StatusInternalServerError, nil
	}

	runner := &com.Runner{
		Token:       token,
		Description: body.Description,
		Tags:        com.ParseTags(body.Tags),
		RunUntagged: body.RunUntagged,
		MaxTimeout:  time.Duration(body.MaximumTimeout) * time.Second,
		Locked:      body.Locked,
		Active:      body.Active,
	}
	ctx := req.Context()
	if err := s.db.CreateRunner(ctx, runner); err != nil {
		log.Printf("Error creating runner: %v", err)
		return http.StatusInternalServerError, nil
	}

	rep := gciwire.RegisterRunnerResponse{
		Token: token,
	}
	return http.StatusCreated, &rep
}

func (s *Server) PatchTrace(w http.ResponseWriter, req *http.Request, params httprouter.Params) (code int, msg interface{}) {
	trace, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Printf("Unable to consume trace: %v job=%s", err, params[0].Value)
		return http.StatusBadRequest, errBadRequest
	}

	log.Printf("-- START TRACE PATCH --\n%s\n-- END TRACE PATCH --", trace)

	return http.StatusAccepted, nil
}

func (s *Server) UpdateJob(w http.ResponseWriter, req *http.Request, params httprouter.Params) (code int, msg interface{}) {
	var body gciwire.UpdateJobRequest
	if err := ReadJSON(req.Body, &body); err != nil {
		return http.StatusBadRequest, errBadRequest
	}

	if body.Trace != nil {
		log.Printf("-- START TRACE --\n%s\n-- END TRACE --", *body.Trace)
	}

	log.Printf("Update job: %#+v %v %v", body.Info, body.State, body.FailureReason)
	w.Header().Set("Job-Status", string(body.State))
	return http.StatusOK, nil
}

func (s *Server) RequestJob(w http.ResponseWriter, req *http.Request, params httprouter.Params) (code int, msg interface{}) {
	var body gciwire.JobRequest
	if err := ReadJSON(req.Body, &body); err != nil {
		return http.StatusBadRequest, errBadRequest
	}

	ctx := req.Context()
	runner, err := s.getRunnerByToken(ctx, body.Token, true)
	if err != nil {
		return runnerFetchErrorCode(err), nil
	}

	if !runner.Active {
		return http.StatusNoContent, nil
	}

	return http.StatusNoContent, nil
}

func (s *Server) GitHubEvent(w http.ResponseWriter, req *http.Request, params httprouter.Params) (code int, msg interface{}) {
	typ := github.WebHookType(req)
	switch typ {
	case "push", "pull_request", "pull_request_review_comment":
	default:
		return http.StatusNotFound, nil
	}

	body, err := github.ValidatePayload(req, s.githubToken)
	if err != nil {
		return http.StatusForbidden, nil
	}

	payload, err := github.ParseWebHook(typ, body)
	if err != nil {
		return http.StatusBadRequest, nil
	}

	log.Printf("-- WEBHOOK --\n%s\n-- END WEBHOOK --", body)
	log.Print(payload)

	return http.StatusAccepted, nil
}
