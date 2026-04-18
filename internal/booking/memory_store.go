package booking

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore is a thread-safe in-memory implementation of Store.
type MemoryStore struct {
	mu       sync.RWMutex
	movies   map[string]Movie
	// bookings keyed by "movieID|seatID" so each seat has at most one active record.
	bookings map[string]*Booking
	// sessionIndex maps sessionID → booking key for fast lookup on confirm/release.
	sessionIndex map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		movies:       map[string]Movie{},
		bookings:     map[string]*Booking{},
		sessionIndex: map[string]string{},
	}
}

func (s *MemoryStore) SeedMovies(movies []Movie) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range movies {
		s.movies[m.ID] = m
	}
}

func (s *MemoryStore) ListMovies() []Movie {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Movie, 0, len(s.movies))
	for _, m := range s.movies {
		out = append(out, m)
	}
	return out
}

func (s *MemoryStore) GetMovie(movieID string) (Movie, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.movies[movieID]
	return m, ok
}

func seatKey(movieID, seatID string) string {
	return movieID + "|" + seatID
}

// Hold reserves a seat for a user with a TTL; fails if the seat is already held or confirmed.
func (s *MemoryStore) Hold(_ context.Context, movieID, seatID, userID string, ttl time.Duration) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.movies[movieID]; !ok {
		return Session{}, ErrMovieNotFound
	}

	key := seatKey(movieID, seatID)
	now := time.Now()
	if existing, ok := s.bookings[key]; ok {
		if existing.Status == StatusConfirmed {
			return Session{}, ErrSeatAlreadyBooked
		}
		if existing.Status == StatusHeld && existing.ExpiresAt.After(now) {
			return Session{}, ErrSeatAlreadyHeld
		}
		// expired hold — evict before taking a new one
		delete(s.sessionIndex, existing.ID)
		delete(s.bookings, key)
	}

	session := Session{
		ID:        uuid.NewString(),
		MovieID:   movieID,
		SeatID:    seatID,
		UserID:    userID,
		ExpiresAt: now.Add(ttl),
	}
	s.bookings[key] = &Booking{
		ID:        session.ID,
		MovieID:   movieID,
		SeatID:    seatID,
		UserID:    userID,
		Status:    StatusHeld,
		CreatedAt: now,
		ExpiresAt: session.ExpiresAt,
	}
	s.sessionIndex[session.ID] = key
	return session, nil
}

// Confirm upgrades an active hold owned by userID into a confirmed booking.
func (s *MemoryStore) Confirm(_ context.Context, sessionID, userID string) (Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, ok := s.sessionIndex[sessionID]
	if !ok {
		return Booking{}, ErrSessionNotFound
	}
	b, ok := s.bookings[key]
	if !ok {
		delete(s.sessionIndex, sessionID)
		return Booking{}, ErrSessionNotFound
	}
	if b.UserID != userID {
		return Booking{}, ErrForbidden
	}
	if b.Status == StatusConfirmed {
		return *b, nil
	}
	if time.Now().After(b.ExpiresAt) {
		delete(s.bookings, key)
		delete(s.sessionIndex, sessionID)
		return Booking{}, ErrSessionExpired
	}
	b.Status = StatusConfirmed
	b.ExpiresAt = time.Time{}
	return *b, nil
}

// Release deletes an active hold owned by userID. Confirmed bookings cannot be released.
func (s *MemoryStore) Release(_ context.Context, sessionID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, ok := s.sessionIndex[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	b, ok := s.bookings[key]
	if !ok {
		delete(s.sessionIndex, sessionID)
		return ErrSessionNotFound
	}
	if b.UserID != userID {
		return ErrForbidden
	}
	if b.Status == StatusConfirmed {
		return ErrForbidden
	}
	delete(s.bookings, key)
	delete(s.sessionIndex, sessionID)
	return nil
}

// SeatStatuses returns the booked/confirmed status of every recorded seat for a movie.
func (s *MemoryStore) SeatStatuses(movieID string) []SeatStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	out := make([]SeatStatus, 0)
	for _, b := range s.bookings {
		if b.MovieID != movieID {
			continue
		}
		if b.Status == StatusHeld && !b.ExpiresAt.After(now) {
			continue // expired; treat as free until sweep runs
		}
		out = append(out, SeatStatus{
			SeatID:    b.SeatID,
			Booked:    true,
			Confirmed: b.Status == StatusConfirmed,
			UserID:    b.UserID,
		})
	}
	return out
}

// SweepExpired deletes all held bookings whose ExpiresAt is before now. Returns the count removed.
func (s *MemoryStore) SweepExpired(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for key, b := range s.bookings {
		if b.Status == StatusHeld && !b.ExpiresAt.After(now) {
			delete(s.sessionIndex, b.ID)
			delete(s.bookings, key)
			removed++
		}
	}
	return removed
}
