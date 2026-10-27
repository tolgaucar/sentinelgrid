// Package sync — YURT çift yönlü asenkron senkronizasyon motoru.
//
// Edge düğüm ile bulut arası delta senkronizasyon, vector clock tabanlı
// çatışma tespiti ve çözümü (LWW, CloudWins, EdgeWins, MergeAll).
package sync

import (
	"fmt"
	"sync"
	"time"

	"github.com/otukenmatris/yurt/internal/config"
	"github.com/otukenmatris/yurt/internal/store"
)

// SyncPair — bir anahtar için yerel ve uzak versiyon
type SyncPair struct {
	Key        string
	LocalEntry *store.Entry
	RemoteEntry *store.Entry
	Conflict   bool
}

// ConflictRecord — çatışma kaydı
type ConflictRecord struct {
	Key         string           `json:"key"`
	LocalVC     store.VectorClock `json:"local_vc"`
	RemoteVC    store.VectorClock `json:"remote_vc"`
	Resolution  string           `json:"resolution"`
	Winner      string           `json:"winner"` // local, remote, merged
	ResolvedAt  time.Time        `json:"resolved_at"`
}

// Engine — senkronizasyon motoru
type Engine struct {
	mu            sync.Mutex
	localStore    *store.Store
	remoteStore   *store.Store // simülasyon için bellek-içi uzak depo
	cfg           config.Config
	state         config.SyncState
	conflicts     []ConflictRecord
	stats         Stats
	lastSync      time.Time
	stopCh        chan struct{}
	running       bool
}

// Stats — senkronizasyon istatistikleri
type Stats struct {
	PushCount      int64 `json:"push_count"`
	PullCount      int64 `json:"pull_count"`
	ConflictCount  int64 `json:"conflict_count"`
	ResolvedCount  int64 `json:"resolved_count"`
	SyncCycles     int64 `json:"sync_cycles"`
	ErrorCount     int64 `json:"error_count"`
	BytesSynced    int64 `json:"bytes_synced"`
	LastSyncMs     int64 `json:"last_sync_ms"`
}

// New — yeni senkronizasyon motoru
func New(local *store.Store, cfg config.Config) *Engine {
	return &Engine{
		localStore:  local,
		remoteStore: store.New("cloud", cfg.MaxEntries, cfg.DefaultTTL),
		cfg:         cfg,
		conflicts:   make([]ConflictRecord, 0, 128),
		stopCh:      make(chan struct{}),
	}
}

// SetRemoteStore — uzak depoyu ayarla (test için)
func (e *Engine) SetRemoteStore(remote *store.Store) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.remoteStore = remote
}

// GetState — senkronizasyon durumu
func (e *Engine) GetState() config.SyncState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

// Push — yerel dirty kayıtları buluta gönder
func (e *Engine) Push() (int, error) {
	e.mu.Lock()
	e.state = config.SyncPushing
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.state = config.SyncIdle
		e.mu.Unlock()
	}()

	dirtyEntries := e.localStore.GetDirty(e.cfg.SyncBatchSize)
	if len(dirtyEntries) == 0 {
		return 0, nil
	}

	pushed := 0
	cleanKeys := make([]string, 0, len(dirtyEntries))

	for _, entry := range dirtyEntries {
		// Uzak depoda çakışma kontrolü
		remote, remoteExists := e.remoteStore.Get(entry.Key)

		if remoteExists && remote.Version.Concurrent(entry.Version) {
			// Çatışma! Çözüm politikasına göre birleştir
			resolved := e.resolveConflict(&entry, remote)
			e.remoteStore.PutWithVersion(entry.Key, resolved.Value, resolved.Version, resolved.Source, resolved.ModifiedAt)
			e.stats.ConflictCount++
			e.stats.ResolvedCount++
		} else {
			// Çakışma yok — doğrudan gönder
			e.remoteStore.PutWithVersion(entry.Key, entry.Value, entry.Version, entry.Source, entry.ModifiedAt)
		}

		cleanKeys = append(cleanKeys, entry.Key)
		pushed++
	}

	e.localStore.MarkClean(cleanKeys)
	e.stats.PushCount += int64(pushed)
	return pushed, nil
}

// Pull — buluttaki yeni verileri yerel depoya çek
func (e *Engine) Pull() (int, error) {
	e.mu.Lock()
	e.state = config.SyncPulling
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.state = config.SyncIdle
		e.mu.Unlock()
	}()

	remoteSnapshot := e.remoteStore.Snapshot()
	pulled := 0

	for key, remoteEntry := range remoteSnapshot {
		localEntry, localExists := e.localStore.Get(key)

		if !localExists {
			// Yeni kayıt — doğrudan al
			e.localStore.PutWithVersion(key, remoteEntry.Value, remoteEntry.Version, remoteEntry.Source, remoteEntry.ModifiedAt)
			pulled++
		} else if remoteEntry.Version.HappensBefore(localEntry.Version) {
			// Yerel daha yeni — atla
			continue
		} else if localEntry.Version.HappensBefore(remoteEntry.Version) {
			// Uzak daha yeni — güncelle
			e.localStore.PutWithVersion(key, remoteEntry.Value, remoteEntry.Version, remoteEntry.Source, remoteEntry.ModifiedAt)
			pulled++
		} else if remoteEntry.Version.Concurrent(localEntry.Version) {
			// Çatışma
			resolved := e.resolveConflict(localEntry, &remoteEntry)
			e.localStore.PutWithVersion(key, resolved.Value, resolved.Version, resolved.Source, resolved.ModifiedAt)
			e.stats.ConflictCount++
			e.stats.ResolvedCount++
			pulled++
		}
	}

	e.stats.PullCount += int64(pulled)
	return pulled, nil
}

// Sync — tam çift yönlü senkronizasyon
func (e *Engine) Sync() (pushed int, pulled int, err error) {
	start := time.Now()
	pushed, err = e.Push()
	if err != nil {
		e.stats.ErrorCount++
		return 0, 0, fmt.Errorf("push hatası: %w", err)
	}

	pulled, err = e.Pull()
	if err != nil {
		e.stats.ErrorCount++
		return pushed, 0, fmt.Errorf("pull hatası: %w", err)
	}

	e.mu.Lock()
	e.lastSync = time.Now()
	e.stats.SyncCycles++
	e.stats.LastSyncMs = time.Since(start).Milliseconds()
	e.mu.Unlock()

	return pushed, pulled, nil
}

// resolveConflict — çatışma çözümü
func (e *Engine) resolveConflict(local *store.Entry, remote *store.Entry) *store.Entry {
	record := ConflictRecord{
		Key:        local.Key,
		LocalVC:    local.Version.Clone(),
		RemoteVC:   remote.Version.Clone(),
		ResolvedAt: time.Now(),
	}

	result := &store.Entry{
		Key:        local.Key,
		ModifiedAt: time.Now(),
	}

	switch e.cfg.ConflictPolicy {
	case config.LWW:
		if local.ModifiedAt.After(remote.ModifiedAt) {
			result.Value = local.Value
			result.Source = local.Source
			record.Winner = "local"
		} else {
			result.Value = remote.Value
			result.Source = remote.Source
			record.Winner = "remote"
		}
		record.Resolution = "LWW"

	case config.CloudWins:
		result.Value = remote.Value
		result.Source = remote.Source
		record.Winner = "remote"
		record.Resolution = "CLOUD_WINS"

	case config.EdgeWins:
		result.Value = local.Value
		result.Source = local.Source
		record.Winner = "local"
		record.Resolution = "EDGE_WINS"

	case config.MergeAll:
		merged := make(map[string]interface{})
		// Önce remote, sonra local (local üzerine yazar)
		for k, v := range remote.Value {
			merged[k] = v
		}
		for k, v := range local.Value {
			merged[k] = v
		}
		result.Value = merged
		result.Source = "merged"
		record.Winner = "merged"
		record.Resolution = "MERGE_ALL"
	}

	// Birleştirilmiş vector clock
	mergedVC := local.Version.Clone()
	mergedVC.Merge(remote.Version)
	mergedVC.Increment(e.cfg.NodeID)
	result.Version = mergedVC

	e.mu.Lock()
	e.conflicts = append(e.conflicts, record)
	if len(e.conflicts) > 1000 {
		e.conflicts = e.conflicts[len(e.conflicts)-500:]
	}
	e.mu.Unlock()

	return result
}

// Start — periyodik senkronizasyonu başlat
func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.mu.Unlock()

	go func() {
		ticker := time.NewTicker(e.cfg.SyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-e.stopCh:
				return
			case <-ticker.C:
				e.Sync()
			}
		}
	}()
}

// Stop — senkronizasyonu durdur
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		e.running = false
		close(e.stopCh)
	}
}

// GetConflicts — çatışma geçmişi
func (e *Engine) GetConflicts(n int) []ConflictRecord {
	e.mu.Lock()
	defer e.mu.Unlock()
	if n <= 0 || n > len(e.conflicts) {
		n = len(e.conflicts)
	}
	result := make([]ConflictRecord, n)
	copy(result, e.conflicts[len(e.conflicts)-n:])
	return result
}

// GetStats — senkronizasyon istatistikleri
func (e *Engine) GetStats() Stats {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stats
}

// GetLastSync — son senkronizasyon zamanı
func (e *Engine) GetLastSync() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastSync
}
