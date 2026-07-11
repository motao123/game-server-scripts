package app

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (s *Server) handleFilesRename(w http.ResponseWriter, r *http.Request) {
	var body struct{ OldPath, NewPath string }
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !s.safeRoot(body.OldPath) || !s.safeRoot(filepath.Dir(body.NewPath)) {
		writeError(w, 403, "路径不允许访问")
		return
	}
	err := os.Rename(body.OldPath, body.NewPath)
	writeJSON(w, map[string]any{"ok": err == nil, "error": errString(err)})
}

func (s *Server) handleFilesExtract(w http.ResponseWriter, r *http.Request) {
	var body struct{ Archive, Dest string }
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Dest == "" {
		body.Dest = filepath.Dir(body.Archive)
	}
	if !s.safeRoot(body.Archive) || !s.safeRoot(body.Dest) {
		writeError(w, 403, "路径不允许访问")
		return
	}
	err := extractTarGz(body.Archive, body.Dest)
	writeJSON(w, map[string]any{"ok": err == nil, "error": errString(err)})
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	cleanDest := filepath.Clean(dest)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		cleanName := filepath.Clean(h.Name)
		if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
			continue
		}
		target := filepath.Join(cleanDest, cleanName)
		if !strings.HasPrefix(filepath.Clean(target), cleanDest+string(os.PathSeparator)) && filepath.Clean(target) != cleanDest {
			continue
		}
		if h.FileInfo().IsDir() {
			if err := os.MkdirAll(target, h.FileInfo().Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, h.FileInfo().Mode())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
