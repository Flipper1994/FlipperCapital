package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Email       string    `json:"email" gorm:"uniqueIndex;not null"`
	Username    string    `json:"username" gorm:"uniqueIndex;not null"`
	Password    string    `json:"-" gorm:"not null"`
	IsAdmin     bool      `json:"is_admin" gorm:"default:false"`
	LoginCount  int       `json:"login_count" gorm:"default:0"`
	LastActive  time.Time `json:"last_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ActivityLog tracks user activities
type ActivityLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index"`
	Username  string    `json:"username"`
	Action    string    `json:"action" gorm:"index"` // login, search, page_view, add_stock, etc.
	Details   string    `json:"details"`             // JSON with extra info
	IPAddress string    `json:"ip_address" gorm:"index"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

// Category for organizing stocks in watchlist
type Category struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"not null"`
	SortOrder int       `json:"sort_order" gorm:"default:0"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Stock struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Symbol        string    `json:"symbol" gorm:"not null;uniqueIndex"`
	Name          string    `json:"name" gorm:"not null"`
	CategoryID    *uint     `json:"category_id" gorm:"index"`
	AddedByUserID uint      `json:"added_by_user_id"`
	AddedByUser   string    `json:"added_by_user"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type StockWithQuote struct {
	ID            uint      `json:"id"`
	Symbol        string    `json:"symbol"`
	Name          string    `json:"name"`
	Price         float64   `json:"price"`
	Change        float64   `json:"change"`
	ChangePercent float64   `json:"change_percent"`
	PrevClose     float64   `json:"prev_close"`
	Sector        string    `json:"sector"`
	MarketCap     int64     `json:"market_cap"`
	CategoryID    *uint     `json:"category_id"`
	CategoryName  string    `json:"category_name"`
	CreatedAt     time.Time `json:"created_at"`
}

type SearchResult struct {
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Exchange string `json:"exchange"`
}

type YahooQuoteResponse struct {
	QuoteResponse struct {
		Result []struct {
			Symbol                     string  `json:"symbol"`
			ShortName                  string  `json:"shortName"`
			LongName                   string  `json:"longName"`
			RegularMarketPrice         float64 `json:"regularMarketPrice"`
			RegularMarketChange        float64 `json:"regularMarketChange"`
			RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
			RegularMarketPreviousClose float64 `json:"regularMarketPreviousClose"`
			Sector                     string  `json:"sector"`
			MarketCap                  int64   `json:"marketCap"`
		} `json:"result"`
	} `json:"quoteResponse"`
}

type YahooSearchResponse struct {
	Quotes []struct {
		Symbol    string `json:"symbol"`
		ShortName string `json:"shortname"`
		LongName  string `json:"longname"`
		QuoteType string `json:"quoteType"`
		Exchange  string `json:"exchange"`
	} `json:"quotes"`
}

type Session struct {
	UserID  uint
	IsAdmin bool
	Expiry  time.Time
}

// Persistent session in database
type DBSession struct {
	Token     string    `gorm:"primaryKey"`
	UserID    uint      `gorm:"not null"`
	IsAdmin   bool      `gorm:"default:false"`
	Expiry    time.Time `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PortfolioPosition struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	UserID       uint       `json:"user_id" gorm:"index;not null"`
	Symbol       string     `json:"symbol" gorm:"not null"`
	Name         string     `json:"name" gorm:"not null"`
	PurchaseDate *time.Time `json:"purchase_date"`
	AvgPrice     float64    `json:"avg_price" gorm:"not null"`
	Currency     string     `json:"currency" gorm:"default:EUR"`
	Quantity     *float64   `json:"quantity"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type PortfolioPositionWithQuote struct {
	ID              uint       `json:"id"`
	Symbol          string     `json:"symbol"`
	Name            string     `json:"name"`
	PurchaseDate    *time.Time `json:"purchase_date"`
	AvgPrice        float64    `json:"avg_price"`
	AvgPriceUSD     float64    `json:"avg_price_usd"`
	Currency        string     `json:"currency"`
	Quantity        *float64   `json:"quantity"`
	CurrentPrice    float64    `json:"current_price"`
	Change          float64    `json:"change"`
	ChangePercent   float64    `json:"change_percent"`
	TotalReturn     float64    `json:"total_return"`
	TotalReturnPct  float64    `json:"total_return_pct"`
	CurrentValue    float64    `json:"current_value"`
	InvestedValue   float64    `json:"invested_value"`
}

// PortfolioTradeHistory stores closed/sold positions for performance calculation
type PortfolioTradeHistory struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	UserID       uint       `json:"user_id" gorm:"index;not null"`
	Symbol       string     `json:"symbol" gorm:"not null"`
	Name         string     `json:"name" gorm:"not null"`
	BuyPrice     float64    `json:"buy_price" gorm:"not null"`
	SellPrice    float64    `json:"sell_price" gorm:"not null"`
	Currency     string     `json:"currency" gorm:"default:EUR"`
	Quantity     float64    `json:"quantity" gorm:"default:1"`
	BuyDate      *time.Time `json:"buy_date"`
	SellDate     time.Time  `json:"sell_date" gorm:"not null"`
	ProfitLoss   float64    `json:"profit_loss"`
	ProfitLossPct float64   `json:"profit_loss_pct"`
	CreatedAt    time.Time  `json:"created_at"`
}

// StockPerformance stores BX Trender performance data for tracked stocks
type StockPerformance struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Symbol       string    `json:"symbol" gorm:"uniqueIndex;not null"`
	Name         string    `json:"name"`
	WinRate      float64   `json:"win_rate"`
	RiskReward   float64   `json:"risk_reward"`
	TotalReturn  float64   `json:"total_return"`
	TotalTrades  int       `json:"total_trades"`
	Wins         int       `json:"wins"`
	Losses       int       `json:"losses"`
	Signal       string    `json:"signal"` // BUY, SELL, HOLD, WAIT
	SignalBars   int       `json:"signal_bars"` // How many bars in current signal
	TradesJSON   string    `json:"trades_json" gorm:"type:text"` // JSON array of trades
	CurrentPrice float64   `json:"current_price"`
	UpdatedAt    time.Time `json:"updated_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type TradeData struct {
	EntryDate    int64   `json:"entryDate"`
	EntryPrice   float64 `json:"entryPrice"`
	ExitDate     *int64  `json:"exitDate"`
	ExitPrice    *float64 `json:"exitPrice"`
	CurrentPrice *float64 `json:"currentPrice"`
	ReturnPct    float64 `json:"returnPct"`
	IsOpen       bool    `json:"isOpen"`
}

// FlipperBotTrade tracks all trades made by the FlipperBot
type FlipperBotTrade struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	Symbol       string     `json:"symbol" gorm:"index;not null"`
	Name         string     `json:"name"`
	Action       string     `json:"action" gorm:"not null"` // BUY or SELL
	Quantity     float64    `json:"quantity" gorm:"default:1"`
	IsLive       bool       `json:"is_live" gorm:"default:false"` // True if this is a real executed trade
	Price        float64    `json:"price" gorm:"not null"`
	SignalDate   time.Time  `json:"signal_date" gorm:"not null"` // When the signal occurred
	ExecutedAt   time.Time  `json:"executed_at" gorm:"not null"` // When we recorded this trade
	ProfitLoss   *float64   `json:"profit_loss"`                 // Only for SELL trades
	ProfitLossPct *float64  `json:"profit_loss_pct"`             // Only for SELL trades
	CreatedAt    time.Time  `json:"created_at"`
}

// FlipperBotPosition tracks current open positions of the FlipperBot
type FlipperBotPosition struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	Symbol       string     `json:"symbol" gorm:"uniqueIndex;not null"`
	Name         string     `json:"name"`
	Quantity     float64    `json:"quantity" gorm:"default:1"`
	AvgPrice     float64    `json:"avg_price" gorm:"not null"`
	InvestedEUR  float64    `json:"invested_eur" gorm:"default:0"` // Exact EUR amount invested
	IsLive       bool       `json:"is_live" gorm:"default:false"`  // True if this is a real executed position
	BuyDate      time.Time  `json:"buy_date" gorm:"not null"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

const FLIPPERBOT_START_DATE = "2026-01-01"
const FLIPPERBOT_USER_ID = 999999 // Special user ID for FlipperBot
const LUTZ_USER_ID = 999998       // Special user ID for Lutz (aggressive mode bot)

// AggressiveStockPerformance stores performance data for aggressive trading mode
type AggressiveStockPerformance struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Symbol       string    `json:"symbol" gorm:"uniqueIndex;not null"`
	Name         string    `json:"name"`
	WinRate      float64   `json:"win_rate"`
	RiskReward   float64   `json:"risk_reward"`
	TotalReturn  float64   `json:"total_return"`
	TotalTrades  int       `json:"total_trades"`
	Wins         int       `json:"wins"`
	Losses       int       `json:"losses"`
	Signal       string    `json:"signal"` // BUY, SELL, HOLD, WAIT
	SignalBars   int       `json:"signal_bars"`
	TradesJSON   string    `json:"trades_json" gorm:"type:text"`
	CurrentPrice float64   `json:"current_price"`
	UpdatedAt    time.Time `json:"updated_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// LutzTrade tracks all trades made by the Lutz bot (aggressive mode)
type LutzTrade struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Symbol        string     `json:"symbol" gorm:"index;not null"`
	Name          string     `json:"name"`
	Action        string     `json:"action" gorm:"not null"` // BUY or SELL
	Quantity      float64    `json:"quantity" gorm:"default:1"`
	IsLive        bool       `json:"is_live" gorm:"default:false"` // True if this is a real executed trade
	Price         float64    `json:"price" gorm:"not null"`
	SignalDate    time.Time  `json:"signal_date" gorm:"not null"`
	ExecutedAt    time.Time  `json:"executed_at" gorm:"not null"`
	ProfitLoss    *float64   `json:"profit_loss"`
	ProfitLossPct *float64   `json:"profit_loss_pct"`
	CreatedAt     time.Time  `json:"created_at"`
}

// LutzPosition tracks current open positions of the Lutz bot
type LutzPosition struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Symbol      string    `json:"symbol" gorm:"uniqueIndex;not null"`
	Name        string    `json:"name"`
	Quantity    float64   `json:"quantity" gorm:"default:1"`
	AvgPrice    float64   `json:"avg_price" gorm:"not null"`
	InvestedEUR float64   `json:"invested_eur" gorm:"default:0"` // Exact EUR amount invested
	IsLive      bool      `json:"is_live" gorm:"default:false"`  // True if this is a real executed position
	BuyDate     time.Time `json:"buy_date" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BotLog stores debug logs for bots (persistent)
type BotLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Bot       string    `json:"bot" gorm:"index;not null"` // "flipperbot" or "lutz"
	Level     string    `json:"level" gorm:"not null"`     // INFO, WARN, ERROR, ACTION, SKIP, DEBUG
	Message   string    `json:"message" gorm:"not null"`
	SessionID string    `json:"session_id" gorm:"index"`   // Groups logs from same update run
	CreatedAt time.Time `json:"created_at"`
}

// BotTodo stores pending actions for bots (persistent)
type BotTodo struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Bot         string     `json:"bot" gorm:"index;not null"` // "flipperbot" or "lutz"
	Type        string     `json:"type" gorm:"not null"`      // BUY or SELL
	Symbol      string     `json:"symbol" gorm:"not null"`
	Name        string     `json:"name"`
	Quantity    float64    `json:"quantity"`
	AvgPrice    float64    `json:"avg_price"`   // For SELL: position's avg buy price
	Price       float64    `json:"price"`       // Execution price (buy price or sell price)
	Signal      string     `json:"signal"`
	SignalBars  int        `json:"signal_bars"`
	SignalSince string     `json:"signal_since"`
	Reason      string     `json:"reason"`
	Done        bool       `json:"done" gorm:"default:false"`
	Decision    string     `json:"decision"`    // executed, discarded, deleted
	DoneAt      *time.Time `json:"done_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

var db *gorm.DB
var sessions = make(map[string]Session) // Legacy in-memory cache, DB is source of truth
var httpClient = &http.Client{Timeout: 10 * time.Second}

// Session helper functions for persistent storage
func createSession(userID uint, isAdmin bool) string {
	token := uuid.New().String()
	expiry := time.Now().Add(7 * 24 * time.Hour)

	// Store in database
	dbSession := DBSession{
		Token:   token,
		UserID:  userID,
		IsAdmin: isAdmin,
		Expiry:  expiry,
	}
	db.Create(&dbSession)

	// Also cache in memory for performance
	sessions[token] = Session{
		UserID:  userID,
		IsAdmin: isAdmin,
		Expiry:  expiry,
	}

	return token
}

func getSession(token string) (*Session, bool) {
	// Try memory cache first
	if session, exists := sessions[token]; exists {
		if time.Now().Before(session.Expiry) {
			return &session, true
		}
		// Expired in cache, remove it
		delete(sessions, token)
	}

	// Check database
	var dbSession DBSession
	if err := db.Where("token = ?", token).First(&dbSession).Error; err != nil {
		return nil, false
	}

	if time.Now().After(dbSession.Expiry) {
		// Expired, delete from DB
		db.Delete(&dbSession)
		return nil, false
	}

	// Extend session expiry on each use (rolling session)
	newExpiry := time.Now().Add(7 * 24 * time.Hour)
	db.Model(&dbSession).Update("expiry", newExpiry)

	// Update memory cache
	session := Session{
		UserID:  dbSession.UserID,
		IsAdmin: dbSession.IsAdmin,
		Expiry:  newExpiry,
	}
	sessions[token] = session

	return &session, true
}

func deleteSession(token string) {
	delete(sessions, token)
	db.Where("token = ?", token).Delete(&DBSession{})
}

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/watchlist.db"
	}

	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to database: " + err.Error())
	}

	db.AutoMigrate(&User{}, &Stock{}, &Category{}, &PortfolioPosition{}, &PortfolioTradeHistory{}, &StockPerformance{}, &ActivityLog{}, &FlipperBotTrade{}, &FlipperBotPosition{}, &AggressiveStockPerformance{}, &LutzTrade{}, &LutzPosition{}, &DBSession{}, &BotLog{}, &BotTodo{})

	// Ensure "Sonstiges" category exists
	ensureSonstigesCategory()

	// Ensure is_live columns exist (SQLite doesn't always add new columns)
	db.Exec("ALTER TABLE flipper_bot_trades ADD COLUMN is_live BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE flipper_bot_positions ADD COLUMN is_live BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE lutz_trades ADD COLUMN is_live BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE lutz_positions ADD COLUMN is_live BOOLEAN DEFAULT 0")

	// Clean up expired sessions on startup
	db.Where("expiry < ?", time.Now()).Delete(&DBSession{})

	// Ensure FlipperBot and Lutz users exist for portfolio comparison
	ensureFlipperBotUser()
	ensureLutzUser()

	// Fetch live exchange rates on startup
	go fetchLiveExchangeRates()

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, PUT, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := r.Group("/api")
	{
		// Auth routes
		api.POST("/register", register)
		api.POST("/login", login)
		api.POST("/logout", logout)
		api.GET("/verify", verifyToken)
		api.GET("/me", authMiddleware(), getCurrentUser)

		// Stock routes
		api.GET("/stocks", getStocks)
		api.POST("/stocks", optionalAuthMiddleware(), createStock)
		api.DELETE("/stocks/:id", authMiddleware(), adminOnly(), deleteStock)
		api.PUT("/stocks/:id/category", authMiddleware(), adminOnly(), updateStockCategory)
		api.GET("/search", searchStocks)
		api.GET("/quote/:symbol", getQuote)
		api.GET("/history/:symbol", getHistory)

		// Category routes
		api.GET("/categories", getCategories)
		api.POST("/categories", authMiddleware(), adminOnly(), createCategory)
		api.PUT("/categories/:id", authMiddleware(), adminOnly(), updateCategory)
		api.DELETE("/categories/:id", authMiddleware(), adminOnly(), deleteCategory)
		api.PUT("/categories/reorder", authMiddleware(), adminOnly(), reorderCategories)

		// Portfolio routes
		api.GET("/portfolio", authMiddleware(), getPortfolio)
		api.POST("/portfolio", authMiddleware(), createPortfolioPosition)
		api.PUT("/portfolio/:id", authMiddleware(), updatePortfolioPosition)
		api.DELETE("/portfolio/:id", authMiddleware(), deletePortfolioPosition)
		api.POST("/portfolio/:id/sell", authMiddleware(), sellPortfolioPosition)
		api.GET("/portfolio/performance", authMiddleware(), getPortfolioPerformance)
		api.GET("/portfolio/trades", authMiddleware(), getPortfolioTrades)
		api.GET("/portfolio/history", authMiddleware(), getPortfolioHistory)
		api.GET("/portfolios/compare", authMiddleware(), getAllPortfoliosForComparison)
		api.GET("/portfolios/history/all", authMiddleware(), getAllPortfoliosHistory)
		api.GET("/portfolios/history/:userId", authMiddleware(), getUserPortfolioHistory)

		// Stock Performance Tracker routes (Defensive mode)
		api.POST("/performance", saveStockPerformance)
		api.GET("/performance", getTrackedStocks)
		api.GET("/performance/:symbol", getStockPerformance)

		// Aggressive mode performance routes
		api.POST("/performance/aggressive", saveAggressiveStockPerformance)
		api.GET("/performance/aggressive", getAggressiveTrackedStocks)
		api.GET("/performance/aggressive/:symbol", getAggressiveStockPerformance)

		// User permission check
		api.GET("/can-add-stocks", optionalAuthMiddleware(), canAddStocks)

		// Activity logging
		api.POST("/activity", optionalAuthMiddleware(), logActivity)

		// Admin routes
		api.GET("/admin/users", authMiddleware(), adminOnly(), getAdminUsers)
		api.DELETE("/admin/users/:id", authMiddleware(), adminOnly(), deleteAdminUser)
		api.PUT("/admin/users/:id", authMiddleware(), adminOnly(), updateAdminUser)
		api.GET("/admin/activity", authMiddleware(), adminOnly(), getAdminActivity)
		api.GET("/admin/stats", authMiddleware(), adminOnly(), getAdminStats)
		api.GET("/admin/traffic", authMiddleware(), adminOnly(), getAdminTraffic)
		api.GET("/admin/update-all-stocks", authMiddleware(), adminOnly(), updateAllWatchlistStocks)
		api.GET("/admin/tracked-diff", authMiddleware(), adminOnly(), getTrackedDiff)
		api.DELETE("/admin/tracked/:symbol", authMiddleware(), adminOnly(), deleteTrackedStock)

		// FlipperBot routes - Defensive mode (view: all users, actions: admin only)
		api.GET("/flipperbot/update", authMiddleware(), adminOnly(), flipperBotUpdate)
		api.GET("/flipperbot/portfolio", authMiddleware(), getFlipperBotPortfolio)
		api.GET("/flipperbot/actions", authMiddleware(), getFlipperBotActions)
		api.GET("/flipperbot/performance", authMiddleware(), getFlipperBotPerformance)
		api.POST("/flipperbot/reset", authMiddleware(), adminOnly(), resetFlipperBot)
		api.PUT("/flipperbot/position/:id", authMiddleware(), adminOnly(), updateFlipperBotPosition)
		api.PUT("/flipperbot/trade/:id", authMiddleware(), adminOnly(), updateFlipperBotTrade)
		api.DELETE("/flipperbot/trade/:id", authMiddleware(), adminOnly(), deleteFlipperBotTrade)
		api.GET("/flipperbot/pending", authMiddleware(), adminOnly(), getFlipperBotPending)
		api.GET("/flipperbot/logs", authMiddleware(), getFlipperBotLogs)
		api.GET("/flipperbot/todos", authMiddleware(), getFlipperBotTodos)
		api.PUT("/flipperbot/todos/:id/done", authMiddleware(), adminOnly(), markFlipperBotTodoDone)
		api.PUT("/flipperbot/todos/:id/reopen", authMiddleware(), adminOnly(), reopenFlipperBotTodo)
		api.DELETE("/flipperbot/todos/:id", authMiddleware(), adminOnly(), deleteFlipperBotTodo)
		api.POST("/flipperbot/todos/:id/execute", authMiddleware(), adminOnly(), executeFlipperBotTodo)
		api.POST("/flipperbot/sync", authMiddleware(), adminOnly(), syncFlipperBot)
		api.GET("/flipperbot/completed-trades", authMiddleware(), getFlipperBotCompletedTrades)
		api.POST("/flipperbot/fix-db", authMiddleware(), adminOnly(), fixFlipperBotDB)

		// Lutz routes - Aggressive mode bot (view: all users, actions: admin only)
		api.GET("/lutz/update", authMiddleware(), adminOnly(), lutzUpdate)
		api.GET("/lutz/portfolio", authMiddleware(), getLutzPortfolio)
		api.GET("/lutz/actions", authMiddleware(), getLutzActions)
		api.GET("/lutz/performance", authMiddleware(), getLutzPerformance)
		api.POST("/lutz/reset", authMiddleware(), adminOnly(), resetLutz)
		api.PUT("/lutz/position/:id", authMiddleware(), adminOnly(), updateLutzPosition)
		api.PUT("/lutz/trade/:id", authMiddleware(), adminOnly(), updateLutzTrade)
		api.DELETE("/lutz/trade/:id", authMiddleware(), adminOnly(), deleteLutzTrade)
		api.GET("/lutz/pending", authMiddleware(), adminOnly(), getLutzPending)
		api.GET("/lutz/logs", authMiddleware(), getLutzLogs)
		api.GET("/lutz/todos", authMiddleware(), getLutzTodos)
		api.PUT("/lutz/todos/:id/done", authMiddleware(), adminOnly(), markLutzTodoDone)
		api.PUT("/lutz/todos/:id/reopen", authMiddleware(), adminOnly(), reopenLutzTodo)
		api.DELETE("/lutz/todos/:id", authMiddleware(), adminOnly(), deleteLutzTodo)
		api.POST("/lutz/todos/:id/execute", authMiddleware(), adminOnly(), executeLutzTodo)
		api.POST("/lutz/sync", authMiddleware(), adminOnly(), syncLutz)
		api.GET("/lutz/completed-trades", authMiddleware(), getLutzCompletedTrades)

		// Performance page - combined view of both bots
		api.GET("/performance/history", authMiddleware(), getPerformanceHistory)
	}

	r.Run(":8080")
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func checkPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func register(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email, username and password are required"})
		return
	}

	// Validate email format
	if !strings.Contains(req.Email, "@") || !strings.Contains(req.Email, ".") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	// Validate username length
	if len(req.Username) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username must be at least 3 characters"})
		return
	}

	// Validate password length
	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 6 characters"})
		return
	}

	// Check if email exists
	var existingUser User
	if err := db.Where("email = ?", strings.ToLower(req.Email)).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Check if username exists
	if err := db.Where("username = ?", strings.ToLower(req.Username)).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already taken"})
		return
	}

	// Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Check if this is the first user (make them admin)
	var userCount int64
	db.Model(&User{}).Count(&userCount)

	user := User{
		Email:    strings.ToLower(req.Email),
		Username: strings.ToLower(req.Username),
		Password: hashedPassword,
		IsAdmin:  userCount == 0, // First user becomes admin
	}

	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Create session
	token := createSession(user.ID, user.IsAdmin)

	c.JSON(http.StatusCreated, gin.H{
		"success":  true,
		"token":    token,
		"user":     gin.H{"id": user.ID, "email": user.Email, "username": user.Username, "is_admin": user.IsAdmin},
	})
}

func login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required"})
		return
	}

	var user User
	// Allow login with email or username
	if err := db.Where("email = ? OR username = ?", strings.ToLower(req.Email), strings.ToLower(req.Email)).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if !checkPassword(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token := createSession(user.ID, user.IsAdmin)

	// Update login count and last active
	db.Model(&user).Updates(map[string]interface{}{
		"login_count": user.LoginCount + 1,
		"last_active": time.Now(),
	})

	// Log activity
	logUserActivity(user.ID, user.Username, "login", "", c.ClientIP(), c.GetHeader("User-Agent"))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   token,
		"user":    gin.H{"id": user.ID, "email": user.Email, "username": user.Username, "is_admin": user.IsAdmin},
	})
}

func logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		deleteSession(token)
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func verifyToken(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"valid": false})
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	session, exists := getSession(token)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"valid": false})
		return
	}

	var user User
	if err := db.First(&user, session.UserID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"valid": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"user":  gin.H{"id": user.ID, "email": user.Email, "username": user.Username, "is_admin": user.IsAdmin},
	})
}

func getCurrentUser(c *gin.Context) {
	userID, _ := c.Get("userID")
	var user User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": user.ID, "email": user.Email, "username": user.Username, "is_admin": user.IsAdmin})
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization required"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		session, exists := getSession(token)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Update last active
		db.Model(&User{}).Where("id = ?", session.UserID).Update("last_active", time.Now())

		c.Set("userID", session.UserID)
		c.Set("isAdmin", session.IsAdmin)
		c.Next()
	}
}

func optionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			session, exists := getSession(token)
			if exists {
				c.Set("userID", session.UserID)
				c.Set("isAdmin", session.IsAdmin)
				db.Model(&User{}).Where("id = ?", session.UserID).Update("last_active", time.Now())
			}
		}
		c.Next()
	}
}

func adminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("isAdmin")
		if !exists || !isAdmin.(bool) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// Helper to log activity
func logUserActivity(userID uint, username, action, details, ip, userAgent string) {
	log := ActivityLog{
		UserID:    userID,
		Username:  username,
		Action:    action,
		Details:   details,
		IPAddress: ip,
		UserAgent: userAgent,
	}
	db.Create(&log)
}

func getStocks(c *gin.Context) {
	var stocks []Stock
	db.Order("created_at desc").Find(&stocks)

	if len(stocks) == 0 {
		c.JSON(http.StatusOK, []StockWithQuote{})
		return
	}

	// Load all categories for mapping
	var categories []Category
	db.Find(&categories)
	categoryMap := make(map[uint]string)
	for _, cat := range categories {
		categoryMap[cat.ID] = cat.Name
	}

	symbols := make([]string, len(stocks))
	for i, s := range stocks {
		symbols[i] = s.Symbol
	}

	quotes := fetchQuotes(symbols)

	result := make([]StockWithQuote, len(stocks))
	for i, stock := range stocks {
		result[i] = StockWithQuote{
			ID:         stock.ID,
			Symbol:     stock.Symbol,
			Name:       stock.Name,
			CategoryID: stock.CategoryID,
			CreatedAt:  stock.CreatedAt,
		}
		// Set category name
		if stock.CategoryID != nil {
			if name, ok := categoryMap[*stock.CategoryID]; ok {
				result[i].CategoryName = name
			}
		}
		if q, ok := quotes[stock.Symbol]; ok {
			result[i].Price = q.Price
			result[i].Change = q.Change
			result[i].ChangePercent = q.ChangePercent
			result[i].PrevClose = q.PrevClose
			result[i].Sector = q.Sector
			result[i].MarketCap = q.MarketCap
		}
	}

	c.JSON(http.StatusOK, result)
}

type QuoteData struct {
	Price         float64
	Change        float64
	ChangePercent float64
	PrevClose     float64
	Sector        string
	MarketCap     int64
}

func fetchQuotes(symbols []string) map[string]QuoteData {
	result := make(map[string]QuoteData)
	if len(symbols) == 0 {
		return result
	}

	// Yahoo limits to 20 symbols per request - batch them
	const batchSize = 20
	for i := 0; i < len(symbols); i += batchSize {
		end := i + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batch := symbols[i:end]

		// URL encode each symbol individually, then join with commas
		encodedSymbols := make([]string, len(batch))
		for j, s := range batch {
			encodedSymbols[j] = url.QueryEscape(s)
		}
		symbolsStr := strings.Join(encodedSymbols, ",")

		// Use spark API (v7 quote API is now blocked by Yahoo)
		sparkURL := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/spark?symbols=%s&range=1d&interval=1d", symbolsStr)
		batchResult := trySparkAPI(sparkURL)

		// Merge results
		for k, v := range batchResult {
			result[k] = v
		}
	}

	return result
}

// Simple spark response format: {"SYMBOL": {"close": [...], "chartPreviousClose": ...}}
type SimpleSparkData struct {
	Symbol             string    `json:"symbol"`
	Timestamp          []int64   `json:"timestamp"`
	Close              []float64 `json:"close"`
	ChartPreviousClose float64   `json:"chartPreviousClose"`
	PreviousClose      *float64  `json:"previousClose"`
}

func trySparkAPI(apiURL string) map[string]QuoteData {
	result := make(map[string]QuoteData)

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := httpClient.Do(req)
	if err != nil {
		return result
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return result
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Parse as map of symbol -> data
	var sparkResp map[string]SimpleSparkData
	if err := json.Unmarshal(body, &sparkResp); err != nil {
		return result
	}

	for symbol, data := range sparkResp {
		if len(data.Close) > 0 {
			price := data.Close[len(data.Close)-1]
			prevClose := data.ChartPreviousClose
			if prevClose == 0 && data.PreviousClose != nil {
				prevClose = *data.PreviousClose
			}
			change := price - prevClose
			changePercent := 0.0
			if prevClose > 0 {
				changePercent = (change / prevClose) * 100
			}
			result[symbol] = QuoteData{
				Price:         price,
				Change:        change,
				ChangePercent: changePercent,
				PrevClose:     prevClose,
			}
		}
	}

	return result
}

func searchStocks(c *gin.Context) {
	query := c.Query("q")
	if query == "" || len(query) < 1 {
		c.JSON(http.StatusOK, []SearchResult{})
		return
	}

	apiURL := fmt.Sprintf("https://query1.finance.yahoo.com/v1/finance/search?q=%s&quotesCount=10&newsCount=0", url.QueryEscape(query))

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, []SearchResult{})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var yahooResp YahooSearchResponse
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		c.JSON(http.StatusOK, []SearchResult{})
		return
	}

	results := make([]SearchResult, 0)
	for _, q := range yahooResp.Quotes {
		if q.QuoteType == "EQUITY" || q.QuoteType == "ETF" {
			name := q.LongName
			if name == "" {
				name = q.ShortName
			}
			results = append(results, SearchResult{
				Symbol:   q.Symbol,
				Name:     name,
				Type:     q.QuoteType,
				Exchange: q.Exchange,
			})
		}
	}

	c.JSON(http.StatusOK, results)
}

func getQuote(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))

	quotes := fetchQuotes([]string{symbol})
	if q, ok := quotes[symbol]; ok {
		c.JSON(http.StatusOK, gin.H{
			"symbol":         symbol,
			"price":          q.Price,
			"change":         q.Change,
			"change_percent": q.ChangePercent,
			"prev_close":     q.PrevClose,
		})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Quote not found"})
}

type OHLCV struct {
	Time   int64   `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

type YahooChartResponse struct {
	Chart struct {
		Result []struct {
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []float64 `json:"open"`
					High   []float64 `json:"high"`
					Low    []float64 `json:"low"`
					Close  []float64 `json:"close"`
					Volume []float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

func getHistory(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))
	period := c.DefaultQuery("period", "6mo")
	interval := c.DefaultQuery("interval", "1d")

	apiURL := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=%s&interval=%s",
		url.QueryEscape(symbol), period, interval)

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch data"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var yahooResp YahooChartResponse
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse data"})
		return
	}

	if len(yahooResp.Chart.Result) == 0 || len(yahooResp.Chart.Result[0].Timestamp) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No data found"})
		return
	}

	result := yahooResp.Chart.Result[0]
	quotes := result.Indicators.Quote[0]
	data := make([]OHLCV, 0)

	for i, ts := range result.Timestamp {
		if i < len(quotes.Open) && i < len(quotes.High) && i < len(quotes.Low) && i < len(quotes.Close) {
			if quotes.Close[i] > 0 {
				data = append(data, OHLCV{
					Time:   ts,
					Open:   quotes.Open[i],
					High:   quotes.High[i],
					Low:    quotes.Low[i],
					Close:  quotes.Close[i],
					Volume: quotes.Volume[i],
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol": symbol,
		"data":   data,
	})
}

func createStock(c *gin.Context) {
	userID, hasUser := c.Get("userID")
	isAdmin, _ := c.Get("isAdmin")

	// Check if user can add stocks
	canAdd := false
	var username string

	if hasUser {
		if isAdmin != nil && isAdmin.(bool) {
			canAdd = true
		} else {
			// Check if user has at least one portfolio position
			var count int64
			db.Model(&PortfolioPosition{}).Where("user_id = ?", userID).Count(&count)
			canAdd = count > 0
		}

		// Get username
		var user User
		if db.First(&user, userID).Error == nil {
			username = user.Username
		}
	}

	if !canAdd {
		c.JSON(http.StatusForbidden, gin.H{"error": "Du musst mindestens eine Aktie in deinem Portfolio haben um Aktien hinzuzufügen"})
		return
	}

	var req struct {
		Symbol string `json:"symbol"`
		Name   string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Symbol is required"})
		return
	}

	symbol := strings.ToUpper(req.Symbol)

	var existing Stock
	if err := db.Where("symbol = ?", symbol).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Stock already in watchlist"})
		return
	}

	name := req.Name
	if name == "" {
		name = symbol
	}

	// Find "Sonstiges" category for default assignment
	var sonstigesCategory Category
	var categoryID *uint
	if err := db.Where("name = ?", "Sonstiges").First(&sonstigesCategory).Error; err == nil {
		categoryID = &sonstigesCategory.ID
	}

	stock := Stock{
		Symbol:        symbol,
		Name:          name,
		CategoryID:    categoryID,
		AddedByUserID: userID.(uint),
		AddedByUser:   username,
	}

	db.Create(&stock)

	// Log activity
	logUserActivity(userID.(uint), username, "add_stock", fmt.Sprintf(`{"symbol":"%s"}`, symbol), c.ClientIP(), c.GetHeader("User-Agent"))

	c.JSON(http.StatusCreated, stock)
}

func deleteStock(c *gin.Context) {
	id := c.Param("id")
	var stock Stock
	if err := db.First(&stock, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stock not found"})
		return
	}
	db.Delete(&stock)
	c.JSON(http.StatusOK, gin.H{"message": "Stock deleted"})
}

// Exchange rates FROM USD TO other currencies
// Cached rates from frankfurter.app API
var exchangeRatesFromUSD = map[string]float64{
	"USD": 1.0,
	"EUR": 0.92,  // fallback
	"GBP": 0.79,  // fallback
	"CHF": 0.88,  // fallback
	"HKD": 7.8,   // fallback
	"JPY": 150.0, // fallback
	"CNY": 7.2,   // fallback
	"KRW": 1350,  // fallback
	"TWD": 32.0,  // fallback
	"INR": 83.0,  // fallback
	"AUD": 1.55,  // fallback
	"CAD": 1.36,  // fallback
}
var exchangeRatesLastFetched time.Time
var exchangeRatesMutex = &sync.Mutex{}

// FrankfurterResponse represents the API response from frankfurter.app
type FrankfurterResponse struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

// Fetch live exchange rates from multiple APIs
func fetchLiveExchangeRates() {
	exchangeRatesMutex.Lock()
	defer exchangeRatesMutex.Unlock()

	// Only fetch if rates are older than 1 hour
	if time.Since(exchangeRatesLastFetched) < time.Hour {
		return
	}

	// Try open.er-api.com first (has all currencies)
	apiURL := "https://open.er-api.com/v6/latest/USD"
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "FlipperCapital/1.0")

	resp, err := httpClient.Do(req)
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var erApiResp struct {
			Rates map[string]float64 `json:"rates"`
		}
		if err := json.Unmarshal(body, &erApiResp); err == nil && erApiResp.Rates != nil {
			// Update all rates we need
			currencies := []string{"EUR", "GBP", "CHF", "HKD", "JPY", "CNY", "KRW", "TWD", "INR", "AUD", "CAD"}
			for _, curr := range currencies {
				if rate, ok := erApiResp.Rates[curr]; ok {
					exchangeRatesFromUSD[curr] = rate
				}
			}
			exchangeRatesLastFetched = time.Now()
			fmt.Printf("Updated exchange rates: EUR=%.4f, HKD=%.4f, JPY=%.4f\n",
				exchangeRatesFromUSD["EUR"], exchangeRatesFromUSD["HKD"], exchangeRatesFromUSD["JPY"])
			return
		}
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Fallback to frankfurter.app for EUR, GBP, CHF
	apiURL = "https://api.frankfurter.app/latest?from=USD&to=EUR,GBP,CHF,JPY,AUD,CAD"
	req, _ = http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "FlipperCapital/1.0")

	resp, err = httpClient.Do(req)
	if err != nil {
		fmt.Println("Failed to fetch exchange rates:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Println("Exchange rate API returned status:", resp.StatusCode)
		return
	}

	body, _ := io.ReadAll(resp.Body)

	var frankfurterResp FrankfurterResponse
	if err := json.Unmarshal(body, &frankfurterResp); err != nil {
		fmt.Println("Failed to parse exchange rates:", err)
		return
	}

	// Update rates
	for currency, rate := range frankfurterResp.Rates {
		exchangeRatesFromUSD[currency] = rate
	}
	exchangeRatesLastFetched = time.Now()
	fmt.Printf("Updated exchange rates: EUR=%.4f, GBP=%.4f, CHF=%.4f\n",
		exchangeRatesFromUSD["EUR"], exchangeRatesFromUSD["GBP"], exchangeRatesFromUSD["CHF"])
}

// Get current exchange rate (fetches live if needed)
func getExchangeRate(currency string) float64 {
	// Trigger fetch if needed (non-blocking for first request after cache expires)
	go fetchLiveExchangeRates()

	exchangeRatesMutex.Lock()
	defer exchangeRatesMutex.Unlock()

	if rate, ok := exchangeRatesFromUSD[currency]; ok {
		return rate
	}
	return 1.0
}

// Convert USD price to user's currency using live rates
func convertFromUSD(usdAmount float64, toCurrency string) float64 {
	rate := getExchangeRate(toCurrency)
	return usdAmount * rate
}

func convertToUSD(amount float64, fromCurrency string) float64 {
	// Convert from user's currency to USD by dividing by the USD->currency rate
	rate := getExchangeRate(fromCurrency)
	if rate > 0 {
		return amount / rate
	}
	return amount // Default to USD if unknown
}

// Detect stock's native trading currency based on exchange suffix
func getStockCurrency(symbol string) string {
	s := strings.ToUpper(symbol)

	// European exchanges - EUR
	if strings.HasSuffix(s, ".PA") || strings.HasSuffix(s, ".DE") || strings.HasSuffix(s, ".F") ||
		strings.HasSuffix(s, ".AS") || strings.HasSuffix(s, ".BR") || strings.HasSuffix(s, ".MI") ||
		strings.HasSuffix(s, ".MC") || strings.HasSuffix(s, ".VI") || strings.HasSuffix(s, ".HE") ||
		strings.HasSuffix(s, ".LS") || strings.HasSuffix(s, ".IR") {
		return "EUR"
	}
	// London - GBP
	if strings.HasSuffix(s, ".L") {
		return "GBP"
	}
	// Swiss - CHF
	if strings.HasSuffix(s, ".SW") || strings.HasSuffix(s, ".VX") {
		return "CHF"
	}
	// Hong Kong - HKD
	if strings.HasSuffix(s, ".HK") {
		return "HKD"
	}
	// Japan - JPY
	if strings.HasSuffix(s, ".T") || strings.HasSuffix(s, ".TYO") {
		return "JPY"
	}
	// China - CNY
	if strings.HasSuffix(s, ".SS") || strings.HasSuffix(s, ".SZ") {
		return "CNY"
	}
	// Korea - KRW
	if strings.HasSuffix(s, ".KS") || strings.HasSuffix(s, ".KQ") {
		return "KRW"
	}
	// Taiwan - TWD
	if strings.HasSuffix(s, ".TW") || strings.HasSuffix(s, ".TWO") {
		return "TWD"
	}
	// India - INR
	if strings.HasSuffix(s, ".NS") || strings.HasSuffix(s, ".BO") {
		return "INR"
	}
	// Australia - AUD
	if strings.HasSuffix(s, ".AX") {
		return "AUD"
	}
	// Canada - CAD
	if strings.HasSuffix(s, ".TO") || strings.HasSuffix(s, ".V") {
		return "CAD"
	}
	// US exchanges or no suffix = USD
	return "USD"
}

// Convert stock price from its native currency to target currency
func convertStockPrice(price float64, symbol string, toCurrency string) float64 {
	stockCurrency := getStockCurrency(symbol)

	if stockCurrency == toCurrency {
		return price // No conversion needed
	}

	// First convert to USD, then to target currency
	priceInUSD := price
	if stockCurrency != "USD" {
		stockRate := getExchangeRate(stockCurrency)
		if stockRate > 0 {
			priceInUSD = price / stockRate
		}
	}

	// Then convert from USD to target
	if toCurrency == "USD" {
		return priceInUSD
	}
	return convertFromUSD(priceInUSD, toCurrency)
}

// Portfolio functions
func getPortfolio(c *gin.Context) {
	userID, _ := c.Get("userID")

	var positions []PortfolioPosition
	db.Where("user_id = ?", userID).Order("created_at desc").Find(&positions)

	if len(positions) == 0 {
		c.JSON(http.StatusOK, []PortfolioPositionWithQuote{})
		return
	}

	// Fetch current quotes
	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	result := make([]PortfolioPositionWithQuote, len(positions))
	for i, pos := range positions {
		currency := pos.Currency
		if currency == "" {
			currency = "EUR"
		}

		// Convert avg price to USD for internal calculations
		avgPriceUSD := convertToUSD(pos.AvgPrice, currency)

		result[i] = PortfolioPositionWithQuote{
			ID:           pos.ID,
			Symbol:       pos.Symbol,
			Name:         pos.Name,
			PurchaseDate: pos.PurchaseDate,
			AvgPrice:     pos.AvgPrice,
			AvgPriceUSD:  avgPriceUSD,
			Currency:     currency,
			Quantity:     pos.Quantity,
		}

		if q, ok := quotes[pos.Symbol]; ok {
			result[i].CurrentPrice = q.Price
			result[i].Change = q.Change
			result[i].ChangePercent = q.ChangePercent

			// Calculate returns by converting current stock price to user's currency
			// Stock price is in the stock's native currency (e.g., HKD for .HK stocks)
			currentPriceInUserCurrency := convertStockPrice(q.Price, pos.Symbol, currency)

			if pos.AvgPrice > 0 {
				// Return in user's currency terms
				result[i].TotalReturn = currentPriceInUserCurrency - pos.AvgPrice
				result[i].TotalReturnPct = ((currentPriceInUserCurrency - pos.AvgPrice) / pos.AvgPrice) * 100
			}

			// Calculate values if quantity is set (convert to user's currency)
			if pos.Quantity != nil && *pos.Quantity > 0 {
				result[i].CurrentValue = currentPriceInUserCurrency * (*pos.Quantity)
				result[i].InvestedValue = pos.AvgPrice * (*pos.Quantity)
			}
		}
	}

	c.JSON(http.StatusOK, result)
}

func createPortfolioPosition(c *gin.Context) {
	userID, _ := c.Get("userID")

	var req struct {
		Symbol       string   `json:"symbol" binding:"required"`
		Name         string   `json:"name"`
		PurchaseDate *string  `json:"purchase_date"`
		AvgPrice     float64  `json:"avg_price" binding:"required"`
		Currency     string   `json:"currency"`
		Quantity     *float64 `json:"quantity"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Symbol and avg_price are required"})
		return
	}

	if req.AvgPrice <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Average price must be greater than 0"})
		return
	}

	symbol := strings.ToUpper(req.Symbol)
	name := req.Name
	if name == "" {
		name = symbol
	}

	currency := req.Currency
	if currency == "" {
		currency = "EUR"
	}

	var purchaseDate *time.Time
	if req.PurchaseDate != nil && *req.PurchaseDate != "" {
		parsed, err := time.Parse("2006-01-02", *req.PurchaseDate)
		if err == nil {
			purchaseDate = &parsed
		}
	}

	position := PortfolioPosition{
		UserID:       userID.(uint),
		Symbol:       symbol,
		Name:         name,
		PurchaseDate: purchaseDate,
		AvgPrice:     req.AvgPrice,
		Currency:     currency,
		Quantity:     req.Quantity,
	}

	if err := db.Create(&position).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create position"})
		return
	}

	c.JSON(http.StatusCreated, position)
}

func updatePortfolioPosition(c *gin.Context) {
	userID, _ := c.Get("userID")
	id := c.Param("id")

	var position PortfolioPosition
	if err := db.Where("id = ? AND user_id = ?", id, userID).First(&position).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found"})
		return
	}

	var req struct {
		Symbol       string   `json:"symbol"`
		Name         string   `json:"name"`
		PurchaseDate *string  `json:"purchase_date"`
		AvgPrice     float64  `json:"avg_price"`
		Currency     string   `json:"currency"`
		Quantity     *float64 `json:"quantity"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.Symbol != "" {
		position.Symbol = strings.ToUpper(req.Symbol)
	}
	if req.Name != "" {
		position.Name = req.Name
	}
	if req.AvgPrice > 0 {
		position.AvgPrice = req.AvgPrice
	}
	if req.Currency != "" {
		position.Currency = req.Currency
	}
	if req.Quantity != nil {
		position.Quantity = req.Quantity
	}
	if req.PurchaseDate != nil {
		if *req.PurchaseDate == "" {
			position.PurchaseDate = nil
		} else {
			parsed, err := time.Parse("2006-01-02", *req.PurchaseDate)
			if err == nil {
				position.PurchaseDate = &parsed
			}
		}
	}

	db.Save(&position)
	c.JSON(http.StatusOK, position)
}

func deletePortfolioPosition(c *gin.Context) {
	userID, _ := c.Get("userID")
	id := c.Param("id")

	var position PortfolioPosition
	if err := db.Where("id = ? AND user_id = ?", id, userID).First(&position).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found"})
		return
	}

	db.Delete(&position)
	c.JSON(http.StatusOK, gin.H{"message": "Position deleted"})
}

func sellPortfolioPosition(c *gin.Context) {
	userID, _ := c.Get("userID")
	id := c.Param("id")

	var position PortfolioPosition
	if err := db.Where("id = ? AND user_id = ?", id, userID).First(&position).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found"})
		return
	}

	var input struct {
		SellPrice float64 `json:"sell_price" binding:"required"`
		Quantity  *float64 `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sell_price is required"})
		return
	}

	// Calculate profit/loss
	quantity := 1.0
	if position.Quantity != nil && *position.Quantity > 0 {
		quantity = *position.Quantity
	}
	if input.Quantity != nil && *input.Quantity > 0 {
		quantity = *input.Quantity
	}

	profitLoss := (input.SellPrice - position.AvgPrice) * quantity
	profitLossPct := 0.0
	if position.AvgPrice > 0 {
		profitLossPct = ((input.SellPrice - position.AvgPrice) / position.AvgPrice) * 100
	}

	// Create trade history entry
	tradeHistory := PortfolioTradeHistory{
		UserID:        userID.(uint),
		Symbol:        position.Symbol,
		Name:          position.Name,
		BuyPrice:      position.AvgPrice,
		SellPrice:     input.SellPrice,
		Currency:      position.Currency,
		Quantity:      quantity,
		BuyDate:       position.PurchaseDate,
		SellDate:      time.Now(),
		ProfitLoss:    profitLoss,
		ProfitLossPct: profitLossPct,
	}

	if err := db.Create(&tradeHistory).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create trade history"})
		return
	}

	// Delete the position
	db.Delete(&position)

	c.JSON(http.StatusOK, gin.H{
		"message":        "Position sold successfully",
		"profit_loss":    profitLoss,
		"profit_loss_pct": profitLossPct,
		"trade":          tradeHistory,
	})
}

func getPortfolioTrades(c *gin.Context) {
	userID, _ := c.Get("userID")

	var trades []PortfolioTradeHistory
	db.Where("user_id = ?", userID).Order("sell_date desc").Find(&trades)

	c.JSON(http.StatusOK, trades)
}

func getPortfolioPerformance(c *gin.Context) {
	userID, _ := c.Get("userID")

	var positions []PortfolioPosition
	db.Where("user_id = ?", userID).Find(&positions)

	if len(positions) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"total_value":     0,
			"total_invested":  0,
			"total_return":    0,
			"total_return_pct": 0,
			"positions_count": 0,
			"period_changes":  gin.H{},
		})
		return
	}

	// Fetch current quotes
	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	// Check if any position has quantity set
	hasQuantities := false
	for _, p := range positions {
		if p.Quantity != nil && *p.Quantity > 0 {
			hasQuantities = true
			break
		}
	}

	var totalValue, totalInvested float64
	var totalReturnPct float64

	if hasQuantities {
		// Calculate with actual quantities
		for _, pos := range positions {
			if q, ok := quotes[pos.Symbol]; ok && pos.Quantity != nil && *pos.Quantity > 0 {
				currency := pos.Currency
				if currency == "" {
					currency = "EUR"
				}
				// Convert current stock price to user's currency for proper comparison
				currentPriceInUserCurrency := convertStockPrice(q.Price, pos.Symbol, currency)

				// Calculate in user's currency
				totalValue += currentPriceInUserCurrency * (*pos.Quantity)
				totalInvested += pos.AvgPrice * (*pos.Quantity)
			}
		}
	} else {
		// Equal weight assumption - calculate average return
		validPositions := 0
		for _, pos := range positions {
			if q, ok := quotes[pos.Symbol]; ok && pos.AvgPrice > 0 {
				currency := pos.Currency
				if currency == "" {
					currency = "EUR"
				}
				// Convert current stock price to user's currency for proper comparison
				currentPriceInUserCurrency := convertStockPrice(q.Price, pos.Symbol, currency)

				returnPct := ((currentPriceInUserCurrency - pos.AvgPrice) / pos.AvgPrice) * 100
				totalReturnPct += returnPct
				validPositions++
			}
		}
		if validPositions > 0 {
			totalReturnPct = totalReturnPct / float64(validPositions)
		}
		// Use placeholder values for display
		totalInvested = 10000 // Assume 10k invested for display
		totalValue = totalInvested * (1 + totalReturnPct/100)
	}

	totalReturn := totalValue - totalInvested
	if hasQuantities && totalInvested > 0 {
		totalReturnPct = (totalReturn / totalInvested) * 100
	}

	// Fetch historical data for period changes
	periodChanges := calculatePeriodChanges(positions, quotes)

	c.JSON(http.StatusOK, gin.H{
		"total_value":      totalValue,
		"total_invested":   totalInvested,
		"total_return":     totalReturn,
		"total_return_pct": totalReturnPct,
		"positions_count":  len(positions),
		"has_quantities":   hasQuantities,
		"period_changes":   periodChanges,
	})
}

func calculatePeriodChanges(positions []PortfolioPosition, currentQuotes map[string]QuoteData) map[string]float64 {
	// Period durations in days (approximate)
	periodDays := map[string]int{
		"1d":  1,
		"1w":  7,
		"1m":  30,
		"3m":  90,
		"6m":  180,
		"ytd": getDaysFromYearStart(),
		"1y":  365,
		"5y":  1825,
	}

	periods := map[string]string{
		"1d":  "1d",
		"1w":  "5d",
		"1m":  "1mo",
		"3m":  "3mo",
		"6m":  "6mo",
		"ytd": "ytd",
		"1y":  "1y",
		"5y":  "5y",
	}

	result := make(map[string]float64)
	now := time.Now()

	// For day change, we can use the current quotes (only for positions owned at least 1 day)
	dayChange := 0.0
	validCount := 0
	for _, pos := range positions {
		// Skip if position was purchased today or has no purchase date
		if pos.PurchaseDate != nil && pos.PurchaseDate.After(now.AddDate(0, 0, -1)) {
			continue
		}
		if q, ok := currentQuotes[pos.Symbol]; ok && q.PrevClose > 0 {
			dayChange += q.ChangePercent
			validCount++
		}
	}
	if validCount > 0 {
		result["1d"] = dayChange / float64(validCount)
	}

	// For other periods, fetch historical data
	for periodKey, yahooRange := range periods {
		if periodKey == "1d" {
			continue
		}

		totalChange := 0.0
		count := 0
		periodStartDate := now.AddDate(0, 0, -periodDays[periodKey])

		for _, pos := range positions {
			currency := pos.Currency
			if currency == "" {
				currency = "EUR"
			}

			// Only include position if it was purchased before or at the start of the period
			if pos.PurchaseDate != nil && pos.PurchaseDate.After(periodStartDate) {
				// Position was purchased during this period - calculate from purchase date
				if q, ok := currentQuotes[pos.Symbol]; ok && q.Price > 0 {
					// Convert current stock price to user's currency for proper comparison
					currentPriceInUserCurrency := convertStockPrice(q.Price, pos.Symbol, currency)
					if pos.AvgPrice > 0 {
						change := ((currentPriceInUserCurrency - pos.AvgPrice) / pos.AvgPrice) * 100
						totalChange += change
						count++
					}
				}
				continue
			}

			// Position was owned before this period - use historical price
			histPrice := getHistoricalPrice(pos.Symbol, yahooRange)
			if histPrice > 0 {
				if q, ok := currentQuotes[pos.Symbol]; ok && q.Price > 0 {
					change := ((q.Price - histPrice) / histPrice) * 100
					totalChange += change
					count++
				}
			}
		}

		if count > 0 {
			result[periodKey] = totalChange / float64(count)
		}
	}

	return result
}

func getDaysFromYearStart() int {
	now := time.Now()
	yearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	return int(now.Sub(yearStart).Hours() / 24)
}

func getHistoricalPrice(symbol string, period string) float64 {
	apiURL := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=%s&interval=1d",
		url.QueryEscape(symbol), period)

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return 0
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var yahooResp YahooChartResponse
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return 0
	}

	if len(yahooResp.Chart.Result) == 0 {
		return 0
	}

	result := yahooResp.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 || len(result.Indicators.Quote[0].Close) == 0 {
		return 0
	}

	// Return the first valid close price
	for _, price := range result.Indicators.Quote[0].Close {
		if price > 0 {
			return price
		}
	}

	return 0
}

// Get all portfolios for comparison (public view)
func getAllPortfoliosForComparison(c *gin.Context) {
	// Get all users with positions
	var users []User
	db.Find(&users)

	type PositionSummary struct {
		Symbol         string  `json:"symbol"`
		Name           string  `json:"name"`
		CurrentPrice   float64 `json:"current_price"`
		AvgPrice       float64 `json:"avg_price"`
		AvgPriceUSD    float64 `json:"avg_price_usd"`
		Currency       string  `json:"currency"`
		TotalReturnPct float64 `json:"total_return_pct"`
		ChangePercent  float64 `json:"change_percent"`
		IsLive         bool    `json:"is_live"`
	}

	type PortfolioSummary struct {
		UserID         uint              `json:"user_id"`
		Username       string            `json:"username"`
		Positions      []PositionSummary `json:"positions"`
		TotalReturnPct float64           `json:"total_return_pct"`
		PositionCount  int               `json:"position_count"`
	}

	var portfolios []PortfolioSummary

	// Collect all unique symbols for batch quote fetch
	allSymbols := make(map[string]bool)
	userPositions := make(map[uint][]PortfolioPosition)

	for _, user := range users {
		var positions []PortfolioPosition
		db.Where("user_id = ?", user.ID).Find(&positions)
		if len(positions) > 0 {
			userPositions[user.ID] = positions
			for _, p := range positions {
				allSymbols[p.Symbol] = true
			}
		}
	}

	// Fetch all quotes at once
	symbols := make([]string, 0, len(allSymbols))
	for s := range allSymbols {
		symbols = append(symbols, s)
	}
	quotes := fetchQuotes(symbols)

	// Build portfolio summaries
	for _, user := range users {
		positions, exists := userPositions[user.ID]
		if !exists || len(positions) == 0 {
			continue
		}

		var posSummaries []PositionSummary
		totalInvested := 0.0
		totalUnrealizedGain := 0.0

		for _, pos := range positions {
			currency := pos.Currency
			if currency == "" {
				currency = "EUR"
			}
			avgPriceUSD := convertToUSD(pos.AvgPrice, currency)

			// Get quantity (default to 1 if not set)
			qty := 1.0
			if pos.Quantity != nil && *pos.Quantity > 0 {
				qty = *pos.Quantity
			}

			summary := PositionSummary{
				Symbol:      pos.Symbol,
				Name:        pos.Name,
				AvgPrice:    pos.AvgPrice,
				AvgPriceUSD: avgPriceUSD,
				Currency:    currency,
			}

			// Check if this is a bot position and get is_live status
			if user.ID == FLIPPERBOT_USER_ID {
				var botPos FlipperBotPosition
				if db.Where("symbol = ?", pos.Symbol).First(&botPos).Error == nil {
					summary.IsLive = botPos.IsLive
				}
			} else if user.ID == LUTZ_USER_ID {
				var botPos LutzPosition
				if db.Where("symbol = ?", pos.Symbol).First(&botPos).Error == nil {
					summary.IsLive = botPos.IsLive
				}
			}

			if q, ok := quotes[pos.Symbol]; ok {
				summary.CurrentPrice = q.Price
				summary.ChangePercent = q.ChangePercent
				// Convert current stock price to user's currency for proper return calculation
				currentPriceInUserCurrency := convertStockPrice(q.Price, pos.Symbol, currency)
				if pos.AvgPrice > 0 {
					summary.TotalReturnPct = ((currentPriceInUserCurrency - pos.AvgPrice) / pos.AvgPrice) * 100
					// Calculate weighted values for portfolio return
					invested := pos.AvgPrice * qty
					currentValue := currentPriceInUserCurrency * qty
					totalInvested += invested
					totalUnrealizedGain += (currentValue - invested)
				}
			}

			posSummaries = append(posSummaries, summary)
		}

		// Calculate weighted average return based on investment amounts
		weightedReturn := 0.0
		if totalInvested > 0 {
			weightedReturn = (totalUnrealizedGain / totalInvested) * 100
		}

		portfolios = append(portfolios, PortfolioSummary{
			UserID:         user.ID,
			Username:       user.Username,
			Positions:      posSummaries,
			TotalReturnPct: weightedReturn,
			PositionCount:  len(positions),
		})
	}

	c.JSON(http.StatusOK, portfolios)
}

// Get historical portfolio performance data for charting
func getPortfolioHistory(c *gin.Context) {
	userID, _ := c.Get("userID")
	period := c.DefaultQuery("period", "1mo")

	history := calculatePortfolioHistoryForUser(userID.(uint), period)
	c.JSON(http.StatusOK, history)
}

// Get historical portfolio performance for a specific user (for comparison)
func getUserPortfolioHistory(c *gin.Context) {
	userIDParam := c.Param("userId")
	period := c.DefaultQuery("period", "1mo")

	var userID uint
	fmt.Sscanf(userIDParam, "%d", &userID)

	history := calculatePortfolioHistoryForUser(userID, period)
	c.JSON(http.StatusOK, history)
}

// Get historical portfolio performance for ALL users (for comparison chart)
func getAllPortfoliosHistory(c *gin.Context) {
	period := c.DefaultQuery("period", "1mo")

	// Get all users with positions
	var users []User
	db.Find(&users)

	type PortfolioHistory struct {
		UserID   uint                     `json:"user_id"`
		Username string                   `json:"username"`
		History  []map[string]interface{} `json:"history"`
	}

	var result []PortfolioHistory

	for _, user := range users {
		// Check if user has any positions
		var count int64
		db.Model(&PortfolioPosition{}).Where("user_id = ?", user.ID).Count(&count)
		if count == 0 {
			continue
		}

		history := calculatePortfolioHistoryForUser(user.ID, period)
		if len(history) > 0 {
			result = append(result, PortfolioHistory{
				UserID:   user.ID,
				Username: user.Username,
				History:  history,
			})
		}
	}

	c.JSON(http.StatusOK, result)
}

func calculatePortfolioHistoryForUser(userID uint, period string) []map[string]interface{} {
	var positions []PortfolioPosition
	db.Where("user_id = ?", userID).Find(&positions)

	if len(positions) == 0 {
		return []map[string]interface{}{}
	}

	// Map period to Yahoo Finance range
	yahooRange := "1mo"
	switch period {
	case "1w":
		yahooRange = "5d"
	case "1m":
		yahooRange = "1mo"
	case "3m":
		yahooRange = "3mo"
	case "6m":
		yahooRange = "6mo"
	case "1y":
		yahooRange = "1y"
	case "ytd":
		yahooRange = "ytd"
	case "5y":
		yahooRange = "5y"
	}

	// Collect symbols and fetch historical data
	symbolData := make(map[string][]OHLCV)
	for _, pos := range positions {
		data := fetchHistoricalData(pos.Symbol, yahooRange)
		if len(data) > 0 {
			symbolData[pos.Symbol] = data
		}
	}

	if len(symbolData) == 0 {
		return []map[string]interface{}{}
	}

	// Find the common time range across all symbols
	var allTimes []int64
	timeValues := make(map[int64]map[string]float64) // time -> symbol -> close price

	// First pass: collect all timestamps
	for symbol, data := range symbolData {
		for _, candle := range data {
			if _, exists := timeValues[candle.Time]; !exists {
				timeValues[candle.Time] = make(map[string]float64)
				allTimes = append(allTimes, candle.Time)
			}
			timeValues[candle.Time][symbol] = candle.Close
		}
	}

	// Sort times
	for i := 0; i < len(allTimes)-1; i++ {
		for j := i + 1; j < len(allTimes); j++ {
			if allTimes[i] > allTimes[j] {
				allTimes[i], allTimes[j] = allTimes[j], allTimes[i]
			}
		}
	}

	// Calculate portfolio value at each time point
	result := make([]map[string]interface{}, 0)

	// Get initial invested value for normalization
	// Use user's original currency values for consistency
	var totalInvested float64
	hasQuantities := false
	for _, pos := range positions {
		if pos.Quantity != nil && *pos.Quantity > 0 {
			hasQuantities = true
			totalInvested += pos.AvgPrice * (*pos.Quantity)
		}
	}

	if !hasQuantities {
		// Assume equal investment of 1000 per position for visualization
		totalInvested = float64(len(positions)) * 1000
	}

	// Track last known prices for each symbol (for filling gaps)
	lastPrices := make(map[string]float64)

	for _, t := range allTimes {
		prices := timeValues[t]

		// Update last known prices
		for symbol, price := range prices {
			lastPrices[symbol] = price
		}

		// Calculate portfolio value at this time
		var portfolioValue float64

		if hasQuantities {
			for _, pos := range positions {
				if pos.Quantity != nil && *pos.Quantity > 0 {
					if price, ok := lastPrices[pos.Symbol]; ok {
						// Convert stock price to user's currency
						currency := pos.Currency
						if currency == "" {
							currency = "EUR"
						}
						priceInUserCurrency := convertStockPrice(price, pos.Symbol, currency)
						portfolioValue += priceInUserCurrency * (*pos.Quantity)
					}
				}
			}
		} else {
			// Equal weight: 1000 per position, calculate based on price change ratio
			for _, pos := range positions {
				if price, ok := lastPrices[pos.Symbol]; ok {
					currency := pos.Currency
					if currency == "" {
						currency = "EUR"
					}
					// Convert stock price to user's currency for proper comparison
					priceInUserCurrency := convertStockPrice(price, pos.Symbol, currency)
					if pos.AvgPrice > 0 {
						// Value = initial investment * (current price / purchase price)
						portfolioValue += 1000 * (priceInUserCurrency / pos.AvgPrice)
					}
				}
			}
		}

		if portfolioValue > 0 {
			// Calculate percentage change from initial investment
			pctChange := ((portfolioValue - totalInvested) / totalInvested) * 100

			result = append(result, map[string]interface{}{
				"time":  t,
				"value": portfolioValue,
				"pct":   pctChange,
			})
		}
	}

	return result
}

func fetchHistoricalData(symbol string, period string) []OHLCV {
	apiURL := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=%s&interval=1d",
		url.QueryEscape(symbol), period)

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var yahooResp YahooChartResponse
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return nil
	}

	if len(yahooResp.Chart.Result) == 0 || len(yahooResp.Chart.Result[0].Timestamp) == 0 {
		return nil
	}

	result := yahooResp.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 {
		return nil
	}

	quotes := result.Indicators.Quote[0]
	data := make([]OHLCV, 0)

	for i, ts := range result.Timestamp {
		if i < len(quotes.Close) && quotes.Close[i] > 0 {
			open := quotes.Close[i]
			high := quotes.Close[i]
			low := quotes.Close[i]
			if i < len(quotes.Open) && quotes.Open[i] > 0 {
				open = quotes.Open[i]
			}
			if i < len(quotes.High) && quotes.High[i] > 0 {
				high = quotes.High[i]
			}
			if i < len(quotes.Low) && quotes.Low[i] > 0 {
				low = quotes.Low[i]
			}
			volume := 0.0
			if i < len(quotes.Volume) {
				volume = quotes.Volume[i]
			}

			data = append(data, OHLCV{
				Time:   ts,
				Open:   open,
				High:   high,
				Low:    low,
				Close:  quotes.Close[i],
				Volume: volume,
			})
		}
	}

	return data
}

// getPriceAtDate fetches the closing price for a symbol at a specific date
func getPriceAtDate(symbol string, targetDate time.Time) float64 {
	// Fetch enough historical data to cover the target date
	now := time.Now()
	daysSince := int(now.Sub(targetDate).Hours() / 24)

	// Add buffer and use appropriate range
	var period string
	if daysSince <= 5 {
		period = "5d"
	} else if daysSince <= 30 {
		period = "1mo"
	} else if daysSince <= 90 {
		period = "3mo"
	} else if daysSince <= 180 {
		period = "6mo"
	} else {
		period = "1y"
	}

	data := fetchHistoricalData(symbol, period)
	if data == nil || len(data) == 0 {
		return 0
	}

	// Find the closest price to the target date
	targetUnix := targetDate.Unix()
	var closestPrice float64
	var closestDiff int64 = 999999999

	for _, d := range data {
		diff := abs64(d.Time - targetUnix)
		if diff < closestDiff {
			closestDiff = diff
			closestPrice = d.Close
		}
	}

	// Only use the price if it's within 5 days of the target date
	if closestDiff <= 5*24*60*60 {
		return closestPrice
	}

	return 0
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// adjustToTradingDay adjusts a date to the nearest valid trading day (weekday)
// It moves backwards to find a weekday (Mon-Fri)
func adjustToTradingDay(date time.Time) time.Time {
	weekday := date.Weekday()

	// If Saturday, go back to Friday
	if weekday == time.Saturday {
		return date.AddDate(0, 0, -1)
	}
	// If Sunday, go back to Friday
	if weekday == time.Sunday {
		return date.AddDate(0, 0, -2)
	}

	// Already a weekday
	return date
}

// isWeekend checks if a date is a weekend
func isWeekend(date time.Time) bool {
	weekday := date.Weekday()
	return weekday == time.Saturday || weekday == time.Sunday
}

// Stock Performance Tracker handlers

func saveStockPerformance(c *gin.Context) {
	var req struct {
		Symbol       string      `json:"symbol" binding:"required"`
		Name         string      `json:"name"`
		WinRate      float64     `json:"win_rate"`
		RiskReward   float64     `json:"risk_reward"`
		TotalReturn  float64     `json:"total_return"`
		TotalTrades  int         `json:"total_trades"`
		Wins         int         `json:"wins"`
		Losses       int         `json:"losses"`
		Signal       string      `json:"signal"`
		SignalBars   int         `json:"signal_bars"`
		Trades       []TradeData `json:"trades"`
		CurrentPrice float64     `json:"current_price"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	symbol := strings.ToUpper(req.Symbol)

	// Convert trades to JSON
	tradesJSON, _ := json.Marshal(req.Trades)

	// Check if performance record exists
	var existing StockPerformance
	result := db.Where("symbol = ?", symbol).First(&existing)

	if result.Error == nil {
		// Update existing
		existing.Name = req.Name
		existing.WinRate = req.WinRate
		existing.RiskReward = req.RiskReward
		existing.TotalReturn = req.TotalReturn
		existing.TotalTrades = req.TotalTrades
		existing.Wins = req.Wins
		existing.Losses = req.Losses
		existing.Signal = req.Signal
		existing.SignalBars = req.SignalBars
		existing.TradesJSON = string(tradesJSON)
		existing.CurrentPrice = req.CurrentPrice
		existing.UpdatedAt = time.Now()
		db.Save(&existing)
		c.JSON(http.StatusOK, existing)
	} else {
		// Create new
		perf := StockPerformance{
			Symbol:       symbol,
			Name:         req.Name,
			WinRate:      req.WinRate,
			RiskReward:   req.RiskReward,
			TotalReturn:  req.TotalReturn,
			TotalTrades:  req.TotalTrades,
			Wins:         req.Wins,
			Losses:       req.Losses,
			Signal:       req.Signal,
			SignalBars:   req.SignalBars,
			TradesJSON:   string(tradesJSON),
			CurrentPrice: req.CurrentPrice,
		}
		db.Create(&perf)
		c.JSON(http.StatusCreated, perf)
	}
}

func getTrackedStocks(c *gin.Context) {
	var performances []StockPerformance
	db.Order("updated_at desc").Find(&performances)

	// Parse trades JSON for each
	type PerformanceWithTrades struct {
		StockPerformance
		Trades []TradeData `json:"trades"`
	}

	result := make([]PerformanceWithTrades, len(performances))
	for i, p := range performances {
		result[i].StockPerformance = p
		if p.TradesJSON != "" {
			json.Unmarshal([]byte(p.TradesJSON), &result[i].Trades)
		}
	}

	c.JSON(http.StatusOK, result)
}

func getStockPerformance(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))

	var perf StockPerformance
	if err := db.Where("symbol = ?", symbol).First(&perf).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stock not found"})
		return
	}

	// Parse trades
	var trades []TradeData
	if perf.TradesJSON != "" {
		json.Unmarshal([]byte(perf.TradesJSON), &trades)
	}

	c.JSON(http.StatusOK, gin.H{
		"performance": perf,
		"trades":      trades,
	})
}

// Aggressive mode performance handlers
func saveAggressiveStockPerformance(c *gin.Context) {
	var req struct {
		Symbol       string      `json:"symbol" binding:"required"`
		Name         string      `json:"name"`
		WinRate      float64     `json:"win_rate"`
		RiskReward   float64     `json:"risk_reward"`
		TotalReturn  float64     `json:"total_return"`
		TotalTrades  int         `json:"total_trades"`
		Wins         int         `json:"wins"`
		Losses       int         `json:"losses"`
		Signal       string      `json:"signal"`
		SignalBars   int         `json:"signal_bars"`
		Trades       []TradeData `json:"trades"`
		CurrentPrice float64     `json:"current_price"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	symbol := strings.ToUpper(req.Symbol)
	tradesJSON, _ := json.Marshal(req.Trades)

	var existing AggressiveStockPerformance
	result := db.Where("symbol = ?", symbol).First(&existing)

	if result.Error == nil {
		existing.Name = req.Name
		existing.WinRate = req.WinRate
		existing.RiskReward = req.RiskReward
		existing.TotalReturn = req.TotalReturn
		existing.TotalTrades = req.TotalTrades
		existing.Wins = req.Wins
		existing.Losses = req.Losses
		existing.Signal = req.Signal
		existing.SignalBars = req.SignalBars
		existing.TradesJSON = string(tradesJSON)
		existing.CurrentPrice = req.CurrentPrice
		existing.UpdatedAt = time.Now()
		db.Save(&existing)
		c.JSON(http.StatusOK, existing)
	} else {
		perf := AggressiveStockPerformance{
			Symbol:       symbol,
			Name:         req.Name,
			WinRate:      req.WinRate,
			RiskReward:   req.RiskReward,
			TotalReturn:  req.TotalReturn,
			TotalTrades:  req.TotalTrades,
			Wins:         req.Wins,
			Losses:       req.Losses,
			Signal:       req.Signal,
			SignalBars:   req.SignalBars,
			TradesJSON:   string(tradesJSON),
			CurrentPrice: req.CurrentPrice,
		}
		db.Create(&perf)
		c.JSON(http.StatusCreated, perf)
	}
}

func getAggressiveTrackedStocks(c *gin.Context) {
	var performances []AggressiveStockPerformance
	db.Order("updated_at desc").Find(&performances)

	type PerformanceWithTrades struct {
		AggressiveStockPerformance
		Trades []TradeData `json:"trades"`
	}

	result := make([]PerformanceWithTrades, len(performances))
	for i, p := range performances {
		result[i].AggressiveStockPerformance = p
		if p.TradesJSON != "" {
			json.Unmarshal([]byte(p.TradesJSON), &result[i].Trades)
		}
	}

	c.JSON(http.StatusOK, result)
}

func getAggressiveStockPerformance(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))

	var perf AggressiveStockPerformance
	if err := db.Where("symbol = ?", symbol).First(&perf).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stock not found"})
		return
	}

	var trades []TradeData
	if perf.TradesJSON != "" {
		json.Unmarshal([]byte(perf.TradesJSON), &trades)
	}

	c.JSON(http.StatusOK, gin.H{
		"performance": perf,
		"trades":      trades,
	})
}

// Check if user can add stocks to watchlist
func canAddStocks(c *gin.Context) {
	userID, hasUser := c.Get("userID")
	isAdmin, _ := c.Get("isAdmin")

	if !hasUser {
		c.JSON(http.StatusOK, gin.H{
			"can_add":  false,
			"reason":   "not_logged_in",
			"message":  "Melde dich an und pflege mindestens eine Aktie in deinem Portfolio ein, um Aktien zur Watchlist hinzuzufügen.",
		})
		return
	}

	if isAdmin != nil && isAdmin.(bool) {
		c.JSON(http.StatusOK, gin.H{
			"can_add":  true,
			"reason":   "admin",
			"message":  "",
		})
		return
	}

	// Check if user has at least one portfolio position
	var count int64
	db.Model(&PortfolioPosition{}).Where("user_id = ?", userID).Count(&count)

	if count > 0 {
		c.JSON(http.StatusOK, gin.H{
			"can_add":  true,
			"reason":   "has_portfolio",
			"message":  "",
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"can_add":  false,
			"reason":   "no_portfolio",
			"message":  "Pflege mindestens eine Aktie in deinem Portfolio ein, um Aktien zur Watchlist hinzuzufügen.",
		})
	}
}

// Log user activity from frontend
func logActivity(c *gin.Context) {
	var req struct {
		Action  string `json:"action"`
		Details string `json:"details"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	userID, hasUser := c.Get("userID")
	var uid uint
	var username string

	if hasUser {
		uid = userID.(uint)
		var user User
		if db.First(&user, uid).Error == nil {
			username = user.Username
		}
	}

	logUserActivity(uid, username, req.Action, req.Details, c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Admin: Get all users with stats
func getAdminUsers(c *gin.Context) {
	type UserWithStats struct {
		User
		PortfolioCount int64 `json:"portfolio_count"`
		ActivityCount  int64 `json:"activity_count"`
	}

	var users []User
	db.Order("created_at desc").Find(&users)

	result := make([]UserWithStats, len(users))
	for i, u := range users {
		result[i].User = u

		// Count portfolio positions
		db.Model(&PortfolioPosition{}).Where("user_id = ?", u.ID).Count(&result[i].PortfolioCount)

		// Count activities
		db.Model(&ActivityLog{}).Where("user_id = ?", u.ID).Count(&result[i].ActivityCount)
	}

	c.JSON(http.StatusOK, result)
}

// Admin: Delete user
func deleteAdminUser(c *gin.Context) {
	id := c.Param("id")

	var user User
	if err := db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Don't allow deleting the last admin
	if user.IsAdmin {
		var adminCount int64
		db.Model(&User{}).Where("is_admin = ?", true).Count(&adminCount)
		if adminCount <= 1 {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete the last admin"})
			return
		}
	}

	// Delete user's portfolio positions
	db.Where("user_id = ?", user.ID).Delete(&PortfolioPosition{})

	// Delete user's activity logs
	db.Where("user_id = ?", user.ID).Delete(&ActivityLog{})

	// Delete user
	db.Delete(&user)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Admin: Update user
func updateAdminUser(c *gin.Context) {
	id := c.Param("id")

	var user User
	if err := db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		IsAdmin  *bool  `json:"is_admin"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.Username != "" {
		user.Username = req.Username
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.IsAdmin != nil {
		// Don't allow removing admin from last admin
		if user.IsAdmin && !*req.IsAdmin {
			var adminCount int64
			db.Model(&User{}).Where("is_admin = ?", true).Count(&adminCount)
			if adminCount <= 1 {
				c.JSON(http.StatusForbidden, gin.H{"error": "Cannot remove admin from last admin"})
				return
			}
		}
		user.IsAdmin = *req.IsAdmin
	}

	db.Save(&user)
	c.JSON(http.StatusOK, user)
}

// Admin: Get activity logs
func getAdminActivity(c *gin.Context) {
	limit := 100
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	action := c.Query("action")
	userID := c.Query("user_id")

	query := db.Model(&ActivityLog{}).Order("created_at desc").Limit(limit)

	if action != "" {
		query = query.Where("action = ?", action)
	}
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	var logs []ActivityLog
	query.Find(&logs)

	c.JSON(http.StatusOK, logs)
}

// Admin: Get dashboard stats
func getAdminStats(c *gin.Context) {
	var userCount int64
	var stockCount int64
	var positionCount int64
	var trackedCount int64

	db.Model(&User{}).Count(&userCount)
	db.Model(&Stock{}).Count(&stockCount)
	db.Model(&PortfolioPosition{}).Count(&positionCount)
	db.Model(&StockPerformance{}).Count(&trackedCount)

	// Activity stats for last 7 days
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)

	var loginCount int64
	db.Model(&ActivityLog{}).Where("action = ? AND created_at > ?", "login", sevenDaysAgo).Count(&loginCount)

	var searchCount int64
	db.Model(&ActivityLog{}).Where("action = ? AND created_at > ?", "search", sevenDaysAgo).Count(&searchCount)

	var pageViewCount int64
	db.Model(&ActivityLog{}).Where("action = ? AND created_at > ?", "page_view", sevenDaysAgo).Count(&pageViewCount)

	// Most active users
	type UserActivity struct {
		UserID   uint   `json:"user_id"`
		Username string `json:"username"`
		Count    int64  `json:"count"`
	}

	var mostActive []UserActivity
	db.Model(&ActivityLog{}).
		Select("user_id, username, count(*) as count").
		Where("created_at > ? AND user_id > 0", sevenDaysAgo).
		Group("user_id, username").
		Order("count desc").
		Limit(10).
		Find(&mostActive)

	// Most searched stocks
	type StockSearch struct {
		Symbol string `json:"symbol"`
		Count  int64  `json:"count"`
	}

	var mostSearched []StockSearch
	db.Model(&ActivityLog{}).
		Select("details as symbol, count(*) as count").
		Where("action = ? AND created_at > ?", "search", sevenDaysAgo).
		Group("details").
		Order("count desc").
		Limit(10).
		Find(&mostSearched)

	// Recently added stocks
	var recentStocks []Stock
	db.Order("created_at desc").Limit(10).Find(&recentStocks)

	// New users this week
	var newUsers int64
	db.Model(&User{}).Where("created_at > ?", sevenDaysAgo).Count(&newUsers)

	c.JSON(http.StatusOK, gin.H{
		"users":          userCount,
		"stocks":         stockCount,
		"positions":      positionCount,
		"tracked_stocks": trackedCount,
		"week_stats": gin.H{
			"logins":     loginCount,
			"searches":   searchCount,
			"page_views": pageViewCount,
			"new_users":  newUsers,
		},
		"most_active":    mostActive,
		"most_searched":  mostSearched,
		"recent_stocks":  recentStocks,
	})
}

// getAdminTraffic returns traffic statistics grouped by IP and device
func getAdminTraffic(c *gin.Context) {
	// Traffic by IP
	type IPTraffic struct {
		IPAddress string `json:"ip_address"`
		Count     int64  `json:"count"`
		LastVisit string `json:"last_visit"`
	}

	var ipTraffic []IPTraffic
	db.Model(&ActivityLog{}).
		Select("ip_address, count(*) as count, max(created_at) as last_visit").
		Where("ip_address != ''").
		Group("ip_address").
		Order("count desc").
		Limit(50).
		Find(&ipTraffic)

	// Traffic by device (parsed from User-Agent)
	type DeviceTraffic struct {
		UserAgent string `json:"user_agent"`
		Device    string `json:"device"`
		Count     int64  `json:"count"`
	}

	var rawDeviceTraffic []struct {
		UserAgent string
		Count     int64
	}
	db.Model(&ActivityLog{}).
		Select("user_agent, count(*) as count").
		Where("user_agent != ''").
		Group("user_agent").
		Order("count desc").
		Limit(30).
		Find(&rawDeviceTraffic)

	// Parse User-Agent to get device type
	var deviceTraffic []DeviceTraffic
	for _, d := range rawDeviceTraffic {
		device := "Desktop"
		ua := strings.ToLower(d.UserAgent)
		if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") {
			device = "Mobile"
		} else if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
			device = "Tablet"
		} else if strings.Contains(ua, "bot") || strings.Contains(ua, "crawler") || strings.Contains(ua, "spider") {
			device = "Bot"
		}

		// Shorten User-Agent for display
		shortUA := d.UserAgent
		if len(shortUA) > 80 {
			shortUA = shortUA[:80] + "..."
		}

		deviceTraffic = append(deviceTraffic, DeviceTraffic{
			UserAgent: shortUA,
			Device:    device,
			Count:     d.Count,
		})
	}

	// Traffic summary by day (last 7 days)
	type DailyTraffic struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}

	var dailyTraffic []DailyTraffic
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	db.Model(&ActivityLog{}).
		Select("DATE(created_at) as date, count(*) as count").
		Where("created_at > ?", sevenDaysAgo).
		Group("DATE(created_at)").
		Order("date desc").
		Find(&dailyTraffic)

	// Unique visitors (unique IPs) today
	var uniqueToday int64
	today := time.Now().Truncate(24 * time.Hour)
	db.Model(&ActivityLog{}).
		Where("created_at > ?", today).
		Distinct("ip_address").
		Count(&uniqueToday)

	// Total page views today
	var viewsToday int64
	db.Model(&ActivityLog{}).
		Where("created_at > ?", today).
		Count(&viewsToday)

	c.JSON(http.StatusOK, gin.H{
		"by_ip":         ipTraffic,
		"by_device":     deviceTraffic,
		"daily":         dailyTraffic,
		"unique_today":  uniqueToday,
		"views_today":   viewsToday,
	})
}

// updateAllWatchlistStocks returns all watchlist stocks for bulk update
// The actual BX-Trender calculation happens in the frontend
func updateAllWatchlistStocks(c *gin.Context) {
	mode := c.DefaultQuery("mode", "defensive")

	// Get all stocks from watchlist
	var stocks []Stock
	db.Order("symbol asc").Find(&stocks)

	c.JSON(http.StatusOK, gin.H{
		"mode":   mode,
		"stocks": stocks,
		"total":  len(stocks),
	})
}

// getTrackedDiff returns tracked stocks that are NOT in the watchlist
func getTrackedDiff(c *gin.Context) {
	// Get all watchlist symbols
	var watchlistStocks []Stock
	db.Find(&watchlistStocks)
	watchlistSymbols := make(map[string]bool)
	for _, s := range watchlistStocks {
		watchlistSymbols[s.Symbol] = true
	}

	// Get defensive tracked stocks not in watchlist
	var defensivePerfs []StockPerformance
	db.Find(&defensivePerfs)
	var defensiveDiff []StockPerformance
	for _, p := range defensivePerfs {
		if !watchlistSymbols[p.Symbol] {
			defensiveDiff = append(defensiveDiff, p)
		}
	}

	// Get aggressive tracked stocks not in watchlist
	var aggressivePerfs []AggressiveStockPerformance
	db.Find(&aggressivePerfs)
	var aggressiveDiff []AggressiveStockPerformance
	for _, p := range aggressivePerfs {
		if !watchlistSymbols[p.Symbol] {
			aggressiveDiff = append(aggressiveDiff, p)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"defensive":  defensiveDiff,
		"aggressive": aggressiveDiff,
	})
}

// deleteTrackedStock deletes a tracked stock from both performance tables
func deleteTrackedStock(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Symbol required"})
		return
	}

	// Delete from defensive performance
	db.Where("symbol = ?", symbol).Delete(&StockPerformance{})

	// Delete from aggressive performance
	db.Where("symbol = ?", symbol).Delete(&AggressiveStockPerformance{})

	c.JSON(http.StatusOK, gin.H{"message": "Deleted", "symbol": symbol})
}

// ========================================
// FlipperBot Functions
// ========================================

func ensureFlipperBotUser() {
	// Create FlipperBot user if not exists (for portfolio comparison visibility)
	var user User
	result := db.Where("id = ?", FLIPPERBOT_USER_ID).First(&user)
	if result.Error != nil {
		hashedPassword, _ := hashPassword("flipperbot-system-user-no-login")
		botUser := User{
			ID:       FLIPPERBOT_USER_ID,
			Email:    "flipperbot@system.local",
			Username: "FlipperBot",
			Password: hashedPassword,
			IsAdmin:  false,
		}
		db.Create(&botUser)
	}
}

func ensureLutzUser() {
	// Create Lutz user if not exists (for portfolio comparison visibility)
	var user User
	result := db.Where("id = ?", LUTZ_USER_ID).First(&user)
	if result.Error != nil {
		hashedPassword, _ := hashPassword("lutz-system-user-no-login")
		botUser := User{
			ID:       LUTZ_USER_ID,
			Email:    "lutz@system.local",
			Username: "Lutz",
			Password: hashedPassword,
			IsAdmin:  false,
		}
		db.Create(&botUser)
	}
}

func flipperBotUpdate(c *gin.Context) {
	startDate, _ := time.Parse("2006-01-02", FLIPPERBOT_START_DATE)
	now := time.Now()
	sessionID := uuid.New().String()

	// Detailed log for debugging
	var logs []map[string]interface{}
	addLog := func(level, message string, data map[string]interface{}) {
		entry := map[string]interface{}{
			"level":   level,
			"message": message,
			"time":    time.Now().Format("15:04:05"),
		}
		for k, v := range data {
			entry[k] = v
		}
		logs = append(logs, entry)
		// Also persist to DB
		saveBotLog("flipperbot", level, message, sessionID)
	}

	addLog("INFO", "FlipperBot Update gestartet", map[string]interface{}{
		"start_date": FLIPPERBOT_START_DATE,
		"now":        now.Format("2006-01-02 15:04:05"),
	})

	// Get all tracked stocks with their performance data
	var trackedStocks []StockPerformance
	db.Find(&trackedStocks)

	if len(trackedStocks) == 0 {
		addLog("WARN", "Keine getrackten Aktien gefunden", nil)
		c.JSON(http.StatusOK, gin.H{
			"message": "No tracked stocks found",
			"actions": []string{},
			"logs":    logs,
		})
		return
	}

	addLog("INFO", fmt.Sprintf("%d Aktien im Tracker gefunden", len(trackedStocks)), nil)

	var actions []map[string]interface{}

	// Fetch current quotes for all symbols
	symbols := make([]string, len(trackedStocks))
	for i, s := range trackedStocks {
		symbols[i] = s.Symbol
	}
	quotes := fetchQuotes(symbols)

	for _, stock := range trackedStocks {
		addLog("DEBUG", fmt.Sprintf("Prüfe %s", stock.Symbol), map[string]interface{}{
			"signal":      stock.Signal,
			"signal_bars": stock.SignalBars,
			"updated_at":  stock.UpdatedAt.Format("2006-01-02"),
		})

		// Check current position
		var existingPosition FlipperBotPosition
		hasPosition := db.Where("symbol = ?", stock.Symbol).First(&existingPosition).Error == nil

		// NEW LOGIC: Use current signal and SignalBars to determine action
		// SignalBars = how many monthly bars the signal has been active
		// We calculate when the signal started based on this

		// Calculate when the current signal started (approximate - monthly bars)
		signalStartDate := now.AddDate(0, -stock.SignalBars, 0)
		// Adjust to trading day (weekday) - trades only happen on weekdays
		signalStartDate = adjustToTradingDay(signalStartDate)

		addLog("DEBUG", fmt.Sprintf("%s: Signal seit ca. %s (%d Bars), Position: %v",
			stock.Symbol, signalStartDate.Format("2006-01-02"), stock.SignalBars, hasPosition), nil)

		if stock.Signal == "BUY" {
			// BUY signal is active - we should enter a position if we don't have one
			if !hasPosition {
				// NEW RULE: No retroactive trades! Always use TODAY's date and current price
				today := time.Now().Truncate(24 * time.Hour)
				tradeDate := adjustToTradingDay(today)

				// Skip if signal started before our start date (bot wasn't active then)
				if signalStartDate.Before(startDate) {
					addLog("INFO", fmt.Sprintf("%s: BUY-Signal startete vor %s, Trade heute zum aktuellen Kurs",
						stock.Symbol, FLIPPERBOT_START_DATE), nil)
				}

				// Check if we already have a buy trade for this stock
				var existingBuy FlipperBotTrade
				alreadyBought := db.Where("symbol = ? AND action = ?", stock.Symbol, "BUY").
					Order("signal_date desc").First(&existingBuy).Error == nil

				if alreadyBought {
					// Check if there was a sell after the last buy
					var lastSell FlipperBotTrade
					hasSoldAfter := db.Where("symbol = ? AND action = ? AND signal_date > ?",
						stock.Symbol, "SELL", existingBuy.SignalDate).First(&lastSell).Error == nil

					if !hasSoldAfter {
						addLog("SKIP", fmt.Sprintf("%s: Bereits gekauft am %s, kein Verkauf seitdem",
							stock.Symbol, existingBuy.SignalDate.Format("2006-01-02")), nil)
						continue
					}
				}

				// ALWAYS use current price - no retroactive trades!
				var buyPrice float64
				if quote, ok := quotes[stock.Symbol]; ok && quote.Price > 0 {
					buyPrice = quote.Price
				} else {
					addLog("ERROR", fmt.Sprintf("%s: Kein aktueller Preis verfügbar - überspringe", stock.Symbol), nil)
					continue
				}

				// Use today's date for the trade
				signalStartDate = tradeDate

				// Calculate quantity: invest 100 EUR worth
				investmentEUR := 100.0
				investmentUSD := convertToUSD(investmentEUR, "EUR")
				qty := investmentUSD / buyPrice

				// Create TODO instead of executing trade
				addLog("TODO", fmt.Sprintf("%s: BUY-Vorschlag erstellt - %.4f Anteile @ $%.2f (100€ = $%.2f)",
					stock.Symbol, qty, buyPrice, investmentUSD), nil)

				saveBotTodo("flipperbot", "BUY", stock.Symbol, stock.Name, qty, 0, buyPrice,
					stock.Signal, stock.SignalBars, signalStartDate.Format("2006-01-02"),
					fmt.Sprintf("BUY Signal aktiv seit %d Bars", stock.SignalBars))

				actions = append(actions, map[string]interface{}{
					"action":   "BUY",
					"symbol":   stock.Symbol,
					"name":     stock.Name,
					"price":    buyPrice,
					"date":     signalStartDate.Format("2006-01-02"),
					"quantity": qty,
					"is_todo":  true,
				})
			} else {
				addLog("SKIP", fmt.Sprintf("%s: BUY-Signal aktiv, Position bereits vorhanden", stock.Symbol), nil)
			}
		} else if stock.Signal == "HOLD" {
			// HOLD means we should be in a position from a previous BUY
			// If we don't have a position, the BUY was before our start date - skip
			if !hasPosition {
				addLog("SKIP", fmt.Sprintf("%s: HOLD-Signal, aber keine Position - BUY war vor %s",
					stock.Symbol, FLIPPERBOT_START_DATE), nil)
			} else {
				addLog("SKIP", fmt.Sprintf("%s: HOLD-Signal, behalte Position", stock.Symbol), nil)
			}
		} else if stock.Signal == "SELL" {
			// Should NOT have a position - check if we need to sell
			if hasPosition {
				// Calculate when SELL signal started based on SignalBars
				sellSignalDate := now.AddDate(0, -stock.SignalBars, 0)
				// Adjust to trading day (weekday)
				sellSignalDate = adjustToTradingDay(sellSignalDate)

				// Get price at sell signal date
				var sellDate time.Time
				var sellPrice float64

				// Try to get historical price at signal date
				sellPrice = getPriceAtDate(stock.Symbol, sellSignalDate)
				if sellPrice > 0 {
					sellDate = sellSignalDate
				} else {
					// Fallback: use current price
					sellDate = now
					if quote, ok := quotes[stock.Symbol]; ok && quote.Price > 0 {
						sellPrice = quote.Price
					} else {
						sellPrice = stock.CurrentPrice
					}
					addLog("WARN", fmt.Sprintf("%s: Kein historischer Preis, nutze aktuellen Preis", stock.Symbol), nil)
				}

				// Make sure sell date is after buy date
				if sellDate.Before(existingPosition.BuyDate) {
					sellDate = now
					if quote, ok := quotes[stock.Symbol]; ok && quote.Price > 0 {
						sellPrice = quote.Price
					}
					addLog("INFO", fmt.Sprintf("%s: Sell-Datum vor Buy-Datum, verwende heute", stock.Symbol), nil)
				}

				// Check if we already sold
				var existingSell FlipperBotTrade
				if db.Where("symbol = ? AND action = ? AND signal_date >= ?",
					stock.Symbol, "SELL", existingPosition.BuyDate).First(&existingSell).Error == nil {
					addLog("SKIP", fmt.Sprintf("%s: Bereits verkauft", stock.Symbol), nil)
					continue
				}

				// Create TODO instead of executing trade
				profitLoss := (sellPrice - existingPosition.AvgPrice) * existingPosition.Quantity
				profitLossPct := ((sellPrice - existingPosition.AvgPrice) / existingPosition.AvgPrice) * 100

				addLog("TODO", fmt.Sprintf("%s: SELL-Vorschlag erstellt - %.4f Anteile @ $%.2f (erwarteter Gewinn: $%.2f / %.2f%%)",
					stock.Symbol, existingPosition.Quantity, sellPrice, profitLoss, profitLossPct), nil)

				saveBotTodo("flipperbot", "SELL", stock.Symbol, stock.Name, existingPosition.Quantity,
					existingPosition.AvgPrice, sellPrice, stock.Signal, stock.SignalBars,
					sellDate.Format("2006-01-02"),
					fmt.Sprintf("SELL Signal - erwarteter Gewinn: $%.2f (%.2f%%)", profitLoss, profitLossPct))

				actions = append(actions, map[string]interface{}{
					"action":          "SELL",
					"symbol":          stock.Symbol,
					"name":            stock.Name,
					"price":           sellPrice,
					"date":            sellDate.Format("2006-01-02"),
					"quantity":        existingPosition.Quantity,
					"profit_loss":     profitLoss,
					"profit_loss_pct": profitLossPct,
					"is_todo":         true,
				})
			} else {
				addLog("SKIP", fmt.Sprintf("%s: Signal ist %s, keine Position zum Verkaufen", stock.Symbol, stock.Signal), nil)
			}
		} else {
			addLog("SKIP", fmt.Sprintf("%s: Unbekanntes Signal '%s'", stock.Symbol, stock.Signal), nil)
		}
	}

	addLog("INFO", fmt.Sprintf("Update abgeschlossen: %d Vorschläge erstellt", len(actions)), nil)

	c.JSON(http.StatusOK, gin.H{
		"message":       "FlipperBot update completed",
		"actions":       actions,
		"action_count":  len(actions),
		"logs":          logs,
		"todos_created": len(actions),
	})
}

func getFlipperBotPortfolio(c *gin.Context) {
	var positions []FlipperBotPosition
	db.Order("buy_date desc").Find(&positions)

	// Fetch current quotes
	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	type PositionWithQuote struct {
		ID            uint      `json:"id"`
		Symbol        string    `json:"symbol"`
		Name          string    `json:"name"`
		Quantity      float64   `json:"quantity"`
		AvgPrice      float64   `json:"avg_price"`
		BuyDate       time.Time `json:"buy_date"`
		CurrentPrice  float64   `json:"current_price"`
		Change        float64   `json:"change"`
		ChangePercent float64   `json:"change_percent"`
		TotalReturn   float64   `json:"total_return"`
		TotalReturnPct float64  `json:"total_return_pct"`
		CurrentValue  float64   `json:"current_value"`
		InvestedValue float64   `json:"invested_value"`
		IsLive        bool      `json:"is_live"`
	}

	result := make([]PositionWithQuote, len(positions))
	var totalValue, totalInvested float64

	for i, pos := range positions {
		quote := quotes[pos.Symbol]
		currentPrice := quote.Price
		if currentPrice == 0 {
			currentPrice = pos.AvgPrice
		}

		currentValue := currentPrice * pos.Quantity
		investedValue := pos.AvgPrice * pos.Quantity
		totalReturn := currentValue - investedValue
		totalReturnPct := 0.0
		if investedValue > 0 {
			totalReturnPct = (totalReturn / investedValue) * 100
		}

		totalValue += currentValue
		totalInvested += investedValue

		result[i] = PositionWithQuote{
			ID:            pos.ID,
			Symbol:        pos.Symbol,
			Name:          pos.Name,
			Quantity:      pos.Quantity,
			AvgPrice:      pos.AvgPrice,
			BuyDate:       pos.BuyDate,
			CurrentPrice:  currentPrice,
			Change:        quote.Change,
			ChangePercent: quote.ChangePercent,
			TotalReturn:   totalReturn,
			TotalReturnPct: totalReturnPct,
			CurrentValue:  currentValue,
			InvestedValue: investedValue,
			IsLive:        pos.IsLive,
		}
	}

	// Calculate overall performance
	overallReturn := totalValue - totalInvested
	overallReturnPct := 0.0
	if totalInvested > 0 {
		overallReturnPct = (overallReturn / totalInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"positions":        result,
		"total_value":      totalValue,
		"total_invested":   totalInvested,
		"total_return":     overallReturn,
		"total_return_pct": overallReturnPct,
		"position_count":   len(positions),
	})
}

func getFlipperBotActions(c *gin.Context) {
	limit := 50
	var trades []FlipperBotTrade
	db.Order("signal_date desc, executed_at desc").Limit(limit).Find(&trades)

	c.JSON(http.StatusOK, trades)
}

func getFlipperBotPerformance(c *gin.Context) {
	// Get all completed trades (sells)
	var sellTrades []FlipperBotTrade
	db.Where("action = ?", "SELL").Find(&sellTrades)

	// Get all buy trades to calculate total invested
	var buyTrades []FlipperBotTrade
	db.Where("action = ?", "BUY").Find(&buyTrades)

	wins := 0
	losses := 0
	totalProfitLoss := 0.0
	var profitLossList []float64

	for _, trade := range sellTrades {
		if trade.ProfitLoss != nil {
			totalProfitLoss += *trade.ProfitLoss
			profitLossList = append(profitLossList, *trade.ProfitLoss)
			if *trade.ProfitLoss >= 0 {
				wins++
			} else {
				losses++
			}
		}
	}

	winRate := 0.0
	if wins+losses > 0 {
		winRate = float64(wins) / float64(wins+losses) * 100
	}

	// Calculate current unrealized gains and invested amounts
	var positions []FlipperBotPosition
	db.Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	unrealizedGain := 0.0
	investedInPositions := 0.0
	currentValue := 0.0

	for _, pos := range positions {
		investedInPositions += pos.AvgPrice * pos.Quantity
		quote := quotes[pos.Symbol]
		if quote.Price > 0 {
			currentValue += quote.Price * pos.Quantity
			unrealizedGain += (quote.Price - pos.AvgPrice) * pos.Quantity
		} else {
			currentValue += pos.AvgPrice * pos.Quantity // fallback to buy price
		}
	}

	// Calculate total invested ever (all BUYs)
	totalInvested := 0.0
	for _, trade := range buyTrades {
		totalInvested += trade.Price * trade.Quantity
	}

	// Calculate total return percentage
	totalReturnPct := 0.0
	if investedInPositions > 0 {
		totalReturnPct = (unrealizedGain / investedInPositions) * 100
	}

	// Calculate overall performance (including realized)
	overallReturnPct := 0.0
	if totalInvested > 0 {
		overallReturnPct = ((totalProfitLoss + unrealizedGain) / totalInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_trades":        len(sellTrades),
		"total_buys":          len(buyTrades),
		"wins":                wins,
		"losses":              losses,
		"win_rate":            winRate,
		"realized_profit":     totalProfitLoss,
		"unrealized_gain":     unrealizedGain,
		"total_gain":          totalProfitLoss + unrealizedGain,
		"open_positions":      len(positions),
		"invested_in_positions": investedInPositions,
		"current_value":       currentValue,
		"total_invested":      totalInvested,
		"total_return_pct":    totalReturnPct,
		"overall_return_pct":  overallReturnPct,
	})
}

func resetFlipperBot(c *gin.Context) {
	// Delete all FlipperBot data
	db.Where("1 = 1").Delete(&FlipperBotTrade{})
	db.Where("1 = 1").Delete(&FlipperBotPosition{})
	db.Where("user_id = ?", FLIPPERBOT_USER_ID).Delete(&PortfolioPosition{})

	c.JSON(http.StatusOK, gin.H{
		"message": "FlipperBot reset completed",
	})
}

// fixFlipperBotDB fixes corrupt data in the database
func fixFlipperBotDB(c *gin.Context) {
	var fixes []string

	// 1. Delete trades with invalid quantity (inf, 0, negative)
	result := db.Where("quantity <= 0 OR quantity > 1000000").Delete(&FlipperBotTrade{})
	if result.RowsAffected > 0 {
		fixes = append(fixes, fmt.Sprintf("Deleted %d invalid trades (bad quantity)", result.RowsAffected))
	}

	// 2. Delete positions with invalid quantity or price
	result = db.Where("quantity <= 0 OR quantity > 1000000 OR avg_price <= 0").Delete(&FlipperBotPosition{})
	if result.RowsAffected > 0 {
		fixes = append(fixes, fmt.Sprintf("Deleted %d invalid positions", result.RowsAffected))
	}

	// 3. Find and remove duplicate trades (keep only first BUY per symbol without subsequent SELL)
	var trades []FlipperBotTrade
	db.Order("id asc").Find(&trades)

	// Track which symbols have open positions (BUY without SELL after)
	type tradeInfo struct {
		id         uint
		signalDate time.Time
	}
	openBuys := make(map[string]tradeInfo)
	duplicatesToDelete := []uint{}

	for _, trade := range trades {
		if trade.Action == "BUY" {
			if existing, ok := openBuys[trade.Symbol]; ok {
				// Already have an open buy for this symbol - this is a duplicate
				duplicatesToDelete = append(duplicatesToDelete, trade.ID)
			} else {
				openBuys[trade.Symbol] = tradeInfo{id: trade.ID, signalDate: trade.SignalDate}
			}
		} else if trade.Action == "SELL" {
			// SELL closes the position
			delete(openBuys, trade.Symbol)
		}
	}

	if len(duplicatesToDelete) > 0 {
		db.Where("id IN ?", duplicatesToDelete).Delete(&FlipperBotTrade{})
		fixes = append(fixes, fmt.Sprintf("Deleted %d duplicate BUY trades", len(duplicatesToDelete)))
	}

	// 4. Remove orphaned positions (no matching BUY trade)
	var positions []FlipperBotPosition
	db.Find(&positions)
	for _, pos := range positions {
		var buyTrade FlipperBotTrade
		if db.Where("symbol = ? AND action = ?", pos.Symbol, "BUY").First(&buyTrade).Error != nil {
			db.Delete(&pos)
			fixes = append(fixes, fmt.Sprintf("Deleted orphaned position: %s", pos.Symbol))
		}
	}

	// 5. Sync positions with trades (rebuild from trades)
	// Get fresh list of open buys
	openBuys = make(map[string]tradeInfo)
	db.Order("id asc").Find(&trades)

	for _, trade := range trades {
		if trade.Action == "BUY" {
			openBuys[trade.Symbol] = tradeInfo{id: trade.ID, signalDate: trade.SignalDate}
		} else if trade.Action == "SELL" {
			delete(openBuys, trade.Symbol)
		}
	}

	// Ensure positions exist for all open buys
	for symbol := range openBuys {
		var pos FlipperBotPosition
		if db.Where("symbol = ?", symbol).First(&pos).Error != nil {
			// Position missing - get trade and recreate
			var trade FlipperBotTrade
			if db.Where("symbol = ? AND action = ?", symbol, "BUY").Order("id desc").First(&trade).Error == nil {
				newPos := FlipperBotPosition{
					Symbol:   symbol,
					Name:     trade.Name,
					Quantity: trade.Quantity,
					AvgPrice: trade.Price,
					BuyDate:  trade.SignalDate,
					IsLive:   trade.IsLive,
				}
				db.Create(&newPos)
				fixes = append(fixes, fmt.Sprintf("Recreated position: %s", symbol))
			}
		}
	}

	if len(fixes) == 0 {
		fixes = append(fixes, "No issues found - database is clean")
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Database fix completed",
		"fixes":   fixes,
	})
}

// syncFlipperBot synchronizes positions with trades
func syncFlipperBot(c *gin.Context) {
	var results []map[string]interface{}

	// Get all BUY trades
	var buyTrades []FlipperBotTrade
	db.Where("action = ?", "BUY").Order("signal_date asc").Find(&buyTrades)

	// Get all SELL trades
	var sellTrades []FlipperBotTrade
	db.Where("action = ?", "SELL").Find(&sellTrades)

	// Build map of sells by symbol
	sellsBySymbol := make(map[string][]FlipperBotTrade)
	for _, sell := range sellTrades {
		sellsBySymbol[sell.Symbol] = append(sellsBySymbol[sell.Symbol], sell)
	}

	// For each symbol, check if there's an open position (BUY without matching SELL)
	openBuys := make(map[string]FlipperBotTrade)
	for _, buy := range buyTrades {
		sells := sellsBySymbol[buy.Symbol]
		hasSellAfter := false
		for _, sell := range sells {
			if sell.SignalDate.After(buy.SignalDate) || sell.SignalDate.Equal(buy.SignalDate) {
				hasSellAfter = true
				break
			}
		}
		if !hasSellAfter {
			// This is an open buy - keep the latest one
			if existing, ok := openBuys[buy.Symbol]; ok {
				if buy.SignalDate.After(existing.SignalDate) {
					openBuys[buy.Symbol] = buy
				}
			} else {
				openBuys[buy.Symbol] = buy
			}
		}
	}

	// Delete all existing positions and recreate from open buys
	db.Where("1 = 1").Delete(&FlipperBotPosition{})
	db.Where("user_id = ?", FLIPPERBOT_USER_ID).Delete(&PortfolioPosition{})

	for symbol, buy := range openBuys {
		// Create position
		pos := FlipperBotPosition{
			Symbol:   symbol,
			Name:     buy.Name,
			Quantity: buy.Quantity,
			AvgPrice: buy.Price,
			IsLive:   buy.IsLive,
			BuyDate:  buy.SignalDate,
		}
		db.Create(&pos)

		// Create portfolio position
		portfolioPos := PortfolioPosition{
			UserID:       FLIPPERBOT_USER_ID,
			Symbol:       symbol,
			Name:         buy.Name,
			PurchaseDate: &buy.SignalDate,
			AvgPrice:     buy.Price,
			Currency:     "USD",
			Quantity:     &buy.Quantity,
		}
		db.Create(&portfolioPos)

		results = append(results, map[string]interface{}{
			"symbol":   symbol,
			"quantity": buy.Quantity,
			"price":    buy.Price,
			"date":     buy.SignalDate.Format("2006-01-02"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          "Sync completed",
		"positions_synced": len(results),
		"positions":        results,
	})
}

// getFlipperBotCompletedTrades returns completed trades (BUY + SELL pairs)
func getFlipperBotCompletedTrades(c *gin.Context) {
	var trades []FlipperBotTrade
	db.Where("action = ?", "SELL").Order("signal_date desc").Find(&trades)

	type CompletedTrade struct {
		Symbol        string    `json:"symbol"`
		Name          string    `json:"name"`
		BuyDate       time.Time `json:"buy_date"`
		BuyPrice      float64   `json:"buy_price"`
		SellDate      time.Time `json:"sell_date"`
		SellPrice     float64   `json:"sell_price"`
		Quantity      float64   `json:"quantity"`
		ProfitLoss    float64   `json:"profit_loss"`
		ProfitLossPct float64   `json:"profit_loss_pct"`
		IsLive        bool      `json:"is_live"`
	}

	var completed []CompletedTrade
	for _, sell := range trades {
		// Find the matching BUY
		var buy FlipperBotTrade
		if err := db.Where("symbol = ? AND action = ? AND signal_date < ?",
			sell.Symbol, "BUY", sell.SignalDate).
			Order("signal_date desc").First(&buy).Error; err == nil {

			pl := 0.0
			plPct := 0.0
			if sell.ProfitLoss != nil {
				pl = *sell.ProfitLoss
			}
			if sell.ProfitLossPct != nil {
				plPct = *sell.ProfitLossPct
			}

			completed = append(completed, CompletedTrade{
				Symbol:        sell.Symbol,
				Name:          sell.Name,
				BuyDate:       buy.SignalDate,
				BuyPrice:      buy.Price,
				SellDate:      sell.SignalDate,
				SellPrice:     sell.Price,
				Quantity:      sell.Quantity,
				ProfitLoss:    pl,
				ProfitLossPct: plPct,
				IsLive:        sell.IsLive,
			})
		}
	}

	c.JSON(http.StatusOK, completed)
}

// getPerformanceHistory returns trade history from StockPerformance (defensive) and AggressiveStockPerformance (aggressive)
func getPerformanceHistory(c *gin.Context) {
	type TradeEntry struct {
		ID           uint    `json:"id"`
		Mode         string  `json:"mode"` // "defensive" or "aggressive"
		Symbol       string  `json:"symbol"`
		Name         string  `json:"name"`
		EntryPrice   float64 `json:"entry_price"`
		ExitPrice    float64 `json:"exit_price"`
		CurrentPrice float64 `json:"current_price"`
		EntryDate    int64   `json:"entry_date"`
		ExitDate     int64   `json:"exit_date"`
		Status       string  `json:"status"` // "OPEN" or "CLOSED"
		ReturnPct    float64 `json:"return_pct"`
	}

	var entries []TradeEntry
	var idCounter uint = 1

	// Get defensive stock performances
	var defensiveStocks []StockPerformance
	db.Find(&defensiveStocks)

	for _, stock := range defensiveStocks {
		if stock.TradesJSON == "" {
			continue
		}
		var trades []TradeData
		if err := json.Unmarshal([]byte(stock.TradesJSON), &trades); err != nil {
			continue
		}

		for _, trade := range trades {
			entry := TradeEntry{
				ID:         idCounter,
				Mode:       "defensive",
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				EntryPrice: trade.EntryPrice,
				EntryDate:  trade.EntryDate,
				ReturnPct:  trade.ReturnPct,
			}
			idCounter++

			if trade.IsOpen {
				entry.Status = "OPEN"
				if trade.CurrentPrice != nil {
					entry.CurrentPrice = *trade.CurrentPrice
				}
			} else {
				entry.Status = "CLOSED"
				if trade.ExitPrice != nil {
					entry.ExitPrice = *trade.ExitPrice
				}
				if trade.ExitDate != nil {
					entry.ExitDate = *trade.ExitDate
				}
			}

			entries = append(entries, entry)
		}
	}

	// Get aggressive stock performances
	var aggressiveStocks []AggressiveStockPerformance
	db.Find(&aggressiveStocks)

	for _, stock := range aggressiveStocks {
		if stock.TradesJSON == "" {
			continue
		}
		var trades []TradeData
		if err := json.Unmarshal([]byte(stock.TradesJSON), &trades); err != nil {
			continue
		}

		for _, trade := range trades {
			entry := TradeEntry{
				ID:         idCounter,
				Mode:       "aggressive",
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				EntryPrice: trade.EntryPrice,
				EntryDate:  trade.EntryDate,
				ReturnPct:  trade.ReturnPct,
			}
			idCounter++

			if trade.IsOpen {
				entry.Status = "OPEN"
				if trade.CurrentPrice != nil {
					entry.CurrentPrice = *trade.CurrentPrice
				}
			} else {
				entry.Status = "CLOSED"
				if trade.ExitPrice != nil {
					entry.ExitPrice = *trade.ExitPrice
				}
				if trade.ExitDate != nil {
					entry.ExitDate = *trade.ExitDate
				}
			}

			entries = append(entries, entry)
		}
	}

	c.JSON(http.StatusOK, entries)
}

// Update FlipperBot position with real trade data (Admin only)
func updateFlipperBotPosition(c *gin.Context) {
	id := c.Param("id")

	var position FlipperBotPosition
	if err := db.First(&position, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found"})
		return
	}

	var req struct {
		Quantity float64 `json:"quantity"`
		AvgPrice float64 `json:"avg_price"`
		IsLive   bool    `json:"is_live"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Update position
	position.Quantity = req.Quantity
	position.AvgPrice = req.AvgPrice
	position.IsLive = req.IsLive
	db.Save(&position)

	// Also update corresponding portfolio position
	var portfolioPos PortfolioPosition
	if err := db.Where("user_id = ? AND symbol = ?", FLIPPERBOT_USER_ID, position.Symbol).First(&portfolioPos).Error; err == nil {
		portfolioPos.Quantity = &req.Quantity
		portfolioPos.AvgPrice = req.AvgPrice
		db.Save(&portfolioPos)
	}

	c.JSON(http.StatusOK, position)
}

// Update FlipperBot trade with real trade data (Admin only)
func updateFlipperBotTrade(c *gin.Context) {
	id := c.Param("id")

	var trade FlipperBotTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	var req struct {
		Quantity float64 `json:"quantity"`
		Price    float64 `json:"price"`
		IsLive   bool    `json:"is_live"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Update trade
	trade.Quantity = req.Quantity
	trade.Price = req.Price
	trade.IsLive = req.IsLive
	db.Save(&trade)

	// If BUY trade, update corresponding position
	if trade.Action == "BUY" {
		var position FlipperBotPosition
		if err := db.Where("symbol = ?", trade.Symbol).First(&position).Error; err == nil {
			position.Quantity = req.Quantity
			position.AvgPrice = req.Price
			position.IsLive = req.IsLive
			db.Save(&position)

			// Also update portfolio position
			var portfolioPos PortfolioPosition
			if err := db.Where("user_id = ? AND symbol = ?", FLIPPERBOT_USER_ID, trade.Symbol).First(&portfolioPos).Error; err == nil {
				portfolioPos.Quantity = &req.Quantity
				portfolioPos.AvgPrice = req.Price
				db.Save(&portfolioPos)
			}
		}
	}

	c.JSON(http.StatusOK, trade)
}

// deleteFlipperBotTrade deletes a trade and its associated position
func deleteFlipperBotTrade(c *gin.Context) {
	id := c.Param("id")

	var trade FlipperBotTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	symbol := trade.Symbol

	// Delete the trade
	db.Delete(&trade)

	// If it was a BUY trade, also delete the position
	if trade.Action == "BUY" {
		db.Where("symbol = ?", symbol).Delete(&FlipperBotPosition{})
		db.Where("user_id = ? AND symbol = ?", FLIPPERBOT_USER_ID, symbol).Delete(&PortfolioPosition{})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Trade deleted", "symbol": symbol})
}

// getFlipperBotPending returns pending actions (positions to sell, stocks to buy)
func getFlipperBotPending(c *gin.Context) {
	var pending []map[string]interface{}

	// Get all current positions
	var positions []FlipperBotPosition
	db.Find(&positions)

	// Get tracked stocks performance data
	var trackedStocks []StockPerformance
	db.Find(&trackedStocks)

	// Create a map for quick lookup
	stockSignals := make(map[string]StockPerformance)
	for _, s := range trackedStocks {
		stockSignals[s.Symbol] = s
	}

	// Check positions that need to be sold (signal changed to SELL)
	for _, pos := range positions {
		if perf, ok := stockSignals[pos.Symbol]; ok {
			if perf.Signal == "SELL" {
				signalSince := time.Now().AddDate(0, -perf.SignalBars, 0).Format("2006-01-02")
				pending = append(pending, map[string]interface{}{
					"type":         "SELL",
					"symbol":       pos.Symbol,
					"name":         pos.Name,
					"quantity":     pos.Quantity,
					"avg_price":    pos.AvgPrice,
					"signal":       perf.Signal,
					"signal_bars":  perf.SignalBars,
					"signal_since": signalSince,
					"reason":       "Position hat SELL-Signal",
				})
				// Create/update todo
				saveBotTodo("flipperbot", "SELL", pos.Symbol, pos.Name, pos.Quantity, pos.AvgPrice, 0, perf.Signal, perf.SignalBars, signalSince, "Position hat SELL-Signal")
			}
		}
	}

	// Check tracked stocks with BUY signal that we don't own yet
	positionSymbols := make(map[string]bool)
	for _, p := range positions {
		positionSymbols[p.Symbol] = true
	}

	for _, stock := range trackedStocks {
		if stock.Signal == "BUY" && !positionSymbols[stock.Symbol] {
			// Check if we already have a buy trade without subsequent sell
			var existingBuy FlipperBotTrade
			alreadyBought := db.Where("symbol = ? AND action = ?", stock.Symbol, "BUY").
				Order("signal_date desc").First(&existingBuy).Error == nil

			if alreadyBought {
				var lastSell FlipperBotTrade
				hasSoldAfter := db.Where("symbol = ? AND action = ? AND signal_date > ?",
					stock.Symbol, "SELL", existingBuy.SignalDate).First(&lastSell).Error == nil
				if !hasSoldAfter {
					continue // Already bought, skip
				}
			}

			signalSince := time.Now().AddDate(0, -stock.SignalBars, 0).Format("2006-01-02")
			pending = append(pending, map[string]interface{}{
				"type":         "BUY",
				"symbol":       stock.Symbol,
				"name":         stock.Name,
				"signal":       stock.Signal,
				"signal_bars":  stock.SignalBars,
				"signal_since": signalSince,
				"reason":       "Neues BUY-Signal erkannt",
			})
			// Create/update todo
			saveBotTodo("flipperbot", "BUY", stock.Symbol, stock.Name, 0, 0, 0, stock.Signal, stock.SignalBars, signalSince, "Neues BUY-Signal erkannt")
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"pending": pending,
		"count":   len(pending),
	})
}

// getFlipperBotLogs returns the last 100 logs for FlipperBot
func getFlipperBotLogs(c *gin.Context) {
	var logs []BotLog
	db.Where("bot = ?", "flipperbot").Order("created_at desc").Limit(100).Find(&logs)
	c.JSON(http.StatusOK, logs)
}

// getFlipperBotTodos returns all todos for FlipperBot (open first, then done)
func getFlipperBotTodos(c *gin.Context) {
	var todos []BotTodo
	db.Where("bot = ?", "flipperbot").Order("done asc, created_at desc").Find(&todos)
	c.JSON(http.StatusOK, todos)
}

// markFlipperBotTodoDone marks a todo as done (discarded)
func markFlipperBotTodoDone(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	now := time.Now()
	todo.Done = true
	todo.Decision = "discarded"
	todo.DoneAt = &now
	db.Save(&todo)
	c.JSON(http.StatusOK, todo)
}

// reopenFlipperBotTodo reopens a done todo
func reopenFlipperBotTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	if !todo.Done {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Todo is not done"})
		return
	}
	todo.Done = false
	todo.Decision = ""
	todo.DoneAt = nil
	todo.UpdatedAt = time.Now()
	db.Save(&todo)
	c.JSON(http.StatusOK, todo)
}

// deleteFlipperBotTodo deletes a done todo
func deleteFlipperBotTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	db.Delete(&todo)
	c.JSON(http.StatusOK, gin.H{"message": "Todo deleted"})
}

// executeFlipperBotTodo executes a pending todo (actually performs the trade)
func executeFlipperBotTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	if todo.Done {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Todo already completed"})
		return
	}

	if todo.Bot != "flipperbot" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Wrong bot type"})
		return
	}

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	tradeDate := adjustToTradingDay(today)

	// Fetch fresh price
	quotes := fetchQuotes([]string{todo.Symbol})
	currentPrice := todo.Price
	if quote, ok := quotes[todo.Symbol]; ok && quote.Price > 0 {
		currentPrice = quote.Price
	}

	// Validate price - prevent division by zero
	if currentPrice <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price - cannot execute trade with price <= 0"})
		return
	}

	if todo.Type == "BUY" {
		// Check if position already exists - prevent duplicate buys
		var existingPos FlipperBotPosition
		if db.Where("symbol = ?", todo.Symbol).First(&existingPos).Error == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Position already exists for " + todo.Symbol})
			return
		}

		// Calculate quantity for exactly 100 EUR investment
		investmentEUR := 100.0
		investmentUSD := convertToUSD(investmentEUR, "EUR")
		// Calculate quantity and round to 6 decimal places
		qty := investmentUSD / currentPrice
		qty = math.Round(qty*1000000) / 1000000
		// Recalculate price so that qty * actualPrice = exactly investmentUSD
		actualPrice := investmentUSD / qty

		// Create trade (IsLive=false, Admin sets to true manually for real trades)
		newTrade := FlipperBotTrade{
			Symbol:     todo.Symbol,
			Name:       todo.Name,
			Action:     "BUY",
			Quantity:   qty,
			Price:      actualPrice,
			SignalDate: tradeDate,
			ExecutedAt: now,
		}
		db.Create(&newTrade)

		// Create position (IsLive=false, Admin sets to true manually for real trades)
		newPosition := FlipperBotPosition{
			Symbol:      todo.Symbol,
			Name:        todo.Name,
			Quantity:    qty,
			AvgPrice:    actualPrice,
			InvestedEUR: investmentEUR,
			BuyDate:     tradeDate,
		}
		if err := db.Create(&newPosition).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create position: " + err.Error()})
			return
		}

		// Add to portfolio
		portfolioPos := PortfolioPosition{
			UserID:       FLIPPERBOT_USER_ID,
			Symbol:       todo.Symbol,
			Name:         todo.Name,
			PurchaseDate: &tradeDate,
			AvgPrice:     actualPrice,
			Currency:     "USD",
			Quantity:     &qty,
		}
		db.Create(&portfolioPos)

		// Mark todo as done with decision
		todo.Done = true
		todo.Decision = "executed"
		todo.DoneAt = &now
		db.Save(&todo)

		c.JSON(http.StatusOK, gin.H{
			"message":        "BUY executed",
			"symbol":         todo.Symbol,
			"quantity":       qty,
			"price":          actualPrice,
			"invested_eur":   investmentEUR,
			"invested_usd":   investmentUSD,
			"trade_id":       newTrade.ID,
		})
	} else if todo.Type == "SELL" {
		// Get current position
		var position FlipperBotPosition
		if err := db.Where("symbol = ?", todo.Symbol).First(&position).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No position found to sell"})
			return
		}

		profitLoss := (currentPrice - position.AvgPrice) * position.Quantity
		profitLossPct := ((currentPrice - position.AvgPrice) / position.AvgPrice) * 100

		// Create trade (IsLive=false, Admin sets to true manually for real trades)
		newTrade := FlipperBotTrade{
			Symbol:        todo.Symbol,
			Name:          todo.Name,
			Action:        "SELL",
			Quantity:      position.Quantity,
			Price:         currentPrice,
			SignalDate:    tradeDate,
			ExecutedAt:    now,
			ProfitLoss:    &profitLoss,
			ProfitLossPct: &profitLossPct,
		}
		db.Create(&newTrade)

		// Delete position
		db.Delete(&position)
		db.Where("user_id = ? AND symbol = ?", FLIPPERBOT_USER_ID, todo.Symbol).Delete(&PortfolioPosition{})

		// Mark todo as done with decision
		todo.Done = true
		todo.Decision = "executed"
		todo.DoneAt = &now
		db.Save(&todo)

		c.JSON(http.StatusOK, gin.H{
			"message":         "SELL executed",
			"symbol":          todo.Symbol,
			"quantity":        position.Quantity,
			"price":           currentPrice,
			"profit_loss":     profitLoss,
			"profit_loss_pct": profitLossPct,
			"trade_id":        newTrade.ID,
		})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown todo type"})
	}
}

// executeLutzTodo executes a pending Lutz todo
func executeLutzTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	if todo.Done {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Todo already completed"})
		return
	}

	if todo.Bot != "lutz" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Wrong bot type"})
		return
	}

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	tradeDate := adjustToTradingDay(today)

	// Fetch fresh price
	quotes := fetchQuotes([]string{todo.Symbol})
	currentPrice := todo.Price
	if quote, ok := quotes[todo.Symbol]; ok && quote.Price > 0 {
		currentPrice = quote.Price
	}

	// Validate price - prevent division by zero
	if currentPrice <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid price - cannot execute trade with price <= 0"})
		return
	}

	if todo.Type == "BUY" {
		// Check if position already exists - prevent duplicate buys
		var existingPos LutzPosition
		if db.Where("symbol = ?", todo.Symbol).First(&existingPos).Error == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Position already exists for " + todo.Symbol})
			return
		}

		// Calculate quantity for exactly 100 EUR investment
		investmentEUR := 100.0
		investmentUSD := convertToUSD(investmentEUR, "EUR")
		qty := investmentUSD / currentPrice
		qty = math.Round(qty*1000000) / 1000000
		// Recalculate price so that qty * actualPrice = exactly investmentUSD
		actualPrice := investmentUSD / qty

		newTrade := LutzTrade{
			Symbol:     todo.Symbol,
			Name:       todo.Name,
			Action:     "BUY",
			Quantity:   qty,
			Price:      actualPrice,
			SignalDate: tradeDate,
			ExecutedAt: now,
		}
		db.Create(&newTrade)

		newPosition := LutzPosition{
			Symbol:      todo.Symbol,
			Name:        todo.Name,
			Quantity:    qty,
			AvgPrice:    actualPrice,
			InvestedEUR: investmentEUR,
			BuyDate:     tradeDate,
		}
		if err := db.Create(&newPosition).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create position: " + err.Error()})
			return
		}

		portfolioPos := PortfolioPosition{
			UserID:       LUTZ_USER_ID,
			Symbol:       todo.Symbol,
			Name:         todo.Name,
			PurchaseDate: &tradeDate,
			AvgPrice:     actualPrice,
			Currency:     "USD",
			Quantity:     &qty,
		}
		db.Create(&portfolioPos)

		todo.Done = true
		todo.Decision = "executed"
		todo.DoneAt = &now
		db.Save(&todo)

		c.JSON(http.StatusOK, gin.H{
			"message":      "BUY executed",
			"symbol":       todo.Symbol,
			"quantity":     qty,
			"price":        actualPrice,
			"invested_eur": investmentEUR,
			"invested_usd": investmentUSD,
			"trade_id":     newTrade.ID,
		})
	} else if todo.Type == "SELL" {
		var position LutzPosition
		if err := db.Where("symbol = ?", todo.Symbol).First(&position).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No position found to sell"})
			return
		}

		profitLoss := (currentPrice - position.AvgPrice) * position.Quantity
		profitLossPct := ((currentPrice - position.AvgPrice) / position.AvgPrice) * 100

		newTrade := LutzTrade{
			Symbol:        todo.Symbol,
			Name:          todo.Name,
			Action:        "SELL",
			Quantity:      position.Quantity,
			Price:         currentPrice,
			SignalDate:    tradeDate,
			ExecutedAt:    now,
			ProfitLoss:    &profitLoss,
			ProfitLossPct: &profitLossPct,
		}
		db.Create(&newTrade)

		db.Delete(&position)
		db.Where("user_id = ? AND symbol = ?", LUTZ_USER_ID, todo.Symbol).Delete(&PortfolioPosition{})

		todo.Done = true
		todo.Decision = "executed"
		todo.DoneAt = &now
		db.Save(&todo)

		c.JSON(http.StatusOK, gin.H{
			"message":         "SELL executed",
			"symbol":          todo.Symbol,
			"quantity":        position.Quantity,
			"price":           currentPrice,
			"profit_loss":     profitLoss,
			"profit_loss_pct": profitLossPct,
			"trade_id":        newTrade.ID,
		})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown todo type"})
	}
}

// saveBotLog saves a log entry to the database
func saveBotLog(bot, level, message, sessionID string) {
	log := BotLog{
		Bot:       bot,
		Level:     level,
		Message:   message,
		SessionID: sessionID,
	}
	db.Create(&log)
}

// saveBotTodo creates or updates a todo entry. If a done todo exists, it reopens it.
func saveBotTodo(bot, todoType, symbol, name string, quantity, avgPrice, price float64, signal string, signalBars int, signalSince, reason string) {
	// Check if there's already an open todo for this symbol and type
	var existing BotTodo
	if err := db.Where("bot = ? AND symbol = ? AND type = ? AND done = ?", bot, symbol, todoType, false).First(&existing).Error; err == nil {
		// Update existing open todo
		existing.Quantity = quantity
		existing.AvgPrice = avgPrice
		existing.Price = price
		existing.Signal = signal
		existing.SignalBars = signalBars
		existing.SignalSince = signalSince
		existing.UpdatedAt = time.Now()
		db.Save(&existing)
		return
	}

	// Check if there's a done todo for this symbol and type - reopen it
	var doneTodo BotTodo
	if err := db.Where("bot = ? AND symbol = ? AND type = ? AND done = ?", bot, symbol, todoType, true).First(&doneTodo).Error; err == nil {
		// Reopen the done todo
		doneTodo.Quantity = quantity
		doneTodo.AvgPrice = avgPrice
		doneTodo.Price = price
		doneTodo.Signal = signal
		doneTodo.SignalBars = signalBars
		doneTodo.SignalSince = signalSince
		doneTodo.Reason = reason
		doneTodo.Done = false
		doneTodo.DoneAt = nil
		doneTodo.UpdatedAt = time.Now()
		db.Save(&doneTodo)
		return
	}

	// Create new todo
	todo := BotTodo{
		Bot:         bot,
		Type:        todoType,
		Symbol:      symbol,
		Name:        name,
		Quantity:    quantity,
		AvgPrice:    avgPrice,
		Price:       price,
		Signal:      signal,
		SignalBars:  signalBars,
		SignalSince: signalSince,
		Reason:      reason,
		Done:        false,
	}
	db.Create(&todo)
}

// ==================== LUTZ BOT (Aggressive Mode) ====================

func lutzUpdate(c *gin.Context) {
	startDate, _ := time.Parse("2006-01-02", FLIPPERBOT_START_DATE)
	now := time.Now()
	sessionID := uuid.New().String()

	var logs []map[string]interface{}
	addLog := func(level, message string, data map[string]interface{}) {
		entry := map[string]interface{}{
			"level":   level,
			"message": message,
			"time":    time.Now().Format("15:04:05"),
		}
		for k, v := range data {
			entry[k] = v
		}
		logs = append(logs, entry)
		// Also persist to DB
		saveBotLog("lutz", level, message, sessionID)
	}

	addLog("INFO", "Lutz Update gestartet (Aggressiver Modus)", map[string]interface{}{
		"start_date": FLIPPERBOT_START_DATE,
		"now":        now.Format("2006-01-02 15:04:05"),
	})

	// Get all tracked stocks from AGGRESSIVE performance data
	var trackedStocks []AggressiveStockPerformance
	db.Find(&trackedStocks)

	if len(trackedStocks) == 0 {
		addLog("WARN", "Keine aggressiven Performance-Daten gefunden", nil)
		c.JSON(http.StatusOK, gin.H{
			"message": "No aggressive performance data found",
			"actions": []string{},
			"logs":    logs,
		})
		return
	}

	addLog("INFO", fmt.Sprintf("%d Aktien mit aggressiven Daten gefunden", len(trackedStocks)), nil)

	var actions []map[string]interface{}

	symbols := make([]string, len(trackedStocks))
	for i, s := range trackedStocks {
		symbols[i] = s.Symbol
	}
	quotes := fetchQuotes(symbols)

	for _, stock := range trackedStocks {
		addLog("DEBUG", fmt.Sprintf("Prüfe %s", stock.Symbol), map[string]interface{}{
			"signal":      stock.Signal,
			"signal_bars": stock.SignalBars,
		})

		var existingPosition LutzPosition
		hasPosition := db.Where("symbol = ?", stock.Symbol).First(&existingPosition).Error == nil

		signalStartDate := now.AddDate(0, -stock.SignalBars, 0)
		signalStartDate = adjustToTradingDay(signalStartDate)

		addLog("DEBUG", fmt.Sprintf("%s: Signal=%s, Bars=%d, Position: %v",
			stock.Symbol, stock.Signal, stock.SignalBars, hasPosition), nil)

		// Aggressive mode signals (based on trade history):
		// BUY = BUY triggered this/last month AND open position in BX-Trender simulation
		// SELL = SELL triggered this/last month AND no open position
		// HOLD = Open position but no recent BUY (do NOT buy)
		// WAIT = No open position and no recent SELL (do nothing)
		//
		// Bot actions:
		// - BUY signal ONLY -> buy (HOLD means already in position, don't buy again)
		// - SELL signal ONLY -> sell (WAIT means already out, don't sell)
		if stock.Signal == "BUY" {
			if !hasPosition {
				// NEW RULE: No retroactive trades! Always use TODAY's date and current price
				today := time.Now().Truncate(24 * time.Hour)
				tradeDate := adjustToTradingDay(today)

				if signalStartDate.Before(startDate) {
					addLog("INFO", fmt.Sprintf("%s: Signal startete vor %s, Trade heute zum aktuellen Kurs", stock.Symbol, FLIPPERBOT_START_DATE), nil)
				}

				var existingBuy LutzTrade
				alreadyBought := db.Where("symbol = ? AND action = ?", stock.Symbol, "BUY").
					Order("signal_date desc").First(&existingBuy).Error == nil

				if alreadyBought {
					var lastSell LutzTrade
					hasSoldAfter := db.Where("symbol = ? AND action = ? AND signal_date > ?",
						stock.Symbol, "SELL", existingBuy.SignalDate).First(&lastSell).Error == nil

					if !hasSoldAfter {
						addLog("SKIP", fmt.Sprintf("%s: Bereits gekauft am %s", stock.Symbol, existingBuy.SignalDate.Format("2006-01-02")), nil)
						continue
					}
				}

				// ALWAYS use current price - no retroactive trades!
				var buyPrice float64
				if quote, ok := quotes[stock.Symbol]; ok && quote.Price > 0 {
					buyPrice = quote.Price
				} else {
					addLog("ERROR", fmt.Sprintf("%s: Kein aktueller Preis verfügbar", stock.Symbol), nil)
					continue
				}

				// Use today's date for the trade
				signalStartDate = tradeDate

				// Calculate quantity: invest 100 EUR worth
				investmentEUR := 100.0
				investmentUSD := convertToUSD(investmentEUR, "EUR")
				qty := investmentUSD / buyPrice

				// Create TODO instead of executing trade
				addLog("TODO", fmt.Sprintf("%s: BUY-Vorschlag erstellt - %.4f Anteile @ $%.2f (100€ = $%.2f)",
					stock.Symbol, qty, buyPrice, investmentUSD), nil)

				saveBotTodo("lutz", "BUY", stock.Symbol, stock.Name, qty, 0, buyPrice,
					stock.Signal, stock.SignalBars, signalStartDate.Format("2006-01-02"),
					fmt.Sprintf("BUY Signal aktiv seit %d Bars", stock.SignalBars))

				actions = append(actions, map[string]interface{}{
					"action":   "BUY",
					"symbol":   stock.Symbol,
					"name":     stock.Name,
					"price":    buyPrice,
					"date":     signalStartDate.Format("2006-01-02"),
					"quantity": qty,
					"is_todo":  true,
				})
			} else {
				addLog("SKIP", fmt.Sprintf("%s: Position bereits vorhanden", stock.Symbol), nil)
			}
		} else if stock.Signal == "SELL" {
			if hasPosition {
				sellSignalDate := now.AddDate(0, -stock.SignalBars, 0)
				sellSignalDate = adjustToTradingDay(sellSignalDate)

				var sellPrice float64
				var sellDate time.Time

				sellPrice = getPriceAtDate(stock.Symbol, sellSignalDate)
				if sellPrice > 0 {
					sellDate = sellSignalDate
				} else {
					sellDate = now
					if quote, ok := quotes[stock.Symbol]; ok && quote.Price > 0 {
						sellPrice = quote.Price
					} else {
						sellPrice = stock.CurrentPrice
					}
				}

				if sellDate.Before(existingPosition.BuyDate) {
					sellDate = now
					if quote, ok := quotes[stock.Symbol]; ok && quote.Price > 0 {
						sellPrice = quote.Price
					}
				}

				var existingSell LutzTrade
				if db.Where("symbol = ? AND action = ? AND signal_date >= ?",
					stock.Symbol, "SELL", existingPosition.BuyDate).First(&existingSell).Error == nil {
					addLog("SKIP", fmt.Sprintf("%s: Bereits verkauft", stock.Symbol), nil)
					continue
				}

				// Create TODO instead of executing trade
				profitLoss := (sellPrice - existingPosition.AvgPrice) * existingPosition.Quantity
				profitLossPct := ((sellPrice - existingPosition.AvgPrice) / existingPosition.AvgPrice) * 100

				addLog("TODO", fmt.Sprintf("%s: SELL-Vorschlag erstellt - %.4f Anteile @ $%.2f (erwarteter Gewinn: $%.2f / %.2f%%)",
					stock.Symbol, existingPosition.Quantity, sellPrice, profitLoss, profitLossPct), nil)

				saveBotTodo("lutz", "SELL", stock.Symbol, stock.Name, existingPosition.Quantity,
					existingPosition.AvgPrice, sellPrice, stock.Signal, stock.SignalBars,
					sellDate.Format("2006-01-02"),
					fmt.Sprintf("SELL Signal - erwarteter Gewinn: $%.2f (%.2f%%)", profitLoss, profitLossPct))

				actions = append(actions, map[string]interface{}{
					"action":          "SELL",
					"symbol":          stock.Symbol,
					"name":            stock.Name,
					"price":           sellPrice,
					"date":            sellDate.Format("2006-01-02"),
					"quantity":        existingPosition.Quantity,
					"profit_loss":     profitLoss,
					"profit_loss_pct": profitLossPct,
					"is_todo":         true,
				})
			} else {
				addLog("SKIP", fmt.Sprintf("%s: Keine Position zum Verkaufen", stock.Symbol), nil)
			}
		}
	}

	addLog("INFO", fmt.Sprintf("Lutz Update abgeschlossen: %d Vorschläge erstellt", len(actions)), nil)

	c.JSON(http.StatusOK, gin.H{
		"message":       "Lutz update completed",
		"actions":       actions,
		"action_count":  len(actions),
		"logs":          logs,
		"todos_created": len(actions),
	})
}

func getLutzPortfolio(c *gin.Context) {
	var positions []LutzPosition
	db.Order("buy_date desc").Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	type PositionWithQuote struct {
		ID             uint      `json:"id"`
		Symbol         string    `json:"symbol"`
		Name           string    `json:"name"`
		Quantity       float64   `json:"quantity"`
		AvgPrice       float64   `json:"avg_price"`
		BuyDate        time.Time `json:"buy_date"`
		CurrentPrice   float64   `json:"current_price"`
		Change         float64   `json:"change"`
		ChangePercent  float64   `json:"change_percent"`
		TotalReturn    float64   `json:"total_return"`
		TotalReturnPct float64   `json:"total_return_pct"`
		CurrentValue   float64   `json:"current_value"`
		IsLive         bool      `json:"is_live"`
	}

	result := make([]PositionWithQuote, 0)
	totalValue := 0.0
	totalInvested := 0.0
	totalReturn := 0.0

	for _, pos := range positions {
		quote := quotes[pos.Symbol]
		currentPrice := quote.Price
		if currentPrice <= 0 {
			currentPrice = pos.AvgPrice
		}

		posReturn := (currentPrice - pos.AvgPrice) * pos.Quantity
		posReturnPct := ((currentPrice - pos.AvgPrice) / pos.AvgPrice) * 100
		posValue := currentPrice * pos.Quantity

		totalValue += posValue
		totalInvested += pos.AvgPrice * pos.Quantity
		totalReturn += posReturn

		result = append(result, PositionWithQuote{
			ID:             pos.ID,
			Symbol:         pos.Symbol,
			Name:           pos.Name,
			Quantity:       pos.Quantity,
			AvgPrice:       pos.AvgPrice,
			BuyDate:        pos.BuyDate,
			CurrentPrice:   currentPrice,
			Change:         quote.Change,
			ChangePercent:  quote.ChangePercent,
			TotalReturn:    posReturn,
			TotalReturnPct: posReturnPct,
			CurrentValue:   posValue,
			IsLive:         pos.IsLive,
		})
	}

	totalReturnPct := 0.0
	if totalInvested > 0 {
		totalReturnPct = (totalReturn / totalInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"positions":        result,
		"total_value":      totalValue,
		"total_invested":   totalInvested,
		"total_return":     totalReturn,
		"total_return_pct": totalReturnPct,
	})
}

func getLutzActions(c *gin.Context) {
	var trades []LutzTrade
	db.Order("signal_date desc").Limit(50).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getLutzPerformance(c *gin.Context) {
	var sellTrades []LutzTrade
	db.Where("action = ?", "SELL").Find(&sellTrades)

	var buyTrades []LutzTrade
	db.Where("action = ?", "BUY").Find(&buyTrades)

	wins := 0
	losses := 0
	totalProfitLoss := 0.0

	for _, trade := range sellTrades {
		if trade.ProfitLoss != nil {
			totalProfitLoss += *trade.ProfitLoss
			if *trade.ProfitLoss >= 0 {
				wins++
			} else {
				losses++
			}
		}
	}

	winRate := 0.0
	if wins+losses > 0 {
		winRate = float64(wins) / float64(wins+losses) * 100
	}

	var positions []LutzPosition
	db.Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	unrealizedGain := 0.0
	investedInPositions := 0.0
	currentValue := 0.0

	for _, pos := range positions {
		investedInPositions += pos.AvgPrice * pos.Quantity
		quote := quotes[pos.Symbol]
		if quote.Price > 0 {
			currentValue += quote.Price * pos.Quantity
			unrealizedGain += (quote.Price - pos.AvgPrice) * pos.Quantity
		} else {
			currentValue += pos.AvgPrice * pos.Quantity
		}
	}

	totalInvested := 0.0
	for _, trade := range buyTrades {
		totalInvested += trade.Price * trade.Quantity
	}

	totalReturnPct := 0.0
	if investedInPositions > 0 {
		totalReturnPct = (unrealizedGain / investedInPositions) * 100
	}

	overallReturnPct := 0.0
	if totalInvested > 0 {
		overallReturnPct = ((totalProfitLoss + unrealizedGain) / totalInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_trades":          len(sellTrades),
		"total_buys":            len(buyTrades),
		"wins":                  wins,
		"losses":                losses,
		"win_rate":              winRate,
		"realized_profit":       totalProfitLoss,
		"unrealized_gain":       unrealizedGain,
		"total_gain":            totalProfitLoss + unrealizedGain,
		"open_positions":        len(positions),
		"invested_in_positions": investedInPositions,
		"current_value":         currentValue,
		"total_invested":        totalInvested,
		"total_return_pct":      totalReturnPct,
		"overall_return_pct":    overallReturnPct,
	})
}

func resetLutz(c *gin.Context) {
	db.Where("1 = 1").Delete(&LutzTrade{})
	db.Where("1 = 1").Delete(&LutzPosition{})
	db.Where("user_id = ?", LUTZ_USER_ID).Delete(&PortfolioPosition{})

	c.JSON(http.StatusOK, gin.H{
		"message": "Lutz reset completed",
	})
}

// syncLutz synchronizes positions with trades
func syncLutz(c *gin.Context) {
	var results []map[string]interface{}

	// Get all BUY trades
	var buyTrades []LutzTrade
	db.Where("action = ?", "BUY").Order("signal_date asc").Find(&buyTrades)

	// Get all SELL trades
	var sellTrades []LutzTrade
	db.Where("action = ?", "SELL").Find(&sellTrades)

	// Build map of sells by symbol
	sellsBySymbol := make(map[string][]LutzTrade)
	for _, sell := range sellTrades {
		sellsBySymbol[sell.Symbol] = append(sellsBySymbol[sell.Symbol], sell)
	}

	// For each symbol, check if there's an open position (BUY without matching SELL)
	openBuys := make(map[string]LutzTrade)
	for _, buy := range buyTrades {
		sells := sellsBySymbol[buy.Symbol]
		hasSellAfter := false
		for _, sell := range sells {
			if sell.SignalDate.After(buy.SignalDate) || sell.SignalDate.Equal(buy.SignalDate) {
				hasSellAfter = true
				break
			}
		}
		if !hasSellAfter {
			// This is an open buy - keep the latest one
			if existing, ok := openBuys[buy.Symbol]; ok {
				if buy.SignalDate.After(existing.SignalDate) {
					openBuys[buy.Symbol] = buy
				}
			} else {
				openBuys[buy.Symbol] = buy
			}
		}
	}

	// Delete all existing positions and recreate from open buys
	db.Where("1 = 1").Delete(&LutzPosition{})
	db.Where("user_id = ?", LUTZ_USER_ID).Delete(&PortfolioPosition{})

	for symbol, buy := range openBuys {
		// Create position
		pos := LutzPosition{
			Symbol:   symbol,
			Name:     buy.Name,
			Quantity: buy.Quantity,
			AvgPrice: buy.Price,
			IsLive:   buy.IsLive,
			BuyDate:  buy.SignalDate,
		}
		db.Create(&pos)

		// Create portfolio position
		portfolioPos := PortfolioPosition{
			UserID:       LUTZ_USER_ID,
			Symbol:       symbol,
			Name:         buy.Name,
			PurchaseDate: &buy.SignalDate,
			AvgPrice:     buy.Price,
			Currency:     "USD",
			Quantity:     &buy.Quantity,
		}
		db.Create(&portfolioPos)

		results = append(results, map[string]interface{}{
			"symbol":   symbol,
			"quantity": buy.Quantity,
			"price":    buy.Price,
			"date":     buy.SignalDate.Format("2006-01-02"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          "Sync completed",
		"positions_synced": len(results),
		"positions":        results,
	})
}

// getLutzCompletedTrades returns completed trades (BUY + SELL pairs)
func getLutzCompletedTrades(c *gin.Context) {
	var trades []LutzTrade
	db.Where("action = ?", "SELL").Order("signal_date desc").Find(&trades)

	type CompletedTrade struct {
		Symbol        string    `json:"symbol"`
		Name          string    `json:"name"`
		BuyDate       time.Time `json:"buy_date"`
		BuyPrice      float64   `json:"buy_price"`
		SellDate      time.Time `json:"sell_date"`
		SellPrice     float64   `json:"sell_price"`
		Quantity      float64   `json:"quantity"`
		ProfitLoss    float64   `json:"profit_loss"`
		ProfitLossPct float64   `json:"profit_loss_pct"`
		IsLive        bool      `json:"is_live"`
	}

	var completed []CompletedTrade
	for _, sell := range trades {
		// Find the matching BUY
		var buy LutzTrade
		if err := db.Where("symbol = ? AND action = ? AND signal_date < ?",
			sell.Symbol, "BUY", sell.SignalDate).
			Order("signal_date desc").First(&buy).Error; err == nil {

			pl := 0.0
			plPct := 0.0
			if sell.ProfitLoss != nil {
				pl = *sell.ProfitLoss
			}
			if sell.ProfitLossPct != nil {
				plPct = *sell.ProfitLossPct
			}

			completed = append(completed, CompletedTrade{
				Symbol:        sell.Symbol,
				Name:          sell.Name,
				BuyDate:       buy.SignalDate,
				BuyPrice:      buy.Price,
				SellDate:      sell.SignalDate,
				SellPrice:     sell.Price,
				Quantity:      sell.Quantity,
				ProfitLoss:    pl,
				ProfitLossPct: plPct,
				IsLive:        sell.IsLive,
			})
		}
	}

	c.JSON(http.StatusOK, completed)
}

// Update Lutz position with real trade data (Admin only)
func updateLutzPosition(c *gin.Context) {
	id := c.Param("id")

	var position LutzPosition
	if err := db.First(&position, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found"})
		return
	}

	var req struct {
		Quantity float64 `json:"quantity"`
		AvgPrice float64 `json:"avg_price"`
		IsLive   bool    `json:"is_live"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Update position
	position.Quantity = req.Quantity
	position.AvgPrice = req.AvgPrice
	position.IsLive = req.IsLive
	db.Save(&position)

	// Also update corresponding portfolio position
	var portfolioPos PortfolioPosition
	if err := db.Where("user_id = ? AND symbol = ?", LUTZ_USER_ID, position.Symbol).First(&portfolioPos).Error; err == nil {
		portfolioPos.Quantity = &req.Quantity
		portfolioPos.AvgPrice = req.AvgPrice
		db.Save(&portfolioPos)
	}

	c.JSON(http.StatusOK, position)
}

// Update Lutz trade with real trade data (Admin only)
func updateLutzTrade(c *gin.Context) {
	id := c.Param("id")

	var trade LutzTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	var req struct {
		Quantity float64 `json:"quantity"`
		Price    float64 `json:"price"`
		IsLive   bool    `json:"is_live"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Update trade
	trade.Quantity = req.Quantity
	trade.Price = req.Price
	trade.IsLive = req.IsLive
	db.Save(&trade)

	// If BUY trade, update corresponding position
	if trade.Action == "BUY" {
		var position LutzPosition
		if err := db.Where("symbol = ?", trade.Symbol).First(&position).Error; err == nil {
			position.Quantity = req.Quantity
			position.AvgPrice = req.Price
			position.IsLive = req.IsLive
			db.Save(&position)

			// Also update portfolio position
			var portfolioPos PortfolioPosition
			if err := db.Where("user_id = ? AND symbol = ?", LUTZ_USER_ID, trade.Symbol).First(&portfolioPos).Error; err == nil {
				portfolioPos.Quantity = &req.Quantity
				portfolioPos.AvgPrice = req.Price
				db.Save(&portfolioPos)
			}
		}
	}

	c.JSON(http.StatusOK, trade)
}

// deleteLutzTrade deletes a trade and its associated position
func deleteLutzTrade(c *gin.Context) {
	id := c.Param("id")

	var trade LutzTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	symbol := trade.Symbol

	// Delete the trade
	db.Delete(&trade)

	// If it was a BUY trade, also delete the position
	if trade.Action == "BUY" {
		db.Where("symbol = ?", symbol).Delete(&LutzPosition{})
		db.Where("user_id = ? AND symbol = ?", LUTZ_USER_ID, symbol).Delete(&PortfolioPosition{})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Trade deleted", "symbol": symbol})
}

// getLutzPending returns pending actions for Lutz (aggressive mode)
func getLutzPending(c *gin.Context) {
	var pending []map[string]interface{}

	// Get all current positions
	var positions []LutzPosition
	db.Find(&positions)

	// Get tracked stocks performance data (aggressive mode)
	var trackedStocks []AggressiveStockPerformance
	db.Find(&trackedStocks)

	// Create a map for quick lookup
	stockSignals := make(map[string]AggressiveStockPerformance)
	for _, s := range trackedStocks {
		stockSignals[s.Symbol] = s
	}

	// Check positions that need to be sold (signal changed to SELL)
	for _, pos := range positions {
		if perf, ok := stockSignals[pos.Symbol]; ok {
			if perf.Signal == "SELL" {
				signalSince := time.Now().AddDate(0, -perf.SignalBars, 0).Format("2006-01-02")
				pending = append(pending, map[string]interface{}{
					"type":         "SELL",
					"symbol":       pos.Symbol,
					"name":         pos.Name,
					"quantity":     pos.Quantity,
					"avg_price":    pos.AvgPrice,
					"signal":       perf.Signal,
					"signal_bars":  perf.SignalBars,
					"signal_since": signalSince,
					"reason":       "Position hat SELL-Signal",
				})
				// Create/update todo
				saveBotTodo("lutz", "SELL", pos.Symbol, pos.Name, pos.Quantity, pos.AvgPrice, 0, perf.Signal, perf.SignalBars, signalSince, "Position hat SELL-Signal")
			}
		}
	}

	// Check tracked stocks with BUY signal that we don't own yet
	positionSymbols := make(map[string]bool)
	for _, p := range positions {
		positionSymbols[p.Symbol] = true
	}

	for _, stock := range trackedStocks {
		if stock.Signal == "BUY" && !positionSymbols[stock.Symbol] {
			// Check if we already have a buy trade without subsequent sell
			var existingBuy LutzTrade
			alreadyBought := db.Where("symbol = ? AND action = ?", stock.Symbol, "BUY").
				Order("signal_date desc").First(&existingBuy).Error == nil

			if alreadyBought {
				var lastSell LutzTrade
				hasSoldAfter := db.Where("symbol = ? AND action = ? AND signal_date > ?",
					stock.Symbol, "SELL", existingBuy.SignalDate).First(&lastSell).Error == nil
				if !hasSoldAfter {
					continue // Already bought, skip
				}
			}

			signalSince := time.Now().AddDate(0, -stock.SignalBars, 0).Format("2006-01-02")
			pending = append(pending, map[string]interface{}{
				"type":         "BUY",
				"symbol":       stock.Symbol,
				"name":         stock.Name,
				"signal":       stock.Signal,
				"signal_bars":  stock.SignalBars,
				"signal_since": signalSince,
				"reason":       "Neues BUY-Signal erkannt",
			})
			// Create/update todo
			saveBotTodo("lutz", "BUY", stock.Symbol, stock.Name, 0, 0, 0, stock.Signal, stock.SignalBars, signalSince, "Neues BUY-Signal erkannt")
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"pending": pending,
		"count":   len(pending),
	})
}

// getLutzLogs returns the last 100 logs for Lutz
func getLutzLogs(c *gin.Context) {
	var logs []BotLog
	db.Where("bot = ?", "lutz").Order("created_at desc").Limit(100).Find(&logs)
	c.JSON(http.StatusOK, logs)
}

// getLutzTodos returns all todos for Lutz (open first, then done)
func getLutzTodos(c *gin.Context) {
	var todos []BotTodo
	db.Where("bot = ?", "lutz").Order("done asc, created_at desc").Find(&todos)
	c.JSON(http.StatusOK, todos)
}

// markLutzTodoDone marks a todo as done (discarded)
func markLutzTodoDone(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	now := time.Now()
	todo.Done = true
	todo.Decision = "discarded"
	todo.DoneAt = &now
	db.Save(&todo)
	c.JSON(http.StatusOK, todo)
}

// reopenLutzTodo reopens a done todo
func reopenLutzTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	if !todo.Done {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Todo is not done"})
		return
	}
	todo.Done = false
	todo.Decision = ""
	todo.DoneAt = nil
	todo.UpdatedAt = time.Now()
	db.Save(&todo)
	c.JSON(http.StatusOK, todo)
}

// deleteLutzTodo deletes a done todo
func deleteLutzTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	db.Delete(&todo)
	c.JSON(http.StatusOK, gin.H{"message": "Todo deleted"})
}

// Category management functions

// ensureSonstigesCategory ensures the default "Sonstiges" category exists
func ensureSonstigesCategory() {
	var count int64
	db.Model(&Category{}).Count(&count)
	if count == 0 {
		// Create default "Sonstiges" category
		sonstiges := Category{
			Name:      "Sonstiges",
			SortOrder: 9999, // Always last
		}
		db.Create(&sonstiges)
	}
}

// getCategories returns all categories sorted by order
func getCategories(c *gin.Context) {
	var categories []Category
	db.Order("sort_order asc, name asc").Find(&categories)
	c.JSON(http.StatusOK, categories)
}

// createCategory creates a new category
func createCategory(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}

	// Get max sort order
	var maxOrder int
	db.Model(&Category{}).Select("COALESCE(MAX(sort_order), 0)").Where("sort_order < 9999").Scan(&maxOrder)

	category := Category{
		Name:      req.Name,
		SortOrder: maxOrder + 1,
	}
	if err := db.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}
	c.JSON(http.StatusOK, category)
}

// updateCategory updates a category's name
func updateCategory(c *gin.Context) {
	id := c.Param("id")
	var category Category
	if err := db.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}

	category.Name = req.Name
	db.Save(&category)
	c.JSON(http.StatusOK, category)
}

// deleteCategory deletes a category and moves its stocks to "Sonstiges"
func deleteCategory(c *gin.Context) {
	id := c.Param("id")
	var category Category
	if err := db.First(&category, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	// Don't allow deleting "Sonstiges"
	if category.Name == "Sonstiges" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete default category"})
		return
	}

	// Find "Sonstiges" category
	var sonstiges Category
	if err := db.Where("name = ?", "Sonstiges").First(&sonstiges).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Default category not found"})
		return
	}

	// Move all stocks from this category to "Sonstiges"
	db.Model(&Stock{}).Where("category_id = ?", category.ID).Update("category_id", sonstiges.ID)

	// Delete category
	db.Delete(&category)
	c.JSON(http.StatusOK, gin.H{"message": "Category deleted"})
}

// reorderCategories updates the sort order of categories
func reorderCategories(c *gin.Context) {
	var req struct {
		Order []uint `json:"order" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order array is required"})
		return
	}

	for i, id := range req.Order {
		db.Model(&Category{}).Where("id = ?", id).Update("sort_order", i+1)
	}

	// Ensure "Sonstiges" stays at the end
	db.Model(&Category{}).Where("name = ?", "Sonstiges").Update("sort_order", 9999)

	c.JSON(http.StatusOK, gin.H{"message": "Order updated"})
}

// updateStockCategory updates the category of a stock
func updateStockCategory(c *gin.Context) {
	id := c.Param("id")
	var stock Stock
	if err := db.First(&stock, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Stock not found"})
		return
	}

	var req struct {
		CategoryID *uint `json:"category_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// If no category provided, set to "Sonstiges"
	if req.CategoryID == nil {
		var sonstiges Category
		if err := db.Where("name = ?", "Sonstiges").First(&sonstiges).Error; err == nil {
			req.CategoryID = &sonstiges.ID
		}
	}

	stock.CategoryID = req.CategoryID
	db.Save(&stock)
	c.JSON(http.StatusOK, gin.H{"message": "Stock category updated"})
}
