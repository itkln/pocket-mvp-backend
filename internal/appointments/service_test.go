package appointments

import (
	"strings"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	valid := Input{ServiceID: "40000000-0000-4000-8000-000000020101", StartsAt: time.Now(), Name: "Анна", Phone: "+421900000000", Key: "appointment-test-key", Notes: strings.Repeat("я", 500)}
	if _, err := normalize(valid); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Input){func(i *Input) { i.Notes += "я" }, func(i *Input) { i.ServiceID = "invalid" }, func(i *Input) { i.ExpectedPrice = -1 }, func(i *Input) { i.Name = " " }, func(i *Input) { i.Phone = "" }} {
		invalid := valid
		mutate(&invalid)
		if _, err := normalize(invalid); err == nil {
			t.Fatal("accepted invalid appointment")
		}
	}
}
