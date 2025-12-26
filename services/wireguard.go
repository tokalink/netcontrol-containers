package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type WireGuardService struct {
	ConfigPath string
	Interface  string
	mu         sync.Mutex
}

var (
	wgService *WireGuardService
	wgOnce    sync.Once
)

type WireGuardStatus struct {
	IsActive      bool   `json:"is_active"`
	Interface     string `json:"interface"`
	IP            string `json:"ip"`
	PublicKey     string `json:"public_key"`
	Endpoint      string `json:"endpoint"`
	LastHandshake string `json:"last_handshake"`
	RxBytes       string `json:"rx_bytes"`
	TxBytes       string `json:"tx_bytes"`
}

func GetWireGuardService() *WireGuardService {
	wgOnce.Do(func() {
		// Determine best path for config
		configDir := "./data/wireguard"
		if runtime.GOOS == "linux" {
			// On Linux, prefer standard location if writable, otherwise fallback
			// But for wg-quick, /etc/wireguard is standard.
			// Users running this server might not be root, but wg-quick needs root.
			// For now, we assume user solves rights or runs as root.
			if _, err := os.Stat("/etc/wireguard"); err == nil {
				configDir = "/etc/wireguard"
			}
		}

		// Ensure directory exists if it's our local one
		if configDir == "./data/wireguard" {
			os.MkdirAll(configDir, 0755)
		}

		wgService = &WireGuardService{
			ConfigPath: configDir,
			Interface:  "wg0",
		}
	})
	return wgService
}

func (s *WireGuardService) GetConfigPath() string {
	return filepath.Join(s.ConfigPath, s.Interface+".conf")
}

func (s *WireGuardService) SaveConfig(content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Basic validation: Check for [Interface] and [Peer]
	if !strings.Contains(content, "[Interface]") {
		return fmt.Errorf("invalid config: missing [Interface] section")
	}

	path := s.GetConfigPath()
	return os.WriteFile(path, []byte(content), 0600)
}

func (s *WireGuardService) GetConfig() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.GetConfigPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil // Return empty if no config
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (s *WireGuardService) Connect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if runtime.GOOS != "linux" {
		return fmt.Errorf("vpn connection is only supported on Linux")
	}

	// Check and install if missing
	if err := s.checkAndInstall(); err != nil {
		return fmt.Errorf("failed to ensure wireguard is installed: %v", err)
	}

	// wg-quick up wg0
	cmd := exec.Command("wg-quick", "up", s.GetConfigPath())
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If already running, treat as success or ignore
		if strings.Contains(string(output), "already exists") {
			return nil
		}
		return fmt.Errorf("failed to connect: %s (%v)", string(output), err)
	}
	return nil
}

func (s *WireGuardService) checkAndInstall() error {
	// Check if wg-quick exists
	_, err := exec.LookPath("wg-quick")
	if err == nil {
		return nil // Already installed
	}

	fmt.Println("WireGuard tools not found. Attempting automatic installation...")

	// Detect package manager
	if _, err := exec.LookPath("apt-get"); err == nil {
		// Debian/Ubuntu
		// Check for resolvconf too, often needed for DNS
		exec.Command("apt-get", "update").Run()
		cmd := exec.Command("apt-get", "install", "-y", "wireguard", "wireguard-tools", "resolvconf")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install (apt): %s", string(out))
		}
	} else if _, err := exec.LookPath("apk"); err == nil {
		// Alpine
		cmd := exec.Command("apk", "add", "--no-cache", "wireguard-tools")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install (apk): %s", string(out))
		}
	} else if _, err := exec.LookPath("yum"); err == nil {
		// CentOS/RHEL
		cmd := exec.Command("yum", "install", "-y", "wireguard-tools")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install (yum): %s", string(out))
		}
	} else {
		return fmt.Errorf("unsupported package manager. please install wireguard-tools manually")
	}

	return nil
}

func (s *WireGuardService) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if runtime.GOOS != "linux" {
		return fmt.Errorf("vpn connection is only supported on Linux")
	}

	// wg-quick down wg0
	cmd := exec.Command("wg-quick", "down", s.GetConfigPath())
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore error if it's just "not running"
		if strings.Contains(string(output), "is not a WireGuard interface") {
			return nil
		}
		return fmt.Errorf("failed to disconnect: %s (%v)", string(output), err)
	}
	return nil
}

func (s *WireGuardService) GetStatus() (*WireGuardStatus, error) {
	// Check if we have a config
	configExists := false
	if _, err := os.Stat(s.GetConfigPath()); err == nil {
		configExists = true
	}

	status := &WireGuardStatus{
		Interface: s.Interface,
		IsActive:  false,
	}

	if !configExists {
		return status, nil
	}

	if runtime.GOOS != "linux" {
		// Mock status for Windows/Dev
		return status, nil
	}

	// Check if interface exists via direct check or 'wg show'
	// 'wg show wg0' returns "interface: wg0" if active
	cmd := exec.Command("wg", "show", s.Interface)
	outputBytes, err := cmd.Output()
	if err != nil {
		// If 'wg show' fails (e.g. permissions), try check if interface exists via ip link
		// This at least confirms it is UP, even if we can't get stats.
		ipCmd := exec.Command("ip", "link", "show", s.Interface)
		if ipCmd.Run() == nil {
			status.IsActive = true
			status.Endpoint = "Connected (Stats unavailable)"
		}
		return status, nil
	}

	output := string(outputBytes)
	if strings.Contains(output, "interface: "+s.Interface) {
		status.IsActive = true

		// Parse rudimentary info if needed, e.g.
		// interface: wg0
		//   public key: ...
		//   private key: (hidden)
		//   listening port: ...
		// peer: ...
		//   endpoint: ...
		//   latest handshake: ...
		//   transfer: ...

		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "interface:") {
				status.Interface = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
			}
			if strings.HasPrefix(line, "public key:") {
				status.PublicKey = strings.TrimSpace(strings.TrimPrefix(line, "public key:"))
			}
			if strings.HasPrefix(line, "endpoint:") {
				status.Endpoint = strings.TrimSpace(strings.TrimPrefix(line, "endpoint:"))
			}
			if strings.HasPrefix(line, "latest handshake:") {
				status.LastHandshake = strings.TrimSpace(strings.TrimPrefix(line, "latest handshake:"))
			}
			if strings.HasPrefix(line, "transfer:") {
				parts := strings.Fields(strings.TrimPrefix(line, "transfer:"))
				if len(parts) >= 4 {
					status.RxBytes = parts[0] + " " + parts[1]
					status.TxBytes = parts[3] + " " + parts[4]
				}
			}
		}
	}

	return status, nil
}
