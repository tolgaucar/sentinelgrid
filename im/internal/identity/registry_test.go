package identity

import (
	"testing"
)

func TestRegister(t *testing.T) {
	r := NewRegistry()
	unit, err := r.Register("drone-001", "Keşif Dronu 1", "drone", nil)
	if err != nil {
		t.Fatalf("Register hatası: %v", err)
	}
	if unit.ID != "drone-001" {
		t.Errorf("ID beklenen: drone-001, alınan: %s", unit.ID)
	}
	if unit.Status != StatusActive {
		t.Errorf("Status beklenen: active, alınan: %s", unit.Status)
	}
	if len(unit.Permissions) == 0 {
		t.Error("İzinler boş")
	}
}

func TestRegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	r.Register("u1", "Birim 1", "drone", nil)
	_, err := r.Register("u1", "Birim 1 Copy", "drone", nil)
	if err == nil {
		t.Fatal("Çift kayıt hatası bekleniyor")
	}
}

func TestGet(t *testing.T) {
	r := NewRegistry()
	r.Register("tank-001", "Tank 1", "vehicle", nil)

	unit, ok := r.Get("tank-001")
	if !ok {
		t.Fatal("Birim bulunamadı")
	}
	if unit.Type != "vehicle" {
		t.Errorf("Type beklenen: vehicle, alınan: %s", unit.Type)
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Fatal("Olmayan birim bulunamazdı")
	}
}

func TestRolePermissions(t *testing.T) {
	r := NewRegistry()
	unit, _ := r.Register("cmd-001", "Komutan", "commander", []string{"commander"})

	// Commander admin izni almalı
	hasAdmin := false
	for _, p := range unit.Permissions {
		if p == "admin" {
			hasAdmin = true
			break
		}
	}
	if !hasAdmin {
		t.Errorf("Commander admin iznine sahip olmalı, izinler: %v", unit.Permissions)
	}
}

func TestAuthorize(t *testing.T) {
	r := NewRegistry()
	r.Register("cmd-001", "Komutan", "commander", []string{"commander"})
	r.Register("drone-001", "Dron", "drone", []string{"drone"})

	// Commander admin izni olmalı
	if err := r.Authorize("cmd-001", "admin"); err != nil {
		t.Errorf("Commander admin yetkisi olmalı: %v", err)
	}

	// Drone admin izni olmamalı
	if err := r.Authorize("drone-001", "admin"); err == nil {
		t.Error("Drone admin yetkisi olmamalı")
	}

	// Drone read izni olmalı
	if err := r.Authorize("drone-001", "read"); err != nil {
		t.Errorf("Drone read yetkisi olmalı: %v", err)
	}
}

func TestSuspendAndReactivate(t *testing.T) {
	r := NewRegistry()
	r.Register("u1", "Birim", "drone", nil)

	// Askıya al
	if err := r.Suspend("u1"); err != nil {
		t.Fatalf("Suspend hatası: %v", err)
	}
	u, _ := r.Get("u1")
	if u.Status != StatusSuspend {
		t.Errorf("Status beklenen: suspended, alınan: %s", u.Status)
	}

	// Askıya alınmış birim authorize olmamalı
	if err := r.Authorize("u1", "read"); err == nil {
		t.Error("Askıya alınmış birim authorize olmamalı")
	}

	// Tekrar aktifleştir
	if err := r.Reactivate("u1"); err != nil {
		t.Fatalf("Reactivate hatası: %v", err)
	}
	u, _ = r.Get("u1")
	if u.Status != StatusActive {
		t.Errorf("Status beklenen: active, alınan: %s", u.Status)
	}
}

func TestRevoke(t *testing.T) {
	r := NewRegistry()
	r.Register("u2", "Birim", "drone", nil)

	if err := r.Revoke("u2"); err != nil {
		t.Fatalf("Revoke hatası: %v", err)
	}
	u, _ := r.Get("u2")
	if u.Status != StatusRevoked {
		t.Errorf("Status beklenen: revoked, alınan: %s", u.Status)
	}

	// İptal edilen birim askıya alınamaz
	if err := r.Suspend("u2"); err == nil {
		t.Error("İptal edilen birim askıya alınamamalı")
	}

	stats := r.Stats()
	if stats.RevokedUnits != 1 {
		t.Errorf("RevokedUnits beklenen: 1, alınan: %d", stats.RevokedUnits)
	}
}

func TestReactivateNotSuspended(t *testing.T) {
	r := NewRegistry()
	r.Register("u3", "Birim", "drone", nil)

	err := r.Reactivate("u3")
	if err == nil {
		t.Error("Aktif birim tekrar aktifleştirilemez")
	}
}

func TestHeartbeat(t *testing.T) {
	r := NewRegistry()
	r.Register("u4", "Birim", "drone", nil)

	if err := r.Heartbeat("u4"); err != nil {
		t.Fatalf("Heartbeat hatası: %v", err)
	}

	// Askıya alınmış birim heartbeat atamaz
	r.Suspend("u4")
	if err := r.Heartbeat("u4"); err == nil {
		t.Error("Askıya alınmış birim heartbeat atamamalı")
	}
}

func TestListByType(t *testing.T) {
	r := NewRegistry()
	r.Register("d1", "Dron 1", "drone", nil)
	r.Register("d2", "Dron 2", "drone", nil)
	r.Register("t1", "Tank 1", "vehicle", nil)

	drones := r.ListByType("drone")
	if len(drones) != 2 {
		t.Errorf("Beklenen 2 drone, alınan: %d", len(drones))
	}

	all := r.ListAll()
	if len(all) != 3 {
		t.Errorf("Beklenen 3 birim, alınan: %d", len(all))
	}
}

func TestSetCertSerial(t *testing.T) {
	r := NewRegistry()
	r.Register("u5", "Birim", "drone", nil)

	if err := r.SetCertSerial("u5", "CERT-123"); err != nil {
		t.Fatalf("SetCertSerial hatası: %v", err)
	}

	u, _ := r.Get("u5")
	if u.CertSerial != "CERT-123" {
		t.Errorf("CertSerial beklenen: CERT-123, alınan: %s", u.CertSerial)
	}
}

func TestStats(t *testing.T) {
	r := NewRegistry()
	r.Register("a", "A", "drone", nil)
	r.Register("b", "B", "drone", nil)
	r.Register("c", "C", "drone", nil)
	r.Suspend("b")
	r.Revoke("c")

	stats := r.Stats()
	if stats.TotalRegistered != 3 {
		t.Errorf("TotalRegistered beklenen: 3, alınan: %d", stats.TotalRegistered)
	}
	if stats.ActiveUnits != 1 {
		t.Errorf("ActiveUnits beklenen: 1, alınan: %d", stats.ActiveUnits)
	}
	if stats.SuspendedUnits != 1 {
		t.Errorf("SuspendedUnits beklenen: 1, alınan: %d", stats.SuspendedUnits)
	}
	if stats.RevokedUnits != 1 {
		t.Errorf("RevokedUnits beklenen: 1, alınan: %d", stats.RevokedUnits)
	}
}
