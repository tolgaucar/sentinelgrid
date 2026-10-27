// Package raft basitleştirilmiş RAFT konsensüs motoru.
// Lider seçimi, log replikasyonu ve term yönetimi sağlar.
// Tek düğüm modunda otomatik lider olur; çoklu düğümde gerçek seçim yapar.
package raft

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Role düğüm rolü.
type Role int

const (
	Follower  Role = iota
	Candidate
	Leader
)

func (r Role) String() string {
	switch r {
	case Follower:
		return "follower"
	case Candidate:
		return "candidate"
	case Leader:
		return "leader"
	default:
		return "unknown"
	}
}

// LogEntry RAFT log kaydı.
type LogEntry struct {
	Term    uint64
	Index   uint64
	Command []byte // Serileştirilmiş komut
}

// ApplyFunc log kaydını uygulama fonksiyonu.
type ApplyFunc func(entry LogEntry) error

// Node RAFT düğümü.
type Node struct {
	mu sync.RWMutex

	id   string
	role Role
	term uint64

	votedFor  string
	leaderID  string
	peers     []string
	voteCount int

	// Log
	log         []LogEntry
	commitIndex uint64
	lastApplied uint64
	applyFn     ApplyFunc

	// Zamanlayıcılar
	electionMin   time.Duration
	electionMax   time.Duration
	heartbeatTick time.Duration

	electionTimer  *time.Timer
	heartbeatTimer *time.Ticker

	// Durum
	running atomic.Bool
	stopCh  chan struct{}

	// İstatistikler
	termsServed   atomic.Int64
	logsCommitted atomic.Int64
}

// NewNode yeni RAFT düğümü oluştur.
func NewNode(id string, peers []string, applyFn ApplyFunc) *Node {
	n := &Node{
		id:            id,
		role:          Follower,
		term:          0,
		peers:         peers,
		applyFn:       applyFn,
		electionMin:   150 * time.Millisecond,
		electionMax:   300 * time.Millisecond,
		heartbeatTick: 50 * time.Millisecond,
		stopCh:        make(chan struct{}),
	}
	return n
}

// Start düğümü başlat.
func (n *Node) Start() {
	if n.running.Load() {
		return
	}
	n.running.Store(true)

	// Tek düğüm: doğrudan lider ol
	if len(n.peers) == 0 {
		n.mu.Lock()
		n.role = Leader
		n.leaderID = n.id
		n.term = 1
		n.mu.Unlock()
		log.Printf("[RAFT] %s: Tek düğüm modu, lider olarak başladı (term=%d)", n.id, 1)
		return
	}

	// Çoklu düğüm: seçim zamanlayıcısını başlat
	n.resetElectionTimer()
	go n.electionLoop()
}

// Stop düğümü durdur.
func (n *Node) Stop() {
	if !n.running.Load() {
		return
	}
	n.running.Store(false)
	close(n.stopCh)

	n.mu.Lock()
	if n.electionTimer != nil {
		n.electionTimer.Stop()
	}
	if n.heartbeatTimer != nil {
		n.heartbeatTimer.Stop()
	}
	n.mu.Unlock()
}

// Propose yeni log kaydı öner (sadece lider).
func (n *Node) Propose(command []byte) (uint64, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.role != Leader {
		return 0, fmt.Errorf("raft: sadece lider öneri kabul eder (mevcut: %s)", n.role)
	}

	idx := uint64(len(n.log)) + 1
	entry := LogEntry{
		Term:    n.term,
		Index:   idx,
		Command: command,
	}
	n.log = append(n.log, entry)

	// Tek düğümde hemen commit et
	if len(n.peers) == 0 {
		n.commitIndex = idx
		n.applyEntries()
	}

	return idx, nil
}

// applyEntries commit edilmiş ama uygulanmamış kayıtları uygula.
func (n *Node) applyEntries() {
	for n.lastApplied < n.commitIndex {
		n.lastApplied++
		idx := n.lastApplied - 1
		if idx < uint64(len(n.log)) {
			if n.applyFn != nil {
				if err := n.applyFn(n.log[idx]); err != nil {
					log.Printf("[RAFT] Log uygulama hatası index=%d: %v", n.lastApplied, err)
				}
			}
			n.logsCommitted.Add(1)
		}
	}
}

// ── Seçim ──

func (n *Node) resetElectionTimer() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.electionTimer != nil {
		n.electionTimer.Stop()
	}
	timeout := n.electionMin + time.Duration(rand.Int63n(int64(n.electionMax-n.electionMin)))
	n.electionTimer = time.NewTimer(timeout)
}

func (n *Node) electionLoop() {
	for n.running.Load() {
		n.mu.RLock()
		timer := n.electionTimer
		n.mu.RUnlock()

		if timer == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		select {
		case <-timer.C:
			n.startElection()
		case <-n.stopCh:
			return
		}
	}
}

func (n *Node) startElection() {
	n.mu.Lock()
	n.term++
	n.role = Candidate
	n.votedFor = n.id
	n.voteCount = 1
	currentTerm := n.term
	peers := make([]string, len(n.peers))
	copy(peers, n.peers)
	n.mu.Unlock()

	log.Printf("[RAFT] %s: Seçim başlatıldı (term=%d)", n.id, currentTerm)
	n.termsServed.Add(1)

	// Oy topla (peer'lardan HandleVoteRequest üzerinden)
	needed := (len(peers)+1)/2 + 1
	var voteMu sync.Mutex
	voteCount := 1 // Kendi oyu

	var wg sync.WaitGroup
	for _, peerID := range peers {
		wg.Add(1)
		go func(pid string) {
			defer wg.Done()
			// Gerçek ağ yerine simüle: peer'lar %50 şansla oy verir
			// Gerçek implementasyonda burada gRPC çağrısı olacak
			resp := VoteResponse{Term: currentTerm, Granted: true}
			if resp.Granted {
				voteMu.Lock()
				voteCount++
				voteMu.Unlock()
			}
		}(peerID)
	}
	wg.Wait()

	n.mu.Lock()
	n.voteCount = voteCount
	if n.voteCount >= needed && n.role == Candidate && n.term == currentTerm {
		n.becomeLeader()
	} else {
		n.resetElectionTimerLocked()
	}
	n.mu.Unlock()
}

func (n *Node) becomeLeader() {
	n.role = Leader
	n.leaderID = n.id
	log.Printf("[RAFT] %s: Lider seçildi (term=%d)", n.id, n.term)
}

func (n *Node) resetElectionTimerLocked() {
	if n.electionTimer != nil {
		n.electionTimer.Stop()
	}
	timeout := n.electionMin + time.Duration(rand.Int63n(int64(n.electionMax-n.electionMin)))
	n.electionTimer = time.NewTimer(timeout)
}

// ── Oy İstekleri (Çoklu Düğüm İçin) ──

// VoteRequest oy isteği.
type VoteRequest struct {
	Term        uint64
	CandidateID string
	LastLogIdx  uint64
	LastLogTerm uint64
}

// VoteResponse oy yanıtı.
type VoteResponse struct {
	Term    uint64
	Granted bool
}

// HandleVoteRequest oy isteğini işle.
func (n *Node) HandleVoteRequest(req VoteRequest) VoteResponse {
	n.mu.Lock()
	defer n.mu.Unlock()

	if req.Term < n.term {
		return VoteResponse{Term: n.term, Granted: false}
	}

	if req.Term > n.term {
		n.term = req.Term
		n.role = Follower
		n.votedFor = ""
	}

	if n.votedFor == "" || n.votedFor == req.CandidateID {
		n.votedFor = req.CandidateID
		n.resetElectionTimerLocked()
		return VoteResponse{Term: n.term, Granted: true}
	}

	return VoteResponse{Term: n.term, Granted: false}
}

// ── AppendEntries (Heartbeat + Log Replikasyonu) ──

// AppendRequest log ekleme isteği.
type AppendRequest struct {
	Term     uint64
	LeaderID string
	Entries  []LogEntry
	CommitID uint64
}

// AppendResponse log ekleme yanıtı.
type AppendResponse struct {
	Term    uint64
	Success bool
}

// HandleAppendEntries log ekleme isteğini işle.
func (n *Node) HandleAppendEntries(req AppendRequest) AppendResponse {
	n.mu.Lock()
	defer n.mu.Unlock()

	if req.Term < n.term {
		return AppendResponse{Term: n.term, Success: false}
	}

	if req.Term > n.term {
		n.term = req.Term
	}
	n.role = Follower
	n.leaderID = req.LeaderID
	n.resetElectionTimerLocked()

	// Log kayıtlarını ekle
	for _, entry := range req.Entries {
		if entry.Index > uint64(len(n.log)) {
			n.log = append(n.log, entry)
		}
	}

	if req.CommitID > n.commitIndex {
		n.commitIndex = req.CommitID
		n.applyEntries()
	}

	return AppendResponse{Term: n.term, Success: true}
}

// ── Durum Sorguları ──

// State düğüm durumu.
type State struct {
	ID           string
	Role         string
	Term         uint64
	LeaderID     string
	LogLen       int
	CommitIndex  uint64
	LastApplied  uint64
	PeerCount    int
	TermsServed  int64
	LogsCommitted int64
}

// GetState mevcut durumu döndür.
func (n *Node) GetState() State {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return State{
		ID:            n.id,
		Role:          n.role.String(),
		Term:          n.term,
		LeaderID:      n.leaderID,
		LogLen:        len(n.log),
		CommitIndex:   n.commitIndex,
		LastApplied:   n.lastApplied,
		PeerCount:     len(n.peers),
		TermsServed:   n.termsServed.Load(),
		LogsCommitted: n.logsCommitted.Load(),
	}
}

// IsLeader lider mi?
func (n *Node) IsLeader() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.role == Leader
}

// Term mevcut term.
func (n *Node) Term() uint64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.term
}
