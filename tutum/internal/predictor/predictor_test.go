package predictor

import (
	"math"
	"testing"
)

func TestLinearRegression_Perfect(t *testing.T) {
	// y = -2x + 100 (mükemmel doğrusal boşalma)
	pts := []DataPoint{
		{0, 100}, {1, 98}, {2, 96}, {3, 94}, {4, 92},
	}
	slope, intercept, r2 := LinearRegression(pts)
	if math.Abs(slope-(-2.0)) > 0.01 {
		t.Fatalf("slope = %f, want -2", slope)
	}
	if math.Abs(intercept-100.0) > 0.01 {
		t.Fatalf("intercept = %f, want 100", intercept)
	}
	if math.Abs(r2-1.0) > 0.01 {
		t.Fatalf("R² = %f, want 1.0", r2)
	}
}

func TestLinearRegression_Noisy(t *testing.T) {
	pts := []DataPoint{
		{0, 100}, {1, 97}, {2, 95}, {3, 94}, {4, 90},
		{5, 88}, {6, 87}, {7, 84}, {8, 81}, {9, 79},
	}
	slope, _, r2 := LinearRegression(pts)
	if slope >= 0 {
		t.Fatal("slope should be negative (draining)")
	}
	if r2 < 0.9 {
		t.Fatalf("R² = %f, should be > 0.9 for near-linear data", r2)
	}
}

func TestLinearRegression_TooFewPoints(t *testing.T) {
	pts := []DataPoint{{0, 100}}
	slope, _, _ := LinearRegression(pts)
	if slope != 0 {
		t.Fatal("should return 0 for < 2 points")
	}
}

func TestEngine_AddSample(t *testing.T) {
	e := NewEngine(3, 50, 15.0)
	e.AddSample("iha-01", 100)
	e.AddSample("iha-01", 95)
	if e.SampleCount("iha-01") != 2 {
		t.Fatal("sample count should be 2")
	}
}

func TestEngine_Predict_InsufficientData(t *testing.T) {
	e := NewEngine(5, 50, 15.0)
	e.AddSample("iha-01", 100)
	e.AddSample("iha-01", 95)
	_, err := e.Predict("iha-01")
	if err == nil {
		t.Fatal("should error with insufficient data")
	}
}

func TestEngine_Predict_Draining(t *testing.T) {
	e := NewEngine(5, 100, 15.0)
	// Simüle: %100'den %80'e düşüş (20 adım, ~1%/adım)
	for i := 0; i < 20; i++ {
		e.AddSample("iha-01", 100-float64(i))
	}
	pred, err := e.Predict("iha-01")
	if err != nil {
		t.Fatal(err)
	}
	if pred.Slope >= 0 {
		t.Fatal("slope should be negative (draining)")
	}
	if pred.PredictedMinutes <= 0 {
		t.Fatalf("predicted = %f, should be > 0", pred.PredictedMinutes)
	}
	if pred.Confidence < 0.9 {
		t.Fatalf("R² = %f, should be high for linear drain", pred.Confidence)
	}
}

func TestEngine_Predict_Stable(t *testing.T) {
	e := NewEngine(5, 100, 15.0)
	for i := 0; i < 10; i++ {
		e.AddSample("radyo-01", 80) // Sabit SoC
	}
	pred, err := e.Predict("radyo-01")
	if err != nil {
		t.Fatal(err)
	}
	// Eğim ≈ 0, predicted = -1 (boşalma yok)
	if pred.PredictedMinutes > 0 {
		t.Logf("predicted=%f, slope=%f (should be ~0)", pred.PredictedMinutes, pred.Slope)
	}
}

func TestEngine_CheckMissionFeasibility(t *testing.T) {
	e := NewEngine(5, 100, 15.0)
	// %100'den %80'e düşen platform
	for i := 0; i < 20; i++ {
		e.AddSample("iha-02", 100-float64(i))
	}
	// 30 dakikalık görev
	result, err := e.CheckMissionFeasibility("iha-02", 30)
	if err != nil {
		t.Fatal(err)
	}
	if result.PlatformID != "iha-02" {
		t.Fatal("wrong platform")
	}
	// 80 dakika kalan + %15 RTB = ~12 dk yedek → 30+12=42 < 80 → yapılabilir
	t.Logf("Feasible: %v, Remaining: %.1f%%, Message: %s",
		result.Feasible, result.RemainingAfter, result.Message)
}

func TestEngine_WindowSize(t *testing.T) {
	e := NewEngine(3, 10, 15.0)
	for i := 0; i < 20; i++ {
		e.AddSample("w", float64(100-i))
	}
	if e.SampleCount("w") != 10 {
		t.Fatalf("sample count = %d, want 10 (window)", e.SampleCount("w"))
	}
}

func TestEngine_Reset(t *testing.T) {
	e := NewEngine(3, 50, 15.0)
	e.AddSample("x", 100)
	e.AddSample("x", 90)
	e.AddSample("x", 80)
	e.Predict("x")
	e.Reset("x")
	if e.SampleCount("x") != 0 {
		t.Fatal("should be 0 after reset")
	}
	_, ok := e.GetLastPrediction("x")
	if ok {
		t.Fatal("should not have prediction after reset")
	}
}

func TestEngine_AllPredictions(t *testing.T) {
	e := NewEngine(3, 50, 15.0)
	for i := 0; i < 5; i++ {
		e.AddSample("a", float64(100-i*2))
		e.AddSample("b", float64(80-i))
	}
	e.Predict("a")
	e.Predict("b")
	preds := e.AllPredictions()
	if len(preds) != 2 {
		t.Fatalf("predictions count = %d, want 2", len(preds))
	}
}
