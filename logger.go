package main

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func initLogger(debug bool) {
	level := zerolog.InfoLevel
	if debug {
		level = zerolog.TraceLevel
	}
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.InterfaceMarshalFunc = sonic.Marshal

	// Caller Marshal Function
	zerolog.CallerMarshalFunc = func(_ uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + strconv.Itoa(line)
	}

	l := zerolog.
		New(&zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}).
		With().
		Timestamp().
		Caller().
		Logger()
	log.Logger = l
}
