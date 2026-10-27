package classifier

import (
	"testing"
	"time"

	"github.com/otukenmatris/tani/internal/detector"
)

func makeDet(id string, cls detector.ObjectClass, conf float64) detector.Detection {
	return detector.Detection{
		ID:         id,
		Class:      cls,
		Confidence: conf,
		BBox:       detector.BoundingBox{X: 10, Y: 10, Width: 50, Height: 50},
		Timestamp:  time.Now(),
		FrameID:    "f1",
	}
}

func TestNewEngine(t *testing.T) {
	e := NewEngine(true, 0.4)
	if e == nil {
		t.Fatal("NewEngine() nil döndü")
	}
	stats := e.GetStats()
	if !stats.IFFEnabled {
		t.Error("IFFEnabled = false, want true")
	}
}

func TestClassify_Tank(t *testing.T) {
	e := NewEngine(true, 0.3)
	det := makeDet("d1", detector.ClassTank, 0.9)

	obj := e.Classify(det)
	if obj.Class != detector.ClassTank {
		t.Errorf("Class = %s, want tank", obj.Class)
	}
	if obj.Category != CatVehicle {
		t.Errorf("Category = %s, want vehicle", obj.Category)
	}
	if obj.ThreatScore <= 0 {
		t.Errorf("ThreatScore = %f, pozitif olmalı", obj.ThreatScore)
	}
}

func TestClassify_Civilian(t *testing.T) {
	e := NewEngine(true, 0.3)
	det := makeDet("d2", detector.ClassCivilian, 0.8)

	obj := e.Classify(det)
	if obj.Allegiance != AllegianceNeutral {
		t.Errorf("Civilian Allegiance = %s, want neutral", obj.Allegiance)
	}
	if obj.ThreatLevel == ThreatCritical || obj.ThreatLevel == ThreatHigh {
		t.Errorf("Civilian ThreatLevel = %s, yüksek olmamalı", obj.ThreatLevel)
	}
}

func TestClassify_IFFDisabled(t *testing.T) {
	e := NewEngine(false, 0.3)
	det := makeDet("d3", detector.ClassSoldier, 0.9)

	obj := e.Classify(det)
	if obj.Allegiance != AllegianceUnknown {
		t.Errorf("IFF kapalı: Allegiance = %s, want unknown", obj.Allegiance)
	}
}

func TestClassify_FriendlyTransponder(t *testing.T) {
	e := NewEngine(true, 0.3)
	e.RegisterFriendly("TR-001")

	det := makeDet("d4", detector.ClassTank, 0.9)
	det.Metadata = map[string]any{"transponder_id": "TR-001"}

	obj := e.Classify(det)
	if obj.Allegiance != AllegianceFriendly {
		t.Errorf("Friendly transponder: Allegiance = %s, want friendly", obj.Allegiance)
	}
	if obj.ThreatScore > 0.1 {
		t.Errorf("Dost tehdit skoru düşük olmalı: %f", obj.ThreatScore)
	}
}

func TestClassify_HostileFromIFFResponse(t *testing.T) {
	e := NewEngine(true, 0.3)

	det := makeDet("d5", detector.ClassAPC, 0.8)
	det.Metadata = map[string]any{"iff_response": "hostile"}

	obj := e.Classify(det)
	if obj.Allegiance != AllegianceHostile {
		t.Errorf("IFF hostile: Allegiance = %s, want hostile", obj.Allegiance)
	}
}

func TestClassifyBatch(t *testing.T) {
	e := NewEngine(true, 0.5)

	dets := []detector.Detection{
		makeDet("d1", detector.ClassTank, 0.9),
		makeDet("d2", detector.ClassSoldier, 0.3), // minConf altında, atlanmalı
		makeDet("d3", detector.ClassDrone, 0.7),
	}

	results := e.ClassifyBatch(dets)
	if len(results) != 2 {
		t.Errorf("ClassifyBatch = %d, want 2 (low conf filtered)", len(results))
	}
}

func TestRegisterUnregisterFriendly(t *testing.T) {
	e := NewEngine(true, 0.3)
	e.RegisterFriendly("A-001")

	det := makeDet("d1", detector.ClassTank, 0.9)
	det.Metadata = map[string]any{"transponder_id": "A-001"}
	obj := e.Classify(det)
	if obj.Allegiance != AllegianceFriendly {
		t.Error("Kayıtlı dost bulunamadı")
	}

	e.UnregisterFriendly("A-001")
	obj2 := e.Classify(det)
	if obj2.Allegiance == AllegianceFriendly {
		t.Error("Silinen dost hala friendly dönüyor")
	}
}

func TestThreatLevels(t *testing.T) {
	tests := []struct {
		score float64
		want  ThreatLevel
	}{
		{0.95, ThreatCritical},
		{0.80, ThreatCritical},
		{0.70, ThreatHigh},
		{0.60, ThreatHigh},
		{0.45, ThreatMedium},
		{0.35, ThreatMedium},
		{0.20, ThreatLow},
		{0.10, ThreatLow},
		{0.05, ThreatNone},
		{0.00, ThreatNone},
	}
	for _, tt := range tests {
		got := scoreThreatLevel(tt.score)
		if got != tt.want {
			t.Errorf("scoreThreatLevel(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestStats(t *testing.T) {
	e := NewEngine(true, 0.3)

	e.Classify(makeDet("d1", detector.ClassTank, 0.9))
	e.Classify(makeDet("d2", detector.ClassCivilian, 0.8))
	e.Classify(makeDet("d3", detector.ClassTank, 0.7))

	stats := e.GetStats()
	if stats.TotalClassified != 3 {
		t.Errorf("TotalClassified = %d, want 3", stats.TotalClassified)
	}
	if stats.NeutralCount < 1 {
		t.Errorf("NeutralCount = %d, want >= 1 (civilian)", stats.NeutralCount)
	}
	if stats.ClassCounts["tank"] != 2 {
		t.Errorf("ClassCounts[tank] = %d, want 2", stats.ClassCounts["tank"])
	}
	if stats.CategoryCounts["vehicle"] != 2 {
		t.Errorf("CategoryCounts[vehicle] = %d, want 2", stats.CategoryCounts["vehicle"])
	}
}

func TestDescribe(t *testing.T) {
	e := NewEngine(true, 0.3)
	desc := e.describe(detector.ClassTank, AllegianceHostile)
	if desc == "" {
		t.Error("describe() boş string döndü")
	}
}

func TestClassMapping(t *testing.T) {
	// Her sınıfın kategori eşlemesi olmalı
	for _, cls := range detector.AllClasses {
		if _, ok := classMapping[cls]; !ok {
			t.Errorf("classMapping[%s] tanımsız", cls)
		}
	}
}

func TestBaseThreat(t *testing.T) {
	for _, cls := range detector.AllClasses {
		if _, ok := baseThreat[cls]; !ok {
			t.Errorf("baseThreat[%s] tanımsız", cls)
		}
	}
}
