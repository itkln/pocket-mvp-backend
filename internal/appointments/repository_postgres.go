// Package appointments books individual services using the existing reservation
// lifecycle and exclusion constraint. A venue table acts as a staff/workstation
// resource; its restaurant-specific label is never exposed by the service UI.
package appointments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/availability"
	"pocket-mvp-backend/internal/modules/reservations"
)

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

const eligible = `v.status = 'active' AND v.deleted_at IS NULL AND v.settings->>'public_category' IN ('yoga-pilates','masterclasses','beauty')`

func (s *PostgresRepository) List(ctx context.Context, venueID string) ([]Item, error) {
	if !uuid.MatchString(venueID) {
		return nil, appfault.ErrInvalidInput
	}
	rows, err := s.db.Query(ctx, `SELECT s.category_name,COALESCE(s.resource_id::text,''),s.id::text,s.name,s.description,s.duration_minutes,s.price_minor,s.currency FROM bookable_services s JOIN venues v ON v.id=s.venue_id WHERE s.venue_id=$1 AND s.is_active AND `+eligible+` ORDER BY s.sort_order,s.id`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Item{}
	for rows.Next() {
		var item Item
		if err = rows.Scan(&item.Category, &item.ResourceID, &item.ID, &item.Name, &item.Description, &item.Duration, &item.Price, &item.Currency); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

type resourceRepository struct {
	*availability.PostgresRepository
	venue      availability.Venue
	resourceID string
}

func (r resourceRepository) GetVenue(context.Context, string) (availability.Venue, error) {
	return r.venue, nil
}

func (r resourceRepository) ListTables(ctx context.Context, id string, start, end time.Time) ([]availability.Table, error) {
	tables, err := r.PostgresRepository.ListTables(ctx, id, start, end)
	if err != nil || r.resourceID == "" {
		return tables, err
	}
	selected := []availability.Table{}
	for _, table := range tables {
		if table.ID == r.resourceID {
			selected = append(selected, table)
		}
	}
	return selected, nil
}

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func load(ctx context.Context, q queryer, venueID, serviceID string) (Item, availability.Venue, error) {
	var item Item
	var venue availability.Venue
	err := q.QueryRow(ctx, `SELECT s.category_name,COALESCE(s.resource_id::text,''),s.id::text,s.name,s.description,s.duration_minutes,s.price_minor,s.currency,v.timezone,COALESCE((v.settings->>'schedule_enabled')::boolean,false),COALESCE((v.settings->>'same_day_booking')::boolean,false) FROM bookable_services s JOIN venues v ON v.id=s.venue_id WHERE s.venue_id=$1 AND s.id=$2 AND s.is_active AND `+eligible, venueID, serviceID).Scan(&item.Category, &item.ResourceID, &item.ID, &item.Name, &item.Description, &item.Duration, &item.Price, &item.Currency, &venue.Timezone, &venue.ScheduleEnabled, &venue.SameDayBooking)
	if errors.Is(err, pgx.ErrNoRows) {
		err = appfault.ErrNotFound
	}
	venue.DurationMinutes = item.Duration
	return item, venue, err
}
func (s *PostgresRepository) slots(ctx context.Context, venueID, date string, venue availability.Venue, resourceID string) (availability.Result, error) {
	return availability.NewService(resourceRepository{availability.NewPostgresRepository(s.db), venue, resourceID}).Get(ctx, venueID, date, 1)
}
func (s *PostgresRepository) Availability(ctx context.Context, venueID, serviceID, date string) (availability.Result, error) {
	if !uuid.MatchString(venueID) || !uuid.MatchString(serviceID) {
		return availability.Result{}, appfault.ErrInvalidInput
	}
	item, venue, err := load(ctx, s.db, venueID, serviceID)
	if err != nil {
		return availability.Result{}, err
	}
	return s.slots(ctx, venueID, date, venue, item.ResourceID)
}
func (s *PostgresRepository) Create(ctx context.Context, venueID string, input Input) (reservations.Result, error) {

	// Include authenticated identity in idempotency matching, never trust a client ID.
	encoded, _ := json.Marshal(struct {
		Input
		Customer string
	}{input, input.CustomerID})
	sum := sha256.Sum256(encoded)
	hash := hex.EncodeToString(sum[:])
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return reservations.Result{}, err
	}
	defer tx.Rollback(ctx)
	// Serialize bookings for a resource pool so simultaneous clients can select the
	// next available workstation. The database exclusion constraint is the final guard.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "appointments:"+venueID); err != nil {
		return reservations.Result{}, err
	}
	var result reservations.Result
	var previousHash, status string
	err = tx.QueryRow(ctx, `SELECT r.id::text,r.reservation_code,r.starts_at,r.guest_count,d.currency,r.request_hash,r.status FROM reservations r JOIN reservation_services d ON d.reservation_id=r.id WHERE r.venue_id=$1 AND r.idempotency_key=$2`, venueID, input.Key).Scan(&result.ID, &result.Code, &result.StartsAt, &result.GuestCount, &result.Currency, &previousHash, &status)
	if err == nil {
		if previousHash != hash || status != "confirmed" {
			return result, appfault.ErrConflict
		}
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	item, venue, err := load(ctx, tx, venueID, input.ServiceID)
	if err != nil {
		return result, err
	}
	if item.Price != input.ExpectedPrice {
		return result, reservations.ErrPriceChanged
	}
	zone, err := time.LoadLocation(venue.Timezone)
	if err != nil {
		return result, err
	}
	available, err := s.slots(ctx, venueID, input.StartsAt.In(zone).Format("2006-01-02"), venue, item.ResourceID)
	if err != nil {
		return result, err
	}
	var chosen *availability.Slot
	for _, slot := range available.Slots {
		if slot.StartsAt.Equal(input.StartsAt) {
			copy := slot
			chosen = &copy
			break
		}
	}
	if chosen == nil || len(chosen.Tables) == 0 {
		return result, appfault.ErrConflict
	}
	// Snapshot details separately, with a readable service name in existing history.
	notes := item.Name
	if item.Category != "" {
		notes = item.Category + " · " + notes
	}
	if input.Notes != "" {
		notes += " · " + input.Notes
	}
	err = tx.QueryRow(ctx, `WITH new_id AS (SELECT gen_random_uuid() id) INSERT INTO reservations(id,venue_id,venue_table_id,customer_user_id,reservation_code,guest_name,guest_phone,guest_count,starts_at,ends_at,status,notes,idempotency_key,request_hash) SELECT id,$1,$2,NULLIF($3,'')::uuid,upper(substr(replace(id::text,'-',''),1,10)),$4,$5,1,$6,$7,'confirmed',$8,$9,$10 FROM new_id RETURNING id::text,reservation_code,starts_at,guest_count`, venueID, chosen.Tables[0].ID, input.CustomerID, input.Name, input.Phone, chosen.StartsAt, chosen.EndsAt, notes, input.Key, hash).Scan(&result.ID, &result.Code, &result.StartsAt, &result.GuestCount)
	if err != nil {
		return result, appfault.MapWriteError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO reservation_services(reservation_id,service_id,name,duration_minutes,price_minor,currency) VALUES($1,$2,$3,$4,$5,$6)`, result.ID, item.ID, item.Name, item.Duration, item.Price, item.Currency)
	if err != nil {
		return result, err
	}
	result.Currency = item.Currency
	return result, tx.Commit(ctx)
}
