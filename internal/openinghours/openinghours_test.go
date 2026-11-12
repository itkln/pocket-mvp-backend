package openinghours

import (
	"encoding/json"
	"testing"
	"time"
)

func decode(t *testing.T, raw string) any {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

const fullWeek = `{"mon":{"open":"10:00","close":"23:30"},"tue":null,"wed":{"open":"09:00","close":"22:00"},
	"thu":{"open":"09:00","close":"22:00"},"fri":{"open":"09:00","close":"24:00"},
	"sat":{"open":"12:00","close":"22:00"},"sun":null}`

func TestParse(t *testing.T) {
	week, err := Parse(decode(t, fullWeek))
	if err != nil {
		t.Fatal(err)
	}
	if week["tue"] != nil || week["mon"].Open != "10:00" || week["fri"].Close != "24:00" {
		t.Fatalf("parsed week: %+v", week)
	}
	if week, err = Parse(nil); err != nil || week["sun"] == nil || week["sun"].Open != "09:00" {
		t.Fatalf("default week: %+v %v", week, err)
	}
	for _, invalid := range []string{
		`[]`, `{"mon":null}`,
		`{"mon":{"open":"22:00","close":"09:00"},"tue":null,"wed":null,"thu":null,"fri":null,"sat":null,"sun":null}`,
		`{"mon":{"open":"9:00","close":"22:00"},"tue":null,"wed":null,"thu":null,"fri":null,"sat":null,"sun":null}`,
		`{"mon":{"open":"09:00","close":"24:30"},"tue":null,"wed":null,"thu":null,"fri":null,"sat":null,"sun":null}`,
		`{"mon":null,"tue":null,"wed":null,"thu":null,"fri":null,"sat":null,"sun":null,"xyz":null}`,
		`{"mon":"closed","tue":null,"wed":null,"thu":null,"fri":null,"sat":null,"sun":null}`,
	} {
		if _, err := Parse(decode(t, invalid)); err == nil {
			t.Errorf("accepted %s", invalid)
		}
	}
}

func TestWindow(t *testing.T) {
	week, err := Parse(decode(t, fullWeek))
	if err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation("Europe/Bratislava")
	if err != nil {
		t.Fatal(err)
	}
	monday := time.Date(2026, 11, 16, 0, 0, 0, 0, loc)
	open, close, ok := week.Window(monday, loc)
	if !ok || open.Format("15:04") != "10:00" || close.Format("15:04") != "23:30" {
		t.Fatalf("monday: %v %v %v", open, close, ok)
	}
	if _, _, ok = week.Window(monday.AddDate(0, 0, 1), loc); ok {
		t.Fatal("closed tuesday has hours")
	}
	_, close, ok = week.Window(monday.AddDate(0, 0, 4), loc)
	if !ok || !close.Equal(time.Date(2026, 11, 21, 0, 0, 0, 0, loc)) {
		t.Fatalf("friday closes at midnight: %v", close)
	}
}
