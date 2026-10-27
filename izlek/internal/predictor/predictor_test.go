package predictor

import (
	"math"
	"testing"
	"time"

	"github.com/otukenmatris/izlek/internal/tracker"
)

func makeTrack(id string, lat, lon, velN, velE, accN, accE float64, nPoints int) *tracker.Track {
	now := time.Now()
	points := make([]tracker.Point, nPoints)
	for i := 0; i < nPoints; i++ {
		points[i] = tracker.Point{
			Lat:       lat - float64(nPoints-1-i)*0.001,
			Lon:       lon,
			Timestamp: now.Add(-time.Duration(nPoints-1-i) * time.Second),
		}
	}
	return &tracker.Track{
		ID:       id,
		EntityID: id,
		VelNorth: velN,
		VelEast:  velE,
		AccNorth: accN,
		AccEast:  accE,
		Points:   points,
		Updates:  nPoints,
		LastSeen: now,
	}
}

func TestPredict_StraightLine(t *testing.T) {
	e := NewEngine(0.1, 0.5, 60, 600, 5, 15, 3)

	// Kuzeye 10 m/s sabit hızla giden hedef
	track := makeTrack("t1", 39.0, 32.0, 10.0, 0.0, 0.0, 0.0, 5)
	pred := e.Predict(track, 30)

	if pred.EntityID != "t1" {
		t.Errorf("entityID = %s", pred.EntityID)
	}
	if len(pred.Points) < 1 {
		t.Fatal("tahmin noktası yok")
	}

	// İlk nokta kuzeye kaymalı (lat artmalı)
	lastPt := pred.Points[len(pred.Points)-1]
	if lastPt.Lat <= 39.0 {
		t.Errorf("kuzeye gitmeli, lat = %f", lastPt.Lat)
	}
	// Lon sabit kalmalı (doğuya hız yok)
	if math.Abs(lastPt.Lon-32.0) > 0.001 {
		t.Errorf("lon sabit kalmalı, lon = %f", lastPt.Lon)
	}
}

func TestPredict_WithAcceleration(t *testing.T) {
	e := NewEngine(0.1, 0.5, 60, 600, 5, 15, 3)

	// Kuzeye 5 m/s + 2 m/s² ivme
	track := makeTrack("t2", 39.0, 32.0, 5.0, 0.0, 2.0, 0.0, 5)
	pred := e.Predict(track, 30)

	// İvmeli hareket düz hareketten daha fazla mesafe katetmeli
	trackNoAccel := makeTrack("t3", 39.0, 32.0, 5.0, 0.0, 0.0, 0.0, 5)
	predNoAccel := e.Predict(trackNoAccel, 30)

	lastAccel := pred.Points[len(pred.Points)-1]
	lastNoAccel := predNoAccel.Points[len(predNoAccel.Points)-1]

	if lastAccel.Lat <= lastNoAccel.Lat {
		t.Errorf("ivmeli hareket daha kuzeyde olmalı: accel=%f, noAccel=%f", lastAccel.Lat, lastNoAccel.Lat)
	}
}

func TestPredict_ManeuverLowersConfidence(t *testing.T) {
	e := NewEngine(0.1, 0.5, 60, 600, 5, 15, 3)

	track := makeTrack("t4", 39.0, 32.0, 10.0, 5.0, 0.0, 0.0, 5)
	track.Maneuver = false
	pred1 := e.Predict(track, 30)

	track.Maneuver = true
	pred2 := e.Predict(track, 30)

	last1 := pred1.Points[len(pred1.Points)-1]
	last2 := pred2.Points[len(pred2.Points)-1]

	if last2.Confidence >= last1.Confidence {
		t.Errorf("manevradayken güven düşmeli: normal=%f, manevra=%f", last1.Confidence, last2.Confidence)
	}
}

func TestIntercept_StationaryTarget(t *testing.T) {
	e := NewEngine(0.1, 0.5, 60, 600, 5, 15, 3)

	// Sabit hedef
	track := makeTrack("s1", 39.1, 32.0, 0.0, 0.0, 0.0, 0.0, 3)
	result := e.Intercept(track, 800.0, 39.0, 32.0) // ~11km mesafe, 800m/s mermi

	if !result.Feasible {
		t.Error("sabit hedef uygulanabilir olmalı")
	}
	if math.Abs(result.InterceptLat-39.1) > 0.001 {
		t.Errorf("sabit hedef için intercept = hedef konumu olmalı, lat=%f", result.InterceptLat)
	}
}

func TestIntercept_MovingTarget(t *testing.T) {
	e := NewEngine(0.1, 0.5, 60, 600, 5, 15, 3)

	// Kuzeye 20 m/s giden hedef
	track := makeTrack("m1", 39.1, 32.0, 20.0, 0.0, 0.0, 0.0, 5)
	result := e.Intercept(track, 800.0, 39.0, 32.0)

	if !result.Feasible {
		t.Error("hareketli hedef uygulanabilir olmalı")
	}
	// Önleme noktası hedefin mevcut konumundan kuzeyde olmalı
	if result.InterceptLat <= 39.1 {
		t.Errorf("önleme noktası kuzeyde olmalı, lat=%f", result.InterceptLat)
	}
	if result.TimeSec <= 0 {
		t.Errorf("süre pozitif olmalı, t=%f", result.TimeSec)
	}
}

func TestEngine_Stats(t *testing.T) {
	e := NewEngine(0.1, 0.5, 60, 600, 5, 15, 3)
	track := makeTrack("s1", 39.0, 32.0, 10.0, 0.0, 0.0, 0.0, 3)

	e.Predict(track, 10)
	e.Predict(track, 20)
	e.Intercept(track, 800, 39.1, 32.1)

	stats := e.Stats()
	if stats["total_predictions"] != 2 {
		t.Errorf("predictions = %v, beklenen 2", stats["total_predictions"])
	}
	if stats["total_intercepts"] != 1 {
		t.Errorf("intercepts = %v, beklenen 1", stats["total_intercepts"])
	}
}
