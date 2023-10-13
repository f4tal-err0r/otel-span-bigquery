package main

import (
	"os"

	log "github.com/sirupsen/logrus"
)

var (
	WarnLogger *log.Logger
	InfoLogger *log.Logger
	ErrLogger  *log.Logger
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)

	lvl, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		lvl = "info" //Default log level, just change the env var to debug or some other log level if needed
	}

	ll, err := log.ParseLevel(lvl)
	if err != nil {
		ll = log.InfoLevel
	}
	// set global log level
	log.SetLevel(ll)
}
