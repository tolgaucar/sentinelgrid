package event

import (
	"sync"
	"testing"
	"time"

	"github.com/otukenmatris/yuzey/internal/config"
)

func TestNewBus(t *testing.T) {
	b := NewBus(100)
	if b == nil {
		t.Fatal("Bus nil olmamalı")
	}
	if b.maxEvents != 100 {
		t.Errorf("maxEvents=100 bekleniyordu, %d geldi", b.maxEvents)
	}
}

func TestNewBusDefaultMax(t *testing.T) {
	b := NewBus(0)
	if b.maxEvents != 10000 {
		t.Errorf("varsayılan maxEvents=10000 bekleniyordu, %d geldi", b.maxEvents)
	}
}

func TestEmit(t *testing.T) {
	b := NewBus(100)
	evt := b.Emit("test_type", "src1", SevInfo, "Test Olayı", "detay", 39.9, 32.8)
	if evt.ID == "" {
		t.Error("Olay ID boş olmamalı")
	}
	if evt.Type != "test_type" {
		t.Errorf("Type=test_type bekleniyordu, %s geldi", evt.Type)
	}
	if evt.Source != "src1" {
		t.Errorf("Source=src1 bekleniyordu, %s geldi", evt.Source)
	}
	if evt.Severity != SevInfo {
		t.Errorf("Severity=info bekleniyordu, %s geldi", evt.Severity)
	}
	if evt.Timestamp == 0 {
		t.Error("Timestamp 0 olmamalı")
	}
}

func TestEmitSimple(t *testing.T) {
	b := NewBus(100)
	evt := b.EmitSimple("ping", "test", SevWarning, "Test")
	if evt.Type != "ping" {
		t.Errorf("Type=ping bekleniyordu, %s geldi", evt.Type)
	}
	if evt.Detail != "" {
		t.Error("EmitSimple detay boş olmalı")
	}
}

func TestSubscribe(t *testing.T) {
	b := NewBus(100)
	received := make(chan config.TacticalEvent, 1)
	b.Subscribe(func(evt config.TacticalEvent) {
		received <- evt
	})
	b.EmitSimple("sub_test", "test", SevInfo, "Abonelik Testi")
	select {
	case evt := <-received:
		if evt.Type != "sub_test" {
			t.Errorf("Type=sub_test bekleniyordu, %s geldi", evt.Type)
		}
	case <-time.After(time.Second):
		t.Error("Subscriber olayı almadı")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	b := NewBus(100)
	count := int32(0)
	var mu sync.Mutex
	for i := 0; i < 5; i++ {
		b.Subscribe(func(evt config.TacticalEvent) {
			mu.Lock()
			count++
			mu.Unlock()
		})
	}
	b.EmitSimple("multi", "test", SevInfo, "Çoklu Abone")
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	if count != 5 {
		t.Errorf("5 subscriber çağrılmalıydı, %d çağrıldı", count)
	}
	mu.Unlock()
}

func TestGetEvents(t *testing.T) {
	b := NewBus(100)
	for i := 0; i < 10; i++ {
		b.EmitSimple("test", "src", SevInfo, "Olay")
	}
	evts := b.GetEvents(5)
	if len(evts) != 5 {
		t.Errorf("5 olay bekleniyordu, %d geldi", len(evts))
	}
	all := b.GetEvents(0)
	if len(all) != 10 {
		t.Errorf("10 olay bekleniyordu, %d geldi", len(all))
	}
}

func TestGetEventsOverCount(t *testing.T) {
	b := NewBus(100)
	for i := 0; i < 3; i++ {
		b.EmitSimple("test", "src", SevInfo, "Olay")
	}
	evts := b.GetEvents(50)
	if len(evts) != 3 {
		t.Errorf("3 olay bekleniyordu, %d geldi", len(evts))
	}
}

func TestGetEventsSince(t *testing.T) {
	b := NewBus(100)
	b.EmitSimple("old", "src", SevInfo, "Eski")
	time.Sleep(10 * time.Millisecond)
	mid := time.Now().UnixMilli()
	time.Sleep(10 * time.Millisecond)
	b.EmitSimple("new1", "src", SevInfo, "Yeni 1")
	b.EmitSimple("new2", "src", SevInfo, "Yeni 2")

	evts := b.GetEventsSince(mid)
	if len(evts) != 2 {
		t.Errorf("2 olay bekleniyordu, %d geldi", len(evts))
	}
}

func TestGetByType(t *testing.T) {
	b := NewBus(100)
	b.EmitSimple("alpha", "src", SevInfo, "A1")
	b.EmitSimple("beta", "src", SevInfo, "B1")
	b.EmitSimple("alpha", "src", SevInfo, "A2")
	b.EmitSimple("beta", "src", SevInfo, "B2")

	evts := b.GetByType("alpha", 10)
	if len(evts) != 2 {
		t.Errorf("2 alpha olay bekleniyordu, %d geldi", len(evts))
	}
}

func TestGetBySource(t *testing.T) {
	b := NewBus(100)
	b.EmitSimple("test", "dirlik", SevInfo, "1")
	b.EmitSimple("test", "bag", SevInfo, "2")
	b.EmitSimple("test", "dirlik", SevInfo, "3")

	evts := b.GetBySource("dirlik", 10)
	if len(evts) != 2 {
		t.Errorf("2 dirlik olay bekleniyordu, %d geldi", len(evts))
	}
}

func TestGetBySeverity(t *testing.T) {
	b := NewBus(100)
	b.EmitSimple("a", "src", SevInfo, "1")
	b.EmitSimple("b", "src", SevWarning, "2")
	b.EmitSimple("c", "src", SevCritical, "3")
	b.EmitSimple("d", "src", SevWarning, "4")

	evts := b.GetBySeverity(SevWarning, 10)
	if len(evts) != 2 {
		t.Errorf("2 warning olay bekleniyordu, %d geldi", len(evts))
	}
}

func TestCount(t *testing.T) {
	b := NewBus(100)
	if b.Count() != 0 {
		t.Error("Başlangıçta 0 olay olmalı")
	}
	b.EmitSimple("a", "src", SevInfo, "1")
	b.EmitSimple("b", "src", SevInfo, "2")
	if b.Count() != 2 {
		t.Errorf("2 olay bekleniyordu, %d geldi", b.Count())
	}
}

func TestClear(t *testing.T) {
	b := NewBus(100)
	for i := 0; i < 5; i++ {
		b.EmitSimple("test", "src", SevInfo, "Olay")
	}
	n := b.Clear()
	if n != 5 {
		t.Errorf("5 olay temizlenmeliydi, %d geldi", n)
	}
	if b.Count() != 0 {
		t.Errorf("Temizlendikten sonra 0 olmalı, %d var", b.Count())
	}
}

func TestStats(t *testing.T) {
	b := NewBus(100)
	b.EmitSimple("a", "src", SevInfo, "1")
	b.EmitSimple("b", "src", SevWarning, "2")
	b.EmitSimple("c", "src", SevCritical, "3")
	b.EmitSimple("d", "src", SevInfo, "4")

	s := b.GetStats()
	if s.TotalEmitted != 4 {
		t.Errorf("TotalEmitted=4 bekleniyordu, %d geldi", s.TotalEmitted)
	}
	if s.InfoCount != 2 {
		t.Errorf("InfoCount=2 bekleniyordu, %d geldi", s.InfoCount)
	}
	if s.WarningCount != 1 {
		t.Errorf("WarningCount=1 bekleniyordu, %d geldi", s.WarningCount)
	}
	if s.CriticalCount != 1 {
		t.Errorf("CriticalCount=1 bekleniyordu, %d geldi", s.CriticalCount)
	}
	if s.BufferedEvents != 4 {
		t.Errorf("BufferedEvents=4 bekleniyordu, %d geldi", s.BufferedEvents)
	}
}

func TestAutoTrim(t *testing.T) {
	b := NewBus(20)
	for i := 0; i < 25; i++ {
		b.EmitSimple("test", "src", SevInfo, "Olay")
	}
	// maxEvents=20, trim sonrası 10 olmalı
	if b.Count() > 20 {
		t.Errorf("Olay sayısı 20'yi geçmemeli, %d var", b.Count())
	}
}

func TestConcurrentEmit(t *testing.T) {
	b := NewBus(10000)
	var wg sync.WaitGroup
	n := 100
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			b.EmitSimple("concurrent", "src", SevInfo, "Eşzamanlı Olay")
		}(i)
	}
	wg.Wait()
	if b.Count() != n {
		t.Errorf("%d olay bekleniyordu, %d geldi", n, b.Count())
	}
}

func TestConcurrentEmitAndRead(t *testing.T) {
	b := NewBus(10000)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			b.EmitSimple("write", "src", SevInfo, "Yazma")
		}()
		go func() {
			defer wg.Done()
			b.GetEvents(10)
		}()
	}
	wg.Wait()
	if b.Count() < 50 {
		t.Errorf("En az 50 olay olmalı, %d var", b.Count())
	}
}

func TestGetByTypeLimit(t *testing.T) {
	b := NewBus(100)
	for i := 0; i < 10; i++ {
		b.EmitSimple("target", "src", SevInfo, "Olay")
	}
	evts := b.GetByType("target", 3)
	if len(evts) != 3 {
		t.Errorf("3 olay bekleniyordu, %d geldi", len(evts))
	}
}

func TestGetBySourceLimit(t *testing.T) {
	b := NewBus(100)
	for i := 0; i < 10; i++ {
		b.EmitSimple("test", "mysource", SevInfo, "Olay")
	}
	evts := b.GetBySource("mysource", 3)
	if len(evts) != 3 {
		t.Errorf("3 olay bekleniyordu, %d geldi", len(evts))
	}
}

func TestGetBySeverityLimit(t *testing.T) {
	b := NewBus(100)
	for i := 0; i < 10; i++ {
		b.EmitSimple("test", "src", SevCritical, "Olay")
	}
	evts := b.GetBySeverity(SevCritical, 4)
	if len(evts) != 4 {
		t.Errorf("4 olay bekleniyordu, %d geldi", len(evts))
	}
}

func TestEmitReturnsCorrectEvent(t *testing.T) {
	b := NewBus(100)
	evt := b.Emit("explosion", "sensor1", SevCritical, "Patlama", "Büyük bir patlama", 40.1, 33.2)
	if evt.Title != "Patlama" {
		t.Errorf("Title=Patlama bekleniyordu, %s geldi", evt.Title)
	}
	if evt.Detail != "Büyük bir patlama" {
		t.Errorf("Detail yanlış")
	}
	if evt.Lat != 40.1 || evt.Lon != 33.2 {
		t.Error("Koordinatlar yanlış")
	}
}

func TestStatsListenerCount(t *testing.T) {
	b := NewBus(100)
	b.Subscribe(func(evt config.TacticalEvent) {})
	b.Subscribe(func(evt config.TacticalEvent) {})
	s := b.GetStats()
	if s.ListenerCount != 2 {
		t.Errorf("ListenerCount=2 bekleniyordu, %d geldi", s.ListenerCount)
	}
}

func TestEventOrder(t *testing.T) {
	b := NewBus(100)
	b.EmitSimple("first", "src", SevInfo, "İlk")
	b.EmitSimple("second", "src", SevInfo, "İkinci")
	b.EmitSimple("third", "src", SevInfo, "Üçüncü")
	evts := b.GetEvents(3)
	if evts[0].Type != "first" || evts[1].Type != "second" || evts[2].Type != "third" {
		t.Error("Olaylar sıralı olmalı")
	}
}

func TestGetEventsNegativeN(t *testing.T) {
	b := NewBus(100)
	b.EmitSimple("test", "src", SevInfo, "Olay")
	evts := b.GetEvents(-1)
	if len(evts) != 1 {
		t.Errorf("Negatif N tüm olayları döndürmeli, %d geldi", len(evts))
	}
}

func TestClearReturnsCount(t *testing.T) {
	b := NewBus(100)
	for i := 0; i < 7; i++ {
		b.EmitSimple("test", "src", SevInfo, "Olay")
	}
	n := b.Clear()
	if n != 7 {
		t.Errorf("7 temizlenmeliydi, %d geldi", n)
	}
	n2 := b.Clear()
	if n2 != 0 {
		t.Errorf("İkinci clear 0 döndürmeli, %d geldi", n2)
	}
}
