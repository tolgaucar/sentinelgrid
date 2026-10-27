package texture

import (
	"testing"
)

func newTestEngine() *Engine {
	return NewEngine(4, 6, 0.05, 0.40, 16, 0.35)
}

func TestGaborBankSize(t *testing.T) {
	eng := newTestEngine()
	expected := 4 * 6 // scales * orientations
	if len(eng.gaborBank) != expected {
		t.Errorf("Gabor bankası boyutu %d beklenirken %d döndü", expected, len(eng.gaborBank))
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	eng := newTestEngine()
	results := eng.Analyze(nil)
	if len(results) != 0 {
		t.Errorf("boş girişte 0 sonuç beklenirken %d döndü", len(results))
	}
}

func TestAnalyzeSingleRegion(t *testing.T) {
	eng := newTestEngine()
	regions := []RegionInput{
		{ID: "r1", Lat: 39.0, Lon: 32.0, Terrain: TerrainForest, Width: 256, Height: 256},
	}
	results := eng.Analyze(regions)
	if len(results) != 1 {
		t.Fatalf("1 sonuç beklenirken %d döndü", len(results))
	}
	r := results[0]
	if r.RegionID != "r1" {
		t.Errorf("region_id='r1' beklenirken '%s' döndü", r.RegionID)
	}
	if r.Terrain != TerrainForest {
		t.Errorf("terrain='forest' beklenirken '%s' döndü", r.Terrain)
	}
	if len(r.GaborResponse) != 24 {
		t.Errorf("24 Gabor yanıtı beklenirken %d döndü", len(r.GaborResponse))
	}
}

func TestAnalyzeMultipleTerrains(t *testing.T) {
	eng := newTestEngine()
	regions := []RegionInput{
		{ID: "forest", Lat: 39.0, Lon: 32.0, Terrain: TerrainForest, Width: 256, Height: 256},
		{ID: "desert", Lat: 38.0, Lon: 31.0, Terrain: TerrainDesert, Width: 256, Height: 256},
		{ID: "urban", Lat: 40.0, Lon: 33.0, Terrain: TerrainUrban, Width: 256, Height: 256},
	}
	results := eng.Analyze(regions)
	if len(results) != 3 {
		t.Fatalf("3 sonuç beklenirken %d döndü", len(results))
	}
	for _, r := range results {
		if r.Feature.Energy < 0 || r.Feature.Energy > 1.0 {
			t.Errorf("enerji 0-1 arasında olmalı, %.3f döndü (bölge: %s)", r.Feature.Energy, r.RegionID)
		}
	}
}

func TestFeatureExtraction(t *testing.T) {
	eng := newTestEngine()
	gaborResp := make([]float64, 24)
	for i := range gaborResp {
		gaborResp[i] = 0.3 + float64(i)*0.01
	}
	feature := eng.extractFeatures(gaborResp, TerrainForest)

	if feature.Energy <= 0 {
		t.Error("enerji pozitif olmalı")
	}
	if feature.Homogeneity < 0 || feature.Homogeneity > 1.0 {
		t.Errorf("homojenlik 0-1 arasında olmalı, %.3f döndü", feature.Homogeneity)
	}
	if feature.Entropy < 0 || feature.Entropy > 1.0 {
		t.Errorf("entropi 0-1 arasında olmalı, %.3f döndü", feature.Entropy)
	}
}

func TestFeatureExtractionEmpty(t *testing.T) {
	eng := newTestEngine()
	feature := eng.extractFeatures(nil, TerrainForest)
	if feature.Energy != 0 {
		t.Errorf("boş giriş için enerji 0 olmalı, %.3f döndü", feature.Energy)
	}
}

func TestDivergenceComputation(t *testing.T) {
	eng := newTestEngine()
	profile := TerrainProfiles[TerrainForest]
	bgFeature := TextureFeature{
		Energy:   profile.AvgEnergy,
		Contrast: profile.AvgContrast,
		Variance: profile.AvgVariance,
		Entropy:  profile.AvgEntropy,
	}

	// arkaplanla aynı — düşük sapma
	sameFeat := bgFeature
	divSame := eng.computeDivergence(sameFeat, bgFeature, profile)
	if divSame > 0.1 {
		t.Errorf("aynı özellik düşük sapma vermeli, %.3f döndü", divSame)
	}

	// çok farklı — yüksek sapma
	diffFeat := TextureFeature{
		Energy:   0.9,
		Contrast: 0.1,
		Variance: 0.9,
		Entropy:  0.1,
	}
	divDiff := eng.computeDivergence(diffFeat, bgFeature, profile)
	if divDiff <= divSame {
		t.Errorf("farklı özellik daha yüksek sapma vermeli: same=%.3f, diff=%.3f", divSame, divDiff)
	}
}

func TestConvexScore(t *testing.T) {
	eng := newTestEngine()
	resp := make([]float64, 24)
	for i := range resp {
		resp[i] = 0.5
	}
	feature := TextureFeature{
		Energy:      0.7,
		Variance:    0.02,
		Correlation: 0.8,
	}
	score := eng.computeConvexScore(resp, feature)
	if score < 0 || score > 1.0 {
		t.Errorf("konveks skor 0-1 arasında olmalı, %.3f döndü", score)
	}
}

func TestConcaveScore(t *testing.T) {
	eng := newTestEngine()
	resp := make([]float64, 24)
	for i := range resp {
		resp[i] = 0.2
	}
	feature := TextureFeature{
		Energy:   0.1,
		Contrast: 0.8,
	}
	score := eng.computeConcaveScore(resp, feature)
	if score < 0 || score > 1.0 {
		t.Errorf("konkav skor 0-1 arasında olmalı, %.3f döndü", score)
	}
}

func TestConvexScoreTooFewResponses(t *testing.T) {
	eng := newTestEngine()
	score := eng.computeConvexScore([]float64{0.1, 0.2}, TextureFeature{})
	if score != 0 {
		t.Errorf("yetersiz Gabor yanıtında 0 skor beklenirken %.3f döndü", score)
	}
}

func TestTerrainProfiles(t *testing.T) {
	terrains := []TerrainType{TerrainForest, TerrainRocky, TerrainDesert, TerrainUrban, TerrainGrass, TerrainSnow, TerrainWater}
	for _, tr := range terrains {
		profile, ok := TerrainProfiles[tr]
		if !ok {
			t.Errorf("arazi profili bulunamadı: %s", tr)
			continue
		}
		if profile.AvgEnergy <= 0 {
			t.Errorf("%s arazi profili geçersiz enerji: %.3f", tr, profile.AvgEnergy)
		}
		if profile.StdVariance <= 0 {
			t.Errorf("%s arazi profili geçersiz std varyans: %.3f", tr, profile.StdVariance)
		}
	}
}

func TestUnknownTerrainFallback(t *testing.T) {
	eng := newTestEngine()
	regions := []RegionInput{
		{ID: "unknown", Lat: 39.0, Lon: 32.0, Terrain: "alien_terrain", Width: 128, Height: 128},
	}
	results := eng.Analyze(regions)
	if len(results) != 1 {
		t.Fatalf("1 sonuç beklenirken %d döndü", len(results))
	}
	// bilinmeyen arazi türünde çökmemeli
}

func TestStats(t *testing.T) {
	eng := newTestEngine()
	eng.Analyze([]RegionInput{{ID: "r1", Terrain: TerrainForest, Width: 64, Height: 64}})
	stats := eng.Stats()
	if stats["analysis_count"].(int64) != 1 {
		t.Errorf("analysis_count=1 beklenirken %v döndü", stats["analysis_count"])
	}
	if stats["gabor_bank_size"].(int) != 24 {
		t.Errorf("gabor_bank_size=24 beklenirken %v döndü", stats["gabor_bank_size"])
	}
}
