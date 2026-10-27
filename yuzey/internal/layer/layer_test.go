package layer

import (
	"sync"
	"testing"

	"github.com/otukenmatris/yuzey/internal/config"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("Manager nil olmamalı")
	}
	layers := m.GetAllLayers()
	if len(layers) != 8 {
		t.Errorf("8 varsayılan katman bekleniyordu, %d geldi", len(layers))
	}
}

func TestGetLayer(t *testing.T) {
	m := NewManager()
	l, ok := m.GetLayer(LayerEntities)
	if !ok {
		t.Fatal("entities katmanı bulunamadı")
	}
	if l.Name != "Varlıklar" {
		t.Errorf("Name=Varlıklar bekleniyordu, %s geldi", l.Name)
	}
	if !l.Visible {
		t.Error("Varsayılan olarak visible=true olmalı")
	}
	if l.Opacity != 1.0 {
		t.Errorf("Opacity=1.0 bekleniyordu, %f geldi", l.Opacity)
	}
}

func TestGetLayerNotFound(t *testing.T) {
	m := NewManager()
	_, ok := m.GetLayer("nonexistent")
	if ok {
		t.Error("Olmayan katman bulunamazdı")
	}
}

func TestSetVisibility(t *testing.T) {
	m := NewManager()
	err := m.SetVisibility(LayerThreats, false)
	if err != nil {
		t.Fatalf("SetVisibility hatası: %v", err)
	}
	l, _ := m.GetLayer(LayerThreats)
	if l.Visible {
		t.Error("Visible false olmalıydı")
	}
	err = m.SetVisibility(LayerThreats, true)
	if err != nil {
		t.Fatalf("SetVisibility hatası: %v", err)
	}
	l, _ = m.GetLayer(LayerThreats)
	if !l.Visible {
		t.Error("Visible true olmalıydı")
	}
}

func TestSetVisibilityNotFound(t *testing.T) {
	m := NewManager()
	err := m.SetVisibility("yok", true)
	if err == nil {
		t.Error("Olmayan katman için hata bekleniyordu")
	}
}

func TestSetOpacity(t *testing.T) {
	m := NewManager()
	err := m.SetOpacity(LayerSensors, 0.5)
	if err != nil {
		t.Fatalf("SetOpacity hatası: %v", err)
	}
	l, _ := m.GetLayer(LayerSensors)
	if l.Opacity != 0.5 {
		t.Errorf("Opacity=0.5 bekleniyordu, %f geldi", l.Opacity)
	}
}

func TestSetOpacityNotFound(t *testing.T) {
	m := NewManager()
	err := m.SetOpacity("yok", 0.5)
	if err == nil {
		t.Error("Olmayan katman için hata bekleniyordu")
	}
}

func TestAddMarker(t *testing.T) {
	m := NewManager()
	mk := Marker{ID: "mk-1", Label: "Test", Lat: 39.9, Lon: 32.8, Icon: "◆", Color: "#f00"}
	err := m.AddMarker(LayerEntities, mk)
	if err != nil {
		t.Fatalf("AddMarker hatası: %v", err)
	}
	l, _ := m.GetLayer(LayerEntities)
	if len(l.Markers) != 1 {
		t.Errorf("1 marker bekleniyordu, %d geldi", len(l.Markers))
	}
	if l.Markers[0].ID != "mk-1" {
		t.Errorf("ID=mk-1 bekleniyordu, %s geldi", l.Markers[0].ID)
	}
}

func TestAddMarkerNotFound(t *testing.T) {
	m := NewManager()
	err := m.AddMarker("yok", Marker{})
	if err == nil {
		t.Error("Olmayan katman için hata bekleniyordu")
	}
}

func TestSetMarkers(t *testing.T) {
	m := NewManager()
	markers := []Marker{
		{ID: "m1", Lat: 39.9, Lon: 32.8},
		{ID: "m2", Lat: 40.0, Lon: 33.0},
		{ID: "m3", Lat: 39.8, Lon: 32.6},
	}
	err := m.SetMarkers(LayerSensors, markers)
	if err != nil {
		t.Fatalf("SetMarkers hatası: %v", err)
	}
	l, _ := m.GetLayer(LayerSensors)
	if len(l.Markers) != 3 {
		t.Errorf("3 marker bekleniyordu, %d geldi", len(l.Markers))
	}
}

func TestRemoveMarker(t *testing.T) {
	m := NewManager()
	m.AddMarker(LayerEntities, Marker{ID: "mk-1"})
	m.AddMarker(LayerEntities, Marker{ID: "mk-2"})
	m.AddMarker(LayerEntities, Marker{ID: "mk-3"})

	err := m.RemoveMarker(LayerEntities, "mk-2")
	if err != nil {
		t.Fatalf("RemoveMarker hatası: %v", err)
	}
	l, _ := m.GetLayer(LayerEntities)
	if len(l.Markers) != 2 {
		t.Errorf("2 marker bekleniyordu, %d geldi", len(l.Markers))
	}
}

func TestRemoveMarkerNotFound(t *testing.T) {
	m := NewManager()
	err := m.RemoveMarker(LayerEntities, "yok")
	if err == nil {
		t.Error("Olmayan marker için hata bekleniyordu")
	}
}

func TestRemoveMarkerBadLayer(t *testing.T) {
	m := NewManager()
	err := m.RemoveMarker("yok", "mk-1")
	if err == nil {
		t.Error("Olmayan katman için hata bekleniyordu")
	}
}

func TestAddLine(t *testing.T) {
	m := NewManager()
	line := Polyline{
		ID: "line-1", Color: "#0f0", Width: 2, Style: "solid",
		Points: [][]float64{{39.9, 32.8}, {40.0, 33.0}},
	}
	err := m.AddLine(LayerRoutes, line)
	if err != nil {
		t.Fatalf("AddLine hatası: %v", err)
	}
	l, _ := m.GetLayer(LayerRoutes)
	if len(l.Lines) != 1 {
		t.Errorf("1 çizgi bekleniyordu, %d geldi", len(l.Lines))
	}
}

func TestAddLineNotFound(t *testing.T) {
	m := NewManager()
	err := m.AddLine("yok", Polyline{})
	if err == nil {
		t.Error("Olmayan katman için hata bekleniyordu")
	}
}

func TestSetLines(t *testing.T) {
	m := NewManager()
	lines := []Polyline{
		{ID: "l1", Points: [][]float64{{39.9, 32.8}, {40.0, 33.0}}},
		{ID: "l2", Points: [][]float64{{39.8, 32.6}, {39.7, 32.4}}},
	}
	err := m.SetLines(LayerRoutes, lines)
	if err != nil {
		t.Fatalf("SetLines hatası: %v", err)
	}
	l, _ := m.GetLayer(LayerRoutes)
	if len(l.Lines) != 2 {
		t.Errorf("2 çizgi bekleniyordu, %d geldi", len(l.Lines))
	}
}

func TestSetLinesNotFound(t *testing.T) {
	m := NewManager()
	err := m.SetLines("yok", nil)
	if err == nil {
		t.Error("Olmayan katman için hata bekleniyordu")
	}
}

func TestAddZone(t *testing.T) {
	m := NewManager()
	zone := Zone{
		ID: "z1", Label: "Tehdit", Type: "circle", Radius: 1000,
		Color: "#f00", Points: [][]float64{{39.9, 32.8}},
	}
	err := m.AddZone(LayerThreats, zone)
	if err != nil {
		t.Fatalf("AddZone hatası: %v", err)
	}
	l, _ := m.GetLayer(LayerThreats)
	if len(l.Zones) != 1 {
		t.Errorf("1 bölge bekleniyordu, %d geldi", len(l.Zones))
	}
}

func TestAddZoneNotFound(t *testing.T) {
	m := NewManager()
	err := m.AddZone("yok", Zone{})
	if err == nil {
		t.Error("Olmayan katman için hata bekleniyordu")
	}
}

func TestSetZones(t *testing.T) {
	m := NewManager()
	zones := []Zone{
		{ID: "z1", Type: "circle", Radius: 500},
		{ID: "z2", Type: "polygon"},
	}
	err := m.SetZones(LayerPartitions, zones)
	if err != nil {
		t.Fatalf("SetZones hatası: %v", err)
	}
	l, _ := m.GetLayer(LayerPartitions)
	if len(l.Zones) != 2 {
		t.Errorf("2 bölge bekleniyordu, %d geldi", len(l.Zones))
	}
}

func TestSetZonesNotFound(t *testing.T) {
	m := NewManager()
	err := m.SetZones("yok", nil)
	if err == nil {
		t.Error("Olmayan katman için hata bekleniyordu")
	}
}

func TestRemoveZone(t *testing.T) {
	m := NewManager()
	m.AddZone(LayerThreats, Zone{ID: "z1"})
	m.AddZone(LayerThreats, Zone{ID: "z2"})
	err := m.RemoveZone(LayerThreats, "z1")
	if err != nil {
		t.Fatalf("RemoveZone hatası: %v", err)
	}
	l, _ := m.GetLayer(LayerThreats)
	if len(l.Zones) != 1 {
		t.Errorf("1 bölge bekleniyordu, %d geldi", len(l.Zones))
	}
}

func TestRemoveZoneNotFound(t *testing.T) {
	m := NewManager()
	err := m.RemoveZone(LayerThreats, "yok")
	if err == nil {
		t.Error("Olmayan bölge için hata bekleniyordu")
	}
}

func TestRemoveZoneBadLayer(t *testing.T) {
	m := NewManager()
	err := m.RemoveZone("yok", "z1")
	if err == nil {
		t.Error("Olmayan katman için hata bekleniyordu")
	}
}

func TestUpdateEntityLayer(t *testing.T) {
	m := NewManager()
	entities := []config.Entity{
		{ID: "drone-1", Type: "drone", Faction: "friendly", Lat: 39.9, Lon: 32.8, Status: "active", Health: 90},
		{ID: "tank-1", Type: "tank", Faction: "enemy", Lat: 40.0, Lon: 33.0, Status: "active", Health: 100},
		{ID: "infantry-1", Type: "infantry", Faction: "neutral", Lat: 39.8, Lon: 32.6, Status: "idle", Health: 80},
		{ID: "radar-1", Type: "radar", Faction: "friendly", Lat: 39.95, Lon: 32.85, Status: "active"},
		{ID: "ship-1", Type: "ship", Faction: "enemy", Lat: 40.1, Lon: 33.2, Status: "moving"},
	}
	m.UpdateEntityLayer(entities)

	l, ok := m.GetLayer(LayerEntities)
	if !ok {
		t.Fatal("entities katmanı bulunamadı")
	}
	if len(l.Markers) != 5 {
		t.Errorf("5 marker bekleniyordu, %d geldi", len(l.Markers))
	}

	// Marker renklerini kontrol et
	colors := map[string]string{}
	icons := map[string]string{}
	for _, mk := range l.Markers {
		colors[mk.ID] = mk.Color
		icons[mk.ID] = mk.Icon
	}
	if colors["drone-1"] != "#3b82f6" {
		t.Errorf("Friendly renk yanlış: %s", colors["drone-1"])
	}
	if colors["tank-1"] != "#ef4444" {
		t.Errorf("Enemy renk yanlış: %s", colors["tank-1"])
	}
	if colors["infantry-1"] != "#eab308" {
		t.Errorf("Neutral renk yanlış: %s", colors["infantry-1"])
	}
	// İkon testleri
	if icons["drone-1"] != "✈" {
		t.Errorf("Drone ikonu yanlış: %s", icons["drone-1"])
	}
	if icons["tank-1"] != "▣" {
		t.Errorf("Tank ikonu yanlış: %s", icons["tank-1"])
	}
	if icons["infantry-1"] != "☺" {
		t.Errorf("Infantry ikonu yanlış: %s", icons["infantry-1"])
	}
	if icons["radar-1"] != "◎" {
		t.Errorf("Radar ikonu yanlış: %s", icons["radar-1"])
	}
	if icons["ship-1"] != "⚓" {
		t.Errorf("Ship ikonu yanlış: %s", icons["ship-1"])
	}
}

func TestUpdateEntityLayerEmpty(t *testing.T) {
	m := NewManager()
	m.UpdateEntityLayer([]config.Entity{})
	l, _ := m.GetLayer(LayerEntities)
	if len(l.Markers) != 0 {
		t.Error("Boş varlık listesi boş marker demek")
	}
}

func TestUpdateEntityLayerOverwrite(t *testing.T) {
	m := NewManager()
	m.UpdateEntityLayer([]config.Entity{
		{ID: "a", Type: "tank", Faction: "enemy"},
		{ID: "b", Type: "tank", Faction: "enemy"},
	})
	m.UpdateEntityLayer([]config.Entity{
		{ID: "c", Type: "drone", Faction: "friendly"},
	})
	l, _ := m.GetLayer(LayerEntities)
	if len(l.Markers) != 1 {
		t.Errorf("SetMarkers sonrası 1 marker olmalı, %d geldi", len(l.Markers))
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()
	m.AddMarker(LayerEntities, Marker{ID: "m1"})
	m.AddMarker(LayerSensors, Marker{ID: "m2"})
	m.AddLine(LayerRoutes, Polyline{ID: "l1"})
	m.AddZone(LayerThreats, Zone{ID: "z1"})

	s := m.GetStats()
	if s.TotalMarkers != 2 {
		t.Errorf("TotalMarkers=2 bekleniyordu, %d geldi", s.TotalMarkers)
	}
	if s.TotalLines != 1 {
		t.Errorf("TotalLines=1 bekleniyordu, %d geldi", s.TotalLines)
	}
	if s.TotalZones != 1 {
		t.Errorf("TotalZones=1 bekleniyordu, %d geldi", s.TotalZones)
	}
	if s.LayerCount != 8 {
		t.Errorf("LayerCount=8 bekleniyordu, %d geldi", s.LayerCount)
	}
	if s.UpdateCount < 3 {
		t.Errorf("UpdateCount en az 3 olmalı, %d geldi", s.UpdateCount)
	}
}

func TestSummary(t *testing.T) {
	m := NewManager()
	s := m.Summary()
	if s["layers"] != 8 {
		t.Errorf("layers=8 bekleniyordu, %v geldi", s["layers"])
	}
}

func TestLayerCopy(t *testing.T) {
	m := NewManager()
	m.AddMarker(LayerEntities, Marker{ID: "orig"})
	l, _ := m.GetLayer(LayerEntities)
	// Kopyayı değiştir → orijinal etkilenmemeli
	l.Markers = append(l.Markers, Marker{ID: "extra"})
	orig, _ := m.GetLayer(LayerEntities)
	if len(orig.Markers) != 1 {
		t.Error("GetLayer kopyalama yapmalı, orijinal etkilenmemeli")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func(idx int) {
			defer wg.Done()
			m.AddMarker(LayerEntities, Marker{ID: "m-" + string(rune('a'+idx%26))})
		}(i)
		go func(idx int) {
			defer wg.Done()
			m.GetLayer(LayerEntities)
		}(i)
		go func(idx int) {
			defer wg.Done()
			m.GetAllLayers()
		}(i)
	}
	wg.Wait()
}

func TestAllDefaultLayerTypes(t *testing.T) {
	m := NewManager()
	types := []LayerType{
		LayerEntities, LayerSensors, LayerRoutes, LayerThreats,
		LayerPartitions, LayerCoverage, LayerTraffic, LayerAnnotations,
	}
	for _, lt := range types {
		l, ok := m.GetLayer(lt)
		if !ok {
			t.Errorf("%s katmanı bulunamadı", lt)
		}
		if l.Name == "" {
			t.Errorf("%s katmanının adı boş", lt)
		}
	}
}

func TestSetMarkersNotFound(t *testing.T) {
	m := NewManager()
	err := m.SetMarkers("yok", nil)
	if err == nil {
		t.Error("Olmayan katman için hata bekleniyordu")
	}
}

func TestMarkerMetadata(t *testing.T) {
	m := NewManager()
	mk := Marker{
		ID: "meta-1", Lat: 39.9, Lon: 32.8,
		Metadata: map[string]interface{}{"speed": 50, "ammo": "full"},
	}
	m.AddMarker(LayerEntities, mk)
	l, _ := m.GetLayer(LayerEntities)
	if l.Markers[0].Metadata["speed"] != 50 {
		t.Error("Metadata korunmalı")
	}
}

func TestZoneTypes(t *testing.T) {
	m := NewManager()
	m.AddZone(LayerThreats, Zone{ID: "circle", Type: "circle", Radius: 500})
	m.AddZone(LayerThreats, Zone{ID: "polygon", Type: "polygon"})
	l, _ := m.GetLayer(LayerThreats)
	if len(l.Zones) != 2 {
		t.Error("2 zone olmalı")
	}
	if l.Zones[0].Radius != 500 {
		t.Error("Circle zone radius=500 olmalı")
	}
}

func TestEntityToMarkerAllTypes(t *testing.T) {
	m := NewManager()
	entities := []config.Entity{
		{ID: "1", Type: "drone", Faction: "friendly"},
		{ID: "2", Type: "uav", Faction: "friendly"},
		{ID: "3", Type: "tank", Faction: "enemy"},
		{ID: "4", Type: "vehicle", Faction: "enemy"},
		{ID: "5", Type: "infantry", Faction: "neutral"},
		{ID: "6", Type: "radar", Faction: "friendly"},
		{ID: "7", Type: "sensor", Faction: "friendly"},
		{ID: "8", Type: "ship", Faction: "enemy"},
		{ID: "9", Type: "aircraft", Faction: "neutral"},
		{ID: "10", Type: "unknown", Faction: "unknown"},
	}
	m.UpdateEntityLayer(entities)
	l, _ := m.GetLayer(LayerEntities)
	if len(l.Markers) != 10 {
		t.Errorf("10 marker bekleniyordu, %d geldi", len(l.Markers))
	}
}
