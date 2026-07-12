package app

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if subtle.ConstantTimeCompare([]byte(body.OldPassword), []byte(s.auth.Password())) != 1 {
		writeError(w, http.StatusUnauthorized, "旧密码错误")
		return
	}
	if len(body.NewPassword) < 6 {
		writeError(w, http.StatusBadRequest, "新密码至少 6 位")
		return
	}
	s.auth.SetPassword(body.NewPassword)
	writeJSON(w, map[string]any{"ok": true, "message": "密码已修改"})
}
