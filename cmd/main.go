package main

import (
	"context"
	"errors"
	"log"
	nethttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cinema-booking/internal/booking"
	apphttp "cinema-booking/internal/http"
)

func main() {
	addr := envOr("CINEMA_ADDR", ":8080")
	staticDir := envOr("CINEMA_STATIC_DIR", "./static")

	store := booking.NewMemoryStore()
	store.SeedMovies(defaultMovies())
	svc := booking.NewService(store)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go svc.RunSweeper(ctx, 30*time.Second)

	handlers := apphttp.NewHandlers(svc)
	srv := &nethttp.Server{
		Addr:              addr,
		Handler:           apphttp.NewRouter(handlers, staticDir),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("cinema-booking listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func defaultMovies() []booking.Movie {
	return []booking.Movie{
		{ID: "inception", Title: "Inception", Rows: 6, SeatsPerRow: 10},
		{ID: "arrival", Title: "Arrival", Rows: 5, SeatsPerRow: 8},
		{ID: "interstellar", Title: "Interstellar", Rows: 7, SeatsPerRow: 12},
	}
}
