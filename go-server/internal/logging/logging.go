package logging

import (
	"log"

	"github.com/sadegh/gows/internal/config"
)

func isDebugMode() bool {
	return config.Mode() == "debug"
}

func Debug(format string, v ...interface{}) {
	if isDebugMode() {
		log.Printf(format, v...)
	}
}

func Info(format string, v ...interface{}) {
	log.Printf(format, v...)
}
