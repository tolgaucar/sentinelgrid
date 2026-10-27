package transport

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/otukenmatris/bag/internal/queue"
)

func TestSender_SuccessFirstTry(t *testing.T) {
	sender := NewSender(3, 10*time.Millisecond, 100*time.Millisecond, 1*time.Second)

	msg := queue.Message{ID: "s1", Topic: "test"}
	handler := func(m queue.Message) error { return nil }

	err := sender.Send(context.Background(), msg, handler)
	if err != nil {
		t.Fatalf("başarılı gönderim beklendi: %v", err)
	}

	sent, failed, _ := sender.Stats()
	if sent != 1 || failed != 0 {
		t.Fatalf("1 gönderim beklendi: sent=%d failed=%d", sent, failed)
	}
}

func TestSender_RetryAndSucceed(t *testing.T) {
	sender := NewSender(3, 1*time.Millisecond, 10*time.Millisecond, 1*time.Second)

	attempt := 0
	handler := func(m queue.Message) error {
		attempt++
		if attempt < 3 {
			return errors.New("geçici hata")
		}
		return nil
	}

	msg := queue.Message{ID: "r1", Topic: "test"}
	err := sender.Send(context.Background(), msg, handler)
	if err != nil {
		t.Fatalf("retry sonrası başarı beklendi: %v", err)
	}

	_, _, retried := sender.Stats()
	if retried < 1 {
		t.Fatalf("en az 1 retry beklendi: %d", retried)
	}
}

func TestSender_MaxRetriesExceeded(t *testing.T) {
	sender := NewSender(2, 1*time.Millisecond, 10*time.Millisecond, 1*time.Second)

	handler := func(m queue.Message) error {
		return errors.New("kalıcı hata")
	}

	msg := queue.Message{ID: "f1", Topic: "test"}
	err := sender.Send(context.Background(), msg, handler)
	if err == nil {
		t.Fatal("hata beklendi")
	}
	if !errors.Is(err, ErrMaxRetries) {
		t.Fatalf("ErrMaxRetries beklendi: %v", err)
	}

	_, failed, _ := sender.Stats()
	if failed != 1 {
		t.Fatalf("1 failed beklendi: %d", failed)
	}
}

func TestSender_ContextCancellation(t *testing.T) {
	sender := NewSender(5, 100*time.Millisecond, 1*time.Second, 1*time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	handler := func(m queue.Message) error {
		return errors.New("hata")
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	msg := queue.Message{ID: "ctx1", Topic: "test"}
	err := sender.Send(ctx, msg, handler)
	if err == nil {
		t.Fatal("context iptal hatası beklendi")
	}
}

func TestReceiver_ProcessMessage(t *testing.T) {
	recv := NewReceiver()

	var processed bool
	recv.RegisterHandler("sensor.radar", func(m queue.Message) error {
		processed = true
		return nil
	})

	msg := queue.Message{ID: "p1", Topic: "sensor.radar"}
	err := recv.Process(msg)
	if err != nil {
		t.Fatalf("işleme hatası: %v", err)
	}
	if !processed {
		t.Fatal("mesaj işlenmeliydi")
	}

	received, proc, errs := recv.Stats()
	if received != 1 || proc != 1 || errs != 0 {
		t.Fatalf("stats hatalı: recv=%d proc=%d err=%d", received, proc, errs)
	}
}

func TestReceiver_NoHandler(t *testing.T) {
	recv := NewReceiver()

	msg := queue.Message{ID: "nh1", Topic: "unknown_topic"}
	err := recv.Process(msg)
	if err == nil {
		t.Fatal("handler yok hatası beklendi")
	}
	if !errors.Is(err, ErrNoHandler) {
		t.Fatalf("ErrNoHandler beklendi: %v", err)
	}
}

func TestDispatcher_BatchFlush(t *testing.T) {
	sender := NewSender(1, time.Millisecond, time.Millisecond, time.Second)
	recv := NewReceiver()

	var count int
	recv.RegisterHandler("batch", func(m queue.Message) error {
		count++
		return nil
	})

	disp := NewDispatcher(sender, recv, 5, 50*time.Millisecond)

	// 5 mesaj ekle (batch size'a ulaşınca otomatik flush)
	for i := 0; i < 5; i++ {
		disp.Add(queue.Message{ID: "b", Topic: "batch"})
	}

	time.Sleep(10 * time.Millisecond) // flush'ın tamamlanmasını bekle

	if count != 5 {
		t.Fatalf("5 mesaj işlenmeli: %d", count)
	}
}
