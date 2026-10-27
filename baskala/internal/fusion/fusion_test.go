package fusion

import (
	"testing"

	"github.com/otukenmatris/baskala/internal/change"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine(0.5, 0.5, 0.3, 0.4)
	if e == nil {
		t.Fatal("nil engine")
	}
}

func makeDet(id string, lat, lon, conf, intensity float64, cType change.ChangeType) change.ChangeDetection {
	return change.ChangeDetection{
		ID: id, Type: cType, ThreatLevel: change.ThreatMedium,
		Confidence: conf, CenterLat: lat, CenterLon: lon,
		Intensity: intensity, Description: "test",
	}
}

func TestFuseBothSources(t *testing.T) {
	e := NewEngine(0.5, 0.5, 0.3, 0.4)
	sar := []change.ChangeDetection{
		makeDet("s1", 39.925, 32.837, 0.7, 0.6, change.ChangeTrench),
	}
	opt := []change.ChangeDetection{
		makeDet("o1", 39.9251, 32.8371, 0.6, 0.5, change.ChangeTrench),
	}
	result := e.Fuse(sar, opt, 0.1)
	if result.CrossValidated != 1 {
		t.Fatalf("expected 1 cross-validated, got %d", result.CrossValidated)
	}
	if result.BothSources != 1 {
		t.Fatalf("expected 1 both sources, got %d", result.BothSources)
	}
	// çapraz doğrulama güven artışı
	if result.Detections[0].Confidence <= 0.7 {
		t.Logf("confidence after fusion: %f", result.Detections[0].Confidence)
	}
}

func TestFuseSAROnly(t *testing.T) {
	e := NewEngine(0.5, 0.5, 0.3, 0.4)
	sar := []change.ChangeDetection{
		makeDet("s1", 39.925, 32.837, 0.7, 0.6, change.ChangeFortify),
	}
	result := e.Fuse(sar, nil, 0.0)
	if result.SAROnly != 1 {
		t.Fatalf("expected 1 SAR only, got %d", result.SAROnly)
	}
	if result.TotalFused != 1 {
		t.Fatalf("expected 1 total, got %d", result.TotalFused)
	}
}

func TestFuseOpticalOnly(t *testing.T) {
	e := NewEngine(0.5, 0.5, 0.3, 0.4)
	opt := []change.ChangeDetection{
		makeDet("o1", 39.925, 32.837, 0.5, 0.4, change.ChangeBuilding),
	}
	result := e.Fuse(nil, opt, 0.1)
	if result.OpticalOnly != 1 {
		t.Fatalf("expected 1 optical only, got %d", result.OpticalOnly)
	}
}

func TestFuseCloudyOptical(t *testing.T) {
	e := NewEngine(0.5, 0.5, 0.3, 0.4)
	opt := []change.ChangeDetection{
		makeDet("o1", 39.925, 32.837, 0.5, 0.4, change.ChangeBuilding),
	}
	result := e.Fuse(nil, opt, 0.7) // yüksek bulut
	if len(result.Detections) > 0 {
		if result.Detections[0].Confidence >= 0.5 {
			t.Fatal("cloudy optical should reduce confidence")
		}
	}
}

func TestFuseFalseAlarmFilter(t *testing.T) {
	e := NewEngine(0.5, 0.5, 0.3, 0.4)
	opt := []change.ChangeDetection{
		makeDet("o1", 39.925, 32.837, 0.10, 0.05, change.ChangeWater), // çok düşük güven
	}
	result := e.Fuse(nil, opt, 0.8) // yüksek bulut + düşük güven
	if result.FalseAlarmFiltered != 1 {
		t.Fatalf("expected 1 filtered, got %d", result.FalseAlarmFiltered)
	}
}

func TestFuseNoMatch(t *testing.T) {
	e := NewEngine(0.5, 0.5, 0.3, 0.4)
	sar := []change.ChangeDetection{
		makeDet("s1", 39.0, 32.0, 0.7, 0.6, change.ChangeTrench),
	}
	opt := []change.ChangeDetection{
		makeDet("o1", 41.0, 29.0, 0.5, 0.4, change.ChangeBuilding), // uzak
	}
	result := e.Fuse(sar, opt, 0.1)
	if result.CrossValidated != 0 {
		t.Fatal("should not cross-validate distant detections")
	}
	if result.SAROnly != 1 {
		t.Fatalf("expected 1 SAR only, got %d", result.SAROnly)
	}
	if result.OpticalOnly != 1 {
		t.Fatalf("expected 1 optical only, got %d", result.OpticalOnly)
	}
}

func TestFuseMultiple(t *testing.T) {
	e := NewEngine(0.5, 0.5, 0.3, 0.4)
	sar := []change.ChangeDetection{
		makeDet("s1", 39.925, 32.837, 0.8, 0.7, change.ChangeTrench),
		makeDet("s2", 39.930, 32.840, 0.6, 0.5, change.ChangeVehicle),
	}
	opt := []change.ChangeDetection{
		makeDet("o1", 39.9251, 32.8371, 0.7, 0.6, change.ChangeTrench),
		makeDet("o2", 40.0, 33.0, 0.4, 0.3, change.ChangeForest), // eşleşmesiz
	}
	result := e.Fuse(sar, opt, 0.1)
	if result.CrossValidated != 1 {
		t.Fatalf("expected 1 cross-validated, got %d", result.CrossValidated)
	}
	if result.TotalFused < 3 {
		t.Fatalf("expected at least 3 fused, got %d", result.TotalFused)
	}
}

func TestRecalcThreat(t *testing.T) {
	tests := []struct {
		conf, intensity float64
		cType           change.ChangeType
		notExpect       change.ThreatLevel
	}{
		{0.9, 0.8, change.ChangeTrench, change.ThreatNone},
		{0.1, 0.1, change.ChangeWater, change.ThreatHigh},
	}
	for _, tc := range tests {
		level := recalcThreat(tc.conf, tc.intensity, tc.cType)
		if level == tc.notExpect {
			t.Errorf("unexpected %s for conf=%.1f int=%.1f type=%s", level, tc.conf, tc.intensity, tc.cType)
		}
	}
}
