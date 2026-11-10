package ordering

import (
	"context"
	"pocket-mvp-backend/internal/validate"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
)

type AlertSettings struct {
	WarningMinutes      int    `json:"warning_minutes"`
	CriticalMinutes     int    `json:"critical_minutes"`
	WarningColor        string `json:"warning_color"`
	CriticalColor       string `json:"critical_color"`
	PreorderLeadMinutes int    `json:"preorder_lead_minutes"`
	CanEdit             bool   `json:"can_edit"`
}

func (v AlertSettings) validate() error {
	if v.WarningMinutes <= 0 || v.CriticalMinutes <= v.WarningMinutes || v.CriticalMinutes > 1440 || v.PreorderLeadMinutes < 0 || v.PreorderLeadMinutes > 1440 || !validate.HexColor(v.WarningColor) || !validate.HexColor(v.CriticalColor) {
		return appfault.ErrInvalidInput
	}
	return nil
}

type alertRepository interface {
	GetAlertSettings(context.Context, string) (AlertSettings, error)
	SaveAlertSettings(context.Context, string, string, AlertSettings) (AlertSettings, error)
}

func (s *Service) GetAlertSettings(ctx context.Context, user, venue string) (AlertSettings, error) {
	if !validate.UUID(venue) {
		return AlertSettings{}, appfault.ErrInvalidInput
	}
	role, err := s.authorizer.OrderRole(ctx, user, venue)
	if err != nil {
		return AlertSettings{}, err
	}
	v, err := s.repository.GetAlertSettings(ctx, venue)
	v.CanEdit = access.Allows(role, access.ManageVenue)
	return v, err
}
func (s *Service) SaveAlertSettings(ctx context.Context, user, venue string, v AlertSettings) (AlertSettings, error) {
	if !validate.UUID(venue) {
		return AlertSettings{}, appfault.ErrInvalidInput
	}
	role, err := s.authorizer.OrderRole(ctx, user, venue)
	if err != nil {
		return AlertSettings{}, err
	}
	if !access.Allows(role, access.ManageVenue) {
		return AlertSettings{}, appfault.ErrForbidden
	}
	if err = v.validate(); err != nil {
		return AlertSettings{}, err
	}
	return s.repository.SaveAlertSettings(ctx, user, venue, v)
}
