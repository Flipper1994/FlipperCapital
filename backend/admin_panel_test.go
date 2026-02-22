package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// setupAdminConfigRouter creates a gin router with all BXtrender config routes and an admin session.
func setupAdminConfigRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Create admin user
	admin := User{Email: "configadmin@test.com", Username: "configadmin", Password: "hashed", IsAdmin: true}
	db.Create(&admin)

	// Create session
	token := "test-config-admin-token"
	dbSession := DBSession{
		Token:   token,
		UserID:  admin.ID,
		IsAdmin: true,
		Expiry:  time.Now().Add(1 * time.Hour),
	}
	db.Create(&dbSession)
	sessions[token] = Session{UserID: admin.ID, IsAdmin: true, Expiry: dbSession.Expiry}

	r := gin.New()
	api := r.Group("/api")

	// Admin config endpoints (GET + PUT)
	api.GET("/admin/bxtrender-config", authMiddleware(), adminOnly(), getBXtrenderConfig)
	api.PUT("/admin/bxtrender-config", authMiddleware(), adminOnly(), updateBXtrenderConfig)
	api.GET("/admin/bxtrender-quant-config", authMiddleware(), adminOnly(), getBXtrenderQuantConfig)
	api.PUT("/admin/bxtrender-quant-config", authMiddleware(), adminOnly(), updateBXtrenderQuantConfig)
	api.GET("/admin/bxtrender-ditz-config", authMiddleware(), adminOnly(), getBXtrenderDitzConfig)
	api.PUT("/admin/bxtrender-ditz-config", authMiddleware(), adminOnly(), updateBXtrenderDitzConfig)
	api.GET("/admin/bxtrender-trader-config", authMiddleware(), adminOnly(), getBXtrenderTraderConfig)
	api.PUT("/admin/bxtrender-trader-config", authMiddleware(), adminOnly(), updateBXtrenderTraderConfig)

	// Public config endpoints (GET only, no auth)
	api.GET("/bxtrender-config", getBXtrenderConfigPublic)
	api.GET("/bxtrender-quant-config", getBXtrenderQuantConfigPublic)
	api.GET("/bxtrender-ditz-config", getBXtrenderDitzConfigPublic)
	api.GET("/bxtrender-trader-config", getBXtrenderTraderConfigPublic)

	return r, token
}

// adminPUT is a helper that sends a PUT request with JSON body and admin auth token.
func adminPUT(r *gin.Engine, token, path string, body interface{}) *httptest.ResponseRecorder {
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", path, bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// adminGET is a helper that sends a GET request with admin auth token.
func adminGET(r *gin.Engine, token, path string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest("GET", path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool {
	return &b
}

// ============================================================
// Trader Config Tests
// ============================================================

func TestAdminTraderConfig_CreateAndGet(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	// PUT to create
	putBody := map[string]interface{}{
		"short_l1":     10,
		"short_l2":     25,
		"short_l3":     18,
		"long_l1":      30,
		"long_l2":      20,
		"ma_filter_on": true,
		"ma_length":    100,
		"ma_type":      "SMA",
		"tsl_percent":  15.0,
		"tsl_enabled":  true,
	}
	w := adminPUT(r, token, "/api/admin/bxtrender-trader-config", putBody)
	if w.Code != 200 {
		t.Fatalf("PUT expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// GET to verify
	w = adminGET(r, token, "/api/admin/bxtrender-trader-config")
	if w.Code != 200 {
		t.Fatalf("GET expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var cfg BXtrenderTraderConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	if cfg.ShortL1 != 10 {
		t.Errorf("ShortL1: expected 10, got %d", cfg.ShortL1)
	}
	if cfg.ShortL2 != 25 {
		t.Errorf("ShortL2: expected 25, got %d", cfg.ShortL2)
	}
	if cfg.ShortL3 != 18 {
		t.Errorf("ShortL3: expected 18, got %d", cfg.ShortL3)
	}
	if cfg.LongL1 != 30 {
		t.Errorf("LongL1: expected 30, got %d", cfg.LongL1)
	}
	if cfg.LongL2 != 20 {
		t.Errorf("LongL2: expected 20, got %d", cfg.LongL2)
	}
	if !cfg.MaFilterOn {
		t.Error("MaFilterOn: expected true, got false")
	}
	if cfg.MaLength != 100 {
		t.Errorf("MaLength: expected 100, got %d", cfg.MaLength)
	}
	if cfg.MaType != "SMA" {
		t.Errorf("MaType: expected SMA, got %s", cfg.MaType)
	}
	if cfg.TslPercent != 15.0 {
		t.Errorf("TslPercent: expected 15.0, got %f", cfg.TslPercent)
	}
	if !cfg.TslEnabled {
		t.Error("TslEnabled: expected true, got false")
	}
}

func TestAdminTraderConfig_UpdateExisting(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	// Create initial config
	initial := map[string]interface{}{
		"short_l1":     5,
		"short_l2":     20,
		"short_l3":     15,
		"long_l1":      20,
		"long_l2":      15,
		"ma_filter_on": false,
		"ma_length":    200,
		"ma_type":      "EMA",
		"tsl_percent":  20.0,
		"tsl_enabled":  true,
	}
	w := adminPUT(r, token, "/api/admin/bxtrender-trader-config", initial)
	if w.Code != 200 {
		t.Fatalf("initial PUT: expected 200, got %d", w.Code)
	}

	// Update with different values
	updated := map[string]interface{}{
		"short_l1":     8,
		"short_l2":     30,
		"short_l3":     12,
		"long_l1":      25,
		"long_l2":      18,
		"ma_filter_on": true,
		"ma_length":    150,
		"ma_type":      "SMA",
		"tsl_percent":  10.0,
		"tsl_enabled":  false,
	}
	w = adminPUT(r, token, "/api/admin/bxtrender-trader-config", updated)
	if w.Code != 200 {
		t.Fatalf("update PUT: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the update via GET
	w = adminGET(r, token, "/api/admin/bxtrender-trader-config")
	var cfg BXtrenderTraderConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	if cfg.ShortL1 != 8 {
		t.Errorf("ShortL1: expected 8, got %d", cfg.ShortL1)
	}
	if cfg.ShortL2 != 30 {
		t.Errorf("ShortL2: expected 30, got %d", cfg.ShortL2)
	}
	if cfg.LongL1 != 25 {
		t.Errorf("LongL1: expected 25, got %d", cfg.LongL1)
	}
	if !cfg.MaFilterOn {
		t.Error("MaFilterOn: expected true after update")
	}
	if cfg.MaLength != 150 {
		t.Errorf("MaLength: expected 150, got %d", cfg.MaLength)
	}
	if cfg.MaType != "SMA" {
		t.Errorf("MaType: expected SMA, got %s", cfg.MaType)
	}
	if cfg.TslPercent != 10.0 {
		t.Errorf("TslPercent: expected 10.0, got %f", cfg.TslPercent)
	}
	if cfg.TslEnabled {
		t.Error("TslEnabled: expected false after update, got true")
	}
}

func TestAdminTraderConfig_TslEnabledFalse(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	// Create config with tsl_enabled explicitly set to false
	body := map[string]interface{}{
		"short_l1":     5,
		"short_l2":     20,
		"short_l3":     15,
		"long_l1":      20,
		"long_l2":      15,
		"ma_filter_on": false,
		"ma_length":    200,
		"ma_type":      "EMA",
		"tsl_percent":  20.0,
		"tsl_enabled":  false,
	}
	w := adminPUT(r, token, "/api/admin/bxtrender-trader-config", body)
	if w.Code != 200 {
		t.Fatalf("PUT expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify via PUT response
	var putResp BXtrenderTraderConfig
	json.Unmarshal(w.Body.Bytes(), &putResp)
	if putResp.TslEnabled {
		t.Error("PUT response: tsl_enabled should be false, got true")
	}

	// Verify via GET (re-read from DB)
	w = adminGET(r, token, "/api/admin/bxtrender-trader-config")
	var cfg BXtrenderTraderConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	if cfg.TslEnabled {
		t.Error("CRITICAL BUG: tsl_enabled=false was not persisted! Got true after re-read from DB")
	}

	// Also verify directly in DB
	var dbCfg BXtrenderTraderConfig
	db.First(&dbCfg)
	if dbCfg.TslEnabled {
		t.Error("CRITICAL BUG: tsl_enabled=false not stored in DB (GORM zero-value issue)")
	}
}

func TestAdminTraderConfig_TslEnabledToggle(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	base := map[string]interface{}{
		"short_l1":     5,
		"short_l2":     20,
		"short_l3":     15,
		"long_l1":      20,
		"long_l2":      15,
		"ma_filter_on": false,
		"ma_length":    200,
		"ma_type":      "EMA",
		"tsl_percent":  20.0,
	}

	// Step 1: Set tsl_enabled = true
	base["tsl_enabled"] = true
	w := adminPUT(r, token, "/api/admin/bxtrender-trader-config", base)
	if w.Code != 200 {
		t.Fatalf("step1 PUT: expected 200, got %d", w.Code)
	}
	w = adminGET(r, token, "/api/admin/bxtrender-trader-config")
	var cfg BXtrenderTraderConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)
	if !cfg.TslEnabled {
		t.Error("step1: tsl_enabled should be true")
	}

	// Step 2: Toggle to false
	base["tsl_enabled"] = false
	w = adminPUT(r, token, "/api/admin/bxtrender-trader-config", base)
	if w.Code != 200 {
		t.Fatalf("step2 PUT: expected 200, got %d", w.Code)
	}
	w = adminGET(r, token, "/api/admin/bxtrender-trader-config")
	json.Unmarshal(w.Body.Bytes(), &cfg)
	if cfg.TslEnabled {
		t.Error("step2: tsl_enabled should be false after toggle")
	}

	// Step 3: Toggle back to true
	base["tsl_enabled"] = true
	w = adminPUT(r, token, "/api/admin/bxtrender-trader-config", base)
	if w.Code != 200 {
		t.Fatalf("step3 PUT: expected 200, got %d", w.Code)
	}
	w = adminGET(r, token, "/api/admin/bxtrender-trader-config")
	json.Unmarshal(w.Body.Bytes(), &cfg)
	if !cfg.TslEnabled {
		t.Error("step3: tsl_enabled should be true after toggle back")
	}
}

func TestAdminTraderConfig_DefaultValues(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	// GET before any PUT should return defaults with id=0
	w := adminGET(r, token, "/api/admin/bxtrender-trader-config")
	if w.Code != 200 {
		t.Fatalf("GET expected 200, got %d", w.Code)
	}

	var cfg BXtrenderTraderConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	if cfg.ID != 0 {
		t.Errorf("expected id=0 for default config, got %d", cfg.ID)
	}
	if cfg.ShortL1 != 5 {
		t.Errorf("default ShortL1: expected 5, got %d", cfg.ShortL1)
	}
	if cfg.ShortL2 != 20 {
		t.Errorf("default ShortL2: expected 20, got %d", cfg.ShortL2)
	}
	if cfg.ShortL3 != 15 {
		t.Errorf("default ShortL3: expected 15, got %d", cfg.ShortL3)
	}
	if cfg.LongL1 != 20 {
		t.Errorf("default LongL1: expected 20, got %d", cfg.LongL1)
	}
	if cfg.LongL2 != 15 {
		t.Errorf("default LongL2: expected 15, got %d", cfg.LongL2)
	}
	// Trader default: MaFilterOn = false
	if cfg.MaFilterOn {
		t.Error("default MaFilterOn: expected false for Trader, got true")
	}
	if cfg.MaLength != 200 {
		t.Errorf("default MaLength: expected 200, got %d", cfg.MaLength)
	}
	if cfg.MaType != "EMA" {
		t.Errorf("default MaType: expected EMA, got %s", cfg.MaType)
	}
	if cfg.TslPercent != 20.0 {
		t.Errorf("default TslPercent: expected 20.0, got %f", cfg.TslPercent)
	}
}

func TestAdminTraderConfig_PartialUpdate(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	// Create initial config with specific values
	initial := map[string]interface{}{
		"short_l1":     10,
		"short_l2":     25,
		"short_l3":     18,
		"long_l1":      30,
		"long_l2":      20,
		"ma_filter_on": true,
		"ma_length":    150,
		"ma_type":      "SMA",
		"tsl_percent":  15.0,
		"tsl_enabled":  true,
	}
	w := adminPUT(r, token, "/api/admin/bxtrender-trader-config", initial)
	if w.Code != 200 {
		t.Fatalf("initial PUT: expected 200, got %d", w.Code)
	}

	// Update only tsl_enabled to false, send all other fields unchanged
	// (The handler replaces all fields from the request body)
	partial := map[string]interface{}{
		"short_l1":     10,
		"short_l2":     25,
		"short_l3":     18,
		"long_l1":      30,
		"long_l2":      20,
		"ma_filter_on": true,
		"ma_length":    150,
		"ma_type":      "SMA",
		"tsl_percent":  15.0,
		"tsl_enabled":  false, // Only this changed
	}
	w = adminPUT(r, token, "/api/admin/bxtrender-trader-config", partial)
	if w.Code != 200 {
		t.Fatalf("partial PUT: expected 200, got %d", w.Code)
	}

	w = adminGET(r, token, "/api/admin/bxtrender-trader-config")
	var cfg BXtrenderTraderConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	// Changed field
	if cfg.TslEnabled {
		t.Error("TslEnabled: expected false after partial update")
	}
	// Unchanged fields
	if cfg.ShortL1 != 10 {
		t.Errorf("ShortL1 should remain 10, got %d", cfg.ShortL1)
	}
	if cfg.ShortL2 != 25 {
		t.Errorf("ShortL2 should remain 25, got %d", cfg.ShortL2)
	}
	if !cfg.MaFilterOn {
		t.Error("MaFilterOn should remain true")
	}
	if cfg.MaLength != 150 {
		t.Errorf("MaLength should remain 150, got %d", cfg.MaLength)
	}
	if cfg.MaType != "SMA" {
		t.Errorf("MaType should remain SMA, got %s", cfg.MaType)
	}
	if cfg.TslPercent != 15.0 {
		t.Errorf("TslPercent should remain 15.0, got %f", cfg.TslPercent)
	}
}

// ============================================================
// Ditz Config Tests
// ============================================================

func TestAdminDitzConfig_CreateAndGet(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	putBody := map[string]interface{}{
		"short_l1":     7,
		"short_l2":     22,
		"short_l3":     16,
		"long_l1":      25,
		"long_l2":      18,
		"ma_filter_on": true,
		"ma_length":    150,
		"ma_type":      "SMA",
		"tsl_percent":  18.0,
		"tsl_enabled":  true,
	}
	w := adminPUT(r, token, "/api/admin/bxtrender-ditz-config", putBody)
	if w.Code != 200 {
		t.Fatalf("PUT expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = adminGET(r, token, "/api/admin/bxtrender-ditz-config")
	if w.Code != 200 {
		t.Fatalf("GET expected 200, got %d", w.Code)
	}

	var cfg BXtrenderDitzConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	if cfg.ShortL1 != 7 {
		t.Errorf("ShortL1: expected 7, got %d", cfg.ShortL1)
	}
	if cfg.ShortL2 != 22 {
		t.Errorf("ShortL2: expected 22, got %d", cfg.ShortL2)
	}
	if cfg.ShortL3 != 16 {
		t.Errorf("ShortL3: expected 16, got %d", cfg.ShortL3)
	}
	if cfg.LongL1 != 25 {
		t.Errorf("LongL1: expected 25, got %d", cfg.LongL1)
	}
	if cfg.LongL2 != 18 {
		t.Errorf("LongL2: expected 18, got %d", cfg.LongL2)
	}
	if !cfg.MaFilterOn {
		t.Error("MaFilterOn: expected true, got false")
	}
	if cfg.MaLength != 150 {
		t.Errorf("MaLength: expected 150, got %d", cfg.MaLength)
	}
	if cfg.MaType != "SMA" {
		t.Errorf("MaType: expected SMA, got %s", cfg.MaType)
	}
	if cfg.TslPercent != 18.0 {
		t.Errorf("TslPercent: expected 18.0, got %f", cfg.TslPercent)
	}
	if !cfg.TslEnabled {
		t.Error("TslEnabled: expected true, got false")
	}
}

func TestAdminDitzConfig_TslEnabledFalse(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	body := map[string]interface{}{
		"short_l1":     5,
		"short_l2":     20,
		"short_l3":     15,
		"long_l1":      20,
		"long_l2":      15,
		"ma_filter_on": true,
		"ma_length":    200,
		"ma_type":      "EMA",
		"tsl_percent":  20.0,
		"tsl_enabled":  false,
	}
	w := adminPUT(r, token, "/api/admin/bxtrender-ditz-config", body)
	if w.Code != 200 {
		t.Fatalf("PUT expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Re-read from DB via GET
	w = adminGET(r, token, "/api/admin/bxtrender-ditz-config")
	var cfg BXtrenderDitzConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	if cfg.TslEnabled {
		t.Error("CRITICAL BUG: Ditz tsl_enabled=false not persisted! Got true after re-read")
	}

	// Direct DB check
	var dbCfg BXtrenderDitzConfig
	db.First(&dbCfg)
	if dbCfg.TslEnabled {
		t.Error("CRITICAL BUG: Ditz tsl_enabled=false not stored in DB")
	}
}

func TestAdminDitzConfig_MaFilterOnFalse(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	body := map[string]interface{}{
		"short_l1":     5,
		"short_l2":     20,
		"short_l3":     15,
		"long_l1":      20,
		"long_l2":      15,
		"ma_filter_on": false,
		"ma_length":    200,
		"ma_type":      "EMA",
		"tsl_percent":  20.0,
		"tsl_enabled":  true,
	}
	w := adminPUT(r, token, "/api/admin/bxtrender-ditz-config", body)
	if w.Code != 200 {
		t.Fatalf("PUT expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Re-read from DB via GET
	w = adminGET(r, token, "/api/admin/bxtrender-ditz-config")
	var cfg BXtrenderDitzConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	if cfg.MaFilterOn {
		t.Error("CRITICAL BUG: Ditz ma_filter_on=false not persisted! Got true after re-read")
	}

	// Direct DB check
	var dbCfg BXtrenderDitzConfig
	db.First(&dbCfg)
	if dbCfg.MaFilterOn {
		t.Error("CRITICAL BUG: Ditz ma_filter_on=false not stored in DB")
	}
}

// ============================================================
// Quant Config Tests
// ============================================================

func TestAdminQuantConfig_CreateAndGet(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	putBody := map[string]interface{}{
		"short_l1":     6,
		"short_l2":     18,
		"short_l3":     12,
		"long_l1":      22,
		"long_l2":      16,
		"ma_filter_on": true,
		"ma_length":    100,
		"ma_type":      "SMA",
		"tsl_percent":  25.0,
		"tsl_enabled":  true,
	}
	w := adminPUT(r, token, "/api/admin/bxtrender-quant-config", putBody)
	if w.Code != 200 {
		t.Fatalf("PUT expected 200, got %d: %s", w.Code, w.Body.String())
	}

	w = adminGET(r, token, "/api/admin/bxtrender-quant-config")
	if w.Code != 200 {
		t.Fatalf("GET expected 200, got %d", w.Code)
	}

	var cfg BXtrenderQuantConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	if cfg.ShortL1 != 6 {
		t.Errorf("ShortL1: expected 6, got %d", cfg.ShortL1)
	}
	if cfg.ShortL2 != 18 {
		t.Errorf("ShortL2: expected 18, got %d", cfg.ShortL2)
	}
	if cfg.ShortL3 != 12 {
		t.Errorf("ShortL3: expected 12, got %d", cfg.ShortL3)
	}
	if cfg.LongL1 != 22 {
		t.Errorf("LongL1: expected 22, got %d", cfg.LongL1)
	}
	if cfg.LongL2 != 16 {
		t.Errorf("LongL2: expected 16, got %d", cfg.LongL2)
	}
	if !cfg.MaFilterOn {
		t.Error("MaFilterOn: expected true, got false")
	}
	if cfg.MaLength != 100 {
		t.Errorf("MaLength: expected 100, got %d", cfg.MaLength)
	}
	if cfg.MaType != "SMA" {
		t.Errorf("MaType: expected SMA, got %s", cfg.MaType)
	}
	if cfg.TslPercent != 25.0 {
		t.Errorf("TslPercent: expected 25.0, got %f", cfg.TslPercent)
	}
	if !cfg.TslEnabled {
		t.Error("TslEnabled: expected true, got false")
	}
}

func TestAdminQuantConfig_TslEnabledFalse(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	body := map[string]interface{}{
		"short_l1":     5,
		"short_l2":     20,
		"short_l3":     15,
		"long_l1":      20,
		"long_l2":      15,
		"ma_filter_on": true,
		"ma_length":    200,
		"ma_type":      "EMA",
		"tsl_percent":  20.0,
		"tsl_enabled":  false,
	}
	w := adminPUT(r, token, "/api/admin/bxtrender-quant-config", body)
	if w.Code != 200 {
		t.Fatalf("PUT expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Re-read from DB
	w = adminGET(r, token, "/api/admin/bxtrender-quant-config")
	var cfg BXtrenderQuantConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	if cfg.TslEnabled {
		t.Error("CRITICAL BUG: Quant tsl_enabled=false not persisted! Got true after re-read")
	}

	// Direct DB check
	var dbCfg BXtrenderQuantConfig
	db.First(&dbCfg)
	if dbCfg.TslEnabled {
		t.Error("CRITICAL BUG: Quant tsl_enabled=false not stored in DB")
	}
}

func TestAdminQuantConfig_MaFilterOnFalse(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	body := map[string]interface{}{
		"short_l1":     5,
		"short_l2":     20,
		"short_l3":     15,
		"long_l1":      20,
		"long_l2":      15,
		"ma_filter_on": false,
		"ma_length":    200,
		"ma_type":      "EMA",
		"tsl_percent":  20.0,
		"tsl_enabled":  true,
	}
	w := adminPUT(r, token, "/api/admin/bxtrender-quant-config", body)
	if w.Code != 200 {
		t.Fatalf("PUT expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Re-read from DB
	w = adminGET(r, token, "/api/admin/bxtrender-quant-config")
	var cfg BXtrenderQuantConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	if cfg.MaFilterOn {
		t.Error("CRITICAL BUG: Quant ma_filter_on=false not persisted! Got true after re-read")
	}

	// Direct DB check
	var dbCfg BXtrenderQuantConfig
	db.First(&dbCfg)
	if dbCfg.MaFilterOn {
		t.Error("CRITICAL BUG: Quant ma_filter_on=false not stored in DB")
	}
}

// ============================================================
// Defensive Config Tests (BXtrenderConfig with mode="defensive")
// ============================================================

func TestAdminDefensiveConfig_CreateAndGet(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	putBody := map[string]interface{}{
		"mode":        "defensive",
		"short_l1":    8,
		"short_l2":    22,
		"short_l3":    17,
		"long_l1":     25,
		"long_l2":     18,
		"tsl_percent": 15.0,
		"tsl_enabled": true,
	}
	w := adminPUT(r, token, "/api/admin/bxtrender-config", putBody)
	if w.Code != 200 {
		t.Fatalf("PUT expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// GET returns an array of configs (defensive + aggressive)
	w = adminGET(r, token, "/api/admin/bxtrender-config")
	if w.Code != 200 {
		t.Fatalf("GET expected 200, got %d", w.Code)
	}

	var configs []BXtrenderConfig
	json.Unmarshal(w.Body.Bytes(), &configs)

	// Find the defensive config
	var found bool
	for _, cfg := range configs {
		if cfg.Mode == "defensive" {
			found = true
			if cfg.ShortL1 != 8 {
				t.Errorf("ShortL1: expected 8, got %d", cfg.ShortL1)
			}
			if cfg.ShortL2 != 22 {
				t.Errorf("ShortL2: expected 22, got %d", cfg.ShortL2)
			}
			if cfg.ShortL3 != 17 {
				t.Errorf("ShortL3: expected 17, got %d", cfg.ShortL3)
			}
			if cfg.LongL1 != 25 {
				t.Errorf("LongL1: expected 25, got %d", cfg.LongL1)
			}
			if cfg.LongL2 != 18 {
				t.Errorf("LongL2: expected 18, got %d", cfg.LongL2)
			}
			if cfg.TslPercent != 15.0 {
				t.Errorf("TslPercent: expected 15.0, got %f", cfg.TslPercent)
			}
			if !cfg.TslEnabled {
				t.Error("TslEnabled: expected true, got false")
			}
		}
	}
	if !found {
		t.Error("defensive config not found in GET response")
	}
}

func TestAdminDefensiveConfig_TslEnabledFalse(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	body := map[string]interface{}{
		"mode":        "defensive",
		"short_l1":    5,
		"short_l2":    20,
		"short_l3":    15,
		"long_l1":     20,
		"long_l2":     15,
		"tsl_percent": 20.0,
		"tsl_enabled": false,
	}
	w := adminPUT(r, token, "/api/admin/bxtrender-config", body)
	if w.Code != 200 {
		t.Fatalf("PUT expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify via PUT response
	var putResp BXtrenderConfig
	json.Unmarshal(w.Body.Bytes(), &putResp)
	if putResp.TslEnabled {
		t.Error("PUT response: tsl_enabled should be false")
	}

	// Verify via GET
	w = adminGET(r, token, "/api/admin/bxtrender-config")
	var configs []BXtrenderConfig
	json.Unmarshal(w.Body.Bytes(), &configs)

	for _, cfg := range configs {
		if cfg.Mode == "defensive" {
			if cfg.TslEnabled {
				t.Error("CRITICAL BUG: defensive tsl_enabled=false not persisted! Got true after re-read")
			}
		}
	}

	// Direct DB check
	var dbCfg BXtrenderConfig
	db.Where("mode = ?", "defensive").First(&dbCfg)
	if dbCfg.TslEnabled {
		t.Error("CRITICAL BUG: defensive tsl_enabled=false not stored in DB")
	}
}

// ============================================================
// Aggressive Config Tests (BXtrenderConfig with mode="aggressive")
// ============================================================

func TestAdminAggressiveConfig_TslEnabledFalse(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	body := map[string]interface{}{
		"mode":        "aggressive",
		"short_l1":    5,
		"short_l2":    20,
		"short_l3":    15,
		"long_l1":     20,
		"long_l2":     15,
		"tsl_percent": 20.0,
		"tsl_enabled": false,
	}
	w := adminPUT(r, token, "/api/admin/bxtrender-config", body)
	if w.Code != 200 {
		t.Fatalf("PUT expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify PUT response
	var putResp BXtrenderConfig
	json.Unmarshal(w.Body.Bytes(), &putResp)
	if putResp.TslEnabled {
		t.Error("PUT response: aggressive tsl_enabled should be false")
	}

	// Verify via GET
	w = adminGET(r, token, "/api/admin/bxtrender-config")
	var configs []BXtrenderConfig
	json.Unmarshal(w.Body.Bytes(), &configs)

	for _, cfg := range configs {
		if cfg.Mode == "aggressive" {
			if cfg.TslEnabled {
				t.Error("CRITICAL BUG: aggressive tsl_enabled=false not persisted! Got true after re-read")
			}
		}
	}

	// Direct DB check
	var dbCfg BXtrenderConfig
	db.Where("mode = ?", "aggressive").First(&dbCfg)
	if dbCfg.TslEnabled {
		t.Error("CRITICAL BUG: aggressive tsl_enabled=false not stored in DB")
	}
}

// ============================================================
// Comprehensive Bool Zero-Value Test (all config types)
// ============================================================

func TestAdminAllConfigs_BoolFieldsSurviveZeroValue(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	// Step 1: Create all configs with all bools = true
	// Defensive
	w := adminPUT(r, token, "/api/admin/bxtrender-config", map[string]interface{}{
		"mode": "defensive", "short_l1": 5, "short_l2": 20, "short_l3": 15,
		"long_l1": 20, "long_l2": 15, "tsl_percent": 20.0, "tsl_enabled": true,
	})
	if w.Code != 200 {
		t.Fatalf("defensive create: %d - %s", w.Code, w.Body.String())
	}

	// Aggressive
	w = adminPUT(r, token, "/api/admin/bxtrender-config", map[string]interface{}{
		"mode": "aggressive", "short_l1": 5, "short_l2": 20, "short_l3": 15,
		"long_l1": 20, "long_l2": 15, "tsl_percent": 20.0, "tsl_enabled": true,
	})
	if w.Code != 200 {
		t.Fatalf("aggressive create: %d - %s", w.Code, w.Body.String())
	}

	// Quant
	w = adminPUT(r, token, "/api/admin/bxtrender-quant-config", map[string]interface{}{
		"short_l1": 5, "short_l2": 20, "short_l3": 15, "long_l1": 20, "long_l2": 15,
		"ma_filter_on": true, "ma_length": 200, "ma_type": "EMA",
		"tsl_percent": 20.0, "tsl_enabled": true,
	})
	if w.Code != 200 {
		t.Fatalf("quant create: %d - %s", w.Code, w.Body.String())
	}

	// Ditz
	w = adminPUT(r, token, "/api/admin/bxtrender-ditz-config", map[string]interface{}{
		"short_l1": 5, "short_l2": 20, "short_l3": 15, "long_l1": 20, "long_l2": 15,
		"ma_filter_on": true, "ma_length": 200, "ma_type": "EMA",
		"tsl_percent": 20.0, "tsl_enabled": true,
	})
	if w.Code != 200 {
		t.Fatalf("ditz create: %d - %s", w.Code, w.Body.String())
	}

	// Trader
	w = adminPUT(r, token, "/api/admin/bxtrender-trader-config", map[string]interface{}{
		"short_l1": 5, "short_l2": 20, "short_l3": 15, "long_l1": 20, "long_l2": 15,
		"ma_filter_on": true, "ma_length": 200, "ma_type": "EMA",
		"tsl_percent": 20.0, "tsl_enabled": true,
	})
	if w.Code != 200 {
		t.Fatalf("trader create: %d - %s", w.Code, w.Body.String())
	}

	// Step 2: Update ALL configs to have all bools = false
	w = adminPUT(r, token, "/api/admin/bxtrender-config", map[string]interface{}{
		"mode": "defensive", "short_l1": 5, "short_l2": 20, "short_l3": 15,
		"long_l1": 20, "long_l2": 15, "tsl_percent": 20.0, "tsl_enabled": false,
	})
	if w.Code != 200 {
		t.Fatalf("defensive update: %d", w.Code)
	}

	w = adminPUT(r, token, "/api/admin/bxtrender-config", map[string]interface{}{
		"mode": "aggressive", "short_l1": 5, "short_l2": 20, "short_l3": 15,
		"long_l1": 20, "long_l2": 15, "tsl_percent": 20.0, "tsl_enabled": false,
	})
	if w.Code != 200 {
		t.Fatalf("aggressive update: %d", w.Code)
	}

	w = adminPUT(r, token, "/api/admin/bxtrender-quant-config", map[string]interface{}{
		"short_l1": 5, "short_l2": 20, "short_l3": 15, "long_l1": 20, "long_l2": 15,
		"ma_filter_on": false, "ma_length": 200, "ma_type": "EMA",
		"tsl_percent": 20.0, "tsl_enabled": false,
	})
	if w.Code != 200 {
		t.Fatalf("quant update: %d", w.Code)
	}

	w = adminPUT(r, token, "/api/admin/bxtrender-ditz-config", map[string]interface{}{
		"short_l1": 5, "short_l2": 20, "short_l3": 15, "long_l1": 20, "long_l2": 15,
		"ma_filter_on": false, "ma_length": 200, "ma_type": "EMA",
		"tsl_percent": 20.0, "tsl_enabled": false,
	})
	if w.Code != 200 {
		t.Fatalf("ditz update: %d", w.Code)
	}

	w = adminPUT(r, token, "/api/admin/bxtrender-trader-config", map[string]interface{}{
		"short_l1": 5, "short_l2": 20, "short_l3": 15, "long_l1": 20, "long_l2": 15,
		"ma_filter_on": false, "ma_length": 200, "ma_type": "EMA",
		"tsl_percent": 20.0, "tsl_enabled": false,
	})
	if w.Code != 200 {
		t.Fatalf("trader update: %d", w.Code)
	}

	// Step 3: Re-read ALL configs and verify all bools are false
	// Defensive + Aggressive
	w = adminGET(r, token, "/api/admin/bxtrender-config")
	var bxConfigs []BXtrenderConfig
	json.Unmarshal(w.Body.Bytes(), &bxConfigs)
	for _, cfg := range bxConfigs {
		if cfg.TslEnabled {
			t.Errorf("%s: tsl_enabled should be false after update, got true", cfg.Mode)
		}
	}

	// Quant
	w = adminGET(r, token, "/api/admin/bxtrender-quant-config")
	var quantCfg BXtrenderQuantConfig
	json.Unmarshal(w.Body.Bytes(), &quantCfg)
	if quantCfg.TslEnabled {
		t.Error("Quant tsl_enabled should be false")
	}
	if quantCfg.MaFilterOn {
		t.Error("Quant ma_filter_on should be false")
	}

	// Ditz
	w = adminGET(r, token, "/api/admin/bxtrender-ditz-config")
	var ditzCfg BXtrenderDitzConfig
	json.Unmarshal(w.Body.Bytes(), &ditzCfg)
	if ditzCfg.TslEnabled {
		t.Error("Ditz tsl_enabled should be false")
	}
	if ditzCfg.MaFilterOn {
		t.Error("Ditz ma_filter_on should be false")
	}

	// Trader
	w = adminGET(r, token, "/api/admin/bxtrender-trader-config")
	var traderCfg BXtrenderTraderConfig
	json.Unmarshal(w.Body.Bytes(), &traderCfg)
	if traderCfg.TslEnabled {
		t.Error("Trader tsl_enabled should be false")
	}
	if traderCfg.MaFilterOn {
		t.Error("Trader ma_filter_on should be false")
	}

	// Step 4: Also verify directly in DB to catch any caching issues
	var dbDefensive BXtrenderConfig
	db.Where("mode = ?", "defensive").First(&dbDefensive)
	if dbDefensive.TslEnabled {
		t.Error("DB: defensive tsl_enabled should be false")
	}

	var dbAggressive BXtrenderConfig
	db.Where("mode = ?", "aggressive").First(&dbAggressive)
	if dbAggressive.TslEnabled {
		t.Error("DB: aggressive tsl_enabled should be false")
	}

	var dbQuant BXtrenderQuantConfig
	db.First(&dbQuant)
	if dbQuant.TslEnabled {
		t.Error("DB: quant tsl_enabled should be false")
	}
	if dbQuant.MaFilterOn {
		t.Error("DB: quant ma_filter_on should be false")
	}

	var dbDitz BXtrenderDitzConfig
	db.First(&dbDitz)
	if dbDitz.TslEnabled {
		t.Error("DB: ditz tsl_enabled should be false")
	}
	if dbDitz.MaFilterOn {
		t.Error("DB: ditz ma_filter_on should be false")
	}

	var dbTrader BXtrenderTraderConfig
	db.First(&dbTrader)
	if dbTrader.TslEnabled {
		t.Error("DB: trader tsl_enabled should be false")
	}
	if dbTrader.MaFilterOn {
		t.Error("DB: trader ma_filter_on should be false")
	}
}

// ============================================================
// Defensive Config: Update existing test (true -> false -> true for TSL)
// ============================================================

func TestAdminDefensiveConfig_TslEnabledToggle(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	base := map[string]interface{}{
		"mode":        "defensive",
		"short_l1":    5,
		"short_l2":    20,
		"short_l3":    15,
		"long_l1":     20,
		"long_l2":     15,
		"tsl_percent": 20.0,
	}

	// Step 1: true
	base["tsl_enabled"] = true
	w := adminPUT(r, token, "/api/admin/bxtrender-config", base)
	if w.Code != 200 {
		t.Fatalf("step1: %d", w.Code)
	}

	var dbCfg BXtrenderConfig
	db.Where("mode = ?", "defensive").First(&dbCfg)
	if !dbCfg.TslEnabled {
		t.Error("step1: tsl_enabled should be true")
	}

	// Step 2: false
	base["tsl_enabled"] = false
	w = adminPUT(r, token, "/api/admin/bxtrender-config", base)
	if w.Code != 200 {
		t.Fatalf("step2: %d", w.Code)
	}

	db.Where("mode = ?", "defensive").First(&dbCfg)
	if dbCfg.TslEnabled {
		t.Error("step2: tsl_enabled should be false")
	}

	// Step 3: true again
	base["tsl_enabled"] = true
	w = adminPUT(r, token, "/api/admin/bxtrender-config", base)
	if w.Code != 200 {
		t.Fatalf("step3: %d", w.Code)
	}

	db.Where("mode = ?", "defensive").First(&dbCfg)
	if !dbCfg.TslEnabled {
		t.Error("step3: tsl_enabled should be true")
	}
}

// ============================================================
// Quant Config: TslEnabled toggle roundtrip
// ============================================================

func TestAdminQuantConfig_TslEnabledToggle(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	base := map[string]interface{}{
		"short_l1": 5, "short_l2": 20, "short_l3": 15, "long_l1": 20, "long_l2": 15,
		"ma_filter_on": true, "ma_length": 200, "ma_type": "EMA", "tsl_percent": 20.0,
	}

	// true
	base["tsl_enabled"] = true
	w := adminPUT(r, token, "/api/admin/bxtrender-quant-config", base)
	if w.Code != 200 {
		t.Fatalf("step1: %d", w.Code)
	}
	var cfg BXtrenderQuantConfig
	db.First(&cfg)
	if !cfg.TslEnabled {
		t.Error("step1: tsl_enabled should be true")
	}

	// false
	base["tsl_enabled"] = false
	w = adminPUT(r, token, "/api/admin/bxtrender-quant-config", base)
	if w.Code != 200 {
		t.Fatalf("step2: %d", w.Code)
	}
	db.First(&cfg)
	if cfg.TslEnabled {
		t.Error("step2: tsl_enabled should be false")
	}

	// true again
	base["tsl_enabled"] = true
	w = adminPUT(r, token, "/api/admin/bxtrender-quant-config", base)
	if w.Code != 200 {
		t.Fatalf("step3: %d", w.Code)
	}
	db.First(&cfg)
	if !cfg.TslEnabled {
		t.Error("step3: tsl_enabled should be true")
	}
}

// ============================================================
// Ditz Config: Both bool fields toggle roundtrip
// ============================================================

func TestAdminDitzConfig_BoolToggleRoundtrip(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	base := map[string]interface{}{
		"short_l1": 5, "short_l2": 20, "short_l3": 15, "long_l1": 20, "long_l2": 15,
		"ma_length": 200, "ma_type": "EMA", "tsl_percent": 20.0,
	}

	// Both true
	base["tsl_enabled"] = true
	base["ma_filter_on"] = true
	w := adminPUT(r, token, "/api/admin/bxtrender-ditz-config", base)
	if w.Code != 200 {
		t.Fatalf("step1: %d", w.Code)
	}
	var cfg BXtrenderDitzConfig
	db.First(&cfg)
	if !cfg.TslEnabled || !cfg.MaFilterOn {
		t.Errorf("step1: expected both true, got tsl=%v ma=%v", cfg.TslEnabled, cfg.MaFilterOn)
	}

	// Both false
	base["tsl_enabled"] = false
	base["ma_filter_on"] = false
	w = adminPUT(r, token, "/api/admin/bxtrender-ditz-config", base)
	if w.Code != 200 {
		t.Fatalf("step2: %d", w.Code)
	}
	db.First(&cfg)
	if cfg.TslEnabled || cfg.MaFilterOn {
		t.Errorf("step2: expected both false, got tsl=%v ma=%v", cfg.TslEnabled, cfg.MaFilterOn)
	}

	// Mixed: tsl=true, ma=false
	base["tsl_enabled"] = true
	base["ma_filter_on"] = false
	w = adminPUT(r, token, "/api/admin/bxtrender-ditz-config", base)
	if w.Code != 200 {
		t.Fatalf("step3: %d", w.Code)
	}
	db.First(&cfg)
	if !cfg.TslEnabled {
		t.Error("step3: tsl_enabled should be true")
	}
	if cfg.MaFilterOn {
		t.Error("step3: ma_filter_on should be false")
	}

	// Mixed: tsl=false, ma=true
	base["tsl_enabled"] = false
	base["ma_filter_on"] = true
	w = adminPUT(r, token, "/api/admin/bxtrender-ditz-config", base)
	if w.Code != 200 {
		t.Fatalf("step4: %d", w.Code)
	}
	db.First(&cfg)
	if cfg.TslEnabled {
		t.Error("step4: tsl_enabled should be false")
	}
	if !cfg.MaFilterOn {
		t.Error("step4: ma_filter_on should be true")
	}
}

// ============================================================
// Public Endpoint Tests
// ============================================================

func TestAdminPublicEndpoint_TraderConfig(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	// Create config via admin PUT
	adminPUT(r, token, "/api/admin/bxtrender-trader-config", map[string]interface{}{
		"short_l1": 8, "short_l2": 25, "short_l3": 18, "long_l1": 30, "long_l2": 20,
		"ma_filter_on": true, "ma_length": 150, "ma_type": "SMA",
		"tsl_percent": 15.0, "tsl_enabled": false,
	})

	// Public GET (no auth)
	req, _ := http.NewRequest("GET", "/api/bxtrender-trader-config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("public GET expected 200, got %d", w.Code)
	}

	var cfg BXtrenderTraderConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	if cfg.ShortL1 != 8 {
		t.Errorf("public: ShortL1 expected 8, got %d", cfg.ShortL1)
	}
	if cfg.TslEnabled {
		t.Error("public: tsl_enabled should be false (matches admin-saved value)")
	}
	if !cfg.MaFilterOn {
		t.Error("public: ma_filter_on should be true")
	}
}

func TestAdminPublicEndpoint_QuantConfig(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	// Create config via admin PUT with tsl_enabled=false and ma_filter_on=false
	adminPUT(r, token, "/api/admin/bxtrender-quant-config", map[string]interface{}{
		"short_l1": 5, "short_l2": 20, "short_l3": 15, "long_l1": 20, "long_l2": 15,
		"ma_filter_on": false, "ma_length": 200, "ma_type": "EMA",
		"tsl_percent": 20.0, "tsl_enabled": false,
	})

	// Public GET
	req, _ := http.NewRequest("GET", "/api/bxtrender-quant-config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("public GET expected 200, got %d", w.Code)
	}

	var cfg BXtrenderQuantConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	if cfg.TslEnabled {
		t.Error("public quant: tsl_enabled should be false")
	}
	if cfg.MaFilterOn {
		t.Error("public quant: ma_filter_on should be false")
	}
}

func TestAdminPublicEndpoint_DitzConfig(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	adminPUT(r, token, "/api/admin/bxtrender-ditz-config", map[string]interface{}{
		"short_l1": 5, "short_l2": 20, "short_l3": 15, "long_l1": 20, "long_l2": 15,
		"ma_filter_on": false, "ma_length": 200, "ma_type": "EMA",
		"tsl_percent": 20.0, "tsl_enabled": false,
	})

	req, _ := http.NewRequest("GET", "/api/bxtrender-ditz-config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("public GET expected 200, got %d", w.Code)
	}

	var cfg BXtrenderDitzConfig
	json.Unmarshal(w.Body.Bytes(), &cfg)

	if cfg.TslEnabled {
		t.Error("public ditz: tsl_enabled should be false")
	}
	if cfg.MaFilterOn {
		t.Error("public ditz: ma_filter_on should be false")
	}
}

func TestAdminPublicEndpoint_BXtrenderConfigMap(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	// Create defensive with tsl_enabled=false
	adminPUT(r, token, "/api/admin/bxtrender-config", map[string]interface{}{
		"mode": "defensive", "short_l1": 5, "short_l2": 20, "short_l3": 15,
		"long_l1": 20, "long_l2": 15, "tsl_percent": 20.0, "tsl_enabled": false,
	})

	// Create aggressive with tsl_enabled=true
	adminPUT(r, token, "/api/admin/bxtrender-config", map[string]interface{}{
		"mode": "aggressive", "short_l1": 5, "short_l2": 20, "short_l3": 15,
		"long_l1": 20, "long_l2": 15, "tsl_percent": 15.0, "tsl_enabled": true,
	})

	// Public endpoint returns a map keyed by mode
	req, _ := http.NewRequest("GET", "/api/bxtrender-config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("public GET expected 200, got %d", w.Code)
	}

	var result map[string]BXtrenderConfig
	json.Unmarshal(w.Body.Bytes(), &result)

	def, ok := result["defensive"]
	if !ok {
		t.Fatal("defensive not found in public config map")
	}
	if def.TslEnabled {
		t.Error("public: defensive tsl_enabled should be false")
	}

	agg, ok := result["aggressive"]
	if !ok {
		t.Fatal("aggressive not found in public config map")
	}
	if !agg.TslEnabled {
		t.Error("public: aggressive tsl_enabled should be true")
	}
	if agg.TslPercent != 15.0 {
		t.Errorf("public: aggressive tsl_percent expected 15.0, got %f", agg.TslPercent)
	}
}

// ============================================================
// Auth/Permission Tests
// ============================================================

func TestAdminConfigEndpoints_RequireAuth(t *testing.T) {
	setupTestDB(t)
	r, _ := setupAdminConfigRouter(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/admin/bxtrender-config"},
		{"PUT", "/api/admin/bxtrender-config"},
		{"GET", "/api/admin/bxtrender-quant-config"},
		{"PUT", "/api/admin/bxtrender-quant-config"},
		{"GET", "/api/admin/bxtrender-ditz-config"},
		{"PUT", "/api/admin/bxtrender-ditz-config"},
		{"GET", "/api/admin/bxtrender-trader-config"},
		{"PUT", "/api/admin/bxtrender-trader-config"},
	}

	for _, ep := range endpoints {
		var req *http.Request
		if ep.method == "PUT" {
			body, _ := json.Marshal(map[string]interface{}{"mode": "defensive"})
			req, _ = http.NewRequest(ep.method, ep.path, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(ep.method, ep.path, nil)
		}
		// No Authorization header
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != 401 {
			t.Errorf("%s %s: expected 401 without auth, got %d", ep.method, ep.path, w.Code)
		}
	}
}

// ============================================================
// Idempotent Config Record Test (no duplicate rows)
// ============================================================

func TestAdminTraderConfig_NoDuplicateRows(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	body := map[string]interface{}{
		"short_l1": 5, "short_l2": 20, "short_l3": 15, "long_l1": 20, "long_l2": 15,
		"ma_filter_on": true, "ma_length": 200, "ma_type": "EMA",
		"tsl_percent": 20.0, "tsl_enabled": true,
	}

	// PUT 5 times
	for i := 0; i < 5; i++ {
		w := adminPUT(r, token, "/api/admin/bxtrender-trader-config", body)
		if w.Code != 200 {
			t.Fatalf("PUT #%d: expected 200, got %d", i+1, w.Code)
		}
	}

	// Should only be 1 row in DB
	var count int64
	db.Model(&BXtrenderTraderConfig{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 trader config row, got %d", count)
	}
}

func TestAdminQuantConfig_NoDuplicateRows(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	body := map[string]interface{}{
		"short_l1": 5, "short_l2": 20, "short_l3": 15, "long_l1": 20, "long_l2": 15,
		"ma_filter_on": true, "ma_length": 200, "ma_type": "EMA",
		"tsl_percent": 20.0, "tsl_enabled": true,
	}

	for i := 0; i < 5; i++ {
		w := adminPUT(r, token, "/api/admin/bxtrender-quant-config", body)
		if w.Code != 200 {
			t.Fatalf("PUT #%d: expected 200, got %d", i+1, w.Code)
		}
	}

	var count int64
	db.Model(&BXtrenderQuantConfig{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 quant config row, got %d", count)
	}
}

func TestAdminDitzConfig_NoDuplicateRows(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	body := map[string]interface{}{
		"short_l1": 5, "short_l2": 20, "short_l3": 15, "long_l1": 20, "long_l2": 15,
		"ma_filter_on": true, "ma_length": 200, "ma_type": "EMA",
		"tsl_percent": 20.0, "tsl_enabled": true,
	}

	for i := 0; i < 5; i++ {
		w := adminPUT(r, token, "/api/admin/bxtrender-ditz-config", body)
		if w.Code != 200 {
			t.Fatalf("PUT #%d: expected 200, got %d", i+1, w.Code)
		}
	}

	var count int64
	db.Model(&BXtrenderDitzConfig{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 ditz config row, got %d", count)
	}
}

func TestAdminDefensiveConfig_NoDuplicateRows(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminConfigRouter(t)

	body := map[string]interface{}{
		"mode": "defensive", "short_l1": 5, "short_l2": 20, "short_l3": 15,
		"long_l1": 20, "long_l2": 15, "tsl_percent": 20.0, "tsl_enabled": true,
	}

	for i := 0; i < 5; i++ {
		w := adminPUT(r, token, "/api/admin/bxtrender-config", body)
		if w.Code != 200 {
			t.Fatalf("PUT #%d: expected 200, got %d", i+1, w.Code)
		}
	}

	var count int64
	db.Model(&BXtrenderConfig{}).Where("mode = ?", "defensive").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 defensive config row, got %d", count)
	}
}
