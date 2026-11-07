// Command server starts the TerraFlow API server.
// Phase 1: serves graph JSON from local samples. No auth, no DB.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/Anmewzaa/TerraFlow/server/internal/api"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/graph", api.GraphHandler(logger))

	// CORS for local dev (Vite on :5173)
	handler := corsMiddleware(mux)

	logger.Info("terraflow server", "addr", *addr)
	fmt.Fprintf(os.Stderr, "listening on %s\n", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
