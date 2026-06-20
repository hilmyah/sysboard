# SysBoard

![Go Version](https://img.shields.io/badge/Go-%3E%3D1.21-00ADD8?logo=go&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green)
![Platform](https://img.shields.io/badge/Platform-Linux-lightgrey?logo=linux)
![Zero Dependencies](https://img.shields.io/badge/Runtime_Dependencies-none-brightgreen)

Self-hosted Linux system dashboard built with Go (stdlib only) and vanilla JS. Runs as a single binary with no runtime dependencies and makes zero external network requests on page load.

---

## Features

**Overview** -- CPU usage, RAM, CPU temperature (hwmon + thermal_zone fallback), system uptime, network RX/TX rates with cumulative counters, disk usage per mount (auto-detected from `/proc/mounts`).

**Processes** -- Top 30 processes by CPU consumption, read from `/proc`. Shows PID, CPU%, RAM, user, and state. Filterable by name.

**Services** -- All systemd units with live RAM usage and uptime per service. Sortable by name, RAM, or status. Full-text search. Start / stop / restart actions.

**Containers** -- Multi-engine container discovery: Docker, Podman, containerd (via nerdctl). Engine detection is automatic. Start / stop / restart actions. Optional at install time.

---

## Requirements

- Linux (Ubuntu 20.04+ / Debian 11+ recommended)
- Go >= 1.21 (build-time only; binary has zero runtime deps)
- `systemd` (for service management and service metrics)

---

## Install

One-command install with optional feature selection:

```bash
curl -fsSL https://raw.githubusercontent.com/hilmyah/SysBoard/master/install.sh | bash
```

The script will ask whether to include Containers support, then download, build, and install the service automatically. Running it again on an existing install performs an update.

---

## Repository Structure

```
/
├── main.go                  routing and entry point
├── middleware.go            CORS headers, auth middleware, login handler
├── metrics.go               CPU, RAM, disk, network, temperature
├── processes.go             /proc-based process listing
├── services.go              systemd service management
├── containers.go            container support (Docker/Podman/nerdctl)
├── containers_stub.go       no-op replacement when containers not selected
├── go.mod
├── static/
│   ├── index.html           HTML structure
│   ├── style.css            all styles
│   └── app.js               all frontend logic
├── systemd/
│   └── sysboard.service     systemd unit template
├── .env.example             configuration template
├── .gitignore
├── install.sh               interactive installer
└── README.md
```

---

## Manual Deploy

### 1. Clone and configure

```bash
git clone https://github.com/hilmyah/SysBoard.git /opt/sysboard
cd /opt/sysboard

cp .env.example .env
chmod 600 .env
```

Edit `.env` and set `SYSBOARD_TOKEN`:

```bash
openssl rand -hex 32
```

### 2. Build

With container support (default):

```bash
go build -ldflags="-s -w" -o sysboard .
```

Without container support (uses containers_stub.go instead of containers.go):

```bash
# Remove or exclude containers.go before building.
# The install script handles this automatically.
```

### 3. Install and start

```bash
cp systemd/sysboard.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now sysboard
```

### 4. Verify

```bash
systemctl status sysboard
journalctl -u sysboard -f

curl -s http://localhost:8888/api/login \
  -H 'Content-Type: application/json' \
  -d '{"token":"YOUR_TOKEN"}'
# Expected: {"ok":true}
```

---

## Configuration (.env)

| Variable | Required | Default | Description |
|---|---|---|---|
| `SYSBOARD_TOKEN` | Yes | none | Static token for all API authentication |
| `SYSBOARD_PORT` | No | `8888` | TCP listen port |

The binary will refuse to start if `SYSBOARD_TOKEN` is not set.

To change configuration without rebuilding:

```bash
nano /opt/sysboard/.env
systemctl restart sysboard
```

---

## Rebuild After Editing Source

```bash
cd /opt/sysboard
go build -ldflags="-s -w" -o sysboard .
systemctl restart sysboard
```

---

## API Reference

All endpoints except `/api/login` require the header `X-Auth-Token: <token>`.

| Endpoint | Method | Description |
|---|---|---|
| `POST /api/login` | POST | `{"token":"..."}` -- returns `{"ok":true}` |
| `GET /api/metrics` | GET | CPU, RAM, disk, temperature, network I/O |
| `GET /api/processes` | GET | Top 30 processes by CPU |
| `GET /api/services` | GET | All systemd services with RAM and uptime |
| `POST /api/services/action` | POST | `{"service":"name","action":"start\|stop\|restart"}` |
| `GET /api/containers` | GET | Containers from all detected engines |
| `POST /api/containers/action` | POST | `{"id":"...","engine":"docker\|podman\|containerd","action":"..."}` |

Static files (`/`, `/style.css`, `/app.js`) are served from `./static/` by Go's built-in file server.

---

## Security Notes

- `.env` contains your token. Keep permissions at `600` and owner `root`.
- The binary is served over plain HTTP. For public exposure, put it behind a reverse proxy with TLS (nginx, Caddy).
- Service actions execute as `root`. Only expose the dashboard on trusted networks.
- The frontend makes zero external network requests: no CDN, no fonts, no analytics.

---

## Contributing

1. Fork the repository.
2. Create a feature branch: `git checkout -b feature/your-feature`.
3. Keep `go.mod` free of external dependencies -- stdlib only.
4. Test on a real Linux system; this code reads from `/proc` and `/sys` directly.
5. Open a pull request with a clear description of the change.

Bug reports: open an issue with `journalctl -u sysboard --no-pager -n 50`.
