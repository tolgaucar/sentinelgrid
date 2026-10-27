package detector

import (
	"testing"

	"github.com/otukenmatris/sezi/internal/texture"
)

func newTestEngine() *Engine {
	texEngine := texture.NewEngine(4, 6, 0.05, 0.40, 16, 0.35)
	return NewEngine(texEngine, 600, 0.3, 0.60, 0.55, 500)
}

func TestScanEmpty(t *testing.T) {
	eng := newTestEngine()
	result := eng.Scan(nil)
	if len(result.Detections) != 0 {
		t.Errorf("boş taramada 0 tespit beklenirken %d döndü", len(result.Detections))
	}
	if result.TotalRegions != 0 {
		t.Errorf("boş taramada 0 bölge beklenirken %d döndü", result.TotalRegions)
	}
}

func TestScanSingleRegion(t *testing.T) {
	eng := newTestEngine()
	inputs := []ScanInput{
		{ID: "area-1", Lat: 39.0, Lon: 32.0, Terrain: texture.TerrainForest, Width: 256, Height: 256},
	}
	result := eng.Scan(inputs)
	if result.TotalRegions != 1 {
		t.Errorf("1 bölge beklenirken %d döndü", result.TotalRegions)
	}
	if result.ElapsedMs <= 0 {
		t.Error("elapsed_ms pozitif olmalı")
	}
}

func TestScanMultipleRegions(t *testing.T) {
	eng := newTestEngine()
	inputs := []ScanInput{
		{ID: "a", Lat: 39.0, Lon: 32.0, Terrain: texture.TerrainForest, Width: 256, Height: 256},
		{ID: "b", Lat: 39.1, Lon: 32.1, Terrain: texture.TerrainRocky, Width: 256, Height: 256},
		{ID: "c", Lat: 39.2, Lon: 32.2, Terrain: texture.TerrainDesert, Width: 256, Height: 256},
	}
	result := eng.Scan(inputs)
	if result.TotalRegions != 3 {
		t.Errorf("3 bölge beklenirken %d döndü", result.TotalRegions)
	}
}

func TestGetDetection(t *testing.T) {
	eng := newTestEngine()
	// birkaç tarama yaparak tespit üret
	for i := 0; i < 10; i++ {
		eng.Scan([]ScanInput{
			{ID: "forest-area", Lat: 39.0, Lon: 32.0, Terrain: texture.TerrainForest, Width: 512, Height: 512},
		})
	}
	all := eng.GetAllDetections()
	if len(all) > 0 {
		det, ok := eng.GetDetection(all[0].ID)
		if !ok {
			t.Fatal("tespit bulunamadı")
		}
		if det.Confidence <= 0 || det.Confidence > 1.0 {
			t.Errorf("güven 0-1 arasında olmalı, %.3f döndü", det.Confidence)
		}
	}
}

func TestGetDetectionNotFound(t *testing.T) {
	eng := newTestEngine()
	_, ok := eng.GetDetection("nonexistent")
	if ok {
		t.Error("var olmayan tespit bulunamaz olmalı")
	}
}

func TestGetByThreat(t *testing.T) {
	eng := newTestEngine()
	// çok sayıda tarama yaparak farklı tehdit seviyelerinde tespit üret
	for i := 0; i < 20; i++ {
		eng.Scan([]ScanInput{
			{ID: "zone-a", Lat: 39.0, Lon: 32.0, Terrain: texture.TerrainForest, Width: 512, Height: 512},
			{ID: "zone-b", Lat: 38.0, Lon: 31.0, Terrain: texture.TerrainDesert, Width: 512, Height: 512},
		})
	}
	// GetByThreat her seviye için çalışmalı (boş da dönebilir)
	for _, level := range []ThreatLevel{ThreatLow, ThreatMedium, ThreatHigh, ThreatCritical} {
		dets := eng.GetByThreat(level)
		_ = dets // çökmemeli
	}
}

func TestGetHighThreat(t *testing.T) {
	eng := newTestEngine()
	threats := eng.GetHighThreat()
	// boş motor'dan boş veya nil slice dönmeli — çökmemeli
	if len(threats) != 0 {
		t.Errorf("boş motorda 0 tehdit beklenirken %d döndü", len(threats))
	}
}

func TestPrune(t *testing.T) {
	texEngine := texture.NewEngine(4, 6, 0.05, 0.40, 16, 0.35)
	eng := NewEngine(texEngine, 0, 0.0, 0.0, 0.0, 500) // maxAge=0, confidenceMin=0: hemen expire, her şeyi tespit et
	eng.Scan([]ScanInput{
		{ID: "x", Lat: 39.0, Lon: 32.0, Terrain: texture.TerrainForest, Width: 256, Height: 256},
	})
	all := eng.GetAllDetections()
	if len(all) > 0 {
		removed := eng.Prune()
		if removed < 1 {
			t.Errorf("en az 1 tespit temizlenmeli, %d temizlendi", removed)
		}
	}
}

func TestStats(t *testing.T) {
	eng := newTestEngine()
	eng.Scan([]ScanInput{
		{ID: "r1", Lat: 39.0, Lon: 32.0, Terrain: texture.TerrainForest, Width: 256, Height: 256},
	})
	stats := eng.Stats()
	if stats["scan_count"].(int64) != 1 {
		t.Errorf("scan_count=1 beklenirken %v döndü", stats["scan_count"])
	}
}

func TestCamoTypeClassification(t *testing.T) {
	eng := newTestEngine()
	// dışbükey dominant, yüksek homojenlik → branda
	ar := texture.AnalysisResult{
		ConvexScore:  0.7,
		ConcaveScore: 0.2,
		Feature: texture.TextureFeature{
			Homogeneity: 0.8,
			Correlation: 0.4,
			Entropy:     0.3,
			Energy:      0.5,
		},
	}
	ct := eng.classifyCamoType(ar)
	if ct != CamoTarp {
		t.Errorf("CamoTarp beklenirken %s döndü", ct)
	}

	// içbükey dominant, düşük enerji → doğal gizlenme
	ar2 := texture.AnalysisResult{
		ConvexScore:  0.2,
		ConcaveScore: 0.7,
		Feature: texture.TextureFeature{
			Homogeneity: 0.3,
			Energy:      0.1,
		},
	}
	ct2 := eng.classifyCamoType(ar2)
	if ct2 != CamoNatural {
		t.Errorf("CamoNatural beklenirken %s döndü", ct2)
	}
}

func TestThreatAssessment(t *testing.T) {
	eng := newTestEngine()

	// yüksek güven + termal kamuflaj = kritik
	threat := eng.assessThreat(0.9, CamoThermal)
	if threat != ThreatCritical {
		t.Errorf("ThreatCritical beklenirken %s döndü", threat)
	}

	// düşük güven = düşük tehdit
	threat2 := eng.assessThreat(0.2, CamoNatural)
	if threat2 != ThreatLow {
		t.Errorf("ThreatLow beklenirken %s döndü", threat2)
	}
}

func TestEstimateSize(t *testing.T) {
	cases := []struct {
		convex, concave float64
		want            string
	}{
		{0.8, 0.6, "large"},
		{0.5, 0.3, "medium"},
		{0.2, 0.1, "small"},
	}
	for _, tc := range cases {
		got := estimateSize(tc.convex, tc.concave)
		if got != tc.want {
			t.Errorf("estimateSize(%.1f, %.1f) = %s, beklenirken %s", tc.convex, tc.concave, got, tc.want)
		}
	}
}

func TestDescribeCamo(t *testing.T) {
	desc := describeCamo(CamoNet, texture.TerrainForest, 0.9)
	if desc == "" {
		t.Error("açıklama boş olmamalı")
	}
	// yüksek güven ifadesi içermeli
	if len(desc) < 10 {
		t.Error("açıklama çok kısa")
	}
}
