package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/bihua-university/alisten/internal/snapshot"
)

var (
	mu         sync.RWMutex
	current    *snapshot.Snapshot
	persistDir string
)

func main() {
	addr := flag.String("addr", ":9090", "listen address")
	dir := flag.String("persist", "./snapshots", "persist directory")
	flag.Parse()

	persistDir = *dir
	os.MkdirAll(persistDir, 0755)

	// Load latest snapshot from disk if exists
	loadFromDisk()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /snapshot/save", handleSave)
	mux.HandleFunc("GET /snapshot/load", handleLoad)
	mux.HandleFunc("GET /snapshot/room/{id}", handleGetRoom)
	mux.HandleFunc("PUT /snapshot/room/{id}", handleUpdateRoom)
	mux.HandleFunc("DELETE /snapshot/room/{id}", handleDeleteRoom)
	mux.HandleFunc("GET /health", handleHealth)

	log.Printf("alisten-snapshot listening on %s, persist dir: %s", *addr, persistDir)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	count := 0
	if current != nil {
		count = len(current.Rooms)
	}
	mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"rooms":  count,
	})
}

func handleSave(w http.ResponseWriter, r *http.Request) {
	var snap snapshot.Snapshot
	if err := json.NewDecoder(r.Body).Decode(&snap); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	snap.SavedAt = time.Now().UnixMilli()
	snap.Version = 1

	mu.Lock()
	current = &snap
	mu.Unlock()

	go saveToDisk(&snap)

	writeJSON(w, http.StatusOK, map[string]any{
		"savedAt": snap.SavedAt,
		"rooms":   len(snap.Rooms),
	})
}

func handleLoad(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	snap := current
	mu.RUnlock()

	if snap == nil {
		writeJSON(w, http.StatusOK, snapshot.NewSnapshot())
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func handleGetRoom(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mu.RLock()
	snap := current
	mu.RUnlock()

	if snap == nil || snap.Rooms == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no snapshot"})
		return
	}
	room, ok := snap.Rooms[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "room not found"})
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func handleUpdateRoom(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var room snapshot.RoomState
	if err := json.NewDecoder(r.Body).Decode(&room); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	room.ID = id
	room.UpdatedAt = time.Now().UnixMilli()

	mu.Lock()
	if current == nil {
		current = snapshot.NewSnapshot()
	}
	current.Rooms[id] = &room
	current.SavedAt = time.Now().UnixMilli()
	snap := current
	mu.Unlock()

	go saveToDisk(snap)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	mu.Lock()
	if current != nil && current.Rooms != nil {
		delete(current.Rooms, id)
		current.SavedAt = time.Now().UnixMilli()
	}
	snap := current
	mu.Unlock()

	if snap != nil {
		go saveToDisk(snap)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func saveToDisk(snap *snapshot.Snapshot) {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		log.Printf("failed to marshal snapshot: %v", err)
		return
	}
	path := fmt.Sprintf("%s/latest.json", persistDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		log.Printf("failed to write snapshot: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Printf("failed to rename snapshot: %v", err)
		return
	}
	log.Printf("snapshot saved to %s (%d rooms)", path, len(snap.Rooms))
}

func loadFromDisk() {
	path := fmt.Sprintf("%s/latest.json", persistDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("failed to read snapshot: %v", err)
		}
		return
	}
	var snap snapshot.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		log.Printf("failed to parse snapshot: %v", err)
		return
	}
	current = &snap
	log.Printf("loaded snapshot from %s (%d rooms)", path, len(snap.Rooms))
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
