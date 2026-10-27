package battery

import (
	"testing"
	"time"
)

func TestNewPlatform(t *testing.T) {
	p := NewPlatform("iha-01", "Kartal-1", PlatformDrone, 500.0)
	if p.SoCPercent != 100.0 {
		t.Fatalf("initial SoC = %f, want 100", p.SoCPercent)
	}
	if p.Status != StatusFull {
		t.Fatalf("initial status = %s, want full", p.Status)
	}
	if p.RemainingWh != 500.0 {
		t.Fatalf("remaining = %f, want 500", p.RemainingWh)
	}
}

func TestRecordSample_Drain(t *testing.T) {
	p := NewPlatform("iha-02", "Kartal-2", PlatformDrone, 100.0)
	// Simülasyon: 10V, -5A, 25°C → 50W tüketim
	// Ama LastUpdate=now, dt≈0 → ilk ölçüm az etkili
	time.Sleep(10 * time.Millisecond)
	s := p.RecordSample(10.0, -5.0, 25.0)
	if s.PowerW <= 0 {
		t.Fatal("power should be positive")
	}
	if p.SoCPercent > 100.0 {
		t.Fatal("SoC shouldn't exceed 100")
	}
}

func TestRecordSample_CriticalLevel(t *testing.T) {
	p := NewPlatform("radyo-01", "TR-01", PlatformRadio, 50.0)
	p.RemainingWh = 4.0 // %8 → kritik
	p.SoCPercent = 8.0
	p.LastUpdate = time.Now().Add(-1 * time.Minute)
	p.RecordSample(3.7, -0.5, 25.0)
	if p.Status != StatusCritical && p.Status != StatusDead {
		t.Fatalf("status = %s, want critical or dead", p.Status)
	}
}

func TestWindFactor(t *testing.T) {
	tests := []struct {
		wind float64
		min  float64
		max  float64
	}{
		{0, 1.0, 1.0},
		{3, 1.0, 1.1},
		{5, 1.08, 1.12},
		{15, 1.1, 2.0},
	}
	for _, tc := range tests {
		f := WindFactor(tc.wind)
		if f < tc.min || f > tc.max {
			t.Errorf("WindFactor(%f) = %f, want [%f, %f]", tc.wind, f, tc.min, tc.max)
		}
	}
}

func TestTerrainFactors(t *testing.T) {
	p := NewPlatform("ugv-01", "Kurt-1", PlatformUGV, 2000.0)
	p.UpdateConditions(TerrainMountain, 10.0, 1.5)
	mult := p.EffectiveDischargeMultiplier()
	// Dağlık(1.70) × Rüzgar(~1.23) × Yük(1.5) = ~3.13
	if mult < 2.5 || mult > 4.0 {
		t.Fatalf("multiplier = %f, want [2.5, 4.0]", mult)
	}
}

func TestTempDegradation(t *testing.T) {
	if f := TempDegradation(25); f != 1.0 {
		t.Fatalf("25°C factor = %f, want 1.0", f)
	}
	if f := TempDegradation(-5); f >= 1.0 || f <= 0.5 {
		t.Fatalf("-5°C factor = %f, want (0.5, 1.0)", f)
	}
	if f := TempDegradation(45); f >= 1.0 || f <= 0.7 {
		t.Fatalf("45°C factor = %f, want (0.7, 1.0)", f)
	}
}

func TestMonitor_RegisterAndGet(t *testing.T) {
	m := NewMonitor(10)
	p := NewPlatform("iha-03", "Kartal-3", PlatformDrone, 300.0)
	if err := m.Register(p); err != nil {
		t.Fatal(err)
	}
	if m.Count() != 1 {
		t.Fatal("count should be 1")
	}
	got, ok := m.Get("iha-03")
	if !ok || got.ID != "iha-03" {
		t.Fatal("should find platform")
	}
}

func TestMonitor_MaxPlatforms(t *testing.T) {
	m := NewMonitor(2)
	m.Register(NewPlatform("a", "A", PlatformDrone, 100))
	m.Register(NewPlatform("b", "B", PlatformDrone, 100))
	err := m.Register(NewPlatform("c", "C", PlatformDrone, 100))
	if err == nil {
		t.Fatal("should reject when full")
	}
}

func TestMonitor_Stats(t *testing.T) {
	m := NewMonitor(10)
	p1 := NewPlatform("a", "A", PlatformDrone, 100)
	p2 := NewPlatform("b", "B", PlatformRadio, 50)
	p2.RemainingWh = 5
	p2.SoCPercent = 10
	p2.Status = StatusCritical
	m.Register(p1)
	m.Register(p2)

	s := m.GetStats()
	if s.TotalPlatforms != 2 {
		t.Fatalf("total = %d, want 2", s.TotalPlatforms)
	}
	if s.CriticalCount != 1 {
		t.Fatalf("critical = %d, want 1", s.CriticalCount)
	}
	if s.MinSoCPercent > 10.0 {
		t.Fatalf("min soc = %f, want <= 10", s.MinSoCPercent)
	}
}

func TestMonitor_CriticalPlatforms(t *testing.T) {
	m := NewMonitor(10)
	p1 := NewPlatform("ok", "OK", PlatformDrone, 100)
	p2 := NewPlatform("bad", "BAD", PlatformDrone, 100)
	p2.SoCPercent = 5
	p2.Status = StatusCritical
	m.Register(p1)
	m.Register(p2)
	crits := m.CriticalPlatforms()
	if len(crits) != 1 || crits[0].ID != "bad" {
		t.Fatalf("expected 1 critical platform 'bad', got %v", crits)
	}
}

func TestMonitor_Remove(t *testing.T) {
	m := NewMonitor(10)
	m.Register(NewPlatform("x", "X", PlatformRelay, 100))
	if !m.Remove("x") {
		t.Fatal("should remove")
	}
	if m.Count() != 0 {
		t.Fatal("count should be 0")
	}
	if m.Remove("nonexist") {
		t.Fatal("should return false for missing")
	}
}

func TestEstimateRemainingMinutes(t *testing.T) {
	p := NewPlatform("test", "T", PlatformDrone, 100)
	p.DischargeRate = 2.0 // 2 Wh/dk
	p.RemainingWh = 40.0
	mins := p.EstimateRemainingMinutes()
	if mins < 19 || mins > 21 {
		t.Fatalf("estimate = %f, want ~20", mins)
	}
}

func TestSnapshot(t *testing.T) {
	p := NewPlatform("snap", "S", PlatformSensor, 200)
	p.UpdateConditions(TerrainHilly, 5.0, 1.2)
	snap := p.Snapshot()
	if snap.ID != "snap" {
		t.Fatal("wrong id")
	}
	if snap.EffMultiplier <= 1.0 {
		t.Fatal("multiplier should be > 1 for hilly terrain + wind")
	}
}
