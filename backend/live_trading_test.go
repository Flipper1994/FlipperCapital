package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupLiveTestDB(t *testing.T) {
	t.Helper()

	// Stop any running schedulers from previous test
	liveSchedulerMu.Lock()
	for id, state := range liveSchedulers {
		close(state.StopChan)
		delete(liveSchedulers, id)
	}
	liveSchedulerMu.Unlock()
	time.Sleep(10 * time.Millisecond)

	var err error
	db, err = gorm.Open(sqlite.Open(fmt.Sprintf("file:memdb_%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	db.AutoMigrate(
		&User{}, &DBSession{}, &LiveTradingConfig{}, &LiveTradingSession{}, &LiveTradingPosition{},
		&TradingWatchlistItem{}, &TradingVirtualPosition{},
	)
}

func setupLiveRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	admin := User{Email: "admin@test.com", Username: "admin", Password: "hashed", IsAdmin: true}
	db.Create(&admin)

	token := "test-live-token"
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
	api.POST("/trading/live/config", authMiddleware(), adminOnly(), saveLiveTradingConfig)
	api.GET("/trading/live/config", authMiddleware(), adminOnly(), getLiveTradingConfig)
	api.POST("/trading/live/start", authMiddleware(), adminOnly(), startLiveTrading)
	api.POST("/trading/live/stop", authMiddleware(), adminOnly(), stopLiveTrading)
	api.GET("/trading/live/status", authMiddleware(), adminOnly(), getLiveTradingStatus)
	api.GET("/trading/live/sessions", authMiddleware(), adminOnly(), getLiveTradingSessions)
	api.GET("/trading/live/session/:id", authMiddleware(), adminOnly(), getLiveTradingSession)

	return r, token
}

func postJSON(r *gin.Engine, path, token string, body interface{}) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", path, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func getJSON(r *gin.Engine, path, token string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest("GET", path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ============ Config Tests ============

func TestSaveLiveTradingConfig(t *testing.T) {
	setupLiveTestDB(t)
	r, token := setupLiveRouter(t)

	// First save
	body := map[string]interface{}{
		"strategy":    "hybrid_ai_trend",
		"interval":    "5m",
		"params":      map[string]interface{}{"bb1_period": 20},
		"symbols":     []string{"AAPL", "MSFT"},
		"long_only":   true,
		"trade_amount": 500,
		"filters":     map[string]interface{}{"minWinRate": "50"},
		"currency":    "EUR",
	}
	w := postJSON(r, "/api/trading/live/config", token, body)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["strategy"] != "hybrid_ai_trend" {
		t.Fatalf("expected hybrid_ai_trend, got %v", resp["strategy"])
	}

	// Update (upsert) — should not create duplicate
	body["interval"] = "15m"
	w = postJSON(r, "/api/trading/live/config", token, body)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var count int64
	db.Model(&LiveTradingConfig{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 config, got %d", count)
	}

	var config LiveTradingConfig
	db.First(&config)
	if config.Interval != "15m" {
		t.Fatalf("expected 15m, got %s", config.Interval)
	}
}

func TestGetLiveTradingConfig(t *testing.T) {
	setupLiveTestDB(t)
	r, token := setupLiveRouter(t)

	// No config yet
	w := getJSON(r, "/api/trading/live/config", token)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Save a config
	body := map[string]interface{}{
		"strategy":    "regression_scalping",
		"interval":    "5m",
		"params":      map[string]interface{}{},
		"symbols":     []string{"TSLA"},
		"long_only":   false,
		"trade_amount": 200,
		"currency":    "USD",
	}
	postJSON(r, "/api/trading/live/config", token, body)

	// Get it back
	w = getJSON(r, "/api/trading/live/config", token)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["strategy"] != "regression_scalping" {
		t.Fatalf("expected regression_scalping, got %v", resp["strategy"])
	}
	symbols := resp["symbols"].([]interface{})
	if len(symbols) != 1 || symbols[0] != "TSLA" {
		t.Fatalf("expected [TSLA], got %v", symbols)
	}
}

// ============ Session Tests ============

func TestStartLiveSession(t *testing.T) {
	setupLiveTestDB(t)
	r, token := setupLiveRouter(t)

	// Save config first
	postJSON(r, "/api/trading/live/config", token, map[string]interface{}{
		"strategy": "hybrid_ai_trend", "interval": "5m",
		"symbols": []string{"AAPL"}, "trade_amount": 500, "currency": "EUR",
	})

	// Start session
	w := postJSON(r, "/api/trading/live/start", token, nil)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify via response (not direct DB query to avoid connection contention)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	sessionData := resp["session"].(map[string]interface{})
	if sessionData["is_active"] != true {
		t.Fatal("expected active session")
	}
	if sessionData["strategy"] != "hybrid_ai_trend" {
		t.Fatalf("expected hybrid_ai_trend, got %v", sessionData["strategy"])
	}

	// Stop the scheduler so goroutine doesn't leak
	liveSchedulerMu.Lock()
	for id, state := range liveSchedulers {
		close(state.StopChan)
		delete(liveSchedulers, id)
	}
	liveSchedulerMu.Unlock()
}

func TestStartSessionMultipleAdmin(t *testing.T) {
	setupLiveTestDB(t)
	r, token := setupLiveRouter(t)

	postJSON(r, "/api/trading/live/config", token, map[string]interface{}{
		"strategy": "hybrid_ai_trend", "interval": "5m",
		"symbols": []string{"AAPL"}, "trade_amount": 500, "currency": "EUR",
	})

	// Start first session
	w1 := postJSON(r, "/api/trading/live/start", token, nil)
	if w1.Code != 200 {
		t.Fatalf("first start failed: %d", w1.Code)
	}

	// Admin can start multiple sessions
	w2 := postJSON(r, "/api/trading/live/start", token, nil)
	if w2.Code != 200 {
		t.Fatalf("admin should be able to start multiple sessions, got %d", w2.Code)
	}

	// Cleanup
	liveSchedulerMu.Lock()
	for id, state := range liveSchedulers {
		close(state.StopChan)
		delete(liveSchedulers, id)
	}
	liveSchedulerMu.Unlock()
}

func TestStopLiveSession(t *testing.T) {
	setupLiveTestDB(t)
	r, token := setupLiveRouter(t)

	postJSON(r, "/api/trading/live/config", token, map[string]interface{}{
		"strategy": "hybrid_ai_trend", "interval": "5m",
		"symbols": []string{"AAPL"}, "trade_amount": 500, "currency": "EUR",
	})
	postJSON(r, "/api/trading/live/start", token, nil)

	// Stop
	w := postJSON(r, "/api/trading/live/stop", token, nil)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var session LiveTradingSession
	db.First(&session)
	if session.IsActive {
		t.Fatal("expected session to be inactive")
	}
	if session.StoppedAt == nil {
		t.Fatal("expected stopped_at to be set")
	}
}

func TestStopClosesOpenPositions(t *testing.T) {
	setupLiveTestDB(t)
	r, token := setupLiveRouter(t)

	postJSON(r, "/api/trading/live/config", token, map[string]interface{}{
		"strategy": "hybrid_ai_trend", "interval": "5m",
		"symbols": []string{"AAPL"}, "trade_amount": 500, "currency": "EUR",
	})
	postJSON(r, "/api/trading/live/start", token, nil)

	var session LiveTradingSession
	db.Where("is_active = ?", true).First(&session)

	// Create an open position
	pos := LiveTradingPosition{
		SessionID: session.ID, Symbol: "AAPL", Direction: "LONG",
		EntryPrice: 100, EntryPriceUSD: 100, CurrentPrice: 105,
		StopLoss: 95, TakeProfit: 115, InvestedAmount: 500,
		NativeCurrency: "USD", EntryTime: time.Now(),
	}
	db.Create(&pos)

	// Stop session
	postJSON(r, "/api/trading/live/stop", token, nil)

	var closedPos LiveTradingPosition
	db.First(&closedPos, pos.ID)
	if !closedPos.IsClosed {
		t.Fatal("expected position to be closed")
	}
	if closedPos.CloseReason != "MANUAL" {
		t.Fatalf("expected MANUAL, got %s", closedPos.CloseReason)
	}
	if math.Abs(closedPos.ProfitLossPct-5.0) > 0.1 {
		t.Fatalf("expected ~5%% P&L, got %.2f%%", closedPos.ProfitLossPct)
	}
}

// ============ P&L Calculation Tests ============

func TestPnLCalculationLong(t *testing.T) {
	pos := &LiveTradingPosition{
		Direction: "LONG", EntryPrice: 100, InvestedAmount: 500, NativeCurrency: "USD",
	}
	closeLivePosition(pos, 110, "TP", "USD")
	if math.Abs(pos.ProfitLossPct-10.0) > 0.01 {
		t.Fatalf("expected 10%%, got %.2f%%", pos.ProfitLossPct)
	}
	if math.Abs(pos.ProfitLossAmt-50.0) > 0.01 {
		t.Fatalf("expected 50 EUR, got %.2f", pos.ProfitLossAmt)
	}
}

func TestPnLCalculationShort(t *testing.T) {
	pos := &LiveTradingPosition{
		Direction: "SHORT", EntryPrice: 100, InvestedAmount: 500, NativeCurrency: "USD",
	}
	closeLivePosition(pos, 90, "TP", "USD")
	if math.Abs(pos.ProfitLossPct-10.0) > 0.01 {
		t.Fatalf("expected 10%%, got %.2f%%", pos.ProfitLossPct)
	}
	if math.Abs(pos.ProfitLossAmt-50.0) > 0.01 {
		t.Fatalf("expected 50 EUR, got %.2f", pos.ProfitLossAmt)
	}
}

func TestPnLCalculationLoss(t *testing.T) {
	pos := &LiveTradingPosition{
		Direction: "LONG", EntryPrice: 100, InvestedAmount: 500, NativeCurrency: "USD",
	}
	closeLivePosition(pos, 95, "SL", "USD")
	if math.Abs(pos.ProfitLossPct-(-5.0)) > 0.01 {
		t.Fatalf("expected -5%%, got %.2f%%", pos.ProfitLossPct)
	}
	if math.Abs(pos.ProfitLossAmt-(-25.0)) > 0.01 {
		t.Fatalf("expected -25, got %.2f", pos.ProfitLossAmt)
	}
}

// ============ Strategy Factory Tests ============

func TestCreateStrategyFromJSON(t *testing.T) {
	s := createStrategyFromJSON("hybrid_ai_trend", `{"bb1_period":20,"bb1_stdev":3.0}`)
	if s == nil {
		t.Fatal("expected strategy, got nil")
	}

	s2 := createStrategyFromJSON("regression_scalping", `{}`)
	if s2 == nil {
		t.Fatal("expected regression_scalping strategy")
	}

	s3 := createStrategyFromJSON("diamond_signals", `{}`)
	if s3 == nil {
		t.Fatal("expected diamond_signals strategy")
	}

	s4 := createStrategyFromJSON("unknown", `{}`)
	if s4 != nil {
		t.Fatal("expected nil for unknown strategy")
	}
}

// ============ Interval Tests ============

func TestIntervalToDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"5m", 5 * time.Minute},
		{"15m", 15 * time.Minute},
		{"1h", 1 * time.Hour},
		{"60m", 1 * time.Hour},
		{"2h", 2 * time.Hour},
		{"4h", 4 * time.Hour},
		{"1d", 24 * time.Hour},
		{"1D", 24 * time.Hour},
		{"1wk", 7 * 24 * time.Hour},
		{"1W", 7 * 24 * time.Hour},
		{"unknown", 5 * time.Minute},
	}
	for _, tt := range tests {
		got := intervalToDuration(tt.input)
		if got != tt.expected {
			t.Errorf("intervalToDuration(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

// ============ Currency Detection Tests ============

func TestCurrencyDetection(t *testing.T) {
	tests := []struct {
		symbol   string
		expected string
	}{
		{"AAPL", "USD"},
		{"SAP.DE", "EUR"},
		{"BP.L", "GBP"},
		{"NESN.SW", "CHF"},
		{"9988.HK", "HKD"},
		{"7203.T", "JPY"},
	}
	for _, tt := range tests {
		got := getStockCurrency(tt.symbol)
		if got != tt.expected {
			t.Errorf("getStockCurrency(%q) = %q, want %q", tt.symbol, got, tt.expected)
		}
	}
}

// ============ Live Status API Tests ============

func TestGetLiveStatus(t *testing.T) {
	setupLiveTestDB(t)
	r, token := setupLiveRouter(t)

	// No active session
	w := getJSON(r, "/api/trading/live/status", token)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["is_running"] != false {
		t.Fatal("expected is_running=false")
	}

	// Start session
	postJSON(r, "/api/trading/live/config", token, map[string]interface{}{
		"strategy": "hybrid_ai_trend", "interval": "15m",
		"symbols": []string{"AAPL", "MSFT", "GOOG"}, "trade_amount": 500, "currency": "EUR",
	})
	postJSON(r, "/api/trading/live/start", token, nil)

	w = getJSON(r, "/api/trading/live/status", token)
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["is_running"] != true {
		t.Fatal("expected is_running=true")
	}
	if resp["interval"] != "15m" {
		t.Fatalf("expected 15m, got %v", resp["interval"])
	}
	if resp["symbols_count"].(float64) != 3 {
		t.Fatalf("expected 3 symbols, got %v", resp["symbols_count"])
	}

	// Cleanup
	liveSchedulerMu.Lock()
	for id, state := range liveSchedulers {
		close(state.StopChan)
		delete(liveSchedulers, id)
	}
	liveSchedulerMu.Unlock()
}

// ============ Session History Tests ============

func TestGetSessionHistory(t *testing.T) {
	setupLiveTestDB(t)
	r, token := setupLiveRouter(t)

	postJSON(r, "/api/trading/live/config", token, map[string]interface{}{
		"strategy": "hybrid_ai_trend", "interval": "5m",
		"symbols": []string{"AAPL"}, "trade_amount": 500, "currency": "EUR",
	})

	// Start and stop two sessions
	postJSON(r, "/api/trading/live/start", token, nil)
	postJSON(r, "/api/trading/live/stop", token, nil)
	time.Sleep(10 * time.Millisecond)
	postJSON(r, "/api/trading/live/start", token, nil)
	postJSON(r, "/api/trading/live/stop", token, nil)

	w := getJSON(r, "/api/trading/live/sessions", token)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	sessions := resp["sessions"].([]interface{})
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	// First in list should be most recent (DESC order)
	s1 := sessions[0].(map[string]interface{})
	s2 := sessions[1].(map[string]interface{})
	if s1["id"].(float64) <= s2["id"].(float64) {
		t.Fatal("expected sessions in DESC order")
	}
}

// ============ Session Detail Tests ============

func TestGetSessionDetail(t *testing.T) {
	setupLiveTestDB(t)
	r, token := setupLiveRouter(t)

	postJSON(r, "/api/trading/live/config", token, map[string]interface{}{
		"strategy": "hybrid_ai_trend", "interval": "5m",
		"symbols": []string{"AAPL"}, "trade_amount": 500, "currency": "EUR",
	})
	w := postJSON(r, "/api/trading/live/start", token, nil)
	if w.Code != 200 {
		t.Fatalf("start failed: %d %s", w.Code, w.Body.String())
	}

	// Parse session ID from response
	var startResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &startResp)
	sessionData := startResp["session"].(map[string]interface{})
	sessionID := int(sessionData["id"].(float64))

	// Create a position directly in DB
	db.Create(&LiveTradingPosition{
		SessionID: uint(sessionID), Symbol: "AAPL", Direction: "LONG",
		EntryPrice: 150, InvestedAmount: 500, NativeCurrency: "USD",
		EntryTime: time.Now(),
	})

	postJSON(r, "/api/trading/live/stop", token, nil)

	w2 := getJSON(r, fmt.Sprintf("/api/trading/live/session/%d", sessionID), token)
	if w2.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp)
	positions := resp["positions"].([]interface{})
	if len(positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(positions))
	}
}

// ============ Start Without Config Tests ============

func TestStartSessionRequiresConfig(t *testing.T) {
	setupLiveTestDB(t)
	r, token := setupLiveRouter(t)

	w := postJSON(r, "/api/trading/live/start", token, nil)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
