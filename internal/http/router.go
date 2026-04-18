package http

import (
	"log"
	"net/http"
	"time"
)

// NewRouter wires all routes and serves static assets from staticDir at "/".
func NewRouter(h *Handlers, staticDir string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /movies", h.ListMovies)
	mux.HandleFunc("GET /movies/{movieID}/seats", h.ListSeats)
	mux.HandleFunc("POST /movies/{movieID}/seats/{seatID}/hold", h.HoldSeat)
	mux.HandleFunc("PUT /sessions/{sessionID}/confirm", h.ConfirmSession)
	mux.HandleFunc("DELETE /sessions/{sessionID}", h.ReleaseSession)

	if staticDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(staticDir)))
	}

	return withLogging(mux)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}
