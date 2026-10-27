package broadcast

import (
	"bytes"
	"testing"
	"time"

	"github.com/otukenmatris/yayin/internal/compress"
	"github.com/otukenmatris/yayin/internal/crypto"
	"github.com/otukenmatris/yayin/internal/fec"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	eng, err := crypto.NewEngine(3)
	if err != nil {
		t.Fatalf("crypto.NewEngine hatası: %v", err)
	}
	comp := compress.NewCompressor(6)
	fecEnc, err := fec.NewEncoder(4, 256)
	if err != nil {
		t.Fatalf("fec.NewEncoder hatası: %v", err)
	}
	return NewManager(eng, comp, fecEnc, 100, 100, 10*1024*1024, 100, 24*time.Hour)
}

func TestCreateChannel(t *testing.T) {
	m := newTestManager(t)
	ch, err := m.CreateChannel("komut", "Komut kanalı", PriorityHigh, true, true, true)
	if err != nil {
		t.Fatalf("CreateChannel hatası: %v", err)
	}
	if ch.Name != "komut" {
		t.Errorf("Kanal adı beklenen: komut, alınan: %s", ch.Name)
	}
	if !ch.Encrypted || !ch.Compressed || !ch.FEC {
		t.Error("Kanal özellikleri doğru set edilmemiş")
	}
}

func TestCreateDuplicateChannel(t *testing.T) {
	m := newTestManager(t)
	m.CreateChannel("komut", "Komut", PriorityHigh, true, true, true)
	_, err := m.CreateChannel("komut", "Komut2", PriorityNormal, false, false, false)
	if err == nil {
		t.Fatal("Aynı isimli kanal için hata bekleniyor")
	}
}

func TestDeleteChannel(t *testing.T) {
	m := newTestManager(t)
	ch, _ := m.CreateChannel("silinecek", "Test", PriorityLow, false, false, false)
	err := m.DeleteChannel(ch.ID)
	if err != nil {
		t.Fatalf("DeleteChannel hatası: %v", err)
	}
	_, err = m.GetChannel(ch.ID)
	if err == nil {
		t.Fatal("Silinen kanal hâlâ bulunuyor")
	}
}

func TestSubscribeUnsubscribe(t *testing.T) {
	m := newTestManager(t)
	ch, _ := m.CreateChannel("sensor", "Sensör", PriorityNormal, false, false, false)

	sub, err := m.Subscribe(ch.ID, "drone-001")
	if err != nil {
		t.Fatalf("Subscribe hatası: %v", err)
	}
	if sub.UnitID != "drone-001" {
		t.Errorf("UnitID beklenen: drone-001, alınan: %s", sub.UnitID)
	}

	// Tekrar abone — aynı nesne dönmeli
	sub2, err := m.Subscribe(ch.ID, "drone-001")
	if err != nil {
		t.Fatalf("Tekrar subscribe hatası: %v", err)
	}
	if sub2.ID != sub.ID {
		t.Error("Aynı birime yeni abonelik oluşturulmamalı")
	}

	err = m.Unsubscribe(ch.ID, "drone-001")
	if err != nil {
		t.Fatalf("Unsubscribe hatası: %v", err)
	}
}

func TestPublishPlain(t *testing.T) {
	m := newTestManager(t)
	ch, _ := m.CreateChannel("plain", "Düz kanal", PriorityNormal, false, false, false)
	m.Subscribe(ch.ID, "unit-a")

	payload := []byte("Mevziye geçin!")
	msg, err := m.Publish(ch.ID, "komutan", "command", payload, PriorityHigh)
	if err != nil {
		t.Fatalf("Publish hatası: %v", err)
	}
	if msg.Compressed || msg.Encrypted || msg.FECApplied {
		t.Error("Düz kanalda işlem uygulanmamalı")
	}
	if !bytes.Equal(msg.Wire, payload) {
		t.Error("Wire verisi orijinal payload ile eşleşmiyor")
	}
}

func TestPublishEncryptedCompressed(t *testing.T) {
	m := newTestManager(t)
	ch, _ := m.CreateChannel("secure", "Güvenli kanal", PriorityCritical, true, true, true)
	m.Subscribe(ch.ID, "unit-b")

	payload := []byte("39.9334N 32.8597E koordinatına top ateşi! Tekrar: top ateşi! Hemen harekete geçin. Gizli emir.")
	msg, err := m.Publish(ch.ID, "komutan", "command", payload, PriorityCritical)
	if err != nil {
		t.Fatalf("Publish hatası: %v", err)
	}
	if !msg.Compressed || !msg.Encrypted || !msg.FECApplied {
		t.Errorf("İşlem uygulanmamış: comp=%v enc=%v fec=%v", msg.Compressed, msg.Encrypted, msg.FECApplied)
	}

	// Wire verisi orijinalden farklı olmalı
	if bytes.Equal(msg.Wire, payload) {
		t.Error("Şifreli wire veri orijinal ile aynı!")
	}

	// Decode ile orijinal geri gelmeli
	decoded, err := m.Decode(msg)
	if err != nil {
		t.Fatalf("Decode hatası: %v", err)
	}
	if !bytes.Equal(decoded, payload) {
		t.Errorf("Decode sonucu eşleşmiyor: %q", decoded)
	}
}

func TestHistory(t *testing.T) {
	m := newTestManager(t)
	ch, _ := m.CreateChannel("hist", "Geçmiş", PriorityNormal, false, false, false)

	for i := 0; i < 5; i++ {
		m.Publish(ch.ID, "sender", "command", []byte("mesaj"), PriorityNormal)
	}

	hist, err := m.History(ch.ID, 3)
	if err != nil {
		t.Fatalf("History hatası: %v", err)
	}
	if len(hist) != 3 {
		t.Errorf("Geçmiş sayısı beklenen: 3, alınan: %d", len(hist))
	}
}

func TestListChannels(t *testing.T) {
	m := newTestManager(t)
	m.CreateChannel("ch1", "Kanal 1", PriorityLow, false, false, false)
	m.CreateChannel("ch2", "Kanal 2", PriorityHigh, true, true, true)

	channels := m.ListChannels()
	if len(channels) != 2 {
		t.Errorf("Kanal sayısı beklenen: 2, alınan: %d", len(channels))
	}
}

func TestChannelSubscribers(t *testing.T) {
	m := newTestManager(t)
	ch, _ := m.CreateChannel("subs", "Abone test", PriorityNormal, false, false, false)
	m.Subscribe(ch.ID, "unit-1")
	m.Subscribe(ch.ID, "unit-2")

	subs, err := m.ChannelSubscribers(ch.ID)
	if err != nil {
		t.Fatalf("ChannelSubscribers hatası: %v", err)
	}
	if len(subs) != 2 {
		t.Errorf("Abone sayısı beklenen: 2, alınan: %d", len(subs))
	}
}

func TestPublishTooLarge(t *testing.T) {
	m := newTestManager(t)
	ch, _ := m.CreateChannel("limit", "Limit", PriorityNormal, false, false, false)
	bigData := make([]byte, 11*1024*1024) // 11 MB > 10 MB limit
	_, err := m.Publish(ch.ID, "sender", "data", bigData, PriorityNormal)
	if err == nil {
		t.Fatal("Büyük mesaj için hata bekleniyor")
	}
}

func TestCleanExpired(t *testing.T) {
	eng, _ := crypto.NewEngine(3)
	comp := compress.NewCompressor(6)
	fecEnc, _ := fec.NewEncoder(4, 256)
	m := NewManager(eng, comp, fecEnc, 100, 100, 10*1024*1024, 100, 1*time.Millisecond)

	ch, _ := m.CreateChannel("expire", "Expire test", PriorityNormal, false, false, false)
	m.Publish(ch.ID, "sender", "command", []byte("eski mesaj"), PriorityNormal)

	time.Sleep(10 * time.Millisecond)
	cleaned := m.CleanExpired()
	if cleaned < 1 {
		t.Errorf("Temizlenen beklenen: >=1, alınan: %d", cleaned)
	}
}

func TestStats(t *testing.T) {
	m := newTestManager(t)
	ch, _ := m.CreateChannel("stat", "Stats", PriorityNormal, true, true, false)
	m.Subscribe(ch.ID, "unit-s")
	m.Publish(ch.ID, "sender", "command", []byte("test"), PriorityNormal)

	s := m.Stats()
	if s.TotalPublished != 1 {
		t.Errorf("TotalPublished beklenen: 1, alınan: %d", s.TotalPublished)
	}
	if s.TotalChannels != 1 {
		t.Errorf("TotalChannels beklenen: 1, alınan: %d", s.TotalChannels)
	}
	if s.TotalSubscribers != 1 {
		t.Errorf("TotalSubscribers beklenen: 1, alınan: %d", s.TotalSubscribers)
	}
}
