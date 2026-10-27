package analyzer

import (
	"testing"
	"time"

	"github.com/otukenmatris/duyum/internal/collector"
)

func newTestEngine() *Engine {
	return NewEngine(100, 0.1, 2, 1800)
}

func TestNewEngine(t *testing.T) {
	e := newTestEngine()
	if e == nil {
		t.Fatal("Engine nil")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	e := newTestEngine()
	result := e.Analyze(nil)
	if len(result.Reports) != 0 {
		t.Fatalf("expected 0 reports for nil input")
	}
}

func TestAnalyzeSingleItem(t *testing.T) {
	e := newTestEngine()
	items := []collector.SourceItem{
		{
			ID: "item-1", SourceID: "src-1", SourceType: collector.SourceSocialMedia,
			Title: "Kuzey Sektörde konvoy hareketi tespit edildi",
			Content: "Düşman konvoy birlik hareket halinde, tank ve zırhlı araçlar görüntülendi",
			Lat: 39.5, Lon: 33.0, Credibility: 0.7, Sentiment: -0.6,
			CollectedAt: time.Now(),
		},
	}
	result := e.Analyze(items)
	if result.ItemsInput != 1 {
		t.Fatalf("expected 1 input, got %d", result.ItemsInput)
	}
	if len(result.Reports) < 1 {
		t.Fatalf("expected at least 1 report, got %d", len(result.Reports))
	}
	rpt := result.Reports[0]
	if rpt.ThreatType != ThreatTroopMovement {
		t.Fatalf("expected troop_movement, got %s", rpt.ThreatType)
	}
	if rpt.Confidence <= 0 {
		t.Fatal("expected positive confidence")
	}
}

func TestAnalyzeMultipleSourcesCrossRef(t *testing.T) {
	e := newTestEngine()
	items := []collector.SourceItem{
		{ID: "item-1", SourceID: "src-1", Title: "Sınır bölgesinde askeri yığınak", Content: "Birlik hareketi devam ediyor", Lat: 39.5, Lon: 33.0, Credibility: 0.8, Sentiment: -0.7, CollectedAt: time.Now()},
		{ID: "item-2", SourceID: "src-2", Title: "Aynı bölgede konvoy geçişi", Content: "Tank konvoyu gözlemlendi", Lat: 39.501, Lon: 33.001, Credibility: 0.7, Sentiment: -0.5, CollectedAt: time.Now()},
		{ID: "item-3", SourceID: "src-3", Title: "Uydu görüntüsü doğruladı", Content: "Uydu keşif gözetleme ile birlik teyit", Lat: 39.502, Lon: 33.002, Credibility: 0.9, Sentiment: -0.4, CollectedAt: time.Now()},
	}
	result := e.Analyze(items)
	if result.CrossRefed < 1 {
		t.Fatalf("expected at least 1 cross-referenced, got %d", result.CrossRefed)
	}
	if len(result.Reports) < 1 {
		t.Fatal("expected at least 1 report")
	}
	rpt := result.Reports[0]
	if rpt.SourceCount < 3 {
		t.Fatalf("expected at least 3 sources, got %d", rpt.SourceCount)
	}
}

func TestGetReport(t *testing.T) {
	e := newTestEngine()
	items := []collector.SourceItem{
		{ID: "i1", SourceID: "s1", Title: "Test konvoy", Content: "Birlik hareket", Lat: 39.0, Lon: 33.0, Credibility: 0.7, CollectedAt: time.Now()},
	}
	result := e.Analyze(items)
	if len(result.Reports) == 0 {
		t.Skip("no reports generated")
	}
	rpt, ok := e.GetReport(result.Reports[0].ID)
	if !ok || rpt == nil {
		t.Fatal("expected report")
	}
}

func TestGetReportNotFound(t *testing.T) {
	e := newTestEngine()
	_, ok := e.GetReport("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestGetAllReports(t *testing.T) {
	e := newTestEngine()
	items := []collector.SourceItem{
		{ID: "i1", SourceID: "s1", Title: "Konvoy hareket", Content: "Birlik", Lat: 39.0, Lon: 33.0, Credibility: 0.7, CollectedAt: time.Now()},
		{ID: "i2", SourceID: "s2", Title: "Hava aktivite", Content: "Helikopter", Lat: 40.0, Lon: 34.0, Credibility: 0.8, CollectedAt: time.Now()},
	}
	e.Analyze(items)
	all := e.GetAllReports()
	if len(all) == 0 {
		t.Fatal("expected some reports")
	}
}

func TestGetHighThreat(t *testing.T) {
	e := newTestEngine()
	// yüksek güvenilirlikli, çoklu kaynak, olumsuz duygu → high/critical
	items := []collector.SourceItem{
		{ID: "i1", SourceID: "s1", Title: "Silah konuşlandırma mevzi", Content: "Tank füze top obüs konuşlandırıldı", Lat: 39.0, Lon: 33.0, Credibility: 0.95, Sentiment: -0.9, CollectedAt: time.Now()},
		{ID: "i2", SourceID: "s2", Title: "Doğrulama", Content: "Silah konuşlandırma teyit mevzi", Lat: 39.001, Lon: 33.001, Credibility: 0.9, Sentiment: -0.8, CollectedAt: time.Now()},
		{ID: "i3", SourceID: "s3", Title: "Üçüncü kaynak", Content: "Tank konuşlandırma mevzi bölge", Lat: 39.002, Lon: 33.002, Credibility: 0.85, Sentiment: -0.7, CollectedAt: time.Now()},
	}
	e.Analyze(items)
	high := e.GetHighThreat()
	if len(high) == 0 {
		t.Fatal("expected some high threats")
	}
}

func TestGetTrends(t *testing.T) {
	e := newTestEngine()
	items := []collector.SourceItem{
		{ID: "i1", SourceID: "s1", Title: "Birlik hareket", Content: "Konvoy", Lat: 39.0, Lon: 33.0, Credibility: 0.7, CollectedAt: time.Now()},
	}
	e.Analyze(items)
	trends := e.GetTrends()
	if len(trends) == 0 {
		t.Fatal("expected some trends")
	}
}

func TestPrune(t *testing.T) {
	e := NewEngine(100, 0.1, 2, 1) // 1 saniye expiry
	e.reports["old"] = &IntelReport{ID: "old", ExpiresAt: time.Now().Add(-2 * time.Second)}
	e.reports["new"] = &IntelReport{ID: "new", ExpiresAt: time.Now().Add(1 * time.Hour)}
	removed := e.Prune()
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	if len(e.reports) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(e.reports))
	}
}

func TestStats(t *testing.T) {
	e := newTestEngine()
	stats := e.Stats()
	if stats["active_reports"].(int) != 0 {
		t.Fatalf("expected 0, got %v", stats["active_reports"])
	}
}

func TestThreatClassification(t *testing.T) {
	e := newTestEngine()
	tests := []struct {
		name    string
		content string
		want    ThreatType
	}{
		{"troop", "konvoy hareket birlik", ThreatTroopMovement},
		{"logistics", "lojistik ikmal yakıt", ThreatLogisticsSupply},
		{"aerial", "helikopter hava uçak", ThreatAerialActivity},
		{"naval", "deniz filo gemi", ThreatNavalActivity},
		{"cyber", "sinyal emisyon siber", ThreatCyberSignal},
		{"recon", "keşif gözetleme uydu", ThreatReconnaissance},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := []collector.SourceItem{
				{Title: tt.content, Content: tt.content},
			}
			got := e.classifyThreat(items, nil)
			if got != tt.want {
				t.Errorf("classifyThreat(%s) = %s, want %s", tt.name, got, tt.want)
			}
		})
	}
}

func TestHaversine(t *testing.T) {
	d := haversineKm(41.0082, 28.9784, 39.9334, 32.8597)
	if d < 300 || d > 400 {
		t.Fatalf("unexpected distance: %.0f km", d)
	}
}
