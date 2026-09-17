package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"mac-monitor/internal/storage"
)

const sendTimeout = 30 * time.Second

// Delays before each retry; a notification that still fails after the last
// one is marked failed.
var retryBackoff = []time.Duration{
	time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 3 * time.Hour,
}

// runDelivery sleeps until a notification is due or a new one is queued —
// with nothing pending it doesn't wake up at all.
func (e *Engine) runDelivery(ctx context.Context) {
	for {
		e.deliverDue(ctx)

		var timer <-chan time.Time
		if at, ok, err := e.store.NextAttemptAt(); err != nil {
			log.Printf("alerts: next attempt: %v", err)
			timer = time.After(time.Minute)
		} else if ok {
			timer = time.After(max(0, time.Until(time.Unix(at, 0))))
		}
		select {
		case <-ctx.Done():
			return
		case <-e.kick:
		case <-timer:
		}
	}
}

func (e *Engine) deliverDue(ctx context.Context) {
	due, err := e.store.DueNotifications(e.now().Unix())
	if err != nil {
		log.Printf("alerts: due notifications: %v", err)
		return
	}
	for _, n := range due {
		if ctx.Err() != nil {
			return
		}
		err := e.deliver(ctx, n)
		n.Attempts++
		now := e.now()
		switch {
		case err == nil:
			n.Status, n.SentAt, n.LastError = storage.StatusSent, now.Unix(), ""
		case n.Attempts > len(retryBackoff):
			n.Status, n.LastError = storage.StatusFailed, err.Error()
		default:
			n.NextAttemptAt, n.LastError = now.Add(retryBackoff[n.Attempts-1]).Unix(), err.Error()
		}
		if err != nil {
			log.Printf("alerts: deliver %s via %s (attempt %d): %v", n.AlertKey, n.Channel, n.Attempts, err)
		}
		if err := e.store.UpdateNotification(n); err != nil {
			log.Printf("alerts: update notification: %v", err)
		}
	}
}

func (e *Engine) deliver(ctx context.Context, n storage.Notification) error {
	send, ok := e.channels[n.Channel]
	if !ok {
		return fmt.Errorf("%s notifications are not available", n.Channel)
	}
	if !slices.Contains(enabledChannels(e.cfg.Get().Alerts), n.Channel) {
		return fmt.Errorf("%s notifications have been disabled in config", n.Channel)
	}
	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	return send(ctx, n)
}

func levelName(level int) string {
	switch level {
	case LevelCritical:
		return "critical"
	case LevelWarning:
		return "warning"
	}
	return "resolved"
}

// runCommand runs the configured shell command. Alert details go in the
// environment and on stdin, never into the command string, so process names
// can't inject shell syntax.
func (e *Engine) runCommand(ctx context.Context, n storage.Notification) error {
	command := e.cfg.Get().Alerts.Command
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Env = append(os.Environ(),
		"MM_ALERT_KEY="+n.AlertKey,
		"MM_ALERT_KIND="+n.Kind,
		"MM_ALERT_SUBJECT="+n.Subject,
		"MM_ALERT_LEVEL="+levelName(n.Level),
		"MM_ALERT_TITLE="+n.Title,
		"MM_ALERT_BODY="+n.Body,
		"MM_ALERT_ATTEMPT="+strconv.Itoa(n.Attempts+1),
	)
	payload, err := json.Marshal(map[string]any{
		"key": n.AlertKey, "kind": n.Kind, "subject": n.Subject,
		"level": levelName(n.Level), "title": n.Title, "body": n.Body,
		"created_at": n.CreatedAt,
	})
	if err != nil {
		return err
	}
	cmd.Stdin = bytes.NewReader(payload)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("command timed out after %v", sendTimeout)
		}
		msg := strings.TrimSpace(out.String())
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		if msg != "" {
			return fmt.Errorf("%v: %s", err, msg)
		}
		return err
	}
	return nil
}
