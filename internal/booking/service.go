package booking

import (
	"context"
	"time"
)

const DefaultHoldTTL = 5 * time.Minute

// Service coordinates hold/confirm/release operations over a Store.
type Service struct {
	store Store
	ttl   time.Duration
	now   func() time.Time
}

type Option func(*Service)

func WithHoldTTL(d time.Duration) Option {
	return func(s *Service) { s.ttl = d }
}

func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

func NewService(store Store, opts ...Option) *Service {
	s := &Service{store: store, ttl: DefaultHoldTTL, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// HoldSeat validates the seat and creates a hold for the given user.
func (s *Service) HoldSeat(ctx context.Context, movieID, seatID, userID string) (Session, error) {
	movie, ok := s.store.GetMovie(movieID)
	if !ok {
		return Session{}, ErrMovieNotFound
	}
	if userID == "" {
		return Session{}, ErrForbidden
	}
	if !isValidSeat(seatID, movie) {
		return Session{}, ErrInvalidSeat
	}
	return s.store.Hold(ctx, movieID, seatID, userID, s.ttl)
}

func (s *Service) ConfirmBooking(ctx context.Context, sessionID, userID string) (Booking, error) {
	if userID == "" {
		return Booking{}, ErrForbidden
	}
	return s.store.Confirm(ctx, sessionID, userID)
}

func (s *Service) ReleaseSession(ctx context.Context, sessionID, userID string) error {
	if userID == "" {
		return ErrForbidden
	}
	return s.store.Release(ctx, sessionID, userID)
}

func (s *Service) ListMovies() []Movie {
	return s.store.ListMovies()
}

func (s *Service) ListSeats(movieID string) ([]SeatStatus, error) {
	if _, ok := s.store.GetMovie(movieID); !ok {
		return nil, ErrMovieNotFound
	}
	return s.store.SeatStatuses(movieID), nil
}

// RunSweeper periodically evicts expired holds until ctx is cancelled.
func (s *Service) RunSweeper(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			s.store.SweepExpired(now)
		}
	}
}

// isValidSeat expects seats in the form "<row letter><column>" — e.g. A1, C7.
func isValidSeat(seatID string, m Movie) bool {
	if len(seatID) < 2 {
		return false
	}
	row := seatID[0]
	if row < 'A' || row >= 'A'+byte(m.Rows) {
		return false
	}
	col, err := parsePositiveInt(seatID[1:])
	if err != nil || col < 1 || col > m.SeatsPerRow {
		return false
	}
	return true
}

func parsePositiveInt(s string) (int, error) {
	if s == "" {
		return 0, ErrInvalidSeat
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, ErrInvalidSeat
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

