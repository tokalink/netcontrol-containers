package services

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

type InstallerService struct {
	mu           sync.Mutex
	isInstalling bool
	currentTask  string
	progress     int
	logs         []string
}

type InstallStatus struct {
	IsInstalling bool     `json:"is_installing"`
	CurrentTask  string   `json:"current_task"`
	Progress     int      `json:"progress"`
	Logs         []string `json:"logs"`
}

type SoftwareStatus struct {
	Docker     *SoftwareInfo `json:"docker"`
	Kubernetes *SoftwareInfo `json:"kubernetes"`
}

type SoftwareInfo struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Running   bool   `json:"running"`
}

var installerService *InstallerService

func GetInstallerService() *InstallerService {
	if installerService == nil {
		installerService = &InstallerService{
			logs: make([]string, 0),
		}
	}
	return installerService
}

func (i *InstallerService) GetStatus() InstallStatus {
	i.mu.Lock()
	defer i.mu.Unlock()

	return InstallStatus{
		IsInstalling: i.isInstalling,
		CurrentTask:  i.currentTask,
		Progress:     i.progress,
		Logs:         i.logs,
	}
}

func (i *InstallerService) addLog(msg string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.logs = append(i.logs, msg)
	if len(i.logs) > 1000 {
		i.logs = i.logs[len(i.logs)-1000:]
	}
}

func (i *InstallerService) setProgress(task string, progress int) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.currentTask = task
	i.progress = progress
}

func (i *InstallerService) CheckSoftwareStatus() *SoftwareStatus {
	status := &SoftwareStatus{
		Docker:     i.checkDocker(),
		Kubernetes: i.checkKubernetes(),
	}
	return status
}

func (i *InstallerService) InstallDocker(progressChan chan<- string) error {
	i.mu.Lock()
	if i.isInstalling {
		i.mu.Unlock()
		return fmt.Errorf("another installation is in progress")
	}
	i.isInstalling = true
	i.logs = make([]string, 0)
	i.mu.Unlock()

	defer func() {
		i.mu.Lock()
		i.isInstalling = false
		i.mu.Unlock()
	}()

	os := runtime.GOOS

	switch os {
	case "linux":
		return i.installDockerLinux(progressChan)
	case "windows":
		return i.installDockerWindows(progressChan)
	default:
		return fmt.Errorf("unsupported operating system: %s", os)
	}
}

func (i *InstallerService) installDockerLinux(progressChan chan<- string) error {
	distro, err := i.detectLinuxDistro()
	if err != nil {
		return fmt.Errorf("failed to detect linux distribution: %v", err)
	}

	i.addLog(fmt.Sprintf("Detected Linux distribution: %s", distro))

	switch distro {
	case "ubuntu", "debian":
		return i.installDockerDebian(progressChan, distro)
	case "centos", "rhel", "fedora", "almalinux", "rocky":
		return i.installDockerRedHat(progressChan)
	case "alpine":
		return i.installDockerAlpine(progressChan)
	default:
		// Fallback to generic script if unknown
		i.addLog(fmt.Sprintf("Untitled distribution '%s', attempting generic installation script...", distro))
		return i.installDockerGeneric(progressChan)
	}
}

func (i *InstallerService) detectLinuxDistro() (string, error) {
	out, err := exec.Command("cat", "/etc/os-release").Output()
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			// Remove ID= prefix
			val := strings.TrimPrefix(line, "ID=")
			// Remove surrounding quotes and any whitespace/newlines
			val = strings.Trim(val, `"'`+"\n\r\t")
			return strings.ToLower(val), nil
		}
	}
	return "unknown", nil
}

func (i *InstallerService) installDockerDebian(progressChan chan<- string, distro string) error {
	// Clean up potential leftover bad config from previous attempts
	exec.Command("rm", "-f", "/etc/apt/sources.list.d/docker.list").Run()
	exec.Command("rm", "-f", "/usr/share/keyrings/docker-archive-keyring.gpg").Run()

	// Determine correct repo URL base
	// Default to ubuntu
	repoBase := "ubuntu"
	if distro == "debian" || distro == "kali" || distro == "raspbian" {
		repoBase = "debian"
	}

	repoCmd := fmt.Sprintf(`echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/%s $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null`, repoBase)
	gpgCmd := fmt.Sprintf("curl -fsSL https://download.docker.com/linux/%s/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg", repoBase)

	steps := []struct {
		name    string
		cmd     string
		args    []string
		percent int
	}{
		{"Updating package index", "apt-get", []string{"update", "-y"}, 10},
		{"Installing prerequisites", "apt-get", []string{"install", "-y", "apt-transport-https", "ca-certificates", "curl", "gnupg", "lsb-release"}, 20},
		{"Adding Docker GPG key", "sh", []string{"-c", gpgCmd}, 30},
		{"Adding Docker repository", "sh", []string{"-c", repoCmd}, 40},
		{"Updating package index", "apt-get", []string{"update", "-y"}, 50},
		{"Installing Docker Engine", "apt-get", []string{"install", "-y", "docker-ce", "docker-ce-cli", "containerd.io", "docker-compose-plugin"}, 80},
		{"Starting Docker service", "systemctl", []string{"start", "docker"}, 90},
		{"Enabling Docker service", "systemctl", []string{"enable", "docker"}, 100},
	}
	return i.executeSteps(steps, progressChan)
}

func (i *InstallerService) installDockerRedHat(progressChan chan<- string) error {
	// Detect yum or dnf
	pkgMgr := "yum"
	if _, err := exec.LookPath("dnf"); err == nil {
		pkgMgr = "dnf"
	}

	steps := []struct {
		name    string
		cmd     string
		args    []string
		percent int
	}{
		{"Installing utils", pkgMgr, []string{"install", "-y", "yum-utils"}, 20},
		{"Adding Docker repository", "yum-config-manager", []string{"--add-repo", "https://download.docker.com/linux/centos/docker-ce.repo"}, 40},
		{"Installing Docker Engine", pkgMgr, []string{"install", "-y", "docker-ce", "docker-ce-cli", "containerd.io", "docker-compose-plugin"}, 80},
		{"Starting Docker service", "systemctl", []string{"start", "docker"}, 90},
		{"Enabling Docker service", "systemctl", []string{"enable", "docker"}, 100},
	}
	return i.executeSteps(steps, progressChan)
}

func (i *InstallerService) installDockerAlpine(progressChan chan<- string) error {
	steps := []struct {
		name    string
		cmd     string
		args    []string
		percent int
	}{
		{"Updating package index", "apk", []string{"update"}, 20},
		{"Installing Docker", "apk", []string{"add", "docker", "docker-compose"}, 60},
		{"Starting Docker service", "rc-service", []string{"docker", "start"}, 80},
		{"Enabling Docker on boot", "rc-update", []string{"add", "docker", "default"}, 100},
	}
	return i.executeSteps(steps, progressChan)
}

func (i *InstallerService) installDockerGeneric(progressChan chan<- string) error {
	// Use the convenience script
	steps := []struct {
		name    string
		cmd     string
		args    []string
		percent int
	}{
		{"Downloading generic install script", "curl", []string{"-fsSL", "https://get.docker.com", "-o", "get-docker.sh"}, 20},
		{"Executing install script", "sh", []string{"get-docker.sh"}, 90},
	}
	return i.executeSteps(steps, progressChan)
}

func (i *InstallerService) executeSteps(steps []struct {
	name    string
	cmd     string
	args    []string
	percent int
}, progressChan chan<- string) error {
	for _, step := range steps {
		i.setProgress(step.name, step.percent)
		if progressChan != nil {
			progressChan <- fmt.Sprintf("[%d%%] %s...", step.percent, step.name)
		}
		i.addLog(fmt.Sprintf("[%d%%] %s...", step.percent, step.name))

		cmd := exec.Command(step.cmd, step.args...)
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			errMsg := fmt.Sprintf("Error: %v", err)
			i.addLog(errMsg)
			if progressChan != nil {
				progressChan <- errMsg
			}
			return err
		}

		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				i.addLog(line)
				if progressChan != nil {
					progressChan <- line
				}
			}
		}()

		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				line := scanner.Text()
				i.addLog(line)
				if progressChan != nil {
					progressChan <- line
				}
			}
		}()

		if err := cmd.Wait(); err != nil {
			errMsg := fmt.Sprintf("Command failed: %v", err)
			i.addLog(errMsg)
			if progressChan != nil {
				progressChan <- errMsg
			}
			return err
		}
	}

	successMsg := "Installation completed successfully!"
	i.addLog(successMsg)
	if progressChan != nil {
		progressChan <- successMsg
	}
	return nil
}

func (i *InstallerService) installDockerWindows(progressChan chan<- string) error {
	msg := "Docker Desktop for Windows must be installed manually. Please download from https://www.docker.com/products/docker-desktop"
	i.addLog(msg)
	if progressChan != nil {
		progressChan <- msg
	}
	return fmt.Errorf(msg)
}

func (i *InstallerService) InstallKubernetes(progressChan chan<- string) error {
	i.mu.Lock()
	if i.isInstalling {
		i.mu.Unlock()
		return fmt.Errorf("another installation is in progress")
	}
	i.isInstalling = true
	i.logs = make([]string, 0)
	i.mu.Unlock()

	defer func() {
		i.mu.Lock()
		i.isInstalling = false
		i.mu.Unlock()
	}()

	os := runtime.GOOS

	switch os {
	case "linux":
		return i.installKubernetesLinux(progressChan)
	case "windows":
		return i.installKubernetesWindows(progressChan)
	default:
		return fmt.Errorf("unsupported operating system: %s", os)
	}
}

func (i *InstallerService) installKubernetesLinux(progressChan chan<- string) error {
	distro, err := i.detectLinuxDistro()
	if err != nil {
		return fmt.Errorf("failed to detect linux distribution: %v", err)
	}

	i.addLog(fmt.Sprintf("Detected Linux distribution: %s", distro))

	switch distro {
	case "ubuntu", "debian", "kali", "raspbian":
		return i.installKubernetesDebian(progressChan)
	case "centos", "rhel", "fedora", "almalinux", "rocky":
		return i.installKubernetesRedHat(progressChan)
	default:
		return fmt.Errorf("automatic kubernetes installation is not yet supported for %s. please install kubeadm/kubectl manually", distro)
	}
}

func (i *InstallerService) installKubernetesDebian(progressChan chan<- string) error {
	// Clean up potential leftover bad config
	exec.Command("rm", "-f", "/etc/apt/sources.list.d/kubernetes.list").Run()
	exec.Command("rm", "-f", "/etc/apt/keyrings/kubernetes-apt-keyring.gpg").Run()

	steps := []struct {
		name    string
		cmd     string
		args    []string
		percent int
	}{
		{"Updating package index", "apt-get", []string{"update", "-y"}, 10},
		{"Installing prerequisites", "apt-get", []string{"install", "-y", "apt-transport-https", "ca-certificates", "curl", "gnupg"}, 20},
		{"Adding Kubernetes GPG key", "sh", []string{"-c", "curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.29/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg"}, 30},
		{"Adding Kubernetes repository", "sh", []string{"-c", `echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.29/deb/ /" | tee /etc/apt/sources.list.d/kubernetes.list`}, 40},
		{"Updating package index", "apt-get", []string{"update", "-y"}, 50},
		{"Installing kubeadm, kubelet, kubectl", "apt-get", []string{"install", "-y", "kubelet", "kubeadm", "kubectl"}, 80},
		{"Holding Kubernetes packages", "apt-mark", []string{"hold", "kubelet", "kubeadm", "kubectl"}, 90},
		{"Enabling kubelet", "systemctl", []string{"enable", "--now", "kubelet"}, 100},
	}
	return i.executeSteps(steps, progressChan)
}

func (i *InstallerService) installKubernetesRedHat(progressChan chan<- string) error {
	// Detect yum or dnf
	pkgMgr := "yum"
	if _, err := exec.LookPath("dnf"); err == nil {
		pkgMgr = "dnf"
	}

	repoContent := `[kubernetes]
name=Kubernetes
baseurl=https://pkgs.k8s.io/core:/stable:/v1.29/rpm/
enabled=1
gpgcheck=1
gpgkey=https://pkgs.k8s.io/core:/stable:/v1.29/rpm/repodata/repomd.xml.key
`
	// Write repo file
	exec.Command("sh", "-c", fmt.Sprintf("echo '%s' > /etc/yum.repos.d/kubernetes.repo", repoContent)).Run()

	steps := []struct {
		name    string
		cmd     string
		args    []string
		percent int
	}{
		{"Setting SELinux to permissive", "setenforce", []string{"0"}, 10},
		{"Persisting SELinux permissive mode", "sed", []string{"-i", "s/^SELINUX=enforcing$/SELINUX=permissive/", "/etc/selinux/config"}, 20},
		{"Installing kubeadm, kubelet, kubectl", pkgMgr, []string{"install", "-y", "kubelet", "kubeadm", "kubectl", "--disableexcludes=kubernetes"}, 70},
		{"Enabling kubelet", "systemctl", []string{"enable", "--now", "kubelet"}, 100},
	}
	return i.executeSteps(steps, progressChan)
}

func (i *InstallerService) installKubernetesWindows(progressChan chan<- string) error {
	msg := "Kubernetes for Windows is available through Docker Desktop or WSL2. Please enable Kubernetes in Docker Desktop settings."
	i.addLog(msg)
	if progressChan != nil {
		progressChan <- msg
	}
	return fmt.Errorf(msg)
}
func (i *InstallerService) UninstallDocker(progressChan chan<- string) error {
	i.mu.Lock()
	if i.isInstalling {
		i.mu.Unlock()
		return fmt.Errorf("another installation is in progress")
	}
	i.isInstalling = true
	i.logs = make([]string, 0)
	i.mu.Unlock()

	defer func() {
		i.mu.Lock()
		i.isInstalling = false
		i.mu.Unlock()
	}()

	if runtime.GOOS != "linux" {
		return fmt.Errorf("uninstall only supported on Linux")
	}

	steps := []struct {
		name    string
		cmd     string
		args    []string
		percent int
	}{
		{"Stopping Docker service", "systemctl", []string{"stop", "docker"}, 20},
		{"Removing Docker packages", "apt-get", []string{"purge", "-y", "docker-ce", "docker-ce-cli", "containerd.io", "docker-compose-plugin"}, 60},
		{"Removing Docker data", "rm", []string{"-rf", "/var/lib/docker"}, 80},
		{"Removing Docker config", "rm", []string{"-rf", "/etc/docker"}, 100},
	}

	for _, step := range steps {
		i.setProgress(step.name, step.percent)
		if progressChan != nil {
			progressChan <- fmt.Sprintf("[%d%%] %s...", step.percent, step.name)
		}
		i.addLog(fmt.Sprintf("[%d%%] %s...", step.percent, step.name))

		cmd := exec.Command(step.cmd, step.args...)
		output, _ := cmd.CombinedOutput()

		if len(output) > 0 {
			i.addLog(string(output))
			if progressChan != nil {
				progressChan <- string(output)
			}
		}
	}

	successMsg := "Docker uninstalled successfully!"
	i.addLog(successMsg)
	if progressChan != nil {
		progressChan <- successMsg
	}
	return nil
}

func (i *InstallerService) UninstallKubernetes(progressChan chan<- string) error {
	i.mu.Lock()
	if i.isInstalling {
		i.mu.Unlock()
		return fmt.Errorf("another installation is in progress")
	}
	i.isInstalling = true
	i.logs = make([]string, 0)
	i.mu.Unlock()

	defer func() {
		i.mu.Lock()
		i.isInstalling = false
		i.mu.Unlock()
	}()

	if runtime.GOOS != "linux" {
		return fmt.Errorf("uninstall only supported on Linux")
	}

	// Kubeadm reset is good practice before uninstalling
	exec.Command("kubeadm", "reset", "-f").Run()

	steps := []struct {
		name    string
		cmd     string
		args    []string
		percent int
	}{
		{"Stopping kubelet", "systemctl", []string{"stop", "kubelet"}, 10},
		{"Removing Kubernetes packages", "apt-get", []string{"purge", "-y", "kubelet", "kubeadm", "kubectl"}, 50},
		{"Removing configs", "rm", []string{"-rf", "/etc/kubernetes", "/var/lib/kubelet", "/root/.kube"}, 80},
		{"Cleaning CNI", "rm", []string{"-rf", "/etc/cni/net.d", "/opt/cni/bin"}, 90},
		{"Refresing apt", "apt-get", []string{"autoremove", "-y"}, 100},
	}

	for _, step := range steps {
		i.setProgress(step.name, step.percent)
		if progressChan != nil {
			progressChan <- fmt.Sprintf("[%d%%] %s...", step.percent, step.name)
		}
		i.addLog(fmt.Sprintf("[%d%%] %s...", step.percent, step.name))

		cmd := exec.Command(step.cmd, step.args...)
		output, _ := cmd.CombinedOutput()

		if len(output) > 0 {
			i.addLog(string(output))
			if progressChan != nil {
				progressChan <- string(output)
			}
		}
	}

	successMsg := "Kubernetes uninstalled successfully!"
	i.addLog(successMsg)
	if progressChan != nil {
		progressChan <- successMsg
	}
	return nil
}

func (i *InstallerService) RestartService(serviceName string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("restart only supported on Linux")
	}
	cmd := exec.Command("systemctl", "restart", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart %s: %s (%v)", serviceName, string(output), err)
	}
	return nil
}

// Improved Check Helpers
func (i *InstallerService) resolveBinary(name string) string {
	path, err := exec.LookPath(name)
	if err == nil {
		return path
	}
	// Fallback to common paths
	commonPaths := []string{
		"/usr/bin/" + name,
		"/usr/local/bin/" + name,
		"/snap/bin/" + name,
	}
	for _, p := range commonPaths {
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
	}
	return name // return original name to let standard failure handle it
}

func (i *InstallerService) checkDocker() *SoftwareInfo {
	info := &SoftwareInfo{Installed: false}

	bin := i.resolveBinary("docker")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "--version")
	output, err := cmd.Output()
	if err == nil {
		info.Installed = true
		info.Version = strings.TrimSpace(string(output))

		// Check if Docker is running
		ctxRun, cancelRun := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelRun()

		checkCmd := exec.CommandContext(ctxRun, bin, "info")
		if checkCmd.Run() == nil {
			info.Running = true
		}
	}
	return info
}

func (i *InstallerService) checkKubernetes() *SoftwareInfo {
	info := &SoftwareInfo{Installed: false}

	bin := i.resolveBinary("kubectl")

	// Check version (quick check)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "version", "--client")
	output, err := cmd.Output()
	if err == nil {
		info.Installed = true
		info.Version = strings.TrimSpace(string(output))

		// Check if kubectl can connect (with timeout)
		// This command attempts to contact the APIServer, which can hang if network is down/misconfigured
		ctxRun, cancelRun := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelRun()

		checkCmd := exec.CommandContext(ctxRun, bin, "cluster-info")
		if checkCmd.Run() == nil {
			info.Running = true
		}
	}
	return info
}

func (i *InstallerService) SetupKubernetes(progressChan chan<- string) error {
	i.mu.Lock()
	if i.isInstalling {
		i.mu.Unlock()
		return fmt.Errorf("another operation is in progress")
	}
	i.isInstalling = true
	i.logs = make([]string, 0)
	i.mu.Unlock()

	defer func() {
		i.mu.Lock()
		i.isInstalling = false
		i.mu.Unlock()
	}()

	if runtime.GOOS != "linux" {
		return fmt.Errorf("kubernetes setup only supported on Linux")
	}

	steps := []struct {
		name    string
		cmd     string
		args    []string
		percent int
	}{
		// Auto-fix: Install crictl (cri-tools)
		{"Installing crictl", "apt-get", []string{"install", "-y", "cri-tools"}, 2},

		// Auto-fix: Configure containerd (Critical for Kubeadm 1.24+)
		// 1. Generate default config
		{"Generating containerd config", "sh", []string{"-c", "mkdir -p /etc/containerd && containerd config default > /etc/containerd/config.toml"}, 5},
		// 2. Enable SystemdCgroup (sed replacement)
		{"Enabling SystemdCgroup for containerd", "sed", []string{"-i", "s/SystemdCgroup = false/SystemdCgroup = true/g", "/etc/containerd/config.toml"}, 7},
		// 3. Restart containerd
		{"Restarting containerd", "systemctl", []string{"restart", "containerd"}, 9},

		// Auto-fix: Disable Swap (Critical for K8s)
		{"Disabling Swap", "swapoff", []string{"-a"}, 12},

		// Auto-fix: Reset existing state to avoid "Port in use" or "File exists" errors
		{"Resetting previous state (ignore errors)", "kubeadm", []string{"reset", "-f"}, 15},

		// Initialize the cluster with a pod network cidr compatible with flannel
		{"Initializing Cluster (this may take a minute)", "kubeadm", []string{"init", "--pod-network-cidr=10.244.0.0/16", "--cri-socket", "unix:///var/run/containerd/containerd.sock"}, 20},

		// Setup kubeconfig for root/user so kubectl works
		{"Configuring kubeconfig", "sh", []string{"-c", "mkdir -p $HOME/.kube && cp -f /etc/kubernetes/admin.conf $HOME/.kube/config && chown $(id -u):$(id -g) $HOME/.kube/config"}, 40},

		// Install CNI Plugin (Flannel)
		{"Installing Flannel CNI", "kubectl", []string{"apply", "-f", "https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml"}, 60},

		// Allow scheduling on the control plane (important for single node setups)
		{"Untainting control-plane node", "kubectl", []string{"taint", "nodes", "--all", "node-role.kubernetes.io/control-plane-"}, 80},
		{"Untainting master node (legacy)", "kubectl", []string{"taint", "nodes", "--all", "node-role.kubernetes.io/master-"}, 90},
	}

	for _, step := range steps {
		i.setProgress(step.name, step.percent)
		if progressChan != nil {
			progressChan <- fmt.Sprintf("[%d%%] %s...", step.percent, step.name)
		}
		i.addLog(fmt.Sprintf("[%d%%] %s...", step.percent, step.name))

		cmd := exec.Command(step.cmd, step.args...)
		// Set environment for root to find kubeadm if needed
		cmd.Env = append(cmd.Env, "KUBECONFIG=/etc/kubernetes/admin.conf")

		// Special handling for legacy taint command that might fail on newer k8s
		if strings.Contains(step.name, "legacy") {
			cmd.Run() // Ignore error
			continue
		}

		output, err := cmd.CombinedOutput()
		if len(output) > 0 {
			i.addLog(string(output))
			if progressChan != nil {
				progressChan <- string(output)
			}
		}

		if err != nil {
			// If it's the taint command, it might fail if already untainted or different version, treat as warning
			if strings.Contains(step.name, "Untainting") {
				i.addLog("Warning: Taint command failed (this is often expected on re-runs): " + err.Error())
				continue
			}
			return fmt.Errorf("step '%s' failed: %v", step.name, err)
		}
	}

	successMsg := "Kubernetes Cluster initialized successfully! You can now use kubectl."
	i.addLog(successMsg)
	if progressChan != nil {
		progressChan <- successMsg
	}
	return nil
}
