// ABOUTME: HTTP server for meet. Serves the branded 8x8 JaaS page with
// ABOUTME: room name derived from the URL path, plus static assets.

package server

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

//go:embed all:static
var staticFS embed.FS

//go:embed static/index.html
var indexHTML string

// Config holds the server configuration.
type Config struct {
	Addr         string
	BaseURL      string
	AppID        string
	DefaultRoom  string
	DataDir      string
	WebDAV       *WebDAVConfig
	WebhookToken string
	Logger       *slog.Logger
}

// Server wraps net/http.Server with meet-specific routing.
type Server struct {
	http   *http.Server
	cfg    Config
	tmpl   *template.Template
	logger *slog.Logger
	dedup  *deduplicator
}

type pageData struct {
	AppID        string
	RoomName       string
	DomainFull     string
	DomainFirst    string
	DomainRest     string
}

// New creates a configured Server ready to listen.
func New(cfg Config) *Server {
	tmpl := template.Must(template.New("index").Parse(indexHTML))

	s := &Server{
		cfg:    cfg,
		tmpl:   tmpl,
		logger: cfg.Logger,
		dedup:  newDeduplicator(1000),
	}

	mux := http.NewServeMux()

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		cfg.Logger.Error("failed to create static sub-filesystem", "error", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/webhook/recording", s.handleWebhook)
	mux.HandleFunc("/", s.handleRoom)

	s.http = &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}

	return s
}

// Handler returns the server's HTTP handler for use in tests.
func (s *Server) Handler() http.Handler {
	return s.http.Handler
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	return s.http.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// StartPurgeTicker runs a daily ticker that removes files older than 30 days
// from the uploaded directory. Cancel the context to stop.
func (s *Server) StartPurgeTicker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		// Run once on startup.
		s.PurgeOldUploads(30 * 24 * time.Hour)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.PurgeOldUploads(30 * 24 * time.Hour)
			}
		}
	}()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) handleRoom(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	path = strings.TrimSuffix(path, "/")

	if path == "" {
		path = s.cfg.DefaultRoom
	}

	// Reject paths with slashes (only single-segment room names).
	if strings.Contains(path, "/") {
		http.Error(w, "Invalid room name", http.StatusBadRequest)
		return
	}

	domainFull, domainFirst, domainRest := parseDomain(s.cfg.BaseURL)

	data := pageData{
		AppID:     s.cfg.AppID,
		RoomName:    path,
		DomainFull:  domainFull,
		DomainFirst: domainFirst,
		DomainRest:  domainRest,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, data); err != nil {
		s.logger.Error("template render failed", "error", err)
	}
}

// parseDomain extracts the domain from a URL and splits it into the first
// label (bright) and the rest (dim). E.g. "https://meet.lobb.ie" yields
// ("meet.lobb.ie", "meet", ".lobb.ie").
func parseDomain(baseURL string) (full, first, rest string) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL, baseURL, ""
	}

	host := u.Hostname()
	full = host

	dot := strings.Index(host, ".")
	if dot < 0 {
		return host, host, ""
	}

	first = host[:dot]
	rest = host[dot:]
	return full, first, rest
}
