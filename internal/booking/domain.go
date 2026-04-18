package booking

import (
	"context"
	"errors"
	"time"
)

var (
	ErrSeatAlreadyBooked = errors.New("seat already booked")
	ErrSeatAlreadyHeld   = errors.New("seat already held")
	ErrSessionNotFound   = errors.New("session not found")
	ErrSessionExpired    = errors.New("session expired")
	ErrForbidden         = errors.New("forbidden")
	ErrMovieNotFound     = errors.New("movie not found")
	ErrInvalidSeat       = errors.New("invalid seat")
)

const (
	StatusHeld      = "held"
	StatusConfirmed = "confirmed"
)

// Movie describes a screening with its seating layout.
type Movie struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Rows         int    `json:"rows"`
	SeatsPerRow  int    `json:"seats_per_row"`
}

// Session is a temporary hold on a seat pending confirmation.
type Session struct {
	ID        string    `json:"session_id"`
	MovieID   string    `json:"movie_id"`
	SeatID    string    `json:"seat_id"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Booking represents a seat reservation (held or confirmed).
type Booking struct {
	ID        string    `json:"id"`
	MovieID   string    `json:"movie_id"`
	SeatID    string    `json:"seat_id"`
	UserID    string    `json:"user_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SeatStatus is the public view of a seat for a given movie.
type SeatStatus struct {
	SeatID    string `json:"seat_id"`
	Booked    bool   `json:"booked"`
	Confirmed bool   `json:"confirmed"`
	UserID    string `json:"user_id,omitempty"`
}

type Store interface {
	ListMovies() []Movie
	GetMovie(movieID string) (Movie, bool)

	Hold(ctx context.Context, movieID, seatID, userID string, ttl time.Duration) (Session, error)
	Confirm(ctx context.Context, sessionID, userID string) (Booking, error)
	Release(ctx context.Context, sessionID, userID string) error

	SeatStatuses(movieID string) []SeatStatus
	SweepExpired(now time.Time) int
}
