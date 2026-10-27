package collector

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c := New(100*time.Millisecond, 100)
	if c == nil {
		t.Fatal("collector nil")
	}
	if c.interval != 100*time.Millisecond {
		t.Errorf("interval beklenen 100ms, aldık %v", c.interval)
	}
	if c.maxHistory != 100 {
		t.Errorf("maxHistory beklenen 100, aldık %d", c.maxHistory)
	}
}

func TestNewDefaultHistory(t *testing.T) {
	c := New(time.Second, 0)
	if c.maxHistory != 3600 {
		t.Errorf("varsayılan maxHistory beklenen 3600, aldık %d", c.maxHistory)
	}
}

func TestCollect(t *testing.T) {
	c := New(time.Second, 100)
	snap := c.Collect()

	if snap.Timestamp.IsZero() {
		t.Error("timestamp boş")
	}
	if snap.Goroutines <= 0 {
		t.Errorf("goroutines beklenen >0, aldık %d", snap.Goroutines)
	}
	if snap.HeapAlloc == 0 {
		t.Error("heap alloc sıfır")
	}
	if snap.CPUPercent < 0 {
		t.Errorf("cpu_percent negatif: %f", snap.CPUPercent)
	}
	if snap.Uptime <= 0 {
		t.Error("uptime sıfır veya negatif")
	}
}

func TestStartStop(t *testing.T) {
	c := New(50*time.Millisecond, 100)
	c.Start()
	if !c.IsRunning() {
		t.Error("collector çalışmıyor")
	}

	// İki kez start çağrılabilmeli (idempotent)
	c.Start()
	if !c.IsRunning() {
		t.Error("ikinci start sonrası çalışmıyor")
	}

	time.Sleep(200 * time.Millisecond)

	count := c.SnapshotCount()
	if count == 0 {
		t.Error("snapshot toplanmadı")
	}

	c.Stop()
	if c.IsRunning() {
		t.Error("collector hala çalışıyor")
	}
}

func TestLatest(t *testing.T) {
	c := New(50*time.Millisecond, 100)

	// Boş durumda
	_, ok := c.Latest()
	if ok {
		t.Error("boş collector'dan latest alındı")
	}

	c.Start()
	time.Sleep(150 * time.Millisecond)
	c.Stop()

	snap, ok := c.Latest()
	if !ok {
		t.Error("latest alınamadı")
	}
	if snap.Timestamp.IsZero() {
		t.Error("latest timestamp boş")
	}
}

func TestHistory(t *testing.T) {
	c := New(50*time.Millisecond, 10)
	c.Start()
	time.Sleep(350 * time.Millisecond)
	c.Stop()

	all := c.History(0)
	if len(all) == 0 {
		t.Error("geçmiş boş")
	}

	last3 := c.History(3)
	if len(last3) > 3 {
		t.Errorf("3 istendi, %d geldi", len(last3))
	}
}

func TestMaxHistory(t *testing.T) {
	c := New(10*time.Millisecond, 5)
	c.Start()
	time.Sleep(200 * time.Millisecond)
	c.Stop()

	count := c.SnapshotCount()
	if count > 5 {
		t.Errorf("maxHistory aşıldı: %d", count)
	}
}

func TestSubscribe(t *testing.T) {
	c := New(50*time.Millisecond, 100)
	ch := c.Subscribe()

	c.Start()

	select {
	case snap := <-ch:
		if snap.Timestamp.IsZero() {
			t.Error("subscriber'a gelen snapshot timestamp boş")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("subscriber'a bildirim gelmedi")
	}

	c.Unsubscribe(ch)
	c.Stop()
}

func TestRecordRequest(t *testing.T) {
	c := New(time.Second, 100)

	c.RecordRequest("bag", 5.2, false)
	c.RecordRequest("bag", 3.8, false)
	c.RecordRequest("bag", 10.1, true)
	c.RecordRequest("kutuk", 1.5, false)

	metrics := c.GetServiceMetrics()
	if len(metrics) != 2 {
		t.Fatalf("2 servis beklendi, %d geldi", len(metrics))
	}

	var bagMetric ServiceMetric
	for _, m := range metrics {
		if m.ServiceID == "bag" {
			bagMetric = m
		}
	}

	if bagMetric.RequestCount != 3 {
		t.Errorf("bag request count beklenen 3, aldık %d", bagMetric.RequestCount)
	}
	if bagMetric.ErrorCount != 1 {
		t.Errorf("bag error count beklenen 1, aldık %d", bagMetric.ErrorCount)
	}
	if bagMetric.ErrorRate < 0.33 || bagMetric.ErrorRate > 0.34 {
		t.Errorf("bag error rate beklenen ~0.333, aldık %f", bagMetric.ErrorRate)
	}
}

func TestDiskCollect(t *testing.T) {
	c := New(time.Second, 100)
	snap := c.Collect()
	// Disk istatistikleri platforma bağlı, en azından hata vermemeli
	if snap.DiskPercent < 0 || snap.DiskPercent > 100 {
		t.Errorf("disk percent aralık dışı: %f", snap.DiskPercent)
	}
}

func BenchmarkCollect(b *testing.B) {
	c := New(time.Second, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Collect()
	}
}
