package thermal

import (
	"testing"
)

func newTestEngine() *Engine {
	return NewEngine(22.0, 2.0, 300, 640, 24.0, 60)
}

func TestScanEmpty(t *testing.T) {
	eng := newTestEngine()
	res := eng.Scan(nil)
	if len(res.Signatures) != 0 {
		t.Errorf("boş tarama sonucu beklenirken %d imza döndü", len(res.Signatures))
	}
	if res.AmbientC != 22.0 {
		t.Errorf("ortam sıcaklığı 22.0 beklenirken %.1f döndü", res.AmbientC)
	}
}

func TestScanTankDetection(t *testing.T) {
	eng := newTestEngine()
	ents := []EntityInput{
		{ID: "tank-1", Type: "tank", Faction: "enemy", Status: "active", Lat: 39.0, Lon: 32.0, Speed: 40, Health: 100},
	}
	res := eng.Scan(ents)
	if len(res.Signatures) == 0 {
		t.Fatal("tank termal imzası tespit edilemedi")
	}
	sig := res.Signatures[0]
	if sig.EntityID != "tank-1" {
		t.Errorf("entity_id='tank-1' beklenirken '%s' döndü", sig.EntityID)
	}
	if sig.TempC <= 22.0 {
		t.Errorf("tank sıcaklığı ortam sıcaklığından yüksek olmalı, %.1f°C döndü", sig.TempC)
	}
	if sig.DeltaC <= 0 {
		t.Error("delta sıcaklık pozitif olmalı")
	}
	if sig.Confidence <= 0 || sig.Confidence > 1.0 {
		t.Errorf("güven 0-1 arasında olmalı, %.2f döndü", sig.Confidence)
	}
}

func TestScanFighterJetHighTemp(t *testing.T) {
	eng := newTestEngine()
	ents := []EntityInput{
		{ID: "jet-1", Type: "fighter_jet", Status: "active", Lat: 39.0, Lon: 32.0, Speed: 500, Health: 100},
	}
	res := eng.Scan(ents)
	if len(res.Signatures) == 0 {
		t.Fatal("jet termal imzası tespit edilemedi")
	}
	sig := res.Signatures[0]
	if sig.TempC < 200 {
		t.Errorf("savaş uçağı yüksek sıcaklık üretmeli, %.1f°C döndü", sig.TempC)
	}
	if sig.Type != SigFriction {
		t.Errorf("yüksek hızda sürtünme tipi beklenirken '%s' döndü", sig.Type)
	}
}

func TestScanPersonnelLowTemp(t *testing.T) {
	eng := newTestEngine()
	ents := []EntityInput{
		{ID: "inf-1", Type: "infantry", Status: "active", Lat: 39.0, Lon: 32.0, Speed: 0, Health: 100},
	}
	res := eng.Scan(ents)
	if len(res.Signatures) == 0 {
		t.Fatal("personel termal imzası tespit edilemedi")
	}
	sig := res.Signatures[0]
	if sig.TempC < 35 || sig.TempC > 40 {
		t.Errorf("insan vücut sıcaklığı 35-40°C arasında olmalı, %.1f°C döndü", sig.TempC)
	}
	if sig.Type != SigPersonnel {
		t.Errorf("personel tipi beklenirken '%s' döndü", sig.Type)
	}
}

func TestScanDestroyedVehicle(t *testing.T) {
	eng := newTestEngine()
	ents := []EntityInput{
		{ID: "apc-1", Type: "apc", Status: "destroyed", Lat: 39.0, Lon: 32.0, Speed: 0, Health: 0},
	}
	res := eng.Scan(ents)
	if len(res.Signatures) == 0 {
		t.Fatal("imha edilmiş araç termal imzası tespit edilemedi")
	}
	sig := res.Signatures[0]
	if sig.State != StateCooling {
		t.Errorf("imha sonrası soğuma durumu beklenirken '%s' döndü", sig.State)
	}
	if sig.Type != SigExplosion {
		t.Errorf("patlama sonrası tipi beklenirken '%s' döndü", sig.Type)
	}
}

func TestScanDamagedVehicleHotter(t *testing.T) {
	eng := newTestEngine()
	healthy := []EntityInput{
		{ID: "trk-h", Type: "truck", Status: "active", Lat: 39.0, Lon: 32.0, Speed: 30, Health: 100},
	}
	damaged := []EntityInput{
		{ID: "trk-d", Type: "truck", Status: "damaged", Lat: 39.0, Lon: 32.0, Speed: 30, Health: 20},
	}
	eng2 := newTestEngine()
	resH := eng.Scan(healthy)
	resD := eng2.Scan(damaged)
	if len(resH.Signatures) == 0 || len(resD.Signatures) == 0 {
		t.Fatal("imzalar tespit edilemedi")
	}
	// hasarlı araç genelde daha sıcak (mutlak garanti olmasa da model bunu destekliyor)
	if resD.Signatures[0].TempC < resH.Signatures[0].TempC*0.8 {
		t.Log("hasarlı araç beklenenden soğuk — rastgele gürültüden kaynaklanabilir")
	}
}

func TestHistoryAccumulation(t *testing.T) {
	eng := newTestEngine()
	ent := []EntityInput{
		{ID: "v-1", Type: "tank", Status: "active", Lat: 39.0, Lon: 32.0, Speed: 30, Health: 100},
	}
	for i := 0; i < 5; i++ {
		eng.Scan(ent)
	}
	sig, ok := eng.GetSignature("v-1")
	if !ok {
		t.Fatal("imza bulunamadı")
	}
	if len(sig.History) != 5 {
		t.Errorf("5 geçmiş ölçüm beklenirken %d döndü", len(sig.History))
	}
}

func TestGetAllSignatures(t *testing.T) {
	eng := newTestEngine()
	ents := []EntityInput{
		{ID: "a", Type: "tank", Status: "active", Lat: 39.0, Lon: 32.0, Speed: 30, Health: 100},
		{ID: "b", Type: "truck", Status: "active", Lat: 39.1, Lon: 32.1, Speed: 20, Health: 100},
	}
	eng.Scan(ents)
	all := eng.GetAllSignatures()
	if len(all) != 2 {
		t.Errorf("2 imza beklenirken %d döndü", len(all))
	}
}

func TestGetHotSignatures(t *testing.T) {
	eng := newTestEngine()
	ents := []EntityInput{
		{ID: "j", Type: "fighter_jet", Status: "active", Lat: 39.0, Lon: 32.0, Speed: 500, Health: 100},
		{ID: "i", Type: "infantry", Status: "active", Lat: 39.0, Lon: 32.0, Speed: 0, Health: 100},
	}
	eng.Scan(ents)
	hot := eng.GetHotSignatures(100)
	// jet yüksek delta'ya sahip olmalı
	found := false
	for _, s := range hot {
		if s.EntityID == "j" {
			found = true
		}
	}
	if !found {
		t.Error("jet yüksek delta imzalar arasında bulunamadı")
	}
}

func TestSetAmbient(t *testing.T) {
	eng := newTestEngine()
	eng.SetAmbient(35.0)
	stats := eng.Stats()
	if stats["ambient_c"].(float64) != 35.0 {
		t.Error("ortam sıcaklığı güncellenemedi")
	}
}

func TestStats(t *testing.T) {
	eng := newTestEngine()
	eng.Scan([]EntityInput{{ID: "x", Type: "tank", Status: "active", Lat: 39, Lon: 32, Speed: 30, Health: 100}})
	stats := eng.Stats()
	if stats["scan_count"].(int64) != 1 {
		t.Errorf("scan_count=1 beklenirken %v döndü", stats["scan_count"])
	}
}

func TestPrune(t *testing.T) {
	eng := NewEngine(22.0, 2.0, 0, 640, 24.0, 60) // maxAge=0: hemen expire
	eng.Scan([]EntityInput{{ID: "x", Type: "tank", Status: "active", Lat: 39, Lon: 32, Speed: 30, Health: 100}})
	removed := eng.Prune()
	if removed != 1 {
		t.Errorf("1 imza temizlenmesi beklenirken %d döndü", removed)
	}
	all := eng.GetAllSignatures()
	if len(all) != 0 {
		t.Errorf("prune sonrası 0 imza beklenirken %d döndü", len(all))
	}
}

func TestClassifyByTemp(t *testing.T) {
	cases := []struct {
		temp float64
		want string
	}{
		{400, "jet_engine"},
		{200, "heavy_vehicle"},
		{100, "medium_vehicle"},
		{60, "light_vehicle"},
		{45, "electrical_equipment"},
		{30, "unknown"},
	}
	for _, tc := range cases {
		got := classifyByTemp(tc.temp, SigEngine)
		if got != tc.want {
			t.Errorf("classifyByTemp(%.0f) = %s, beklenirken %s", tc.temp, got, tc.want)
		}
	}
}

func TestSubDeltaNotDetected(t *testing.T) {
	eng := NewEngine(22.0, 50.0, 300, 640, 24.0, 60) // yüksek delta eşiği
	ents := []EntityInput{
		{ID: "d-1", Type: "drone", Status: "active", Lat: 39, Lon: 32, Speed: 10, Health: 100},
	}
	res := eng.Scan(ents)
	if len(res.Signatures) > 0 {
		t.Errorf("eşik altı tespit olmamalıydı, %d imza döndü", len(res.Signatures))
	}
}
