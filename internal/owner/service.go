package owner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"pocket-mvp-backend/internal/security"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) ListVenues(ctx context.Context, userID string) ([]Venue, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, name, slug, COALESCE(description,''), COALESCE(cuisine_type,''),
		       COALESCE(phone,''), COALESCE(email,''), address_line1, city,
		       COALESCE(postal_code,''), country_code, timezone, currency, status, settings, created_at
		FROM venues WHERE owner_user_id=$1 AND deleted_at IS NULL ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	venues := []Venue{}
	for rows.Next() {
		venue, err := scanVenue(rows)
		if err != nil {
			return nil, err
		}
		venues = append(venues, venue)
	}
	return venues, rows.Err()
}

func (s *Service) CreateVenue(ctx context.Context, userID string, input VenueInput) (Venue, error) {
	input = normalizeVenueInput(input)
	if !validVenueInput(input) {
		return Venue{}, ErrInvalidInput
	}
	token, err := security.NewSessionToken()
	if err != nil {
		return Venue{}, err
	}
	slugBase := slugify(input.Name)
	if slugBase == "" {
		slugBase = "venue"
	}
	suffix := slugify(token)
	if len(suffix) > 7 {
		suffix = suffix[:7]
	}
	if suffix == "" {
		return Venue{}, errors.New("failed to generate venue slug")
	}
	slug := fmt.Sprintf("%s-%s", slugBase, suffix)
	settings, _ := json.Marshal(input.Settings)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Venue{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	row := tx.QueryRow(ctx, `
		INSERT INTO venues (owner_user_id,slug,name,description,cuisine_type,phone,email,address_line1,city,postal_code,country_code,timezone,currency,status,settings)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),$8,$9,NULLIF($10,''),$11,$12,$13,$14,$15)
		RETURNING id::text,name,slug,COALESCE(description,''),COALESCE(cuisine_type,''),COALESCE(phone,''),COALESCE(email,''),
		 address_line1,city,COALESCE(postal_code,''),country_code,timezone,currency,status,settings,created_at`,
		userID, slug, input.Name, input.Description, input.CuisineType, input.Phone, input.Email,
		input.Address, input.City, input.PostalCode, input.CountryCode, input.Timezone, input.Currency, input.Status, settings)
	venue, err := scanVenue(row)
	if err != nil {
		return Venue{}, mapWriteError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET account_role='venue_owner' WHERE id=$1`, userID); err != nil {
		return Venue{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Venue{}, err
	}
	return venue, nil
}

func (s *Service) UpdateVenue(ctx context.Context, userID, venueID string, input VenueInput) (Venue, error) {
	input = normalizeVenueInput(input)
	if !validVenueInput(input) {
		return Venue{}, ErrInvalidInput
	}
	settings, _ := json.Marshal(input.Settings)
	row := s.db.QueryRow(ctx, `
		UPDATE venues SET name=$3,description=NULLIF($4,''),cuisine_type=NULLIF($5,''),phone=NULLIF($6,''),
		 email=NULLIF($7,''),address_line1=$8,city=$9,postal_code=NULLIF($10,''),country_code=$11,
		 timezone=$12,currency=$13,status=$14,settings=$15
		WHERE id=$2 AND owner_user_id=$1 AND deleted_at IS NULL
		RETURNING id::text,name,slug,COALESCE(description,''),COALESCE(cuisine_type,''),COALESCE(phone,''),COALESCE(email,''),
		 address_line1,city,COALESCE(postal_code,''),country_code,timezone,currency,status,settings,created_at`,
		userID, venueID, input.Name, input.Description, input.CuisineType, input.Phone, input.Email,
		input.Address, input.City, input.PostalCode, input.CountryCode, input.Timezone, input.Currency, input.Status, settings)
	venue, err := scanVenue(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Venue{}, ErrNotFound
	}
	return venue, err
}

func (s *Service) DeleteVenue(ctx context.Context, userID, venueID string) error {
	result, err := s.db.Exec(ctx, `UPDATE venues SET deleted_at=now(),status='closed' WHERE id=$2 AND owner_user_id=$1 AND deleted_at IS NULL`, userID, venueID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) Dashboard(ctx context.Context, userID, venueID string) (Dashboard, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return Dashboard{}, err
	}
	var result Dashboard
	err := s.db.QueryRow(ctx, `
		SELECT
		 COALESCE((SELECT SUM(total_minor) FROM orders WHERE venue_id=$1 AND status<>'cancelled' AND created_at>=CURRENT_DATE),0),
		 COALESCE((SELECT COUNT(*) FROM orders WHERE venue_id=$1 AND created_at>=CURRENT_DATE),0),
		 COALESCE((SELECT AVG(total_minor)::bigint FROM orders WHERE venue_id=$1 AND status<>'cancelled' AND created_at>=CURRENT_DATE),0),
		 COALESCE((SELECT COUNT(*) FROM venue_tables WHERE venue_id=$1 AND status='occupied' AND deleted_at IS NULL),0),
		 COALESCE((SELECT COUNT(*) FROM venue_tables WHERE venue_id=$1 AND deleted_at IS NULL),0),
		 COALESCE((SELECT COUNT(*) FROM orders WHERE venue_id=$1 AND status='new'),0),
		 COALESCE((SELECT AVG(rating)::float8 FROM reviews WHERE venue_id=$1 AND status='published'),0)`, venueID).
		Scan(&result.RevenueMinor, &result.OrdersToday, &result.AverageOrderMinor, &result.ActiveTables, &result.TotalTables, &result.NewOrders, &result.AverageRating)
	return result, err
}

func (s *Service) ListCategories(ctx context.Context, userID, venueID string) ([]Category, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT c.id::text,c.name,c.sort_order,c.is_active,COUNT(i.id)
	 FROM menu_categories c LEFT JOIN menu_items i ON i.category_id=c.id AND i.deleted_at IS NULL
	 WHERE c.venue_id=$1 GROUP BY c.id ORDER BY c.sort_order,c.created_at`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Category{}
	for rows.Next() {
		var item Category
		if err := rows.Scan(&item.ID, &item.Name, &item.SortOrder, &item.IsActive, &item.ItemCount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Service) CreateCategory(ctx context.Context, userID, venueID string, input CategoryInput) (Category, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return Category{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 80 {
		return Category{}, ErrInvalidInput
	}
	active := true
	if input.IsActive != nil {
		active = *input.IsActive
	}
	var item Category
	err := s.db.QueryRow(ctx, `INSERT INTO menu_categories(venue_id,name,sort_order,is_active) VALUES($1,$2,$3,$4)
	 RETURNING id::text,name,sort_order,is_active,0`, venueID, input.Name, input.SortOrder, active).Scan(&item.ID, &item.Name, &item.SortOrder, &item.IsActive, &item.ItemCount)
	return item, mapWriteError(err)
}

func (s *Service) UpdateCategory(ctx context.Context, userID, venueID, categoryID string, input CategoryInput) (Category, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return Category{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return Category{}, ErrInvalidInput
	}
	active := true
	if input.IsActive != nil {
		active = *input.IsActive
	}
	var item Category
	err := s.db.QueryRow(ctx, `UPDATE menu_categories SET name=$3,sort_order=$4,is_active=$5 WHERE id=$2 AND venue_id=$1
	 RETURNING id::text,name,sort_order,is_active,(SELECT COUNT(*) FROM menu_items WHERE category_id=$2 AND deleted_at IS NULL)`,
		venueID, categoryID, input.Name, input.SortOrder, active).Scan(&item.ID, &item.Name, &item.SortOrder, &item.IsActive, &item.ItemCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	return item, mapWriteError(err)
}

func (s *Service) DeleteCategory(ctx context.Context, userID, venueID, categoryID string) error {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return err
	}
	result, err := s.db.Exec(ctx, `DELETE FROM menu_categories WHERE id=$2 AND venue_id=$1`, venueID, categoryID)
	if err != nil {
		return mapWriteError(err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListMenuItems(ctx context.Context, userID, venueID string) ([]MenuItem, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT i.id::text,i.category_id::text,c.name,i.name,COALESCE(i.description,''),i.price_minor,i.currency,
	 i.is_available,i.is_popular,i.sort_order,COALESCE((SELECT public_url FROM menu_item_images WHERE menu_item_id=i.id ORDER BY sort_order LIMIT 1),'')
	 FROM menu_items i JOIN menu_categories c ON c.id=i.category_id WHERE i.venue_id=$1 AND i.deleted_at IS NULL ORDER BY c.sort_order,i.sort_order,i.created_at`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []MenuItem{}
	for rows.Next() {
		var i MenuItem
		if err := rows.Scan(&i.ID, &i.CategoryID, &i.Category, &i.Name, &i.Description, &i.PriceMinor, &i.Currency, &i.IsAvailable, &i.IsPopular, &i.SortOrder, &i.ImageURL); err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, rows.Err()
}

func (s *Service) CreateMenuItem(ctx context.Context, userID, venueID string, input MenuItemInput) (MenuItem, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return MenuItem{}, err
	}
	input = normalizeMenuItem(input)
	if !validMenuItem(input) {
		return MenuItem{}, ErrInvalidInput
	}
	available := true
	if input.IsAvailable != nil {
		available = *input.IsAvailable
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return MenuItem{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var item MenuItem
	err = tx.QueryRow(ctx, `INSERT INTO menu_items(venue_id,category_id,name,description,price_minor,currency,is_available,is_popular,sort_order)
	 SELECT $1,c.id,$3,NULLIF($4,''),$5,$6,$7,$8,$9 FROM menu_categories c WHERE c.id=$2 AND c.venue_id=$1
	 RETURNING id::text,category_id::text,(SELECT name FROM menu_categories WHERE id=category_id),name,COALESCE(description,''),price_minor,currency,is_available,is_popular,sort_order,''`,
		venueID, input.CategoryID, input.Name, input.Description, input.PriceMinor, input.Currency, available, input.IsPopular, input.SortOrder).
		Scan(&item.ID, &item.CategoryID, &item.Category, &item.Name, &item.Description, &item.PriceMinor, &item.Currency, &item.IsAvailable, &item.IsPopular, &item.SortOrder, &item.ImageURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return MenuItem{}, ErrInvalidInput
	}
	if err != nil {
		return MenuItem{}, mapWriteError(err)
	}
	if input.ImageURL != "" {
		_, err = tx.Exec(ctx, `INSERT INTO menu_item_images(menu_item_id,storage_key,public_url,content_type,byte_size) VALUES($1,$2,$3,'image/external',1)`, item.ID, "external/"+item.ID, input.ImageURL)
		if err != nil {
			return MenuItem{}, err
		}
		item.ImageURL = input.ImageURL
	}
	if err = tx.Commit(ctx); err != nil {
		return MenuItem{}, err
	}
	return item, nil
}

func (s *Service) UpdateMenuItem(ctx context.Context, userID, venueID, itemID string, input MenuItemInput) (MenuItem, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return MenuItem{}, err
	}
	input = normalizeMenuItem(input)
	if !validMenuItem(input) {
		return MenuItem{}, ErrInvalidInput
	}
	available := true
	if input.IsAvailable != nil {
		available = *input.IsAvailable
	}
	var item MenuItem
	err := s.db.QueryRow(ctx, `UPDATE menu_items i SET category_id=$3,name=$4,description=NULLIF($5,''),price_minor=$6,currency=$7,is_available=$8,is_popular=$9,sort_order=$10
	 FROM menu_categories c WHERE i.id=$2 AND i.venue_id=$1 AND c.id=$3 AND c.venue_id=$1 AND i.deleted_at IS NULL
	 RETURNING i.id::text,i.category_id::text,c.name,i.name,COALESCE(i.description,''),i.price_minor,i.currency,i.is_available,i.is_popular,i.sort_order,
	 COALESCE((SELECT public_url FROM menu_item_images WHERE menu_item_id=i.id ORDER BY sort_order LIMIT 1),'')`,
		venueID, itemID, input.CategoryID, input.Name, input.Description, input.PriceMinor, input.Currency, available, input.IsPopular, input.SortOrder).
		Scan(&item.ID, &item.CategoryID, &item.Category, &item.Name, &item.Description, &item.PriceMinor, &item.Currency, &item.IsAvailable, &item.IsPopular, &item.SortOrder, &item.ImageURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return MenuItem{}, ErrNotFound
	}
	return item, mapWriteError(err)
}

func (s *Service) DeleteMenuItem(ctx context.Context, userID, venueID, itemID string) error {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return err
	}
	r, err := s.db.Exec(ctx, `UPDATE menu_items SET deleted_at=now(),is_available=false WHERE id=$2 AND venue_id=$1 AND deleted_at IS NULL`, venueID, itemID)
	if err != nil {
		return err
	}
	if r.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListStaff(ctx context.Context, userID, venueID string) ([]StaffMember, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT id::text,COALESCE(display_name,''),invited_email,role,status,invited_at,accepted_at FROM venue_staff WHERE venue_id=$1 AND status<>'removed' ORDER BY created_at`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []StaffMember{}
	for rows.Next() {
		var i StaffMember
		if err := rows.Scan(&i.ID, &i.DisplayName, &i.Email, &i.Role, &i.Status, &i.InvitedAt, &i.AcceptedAt); err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, rows.Err()
}

func (s *Service) CreateStaff(ctx context.Context, userID, venueID string, input StaffInput) (StaffMember, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return StaffMember{}, err
	}
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.DisplayName == "" || !strings.Contains(input.Email, "@") || !validStaffRole(input.Role) {
		return StaffMember{}, ErrInvalidInput
	}
	var i StaffMember
	err := s.db.QueryRow(ctx, `INSERT INTO venue_staff(venue_id,display_name,invited_email,role,status,invited_by_user_id) VALUES($1,$2,$3,$4,'invited',$5) RETURNING id::text,display_name,invited_email,role,status,invited_at,accepted_at`, venueID, input.DisplayName, input.Email, input.Role, userID).Scan(&i.ID, &i.DisplayName, &i.Email, &i.Role, &i.Status, &i.InvitedAt, &i.AcceptedAt)
	return i, mapWriteError(err)
}

func (s *Service) UpdateStaff(ctx context.Context, userID, venueID, staffID string, input StaffInput) (StaffMember, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return StaffMember{}, err
	}
	if !validStaffRole(input.Role) {
		return StaffMember{}, ErrInvalidInput
	}
	status := input.Status
	if status == "" {
		status = "invited"
	}
	if !regexp.MustCompile(`^(invited|active|inactive)$`).MatchString(status) {
		return StaffMember{}, ErrInvalidInput
	}
	var i StaffMember
	err := s.db.QueryRow(ctx, `UPDATE venue_staff SET role=$3,status=$4 WHERE id=$2 AND venue_id=$1 AND status<>'removed' RETURNING id::text,COALESCE(display_name,''),invited_email,role,status,invited_at,accepted_at`, venueID, staffID, input.Role, status).Scan(&i.ID, &i.DisplayName, &i.Email, &i.Role, &i.Status, &i.InvitedAt, &i.AcceptedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return StaffMember{}, ErrNotFound
	}
	return i, mapWriteError(err)
}

func (s *Service) DeleteStaff(ctx context.Context, userID, venueID, staffID string) error {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return err
	}
	r, err := s.db.Exec(ctx, `UPDATE venue_staff SET status='removed',removed_at=now() WHERE id=$2 AND venue_id=$1 AND status<>'removed'`, venueID, staffID)
	if err != nil {
		return err
	}
	if r.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListOrders(ctx context.Context, userID, venueID string) ([]Order, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT o.id::text,o.order_number,o.channel,CASE WHEN t.identifier IS NOT NULL THEN 'Стол '||t.identifier WHEN o.channel='pickup' THEN 'Самовывоз' ELSE 'Онлайн' END,COALESCE(o.guest_name,''),o.total_minor,o.currency,o.status,COALESCE(o.notes,''),o.created_at FROM orders o LEFT JOIN venue_tables t ON t.id=o.venue_table_id WHERE o.venue_id=$1 ORDER BY o.created_at DESC LIMIT 200`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Order{}
	for rows.Next() {
		var i Order
		if err := rows.Scan(&i.ID, &i.Number, &i.Channel, &i.Source, &i.GuestName, &i.TotalMinor, &i.Currency, &i.Status, &i.Notes, &i.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, rows.Err()
}

func (s *Service) UpdateOrderStatus(ctx context.Context, userID, venueID, orderID, status string) (Order, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return Order{}, err
	}
	if !regexp.MustCompile(`^(new|accepted|preparing|ready|served|completed|cancelled)$`).MatchString(status) {
		return Order{}, ErrInvalidInput
	}
	var i Order
	err := s.db.QueryRow(ctx, `UPDATE orders o SET status=$3,completed_at=CASE WHEN $3 IN ('completed','cancelled') THEN now() ELSE completed_at END
	 WHERE o.id=$2 AND o.venue_id=$1
	 RETURNING o.id::text,o.order_number,o.channel,
	 CASE WHEN o.venue_table_id IS NOT NULL THEN 'Стол '||COALESCE((SELECT identifier FROM venue_tables WHERE id=o.venue_table_id),'')
	      WHEN o.channel='pickup' THEN 'Самовывоз' ELSE 'Онлайн' END,
	 COALESCE(o.guest_name,''),o.total_minor,o.currency,o.status,COALESCE(o.notes,''),o.created_at`, venueID, orderID, status).Scan(&i.ID, &i.Number, &i.Channel, &i.Source, &i.GuestName, &i.TotalMinor, &i.Currency, &i.Status, &i.Notes, &i.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	return i, err
}

func (s *Service) ListReviews(ctx context.Context, userID, venueID string) ([]Review, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT r.id::text,COALESCE(NULLIF(o.guest_name,''),'Гость'),r.rating,COALESCE(t.identifier,''),COALESCE('#'||o.order_number::text,''),COALESCE(r.body,''),COALESCE(r.owner_reply,''),r.replied_at,r.created_at FROM reviews r LEFT JOIN orders o ON o.id=r.order_id LEFT JOIN venue_tables t ON t.id=r.venue_table_id WHERE r.venue_id=$1 AND r.status='published' ORDER BY r.created_at DESC`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Review{}
	for rows.Next() {
		var i Review
		if err := rows.Scan(&i.ID, &i.GuestName, &i.Rating, &i.Table, &i.Order, &i.Body, &i.OwnerReply, &i.RepliedAt, &i.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, rows.Err()
}

func (s *Service) ReplyReview(ctx context.Context, userID, venueID, reviewID, reply string) (Review, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return Review{}, err
	}
	reply = strings.TrimSpace(reply)
	if reply == "" || utf8.RuneCountInString(reply) > 2000 {
		return Review{}, ErrInvalidInput
	}
	var i Review
	err := s.db.QueryRow(ctx, `UPDATE reviews r SET owner_reply=$4,replied_by_user_id=$1,replied_at=now()
	 WHERE r.id=$3 AND r.venue_id=$2
	 RETURNING r.id::text,
	 COALESCE(NULLIF((SELECT guest_name FROM orders WHERE id=r.order_id),''),'Гость'),
	 r.rating,COALESCE((SELECT identifier FROM venue_tables WHERE id=r.venue_table_id),''),
	 COALESCE('#'||(SELECT order_number::text FROM orders WHERE id=r.order_id),''),
	 COALESCE(r.body,''),COALESCE(r.owner_reply,''),r.replied_at,r.created_at`, userID, venueID, reviewID, reply).Scan(&i.ID, &i.GuestName, &i.Rating, &i.Table, &i.Order, &i.Body, &i.OwnerReply, &i.RepliedAt, &i.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, ErrNotFound
	}
	return i, err
}

func (s *Service) ListPayments(ctx context.Context, userID, venueID string) ([]Payment, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT id::text,provider,status,amount_minor,refunded_minor,currency,paid_at,created_at FROM payments WHERE venue_id=$1 ORDER BY created_at DESC LIMIT 200`, venueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Payment{}
	for rows.Next() {
		var i Payment
		if err := rows.Scan(&i.ID, &i.Provider, &i.Status, &i.AmountMinor, &i.RefundedMinor, &i.Currency, &i.PaidAt, &i.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, rows.Err()
}

func (s *Service) GetFloorPlan(ctx context.Context, userID, venueID string) (json.RawMessage, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return nil, err
	}
	var plan []byte
	err := s.db.QueryRow(ctx, `SELECT COALESCE(settings->'floor_plan','[]'::jsonb) FROM venues WHERE id=$1`, venueID).Scan(&plan)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return json.RawMessage(plan), err
}

func (s *Service) GetSubscription(ctx context.Context, userID string) (*Subscription, error) {
	var item Subscription
	err := s.db.QueryRow(ctx, `SELECT id::text,plan,billing_cycle,status,venue_limit,trial_ends_at,current_period_ends_at
	 FROM workspace_subscriptions WHERE owner_user_id=$1 AND status IN ('trialing','active','past_due')
	 ORDER BY created_at DESC LIMIT 1`, userID).
		Scan(&item.ID, &item.Plan, &item.BillingCycle, &item.Status, &item.VenueLimit, &item.TrialEndsAt, &item.CurrentPeriodEnds)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &item, err
}

func (s *Service) UpsertSubscription(ctx context.Context, userID string, input SubscriptionInput) (Subscription, error) {
	if !regexp.MustCompile(`^(start|business|pro)$`).MatchString(input.Plan) || !regexp.MustCompile(`^(monthly|yearly)$`).MatchString(input.BillingCycle) {
		return Subscription{}, ErrInvalidInput
	}
	limit := 1
	if input.Plan == "business" {
		limit = 3
	}
	var limitValue any = limit
	if input.Plan == "pro" {
		limitValue = nil
	}
	var venueCount int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM venues WHERE owner_user_id=$1 AND deleted_at IS NULL`, userID).Scan(&venueCount); err != nil {
		return Subscription{}, err
	}
	if limitValue != nil && venueCount > limit {
		return Subscription{}, ErrConflict
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Subscription{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `UPDATE workspace_subscriptions SET status='cancelled',cancelled_at=now() WHERE owner_user_id=$1 AND status IN ('trialing','active','past_due')`, userID); err != nil {
		return Subscription{}, err
	}
	var item Subscription
	err = tx.QueryRow(ctx, `INSERT INTO workspace_subscriptions(owner_user_id,provider,plan,billing_cycle,status,venue_limit,trial_ends_at,current_period_ends_at)
	 VALUES($1,'manual',$2,$3,'trialing',$4,now()+interval '14 days',now()+CASE WHEN $3='yearly' THEN interval '1 year' ELSE interval '1 month' END)
	 RETURNING id::text,plan,billing_cycle,status,venue_limit,trial_ends_at,current_period_ends_at`, userID, input.Plan, input.BillingCycle, limitValue).
		Scan(&item.ID, &item.Plan, &item.BillingCycle, &item.Status, &item.VenueLimit, &item.TrialEndsAt, &item.CurrentPeriodEnds)
	if err != nil {
		return Subscription{}, mapWriteError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Subscription{}, err
	}
	return item, nil
}

func (s *Service) SaveFloorPlan(ctx context.Context, userID, venueID string, plan json.RawMessage) (json.RawMessage, error) {
	if err := s.ensureOwner(ctx, userID, venueID); err != nil {
		return nil, err
	}
	if len(plan) == 0 || !json.Valid(plan) {
		return nil, ErrInvalidInput
	}
	result, err := s.db.Exec(ctx, `UPDATE venues SET settings=jsonb_set(settings,'{floor_plan}',$3::jsonb,true) WHERE id=$2 AND owner_user_id=$1 AND deleted_at IS NULL`, userID, venueID, []byte(plan))
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return plan, nil
}

func (s *Service) ensureOwner(ctx context.Context, userID, venueID string) error {
	var ok bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM venues WHERE id=$2 AND owner_user_id=$1 AND deleted_at IS NULL)`, userID, venueID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

type venueScanner interface{ Scan(...any) error }

func scanVenue(row venueScanner) (Venue, error) {
	var v Venue
	var settings []byte
	err := row.Scan(&v.ID, &v.Name, &v.Slug, &v.Description, &v.CuisineType, &v.Phone, &v.Email, &v.Address, &v.City, &v.PostalCode, &v.CountryCode, &v.Timezone, &v.Currency, &v.Status, &settings, &v.CreatedAt)
	if err == nil {
		_ = json.Unmarshal(settings, &v.Settings)
		if v.Settings == nil {
			v.Settings = map[string]any{}
		}
	}
	return v, err
}
func normalizeVenueInput(i VenueInput) VenueInput {
	i.Name = strings.TrimSpace(i.Name)
	i.Description = strings.TrimSpace(i.Description)
	i.CuisineType = strings.TrimSpace(i.CuisineType)
	i.Phone = strings.TrimSpace(i.Phone)
	i.Email = strings.TrimSpace(i.Email)
	i.Address = strings.TrimSpace(i.Address)
	i.City = strings.TrimSpace(i.City)
	i.PostalCode = strings.TrimSpace(i.PostalCode)
	i.CountryCode = strings.ToUpper(strings.TrimSpace(i.CountryCode))
	i.Timezone = strings.TrimSpace(i.Timezone)
	i.Currency = strings.ToUpper(strings.TrimSpace(i.Currency))
	i.Status = strings.TrimSpace(i.Status)
	if i.Address == "" {
		i.Address = "Адрес не указан"
	}
	if i.CountryCode == "" {
		i.CountryCode = "SK"
	}
	if i.Timezone == "" {
		i.Timezone = "Europe/Bratislava"
	}
	if i.Currency == "" {
		i.Currency = "EUR"
	}
	if i.Status == "" {
		i.Status = "draft"
	}
	if i.Settings == nil {
		i.Settings = map[string]any{}
	}
	return i
}
func validVenueInput(i VenueInput) bool {
	return i.Name != "" && i.City != "" && len(i.CountryCode) == 2 && len(i.Currency) == 3 && regexp.MustCompile(`^(draft|active|paused|closed)$`).MatchString(i.Status)
}
func normalizeMenuItem(i MenuItemInput) MenuItemInput {
	i.Name = strings.TrimSpace(i.Name)
	i.Description = strings.TrimSpace(i.Description)
	i.Currency = strings.ToUpper(strings.TrimSpace(i.Currency))
	if i.Currency == "" {
		i.Currency = "EUR"
	}
	return i
}
func validMenuItem(i MenuItemInput) bool {
	return i.CategoryID != "" && i.Name != "" && i.PriceMinor >= 0 && len(i.Currency) == 3
}
func validStaffRole(value string) bool {
	return regexp.MustCompile(`^(manager|waiter|kitchen|viewer)$`).MatchString(value)
}
func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}
func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			return ErrConflict
		}
		if pgErr.Code == "23503" || pgErr.Code == "23514" {
			return ErrInvalidInput
		}
	}
	return err
}
