package dinein

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

// Bill is the current table visit. No guest identities or private order notes
// are exposed to other phones holding the table QR capability.
type Bill struct {
	ID              string     `json:"id"`
	VenueID         string     `json:"venue_id"`
	TableID         string     `json:"table_id"`
	Status          string     `json:"status"`
	Currency        string     `json:"currency"`
	OpenedAt        time.Time  `json:"opened_at"`
	SubtotalMinor   int64      `json:"subtotal_minor"`
	TipMinor        int64      `json:"tip_minor"`
	TaxMinor        int64      `json:"tax_minor"`
	ServiceFeeMinor int64      `json:"service_fee_minor"`
	TotalMinor      int64      `json:"total_minor"`
	Items           []BillItem `json:"items"`
}
type BillItem struct {
	OrderID        string `json:"order_id"`
	OrderNumber    int64  `json:"order_number"`
	Name           string `json:"name"`
	Quantity       int    `json:"quantity"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
	TotalMinor     int64  `json:"total_minor"`
}

func (s *Service) CurrentBill(ctx context.Context, token string) (*Bill, error) {
	if !validToken(token) {
		return nil, ErrQRInvalid
	}
	return s.repository.CurrentBill(ctx, tokenHash(token))
}

func (r *PostgresRepository) CurrentBill(ctx context.Context, hash string) (*Bill, error) {
	// Resolve and bill rows share a snapshot, including concurrent QR rotation,
	// cancellation and settlement. Reading never creates or reopens a visit.
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	table, err := resolve(ctx, tx, hash, "")
	if err != nil {
		return nil, err
	}
	var b Bill
	var items []byte
	err = tx.QueryRow(ctx, `
 SELECT b.id::text,b.venue_id::text,b.venue_table_id::text,b.status,b.currency,b.opened_at,
 COALESCE((SELECT sum(oi.total_minor) FROM order_items oi JOIN orders o ON o.id=oi.order_id
 WHERE o.bill_id=b.id AND o.venue_id=b.venue_id AND o.venue_table_id=b.venue_table_id
 AND o.status NOT IN ('draft','cancelled') AND oi.status<>'cancelled'),0),
 COALESCE((SELECT sum(o.tip_minor) FROM orders o WHERE o.bill_id=b.id
 AND o.venue_id=b.venue_id AND o.venue_table_id=b.venue_table_id AND o.status NOT IN ('draft','cancelled')
 AND EXISTS (SELECT 1 FROM order_items oi WHERE oi.order_id=o.id AND oi.status<>'cancelled')),0),
 b.tax_minor,b.service_fee_minor,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('order_id',o.id,'order_number',o.order_number,
 'name',oi.item_name,'quantity',oi.quantity,'unit_price_minor',oi.unit_price_minor,'total_minor',oi.total_minor)
 ORDER BY o.created_at,o.id,oi.created_at,oi.id)
 FROM orders o JOIN order_items oi ON oi.order_id=o.id
 WHERE o.bill_id=b.id AND o.venue_id=b.venue_id AND o.venue_table_id=b.venue_table_id
 AND o.status NOT IN ('draft','cancelled') AND oi.status<>'cancelled'),'[]'::jsonb)
 FROM bills b WHERE b.venue_id=$1 AND b.venue_table_id=$2 AND b.status IN ('open','payment_pending')`, table.VenueID, table.ID).Scan(
		&b.ID, &b.VenueID, &b.TableID, &b.Status, &b.Currency, &b.OpenedAt, &b.SubtotalMinor, &b.TipMinor, &b.TaxMinor, &b.ServiceFeeMinor, &items)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(items, &b.Items); err != nil {
		return nil, err
	}
	b.TotalMinor = b.SubtotalMinor + b.TipMinor + b.TaxMinor + b.ServiceFeeMinor
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &b, nil
}
