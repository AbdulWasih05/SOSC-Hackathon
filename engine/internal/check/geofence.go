// Package check runs the inline per-message alerts (zone, spoof) and, later, the
// dark-event sweep. The pure decision functions live here so they can be
// table-tested; the Processor wires them to state and the grid.
package check

import "palkwatch/internal/geo"

// ZoneViolation reports whether a vessel newly entering a zone constitutes a
// violation. An MPA is restricted outright, so any entry violates. An EEZ is
// violated only by a foreign-flagged vessel: the flag must be known and differ
// from the zone's country.
func ZoneViolation(kind string, zoneCountry, vesselFlag uint16) bool {
	switch kind {
	case geo.KindMPA:
		return true
	case geo.KindEEZ:
		return vesselFlag != geo.CountryUnknown && vesselFlag != zoneCountry
	default:
		return false
	}
}
