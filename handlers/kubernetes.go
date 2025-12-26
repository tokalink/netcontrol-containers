package handlers

import (
	"net/http"
	"strconv"

	"netcontrol-containers/services"

	"github.com/gin-gonic/gin"
)

func KubernetesStatus(c *gin.Context) {
	k8s, err := services.GetKubernetesService()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"available": false,
			"error":     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"available": k8s.IsAvailable(),
	})
}

func ListNamespaces(c *gin.Context) {
	k8s, err := services.GetKubernetesService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	namespaces, err := k8s.ListNamespaces()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, namespaces)
}

func ListPods(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	k8s, err := services.GetKubernetesService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	pods, err := k8s.ListPods(namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, pods)
}

func ListDeployments(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	k8s, err := services.GetKubernetesService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	deployments, err := k8s.ListDeployments(namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, deployments)
}

func ListK8sServices(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	k8s, err := services.GetKubernetesService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	svcs, err := k8s.ListServices(namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, svcs)
}

func GetPodLogs(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	podName := c.Param("name")
	container := c.Query("container")
	tailLines := c.DefaultQuery("tail", "100")

	lines, _ := strconv.ParseInt(tailLines, 10, 64)
	if lines <= 0 {
		lines = 100
	}

	k8s, err := services.GetKubernetesService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logs, err := k8s.GetPodLogs(namespace, podName, container, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

func ScaleDeployment(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	deploymentName := c.Param("name")

	var req struct {
		Replicas int32 `json:"replicas" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Replicas count is required"})
		return
	}

	k8s, err := services.GetKubernetesService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := k8s.ScaleDeployment(namespace, deploymentName, req.Replicas); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deployment scaled successfully"})
}

func RestartDeployment(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	deploymentName := c.Param("name")

	k8s, err := services.GetKubernetesService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := k8s.RestartDeployment(namespace, deploymentName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Deployment restarted successfully"})
}

func DeletePod(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")
	podName := c.Param("name")

	k8s, err := services.GetKubernetesService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := k8s.DeletePod(namespace, podName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod deleted successfully"})
}
