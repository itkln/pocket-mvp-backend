package bootstrap

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Profiles the actual schema and verifies the reversible index migration on an
// isolated dataset. Execution times are diagnostic, not a flaky CI threshold.
func TestPreorderLookupIndexIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var ownerID, venueID, reservationID, layoutID, tableID string
	must(db.QueryRow(ctx, `INSERT INTO users(email,password_hash,first_name,last_name) VALUES('index@example.test','test','Index','Test') RETURNING id::text`).Scan(&ownerID))
	must(db.QueryRow(ctx, `INSERT INTO venues(owner_user_id,slug,name,address_line1,city,country_code) VALUES($1,'index-test','Index','Main','Bratislava','SK') RETURNING id::text`, ownerID).Scan(&venueID))
	must(db.QueryRow(ctx, `INSERT INTO venue_layouts(venue_id,name) VALUES($1,'Main') RETURNING id::text`, venueID).Scan(&layoutID))
	must(db.QueryRow(ctx, `INSERT INTO venue_tables(venue_id,layout_id,identifier,seats,qr_token_hash) VALUES($1,$2,'01',4,'index-qr') RETURNING id::text`, venueID, layoutID).Scan(&tableID))
	_, err := db.Exec(ctx, `INSERT INTO reservations(venue_id,venue_table_id,reservation_code,guest_name,guest_count,starts_at,ends_at)
 SELECT $1,$2,'INDEX-'||n,'Guest',2,now()+n*interval '2 hours',now()+n*interval '2 hours'+interval '1 hour'
 FROM generate_series(1,1000) n`, venueID, tableID)
	must(err)
	_, err = db.Exec(ctx, `INSERT INTO orders(venue_id,reservation_id,channel,total_minor,subtotal_minor,created_at)
 SELECT $1,r.id,'preorder',1000,1000,now()+n*interval '1 second'
 FROM reservations r CROSS JOIN generate_series(1,50) n WHERE r.venue_id=$1`, venueID)
	must(err)
	must(db.QueryRow(ctx, `SELECT id::text FROM reservations WHERE venue_id=$1 ORDER BY id LIMIT 1`, venueID).Scan(&reservationID))
	_, err = db.Exec(ctx, `ANALYZE orders`)
	must(err)
	const query = `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)
 SELECT total_minor,tip_minor,order_number,currency,status FROM orders
 WHERE reservation_id=$1 AND channel='preorder' ORDER BY created_at,id LIMIT 1`
	profile := func(label string) string {
		var raw []byte
		must(db.QueryRow(ctx, query, reservationID).Scan(&raw))
		var plans []struct {
			Execution float64 `json:"Execution Time"`
			Plan      struct {
				Hits  int `json:"Shared Hit Blocks"`
				Reads int `json:"Shared Read Blocks"`
			} `json:"Plan"`
		}
		must(json.Unmarshal(raw, &plans))
		t.Logf("%s: execution_ms=%.3f shared_blocks=%d", label, plans[0].Execution, plans[0].Plan.Hits+plans[0].Plan.Reads)
		return string(raw)
	}
	// Exercise both migration directions without touching any real data.
	down, err := os.ReadFile("../../migrations/000018_preorder_lookup_index.down.sql")
	must(err)
	_, err = db.Exec(ctx, string(down))
	must(err)
	before := profile("without index")
	if !strings.Contains(before, `"Seq Scan"`) {
		t.Fatalf("unexpected baseline plan: %s", before)
	}
	up, err := os.ReadFile("../../migrations/000018_preorder_lookup_index.up.sql")
	must(err)
	_, err = db.Exec(ctx, string(up))
	must(err)
	after := profile("with index")
	if !strings.Contains(after, "orders_reservation_preorder_idx") {
		t.Fatalf("lookup index unused: %s", after)
	}
}
