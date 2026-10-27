// Package store KÜTÜK'ün çekirdeği: eşzamanlı, TTL destekli, namespace'li
// bellek-içi anahtar-değer deposu. Okuma/yazma mikrosaniye seviyesinde.
package store

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Entry tek bir kayıt.
type Entry struct {
	Key       string
	Value     []byte
	Namespace string
	Version   uint64
	CreatedAt int64 // UnixNano
	UpdatedAt int64
	ExpiresAt int64 // 0 = sonsuz
	Metadata  map[string]string
}

// IsExpired süresi dolmuş mu.
func (e *Entry) IsExpired() bool {
	if e.ExpiresAt == 0 {
		return false
	}
	return time.Now().UnixNano() > e.ExpiresAt
}

// Watcher değişiklik izleyici.
type Watcher struct {
	Ch        chan WatchEvent
	Prefix    string
	Namespace string
}

// WatchEvent değişiklik olayı.
type WatchEvent struct {
	Type      string // "put", "delete", "expire"
	Key       string
	Namespace string
	Entry     *Entry
	Timestamp int64
}

// Stats depo istatistikleri.
type Stats struct {
	TotalKeys    int64
	TotalGets    int64
	TotalPuts    int64
	TotalDeletes int64
	TotalHits    int64
	TotalMisses  int64
	TotalExpired int64
	Namespaces   int
	UptimeNs     int64
}

// shardCount parça sayısı - eşzamanlılığı artırmak için.
const shardCount = 64

// shard tek bir parça.
type shard struct {
	mu    sync.RWMutex
	items map[string]*Entry
}

// Store ana bellek-içi depo.
type Store struct {
	shards   [shardCount]*shard
	watchers sync.Map // map[uint64]*Watcher
	watchID  atomic.Uint64
	version  atomic.Uint64

	// İstatistikler
	gets    atomic.Int64
	puts    atomic.Int64
	deletes atomic.Int64
	hits    atomic.Int64
	misses  atomic.Int64
	expired atomic.Int64

	startTime int64
	stopCh    chan struct{}
}

// New yeni depo oluştur.
func New() *Store {
	s := &Store{
		startTime: time.Now().UnixNano(),
		stopCh:    make(chan struct{}),
	}
	for i := 0; i < shardCount; i++ {
		s.shards[i] = &shard{items: make(map[string]*Entry, 256)}
	}
	return s
}

// fnv32 hızlı hash.
func fnv32(key string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return h
}

// getShard anahtar için parçayı döndür.
func (s *Store) getShard(key string) *shard {
	return s.shards[fnv32(key)%shardCount]
}

// fullKey namespace:key formatında tam anahtar.
func fullKey(ns, key string) string {
	if ns == "" {
		return key
	}
	return ns + ":" + key
}

// Put anahtar-değer yaz.
func (s *Store) Put(ns, key string, value []byte, ttl time.Duration, meta map[string]string) *Entry {
	fk := fullKey(ns, key)
	sh := s.getShard(fk)
	now := time.Now().UnixNano()
	ver := s.version.Add(1)

	var expiresAt int64
	if ttl > 0 {
		expiresAt = now + int64(ttl)
	}

	entry := &Entry{
		Key:       key,
		Value:     value,
		Namespace: ns,
		Version:   ver,
		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: expiresAt,
		Metadata:  meta,
	}

	sh.mu.Lock()
	existing, exists := sh.items[fk]
	if exists {
		entry.CreatedAt = existing.CreatedAt
	}
	sh.items[fk] = entry
	sh.mu.Unlock()

	s.puts.Add(1)
	s.notifyWatchers("put", fk, ns, entry)
	return entry
}

// Get anahtar oku.
func (s *Store) Get(ns, key string) (*Entry, bool) {
	fk := fullKey(ns, key)
	sh := s.getShard(fk)
	s.gets.Add(1)

	sh.mu.RLock()
	entry, ok := sh.items[fk]
	sh.mu.RUnlock()

	if !ok {
		s.misses.Add(1)
		return nil, false
	}

	if entry.IsExpired() {
		s.Delete(ns, key)
		s.misses.Add(1)
		s.expired.Add(1)
		return nil, false
	}

	s.hits.Add(1)
	return entry, true
}

// Delete anahtar sil.
func (s *Store) Delete(ns, key string) bool {
	fk := fullKey(ns, key)
	sh := s.getShard(fk)

	sh.mu.Lock()
	entry, ok := sh.items[fk]
	if ok {
		delete(sh.items, fk)
	}
	sh.mu.Unlock()

	if ok {
		s.deletes.Add(1)
		s.notifyWatchers("delete", fk, ns, entry)
	}
	return ok
}

// Scan belirli prefix ile eşleşen kayıtları döndür.
func (s *Store) Scan(ns, prefix string, limit int) []*Entry {
	fp := fullKey(ns, prefix)
	var results []*Entry

	for i := 0; i < shardCount; i++ {
		sh := s.shards[i]
		sh.mu.RLock()
		for k, entry := range sh.items {
			if strings.HasPrefix(k, fp) && !entry.IsExpired() {
				results = append(results, entry)
				if limit > 0 && len(results) >= limit {
					sh.mu.RUnlock()
					return results
				}
			}
		}
		sh.mu.RUnlock()
	}
	return results
}

// PutJSON JSON değer yaz (kolaylık metodu).
func (s *Store) PutJSON(ns, key string, val interface{}, ttl time.Duration) (*Entry, error) {
	data, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	return s.Put(ns, key, data, ttl, nil), nil
}

// GetJSON JSON değer oku.
func (s *Store) GetJSON(ns, key string, dest interface{}) error {
	entry, ok := s.Get(ns, key)
	if !ok {
		return ErrKeyNotFound
	}
	return json.Unmarshal(entry.Value, dest)
}

// Keys tüm anahtarları döndür (namespace bazında).
func (s *Store) Keys(ns string) []string {
	prefix := ""
	if ns != "" {
		prefix = ns + ":"
	}
	var keys []string
	for i := 0; i < shardCount; i++ {
		sh := s.shards[i]
		sh.mu.RLock()
		for k, entry := range sh.items {
			if !entry.IsExpired() && (prefix == "" || strings.HasPrefix(k, prefix)) {
				keys = append(keys, entry.Key)
			}
		}
		sh.mu.RUnlock()
	}
	return keys
}

// Count toplam kayıt sayısı.
func (s *Store) Count() int64 {
	var total int64
	for i := 0; i < shardCount; i++ {
		sh := s.shards[i]
		sh.mu.RLock()
		total += int64(len(sh.items))
		sh.mu.RUnlock()
	}
	return total
}

// Namespaces mevcut namespace'leri döndür.
func (s *Store) Namespaces() []string {
	nsMap := make(map[string]struct{})
	for i := 0; i < shardCount; i++ {
		sh := s.shards[i]
		sh.mu.RLock()
		for _, entry := range sh.items {
			if entry.Namespace != "" {
				nsMap[entry.Namespace] = struct{}{}
			}
		}
		sh.mu.RUnlock()
	}
	result := make([]string, 0, len(nsMap))
	for ns := range nsMap {
		result = append(result, ns)
	}
	return result
}

// Watch değişiklikleri izle.
func (s *Store) Watch(ns, prefix string, bufSize int) (uint64, *Watcher) {
	if bufSize <= 0 {
		bufSize = 128
	}
	id := s.watchID.Add(1)
	w := &Watcher{
		Ch:        make(chan WatchEvent, bufSize),
		Prefix:    prefix,
		Namespace: ns,
	}
	s.watchers.Store(id, w)
	return id, w
}

// Unwatch izlemeyi durdur.
func (s *Store) Unwatch(id uint64) {
	if v, ok := s.watchers.LoadAndDelete(id); ok {
		w := v.(*Watcher)
		close(w.Ch)
	}
}

func (s *Store) notifyWatchers(evType, fk, ns string, entry *Entry) {
	ev := WatchEvent{
		Type:      evType,
		Key:       entry.Key,
		Namespace: ns,
		Entry:     entry,
		Timestamp: time.Now().UnixNano(),
	}
	s.watchers.Range(func(_, val interface{}) bool {
		w := val.(*Watcher)
		if w.Namespace != "" && w.Namespace != ns {
			return true
		}
		if w.Prefix != "" && !strings.HasPrefix(fk, fullKey(w.Namespace, w.Prefix)) {
			return true
		}
		select {
		case w.Ch <- ev:
		default:
			// Tampon dolu, at
		}
		return true
	})
}

// Cleanup süresi dolmuş kayıtları temizle.
func (s *Store) Cleanup() int64 {
	var cleaned int64
	now := time.Now().UnixNano()
	for i := 0; i < shardCount; i++ {
		sh := s.shards[i]
		sh.mu.Lock()
		for k, entry := range sh.items {
			if entry.ExpiresAt > 0 && now > entry.ExpiresAt {
				delete(sh.items, k)
				cleaned++
				s.expired.Add(1)
			}
		}
		sh.mu.Unlock()
	}
	return cleaned
}

// StartCleanup periyodik temizlik başlat.
func (s *Store) StartCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.Cleanup()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// GetStats istatistikleri döndür.
func (s *Store) GetStats() Stats {
	return Stats{
		TotalKeys:    s.Count(),
		TotalGets:    s.gets.Load(),
		TotalPuts:    s.puts.Load(),
		TotalDeletes: s.deletes.Load(),
		TotalHits:    s.hits.Load(),
		TotalMisses:  s.misses.Load(),
		TotalExpired: s.expired.Load(),
		Namespaces:   len(s.Namespaces()),
		UptimeNs:     time.Now().UnixNano() - s.startTime,
	}
}

// Close depoyu kapat.
func (s *Store) Close() {
	close(s.stopCh)
	s.watchers.Range(func(key, val interface{}) bool {
		w := val.(*Watcher)
		close(w.Ch)
		s.watchers.Delete(key)
		return true
	})
}

// Snapshot tüm veriyi serileştir (yedekleme için).
func (s *Store) Snapshot() ([]byte, error) {
	entries := make([]*Entry, 0, s.Count())
	for i := 0; i < shardCount; i++ {
		sh := s.shards[i]
		sh.mu.RLock()
		for _, entry := range sh.items {
			if !entry.IsExpired() {
				entries = append(entries, entry)
			}
		}
		sh.mu.RUnlock()
	}
	return json.Marshal(entries)
}

// Restore snapshot'tan geri yükle.
func (s *Store) Restore(data []byte) error {
	var entries []*Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	for _, entry := range entries {
		fk := fullKey(entry.Namespace, entry.Key)
		sh := s.getShard(fk)
		sh.mu.Lock()
		sh.items[fk] = entry
		sh.mu.Unlock()
	}
	return nil
}
