package queue

import (
	"fmt"
	"sync"
	"testing"
)

func TestRingBuffer_EnqueueDequeue(t *testing.T) {
	rb := NewRingBuffer(8)

	// Boş kuyruktan okuma
	_, err := rb.Dequeue()
	if err != ErrQueueEmpty {
		t.Fatalf("boş kuyruktan ErrQueueEmpty beklendi, alındı: %v", err)
	}

	// Mesaj ekle ve oku
	msg := Message{ID: "m1", Topic: "test", Payload: []byte("veri"), Priority: 1}
	if err := rb.Enqueue(msg); err != nil {
		t.Fatalf("enqueue hatası: %v", err)
	}

	if rb.Len() != 1 {
		t.Fatalf("uzunluk 1 beklendi, alındı: %d", rb.Len())
	}

	got, err := rb.Dequeue()
	if err != nil {
		t.Fatalf("dequeue hatası: %v", err)
	}
	if got.ID != "m1" {
		t.Fatalf("ID 'm1' beklendi, alındı: %s", got.ID)
	}
}

func TestRingBuffer_Full(t *testing.T) {
	rb := NewRingBuffer(2)

	rb.Enqueue(Message{ID: "1"})
	rb.Enqueue(Message{ID: "2"})

	if !rb.IsFull() {
		t.Fatal("kuyruk dolu olmalıydı")
	}

	err := rb.Enqueue(Message{ID: "3"})
	if err != ErrQueueFull {
		t.Fatalf("ErrQueueFull beklendi, alındı: %v", err)
	}
}

func TestRingBuffer_FIFO(t *testing.T) {
	rb := NewRingBuffer(16)

	for i := 0; i < 10; i++ {
		rb.Enqueue(Message{ID: fmt.Sprintf("m%d", i)})
	}

	for i := 0; i < 10; i++ {
		msg, err := rb.Dequeue()
		if err != nil {
			t.Fatalf("dequeue hatası: %v", err)
		}
		expected := fmt.Sprintf("m%d", i)
		if msg.ID != expected {
			t.Fatalf("FIFO sırası bozuldu: beklenen %s, alınan %s", expected, msg.ID)
		}
	}
}

func TestRingBuffer_Close(t *testing.T) {
	rb := NewRingBuffer(4)
	rb.Enqueue(Message{ID: "1"})
	rb.Close()

	if !rb.IsClosed() {
		t.Fatal("kuyruk kapalı olmalıydı")
	}

	err := rb.Enqueue(Message{ID: "2"})
	if err != ErrClosed {
		t.Fatalf("ErrClosed beklendi, alındı: %v", err)
	}

	// Kapalı ama içinde mesaj olan kuyruktan okuma yapılabilmeli
	msg, err := rb.Dequeue()
	if err != nil {
		t.Fatalf("kapalı kuyruktan mevcut mesaj okunabilmeli: %v", err)
	}
	if msg.ID != "1" {
		t.Fatalf("ID '1' beklendi: %s", msg.ID)
	}
}

func TestRingBuffer_Drain(t *testing.T) {
	rb := NewRingBuffer(16)
	for i := 0; i < 5; i++ {
		rb.Enqueue(Message{ID: fmt.Sprintf("d%d", i)})
	}

	msgs := rb.Drain()
	if len(msgs) != 5 {
		t.Fatalf("5 mesaj beklendi, alındı: %d", len(msgs))
	}
	if !rb.IsEmpty() {
		t.Fatal("drain sonrası kuyruk boş olmalıydı")
	}
}

func TestRingBuffer_Concurrent(t *testing.T) {
	rb := NewRingBuffer(10000)
	var wg sync.WaitGroup
	n := 1000

	// Eşzamanlı yazma
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(id int) {
			defer wg.Done()
			rb.Enqueue(Message{ID: fmt.Sprintf("c%d", id)})
		}(i)
	}
	wg.Wait()

	if rb.Len() != int64(n) {
		t.Fatalf("%d mesaj beklendi, alındı: %d", n, rb.Len())
	}

	// Eşzamanlı okuma
	var readCount int64
	wg.Add(n)
	var mu sync.Mutex
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if _, err := rb.Dequeue(); err == nil {
				mu.Lock()
				readCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if readCount != int64(n) {
		t.Fatalf("%d okuma beklendi, alındı: %d", n, readCount)
	}
}

// fmt import kullanıldığından bu satır lint uyarısı için:
var _ = fmt.Sprintf
