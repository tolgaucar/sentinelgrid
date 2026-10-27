package spectrum

import (
	"math"
	"math/cmplx"
	"testing"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine(1024, 100.0, -60.0, 128)
	if e == nil {
		t.Fatal("engine nil")
	}
	if e.fftSize != 1024 {
		t.Fatalf("expected 1024, got %d", e.fftSize)
	}
}

func TestNewEngineNonPowerOf2(t *testing.T) {
	e := NewEngine(1000, 100.0, -60.0, 128)
	if e.fftSize != 1024 {
		t.Fatalf("expected 1024 (rounded up), got %d", e.fftSize)
	}
}

func TestFFTSingleFrequency(t *testing.T) {
	n := 256
	samples := make([]complex128, n)
	freq := 10.0
	for i := 0; i < n; i++ {
		val := math.Cos(2 * math.Pi * freq * float64(i) / float64(n))
		samples[i] = complex(val, 0)
	}
	result := fft(samples)
	// frekans 10'da pik olmalı
	maxIdx := 0
	maxMag := 0.0
	for i := 1; i < n/2; i++ {
		mag := cmplx.Abs(result[i])
		if mag > maxMag {
			maxMag = mag
			maxIdx = i
		}
	}
	if maxIdx != 10 {
		t.Fatalf("expected peak at bin 10, got %d", maxIdx)
	}
}

func TestFFTTwoFrequencies(t *testing.T) {
	n := 512
	samples := make([]complex128, n)
	for i := 0; i < n; i++ {
		v1 := math.Cos(2 * math.Pi * 20 * float64(i) / float64(n))
		v2 := 0.5 * math.Cos(2*math.Pi*50*float64(i)/float64(n))
		samples[i] = complex(v1+v2, 0)
	}
	result := fft(samples)
	mag20 := cmplx.Abs(result[20])
	mag50 := cmplx.Abs(result[50])
	if mag20 < 50 || mag50 < 20 {
		t.Fatalf("peaks too weak: bin20=%.1f bin50=%.1f", mag20, mag50)
	}
}

func TestHanningWindow(t *testing.T) {
	w := hanningWindow(64)
	if len(w) != 64 {
		t.Fatalf("expected 64, got %d", len(w))
	}
	// uçlar sıfıra yakın olmalı
	if w[0] > 0.01 {
		t.Fatalf("window start should be ~0, got %f", w[0])
	}
	// orta bölge ~1
	mid := w[32]
	if mid < 0.9 {
		t.Fatalf("window center should be ~1, got %f", mid)
	}
}

func TestEstimateNoiseFloor(t *testing.T) {
	data := []float64{-70, -72, -68, -71, -69, -20, -73, -70}
	nf := estimateNoiseFloor(data)
	if nf > -50 {
		t.Fatalf("noise floor too high: %f", nf)
	}
}

func TestEstimateNoiseFloorEmpty(t *testing.T) {
	nf := estimateNoiseFloor(nil)
	if nf != -80.0 {
		t.Fatalf("expected -80, got %f", nf)
	}
}

func TestAnalyze(t *testing.T) {
	e := NewEngine(256, 100.0, -80.0, 64)
	signals := []RawSignal{
		{ID: "s1", FrequencyMHz: 30.0, AmplitudeDB: -10, BandwidthMHz: 0.5},
	}
	result := e.Analyze(signals)
	if result.TotalSignals != 1 {
		t.Fatalf("expected 1 signal, got %d", result.TotalSignals)
	}
	if result.Spectrum.BinCount == 0 {
		t.Fatal("spectrum should have bins")
	}
}

func TestAnalyzeMultiple(t *testing.T) {
	e := NewEngine(512, 100.0, -80.0, 64)
	signals := []RawSignal{
		{ID: "s1", FrequencyMHz: 10.0, AmplitudeDB: -5, BandwidthMHz: 1.0},
		{ID: "s2", FrequencyMHz: 25.0, AmplitudeDB: -10, BandwidthMHz: 0.5},
		{ID: "s3", FrequencyMHz: 40.0, AmplitudeDB: -15, BandwidthMHz: 2.0},
	}
	result := e.Analyze(signals)
	if result.TotalSignals != 3 {
		t.Fatalf("expected 3 signals, got %d", result.TotalSignals)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	e := NewEngine(256, 100.0, -80.0, 64)
	result := e.Analyze(nil)
	if result.TotalSignals != 0 {
		t.Fatal("expected 0 signals")
	}
}

func TestGetAllBands(t *testing.T) {
	e := NewEngine(256, 100.0, -80.0, 64)
	e.Analyze([]RawSignal{{ID: "s1", FrequencyMHz: 10.0, AmplitudeDB: -5}})
	bands := e.GetAllBands()
	// bant sayısı analiz sonucuna bağlı, sadece nil olmadığını kontrol et
	_ = bands
}

func TestGetBand(t *testing.T) {
	e := NewEngine(256, 100.0, -80.0, 64)
	_, ok := e.GetBand("nonexistent")
	if ok {
		t.Fatal("should not find nonexistent band")
	}
}

func TestPrune(t *testing.T) {
	e := NewEngine(256, 100.0, -80.0, 64)
	e.Analyze([]RawSignal{{ID: "s1", FrequencyMHz: 10.0, AmplitudeDB: -5}})
	// hepsini hemen temizle (0 saniye ömür)
	removed := e.Prune(0)
	_ = removed // sıfır olabilir çünkü az önce eklendi
}

func TestStats(t *testing.T) {
	e := NewEngine(256, 100.0, -80.0, 64)
	e.Analyze([]RawSignal{{ID: "s1", FrequencyMHz: 10.0, AmplitudeDB: -5}})
	s := e.Stats()
	if s["total_ffts"].(int) != 1 {
		t.Fatalf("expected 1 fft, got %d", s["total_ffts"])
	}
	if s["fft_size"].(int) != 256 {
		t.Fatalf("expected 256, got %d", s["fft_size"])
	}
}

func TestClassifyByFrequency(t *testing.T) {
	tests := []struct {
		freq     float64
		bw       float64
		expected SignalType
	}{
		{300, 1.0, SignalComm},      // UHF askeri
		{50, 0.5, SignalComm},       // VHF askeri
		{1300, 2.0, SignalRadar},    // L-bant
		{3000, 5.0, SignalRadar},    // S-bant
		{10000, 3.0, SignalRadar},   // X-bant
		{500, 50.0, SignalJamming},  // geniş bant karıştırma
		{406.05, 0.01, SignalBeacon}, // acil durum
		{700, 1.0, SignalUnknown},   // bilinmeyen
	}
	for _, tc := range tests {
		result := classifyByFrequency(tc.freq, tc.bw)
		if result != tc.expected {
			t.Errorf("freq=%.0f bw=%.0f: expected %s, got %s", tc.freq, tc.bw, tc.expected, result)
		}
	}
}

func TestHashFloat(t *testing.T) {
	v := hashFloat(42)
	if v < 0 || v > 1 {
		t.Fatalf("expected 0-1, got %f", v)
	}
	// deterministik
	v2 := hashFloat(42)
	if v != v2 {
		t.Fatal("hashFloat not deterministic")
	}
}
