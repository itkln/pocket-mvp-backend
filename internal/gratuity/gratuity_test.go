package gratuity

import (
	"errors"
	"testing"

	"pocket-mvp-backend/internal/appfault"
)

func TestCalculateMinorUnits(t *testing.T) {
	for _, test := range []struct {
		name string
		base int
		tip  *Selection
		want int
	}{
		{"legacy", 1090, nil, 0},
		{"none", 1090, &Selection{Mode: "none"}, 0},
		{"zero percent", 1090, &Selection{Mode: "percent"}, 0},
		{"half cent rounds up", 1090, &Selection{Mode: "percent", Percent: 5}, 55},
		{"less than half rounds down", 1099, &Selection{Mode: "percent", Percent: 10}, 110},
		{"aggregate before rounding", 30, &Selection{Mode: "percent", Percent: 5}, 2},
		{"custom", 1090, &Selection{Mode: "custom", AmountMinor: 123}, 123},
		{"custom zero", 1090, &Selection{Mode: "custom"}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Calculate(test.base, test.tip)
			if err != nil || got != test.want {
				t.Fatalf("got %d, %v; want %d", got, err, test.want)
			}
		})
	}
}

func TestRejectInvalidOrOverflowingTip(t *testing.T) {
	for _, tip := range []*Selection{
		{Mode: "invalid"}, {Mode: "none", AmountMinor: 1},
		{Mode: "percent", Percent: -1}, {Mode: "percent", Percent: 101},
		{Mode: "percent", Percent: 5, AmountMinor: 10},
		{Mode: "custom", Percent: 5}, {Mode: "custom", AmountMinor: -1},
		{Mode: "custom", AmountMinor: MaxTotal + 1},
	} {
		if _, err := Calculate(1000, tip); !errors.Is(err, appfault.ErrInvalidInput) {
			t.Fatalf("accepted %+v: %v", tip, err)
		}
	}
	for _, base := range []int{-1, MaxTotal + 1, MaxTotal, 0} {
		if _, err := Calculate(base, &Selection{Mode: "custom", AmountMinor: 1}); !errors.Is(err, appfault.ErrInvalidInput) {
			t.Fatalf("accepted base %d: %v", base, err)
		}
	}
}
