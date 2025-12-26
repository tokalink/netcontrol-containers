package handlers

import (
	"bufio"
	"net/http"

	"netcontrol-containers/services"

	"github.com/gin-gonic/gin"
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

func InspectContainer(c *gin.Context) {
	containerID := c.Param("id")

	docker, err := services.GetDockerService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	info, err := docker.InspectContainer(containerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
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
