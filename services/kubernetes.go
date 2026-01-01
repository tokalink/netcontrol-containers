package services

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type KubernetesService struct {
	clientset *kubernetes.Clientset
}

type PodInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Status    string            `json:"status"`
	Ready     string            `json:"ready"`
	Restarts  int32             `json:"restarts"`
	Age       string            `json:"age"`
	IP        string            `json:"ip"`
	Node      string            `json:"node"`
	Ports     string            `json:"ports"`
	Labels    map[string]string `json:"labels"`
}

type DeploymentInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Ready     string            `json:"ready"`
	UpToDate  int32             `json:"up_to_date"`
	Available int32             `json:"available"`
	Age       string            `json:"age"`
	Labels    map[string]string `json:"labels"`
}

type ServiceInfo struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Type       string            `json:"type"`
	ClusterIP  string            `json:"cluster_ip"`
	ExternalIP string            `json:"external_ip"`
	Ports      string            `json:"ports"`
	Age        string            `json:"age"`
	Labels     map[string]string `json:"labels"`
}

type NamespaceInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Age    string `json:"age"`
}

type ClusterStats struct {
	Nodes          int    `json:"nodes"`
	NodesReady     int    `json:"nodes_ready"`
	Pods           int    `json:"pods"`
	PodsRunning    int    `json:"pods_running"`
	Deployments    int    `json:"deployments"`
	Services       int    `json:"services"`
	CPUCapacity    string `json:"cpu_capacity"`
	MemoryCapacity string `json:"memory_capacity"`
	Version        string `json:"version"`
}

var k8sService *KubernetesService

func GetKubernetesService() (*KubernetesService, error) {
	if k8sService != nil {
		return k8sService, nil
	}

	config, err := getKubeConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	k8sService = &KubernetesService{clientset: clientset}
	return k8sService, nil
}

func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig file
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (k *KubernetesService) IsAvailable() bool {
	_, err := k.clientset.Discovery().ServerVersion()
	return err == nil
}

func (k *KubernetesService) ListNamespaces() ([]NamespaceInfo, error) {
	ctx := context.Background()
	namespaces, err := k.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []NamespaceInfo
	for _, ns := range namespaces.Items {
		result = append(result, NamespaceInfo{
			Name:   ns.Name,
			Status: string(ns.Status.Phase),
			Age:    formatDuration(ns.CreationTimestamp.Time),
		})
	}

	return result, nil
}

func (k *KubernetesService) ListPods(namespace string) ([]PodInfo, error) {
	ctx := context.Background()

	if namespace == "" {
		namespace = "default"
	}

	pods, err := k.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []PodInfo
	for _, pod := range pods.Items {
		ready := 0
		total := len(pod.Status.ContainerStatuses)
		var restarts int32

		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				ready++
			}
			restarts += cs.RestartCount
		}

		result = append(result, PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			Ready:     fmt.Sprintf("%d/%d", ready, total),
			Restarts:  restarts,
			Age:       formatDuration(pod.CreationTimestamp.Time),
			IP:        pod.Status.PodIP,
			Node:      pod.Spec.NodeName,
			Ports:     getPodPorts(&pod),
			Labels:    pod.Labels,
		})
	}

	return result, nil
}

func getPodPorts(pod *corev1.Pod) string {
	var ports []string
	for _, container := range pod.Spec.Containers {
		for _, p := range container.Ports {
			ports = append(ports, fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol))
		}
	}
	if len(ports) == 0 {
		return "-"
	}
	return strings.Join(ports, ", ")
}

func (k *KubernetesService) ListDeployments(namespace string) ([]DeploymentInfo, error) {
	ctx := context.Background()

	if namespace == "" {
		namespace = "default"
	}

	deployments, err := k.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []DeploymentInfo
	for _, dep := range deployments.Items {
		result = append(result, DeploymentInfo{
			Name:      dep.Name,
			Namespace: dep.Namespace,
			Ready:     fmt.Sprintf("%d/%d", dep.Status.ReadyReplicas, *dep.Spec.Replicas),
			UpToDate:  dep.Status.UpdatedReplicas,
			Available: dep.Status.AvailableReplicas,
			Age:       formatDuration(dep.CreationTimestamp.Time),
			Labels:    dep.Labels,
		})
	}

	return result, nil
}

func (k *KubernetesService) ListServices(namespace string) ([]ServiceInfo, error) {
	ctx := context.Background()

	if namespace == "" {
		namespace = "default"
	}

	services, err := k.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []ServiceInfo
	for _, svc := range services.Items {
		var ports []string
		for _, p := range svc.Spec.Ports {
			if p.NodePort > 0 {
				ports = append(ports, fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol))
			} else {
				ports = append(ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
			}
		}

		externalIP := "<none>"
		if len(svc.Spec.ExternalIPs) > 0 {
			externalIP = strings.Join(svc.Spec.ExternalIPs, ",")
		} else if svc.Spec.Type == corev1.ServiceTypeLoadBalancer && len(svc.Status.LoadBalancer.Ingress) > 0 {
			externalIP = svc.Status.LoadBalancer.Ingress[0].IP
		}

		result = append(result, ServiceInfo{
			Name:       svc.Name,
			Namespace:  svc.Namespace,
			Type:       string(svc.Spec.Type),
			ClusterIP:  svc.Spec.ClusterIP,
			ExternalIP: externalIP,
			Ports:      strings.Join(ports, ","),
			Age:        formatDuration(svc.CreationTimestamp.Time),
			Labels:     svc.Labels,
		})
	}

	return result, nil
}

func (k *KubernetesService) GetPodLogs(namespace, podName, container string, tailLines int64) (string, error) {
	ctx := context.Background()

	options := &corev1.PodLogOptions{
		TailLines: &tailLines,
	}

	if container != "" {
		options.Container = container
	}

	req := k.clientset.CoreV1().Pods(namespace).GetLogs(podName, options)
	logs, err := req.Stream(ctx)
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

func (k *KubernetesService) ScaleDeployment(namespace, deploymentName string, replicas int32) error {
	ctx := context.Background()

	deployment, err := k.clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment.Spec.Replicas = &replicas

	_, err = k.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	return err
}

func (k *KubernetesService) RestartDeployment(namespace, deploymentName string) error {
	ctx := context.Background()

	deployment, err := k.clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = metav1.Now().Format("2006-01-02T15:04:05Z")

	_, err = k.clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	return err
}

func (k *KubernetesService) DeletePod(namespace, podName string) error {
	ctx := context.Background()
	return k.clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

func (k *KubernetesService) GetClusterStats(namespace string) (*ClusterStats, error) {
	ctx := context.Background()

	// 1. Get Nodes (Cluster-wide)
	nodes, err := k.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nodesReady := 0
	var cpuCap, memCap int64

	for _, node := range nodes.Items {
		// Check readiness
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				nodesReady++
			}
		}
		// Sum capacity
		cpu := node.Status.Capacity[corev1.ResourceCPU]
		mem := node.Status.Capacity[corev1.ResourceMemory]
		cpuCap += cpu.Value()
		memCap += mem.Value()
	}

	// 2. Get Resources (Namespaced or All)
	if namespace == "" || namespace == "all" {
		namespace = "" // metav1.NamespaceAll
	}

	// Pods
	pods, err := k.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	podsRunning := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			podsRunning++
		}
	}

	// Deployments
	deps, err := k.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Services
	svcs, err := k.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Version
	versionInfo, err := k.clientset.Discovery().ServerVersion()
	version := ""
	if err == nil {
		version = versionInfo.GitVersion
	}

	return &ClusterStats{
		Nodes:          len(nodes.Items),
		NodesReady:     nodesReady,
		Pods:           len(pods.Items),
		PodsRunning:    podsRunning,
		Deployments:    len(deps.Items),
		Services:       len(svcs.Items),
		CPUCapacity:    fmt.Sprintf("%d Cores", cpuCap),
		MemoryCapacity: formatBytes(memCap),
		Version:        version,
	}, nil
}

func formatDuration(t time.Time) string {
	duration := time.Since(t)

	if duration.Hours() > 24*365 {
		return fmt.Sprintf("%dy", int(duration.Hours()/(24*365)))
	} else if duration.Hours() > 24 {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	} else if duration.Hours() >= 1 {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	} else if duration.Minutes() >= 1 {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	return fmt.Sprintf("%ds", int(duration.Seconds()))
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
