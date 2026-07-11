package app

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed frontend/*
var frontend embed.FS

func (s *Server) handleFrontend(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "接口不存在")
		return
	}
	sub, err := fs.Sub(frontend, "frontend")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := sub.Open(path); err != nil {
		path = "index.html"
	}
	http.ServeFileFS(w, r, sub, path)
}
