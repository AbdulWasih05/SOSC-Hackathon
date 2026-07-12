package check

import "palkwatch/internal/alert"

// Sea-state thresholds (wave height, meters) for weather-modulated fishing
// confidence. Weather discounts each pattern by how confusable it is with
// sea-keeping:
//
//   - TRAWLING (slow + high course spread) is directly mimicked by swell, so it
//     is discounted at a moderate sea state.
//   - LONGLINING (sawtooth speed on a straight track) is only partly confusable
//     (swell adds speed noise), so it takes a rougher sea to explain away, hence
//     a higher threshold.
//   - PURSE_SEINING (a deliberate 270-degree net loop) is geometric; weather can
//     neither fake nor mask it, so weather is not a confidence factor at all.
//
// In calm water (below calmM) a weather-sensitive pattern has no environmental
// excuse, so confidence is raised.
const (
	calmM       = 0.8 // below this: calm, behavior unexplained by weather
	roughTrawlM = 2.0 // trawling: discounted at/above this sea state
	roughLongM  = 2.5 // longlining: needs a rougher sea to be explained away
)

// weatherConfidence returns the confidence label for a fishing pattern given the
// sea state in meters: alert.WeatherConfLow (sea state likely explains the
// track), alert.WeatherConfHigh (calm; not weather-induced), or "" when weather
// is not a factor for this pattern / sea state. Pure and side-effect free so it
// is table-testable.
func weatherConfidence(kind string, waveM float64) string {
	switch kind {
	case alert.KindTrawling:
		if waveM >= roughTrawlM {
			return alert.WeatherConfLow
		}
		if waveM < calmM {
			return alert.WeatherConfHigh
		}
	case alert.KindLonglining:
		if waveM >= roughLongM {
			return alert.WeatherConfLow
		}
		if waveM < calmM {
			return alert.WeatherConfHigh
		}
	case alert.KindPurseSeining:
		// Geometric signature; weather is not a confidence factor.
		return ""
	}
	return ""
}
