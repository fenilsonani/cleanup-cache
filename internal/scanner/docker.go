package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DockerItemType represents different types of Docker resources
type DockerItemType int

const (
	TypeDockerImage DockerItemType = iota
	TypeDockerContainer
	TypeDockerVolume
	TypeDockerBuildCache
)

// DockerImage represents a Docker image
type DockerImage struct {
	ID        string    `json:"Id"`
	RepoTags  []string  `json:"RepoTags"`
	Size      int64     `json:"Size"`
	Created   int64     `json:"Created"`
	CreatedAt time.Time `json:"-"`
	Dangling  bool      `json:"-"`
}

// DockerContainer represents a Docker container
type DockerContainer struct {
	ID        string `json:"Id"`
	Names     []string `json:"Names"`
	Image     string `json:"Image"`
	State     string `json:"State"`
	Status    string `json:"Status"`
	Created   int64  `json:"Created"`
	CreatedAt time.Time `json:"-"`
	SizeRw    int64  `json:"SizeRw"`
	SizeRootFs int64 `json:"SizeRootFs"`
}

// DockerVolume represents a Docker volume
type DockerVolume struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Mountpoint string            `json:"Mountpoint"`
	CreatedAt  string            `json:"CreatedAt"`
	Labels     map[string]string `json:"Labels"`
	Scope      string            `json:"Scope"`
	UsageData  *VolumeUsageData  `json:"UsageData"`
}

// VolumeUsageData represents volume usage information
type VolumeUsageData struct {
	Size     int64 `json:"Size"`
	RefCount int   `json:"RefCount"`
}

// DockerBuildCacheInfo represents build cache information
type DockerBuildCacheInfo struct {
	ID          string `json:"ID"`
	Type        string `json:"Type"`
	Size        int64  `json:"Size"`
	CreatedAt   string `json:"CreatedAt"`
	LastUsedAt  string `json:"LastUsedAt"`
	UsageCount  int    `json:"UsageCount"`
	InUse       bool   `json:"InUse"`
	Shared      bool   `json:"Shared"`
	Description string `json:"Description"`
}

// DockerClient provides an interface to Docker daemon
type DockerClient struct {
	socketPath string
	httpClient *http.Client
	connected  bool
	mu         sync.RWMutex
}

// NewDockerClient creates a new Docker client
func NewDockerClient() *DockerClient {
	socketPaths := getDockerSocketPaths()

	for _, path := range socketPaths {
		if _, err := os.Stat(path); err == nil {
			client := &DockerClient{
				socketPath: path,
				httpClient: &http.Client{
					Transport: &http.Transport{
						DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
							return net.Dial("unix", path)
						},
					},
					Timeout: 30 * time.Second,
				},
			}
			return client
		}
	}

	return nil
}

// getDockerSocketPaths returns possible Docker socket paths
func getDockerSocketPaths() []string {
	homeDir, _ := os.UserHomeDir()
	return []string{
		"/var/run/docker.sock",
		"/run/docker.sock",
		filepath.Join(homeDir, ".docker", "run", "docker.sock"),
		filepath.Join(homeDir, ".colima", "default", "docker.sock"),
		"/var/run/docker.sock",
	}
}

// IsAvailable checks if Docker is available
func (dc *DockerClient) IsAvailable() bool {
	if dc == nil {
		return false
	}

	resp, err := dc.httpClient.Get("http://localhost/_ping")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// ListImages returns list of Docker images
func (dc *DockerClient) ListImages() ([]DockerImage, error) {
	if dc == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}

	resp, err := dc.httpClient.Get("http://localhost/images/json?all=true")
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	defer resp.Body.Close()

	var images []DockerImage
	if err := json.NewDecoder(resp.Body).Decode(&images); err != nil {
		return nil, fmt.Errorf("failed to decode images: %w", err)
	}

	// Post-process images
	for i := range images {
		images[i].CreatedAt = time.Unix(images[i].Created, 0)
		// Check if dangling (no tags)
		if len(images[i].RepoTags) == 0 ||
			(len(images[i].RepoTags) == 1 && images[i].RepoTags[0] == "<none>:<none>") {
			images[i].Dangling = true
		}
	}

	return images, nil
}

// ListContainers returns list of Docker containers
func (dc *DockerClient) ListContainers(all bool) ([]DockerContainer, error) {
	if dc == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}

	url := "http://localhost/containers/json"
	if all {
		url += "?all=true&size=true"
	}

	resp, err := dc.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	defer resp.Body.Close()

	var containers []DockerContainer
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, fmt.Errorf("failed to decode containers: %w", err)
	}

	// Post-process containers
	for i := range containers {
		containers[i].CreatedAt = time.Unix(containers[i].Created, 0)
	}

	return containers, nil
}

// ListVolumes returns list of Docker volumes
func (dc *DockerClient) ListVolumes() ([]DockerVolume, error) {
	if dc == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}

	resp, err := dc.httpClient.Get("http://localhost/volumes")
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}
	defer resp.Body.Close()

	var volumeResponse struct {
		Volumes []DockerVolume `json:"Volumes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&volumeResponse); err != nil {
		return nil, fmt.Errorf("failed to decode volumes: %w", err)
	}

	return volumeResponse.Volumes, nil
}

// GetBuildCache returns build cache information
func (dc *DockerClient) GetBuildCache() ([]DockerBuildCacheInfo, error) {
	if dc == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}

	resp, err := dc.httpClient.Get("http://localhost/system/df")
	if err != nil {
		return nil, fmt.Errorf("failed to get system df: %w", err)
	}
	defer resp.Body.Close()

	var dfResponse struct {
		BuildCache []DockerBuildCacheInfo `json:"BuildCache"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&dfResponse); err != nil {
		return nil, fmt.Errorf("failed to decode build cache: %w", err)
	}

	return dfResponse.BuildCache, nil
}

// RemoveImage removes a Docker image
func (dc *DockerClient) RemoveImage(imageID string) error {
	if dc == nil {
		return fmt.Errorf("docker client not initialized")
	}

	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://localhost/images/%s", imageID), nil)
	if err != nil {
		return err
	}

	resp, err := dc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to remove image: status %d", resp.StatusCode)
	}

	return nil
}

// RemoveContainer removes a Docker container
func (dc *DockerClient) RemoveContainer(containerID string) error {
	if dc == nil {
		return fmt.Errorf("docker client not initialized")
	}

	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://localhost/containers/%s?v=true", containerID), nil)
	if err != nil {
		return err
	}

	resp, err := dc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to remove container: status %d", resp.StatusCode)
	}

	return nil
}

// RemoveVolume removes a Docker volume
func (dc *DockerClient) RemoveVolume(volumeName string) error {
	if dc == nil {
		return fmt.Errorf("docker client not initialized")
	}

	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://localhost/volumes/%s", volumeName), nil)
	if err != nil {
		return err
	}

	resp, err := dc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to remove volume: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to remove volume: status %d", resp.StatusCode)
	}

	return nil
}

// PruneBuildCache prunes the build cache
func (dc *DockerClient) PruneBuildCache() (int64, error) {
	if dc == nil {
		return 0, fmt.Errorf("docker client not initialized")
	}

	req, err := http.NewRequest("POST", "http://localhost/build/prune", nil)
	if err != nil {
		return 0, err
	}

	resp, err := dc.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to prune build cache: %w", err)
	}
	defer resp.Body.Close()

	var pruneResponse struct {
		SpaceReclaimed int64 `json:"SpaceReclaimed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pruneResponse); err != nil {
		return 0, fmt.Errorf("failed to decode prune response: %w", err)
	}

	return pruneResponse.SpaceReclaimed, nil
}

// DockerScanner handles Docker resource scanning
type DockerScanner struct {
	client     *DockerClient
	config     *DockerScanConfig
}

// DockerScanConfig holds Docker scanning configuration
type DockerScanConfig struct {
	ScanImages         bool
	ScanContainers     bool
	ScanVolumes        bool
	ScanBuildCache     bool
	OnlyDanglingImages bool
	OnlyStoppedContainers bool
	OnlyUnusedVolumes  bool
	ImageAgeThreshold  time.Duration
	ContainerAgeThreshold time.Duration
}

// DefaultDockerScanConfig returns default Docker scan configuration
func DefaultDockerScanConfig() *DockerScanConfig {
	return &DockerScanConfig{
		ScanImages:            true,
		ScanContainers:        true,
		ScanVolumes:           true,
		ScanBuildCache:        true,
		OnlyDanglingImages:    true,
		OnlyStoppedContainers: true,
		OnlyUnusedVolumes:     true,
		ImageAgeThreshold:     7 * 24 * time.Hour, // 7 days
		ContainerAgeThreshold: 24 * time.Hour,     // 1 day
	}
}

// NewDockerScanner creates a new Docker scanner
func NewDockerScanner(scanConfig *DockerScanConfig) *DockerScanner {
	if scanConfig == nil {
		scanConfig = DefaultDockerScanConfig()
	}

	return &DockerScanner{
		client: NewDockerClient(),
		config: scanConfig,
	}
}

// ScanDocker scans for cleanable Docker resources
func (s *Scanner) ScanDocker() *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: "docker",
		Errors:   []error{},
	}

	dockerScanner := NewDockerScanner(nil)
	if dockerScanner.client == nil || !dockerScanner.client.IsAvailable() {
		// Docker not available, return empty result
		return result
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Scan images in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		imageResult := dockerScanner.ScanDockerImages()
		mu.Lock()
		result.merge(imageResult)
		mu.Unlock()
	}()

	// Scan containers in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		containerResult := dockerScanner.ScanDockerContainers()
		mu.Lock()
		result.merge(containerResult)
		mu.Unlock()
	}()

	// Scan volumes in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		volumeResult := dockerScanner.ScanDockerVolumes()
		mu.Lock()
		result.merge(volumeResult)
		mu.Unlock()
	}()

	// Scan build cache in parallel
	wg.Add(1)
	go func() {
		defer wg.Done()
		buildCacheResult := dockerScanner.ScanDockerBuildCache()
		mu.Lock()
		result.merge(buildCacheResult)
		mu.Unlock()
	}()

	wg.Wait()
	return result
}

// ScanDockerImages scans for cleanable Docker images
func (ds *DockerScanner) ScanDockerImages() *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: "docker_images",
		Errors:   []error{},
	}

	if ds.client == nil {
		return result
	}

	images, err := ds.client.ListImages()
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	for _, image := range images {
		// Safety: Only clean dangling images by default
		if ds.config.OnlyDanglingImages && !image.Dangling {
			continue
		}

		// Check age threshold
		if ds.config.ImageAgeThreshold > 0 && time.Since(image.CreatedAt) < ds.config.ImageAgeThreshold {
			continue
		}

		// Create FileInfo for the image
		tags := strings.Join(image.RepoTags, ", ")
		if tags == "" || tags == "<none>:<none>" {
			tags = "dangling"
		}

		fileInfo := FileInfo{
			Path:     fmt.Sprintf("docker:image:%s", image.ID[:12]),
			Size:     image.Size,
			ModTime:  image.CreatedAt,
			Category: "docker_images",
			Reason:   fmt.Sprintf("Docker image (%s)", tags),
			Hash:     image.ID,
		}

		result.Files = append(result.Files, fileInfo)
		result.TotalSize += image.Size
		result.TotalCount++
	}

	return result
}

// ScanDockerContainers scans for cleanable Docker containers
func (ds *DockerScanner) ScanDockerContainers() *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: "docker_containers",
		Errors:   []error{},
	}

	if ds.client == nil {
		return result
	}

	containers, err := ds.client.ListContainers(true)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	for _, container := range containers {
		// Safety: NEVER delete running containers
		if container.State == "running" {
			continue
		}

		// Only clean stopped containers if configured
		if ds.config.OnlyStoppedContainers && container.State != "exited" && container.State != "dead" {
			continue
		}

		// Check age threshold
		if ds.config.ContainerAgeThreshold > 0 && time.Since(container.CreatedAt) < ds.config.ContainerAgeThreshold {
			continue
		}

		// Calculate container size
		size := container.SizeRw
		if size == 0 {
			size = container.SizeRootFs
		}

		name := "unnamed"
		if len(container.Names) > 0 {
			name = strings.TrimPrefix(container.Names[0], "/")
		}

		fileInfo := FileInfo{
			Path:     fmt.Sprintf("docker:container:%s", container.ID[:12]),
			Size:     size,
			ModTime:  container.CreatedAt,
			Category: "docker_containers",
			Reason:   fmt.Sprintf("Stopped container (%s, state: %s)", name, container.State),
			Hash:     container.ID,
		}

		result.Files = append(result.Files, fileInfo)
		result.TotalSize += size
		result.TotalCount++
	}

	return result
}

// ScanDockerVolumes scans for cleanable Docker volumes
func (ds *DockerScanner) ScanDockerVolumes() *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: "docker_volumes",
		Errors:   []error{},
	}

	if ds.client == nil {
		return result
	}

	volumes, err := ds.client.ListVolumes()
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	for _, volume := range volumes {
		// Safety: Only clean unused volumes
		if ds.config.OnlyUnusedVolumes && volume.UsageData != nil && volume.UsageData.RefCount > 0 {
			continue
		}

		size := int64(0)
		if volume.UsageData != nil {
			size = volume.UsageData.Size
		}

		fileInfo := FileInfo{
			Path:     fmt.Sprintf("docker:volume:%s", volume.Name),
			Size:     size,
			ModTime:  time.Now(), // Volumes don't have creation time in list
			Category: "docker_volumes",
			Reason:   fmt.Sprintf("Unused Docker volume (driver: %s)", volume.Driver),
			Hash:     volume.Name,
		}

		result.Files = append(result.Files, fileInfo)
		result.TotalSize += size
		result.TotalCount++
	}

	return result
}

// ScanDockerBuildCache scans for Docker build cache
func (ds *DockerScanner) ScanDockerBuildCache() *ScanResult {
	result := &ScanResult{
		Files:    []FileInfo{},
		Category: "docker_build_cache",
		Errors:   []error{},
	}

	if ds.client == nil {
		return result
	}

	buildCache, err := ds.client.GetBuildCache()
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	for _, cache := range buildCache {
		// Skip items in use
		if cache.InUse {
			continue
		}

		fileInfo := FileInfo{
			Path:     fmt.Sprintf("docker:buildcache:%s", cache.ID[:12]),
			Size:     cache.Size,
			ModTime:  time.Now(),
			Category: "docker_build_cache",
			Reason:   fmt.Sprintf("Build cache (%s)", cache.Type),
			Hash:     cache.ID,
		}

		result.Files = append(result.Files, fileInfo)
		result.TotalSize += cache.Size
		result.TotalCount++
	}

	return result
}

