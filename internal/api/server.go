package api

import (
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"
	"uptime-mon/internal/config"
	"uptime-mon/internal/storage"
)

// Server coordinates HTTP routing for API and embedded web frontend
type Server struct {
	Config              *config.RuntimeConfig
	DB                  *storage.DB
	Broker              *SSEBroker
	IsMaintenanceActive func(string, string, time.Time) bool
	WebFS               fs.FS
	mux                 *http.ServeMux
	mu                  sync.RWMutex
}

// NewServer initializes the HTTP router
func NewServer(cfg *config.RuntimeConfig, db *storage.DB, broker *SSEBroker, webFiles fs.FS) *Server {
	s := &Server{
		Config: cfg,
		DB:     db,
		Broker: broker,
		mux:    http.NewServeMux(),
	}

	if webFiles != nil {
		if distFS, err := fs.Sub(webFiles, "dist"); err == nil {
			s.WebFS = distFS
		} else {
			s.WebFS = webFiles
		}
	}

	s.setupRoutes()
	return s
}

func (s *Server) UpdateConfig(cfg *config.RuntimeConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Config = cfg
}

func (s *Server) setupRoutes() {
	// Public API endpoints
	s.mux.HandleFunc("GET /api/v1/dashboard", s.HandleDashboard)
	s.mux.HandleFunc("GET /api/v1/events", s.Broker.HandleEvents)

	// Homepage Custom API endpoints (protected via auth middleware in User Story 5)
	s.mux.HandleFunc("GET /api/v1/status/summary", s.HandleHomepageSummary)
	s.mux.HandleFunc("GET /api/homepage", s.HandleHomepageSummary)

	// Web frontend static handler with SPA fallback
	s.mux.HandleFunc("/", s.handleStaticOrSPA)
}

func (s *Server) handleStaticOrSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	if s.WebFS == nil {
		// Frontend not compiled yet or in dev mode
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html class="dark"><body class="bg-slate-900 text-white p-8"><h1>uptime-mon</h1><p>Frontend assets not embedded. Run npm run build in web/.</p></body></html>`))
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// Try reading the file directly
	if f, err := s.WebFS.Open(path); err == nil {
		f.Close()
		http.FileServer(http.FS(s.WebFS)).ServeHTTP(w, r)
		return
	}

	// If file not found and doesn't contain a file extension, fallback to index.html for SPA
	if !strings.Contains(filepathBase(path), ".") {
		if indexFile, err := s.WebFS.Open("index.html"); err == nil {
			indexFile.Close()
			r.URL.Path = "/index.html"
			http.FileServer(http.FS(s.WebFS)).ServeHTTP(w, r)
			return
		}
	}

	http.NotFound(w, r)
}

func filepathBase(p string) string {
	parts := strings.Split(p, "/")
	return parts[len(parts)-1]
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
