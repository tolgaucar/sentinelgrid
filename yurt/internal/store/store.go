// Package store — YURT yerel uç depolama.
//
// Shard'lı concurrent KV store. Her kayıt vector clock ile versiyonlanır.
// Offline çalışma sırasında veri kaybını önlemek için dirty-tracking
// ve değişiklik günlüğü (change log) tutar.
package store

import (
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"
)

const shardCount = 32

// VectorClock — dağıtık sürüm izleme
type VectorClock map[string]uint64

// Increment — belirli düğümün saatini artır
func (vc VectorClock) Increment(nodeID string) {
	vc[nodeID]++
}

// Merge — iki vector clock'u birleştir (her düğüm için max)
func (vc VectorClock) Merge(other VectorClock) {
	for k, v := range other {
		if v > vc[k] {
			vc[k] = v
		}
	}
}

// Clone — derin kopya
func (vc VectorClock) Clone() VectorClock {
	cp := make(VectorClock, len(vc))
	for k, v := range vc {
		cp[k] = v
	}
	return cp
}

// HappensBefore — bu saat other'dan önce mi?
func (vc VectorClock) HappensBefore(other VectorClock) bool {
	atLeastOneLess := false
	for k, v := range vc {
		ov := other[k]
		if v > ov {
			return false
		}
		if v < ov {
			atLeastOneLess = true
		}
	}
	for k, ov := range other {
		if _, ok := vc[k]; !ok && ov > 0 {
			atLeastOneLess = true
		}
	}
	return atLeastOneLess
}

// Concurrent — iki saat eşzamanlı mı? (ne biri diğerinden önce, ne sonra)
func (vc VectorClock) Concurrent(other VectorClock) bool {
	return !vc.HappensBefore(other) && !other.HappensBefore(vc) && !vc.Equal(other)
}

// Equal — iki saat eşit mi?
func (vc VectorClock) Equal(other VectorClock) bool {
	if len(vc) != len(other) {
		return false
	}
	for k, v := range vc {
		if other[k] != v {
			return false
		}
	}
	return true
}

// Entry — depolanmış veri kaydı
type Entry struct {
	Key        string                 `json:"key"`
	Value      map[string]interface{} `json:"value"`
	Version    VectorClock            `json:"version"`
	ModifiedAt time.Time             `json:"modified_at"`
	CreatedAt  time.Time              `json:"created_at"`
	TTL        time.Duration          `json:"ttl"`
	Dirty      bool                   `json:"dirty"`      // senkronize edilmemiş
	Source     string                 `json:"source"`      // hangi düğümden geldi
	Tombstone  bool                   `json:"tombstone"`   // silinmiş işareti
}

// IsExpired — TTL aşılmış mı
func (e *Entry) IsExpired() bool {
	if e.TTL <= 0 {
		return false
	}
	return time.Since(e.ModifiedAt) > e.TTL
}

// ChangeRecord — değişiklik kaydı (delta sync için)
type ChangeRecord struct {
	Key       string      `json:"key"`
	Operation string      `json:"operation"` // put, delete
	Version   VectorClock `json:"version"`
	Timestamp time.Time   `json:"timestamp"`
	NodeID    string      `json:"node_id"`
}

// shard — iç parça
type shard struct {
	mu      sync.RWMutex
	entries map[string]*Entry
}

// Store — YURT yerel depolama
type Store struct {
	shards     [shardCount]shard
	nodeID     string
	maxEntries int
	defaultTTL time.Duration
	changeLog  []ChangeRecord
	clMu       sync.Mutex
	stats      Stats
}

// Stats — depo istatistikleri
type Stats struct {
	Puts       int64 `json:"puts"`
	Gets       int64 `json:"gets"`
	Deletes    int64 `json:"deletes"`
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	Conflicts  int64 `json:"conflicts"`
	Expirations int64 `json:"expirations"`
	DirtyCount int64 `json:"dirty_count"`
	TotalSize  int64 `json:"total_size"`
}

// New — yeni store oluştur
func New(nodeID string, maxEntries int, defaultTTL time.Duration) *Store {
	s := &Store{
		nodeID:     nodeID,
		maxEntries: maxEntries,
		defaultTTL: defaultTTL,
		changeLog:  make([]ChangeRecord, 0, 1024),
	}
	for i := range s.shards {
		s.shards[i].entries = make(map[string]*Entry)
	}
	return s
}

func (s *Store) getShard(key string) *shard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return &s.shards[h.Sum32()%shardCount]
}

// Put — veri yaz (vector clock artır)
func (s *Store) Put(key string, value map[string]interface{}) error {
	if key == "" {
		return fmt.Errorf("anahtar boş olamaz")
	}
	sh := s.getShard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()

	now := time.Now()
	existing, exists := sh.entries[key]

	var vc VectorClock
	if exists {
		vc = existing.Version.Clone()
	} else {
		vc = make(VectorClock)
	}
	vc.Increment(s.nodeID)

	entry := &Entry{
		Key:        key,
		Value:      value,
		Version:    vc,
		ModifiedAt: now,
		TTL:        s.defaultTTL,
		Dirty:      true,
		Source:     s.nodeID,
	}
	if exists {
		entry.CreatedAt = existing.CreatedAt
	} else {
		entry.CreatedAt = now
	}

	sh.entries[key] = entry
	atomic.AddInt64(&s.stats.Puts, 1)
	atomic.AddInt64(&s.stats.DirtyCount, 1)
	atomic.AddInt64(&s.stats.TotalSize, 1)

	s.recordChange(key, "put", vc)
	return nil
}

// PutWithVersion — belirli bir vector clock ile yaz (sync sırasında kullanılır)
func (s *Store) PutWithVersion(key string, value map[string]interface{}, version VectorClock, source string, modTime time.Time) {
	sh := s.getShard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.entries[key] = &Entry{
		Key:        key,
		Value:      value,
		Version:    version.Clone(),
		ModifiedAt: modTime,
		CreatedAt:  modTime,
		TTL:        s.defaultTTL,
		Dirty:      false,
		Source:     source,
	}
}

// Get — veri oku
func (s *Store) Get(key string) (*Entry, bool) {
	sh := s.getShard(key)
	sh.mu.RLock()
	defer sh.mu.RUnlock()

	atomic.AddInt64(&s.stats.Gets, 1)

	entry, ok := sh.entries[key]
	if !ok {
		atomic.AddInt64(&s.stats.Misses, 1)
		return nil, false
	}
	if entry.Tombstone || entry.IsExpired() {
		atomic.AddInt64(&s.stats.Misses, 1)
		return nil, false
	}
	atomic.AddInt64(&s.stats.Hits, 1)
	cp := *entry
	cp.Version = entry.Version.Clone()
	return &cp, true
}

// Delete — kayıt sil (tombstone)
func (s *Store) Delete(key string) error {
	sh := s.getShard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()

	entry, ok := sh.entries[key]
	if !ok {
		return fmt.Errorf("anahtar bulunamadı: %s", key)
	}

	entry.Version.Increment(s.nodeID)
	entry.Tombstone = true
	entry.Dirty = true
	entry.ModifiedAt = time.Now()

	atomic.AddInt64(&s.stats.Deletes, 1)
	atomic.AddInt64(&s.stats.DirtyCount, 1)

	s.recordChange(key, "delete", entry.Version)
	return nil
}

// GetDirty — senkronize edilmemiş kayıtları getir
func (s *Store) GetDirty(limit int) []Entry {
	result := make([]Entry, 0)
	for i := range s.shards {
		s.shards[i].mu.RLock()
		for _, e := range s.shards[i].entries {
			if e.Dirty {
				cp := *e
				cp.Version = e.Version.Clone()
				result = append(result, cp)
				if limit > 0 && len(result) >= limit {
					s.shards[i].mu.RUnlock()
					return result
				}
			}
		}
		s.shards[i].mu.RUnlock()
	}
	return result
}

// MarkClean — kayıtları senkronize edildi olarak işaretle
func (s *Store) MarkClean(keys []string) {
	for _, key := range keys {
		sh := s.getShard(key)
		sh.mu.Lock()
		if e, ok := sh.entries[key]; ok && e.Dirty {
			e.Dirty = false
			atomic.AddInt64(&s.stats.DirtyCount, -1)
		}
		sh.mu.Unlock()
	}
}

// ListKeys — tüm aktif anahtarlar
func (s *Store) ListKeys(prefix string) []string {
	result := make([]string, 0)
	for i := range s.shards {
		s.shards[i].mu.RLock()
		for k, e := range s.shards[i].entries {
			if !e.Tombstone && !e.IsExpired() {
				if prefix == "" || len(k) >= len(prefix) && k[:len(prefix)] == prefix {
					result = append(result, k)
				}
			}
		}
		s.shards[i].mu.RUnlock()
	}
	return result
}

// Size — aktif kayıt sayısı
func (s *Store) Size() int {
	count := 0
	for i := range s.shards {
		s.shards[i].mu.RLock()
		for _, e := range s.shards[i].entries {
			if !e.Tombstone && !e.IsExpired() {
				count++
			}
		}
		s.shards[i].mu.RUnlock()
	}
	return count
}

// ExpireOld — süresi dolmuş kayıtları temizle
func (s *Store) ExpireOld() int {
	expired := 0
	for i := range s.shards {
		s.shards[i].mu.Lock()
		for k, e := range s.shards[i].entries {
			if e.IsExpired() || e.Tombstone {
				delete(s.shards[i].entries, k)
				expired++
			}
		}
		s.shards[i].mu.Unlock()
	}
	atomic.AddInt64(&s.stats.Expirations, int64(expired))
	return expired
}

// GetChangeLog — değişiklik günlüğü (delta sync)
func (s *Store) GetChangeLog(since time.Time) []ChangeRecord {
	s.clMu.Lock()
	defer s.clMu.Unlock()
	result := make([]ChangeRecord, 0)
	for _, cr := range s.changeLog {
		if cr.Timestamp.After(since) {
			result = append(result, cr)
		}
	}
	return result
}

// ClearChangeLog — değişiklik günlüğünü temizle
func (s *Store) ClearChangeLog() {
	s.clMu.Lock()
	defer s.clMu.Unlock()
	s.changeLog = s.changeLog[:0]
}

func (s *Store) recordChange(key, op string, vc VectorClock) {
	s.clMu.Lock()
	defer s.clMu.Unlock()
	s.changeLog = append(s.changeLog, ChangeRecord{
		Key:       key,
		Operation: op,
		Version:   vc.Clone(),
		Timestamp: time.Now(),
		NodeID:    s.nodeID,
	})
	if len(s.changeLog) > 10000 {
		s.changeLog = s.changeLog[len(s.changeLog)-5000:]
	}
}

// GetStats — depo istatistikleri
func (s *Store) GetStats() Stats {
	return Stats{
		Puts:        atomic.LoadInt64(&s.stats.Puts),
		Gets:        atomic.LoadInt64(&s.stats.Gets),
		Deletes:     atomic.LoadInt64(&s.stats.Deletes),
		Hits:        atomic.LoadInt64(&s.stats.Hits),
		Misses:      atomic.LoadInt64(&s.stats.Misses),
		Conflicts:   atomic.LoadInt64(&s.stats.Conflicts),
		Expirations: atomic.LoadInt64(&s.stats.Expirations),
		DirtyCount:  atomic.LoadInt64(&s.stats.DirtyCount),
		TotalSize:   int64(s.Size()),
	}
}

// Snapshot — tüm aktif kayıtların kopyası
func (s *Store) Snapshot() map[string]Entry {
	result := make(map[string]Entry)
	for i := range s.shards {
		s.shards[i].mu.RLock()
		for k, e := range s.shards[i].entries {
			if !e.Tombstone && !e.IsExpired() {
				cp := *e
				cp.Version = e.Version.Clone()
				result[k] = cp
			}
		}
		s.shards[i].mu.RUnlock()
	}
	return result
}
