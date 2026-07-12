package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

type Manager struct {
	password string
	mu       sync.Mutex
	sessions map[string]Session
	attempts map[string][]time.Time
}

type Session struct {
	Token     string    `json:"token"`
	CSRF      string    `json:"csrf"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func NewManager(password string) *Manager {
	return &Manager{
		password: password,
		sessions: make(map[string]Session),
		attempts: make(map[string][]time.Time),
	}
}

func (m *Manager) Password() string { return m.password }

func (m *Manager) SetPassword(pw string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.password = pw
}

func (m *Manager) Login(ip, password string) (Session, bool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	attempts := m.attempts[ip]
	kept := attempts[:0]
	for _, t := range attempts {
		if now.Sub(t) < time.Minute {
			kept = append(kept, t)
		}
	}
	m.attempts[ip] = kept
	if len(kept) >= 5 {
		return Session{}, false, true
	}

	if subtle.ConstantTimeCompare([]byte(password), []byte(m.password)) != 1 || m.password == "" {
		m.attempts[ip] = append(m.attempts[ip], now)
		return Session{}, false, false
	}

	s := Session{Token: randomToken(32), CSRF: randomToken(18), ExpiresAt: now.Add(2 * time.Hour)}
	m.sessions[s.Token] = s
	delete(m.attempts, ip)
	return s, true, false
}

func (m *Manager) Validate(token string) (Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[token]
	if !ok || time.Now().After(s.ExpiresAt) {
		delete(m.sessions, token)
		return Session{}, false
	}
	return s, true
}

func (m *Manager) Logout(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
}

func (m *Manager) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := m.RequestSession(r); !ok {
			http.Error(w, `{"error":"未登录"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Manager) RequestSession(r *http.Request) (Session, bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return Session{}, false
	}
	return m.Validate(cookie.Value)
}

func (m *Manager) RequireCSRF(r *http.Request) bool {
	s, ok := m.RequestSession(r)
	if !ok {
		return false
	}
	return r.Header.Get("X-CSRF-Token") == s.CSRF
}

func SetSessionCookie(w http.ResponseWriter, s Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    s.Token,
		Path:     "/",
		Expires:  s.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
