// Package schema — Şema kayıt defteri ve versiyonlama.
//
// Farklı modüller arasındaki veri sözleşmelerini yönetir.
// Her şema bir isim, versiyon ve alan tanımları içerir.
// Forward/Backward uyumluluk kontrolü sağlar.
package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// FieldType — Alan veri tipleri
type FieldType string

const (
	TypeString  FieldType = "string"
	TypeInt     FieldType = "int"
	TypeFloat   FieldType = "float"
	TypeBool    FieldType = "bool"
	TypeBytes   FieldType = "bytes"
	TypeArray   FieldType = "array"
	TypeMap     FieldType = "map"
	TypeAny     FieldType = "any"
)

// ValidFieldTypes — Geçerli alan tipleri
var ValidFieldTypes = map[FieldType]bool{
	TypeString: true, TypeInt: true, TypeFloat: true,
	TypeBool: true, TypeBytes: true, TypeArray: true,
	TypeMap: true, TypeAny: true,
}

// Field — Şema alanı tanımı
type Field struct {
	Name     string    `json:"name"`
	Type     FieldType `json:"type"`
	Required bool      `json:"required"`
	Default  interface{} `json:"default,omitempty"`
}

// Schema — Veri şeması
type Schema struct {
	Name      string    `json:"name"`
	Version   int       `json:"version"`
	Fields    []Field   `json:"fields"`
	CreatedAt time.Time `json:"created_at"`
}

// Errors
var (
	ErrSchemaNotFound     = errors.New("şema bulunamadı")
	ErrVersionNotFound    = errors.New("versiyon bulunamadı")
	ErrSchemaExists       = errors.New("şema zaten mevcut")
	ErrInvalidSchema      = errors.New("geçersiz şema")
	ErrIncompatible       = errors.New("uyumsuz şema değişikliği")
	ErrValidationFailed   = errors.New("doğrulama hatası")
)

// CompatMode — Uyumluluk modu
type CompatMode int

const (
	CompatNone     CompatMode = iota // Uyumluluk kontrolü yok
	CompatBackward                    // Eski veriler yeni şemada okunabilmeli
	CompatForward                     // Yeni veriler eski şemada okunabilmeli
	CompatFull                        // Her iki yönde uyumlu
)

func (m CompatMode) String() string {
	switch m {
	case CompatNone:
		return "none"
	case CompatBackward:
		return "backward"
	case CompatForward:
		return "forward"
	case CompatFull:
		return "full"
	default:
		return "unknown"
	}
}

// ParseCompatMode — String'den uyumluluk moduna dönüşüm
func ParseCompatMode(s string) (CompatMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "none", "":
		return CompatNone, nil
	case "backward":
		return CompatBackward, nil
	case "forward":
		return CompatForward, nil
	case "full":
		return CompatFull, nil
	default:
		return CompatNone, fmt.Errorf("bilinmeyen uyumluluk modu: %q", s)
	}
}

// Registry — Şema kayıt defteri (thread-safe)
type Registry struct {
	mu      sync.RWMutex
	schemas map[string][]Schema   // name → [v1, v2, ...]
	compat  map[string]CompatMode // name → uyumluluk modu
}

// NewRegistry — Yeni şema kayıt defteri
func NewRegistry() *Registry {
	return &Registry{
		schemas: make(map[string][]Schema),
		compat:  make(map[string]CompatMode),
	}
}

// Register — Yeni şema kaydet (otomatik versiyonlama)
func (r *Registry) Register(name string, fields []Field) (*Schema, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: şema adı boş", ErrInvalidSchema)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("%w: alan listesi boş", ErrInvalidSchema)
	}

	// Alan adlarının benzersiz olduğunu kontrol et
	seen := make(map[string]bool)
	for _, f := range fields {
		if f.Name == "" {
			return nil, fmt.Errorf("%w: alan adı boş", ErrInvalidSchema)
		}
		if !ValidFieldTypes[f.Type] {
			return nil, fmt.Errorf("%w: geçersiz alan tipi %q alan %q için", ErrInvalidSchema, f.Type, f.Name)
		}
		if seen[f.Name] {
			return nil, fmt.Errorf("%w: tekrarlanan alan adı %q", ErrInvalidSchema, f.Name)
		}
		seen[f.Name] = true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	versions := r.schemas[name]
	newVersion := len(versions) + 1

	// Uyumluluk kontrolü
	if len(versions) > 0 && r.compat[name] != CompatNone {
		prev := versions[len(versions)-1]
		if err := checkCompat(prev, fields, r.compat[name]); err != nil {
			return nil, err
		}
	}

	s := Schema{
		Name:      name,
		Version:   newVersion,
		Fields:    fields,
		CreatedAt: time.Now(),
	}
	r.schemas[name] = append(versions, s)
	return &s, nil
}

// Get — Belirli versiyon getir
func (r *Registry) Get(name string, version int) (*Schema, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, ok := r.schemas[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSchemaNotFound, name)
	}
	if version < 1 || version > len(versions) {
		return nil, fmt.Errorf("%w: %s v%d", ErrVersionNotFound, name, version)
	}
	s := versions[version-1]
	return &s, nil
}

// GetLatest — En son versiyon
func (r *Registry) GetLatest(name string) (*Schema, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, ok := r.schemas[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSchemaNotFound, name)
	}
	s := versions[len(versions)-1]
	return &s, nil
}

// List — Tüm şema adlarını listele
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.schemas))
	for n := range r.schemas {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Versions — Bir şemanın tüm versiyonlarını getir
func (r *Registry) Versions(name string) ([]Schema, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, ok := r.schemas[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSchemaNotFound, name)
	}
	out := make([]Schema, len(versions))
	copy(out, versions)
	return out, nil
}

// SetCompat — Uyumluluk modu ayarla
func (r *Registry) SetCompat(name string, mode CompatMode) {
	r.mu.Lock()
	r.compat[name] = mode
	r.mu.Unlock()
}

// GetCompat — Uyumluluk modu oku
func (r *Registry) GetCompat(name string) CompatMode {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.compat[name]
}

// Delete — Şemayı tamamen sil
func (r *Registry) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.schemas[name]; !ok {
		return fmt.Errorf("%w: %s", ErrSchemaNotFound, name)
	}
	delete(r.schemas, name)
	delete(r.compat, name)
	return nil
}

// Count — Toplam şema sayısı
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.schemas)
}

// Validate — Veriyi şemaya göre doğrula
func (r *Registry) Validate(name string, version int, data map[string]interface{}) []string {
	schema, err := r.Get(name, version)
	if err != nil {
		return []string{err.Error()}
	}
	return ValidateData(schema, data)
}

// ValidateData — Veriyi şema alanlarına göre doğrula
func ValidateData(s *Schema, data map[string]interface{}) []string {
	var errs []string
	fieldMap := make(map[string]Field, len(s.Fields))
	for _, f := range s.Fields {
		fieldMap[f.Name] = f
	}

	// Zorunlu alan kontrolü
	for _, f := range s.Fields {
		if f.Required {
			v, ok := data[f.Name]
			if !ok || v == nil {
				errs = append(errs, fmt.Sprintf("zorunlu alan eksik: %s", f.Name))
			}
		}
	}

	// Tip kontrolü
	for k, v := range data {
		f, ok := fieldMap[k]
		if !ok {
			continue // Fazladan alan kabul edilir (forward compat)
		}
		if v == nil {
			continue
		}
		if err := checkType(f, v); err != "" {
			errs = append(errs, err)
		}
	}

	return errs
}

// checkType — Değerin beklenen tiple uyumlu olduğunu kontrol et
func checkType(f Field, v interface{}) string {
	switch f.Type {
	case TypeString:
		if _, ok := v.(string); !ok {
			return fmt.Sprintf("alan %q string olmalı, gelen: %T", f.Name, v)
		}
	case TypeInt:
		switch v.(type) {
		case int, int8, int16, int32, int64:
		case float64:
			fv := v.(float64)
			if fv != float64(int64(fv)) {
				return fmt.Sprintf("alan %q int olmalı, gelen float: %v", f.Name, v)
			}
		case json.Number:
			if _, err := v.(json.Number).Int64(); err != nil {
				return fmt.Sprintf("alan %q int olmalı, gelen: %v", f.Name, v)
			}
		default:
			return fmt.Sprintf("alan %q int olmalı, gelen: %T", f.Name, v)
		}
	case TypeFloat:
		switch v.(type) {
		case float32, float64, int, int8, int16, int32, int64:
		case json.Number:
		default:
			return fmt.Sprintf("alan %q float olmalı, gelen: %T", f.Name, v)
		}
	case TypeBool:
		if _, ok := v.(bool); !ok {
			return fmt.Sprintf("alan %q bool olmalı, gelen: %T", f.Name, v)
		}
	case TypeBytes:
		if _, ok := v.([]byte); !ok {
			return fmt.Sprintf("alan %q bytes olmalı, gelen: %T", f.Name, v)
		}
	case TypeArray:
		if _, ok := v.([]interface{}); !ok {
			return fmt.Sprintf("alan %q array olmalı, gelen: %T", f.Name, v)
		}
	case TypeMap:
		if _, ok := v.(map[string]interface{}); !ok {
			return fmt.Sprintf("alan %q map olmalı, gelen: %T", f.Name, v)
		}
	case TypeAny:
		// Her şey kabul edilir
	}
	return ""
}

// checkCompat — İki şema versiyonu arası uyumluluk kontrolü
func checkCompat(prev Schema, newFields []Field, mode CompatMode) error {
	prevMap := make(map[string]Field, len(prev.Fields))
	for _, f := range prev.Fields {
		prevMap[f.Name] = f
	}
	newMap := make(map[string]Field, len(newFields))
	for _, f := range newFields {
		newMap[f.Name] = f
	}

	switch mode {
	case CompatBackward:
		// Yeni şema eski verileri okumalı
		// Eski zorunlu alanlar kaldırılamaz
		for _, pf := range prev.Fields {
			if pf.Required {
				if _, ok := newMap[pf.Name]; !ok {
					return fmt.Errorf("%w: zorunlu alan %q kaldırılamaz (backward)", ErrIncompatible, pf.Name)
				}
			}
		}
		// Yeni zorunlu alan eklenemez (eski veri bunu içermez)
		for _, nf := range newFields {
			if nf.Required {
				if _, ok := prevMap[nf.Name]; !ok {
					if nf.Default == nil {
						return fmt.Errorf("%w: yeni zorunlu alan %q default olmadan eklenemez (backward)", ErrIncompatible, nf.Name)
					}
				}
			}
		}

	case CompatForward:
		// Eski şema yeni verileri okumalı
		// Yeni zorunlu alanlar kaldırılamaz
		for _, nf := range newFields {
			if nf.Required {
				if _, ok := prevMap[nf.Name]; !ok {
					return fmt.Errorf("%w: yeni zorunlu alan %q forward uyumsuz", ErrIncompatible, nf.Name)
				}
			}
		}

	case CompatFull:
		// Her iki yönde uyumlu
		for _, pf := range prev.Fields {
			if pf.Required {
				if _, ok := newMap[pf.Name]; !ok {
					return fmt.Errorf("%w: zorunlu alan %q kaldırılamaz (full)", ErrIncompatible, pf.Name)
				}
			}
		}
		for _, nf := range newFields {
			if nf.Required {
				if _, ok := prevMap[nf.Name]; !ok {
					return fmt.Errorf("%w: yeni zorunlu alan %q eklenemez (full)", ErrIncompatible, nf.Name)
				}
			}
		}
		// Mevcut alanların tipi değişemez
		for _, nf := range newFields {
			if pf, ok := prevMap[nf.Name]; ok {
				if pf.Type != nf.Type {
					return fmt.Errorf("%w: alan %q tip değişikliği %s→%s (full)", ErrIncompatible, nf.Name, pf.Type, nf.Type)
				}
			}
		}
	}

	return nil
}

// ExportJSON — Tüm şemaları JSON olarak dışarı aktar
func (r *Registry) ExportJSON() ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return json.Marshal(r.schemas)
}

// ImportJSON — JSON'dan şemaları içe aktar
func (r *Registry) ImportJSON(data []byte) error {
	var imported map[string][]Schema
	if err := json.Unmarshal(data, &imported); err != nil {
		return fmt.Errorf("şema import hatası: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, versions := range imported {
		r.schemas[name] = versions
	}
	return nil
}
