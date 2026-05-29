package main

import (
	"context"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"mac-monitor/internal/collector"
	"mac-monitor/internal/server"
	"mac-monitor/internal/storage"
	"mac-monitor/internal/tray"
)

const (
	addr            = ":8080"
	staticDir       = "web/dist"
	dbPath          = "mac-monitor.db"
	collectInterval = 5 * time.Second
	pruneInterval   = time.Hour
	retentionPeriod = 30 * 24 * time.Hour
)

func main() {
	// Pin the main goroutine to OS thread 0. Cocoa requires all UI operations
	// (NSStatusItem, NSMenu, NSApp) to run on the OS main thread.
	runtime.LockOSThread()

	db, err := storage.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := server.NewHub()
	t := tray.New(cancel, addr)

	go runCollector(ctx, db, hub, t)
	go runPruner(ctx, db)

	go func() {
		log.Printf("Listening on http://localhost%s", addr)
		srv := server.New(db, hub, staticDir)
		if err := srv.ListenAndServe(addr); err != nil {
			log.Printf("server stopped: %v", err)
			cancel()
		}
	}()

	go func() {
		log.Println("pprof listening on http://localhost:6060/debug/pprof/")
		http.ListenAndServe("localhost:6060", nil)
	}()

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		cancel()
	}()

	t.Run(ctx) // blocks on main goroutine until quit
	log.Println("Shutting down")
}

func runCollector(ctx context.Context, db *storage.DB, hub *server.Hub, t *tray.Tray) {
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
			t.SetCPU(snap.CPUPercent)
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
