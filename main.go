package main

import (
	"log"
	"net/http"
	"os"

	"github.com/coder/arr-tracker/api"
	"github.com/coder/arr-tracker/internal/campfire"
	"github.com/coder/arr-tracker/internal/db"
)

func main() {
	// --- Config from environment ---
	campfireAPIKey := mustEnv("CAMPFIRE_API_KEY")
	dbURL := mustEnv("DATABASE_URL") // Supabase connection string
	port := envOr("PORT", "8080")

	// --- Database ---
	database, err := db.New(dbURL)
	if err != nil {
		log.Fatalf("FATAL: could not connect to database: %v", err)
	}
	if err := database.Migrate(); err != nil {
		log.Fatalf("FATAL: migration failed: %v", err)
	}
	log.Println("Database connected and migrated")

	// --- Campfire client ---
	cf := campfire.New(campfireAPIKey)

	// --- API handler ---
	handler := api.New(database, cf)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// --- Background scheduler (24hr auto-sync) ---
	handler.StartScheduler()

	// --- Start server ---
	addr := ":" + port
	log.Printf("Server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("FATAL: server error: %v", err)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("FATAL: required environment variable %s is not set", key)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
