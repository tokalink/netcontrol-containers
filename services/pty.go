package services

import (
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"github.com/creack/pty"
)

type PTYSession struct {
	ID   string
	Cmd  *exec.Cmd
	PTY  *os.File
	Rows uint16
	Cols uint16
	Done chan struct{}
	mu   sync.Mutex
}

type PTYManager struct {
	sessions map[string]*PTYSession
	mu       sync.RWMutex
}

var ptyManager *PTYManager

func GetPTYManager() *PTYManager {
	if ptyManager == nil {
		ptyManager = &PTYManager{
			sessions: make(map[string]*PTYSession),
		}
	}
	return ptyManager
}

func (m *PTYManager) CreateSession(sessionID string, rows, cols uint16) (*PTYSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if session already exists
	if session, exists := m.sessions[sessionID]; exists {
		return session, nil
	}

	// Get shell command based on OS
	shell := getShell()
	cmd := exec.Command(shell)

	// Set environment
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	// Start PTY
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
	if err != nil {
		return nil, err
	}

	session := &PTYSession{
		ID:   sessionID,
		Cmd:  cmd,
		PTY:  ptmx,
		Rows: rows,
		Cols: cols,
		Done: make(chan struct{}),
	}

	m.sessions[sessionID] = session

	// Wait for command to finish
	go func() {
		cmd.Wait()
		close(session.Done)
		m.CloseSession(sessionID)
	}()

	return session, nil
}

func (m *PTYManager) GetSession(sessionID string) *PTYSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

func (m *PTYManager) CloseSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil
	}

	if session.PTY != nil {
		session.PTY.Close()
	}

	if session.Cmd != nil && session.Cmd.Process != nil {
		session.Cmd.Process.Kill()
	}

	delete(m.sessions, sessionID)
	return nil
}

func (m *PTYManager) ListSessions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []string
	for id := range m.sessions {
		sessions = append(sessions, id)
	}
	return sessions
}

func (s *PTYSession) Read(p []byte) (n int, err error) {
	return s.PTY.Read(p)
}

func (s *PTYSession) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.PTY.Write(p)
}

func (s *PTYSession) Resize(rows, cols uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Rows = rows
	s.Cols = cols

	return pty.Setsize(s.PTY, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

func (s *PTYSession) Close() error {
	return GetPTYManager().CloseSession(s.ID)
}

func getShell() string {
	if runtime.GOOS == "windows" {
		// Try PowerShell first
		if _, err := exec.LookPath("powershell.exe"); err == nil {
			return "powershell.exe"
		}
		return "cmd.exe"
	}

	// Check $SHELL environment variable
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}

	// Try common shells
	shells := []string{"/bin/bash", "/bin/zsh", "/bin/sh"}
	for _, shell := range shells {
		if _, err := os.Stat(shell); err == nil {
			return shell
		}
	}

	return "/bin/sh"
}

// StreamOutput streams PTY output to a writer
func (s *PTYSession) StreamOutput(w io.Writer) error {
	buf := make([]byte, 1024)
	for {
		select {
		case <-s.Done:
			return nil
		default:
			n, err := s.PTY.Read(buf)
			if err != nil {
				return err
			}
			if n > 0 {
				if _, err := w.Write(buf[:n]); err != nil {
					return err
				}
			}
		}
	}
}
