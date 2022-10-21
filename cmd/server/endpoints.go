package main

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/akuityio/bookkeeper"
	"github.com/akuityio/bookkeeper/internal/version"
)

func getRenderRequestHandler(svc bookkeeper.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		logger := logger.WithFields(log.Fields{})

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			// We're going to assume this is because the request body is missing and
			// treat it as a bad request.
			logger.Infof("Error reading request body: %s", err)
			if err = handleError(
				&bookkeeper.ErrBadRequest{
					Reason: "Bookkeeper server was unable to read the request body",
				},
				w,
			); err != nil {
				logger.Error(err)
			}
			return
		}

		req := bookkeeper.RenderRequest{}
		if err = json.Unmarshal(bodyBytes, &req); err != nil {
			// The request body must be malformed.
			logger.Infof("Error unmarshaling request body: %s", err)
			if err = handleError(
				&bookkeeper.ErrBadRequest{
					Reason: "Bookkeeper server was unable to unmarshal the request body",
				},
				w,
			); err != nil {
				logger.Error(err)
			}
			return
		}

		// TODO: We should apply some kind of request body validation

		// Now that we have details from the request body, we can attach some more
		// context to the logger.
		logger = logger.WithFields(log.Fields{
			"repo":         req.RepoURL,
			"targetBranch": req.TargetBranch,
		})

		res, err := svc.RenderConfig(r.Context(), req)
		if err != nil {
			if err = handleError(
				errors.Wrap(err, "error handling request"),
				w,
			); err != nil {
				logger.Error(err)
			}
			return
		}

		if err = writeResponse(w, http.StatusOK, res); err != nil {
			logger.Error(err)
		}
	}
}

func handleVersionRequest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if err := writeResponse(w, http.StatusOK, version.GetVersion()); err != nil {
		logger.Error(err)
	}
}

func handleError(err error, w http.ResponseWriter) error {
	var writeErr error
	switch typedErr := errors.Cause(err).(type) {
	case *bookkeeper.ErrBadRequest:
		writeErr = writeResponse(w, http.StatusBadRequest, typedErr)
	case *bookkeeper.ErrNotFound:
		writeErr = writeResponse(w, http.StatusNotFound, typedErr)
	case *bookkeeper.ErrConflict:
		writeErr = writeResponse(w, http.StatusConflict, typedErr)
	case *bookkeeper.ErrNotSupported:
		writeErr = writeResponse(w, http.StatusNotImplemented, typedErr)
	case *bookkeeper.ErrInternalServer:
		writeErr = writeResponse(w, http.StatusInternalServerError, typedErr)
	default:
		writeErr = writeResponse(
			w,
			http.StatusInternalServerError,
			&bookkeeper.ErrInternalServer{},
		)
	}
	return writeErr
}

func writeResponse(w http.ResponseWriter, statusCode int, response any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	responseBody, err := json.Marshal(response)
	if err != nil {
		return errors.Wrap(err, "error marshaling response body")
	}
	_, err = w.Write(responseBody)
	return errors.Wrap(err, "error writing response body")
}
