package ordering

import (
	"context"

	"github.com/jackc/pgx/v5"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
)

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
