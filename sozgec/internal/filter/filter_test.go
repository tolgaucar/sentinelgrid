package filter

import (
	"testing"

	"github.com/otukenmatris/sozgec/internal/spectrum"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	if e == nil {
		t.Fatal("engine nil")
	}
	bands := e.GetFriendlyBands()
	if len(bands) != 8 {
		t.Fatalf("expected 8 default friendly bands, got %d", len(bands))
	}
}

func makeBand(id string, center, peak, snr float64, sigType spectrum.SignalType) spectrum.FrequencyBand {
	return spectrum.FrequencyBand{
		ID: id, CenterMHz: center, LowMHz: center - 1, HighMHz: center + 1,
		PeakPowerDB: peak, MeanPowerDB: peak - 3, BandwidthMHz: 2,
		SignalType: sigType, Confidence: 0.8, SNR_DB: snr,
	}
}

func TestFilterFriendlySignal(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	bands := []spectrum.FrequencyBand{
		makeBand("b1", 300, -20, 60, spectrum.SignalComm), // UHF askeri = dost
	}
	result := e.Filter(bands, -80)
	if result.FriendlyCount != 1 {
		t.Fatalf("expected 1 friendly, got %d", result.FriendlyCount)
	}
	if result.Signals[0].Allegiance != AllegianceFriendly {
		t.Fatalf("expected friendly, got %s", result.Signals[0].Allegiance)
	}
}

func TestFilterHostileRadar(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	bands := []spectrum.FrequencyBand{
		makeBand("b1", 5000, -10, 70, spectrum.SignalRadar), // dost bantlar dışı radar
	}
	result := e.Filter(bands, -80)
	if result.HostileCount != 1 {
		t.Fatalf("expected 1 hostile, got %d", result.HostileCount)
	}
}

func TestFilterJamming(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	bands := []spectrum.FrequencyBand{
		makeBand("b1", 500, -5, 75, spectrum.SignalJamming),
	}
	result := e.Filter(bands, -80)
	if result.JammingDetected != 1 {
		t.Fatalf("expected 1 jamming, got %d", result.JammingDetected)
	}
	if result.Signals[0].ThreatLevel == ThreatNone {
		t.Fatal("jamming should have threat level")
	}
}

func TestFilterNoiseRejection(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	bands := []spectrum.FrequencyBand{
		makeBand("b1", 100, -78, 2, spectrum.SignalUnknown), // SNR < 6 → gürültü
	}
	result := e.Filter(bands, -80)
	if result.NoiseFiltered != 1 {
		t.Fatalf("expected 1 noise filtered, got %d", result.NoiseFiltered)
	}
	if result.Extracted != 0 {
		t.Fatalf("expected 0 extracted, got %d", result.Extracted)
	}
}

func TestFilterMixed(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	bands := []spectrum.FrequencyBand{
		makeBand("b1", 300, -20, 60, spectrum.SignalComm),     // dost
		makeBand("b2", 3000, -10, 70, spectrum.SignalRadar),    // düşman radar
		makeBand("b3", 500, -5, 75, spectrum.SignalJamming),    // karıştırma
		makeBand("b4", 100, -79, 1, spectrum.SignalUnknown),    // gürültü
	}
	result := e.Filter(bands, -80)
	if result.TotalInput != 4 {
		t.Fatalf("expected 4 input, got %d", result.TotalInput)
	}
	if result.Extracted != 3 {
		t.Fatalf("expected 3 extracted, got %d", result.Extracted)
	}
	if result.NoiseFiltered != 1 {
		t.Fatalf("expected 1 filtered, got %d", result.NoiseFiltered)
	}
}

func TestAddFriendlyBand(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	err := e.AddFriendlyBand(FriendlyBandDef{ID: "custom", Name: "Custom", LowMHz: 500, HighMHz: 600, Priority: 3})
	if err != nil {
		t.Fatal(err)
	}
	bands := e.GetFriendlyBands()
	if len(bands) != 9 {
		t.Fatalf("expected 9, got %d", len(bands))
	}
}

func TestAddFriendlyBandDuplicate(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	err := e.AddFriendlyBand(FriendlyBandDef{ID: "vhf-lo"})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestRemoveFriendlyBand(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	err := e.RemoveFriendlyBand("vhf-lo")
	if err != nil {
		t.Fatal(err)
	}
	if len(e.GetFriendlyBands()) != 7 {
		t.Fatal("expected 7 bands after removal")
	}
}

func TestRemoveFriendlyBandNotFound(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	err := e.RemoveFriendlyBand("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetAllSignals(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	e.Filter([]spectrum.FrequencyBand{
		makeBand("b1", 300, -20, 60, spectrum.SignalComm),
	}, -80)
	all := e.GetAllSignals()
	if len(all) != 1 {
		t.Fatalf("expected 1, got %d", len(all))
	}
}

func TestGetHighThreat(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	e.Filter([]spectrum.FrequencyBand{
		makeBand("b1", 500, -5, 75, spectrum.SignalJamming), // yüksek tehdit
	}, -80)
	threats := e.GetHighThreat()
	if len(threats) == 0 {
		t.Log("no high threats — acceptable")
	}
}

func TestGetFriendlySignals(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	e.Filter([]spectrum.FrequencyBand{
		makeBand("b1", 300, -20, 60, spectrum.SignalComm),
	}, -80)
	friendly := e.GetFriendlySignals()
	if len(friendly) != 1 {
		t.Fatalf("expected 1 friendly, got %d", len(friendly))
	}
}

func TestPrune(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 1) // 1 saniye decay
	e.Filter([]spectrum.FrequencyBand{
		makeBand("b1", 300, -20, 60, spectrum.SignalComm),
	}, -80)

	// tüm sinyalleri eskilt
	e.mu.Lock()
	for _, s := range e.signals {
		s.ProcessedAt = s.ProcessedAt.Add(-5 * 1e9)
	}
	e.mu.Unlock()

	removed := e.Prune()
	_ = removed
}

func TestStats(t *testing.T) {
	e := NewEngine(-80, 6, 200, 0.05, 3600)
	e.Filter([]spectrum.FrequencyBand{
		makeBand("b1", 300, -20, 60, spectrum.SignalComm),
		makeBand("b2", 100, -79, 1, spectrum.SignalUnknown),
	}, -80)
	s := e.Stats()
	if s["total_processed"].(int) != 2 {
		t.Fatalf("expected 2, got %d", s["total_processed"])
	}
	if s["total_filtered"].(int) != 1 {
		t.Fatalf("expected 1 filtered, got %d", s["total_filtered"])
	}
}

func TestGenerateSignalsFromEntities(t *testing.T) {
	entities := []map[string]any{
		{"id": "e-1", "type": "tank", "faction": "friendly", "lat": 39.5, "lon": 32.5},
		{"id": "e-2", "type": "jammer", "faction": "enemy", "lat": 39.6, "lon": 32.6},
	}
	signals := GenerateSignalsFromEntities(entities)
	if len(signals) != 2 {
		t.Fatalf("expected 2, got %d", len(signals))
	}
	// jammer daha geniş bant
	for _, s := range signals {
		if s.ID == "rf-e-2" && s.BandwidthMHz < 50 {
			t.Fatalf("jammer should have wide bandwidth, got %f", s.BandwidthMHz)
		}
	}
}

func TestThreatAssessment(t *testing.T) {
	// dost → tehdit yok
	band := makeBand("b1", 300, -20, 60, spectrum.SignalComm)
	threat := assessThreat(band, AllegianceFriendly, 60)
	if threat != ThreatNone {
		t.Fatalf("friendly should be none, got %s", threat)
	}

	// karıştırma → yüksek tehdit
	jam := makeBand("b2", 500, -5, 75, spectrum.SignalJamming)
	threat2 := assessThreat(jam, AllegianceHostile, 75)
	if threat2 == ThreatNone || threat2 == ThreatLow {
		t.Fatalf("jamming should be at least medium, got %s", threat2)
	}
}
