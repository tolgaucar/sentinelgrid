package broker

import (
	"fmt"
	"testing"

	"github.com/otukenmatris/bag/internal/queue"
)

func TestBroker_CreateAndPublish(t *testing.T) {
	brk := NewBroker(BrokerOpts{DefaultPartitions: 2, MaxQueueSize: 1000, MaxTopics: 10})
	defer brk.Close()

	// Topic oluştur
	topic, err := brk.CreateTopic("sensor.radar", 2)
	if err != nil {
		t.Fatalf("topic oluşturma hatası: %v", err)
	}
	if topic.Name != "sensor.radar" {
		t.Fatalf("topic adı yanlış: %s", topic.Name)
	}

	// Mesaj yayınla
	msg := queue.Message{ID: "r1", Topic: "sensor.radar", Payload: []byte("radar_data"), Priority: 1}
	if err := brk.Publish("sensor.radar", msg); err != nil {
		t.Fatalf("publish hatası: %v", err)
	}

	// İstatistik kontrolü
	in, _, _, tc := brk.GetStats()
	if in != 1 {
		t.Fatalf("1 mesaj beklendi, alındı: %d", in)
	}
	if tc != 1 {
		t.Fatalf("1 topic beklendi, alındı: %d", tc)
	}
}

func TestBroker_Consume(t *testing.T) {
	brk := NewBroker(BrokerOpts{DefaultPartitions: 1, MaxQueueSize: 100})
	defer brk.Close()

	brk.CreateTopic("test", 1)

	msg := queue.Message{ID: "c1", Topic: "test", Payload: []byte("data")}
	brk.Publish("test", msg)

	got, err := brk.Consume("test", "group1", "consumer1")
	if err != nil {
		t.Fatalf("consume hatası: %v", err)
	}
	if got.ID != "c1" {
		t.Fatalf("ID 'c1' beklendi, alındı: %s", got.ID)
	}
}

func TestBroker_DuplicateTopic(t *testing.T) {
	brk := NewBroker(BrokerOpts{DefaultPartitions: 1})
	defer brk.Close()

	brk.CreateTopic("dup", 1)
	_, err := brk.CreateTopic("dup", 1)
	if err == nil {
		t.Fatal("duplicate topic hatası beklendi")
	}
}

func TestBroker_DeleteTopic(t *testing.T) {
	brk := NewBroker(BrokerOpts{DefaultPartitions: 1})
	defer brk.Close()

	brk.CreateTopic("del", 1)
	if err := brk.DeleteTopic("del"); err != nil {
		t.Fatalf("silme hatası: %v", err)
	}

	if err := brk.DeleteTopic("del"); err == nil {
		t.Fatal("silinmiş topic için hata beklendi")
	}
}

func TestBroker_PublishToNonexistent(t *testing.T) {
	brk := NewBroker(BrokerOpts{DefaultPartitions: 1})
	defer brk.Close()

	msg := queue.Message{ID: "x", Topic: "nonexistent"}
	if err := brk.Publish("nonexistent", msg); err == nil {
		t.Fatal("olmayan topic'e publish hatası beklendi")
	}
}

func TestBroker_MultipleConsumerGroups(t *testing.T) {
	brk := NewBroker(BrokerOpts{DefaultPartitions: 1, MaxQueueSize: 100})
	defer brk.Close()

	brk.CreateTopic("multi", 1)

	// 10 mesaj yayınla
	for i := 0; i < 10; i++ {
		brk.Publish("multi", queue.Message{ID: fmt.Sprintf("m%d", i), Topic: "multi"})
	}

	// Tek consumer group ile tüm mesajları oku
	for i := 0; i < 10; i++ {
		_, err := brk.Consume("multi", "g1", "c1")
		if err != nil {
			t.Fatalf("consume hatası (mesaj %d): %v", i, err)
		}
	}

	// Kuyruk boş olmalı
	_, err := brk.Consume("multi", "g1", "c1")
	if err == nil {
		t.Fatal("boş kuyruktan consume hatası beklendi")
	}
}

func TestBroker_TopicDepths(t *testing.T) {
	brk := NewBroker(BrokerOpts{DefaultPartitions: 1, MaxQueueSize: 100})
	defer brk.Close()

	brk.CreateTopic("depth", 1)
	for i := 0; i < 5; i++ {
		brk.Publish("depth", queue.Message{ID: fmt.Sprintf("d%d", i), Topic: "depth"})
	}

	depths := brk.TopicDepths()
	if depths["depth"] != 5 {
		t.Fatalf("depth 5 beklendi, alındı: %d", depths["depth"])
	}
}
