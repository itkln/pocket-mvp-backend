package ordering

import (
	"errors"
	"testing"

	"pocket-mvp-backend/internal/appfault"
)

func TestTransitionMatrix(t *testing.T) {
	statuses := []string{"new", "preparing", "served", "completed", "cancelled"}
	for _, channel := range []string{"dine_in", "preorder", "online"} {
		for _, role := range []string{"owner", "manager", "waiter", "kitchen", "viewer"} {
			for _, from := range statuses {
				for _, to := range statuses {
					authorized := role == "owner" || role == "manager" || role == "waiter"
					edge := from == "new" && to == "preparing" || from == "preparing" && to == "served" || from == "served" && to == "completed"
					want := authorized && (from == to || from != "completed" && from != "cancelled" && (edge || to == "cancelled"))
					err := validateTransition(role, channel, from, to, from)
					if (err == nil) != want {
						t.Fatalf("%s %s %s→%s: %v", role, channel, from, to, err)
					}
				}
			}
		}
	}
}

func TestRemovedStatusesAreRejected(t *testing.T) {
	for _, status := range []string{"accepted", "ready"} {
		if err := validateTransition("owner", "dine_in", "new", status, "new"); !errors.Is(err, appfault.ErrInvalidInput) {
			t.Fatalf("%s accepted: %v", status, err)
		}
	}
}

func TestStaleAndReplay(t *testing.T) {
	err := validateTransition("owner", "dine_in", "preparing", "served", "new")
	var conflict *StatusConflict
	if !errors.As(err, &conflict) || conflict.CurrentStatus != "preparing" {
		t.Fatal(err)
	}
	if err := validateTransition("waiter", "dine_in", "served", "served", "preparing"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(validateTransition("kitchen", "dine_in", "new", "preparing", "new"), appfault.ErrForbidden) {
		t.Fatal("removed kitchen role may still change orders")
	}
}
