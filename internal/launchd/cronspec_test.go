package launchd

import (
	"testing"
)

// p returns a pointer to v — convenience for table-driven tests where the
// expected CalEntry fields are *int.
func p(v int) *int { return &v }

func TestParseCron(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		want    []CalEntry
		wantErr bool
	}{
		{
			name: "daily at 9:00",
			expr: "0 9 * * *",
			want: []CalEntry{{Minute: p(0), Hour: p(9)}},
		},
		{
			name: "every Sunday at 8:00",
			expr: "0 8 * * 0",
			want: []CalEntry{{Minute: p(0), Hour: p(8), Weekday: p(0)}},
		},
		{
			name: "1st and 15th at 9:00",
			expr: "0 9 1,15 * *",
			want: []CalEntry{
				{Minute: p(0), Hour: p(9), Day: p(1)},
				{Minute: p(0), Hour: p(9), Day: p(15)},
			},
		},
		{
			name: "every 15 minutes",
			expr: "*/15 * * * *",
			want: []CalEntry{
				{Minute: p(0)},
				{Minute: p(15)},
				{Minute: p(30)},
				{Minute: p(45)},
			},
		},
		{
			name: "weekdays at 2:30",
			expr: "30 2 * * 1-5",
			want: []CalEntry{
				{Minute: p(30), Hour: p(2), Weekday: p(1)},
				{Minute: p(30), Hour: p(2), Weekday: p(2)},
				{Minute: p(30), Hour: p(2), Weekday: p(3)},
				{Minute: p(30), Hour: p(2), Weekday: p(4)},
				{Minute: p(30), Hour: p(2), Weekday: p(5)},
			},
		},
		{
			name:    "wrong field count",
			expr:    "0 9 * *",
			wantErr: true,
		},
		{
			name:    "invalid minute",
			expr:    "60 9 * * *",
			wantErr: true,
		},
		{
			name:    "invalid range",
			expr:    "0 9 * * 7",
			wantErr: true,
		},
		{
			name:    "step zero",
			expr:    "*/0 * * * *",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCron(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseCron(%q) err = %v, wantErr=%v", tt.expr, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !calEntriesEqual(got, tt.want) {
				t.Errorf("ParseCron(%q):\n got = %v\nwant = %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestParseCronExpansionCap(t *testing.T) {
	// Wildcard fields collapse to a single (nil) entry, so they don't
	// blow up. To exceed the 256-entry cap we need EXPLICIT enumeration,
	// e.g. "every minute of every hour" = 60×24 = 1440.
	_, err := ParseCron("*/1 */1 * * *")
	if err == nil {
		t.Error("expected error for over-expansion, got nil")
	}
}

func calEntriesEqual(a, b []CalEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !intPtrEq(a[i].Minute, b[i].Minute) ||
			!intPtrEq(a[i].Hour, b[i].Hour) ||
			!intPtrEq(a[i].Day, b[i].Day) ||
			!intPtrEq(a[i].Month, b[i].Month) ||
			!intPtrEq(a[i].Weekday, b[i].Weekday) {
			return false
		}
	}
	return true
}

func intPtrEq(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
