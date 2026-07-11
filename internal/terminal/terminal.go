package terminal

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const maxBuffer = 1000

type Session struct {
	ID           string
	Cmd          *exec.Cmd
	PTY          *os.File
	Done         chan struct{}
	output       []string
	mu           sync.Mutex
	createdAt    time.Time
	lastActivity time.Time
	disconnected bool
}

type PersistedSession struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	WorkingDir   string `json:"workingDirectory"`
	CreatedAt    string `json:"createdAt"`
	LastActivity string `json:"lastActivity"`
	IsActive     bool   `json:"isActive"`
}

type Manager struct {
	mu          sync.Mutex
	sessions    map[string]*Session
	persistPath string
}

func NewManager() *Manager {
	m := &Manager{sessions: map[string]*Session{}, persistPath: "data/terminal-sessions.json"}
	m.loadPersisted()
	return m
}

func (m *Manager) List() []PersistedSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []PersistedSession{}
	for _, s := range m.sessions {
		s.mu.Lock()
		out = append(out, PersistedSession{
			ID: s.ID, WorkingDir: s.Cmd.Dir, CreatedAt: s.createdAt.Format(time.RFC3339), LastActivity: s.lastActivity.Format(time.RFC3339), IsActive: !s.disconnected,
		})
		s.mu.Unlock()
	}
	return out
}

func (m *Manager) Start(conn *websocket.Conn, shell, cwd string, cols, rows uint16) (string, error) {
	if shell == "" {
		if runtime.GOOS == "windows" {
			shell = "powershell.exe"
		} else {
			shell = "/bin/bash"
		}
	}
	cmd := exec.Command(shell)
	if cwd != "" {
		cmd.Dir = cwd
	}
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: cols, Rows: rows})
	if err != nil {
		return "", err
	}
	id := uuid.NewString()
	s := &Session{ID: id, Cmd: cmd, PTY: ptmx, Done: make(chan struct{}), createdAt: time.Now(), lastActivity: time.Now()}
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	go m.readLoop(s, conn)
	m.persist()
	return id, nil
}

func (m *Manager) readLoop(s *Session, conn *websocket.Conn) {
	buf := make([]byte, 4096)
	for {
		n, err := s.PTY.Read(buf)
		if n > 0 {
			data := string(buf[:n])
			s.mu.Lock()
			s.output = append(s.output, data)
			if len(s.output) > maxBuffer {
				s.output = s.output[len(s.output)-maxBuffer:]
			}
			s.lastActivity = time.Now()
			s.mu.Unlock()
			_ = conn.WriteJSON(map[string]any{"type": "terminal-output", "sessionId": s.ID, "data": data})
		}
		if err != nil {
			if err != io.EOF {
				_ = conn.WriteJSON(map[string]any{"type": "terminal-exit", "sessionId": s.ID, "error": err.Error()})
			} else {
				_ = conn.WriteJSON(map[string]any{"type": "terminal-exit", "sessionId": s.ID})
			}
			break
		}
	}
	m.Close(s.ID)
}

func (m *Manager) Write(id, data string) error {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s == nil {
		return os.ErrNotExist
	}
	s.mu.Lock()
	s.lastActivity = time.Now()
	s.mu.Unlock()
	_, err := s.PTY.Write([]byte(data))
	return err
}

func (m *Manager) Resize(id string, cols, rows uint16) error {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s == nil {
		return os.ErrNotExist
	}
	return pty.Setsize(s.PTY, &pty.Winsize{Cols: cols, Rows: rows})
}

func (m *Manager) Reconnect(conn *websocket.Conn, id string) bool {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s == nil {
		return false
	}
	s.mu.Lock()
	s.disconnected = false
	historical := ""
	for _, chunk := range s.output {
		historical += chunk
	}
	s.mu.Unlock()
	if historical != "" {
		_ = conn.WriteJSON(map[string]any{"type": "terminal-output", "sessionId": id, "data": historical, "isHistorical": true})
	}
	_ = conn.WriteJSON(map[string]any{"type": "session-reconnected", "sessionId": id})
	return true
}

func (m *Manager) MarkDisconnected(id string) {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s != nil {
		s.mu.Lock()
		s.disconnected = true
		s.mu.Unlock()
	}
}

func (m *Manager) Close(id string) {
	m.mu.Lock()
	s := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if s != nil {
		_ = s.PTY.Close()
		if s.Cmd.Process != nil {
			_ = s.Cmd.Process.Kill()
		}
	}
	m.persist()
}

func (m *Manager) persist() {
	m.mu.Lock()
	sessions := make([]PersistedSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		s.mu.Lock()
		sessions = append(sessions, PersistedSession{
			ID: s.ID, WorkingDir: s.Cmd.Dir, CreatedAt: s.createdAt.Format(time.RFC3339), LastActivity: s.lastActivity.Format(time.RFC3339), IsActive: !s.disconnected,
		})
		s.mu.Unlock()
	}
	m.mu.Unlock()
	_ = os.MkdirAll("data", 0755)
	data, _ := json.MarshalIndent(sessions, "", "  ")
	_ = os.WriteFile(m.persistPath, data, 0644)
}

func (m *Manager) loadPersisted() {
	data, err := os.ReadFile(m.persistPath)
	if err != nil {
		return
	}
	var sessions []PersistedSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return
	}
}
