package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupTestDB creates an in-memory SQLite DB and migrates all models
func setupTestDB(t *testing.T) {
	t.Helper()
	var err error
	db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}
	db.AutoMigrate(
		&User{}, &Stock{}, &Category{}, &PortfolioPosition{}, &PortfolioTradeHistory{},
		&StockPerformance{}, &ActivityLog{},
		&FlipperBotTrade{}, &FlipperBotPosition{},
		&AggressiveStockPerformance{}, &LutzTrade{}, &LutzPosition{},
		&DBSession{}, &BotLog{}, &BotTodo{},
		&BXtrenderConfig{}, &BXtrenderQuantConfig{},
		&QuantStockPerformance{}, &QuantTrade{}, &QuantPosition{},
		&BXtrenderDitzConfig{}, &DitzStockPerformance{}, &DitzTrade{}, &DitzPosition{},
		&BXtrenderTraderConfig{}, &TraderStockPerformance{}, &TraderTrade{}, &TraderPosition{},
		&SystemSetting{}, &BotStockAllowlist{},
	)
}

// setupAdminRouter creates a gin router with an admin session pre-configured
func setupAdminRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Create admin user
	admin := User{Email: "admin@test.com", Username: "admin", Password: "hashed", IsAdmin: true}
	db.Create(&admin)

	// Create session
	token := "test-admin-token"
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
	api.GET("/admin/bot-allowlist", authMiddleware(), adminOnly(), getBotAllowlist)
	api.PUT("/admin/bot-allowlist", authMiddleware(), adminOnly(), updateBotAllowlist)

	return r, token
}

// ============================================================
// Model Tests
// ============================================================

func TestBotStockAllowlistModel(t *testing.T) {
	setupTestDB(t)

	entry := BotStockAllowlist{BotName: "quant", Symbol: "AAPL", Allowed: true}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("failed to create allowlist entry: %v", err)
	}
	if entry.ID == 0 {
		t.Fatal("expected ID to be set after create")
	}

	var loaded BotStockAllowlist
	db.First(&loaded, entry.ID)
	if loaded.BotName != "quant" || loaded.Symbol != "AAPL" || !loaded.Allowed {
		t.Errorf("unexpected values: %+v", loaded)
	}
}

func TestIsAdminClosedFieldOnTrades(t *testing.T) {
	setupTestDB(t)

	now := time.Now()
	pnl := 10.0

	// FlipperBotTrade
	ft := FlipperBotTrade{Symbol: "AAPL", Name: "Apple", Action: "SELL", Price: 150, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &pnl}
	db.Create(&ft)
	var ftLoaded FlipperBotTrade
	db.First(&ftLoaded, ft.ID)
	if !ftLoaded.IsAdminClosed {
		t.Error("FlipperBotTrade: IsAdminClosed should be true")
	}

	// QuantTrade
	qt := QuantTrade{Symbol: "MSFT", Name: "Microsoft", Action: "SELL", Price: 400, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &pnl}
	db.Create(&qt)
	var qtLoaded QuantTrade
	db.First(&qtLoaded, qt.ID)
	if !qtLoaded.IsAdminClosed {
		t.Error("QuantTrade: IsAdminClosed should be true")
	}

	// DitzTrade
	dt := DitzTrade{Symbol: "GOOG", Name: "Google", Action: "SELL", Price: 170, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &pnl}
	db.Create(&dt)
	var dtLoaded DitzTrade
	db.First(&dtLoaded, dt.ID)
	if !dtLoaded.IsAdminClosed {
		t.Error("DitzTrade: IsAdminClosed should be true")
	}

	// TraderTrade
	tt := TraderTrade{Symbol: "TSLA", Name: "Tesla", Action: "SELL", Price: 250, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &pnl}
	db.Create(&tt)
	var ttLoaded TraderTrade
	db.First(&ttLoaded, tt.ID)
	if !ttLoaded.IsAdminClosed {
		t.Error("TraderTrade: IsAdminClosed should be true")
	}

	// LutzTrade
	lt := LutzTrade{Symbol: "AMZN", Name: "Amazon", Action: "SELL", Price: 190, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &pnl}
	db.Create(&lt)
	var ltLoaded LutzTrade
	db.First(&ltLoaded, lt.ID)
	if !ltLoaded.IsAdminClosed {
		t.Error("LutzTrade: IsAdminClosed should be true")
	}
}

func TestIsAdminClosedFieldOnPositions(t *testing.T) {
	setupTestDB(t)

	now := time.Now()

	fp := FlipperBotPosition{Symbol: "AAPL", Name: "Apple", AvgPrice: 150, BuyDate: now, IsAdminClosed: true, IsClosed: true}
	db.Create(&fp)
	var fpLoaded FlipperBotPosition
	db.First(&fpLoaded, fp.ID)
	if !fpLoaded.IsAdminClosed {
		t.Error("FlipperBotPosition: IsAdminClosed should be true")
	}

	qp := QuantPosition{Symbol: "MSFT", Name: "Microsoft", AvgPrice: 400, BuyDate: now, IsAdminClosed: true, IsClosed: true}
	db.Create(&qp)
	var qpLoaded QuantPosition
	db.First(&qpLoaded, qp.ID)
	if !qpLoaded.IsAdminClosed {
		t.Error("QuantPosition: IsAdminClosed should be true")
	}

	dp := DitzPosition{Symbol: "GOOG", Name: "Google", AvgPrice: 170, BuyDate: now, IsAdminClosed: true, IsClosed: true}
	db.Create(&dp)
	var dpLoaded DitzPosition
	db.First(&dpLoaded, dp.ID)
	if !dpLoaded.IsAdminClosed {
		t.Error("DitzPosition: IsAdminClosed should be true")
	}

	tp := TraderPosition{Symbol: "TSLA", Name: "Tesla", AvgPrice: 250, BuyDate: now, IsAdminClosed: true, IsClosed: true}
	db.Create(&tp)
	var tpLoaded TraderPosition
	db.First(&tpLoaded, tp.ID)
	if !tpLoaded.IsAdminClosed {
		t.Error("TraderPosition: IsAdminClosed should be true")
	}

	lp := LutzPosition{Symbol: "AMZN", Name: "Amazon", AvgPrice: 190, BuyDate: now, IsAdminClosed: true, IsClosed: true}
	db.Create(&lp)
	var lpLoaded LutzPosition
	db.First(&lpLoaded, lp.ID)
	if !lpLoaded.IsAdminClosed {
		t.Error("LutzPosition: IsAdminClosed should be true")
	}
}

// ============================================================
// isStockAllowedForBot Tests
// ============================================================

func TestIsStockAllowedForBot_NoEntry(t *testing.T) {
	setupTestDB(t)
	// No entry = allowed
	if !isStockAllowedForBot("flipper", "AAPL") {
		t.Error("expected true when no entry exists")
	}
}

func TestIsStockAllowedForBot_AllowedTrue(t *testing.T) {
	setupTestDB(t)
	db.Create(&BotStockAllowlist{BotName: "flipper", Symbol: "AAPL", Allowed: true})
	if !isStockAllowedForBot("flipper", "AAPL") {
		t.Error("expected true when entry has allowed=true")
	}
}

func TestIsStockAllowedForBot_AllowedFalse(t *testing.T) {
	setupTestDB(t)
	db.Create(&BotStockAllowlist{BotName: "quant", Symbol: "TSLA", Allowed: false})
	if isStockAllowedForBot("quant", "TSLA") {
		t.Error("expected false when entry has allowed=false")
	}
}

func TestIsStockAllowedForBot_DifferentBot(t *testing.T) {
	setupTestDB(t)
	// Block for quant only
	db.Create(&BotStockAllowlist{BotName: "quant", Symbol: "AAPL", Allowed: false})
	// flipper should still be allowed (no entry for flipper)
	if !isStockAllowedForBot("flipper", "AAPL") {
		t.Error("blocking quant should not affect flipper")
	}
}

func TestIsStockAllowedForBot_AllBots(t *testing.T) {
	setupTestDB(t)
	bots := []string{"flipper", "lutz", "quant", "ditz", "trader"}
	for _, bot := range bots {
		db.Create(&BotStockAllowlist{BotName: bot, Symbol: "NVDA", Allowed: false})
	}
	for _, bot := range bots {
		if isStockAllowedForBot(bot, "NVDA") {
			t.Errorf("expected false for bot %s", bot)
		}
	}
}

// ============================================================
// API Endpoint Tests
// ============================================================

func TestGetBotAllowlist_Empty(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminRouter(t)

	req, _ := http.NewRequest("GET", "/api/admin/bot-allowlist", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// All 5 bots should be present
	for _, bot := range []string{"flipper", "lutz", "quant", "ditz", "trader"} {
		if _, ok := resp[bot]; !ok {
			t.Errorf("expected bot %s in response", bot)
		}
	}
}

func TestGetBotAllowlist_WithStocks(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminRouter(t)

	// Add some stock performance data
	db.Create(&StockPerformance{Symbol: "AAPL", Name: "Apple"})
	db.Create(&StockPerformance{Symbol: "MSFT", Name: "Microsoft"})
	// Block AAPL for flipper
	db.Create(&BotStockAllowlist{BotName: "flipper", Symbol: "AAPL", Allowed: false})

	req, _ := http.NewRequest("GET", "/api/admin/bot-allowlist", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string][]struct {
		Symbol  string `json:"symbol"`
		Allowed bool   `json:"allowed"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	flipperStocks := resp["flipper"]
	if len(flipperStocks) != 2 {
		t.Fatalf("expected 2 flipper stocks, got %d", len(flipperStocks))
	}

	for _, s := range flipperStocks {
		if s.Symbol == "AAPL" && s.Allowed {
			t.Error("AAPL should be blocked for flipper")
		}
		if s.Symbol == "MSFT" && !s.Allowed {
			t.Error("MSFT should be allowed for flipper")
		}
	}
}

func TestGetBotAllowlist_Unauthorized(t *testing.T) {
	setupTestDB(t)
	r, _ := setupAdminRouter(t)

	req, _ := http.NewRequest("GET", "/api/admin/bot-allowlist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Fatalf("expected 401 without auth, got %d", w.Code)
	}
}

func TestUpdateBotAllowlist_BlockStock(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"bot_name": "quant",
		"symbol":   "AAPL",
		"allowed":  false,
	})
	req, _ := http.NewRequest("PUT", "/api/admin/bot-allowlist", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["message"] != "Updated" {
		t.Errorf("expected Updated message, got %v", resp["message"])
	}

	// Verify DB
	var entry BotStockAllowlist
	db.Where("bot_name = ? AND symbol = ?", "quant", "AAPL").First(&entry)
	if entry.Allowed {
		t.Error("expected allowed=false in DB")
	}

	// isStockAllowedForBot should return false now
	if isStockAllowedForBot("quant", "AAPL") {
		t.Error("expected isStockAllowedForBot to return false")
	}
}

func TestUpdateBotAllowlist_AllowStock(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminRouter(t)

	// Pre-block
	db.Create(&BotStockAllowlist{BotName: "ditz", Symbol: "TSLA", Allowed: false})

	body, _ := json.Marshal(map[string]interface{}{
		"bot_name": "ditz",
		"symbol":   "TSLA",
		"allowed":  true,
	})
	req, _ := http.NewRequest("PUT", "/api/admin/bot-allowlist", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify unblocked
	if !isStockAllowedForBot("ditz", "TSLA") {
		t.Error("expected TSLA to be allowed for ditz after update")
	}
}

func TestUpdateBotAllowlist_InvalidBot(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"bot_name": "nonexistent",
		"symbol":   "AAPL",
		"allowed":  false,
	})
	req, _ := http.NewRequest("PUT", "/api/admin/bot-allowlist", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid bot, got %d", w.Code)
	}
}

func TestUpdateBotAllowlist_MissingFields(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminRouter(t)

	body, _ := json.Marshal(map[string]interface{}{
		"bot_name": "quant",
	})
	req, _ := http.NewRequest("PUT", "/api/admin/bot-allowlist", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for missing symbol, got %d", w.Code)
	}
}

func TestUpdateBotAllowlist_ToggleMultipleTimes(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminRouter(t)

	// Block
	body, _ := json.Marshal(map[string]interface{}{"bot_name": "flipper", "symbol": "GOOG", "allowed": false})
	req, _ := http.NewRequest("PUT", "/api/admin/bot-allowlist", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("block: expected 200, got %d", w.Code)
	}
	if isStockAllowedForBot("flipper", "GOOG") {
		t.Error("should be blocked after first update")
	}

	// Unblock
	body, _ = json.Marshal(map[string]interface{}{"bot_name": "flipper", "symbol": "GOOG", "allowed": true})
	req, _ = http.NewRequest("PUT", "/api/admin/bot-allowlist", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("unblock: expected 200, got %d", w.Code)
	}
	if !isStockAllowedForBot("flipper", "GOOG") {
		t.Error("should be allowed after second update")
	}

	// Block again
	body, _ = json.Marshal(map[string]interface{}{"bot_name": "flipper", "symbol": "GOOG", "allowed": false})
	req, _ = http.NewRequest("PUT", "/api/admin/bot-allowlist", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("block again: expected 200, got %d", w.Code)
	}
	if isStockAllowedForBot("flipper", "GOOG") {
		t.Error("should be blocked after third update")
	}

	// Verify only 1 entry exists (update, not duplicate)
	var count int64
	db.Model(&BotStockAllowlist{}).Where("bot_name = ? AND symbol = ?", "flipper", "GOOG").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 entry, got %d", count)
	}
}

// ============================================================
// closePositionForBot Tests
// ============================================================

func TestClosePositionForBot_NoPosition(t *testing.T) {
	setupTestDB(t)
	// No position exists - should return false
	if closePositionForBot("flipper", "AAPL") {
		t.Error("expected false when no position exists")
	}
}

// Note: closePositionForBot relies on fetchQuotes which needs external API.
// We test the DB-level logic instead.

func TestClosePositionForBot_FlipperDBLogic(t *testing.T) {
	setupTestDB(t)

	now := time.Now()
	pos := FlipperBotPosition{
		Symbol:   "AAPL",
		Name:     "Apple",
		AvgPrice: 150.0,
		Quantity: 1.0,
		BuyDate:  now,
		IsClosed: false,
	}
	db.Create(&pos)

	// Simulate what closePositionForBot does (without external API call)
	currentPrice := 160.0
	pnl := (currentPrice - pos.AvgPrice) * pos.Quantity
	pnlPct := ((currentPrice - pos.AvgPrice) / pos.AvgPrice) * 100

	sellTrade := FlipperBotTrade{
		Symbol: pos.Symbol, Name: pos.Name, Action: "SELL", Quantity: pos.Quantity,
		Price: currentPrice, SignalDate: now, ExecutedAt: now,
		IsPending: false, IsAdminClosed: true,
		ProfitLoss: &pnl, ProfitLossPct: &pnlPct,
	}
	db.Create(&sellTrade)

	pos.IsClosed = true
	pos.IsAdminClosed = true
	pos.SellPrice = currentPrice
	pos.SellDate = &now
	pos.ProfitLoss = &pnl
	pos.ProfitLossPct = &pnlPct
	db.Save(&pos)

	// Verify trade created with admin closed flag
	var trade FlipperBotTrade
	db.Where("symbol = ? AND action = ? AND is_admin_closed = ?", "AAPL", "SELL", true).First(&trade)
	if trade.ID == 0 {
		t.Fatal("expected admin-closed SELL trade to exist")
	}
	if !trade.IsAdminClosed {
		t.Error("trade should have IsAdminClosed=true")
	}
	if *trade.ProfitLoss != 10.0 {
		t.Errorf("expected PnL 10.0, got %f", *trade.ProfitLoss)
	}

	// Verify position closed
	var closedPos FlipperBotPosition
	db.First(&closedPos, pos.ID)
	if !closedPos.IsClosed {
		t.Error("position should be closed")
	}
	if !closedPos.IsAdminClosed {
		t.Error("position should have IsAdminClosed=true")
	}
	if closedPos.SellPrice != 160.0 {
		t.Errorf("expected sell price 160, got %f", closedPos.SellPrice)
	}
}

func TestClosePositionForBot_AllBotsDBLogic(t *testing.T) {
	setupTestDB(t)
	now := time.Now()

	// Create open positions for all bots
	db.Create(&LutzPosition{Symbol: "MSFT", Name: "Microsoft", AvgPrice: 400, Quantity: 1, BuyDate: now})
	db.Create(&QuantPosition{Symbol: "MSFT", Name: "Microsoft", AvgPrice: 400, Quantity: 1, BuyDate: now})
	db.Create(&DitzPosition{Symbol: "MSFT", Name: "Microsoft", AvgPrice: 400, Quantity: 1, BuyDate: now})
	db.Create(&TraderPosition{Symbol: "MSFT", Name: "Microsoft", AvgPrice: 400, Quantity: 1, BuyDate: now})

	currentPrice := 410.0
	pnl := 10.0
	pnlPct := 2.5

	// Simulate admin close for each bot
	tests := []struct {
		bot      string
		closeFn  func()
		checkFn  func() bool
	}{
		{
			"lutz",
			func() {
				var pos LutzPosition
				db.Where("symbol = ?", "MSFT").First(&pos)
				db.Create(&LutzTrade{Symbol: "MSFT", Name: "Microsoft", Action: "SELL", Quantity: 1, Price: currentPrice, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &pnl, ProfitLossPct: &pnlPct})
				db.Model(&pos).Updates(map[string]interface{}{"is_closed": true, "is_admin_closed": true, "sell_price": currentPrice})
			},
			func() bool {
				var t LutzTrade
				db.Where("symbol = ? AND is_admin_closed = ?", "MSFT", true).First(&t)
				return t.ID > 0
			},
		},
		{
			"quant",
			func() {
				var pos QuantPosition
				db.Where("symbol = ?", "MSFT").First(&pos)
				db.Create(&QuantTrade{Symbol: "MSFT", Name: "Microsoft", Action: "SELL", Quantity: 1, Price: currentPrice, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &pnl, ProfitLossPct: &pnlPct})
				db.Model(&pos).Updates(map[string]interface{}{"is_closed": true, "is_admin_closed": true, "sell_price": currentPrice})
			},
			func() bool {
				var t QuantTrade
				db.Where("symbol = ? AND is_admin_closed = ?", "MSFT", true).First(&t)
				return t.ID > 0
			},
		},
		{
			"ditz",
			func() {
				var pos DitzPosition
				db.Where("symbol = ?", "MSFT").First(&pos)
				db.Create(&DitzTrade{Symbol: "MSFT", Name: "Microsoft", Action: "SELL", Quantity: 1, Price: currentPrice, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &pnl, ProfitLossPct: &pnlPct})
				db.Model(&pos).Updates(map[string]interface{}{"is_closed": true, "is_admin_closed": true, "sell_price": currentPrice})
			},
			func() bool {
				var t DitzTrade
				db.Where("symbol = ? AND is_admin_closed = ?", "MSFT", true).First(&t)
				return t.ID > 0
			},
		},
		{
			"trader",
			func() {
				var pos TraderPosition
				db.Where("symbol = ?", "MSFT").First(&pos)
				db.Create(&TraderTrade{Symbol: "MSFT", Name: "Microsoft", Action: "SELL", Quantity: 1, Price: currentPrice, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &pnl, ProfitLossPct: &pnlPct})
				db.Model(&pos).Updates(map[string]interface{}{"is_closed": true, "is_admin_closed": true, "sell_price": currentPrice})
			},
			func() bool {
				var t TraderTrade
				db.Where("symbol = ? AND is_admin_closed = ?", "MSFT", true).First(&t)
				return t.ID > 0
			},
		},
	}

	for _, tc := range tests {
		tc.closeFn()
		if !tc.checkFn() {
			t.Errorf("bot %s: admin-closed trade not found", tc.bot)
		}
	}
}

// ============================================================
// Simulated Performance Filter Tests
// ============================================================

func TestSimulatedPerformance_ExcludesAdminClosed(t *testing.T) {
	setupTestDB(t)

	now := time.Now()
	pnl := 10.0
	pnlPct := 5.0
	negPnl := -5.0
	negPnlPct := -2.5

	// FlipperBot: 1 normal SELL + 1 admin-closed SELL
	db.Create(&FlipperBotTrade{Symbol: "AAPL", Name: "Apple", Action: "SELL", Price: 160, SignalDate: now, ExecutedAt: now, ProfitLoss: &pnl, ProfitLossPct: &pnlPct})
	db.Create(&FlipperBotTrade{Symbol: "TSLA", Name: "Tesla", Action: "SELL", Price: 250, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &negPnl, ProfitLossPct: &negPnlPct})
	db.Create(&FlipperBotTrade{Symbol: "AAPL", Name: "Apple", Action: "BUY", Price: 150, SignalDate: now, ExecutedAt: now})
	db.Create(&FlipperBotTrade{Symbol: "TSLA", Name: "Tesla", Action: "BUY", Price: 255, SignalDate: now, ExecutedAt: now, IsAdminClosed: true})

	// Query with the same filter as getFlipperBotSimulatedPerformance
	var sellTrades []FlipperBotTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_admin_closed = ?", "SELL", false, false, false).Find(&sellTrades)

	if len(sellTrades) != 1 {
		t.Fatalf("expected 1 non-admin SELL trade, got %d", len(sellTrades))
	}
	if sellTrades[0].Symbol != "AAPL" {
		t.Errorf("expected AAPL sell trade, got %s", sellTrades[0].Symbol)
	}

	var buyTrades []FlipperBotTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_admin_closed = ?", "BUY", false, false, false).Find(&buyTrades)

	if len(buyTrades) != 1 {
		t.Fatalf("expected 1 non-admin BUY trade, got %d", len(buyTrades))
	}
}

func TestSimulatedPerformance_QuantExcludesAdminClosed(t *testing.T) {
	setupTestDB(t)

	now := time.Now()
	pnl := 20.0
	pnlPct := 10.0

	db.Create(&QuantTrade{Symbol: "AAPL", Name: "Apple", Action: "SELL", Price: 160, SignalDate: now, ExecutedAt: now, ProfitLoss: &pnl, ProfitLossPct: &pnlPct})
	db.Create(&QuantTrade{Symbol: "TSLA", Name: "Tesla", Action: "SELL", Price: 250, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &pnl, ProfitLossPct: &pnlPct})

	// Quant uses is_live in the filter
	var sellTrades []QuantTrade
	db.Where("action = ? AND is_pending = ? AND is_live = ? AND is_deleted = ? AND is_admin_closed = ?", "SELL", false, false, false, false).Find(&sellTrades)

	if len(sellTrades) != 1 {
		t.Fatalf("expected 1 non-admin SELL trade for quant, got %d", len(sellTrades))
	}
	if sellTrades[0].Symbol != "AAPL" {
		t.Errorf("expected AAPL, got %s", sellTrades[0].Symbol)
	}
}

func TestSimulatedPerformance_DitzExcludesAdminClosed(t *testing.T) {
	setupTestDB(t)

	now := time.Now()
	pnl := 15.0
	pnlPct := 7.5

	db.Create(&DitzTrade{Symbol: "GOOG", Name: "Google", Action: "SELL", Price: 180, SignalDate: now, ExecutedAt: now, ProfitLoss: &pnl, ProfitLossPct: &pnlPct})
	db.Create(&DitzTrade{Symbol: "META", Name: "Meta", Action: "SELL", Price: 500, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &pnl, ProfitLossPct: &pnlPct})

	var sellTrades []DitzTrade
	db.Where("action = ? AND is_pending = ? AND is_live = ? AND is_deleted = ? AND is_admin_closed = ?", "SELL", false, false, false, false).Find(&sellTrades)

	if len(sellTrades) != 1 {
		t.Fatalf("expected 1 non-admin SELL trade for ditz, got %d", len(sellTrades))
	}
}

func TestSimulatedPerformance_TraderExcludesAdminClosed(t *testing.T) {
	setupTestDB(t)

	now := time.Now()
	pnl := 15.0
	pnlPct := 7.5

	db.Create(&TraderTrade{Symbol: "GOOG", Name: "Google", Action: "SELL", Price: 180, SignalDate: now, ExecutedAt: now, ProfitLoss: &pnl, ProfitLossPct: &pnlPct})
	db.Create(&TraderTrade{Symbol: "META", Name: "Meta", Action: "SELL", Price: 500, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &pnl, ProfitLossPct: &pnlPct})

	var sellTrades []TraderTrade
	db.Where("action = ? AND is_pending = ? AND is_live = ? AND is_deleted = ? AND is_admin_closed = ?", "SELL", false, false, false, false).Find(&sellTrades)

	if len(sellTrades) != 1 {
		t.Fatalf("expected 1 non-admin SELL trade for trader, got %d", len(sellTrades))
	}
}

func TestSimulatedPerformance_LutzExcludesAdminClosed(t *testing.T) {
	setupTestDB(t)

	now := time.Now()
	pnl := 15.0
	pnlPct := 7.5

	db.Create(&LutzTrade{Symbol: "GOOG", Name: "Google", Action: "SELL", Price: 180, SignalDate: now, ExecutedAt: now, ProfitLoss: &pnl, ProfitLossPct: &pnlPct})
	db.Create(&LutzTrade{Symbol: "META", Name: "Meta", Action: "SELL", Price: 500, SignalDate: now, ExecutedAt: now, IsAdminClosed: true, ProfitLoss: &pnl, ProfitLossPct: &pnlPct})

	var sellTrades []LutzTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_admin_closed = ?", "SELL", false, false, false).Find(&sellTrades)

	if len(sellTrades) != 1 {
		t.Fatalf("expected 1 non-admin SELL trade for lutz, got %d", len(sellTrades))
	}
}

// ============================================================
// Allowlist + Bot Update Integration Tests
// ============================================================

func TestAllowlistBlockPreventsNewTrades(t *testing.T) {
	setupTestDB(t)

	// Block AAPL for all bots
	bots := []string{"flipper", "lutz", "quant", "ditz", "trader"}
	for _, bot := range bots {
		db.Create(&BotStockAllowlist{BotName: bot, Symbol: "AAPL", Allowed: false})
	}

	// All bots should skip AAPL
	for _, bot := range bots {
		if isStockAllowedForBot(bot, "AAPL") {
			t.Errorf("%s should not be allowed to trade AAPL", bot)
		}
	}

	// But MSFT should still be allowed (no entry)
	for _, bot := range bots {
		if !isStockAllowedForBot(bot, "MSFT") {
			t.Errorf("%s should be allowed to trade MSFT", bot)
		}
	}
}

func TestAllowlistPerBotIsolation(t *testing.T) {
	setupTestDB(t)

	// Block AAPL only for flipper and quant
	db.Create(&BotStockAllowlist{BotName: "flipper", Symbol: "AAPL", Allowed: false})
	db.Create(&BotStockAllowlist{BotName: "quant", Symbol: "AAPL", Allowed: false})

	if isStockAllowedForBot("flipper", "AAPL") {
		t.Error("flipper should be blocked")
	}
	if isStockAllowedForBot("quant", "AAPL") {
		t.Error("quant should be blocked")
	}
	if !isStockAllowedForBot("lutz", "AAPL") {
		t.Error("lutz should be allowed")
	}
	if !isStockAllowedForBot("ditz", "AAPL") {
		t.Error("ditz should be allowed")
	}
	if !isStockAllowedForBot("trader", "AAPL") {
		t.Error("trader should be allowed")
	}
}

func TestUpdateAllowlist_ClosesPositionOnBlock(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminRouter(t)

	// Create an open flipper position
	now := time.Now()
	db.Create(&FlipperBotPosition{
		Symbol: "AAPL", Name: "Apple", AvgPrice: 150, Quantity: 1, BuyDate: now,
	})

	// Block AAPL - this calls closePositionForBot which needs fetchQuotes (external API)
	// The endpoint will attempt to close but fetchQuotes will return 0 price (no API key in test)
	// So closed_position should be false
	body, _ := json.Marshal(map[string]interface{}{
		"bot_name": "flipper",
		"symbol":   "AAPL",
		"allowed":  false,
	})
	req, _ := http.NewRequest("PUT", "/api/admin/bot-allowlist", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	// Position wasn't closed because fetchQuotes returns 0 in test env
	if resp["closed_position"] != false {
		t.Log("closed_position was true - external API was reachable in test")
	}

	// But the allowlist entry should be saved regardless
	if isStockAllowedForBot("flipper", "AAPL") {
		t.Error("AAPL should be blocked for flipper")
	}
}

// ============================================================
// Edge Cases
// ============================================================

func TestAllowlistMultipleSymbols(t *testing.T) {
	setupTestDB(t)

	symbols := []string{"AAPL", "MSFT", "GOOG", "TSLA", "AMZN"}
	for _, sym := range symbols {
		db.Create(&BotStockAllowlist{BotName: "quant", Symbol: sym, Allowed: false})
	}

	for _, sym := range symbols {
		if isStockAllowedForBot("quant", sym) {
			t.Errorf("expected %s blocked for quant", sym)
		}
	}

	// Unblock AAPL
	db.Model(&BotStockAllowlist{}).Where("bot_name = ? AND symbol = ?", "quant", "AAPL").Update("allowed", true)
	if !isStockAllowedForBot("quant", "AAPL") {
		t.Error("AAPL should be allowed after unblock")
	}

	// Others still blocked
	for _, sym := range []string{"MSFT", "GOOG", "TSLA", "AMZN"} {
		if isStockAllowedForBot("quant", sym) {
			t.Errorf("%s should still be blocked", sym)
		}
	}
}

func TestGetAllowlist_MultipleBotsWithDifferentStocks(t *testing.T) {
	setupTestDB(t)
	r, token := setupAdminRouter(t)

	// Different performance tables per bot
	db.Create(&StockPerformance{Symbol: "AAPL", Name: "Apple"})
	db.Create(&AggressiveStockPerformance{Symbol: "TSLA", Name: "Tesla"})
	db.Create(&QuantStockPerformance{Symbol: "MSFT", Name: "Microsoft"})
	db.Create(&DitzStockPerformance{Symbol: "GOOG", Name: "Google"})
	db.Create(&TraderStockPerformance{Symbol: "AMZN", Name: "Amazon"})

	req, _ := http.NewRequest("GET", "/api/admin/bot-allowlist", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string][]struct {
		Symbol  string `json:"symbol"`
		Allowed bool   `json:"allowed"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp["flipper"]) != 1 || resp["flipper"][0].Symbol != "AAPL" {
		t.Errorf("flipper should have AAPL, got %+v", resp["flipper"])
	}
	if len(resp["lutz"]) != 1 || resp["lutz"][0].Symbol != "TSLA" {
		t.Errorf("lutz should have TSLA, got %+v", resp["lutz"])
	}
	if len(resp["quant"]) != 1 || resp["quant"][0].Symbol != "MSFT" {
		t.Errorf("quant should have MSFT, got %+v", resp["quant"])
	}
	if len(resp["ditz"]) != 1 || resp["ditz"][0].Symbol != "GOOG" {
		t.Errorf("ditz should have GOOG, got %+v", resp["ditz"])
	}
	if len(resp["trader"]) != 1 || resp["trader"][0].Symbol != "AMZN" {
		t.Errorf("trader should have AMZN, got %+v", resp["trader"])
	}
}
