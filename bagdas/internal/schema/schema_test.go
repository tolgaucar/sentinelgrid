package schema

import (
	"testing"
)

func TestRegister_Basic(t *testing.T) {
	r := NewRegistry()
	fields := []Field{
		{Name: "id", Type: TypeString, Required: true},
		{Name: "value", Type: TypeFloat, Required: false},
	}
	s, err := r.Register("test", fields)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "test" || s.Version != 1 {
		t.Errorf("name=%s version=%d", s.Name, s.Version)
	}
}

func TestRegister_AutoVersion(t *testing.T) {
	r := NewRegistry()
	fields := []Field{{Name: "id", Type: TypeString, Required: true}}
	r.Register("test", fields)

	fields2 := []Field{
		{Name: "id", Type: TypeString, Required: true},
		{Name: "name", Type: TypeString, Required: false},
	}
	s2, err := r.Register("test", fields2)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Version != 2 {
		t.Errorf("version = %d, beklenen 2", s2.Version)
	}
}

func TestRegister_EmptyName(t *testing.T) {
	r := NewRegistry()
	_, err := r.Register("", []Field{{Name: "id", Type: TypeString}})
	if err == nil {
		t.Error("boş isim için hata beklendi")
	}
}

func TestRegister_EmptyFields(t *testing.T) {
	r := NewRegistry()
	_, err := r.Register("test", []Field{})
	if err == nil {
		t.Error("boş alan listesi için hata beklendi")
	}
}

func TestRegister_DuplicateFieldName(t *testing.T) {
	r := NewRegistry()
	_, err := r.Register("test", []Field{
		{Name: "id", Type: TypeString},
		{Name: "id", Type: TypeInt},
	})
	if err == nil {
		t.Error("tekrarlanan alan adı için hata beklendi")
	}
}

func TestRegister_InvalidFieldType(t *testing.T) {
	r := NewRegistry()
	_, err := r.Register("test", []Field{
		{Name: "id", Type: "invalid_type"},
	})
	if err == nil {
		t.Error("geçersiz alan tipi için hata beklendi")
	}
}

func TestRegister_EmptyFieldName(t *testing.T) {
	r := NewRegistry()
	_, err := r.Register("test", []Field{
		{Name: "", Type: TypeString},
	})
	if err == nil {
		t.Error("boş alan adı için hata beklendi")
	}
}

func TestGet(t *testing.T) {
	r := NewRegistry()
	r.Register("test", []Field{{Name: "id", Type: TypeString, Required: true}})
	r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
		{Name: "v2", Type: TypeString},
	})

	s, err := r.Get("test", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Fields) != 1 {
		t.Errorf("v1 alan sayısı %d", len(s.Fields))
	}

	s2, err := r.Get("test", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Fields) != 2 {
		t.Errorf("v2 alan sayısı %d", len(s2.Fields))
	}
}

func TestGet_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent", 1)
	if err == nil {
		t.Error("bulunamayan şema için hata beklendi")
	}
}

func TestGet_InvalidVersion(t *testing.T) {
	r := NewRegistry()
	r.Register("test", []Field{{Name: "id", Type: TypeString}})
	_, err := r.Get("test", 0)
	if err == nil {
		t.Error("geçersiz versiyon 0 için hata beklendi")
	}
	_, err = r.Get("test", 99)
	if err == nil {
		t.Error("olmayan versiyon için hata beklendi")
	}
}

func TestGetLatest(t *testing.T) {
	r := NewRegistry()
	r.Register("test", []Field{{Name: "id", Type: TypeString}})
	r.Register("test", []Field{{Name: "id", Type: TypeString}, {Name: "v2", Type: TypeString}})

	s, err := r.GetLatest("test")
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != 2 {
		t.Errorf("latest version = %d", s.Version)
	}
}

func TestGetLatest_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.GetLatest("nope")
	if err == nil {
		t.Error("bulunamayan şema için hata beklendi")
	}
}

func TestList(t *testing.T) {
	r := NewRegistry()
	r.Register("alpha", []Field{{Name: "id", Type: TypeString}})
	r.Register("beta", []Field{{Name: "id", Type: TypeString}})

	names := r.List()
	if len(names) != 2 {
		t.Errorf("list uzunluk %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("sıralama hatalı: %v", names)
	}
}

func TestVersions(t *testing.T) {
	r := NewRegistry()
	r.Register("test", []Field{{Name: "id", Type: TypeString}})
	r.Register("test", []Field{{Name: "id", Type: TypeString}, {Name: "v2", Type: TypeString}})

	versions, err := r.Versions("test")
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Errorf("versiyon sayısı %d", len(versions))
	}
}

func TestVersions_NotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Versions("nope")
	if err == nil {
		t.Error("hata beklendi")
	}
}

func TestDelete(t *testing.T) {
	r := NewRegistry()
	r.Register("test", []Field{{Name: "id", Type: TypeString}})
	if err := r.Delete("test"); err != nil {
		t.Fatal(err)
	}
	if r.Count() != 0 {
		t.Error("silme sonrası sayı 0 olmalı")
	}
}

func TestDelete_NotFound(t *testing.T) {
	r := NewRegistry()
	if err := r.Delete("nope"); err == nil {
		t.Error("bulunamayan şema silme için hata beklendi")
	}
}

func TestCount(t *testing.T) {
	r := NewRegistry()
	if r.Count() != 0 {
		t.Error("başlangıç sayısı 0 olmalı")
	}
	r.Register("a", []Field{{Name: "id", Type: TypeString}})
	r.Register("b", []Field{{Name: "id", Type: TypeString}})
	if r.Count() != 2 {
		t.Errorf("sayı = %d", r.Count())
	}
}

func TestCompatMode_String(t *testing.T) {
	tests := []struct {
		mode CompatMode
		want string
	}{
		{CompatNone, "none"},
		{CompatBackward, "backward"},
		{CompatForward, "forward"},
		{CompatFull, "full"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("%d.String() = %q, beklenen %q", tt.mode, got, tt.want)
		}
	}
}

func TestParseCompatMode(t *testing.T) {
	tests := []struct {
		input string
		want  CompatMode
		ok    bool
	}{
		{"none", CompatNone, true},
		{"backward", CompatBackward, true},
		{"forward", CompatForward, true},
		{"full", CompatFull, true},
		{"", CompatNone, true},
		{"BACKWARD", CompatBackward, true},
		{"invalid", CompatNone, false},
	}
	for _, tt := range tests {
		got, err := ParseCompatMode(tt.input)
		if tt.ok && err != nil {
			t.Errorf("ParseCompatMode(%q) hata: %v", tt.input, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("ParseCompatMode(%q) hata beklendi", tt.input)
		}
		if got != tt.want {
			t.Errorf("ParseCompatMode(%q) = %v, beklenen %v", tt.input, got, tt.want)
		}
	}
}

// ──────────────────────────────────────────────
// Uyumluluk (Compat) Testleri
// ──────────────────────────────────────────────

func TestBackwardCompat_RemoveRequired(t *testing.T) {
	r := NewRegistry()
	r.SetCompat("test", CompatBackward)
	r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
		{Name: "name", Type: TypeString, Required: true},
	})

	// Zorunlu alan kaldırma — reddedilmeli
	_, err := r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
	})
	if err == nil {
		t.Error("backward compat: zorunlu alan kaldırma kabul edilmemeli")
	}
}

func TestBackwardCompat_AddOptionalField(t *testing.T) {
	r := NewRegistry()
	r.SetCompat("test", CompatBackward)
	r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
	})

	// İsteğe bağlı alan ekleme — kabul edilmeli
	s, err := r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
		{Name: "extra", Type: TypeString, Required: false},
	})
	if err != nil {
		t.Fatalf("opsiyonel alan ekleme hata: %v", err)
	}
	if s.Version != 2 {
		t.Errorf("version = %d", s.Version)
	}
}

func TestBackwardCompat_AddRequiredWithDefault(t *testing.T) {
	r := NewRegistry()
	r.SetCompat("test", CompatBackward)
	r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
	})

	// Zorunlu alan default ile ekleme — kabul edilmeli
	_, err := r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
		{Name: "new_field", Type: TypeString, Required: true, Default: "default_val"},
	})
	if err != nil {
		t.Errorf("default'lu zorunlu alan ekleme kabul edilmeli: %v", err)
	}
}

func TestBackwardCompat_AddRequiredWithoutDefault(t *testing.T) {
	r := NewRegistry()
	r.SetCompat("test", CompatBackward)
	r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
	})

	// Zorunlu alan default'suz ekleme — reddedilmeli
	_, err := r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
		{Name: "new_req", Type: TypeString, Required: true},
	})
	if err == nil {
		t.Error("default'suz zorunlu alan ekleme reddedilmeli")
	}
}

func TestForwardCompat_AddRequired(t *testing.T) {
	r := NewRegistry()
	r.SetCompat("test", CompatForward)
	r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
	})

	// Yeni zorunlu alan ekleme — forward'da reddedilmeli
	_, err := r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
		{Name: "new_req", Type: TypeString, Required: true},
	})
	if err == nil {
		t.Error("forward compat: yeni zorunlu alan reddedilmeli")
	}
}

func TestFullCompat_TypeChange(t *testing.T) {
	r := NewRegistry()
	r.SetCompat("test", CompatFull)
	r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
		{Name: "count", Type: TypeInt, Required: false},
	})

	// Alan tip değişikliği — full compat'ta reddedilmeli
	_, err := r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
		{Name: "count", Type: TypeString, Required: false},
	})
	if err == nil {
		t.Error("full compat: tip değişikliği reddedilmeli")
	}
}

func TestFullCompat_RemoveRequired(t *testing.T) {
	r := NewRegistry()
	r.SetCompat("test", CompatFull)
	r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
		{Name: "name", Type: TypeString, Required: true},
	})

	_, err := r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
	})
	if err == nil {
		t.Error("full compat: zorunlu alan kaldırma reddedilmeli")
	}
}

func TestFullCompat_AddRequired(t *testing.T) {
	r := NewRegistry()
	r.SetCompat("test", CompatFull)
	r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
	})

	_, err := r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
		{Name: "new", Type: TypeString, Required: true},
	})
	if err == nil {
		t.Error("full compat: yeni zorunlu alan reddedilmeli")
	}
}

func TestCompatNone_Anything(t *testing.T) {
	r := NewRegistry()
	r.SetCompat("test", CompatNone)
	r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
	})

	// Her değişiklik kabul
	_, err := r.Register("test", []Field{
		{Name: "totally_different", Type: TypeInt, Required: true},
	})
	if err != nil {
		t.Errorf("compat none: her şey kabul edilmeli: %v", err)
	}
}

func TestSetGetCompat(t *testing.T) {
	r := NewRegistry()
	r.SetCompat("test", CompatFull)
	if r.GetCompat("test") != CompatFull {
		t.Error("compat mode korunmadı")
	}
	if r.GetCompat("other") != CompatNone {
		t.Error("varsayılan compat None olmalı")
	}
}

// ──────────────────────────────────────────────
// Validation Testleri
// ──────────────────────────────────────────────

func TestValidateData_AllValid(t *testing.T) {
	s := &Schema{
		Name:    "test",
		Version: 1,
		Fields: []Field{
			{Name: "id", Type: TypeString, Required: true},
			{Name: "count", Type: TypeInt, Required: false},
			{Name: "score", Type: TypeFloat, Required: false},
			{Name: "active", Type: TypeBool, Required: false},
		},
	}
	data := map[string]interface{}{
		"id":     "test-1",
		"count":  int64(42),
		"score":  99.5,
		"active": true,
	}
	errs := ValidateData(s, data)
	if len(errs) > 0 {
		t.Errorf("doğrulama hataları: %v", errs)
	}
}

func TestValidateData_MissingRequired(t *testing.T) {
	s := &Schema{
		Name:    "test",
		Version: 1,
		Fields: []Field{
			{Name: "id", Type: TypeString, Required: true},
			{Name: "name", Type: TypeString, Required: true},
		},
	}
	data := map[string]interface{}{
		"id": "test-1",
		// name eksik
	}
	errs := ValidateData(s, data)
	if len(errs) != 1 {
		t.Errorf("1 hata beklendi, gelen: %v", errs)
	}
}

func TestValidateData_WrongType(t *testing.T) {
	s := &Schema{
		Name:    "test",
		Version: 1,
		Fields: []Field{
			{Name: "count", Type: TypeInt, Required: true},
		},
	}
	data := map[string]interface{}{
		"count": "not_a_number",
	}
	errs := ValidateData(s, data)
	if len(errs) != 1 {
		t.Errorf("tip hatası beklendi: %v", errs)
	}
}

func TestValidateData_FloatAsInt(t *testing.T) {
	s := &Schema{
		Name:    "test",
		Version: 1,
		Fields: []Field{
			{Name: "count", Type: TypeInt, Required: true},
		},
	}
	// 42.0 int kabul edilmeli
	data := map[string]interface{}{
		"count": float64(42),
	}
	errs := ValidateData(s, data)
	if len(errs) > 0 {
		t.Errorf("42.0 int olarak kabul edilmeli: %v", errs)
	}

	// 42.5 int kabul edilmemeli
	data2 := map[string]interface{}{
		"count": float64(42.5),
	}
	errs2 := ValidateData(s, data2)
	if len(errs2) != 1 {
		t.Errorf("42.5 int kabul edilmemeli: %v", errs2)
	}
}

func TestValidateData_NullValue(t *testing.T) {
	s := &Schema{
		Name:    "test",
		Version: 1,
		Fields: []Field{
			{Name: "id", Type: TypeString, Required: true},
		},
	}
	data := map[string]interface{}{
		"id": nil,
	}
	errs := ValidateData(s, data)
	if len(errs) != 1 {
		t.Errorf("null required alan hata üretmeli: %v", errs)
	}
}

func TestValidateData_ExtraFieldsAllowed(t *testing.T) {
	s := &Schema{
		Name:    "test",
		Version: 1,
		Fields: []Field{
			{Name: "id", Type: TypeString, Required: true},
		},
	}
	data := map[string]interface{}{
		"id":    "test-1",
		"extra": "fazladan",
	}
	errs := ValidateData(s, data)
	if len(errs) > 0 {
		t.Errorf("fazladan alanlar kabul edilmeli: %v", errs)
	}
}

func TestValidateData_ArrayType(t *testing.T) {
	s := &Schema{
		Name:    "test",
		Version: 1,
		Fields: []Field{
			{Name: "items", Type: TypeArray, Required: true},
		},
	}
	good := map[string]interface{}{"items": []interface{}{1, 2, 3}}
	bad := map[string]interface{}{"items": "not_array"}

	if errs := ValidateData(s, good); len(errs) > 0 {
		t.Errorf("array doğru: %v", errs)
	}
	if errs := ValidateData(s, bad); len(errs) != 1 {
		t.Errorf("array tip hatası beklendi: %v", errs)
	}
}

func TestValidateData_MapType(t *testing.T) {
	s := &Schema{
		Name:    "test",
		Version: 1,
		Fields: []Field{
			{Name: "config", Type: TypeMap, Required: true},
		},
	}
	good := map[string]interface{}{"config": map[string]interface{}{"k": "v"}}
	bad := map[string]interface{}{"config": "not_map"}

	if errs := ValidateData(s, good); len(errs) > 0 {
		t.Errorf("map doğru: %v", errs)
	}
	if errs := ValidateData(s, bad); len(errs) != 1 {
		t.Errorf("map tip hatası beklendi: %v", errs)
	}
}

func TestValidateData_AnyType(t *testing.T) {
	s := &Schema{
		Name:    "test",
		Version: 1,
		Fields: []Field{
			{Name: "data", Type: TypeAny, Required: true},
		},
	}
	data := map[string]interface{}{"data": "anything"}
	errs := ValidateData(s, data)
	if len(errs) > 0 {
		t.Errorf("any tipi her şey kabul etmeli: %v", errs)
	}
}

func TestRegistryValidate(t *testing.T) {
	r := NewRegistry()
	r.Register("test", []Field{
		{Name: "id", Type: TypeString, Required: true},
	})
	errs := r.Validate("test", 1, map[string]interface{}{"id": "ok"})
	if len(errs) > 0 {
		t.Errorf("hata beklenmiyor: %v", errs)
	}
}

func TestRegistryValidate_NotFound(t *testing.T) {
	r := NewRegistry()
	errs := r.Validate("nope", 1, map[string]interface{}{})
	if len(errs) != 1 {
		t.Error("şema bulunamadı hatası beklendi")
	}
}

// ──────────────────────────────────────────────
// Export/Import Testleri
// ──────────────────────────────────────────────

func TestExportImport(t *testing.T) {
	r := NewRegistry()
	r.Register("test", []Field{{Name: "id", Type: TypeString, Required: true}})
	r.Register("other", []Field{{Name: "val", Type: TypeInt, Required: false}})

	data, err := r.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}

	r2 := NewRegistry()
	if err := r2.ImportJSON(data); err != nil {
		t.Fatal(err)
	}
	if r2.Count() != 2 {
		t.Errorf("import sonrası sayı = %d", r2.Count())
	}
}

func TestImportJSON_Invalid(t *testing.T) {
	r := NewRegistry()
	if err := r.ImportJSON([]byte("invalid")); err == nil {
		t.Error("geçersiz JSON import hatası beklendi")
	}
}
