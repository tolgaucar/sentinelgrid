package raft

import (
	"sync/atomic"
	"testing"
)

func TestSingleNodeLeader(t *testing.T) {
	var applied atomic.Int64
	n := NewNode("node-1", nil, func(entry LogEntry) error {
		applied.Add(1)
		return nil
	})
	n.Start()
	defer n.Stop()

	if !n.IsLeader() {
		t.Fatal("tek düğüm lider olmalı")
	}

	state := n.GetState()
	if state.Role != "leader" || state.Term != 1 {
		t.Fatalf("beklenmeyen durum: %+v", state)
	}
}

func TestPropose(t *testing.T) {
	var applied atomic.Int64
	n := NewNode("node-1", nil, func(entry LogEntry) error {
		applied.Add(1)
		return nil
	})
	n.Start()
	defer n.Stop()

	idx, err := n.Propose([]byte(`{"op":"put","key":"k1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if idx != 1 {
		t.Fatalf("beklenen index 1, alınan: %d", idx)
	}
	if applied.Load() != 1 {
		t.Fatal("log uygulanmalıydı")
	}
}

func TestProposeMultiple(t *testing.T) {
	var count atomic.Int64
	n := NewNode("node-1", nil, func(entry LogEntry) error {
		count.Add(1)
		return nil
	})
	n.Start()
	defer n.Stop()

	for i := 0; i < 100; i++ {
		_, err := n.Propose([]byte(`{"op":"put"}`))
		if err != nil {
			t.Fatal(err)
		}
	}
	if count.Load() != 100 {
		t.Fatalf("beklenen 100, uygulanan: %d", count.Load())
	}

	state := n.GetState()
	if state.LogLen != 100 || state.CommitIndex != 100 {
		t.Fatalf("log durumu yanlış: %+v", state)
	}
}

func TestNonLeaderCannotPropose(t *testing.T) {
	n := NewNode("node-1", []string{"node-2", "node-3"}, nil)
	// Başlatma - follower kalacak çünkü timer tetiklenmeyecek

	_, err := n.Propose([]byte(`{"op":"put"}`))
	if err == nil {
		t.Fatal("follower öneri kabul etmemeli")
	}
}

func TestVoteRequest(t *testing.T) {
	n := NewNode("node-1", []string{"node-2"}, nil)
	n.Start()
	defer n.Stop()

	resp := n.HandleVoteRequest(VoteRequest{
		Term:        5,
		CandidateID: "node-2",
		LastLogIdx:  0,
		LastLogTerm: 0,
	})

	if !resp.Granted {
		t.Fatal("oy verilmeliydi")
	}
	if n.Term() != 5 {
		t.Fatalf("term güncellenmeli: %d", n.Term())
	}
}

func TestAppendEntries(t *testing.T) {
	var applied atomic.Int64
	n := NewNode("node-1", []string{"node-2"}, func(entry LogEntry) error {
		applied.Add(1)
		return nil
	})

	resp := n.HandleAppendEntries(AppendRequest{
		Term:     3,
		LeaderID: "node-2",
		Entries: []LogEntry{
			{Term: 3, Index: 1, Command: []byte("cmd1")},
			{Term: 3, Index: 2, Command: []byte("cmd2")},
		},
		CommitID: 2,
	})

	if !resp.Success {
		t.Fatal("append başarılı olmalı")
	}
	if applied.Load() != 2 {
		t.Fatalf("2 log uygulanmalı, uygulanan: %d", applied.Load())
	}

	state := n.GetState()
	if state.Role != "follower" || state.LeaderID != "node-2" {
		t.Fatalf("follower olmalı: %+v", state)
	}
}
