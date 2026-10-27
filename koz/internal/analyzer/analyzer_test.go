package analyzer

import (
	"testing"
	"time"

	"github.com/otukenmatris/koz/internal/thermal"
)

func TestClusterEmpty(t *testing.T) {
	a := NewAnalyzer(50.0, 0.5)
	cl := a.ClusterSignatures(nil)
	if len(cl) != 0 {
		t.Errorf("boş girişte 0 küme beklenirken %d döndü", len(cl))
	}
}

func TestClusterSingle(t *testing.T) {
	a := NewAnalyzer(50.0, 0.5)
	sigs := []thermal.Signature{
		{ID: "s1", EntityID: "e1", TempC: 100, Lat: 39.0, Lon: 32.0},
	}
	cl := a.ClusterSignatures(sigs)
	if len(cl) != 1 {
		t.Fatalf("1 küme beklenirken %d döndü", len(cl))
	}
	if cl[0].Count != 1 {
		t.Errorf("1 üye beklenirken %d döndü", cl[0].Count)
	}
}

func TestClusterNearby(t *testing.T) {
	a := NewAnalyzer(1000.0, 0.5) // 1km yarıçap
	sigs := []thermal.Signature{
		{ID: "s1", EntityID: "e1", TempC: 100, Lat: 39.0000, Lon: 32.0000},
		{ID: "s2", EntityID: "e2", TempC: 120, Lat: 39.0001, Lon: 32.0001}, // ~15m uzaklıkta
	}
	cl := a.ClusterSignatures(sigs)
	if len(cl) != 1 {
		t.Errorf("yakın imzalar 1 kümeye girmeli, %d küme döndü", len(cl))
	}
	if cl[0].Count != 2 {
		t.Errorf("2 üye beklenirken %d döndü", cl[0].Count)
	}
}

func TestClusterFarApart(t *testing.T) {
	a := NewAnalyzer(50.0, 0.5)
	sigs := []thermal.Signature{
		{ID: "s1", EntityID: "e1", TempC: 100, Lat: 39.0, Lon: 32.0},
		{ID: "s2", EntityID: "e2", TempC: 120, Lat: 40.0, Lon: 33.0}, // ~150km uzak
	}
	cl := a.ClusterSignatures(sigs)
	if len(cl) != 2 {
		t.Errorf("uzak imzalar 2 ayrı kümeye girmeli, %d küme döndü", len(cl))
	}
}

func TestClusterAssessment(t *testing.T) {
	a := NewAnalyzer(500.0, 0.5)
	sigs := make([]thermal.Signature, 6)
	for i := range sigs {
		sigs[i] = thermal.Signature{
			ID: "s" + string(rune('a'+i)), EntityID: "e" + string(rune('a'+i)),
			TempC: 90, Lat: 39.0, Lon: 32.0,
		}
	}
	cl := a.ClusterSignatures(sigs)
	if len(cl) == 0 {
		t.Fatal("küme oluşturulamadı")
	}
	if cl[0].Assessment != "convoy" {
		t.Errorf("5+ yakın imza 'convoy' olmalı, '%s' döndü", cl[0].Assessment)
	}
}

func TestPredictCooling(t *testing.T) {
	a := NewAnalyzer(50.0, 0.5)
	sigs := []thermal.Signature{
		{EntityID: "e1", TempC: 100, State: thermal.StateSteady, CoolingRateC: 0.1},
		{EntityID: "e2", TempC: 50, State: thermal.StateCooling, CoolingRateC: 0.5},
	}
	preds := a.PredictCooling(sigs, 22.0)
	if len(preds) != 2 {
		t.Fatalf("2 tahmin beklenirken %d döndü", len(preds))
	}
	// e1: 100 - 0.1*300 = 70°C
	if preds[0].PredictedTempC < 60 || preds[0].PredictedTempC > 80 {
		t.Errorf("e1 tahmini ~70°C olmalı, %.1f döndü", preds[0].PredictedTempC)
	}
	if preds[0].EngineState != "running" {
		t.Errorf("steady state motor 'running' olmalı, '%s' döndü", preds[0].EngineState)
	}
}

func TestDetectAnomaliesSuddenHeat(t *testing.T) {
	a := NewAnalyzer(50.0, 0.5)
	now := time.Now()
	sigs := []thermal.Signature{
		{
			EntityID: "e1", TempC: 200, DeltaC: 178, Type: thermal.SigEngine,
			History: []thermal.TempReading{
				{TempC: 50, Timestamp: now.Add(-time.Second)},
				{TempC: 200, Timestamp: now}, // 150°C/s artış
			},
		},
	}
	anoms := a.DetectAnomalies(sigs)
	found := false
	for _, an := range anoms {
		if an.Type == "sudden_heat" {
			found = true
		}
	}
	if !found {
		t.Error("ani ısınma anomalisi tespit edilmeliydi")
	}
}

func TestDetectAnomaliesMaskedSig(t *testing.T) {
	a := NewAnalyzer(50.0, 0.5)
	sigs := []thermal.Signature{
		{EntityID: "e1", TempC: 25, DeltaC: 3, Type: thermal.SigEngine},
	}
	anoms := a.DetectAnomalies(sigs)
	found := false
	for _, an := range anoms {
		if an.Type == "masked_sig" {
			found = true
		}
	}
	if !found {
		t.Error("kamuflaj şüphesi anomalisi tespit edilmeliydi")
	}
}

func TestGetAnomalies(t *testing.T) {
	a := NewAnalyzer(50.0, 0.5)
	sigs := []thermal.Signature{
		{EntityID: "e1", TempC: 25, DeltaC: 3, Type: thermal.SigEngine},
	}
	a.DetectAnomalies(sigs)
	anoms := a.GetAnomalies(10)
	if len(anoms) == 0 {
		t.Error("anomali listesi boş olmamalı")
	}
}

func TestAnalyzerStats(t *testing.T) {
	a := NewAnalyzer(50.0, 0.5)
	a.ClusterSignatures([]thermal.Signature{{ID: "s1", EntityID: "e1", TempC: 100, Lat: 39, Lon: 32}})
	stats := a.Stats()
	if stats["analysis_count"].(int64) != 1 {
		t.Errorf("analysis_count=1 beklenirken %v döndü", stats["analysis_count"])
	}
}
