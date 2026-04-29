// ABOUTME: Webhook handler for 8x8 JaaS events. Downloads recordings,
// ABOUTME: transcriptions, and chat logs, then uploads them to Nextcloud via WebDAV.

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

// WebDAVConfig holds Nextcloud WebDAV credentials and destination.
type WebDAVConfig struct {
	URL      string
	Path     string
	User     string
	Password string
}

// webhookPayload is the common structure for all 8x8 webhook events.
type webhookPayload struct {
	EventType      string          `json:"eventType"`
	IdempotencyKey string          `json:"idempotencyKey"`
	AppID          string          `json:"appId"`
	SessionID      string          `json:"sessionId"`
	Timestamp      int64           `json:"timestamp"`
	FQN            string          `json:"fqn"`
	Data           json.RawMessage `json:"data"`
}

// downloadEventData holds fields from events that carry a preAuthenticatedLink.
type downloadEventData struct {
	PreAuthenticatedLink string `json:"preAuthenticatedLink"`
	DurationSec          int    `json:"durationSec"`
	StartTimestamp       int64  `json:"startTimestamp"`
	EndTimestamp         int64  `json:"endTimestamp"`
	RecordingSessionID   string `json:"recordingSessionId"`
}

// deduplicator tracks seen idempotency keys to prevent reprocessing.
type deduplicator struct {
	mu      sync.Mutex
	seen    map[string]time.Time
	maxSize int
}

func newDeduplicator(maxSize int) *deduplicator {
	return &deduplicator{
		seen:    make(map[string]time.Time),
		maxSize: maxSize,
	}
}

// isDuplicate returns true if the key has been seen before. If not, it records the key.
func (d *deduplicator) isDuplicate(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.seen[key]; exists {
		return true
	}

	// Evict oldest entries if at capacity.
	if len(d.seen) >= d.maxSize {
		var oldestKey string
		var oldestTime time.Time
		for k, t := range d.seen {
			if oldestKey == "" || t.Before(oldestTime) {
				oldestKey = k
				oldestTime = t
			}
		}
		delete(d.seen, oldestKey)
	}

	d.seen[key] = time.Now()
	return false
}

// downloadEventTypes are the event types that carry a preAuthenticatedLink.
var downloadEventTypes = map[string]string{
	"RECORDING_UPLOADED":     "recording",
	"TRANSCRIPTION_UPLOADED": "transcript",
	"CHAT_UPLOADED":          "chat",
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate.
	if s.cfg.WebhookToken != "" {
		auth := r.Header.Get("Authorization")
		if auth != s.cfg.WebhookToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Parse payload.
	var payload webhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Log every authenticated event.
	room := extractRoom(payload.FQN)
	s.logger.Info("webhook event",
		"event_type", payload.EventType,
		"room", room,
		"session_id", payload.SessionID,
		"timestamp", payload.Timestamp,
		"idempotency_key", payload.IdempotencyKey,
	)

	// Check if this is a download event.
	fileType, isDownloadEvent := downloadEventTypes[payload.EventType]
	if !isDownloadEvent {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check WebDAV is configured.
	if s.cfg.WebDAV == nil {
		http.Error(w, "recording storage not configured", http.StatusServiceUnavailable)
		return
	}

	// Deduplicate.
	if s.dedup.isDuplicate(payload.IdempotencyKey) {
		s.logger.Info("webhook duplicate, skipping", "idempotency_key", payload.IdempotencyKey)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Parse download-specific data.
	var data downloadEventData
	if err := json.Unmarshal(payload.Data, &data); err != nil {
		s.logger.Error("webhook: failed to parse event data", "error", err)
		http.Error(w, "invalid event data", http.StatusBadRequest)
		return
	}

	// Respond immediately — process asynchronously.
	w.WriteHeader(http.StatusOK)

	go s.processDownload(room, fileType, data)
}

func (s *Server) processDownload(room, fileType string, data downloadEventData) {
	filename := buildFilename(room, fileType, data)
	logger := s.logger.With("room", room, "file_type", fileType, "filename", filename)

	logger.Info("downloading file", "url_length", len(data.PreAuthenticatedLink))

	// Download to temp file.
	tmpFile, err := s.downloadToTemp(data.PreAuthenticatedLink)
	if err != nil {
		logger.Error("download failed", "error", err)
		return
	}
	defer os.Remove(tmpFile)

	// Upload to WebDAV.
	if err := s.uploadToWebDAV(tmpFile, filename); err != nil {
		logger.Error("WebDAV upload failed", "error", err)
		return
	}

	logger.Info("file uploaded to Nextcloud", "destination", path.Join(s.cfg.WebDAV.Path, filename))
}

func buildFilename(room, fileType string, data downloadEventData) string {
	t := time.UnixMilli(data.StartTimestamp).UTC()
	datePart := t.Format("2006-01-02")
	timePart := t.Format("1504")

	switch fileType {
	case "recording":
		return fmt.Sprintf("%s_%s_%s_%ds.mp4", room, datePart, timePart, data.DurationSec)
	case "transcript":
		ext := "json"
		if data.PreAuthenticatedLink != "" {
			ext = extensionFromURL(data.PreAuthenticatedLink, "json")
		}
		return fmt.Sprintf("%s_%s_%s_transcript.%s", room, datePart, timePart, ext)
	case "chat":
		ext := "json"
		if data.PreAuthenticatedLink != "" {
			ext = extensionFromURL(data.PreAuthenticatedLink, "json")
		}
		return fmt.Sprintf("%s_%s_%s_chat.%s", room, datePart, timePart, ext)
	default:
		return fmt.Sprintf("%s_%s_%s_%s", room, datePart, timePart, fileType)
	}
}

// extensionFromURL extracts the file extension from a URL path, or returns the fallback.
func extensionFromURL(rawURL, fallback string) string {
	// Find the last path segment, strip query params.
	idx := strings.LastIndex(rawURL, "/")
	if idx < 0 {
		return fallback
	}
	segment := rawURL[idx+1:]
	if q := strings.Index(segment, "?"); q >= 0 {
		segment = segment[:q]
	}
	if dot := strings.LastIndex(segment, "."); dot >= 0 {
		ext := segment[dot+1:]
		if ext != "" {
			return ext
		}
	}
	return fallback
}

func extractRoom(fqn string) string {
	parts := strings.SplitN(fqn, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return fqn
}

func (s *Server) downloadToTemp(url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET returned %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "meet-recording-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("download copy: %w", err)
	}

	tmp.Close()
	return tmp.Name(), nil
}

func (s *Server) uploadToWebDAV(localPath, filename string) error {
	cfg := s.cfg.WebDAV
	destDir := strings.TrimRight(cfg.URL, "/") + "/" + strings.TrimLeft(cfg.Path, "/")
	destFile := destDir + "/" + filename

	client := &http.Client{Timeout: 10 * time.Minute}

	// Ensure directory exists (MKCOL). Ignore errors — the directory may already exist.
	mkcolReq, err := http.NewRequest("MKCOL", destDir, nil)
	if err != nil {
		return fmt.Errorf("MKCOL request: %w", err)
	}
	mkcolReq.SetBasicAuth(cfg.User, cfg.Password)
	mkcolResp, err := client.Do(mkcolReq)
	if err != nil {
		s.logger.Warn("WebDAV MKCOL failed (directory may already exist)", "error", err)
	} else {
		mkcolResp.Body.Close()
	}

	// Upload the file.
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file: %w", err)
	}
	defer f.Close()

	putReq, err := http.NewRequest("PUT", destFile, f)
	if err != nil {
		return fmt.Errorf("PUT request: %w", err)
	}
	putReq.SetBasicAuth(cfg.User, cfg.Password)

	putResp, err := client.Do(putReq)
	if err != nil {
		return fmt.Errorf("PUT failed: %w", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode < 200 || putResp.StatusCode >= 300 {
		body, _ := io.ReadAll(putResp.Body)
		return fmt.Errorf("PUT returned %d: %s", putResp.StatusCode, string(body))
	}

	return nil
}
