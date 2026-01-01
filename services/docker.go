package services

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type DockerService struct {
	client *client.Client
}

type ContainerInfo struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Image    string            `json:"image"`
	State    string            `json:"state"`
	Status   string            `json:"status"`
	Created  int64             `json:"created"`
	Ports    []PortMapping     `json:"ports"`
	Networks []string          `json:"networks"`
	IP       string            `json:"ip"`
	Labels   map[string]string `json:"labels"`
	Stats    *ContainerStats   `json:"stats,omitempty"`
}

type PortMapping struct {
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port"`
	Type        string `json:"type"`
	IP          string `json:"ip"`
}

type ContainerStats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   uint64  `json:"memory_usage"`
	MemoryLimit   uint64  `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
	NetworkRx     uint64  `json:"network_rx"`
	NetworkTx     uint64  `json:"network_tx"`
	// Raw stats for stateful calculation
	CPUTotalUsage uint64 `json:"cpu_total_usage"`
	SystemUsage   uint64 `json:"system_usage"`
	OnlineCPUs    int    `json:"online_cpus"`
}

type ImageInfo struct {
	ID       string   `json:"id"`
	RepoTags []string `json:"repo_tags"`
	Size     int64    `json:"size"`
	Created  int64    `json:"created"`
}

type CreateContainerRequest struct {
	Name     string   `json:"name"`
	Image    string   `json:"image"`
	Ports    []string `json:"ports"` // "8080:80/tcp" or just "8080:80"
	Env      []string `json:"env"`
	MemoryMB int64    `json:"memory_mb"`
	CPUCores float64  `json:"cpu_cores"`
}

type SystemUsage struct {
	Containers     int   `json:"containers"`
	ContainersSize int64 `json:"containers_size"`
	Images         int   `json:"images"`
	ImagesSize     int64 `json:"images_size"`
	Volumes        int   `json:"volumes"`
	VolumesSize    int64 `json:"volumes_size"`
	Networks       int   `json:"networks"`
}

var dockerService *DockerService

func GetDockerService() (*DockerService, error) {
	if dockerService != nil {
		return dockerService, nil
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	dockerService = &DockerService{client: cli}
	return dockerService, nil
}

func (d *DockerService) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := d.client.Ping(ctx)
	return err == nil
}

func (d *DockerService) ListContainers(all bool) ([]ContainerInfo, error) {
	ctx := context.Background()
	containers, err := d.client.ContainerList(ctx, types.ContainerListOptions{All: all})
	if err != nil {
		return nil, err
	}

	var result []ContainerInfo
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
		}

		var ports []PortMapping
		for _, p := range c.Ports {
			ports = append(ports, PortMapping{
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
				IP:          p.IP,
			})
		}

		result = append(result, ContainerInfo{
			ID:       c.ID[:12],
			Name:     name,
			Image:    c.Image,
			State:    c.State,
			Status:   c.Status,
			Created:  c.Created,
			Ports:    ports,
			Networks: getNetworkNames(c.NetworkSettings),
			IP:       getIPAddress(c.NetworkSettings),
			Labels:   c.Labels,
		})
	}

	return result, nil
}

func getIPAddress(settings *types.SummaryNetworkSettings) string {
	if settings == nil || settings.Networks == nil {
		return ""
	}
	for _, net := range settings.Networks {
		if net.IPAddress != "" {
			return net.IPAddress
		}
	}
	return ""
}

func getNetworkNames(settings *types.SummaryNetworkSettings) []string {
	if settings == nil || settings.Networks == nil {
		return []string{}
	}
	var networks []string
	for name := range settings.Networks {
		networks = append(networks, name)
	}
	return networks
}

func (d *DockerService) GetContainerStats(containerID string) (*ContainerStats, error) {
	ctx := context.Background()
	stats, err := d.client.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, err
	}
	defer stats.Body.Close()

	// Decode into local struct instead of types.StatsJSON
	var statsJSON struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage  uint64   `json:"total_usage"`
				PercpuUsage []uint64 `json:"percpu_usage"`
			} `json:"cpu_usage"`
			SystemUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs  uint32 `json:"online_cpus"`
		} `json:"cpu_stats"`
		PreCPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemUsage uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		MemoryStats struct {
			Usage uint64 `json:"usage"`
			Limit uint64 `json:"limit"`
		} `json:"memory_stats"`
		Networks map[string]struct {
			RxBytes uint64 `json:"rx_bytes"`
			TxBytes uint64 `json:"tx_bytes"`
		} `json:"networks"`
	}

	if err := json.NewDecoder(stats.Body).Decode(&statsJSON); err != nil {
		return nil, err
	}

	cpuPercent := 0.0
	cpuDelta := float64(statsJSON.CPUStats.CPUUsage.TotalUsage - statsJSON.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(statsJSON.CPUStats.SystemUsage - statsJSON.PreCPUStats.SystemUsage)
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(statsJSON.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	memoryPercent := 0.0
	if statsJSON.MemoryStats.Limit > 0 {
		memoryPercent = float64(statsJSON.MemoryStats.Usage) / float64(statsJSON.MemoryStats.Limit) * 100.0
	}

	var networkRx, networkTx uint64
	for _, net := range statsJSON.Networks {
		networkRx += net.RxBytes
		networkTx += net.TxBytes
	}

	onlineCPUs := int(statsJSON.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = len(statsJSON.CPUStats.CPUUsage.PercpuUsage)
	}

	return &ContainerStats{
		CPUPercent:    cpuPercent,
		MemoryUsage:   statsJSON.MemoryStats.Usage,
		MemoryLimit:   statsJSON.MemoryStats.Limit,
		MemoryPercent: memoryPercent,
		NetworkRx:     networkRx,
		NetworkTx:     networkTx,
		CPUTotalUsage: statsJSON.CPUStats.CPUUsage.TotalUsage,
		SystemUsage:   statsJSON.CPUStats.SystemUsage,
		OnlineCPUs:    onlineCPUs,
	}, nil
}

func (d *DockerService) StartContainer(containerID string) error {
	ctx := context.Background()
	return d.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
}

func (d *DockerService) StopContainer(containerID string) error {
	ctx := context.Background()
	timeout := 10
	return d.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

func (d *DockerService) RestartContainer(containerID string) error {
	ctx := context.Background()
	timeout := 10
	return d.client.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

func (d *DockerService) RemoveContainer(containerID string, force bool) error {
	ctx := context.Background()
	return d.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: force})
}

func (d *DockerService) GetContainerLogs(containerID string, tail string) (string, error) {
	ctx := context.Background()
	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
		Timestamps: true,
	}

	logs, err := d.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", err
	}
	defer logs.Close()

	content, err := io.ReadAll(logs)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (d *DockerService) ListImages() ([]ImageInfo, error) {
	ctx := context.Background()
	images, err := d.client.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return nil, err
	}

	var result []ImageInfo
	for _, img := range images {
		result = append(result, ImageInfo{
			ID:       img.ID[7:19],
			RepoTags: img.RepoTags,
			Size:     img.Size,
			Created:  img.Created,
		})
	}

	return result, nil
}

func (d *DockerService) PullImage(imageName string) (io.ReadCloser, error) {
	ctx := context.Background()
	return d.client.ImagePull(ctx, imageName, types.ImagePullOptions{})
}

func (d *DockerService) RemoveImage(imageID string, force bool) error {
	ctx := context.Background()
	_, err := d.client.ImageRemove(ctx, imageID, types.ImageRemoveOptions{Force: force})
	return err
}

func (d *DockerService) InspectContainer(containerID string) (interface{}, error) {
	ctx := context.Background()
	return d.client.ContainerInspect(ctx, containerID)
}

func (d *DockerService) CreateContainer(req CreateContainerRequest) (string, error) {
	ctx := context.Background()

	// Parse Ports
	exposedPorts := make(map[string]struct{})                            // nat.PortSet
	portBindings := make(map[string][]struct{ HostIP, HostPort string }) // nat.PortMap

	for _, p := range req.Ports {
		// format: host:container/proto
		parts := parsePortSpec(p)
		if parts != nil {
			portKey := parts.ContainerPort + "/" + parts.Protocol
			exposedPorts[portKey] = struct{}{}
			portBindings[portKey] = []struct{ HostIP, HostPort string }{
				{HostIP: "", HostPort: parts.HostPort},
			}
		}
	}

	config := &container.Config{
		Image:        req.Image,
		Env:          req.Env,
		ExposedPorts: make(nat.PortSet),
	}
	for k := range exposedPorts {
		config.ExposedPorts[nat.Port(k)] = struct{}{}
	}

	hostConfig := &container.HostConfig{
		PortBindings: make(nat.PortMap),
		Resources: container.Resources{
			Memory:   req.MemoryMB * 1024 * 1024,
			NanoCPUs: int64(req.CPUCores * 1e9),
		},
	}

	for k, v := range portBindings {
		var bindings []nat.PortBinding
		for _, b := range v {
			bindings = append(bindings, nat.PortBinding{HostIP: b.HostIP, HostPort: b.HostPort})
		}
		hostConfig.PortBindings[nat.Port(k)] = bindings
	}

	resp, err := d.client.ContainerCreate(ctx, config, hostConfig, nil, nil, req.Name)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

type PortParts struct {
	HostPort      string
	ContainerPort string
	Protocol      string
}

func parsePortSpec(spec string) *PortParts {
	// Simple parser for "8080:80" or "8080:80/tcp"
	// Only handles simple host:container format
	if spec == "" {
		return nil
	}

	protocol := "tcp"
	// Check protocol
	parts := strings.Split(spec, "/")
	if len(parts) > 1 {
		protocol = parts[1]
		spec = parts[0]
	}

	// Check ports
	ports := strings.Split(spec, ":")
	if len(ports) != 2 {
		return nil
	}

	return &PortParts{
		HostPort:      ports[0],
		ContainerPort: ports[1],
		Protocol:      protocol,
	}
}
func (d *DockerService) GetSystemUsage() (*SystemUsage, error) {
	ctx := context.Background()
	usage, err := d.client.DiskUsage(ctx, types.DiskUsageOptions{})
	if err != nil {
		return nil, err
	}

	var imagesSize int64
	for _, img := range usage.Images {
		imagesSize += img.Size
	}

	var volumesSize int64
	for _, vol := range usage.Volumes {
		if vol.UsageData != nil {
			volumesSize += vol.UsageData.Size
		}
	}

	// Also get networks count
	networks, _ := d.client.NetworkList(ctx, types.NetworkListOptions{})

	return &SystemUsage{
		Containers:     len(usage.Containers),
		ContainersSize: usage.LayersSize,
		Images:         len(usage.Images),
		ImagesSize:     imagesSize,
		Volumes:        len(usage.Volumes),
		VolumesSize:    volumesSize,
		Networks:       len(networks),
	}, nil
}
