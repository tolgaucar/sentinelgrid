// Package collector — OSINT veri toplama motoru.
//
// Açık kaynaklardan (sosyal medya, haber siteleri, forumlar, uydu metadatası,
// resmi yayınlar) veri toplar. Kaynak yönetimi CRUD API ile yapılır.
// Simülasyon modunda sentetik OSINT verisi üretir.
package collector

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"
)

// SourceType — OSINT kaynak türü.
type SourceType string

const (
	SourceSocialMedia   SourceType = "social_media"
	SourceNewsFeed      SourceType = "news_feed"
	SourceForum         SourceType = "forum"
	SourceSatelliteMeta SourceType = "satellite_meta"
	SourceOfficial      SourceType = "official"
)

// Source — OSINT veri kaynağı tanımı.
type Source struct {
	ID              string     `json:"id"`
	Type            SourceType `json:"type"`
	Name            string     `json:"name"`
	URL             string     `json:"url"`
	Language        string     `json:"language"`
	Region          *GeoRegion `json:"region,omitempty"`
	RefreshInterval int        `json:"refresh_interval_s"`
	Credibility     float64    `json:"credibility"`
	Enabled         bool       `json:"enabled"`
	Tags            []string   `json:"tags"`
	LastFetched     time.Time  `json:"last_fetched"`
	ItemCount       int64      `json:"item_count"`
	ErrorCount      int64      `json:"error_count"`
	CreatedAt       time.Time  `json:"created_at"`
}

// GeoRegion — coğrafi alan filtresi.
type GeoRegion struct {
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	RadiusKm float64 `json:"radius_km"`
}

// SourceItem — kaynaktan toplanan tek veri parçası.
type SourceItem struct {
	ID          string     `json:"id"`
	SourceID    string     `json:"source_id"`
	SourceType  SourceType `json:"source_type"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	URL         string     `json:"url"`
	Language    string     `json:"language"`
	Lat         float64    `json:"lat"`
	Lon         float64    `json:"lon"`
	Credibility float64   `json:"credibility"`
	Tags        []string   `json:"tags"`
	Sentiment   float64    `json:"sentiment"` // -1.0 (negatif) ↔ +1.0 (pozitif)
	CollectedAt time.Time  `json:"collected_at"`
}

// CollectResult — toplama sonucu.
type CollectResult struct {
	Items       []SourceItem `json:"items"`
	SourceID    string       `json:"source_id"`
	SourceName  string       `json:"source_name"`
	ItemCount   int          `json:"item_count"`
	ElapsedMs   float64      `json:"elapsed_ms"`
}

// Engine — OSINT veri toplama motoru.
type Engine struct {
	mu            sync.RWMutex
	sources       map[string]*Source
	items         []SourceItem
	maxSources    int
	maxItems      int
	minCredibility float64
	pruneAge      int
	collectCount  int64
	totalItems    int64
}

// NewEngine — yeni OSINT toplama motoru.
func NewEngine(maxSources, maxItems int, minCredibility float64, pruneAge int) *Engine {
	e := &Engine{
		sources:        make(map[string]*Source),
		maxSources:     maxSources,
		maxItems:       maxItems,
		minCredibility: minCredibility,
		pruneAge:       pruneAge,
	}
	e.seedDefaultSources()
	return e
}

// seedDefaultSources — simülasyon için varsayılan kaynaklar.
func (e *Engine) seedDefaultSources() {
	defaults := []Source{
		{ID: "src-sm-01", Type: SourceSocialMedia, Name: "Telegram Askeri Kanal", URL: "https://t.me/mil_intel", Language: "tr", Credibility: 0.6, Enabled: true, Tags: []string{"military", "telegram"}, RefreshInterval: 120,
			Region: &GeoRegion{Lat: 39.9, Lon: 32.8, RadiusKm: 300}},
		{ID: "src-sm-02", Type: SourceSocialMedia, Name: "X/Twitter OSINT", URL: "https://x.com/search?q=military", Language: "en", Credibility: 0.5, Enabled: true, Tags: []string{"osint", "social"}, RefreshInterval: 60,
			Region: &GeoRegion{Lat: 39.0, Lon: 35.0, RadiusKm: 500}},
		{ID: "src-nf-01", Type: SourceNewsFeed, Name: "Reuters Savunma", URL: "https://reuters.com/rss/defense", Language: "en", Credibility: 0.9, Enabled: true, Tags: []string{"news", "defense"}, RefreshInterval: 300,
			Region: &GeoRegion{Lat: 39.9, Lon: 32.8, RadiusKm: 1000}},
		{ID: "src-nf-02", Type: SourceNewsFeed, Name: "Anadolu Ajansı", URL: "https://aa.com.tr/rss", Language: "tr", Credibility: 0.85, Enabled: true, Tags: []string{"news", "turkey"}, RefreshInterval: 300,
			Region: &GeoRegion{Lat: 39.9, Lon: 32.8, RadiusKm: 500}},
		{ID: "src-fo-01", Type: SourceForum, Name: "Savunma Forum", URL: "https://forum.example/defense", Language: "tr", Credibility: 0.4, Enabled: true, Tags: []string{"forum", "discussion"}, RefreshInterval: 600},
		{ID: "src-sa-01", Type: SourceSatelliteMeta, Name: "Sentinel-2 Metadata", URL: "https://sentinel-hub.example/api", Language: "en", Credibility: 0.95, Enabled: true, Tags: []string{"satellite", "imagery"}, RefreshInterval: 900,
			Region: &GeoRegion{Lat: 39.0, Lon: 35.0, RadiusKm: 200}},
		{ID: "src-of-01", Type: SourceOfficial, Name: "MSB Basın Açıklamaları", URL: "https://msb.gov.tr/basin", Language: "tr", Credibility: 0.95, Enabled: true, Tags: []string{"official", "military"}, RefreshInterval: 1800},
	}
	for i := range defaults {
		defaults[i].CreatedAt = time.Now()
		e.sources[defaults[i].ID] = &defaults[i]
	}
}

// ── Kaynak CRUD ──

// AddSource — yeni kaynak ekle.
func (e *Engine) AddSource(s Source) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.sources) >= e.maxSources {
		return fmt.Errorf("maksimum kaynak sayısına ulaşıldı: %d", e.maxSources)
	}
	if s.ID == "" {
		s.ID = fmt.Sprintf("src-%s-%d", s.Type, time.Now().UnixMilli())
	}
	if _, exists := e.sources[s.ID]; exists {
		return fmt.Errorf("kaynak zaten var: %s", s.ID)
	}
	if s.Credibility < e.minCredibility {
		s.Credibility = e.minCredibility
	}
	s.CreatedAt = time.Now()
	e.sources[s.ID] = &s
	return nil
}

// UpdateSource — kaynağı güncelle.
func (e *Engine) UpdateSource(id string, update Source) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	existing, ok := e.sources[id]
	if !ok {
		return fmt.Errorf("kaynak bulunamadı: %s", id)
	}
	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.URL != "" {
		existing.URL = update.URL
	}
	if update.Language != "" {
		existing.Language = update.Language
	}
	if update.Region != nil {
		existing.Region = update.Region
	}
	if update.RefreshInterval > 0 {
		existing.RefreshInterval = update.RefreshInterval
	}
	if update.Credibility > 0 {
		existing.Credibility = update.Credibility
	}
	if update.Tags != nil {
		existing.Tags = update.Tags
	}
	existing.Enabled = update.Enabled
	return nil
}

// DeleteSource — kaynağı sil.
func (e *Engine) DeleteSource(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.sources[id]; !ok {
		return fmt.Errorf("kaynak bulunamadı: %s", id)
	}
	delete(e.sources, id)
	return nil
}

// GetSource — tek kaynak getir.
func (e *Engine) GetSource(id string) (*Source, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s, ok := e.sources[id]
	if !ok {
		return nil, false
	}
	copy := *s
	return &copy, true
}

// GetAllSources — tüm kaynaklar.
func (e *Engine) GetAllSources() []Source {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]Source, 0, len(e.sources))
	for _, s := range e.sources {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// GetEnabledSources — aktif kaynaklar.
func (e *Engine) GetEnabledSources() []Source {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []Source
	for _, s := range e.sources {
		if s.Enabled {
			result = append(result, *s)
		}
	}
	return result
}

// ── Veri Toplama ──

// Collect — belirtilen kaynaktan veri topla (simülasyon: sentetik üretir).
func (e *Engine) Collect(sourceID string) (*CollectResult, error) {
	start := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()

	src, ok := e.sources[sourceID]
	if !ok {
		return nil, fmt.Errorf("kaynak bulunamadı: %s", sourceID)
	}
	if !src.Enabled {
		return nil, fmt.Errorf("kaynak devre dışı: %s", sourceID)
	}

	items := e.generateSyntheticItems(src, 2+rand.Intn(5))
	e.items = append(e.items, items...)
	src.LastFetched = time.Now()
	src.ItemCount += int64(len(items))
	e.collectCount++
	e.totalItems += int64(len(items))

	// kapasite kontrolü
	if len(e.items) > e.maxItems {
		e.items = e.items[len(e.items)-e.maxItems:]
	}

	return &CollectResult{
		Items:      items,
		SourceID:   sourceID,
		SourceName: src.Name,
		ItemCount:  len(items),
		ElapsedMs:  float64(time.Since(start).Microseconds()) / 1000.0,
	}, nil
}

// CollectAll — tüm aktif kaynaklardan veri topla.
func (e *Engine) CollectAll() []CollectResult {
	e.mu.RLock()
	var enabledIDs []string
	for id, s := range e.sources {
		if s.Enabled {
			enabledIDs = append(enabledIDs, id)
		}
	}
	e.mu.RUnlock()

	var results []CollectResult
	for _, id := range enabledIDs {
		r, err := e.Collect(id)
		if err == nil {
			results = append(results, *r)
		}
	}
	return results
}

// CollectFromEntities — savaş alanı varlıklarından sentetik OSINT üret (simülasyon).
func (e *Engine) CollectFromEntities(entities []map[string]any) []SourceItem {
	e.mu.Lock()
	defer e.mu.Unlock()

	var items []SourceItem
	for _, ent := range entities {
		id, _ := ent["id"].(string)
		entType, _ := ent["type"].(string)
		faction, _ := ent["faction"].(string)
		lat, _ := ent["lat"].(float64)
		lon, _ := ent["lon"].(float64)
		speed, _ := ent["speed"].(float64)

		// düşman birimleri daha yüksek istihbarat değerine sahip
		if faction != "enemy" && rand.Float64() > 0.3 {
			continue
		}

		item := e.generateEntityItem(id, entType, faction, lat, lon, speed)
		items = append(items, item)
	}

	e.items = append(e.items, items...)
	e.totalItems += int64(len(items))

	if len(e.items) > e.maxItems {
		e.items = e.items[len(e.items)-e.maxItems:]
	}

	return items
}

// GetItems — toplanan veriler.
func (e *Engine) GetItems(limit int) []SourceItem {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if limit <= 0 || limit > len(e.items) {
		limit = len(e.items)
	}
	start := len(e.items) - limit
	if start < 0 {
		start = 0
	}
	result := make([]SourceItem, limit)
	copy(result, e.items[start:])
	return result
}

// Prune — eski verileri temizle.
func (e *Engine) Prune() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	cutoff := time.Now().Add(-time.Duration(e.pruneAge) * time.Second)
	var kept []SourceItem
	removed := 0
	for _, item := range e.items {
		if item.CollectedAt.After(cutoff) {
			kept = append(kept, item)
		} else {
			removed++
		}
	}
	e.items = kept
	return removed
}

// Stats — motor istatistikleri.
func (e *Engine) Stats() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()
	enabledCount := 0
	for _, s := range e.sources {
		if s.Enabled {
			enabledCount++
		}
	}
	return map[string]any{
		"total_sources":   len(e.sources),
		"enabled_sources": enabledCount,
		"total_items":     e.totalItems,
		"active_items":    len(e.items),
		"collect_count":   e.collectCount,
	}
}

// ── Sentetik veri üretimi (simülasyon modu) ──

var militaryTopics = []string{
	"konvoy hareketi tespit edildi",
	"bilinmeyen araç grubu görüntülendi",
	"mühimmat nakliyesi iddiası",
	"sınır bölgesinde hareketlilik",
	"askeri tatbikat duyurusu",
	"hava savunma sistemi konuşlandırılması",
	"deniz kuvvetleri manevra bildirimi",
	"insansız hava aracı görüntülemesi",
	"elektronik sinyal anomalisi raporu",
	"stratejik köprü yakınında yığınak",
	"yakıt ikmal noktası tespiti",
	"gece operasyonu istihbaratı",
	"komuta merkezi lokasyon tahmini",
	"top mevzisi değişikliği",
	"lojistik koridor aktivitesi",
}

var socialMediaTemplates = []string{
	"🚨 %s bölgesinde yoğun askeri hareketlilik gözlemleniyor",
	"⚠️ Yerel kaynaklar %s civarında konvoy geçişi bildiriyor",
	"📡 %s sektöründe bilinmeyen RF emisyonları tespit edildi",
	"🔥 %s yakınlarında patlamalar duyulduğu iddia ediliyor",
	"🚁 %s üzerinde alçak irtifa helikopter trafiği yoğunlaştı",
}

var newsTemplates = []string{
	"FLAŞ: %s bölgesinde güvenlik güçleri alarma geçti — askeri kaynaklar",
	"Son dakika: %s yakınlarında askeri yığınak uydu görüntüleriyle doğrulandı",
	"Savunma Bakanlığı %s bölgesinde tatbikat yapıldığını açıkladı",
	"%s sınırında artan tansiyon — bölge sakinleri tedirgin",
	"İstihbarat kaynakları: %s'de düşman kuvvetleri mevzi değiştiriyor",
}

var regionNames = []string{
	"Kuzey Sektör", "Güney Hat", "Doğu Koridor", "Batı Cephe",
	"Merkez Bölge", "Sınır Hattı", "Dağ Geçidi", "Kıyı Şeridi",
	"Ova Bölgesi", "Orman Kuşağı",
}

func (e *Engine) generateSyntheticItems(src *Source, count int) []SourceItem {
	items := make([]SourceItem, count)
	for i := range items {
		region := regionNames[rand.Intn(len(regionNames))]
		lat, lon := 39.0+rand.Float64()*2, 32.0+rand.Float64()*6

		if src.Region != nil {
			lat = src.Region.Lat + (rand.Float64()-0.5)*src.Region.RadiusKm/111.0
			lon = src.Region.Lon + (rand.Float64()-0.5)*src.Region.RadiusKm/111.0
		}

		var title, content string
		switch src.Type {
		case SourceSocialMedia:
			title = fmt.Sprintf(socialMediaTemplates[rand.Intn(len(socialMediaTemplates))], region)
			content = title + ". " + militaryTopics[rand.Intn(len(militaryTopics))]
		case SourceNewsFeed:
			title = fmt.Sprintf(newsTemplates[rand.Intn(len(newsTemplates))], region)
			content = title + ". Detaylı analiz devam ediyor."
		case SourceForum:
			content = "Tartışma: " + militaryTopics[rand.Intn(len(militaryTopics))]
			title = content[:min(60, len(content))]
		case SourceSatelliteMeta:
			title = fmt.Sprintf("Uydu Görüntü Değişim — %.4f, %.4f", lat, lon)
			content = fmt.Sprintf("Koordinat: %.4f, %.4f — yapısal değişim skoru: %.2f", lat, lon, 0.3+rand.Float64()*0.7)
		case SourceOfficial:
			title = "Resmi Açıklama: " + militaryTopics[rand.Intn(len(militaryTopics))]
			content = title + ". İlgili birimler bilgilendirildi."
		}

		sentiment := -0.3 - rand.Float64()*0.7 // askeri haberler genellikle olumsuz
		if src.Type == SourceOfficial {
			sentiment = -0.1 + rand.Float64()*0.3 // resmi açıklamalar nötral
		}

		items[i] = SourceItem{
			ID:          fmt.Sprintf("item-%s-%d-%d", src.ID, time.Now().UnixMilli(), i),
			SourceID:    src.ID,
			SourceType:  src.Type,
			Title:       title,
			Content:     content,
			URL:         src.URL + fmt.Sprintf("/%d", time.Now().Unix()),
			Language:    src.Language,
			Lat:         math.Round(lat*10000) / 10000,
			Lon:         math.Round(lon*10000) / 10000,
			Credibility: src.Credibility * (0.8 + rand.Float64()*0.2),
			Tags:        src.Tags,
			Sentiment:   math.Round(sentiment*100) / 100,
			CollectedAt: time.Now(),
		}
	}
	return items
}

func (e *Engine) generateEntityItem(id, entType, faction string, lat, lon, speed float64) SourceItem {
	region := regionNames[rand.Intn(len(regionNames))]
	typeLabel := entityTypeLabel(entType)

	var title, content string
	src := e.randomEnabledSource()
	srcID := "src-sm-01"
	srcType := SourceSocialMedia
	cred := 0.5
	if src != nil {
		srcID = src.ID
		srcType = src.Type
		cred = src.Credibility
	}

	if faction == "enemy" {
		title = fmt.Sprintf("🔴 Düşman %s tespit — %s (%.2f km/h)", typeLabel, region, speed)
		content = fmt.Sprintf("Düşman %s birimi %s bölgesinde hareket halinde. Koordinat: %.4f, %.4f. Hız: %.1f. Kaynak güvenilirliği: %.0f%%",
			typeLabel, region, lat, lon, speed, cred*100)
	} else {
		title = fmt.Sprintf("📊 %s aktivitesi — %s", typeLabel, region)
		content = fmt.Sprintf("%s hareketliliği %s bölgesinde gözlemlendi", typeLabel, region)
	}

	sentiment := -0.5 - rand.Float64()*0.5
	if faction != "enemy" {
		sentiment = -0.1 + rand.Float64()*0.2
	}

	return SourceItem{
		ID:          fmt.Sprintf("ent-%s-%d", id, time.Now().UnixMilli()),
		SourceID:    srcID,
		SourceType:  srcType,
		Title:       title,
		Content:     content,
		Language:    "tr",
		Lat:         math.Round(lat*10000) / 10000,
		Lon:         math.Round(lon*10000) / 10000,
		Credibility: cred * (0.7 + rand.Float64()*0.3),
		Tags:        []string{"entity", faction, entType},
		Sentiment:   math.Round(sentiment*100) / 100,
		CollectedAt: time.Now(),
	}
}

func (e *Engine) randomEnabledSource() *Source {
	for _, s := range e.sources {
		if s.Enabled {
			return s
		}
	}
	return nil
}

func entityTypeLabel(t string) string {
	labels := map[string]string{
		"tank": "Zırhlı", "apc": "ZMA", "infantry": "Piyade",
		"helicopter": "Helikopter", "jet": "Jet", "drone": "İHA",
		"artillery": "Topçu", "command": "Komuta",
	}
	if l, ok := labels[t]; ok {
		return l
	}
	return strings.Title(t)
}
