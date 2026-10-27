package resource

import (
	"testing"
)

func TestNewPlatformResources(t *testing.T) {
	pr := NewPlatformResources("test-01", 100, nil)
	if pr.PlatformID != "test-01" {
		t.Fatal("wrong id")
	}
	if pr.MaxHistory != 100 {
		t.Fatal("wrong max history")
	}
}

func TestRecordMetric_CPU(t *testing.T) {
	pr := NewPlatformResources("p1", 50, nil)
	m := pr.RecordMetric(ResCPU, 45.0, "%")
	if m.Level != LevelNormal {
		t.Fatalf("45%% CPU should be normal, got %s", m.Level)
	}
	if pr.CPU != 45.0 {
		t.Fatal("CPU not updated")
	}
}

func TestRecordMetric_Warning(t *testing.T) {
	pr := NewPlatformResources("p2", 50, nil)
	m := pr.RecordMetric(ResCPU, 75.0, "%")
	if m.Level != LevelWarning {
		t.Fatalf("75%% CPU should be warning, got %s", m.Level)
	}
}

func TestRecordMetric_Critical(t *testing.T) {
	pr := NewPlatformResources("p3", 50, nil)
	m := pr.RecordMetric(ResMemory, 95.0, "%")
	if m.Level != LevelCritical {
		t.Fatalf("95%% Memory should be critical, got %s", m.Level)
	}
}

func TestRecordMetric_Temperature(t *testing.T) {
	pr := NewPlatformResources("p4", 50, nil)
	m := pr.RecordMetric(ResTemp, 82.0, "°C")
	if m.Level != LevelCritical {
		t.Fatalf("82°C should be critical, got %s", m.Level)
	}
}

func TestRecordMetric_History(t *testing.T) {
	pr := NewPlatformResources("p5", 3, nil)
	pr.RecordMetric(ResCPU, 10, "%")
	pr.RecordMetric(ResCPU, 20, "%")
	pr.RecordMetric(ResCPU, 30, "%")
	pr.RecordMetric(ResCPU, 40, "%") // should trim oldest
	hist := pr.History[ResCPU]
	if len(hist) != 3 {
		t.Fatalf("history len = %d, want 3", len(hist))
	}
	if hist[0].Value != 20 {
		t.Fatalf("first value = %f, want 20", hist[0].Value)
	}
}

func TestGetSnapshot(t *testing.T) {
	pr := NewPlatformResources("snap", 50, nil)
	pr.RecordMetric(ResCPU, 85.0, "%")
	pr.RecordMetric(ResMemory, 60.0, "%")
	pr.RecordMetric(ResTemp, 50.0, "°C")
	snap := pr.GetSnapshot()
	if snap.CPULevel != LevelWarning {
		t.Fatalf("CPU level = %s, want warning", snap.CPULevel)
	}
	if snap.MemLevel != LevelNormal {
		t.Fatalf("Mem level = %s, want normal", snap.MemLevel)
	}
}

func TestTracker_GetOrCreate(t *testing.T) {
	tr := NewTracker(100, nil)
	pr := tr.GetOrCreate("plat-1")
	if pr.PlatformID != "plat-1" {
		t.Fatal("wrong id")
	}
	// Second call should return same
	pr2 := tr.GetOrCreate("plat-1")
	if pr2 != pr {
		t.Fatal("should return same instance")
	}
	if tr.Count() != 1 {
		t.Fatal("count should be 1")
	}
}

func TestTracker_Remove(t *testing.T) {
	tr := NewTracker(100, nil)
	tr.GetOrCreate("a")
	if !tr.Remove("a") {
		t.Fatal("should remove")
	}
	if tr.Remove("nonexist") {
		t.Fatal("should return false")
	}
}

func TestTracker_Stats(t *testing.T) {
	tr := NewTracker(100, nil)
	pr1 := tr.GetOrCreate("p1")
	pr1.RecordMetric(ResCPU, 80.0, "%")
	pr2 := tr.GetOrCreate("p2")
	pr2.RecordMetric(ResCPU, 40.0, "%")

	stats := tr.GetStats()
	if stats.TotalPlatforms != 2 {
		t.Fatal("total != 2")
	}
	if stats.AvgCPU < 59 || stats.AvgCPU > 61 {
		t.Fatalf("avg CPU = %f, want ~60", stats.AvgCPU)
	}
	if stats.MaxCPU != 80.0 {
		t.Fatalf("max CPU = %f, want 80", stats.MaxCPU)
	}
}

func TestTracker_CriticalResources(t *testing.T) {
	tr := NewTracker(100, nil)
	pr := tr.GetOrCreate("c1")
	pr.RecordMetric(ResCPU, 95.0, "%")
	pr.RecordMetric(ResTemp, 85.0, "°C")
	crits := tr.CriticalResources()
	if len(crits) != 2 {
		t.Fatalf("critical count = %d, want 2", len(crits))
	}
}

func TestTracker_HistoryForPlatform(t *testing.T) {
	tr := NewTracker(100, nil)
	pr := tr.GetOrCreate("h1")
	pr.RecordMetric(ResCPU, 10, "%")
	pr.RecordMetric(ResCPU, 20, "%")
	pr.RecordMetric(ResCPU, 30, "%")

	hist, err := tr.HistoryForPlatform("h1", ResCPU, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 2 {
		t.Fatalf("hist len = %d, want 2", len(hist))
	}

	_, err = tr.HistoryForPlatform("missing", ResCPU, 10)
	if err == nil {
		t.Fatal("should error for missing platform")
	}
}

func TestDefaultThresholds(t *testing.T) {
	th := DefaultThresholds()
	if _, ok := th[ResCPU]; !ok {
		t.Fatal("missing CPU thresholds")
	}
	if th[ResCPU].Warning != 70 || th[ResCPU].Critical != 90 {
		t.Fatal("wrong CPU thresholds")
	}
}
