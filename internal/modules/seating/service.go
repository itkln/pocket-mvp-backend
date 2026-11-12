package seating

import (
	"context"
	"strings"
	"unicode/utf8"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/validate"
)

type VenueAuthorizer interface {
	Require(context.Context, string, string, access.Capability) error
}

type Repository interface {
	Floor(ctx context.Context, venueID string) (Floor, error)
	SeatReservation(ctx context.Context, venueID, reservationID, tableIdentifier string) error
	SeatWalkIn(ctx context.Context, venueID string, input WalkIn) error
	Release(ctx context.Context, venueID, tableIdentifier string) error
}

type Service struct {
	repository Repository
	authorizer VenueAuthorizer
}

func NewService(repository Repository, authorizer VenueAuthorizer) *Service {
	return &Service{repository: repository, authorizer: authorizer}
}

// Floor reads the live floor. Like the plan itself, it belongs to those who run the floor.
func (s *Service) Floor(ctx context.Context, userID, venueID string) (Floor, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.SeatGuests); err != nil {
		return Floor{}, err
	}
	return s.repository.Floor(ctx, venueID)
}

// SeatReservation seats a confirmed booking — at its own table, or at tableIdentifier when the
// staff choose another one. Seating an already seated booking again changes nothing.
func (s *Service) SeatReservation(ctx context.Context, userID, venueID, reservationID, tableIdentifier string) (Floor, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.SeatGuests); err != nil {
		return Floor{}, err
	}
	tableIdentifier = strings.TrimSpace(tableIdentifier)
	if !validate.UUID(reservationID) || !validIdentifier(tableIdentifier, true) {
		return Floor{}, appfault.ErrInvalidInput
	}
	if err := s.repository.SeatReservation(ctx, venueID, reservationID, tableIdentifier); err != nil {
		return Floor{}, err
	}
	return s.repository.Floor(ctx, venueID)
}

// SeatWalkIn seats a party without a booking at a free table.
func (s *Service) SeatWalkIn(ctx context.Context, userID, venueID string, input WalkIn) (Floor, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.SeatGuests); err != nil {
		return Floor{}, err
	}
	input.TableIdentifier = strings.TrimSpace(input.TableIdentifier)
	input.GuestName = strings.TrimSpace(input.GuestName)
	if !validIdentifier(input.TableIdentifier, false) || input.Guests < 1 || input.Guests > 20 ||
		utf8.RuneCountInString(input.GuestName) > 120 {
		return Floor{}, appfault.ErrInvalidInput
	}
	if err := s.repository.SeatWalkIn(ctx, venueID, input); err != nil {
		return Floor{}, err
	}
	return s.repository.Floor(ctx, venueID)
}

// Release frees a table when its party leaves. An open bill stays with the table until it is paid.
func (s *Service) Release(ctx context.Context, userID, venueID, tableIdentifier string) (Floor, error) {
	if err := s.authorizer.Require(ctx, userID, venueID, access.SeatGuests); err != nil {
		return Floor{}, err
	}
	tableIdentifier = strings.TrimSpace(tableIdentifier)
	if !validIdentifier(tableIdentifier, false) {
		return Floor{}, appfault.ErrInvalidInput
	}
	if err := s.repository.Release(ctx, venueID, tableIdentifier); err != nil {
		return Floor{}, err
	}
	return s.repository.Floor(ctx, venueID)
}

func validIdentifier(identifier string, optional bool) bool {
	if identifier == "" {
		return optional
	}
	return utf8.RuneCountInString(identifier) <= 40
}
