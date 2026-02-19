package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ============================================================
// Setup: shared DB + router with all multi-strategy endpoints
// ============================================================

func setupMultiStratDB(t *testing.T) {
	t.Helper()
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
		&User{}, &DBSession{}, &LiveTradingConfig{}, &LiveTradingSession{},
		&LiveTradingPosition{}, &LiveTradingLog{}, &LiveSessionStrategy{},
		&TradingWatchlistItem{}, &TradingVirtualPosition{},
		&OHLCVCache{}, &Stock{},
	)
}

func setupMultiStratRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	admin := User{Email: "admin@test.com", Username: "admin", Password: "hashed", IsAdmin: true}
	db.Create(&admin)

	token := "test-ms-token-" + t.Name()
	dbSess := DBSession{Token: token, UserID: admin.ID, IsAdmin: true, Expiry: time.Now().Add(1 * time.Hour)}
	db.Create(&dbSess)
	sessions[token] = Session{UserID: admin.ID, IsAdmin: true, Expiry: dbSess.Expiry}

	r := gin.New()
	api := r.Group("/api")
	api.POST("/trading/live/config", authMiddleware(), adminOnly(), saveLiveTradingConfig)
	api.POST("/trading/live/start", authMiddleware(), adminOnly(), startLiveTrading)
	api.POST("/trading/live/stop", authMiddleware(), adminOnly(), stopLiveTrading)
	api.GET("/trading/live/sessions", authMiddleware(), getLiveTradingSessions)
	api.GET("/trading/live/session/:id", authMiddleware(), getLiveTradingSession)
	api.POST("/trading/live/session/:id/resume", authMiddleware(), adminOnly(), resumeLiveTrading)
	api.GET("/trading/live/session/:id/strategies", authMiddleware(), getLiveSessionStrategies)
	api.POST("/trading/live/session/:id/strategy", authMiddleware(), adminOnly(), addLiveSessionStrategy)
	api.PUT("/trading/live/session/:id/strategy/:strategyId", authMiddleware(), adminOnly(), toggleLiveSessionStrategy)
	api.GET("/trading/live/logs/:sessionId", authMiddleware(), getLiveTradingLogs)

	return r, token
}

func putJSON(r *gin.Engine, path, token string, body interface{}) *httptest.ResponseRecorder {
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req, _ = http.NewRequest("PUT", path, bytes.NewBuffer(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest("PUT", path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func createTestSession(t *testing.T, r *gin.Engine, token string) uint {
	t.Helper()
	// Save config
	postJSON(r, "/api/trading/live/config", token, map[string]interface{}{
		"strategy":     "hybrid_ai_trend",
		"interval":     "5m",
		"params":       map[string]interface{}{"bb1_period": 20},
		"symbols":      []string{"AAPL", "MSFT"},
		"long_only":    true,
		"trade_amount": 100,
		"currency":     "EUR",
	})
	// Start session
	w := postJSON(r, "/api/trading/live/start", token, map[string]interface{}{
		"name": "Test Session",
	})
	if w.Code != 200 {
		t.Fatalf("start session failed: %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	sess := resp["session"].(map[string]interface{})
	return uint(sess["id"].(float64))
}

// ============================================================
// Test 1: Session-Erstellung erstellt automatisch eine Strategy
// ============================================================

func TestSessionCreatesInitialStrategy(t *testing.T) {
	setupMultiStratDB(t)
	r, token := setupMultiStratRouter(t)

	sessionID := createTestSession(t, r, token)

	// Check: genau 1 Strategy existiert
	var strategies []LiveSessionStrategy
	db.Where("session_id = ?", sessionID).Find(&strategies)
	if len(strategies) != 1 {
		t.Fatalf("expected 1 initial strategy, got %d", len(strategies))
	}
	s := strategies[0]
	if s.Name != "hybrid_ai_trend" {
		t.Errorf("expected strategy name 'hybrid_ai_trend', got '%s'", s.Name)
	}
	if !s.IsEnabled {
		t.Error("initial strategy should be enabled")
	}
	if !s.LongOnly {
		t.Error("initial strategy should be long_only")
	}
	// Symbols should match session
	var syms []string
	json.Unmarshal([]byte(s.Symbols), &syms)
	if len(syms) != 2 || syms[0] != "AAPL" || syms[1] != "MSFT" {
		t.Errorf("expected [AAPL, MSFT], got %v", syms)
	}
}

// ============================================================
// Test 2: Strategie hinzufügen (Hot-Add) — startet deaktiviert
// ============================================================

func TestAddStrategyStartsDisabled(t *testing.T) {
	setupMultiStratDB(t)
	r, token := setupMultiStratRouter(t)

	sessionID := createTestSession(t, r, token)

	// Add second strategy
	w := postJSON(r, fmt.Sprintf("/api/trading/live/session/%d/strategy", sessionID), token, map[string]interface{}{
		"strategy":  "regression_scalping",
		"params":    `{"period": 14}`,
		"symbols":   []string{"MSFT", "GOOGL", "TSLA"},
		"long_only": false,
	})
	if w.Code != 200 {
		t.Fatalf("add strategy failed: %d — %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	strat := resp["strategy"].(map[string]interface{})
	if strat["is_enabled"].(bool) != false {
		t.Error("hot-added strategy should be disabled")
	}
	if strat["name"].(string) != "regression_scalping" {
		t.Errorf("expected 'regression_scalping', got '%s'", strat["name"])
	}

	// Check: Session symbols were merged (union)
	var session LiveTradingSession
	db.First(&session, sessionID)
	var sessionSyms []string
	json.Unmarshal([]byte(session.Symbols), &sessionSyms)
	expected := map[string]bool{"AAPL": true, "MSFT": true, "GOOGL": true, "TSLA": true}
	if len(sessionSyms) != 4 {
		t.Errorf("expected 4 merged symbols, got %d: %v", len(sessionSyms), sessionSyms)
	}
	for _, s := range sessionSyms {
		if !expected[s] {
			t.Errorf("unexpected symbol in session: %s", s)
		}
	}

	// Check: now 2 strategies
	var strategies []LiveSessionStrategy
	db.Where("session_id = ?", sessionID).Find(&strategies)
	if len(strategies) != 2 {
		t.Fatalf("expected 2 strategies, got %d", len(strategies))
	}
}

// ============================================================
// Test 3: Toggle nur bei gestoppter Session
// ============================================================

func TestToggleStrategyOnlyWhenStopped(t *testing.T) {
	setupMultiStratDB(t)
	r, token := setupMultiStratRouter(t)

	sessionID := createTestSession(t, r, token)

	// Add second strategy
	postJSON(r, fmt.Sprintf("/api/trading/live/session/%d/strategy", sessionID), token, map[string]interface{}{
		"strategy": "regression_scalping", "params": "{}", "symbols": []string{"TSLA"}, "long_only": true,
	})
	var strats []LiveSessionStrategy
	db.Where("session_id = ?", sessionID).Find(&strats)
	secondStratID := strats[1].ID

	// Session is inactive → toggle should work
	w := putJSON(r, fmt.Sprintf("/api/trading/live/session/%d/strategy/%d", sessionID, secondStratID), token, nil)
	if w.Code != 200 {
		t.Fatalf("toggle on stopped session should succeed: %d — %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"].(string) != "aktiviert" {
		t.Errorf("expected 'aktiviert', got '%s'", resp["status"])
	}

	// Make session active
	db.Model(&LiveTradingSession{}).Where("id = ?", sessionID).Update("is_active", true)

	// Session is active → toggle should fail 400
	w = putJSON(r, fmt.Sprintf("/api/trading/live/session/%d/strategy/%d", sessionID, secondStratID), token, nil)
	if w.Code != 400 {
		t.Fatalf("toggle on active session should return 400, got %d", w.Code)
	}
}

// ============================================================
// Test 4: Strategies erscheinen in Session-Detail Response
// ============================================================

func TestSessionDetailIncludesStrategies(t *testing.T) {
	setupMultiStratDB(t)
	r, token := setupMultiStratRouter(t)

	sessionID := createTestSession(t, r, token)

	// Add second strategy
	postJSON(r, fmt.Sprintf("/api/trading/live/session/%d/strategy", sessionID), token, map[string]interface{}{
		"strategy": "regression_scalping", "params": "{}", "symbols": []string{"TSLA"}, "long_only": true,
	})

	// GET session detail
	w := getJSON(r, fmt.Sprintf("/api/trading/live/session/%d", sessionID), token)
	if w.Code != 200 {
		t.Fatalf("get session detail failed: %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	strats, ok := resp["strategies"].([]interface{})
	if !ok || len(strats) != 2 {
		t.Fatalf("expected 2 strategies in detail response, got %v", resp["strategies"])
	}
}

// ============================================================
// Test 5: Sessions-Liste enthält strategies_count
// ============================================================

func TestSessionsListIncludesStrategiesCount(t *testing.T) {
	setupMultiStratDB(t)
	r, token := setupMultiStratRouter(t)

	sessionID := createTestSession(t, r, token)

	// Add 2nd strategy
	postJSON(r, fmt.Sprintf("/api/trading/live/session/%d/strategy", sessionID), token, map[string]interface{}{
		"strategy": "regression_scalping", "params": "{}", "symbols": []string{"TSLA"}, "long_only": true,
	})

	w := getJSON(r, "/api/trading/live/sessions", token)
	if w.Code != 200 {
		t.Fatalf("get sessions failed: %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	sessList := resp["sessions"].([]interface{})
	if len(sessList) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessList))
	}
	s := sessList[0].(map[string]interface{})
	count := s["strategies_count"].(float64)
	if count != 2 {
		t.Errorf("expected strategies_count=2, got %.0f", count)
	}
}

// ============================================================
// Test 6: Positionen sind Strategy-isoliert
// ============================================================

func TestPositionsAreStrategyIsolated(t *testing.T) {
	setupMultiStratDB(t)

	now := time.Now()
	session := LiveTradingSession{
		UserID: 1, Name: "Multi", Strategy: "hybrid_ai_trend", Interval: "5m",
		Symbols: `["AAPL"]`, LongOnly: true, TradeAmount: 100, Currency: "EUR",
		IsActive: true, StartedAt: now.Add(-1 * time.Hour),
	}
	db.Create(&session)

	stratA := LiveSessionStrategy{SessionID: session.ID, Name: "strat_a", ParamsJSON: "{}", Symbols: `["AAPL"]`, IsEnabled: true, LongOnly: true}
	stratB := LiveSessionStrategy{SessionID: session.ID, Name: "strat_b", ParamsJSON: "{}", Symbols: `["AAPL"]`, IsEnabled: true, LongOnly: true}
	db.Create(&stratA)
	db.Create(&stratB)

	// Create position for strategy A
	posA := LiveTradingPosition{
		SessionID: session.ID, StrategyID: stratA.ID, Symbol: "AAPL",
		Direction: "LONG", EntryPrice: 150.0, CurrentPrice: 155.0,
		IsClosed: false, Quantity: 1, SignalIndex: 100,
	}
	db.Create(&posA)

	// Strategy B should NOT see strategy A's position
	var posForB LiveTradingPosition
	err := db.Where("session_id = ? AND strategy_id = ? AND symbol = ? AND is_closed = ?",
		session.ID, stratB.ID, "AAPL", false).First(&posForB).Error
	if err == nil {
		t.Error("strategy B should NOT find strategy A's open position")
	}

	// Strategy A should see its own position
	var posForA LiveTradingPosition
	err = db.Where("session_id = ? AND strategy_id = ? AND symbol = ? AND is_closed = ?",
		session.ID, stratA.ID, "AAPL", false).First(&posForA).Error
	if err != nil {
		t.Error("strategy A should find its own open position")
	}

	// Strategy B can also open a position for AAPL (separate)
	posB := LiveTradingPosition{
		SessionID: session.ID, StrategyID: stratB.ID, Symbol: "AAPL",
		Direction: "LONG", EntryPrice: 152.0, CurrentPrice: 155.0,
		IsClosed: false, Quantity: 1, SignalIndex: 100,
	}
	db.Create(&posB)

	// Both strategies have open positions for AAPL
	var count int64
	db.Model(&LiveTradingPosition{}).Where("session_id = ? AND symbol = ? AND is_closed = ?",
		session.ID, "AAPL", false).Count(&count)
	if count != 2 {
		t.Errorf("expected 2 open AAPL positions (one per strategy), got %d", count)
	}
}

// ============================================================
// Test 7: Signal-Dedup ist Strategy-scoped
// ============================================================

func TestSignalDedupIsStrategyScoped(t *testing.T) {
	setupMultiStratDB(t)

	session := LiveTradingSession{
		UserID: 1, Name: "Dedup", Strategy: "hybrid_ai_trend", Interval: "5m",
		Symbols: `["AAPL"]`, LongOnly: true, TradeAmount: 100, Currency: "EUR",
		IsActive: true, StartedAt: time.Now().Add(-1 * time.Hour),
	}
	db.Create(&session)

	stratA := LiveSessionStrategy{SessionID: session.ID, Name: "strat_a", IsEnabled: true}
	stratB := LiveSessionStrategy{SessionID: session.ID, Name: "strat_b", IsEnabled: true}
	db.Create(&stratA)
	db.Create(&stratB)

	// Position from strategy A at signal_index 50
	db.Create(&LiveTradingPosition{
		SessionID: session.ID, StrategyID: stratA.ID, Symbol: "AAPL",
		SignalIndex: 50, Direction: "LONG", EntryPrice: 100,
	})

	// Strategy B at same signal_index 50 should NOT be a duplicate
	var dup LiveTradingPosition
	isDup := db.Where("session_id = ? AND strategy_id = ? AND symbol = ? AND signal_index = ?",
		session.ID, stratB.ID, "AAPL", 50).First(&dup).Error == nil
	if isDup {
		t.Error("signal_index 50 should NOT be a duplicate for strategy B")
	}

	// Strategy A at same signal_index 50 SHOULD be a duplicate
	isDup = db.Where("session_id = ? AND strategy_id = ? AND symbol = ? AND signal_index = ?",
		session.ID, stratA.ID, "AAPL", 50).First(&dup).Error == nil
	if !isDup {
		t.Error("signal_index 50 SHOULD be a duplicate for strategy A")
	}
}

// ============================================================
// Test 8: closeLivePosition berechnet P&L korrekt
// ============================================================

func TestCloseLivePositionPnL(t *testing.T) {
	setupMultiStratDB(t)

	// LONG position: bought at 100, close at 120 → +20%
	pos := LiveTradingPosition{
		SessionID: 1, StrategyID: 1, Symbol: "AAPL",
		Direction: "LONG", EntryPrice: 100.0, CurrentPrice: 100.0,
		InvestedAmount: 500, Quantity: 5, IsClosed: false,
	}
	db.Create(&pos)

	closeLivePosition(&pos, 120.0, "SIGNAL", "USD")

	if !pos.IsClosed {
		t.Error("position should be closed")
	}
	if pos.ClosePrice != 120.0 {
		t.Errorf("expected close price 120, got %.2f", pos.ClosePrice)
	}
	expectedPct := 20.0 // (120-100)/100 * 100
	if abs(pos.ProfitLossPct-expectedPct) > 0.01 {
		t.Errorf("expected P&L %% = %.2f, got %.2f", expectedPct, pos.ProfitLossPct)
	}
	expectedAmt := 500 * 20.0 / 100 // 100 EUR
	if abs(pos.ProfitLossAmt-expectedAmt) > 0.01 {
		t.Errorf("expected P&L amt = %.2f, got %.2f", expectedAmt, pos.ProfitLossAmt)
	}

	// SHORT position: sold at 100, close at 80 → +20%
	posShort := LiveTradingPosition{
		SessionID: 1, StrategyID: 1, Symbol: "TSLA",
		Direction: "SHORT", EntryPrice: 100.0, CurrentPrice: 100.0,
		InvestedAmount: 500, Quantity: 5, IsClosed: false,
	}
	db.Create(&posShort)

	closeLivePosition(&posShort, 80.0, "SIGNAL", "USD")

	if abs(posShort.ProfitLossPct-20.0) > 0.01 {
		t.Errorf("SHORT P&L %% should be +20%%, got %.2f%%", posShort.ProfitLossPct)
	}

	// SHORT position: sold at 100, close at 120 → -20%
	posShortLoss := LiveTradingPosition{
		SessionID: 1, StrategyID: 1, Symbol: "NVDA",
		Direction: "SHORT", EntryPrice: 100.0, CurrentPrice: 100.0,
		InvestedAmount: 500, Quantity: 5, IsClosed: false,
	}
	db.Create(&posShortLoss)

	closeLivePosition(&posShortLoss, 120.0, "SL", "USD")

	if abs(posShortLoss.ProfitLossPct-(-20.0)) > 0.01 {
		t.Errorf("SHORT loss should be -20%%, got %.2f%%", posShortLoss.ProfitLossPct)
	}
}

// ============================================================
// Test 9: Datenmigration — bestehende Sessions bekommen Strategy
// ============================================================

func TestMigrationCreatesStrategyForExistingSessions(t *testing.T) {
	setupMultiStratDB(t)

	// Create a session WITHOUT a strategy (simulates pre-migration state)
	session := LiveTradingSession{
		UserID: 1, Name: "Legacy", Strategy: "hybrid_ai_trend", Interval: "5m",
		ParamsJSON: `{"bb1_period":20}`, Symbols: `["AAPL","MSFT"]`,
		LongOnly: true, TradeAmount: 100, Currency: "EUR",
	}
	db.Create(&session)

	// Verify no strategy exists yet
	var count int64
	db.Model(&LiveSessionStrategy{}).Where("session_id = ?", session.ID).Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 strategies before migration, got %d", count)
	}

	// Run migration logic (same as in initDB)
	var sessionsWithoutStrategy []LiveTradingSession
	db.Raw(`SELECT s.* FROM live_trading_sessions s LEFT JOIN live_session_strategies ls ON ls.session_id = s.id WHERE ls.id IS NULL`).Scan(&sessionsWithoutStrategy)
	for _, s := range sessionsWithoutStrategy {
		db.Create(&LiveSessionStrategy{
			SessionID:  s.ID,
			Name:       s.Strategy,
			ParamsJSON: s.ParamsJSON,
			Symbols:    s.Symbols,
			IsEnabled:  true,
			LongOnly:   s.LongOnly,
			CreatedAt:  s.CreatedAt,
		})
	}

	// Verify strategy was created
	var strategies []LiveSessionStrategy
	db.Where("session_id = ?", session.ID).Find(&strategies)
	if len(strategies) != 1 {
		t.Fatalf("expected 1 migrated strategy, got %d", len(strategies))
	}
	s := strategies[0]
	if s.Name != "hybrid_ai_trend" {
		t.Errorf("expected strategy 'hybrid_ai_trend', got '%s'", s.Name)
	}
	if !s.IsEnabled {
		t.Error("migrated strategy should be enabled")
	}
	if s.ParamsJSON != `{"bb1_period":20}` {
		t.Errorf("expected params preserved, got '%s'", s.ParamsJSON)
	}
}

// ============================================================
// Test 10: Log-Filterung nach Strategy
// ============================================================

func TestLogFilterByStrategy(t *testing.T) {
	setupMultiStratDB(t)
	r, token := setupMultiStratRouter(t)

	sessionID := createTestSession(t, r, token)

	// Create logs with different strategies
	logLiveEvent(sessionID, "OPEN", "AAPL", "Opened AAPL long", "strat_a")
	logLiveEvent(sessionID, "OPEN", "MSFT", "Opened MSFT long", "strat_b")
	logLiveEvent(sessionID, "SCAN", "-", "Poll started") // no strategy
	logLiveEvent(sessionID, "CLOSE", "AAPL", "Closed AAPL", "strat_a")

	// All logs
	w := getJSON(r, fmt.Sprintf("/api/trading/live/logs/%d", sessionID), token)
	if w.Code != 200 {
		t.Fatalf("get logs failed: %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	allLogs := resp["logs"].([]interface{})
	if len(allLogs) < 4 { // initial session log + our 4
		t.Errorf("expected at least 4 logs, got %d", len(allLogs))
	}

	// Filter by strat_a
	w = getJSON(r, fmt.Sprintf("/api/trading/live/logs/%d?strategy=strat_a", sessionID), token)
	json.Unmarshal(w.Body.Bytes(), &resp)
	filteredLogs := resp["logs"].([]interface{})
	if len(filteredLogs) != 2 {
		t.Errorf("expected 2 logs for strat_a, got %d", len(filteredLogs))
	}

	// Filter by strat_b
	w = getJSON(r, fmt.Sprintf("/api/trading/live/logs/%d?strategy=strat_b", sessionID), token)
	json.Unmarshal(w.Body.Bytes(), &resp)
	filteredLogs = resp["logs"].([]interface{})
	if len(filteredLogs) != 1 {
		t.Errorf("expected 1 log for strat_b, got %d", len(filteredLogs))
	}
}

// ============================================================
// Test 11: Positions haben strategy_name in Session-Detail
// ============================================================

func TestPositionsHaveStrategyNameInDetail(t *testing.T) {
	setupMultiStratDB(t)
	r, token := setupMultiStratRouter(t)

	sessionID := createTestSession(t, r, token)

	// Get the auto-created strategy
	var strat LiveSessionStrategy
	db.Where("session_id = ?", sessionID).First(&strat)

	// Create a position linked to the strategy
	db.Create(&LiveTradingPosition{
		SessionID: sessionID, StrategyID: strat.ID, Symbol: "AAPL",
		Direction: "LONG", EntryPrice: 150, CurrentPrice: 155,
		InvestedAmount: 100, Quantity: 1, EntryTime: time.Now(),
	})

	// GET session detail
	w := getJSON(r, fmt.Sprintf("/api/trading/live/session/%d", sessionID), token)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	positions := resp["positions"].([]interface{})
	if len(positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(positions))
	}
	pos := positions[0].(map[string]interface{})
	stratName, ok := pos["strategy_name"].(string)
	if !ok || stratName != "hybrid_ai_trend" {
		t.Errorf("expected strategy_name='hybrid_ai_trend', got '%v'", pos["strategy_name"])
	}
}

// ============================================================
// Test 12: Symbol-Merge hat keine Duplikate
// ============================================================

func TestSymbolMergeNoDuplicates(t *testing.T) {
	setupMultiStratDB(t)
	r, token := setupMultiStratRouter(t)

	sessionID := createTestSession(t, r, token) // has AAPL, MSFT

	// Add strategy with overlapping symbols
	postJSON(r, fmt.Sprintf("/api/trading/live/session/%d/strategy", sessionID), token, map[string]interface{}{
		"strategy": "regression_scalping", "params": "{}",
		"symbols": []string{"AAPL", "MSFT", "GOOGL"}, // AAPL+MSFT overlap
		"long_only": true,
	})

	var session LiveTradingSession
	db.First(&session, sessionID)
	var syms []string
	json.Unmarshal([]byte(session.Symbols), &syms)

	// Should be exactly 3 (no duplicates)
	if len(syms) != 3 {
		t.Errorf("expected 3 symbols (no dups), got %d: %v", len(syms), syms)
	}
	seen := map[string]int{}
	for _, s := range syms {
		seen[s]++
		if seen[s] > 1 {
			t.Errorf("duplicate symbol: %s", s)
		}
	}
}

// ============================================================
// Test 13: GET strategies Endpoint
// ============================================================

func TestGetStrategiesEndpoint(t *testing.T) {
	setupMultiStratDB(t)
	r, token := setupMultiStratRouter(t)

	sessionID := createTestSession(t, r, token)

	// Add second strategy
	postJSON(r, fmt.Sprintf("/api/trading/live/session/%d/strategy", sessionID), token, map[string]interface{}{
		"strategy": "regression_scalping", "params": "{}", "symbols": []string{"TSLA"}, "long_only": false,
	})

	w := getJSON(r, fmt.Sprintf("/api/trading/live/session/%d/strategies", sessionID), token)
	if w.Code != 200 {
		t.Fatalf("get strategies failed: %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	strats := resp["strategies"].([]interface{})
	if len(strats) != 2 {
		t.Fatalf("expected 2 strategies, got %d", len(strats))
	}

	// First should be enabled (initial), second disabled (hot-add)
	s1 := strats[0].(map[string]interface{})
	s2 := strats[1].(map[string]interface{})
	if s1["is_enabled"].(bool) != true {
		t.Error("first strategy should be enabled")
	}
	if s2["is_enabled"].(bool) != false {
		t.Error("second strategy should be disabled (hot-add)")
	}
	if s2["long_only"].(bool) != false {
		t.Error("second strategy should have long_only=false")
	}
}

// ============================================================
// Test 14: Stop Session schliesst Positionen mit P&L
// ============================================================

func TestStopSessionClosesAllPositions(t *testing.T) {
	setupMultiStratDB(t)
	r, token := setupMultiStratRouter(t)

	sessionID := createTestSession(t, r, token)

	// Activate session
	now := time.Now()
	db.Model(&LiveTradingSession{}).Where("id = ?", sessionID).Updates(map[string]interface{}{
		"is_active": true, "started_at": now,
	})
	// Register in scheduler so stop doesn't panic
	liveSchedulerMu.Lock()
	liveSchedulers[sessionID] = &liveSessionState{StopChan: make(chan struct{})}
	liveSchedulerMu.Unlock()

	// Create open positions from 2 strategies
	var strat LiveSessionStrategy
	db.Where("session_id = ?", sessionID).First(&strat)

	db.Create(&LiveTradingPosition{
		SessionID: sessionID, StrategyID: strat.ID, Symbol: "AAPL",
		Direction: "LONG", EntryPrice: 100, CurrentPrice: 110,
		InvestedAmount: 500, Quantity: 5, EntryTime: now, IsClosed: false,
	})
	db.Create(&LiveTradingPosition{
		SessionID: sessionID, StrategyID: strat.ID, Symbol: "MSFT",
		Direction: "LONG", EntryPrice: 200, CurrentPrice: 190,
		InvestedAmount: 500, Quantity: 2.5, EntryTime: now, IsClosed: false,
	})

	// Stop
	w := postJSON(r, "/api/trading/live/stop?session_id="+fmt.Sprint(sessionID), token, nil)
	if w.Code != 200 {
		t.Fatalf("stop failed: %d — %s", w.Code, w.Body.String())
	}

	// All positions should be closed
	var openCount int64
	db.Model(&LiveTradingPosition{}).Where("session_id = ? AND is_closed = ?", sessionID, false).Count(&openCount)
	if openCount != 0 {
		t.Errorf("expected 0 open positions after stop, got %d", openCount)
	}

	// Check P&L on closed positions
	var positions []LiveTradingPosition
	db.Where("session_id = ?", sessionID).Find(&positions)
	for _, p := range positions {
		if p.CloseReason != "MANUAL" {
			t.Errorf("expected close_reason MANUAL, got %s", p.CloseReason)
		}
		if p.Symbol == "AAPL" && p.ProfitLossPct <= 0 {
			t.Errorf("AAPL (100→110) should have positive P&L, got %.2f%%", p.ProfitLossPct)
		}
		if p.Symbol == "MSFT" && p.ProfitLossPct >= 0 {
			t.Errorf("MSFT (200→190) should have negative P&L, got %.2f%%", p.ProfitLossPct)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
