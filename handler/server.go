package handler

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

const version = "0.1.1"

// ServerVersion holds the server version information
type ServerVersion struct {
	// Version string
	Version string `json:"version"`
}

type ServerHandler struct {
}

// Version handles the /version endpoint, returning the version of the canary-gate server.
func (h *ServerHandler) Version() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		body, err := json.Marshal(&ServerVersion{Version: version})
		if err != nil {
			log.Error().Msgf("Error while marshaling version: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(body); err != nil {
			log.Error().Msgf("Error while writing response: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
