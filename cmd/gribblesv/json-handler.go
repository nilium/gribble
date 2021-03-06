package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"go.spiff.io/gribble/internal/proc"
	"go.uber.org/zap"
)

const megabyte = 1024 * 1024

var (
	errInternalServerError = ErrorRep{Msg: "internal server error"}
)

// ErrorRep is an error message that is written in response to a request.
type ErrorRep struct {
	Msg string `json:"error"`
}

// JSONHandle is an httprouter.Handle that returns a status code and a JSON response object.
type JSONHandle func(w http.ResponseWriter, req *http.Request, params httprouter.Params) (code int, msg interface{})

// ReadJSON reads r as JSON and unmarshals into dest.
func ReadJSON(r io.Reader, dest interface{}) error {
	p, err := ioutil.ReadAll(r)
	if err != nil {
		return fmt.Errorf("error reading JSON body: %w", err)
	}
	if err := json.Unmarshal(p, dest); err != nil {
		return fmt.Errorf("error parsing JSON body: %w", err)
	}
	return nil
}

// HandleJSON returns an httprouter Handler function that wraps a JSON handler function.
//
// The JSON handler function must have the following form:
//
//      func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (code int, rep interface{})
//
// The rep interface{} returned from the JSON handler is encoded as the JSON response body. The code
// returned is the HTTP status code. If the rep returned is an error, the error is written in
// response as an ErrorRep.
func HandleJSON(fn JSONHandle) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		code := http.StatusInternalServerError
		var rep interface{} = errInternalServerError
		defer func() {
			rc := recover()
			writeRep(w, code, rep, req)
			if rc != nil {
				panic(rc)
			}
		}()
		code, rep = fn(w, req, params)
	}
}

func writeRep(w http.ResponseWriter, code int, rep interface{}, req *http.Request) {
	ctx := req.Context()
	err, _ := rep.(error)
	if err != nil {
		if code == 0 {
			code = http.StatusInternalServerError
		}
		rep = ErrorRep{Msg: err.Error()}
	} else if code == 0 {
		code = http.StatusOK
	}

	var p []byte
	if code == http.StatusNoContent || rep == nil {
	} else if p, err = json.Marshal(rep); err != nil {
		code = http.StatusInternalServerError
		rep = ErrorRep{Msg: "error encoding response"}
		p, _ = json.Marshal(rep)
		proc.Error(ctx, "Error encoding response",
			zap.String("http_request", req.RequestURI),
			zap.Error(err),
		)
	}

	if p != nil {
		w.Header().Set("Content-Length", strconv.Itoa(len(p)))
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if p != nil {
		_, _ = w.Write(p)
	}
}
