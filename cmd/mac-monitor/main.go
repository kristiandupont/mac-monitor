package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mac-monitor/internal/collector"
	"mac-monitor/internal/server"
	"mac-monitor/internal/storage"
)

const (
	addr          = ":8080"
	staticDir     = "web/dist"
	dbPath        = "mac-monitor.db"
	collectInterval = 5 * time.Second
	pruneInterval   = time.Hour
	retentionPeriod = 30 * 24 * time.Hour
)

func main() {
	db, err := storage.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := server.NewHub()

	go runCollector(ctx, db, hub)
	go runPruner(ctx, db)

	srv := server.New(db, hub, staticDir)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Listening on http://localhost%s", addr)
		if err := srv.ListenAndServe(addr); err != nil {
			log.Printf("server stopped: %v", err)
			cancel()
		}
	}()

	<-quit
	log.Println("Shutting down")
}

func runCollector(ctx context.Context, db *storage.DB, hub *server.Hub) {
	ticker := time.NewTicker(collectInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap, err := collector.Collect()
			if err != nil {
				log.Printf("collect: %v", err)
				continue
			}
			if err := db.Insert(snap); err != nil {
				log.Printf("insert: %v", err)
			}
			hub.Broadcast(snap)
		}
	}
}

func runPruner(ctx context.Context, db *storage.DB) {
	ticker := time.NewTicker(pruneInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := db.Prune(retentionPeriod); err != nil {
				log.Printf("prune: %v", err)
			}
		}
	}
}
