package main

import (
	"log"
	"net/http"
	"os"
)

var (
	listenPort  string
	staticToken string
)

func main() {
	staticToken = os.Getenv("SYSBOARD_TOKEN")
	if staticToken == "" {
		log.Fatal("SYSBOARD_TOKEN is not set; set it in /opt/sysboard/.env and reload the service")
	}

	port := os.Getenv("SYSBOARD_PORT")
	if port == "" {
		port = "8888"
	}
	listenPort = ":" + port

	mux := http.NewServeMux()

	// Static files: index.html, style.css, app.js served from ./static/
	mux.Handle("/", http.FileServer(http.Dir("./static")))

	// API
	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/metrics", authMiddleware(handleMetrics))
	mux.HandleFunc("/api/processes", authMiddleware(handleProcesses))
	mux.HandleFunc("/api/services", authMiddleware(handleServices))
	mux.HandleFunc("/api/services/action", authMiddleware(handleServiceAction))
	mux.HandleFunc("/api/containers", authMiddleware(handleContainerList))
	mux.HandleFunc("/api/containers/action", authMiddleware(handleContainerAction))

	log.Printf("SysBoard listening on %s", listenPort)
	if err := http.ListenAndServe(listenPort, mux); err != nil {
		log.Fatal(err)
	}
}
