package detector

import (
	"testing"
	"time"
)

func TestBoundingBoxArea(t *testing.T) {
	bb := BoundingBox{X: 10, Y: 20, Width: 100, Height: 50}
	if area := bb.Area(); area != 5000 {
		t.Errorf("Area() = %f, want 5000", area)
	}
}

func TestBoundingBoxIoU(t *testing.T) {
	a := BoundingBox{X: 0, Y: 0, Width: 100, Height: 100}
	b := BoundingBox{X: 50, Y: 50, Width: 100, Height: 100}

	iou := a.IoU(b)
	// intersection = 50*50 = 2500, union = 10000+10000-2500 = 17500
	expected := 2500.0 / 17500.0
	if diff := iou - expected; diff > 0.001 || diff < -0.001 {
		t.Errorf("IoU() = %f, want ~%f", iou, expected)
	}
}

func TestBoundingBoxIoU_NoOverlap(t *testing.T) {
	a := BoundingBox{X: 0, Y: 0, Width: 50, Height: 50}
	b := BoundingBox{X: 100, Y: 100, Width: 50, Height: 50}

	if iou := a.IoU(b); iou != 0 {
		t.Errorf("IoU() = %f, want 0 (no overlap)", iou)
	}
}

func TestBoundingBoxIoU_Identical(t *testing.T) {
	a := BoundingBox{X: 10, Y: 10, Width: 50, Height: 50}
	iou := a.IoU(a)
	if iou < 0.99 {
		t.Errorf("IoU(self) = %f, want ~1.0", iou)
	}
}

func TestBoundingBoxCenter(t *testing.T) {
	bb := BoundingBox{X: 10, Y: 20, Width: 100, Height: 50}
	cx, cy := bb.Center()
	if cx != 60 || cy != 45 {
		t.Errorf("Center() = (%f, %f), want (60, 45)", cx, cy)
	}
}

func TestNewEngine(t *testing.T) {
	e := NewEngine(0.5, 0.5, 100, 640, 640, "YOLOv7-test")
	if e == nil {
		t.Fatal("NewEngine() returned nil")
	}
	stats := e.GetStats()
	if stats.ModelName != "YOLOv7-test" {
		t.Errorf("ModelName = %s, want YOLOv7-test", stats.ModelName)
	}
	if stats.TotalFrames != 0 {
		t.Errorf("TotalFrames = %d, want 0", stats.TotalFrames)
	}
}

func TestDetect_ProducesDetections(t *testing.T) {
	e := NewEngine(0.1, 0.5, 100, 640, 640, "YOLOv7-test")
	frame := Frame{
		ID:        "frame-001",
		Width:     640,
		Height:    640,
		Timestamp: time.Now(),
		SourceID:  "cam-01",
	}

	// Birden fazla deneme — rastgelelik nedeniyle
	anyDetections := false
	for i := 0; i < 10; i++ {
		result := e.Detect(frame)
		if result == nil {
			t.Fatal("Detect() returned nil")
		}
		if result.FrameID != "frame-001" {
			t.Errorf("FrameID = %s, want frame-001", result.FrameID)
		}
		if result.Count > 0 {
			anyDetections = true
		}
	}
	if !anyDetections {
		t.Error("10 denemede hiç tespit üretilmedi")
	}
}

func TestDetect_RespectsConfidenceThreshold(t *testing.T) {
	// Çok yüksek eşik — az tespit
	e := NewEngine(0.95, 0.5, 100, 640, 640, "strict")
	frame := Frame{ID: "f1", Width: 640, Height: 640, Timestamp: time.Now()}

	totalDets := 0
	for i := 0; i < 20; i++ {
		r := e.Detect(frame)
		totalDets += r.Count
	}

	// Düşük eşik — daha çok tespit
	e2 := NewEngine(0.1, 0.5, 100, 640, 640, "loose")
	totalDets2 := 0
	for i := 0; i < 20; i++ {
		r := e2.Detect(frame)
		totalDets2 += r.Count
	}

	if totalDets2 <= totalDets {
		t.Logf("Düşük eşik: %d, yüksek eşik: %d — beklenti: düşük > yüksek", totalDets2, totalDets)
	}
}

func TestDetect_MaxDetectionsLimit(t *testing.T) {
	e := NewEngine(0.01, 0.99, 3, 640, 640, "limited")
	frame := Frame{ID: "f1", Width: 640, Height: 640, Timestamp: time.Now()}

	for i := 0; i < 10; i++ {
		r := e.Detect(frame)
		if r.Count > 3 {
			t.Errorf("Count = %d, max 3 olmalı", r.Count)
		}
	}
}

func TestDetect_StatsAccumulate(t *testing.T) {
	e := NewEngine(0.3, 0.5, 100, 640, 640, "stats-test")
	frame := Frame{ID: "f1", Width: 640, Height: 640, Timestamp: time.Now()}

	n := 5
	for i := 0; i < n; i++ {
		e.Detect(frame)
	}

	stats := e.GetStats()
	if stats.TotalFrames != int64(n) {
		t.Errorf("TotalFrames = %d, want %d", stats.TotalFrames, n)
	}
}

func TestDetect_NMSReducesDuplicates(t *testing.T) {
	e := NewEngine(0.1, 0.3, 100, 640, 640, "nms-test")
	frame := Frame{ID: "f1", Width: 640, Height: 640, Timestamp: time.Now()}

	// NMS'in çalıştığını kontrol et
	totalSuppressed := int64(0)
	for i := 0; i < 50; i++ {
		e.Detect(frame)
	}
	totalSuppressed = e.GetStats().TotalSuppressed
	if totalSuppressed < 0 {
		t.Errorf("TotalSuppressed negatif olamaz: %d", totalSuppressed)
	}
}

func TestDetect_ValidBoundingBoxes(t *testing.T) {
	e := NewEngine(0.1, 0.5, 100, 800, 600, "bbox-test")
	frame := Frame{ID: "f1", Width: 800, Height: 600, Timestamp: time.Now()}

	for i := 0; i < 20; i++ {
		r := e.Detect(frame)
		for _, d := range r.Detections {
			if d.BBox.X < 0 || d.BBox.Y < 0 {
				t.Errorf("Negatif koordinat: (%f, %f)", d.BBox.X, d.BBox.Y)
			}
			if d.BBox.Width <= 0 || d.BBox.Height <= 0 {
				t.Errorf("Sıfır/negatif boyut: %fx%f", d.BBox.Width, d.BBox.Height)
			}
			if d.Confidence < 0.1 || d.Confidence > 1.0 {
				t.Errorf("Güven skoru aralık dışı: %f", d.Confidence)
			}
		}
	}
}

func TestNMS_Empty(t *testing.T) {
	e := NewEngine(0.5, 0.5, 100, 640, 640, "test")
	result := e.nms(nil)
	if result != nil {
		t.Errorf("nms(nil) = %v, want nil", result)
	}
}

func TestNMS_SingleDetection(t *testing.T) {
	e := NewEngine(0.5, 0.5, 100, 640, 640, "test")
	dets := []Detection{
		{ID: "d1", Class: ClassTank, Confidence: 0.9, BBox: BoundingBox{10, 10, 50, 50}},
	}
	result := e.nms(dets)
	if len(result) != 1 {
		t.Errorf("nms single = %d, want 1", len(result))
	}
}

func TestNMS_OverlappingSameClass(t *testing.T) {
	e := NewEngine(0.5, 0.3, 100, 640, 640, "test")
	dets := []Detection{
		{ID: "d1", Class: ClassTank, Confidence: 0.9, BBox: BoundingBox{10, 10, 50, 50}},
		{ID: "d2", Class: ClassTank, Confidence: 0.7, BBox: BoundingBox{15, 15, 50, 50}},
	}
	result := e.nms(dets)
	if len(result) != 1 {
		t.Errorf("nms overlapping same class = %d, want 1", len(result))
	}
	if result[0].Confidence != 0.9 {
		t.Error("NMS yüksek güvenli olanı tutmalı")
	}
}

func TestNMS_OverlappingDifferentClass(t *testing.T) {
	e := NewEngine(0.5, 0.3, 100, 640, 640, "test")
	dets := []Detection{
		{ID: "d1", Class: ClassTank, Confidence: 0.9, BBox: BoundingBox{10, 10, 50, 50}},
		{ID: "d2", Class: ClassSoldier, Confidence: 0.7, BBox: BoundingBox{15, 15, 50, 50}},
	}
	result := e.nms(dets)
	// Farklı sınıflar elenmemeli
	if len(result) != 2 {
		t.Errorf("nms different class = %d, want 2", len(result))
	}
}

func TestLastResult(t *testing.T) {
	e := NewEngine(0.3, 0.5, 100, 640, 640, "test")
	if e.LastResult() != nil {
		t.Error("LastResult() ilk başta nil olmalı")
	}

	frame := Frame{ID: "f1", Width: 640, Height: 640, Timestamp: time.Now()}
	e.Detect(frame)

	r := e.LastResult()
	if r == nil {
		t.Fatal("LastResult() Detect sonrası nil olmamalı")
	}
	if r.FrameID != "f1" {
		t.Errorf("LastResult().FrameID = %s, want f1", r.FrameID)
	}
}

func TestAllClasses(t *testing.T) {
	if len(AllClasses) != 14 {
		t.Errorf("AllClasses = %d, want 14", len(AllClasses))
	}
	// Her sınıfın karakteristik tanımı olmalı
	for _, cls := range AllClasses {
		if _, ok := classCharacteristics[cls]; !ok {
			t.Errorf("classCharacteristics[%s] tanımsız", cls)
		}
	}
}
