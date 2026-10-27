package store

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestPutGet(t *testing.T) {
	s := New()
	defer s.Close()

	s.Put("entity", "tank-001", []byte(`{"health":100}`), 0, nil)
	entry, ok := s.Get("entity", "tank-001")
	if !ok {
		t.Fatal("kayıt bulunamadı")
	}
	if string(entry.Value) != `{"health":100}` {
		t.Fatalf("yanlış değer: %s", entry.Value)
	}
	if entry.Namespace != "entity" {
		t.Fatalf("yanlış namespace: %s", entry.Namespace)
	}
}

func TestDelete(t *testing.T) {
	s := New()
	defer s.Close()

	s.Put("", "key1", []byte("val1"), 0, nil)
	if !s.Delete("", "key1") {
		t.Fatal("silme başarısız")
	}
	_, ok := s.Get("", "key1")
	if ok {
		t.Fatal("silinen kayıt hala mevcut")
	}
}

func TestTTLExpiry(t *testing.T) {
	s := New()
	defer s.Close()

	s.Put("", "temp", []byte("data"), 50*time.Millisecond, nil)
	_, ok := s.Get("", "temp")
	if !ok {
		t.Fatal("kayıt hemen bulunmalı")
	}

	time.Sleep(100 * time.Millisecond)
	_, ok = s.Get("", "temp")
	if ok {
		t.Fatal("süresi dolmuş kayıt hala mevcut")
	}
}

func TestNamespaceIsolation(t *testing.T) {
	s := New()
	defer s.Close()

	s.Put("entity", "id1", []byte("entity-data"), 0, nil)
	s.Put("sensor", "id1", []byte("sensor-data"), 0, nil)

	e1, _ := s.Get("entity", "id1")
	e2, _ := s.Get("sensor", "id1")

	if string(e1.Value) == string(e2.Value) {
		t.Fatal("namespace izolasyonu çalışmıyor")
	}
}

func TestScan(t *testing.T) {
	s := New()
	defer s.Close()

	s.Put("entity", "tank-001", []byte("t1"), 0, nil)
	s.Put("entity", "tank-002", []byte("t2"), 0, nil)
	s.Put("entity", "drone-001", []byte("d1"), 0, nil)

	results := s.Scan("entity", "tank-", 10)
	if len(results) != 2 {
		t.Fatalf("beklenen 2 sonuç, alınan: %d", len(results))
	}
}

func TestCount(t *testing.T) {
	s := New()
	defer s.Close()

	for i := 0; i < 100; i++ {
		s.Put("test", fmt.Sprintf("key-%d", i), []byte("v"), 0, nil)
	}
	if s.Count() != 100 {
		t.Fatalf("beklenen 100, alınan: %d", s.Count())
	}
}

func TestVersion(t *testing.T) {
	s := New()
	defer s.Close()

	e1 := s.Put("", "k", []byte("v1"), 0, nil)
	e2 := s.Put("", "k", []byte("v2"), 0, nil)
	if e2.Version <= e1.Version {
		t.Fatal("versiyon artmalı")
	}
}

func TestWatch(t *testing.T) {
	s := New()
	defer s.Close()

	_, w := s.Watch("entity", "", 10)

	s.Put("entity", "tank-001", []byte("data"), 0, nil)

	select {
	case ev := <-w.Ch:
		if ev.Type != "put" || ev.Key != "tank-001" {
			t.Fatalf("beklenmeyen olay: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("watch olayı gelmedi")
	}
}

func TestWatchPrefix(t *testing.T) {
	s := New()
	defer s.Close()

	_, w := s.Watch("entity", "tank-", 10)

	s.Put("entity", "drone-001", []byte("d"), 0, nil) // Bu gelmemeli
	s.Put("entity", "tank-005", []byte("t"), 0, nil)  // Bu gelmeli

	select {
	case ev := <-w.Ch:
		if ev.Key != "tank-005" {
			t.Fatalf("yanlış anahtar: %s", ev.Key)
		}
	case <-time.After(time.Second):
		t.Fatal("watch olayı gelmedi")
	}
}

func TestPutJSON(t *testing.T) {
	s := New()
	defer s.Close()

	type Entity struct {
		Name   string  `json:"name"`
		Health float64 `json:"health"`
	}

	s.PutJSON("entity", "e1", Entity{Name: "Bozkurt", Health: 95.5}, 0)

	var got Entity
	if err := s.GetJSON("entity", "e1", &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "Bozkurt" || got.Health != 95.5 {
		t.Fatalf("yanlış değer: %+v", got)
	}
}

func TestCleanup(t *testing.T) {
	s := New()
	defer s.Close()

	for i := 0; i < 10; i++ {
		s.Put("", fmt.Sprintf("temp-%d", i), []byte("v"), 50*time.Millisecond, nil)
	}
	s.Put("", "permanent", []byte("v"), 0, nil)

	time.Sleep(100 * time.Millisecond)
	cleaned := s.Cleanup()
	if cleaned != 10 {
		t.Fatalf("beklenen 10 temizlik, yapılan: %d", cleaned)
	}
	if s.Count() != 1 {
		t.Fatalf("beklenen 1 kayıt, mevcut: %d", s.Count())
	}
}

func TestSnapshot(t *testing.T) {
	s := New()
	s.Put("entity", "e1", []byte(`"tank"`), 0, nil)
	s.Put("sensor", "s1", []byte(`"radar"`), 0, nil)

	data, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	// Yeni store'a geri yükle
	s2 := New()
	defer s2.Close()
	if err := s2.Restore(data); err != nil {
		t.Fatal(err)
	}

	e, ok := s2.Get("entity", "e1")
	if !ok {
		t.Fatal("geri yükleme sonrası kayıt bulunamadı")
	}
	if string(e.Value) != `"tank"` {
		t.Fatalf("yanlış değer: %s", e.Value)
	}
}

func TestStats(t *testing.T) {
	s := New()
	defer s.Close()

	s.Put("", "k1", []byte("v"), 0, nil)
	s.Get("", "k1")    // hit
	s.Get("", "miss")  // miss

	stats := s.GetStats()
	if stats.TotalPuts != 1 || stats.TotalGets != 2 ||
		stats.TotalHits != 1 || stats.TotalMisses != 1 {
		t.Fatalf("yanlış istatistik: %+v", stats)
	}
}

func TestNamespaces(t *testing.T) {
	s := New()
	defer s.Close()

	s.Put("entity", "e1", []byte("v"), 0, nil)
	s.Put("sensor", "s1", []byte("v"), 0, nil)
	s.Put("track", "t1", []byte("v"), 0, nil)

	ns := s.Namespaces()
	if len(ns) != 3 {
		t.Fatalf("beklenen 3 namespace, alınan: %d (%v)", len(ns), ns)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := New()
	defer s.Close()

	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				key := fmt.Sprintf("key-%d-%d", id, i)
				s.Put("bench", key, []byte(`{"x":1}`), 0, nil)
				s.Get("bench", key)
			}
		}(g)
	}
	wg.Wait()

	if s.Count() != 10000 {
		t.Fatalf("beklenen 10000, alınan: %d", s.Count())
	}
}

func TestMetadata(t *testing.T) {
	s := New()
	defer s.Close()

	meta := map[string]string{"source": "radar", "priority": "1"}
	s.Put("entity", "e1", []byte("v"), 0, meta)
	e, _ := s.Get("entity", "e1")
	if e.Metadata["source"] != "radar" {
		t.Fatalf("metadata yanlış: %v", e.Metadata)
	}
}

// ── Benchmark ──

func BenchmarkPut(b *testing.B) {
	s := New()
	defer s.Close()
	val := []byte(`{"lat":39.925,"lon":32.837,"health":100}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Put("entity", fmt.Sprintf("e-%d", i), val, 0, nil)
	}
}

func BenchmarkGet(b *testing.B) {
	s := New()
	defer s.Close()
	val := []byte(`{"lat":39.925,"lon":32.837,"health":100}`)
	for i := 0; i < 10000; i++ {
		s.Put("entity", fmt.Sprintf("e-%d", i), val, 0, nil)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Get("entity", fmt.Sprintf("e-%d", i%10000))
	}
}

func BenchmarkPutGetParallel(b *testing.B) {
	s := New()
	defer s.Close()
	val := []byte(`{"lat":39.925,"lon":32.837}`)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("e-%d", i)
			s.Put("entity", key, val, 0, nil)
			s.Get("entity", key)
			i++
		}
	})
}

func BenchmarkJSON(b *testing.B) {
	s := New()
	defer s.Close()
	type Pos struct{ Lat, Lon float64 }
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.PutJSON("entity", fmt.Sprintf("e-%d", i), Pos{39.9, 32.8}, 0)
		var p Pos
		s.GetJSON("entity", fmt.Sprintf("e-%d", i), &p)
	}
}

// JSON yardımcı
func init() {
	_ = json.Marshal
}
