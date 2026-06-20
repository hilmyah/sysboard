package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// ContainerInfo is one row in the containers table.
type ContainerInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	Status string `json:"status"`
	State  string `json:"state"`
	Ports  string `json:"ports"`
	Engine string `json:"engine"`
}

// commandExists returns true if the named binary is found in PATH.
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// getDockerContainers runs `cmd ps -a` and parses the pipe-delimited output.
// Works for docker, podman, and nerdctl since they share the same CLI surface.
func getDockerContainers(engine, cmd string) []ContainerInfo {
	out, err := exec.Command(cmd, "ps", "-a", "--format",
		"{{.ID}}|{{.Names}}|{{.Image}}|{{.Status}}|{{.State}}|{{.Ports}}").Output()
	if err != nil {
		return nil
	}
	var containers []ContainerInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "|", 6)
		if len(parts) == 6 && parts[0] != "" {
			containers = append(containers, ContainerInfo{
				ID:     parts[0],
				Name:   strings.TrimPrefix(parts[1], "/"),
				Image:  parts[2],
				Status: parts[3],
				State:  parts[4],
				Ports:  parts[5],
				Engine: engine,
			})
		}
	}
	return containers
}

func handleContainerList(w http.ResponseWriter, r *http.Request) {
	var all []ContainerInfo
	if commandExists("docker") {
		if _, err := os.Stat("/var/run/docker.sock"); err == nil {
			all = append(all, getDockerContainers("docker", "docker")...)
		}
	}
	if commandExists("podman") {
		all = append(all, getDockerContainers("podman", "podman")...)
	}
	if commandExists("nerdctl") {
		all = append(all, getDockerContainers("containerd", "nerdctl")...)
	}
	if all == nil {
		all = []ContainerInfo{}
	}
	json.NewEncoder(w).Encode(all)
}

func handleContainerAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	id, action, engine := req["id"], req["action"], req["engine"]
	allowed := map[string]bool{"start": true, "stop": true, "restart": true}
	if !allowed[action] || id == "" || strings.ContainsAny(id, " ;|&`$(){}\\") {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	cmd := "docker"
	if engine == "podman" {
		cmd = "podman"
	} else if engine == "containerd" {
		cmd = "nerdctl"
	}
	out, err := exec.Command(cmd, action, id).CombinedOutput()
	result := map[string]string{"output": string(out)}
	if err != nil {
		result["error"] = err.Error()
	}
	json.NewEncoder(w).Encode(result)
}
