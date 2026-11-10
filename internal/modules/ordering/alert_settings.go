package ordering

import (
	"context"
	"regexp"

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

var alertColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (v AlertSettings) validate() error {
	if v.WarningMinutes <= 0 || v.CriticalMinutes <= v.WarningMinutes || v.CriticalMinutes > 1440 || v.PreorderLeadMinutes < 0 || v.PreorderLeadMinutes > 1440 || !alertColor.MatchString(v.WarningColor) || !alertColor.MatchString(v.CriticalColor) {
		return appfault.ErrInvalidInput
	}
	return nil
}

type alertRepository interface {
	GetAlertSettings(context.Context, string) (AlertSettings, error)
	SaveAlertSettings(context.Context, string, string, AlertSettings) (AlertSettings, error)
}

func (s *Service) GetAlertSettings(ctx context.Context, user, venue string) (AlertSettings, error) {
	if !uuidPattern.MatchString(venue) {
		return AlertSettings{}, appfault.ErrInvalidInput
	}
	role, err := s.authorizer.OrderRole(ctx, user, venue)
	if err != nil {
		return AlertSettings{}, err
	}
	v, err := s.repository.GetAlertSettings(ctx, venue)
	v.CanEdit = role == "owner"
	return v, err
}
func (s *Service) SaveAlertSettings(ctx context.Context, user, venue string, v AlertSettings) (AlertSettings, error) {
	if !uuidPattern.MatchString(venue) {
		return AlertSettings{}, appfault.ErrInvalidInput
	}
	role, err := s.authorizer.OrderRole(ctx, user, venue)
	if err != nil {
		return AlertSettings{}, err
	}
	if role != "owner" {
		return AlertSettings{}, appfault.ErrForbidden
	}
	if err = v.validate(); err != nil {
		return AlertSettings{}, err
	}
	return s.repository.SaveAlertSettings(ctx, user, venue, v)
}
