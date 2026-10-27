package tracker

import (
	"testing"
	"time"

	"github.com/otukenmatris/tani/internal/classifier"
	"github.com/otukenmatris/tani/internal/detector"
)

func det(id string, cls detector.ObjectClass, x, y, w, h, conf float64) detector.Detection {
	return detector.Detection{
		ID:         id,
		Class:      cls,
		Confidence: conf,
		BBox:       detector.BoundingBox{X: x, Y: y, Width: w, Height: h},
		Timestamp:  time.Now(),
		FrameID:    "f1",
	}
}

func cls(detID string, alleg classifier.Allegiance) classifier.ClassifiedObject {
	return classifier.ClassifiedObject{
		DetectionID: detID,
		Allegiance:  alleg,
		ThreatLevel: classifier.ThreatMedium,
		ThreatScore: 0.5,
	}
}

func TestVelocitySpeed(t *testing.T) {
	v := Velocity{VX: 3, VY: 4}
	if s := v.Speed(); s != 5 {
		t.Errorf("Speed() = %f, want 5", s)
	}
}

func TestVelocityHeading(t *testing.T) {
	v := Velocity{VX: 1, VY: 0}
	h := v.Heading()
	if h != 0 {
		t.Errorf("Heading(1,0) = %f, want 0", h)
	}
}

func TestNewTrack(t *testing.T) {
	d := det("d1", detector.ClassTank, 10, 20, 50, 50, 0.9)
	c := cls("d1", classifier.AllegianceHostile)
	trk := NewTrack("TRK-001", d, c, 100)

	if trk.ID != "TRK-001" {
		t.Errorf("ID = %s, want TRK-001", trk.ID)
	}
	if trk.Status != StatusTentative {
		t.Errorf("Status = %s, want tentative", trk.Status)
	}
	if trk.Position.X != 35 || trk.Position.Y != 45 {
		t.Errorf("Position = (%f,%f), want (35,45)", trk.Position.X, trk.Position.Y)
	}
}

func TestTrackUpdate(t *testing.T) {
	d1 := det("d1", detector.ClassTank, 10, 20, 50, 50, 0.9)
	c1 := cls("d1", classifier.AllegianceHostile)
	trk := NewTrack("TRK-001", d1, c1, 100)

	d2 := det("d2", detector.ClassTank, 20, 30, 50, 50, 0.85)
	c2 := cls("d2", classifier.AllegianceHostile)
	trk.Update(d2, c2)

	if trk.UpdateCount != 2 {
		t.Errorf("UpdateCount = %d, want 2", trk.UpdateCount)
	}
	if trk.Velocity.VX != 10 || trk.Velocity.VY != 10 {
		t.Errorf("Velocity = (%f,%f), want (10,10)", trk.Velocity.VX, trk.Velocity.VY)
	}
}

func TestTrackConfirmedAfter3Updates(t *testing.T) {
	d := det("d1", detector.ClassTank, 10, 20, 50, 50, 0.9)
	c := cls("d1", classifier.AllegianceHostile)
	trk := NewTrack("TRK-001", d, c, 100)

	if trk.Status != StatusTentative {
		t.Fatalf("İlk status tentative olmalı")
	}

	for i := 0; i < 3; i++ {
		d2 := det("d"+string(rune('2'+i)), detector.ClassTank, float64(15+i*5), 25, 50, 50, 0.85)
		c2 := cls(d2.ID, classifier.AllegianceHostile)
		trk.Update(d2, c2)
	}

	if trk.Status != StatusConfirmed {
		t.Errorf("3 güncellemeden sonra Status = %s, want confirmed", trk.Status)
	}
}

func TestTrackMarkLost(t *testing.T) {
	d := det("d1", detector.ClassTank, 100, 200, 50, 50, 0.9)
	c := cls("d1", classifier.AllegianceHostile)
	trk := NewTrack("TRK-001", d, c, 100)

	// Hız ata
	trk.Velocity = Velocity{VX: 5, VY: -3}
	origX := trk.Position.X
	origY := trk.Position.Y

	trk.MarkLost()

	if trk.Status != StatusLost {
		t.Errorf("Status = %s, want lost", trk.Status)
	}
	if trk.Position.X != origX+5 || trk.Position.Y != origY-3 {
		t.Errorf("Kayıp konumda tahmin yapılmalı")
	}
}

func TestSnapshot(t *testing.T) {
	d := det("d1", detector.ClassDrone, 10, 20, 30, 30, 0.8)
	c := cls("d1", classifier.AllegianceUnknown)
	trk := NewTrack("TRK-001", d, c, 100)

	snap := trk.Snapshot()
	if snap.ID != "TRK-001" {
		t.Errorf("Snapshot ID = %s", snap.ID)
	}
	if snap.Class != detector.ClassDrone {
		t.Errorf("Snapshot Class = %s", snap.Class)
	}
}

func TestEngineProcessFrame_NewTracks(t *testing.T) {
	eng := NewEngine(100, 5, 0.3, 50)

	dets := []detector.Detection{
		det("d1", detector.ClassTank, 10, 20, 50, 50, 0.9),
		det("d2", detector.ClassSoldier, 200, 300, 20, 20, 0.7),
	}
	clsf := []classifier.ClassifiedObject{
		cls("d1", classifier.AllegianceHostile),
		cls("d2", classifier.AllegianceUnknown),
	}

	tracks := eng.ProcessFrame(dets, clsf)
	if len(tracks) != 2 {
		t.Errorf("ProcessFrame new = %d tracks, want 2", len(tracks))
	}
}

func TestEngineProcessFrame_UpdateExisting(t *testing.T) {
	eng := NewEngine(100, 5, 0.3, 50)

	// Kare 1
	dets1 := []detector.Detection{
		det("d1", detector.ClassTank, 10, 20, 50, 50, 0.9),
	}
	clsf1 := []classifier.ClassifiedObject{
		cls("d1", classifier.AllegianceHostile),
	}
	eng.ProcessFrame(dets1, clsf1)

	// Kare 2 — aynı nesne biraz kaymış (IoU > threshold)
	dets2 := []detector.Detection{
		det("d2", detector.ClassTank, 15, 25, 50, 50, 0.85),
	}
	clsf2 := []classifier.ClassifiedObject{
		cls("d2", classifier.AllegianceHostile),
	}
	tracks := eng.ProcessFrame(dets2, clsf2)

	// Hala 1 iz olmalı (güncellendi, yeni oluşturulmadı)
	if len(tracks) != 1 {
		t.Errorf("Update: %d tracks, want 1", len(tracks))
	}

	stats := eng.GetStats()
	if stats.TotalCreated != 1 {
		t.Errorf("TotalCreated = %d, want 1", stats.TotalCreated)
	}
	if stats.TotalUpdated != 1 {
		t.Errorf("TotalUpdated = %d, want 1", stats.TotalUpdated)
	}
}

func TestEngineProcessFrame_LostAndDeleted(t *testing.T) {
	eng := NewEngine(100, 2, 0.3, 50) // max 2 kayıp kare

	// Kare 1: bir iz oluştur
	dets1 := []detector.Detection{
		det("d1", detector.ClassTank, 10, 20, 50, 50, 0.9),
	}
	clsf1 := []classifier.ClassifiedObject{
		cls("d1", classifier.AllegianceHostile),
	}
	eng.ProcessFrame(dets1, clsf1)

	// Kare 2-4: nesne yok → kayıp artar
	empty := []detector.Detection{}
	emptyC := []classifier.ClassifiedObject{}
	eng.ProcessFrame(empty, emptyC) // lost=1
	eng.ProcessFrame(empty, emptyC) // lost=2
	tracks := eng.ProcessFrame(empty, emptyC) // lost=3 > max=2 → deleted

	// Silinen izler listeden çıkar
	for _, tr := range tracks {
		if tr.Status == StatusDeleted {
			t.Error("Deleted tracks aktif listede olmamalı")
		}
	}

	stats := eng.GetStats()
	if stats.TotalDeleted != 1 {
		t.Errorf("TotalDeleted = %d, want 1", stats.TotalDeleted)
	}
}

func TestEngineGetTrack(t *testing.T) {
	eng := NewEngine(100, 5, 0.3, 50)

	dets := []detector.Detection{det("d1", detector.ClassTank, 10, 20, 50, 50, 0.9)}
	clsf := []classifier.ClassifiedObject{cls("d1", classifier.AllegianceHostile)}
	tracks := eng.ProcessFrame(dets, clsf)

	if len(tracks) == 0 {
		t.Fatal("Track oluşmadı")
	}

	snap, ok := eng.GetTrack(tracks[0].ID)
	if !ok {
		t.Fatal("GetTrack bulunamadı")
	}
	if snap.Class != detector.ClassTank {
		t.Errorf("GetTrack class = %s", snap.Class)
	}
}

func TestEngineGetTrack_NotFound(t *testing.T) {
	eng := NewEngine(100, 5, 0.3, 50)
	_, ok := eng.GetTrack("nonexistent")
	if ok {
		t.Error("Olmayan iz bulundu")
	}
}

func TestEngineHostileTracks(t *testing.T) {
	eng := NewEngine(100, 5, 0.3, 50)

	dets := []detector.Detection{
		det("d1", detector.ClassTank, 10, 20, 50, 50, 0.9),
		det("d2", detector.ClassCivilian, 200, 300, 20, 20, 0.7),
	}
	clsf := []classifier.ClassifiedObject{
		cls("d1", classifier.AllegianceHostile),
		cls("d2", classifier.AllegianceNeutral),
	}
	eng.ProcessFrame(dets, clsf)

	hostiles := eng.HostileTracks()
	if len(hostiles) != 1 {
		t.Errorf("HostileTracks = %d, want 1", len(hostiles))
	}
}

func TestEnginePrune(t *testing.T) {
	eng := NewEngine(100, 0, 0.3, 50) // maxLost=0, hemen sil

	dets := []detector.Detection{det("d1", detector.ClassTank, 10, 20, 50, 50, 0.9)}
	clsf := []classifier.ClassifiedObject{cls("d1", classifier.AllegianceHostile)}
	eng.ProcessFrame(dets, clsf)

	// Boş kare → hemen deleted
	eng.ProcessFrame(nil, nil)

	pruned := eng.Prune()
	if pruned != 1 {
		t.Errorf("Prune = %d, want 1", pruned)
	}
}

func TestEngineMaxTracks(t *testing.T) {
	eng := NewEngine(2, 5, 0.01, 50) // Max 2 iz

	dets := []detector.Detection{
		det("d1", detector.ClassTank, 10, 20, 50, 50, 0.9),
		det("d2", detector.ClassSoldier, 200, 300, 20, 20, 0.7),
		det("d3", detector.ClassDrone, 400, 400, 25, 25, 0.8),
	}
	clsf := []classifier.ClassifiedObject{
		cls("d1", classifier.AllegianceHostile),
		cls("d2", classifier.AllegianceUnknown),
		cls("d3", classifier.AllegianceUnknown),
	}
	tracks := eng.ProcessFrame(dets, clsf)
	if len(tracks) > 2 {
		t.Errorf("MaxTracks aşıldı: %d", len(tracks))
	}
}

func TestEngineStats(t *testing.T) {
	eng := NewEngine(100, 5, 0.3, 50)
	stats := eng.GetStats()
	if stats.ActiveTracks != 0 || stats.TotalCreated != 0 {
		t.Error("Başlangıç istatistikleri sıfır olmalı")
	}

	dets := []detector.Detection{det("d1", detector.ClassTank, 10, 20, 50, 50, 0.9)}
	clsf := []classifier.ClassifiedObject{cls("d1", classifier.AllegianceHostile)}
	eng.ProcessFrame(dets, clsf)

	stats = eng.GetStats()
	if stats.ActiveTracks != 1 {
		t.Errorf("ActiveTracks = %d, want 1", stats.ActiveTracks)
	}
	if stats.HostileCount != 1 {
		t.Errorf("HostileCount = %d, want 1", stats.HostileCount)
	}
}
