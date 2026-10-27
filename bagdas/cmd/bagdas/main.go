package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/otukenmatris/bagdas/internal/adapter"
	"github.com/otukenmatris/bagdas/internal/codec"
	"github.com/otukenmatris/bagdas/internal/config"
	"github.com/otukenmatris/bagdas/internal/pipeline"
	"github.com/otukenmatris/bagdas/internal/schema"
	"github.com/otukenmatris/bagdas/internal/server"

	"otukenmatris/ortam"
)

func main() {
	ortam.AutoLoad("../..")

	cfg := config.DefaultConfig()
	cfg.LoadFromEnv()

	// Codec kayıt defteri
	codecReg := codec.NewRegistry()

	// Şema kayıt defteri
	schemaReg := schema.NewRegistry()
	registerDefaultSchemas(schemaReg)

	// Adaptör
	adpt := adapter.NewAdapter(codecReg)
	adpt.RegisterDefaults()

	// Pipeline kayıt defteri
	pipeReg := pipeline.NewPipelineRegistry()
	registerDefaultPipelines(pipeReg, codecReg, schemaReg)

	// HTTP sunucu
	srv := server.NewServer(codecReg, schemaReg, adpt, pipeReg)

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	go func() {
		if err := srv.ListenAndServe(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[BAĞDAŞ] sunucu hatası: %v", err)
		}
	}()

	log.Printf("[BAĞDAŞ] Hazır — HTTP :%d", cfg.HTTPPort)

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[BAĞDAŞ] Kapatılıyor...")
	ctx, cancel := context.WithTimeout(context.Background(), 5000000000) // 5s
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("[BAĞDAŞ] Kapatıldı.")
}

// registerDefaultSchemas — ÖTÜKEN-MATRİS varsayılan şemaları
func registerDefaultSchemas(reg *schema.Registry) {
	// Sensör verisi şeması
	reg.Register("sensor_data", []schema.Field{
		{Name: "sensor_id", Type: schema.TypeString, Required: true},
		{Name: "sensor_type", Type: schema.TypeString, Required: true},
		{Name: "timestamp", Type: schema.TypeString, Required: true},
		{Name: "lat", Type: schema.TypeFloat, Required: false},
		{Name: "lon", Type: schema.TypeFloat, Required: false},
		{Name: "value", Type: schema.TypeFloat, Required: false},
		{Name: "metadata", Type: schema.TypeMap, Required: false},
	})
	reg.SetCompat("sensor_data", schema.CompatBackward)

	// C2 komut şeması
	reg.Register("c2_command", []schema.Field{
		{Name: "command_id", Type: schema.TypeString, Required: true},
		{Name: "command_type", Type: schema.TypeString, Required: true},
		{Name: "source", Type: schema.TypeString, Required: true},
		{Name: "target", Type: schema.TypeString, Required: false},
		{Name: "priority", Type: schema.TypeInt, Required: false},
		{Name: "payload", Type: schema.TypeMap, Required: false},
	})
	reg.SetCompat("c2_command", schema.CompatFull)

	// Alarm şeması
	reg.Register("alert", []schema.Field{
		{Name: "alert_id", Type: schema.TypeString, Required: true},
		{Name: "severity", Type: schema.TypeString, Required: true},
		{Name: "message", Type: schema.TypeString, Required: true},
		{Name: "source", Type: schema.TypeString, Required: false},
		{Name: "timestamp", Type: schema.TypeString, Required: false},
	})

	// Telemetri şeması
	reg.Register("telemetry", []schema.Field{
		{Name: "module", Type: schema.TypeString, Required: true},
		{Name: "cpu_percent", Type: schema.TypeFloat, Required: false},
		{Name: "mem_percent", Type: schema.TypeFloat, Required: false},
		{Name: "goroutines", Type: schema.TypeInt, Required: false},
		{Name: "uptime_sec", Type: schema.TypeFloat, Required: false},
	})

	// Entity (varlık) şeması
	reg.Register("entity", []schema.Field{
		{Name: "entity_id", Type: schema.TypeString, Required: true},
		{Name: "entity_type", Type: schema.TypeString, Required: true},
		{Name: "position", Type: schema.TypeMap, Required: false},
		{Name: "status", Type: schema.TypeString, Required: false},
		{Name: "side", Type: schema.TypeString, Required: false},
	})
}

// registerDefaultPipelines — Varsayılan pipeline'lar
func registerDefaultPipelines(
	pipeReg *pipeline.PipelineRegistry,
	codecReg *codec.Registry,
	schemaReg *schema.Registry,
) {
	// Sensör veri dönüşüm pipeline'ı
	sensorPipe := pipeline.NewPipeline("sensor_convert").
		AddStep(pipeline.NewValidateStep("sensor_data", 1, schemaReg)).
		AddStep(pipeline.NewTimestampStep("processed_at")).
		AddStep(pipeline.NewEnrichStep(map[string]interface{}{
			"processed_by": "bagdas",
		}))
	pipeReg.Register(sensorPipe)

	// C2 komut pipeline'ı
	c2Pipe := pipeline.NewPipeline("c2_process").
		AddStep(pipeline.NewValidateStep("c2_command", 1, schemaReg)).
		AddStep(pipeline.NewTimestampStep("received_at"))
	pipeReg.Register(c2Pipe)

	// Format dönüşüm pipeline'ı (JSON→MsgPack)
	fmtPipe := pipeline.NewPipeline("to_msgpack").
		AddStep(pipeline.NewConvertFormatStep(codec.MsgPack, codecReg))
	pipeReg.Register(fmtPipe)
}
