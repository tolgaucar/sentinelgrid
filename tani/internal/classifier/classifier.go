// Package classifier — Nesne sınıflandırma ve IFF (Dost/Düşman Tanıma).
//
// Tespit edilen nesneleri detaylı sınıflandırır:
// - Araç tipi (ana muharebe tankı, zırhlı personel taşıyıcı, lojistik araç)
// - Personel (muharip, keskin nişancı, sivil)
// - Sabit tesis (radar, hava savunma, topçu, komuta merkezi)
// - IFF: Dost / Düşman / Tarafsız / Bilinmeyen
// - Tehdit seviyesi hesaplama
package classifier

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/tani/internal/detector"
)

// Allegiance dost/düşman/tarafsız.
type Allegiance string

const (
	AllegianceFriendly Allegiance = "friendly"
	AllegianceHostile  Allegiance = "hostile"
	AllegianceNeutral  Allegiance = "neutral"
	AllegianceUnknown  Allegiance = "unknown"
)

// ThreatLevel tehdit seviyesi.
type ThreatLevel string

const (
	ThreatNone     ThreatLevel = "none"
	ThreatLow      ThreatLevel = "low"
	ThreatMedium   ThreatLevel = "medium"
	ThreatHigh     ThreatLevel = "high"
	ThreatCritical ThreatLevel = "critical"
)

// Category nesne ana kategorisi.
type Category string

const (
	CatVehicle   Category = "vehicle"
	CatPersonnel Category = "personnel"
	CatEquipment Category = "equipment"
	CatAircraft  Category = "aircraft"
	CatUnknown   Category = "unknown"
)

// classMapping sınıf → kategori eşlemesi.
var classMapping = map[detector.ObjectClass]Category{
	detector.ClassTank:        CatVehicle,
	detector.ClassAPC:         CatVehicle,
	detector.ClassTruck:       CatVehicle,
	detector.ClassHelicopter:  CatAircraft,
	detector.ClassFighterJet:  CatAircraft,
	detector.ClassDrone:       CatAircraft,
	detector.ClassSoldier:     CatPersonnel,
	detector.ClassSniper:      CatPersonnel,
	detector.ClassCivilian:    CatPersonnel,
	detector.ClassRadar:       CatEquipment,
	detector.ClassSAM:         CatEquipment,
	detector.ClassArtillery:   CatEquipment,
	detector.ClassCommandPost: CatEquipment,
	detector.ClassSupplyDepot: CatEquipment,
}

// baseThreat sınıf bazında temel tehdit.
var baseThreat = map[detector.ObjectClass]float64{
	detector.ClassTank:        0.90,
	detector.ClassAPC:         0.70,
	detector.ClassTruck:       0.30,
	detector.ClassHelicopter:  0.85,
	detector.ClassFighterJet:  0.95,
	detector.ClassDrone:       0.65,
	detector.ClassSoldier:     0.50,
	detector.ClassSniper:      0.80,
	detector.ClassCivilian:    0.05,
	detector.ClassRadar:       0.60,
	detector.ClassSAM:         0.90,
	detector.ClassArtillery:   0.85,
	detector.ClassCommandPost: 0.70,
	detector.ClassSupplyDepot: 0.20,
}

// ClassifiedObject sınıflandırılmış nesne.
type ClassifiedObject struct {
	DetectionID  string              `json:"detection_id"`
	Class        detector.ObjectClass `json:"class"`
	Category     Category            `json:"category"`
	Allegiance   Allegiance          `json:"allegiance"`
	ThreatLevel  ThreatLevel         `json:"threat_level"`
	ThreatScore  float64             `json:"threat_score"`
	Confidence   float64             `json:"confidence"`
	Description  string              `json:"description"`
	Timestamp    time.Time           `json:"timestamp"`
}

// Engine sınıflandırma motoru.
type Engine struct {
	mu sync.RWMutex

	iffEnabled    bool
	minConfidence float64

	// IFF bilinen kimlikler
	knownFriendly map[string]bool

	// İstatistikler
	totalClassified int64
	friendlyCount   int64
	hostileCount    int64
	neutralCount    int64
	unknownCount    int64

	// Sınıf dağılımı
	classCounts map[detector.ObjectClass]int64
	catCounts   map[Category]int64
}

// NewEngine yeni sınıflandırma motoru.
func NewEngine(iffEnabled bool, minConf float64) *Engine {
	return &Engine{
		iffEnabled:    iffEnabled,
		minConfidence: minConf,
		knownFriendly: make(map[string]bool),
		classCounts:   make(map[detector.ObjectClass]int64),
		catCounts:     make(map[Category]int64),
	}
}

// RegisterFriendly dost platform kaydet (IFF).
func (e *Engine) RegisterFriendly(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.knownFriendly[id] = true
}

// UnregisterFriendly dost kaydını sil.
func (e *Engine) UnregisterFriendly(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.knownFriendly, id)
}

// Classify tek bir tespiti sınıflandırır.
func (e *Engine) Classify(det detector.Detection) ClassifiedObject {
	atomic.AddInt64(&e.totalClassified, 1)

	cat := classMapping[det.Class]
	if cat == "" {
		cat = CatUnknown
	}

	// IFF belirleme
	allegiance := e.determineAllegiance(det)

	// Tehdit hesaplama
	threatScore := e.calculateThreat(det, allegiance)
	threatLvl := scoreThreatLevel(threatScore)

	desc := e.describe(det.Class, allegiance)

	obj := ClassifiedObject{
		DetectionID: det.ID,
		Class:       det.Class,
		Category:    cat,
		Allegiance:  allegiance,
		ThreatLevel: threatLvl,
		ThreatScore: math.Round(threatScore*1000) / 1000,
		Confidence:  det.Confidence,
		Description: desc,
		Timestamp:   time.Now(),
	}

	// İstatistik güncelle
	e.mu.Lock()
	e.classCounts[det.Class]++
	e.catCounts[cat]++
	e.mu.Unlock()

	switch allegiance {
	case AllegianceFriendly:
		atomic.AddInt64(&e.friendlyCount, 1)
	case AllegianceHostile:
		atomic.AddInt64(&e.hostileCount, 1)
	case AllegianceNeutral:
		atomic.AddInt64(&e.neutralCount, 1)
	default:
		atomic.AddInt64(&e.unknownCount, 1)
	}

	return obj
}

// ClassifyBatch birden fazla tespiti sınıflandırır.
func (e *Engine) ClassifyBatch(dets []detector.Detection) []ClassifiedObject {
	result := make([]ClassifiedObject, 0, len(dets))
	for _, d := range dets {
		if d.Confidence >= e.minConfidence {
			result = append(result, e.Classify(d))
		}
	}
	return result
}

// determineAllegiance IFF belirleme.
func (e *Engine) determineAllegiance(det detector.Detection) Allegiance {
	if !e.iffEnabled {
		return AllegianceUnknown
	}

	// Siviller her zaman tarafsız
	if det.Class == detector.ClassCivilian {
		return AllegianceNeutral
	}

	// Bilinen dost transponder — metadata kontrolü
	if det.Metadata != nil {
		if tid, ok := det.Metadata["transponder_id"].(string); ok {
			e.mu.RLock()
			friendly := e.knownFriendly[tid]
			e.mu.RUnlock()
			if friendly {
				return AllegianceFriendly
			}
		}
		// IFF sorgusundan gelen cevap
		if iff, ok := det.Metadata["iff_response"].(string); ok {
			switch iff {
			case "friendly":
				return AllegianceFriendly
			case "hostile":
				return AllegianceHostile
			case "neutral":
				return AllegianceNeutral
			}
		}
	}

	// Güven skoru bazlı karar — yüksek güvende düşman kabul et
	if det.Confidence > 0.85 {
		return AllegianceHostile
	}
	return AllegianceUnknown
}

// calculateThreat tehdit skoru hesapla.
func (e *Engine) calculateThreat(det detector.Detection, alleg Allegiance) float64 {
	base := baseThreat[det.Class]
	if base == 0 {
		base = 0.1
	}

	// Dostlar düşük tehdit
	switch alleg {
	case AllegianceFriendly:
		return base * 0.05
	case AllegianceNeutral:
		return base * 0.15
	case AllegianceUnknown:
		base *= 0.5
	}

	// Güven ile ağırlıkla
	return base * det.Confidence
}

// scoreThreatLevel skor → seviye.
func scoreThreatLevel(score float64) ThreatLevel {
	switch {
	case score >= 0.80:
		return ThreatCritical
	case score >= 0.60:
		return ThreatHigh
	case score >= 0.35:
		return ThreatMedium
	case score >= 0.10:
		return ThreatLow
	default:
		return ThreatNone
	}
}

// describe nesne açıklaması.
func (e *Engine) describe(cls detector.ObjectClass, alleg Allegiance) string {
	labels := map[detector.ObjectClass]string{
		detector.ClassTank:        "Ana Muharebe Tankı",
		detector.ClassAPC:         "Zırhlı Personel Taşıyıcı",
		detector.ClassTruck:       "Lojistik Araç",
		detector.ClassHelicopter:  "Taarruz Helikopteri",
		detector.ClassFighterJet:  "Muharip Uçak",
		detector.ClassDrone:       "İnsansız Hava Aracı (İHA)",
		detector.ClassSoldier:     "Piyade Eri",
		detector.ClassSniper:      "Keskin Nişancı",
		detector.ClassCivilian:    "Sivil Kişi",
		detector.ClassRadar:       "Gözetleme Radarı",
		detector.ClassSAM:         "Hava Savunma Sistemi (SAM)",
		detector.ClassArtillery:   "Topçu Bataryası",
		detector.ClassCommandPost: "Komuta Merkezi",
		detector.ClassSupplyDepot: "İkmal Deposu",
	}

	allegLabels := map[Allegiance]string{
		AllegianceFriendly: "DOST",
		AllegianceHostile:  "DÜŞMAN",
		AllegianceNeutral:  "TARAFSIZ",
		AllegianceUnknown:  "BİLİNMEYEN",
	}

	label := labels[cls]
	if label == "" {
		label = string(cls)
	}
	return fmt.Sprintf("[%s] %s", allegLabels[alleg], label)
}

// Stats sınıflandırma istatistikleri.
type Stats struct {
	TotalClassified int64               `json:"total_classified"`
	FriendlyCount   int64               `json:"friendly_count"`
	HostileCount    int64               `json:"hostile_count"`
	NeutralCount    int64               `json:"neutral_count"`
	UnknownCount    int64               `json:"unknown_count"`
	IFFEnabled      bool                `json:"iff_enabled"`
	ClassCounts     map[string]int64    `json:"class_counts"`
	CategoryCounts  map[string]int64    `json:"category_counts"`
}

// GetStats istatistikleri döndürür.
func (e *Engine) GetStats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cc := make(map[string]int64, len(e.classCounts))
	for k, v := range e.classCounts {
		cc[string(k)] = v
	}
	catc := make(map[string]int64, len(e.catCounts))
	for k, v := range e.catCounts {
		catc[string(k)] = v
	}

	return Stats{
		TotalClassified: atomic.LoadInt64(&e.totalClassified),
		FriendlyCount:   atomic.LoadInt64(&e.friendlyCount),
		HostileCount:    atomic.LoadInt64(&e.hostileCount),
		NeutralCount:    atomic.LoadInt64(&e.neutralCount),
		UnknownCount:    atomic.LoadInt64(&e.unknownCount),
		IFFEnabled:      e.iffEnabled,
		ClassCounts:     cc,
		CategoryCounts:  catc,
	}
}
