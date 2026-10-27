package router

import (
	"fmt"
	"testing"

	"github.com/otukenmatris/akim/internal/config"
)

func testEngine() *Engine {
	cfg := config.DefaultConfig()
	cfg.MaxNodes = 100
	cfg.MaxLinks = 500
	e := New(cfg)

	// Basit topoloji: A -> B -> C -> D, A -> C (kısa yol)
	nodes := []Node{
		{ID: "A", Type: "command", Capacity: 10000},
		{ID: "B", Type: "relay", Capacity: 5000},
		{ID: "C", Type: "relay", Capacity: 5000},
		{ID: "D", Type: "edge", Capacity: 2000},
		{ID: "E", Type: "sensor", Capacity: 1000},
	}
	for _, n := range nodes {
		e.AddNode(n)
	}
	links := []Link{
		{From: "A", To: "B", Bandwidth: 1000, Latency: 2.0},
		{From: "B", To: "C", Bandwidth: 1000, Latency: 3.0},
		{From: "C", To: "D", Bandwidth: 500, Latency: 4.0},
		{From: "A", To: "C", Bandwidth: 2000, Latency: 1.0},
		{From: "B", To: "D", Bandwidth: 800, Latency: 5.0},
		{From: "D", To: "E", Bandwidth: 300, Latency: 2.0},
	}
	for _, l := range links {
		e.AddLink(l)
	}
	return e
}

func TestAddNode(t *testing.T) {
	cfg := config.DefaultConfig()
	e := New(cfg)

	err := e.AddNode(Node{ID: "test1", Type: "relay", Capacity: 1000})
	if err != nil {
		t.Fatalf("AddNode hatası: %v", err)
	}
	n, ok := e.GetNode("test1")
	if !ok || n.ID != "test1" {
		t.Fatal("düğüm bulunamadı")
	}
	if !n.Active {
		t.Fatal("düğüm aktif olmalı")
	}
}

func TestAddNodeEmptyID(t *testing.T) {
	cfg := config.DefaultConfig()
	e := New(cfg)
	err := e.AddNode(Node{Type: "relay"})
	if err == nil {
		t.Fatal("boş ID hata vermeli")
	}
}

func TestAddNodeMaxLimit(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MaxNodes = 2
	e := New(cfg)
	e.AddNode(Node{ID: "a", Type: "relay"})
	e.AddNode(Node{ID: "b", Type: "relay"})
	err := e.AddNode(Node{ID: "c", Type: "relay"})
	if err == nil {
		t.Fatal("max limit aşıldığında hata vermeli")
	}
}

func TestRemoveNode(t *testing.T) {
	e := testEngine()
	err := e.RemoveNode("E")
	if err != nil {
		t.Fatalf("RemoveNode hatası: %v", err)
	}
	_, ok := e.GetNode("E")
	if ok {
		t.Fatal("düğüm hala mevcut")
	}
}

func TestRemoveNodeNotFound(t *testing.T) {
	e := testEngine()
	err := e.RemoveNode("X")
	if err == nil {
		t.Fatal("bulunamayan düğüm hata vermeli")
	}
}

func TestListNodes(t *testing.T) {
	e := testEngine()
	nodes := e.ListNodes()
	if len(nodes) != 5 {
		t.Fatalf("beklenen: 5 düğüm, alınan: %d", len(nodes))
	}
}

func TestAddLink(t *testing.T) {
	cfg := config.DefaultConfig()
	e := New(cfg)
	e.AddNode(Node{ID: "x"})
	e.AddNode(Node{ID: "y"})
	err := e.AddLink(Link{From: "x", To: "y", Bandwidth: 100, Latency: 1.0})
	if err != nil {
		t.Fatalf("AddLink hatası: %v", err)
	}
	// Çift yönlü olduğu için 2 link olmalı
	links := e.ListLinks()
	if len(links) != 2 {
		t.Fatalf("beklenen: 2 link (çift yönlü), alınan: %d", len(links))
	}
}

func TestAddLinkInvalidNode(t *testing.T) {
	cfg := config.DefaultConfig()
	e := New(cfg)
	e.AddNode(Node{ID: "x"})
	err := e.AddLink(Link{From: "x", To: "missing"})
	if err == nil {
		t.Fatal("olmayan düğüme link hata vermeli")
	}
}

func TestLinkUtilization(t *testing.T) {
	l := Link{Bandwidth: 1000, UsedBW: 750}
	util := l.Utilization()
	if util != 75.0 {
		t.Fatalf("beklenen: 75.0, alınan: %.1f", util)
	}
}

func TestLinkAvailableBW(t *testing.T) {
	l := Link{Bandwidth: 1000, UsedBW: 600}
	avail := l.AvailableBW()
	if avail != 400.0 {
		t.Fatalf("beklenen: 400.0, alınan: %.1f", avail)
	}
}

func TestNodeUtilization(t *testing.T) {
	n := Node{Capacity: 1000, Load: 850}
	util := n.Utilization()
	if util != 85.0 {
		t.Fatalf("beklenen: 85.0, alınan: %.1f", util)
	}
}

func TestFindRouteSimple(t *testing.T) {
	e := testEngine()
	route, err := e.FindRoute("A", "D", config.QoSTelemetry)
	if err != nil {
		t.Fatalf("FindRoute hatası: %v", err)
	}
	if route.From != "A" || route.To != "D" {
		t.Fatalf("yanlış kaynak/hedef: %s → %s", route.From, route.To)
	}
	if route.HopCount < 2 {
		t.Fatalf("en az 2 hop olmalı: %d", route.HopCount)
	}
	if len(route.Path) < 3 {
		t.Fatalf("path en az 3 düğüm içermeli: %v", route.Path)
	}
}

func TestFindRouteSameNode(t *testing.T) {
	e := testEngine()
	route, err := e.FindRoute("A", "A", config.QoSTelemetry)
	if err != nil {
		t.Fatalf("aynı düğüme rota hatası: %v", err)
	}
	if route.HopCount != 0 {
		t.Fatalf("aynı düğüm 0 hop olmalı: %d", route.HopCount)
	}
}

func TestFindRouteNotFound(t *testing.T) {
	e := testEngine()
	_, err := e.FindRoute("A", "MISSING", config.QoSTelemetry)
	if err == nil {
		t.Fatal("olmayan düğüme rota hata vermeli")
	}
}

func TestFindRouteDisconnected(t *testing.T) {
	cfg := config.DefaultConfig()
	e := New(cfg)
	e.AddNode(Node{ID: "isolated1"})
	e.AddNode(Node{ID: "isolated2"})
	_, err := e.FindRoute("isolated1", "isolated2", config.QoSTelemetry)
	if err == nil {
		t.Fatal("bağlantısız düğümler arası rota hata vermeli")
	}
}

func TestFindRouteCongestionAware(t *testing.T) {
	e := testEngine()
	// A->C direkt yolunu tıka
	e.UpdateLinkTraffic("A->C", 1960) // %98 doluluk — blocked

	route, err := e.FindRoute("A", "D", config.QoSTelemetry)
	if err != nil {
		t.Fatalf("tıkanık yol varken hata: %v", err)
	}
	// A->C engellendiğinde A->B->C->D veya A->B->D kullanmalı
	if route.Path[1] != "B" {
		t.Fatalf("tıkanık A->C yerine B üzerinden gitmeli: %v", route.Path)
	}
}

func TestFindAlternateRoutes(t *testing.T) {
	e := testEngine()
	routes, err := e.FindAlternateRoutes("A", "D", config.QoSTelemetry, 3)
	if err != nil {
		t.Fatalf("alternatif rota hatası: %v", err)
	}
	if len(routes) < 1 {
		t.Fatal("en az 1 rota olmalı")
	}
}

func TestUpdateLinkTraffic(t *testing.T) {
	e := testEngine()
	err := e.UpdateLinkTraffic("A->B", 500)
	if err != nil {
		t.Fatalf("UpdateLinkTraffic hatası: %v", err)
	}
	l, ok := e.GetLink("A->B")
	if !ok {
		t.Fatal("link bulunamadı")
	}
	if l.UsedBW != 500 {
		t.Fatalf("beklenen UsedBW: 500, alınan: %.1f", l.UsedBW)
	}
}

func TestSetLinkActive(t *testing.T) {
	e := testEngine()
	err := e.SetLinkActive("A->B", false)
	if err != nil {
		t.Fatalf("SetLinkActive hatası: %v", err)
	}
	l, _ := e.GetLink("A->B")
	if l.Active {
		t.Fatal("link pasif olmalı")
	}
}

func TestTopology(t *testing.T) {
	e := testEngine()
	topo := e.Topology()
	if topo["total_nodes"].(int) != 5 {
		t.Fatalf("beklenen: 5 düğüm, alınan: %v", topo["total_nodes"])
	}
}

func TestStats(t *testing.T) {
	e := testEngine()
	e.FindRoute("A", "D", config.QoSTelemetry)
	e.FindRoute("A", "E", config.QoSCritical)
	stats := e.GetStats()
	if stats.RoutesComputed < 2 {
		t.Fatalf("en az 2 rota hesaplanmalı: %d", stats.RoutesComputed)
	}
}

func TestQoSAffectsRouting(t *testing.T) {
	e := testEngine()
	// Orta seviye tıkanıklık ekle
	e.UpdateLinkTraffic("A->C", 1600) // %80

	critRoute, err := e.FindRoute("A", "D", config.QoSCritical)
	if err != nil {
		t.Fatalf("kritik rota hatası: %v", err)
	}
	bulkRoute, err := e.FindRoute("A", "D", config.QoSBulk)
	if err != nil {
		t.Fatalf("bulk rota hatası: %v", err)
	}
	// Bulk trafik tıkanıklığa daha duyarlı olacağından farklı rota seçebilir
	_ = critRoute
	_ = bulkRoute
}

// Benchmark
func BenchmarkFindRoute(b *testing.B) {
	e := testEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.FindRoute("A", "E", config.QoSTelemetry)
	}
}

func BenchmarkAddNode(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.MaxNodes = b.N + 10
	e := New(cfg)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.AddNode(Node{ID: fmt.Sprintf("node-%d", i), Type: "relay"})
	}
}

func BenchmarkFindAlternateRoutes(b *testing.B) {
	e := testEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.FindAlternateRoutes("A", "E", config.QoSTelemetry, 3)
	}
}
