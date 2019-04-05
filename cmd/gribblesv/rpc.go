package main

import (
	"crypto/rand"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	com "go.spiff.io/gribble/internal/common"
	gciwire "go.spiff.io/gribble/internal/gci-wire"
)

var (
	errBadRequest = ErrorRep{"bad request"}
)

const (
	defaultTokenLength = 19
	runnerTokenLen     = 19
)

type Server struct {
	toker *randomToken
	rng   io.Reader
	db    DB
}

type ServerConfig struct {
	TokenLength int
	RandSource  io.Reader
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

	tok, _ := toker.Token()
	log.Printf("Server token: %q", tok)

	return &Server{
		toker: toker,
		rng:   rng,
		db:    db,
	}, nil
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
		RunUntagged: body.RunUntagged,
		MaxTimeout:  time.Duration(body.MaximumTimeout) * time.Second,
		Tags:        runnerTags(body.Tags),
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
		log.Printf("Unable to consume trace: %v job=%s", err)
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

	return http.StatusNoContent, nil
}

func runnerTags(tags string) []string {
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
