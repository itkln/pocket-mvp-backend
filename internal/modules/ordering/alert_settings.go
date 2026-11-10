package ordering

import (
	"context"
	"github.com/jackc/pgx/v5"
	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"regexp"
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
func (r *PostgresRepository) GetAlertSettings(ctx context.Context, venue string) (AlertSettings, error) {
	var v AlertSettings
	err := r.db.QueryRow(ctx, `SELECT COALESCE(s.warning_minutes,10),COALESCE(s.critical_minutes,20),COALESCE(s.warning_color,'#F59E0B'),COALESCE(s.critical_color,'#EF4444'),COALESCE(s.preorder_lead_minutes,30) FROM venues v LEFT JOIN venue_order_alert_settings s ON s.venue_id=v.id WHERE v.id=$1 AND v.deleted_at IS NULL`, venue).Scan(&v.WarningMinutes, &v.CriticalMinutes, &v.WarningColor, &v.CriticalColor, &v.PreorderLeadMinutes)
	return v, err
}
func (r *PostgresRepository) SaveAlertSettings(ctx context.Context, user, venue string, v AlertSettings) (AlertSettings, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AlertSettings{}, err
	}
	defer tx.Rollback(ctx)
	role, err := access.ResolveOrderRole(ctx, tx, user, venue, true)
	if err != nil {
		return AlertSettings{}, err
	}
	if role != "owner" {
		return AlertSettings{}, appfault.ErrForbidden
	}
	if err = v.validate(); err != nil {
		return AlertSettings{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO venue_order_alert_settings(venue_id,warning_minutes,critical_minutes,warning_color,critical_color,preorder_lead_minutes) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(venue_id) DO UPDATE SET warning_minutes=EXCLUDED.warning_minutes,critical_minutes=EXCLUDED.critical_minutes,warning_color=EXCLUDED.warning_color,critical_color=EXCLUDED.critical_color,preorder_lead_minutes=EXCLUDED.preorder_lead_minutes`, venue, v.WarningMinutes, v.CriticalMinutes, v.WarningColor, v.CriticalColor, v.PreorderLeadMinutes)
	if err != nil {
		return AlertSettings{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AlertSettings{}, err
	}
	v.CanEdit = true
	return v, nil
}
