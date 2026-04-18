package booking

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newTestService(t *testing.T) (*Service, *MemoryStore) {
	t.Helper()
	store := NewMemoryStore()
	store.SeedMovies([]Movie{
		{ID: "inception", Title: "Inception", Rows: 3, SeatsPerRow: 5},
	})
	svc := NewService(store, WithHoldTTL(100*time.Millisecond))
	return svc, store
}

func TestHoldSeat_Success(t *testing.T) {
	svc, _ := newTestService(t)
	sess, err := svc.HoldSeat(context.Background(), "inception", "A1", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.SeatID != "A1" || sess.UserID != "u1" {
		t.Fatalf("unexpected session: %+v", sess)
	}
	if sess.ExpiresAt.Before(time.Now()) {
		t.Fatalf("expiry should be in the future")
	}
}

func TestHoldSeat_UnknownMovie(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.HoldSeat(context.Background(), "ghost", "A1", "u1")
	if !errors.Is(err, ErrMovieNotFound) {
		t.Fatalf("want ErrMovieNotFound, got %v", err)
	}
}

func TestHoldSeat_InvalidSeat(t *testing.T) {
	svc, _ := newTestService(t)
	cases := []string{"", "A", "A0", "A6", "D1", "Z9", "1A"}
	for _, c := range cases {
		if _, err := svc.HoldSeat(context.Background(), "inception", c, "u1"); !errors.Is(err, ErrInvalidSeat) {
			t.Errorf("seat %q: want ErrInvalidSeat, got %v", c, err)
		}
	}
}

func TestHoldSeat_ConflictWhileActive(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.HoldSeat(context.Background(), "inception", "A1", "u1"); err != nil {
		t.Fatal(err)
	}
	_, err := svc.HoldSeat(context.Background(), "inception", "A1", "u2")
	if !errors.Is(err, ErrSeatAlreadyHeld) {
		t.Fatalf("want ErrSeatAlreadyHeld, got %v", err)
	}
}

func TestHoldSeat_ReclaimAfterExpiry(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.HoldSeat(context.Background(), "inception", "A1", "u1"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	if _, err := svc.HoldSeat(context.Background(), "inception", "A1", "u2"); err != nil {
		t.Fatalf("expected expired hold to be reclaimable, got %v", err)
	}
}

func TestConfirmAndRelease(t *testing.T) {
	svc, _ := newTestService(t)
	sess, err := svc.HoldSeat(context.Background(), "inception", "B2", "u1")
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.ReleaseSession(context.Background(), sess.ID, "u2"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other user should not release: %v", err)
	}

	book, err := svc.ConfirmBooking(context.Background(), sess.ID, "u1")
	if err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if book.Status != StatusConfirmed {
		t.Fatalf("want confirmed, got %s", book.Status)
	}

	if err := svc.ReleaseSession(context.Background(), sess.ID, "u1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("confirmed booking must not be releasable, got %v", err)
	}
}

func TestConfirm_ExpiredSession(t *testing.T) {
	svc, _ := newTestService(t)
	sess, err := svc.HoldSeat(context.Background(), "inception", "C3", "u1")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	if _, err := svc.ConfirmBooking(context.Background(), sess.ID, "u1"); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("want ErrSessionExpired, got %v", err)
	}
}

func TestConfirm_WrongUser(t *testing.T) {
	svc, _ := newTestService(t)
	sess, err := svc.HoldSeat(context.Background(), "inception", "A2", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmBooking(context.Background(), sess.ID, "u2"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestListSeats_ReflectsStateAndExcludesExpired(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.HoldSeat(context.Background(), "inception", "A1", "u1"); err != nil {
		t.Fatal(err)
	}
	sess, err := svc.HoldSeat(context.Background(), "inception", "A2", "u2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmBooking(context.Background(), sess.ID, "u2"); err != nil {
		t.Fatal(err)
	}

	seats, err := svc.ListSeats("inception")
	if err != nil {
		t.Fatal(err)
	}
	if len(seats) != 2 {
		t.Fatalf("want 2 seats reported, got %d", len(seats))
	}
	byID := map[string]SeatStatus{}
	for _, s := range seats {
		byID[s.SeatID] = s
	}
	if byID["A2"].Confirmed != true {
		t.Errorf("A2 should be confirmed")
	}
	if byID["A1"].Confirmed {
		t.Errorf("A1 should not be confirmed")
	}

	time.Sleep(150 * time.Millisecond)
	seats, _ = svc.ListSeats("inception")
	// A1 expired, A2 remains confirmed
	if len(seats) != 1 || seats[0].SeatID != "A2" {
		t.Fatalf("expired hold should not be reported, got %+v", seats)
	}
}

func TestSweepExpired(t *testing.T) {
	svc, store := newTestService(t)
	if _, err := svc.HoldSeat(context.Background(), "inception", "A1", "u1"); err != nil {
		t.Fatal(err)
	}
	if n := store.SweepExpired(time.Now()); n != 0 {
		t.Fatalf("unexpected sweep removals before expiry: %d", n)
	}
	if n := store.SweepExpired(time.Now().Add(time.Second)); n != 1 {
		t.Fatalf("want 1 removal after expiry, got %d", n)
	}
	// the seat is now reusable
	if _, err := svc.HoldSeat(context.Background(), "inception", "A1", "u2"); err != nil {
		t.Fatalf("expected seat to be free after sweep: %v", err)
	}
}
