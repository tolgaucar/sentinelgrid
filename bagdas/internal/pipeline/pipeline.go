// Package pipeline — Composable veri dönüşüm pipeline'ları.
//
// Veri akışı: Girdi → [Validasyon] → [Dönüştürme] → [Zenginleştirme] → Çıktı
// Her aşama bir Step interface'i uygular.
// Pipeline zincir halinde çalışır, herhangi bir adımda hata olursa durur.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/otukenmatris/bagdas/internal/codec"
	"github.com/otukenmatris/bagdas/internal/schema"
)

// Message — Pipeline mesajı
type Message struct {
	Data     map[string]interface{} `json:"data"`
	Metadata map[string]interface{} `json:"metadata"`
	Format   codec.Format           `json:"format"`
}

// NewMessage — Yeni pipeline mesajı
func NewMessage(data map[string]interface{}) *Message {
	return &Message{
		Data:     data,
		Metadata: make(map[string]interface{}),
		Format:   codec.JSON,
	}
}

// Step — Pipeline adımı arayüzü
type Step interface {
	Name() string
	Process(ctx context.Context, msg *Message) (*Message, error)
}

// Pipeline — Dönüşüm pipeline'ı
type Pipeline struct {
	name  string
	steps []Step
}

// NewPipeline — Yeni pipeline oluştur
func NewPipeline(name string) *Pipeline {
	return &Pipeline{
		name:  name,
		steps: make([]Step, 0),
	}
}

// AddStep — Adım ekle
func (p *Pipeline) AddStep(s Step) *Pipeline {
	p.steps = append(p.steps, s)
	return p
}

// StepCount — Adım sayısı
func (p *Pipeline) StepCount() int {
	return len(p.steps)
}

// Name — Pipeline adı
func (p *Pipeline) Name() string {
	return p.name
}

// Result — Pipeline sonucu
type Result struct {
	Output     *Message      `json:"output"`
	StepsRun   int           `json:"steps_run"`
	Duration   time.Duration `json:"duration"`
	StepTimings []StepTiming `json:"step_timings"`
	Error      string        `json:"error,omitempty"`
}

// StepTiming — Adım süre bilgisi
type StepTiming struct {
	Name     string        `json:"name"`
	Duration time.Duration `json:"duration"`
}

// Execute — Pipeline'ı çalıştır
func (p *Pipeline) Execute(ctx context.Context, msg *Message) (*Result, error) {
	start := time.Now()
	result := &Result{
		StepTimings: make([]StepTiming, 0, len(p.steps)),
	}

	current := msg
	for i, step := range p.steps {
		select {
		case <-ctx.Done():
			result.StepsRun = i
			result.Duration = time.Since(start)
			result.Error = ctx.Err().Error()
			return result, ctx.Err()
		default:
		}

		stepStart := time.Now()
		next, err := step.Process(ctx, current)
		stepDur := time.Since(stepStart)

		result.StepTimings = append(result.StepTimings, StepTiming{
			Name:     step.Name(),
			Duration: stepDur,
		})

		if err != nil {
			result.StepsRun = i
			result.Duration = time.Since(start)
			result.Error = fmt.Sprintf("adım %q hatası: %v", step.Name(), err)
			return result, fmt.Errorf("pipeline %q adım %d (%s): %w", p.name, i, step.Name(), err)
		}
		current = next
	}

	result.Output = current
	result.StepsRun = len(p.steps)
	result.Duration = time.Since(start)
	return result, nil
}

// ──────────────────────────────────────────────
// Yerleşik Pipeline Adımları
// ──────────────────────────────────────────────

// ValidateStep — Şema doğrulama adımı
type ValidateStep struct {
	schemaName    string
	schemaVersion int
	reg           *schema.Registry
}

func NewValidateStep(schemaName string, version int, reg *schema.Registry) *ValidateStep {
	return &ValidateStep{
		schemaName:    schemaName,
		schemaVersion: version,
		reg:           reg,
	}
}

func (s *ValidateStep) Name() string { return "validate:" + s.schemaName }

func (s *ValidateStep) Process(_ context.Context, msg *Message) (*Message, error) {
	errs := s.reg.Validate(s.schemaName, s.schemaVersion, msg.Data)
	if len(errs) > 0 {
		return nil, fmt.Errorf("doğrulama hataları: %s", strings.Join(errs, "; "))
	}
	return msg, nil
}

// ConvertFormatStep — Format dönüştürme adımı
type ConvertFormatStep struct {
	target   codec.Format
	registry *codec.Registry
}

func NewConvertFormatStep(target codec.Format, reg *codec.Registry) *ConvertFormatStep {
	return &ConvertFormatStep{target: target, registry: reg}
}

func (s *ConvertFormatStep) Name() string { return "convert:" + string(s.target) }

func (s *ConvertFormatStep) Process(_ context.Context, msg *Message) (*Message, error) {
	if msg.Format == s.target {
		return msg, nil
	}

	// Encode current format → byte → Convert → Decode target
	srcCodec, err := s.registry.Get(msg.Format)
	if err != nil {
		return nil, err
	}

	encoded, err := srcCodec.Encode(msg.Data)
	if err != nil {
		return nil, err
	}

	converted, err := s.registry.Convert(encoded, msg.Format, s.target)
	if err != nil {
		return nil, err
	}

	dstCodec, err := s.registry.Get(s.target)
	if err != nil {
		return nil, err
	}

	decoded, err := dstCodec.Decode(converted)
	if err != nil {
		return nil, err
	}

	newData, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("dönüşüm sonucu map değil: %T", decoded)
	}

	return &Message{
		Data:     newData,
		Metadata: msg.Metadata,
		Format:   s.target,
	}, nil
}

// EnrichStep — Veri zenginleştirme adımı (sabit alanlar ekler)
type EnrichStep struct {
	fields map[string]interface{}
}

func NewEnrichStep(fields map[string]interface{}) *EnrichStep {
	return &EnrichStep{fields: fields}
}

func (s *EnrichStep) Name() string { return "enrich" }

func (s *EnrichStep) Process(_ context.Context, msg *Message) (*Message, error) {
	for k, v := range s.fields {
		if _, exists := msg.Data[k]; !exists {
			msg.Data[k] = v
		}
	}
	return msg, nil
}

// FilterStep — Alan filtreleme adımı (sadece belirli alanları geçir)
type FilterStep struct {
	allowedFields map[string]bool
}

func NewFilterStep(fields []string) *FilterStep {
	allowed := make(map[string]bool, len(fields))
	for _, f := range fields {
		allowed[f] = true
	}
	return &FilterStep{allowedFields: allowed}
}

func (s *FilterStep) Name() string { return "filter" }

func (s *FilterStep) Process(_ context.Context, msg *Message) (*Message, error) {
	filtered := make(map[string]interface{})
	for k, v := range msg.Data {
		if s.allowedFields[k] {
			filtered[k] = v
		}
	}
	msg.Data = filtered
	return msg, nil
}

// RenameStep — Alan yeniden adlandırma adımı
type RenameStep struct {
	renames map[string]string // eski ad → yeni ad
}

func NewRenameStep(renames map[string]string) *RenameStep {
	return &RenameStep{renames: renames}
}

func (s *RenameStep) Name() string { return "rename" }

func (s *RenameStep) Process(_ context.Context, msg *Message) (*Message, error) {
	for old, new_ := range s.renames {
		if v, ok := msg.Data[old]; ok {
			delete(msg.Data, old)
			msg.Data[new_] = v
		}
	}
	return msg, nil
}

// TimestampStep — Zaman damgası ekleme adımı
type TimestampStep struct {
	fieldName string
}

func NewTimestampStep(fieldName string) *TimestampStep {
	if fieldName == "" {
		fieldName = "timestamp"
	}
	return &TimestampStep{fieldName: fieldName}
}

func (s *TimestampStep) Name() string { return "timestamp" }

func (s *TimestampStep) Process(_ context.Context, msg *Message) (*Message, error) {
	msg.Data[s.fieldName] = time.Now().UTC().Format(time.RFC3339Nano)
	return msg, nil
}

// CustomStep — Özel fonksiyon adımı
type CustomStep struct {
	name string
	fn   func(ctx context.Context, msg *Message) (*Message, error)
}

func NewCustomStep(name string, fn func(ctx context.Context, msg *Message) (*Message, error)) *CustomStep {
	return &CustomStep{name: name, fn: fn}
}

func (s *CustomStep) Name() string { return s.name }

func (s *CustomStep) Process(ctx context.Context, msg *Message) (*Message, error) {
	return s.fn(ctx, msg)
}

// ──────────────────────────────────────────────
// Pipeline Registry — Pipeline yönetimi
// ──────────────────────────────────────────────

// PipelineRegistry — Kayıtlı pipeline'lar
type PipelineRegistry struct {
	mu        sync.RWMutex
	pipelines map[string]*Pipeline
}

// NewPipelineRegistry — Yeni pipeline kayıt defteri
func NewPipelineRegistry() *PipelineRegistry {
	return &PipelineRegistry{
		pipelines: make(map[string]*Pipeline),
	}
}

// Register — Pipeline kaydet
func (r *PipelineRegistry) Register(p *Pipeline) {
	r.mu.Lock()
	r.pipelines[p.Name()] = p
	r.mu.Unlock()
}

// Get — Pipeline getir
func (r *PipelineRegistry) Get(name string) (*Pipeline, error) {
	r.mu.RLock()
	p, ok := r.pipelines[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("pipeline bulunamadı: %s", name)
	}
	return p, nil
}

// List — Pipeline listele
func (r *PipelineRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.pipelines))
	for n := range r.pipelines {
		names = append(names, n)
	}
	return names
}

// Delete — Pipeline sil
func (r *PipelineRegistry) Delete(name string) {
	r.mu.Lock()
	delete(r.pipelines, name)
	r.mu.Unlock()
}

// Execute — İsimle pipeline çalıştır
func (r *PipelineRegistry) Execute(ctx context.Context, name string, msg *Message) (*Result, error) {
	p, err := r.Get(name)
	if err != nil {
		return nil, err
	}
	return p.Execute(ctx, msg)
}

// Count — Pipeline sayısı
func (r *PipelineRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.pipelines)
}

// MessageFromJSON — JSON byte'lardan Message oluştur
func MessageFromJSON(data []byte) (*Message, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("JSON parse hatası: %w", err)
	}
	return &Message{
		Data:     m,
		Metadata: make(map[string]interface{}),
		Format:   codec.JSON,
	}, nil
}
