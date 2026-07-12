package app

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type PluginAuditEvent struct {
	Time      string         `json:"time"`
	Action    string         `json:"action"`
	PluginID  string         `json:"pluginId,omitempty"`
	Status    string         `json:"status"`
	Message   string         `json:"message,omitempty"`
	RemoteIP  string         `json:"remoteIp,omitempty"`
	UserAgent string         `json:"userAgent,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

type PluginAuditStore struct {
	mu   sync.Mutex
	path string
}

func NewPluginAuditStore(path string) *PluginAuditStore {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	return &PluginAuditStore{path: path}
}

func (s *PluginAuditStore) Record(r *http.Request, action, pluginID, status, message string, details map[string]any) {
	if s == nil {
		return
	}
	event := PluginAuditEvent{Time: time.Now().Format(time.RFC3339), Action: action, PluginID: pluginID, Status: status, Message: message, Details: details}
	if r != nil {
		event.RemoteIP = clientIP(r)
		event.UserAgent = r.UserAgent()
	}
	data, _ := json.Marshal(event)
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}

func (s *PluginAuditStore) List(limit int) []PluginAuditEvent {
	if s == nil {
		return []PluginAuditEvent{}
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.Open(s.path)
	if err != nil {
		return []PluginAuditEvent{}
	}
	defer f.Close()
	var events []PluginAuditEvent
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		var event PluginAuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err == nil {
			events = append(events, event)
		}
	}
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	if len(events) > limit {
		events = events[:limit]
	}
	return events
}

func (s *Server) handlePluginAudit(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, map[string]any{"events": s.pluginAudit.List(limit)})
}
