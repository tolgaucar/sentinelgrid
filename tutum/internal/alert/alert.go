// Package alert — Çok seviyeli uyarı yönetimi ve otomatik müdahale.
//
// Batarya ve kaynak durumlarına göre uyarılar üretir:
//   - CRITICAL: Hemen müdahale (geri dönüş, görev devri)
//   - WARNING: Planlama gerekli (yaklaşan enerji sınırı)
//   - INFO: Bilgilendirme (durum değişikliği)
//
// Deduplikasyon, önceliklendirme ve otomatik RTB önerisi sağlar.
package alert

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Severity uyarı ciddiyeti.
type Severity string

const (
	SevInfo     Severity = "info"
	SevWarning  Severity = "warning"
	SevCritical Severity = "critical"
)

// Category uyarı kategorisi.
type Category string

const (
	CatBattery     Category = "battery"
	CatResource    Category = "resource"
	CatPrediction  Category = "prediction"
	CatMission     Category = "mission"
	CatRTB         Category = "rtb"       // Return To Base
	CatHandoff     Category = "handoff"   // Görev devri
	CatTemperature Category = "temperature"
)

// Alert tek bir uyarı.
type Alert struct {
	ID         string    `json:"id"`
	PlatformID string    `json:"platform_id"`
	Severity   Severity  `json:"severity"`
	Category   Category  `json:"category"`
	Title      string    `json:"title"`
	Message    string    `json:"message"`
	Value      float64   `json:"value,omitempty"`
	Threshold  float64   `json:"threshold,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
	Acked      bool      `json:"acknowledged"`
	AckedAt    time.Time `json:"acked_at,omitempty"`
	Resolved   bool      `json:"resolved"`
	ResolvedAt time.Time `json:"resolved_at,omitempty"`
}

// RTBRecommendation geri dönüş önerisi.
type RTBRecommendation struct {
	PlatformID       string  `json:"platform_id"`
	CurrentSoC       float64 `json:"current_soc_percent"`
	PredictedMinutes float64 `json:"predicted_remaining_minutes"`
	Urgency          string  `json:"urgency"` // "immediate", "soon", "planned"
	Reason           string  `json:"reason"`
}

// HandoffRecommendation görev devri önerisi.
type HandoffRecommendation struct {
	FromPlatformID string  `json:"from_platform_id"`
	Reason         string  `json:"reason"`
	RemainingMin   float64 `json:"remaining_minutes"`
	CurrentSoC     float64 `json:"current_soc_percent"`
}

// Manager uyarı yöneticisi.
type Manager struct {
	mu sync.RWMutex

	alerts    []*Alert
	maxAlerts int
	retention time.Duration
	dedup     time.Duration
	autoRTB   bool

	// Deduplikasyon: platform+category → son alert zamanı
	lastAlert map[string]time.Time

	// İstatistikler
	totalGenerated int64
	totalAcked     int64
	totalResolved  int64
	seqID          int64

	// RTB ve handoff önerileri
	rtbRecs     map[string]*RTBRecommendation
	handoffRecs map[string]*HandoffRecommendation
}

// NewManager yeni uyarı yöneticisi.
func NewManager(maxAlerts int, retention, dedup time.Duration, autoRTB bool) *Manager {
	return &Manager{
		alerts:      make([]*Alert, 0, 256),
		maxAlerts:   maxAlerts,
		retention:   retention,
		dedup:       dedup,
		autoRTB:     autoRTB,
		lastAlert:   make(map[string]time.Time),
		rtbRecs:     make(map[string]*RTBRecommendation),
		handoffRecs: make(map[string]*HandoffRecommendation),
	}
}

// Generate yeni uyarı üretir. Deduplikasyon kontrolü yapar.
func (m *Manager) Generate(platformID string, sev Severity, cat Category, title, message string, value, threshold float64) *Alert {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Deduplikasyon kontrolü
	key := fmt.Sprintf("%s:%s", platformID, cat)
	if last, ok := m.lastAlert[key]; ok {
		if time.Since(last) < m.dedup {
			return nil // Zaten yakın zamanda aynı uyarı var
		}
	}

	id := atomic.AddInt64(&m.seqID, 1)
	a := &Alert{
		ID:         fmt.Sprintf("ALR-%06d", id),
		PlatformID: platformID,
		Severity:   sev,
		Category:   cat,
		Title:      title,
		Message:    message,
		Value:      value,
		Threshold:  threshold,
		Timestamp:  time.Now(),
	}

	m.alerts = append(m.alerts, a)
	m.lastAlert[key] = time.Now()
	atomic.AddInt64(&m.totalGenerated, 1)

	// Max kontrolü: en eski resolved olanları sil
	m.trimAlerts()

	return a
}

// CheckBattery batarya durumunu kontrol edip uyarı üretir.
func (m *Manager) CheckBattery(platformID string, socPercent float64, criticalPct, warningPct, lowPct float64) *Alert {
	if socPercent <= criticalPct {
		a := m.Generate(platformID, SevCritical, CatBattery,
			"KRİTİK BATARYA",
			fmt.Sprintf("%s: Batarya %%%0.1f — Acil geri dönüş gerekli!", platformID, socPercent),
			socPercent, criticalPct)

		// Otomatik RTB önerisi
		if m.autoRTB && a != nil {
			m.mu.Lock()
			m.rtbRecs[platformID] = &RTBRecommendation{
				PlatformID: platformID,
				CurrentSoC: socPercent,
				Urgency:    "immediate",
				Reason:     fmt.Sprintf("Batarya kritik seviyede: %%%.1f", socPercent),
			}
			m.mu.Unlock()
		}
		return a
	}

	if socPercent <= warningPct {
		return m.Generate(platformID, SevWarning, CatBattery,
			"DÜŞÜK BATARYA",
			fmt.Sprintf("%s: Batarya %%%0.1f — Görev planlaması gözden geçirilmeli", platformID, socPercent),
			socPercent, warningPct)
	}

	if socPercent <= lowPct {
		return m.Generate(platformID, SevInfo, CatBattery,
			"Batarya Azalıyor",
			fmt.Sprintf("%s: Batarya %%%0.1f — İzlemeye devam", platformID, socPercent),
			socPercent, lowPct)
	}

	return nil
}

// CheckPrediction tahmin sonucuna göre uyarı.
func (m *Manager) CheckPrediction(platformID string, predictedMin float64, rtbFeasible bool) *Alert {
	if predictedMin > 0 && predictedMin < 10 {
		a := m.Generate(platformID, SevCritical, CatPrediction,
			"BATARYA ÖMRÜ KRİTİK",
			fmt.Sprintf("%s: Tahmini kalan süre %.0f dakika — Hemen müdahale!", platformID, predictedMin),
			predictedMin, 10)

		if m.autoRTB && a != nil {
			m.mu.Lock()
			m.rtbRecs[platformID] = &RTBRecommendation{
				PlatformID:       platformID,
				PredictedMinutes: predictedMin,
				Urgency:          "immediate",
				Reason:           fmt.Sprintf("Kalan süre %.0f dk", predictedMin),
			}
			m.mu.Unlock()
		}
		return a
	}

	if predictedMin > 0 && predictedMin < 30 {
		a := m.Generate(platformID, SevWarning, CatPrediction,
			"Batarya Ömrü Azalıyor",
			fmt.Sprintf("%s: Tahmini kalan süre %.0f dakika", platformID, predictedMin),
			predictedMin, 30)

		if m.autoRTB && !rtbFeasible && a != nil {
			m.mu.Lock()
			m.rtbRecs[platformID] = &RTBRecommendation{
				PlatformID:       platformID,
				PredictedMinutes: predictedMin,
				Urgency:          "soon",
				Reason:           "RTB için yeterli enerji kalmayabilir",
			}
			m.mu.Unlock()
		}
		return a
	}

	return nil
}

// CheckTemperature sıcaklık kontrolü.
func (m *Manager) CheckTemperature(platformID string, tempC, warningC, criticalC float64) *Alert {
	if tempC >= criticalC {
		return m.Generate(platformID, SevCritical, CatTemperature,
			"KRİTİK SICAKLIK",
			fmt.Sprintf("%s: Sıcaklık %.1f°C — Termal koruma aktif!", platformID, tempC),
			tempC, criticalC)
	}
	if tempC >= warningC {
		return m.Generate(platformID, SevWarning, CatTemperature,
			"Yüksek Sıcaklık",
			fmt.Sprintf("%s: Sıcaklık %.1f°C — Performans kısıtlaması olabilir", platformID, tempC),
			tempC, warningC)
	}
	return nil
}

// RecommendHandoff görev devri önerisi oluşturur.
func (m *Manager) RecommendHandoff(platformID string, remainingMin, currentSoC float64, reason string) *HandoffRecommendation {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec := &HandoffRecommendation{
		FromPlatformID: platformID,
		Reason:         reason,
		RemainingMin:   remainingMin,
		CurrentSoC:     currentSoC,
	}
	m.handoffRecs[platformID] = rec

	return rec
}

// Acknowledge uyarıyı onayla.
func (m *Manager) Acknowledge(alertID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.alerts {
		if a.ID == alertID && !a.Acked {
			a.Acked = true
			a.AckedAt = time.Now()
			atomic.AddInt64(&m.totalAcked, 1)
			return true
		}
	}
	return false
}

// Resolve uyarıyı çöz.
func (m *Manager) Resolve(alertID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.alerts {
		if a.ID == alertID && !a.Resolved {
			a.Resolved = true
			a.ResolvedAt = time.Now()
			atomic.AddInt64(&m.totalResolved, 1)
			return true
		}
	}
	return false
}

// Active çözülmemiş uyarılar.
func (m *Manager) Active() []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Alert
	for _, a := range m.alerts {
		if !a.Resolved {
			result = append(result, a)
		}
	}
	return result
}

// BySeverity ciddiyete göre filtrele.
func (m *Manager) BySeverity(sev Severity) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Alert
	for _, a := range m.alerts {
		if a.Severity == sev && !a.Resolved {
			result = append(result, a)
		}
	}
	return result
}

// ByPlatform platforma göre filtrele.
func (m *Manager) ByPlatform(platformID string) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Alert
	for _, a := range m.alerts {
		if a.PlatformID == platformID {
			result = append(result, a)
		}
	}
	return result
}

// GetRTBRecommendations RTB önerilerini döndürür.
func (m *Manager) GetRTBRecommendations() map[string]*RTBRecommendation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*RTBRecommendation, len(m.rtbRecs))
	for k, v := range m.rtbRecs {
		result[k] = v
	}
	return result
}

// GetHandoffRecommendations görev devri önerilerini döndürür.
func (m *Manager) GetHandoffRecommendations() map[string]*HandoffRecommendation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*HandoffRecommendation, len(m.handoffRecs))
	for k, v := range m.handoffRecs {
		result[k] = v
	}
	return result
}

// ClearRTB RTB önerisini temizler (platforma ulaştı).
func (m *Manager) ClearRTB(platformID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rtbRecs, platformID)
}

// Stats uyarı istatistikleri.
type Stats struct {
	TotalGenerated int64 `json:"total_generated"`
	TotalAcked     int64 `json:"total_acknowledged"`
	TotalResolved  int64 `json:"total_resolved"`
	ActiveCount    int   `json:"active_count"`
	CriticalCount  int   `json:"critical_count"`
	WarningCount   int   `json:"warning_count"`
	InfoCount      int   `json:"info_count"`
	RTBCount       int   `json:"rtb_recommendation_count"`
	HandoffCount   int   `json:"handoff_recommendation_count"`
}

// GetStats istatistikleri döndürür.
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s := Stats{
		TotalGenerated: atomic.LoadInt64(&m.totalGenerated),
		TotalAcked:     atomic.LoadInt64(&m.totalAcked),
		TotalResolved:  atomic.LoadInt64(&m.totalResolved),
		RTBCount:       len(m.rtbRecs),
		HandoffCount:   len(m.handoffRecs),
	}

	for _, a := range m.alerts {
		if !a.Resolved {
			s.ActiveCount++
			switch a.Severity {
			case SevCritical:
				s.CriticalCount++
			case SevWarning:
				s.WarningCount++
			case SevInfo:
				s.InfoCount++
			}
		}
	}
	return s
}

// Cleanup eski uyarıları temizler.
func (m *Manager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.retention)
	kept := make([]*Alert, 0, len(m.alerts))
	removed := 0
	for _, a := range m.alerts {
		if a.Resolved && a.Timestamp.Before(cutoff) {
			removed++
			continue
		}
		kept = append(kept, a)
	}
	m.alerts = kept
	return removed
}

func (m *Manager) trimAlerts() {
	if len(m.alerts) <= m.maxAlerts {
		return
	}
	// Resolved olanları önce sil
	kept := make([]*Alert, 0, m.maxAlerts)
	for _, a := range m.alerts {
		if !a.Resolved {
			kept = append(kept, a)
		}
	}
	// Hâlâ fazlaysa en eskileri sil
	if len(kept) > m.maxAlerts {
		kept = kept[len(kept)-m.maxAlerts:]
	}
	m.alerts = kept
}
