package wal

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDir(t *testing.T) string {
	dir := filepath.Join(os.TempDir(), "kutuk-wal-test")
	os.RemoveAll(dir)
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestWriteAndReplay(t *testing.T) {
	dir := tempDir(t)
	w, err := New(dir, 1024*1024, true)
	if err != nil {
		t.Fatal(err)
	}

	// 10 kayıt yaz
	for i := 0; i < 10; i++ {
		err := w.AppendPut("entity", "key", []byte(`{"v":1}`), 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	w.Close()

	// Replay
	w2, err := New(dir, 1024*1024, true)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()

	count := 0
	_, err = w2.Replay(func(rec Record) error {
		if rec.Type != RecordPut {
			t.Fatalf("beklenmeyen tip: %d", rec.Type)
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 10 {
		t.Fatalf("beklenen 10 kayıt, okunan: %d", count)
	}
}

func TestDeleteRecord(t *testing.T) {
	dir := tempDir(t)
	w, err := New(dir, 1024*1024, true)
	if err != nil {
		t.Fatal(err)
	}

	w.AppendPut("entity", "k1", []byte("v1"), 0)
	w.AppendDelete("entity", "k1")
	w.Close()

	w2, err := New(dir, 1024*1024, true)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()

	var types []RecordType
	w2.Replay(func(rec Record) error {
		types = append(types, rec.Type)
		return nil
	})

	if len(types) != 2 || types[0] != RecordPut || types[1] != RecordDelete {
		t.Fatalf("beklenmeyen kayıt tipleri: %v", types)
	}
}

func TestSegmentRotation(t *testing.T) {
	dir := tempDir(t)
	// Küçük segment boyutu ile döndürme tetikle
	w, err := New(dir, 256, true)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 50; i++ {
		w.AppendPut("test", "key", []byte(`{"data":"some-payload-data-to-fill-segment"}`), 0)
	}
	w.Close()

	// Birden fazla segment oluşmuş olmalı
	entries, _ := os.ReadDir(dir)
	walFiles := 0
	for _, e := range entries {
		if !e.IsDir() {
			walFiles++
		}
	}
	if walFiles < 2 {
		t.Fatalf("segment döndürme çalışmadı, dosya sayısı: %d", walFiles)
	}

	// Hepsi replay edilebilmeli
	w2, err := New(dir, 1024*1024, true)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()

	count := 0
	w2.Replay(func(rec Record) error {
		count++
		return nil
	})
	if count != 50 {
		t.Fatalf("beklenen 50 kayıt, okunan: %d", count)
	}
}

func TestTruncate(t *testing.T) {
	dir := tempDir(t)
	w, err := New(dir, 256, true)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 50; i++ {
		w.AppendPut("test", "key", []byte(`{"data":"payload-to-fill"}`), 0)
	}

	w.Truncate()

	entries, _ := os.ReadDir(dir)
	walFiles := 0
	for _, e := range entries {
		if !e.IsDir() {
			walFiles++
		}
	}
	// Sadece mevcut segment kalmalı
	if walFiles != 1 {
		t.Fatalf("truncate sonrası beklenen 1 dosya, mevcut: %d", walFiles)
	}
	w.Close()
}

func TestStats(t *testing.T) {
	dir := tempDir(t)
	w, err := New(dir, 1024*1024, true)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	w.AppendPut("ns", "k", []byte("v"), 0)
	w.AppendPut("ns", "k2", []byte("v2"), 0)

	writes, bytes := w.Stats()
	if writes != 2 {
		t.Fatalf("beklenen 2 yazma, alınan: %d", writes)
	}
	if bytes == 0 {
		t.Fatal("byte sayısı 0 olmamalı")
	}
}
