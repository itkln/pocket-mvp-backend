package bootstrap

import (
	"context"
	"time"

	"pocket-mvp-backend/internal/appfault"
	"pocket-mvp-backend/internal/modules/availability"
	"pocket-mvp-backend/internal/modules/reservations"
)

type reservationAvailability struct {
	service    *availability.Service
	repository *availability.PostgresRepository
}

func (a reservationAvailability) FindSlot(ctx context.Context, venueID string, start time.Time, guests int) (reservations.Slot, error) {
	venue, err := a.repository.GetVenue(ctx, venueID)
	if err != nil {
		return reservations.Slot{}, err
	}
	zone, err := time.LoadLocation(venue.Timezone)
	if err != nil {
		return reservations.Slot{}, err
	}
	result, err := a.service.Get(ctx, venueID, start.In(zone).Format("2006-01-02"), guests)
	if err != nil {
		return reservations.Slot{}, err
	}
	for _, slot := range result.Slots {
		if slot.StartsAt.Equal(start) && len(slot.Tables) > 0 {
			return reservations.Slot{TableID: slot.Tables[0].ID, StartsAt: slot.StartsAt, EndsAt: slot.EndsAt}, nil
		}
	}
	return reservations.Slot{}, appfault.ErrConflict
}
