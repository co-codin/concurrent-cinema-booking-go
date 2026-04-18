package http

import (
	"context"
	"errors"
	"net/http"

	"cinema-booking/internal/booking"
	"cinema-booking/internal/utils"
)

// bookingService is the subset of booking.Service the handlers depend on.
type bookingService interface {
	ListMovies() []booking.Movie
	ListSeats(movieID string) ([]booking.SeatStatus, error)
	HoldSeat(ctx context.Context, movieID, seatID, userID string) (booking.Session, error)
	ConfirmBooking(ctx context.Context, sessionID, userID string) (booking.Booking, error)
	ReleaseSession(ctx context.Context, sessionID, userID string) error
}

type Handlers struct {
	svc bookingService
}

func NewHandlers(svc bookingService) *Handlers {
	return &Handlers{svc: svc}
}

type userBody struct {
	UserID string `json:"user_id"`
}

func (h *Handlers) ListMovies(w http.ResponseWriter, _ *http.Request) {
	utils.WriteJSON(w, http.StatusOK, h.svc.ListMovies())
}

func (h *Handlers) ListSeats(w http.ResponseWriter, r *http.Request) {
	movieID := r.PathValue("movieID")
	seats, err := h.svc.ListSeats(movieID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	utils.WriteJSON(w, http.StatusOK, seats)
}

func (h *Handlers) HoldSeat(w http.ResponseWriter, r *http.Request) {
	movieID := r.PathValue("movieID")
	seatID := r.PathValue("seatID")

	var body userBody
	if !utils.DecodeJSON(w, r, &body) {
		return
	}
	if body.UserID == "" {
		utils.WriteError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	session, err := h.svc.HoldSeat(r.Context(), movieID, seatID, body.UserID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	utils.WriteJSON(w, http.StatusCreated, session)
}

func (h *Handlers) ConfirmSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")

	var body userBody
	if !utils.DecodeJSON(w, r, &body) {
		return
	}
	if body.UserID == "" {
		utils.WriteError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	if _, err := h.svc.ConfirmBooking(r.Context(), sessionID, body.UserID); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) ReleaseSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")

	var body userBody
	if !utils.DecodeJSON(w, r, &body) {
		return
	}
	if body.UserID == "" {
		utils.WriteError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	if err := h.svc.ReleaseSession(r.Context(), sessionID, body.UserID); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, booking.ErrMovieNotFound), errors.Is(err, booking.ErrSessionNotFound):
		utils.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, booking.ErrSeatAlreadyBooked), errors.Is(err, booking.ErrSeatAlreadyHeld):
		utils.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, booking.ErrSessionExpired):
		utils.WriteError(w, http.StatusGone, err.Error())
	case errors.Is(err, booking.ErrForbidden):
		utils.WriteError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, booking.ErrInvalidSeat):
		utils.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		utils.WriteError(w, http.StatusInternalServerError, "internal error")
	}
}
