// Package analyzer — OSINT semantik analiz ve istihbarat üretim motoru.
//
// Toplanan ham OSINT verisini analiz ederek yapılandırılmış istihbarat
// raporları üretir. Duygu analizi, varlık çıkarma (NER), coğrafi
// korelasyon, çapraz doğrulama ve trend analizi yapar.
package analyzer

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/otukenmatris/duyum/internal/collector"
)

// ThreatType — istihbarat tehdit türü.
type ThreatType string

const (
	ThreatTroopMovement   ThreatType = "troop_movement"     // birlik hareketi
	ThreatWeaponDeployment ThreatType = "weapon_deployment"  // silah konuşlandırma
	ThreatLogisticsSupply ThreatType = "logistics_supply"    // lojistik ikmal
	ThreatAerialActivity  ThreatType = "aerial_activity"     // hava aktivitesi
	ThreatNavalActivity   ThreatType = "naval_activity"      // deniz aktivitesi
	ThreatCyberSignal     ThreatType = "cyber_signal"        // siber/sinyal
	ThreatReconnaissance  ThreatType = "reconnaissance"      // keşif
	ThreatUnknown         ThreatType = "unknown"             // bilinmeyen
)

// ThreatLevel — tehdit seviyesi.
type ThreatLevel string

const (
	ThreatLevelLow      ThreatLevel = "low"
	ThreatLevelMedium   ThreatLevel = "medium"
	ThreatLevelHigh     ThreatLevel = "high"
	ThreatLevelCritical ThreatLevel = "critical"
)

// Entity — çıkarılan varlık (NER sonucu).
type Entity struct {
	Name string `json:"name"`
	Type string `json:"type"` // location, unit, weapon, date, org
	Raw  string `json:"raw"`
}

// IntelReport — istihbarat raporu.
type IntelReport struct {
	ID             string      `json:"id"`
	Title          string      `json:"title"`
	Summary        string      `json:"summary"`
	ThreatType     ThreatType  `json:"threat_type"`
	ThreatLevel    ThreatLevel `json:"threat_level"`
	Confidence     float64     `json:"confidence"`
	Lat            float64     `json:"lat"`
	Lon            float64     `json:"lon"`
	RadiusKm       float64     `json:"radius_km"`
	SourceCount    int         `json:"source_count"`
	Entities       []Entity    `json:"entities"`
	RelatedItems   []string    `json:"related_items"`
	Recommendation string     `json:"recommendation"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	ExpiresAt      time.Time   `json:"expires_at"`
}

// TrendInfo — bölgesel trend bilgisi.
type TrendInfo struct {
	Region      string  `json:"region"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	ItemCount   int     `json:"item_count"`
	AvgSentiment float64 `json:"avg_sentiment"`
	ThreatScore float64 `json:"threat_score"`
	Trend       string  `json:"trend"` // rising, stable, declining
}

// AnalyzeResult — analiz sonucu.
type AnalyzeResult struct {
	Reports     []IntelReport `json:"reports"`
	ItemsInput  int           `json:"items_input"`
	ReportsGen  int           `json:"reports_generated"`
	EntitiesExt int           `json:"entities_extracted"`
	CrossRefed  int           `json:"cross_referenced"`
	ElapsedMs   float64       `json:"elapsed_ms"`
}

// Engine — semantik analiz motoru.
type Engine struct {
	mu              sync.RWMutex
	reports         map[string]*IntelReport
	maxReports      int
	confidenceMin   float64
	crossRefMin     int
	threatDecayS    int
	analyzeCount    int64
	reportsGenerated int64
	entitiesExtracted int64
}

// NewEngine — yeni analiz motoru.
func NewEngine(maxReports int, confidenceMin float64, crossRefMin, threatDecayS int) *Engine {
	return &Engine{
		reports:       make(map[string]*IntelReport),
		maxReports:    maxReports,
		confidenceMin: confidenceMin,
		crossRefMin:   crossRefMin,
		threatDecayS:  threatDecayS,
	}
}

// Analyze — OSINT öğelerini analiz et ve istihbarat raporları üret.
func (e *Engine) Analyze(items []collector.SourceItem) AnalyzeResult {
	start := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()

	e.analyzeCount++
	var reports []IntelReport
	totalEntities := 0
	crossRefed := 0

	// coğrafi kümeleme: yakın konumlardaki öğeleri grupla
	clusters := e.clusterByLocation(items)

	for _, cluster := range clusters {
		if len(cluster) == 0 {
			continue
		}

		// varlık çıkarma (NER)
		entities := e.extractEntities(cluster)
		totalEntities += len(entities)

		// tehdit türü sınıflandırma
		threatType := e.classifyThreat(cluster, entities)

		// duygu analizi ortalaması
		avgSentiment := avgSentiment(cluster)

		// güven hesapla
		confidence := e.computeConfidence(cluster)

		// çapraz doğrulama kontrolü
		uniqueSources := uniqueSourceCount(cluster)
		if uniqueSources >= e.crossRefMin {
			confidence = math.Min(1.0, confidence*1.3) // çapraz doğrulama güven artışı
			crossRefed++
		}

		if confidence < e.confidenceMin {
			continue
		}

		// tehdit seviyesi
		threatLevel := e.assessThreatLevel(threatType, avgSentiment, confidence, len(cluster))

		// konum (ağırlıklı merkez)
		lat, lon := weightedCenter(cluster)

		// rapor oluştur
		report := IntelReport{
			ID:             fmt.Sprintf("rpt-%d-%d", time.Now().UnixMilli(), rand.Intn(1000)),
			Title:          e.generateTitle(threatType, cluster),
			Summary:        e.generateSummary(threatType, cluster, entities),
			ThreatType:     threatType,
			ThreatLevel:    threatLevel,
			Confidence:     math.Round(confidence*1000) / 1000,
			Lat:            math.Round(lat*10000) / 10000,
			Lon:            math.Round(lon*10000) / 10000,
			RadiusKm:       estimateRadius(cluster),
			SourceCount:    uniqueSources,
			Entities:       entities,
			RelatedItems:   itemIDs(cluster),
			Recommendation: e.generateRecommendation(threatType, threatLevel),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
			ExpiresAt:      time.Now().Add(time.Duration(e.threatDecayS) * time.Second),
		}

		e.reports[report.ID] = &report
		reports = append(reports, report)
		e.reportsGenerated++
	}

	// kapasite kontrolü
	e.pruneExcess()

	return AnalyzeResult{
		Reports:     reports,
		ItemsInput:  len(items),
		ReportsGen:  len(reports),
		EntitiesExt: totalEntities,
		CrossRefed:  crossRefed,
		ElapsedMs:   float64(time.Since(start).Microseconds()) / 1000.0,
	}
}

// GetReport — tek rapor.
func (e *Engine) GetReport(id string) (*IntelReport, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	r, ok := e.reports[id]
	if !ok {
		return nil, false
	}
	cp := *r
	return &cp, true
}

// GetAllReports — tüm aktif raporlar.
func (e *Engine) GetAllReports() []IntelReport {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]IntelReport, 0, len(e.reports))
	for _, r := range e.reports {
		result = append(result, *r)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result
}

// GetHighThreat — yüksek ve kritik tehdit raporları.
func (e *Engine) GetHighThreat() []IntelReport {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []IntelReport
	for _, r := range e.reports {
		if r.ThreatLevel == ThreatLevelHigh || r.ThreatLevel == ThreatLevelCritical {
			result = append(result, *r)
		}
	}
	return result
}

// GetTrends — bölgesel trend analizi.
func (e *Engine) GetTrends() []TrendInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	type regionBucket struct {
		lat, lon     float64
		count        int
		sentimentSum float64
		threatSum    float64
	}
	buckets := make(map[string]*regionBucket)

	for _, r := range e.reports {
		key := fmt.Sprintf("%.1f,%.1f", math.Round(r.Lat), math.Round(r.Lon))
		b, ok := buckets[key]
		if !ok {
			b = &regionBucket{lat: r.Lat, lon: r.Lon}
			buckets[key] = b
		}
		b.count++
		b.threatSum += float64(threatLevelWeight(r.ThreatLevel))
	}

	var trends []TrendInfo
	for key, b := range buckets {
		avgThreat := b.threatSum / float64(b.count)
		trend := "stable"
		if avgThreat > 2.5 {
			trend = "rising"
		} else if avgThreat < 1.0 {
			trend = "declining"
		}
		trends = append(trends, TrendInfo{
			Region:      key,
			Lat:         math.Round(b.lat*100) / 100,
			Lon:         math.Round(b.lon*100) / 100,
			ItemCount:   b.count,
			AvgSentiment: math.Round(b.sentimentSum/float64(b.count)*100) / 100,
			ThreatScore: math.Round(avgThreat*100) / 100,
			Trend:       trend,
		})
	}
	sort.Slice(trends, func(i, j int) bool { return trends[i].ThreatScore > trends[j].ThreatScore })
	return trends
}

// Prune — süresi dolmuş raporları temizle.
func (e *Engine) Prune() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	removed := 0
	for id, r := range e.reports {
		if now.After(r.ExpiresAt) {
			delete(e.reports, id)
			removed++
		}
	}
	return removed
}

func (e *Engine) pruneExcess() {
	if len(e.reports) <= e.maxReports {
		return
	}
	type entry struct {
		id string
		t  time.Time
	}
	entries := make([]entry, 0, len(e.reports))
	for id, r := range e.reports {
		entries = append(entries, entry{id, r.CreatedAt})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].t.Before(entries[j].t) })
	toRemove := len(e.reports) - e.maxReports
	for i := 0; i < toRemove; i++ {
		delete(e.reports, entries[i].id)
	}
}

// Stats — motor istatistikleri.
func (e *Engine) Stats() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()
	highCount := 0
	for _, r := range e.reports {
		if r.ThreatLevel == ThreatLevelHigh || r.ThreatLevel == ThreatLevelCritical {
			highCount++
		}
	}
	return map[string]any{
		"analyze_count":      e.analyzeCount,
		"reports_generated":  e.reportsGenerated,
		"entities_extracted": e.entitiesExtracted,
		"active_reports":     len(e.reports),
		"high_threats":       highCount,
	}
}

// ── İç algoritmalar ──

// clusterByLocation — coğrafi kümeleme (~10km yarıçap).
func (e *Engine) clusterByLocation(items []collector.SourceItem) [][]collector.SourceItem {
	if len(items) == 0 {
		return nil
	}

	used := make([]bool, len(items))
	var clusters [][]collector.SourceItem

	for i := range items {
		if used[i] {
			continue
		}
		cluster := []collector.SourceItem{items[i]}
		used[i] = true

		for j := i + 1; j < len(items); j++ {
			if used[j] {
				continue
			}
			dist := haversineKm(items[i].Lat, items[i].Lon, items[j].Lat, items[j].Lon)
			if dist < 10.0 { // 10km yarıçap
				cluster = append(cluster, items[j])
				used[j] = true
			}
		}
		clusters = append(clusters, cluster)
	}
	return clusters
}

// extractEntities — basit NER (anahtar kelime tabanlı).
func (e *Engine) extractEntities(items []collector.SourceItem) []Entity {
	var entities []Entity
	seen := make(map[string]bool)

	locationKW := []string{"sektör", "bölge", "hat", "koridor", "cephe", "sınır", "geçit", "kıyı", "ova", "orman"}
	unitKW := []string{"konvoy", "birlik", "tabur", "bölük", "tugay", "filo", "müfreze", "tim"}
	weaponKW := []string{"tank", "zırhlı", "top", "füze", "havan", "uçak", "helikopter", "İHA", "drone", "obüs"}

	for _, item := range items {
		text := strings.ToLower(item.Title + " " + item.Content)

		for _, kw := range locationKW {
			if strings.Contains(text, kw) && !seen["loc:"+kw] {
				entities = append(entities, Entity{Name: kw, Type: "location", Raw: kw})
				seen["loc:"+kw] = true
			}
		}
		for _, kw := range unitKW {
			if strings.Contains(text, kw) && !seen["unit:"+kw] {
				entities = append(entities, Entity{Name: kw, Type: "unit", Raw: kw})
				seen["unit:"+kw] = true
			}
		}
		for _, kw := range weaponKW {
			if strings.Contains(text, strings.ToLower(kw)) && !seen["wpn:"+kw] {
				entities = append(entities, Entity{Name: kw, Type: "weapon", Raw: kw})
				seen["wpn:"+kw] = true
			}
		}
	}
	e.entitiesExtracted += int64(len(entities))
	return entities
}

// classifyThreat — tehdit türü sınıflandırma.
func (e *Engine) classifyThreat(items []collector.SourceItem, entities []Entity) ThreatType {
	text := ""
	for _, item := range items {
		text += strings.ToLower(item.Title + " " + item.Content + " ")
	}

	// varlık tabanlı sınıflandırma
	hasWeapon := false
	hasUnit := false
	for _, ent := range entities {
		if ent.Type == "weapon" {
			hasWeapon = true
		}
		if ent.Type == "unit" {
			hasUnit = true
		}
	}

	switch {
	case strings.Contains(text, "konvoy") || strings.Contains(text, "hareket") || strings.Contains(text, "manevra"):
		return ThreatTroopMovement
	case hasWeapon && (strings.Contains(text, "konuşlan") || strings.Contains(text, "mevzi")):
		return ThreatWeaponDeployment
	case strings.Contains(text, "lojistik") || strings.Contains(text, "ikmal") || strings.Contains(text, "yakıt"):
		return ThreatLogisticsSupply
	case strings.Contains(text, "hava") || strings.Contains(text, "uçak") || strings.Contains(text, "helikopter") || strings.Contains(text, "iha"):
		return ThreatAerialActivity
	case strings.Contains(text, "deniz") || strings.Contains(text, "filo") || strings.Contains(text, "gemi"):
		return ThreatNavalActivity
	case strings.Contains(text, "sinyal") || strings.Contains(text, "emisyon") || strings.Contains(text, "siber"):
		return ThreatCyberSignal
	case strings.Contains(text, "keşif") || strings.Contains(text, "gözetleme") || strings.Contains(text, "uydu"):
		return ThreatReconnaissance
	case hasUnit:
		return ThreatTroopMovement
	default:
		return ThreatUnknown
	}
}

// computeConfidence — güven hesapla.
func (e *Engine) computeConfidence(items []collector.SourceItem) float64 {
	if len(items) == 0 {
		return 0
	}
	// bileşenler: kaynak güvenilirliği, kaynak çeşitliliği, tutarlılık
	var sumCred float64
	for _, item := range items {
		sumCred += item.Credibility
	}
	avgCred := sumCred / float64(len(items))

	uniqueSrc := uniqueSourceCount(items)
	diversityFactor := math.Min(1.0, float64(uniqueSrc)/3.0)

	countFactor := math.Min(1.0, float64(len(items))/5.0)

	return math.Min(1.0, avgCred*0.4+diversityFactor*0.35+countFactor*0.25)
}

// assessThreatLevel — tehdit seviyesi belirle.
func (e *Engine) assessThreatLevel(tt ThreatType, sentiment float64, confidence float64, itemCount int) ThreatLevel {
	score := confidence

	// tehdit türü ağırlığı
	switch tt {
	case ThreatWeaponDeployment:
		score *= 1.4
	case ThreatTroopMovement:
		score *= 1.2
	case ThreatAerialActivity:
		score *= 1.3
	case ThreatLogisticsSupply:
		score *= 1.0
	case ThreatCyberSignal:
		score *= 1.1
	case ThreatReconnaissance:
		score *= 0.9
	default:
		score *= 0.7
	}

	// olumsuz duygu = daha tehlikeli
	if sentiment < -0.5 {
		score *= 1.2
	}

	// çok kaynaklı doğrulama
	if itemCount >= 3 {
		score *= 1.1
	}

	switch {
	case score >= 0.85:
		return ThreatLevelCritical
	case score >= 0.60:
		return ThreatLevelHigh
	case score >= 0.35:
		return ThreatLevelMedium
	default:
		return ThreatLevelLow
	}
}

func (e *Engine) generateTitle(tt ThreatType, items []collector.SourceItem) string {
	prefix := map[ThreatType]string{
		ThreatTroopMovement:    "BİRLİK HAREKETİ",
		ThreatWeaponDeployment: "SİLAH KONUŞLANDIRMA",
		ThreatLogisticsSupply:  "LOJİSTİK İKMAL",
		ThreatAerialActivity:   "HAVA AKTİVİTESİ",
		ThreatNavalActivity:    "DENİZ AKTİVİTESİ",
		ThreatCyberSignal:      "SİBER/SİNYAL",
		ThreatReconnaissance:   "KEŞİF FAALİYETİ",
		ThreatUnknown:          "BİLİNMEYEN AKTİVİTE",
	}
	p := prefix[tt]
	if len(items) > 0 {
		return fmt.Sprintf("%s — %d kaynak, bölge: %.2f, %.2f", p, len(items), items[0].Lat, items[0].Lon)
	}
	return p
}

func (e *Engine) generateSummary(tt ThreatType, items []collector.SourceItem, entities []Entity) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%d açık kaynak verisi analiz edildi.", len(items)))

	if len(entities) > 0 {
		var names []string
		for _, ent := range entities[:min(5, len(entities))] {
			names = append(names, ent.Name)
		}
		parts = append(parts, fmt.Sprintf("Tespit edilen varlıklar: %s.", strings.Join(names, ", ")))
	}

	uniqueSrc := uniqueSourceCount(items)
	if uniqueSrc >= 2 {
		parts = append(parts, fmt.Sprintf("%d bağımsız kaynak ile çapraz doğrulama yapıldı.", uniqueSrc))
	}

	return strings.Join(parts, " ")
}

func (e *Engine) generateRecommendation(tt ThreatType, tl ThreatLevel) string {
	if tl == ThreatLevelCritical {
		return "ACİL: Komuta merkezine bildirim yapılmalı, sensör takviyesi önerilir"
	}
	switch tt {
	case ThreatTroopMovement:
		return "Bölgeye İHA/keşif uçuşu yönlendirilmeli"
	case ThreatWeaponDeployment:
		return "Hava savunma hazırlığı artırılmalı, ELINT taraması başlatılmalı"
	case ThreatLogisticsSupply:
		return "Lojistik koridor gözetim altına alınmalı"
	case ThreatAerialActivity:
		return "Hava radar kapsamı genişletilmeli"
	case ThreatCyberSignal:
		return "Elektronik harp önlemleri aktifleştirilmeli"
	default:
		return "Ek istihbarat toplama ile doğrulama yapılmalı"
	}
}

// ── Yardımcı fonksiyonlar ──

func avgSentiment(items []collector.SourceItem) float64 {
	if len(items) == 0 {
		return 0
	}
	var sum float64
	for _, item := range items {
		sum += item.Sentiment
	}
	return sum / float64(len(items))
}

func uniqueSourceCount(items []collector.SourceItem) int {
	seen := make(map[string]bool)
	for _, item := range items {
		seen[item.SourceID] = true
	}
	return len(seen)
}

func weightedCenter(items []collector.SourceItem) (float64, float64) {
	var sumLat, sumLon, sumW float64
	for _, item := range items {
		w := item.Credibility
		if w < 0.1 {
			w = 0.1
		}
		sumLat += item.Lat * w
		sumLon += item.Lon * w
		sumW += w
	}
	if sumW < 0.001 {
		return 0, 0
	}
	return sumLat / sumW, sumLon / sumW
}

func estimateRadius(items []collector.SourceItem) float64 {
	if len(items) < 2 {
		return 5.0
	}
	cLat, cLon := weightedCenter(items)
	var maxDist float64
	for _, item := range items {
		d := haversineKm(cLat, cLon, item.Lat, item.Lon)
		if d > maxDist {
			maxDist = d
		}
	}
	return math.Max(1.0, math.Round(maxDist*10)/10)
}

func itemIDs(items []collector.SourceItem) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func threatLevelWeight(tl ThreatLevel) int {
	switch tl {
	case ThreatLevelCritical:
		return 4
	case ThreatLevelHigh:
		return 3
	case ThreatLevelMedium:
		return 2
	case ThreatLevelLow:
		return 1
	default:
		return 0
	}
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
