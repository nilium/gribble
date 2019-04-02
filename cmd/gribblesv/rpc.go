package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"sync"

	"github.com/julienschmidt/httprouter"
	gciwire "go.spiff.io/gribble/internal/gci-wire"
)

var (
	errBadRequest = ErrorRep{"bad request"}
)

type Server struct {
	serveSampleJob sync.Once
}

func (s *Server) RegisterRunner(w http.ResponseWriter, req *http.Request, params httprouter.Params) (int, interface{}) {
	var body gciwire.RegisterRunnerRequest
	if err := ReadJSON(req.Body, &body); err != nil {
		return http.StatusBadRequest, errBadRequest
	}

	rep := gciwire.RegisterRunnerResponse{
		Token: "foobar",
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

	code = http.StatusNoContent
	var rep gciwire.JobResponse

	s.serveSampleJob.Do(func() {
		rep = gciwire.JobResponse{
			ID:            1,
			Token:         "12345678",
			AllowGitFetch: false,
			JobInfo: gciwire.JobInfo{
				Name:        "sample-job",
				Stage:       "build",
				ProjectID:   0,
				ProjectName: "github.com/nilium/codf",
			},
			GitInfo: gciwire.GitInfo{
				RepoURL:   "https://github.com/nilium/codf.git",
				Ref:       "master",
				Sha:       "f20916450a27975500ddfbf3b593d619f0338977",
				BeforeSha: "0000000000000000000000000000000000000000",
				RefType:   gciwire.RefTypeBranch,
				Refspecs: []string{
					"+refs/heads/master:origin/reads/heads/master",
				},
			},
			RunnerInfo: gciwire.RunnerInfo{
				Timeout: 60,
			},
			Variables: gciwire.JobVariables{
				gciwire.JobVariable{
					Key:    "PKG",
					Value:  "go.spiff.io/codf",
					Public: true,
				},
			},
			Steps: gciwire.Steps{
				gciwire.Step{
					Name: gciwire.StepNameScript,
					Script: gciwire.StepScript{
						"go test -coverprofile cover.out $PKG\n",
						"go tool cover -func cover.out\n",
					},
					Timeout: 60,
				},
			},
			Image: gciwire.Image{
				Name: "golang:1.12.1",
			},
		}
		code = http.StatusCreated
		msg = &rep
	})

	return
}
