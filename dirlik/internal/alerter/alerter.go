// Package alerter — Alarm üretme ve dış modüllere bildirim.
// BÜTÜN modülüne crash feedback gönderir, BAĞ üzerinden alert yayınlar.
package alerter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/otukenmatris/dirlik/internal/anomaly"
	"github.com/otukenmatris/dirlik/internal/probe"
)

// AlertAction — Alarm sonrası aksiyon tipi
type AlertAction string

const (
	ActionNotify     AlertAction = "notify"
	ActionRestart    AlertAction = "restart"
	ActionScaleUp    AlertAction = "scale_up"
	ActionIsolate    AlertAction = "isolate"
)

// AlertRecord — Gönderilen alarm kaydı
type AlertRecord struct {
	Alert     anomaly.Alert `json:"alert"`
	Action    AlertAction   `json:"action"`
	Target    string        `json:"target"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// Alerter — Alarm yöneticisi
type Alerter struct {
	mu        sync.RWMutex
	butunURL  string
	bagURL    string
	client    *http.Client
	records   []AlertRecord
	maxRecords int
	sentCount int64
	failCount int64
	wg        sync.WaitGroup

	// Tekrar gönderimi önleme (aynı servis 60s içinde)
	cooldowns map[string]time.Time
	cooldownD time.Duration
}

// New — Yeni alerter oluştur
func New(butunURL, bagURL string) *Alerter {
	return &Alerter{
		butunURL:   butunURL,
		bagURL:     bagURL,
		client:     &http.Client{Timeout: 5 * time.Second},
		records:    make([]AlertRecord, 0, 1000),
		maxRecords: 10000,
		cooldowns:  make(map[string]time.Time),
		cooldownD:  60 * time.Second,
	}
}

// HandleAnomaly — Anomali alarmını işle
func (a *Alerter) HandleAnomaly(alert anomaly.Alert) {
	a.mu.Lock()

	// Cooldown kontrolü
	key := fmt.Sprintf("%s:%s", alert.Metric, alert.ServiceID)
	if t, ok := a.cooldowns[key]; ok && time.Since(t) < a.cooldownD {
		a.mu.Unlock()
		return
	}
	a.cooldowns[key] = time.Now()
	a.mu.Unlock()

	// Aksiyonu belirle
	action := ActionNotify
	if alert.Severity >= anomaly.SeverityCritical {
		if alert.ServiceID != "" {
			action = ActionRestart
		} else {
			action = ActionScaleUp
		}
	}

	// BAĞ'a alarm yayınla
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.publishToBag(alert)
	}()

	// BÜTÜN'e bildir (crash → restart tetikle)
	if alert.ServiceID != "" && alert.Severity >= anomaly.SeverityWarning {
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			a.notifyButun(alert, action)
		}()
	}

	// Kayıt
	a.mu.Lock()
	a.records = append(a.records, AlertRecord{
		Alert:     alert,
		Action:    action,
		Target:    "bag+butun",
		Success:   true,
		Timestamp: time.Now(),
	})
	if len(a.records) > a.maxRecords {
		a.records = a.records[1:]
	}
	atomic.AddInt64(&a.sentCount, 1)
	a.mu.Unlock()
}

// HandleServiceDown — Servis çökmesini işle
func (a *Alerter) HandleServiceDown(serviceID string, oldStatus, newStatus probe.ServiceStatus) {
	if newStatus != probe.StatusDown {
		return
	}

	alert := anomaly.Alert{
		ID:        fmt.Sprintf("SVC-%s-%d", serviceID, time.Now().UnixMilli()),
		Metric:    "service_down",
		Value:     0,
		Threshold: 1,
		Severity:  anomaly.SeverityCritical,
		Message:   fmt.Sprintf("Servis çöktü: %s", serviceID),
		ServiceID: serviceID,
		Timestamp: time.Now(),
	}

	// BÜTÜN'e acil bildirim → replica tetikle
	a.wg.Add(2)
	go func() {
		defer a.wg.Done()
		a.notifyButun(alert, ActionRestart)
	}()
	go func() {
		defer a.wg.Done()
		a.publishToBag(alert)
	}()

	a.mu.Lock()
	a.records = append(a.records, AlertRecord{
		Alert:     alert,
		Action:    ActionRestart,
		Target:    "butun",
		Success:   true,
		Timestamp: time.Now(),
	})
	atomic.AddInt64(&a.sentCount, 1)
	a.mu.Unlock()
}

// Records — Alarm geçmişi (son N)
func (a *Alerter) Records(n int) []AlertRecord {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if n <= 0 || n > len(a.records) {
		n = len(a.records)
	}
	start := len(a.records) - n
	result := make([]AlertRecord, n)
	copy(result, a.records[start:])
	return result
}

// Stats — İstatistikler
func (a *Alerter) Stats() map[string]int64 {
	return map[string]int64{
		"sent":   atomic.LoadInt64(&a.sentCount),
		"failed": atomic.LoadInt64(&a.failCount),
	}
}

func (a *Alerter) publishToBag(alert anomaly.Alert) {
	topic := "alert.warning"
	if alert.Severity >= anomaly.SeverityCritical {
		topic = "alert.critical"
	}

	payload := map[string]interface{}{
		"topic":   topic,
		"key":     alert.ID,
		"payload": alert,
	}
	data, _ := json.Marshal(payload)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", a.bagURL+"/publish", bytes.NewReader(data))
	if err != nil {
		atomic.AddInt64(&a.failCount, 1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		atomic.AddInt64(&a.failCount, 1)
		return
	}
	resp.Body.Close()
}

func (a *Alerter) notifyButun(alert anomaly.Alert, action AlertAction) {
	payload := map[string]interface{}{
		"service_id": alert.ServiceID,
		"alert":      alert,
		"action":     action,
		"source":     "dirlik",
		"timestamp":  time.Now(),
	}
	data, _ := json.Marshal(payload)

	// BÜTÜN'e sistem olayı gönder
	endpoint := a.butunURL + "/api/v1/services/" + alert.ServiceID + "/heartbeat"
	if action == ActionRestart {
		endpoint = a.butunURL + "/api/v1/orchestrator/restart/" + alert.ServiceID
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		atomic.AddInt64(&a.failCount, 1)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Source", "dirlik")

	resp, err := a.client.Do(req)
	if err != nil {
		atomic.AddInt64(&a.failCount, 1)
		return
	}
	resp.Body.Close()
}
