package booking

import (
	"context"
	"errors"
)

var (
	ErrSeatAlreadyBooked = errors.New("seat already booked")
)

// Booking represents a confirmed seat reservation.
type Booking struct {
	ID      string
	MovieID string
	SeatID  string
	UserID  string
	Status  string
}

type BookingStore interface {
	Book(b Booking) (Booking, error)
	ListBookings(movieID string) []Booking

	Confirm(ctx context.Context, sessionID string, userID string) (Booking, error)
	Release(ctx context.Context, sessionID string, bookingID string) error
}
