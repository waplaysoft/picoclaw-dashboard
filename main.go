package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/waplay/picoclaw-dashboard/api"
	"github.com/waplay/picoclaw-dashboard/websocket"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
	// Setup WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Setup API routes
	api.SetupRoutes(hub)

	// Broadcast metrics every 5 seconds
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			health, err := api.GetHealth()
			if err != nil {
				log.Printf("⚠️  Error getting health: %v", err)
				continue
			}
			hub.Broadcast(health)
		}
	}()

	// Serve static files (embedded)
	http.Handle("/", http.FileServer(http.FS(staticFiles)))

	// Get Tailscale IP or use default
	port := "8080"
	addr := fmt.Sprintf(":%s", port)

	log.Printf("🚀 PicoClaw Dashboard starting on %s", addr)
	log.Printf("📊 Metrics: %s/api/health | 🔌 WebSocket: %s/ws", addr, addr)

	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Server error:", err)
	}
}
