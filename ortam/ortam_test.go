package ortam

import (
	"os"
	"testing"
)

func TestCurrent(t *testing.T) {
	os.Setenv("OTUKEN_ENV", "dev")
	if Current() != Dev {
		t.Fatal("expected Dev")
	}
	os.Setenv("OTUKEN_ENV", "prod")
	if Current() != Prod {
		t.Fatal("expected Prod")
	}
	os.Setenv("OTUKEN_ENV", "production")
	if Current() != Prod {
		t.Fatal("expected Prod for 'production'")
	}
	os.Unsetenv("OTUKEN_ENV")
	if Current() != Dev {
		t.Fatal("expected Dev as default")
	}
}

func TestIsProdDev(t *testing.T) {
	os.Setenv("OTUKEN_ENV", "prod")
	if !IsProd() || IsDev() {
		t.Fatal("expected prod")
	}
	os.Setenv("OTUKEN_ENV", "dev")
	if IsProd() || !IsDev() {
		t.Fatal("expected dev")
	}
	os.Unsetenv("OTUKEN_ENV")
}

func TestGetStr(t *testing.T) {
	os.Setenv("TEST_STR", "hello")
	if GetStr("TEST_STR", "world") != "hello" {
		t.Fatal("expected hello")
	}
	os.Unsetenv("TEST_STR")
	if GetStr("TEST_STR", "world") != "world" {
		t.Fatal("expected fallback")
	}
}

func TestGetInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")
	if GetInt("TEST_INT", 0) != 42 {
		t.Fatal("expected 42")
	}
	os.Setenv("TEST_INT", "bad")
	if GetInt("TEST_INT", 7) != 7 {
		t.Fatal("expected fallback on bad input")
	}
	os.Unsetenv("TEST_INT")
	if GetInt("TEST_INT", 99) != 99 {
		t.Fatal("expected fallback")
	}
}

func TestGetBool(t *testing.T) {
	os.Setenv("TEST_BOOL", "true")
	if !GetBool("TEST_BOOL", false) {
		t.Fatal("expected true")
	}
	os.Setenv("TEST_BOOL", "0")
	if GetBool("TEST_BOOL", true) {
		t.Fatal("expected false")
	}
	os.Unsetenv("TEST_BOOL")
	if !GetBool("TEST_BOOL", true) {
		t.Fatal("expected fallback true")
	}
}

func TestLogLevel(t *testing.T) {
	os.Unsetenv("LOG_LEVEL")
	os.Setenv("OTUKEN_ENV", "dev")
	if LogLevel() != "debug" {
		t.Fatalf("expected debug, got %s", LogLevel())
	}
	os.Setenv("OTUKEN_ENV", "prod")
	if LogLevel() != "warn" {
		t.Fatalf("expected warn, got %s", LogLevel())
	}
	os.Setenv("LOG_LEVEL", "info")
	if LogLevel() != "info" {
		t.Fatal("expected explicit info")
	}
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("OTUKEN_ENV")
}

func TestCORSOrigins(t *testing.T) {
	os.Unsetenv("CORS_ORIGINS")
	os.Setenv("OTUKEN_ENV", "dev")
	if CORSOrigins() != "*" {
		t.Fatal("expected * in dev")
	}
	os.Setenv("OTUKEN_ENV", "prod")
	if CORSOrigins() != "" {
		t.Fatal("expected empty in prod")
	}
	os.Setenv("CORS_ORIGINS", "https://example.com")
	if CORSOrigins() != "https://example.com" {
		t.Fatal("expected explicit value")
	}
	os.Unsetenv("CORS_ORIGINS")
	os.Unsetenv("OTUKEN_ENV")
}

func TestLoadEnvFile(t *testing.T) {
	// Create temp env file
	f, err := os.CreateTemp("", ".env.test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("# comment\nTEST_LOAD_A=alpha\nTEST_LOAD_B=beta\n")
	f.Close()

	os.Unsetenv("TEST_LOAD_A")
	os.Unsetenv("TEST_LOAD_B")

	LoadEnvFile(f.Name())

	if os.Getenv("TEST_LOAD_A") != "alpha" {
		t.Fatal("expected alpha")
	}
	if os.Getenv("TEST_LOAD_B") != "beta" {
		t.Fatal("expected beta")
	}

	// Should not overwrite existing
	os.Setenv("TEST_LOAD_A", "keep")
	LoadEnvFile(f.Name())
	if os.Getenv("TEST_LOAD_A") != "keep" {
		t.Fatal("should not overwrite existing")
	}

	os.Unsetenv("TEST_LOAD_A")
	os.Unsetenv("TEST_LOAD_B")
}
