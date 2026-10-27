package pipeline

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/otukenmatris/bagdas/internal/codec"
	"github.com/otukenmatris/bagdas/internal/schema"
)

func TestNewPipeline(t *testing.T) {
	p := NewPipeline("test")
	if p.Name() != "test" {
		t.Errorf("name = %q", p.Name())
	}
	if p.StepCount() != 0 {
		t.Errorf("step count = %d", p.StepCount())
	}
}

func TestPipeline_AddStep(t *testing.T) {
	p := NewPipeline("test")
	p.AddStep(NewTimestampStep("ts"))
	p.AddStep(NewEnrichStep(map[string]interface{}{"k": "v"}))
	if p.StepCount() != 2 {
		t.Errorf("step count = %d", p.StepCount())
	}
}

func TestPipeline_Execute_Empty(t *testing.T) {
	p := NewPipeline("empty")
	msg := NewMessage(map[string]interface{}{"id": "test"})
	result, err := p.Execute(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if result.StepsRun != 0 {
		t.Errorf("steps run = %d", result.StepsRun)
	}
	if result.Output.Data["id"] != "test" {
		t.Error("çıktı verisi korunmadı")
	}
}

func TestPipeline_Execute_TimestampStep(t *testing.T) {
	p := NewPipeline("ts-test").AddStep(NewTimestampStep("processed_at"))
	msg := NewMessage(map[string]interface{}{"id": "test"})

	result, err := p.Execute(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if result.StepsRun != 1 {
		t.Errorf("steps run = %d", result.StepsRun)
	}
	if _, ok := result.Output.Data["processed_at"]; !ok {
		t.Error("timestamp alanı eklenmedi")
	}
}

func TestPipeline_Execute_EnrichStep(t *testing.T) {
	p := NewPipeline("enrich").AddStep(NewEnrichStep(map[string]interface{}{
		"source":  "bagdas",
		"version": "1.0",
	}))
	msg := NewMessage(map[string]interface{}{"id": "test"})

	result, err := p.Execute(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output.Data["source"] != "bagdas" {
		t.Error("enrich alan eklenmedi")
	}
}

func TestEnrichStep_NoOverwrite(t *testing.T) {
	p := NewPipeline("no-overwrite").AddStep(NewEnrichStep(map[string]interface{}{
		"id": "overwritten",
	}))
	msg := NewMessage(map[string]interface{}{"id": "original"})

	result, err := p.Execute(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output.Data["id"] != "original" {
		t.Error("mevcut alan üzerine yazılmamalı")
	}
}

func TestFilterStep(t *testing.T) {
	p := NewPipeline("filter").AddStep(NewFilterStep([]string{"id", "name"}))
	msg := NewMessage(map[string]interface{}{
		"id":     "test",
		"name":   "sensor",
		"secret": "gizli",
	})

	result, err := p.Execute(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.Output.Data["secret"]; ok {
		t.Error("filtrelenmemiş alan var")
	}
	if result.Output.Data["id"] != "test" {
		t.Error("filtrelenen alan kayboldu")
	}
}

func TestRenameStep(t *testing.T) {
	p := NewPipeline("rename").AddStep(NewRenameStep(map[string]string{
		"old_name": "new_name",
	}))
	msg := NewMessage(map[string]interface{}{
		"old_name": "value",
		"keep":     "same",
	})

	result, err := p.Execute(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output.Data["new_name"] != "value" {
		t.Error("alan yeniden adlandırılmadı")
	}
	if _, ok := result.Output.Data["old_name"]; ok {
		t.Error("eski alan adı silinmedi")
	}
	if result.Output.Data["keep"] != "same" {
		t.Error("diğer alan değişmemeli")
	}
}

func TestCustomStep(t *testing.T) {
	p := NewPipeline("custom").AddStep(NewCustomStep("upper", func(ctx context.Context, msg *Message) (*Message, error) {
		if v, ok := msg.Data["name"].(string); ok {
			msg.Data["name_upper"] = v + "_UPPER"
		}
		return msg, nil
	}))
	msg := NewMessage(map[string]interface{}{"name": "test"})

	result, err := p.Execute(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output.Data["name_upper"] != "test_UPPER" {
		t.Error("custom step çalışmadı")
	}
}

func TestPipeline_Execute_ErrorPropagation(t *testing.T) {
	p := NewPipeline("error-test").
		AddStep(NewTimestampStep("ts")).
		AddStep(NewCustomStep("fail", func(ctx context.Context, msg *Message) (*Message, error) {
			return nil, fmt.Errorf("kasıtlı hata")
		})).
		AddStep(NewTimestampStep("ts2"))

	msg := NewMessage(map[string]interface{}{"id": "test"})
	result, err := p.Execute(context.Background(), msg)

	if err == nil {
		t.Fatal("hata beklendi")
	}
	if result.StepsRun != 1 {
		t.Errorf("steps run = %d, beklenen 1", result.StepsRun)
	}
	if result.Error == "" {
		t.Error("error mesajı boş")
	}
}

func TestPipeline_Execute_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Hemen iptal et

	p := NewPipeline("cancel-test").AddStep(NewTimestampStep("ts"))
	msg := NewMessage(map[string]interface{}{"id": "test"})

	_, err := p.Execute(ctx, msg)
	if err == nil {
		t.Error("iptal hatası beklendi")
	}
}

func TestPipeline_Execute_Timing(t *testing.T) {
	p := NewPipeline("timing").
		AddStep(NewTimestampStep("ts")).
		AddStep(NewEnrichStep(map[string]interface{}{"k": "v"}))

	msg := NewMessage(map[string]interface{}{"id": "test"})
	result, err := p.Execute(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.StepTimings) != 2 {
		t.Errorf("step timings = %d", len(result.StepTimings))
	}
	if result.Duration == 0 {
		t.Error("toplam süre 0")
	}
}

func TestValidateStep(t *testing.T) {
	schemaReg := schema.NewRegistry()
	schemaReg.Register("test_schema", []schema.Field{
		{Name: "id", Type: schema.TypeString, Required: true},
		{Name: "value", Type: schema.TypeFloat, Required: false},
	})

	p := NewPipeline("validate").
		AddStep(NewValidateStep("test_schema", 1, schemaReg))

	// Geçerli veri
	msg := NewMessage(map[string]interface{}{"id": "ok", "value": 42.5})
	result, err := p.Execute(context.Background(), msg)
	if err != nil {
		t.Fatalf("geçerli veri hata: %v", err)
	}
	if result.StepsRun != 1 {
		t.Error("adım çalışmadı")
	}

	// Geçersiz veri — zorunlu alan eksik
	msg2 := NewMessage(map[string]interface{}{"value": 42.5})
	_, err = p.Execute(context.Background(), msg2)
	if err == nil {
		t.Error("zorunlu alan eksik hatası beklendi")
	}
}

func TestConvertFormatStep(t *testing.T) {
	codecReg := codec.NewRegistry()
	p := NewPipeline("fmt-convert").
		AddStep(NewConvertFormatStep(codec.MsgPack, codecReg))

	msg := &Message{
		Data:     map[string]interface{}{"id": "test", "count": int64(5)},
		Metadata: make(map[string]interface{}),
		Format:   codec.JSON,
	}

	result, err := p.Execute(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output.Format != codec.MsgPack {
		t.Errorf("format = %s, beklenen msgpack", result.Output.Format)
	}
	if result.Output.Data["id"] != "test" {
		t.Error("veri kayboldu")
	}
}

func TestConvertFormatStep_SameFormat(t *testing.T) {
	codecReg := codec.NewRegistry()
	p := NewPipeline("same-fmt").
		AddStep(NewConvertFormatStep(codec.JSON, codecReg))

	msg := &Message{
		Data:     map[string]interface{}{"id": "test"},
		Metadata: make(map[string]interface{}),
		Format:   codec.JSON,
	}

	result, err := p.Execute(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output.Format != codec.JSON {
		t.Error("format değişmemeli")
	}
}

func TestPipeline_MultiStep(t *testing.T) {
	schemaReg := schema.NewRegistry()
	schemaReg.Register("multi_test", []schema.Field{
		{Name: "sensor_id", Type: schema.TypeString, Required: true},
		{Name: "sensor_type", Type: schema.TypeString, Required: true},
	})

	p := NewPipeline("multi").
		AddStep(NewValidateStep("multi_test", 1, schemaReg)).
		AddStep(NewTimestampStep("processed_at")).
		AddStep(NewEnrichStep(map[string]interface{}{"processed_by": "bagdas"})).
		AddStep(NewFilterStep([]string{"sensor_id", "sensor_type", "processed_at", "processed_by"}))

	msg := NewMessage(map[string]interface{}{
		"sensor_id":   "radar-01",
		"sensor_type": "radar",
		"extra_field": "should_be_removed",
	})

	result, err := p.Execute(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if result.StepsRun != 4 {
		t.Errorf("steps run = %d", result.StepsRun)
	}
	if _, ok := result.Output.Data["extra_field"]; ok {
		t.Error("fazladan alan filtrelenmedi")
	}
	if result.Output.Data["processed_by"] != "bagdas" {
		t.Error("enrich alan eklenmedi")
	}
	if _, ok := result.Output.Data["processed_at"]; !ok {
		t.Error("timestamp eklenmedi")
	}
}

// ──────────────────────────────────────────────
// Pipeline Registry Testleri
// ──────────────────────────────────────────────

func TestPipelineRegistry_Basic(t *testing.T) {
	reg := NewPipelineRegistry()
	p := NewPipeline("test")
	reg.Register(p)

	if reg.Count() != 1 {
		t.Errorf("count = %d", reg.Count())
	}
}

func TestPipelineRegistry_Get(t *testing.T) {
	reg := NewPipelineRegistry()
	reg.Register(NewPipeline("exists"))

	_, err := reg.Get("exists")
	if err != nil {
		t.Fatal(err)
	}

	_, err = reg.Get("nope")
	if err == nil {
		t.Error("bulunamayan pipeline hatası beklendi")
	}
}

func TestPipelineRegistry_List(t *testing.T) {
	reg := NewPipelineRegistry()
	reg.Register(NewPipeline("a"))
	reg.Register(NewPipeline("b"))

	names := reg.List()
	if len(names) != 2 {
		t.Errorf("list = %d", len(names))
	}
}

func TestPipelineRegistry_Delete(t *testing.T) {
	reg := NewPipelineRegistry()
	reg.Register(NewPipeline("del"))
	reg.Delete("del")
	if reg.Count() != 0 {
		t.Error("silme sonrası 0 beklendi")
	}
}

func TestPipelineRegistry_Execute(t *testing.T) {
	reg := NewPipelineRegistry()
	p := NewPipeline("exec-test").AddStep(NewTimestampStep("ts"))
	reg.Register(p)

	msg := NewMessage(map[string]interface{}{"id": "test"})
	result, err := reg.Execute(context.Background(), "exec-test", msg)
	if err != nil {
		t.Fatal(err)
	}
	if result.StepsRun != 1 {
		t.Error("1 step çalışmalı")
	}
}

func TestPipelineRegistry_Execute_NotFound(t *testing.T) {
	reg := NewPipelineRegistry()
	msg := NewMessage(map[string]interface{}{})
	_, err := reg.Execute(context.Background(), "nope", msg)
	if err == nil {
		t.Error("bulunamayan pipeline hatası beklendi")
	}
}

func TestMessageFromJSON(t *testing.T) {
	msg, err := MessageFromJSON([]byte(`{"id":"test","val":42}`))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Data["id"] != "test" {
		t.Error("id kayboldu")
	}
	if msg.Format != codec.JSON {
		t.Errorf("format = %s", msg.Format)
	}
}

func TestMessageFromJSON_Invalid(t *testing.T) {
	_, err := MessageFromJSON([]byte("invalid"))
	if err == nil {
		t.Error("geçersiz JSON hatası beklendi")
	}
}

func TestNewMessage(t *testing.T) {
	msg := NewMessage(map[string]interface{}{"k": "v"})
	if msg.Data["k"] != "v" {
		t.Error("data korunmadı")
	}
	if msg.Metadata == nil {
		t.Error("metadata nil olmamalı")
	}
}

// ──────────────────────────────────────────────
// Step Name Testleri
// ──────────────────────────────────────────────

func TestStepNames(t *testing.T) {
	tests := []struct {
		step Step
		want string
	}{
		{NewTimestampStep("ts"), "timestamp"},
		{NewEnrichStep(nil), "enrich"},
		{NewFilterStep(nil), "filter"},
		{NewRenameStep(nil), "rename"},
		{NewCustomStep("my_step", nil), "my_step"},
	}
	for _, tt := range tests {
		if got := tt.step.Name(); got != tt.want {
			t.Errorf("step name = %q, beklenen %q", got, tt.want)
		}
	}
}

func TestTimestampStep_DefaultField(t *testing.T) {
	step := NewTimestampStep("")
	msg := NewMessage(map[string]interface{}{})
	result, err := step.Process(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result.Data["timestamp"]; !ok {
		t.Error("varsayılan timestamp alanı eklenmedi")
	}
}

func TestTimestampStep_ValidFormat(t *testing.T) {
	step := NewTimestampStep("ts")
	msg := NewMessage(map[string]interface{}{})
	result, _ := step.Process(context.Background(), msg)
	ts := result.Data["ts"].(string)
	_, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t.Errorf("geçersiz zaman formatı: %s", ts)
	}
}
