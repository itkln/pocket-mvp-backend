// Package openinghours reads the venue's weekly opening hours, kept in venues.settings under
// "opening_hours" as {"mon": {"open": "09:00", "close": "22:00"}, "tue": null, ...}: a day set to
// null is closed. Bookings are offered inside these hours and Dish shows them to guests.
package openinghours

import (
	"fmt"
	"time"
)

// SettingKey is the venues.settings key holding the hours.
const SettingKey = "opening_hours"

// Days are the keys of a week, Monday first.
var Days = [7]string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// Day is one day's hours, "HH:MM" in the venue's timezone; close is after open, on the same day
// ("24:00" closes at midnight).
type Day struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

// Week maps each day key to its hours; nil means closed that day.
type Week map[string]*Day

// Default is used while the venue has not set its hours: every day 09:00–22:00.
func Default() Week {
	week := Week{}
	for _, day := range Days {
		week[day] = &Day{Open: "09:00", Close: "22:00"}
	}
	return week
}

// Parse validates the stored or submitted value. A missing value (nil) gives the default week.
func Parse(value any) (Week, error) {
	if value == nil {
		return Default(), nil
	}
	raw, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("opening hours must be an object")
	}
	week := Week{}
	for key, entry := range raw {
		if dayIndex(key) < 0 {
			return nil, fmt.Errorf("unknown day %q", key)
		}
		if entry == nil {
			week[key] = nil
			continue
		}
		fields, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("day %s must be an object or null", key)
		}
		open, _ := fields["open"].(string)
		close, _ := fields["close"].(string)
		openMinutes, okOpen := minutes(open)
		closeMinutes, okClose := minutes(close)
		if !okOpen || !okClose || openMinutes >= closeMinutes || openMinutes == 24*60 {
			return nil, fmt.Errorf("day %s needs open before close", key)
		}
		week[key] = &Day{Open: open, Close: close}
	}
	for _, day := range Days {
		if _, ok := week[day]; !ok {
			return nil, fmt.Errorf("day %s is missing", day)
		}
	}
	return week, nil
}

// Window returns when the venue opens and closes on the given date (its year, month and day in
// loc); ok is false on a closed day.
func (w Week) Window(date time.Time, loc *time.Location) (open, close time.Time, ok bool) {
	day := w[Days[(int(date.Weekday())+6)%7]]
	if day == nil {
		return time.Time{}, time.Time{}, false
	}
	openMinutes, _ := minutes(day.Open)
	closeMinutes, _ := minutes(day.Close)
	midnight := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
	at := func(m int) time.Time {
		return time.Date(midnight.Year(), midnight.Month(), midnight.Day(), m/60, m%60, 0, 0, loc)
	}
	return at(openMinutes), at(closeMinutes), true
}

func dayIndex(key string) int {
	for i, day := range Days {
		if day == key {
			return i
		}
	}
	return -1
}

// minutes parses "HH:MM" (00:00–24:00) into minutes after midnight.
func minutes(value string) (int, bool) {
	if len(value) != 5 || value[2] != ':' {
		return 0, false
	}
	var h, m int
	if _, err := fmt.Sscanf(value, "%02d:%02d", &h, &m); err != nil {
		return 0, false
	}
	if h < 0 || m < 0 || m > 59 || h > 24 || (h == 24 && m != 0) {
		return 0, false
	}
	return h*60 + m, true
}
