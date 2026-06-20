package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ServiceStatus is one row in the services table.
type ServiceStatus struct {
	Name      string `json:"name"`
	Active    string `json:"active"`
	Sub       string `json:"sub"`
	Desc      string `json:"desc"`
	MemBytes  uint64 `json:"mem_bytes"`
	UptimeSec int64  `json:"uptime_sec"`
	UptimeStr string `json:"uptime_str"`
}

// getServiceMemAndUptime queries MemoryCurrent and ActiveEnterTimestamp for one service.
func getServiceMemAndUptime(name string) (memBytes uint64, uptimeSec int64) {
	out, err := exec.Command("systemctl", "show", name+".service",
		"--property=MemoryCurrent,ActiveEnterTimestamp").Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "MemoryCurrent=") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "MemoryCurrent="))
			if val != "[not set]" && val != "" {
				v, err := strconv.ParseUint(val, 10, 64)
				if err == nil {
					memBytes = v
				}
			}
		}
		if strings.HasPrefix(line, "ActiveEnterTimestamp=") {
			ts := strings.TrimSpace(strings.TrimPrefix(line, "ActiveEnterTimestamp="))
			if ts != "" && ts != "n/a" {
				t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", ts)
				if err == nil {
					uptimeSec = int64(time.Since(t).Seconds())
				}
			}
		}
	}
	return
}

// uptimeFmt converts seconds to a human-readable string.
func uptimeFmt(sec int64) string {
	if sec <= 0 {
		return ""
	}
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	if sec < 3600 {
		return fmt.Sprintf("%dm%ds", sec/60, sec%60)
	}
	if sec < 86400 {
		return fmt.Sprintf("%dh%dm", sec/3600, (sec%3600)/60)
	}
	return fmt.Sprintf("%dd%dh", sec/86400, (sec%86400)/3600)
}

func handleServices(w http.ResponseWriter, r *http.Request) {
	out, err := exec.Command("systemctl", "list-units", "--type=service",
		"--all", "--no-pager", "--plain", "--no-legend").Output()
	var statuses []ServiceStatus
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			name := strings.TrimSuffix(fields[0], ".service")
			active := fields[2]
			sub := fields[3]
			desc := strings.Join(fields[4:], " ")
			var mem uint64
			var upSec int64
			if active == "active" {
				mem, upSec = getServiceMemAndUptime(name)
			}
			statuses = append(statuses, ServiceStatus{
				Name: name, Active: active, Sub: sub, Desc: desc,
				MemBytes: mem, UptimeSec: upSec, UptimeStr: uptimeFmt(upSec),
			})
		}
	}
	json.NewEncoder(w).Encode(statuses)
}

func handleServiceAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	action, svc := req["action"], req["service"]
	allowed := map[string]bool{"start": true, "stop": true, "restart": true}
	if !allowed[action] || svc == "" || strings.ContainsAny(svc, " ;|&`$(){}\\") {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}
	out, err := exec.Command("systemctl", action, svc).CombinedOutput()
	result := map[string]string{"output": string(out)}
	if err != nil {
		result["error"] = err.Error()
	}
	json.NewEncoder(w).Encode(result)
}
