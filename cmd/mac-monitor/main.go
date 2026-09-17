package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"mac-monitor/internal/alerts"
	"mac-monitor/internal/collector"
	"mac-monitor/internal/config"
	"mac-monitor/internal/notify"
	"mac-monitor/internal/server"
	"mac-monitor/internal/storage"
	"mac-monitor/internal/tray"
	"mac-monitor/internal/webui"
)

const (
	addr            = ":8080"
	collectInterval = 5 * time.Second
	pruneInterval   = time.Hour
)

func main() {
	// Pin the main goroutine to OS thread 0. Cocoa requires all UI operations
	// (NSStatusItem, NSMenu, NSApp) to run on the OS main thread.
	runtime.LockOSThread()

	dir := dataDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfgStore, err := config.Open(cfgPath)
	if err != nil {
		log.Printf("config: %v (using defaults)", err)
	}
	retention := time.Duration(cfgStore.Get().Retention)
	log.Printf("Config: %s (retention %v)", cfgPath, retention)

	db, err := storage.Open(filepath.Join(dir, "mac-monitor.db"))
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var engine *alerts.Engine
	notify.Init(func(process string) {
		if err := engine.Ignore(process); err != nil {
			log.Printf("ignore %s: %v", process, err)
		}
	}, cfgStore.Get().Alerts.Native)
	engine, err = alerts.New(db, cfgStore, sendNative)
	if err != nil {
		log.Fatalf("alerts: %v", err)
	}
	go engine.Run(ctx)

	hub := server.NewHub()
	t := tray.New(cancel, addr)

	go runCollector(ctx, db, hub, t, engine)
	go runPruner(ctx, db, retention)

	go func() {
		log.Printf("Listening on http://localhost%s", addr)
		srv := server.New(db, hub, webui.FS(), retention, engine)
		if err := srv.ListenAndServe(addr); err != nil {
			log.Printf("server stopped: %v", err)
			cancel()
		}
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

// dataDir holds the database and config; falls back to the working directory.
func dataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	dir := filepath.Join(home, "Library", "Application Support", "Mac Monitor")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "."
	}
	return dir
}

func sendNative(ctx context.Context, n storage.Notification) error {
	category := ""
	if n.Kind == alerts.KindCPU {
		category = "cpu" // adds the "Don't Alert for This App" action
	}
	return notify.Send(ctx, n.AlertKey, n.Title, n.Body, category, n.Subject)
}

func runCollector(ctx context.Context, db *storage.DB, hub *server.Hub, t *tray.Tray, engine *alerts.Engine) {
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
			engine.ObserveSnapshot(snap)
			hub.Broadcast(snap)
			t.SetCPU(snap.CPUPercent)
		}
	}
}

func runPruner(ctx context.Context, db *storage.DB, retention time.Duration) {
	ticker := time.NewTicker(pruneInterval)
	defer ticker.Stop()
	for {
		// Prune immediately on startup so a shortened retention applies right away.
		if err := db.Prune(retention); err != nil {
			log.Printf("prune: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
