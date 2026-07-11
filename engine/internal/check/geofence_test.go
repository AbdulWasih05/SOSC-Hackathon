package check

import (
	"testing"

	"palkwatch/internal/geo"
)

func TestZoneViolation(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		zoneCountry uint16
		vesselFlag  uint16
		want        bool
	}{
		{"mpa entry by anyone", geo.KindMPA, geo.CountryUnknown, geo.CountryIN, true},
		{"mpa entry unknown flag", geo.KindMPA, geo.CountryUnknown, geo.CountryUnknown, true},
		{"eez domestic vessel", geo.KindEEZ, geo.CountryIN, geo.CountryIN, false},
		{"eez foreign vessel", geo.KindEEZ, geo.CountryIN, geo.CountryLK, true},
		{"eez unknown flag ignored", geo.KindEEZ, geo.CountryIN, geo.CountryUnknown, false},
		{"unknown kind never violates", "port", geo.CountryIN, geo.CountryLK, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ZoneViolation(tt.kind, tt.zoneCountry, tt.vesselFlag); got != tt.want {
				t.Fatalf("ZoneViolation(%q, %d, %d) = %v, want %v", tt.kind, tt.zoneCountry, tt.vesselFlag, got, tt.want)
			}
		})
	}
}
