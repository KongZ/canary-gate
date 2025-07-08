package controller

import (
	"github.com/go-logr/logr"
	"github.com/rs/zerolog/log"
)

// LogrAdapter implements the logr.LogSink interface.
type LogrAdapter struct {
}

// Init is called by logr.
func (a *LogrAdapter) Init(info logr.RuntimeInfo) {
	// do nothing here, as we don't need to initialize anything.
}

// Enabled returns true if the logger is enabled. We'll always enable it.
func (a *LogrAdapter) Enabled(level int) bool {
	return true
}

// Info logs an info message.
func (a *LogrAdapter) Info(level int, msg string, keysAndValues ...interface{}) {
	if msg == "Response Body" {
		// do not print response body in info logs
		// the reponse body is out by client-go inside controller-runtime
		log.Trace().Fields(keysAndValues).Msg(msg)
		return
	}
	log.Info().Fields(keysAndValues).Msg(msg)
}

// Error logs an error message.
func (a *LogrAdapter) Error(err error, msg string, keysAndValues ...interface{}) {
	log.Error().Err(err).Fields(keysAndValues).Msg(msg)
}

// WithValues returns a new logger with additional key-value pairs.
func (a *LogrAdapter) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return a
}

// WithName returns a new logger with a name segment. We can ignore it.
func (a *LogrAdapter) WithName(name string) logr.LogSink {
	return a
}
