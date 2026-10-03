package attrition

import "testing"

func TestDateRoundTrip(t *testing.T) {
	cases := []Date{
		NewDate(1970, 1, 1),
		NewDate(2000, 2, 29),
		NewDate(2025, 7, 1),
		NewDate(2025, 12, 31),
		NewDate(1999, 12, 31),
		NewDate(1900, 3, 1),
	}
	for _, c := range cases {
		got := dateFromDays(c.days())
		if got != c {
			t.Fatalf("round trip %v -> %v", c, got)
		}
		parsed, err := ParseDate(c.String())
		if err != nil || parsed != c {
			t.Fatalf("parse %q -> %v, %v", c.String(), parsed, err)
		}
	}
	if NewDate(1970, 1, 1).days() != 0 {
		t.Fatal("epoch day mismatch")
	}
	if NewDate(2025, 7, 1).AddDays(2) != NewDate(2025, 7, 3) {
		t.Fatal("AddDays mismatch")
	}
	if NewDate(2025, 7, 1).DaysUntil(NewDate(2025, 7, 6)) != 5 {
		t.Fatal("DaysUntil mismatch")
	}
	if !NewDate(2025, 2, 28).AddDays(1).Equal(NewDate(2025, 3, 1)) {
		t.Fatal("non-leap Feb rollover mismatch")
	}
	if !NewDate(2024, 2, 28).AddDays(1).Equal(NewDate(2024, 2, 29)) {
		t.Fatal("leap Feb rollover mismatch")
	}
}
