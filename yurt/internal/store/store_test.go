package store

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func testStore() *Store {
	return New("edge-test", 10000, 1*time.Hour)
}

// ── VectorClock Tests ──

func TestVectorClockIncrement(t *testing.T) {
	vc := make(VectorClock)
	vc.Increment("A")
	vc.Increment("A")
	vc.Increment("B")
	if vc["A"] != 2 {
		t.Fatalf("beklenen A=2, alınan: %d", vc["A"])
	}
	if vc["B"] != 1 {
		t.Fatalf("beklenen B=1, alınan: %d", vc["B"])
	}
}

func TestVectorClockMerge(t *testing.T) {
	a := VectorClock{"X": 3, "Y": 1}
	b := VectorClock{"X": 1, "Y": 5, "Z": 2}
	a.Merge(b)
	if a["X"] != 3 || a["Y"] != 5 || a["Z"] != 2 {
		t.Fatalf("merge yanlış: %v", a)
	}
}

func TestVectorClockClone(t *testing.T) {
	a := VectorClock{"X": 3}
	b := a.Clone()
	b.Increment("X")
	if a["X"] != 3 {
		t.Fatal("clone orijinali değiştirmemeli")
	}
	if b["X"] != 4 {
		t.Fatal("clone artırılmamış")
	}
}

func TestVectorClockHappensBefore(t *testing.T) {
	a := VectorClock{"X": 1, "Y": 2}
	b := VectorClock{"X": 2, "Y": 3}
	if !a.HappensBefore(b) {
		t.Fatal("a, b'den önce olmalı")
	}
	if b.HappensBefore(a) {
		t.Fatal("b, a'dan önce olmamalı")
	}
}

func TestVectorClockConcurrent(t *testing.T) {
	a := VectorClock{"X": 2, "Y": 1}
	b := VectorClock{"X": 1, "Y": 2}
	if !a.Concurrent(b) {
		t.Fatal("a ve b eşzamanlı olmalı")
	}
}

func TestVectorClockEqual(t *testing.T) {
	a := VectorClock{"X": 1, "Y": 2}
	b := VectorClock{"X": 1, "Y": 2}
	if !a.Equal(b) {
		t.Fatal("eşit olmalı")
	}
	c := VectorClock{"X": 1, "Y": 3}
	if a.Equal(c) {
		t.Fatal("eşit olmamalı")
	}
}

func TestVectorClockNotHappensBeforeEqual(t *testing.T) {
	a := VectorClock{"X": 1}
	b := VectorClock{"X": 1}
	if a.HappensBefore(b) {
		t.Fatal("eşit saatler happens-before döndürmemeli")
	}
}

func TestVectorClockNotConcurrentEqual(t *testing.T) {
	a := VectorClock{"X": 1}
	b := VectorClock{"X": 1}
	if a.Concurrent(b) {
		t.Fatal("eşit saatler concurrent döndürmemeli")
	}
}

// ── Store Tests ──

func TestPutAndGet(t *testing.T) {
	s := testStore()
	err := s.Put("key1", map[string]interface{}{"name": "test"})
	if err != nil {
		t.Fatalf("Put hatası: %v", err)
	}
	entry, ok := s.Get("key1")
	if !ok {
		t.Fatal("Get bulunamadı")
	}
	if entry.Key != "key1" {
		t.Fatalf("yanlış key: %s", entry.Key)
	}
	if entry.Value["name"] != "test" {
		t.Fatalf("yanlış value: %v", entry.Value)
	}
	if !entry.Dirty {
		t.Fatal("yeni yazma dirty olmalı")
	}
}

func TestPutEmptyKey(t *testing.T) {
	s := testStore()
	err := s.Put("", map[string]interface{}{"x": 1})
	if err == nil {
		t.Fatal("boş key hata vermeli")
	}
}

func TestPutVersionIncrement(t *testing.T) {
	s := testStore()
	s.Put("k", map[string]interface{}{"v": 1})
	s.Put("k", map[string]interface{}{"v": 2})
	entry, _ := s.Get("k")
	if entry.Version["edge-test"] != 2 {
		t.Fatalf("beklenen version=2, alınan: %d", entry.Version["edge-test"])
	}
	if entry.Value["v"].(int) != 2 {
		t.Fatal("value güncellenmemiş")
	}
}

func TestGetNotFound(t *testing.T) {
	s := testStore()
	_, ok := s.Get("missing")
	if ok {
		t.Fatal("olmayan key bulunmamalı")
	}
}

func TestDelete(t *testing.T) {
	s := testStore()
	s.Put("key1", map[string]interface{}{"x": 1})
	err := s.Delete("key1")
	if err != nil {
		t.Fatalf("Delete hatası: %v", err)
	}
	_, ok := s.Get("key1")
	if ok {
		t.Fatal("silinmiş key bulunmamalı")
	}
}

func TestDeleteNotFound(t *testing.T) {
	s := testStore()
	err := s.Delete("missing")
	if err == nil {
		t.Fatal("olmayan key silme hata vermeli")
	}
}

func TestGetDirty(t *testing.T) {
	s := testStore()
	s.Put("a", map[string]interface{}{"x": 1})
	s.Put("b", map[string]interface{}{"x": 2})
	s.Put("c", map[string]interface{}{"x": 3})

	dirty := s.GetDirty(10)
	if len(dirty) != 3 {
		t.Fatalf("beklenen: 3 dirty, alınan: %d", len(dirty))
	}
}

func TestGetDirtyLimit(t *testing.T) {
	s := testStore()
	for i := 0; i < 10; i++ {
		s.Put(fmt.Sprintf("k%d", i), map[string]interface{}{"i": i})
	}
	dirty := s.GetDirty(3)
	if len(dirty) != 3 {
		t.Fatalf("beklenen: 3 (limit), alınan: %d", len(dirty))
	}
}

func TestMarkClean(t *testing.T) {
	s := testStore()
	s.Put("a", map[string]interface{}{"x": 1})
	s.Put("b", map[string]interface{}{"x": 2})
	s.MarkClean([]string{"a"})

	dirty := s.GetDirty(10)
	if len(dirty) != 1 {
		t.Fatalf("beklenen: 1 dirty (b), alınan: %d", len(dirty))
	}
	if dirty[0].Key != "b" {
		t.Fatalf("beklenen key: b, alınan: %s", dirty[0].Key)
	}
}

func TestListKeys(t *testing.T) {
	s := testStore()
	s.Put("sensor/radar", map[string]interface{}{})
	s.Put("sensor/thermal", map[string]interface{}{})
	s.Put("entity/tank", map[string]interface{}{})

	all := s.ListKeys("")
	if len(all) != 3 {
		t.Fatalf("beklenen: 3 key, alınan: %d", len(all))
	}

	sensors := s.ListKeys("sensor/")
	if len(sensors) != 2 {
		t.Fatalf("beklenen: 2 sensor key, alınan: %d", len(sensors))
	}
}

func TestSize(t *testing.T) {
	s := testStore()
	s.Put("a", map[string]interface{}{})
	s.Put("b", map[string]interface{}{})
	if s.Size() != 2 {
		t.Fatalf("beklenen: 2, alınan: %d", s.Size())
	}
}

func TestExpireOld(t *testing.T) {
	s := New("edge", 10000, 1*time.Millisecond) // 1ms TTL
	s.Put("a", map[string]interface{}{})
	s.Put("b", map[string]interface{}{})
	time.Sleep(5 * time.Millisecond)
	expired := s.ExpireOld()
	if expired != 2 {
		t.Fatalf("beklenen: 2 expire, alınan: %d", expired)
	}
	if s.Size() != 0 {
		t.Fatalf("expire sonrası boyut 0 olmalı: %d", s.Size())
	}
}

func TestGetExpiredEntry(t *testing.T) {
	s := New("edge", 10000, 1*time.Millisecond)
	s.Put("a", map[string]interface{}{"x": 1})
	time.Sleep(5 * time.Millisecond)
	_, ok := s.Get("a")
	if ok {
		t.Fatal("süresi dolmuş entry dönmemeli")
	}
}

func TestPutWithVersion(t *testing.T) {
	s := testStore()
	vc := VectorClock{"cloud": 5}
	s.PutWithVersion("k", map[string]interface{}{"from": "cloud"}, vc, "cloud", time.Now())
	entry, ok := s.Get("k")
	if !ok {
		t.Fatal("PutWithVersion sonrası bulunamadı")
	}
	if entry.Version["cloud"] != 5 {
		t.Fatalf("yanlış version: %v", entry.Version)
	}
	if entry.Dirty {
		t.Fatal("PutWithVersion dirty olmamalı")
	}
}

func TestChangeLog(t *testing.T) {
	s := testStore()
	before := time.Now().Add(-time.Second)
	s.Put("a", map[string]interface{}{})
	s.Put("b", map[string]interface{}{})
	s.Delete("a")

	log := s.GetChangeLog(before)
	if len(log) != 3 {
		t.Fatalf("beklenen: 3 change, alınan: %d", len(log))
	}
	if log[0].Operation != "put" || log[2].Operation != "delete" {
		t.Fatal("yanlış operation sırası")
	}
}

func TestClearChangeLog(t *testing.T) {
	s := testStore()
	s.Put("a", map[string]interface{}{})
	s.ClearChangeLog()
	log := s.GetChangeLog(time.Time{})
	if len(log) != 0 {
		t.Fatalf("temizlik sonrası boş olmalı: %d", len(log))
	}
}

func TestSnapshot(t *testing.T) {
	s := testStore()
	s.Put("x", map[string]interface{}{"v": 1})
	s.Put("y", map[string]interface{}{"v": 2})

	snap := s.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("beklenen: 2 kayıt, alınan: %d", len(snap))
	}
}

func TestStats(t *testing.T) {
	s := testStore()
	s.Put("a", map[string]interface{}{})
	s.Get("a")
	s.Get("missing")

	stats := s.GetStats()
	if stats.Puts != 1 {
		t.Fatalf("puts: %d", stats.Puts)
	}
	if stats.Gets != 2 {
		t.Fatalf("gets: %d", stats.Gets)
	}
	if stats.Hits != 1 {
		t.Fatalf("hits: %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Fatalf("misses: %d", stats.Misses)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := testStore()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", idx)
			s.Put(key, map[string]interface{}{"idx": idx})
			s.Get(key)
		}(i)
	}
	wg.Wait()
	if s.Size() != 100 {
		t.Fatalf("concurrent write sonrası boyut 100 olmalı: %d", s.Size())
	}
}

func TestEntryIsExpired(t *testing.T) {
	e := &Entry{ModifiedAt: time.Now().Add(-time.Hour), TTL: time.Minute}
	if !e.IsExpired() {
		t.Fatal("süresi dolmuş olmalı")
	}
	e2 := &Entry{ModifiedAt: time.Now(), TTL: time.Hour}
	if e2.IsExpired() {
		t.Fatal("süresi dolmamalı")
	}
	e3 := &Entry{ModifiedAt: time.Now().Add(-time.Hour), TTL: 0}
	if e3.IsExpired() {
		t.Fatal("TTL=0 ise expire olmamalı")
	}
}

// ── Benchmarks ──

func BenchmarkPut(b *testing.B) {
	s := testStore()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Put(fmt.Sprintf("key-%d", i), map[string]interface{}{"i": i})
	}
}

func BenchmarkGet(b *testing.B) {
	s := testStore()
	for i := 0; i < 1000; i++ {
		s.Put(fmt.Sprintf("key-%d", i), map[string]interface{}{"i": i})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Get(fmt.Sprintf("key-%d", i%1000))
	}
}

func BenchmarkConcurrentPut(b *testing.B) {
	s := testStore()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.Put(fmt.Sprintf("p-key-%d", i), map[string]interface{}{"i": i})
			i++
		}
	})
}
