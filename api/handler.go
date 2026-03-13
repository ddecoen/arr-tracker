package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/arr-tracker/internal/campfire"
	"github.com/coder/arr-tracker/internal/db"
	"github.com/coder/arr-tracker/internal/models"
)

// Handler holds dependencies for all API routes.
type Handler struct {
	db         *db.DB
	campfire   *campfire.Client
	syncMu     sync.Mutex // prevents concurrent syncs
	lastSync   *time.Time
}

// New creates the API handler and registers routes.
func New(database *db.DB, cf *campfire.Client) *Handler {
	return &Handler{
		db:       database,
		campfire: cf,
	}
}

// RegisterRoutes attaches all routes to the provided mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/summary",   h.withCORS(h.handleSummary))
	mux.HandleFunc("/api/contracts", h.withCORS(h.handleContracts))
	mux.HandleFunc("/api/sync",      h.withCORS(h.handleSync))
	mux.HandleFunc("/api/health",    h.withCORS(h.handleHealth))

	// Serve the React frontend for all other routes
	mux.Handle("/", http.FileServer(http.Dir("./web/dist")))
}

// handleSummary returns aggregated ARR metrics.
func (h *Handler) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	summary, err := h.db.GetSummary()
	if err != nil {
		jsonError(w, "failed to get summary", http.StatusInternalServerError)
		log.Printf("ERROR summary: %v", err)
		return
	}
	jsonOK(w, summary)
}

// handleContracts returns the full contract list, optionally filtered by ?status=ACTIVE|CHURNED|ALL
func (h *Handler) handleContracts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "ACTIVE"
	}
	contracts, err := h.db.ListContracts(status)
	if err != nil {
		jsonError(w, "failed to list contracts", http.StatusInternalServerError)
		log.Printf("ERROR contracts: %v", err)
		return
	}
	jsonOK(w, contracts)
}

// handleSync triggers a Campfire sync. Uses incremental mode by default;
// pass ?full=true to force a full re-sync.
func (h *Handler) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.syncMu.TryLock() {
		jsonError(w, "sync already in progress", http.StatusConflict)
		return
	}
	defer h.syncMu.Unlock()

	fullSync := r.URL.Query().Get("full") == "true"

	result, err := h.runSync(fullSync)
	if err != nil {
		h.db.LogSync(result, err.Error())
		jsonError(w, fmt.Sprintf("sync failed: %v", err), http.StatusInternalServerError)
		log.Printf("ERROR sync: %v", err)
		return
	}

	h.db.LogSync(result, "")
	jsonOK(w, result)
}

// handleHealth is a lightweight liveness check.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]string{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
}

// runSync fetches from Campfire and upserts into Postgres.
func (h *Handler) runSync(full bool) (models.SyncResult, error) {
	result := models.SyncResult{
		SyncedAt:    time.Now().UTC(),
		Incremental: !full,
	}

	var sinceTime *time.Time
	if !full {
		last, err := h.db.LastSyncTime()
		if err != nil {
			log.Printf("WARN could not get last sync time, falling back to full sync: %v", err)
		}
		sinceTime = last
	}

	raw, err := h.campfire.FetchAllContracts(sinceTime)
	if err != nil {
		return result, fmt.Errorf("fetching from campfire: %w", err)
	}
	result.Total = len(raw)

	normalized := make([]models.Contract, 0, len(raw))
	for _, c := range raw {
		contract, err := campfire.NormalizeContract(c)
		if err != nil {
			log.Printf("WARN skipping contract %d: %v", c.ID, err)
			continue
		}
		normalized = append(normalized, contract)
	}

	upserted, err := h.db.UpsertContracts(normalized)
	if err != nil {
		return result, fmt.Errorf("upserting contracts: %w", err)
	}
	result.Upserted = upserted

	return result, nil
}

// StartScheduler runs a sync every 24 hours in a background goroutine.
func (h *Handler) StartScheduler() {
	go func() {
		// Initial sync on startup
		log.Println("Scheduler: running initial sync...")
		result, err := h.runSync(false)
		if err != nil {
			h.db.LogSync(result, err.Error())
			log.Printf("Scheduler: initial sync failed: %v", err)
		} else {
			h.db.LogSync(result, "")
			log.Printf("Scheduler: initial sync complete — %d contracts upserted", result.Upserted)
		}

		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			log.Println("Scheduler: running 24hr incremental sync...")
			result, err := h.runSync(false)
			if err != nil {
				h.db.LogSync(result, err.Error())
				log.Printf("Scheduler: sync failed: %v", err)
			} else {
				h.db.LogSync(result, "")
				log.Printf("Scheduler: sync complete — %d/%d contracts upserted", result.Upserted, result.Total)
			}
		}
	}()
}

// withCORS wraps a handler with permissive CORS headers for local dev / Vercel.
func (h *Handler) withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
