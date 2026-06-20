// containers_stub.go is used in place of containers.go when the install script
// is run without container support selected. It satisfies the handler function
// signatures expected by main.go but returns empty/disabled responses.
//
// To switch to full container support:
//   1. Replace this file with containers.go from the repository.
//   2. Rebuild: go build -ldflags="-s -w" -o sysboard .
//   3. Restart: systemctl restart sysboard

package main

import (
	"encoding/json"
	"net/http"
)

func handleContainerList(w http.ResponseWriter, r *http.Request) {
	// Return an empty array so the frontend shows "0 containers".
	json.NewEncoder(w).Encode([]struct{}{})
}

func handleContainerAction(w http.ResponseWriter, r *http.Request) {
	http.Error(w, `{"error":"container support not enabled in this build"}`, http.StatusNotImplemented)
}
