package ordering

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"pocket-mvp-backend/internal/appfault"
)

var orderStatusPattern = regexp.MustCompile(`^(new|accepted|preparing|ready|served|completed|cancelled)$`)

type VenueAuthorizer interface {
	OrderRole(context.Context, string, string) (string, error)
}

type Service struct {
	repository Repository
	authorizer VenueAuthorizer
}

func NewService(repository Repository, authorizer VenueAuthorizer) *Service {
	return &Service{repository: repository, authorizer: authorizer}
}

func (s *Service) List(ctx context.Context, ownerID, venueID string) ([]Order, error) {
	role, err := s.authorizer.OrderRole(ctx, ownerID, venueID)
	if err != nil {
		return nil, err
	}
	orders, err := s.repository.List(ctx, venueID)
	for i := range orders {
		orders[i].kitchenOnly = role == "kitchen"
	}
	return orders, err
}

// Only dishes attached to confirmed reservations appear in a customer's preorder history.

func (s *Service) UpdateStatus(ctx context.Context, ownerID, venueID, orderID, status, expected string, details ...StatusChange) (Order, error) {
	_, err := s.authorizer.OrderRole(ctx, ownerID, venueID)
	if err != nil {
		return Order{}, err
	}
	if !orderStatusPattern.MatchString(status) || !orderStatusPattern.MatchString(expected) {
		return Order{}, appfault.ErrInvalidInput
	}
	change := StatusChange{ExpectedStatus: expected, ActorID: ownerID}
	if len(details) > 0 {
		change.Reason = details[0].Reason
		change.Comment = strings.TrimSpace(details[0].Comment)
	}
	if err := validateCancellation(status, change); err != nil {
		return Order{}, err
	}
	order, err := s.repository.UpdateStatus(ctx, venueID, orderID, status, change)
	return order, err
}

// AllowedActions is advisory; mutation authorization is repeated under transaction locks.
func (o *Order) applyRole(role string) {
	o.kitchenOnly = role == "kitchen"
	o.AllowedActions = []string{}
	if (role == "owner" || role == "manager") && !o.cancelAllowed && o.Status != "cancelled" && o.Status != "completed" {
		o.CancellationBlocked = "payment_or_closed_bill"
	}
	if o.Status == "draft" || o.Status == "completed" || o.Status == "cancelled" {
		return
	}
	for _, target := range []string{"accepted", "preparing", "ready", "served", "completed", "cancelled"} {
		if target == o.Status || (target == "cancelled" && !o.cancelAllowed) || (target != "cancelled" && o.ReservationStatus == "cancelled") {
			continue
		}
		if validateTransition(role, o.Channel, o.Status, target, o.Status) == nil {
			o.AllowedActions = append(o.AllowedActions, target)
		}
	}
}
func (s *Service) ListPage(ctx context.Context, userID, venueID string, query Query) (Page, error) {
	if err := query.normalize(venueID); err != nil {
		return Page{}, err
	}
	role, err := s.authorizer.OrderRole(ctx, userID, venueID)
	if err != nil {
		return Page{}, err
	}
	page, err := s.repository.ListPage(ctx, venueID, query)
	if err != nil {
		return Page{}, err
	}
	for i := range page.Orders {
		page.Orders[i].applyRole(role)
	}
	return page, nil
}
func (s *Service) Get(ctx context.Context, userID, venueID, orderID string) (Order, error) {
	if !uuidPattern.MatchString(venueID) || !uuidPattern.MatchString(orderID) {
		return Order{}, appfault.ErrInvalidInput
	}
	role, err := s.authorizer.OrderRole(ctx, userID, venueID)
	if err != nil {
		return Order{}, err
	}
	order, err := s.repository.Get(ctx, venueID, orderID)
	if err != nil {
		return Order{}, err
	}
	order.applyRole(role)
	return order, nil
}

func validateCancellation(status string, c StatusChange) error {
	if utf8.RuneCountInString(c.Comment) > 500 {
		return appfault.ErrInvalidInput
	}
	if status != "cancelled" {
		if c.Reason != "" || c.Comment != "" {
			return appfault.ErrInvalidInput
		}
		return nil
	}
	switch c.Reason {
	case "guest_request", "unavailable", "duplicate", "kitchen_issue", "other":
		return nil
	}
	return appfault.ErrInvalidInput
}
