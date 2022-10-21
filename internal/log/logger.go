package bookkeeper

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func LoggerOrDie() *log.Logger {
	logLevel := log.InfoLevel
	logLevelStr := os.Getenv("BOOKKEEPER_LOG_LEVEL")
	if logLevelStr != "" {
		var err error
		if logLevel, err = log.ParseLevel(logLevelStr); err != nil {
			log.Fatal(err)
		}
	}
	logger := log.New()
	logger.SetLevel(logLevel)
	return logger
}
