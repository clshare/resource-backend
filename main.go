package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"text/template"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// Response structure for JSON response
type Response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

// Request structure for starting the Docker container
type StartContainerRequest struct {
	CPUs    string `json:"cpus"`
	Memory  string `json:"memory"`
	Storage string `json:"storage"`
	Port    string `json:"port"`
}

// Create Dockerfile dynamically
func createDockerfile() error {
	dockerfileContent := `
# Use the official Debian image as the base image
FROM debian:latest

# Install SSH server
RUN apt-get update && \
    apt-get install -y openssh-server && \
    mkdir /var/run/sshd && \
    echo 'root:password' | chpasswd && \
    sed -i 's/PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config && \
    echo 'ClientAliveInterval 60' >> /etc/ssh/sshd_config && \
    echo 'ClientAliveCountMax 5' >> /etc/ssh/sshd_config

# Expose SSH port
EXPOSE 22

# Start SSH service
CMD ["/usr/sbin/sshd", "-D"]
`
	return ioutil.WriteFile("Dockerfile", []byte(dockerfileContent), 0644)
}

// Build Docker image
func buildDockerImage() error {
	cmd := exec.Command("docker", "build", "-t", "debian-ssh", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Create Docker Compose file dynamically
func createDockerComposeFile(data map[string]string) error {
	composeTemplate := `
version: '3.7'

services:
  debian-ssh:
    image: debian-ssh
    deploy:
      resources:
        limits:
          cpus: "{{.CPUS}}"
          memory: "{{.MEMORY}}M"
    ports:
      - "{{.PORT}}:22"
    storage_opt:
      size: "{{.STORAGE}}M"
`
	tmpl, err := template.New("docker-compose").Parse(composeTemplate)
	if err != nil {
		return err
	}

	var composeFile bytes.Buffer
	if err := tmpl.Execute(&composeFile, data); err != nil {
		return err
	}

	return ioutil.WriteFile("docker-compose.yml", composeFile.Bytes(), 0644)
}

// Run Docker Compose
func runDockerCompose() (string, error) {
	cmd := exec.Command("docker-compose", "up", "-d")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// StartContainerHandler handles starting a Docker container with specified resources
func StartContainerHandler(w http.ResponseWriter, r *http.Request) {
	var req StartContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cpuCount, err := cpu.Counts(true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	virtMem, err := mem.VirtualMemory()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	diskUsage, err := disk.Usage("/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	requiredCPUs, err := strconv.Atoi(req.CPUs)
	if err != nil {
		http.Error(w, "Invalid CPU value", http.StatusBadRequest)
		return
	}

	requiredMemory, err := strconv.Atoi(req.Memory)
	if err != nil {
		http.Error(w, "Invalid Memory value", http.StatusBadRequest)
		return
	}

	requiredStorage, err := strconv.Atoi(req.Storage)
	if err != nil {
		http.Error(w, "Invalid Storage value", http.StatusBadRequest)
		return
	}

	if requiredCPUs > cpuCount || uint64(requiredMemory*1024*1024) > virtMem.Available || uint64(requiredStorage*1024*1024) > diskUsage.Free {
		http.Error(w, "Insufficient resources", http.StatusForbidden)
		return
	}

	// Create Dockerfile
	if err := createDockerfile(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build Docker image
	if err := buildDockerImage(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate Docker Compose file
	data := map[string]string{
		"CPUS":    req.CPUs,
		"MEMORY":  req.Memory,
		"STORAGE": req.Storage,
		"PORT":    req.Port,
	}

	if err := createDockerComposeFile(data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Run Docker Compose
	output, err := runDockerCompose()
	if err != nil {
		http.Error(w, output, http.StatusInternalServerError)
		return
	}

	resp := Response{
		Status: "success",
		Data: map[string]string{
			"docker_compose_output": output,
			"ssh_url":               "ssh root@localhost -p " + req.Port,
			"password":              "password",
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetRAMHandler handles the RAM usage endpoint
func GetRAMHandler(w http.ResponseWriter, r *http.Request) {
	v, err := mem.VirtualMemory()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := Response{
		Status: "success",
		Data:   v,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetCPUCoresHandler handles the CPU cores count endpoint
func GetCPUCoresHandler(w http.ResponseWriter, r *http.Request) {
	c, err := cpu.Counts(true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := Response{
		Status: "success",
		Data:   c,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GetStorageHandler handles the system free storage endpoint
func GetStorageHandler(w http.ResponseWriter, r *http.Request) {
	d, err := disk.Usage("/")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := Response{
		Status: "success",
		Data:   d,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	http.HandleFunc("/ram", GetRAMHandler)
	http.HandleFunc("/cpu", GetCPUCoresHandler)
	http.HandleFunc("/storage", GetStorageHandler)
	http.HandleFunc("/start-container", StartContainerHandler)

	port := ":8085"
	fmt.Printf("Server is running on port %s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		fmt.Printf("Failed to start server: %s\n", err)
	}
}
