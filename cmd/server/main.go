package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/akuityio/bookkeeper"
	libHTTP "github.com/akuityio/bookkeeper/internal/http"
	"github.com/akuityio/bookkeeper/internal/version"
)

func main() {
	version := version.GetVersion()

	if len(os.Args) > 1 && os.Args[1] == "version" {
		versionBytes, err := json.MarshalIndent(version, "", "  ")
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Println(string(versionBytes))
		return
	}

	serverConfig, err := serverConfig()
	if err != nil {
		logger.Fatal(err)
	}

	var tlsStatus = "disabled"
	if serverConfig.TLSEnabled {
		tlsStatus = "enabled"
	}
	logger.WithFields(log.Fields{
		"version": version.Version,
		"commit":  version.GitCommit,
		"port":    serverConfig.Port,
		"tls":     tlsStatus,
	}).Info("Starting Bookkeeper Server")

	router := mux.NewRouter()
	router.StrictSlash(true)
	router.HandleFunc(
		"/v1alpha1/render",
		getRenderRequestHandler(bookkeeper.NewService()),
	).Methods(http.MethodPost)
	router.HandleFunc("/version", handleVersionRequest).Methods(http.MethodGet)
	router.HandleFunc("/healthz", libHTTP.Healthz).Methods(http.MethodGet)

	if err := libHTTP.NewServer(
		router,
		&serverConfig,
	).ListenAndServe(context.Background()); err != nil {
		logger.Fatal(err)
	}
}
