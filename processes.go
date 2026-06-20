package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ProcessInfo is one row in the process table.
type ProcessInfo struct {
	PID      int     `json:"pid"`
	Name     string  `json:"name"`
	CPUPct   float64 `json:"cpu_pct"`
	MemBytes uint64  `json:"mem_bytes"`
	State    string  `json:"state"`
	User     string  `json:"user"`
}

// readProcStat reads /proc/<pid>/stat. Returns name, state, and utime+stime in ticks.
// comm is extracted between the first '(' and last ')' to handle names with spaces.
func readProcStat(pid int) (name, state string, ticks uint64) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return
	}
	s := string(data)
	start := strings.Index(s, "(")
	end := strings.LastIndex(s, ")")
	if start < 0 || end < 0 || end <= start {
		return
	}
	name = s[start+1 : end]
	rest := strings.Fields(s[end+2:])
	if len(rest) < 12 {
		return
	}
	state = rest[0]
	utime, _ := strconv.ParseUint(rest[11], 10, 64)
	stime, _ := strconv.ParseUint(rest[12], 10, 64)
	ticks = utime + stime
	return
}

// readProcMem reads VmRSS from /proc/<pid>/status.
func readProcMem(pid int) uint64 {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, _ := strconv.ParseUint(fields[1], 10, 64)
				return v * 1024
			}
		}
	}
	return 0
}

// readProcUser resolves the real UID from /proc/<pid>/status to a username.
func readProcUser(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				uid := fields[1]
				if uid == "0" {
					return "root"
				}
				return resolveUID(uid)
			}
		}
	}
	return ""
}

// resolveUID does a fast scan of /etc/passwd to map a UID to a username.
func resolveUID(uid string) string {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return uid
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), ":", 4)
		if len(parts) >= 3 && parts[2] == uid {
			return parts[0]
		}
	}
	return uid
}

// clkTck is the number of clock ticks per second (100 on virtually all Linux systems).
const clkTck = 100

var (
	prevProcTicks = map[int]uint64{}
	prevProcTime  time.Time
)

func handleProcesses(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		json.NewEncoder(w).Encode([]ProcessInfo{})
		return
	}

	now := time.Now()
	elapsed := now.Sub(prevProcTime).Seconds()
	if prevProcTime.IsZero() {
		elapsed = 1
	}
	prevProcTime = now

	curTicks := map[int]uint64{}
	var procs []ProcessInfo

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		name, state, ticks := readProcStat(pid)
		if name == "" {
			continue
		}
		curTicks[pid] = ticks
		cpuPct := 0.0
		if prev, ok := prevProcTicks[pid]; ok && elapsed > 0 {
			delta := float64(ticks-prev) / clkTck
			cpuPct = (delta / elapsed) * 100
		}
		mem := readProcMem(pid)
		user := readProcUser(pid)
		procs = append(procs, ProcessInfo{
			PID: pid, Name: name, CPUPct: cpuPct,
			MemBytes: mem, State: state, User: user,
		})
	}

	prevProcTicks = curTicks

	// Sort by CPU desc, memory desc as tiebreaker.
	sort.Slice(procs, func(i, j int) bool {
		if procs[i].CPUPct != procs[j].CPUPct {
			return procs[i].CPUPct > procs[j].CPUPct
		}
		return procs[i].MemBytes > procs[j].MemBytes
	})

	top := 30
	if len(procs) < top {
		top = len(procs)
	}
	json.NewEncoder(w).Encode(procs[:top])
}
