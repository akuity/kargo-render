package main

import (
	"github.com/akuityio/bookkeeper/internal/http"
	libOS "github.com/akuityio/bookkeeper/internal/os"
)

// serverConfig populates configuration for the HTTP/S server from environment
// variables.
func serverConfig() (http.ServerConfig, error) {
	config := http.ServerConfig{}
	var err error
	config.Port, err = libOS.GetIntFromEnvVar("PORT", 8080)
	if err != nil {
		return config, err
	}
	config.TLSEnabled, err =
		libOS.GetBoolFromEnvVar("BOOKKEEPER_TLS_ENABLED", false)
	if err != nil {
		return config, err
	}
	if config.TLSEnabled {
		config.TLSCertPath, err =
			libOS.GetRequiredEnvVar("BOOKKEEPER_TLS_CERT_PATH")
		if err != nil {
			return config, err
		}
		config.TLSKeyPath, err =
			libOS.GetRequiredEnvVar("BOOKKEEPER_TLS_KEY_PATH")
		if err != nil {
			return config, err
		}
	}
	return config, nil
}
