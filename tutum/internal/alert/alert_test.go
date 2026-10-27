package alert

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 30*time.Second, true)
	if m.maxAlerts != 100 {
		t.Fatal("wrong max")
	}
	if !m.autoRTB {
		t.Fatal("autoRTB should be true")
	}
}

func TestGenerate(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 0, false) // dedup=0 → no dedup
	a := m.Generate("iha-01", SevCritical, CatBattery, "Test", "Test msg", 5.0, 10.0)
	if a == nil {
		t.Fatal("should generate alert")
	}
	if a.Severity != SevCritical {
		t.Fatal("wrong severity")
	}
	if a.PlatformID != "iha-01" {
		t.Fatal("wrong platform")
	}
}

func TestGenerate_Dedup(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 1*time.Hour, false)
	a1 := m.Generate("iha-01", SevCritical, CatBattery, "Test", "msg1", 5, 10)
	a2 := m.Generate("iha-01", SevCritical, CatBattery, "Test", "msg2", 5, 10)
	if a1 == nil {
		t.Fatal("first should generate")
	}
	if a2 != nil {
		t.Fatal("second should be deduped")
	}
}

func TestGenerate_DifferentCategory(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 1*time.Hour, false)
	a1 := m.Generate("iha-01", SevCritical, CatBattery, "B", "msg", 5, 10)
	a2 := m.Generate("iha-01", SevWarning, CatTemperature, "T", "msg", 70, 65)
	if a1 == nil || a2 == nil {
		t.Fatal("different categories should both generate")
	}
}

func TestCheckBattery_Critical(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 0, true)
	a := m.CheckBattery("iha-01", 5.0, 10.0, 25.0, 40.0)
	if a == nil {
		t.Fatal("should generate critical alert")
	}
	if a.Severity != SevCritical {
		t.Fatalf("severity = %s, want critical", a.Severity)
	}
	// RTB recommendation should exist
	recs := m.GetRTBRecommendations()
	if _, ok := recs["iha-01"]; !ok {
		t.Fatal("should have RTB recommendation")
	}
}

func TestCheckBattery_Warning(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 0, false)
	a := m.CheckBattery("r-01", 20.0, 10.0, 25.0, 40.0)
	if a == nil || a.Severity != SevWarning {
		t.Fatal("should be warning")
	}
}

func TestCheckBattery_OK(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 0, false)
	a := m.CheckBattery("ok", 80.0, 10.0, 25.0, 40.0)
	if a != nil {
		t.Fatal("should not alert for 80%")
	}
}

func TestCheckPrediction_Critical(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 0, true)
	a := m.CheckPrediction("iha-01", 5.0, false)
	if a == nil || a.Severity != SevCritical {
		t.Fatal("should be critical for < 10 min")
	}
}

func TestCheckPrediction_Warning(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 0, false)
	a := m.CheckPrediction("iha-01", 20.0, true)
	if a == nil || a.Severity != SevWarning {
		t.Fatal("should be warning for < 30 min")
	}
}

func TestCheckTemperature(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 0, false)
	a := m.CheckTemperature("iha-01", 85.0, 65.0, 80.0)
	if a == nil || a.Severity != SevCritical {
		t.Fatal("85°C should be critical")
	}
	m2 := NewManager(100, 24*time.Hour, 0, false)
	a2 := m2.CheckTemperature("iha-01", 70.0, 65.0, 80.0)
	if a2 == nil || a2.Severity != SevWarning {
		t.Fatal("70°C should be warning")
	}
}

func TestAcknowledge(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 0, false)
	a := m.Generate("x", SevInfo, CatMission, "T", "msg", 0, 0)
	if !m.Acknowledge(a.ID) {
		t.Fatal("should ack")
	}
	if m.Acknowledge(a.ID) {
		t.Fatal("double ack should return false")
	}
}

func TestResolve(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 0, false)
	a := m.Generate("x", SevInfo, CatMission, "T", "msg", 0, 0)
	if !m.Resolve(a.ID) {
		t.Fatal("should resolve")
	}
	active := m.Active()
	if len(active) != 0 {
		t.Fatal("should have no active alerts")
	}
}

func TestActive_BySeverity_ByPlatform(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 0, false)
	m.Generate("a", SevCritical, CatBattery, "A", "msg", 0, 0)
	m.Generate("b", SevWarning, CatResource, "B", "msg", 0, 0)
	m.Generate("a", SevInfo, CatMission, "C", "msg", 0, 0)

	active := m.Active()
	if len(active) != 3 {
		t.Fatalf("active = %d, want 3", len(active))
	}

	crits := m.BySeverity(SevCritical)
	if len(crits) != 1 {
		t.Fatalf("critical = %d, want 1", len(crits))
	}

	platA := m.ByPlatform("a")
	if len(platA) != 2 {
		t.Fatalf("platform a = %d, want 2", len(platA))
	}
}

func TestHandoffRecommendation(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 0, false)
	rec := m.RecommendHandoff("iha-01", 8.0, 12.0, "Enerji yetersiz")
	if rec.FromPlatformID != "iha-01" {
		t.Fatal("wrong platform")
	}
	recs := m.GetHandoffRecommendations()
	if len(recs) != 1 {
		t.Fatal("should have 1 handoff rec")
	}
}

func TestClearRTB(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 0, true)
	m.CheckBattery("iha-01", 5.0, 10.0, 25.0, 40.0)
	m.ClearRTB("iha-01")
	recs := m.GetRTBRecommendations()
	if len(recs) != 0 {
		t.Fatal("should be cleared")
	}
}

func TestStats(t *testing.T) {
	m := NewManager(100, 24*time.Hour, 0, false)
	m.Generate("a", SevCritical, CatBattery, "A", "msg", 0, 0)
	m.Generate("b", SevWarning, CatResource, "B", "msg", 0, 0)
	s := m.GetStats()
	if s.TotalGenerated != 2 {
		t.Fatalf("generated = %d, want 2", s.TotalGenerated)
	}
	if s.CriticalCount != 1 {
		t.Fatalf("critical = %d, want 1", s.CriticalCount)
	}
	if s.ActiveCount != 2 {
		t.Fatalf("active = %d, want 2", s.ActiveCount)
	}
}

func TestCleanup(t *testing.T) {
	m := NewManager(100, 1*time.Millisecond, 0, false)
	a := m.Generate("x", SevInfo, CatMission, "T", "msg", 0, 0)
	m.Resolve(a.ID)
	time.Sleep(5 * time.Millisecond)
	removed := m.Cleanup()
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
}

func TestTrimAlerts(t *testing.T) {
	m := NewManager(3, 24*time.Hour, 0, false)
	for i := 0; i < 5; i++ {
		m.Generate("x", SevInfo, CatMission, "T", "msg", 0, 0)
	}
	active := m.Active()
	if len(active) > 3 {
		t.Fatalf("active = %d, should be <= 3", len(active))
	}
}
