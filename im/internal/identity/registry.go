// Package identity — Birim kimlik kaydı ve yönetimi.
//
// Sahadaki tüm birimlerin (piyade, dron, sensör, araç) kayıt
// durumlarını, rollerini ve erişim izinlerini yönetir.
// Zero Trust modeline göre her birim kaydolmalıdır.
package identity

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// UnitStatus birim kayıt durumu.
type UnitStatus string

const (
	StatusPending  UnitStatus = "pending"
	StatusActive   UnitStatus = "active"
	StatusSuspend  UnitStatus = "suspended"
	StatusRevoked  UnitStatus = "revoked"
)

// Unit bir kayıtlı birim.
type Unit struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Type         string     `json:"type"`           // drone, infantry, tank, sensor, relay, commander
	Roles        []string   `json:"roles"`           // roller
	Permissions  []string   `json:"permissions"`     // izinler
	Status       UnitStatus `json:"status"`
	CertSerial   string     `json:"cert_serial"`     // PKI sertifika seri no
	RegisteredAt time.Time  `json:"registered_at"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Registry birim kimlik kaydı.
type Registry struct {
	mu    sync.RWMutex
	units map[string]*Unit
	stats RegistryStats
}

// RegistryStats kayıt istatistikleri.
type RegistryStats struct {
	TotalRegistered int64 `json:"total_registered"`
	ActiveUnits     int64 `json:"active_units"`
	SuspendedUnits  int64 `json:"suspended_units"`
	RevokedUnits    int64 `json:"revoked_units"`
}

// RolePermissions roller için varsayılan izinler.
var RolePermissions = map[string][]string{
	"commander": {"read", "write", "admin", "command", "revoke"},
	"sensor":    {"read", "telemetry.write"},
	"relay":     {"read", "forward"},
	"drone":     {"read", "telemetry.write", "command.receive"},
	"infantry":  {"read", "write", "command.receive"},
	"vehicle":   {"read", "write", "telemetry.write", "command.receive"},
	"observer":  {"read"},
}

// NewRegistry yeni bir kimlik kaydı oluşturur.
func NewRegistry() *Registry {
	return &Registry{
		units: make(map[string]*Unit),
	}
}

// Register yeni bir birim kaydeder.
func (r *Registry) Register(id, name, unitType string, roles []string) (*Unit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.units[id]; exists {
		return nil, fmt.Errorf("birim zaten kayıtlı: %s", id)
	}

	if len(roles) == 0 {
		roles = []string{unitType}
	}

	// Roller için izinleri birleştir
	permSet := make(map[string]bool)
	for _, role := range roles {
		if perms, ok := RolePermissions[role]; ok {
			for _, p := range perms {
				permSet[p] = true
			}
		}
	}
	perms := make([]string, 0, len(permSet))
	for p := range permSet {
		perms = append(perms, p)
	}

	unit := &Unit{
		ID:           id,
		Name:         name,
		Type:         unitType,
		Roles:        roles,
		Permissions:  perms,
		Status:       StatusActive,
		RegisteredAt: time.Now(),
		LastSeenAt:   time.Now(),
		Metadata:     make(map[string]string),
	}

	r.units[id] = unit
	atomic.AddInt64(&r.stats.TotalRegistered, 1)
	atomic.AddInt64(&r.stats.ActiveUnits, 1)

	return unit, nil
}

// Get birim bilgisini döndürür.
func (r *Registry) Get(id string) (*Unit, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.units[id]
	if !ok {
		return nil, false
	}
	cp := *u
	cp.Roles = make([]string, len(u.Roles))
	copy(cp.Roles, u.Roles)
	cp.Permissions = make([]string, len(u.Permissions))
	copy(cp.Permissions, u.Permissions)
	return &cp, true
}

// SetCertSerial sertifika seri numarasını bağlar.
func (r *Registry) SetCertSerial(id, serial string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.units[id]
	if !ok {
		return fmt.Errorf("birim bulunamadı: %s", id)
	}
	u.CertSerial = serial
	return nil
}

// Heartbeat biriMin son görülme zamanını günceller.
func (r *Registry) Heartbeat(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.units[id]
	if !ok {
		return fmt.Errorf("birim bulunamadı: %s", id)
	}
	if u.Status != StatusActive {
		return fmt.Errorf("birim aktif değil: %s (durum: %s)", id, u.Status)
	}
	u.LastSeenAt = time.Now()
	return nil
}

// Suspend birimi askıya alır.
func (r *Registry) Suspend(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.units[id]
	if !ok {
		return fmt.Errorf("birim bulunamadı: %s", id)
	}
	if u.Status == StatusRevoked {
		return fmt.Errorf("iptal edilen birim askıya alınamaz: %s", id)
	}
	if u.Status == StatusActive {
		atomic.AddInt64(&r.stats.ActiveUnits, -1)
	}
	u.Status = StatusSuspend
	atomic.AddInt64(&r.stats.SuspendedUnits, 1)
	return nil
}

// Reactivate askıya alınmış birimi tekrar aktifleştirir.
func (r *Registry) Reactivate(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.units[id]
	if !ok {
		return fmt.Errorf("birim bulunamadı: %s", id)
	}
	if u.Status != StatusSuspend {
		return fmt.Errorf("sadece askıya alınmış birimler aktifleştirilebilir: %s (durum: %s)", id, u.Status)
	}
	u.Status = StatusActive
	u.LastSeenAt = time.Now()
	atomic.AddInt64(&r.stats.SuspendedUnits, -1)
	atomic.AddInt64(&r.stats.ActiveUnits, 1)
	return nil
}

// Revoke birimi kalıcı olarak iptal eder.
func (r *Registry) Revoke(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.units[id]
	if !ok {
		return fmt.Errorf("birim bulunamadı: %s", id)
	}
	prevStatus := u.Status
	u.Status = StatusRevoked
	switch prevStatus {
	case StatusActive:
		atomic.AddInt64(&r.stats.ActiveUnits, -1)
	case StatusSuspend:
		atomic.AddInt64(&r.stats.SuspendedUnits, -1)
	}
	atomic.AddInt64(&r.stats.RevokedUnits, 1)
	return nil
}

// Authorize birimIN belirli bir izne sahip olduğunu doğrular.
func (r *Registry) Authorize(id string, requiredPerm string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.units[id]
	if !ok {
		return fmt.Errorf("birim bulunamadı: %s", id)
	}
	if u.Status != StatusActive {
		return fmt.Errorf("birim aktif değil: %s (durum: %s)", id, u.Status)
	}
	for _, p := range u.Permissions {
		if p == requiredPerm || p == "admin" {
			return nil
		}
	}
	return fmt.Errorf("yetki reddedildi: %s için %s izni yok", id, requiredPerm)
}

// ListByType tipe göre birimleri listeler.
func (r *Registry) ListByType(unitType string) []Unit {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Unit
	for _, u := range r.units {
		if unitType == "" || u.Type == unitType {
			result = append(result, *u)
		}
	}
	return result
}

// ListAll tüm birimleri listeler.
func (r *Registry) ListAll() []Unit {
	return r.ListByType("")
}

// Stats istatistikleri döndürür.
func (r *Registry) Stats() RegistryStats {
	return RegistryStats{
		TotalRegistered: atomic.LoadInt64(&r.stats.TotalRegistered),
		ActiveUnits:     atomic.LoadInt64(&r.stats.ActiveUnits),
		SuspendedUnits:  atomic.LoadInt64(&r.stats.SuspendedUnits),
		RevokedUnits:    atomic.LoadInt64(&r.stats.RevokedUnits),
	}
}
