package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/disk"
)

// Response structure for JSON response
type Response struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
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

	port := ":8085"
	fmt.Printf("Server is running on port %s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		fmt.Printf("Failed to start server: %s\n", err)
	}
}
