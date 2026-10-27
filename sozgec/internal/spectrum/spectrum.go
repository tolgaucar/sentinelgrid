// Package spectrum — Spektral analiz motoru.
//
// FFT (Hızlı Fourier Dönüşümü) ve dalgacık dönüşümü ile karmaşık sinyal
// ortamında frekans ayrıştırma, güç spektral yoğunluğu hesaplama ve
// pik tespiti yapar.
package spectrum

import (
	"fmt"
	"math"
	"math/cmplx"
	"sync"
	"time"
)

// SignalType — sinyal türü.
type SignalType string

const (
	SignalComm      SignalType = "communication"   // haberleşme
	SignalRadar     SignalType = "radar"            // radar
	SignalJamming   SignalType = "jamming"          // karıştırma
	SignalBeacon    SignalType = "beacon"           // işaret
	SignalTelemetry SignalType = "telemetry"        // telemetri
	SignalUnknown   SignalType = "unknown"          // bilinmeyen
)

// RawSignal — ham sinyal girişi (sensör/anten verisinden).
type RawSignal struct {
	ID           string  `json:"id"`
	FrequencyMHz float64 `json:"frequency_mhz"` // merkez frekansı
	BandwidthMHz float64 `json:"bandwidth_mhz"` // bant genişliği
	AmplitudeDB  float64 `json:"amplitude_db"`   // genlik
	PhaseRad     float64 `json:"phase_rad"`      // faz
	DirectionDeg float64 `json:"direction_deg"`  // geliş yönü
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	Timestamp    int64   `json:"timestamp"`
}

// FrequencyBand — ayrıştırılmış frekans bandı.
type FrequencyBand struct {
	ID           string    `json:"id"`
	CenterMHz    float64   `json:"center_mhz"`
	LowMHz       float64   `json:"low_mhz"`
	HighMHz      float64   `json:"high_mhz"`
	PeakPowerDB  float64   `json:"peak_power_db"`
	MeanPowerDB  float64   `json:"mean_power_db"`
	BandwidthMHz float64   `json:"bandwidth_mhz"`
	SignalType   SignalType `json:"signal_type"`
	Confidence   float64   `json:"confidence"`
	SNR_DB       float64   `json:"snr_db"`
	DetectedAt   time.Time `json:"detected_at"`
}

// PowerSpectrum — güç spektral yoğunluğu sonucu.
type PowerSpectrum struct {
	Frequencies []float64 `json:"frequencies"` // frekans ekseni (MHz)
	PowerDB     []float64 `json:"power_db"`    // güç (dB)
	NoiseFloor  float64   `json:"noise_floor_db"`
	PeakCount   int       `json:"peak_count"`
	BinCount    int       `json:"bin_count"`
}

// SpectralResult — tam spektral analiz sonucu.
type SpectralResult struct {
	Bands       []FrequencyBand `json:"bands"`
	Spectrum    PowerSpectrum   `json:"spectrum"`
	TotalSignals int            `json:"total_signals"`
	ElapsedMs   int64           `json:"elapsed_ms"`
}

// Engine — spektral analiz motoru.
type Engine struct {
	mu        sync.RWMutex
	fftSize   int
	sampleRate float64
	peakMinDB float64
	maxBands  int
	window    []float64 // pencere fonksiyonu

	bands      map[string]*FrequencyBand
	totalFFTs  int
	totalPeaks int
	totalBands int
}

// NewEngine — yeni spektral analiz motoru.
func NewEngine(fftSize int, sampleRate, peakMinDB float64, maxBands int) *Engine {
	// 2'nin kuvvetine yuvarla
	n := 1
	for n < fftSize {
		n <<= 1
	}

	e := &Engine{
		fftSize:    n,
		sampleRate: sampleRate,
		peakMinDB:  peakMinDB,
		maxBands:   maxBands,
		bands:      make(map[string]*FrequencyBand),
	}
	e.window = hanningWindow(n)
	return e
}

// Analyze — ham sinyallerden spektral analiz yap.
func (e *Engine) Analyze(signals []RawSignal) SpectralResult {
	start := time.Now()

	// 1. sinyallerden sentetik zaman serisi oluştur
	samples := e.synthesize(signals)

	// 2. pencere fonksiyonu uygula
	windowed := e.applyWindow(samples)

	// 3. FFT hesapla
	fftResult := fft(windowed)

	// 4. güç spektrumu hesapla
	psd := e.computePSD(fftResult)

	// 5. gürültü tabanı tespit et
	noiseFloor := estimateNoiseFloor(psd.PowerDB)

	// 6. pikleri bul
	peaks := e.findPeaks(psd, noiseFloor)

	// 7. frekans bantlarına dönüştür
	bands := e.peaksToBands(peaks, psd, noiseFloor, signals)

	// kaydet
	e.mu.Lock()
	for i := range bands {
		if len(e.bands) < e.maxBands {
			e.bands[bands[i].ID] = &bands[i]
		}
	}
	e.totalFFTs++
	e.totalPeaks += len(peaks)
	e.totalBands += len(bands)
	e.mu.Unlock()

	psd.NoiseFloor = noiseFloor
	psd.PeakCount = len(peaks)

	return SpectralResult{
		Bands:        bands,
		Spectrum:     psd,
		TotalSignals: len(signals),
		ElapsedMs:    time.Since(start).Milliseconds(),
	}
}

// GetAllBands — tüm aktif frekans bantları.
func (e *Engine) GetAllBands() []FrequencyBand {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]FrequencyBand, 0, len(e.bands))
	for _, b := range e.bands {
		result = append(result, *b)
	}
	return result
}

// GetBand — belirli bant.
func (e *Engine) GetBand(id string) (*FrequencyBand, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	b, ok := e.bands[id]
	return b, ok
}

// Prune — eski bantları temizle.
func (e *Engine) Prune(maxAgeS int) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	cutoff := time.Now().Add(-time.Duration(maxAgeS) * time.Second)
	removed := 0
	for id, b := range e.bands {
		if b.DetectedAt.Before(cutoff) {
			delete(e.bands, id)
			removed++
		}
	}
	return removed
}

// Stats — istatistikler.
func (e *Engine) Stats() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()

	byType := make(map[string]int)
	for _, b := range e.bands {
		byType[string(b.SignalType)]++
	}

	return map[string]any{
		"active_bands": len(e.bands),
		"total_ffts":   e.totalFFTs,
		"total_peaks":  e.totalPeaks,
		"total_bands":  e.totalBands,
		"by_type":      byType,
		"fft_size":     e.fftSize,
	}
}

// ── Dahili İşlemler ──

// synthesize — ham sinyallerden zaman alanı örnekleri üret (simülasyon).
func (e *Engine) synthesize(signals []RawSignal) []complex128 {
	n := e.fftSize
	samples := make([]complex128, n)

	for _, sig := range signals {
		freq := sig.FrequencyMHz
		amp := math.Pow(10, sig.AmplitudeDB/20.0) // dB → lineer
		phase := sig.PhaseRad

		for i := 0; i < n; i++ {
			t := float64(i) / (e.sampleRate * 1e6) // saniye
			val := amp * math.Cos(2*math.Pi*freq*1e6*t+phase)
			samples[i] += complex(val, 0)
		}
	}

	// gürültü ekle
	for i := range samples {
		noise := (hashFloat(i*7+13) - 0.5) * 0.001
		samples[i] += complex(noise, 0)
	}

	return samples
}

// applyWindow — pencere fonksiyonu uygula.
func (e *Engine) applyWindow(samples []complex128) []complex128 {
	n := len(samples)
	if n > len(e.window) {
		n = len(e.window)
	}
	result := make([]complex128, n)
	for i := 0; i < n; i++ {
		result[i] = complex(real(samples[i])*e.window[i], imag(samples[i])*e.window[i])
	}
	return result
}

// computePSD — güç spektral yoğunluğu hesapla.
func (e *Engine) computePSD(fftResult []complex128) PowerSpectrum {
	n := len(fftResult)
	halfN := n / 2

	freqs := make([]float64, halfN)
	power := make([]float64, halfN)

	freqRes := e.sampleRate / float64(n) // MHz/bin

	for i := 0; i < halfN; i++ {
		freqs[i] = float64(i) * freqRes
		mag := cmplx.Abs(fftResult[i])
		if mag < 1e-15 {
			mag = 1e-15
		}
		power[i] = 20 * math.Log10(mag/float64(n))
	}

	return PowerSpectrum{
		Frequencies: freqs,
		PowerDB:     power,
		BinCount:    halfN,
	}
}

// findPeaks — güç spektrumundaki pikleri tespit et.
func (e *Engine) findPeaks(psd PowerSpectrum, noiseFloor float64) []int {
	var peaks []int
	n := len(psd.PowerDB)

	for i := 1; i < n-1; i++ {
		if psd.PowerDB[i] > psd.PowerDB[i-1] &&
			psd.PowerDB[i] > psd.PowerDB[i+1] &&
			psd.PowerDB[i] > noiseFloor+6.0 && // gürültü tabanından 6dB üstü
			psd.PowerDB[i] > e.peakMinDB {
			peaks = append(peaks, i)
		}
	}
	return peaks
}

// peaksToBands — pikleri frekans bantlarına dönüştür.
func (e *Engine) peaksToBands(peaks []int, psd PowerSpectrum, noiseFloor float64, signals []RawSignal) []FrequencyBand {
	freqRes := e.sampleRate / float64(e.fftSize)
	var bands []FrequencyBand

	for _, pi := range peaks {
		centerMHz := psd.Frequencies[pi]
		peakPower := psd.PowerDB[pi]

		// -3dB bant genişliği bul
		lowIdx, highIdx := pi, pi
		threshold := peakPower - 3.0
		for lowIdx > 0 && psd.PowerDB[lowIdx] > threshold {
			lowIdx--
		}
		for highIdx < len(psd.PowerDB)-1 && psd.PowerDB[highIdx] > threshold {
			highIdx++
		}

		lowMHz := float64(lowIdx) * freqRes
		highMHz := float64(highIdx) * freqRes
		bwMHz := highMHz - lowMHz
		if bwMHz < freqRes {
			bwMHz = freqRes
		}

		// ortalama güç
		var sumPow float64
		count := 0
		for i := lowIdx; i <= highIdx; i++ {
			sumPow += psd.PowerDB[i]
			count++
		}
		meanPow := sumPow / float64(count)

		snr := peakPower - noiseFloor
		conf := math.Min(snr/30.0, 1.0)
		if conf < 0 {
			conf = 0
		}

		sigType := classifySignal(centerMHz, bwMHz, signals)

		band := FrequencyBand{
			ID:           fmt.Sprintf("band-%d-%.2f", pi, centerMHz),
			CenterMHz:    centerMHz,
			LowMHz:       lowMHz,
			HighMHz:      highMHz,
			PeakPowerDB:  peakPower,
			MeanPowerDB:  meanPow,
			BandwidthMHz: bwMHz,
			SignalType:   sigType,
			Confidence:   conf,
			SNR_DB:       snr,
			DetectedAt:   time.Now(),
		}
		bands = append(bands, band)
	}
	return bands
}

// classifySignal — sinyal türünü belirle.
func classifySignal(centerMHz, bwMHz float64, signals []RawSignal) SignalType {
	// eşleşen ham sinyal bul
	for _, sig := range signals {
		if math.Abs(sig.FrequencyMHz-centerMHz) < bwMHz*2 {
			return classifyByFrequency(sig.FrequencyMHz, sig.BandwidthMHz)
		}
	}
	return classifyByFrequency(centerMHz, bwMHz)
}

func classifyByFrequency(freqMHz, bwMHz float64) SignalType {
	switch {
	case freqMHz >= 225 && freqMHz <= 400:
		return SignalComm // UHF askeri haberleşme
	case freqMHz >= 30 && freqMHz <= 88:
		return SignalComm // VHF askeri
	case freqMHz >= 1200 && freqMHz <= 1400:
		return SignalRadar // L-bant radar
	case freqMHz >= 2700 && freqMHz <= 3500:
		return SignalRadar // S-bant radar
	case freqMHz >= 8000 && freqMHz <= 12000:
		return SignalRadar // X-bant radar
	case bwMHz > 20:
		return SignalJamming // geniş bantlı karıştırma
	case freqMHz >= 406 && freqMHz <= 406.1:
		return SignalBeacon // acil durum işareti
	case freqMHz >= 1544 && freqMHz <= 1545:
		return SignalTelemetry // uydu telemetri
	default:
		return SignalUnknown
	}
}

// ── FFT ──

// fft — Cooley-Tukey radix-2 FFT algoritması.
func fft(x []complex128) []complex128 {
	n := len(x)
	if n <= 1 {
		return x
	}

	// 2'nin kuvvetine pad
	m := 1
	for m < n {
		m <<= 1
	}
	if m != n {
		padded := make([]complex128, m)
		copy(padded, x)
		x = padded
		n = m
	}

	// bit-reversal permutation
	result := make([]complex128, n)
	copy(result, x)
	j := 0
	for i := 1; i < n; i++ {
		bit := n >> 1
		for j&bit != 0 {
			j ^= bit
			bit >>= 1
		}
		j ^= bit
		if i < j {
			result[i], result[j] = result[j], result[i]
		}
	}

	// butterfly operations
	for size := 2; size <= n; size <<= 1 {
		halfSize := size / 2
		wn := cmplx.Exp(complex(0, -2*math.Pi/float64(size)))
		for start := 0; start < n; start += size {
			w := complex(1, 0)
			for k := 0; k < halfSize; k++ {
				t := w * result[start+k+halfSize]
				u := result[start+k]
				result[start+k] = u + t
				result[start+k+halfSize] = u - t
				w *= wn
			}
		}
	}

	return result
}

// ── Pencere Fonksiyonları ──

func hanningWindow(n int) []float64 {
	w := make([]float64, n)
	for i := range w {
		w[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(n-1)))
	}
	return w
}

// estimateNoiseFloor — gürültü tabanını ortanca değer ile tahmin et.
func estimateNoiseFloor(powerDB []float64) float64 {
	if len(powerDB) == 0 {
		return -80.0
	}
	// sıralamadan ortanca tahmin — basit ortalama bazlı
	var sum float64
	count := 0
	for _, v := range powerDB {
		if !math.IsInf(v, 0) && !math.IsNaN(v) {
			sum += v
			count++
		}
	}
	if count == 0 {
		return -80.0
	}
	mean := sum / float64(count)
	// gürültü tabanı ortalamadan biraz düşük
	return mean - 3.0
}

// hashFloat — deterministik pseudo-random float (test tekrarlanabilirliği).
func hashFloat(seed int) float64 {
	h := uint64(seed)
	h = h*6364136223846793005 + 1442695040888963407
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	return float64(h%10000) / 10000.0
}
