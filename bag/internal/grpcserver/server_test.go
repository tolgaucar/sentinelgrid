package grpcserver

import (
	"context"
	"testing"
	"time"

	"github.com/otukenmatris/bag/internal/broker"
	"github.com/otukenmatris/bag/internal/transport"
)

func newTestServer() *Server {
	brk := broker.NewBroker(broker.BrokerOpts{
		DefaultPartitions: 2,
		MaxQueueSize:      10000,
		MaxTopics:         100,
	})
	sender := transport.NewSender(3, time.Millisecond, 10*time.Millisecond, time.Second)
	receiver := transport.NewReceiver()
	return NewServer(brk, sender, receiver)
}

func TestServer_CreateTopicAndSend(t *testing.T) {
	srv := newTestServer()
	defer srv.Stop()

	if err := srv.CreateTopic("test.topic", 2); err != nil {
		t.Fatalf("topic oluşturma hatası: %v", err)
	}

	env := Envelope{
		ID:          "e1",
		Topic:       "test.topic",
		Source:      "test",
		Payload:     []byte("hello"),
		TimestampNs: time.Now().UnixNano(),
		Priority:    5,
	}

	ack := srv.Send(context.Background(), env)
	if !ack.Success {
		t.Fatalf("gönderim başarısız: %s", ack.Error)
	}
}

func TestServer_SendBatch(t *testing.T) {
	srv := newTestServer()
	defer srv.Stop()

	srv.CreateTopic("batch.topic", 2)

	envs := make([]Envelope, 10)
	for i := range envs {
		envs[i] = Envelope{
			ID:          "b" + string(rune('0'+i)),
			Topic:       "batch.topic",
			Payload:     []byte("data"),
			TimestampNs: time.Now().UnixNano(),
		}
	}

	acks, success, fail := srv.SendBatch(context.Background(), envs)
	if len(acks) != 10 {
		t.Fatalf("10 ack beklendi: %d", len(acks))
	}
	if success != 10 || fail != 0 {
		t.Fatalf("10 başarı beklendi: success=%d fail=%d", success, fail)
	}
}

func TestServer_ProcessSensorData(t *testing.T) {
	srv := newTestServer()
	defer srv.Stop()

	data := SensorData{
		SensorID:     "radar-01",
		SensorType:   "radar",
		SourceModule: "TANI",
		Payload:      []byte("radar_signal"),
		TimestampNs:  time.Now().UnixNano(),
		Priority:     1,
	}

	if err := srv.ProcessSensorData(data); err != nil {
		t.Fatalf("sensör verisi işleme hatası: %v", err)
	}

	// Consume ile kontrol
	msg, err := srv.Consume("sensor.radar", "test-group", "test-consumer")
	if err != nil {
		t.Fatalf("consume hatası: %v", err)
	}
	if string(msg.Payload) != "radar_signal" {
		t.Fatalf("payload uyuşmuyor: %s", string(msg.Payload))
	}
}

func TestServer_ListTopics(t *testing.T) {
	srv := newTestServer()
	defer srv.Stop()

	srv.CreateTopic("t1", 1)
	srv.CreateTopic("t2", 2)

	topics := srv.ListTopics()
	if len(topics) != 2 {
		t.Fatalf("2 topic beklendi: %d", len(topics))
	}
}

func TestServer_HealthCheck(t *testing.T) {
	srv := newTestServer()
	defer srv.Stop()

	healthy, status, latency := srv.HealthCheck()
	if !healthy {
		t.Fatalf("sağlıklı beklendi, durum: %s", status)
	}
	if latency <= 0 {
		t.Fatal("gecikme > 0 olmalı")
	}
}

func TestServer_GetStatus(t *testing.T) {
	srv := newTestServer()
	defer srv.Stop()

	status := srv.GetStatus()
	if status.Module != "BAĞ" {
		t.Fatalf("modül BAĞ beklendi: %s", status.Module)
	}
	if status.UptimeMs < 0 {
		t.Fatal("uptime >= 0 olmalı")
	}
}

func TestServer_SendToNonexistentTopic(t *testing.T) {
	srv := newTestServer()
	defer srv.Stop()

	env := Envelope{ID: "fail1", Topic: "nonexistent", Payload: []byte("x")}
	ack := srv.Send(context.Background(), env)
	if ack.Success {
		t.Fatal("olmayan topic'e gönderim başarısız olmalıydı")
	}
}

func TestServer_DeleteTopic(t *testing.T) {
	srv := newTestServer()
	defer srv.Stop()

	srv.CreateTopic("del_topic", 1)
	if err := srv.DeleteTopic("del_topic"); err != nil {
		t.Fatalf("topic silme hatası: %v", err)
	}

	topics := srv.ListTopics()
	if len(topics) != 0 {
		t.Fatalf("0 topic beklendi: %d", len(topics))
	}
}
