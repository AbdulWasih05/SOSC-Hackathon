package check

import (
	"testing"

	"palkwatch/internal/alert"
)

func TestWeatherConfidence(t *testing.T) {
	tests := []struct {
		name  string
		kind  string
		waveM float64
		want  string
	}{
		// Trawling: mimicked by swell, discounted from a moderate sea state up.
		{"trawl calm -> HIGH", alert.KindTrawling, 0.5, alert.WeatherConfHigh},
		{"trawl moderate -> neutral", alert.KindTrawling, 1.3, ""},
		{"trawl at rough threshold -> LOW", alert.KindTrawling, 2.0, alert.WeatherConfLow},
		{"trawl rough -> LOW", alert.KindTrawling, 3.1, alert.WeatherConfLow},

		// Longlining: only partly confusable, so it needs a rougher sea (2.5 m)
		// to be explained away. At 2.0 m (which discounts trawling) it is still
		// neutral, which is the whole point of the per-pattern differentiation.
		{"longline calm -> HIGH", alert.KindLonglining, 0.5, alert.WeatherConfHigh},
		{"longline 2.0m -> still neutral", alert.KindLonglining, 2.0, ""},
		{"longline at rough threshold -> LOW", alert.KindLonglining, 2.5, alert.WeatherConfLow},

		// Purse seining: geometric, weather is never a factor at any sea state.
		{"seine calm -> neutral", alert.KindPurseSeining, 0.3, ""},
		{"seine rough -> neutral", alert.KindPurseSeining, 3.5, ""},

		// Unknown / non-fishing kinds are never weather-modulated.
		{"non-fishing kind -> neutral", alert.KindZone, 3.0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := weatherConfidence(tt.kind, tt.waveM); got != tt.want {
				t.Errorf("weatherConfidence(%s, %.1f) = %q, want %q", tt.kind, tt.waveM, got, tt.want)
			}
		})
	}
}
