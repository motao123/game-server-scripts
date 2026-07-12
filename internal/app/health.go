package app

import (
	"net/http"
	"runtime"
	"time"
)

var startTime = time.Now()

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"status":     "ok",
		"uptime":     int(time.Since(startTime).Seconds()),
		"goroutines": runtime.NumGoroutine(),
		"version":    "0.1.0",
	})
}
