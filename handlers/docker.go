package handlers

import (
	"bufio"
	"net/http"
	"sync"
	"time"

	"netcontrol-containers/services"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func DockerStatus(c *gin.Context) {
	docker, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"available": false,
			"error":     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"available": docker.IsAvailable(),
	})
}

func ListContainers(c *gin.Context) {
	all := c.Query("all") == "true"

	docker, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	containers, err := docker.ListContainers(all)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, containers)
}

func GetContainerStats(c *gin.Context) {
	containerID := c.Param("id")

	docker, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	stats, err := docker.GetContainerStats(containerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func InspectContainer(c *gin.Context) {
	id := c.Param("id")
	data, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	info, err := data.InspectContainer(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, info)
}

func CreateContainer(c *gin.Context) {
	var req services.CreateContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	d, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, err := d.CreateContainer(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Container created successfully", "id": id})
}

func StartContainer(c *gin.Context) {
	containerID := c.Param("id")

	docker, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := docker.StartContainer(containerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Container started successfully"})
}

func StopContainer(c *gin.Context) {
	containerID := c.Param("id")

	docker, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := docker.StopContainer(containerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Container stopped successfully"})
}

func RestartContainer(c *gin.Context) {
	containerID := c.Param("id")

	docker, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := docker.RestartContainer(containerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Container restarted successfully"})
}

func RemoveContainer(c *gin.Context) {
	containerID := c.Param("id")
	force := c.Query("force") == "true"

	docker, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := docker.RemoveContainer(containerID, force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Container removed successfully"})
}

func GetContainerLogs(c *gin.Context) {
	containerID := c.Param("id")
	tail := c.DefaultQuery("tail", "100")

	docker, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logs, err := docker.GetContainerLogs(containerID, tail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

func ListImages(c *gin.Context) {
	docker, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	images, err := docker.ListImages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, images)
}

func PullImage(c *gin.Context) {
	var req struct {
		Image string `json:"image" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Image name is required"})
		return
	}

	docker, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	reader, err := docker.PullImage(req.Image)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer reader.Close()

	// Stream the output
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		c.Writer.Write([]byte("data: " + scanner.Text() + "\n\n"))
		c.Writer.Flush()
	}

	c.Writer.Write([]byte("data: {\"status\":\"complete\"}\n\n"))
	c.Writer.Flush()
}

func GetSystemUsage(c *gin.Context) {
	docker, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	usage, err := docker.GetSystemUsage()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, usage)
}

func RemoveImage(c *gin.Context) {
	imageID := c.Param("id")
	force := c.Query("force") == "true"

	docker, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := docker.RemoveImage(imageID, force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Image removed successfully"})
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func StreamDockerStats(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	d, err := services.GetDockerService()
	if err != nil {
		return
	}

	previousStats := make(map[string]*services.ContainerStats)

	for {
		// Get all running containers
		containers, err := d.ListContainers(false)
		if err != nil {
			break
		}

		// Collect stats
		statsMap := make(map[string]*services.ContainerStats)
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, cont := range containers {
			if cont.State != "running" {
				continue
			}
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				s, err := d.GetContainerStats(id)
				if err == nil {
					mu.Lock()
					statsMap[id] = s
					mu.Unlock()
				}
			}(cont.ID)
		}
		wg.Wait()

		// Calculate CPU percentage based on previous stats
		for id, s := range statsMap {
			if prev, ok := previousStats[id]; ok {
				cpuDelta := float64(s.CPUTotalUsage - prev.CPUTotalUsage)
				systemDelta := float64(s.SystemUsage - prev.SystemUsage)

				if systemDelta > 0 && cpuDelta > 0 {
					s.CPUPercent = (cpuDelta / systemDelta) * float64(s.OnlineCPUs) * 100.0
				}
			}
			// Update previous stats
			previousStats[id] = s
		}

		if err := ws.WriteJSON(statsMap); err != nil {
			break
		}

		time.Sleep(2 * time.Second)
	}
}
