package acoustic

import (
	"math"
	"testing"
	"time"
)

func newTestEngine() *Engine {
	return NewEngine(
		30.0,  // minAmplitude
		10000, // maxRange
		20.0,  // noiseFloor
		343.0, // speedOfSound
		3,     // minSensors
		2.0,   // toaTolerance
		0.1,   // confidenceMin
		600,   // maxAge
		500,   // maxDetections
	)
}

func TestNewEngine(t *testing.T) {
	e := newTestEngine()
	if e == nil {
		t.Fatal("Engine nil")
	}
	if len(e.detections) != 0 {
		t.Fatalf("expected 0 detections, got %d", len(e.detections))
	}
}

func TestDetectEmpty(t *testing.T) {
	e := newTestEngine()
	result := e.Detect(nil)
	if len(result.Detections) != 0 {
		t.Fatalf("expected 0 detections for nil input")
	}
	if result.TotalInputs != 0 {
		t.Fatalf("expected 0 total inputs")
	}
}

func TestDetectBelowThreshold(t *testing.T) {
	e := newTestEngine()
	readings := []SensorReading{
		{SensorID: "s1", AmplitudeDB: 10, FrequencyHz: 1000, Confidence: 0.5},
	}
	result := e.Detect(readings)
	if len(result.Detections) != 0 {
		t.Fatalf("expected 0 detections for below-threshold input")
	}
}

func TestDetectGunshot(t *testing.T) {
	e := newTestEngine()
	now := time.Now().UnixNano()
	readings := []SensorReading{
		{SensorID: "s1", Lat: 39.0, Lon: 35.0, FrequencyHz: 1500, AmplitudeDB: 145, DirectionDeg: 45, DurationMs: 40, SignatureType: "gunshot", Confidence: 0.9, RangeEstKm: 0.5, TimestampNs: now},
		{SensorID: "s2", Lat: 39.001, Lon: 35.001, FrequencyHz: 1600, AmplitudeDB: 140, DirectionDeg: 43, DurationMs: 38, SignatureType: "gunshot", Confidence: 0.85, RangeEstKm: 0.6, TimestampNs: now + 1000000},
		{SensorID: "s3", Lat: 39.002, Lon: 35.002, FrequencyHz: 1400, AmplitudeDB: 135, DirectionDeg: 47, DurationMs: 42, SignatureType: "gunshot", Confidence: 0.88, RangeEstKm: 0.55, TimestampNs: now + 2000000},
	}
	result := e.Detect(readings)
	if result.Classified < 1 {
		t.Fatalf("expected at least 1 classified, got %d", result.Classified)
	}
	if len(result.Detections) < 1 {
		t.Fatalf("expected at least 1 detection, got %d", len(result.Detections))
	}
	det := result.Detections[0]
	if det.EventType != EventGunshot {
		t.Fatalf("expected gunshot, got %s", det.EventType)
	}
	if det.Confidence <= 0 {
		t.Fatalf("expected positive confidence, got %f", det.Confidence)
	}
	if det.SensorCount < 3 {
		t.Fatalf("expected at least 3 sensors, got %d", det.SensorCount)
	}
}

func TestDetectExplosion(t *testing.T) {
	e := newTestEngine()
	readings := []SensorReading{
		{SensorID: "s1", Lat: 39.0, Lon: 35.0, FrequencyHz: 50, AmplitudeDB: 160, SignatureType: "explosion", Confidence: 0.95, RangeEstKm: 1.0, TimestampNs: time.Now().UnixNano()},
	}
	result := e.Detect(readings)
	if len(result.Detections) < 1 {
		t.Fatalf("expected detection for explosion")
	}
}

func TestClassifyEvent(t *testing.T) {
	e := newTestEngine()
	tests := []struct {
		name    string
		freq    float64
		amp     float64
		sigType string
		want    EventType
	}{
		{"gunshot_freq", 1500, 150, "gunshot", EventGunshot},
		{"explosion_freq", 50, 170, "explosion", EventArtillery}, // 10-150Hz + 150-195dB → artillery profili daha yakın
		{"engine_freq", 200, 80, "tracked_vehicle", EventVehicle}, // 60-400Hz + 55-100dB → vehicle/engine örtüşür
		{"jet_freq", 500, 120, "jet_engine", EventJet},
		{"rotor_freq", 100, 85, "helicopter", EventEngine}, // 20-500Hz rotor ve 50-400Hz engine örtüşür
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readings := []SensorReading{
				{FrequencyHz: tt.freq, AmplitudeDB: tt.amp, SignatureType: tt.sigType, Confidence: 0.8},
			}
			got := e.classifyEvent(readings)
			if got != tt.want {
				t.Errorf("classifyEvent(%s) = %s, want %s", tt.name, got, tt.want)
			}
		})
	}
}

func TestLocate(t *testing.T) {
	e := newTestEngine()
	now := time.Now().UnixNano()
	toas := []TOAMeasurement{
		{SensorID: "s1", SensorLat: 39.0, SensorLon: 35.0, ArrivalNs: now, AmplitudeDB: 120, DirectionDeg: 45},
		{SensorID: "s2", SensorLat: 39.001, SensorLon: 35.001, ArrivalNs: now + 3000000, AmplitudeDB: 115, DirectionDeg: 315},
		{SensorID: "s3", SensorLat: 39.002, SensorLon: 35.0, ArrivalNs: now + 5000000, AmplitudeDB: 110, DirectionDeg: 0},
	}
	result := e.Locate(toas)
	if result == nil {
		t.Fatal("expected non-nil locate result")
	}
	if result.SensorCount != 3 {
		t.Fatalf("expected 3 sensors, got %d", result.SensorCount)
	}
	if result.Confidence <= 0 {
		t.Fatal("expected positive confidence")
	}
	if result.RangeM <= 0 {
		t.Fatal("expected positive range")
	}
}

func TestLocateInsufficientSensors(t *testing.T) {
	e := newTestEngine()
	toas := []TOAMeasurement{
		{SensorID: "s1", SensorLat: 39.0, SensorLon: 35.0, ArrivalNs: 1000, AmplitudeDB: 100, DirectionDeg: 0},
	}
	result := e.Locate(toas)
	if result != nil {
		t.Fatal("expected nil for insufficient sensors")
	}
}

func TestThreatAssessment(t *testing.T) {
	e := newTestEngine()
	tests := []struct {
		event EventType
		amp   float64
		conf  float64
		want  ThreatLevel
	}{
		{EventGunshot, 160, 0.95, ThreatCritical},
		{EventExplosion, 140, 0.7, ThreatCritical}, // 0.7 * 1.3 * 1.2 (>140dB) = 1.092 → critical
		{EventEngine, 80, 0.3, ThreatLow},
		{EventFootstep, 40, 0.2, ThreatLow},
	}
	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			got := e.assessThreat(tt.event, tt.amp, tt.conf)
			if got != tt.want {
				t.Errorf("assessThreat(%s, %.0f, %.1f) = %s, want %s", tt.event, tt.amp, tt.conf, got, tt.want)
			}
		})
	}
}

func TestGetDetection(t *testing.T) {
	e := newTestEngine()
	e.detections["test-1"] = &Detection{
		ID:        "test-1",
		EventType: EventGunshot,
		Lat:       39.0,
		Lon:       35.0,
	}
	det, ok := e.GetDetection("test-1")
	if !ok || det == nil {
		t.Fatal("expected detection")
	}
	if det.EventType != EventGunshot {
		t.Fatalf("expected gunshot, got %s", det.EventType)
	}
	_, ok = e.GetDetection("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestGetAllDetections(t *testing.T) {
	e := newTestEngine()
	e.detections["test-1"] = &Detection{ID: "test-1", EventType: EventGunshot}
	e.detections["test-2"] = &Detection{ID: "test-2", EventType: EventExplosion}
	all := e.GetAllDetections()
	if len(all) != 2 {
		t.Fatalf("expected 2, got %d", len(all))
	}
}

func TestGetByType(t *testing.T) {
	e := newTestEngine()
	e.detections["test-1"] = &Detection{ID: "test-1", EventType: EventGunshot}
	e.detections["test-2"] = &Detection{ID: "test-2", EventType: EventExplosion}
	e.detections["test-3"] = &Detection{ID: "test-3", EventType: EventGunshot}
	guns := e.GetByType(EventGunshot)
	if len(guns) != 2 {
		t.Fatalf("expected 2 gunshots, got %d", len(guns))
	}
}

func TestGetHighThreat(t *testing.T) {
	e := newTestEngine()
	e.detections["test-1"] = &Detection{ID: "test-1", ThreatLevel: ThreatHigh}
	e.detections["test-2"] = &Detection{ID: "test-2", ThreatLevel: ThreatLow}
	e.detections["test-3"] = &Detection{ID: "test-3", ThreatLevel: ThreatCritical}
	high := e.GetHighThreat()
	if len(high) != 2 {
		t.Fatalf("expected 2 high threats, got %d", len(high))
	}
}

func TestPrune(t *testing.T) {
	e := newTestEngine()
	e.maxAge = 1 // 1 saniye
	e.detections["old"] = &Detection{ID: "old", LastUpdated: time.Now().Add(-2 * time.Second)}
	e.detections["new"] = &Detection{ID: "new", LastUpdated: time.Now()}
	removed := e.Prune()
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	if len(e.detections) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(e.detections))
	}
}

func TestStats(t *testing.T) {
	e := newTestEngine()
	stats := e.Stats()
	if stats["active_detections"] != 0 {
		t.Fatalf("expected 0 active, got %v", stats["active_detections"])
	}
	if stats["speed_of_sound"] != 343.0 {
		t.Fatalf("expected 343.0, got %v", stats["speed_of_sound"])
	}
}

func TestHaversine(t *testing.T) {
	// İstanbul–Ankara yaklaşık 351 km
	d := haversineM(41.0082, 28.9784, 39.9334, 32.8597)
	if d < 300000 || d > 400000 {
		t.Fatalf("haversine distance unexpected: %.0f m", d)
	}
}

func TestBearing(t *testing.T) {
	b := bearingDeg(39.0, 35.0, 39.0, 36.0)
	if math.Abs(b-89.6) > 1.5 {
		t.Fatalf("bearing unexpected: %.1f", b)
	}
}

func TestFormatRange(t *testing.T) {
	r1 := formatRange(500)
	if r1 != "500 m" {
		t.Fatalf("expected '500 m', got '%s'", r1)
	}
	r2 := formatRange(2500)
	if r2 != "2.5 km" {
		t.Fatalf("expected '2.5 km', got '%s'", r2)
	}
}

func TestMapSigType(t *testing.T) {
	tests := []struct {
		sig  string
		want EventType
	}{
		{"tracked_vehicle", EventEngine},
		{"wheeled_vehicle", EventVehicle},
		{"helicopter", EventRotor},
		{"jet_engine", EventJet},
		{"explosion", EventExplosion},
		{"footsteps", EventFootstep},
		{"gunshot", EventGunshot},
		{"something_else", EventUnknown},
	}
	for _, tt := range tests {
		got := mapSigTypeToEvent(tt.sig)
		if got != tt.want {
			t.Errorf("mapSigTypeToEvent(%s) = %s, want %s", tt.sig, got, tt.want)
		}
	}
}
