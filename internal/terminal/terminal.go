package terminal

import (
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/creack/pty"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

type Session struct {
	ID   string
	Cmd  *exec.Cmd
	PTY  *os.File
	Done chan struct{}
}

func NewManager() *Manager { return &Manager{sessions: map[string]*Session{}} }

func (m *Manager) List() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids
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
	s := &Session{ID: id, Cmd: cmd, PTY: ptmx, Done: make(chan struct{})}
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				_ = conn.WriteJSON(map[string]any{"type": "terminal-output", "sessionId": id, "data": string(buf[:n])})
			}
			if err != nil {
				if err != io.EOF {
					_ = conn.WriteJSON(map[string]any{"type": "terminal-exit", "sessionId": id, "error": err.Error()})
				}
				break
			}
		}
		m.Close(id)
	}()
	return id, nil
}

func (m *Manager) Write(id, data string) error {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s == nil {
		return os.ErrNotExist
	}
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
}
