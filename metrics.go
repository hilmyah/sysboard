package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SystemMetrics is the payload returned by GET /api/metrics.
type SystemMetrics struct {
	CPUPercent float64    `json:"cpu_percent"`
	RAMTotal   uint64     `json:"ram_total"`
	RAMUsed    uint64     `json:"ram_used"`
	RAMPercent float64    `json:"ram_percent"`
	Disks      []DiskInfo `json:"disks"`
	Uptime     string     `json:"uptime"`
	LoadAvg    string     `json:"load_avg"`
	Timestamp  string     `json:"timestamp"`
	NetRxBytes uint64     `json:"net_rx_bytes"`
	NetTxBytes uint64     `json:"net_tx_bytes"`
	NetRxRate  float64    `json:"net_rx_rate"`
	NetTxRate  float64    `json:"net_tx_rate"`
	CPUTemp    float64    `json:"cpu_temp"`
}

// DiskInfo holds per-mount disk usage.
type DiskInfo struct {
	Mount   string  `json:"mount"`
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Free    uint64  `json:"free"`
	Percent float64 `json:"percent"`
}

// parseProcStat reads the aggregate CPU line from /proc/stat.
func parseProcStat() (idle, total uint64) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			var vals []uint64
			for _, s := range fields[1:] {
				v, _ := strconv.ParseUint(s, 10, 64)
				vals = append(vals, v)
			}
			if len(vals) >= 4 {
				idle = vals[3]
				for _, v := range vals {
					total += v
				}
			}
			break
		}
	}
	return
}

// getCPUPercent samples /proc/stat twice over 200ms and returns CPU utilization.
func getCPUPercent() float64 {
	idle1, total1 := parseProcStat()
	time.Sleep(200 * time.Millisecond)
	idle2, total2 := parseProcStat()
	dt := total2 - total1
	di := idle2 - idle1
	if dt == 0 {
		return 0
	}
	return (1 - float64(di)/float64(dt)) * 100
}

// getMemInfo reads MemTotal and MemAvailable from /proc/meminfo.
func getMemInfo() (total, used, free uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()
	vals := map[string]uint64{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 {
			v, _ := strconv.ParseUint(fields[1], 10, 64)
			vals[strings.TrimSuffix(fields[0], ":")] = v * 1024
		}
	}
	total = vals["MemTotal"]
	free = vals["MemAvailable"]
	used = total - free
	return
}

// getDiskMounts returns real block-device mount points from /proc/mounts,
// excluding virtual filesystems, /sys, /proc, /dev, and /run paths.
func getDiskMounts() []string {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return []string{"/"}
	}
	defer f.Close()

	skip := map[string]bool{
		"tmpfs": true, "devtmpfs": true, "sysfs": true, "proc": true,
		"cgroup": true, "cgroup2": true, "pstore": true, "securityfs": true,
		"debugfs": true, "configfs": true, "fusectl": true, "hugetlbfs": true,
		"mqueue": true, "bpf": true, "tracefs": true, "efivarfs": true,
		"autofs": true, "overlay": true, "squashfs": true,
	}
	skipPrefix := []string{"/sys", "/proc", "/dev", "/run"}

	seen := map[string]bool{}
	var mounts []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		fstype := fields[2]
		mount := fields[1]
		if skip[fstype] {
			continue
		}
		ignored := false
		for _, p := range skipPrefix {
			if strings.HasPrefix(mount, p) {
				ignored = true
				break
			}
		}
		if ignored || seen[mount] {
			continue
		}
		seen[mount] = true
		mounts = append(mounts, mount)
	}
	if len(mounts) == 0 {
		return []string{"/"}
	}
	sort.Strings(mounts)
	return mounts
}

// getDiskInfo calls df -B1 for a single mount point.
func getDiskInfo(mount string) DiskInfo {
	out, err := exec.Command("df", "-B1", mount).Output()
	if err != nil {
		return DiskInfo{Mount: mount}
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) >= 2 {
		fields := strings.Fields(lines[1])
		if len(fields) >= 5 {
			total, _ := strconv.ParseUint(fields[1], 10, 64)
			used, _ := strconv.ParseUint(fields[2], 10, 64)
			free, _ := strconv.ParseUint(fields[3], 10, 64)
			pct := 0.0
			if total > 0 {
				pct = float64(used) / float64(total) * 100
			}
			return DiskInfo{Mount: mount, Total: total, Used: used, Free: free, Percent: pct}
		}
	}
	return DiskInfo{Mount: mount}
}

type netStat struct {
	rx, tx uint64
}

// readNetStats sums RX/TX bytes across all non-loopback interfaces.
func readNetStats() netStat {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return netStat{}
	}
	defer f.Close()
	var rx, tx uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "lo:") || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) >= 9 {
			r, _ := strconv.ParseUint(fields[0], 10, 64)
			t, _ := strconv.ParseUint(fields[8], 10, 64)
			rx += r
			tx += t
		}
	}
	return netStat{rx: rx, tx: tx}
}

var (
	lastNetStat netStat
	lastNetTime time.Time
)

// getNetRate returns cumulative bytes and per-second rates since last call.
func getNetRate() (rxBytes, txBytes uint64, rxRate, txRate float64) {
	cur := readNetStats()
	now := time.Now()
	if !lastNetTime.IsZero() {
		elapsed := now.Sub(lastNetTime).Seconds()
		if elapsed > 0 {
			rxRate = float64(cur.rx-lastNetStat.rx) / elapsed
			txRate = float64(cur.tx-lastNetStat.tx) / elapsed
		}
	}
	lastNetStat = cur
	lastNetTime = now
	return cur.rx, cur.tx, rxRate, txRate
}

// getCPUTemp reads from hwmon (preferred) or thermal_zone as fallback.
func getCPUTemp() float64 {
	matches, err := filepath.Glob("/sys/class/hwmon/hwmon*/temp*_input")
	if err == nil {
		for _, m := range matches {
			data, err := os.ReadFile(m)
			if err == nil {
				v, _ := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
				if v > 0 {
					return v / 1000.0
				}
			}
		}
	}
	for i := 0; i < 5; i++ {
		data, err := os.ReadFile(fmt.Sprintf("/sys/class/thermal/thermal_zone%d/temp", i))
		if err == nil {
			v, _ := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
			if v > 1000 {
				return v / 1000.0
			}
		}
	}
	return 0
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	cpu := getCPUPercent()
	total, used, _ := getMemInfo()
	ramPct := 0.0
	if total > 0 {
		ramPct = float64(used) / float64(total) * 100
	}

	mounts := getDiskMounts()
	disks := make([]DiskInfo, 0, len(mounts))
	for _, m := range mounts {
		disks = append(disks, getDiskInfo(m))
	}

	uptimeOut, _ := os.ReadFile("/proc/uptime")
	uptime := "unknown"
	if len(uptimeOut) > 0 {
		var secs float64
		fmt.Sscanf(string(uptimeOut), "%f", &secs)
		d := time.Duration(secs) * time.Second
		uptime = fmt.Sprintf("%dd %dh %dm", int(d.Hours())/24, int(d.Hours())%24, int(d.Minutes())%60)
	}

	loadOut, _ := os.ReadFile("/proc/loadavg")
	loadAvg := "unknown"
	if len(loadOut) > 0 {
		fields := strings.Fields(string(loadOut))
		if len(fields) >= 3 {
			loadAvg = strings.Join(fields[:3], " ")
		}
	}

	rxBytes, txBytes, rxRate, txRate := getNetRate()
	cpuTemp := getCPUTemp()

	json.NewEncoder(w).Encode(SystemMetrics{
		CPUPercent: cpu, RAMTotal: total, RAMUsed: used, RAMPercent: ramPct,
		Disks: disks, Uptime: uptime, LoadAvg: loadAvg, Timestamp: time.Now().Format("15:04:05"),
		NetRxBytes: rxBytes, NetTxBytes: txBytes, NetRxRate: rxRate, NetTxRate: txRate,
		CPUTemp: cpuTemp,
	})
}
