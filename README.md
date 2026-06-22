<div align="center">
  <h1>SysBoard</h1>
  <p>Dashboard sistem Linux self-hosted berbasis Go stdlib dan vanilla JS, berjalan sebagai binary tunggal tanpa dependensi runtime.</p>
</div>

<p align="center">
  <img src="https://img.shields.io/badge/Go-%3E%3D1.21-00ADD8?logo=go&logoColor=white" alt="Go Version">
  <img src="https://img.shields.io/badge/License-MIT-green" alt="License">
  <img src="https://img.shields.io/badge/Platform-Linux-lightgrey?logo=linux" alt="Platform">
  <img src="https://img.shields.io/badge/Runtime_Dependencies-none-brightgreen" alt="Zero Dependencies">
</p>

---

SysBoard adalah dashboard sistem Linux self-hosted yang dibangun dengan Go (stdlib only) dan vanilla JS. Berjalan sebagai binary tunggal tanpa dependensi runtime dan tidak melakukan request jaringan eksternal saat halaman dimuat.

## Daftar Isi

- [Fitur](#fitur)
- [Konsep dan Arsitektur](#konsep-dan-arsitektur)
- [Struktur Repository](#struktur-repository)
- [Prasyarat](#prasyarat)
- [Instalasi](#instalasi)
- [Konfigurasi Environment](#konfigurasi-environment)
- [Deployment](#deployment)
- [Referensi API](#referensi-api)
- [Manajemen dan Operasional](#manajemen-dan-operasional)
- [Pembaruan](#pembaruan)
- [Verifikasi](#verifikasi)
- [Keamanan](#keamanan)
- [Kontribusi](#kontribusi)
- [Lisensi](#lisensi)

---

## Fitur

| Fitur | Deskripsi |
|---|---|
| Overview | Penggunaan CPU, RAM, suhu CPU (hwmon + thermal_zone sebagai fallback), uptime sistem, kecepatan RX/TX jaringan beserta akumulator kumulatif, penggunaan disk per mount (auto-detect dari `/proc/mounts`). |
| Proses | 30 proses teratas berdasarkan konsumsi CPU, dibaca langsung dari `/proc`. Menampilkan PID, CPU%, RAM, user, dan state. Dapat difilter berdasarkan nama. |
| Layanan | Seluruh unit systemd dengan penggunaan RAM dan uptime per layanan. Dapat diurutkan berdasarkan nama, RAM, atau status. Pencarian teks penuh. Aksi start / stop / restart. |
| Container | Deteksi container multi-engine: Docker, Podman, containerd (via nerdctl). Deteksi engine bersifat otomatis. Aksi start / stop / restart. Fitur ini opsional saat instalasi. |

---

## Konsep dan Arsitektur

SysBoard berjalan sebagai satu proses Go yang sekaligus menyajikan file statis frontend dan menangani seluruh endpoint API. Semua data metrik dibaca langsung dari antarmuka kernel Linux tanpa memanggil utilitas eksternal.

```text
+------------+     HTTP (port 8888)      +---------------------------+
|  Browser   | <-----------------------> |   SysBoard (Go binary)    |
+------------+                           +-------------+-------------+
                                                       |
                              +------------------------+------------------------+
                              |                        |                        |
                              v                        v                        v
                    /proc, /sys/fs              systemd (D-Bus)         Docker / Podman /
                    (CPU, RAM, disk,           (service listing,        containerd socket
                     proses, network,           start/stop/restart)     (opsional)
                     suhu CPU)
```

Proses Go tunggal menangani semua concern: routing HTTP, autentikasi token, pembacaan metrik, manajemen layanan, dan manajemen container. Frontend (HTML, CSS, JS) disajikan langsung oleh file server bawaan Go dari direktori `static/` tanpa CDN atau aset eksternal.

---

## Struktur Repository

```
SysBoard/
├── main.go                  Routing dan entry point
├── middleware.go            CORS headers, auth middleware, login handler
├── metrics.go               CPU, RAM, disk, network, temperature
├── processes.go             Pembacaan proses berbasis /proc
├── services.go              Manajemen layanan systemd
├── containers.go            Dukungan container (Docker/Podman/nerdctl)
├── containers_stub.go       No-op pengganti containers.go bila fitur tidak dipilih
├── go.mod
├── static/
│   ├── index.html           Struktur HTML
│   ├── style.css            Seluruh style
│   └── app.js               Seluruh logika frontend
├── systemd/
│   └── sysboard.service     Template unit systemd
├── .env.example             Template konfigurasi
├── .gitignore
├── install.sh               Installer interaktif
└── README.md
```

---

## Prasyarat

| Komponen | Spesifikasi / Versi | Keterangan |
|---|---|---|
| Linux | Ubuntu 20.04+ / Debian 11+ | Sistem operasi target deployment |
| Go | >= 1.21 | Hanya diperlukan saat build; binary tidak memiliki dependensi runtime |
| systemd | - | Diperlukan untuk manajemen dan pengukuran metrik layanan |

### Port Jaringan

| Port | Protokol | Arah | Deskripsi |
|---|---|---|---|
| `8888` | TCP | Inbound | Port HTTP default SysBoard (dapat diubah via `SYSBOARD_PORT`) |

---

## Instalasi

### Instalasi Cepat

```bash
curl -fsSL https://raw.githubusercontent.com/hilmyah/SysBoard/master/install.sh | bash
```

Skrip akan menanyakan apakah fitur Container ingin disertakan, kemudian mengunduh, membangun, dan menginstal service secara otomatis. Menjalankan ulang skrip pada instalasi yang sudah ada akan melakukan pembaruan.

### Instalasi Manual

**1. Clone dan konfigurasi**

```bash
git clone https://github.com/hilmyah/SysBoard.git /opt/sysboard
cd /opt/sysboard

cp .env.example .env
chmod 600 .env
```

Buat nilai token dan isi `SYSBOARD_TOKEN` pada `.env`:

```bash
openssl rand -hex 32
```

**2. Build**

Dengan dukungan container (default):

```bash
go build -ldflags="-s -w" -o sysboard .
```

Tanpa dukungan container (menggunakan `containers_stub.go` sebagai pengganti `containers.go`):

```bash
# Hapus atau keluarkan containers.go sebelum build.
# install.sh menangani ini secara otomatis.
```

---

## Konfigurasi Environment

Salin `.env.example` menjadi `.env`:

```bash
cp .env.example .env
```

| Variabel | Wajib | Default | Deskripsi |
|---|:---:|---|---|
| `SYSBOARD_TOKEN` | Ya | - | Token statis untuk seluruh autentikasi API |
| `SYSBOARD_PORT` | Tidak | `8888` | Port TCP yang digunakan |

Binary akan menolak berjalan apabila `SYSBOARD_TOKEN` tidak diatur.

Untuk mengubah konfigurasi tanpa build ulang:

```bash
nano /opt/sysboard/.env
systemctl restart sysboard
```

---

## Deployment

### Systemd Service

```bash
cp systemd/sysboard.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now sysboard
```

### Rebuild Setelah Mengubah Source

```bash
cd /opt/sysboard
go build -ldflags="-s -w" -o sysboard .
systemctl restart sysboard
```

---

## Referensi API

Seluruh endpoint kecuali `/api/login` membutuhkan header `X-Auth-Token: <token>`.

| Method | Endpoint | Deskripsi |
|:---:|---|---|
| `POST` | `/api/login` | `{"token":"..."}` -- mengembalikan `{"ok":true}` |
| `GET` | `/api/metrics` | CPU, RAM, disk, suhu, network I/O |
| `GET` | `/api/processes` | 30 proses teratas berdasarkan CPU |
| `GET` | `/api/services` | Seluruh layanan systemd beserta RAM dan uptime |
| `POST` | `/api/services/action` | `{"service":"nama","action":"start\|stop\|restart"}` |
| `GET` | `/api/containers` | Container dari seluruh engine yang terdeteksi |
| `POST` | `/api/containers/action` | `{"id":"...","engine":"docker\|podman\|containerd","action":"..."}` |

File statis (`/`, `/style.css`, `/app.js`) disajikan dari `./static/` oleh file server bawaan Go.

---

## Manajemen dan Operasional

### Perintah Layanan

```bash
systemctl start sysboard
systemctl stop sysboard
systemctl status sysboard
```

### Lokasi Log

| Sumber | Isi |
|---|---|
| `journalctl -u sysboard -f` | Log runtime SysBoard secara realtime |
| `journalctl -u sysboard --no-pager -n 50` | 50 baris log terakhir |

---

## Pembaruan

```bash
curl -fsSL https://raw.githubusercontent.com/hilmyah/SysBoard/master/install.sh | bash
```

Menjalankan ulang skrip pada instalasi yang sudah ada secara otomatis melakukan pembaruan: menarik source terbaru, melakukan build ulang, dan me-restart service.

---

## Verifikasi

```bash
systemctl status sysboard
journalctl -u sysboard -f

curl -s http://localhost:8888/api/login \
  -H 'Content-Type: application/json' \
  -d '{"token":"YOUR_TOKEN"}'
```

Output yang diharapkan:

```json
{"ok":true}
```

---

## Keamanan

- `.env` menyimpan token autentikasi. Jaga permission pada `600` dengan owner `root`.
- Service berjalan via HTTP polos. Untuk akses publik, gunakan reverse proxy dengan TLS (nginx, Caddy).
- Aksi layanan dijalankan sebagai `root`. Hanya ekspos dashboard ke jaringan tepercaya.
- Frontend tidak melakukan request jaringan eksternal: tanpa CDN, tanpa font eksternal, tanpa analytics.

---

## Kontribusi

1. Fork repository.
2. Buat branch fitur: `git checkout -b feature/nama-fitur`.
3. Jaga `go.mod` bebas dari dependensi eksternal -- stdlib only.
4. Uji pada sistem Linux nyata; kode ini membaca langsung dari `/proc` dan `/sys`.
5. Buka pull request dengan deskripsi perubahan yang jelas.

Laporan bug: sertakan output `journalctl -u sysboard --no-pager -n 50`.

---

## Lisensi

SysBoard dirilis di bawah lisensi MIT.
