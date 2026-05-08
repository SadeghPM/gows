package main

import "log"

func isDebugMode() bool {
	return serverMode == "debug"
}

func debugLog(format string, v ...interface{}) {
	if isDebugMode() {
		log.Printf(format, v...)
	}
}

func infoLog(format string, v ...interface{}) {
	log.Printf(format, v...)
}
