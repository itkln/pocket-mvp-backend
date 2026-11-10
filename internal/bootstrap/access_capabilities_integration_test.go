package bootstrap

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"pocket-mvp-backend/internal/access"
	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/catalog"
	"pocket-mvp-backend/internal/modules/dinein"
	"pocket-mvp-backend/internal/modules/feedback"
	"pocket-mvp-backend/internal/modules/floorplan"
	"pocket-mvp-backend/internal/modules/workforce"
	"pocket-mvp-backend/internal/security"
)

// V-BE-02: managers run the menu and the floor (plan and table QR codes) besides orders; waiters
// only orders; team, reviews and the rest stay with the owner. Denied users get 404.
func TestVenueCapabilitiesIntegration(t *testing.T) {
	db := resetTestDatabase(t)
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	user := func(name string) string {
		var id string
		must(db.QueryRow(ctx, `INSERT INTO users(email,password_hash,first_name,last_name,account_role) VALUES($1,'x','Test','User','customer') RETURNING id::text`, name+"@capabilities.test").Scan(&id))
		return id
	}
	owner := user("owner")
	var venue string
	must(db.QueryRow(ctx, `INSERT INTO venues(owner_user_id,slug,name,address_line1,city,country_code,timezone,currency,status) VALUES($1,'capabilities','Capabilities','Main','Bratislava','SK','Europe/Bratislava','EUR','active') RETURNING id::text`, owner).Scan(&venue))
	actors := map[string]string{"owner": owner, "outsider": user("outsider")}
	for _, role := range []string{"manager", "waiter"} {
		actors[role] = user(role)
		_, err := db.Exec(ctx, `INSERT INTO venue_staff(venue_id,user_id,invited_email,role,status,invited_by_user_id) VALUES($1,$2,$3,$4,'active',$5)`, venue, actors[role], role+"@capabilities.test", role, owner)
		must(err)
	}
	authorizer := access.NewVenueAuthorizer(db)
	protector, err := security.NewDataProtector(bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32))
	must(err)
	menu := catalog.NewService(catalog.NewPostgresRepository(db), authorizer)
	floor := floorplan.NewService(floorplan.NewPostgresRepository(db), authorizer)
	tables := dinein.NewService(dinein.NewPostgresRepository(db), authorizer, protector, "https://dish.example")
	team := workforce.NewService(workforce.NewPostgresRepository(db), authorizer)
	reviews := feedback.NewService(feedback.NewPostgresRepository(db), authorizer)
	plan := json.RawMessage(`[{"id":"floor-1","name":"Main","tables":[{"id":"07","seats":4,"x":10,"y":20}],"fixtures":[]}]`)

	checks := map[string]func(string) error{
		"menu": func(actor string) error {
			if _, err := menu.CreateCategory(ctx, actor, venue, catalog.CategoryInput{Name: "Menu " + actor}); err != nil {
				return err
			}
			_, err := menu.ListMenuItems(ctx, actor, venue)
			return err
		},
		"floor": func(actor string) error {
			if _, err := floor.Save(ctx, actor, venue, plan); err != nil {
				return err
			}
			if _, err := floor.Get(ctx, actor, venue); err != nil {
				return err
			}
			_, err := tables.OwnerQRCode(ctx, actor, venue, "07", false)
			return err
		},
		"team": func(actor string) error {
			_, err := team.List(ctx, actor, venue)
			return err
		},
		"reviews": func(actor string) error {
			_, err := reviews.List(ctx, actor, venue)
			return err
		},
	}
	allowed := map[string][]string{
		"owner":    {"menu", "floor", "team", "reviews"},
		"manager":  {"menu", "floor"},
		"waiter":   {},
		"outsider": {},
	}
	for actor, grants := range allowed {
		for area, check := range checks {
			want := false
			for _, grant := range grants {
				want = want || grant == area
			}
			err := check(actors[actor])
			if want && err != nil {
				t.Errorf("%s should manage %s: %v", actor, area, err)
			}
			if !want && !errors.Is(err, appfault.ErrNotFound) {
				t.Errorf("%s must not manage %s (want 404), got %v", actor, area, err)
			}
		}
	}
	// A manager whose membership ends loses the menu at once.
	_, err = db.Exec(ctx, `UPDATE venue_staff SET status='inactive' WHERE user_id=$1`, actors["manager"])
	must(err)
	if err := checks["menu"](actors["manager"]); !errors.Is(err, appfault.ErrNotFound) {
		t.Fatalf("inactive manager kept the menu: %v", err)
	}
}
