package imagery

import (
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine(100, 32, 0.15)
	if e == nil {
		t.Fatal("engine nil")
	}
	stats := e.Stats()
	if stats["total_images"].(int) != 0 {
		t.Fatal("expected 0 images")
	}
}

func makeTestImage(id string, imgType ImageType, lat, lon float64, grid int) SatelliteImage {
	pixels := make([][]float64, grid)
	for y := range pixels {
		pixels[y] = make([]float64, grid)
		for x := range pixels[y] {
			pixels[y][x] = 0.1
		}
	}
	return SatelliteImage{
		ID: id, Type: imgType, CaptureTime: time.Now(),
		CenterLat: lat, CenterLon: lon,
		Width: grid, Height: grid, Resolution: 0.5,
		Pixels: pixels,
	}
}

func TestIngest(t *testing.T) {
	e := NewEngine(100, 16, 0.15)
	img := makeTestImage("img-1", TypeOptical, 39.9, 32.8, 16)
	if err := e.Ingest(img); err != nil {
		t.Fatal(err)
	}
	stats := e.Stats()
	if stats["total_images"].(int) != 1 {
		t.Fatalf("expected 1, got %d", stats["total_images"])
	}
}

func TestIngestDuplicate(t *testing.T) {
	e := NewEngine(100, 16, 0.15)
	img := makeTestImage("img-1", TypeOptical, 39.9, 32.8, 16)
	e.Ingest(img)
	err := e.Ingest(img)
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestIngestCapacity(t *testing.T) {
	e := NewEngine(2, 8, 0.15)
	e.Ingest(makeTestImage("a", TypeSAR, 39.0, 32.0, 8))
	e.Ingest(makeTestImage("b", TypeSAR, 39.0, 32.0, 8))
	err := e.Ingest(makeTestImage("c", TypeSAR, 39.0, 32.0, 8))
	if err == nil {
		t.Fatal("expected capacity error")
	}
}

func TestGet(t *testing.T) {
	e := NewEngine(100, 16, 0.15)
	e.Ingest(makeTestImage("img-1", TypeOptical, 39.9, 32.8, 16))
	img, ok := e.Get("img-1")
	if !ok || img.ID != "img-1" {
		t.Fatal("get failed")
	}
	_, ok = e.Get("nonexistent")
	if ok {
		t.Fatal("should not find nonexistent")
	}
}

func TestGetAll(t *testing.T) {
	e := NewEngine(100, 16, 0.15)
	e.Ingest(makeTestImage("a", TypeSAR, 39.0, 32.0, 16))
	e.Ingest(makeTestImage("b", TypeOptical, 39.0, 32.0, 16))
	all := e.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}
	// pikseller döndürülmemeli
	for _, img := range all {
		if img.Pixels != nil {
			t.Fatal("pixels should be nil in GetAll")
		}
	}
}

func TestDelete(t *testing.T) {
	e := NewEngine(100, 16, 0.15)
	e.Ingest(makeTestImage("img-1", TypeOptical, 39.9, 32.8, 16))
	if err := e.Delete("img-1"); err != nil {
		t.Fatal(err)
	}
	if err := e.Delete("img-1"); err == nil {
		t.Fatal("expected error for deleted image")
	}
}

func TestGetByRegion(t *testing.T) {
	e := NewEngine(100, 16, 0.15)
	e.Ingest(makeTestImage("near", TypeSAR, 39.925, 32.837, 16))
	e.Ingest(makeTestImage("far", TypeSAR, 41.0, 29.0, 16))
	result := e.GetByRegion(39.925, 32.837, 10.0)
	if len(result) != 1 {
		t.Fatalf("expected 1 near image, got %d", len(result))
	}
}

func TestComputeDiff(t *testing.T) {
	e := NewEngine(100, 8, 0.15)
	imgA := makeTestImage("a", TypeOptical, 39.9, 32.8, 8)
	imgB := makeTestImage("b", TypeOptical, 39.9, 32.8, 8)
	// B'de bazı pikselleri değiştir
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			imgB.Pixels[y][x] = 0.8
		}
	}
	e.Ingest(imgA)
	e.Ingest(imgB)

	dm, err := e.ComputeDiff("a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if dm.MeanDiff <= 0 {
		t.Fatal("mean diff should be > 0")
	}
	if dm.MaxDiff < 0.5 {
		t.Fatalf("max diff should be >= 0.5, got %f", dm.MaxDiff)
	}
	if dm.ChangePct <= 0 {
		t.Fatal("change pct should be > 0")
	}
}

func TestComputeDiffNotFound(t *testing.T) {
	e := NewEngine(100, 8, 0.15)
	_, err := e.ComputeDiff("a", "b")
	if err == nil {
		t.Fatal("expected error for missing images")
	}
}

func TestHistogram(t *testing.T) {
	e := NewEngine(100, 8, 0.15)
	imgA := makeTestImage("a", TypeOptical, 39.9, 32.8, 8)
	imgB := makeTestImage("b", TypeOptical, 39.9, 32.8, 8)
	imgB.Pixels[0][0] = 0.9
	e.Ingest(imgA)
	e.Ingest(imgB)
	dm, _ := e.ComputeDiff("a", "b")
	h := ComputeHistogram(dm)
	if len(h.Bins) != 10 {
		t.Fatalf("expected 10 bins, got %d", len(h.Bins))
	}
	total := 0
	for _, b := range h.Bins {
		total += b
	}
	if total != 64 {
		t.Fatalf("expected 64 pixels, got %d", total)
	}
}

func TestGenerateFromEntities(t *testing.T) {
	e := NewEngine(100, 32, 0.15)
	entities := []map[string]any{
		{"id": "e-1", "type": "tank", "faction": "enemy", "lat": 39.5, "lon": 32.5, "speed": 30.0},
		{"id": "e-2", "type": "infantry", "faction": "friendly", "lat": 39.51, "lon": 32.51, "speed": 5.0},
	}
	img := e.GenerateFromEntities(entities, TypeOptical)
	if img == nil {
		t.Fatal("nil image")
	}
	if img.Type != TypeOptical {
		t.Fatalf("expected optical, got %s", img.Type)
	}
	if img.Width != 32 || img.Height != 32 {
		t.Fatalf("expected 32x32, got %dx%d", img.Width, img.Height)
	}
}

func TestPrune(t *testing.T) {
	e := NewEngine(100, 8, 0.15)
	img := makeTestImage("old", TypeSAR, 39.0, 32.0, 8)
	img.CreatedAt = time.Now().Add(-72 * time.Hour)
	e.mu.Lock()
	e.images["old"] = &img
	e.mu.Unlock()

	e.Ingest(makeTestImage("new", TypeSAR, 39.0, 32.0, 8))
	removed := e.Prune(48)
	if removed != 1 {
		t.Fatalf("expected 1 pruned, got %d", removed)
	}
}

func TestStats(t *testing.T) {
	e := NewEngine(100, 16, 0.15)
	e.Ingest(makeTestImage("s1", TypeSAR, 39.0, 32.0, 16))
	e.Ingest(makeTestImage("o1", TypeOptical, 39.0, 32.0, 16))
	s := e.Stats()
	if s["sar_images"].(int) != 1 {
		t.Fatal("expected 1 SAR")
	}
	if s["optical_images"].(int) != 1 {
		t.Fatal("expected 1 optical")
	}
}

func TestHaversine(t *testing.T) {
	d := haversine(39.925, 32.837, 39.925, 32.837)
	if d != 0 {
		t.Fatalf("same point should be 0, got %f", d)
	}
	d2 := haversine(39.0, 32.0, 40.0, 33.0)
	if d2 < 100 || d2 > 200 {
		t.Fatalf("distance unexpected: %f", d2)
	}
}
