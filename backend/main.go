package main

import (
	"encoding/json"
	"fmt"
	"io"
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
	IPAddress string    `json:"ip_address"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

type Stock struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Symbol        string    `json:"symbol" gorm:"not null;uniqueIndex"`
	Name          string    `json:"name" gorm:"not null"`
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

var db *gorm.DB
var sessions = make(map[string]Session)
var httpClient = &http.Client{Timeout: 10 * time.Second}

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

	db.AutoMigrate(&User{}, &Stock{}, &PortfolioPosition{}, &StockPerformance{}, &ActivityLog{})

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
		api.GET("/search", searchStocks)
		api.GET("/quote/:symbol", getQuote)
		api.GET("/history/:symbol", getHistory)

		// Portfolio routes
		api.GET("/portfolio", authMiddleware(), getPortfolio)
		api.POST("/portfolio", authMiddleware(), createPortfolioPosition)
		api.PUT("/portfolio/:id", authMiddleware(), updatePortfolioPosition)
		api.DELETE("/portfolio/:id", authMiddleware(), deletePortfolioPosition)
		api.GET("/portfolio/performance", authMiddleware(), getPortfolioPerformance)
		api.GET("/portfolio/history", authMiddleware(), getPortfolioHistory)
		api.GET("/portfolios/compare", authMiddleware(), getAllPortfoliosForComparison)
		api.GET("/portfolios/history/:userId", authMiddleware(), getUserPortfolioHistory)

		// Stock Performance Tracker routes
		api.POST("/performance", saveStockPerformance)
		api.GET("/performance", getTrackedStocks)
		api.GET("/performance/:symbol", getStockPerformance)

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
	token := uuid.New().String()
	sessions[token] = Session{
		UserID:  user.ID,
		IsAdmin: user.IsAdmin,
		Expiry:  time.Now().Add(7 * 24 * time.Hour),
	}

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

	token := uuid.New().String()
	sessions[token] = Session{
		UserID:  user.ID,
		IsAdmin: user.IsAdmin,
		Expiry:  time.Now().Add(7 * 24 * time.Hour),
	}

	// Update login count and last active
	db.Model(&user).Updates(map[string]interface{}{
		"login_count": user.LoginCount + 1,
		"last_active": time.Now(),
	})

	// Log activity
	logUserActivity(user.ID, user.Username, "login", "", c.ClientIP())

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
		delete(sessions, token)
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
	session, exists := sessions[token]
	if !exists || time.Now().After(session.Expiry) {
		delete(sessions, token)
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
		session, exists := sessions[token]
		if !exists || time.Now().After(session.Expiry) {
			delete(sessions, token)
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
			session, exists := sessions[token]
			if exists && !time.Now().After(session.Expiry) {
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
func logUserActivity(userID uint, username, action, details, ip string) {
	log := ActivityLog{
		UserID:    userID,
		Username:  username,
		Action:    action,
		Details:   details,
		IPAddress: ip,
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

	symbols := make([]string, len(stocks))
	for i, s := range stocks {
		symbols[i] = s.Symbol
	}

	quotes := fetchQuotes(symbols)

	result := make([]StockWithQuote, len(stocks))
	for i, stock := range stocks {
		result[i] = StockWithQuote{
			ID:        stock.ID,
			Symbol:    stock.Symbol,
			Name:      stock.Name,
			CreatedAt: stock.CreatedAt,
		}
		if q, ok := quotes[stock.Symbol]; ok {
			result[i].Price = q.Price
			result[i].Change = q.Change
			result[i].ChangePercent = q.ChangePercent
			result[i].PrevClose = q.PrevClose
		}
	}

	c.JSON(http.StatusOK, result)
}

type QuoteData struct {
	Price         float64
	Change        float64
	ChangePercent float64
	PrevClose     float64
}

func fetchQuotes(symbols []string) map[string]QuoteData {
	result := make(map[string]QuoteData)
	if len(symbols) == 0 {
		return result
	}

	symbolsStr := strings.Join(symbols, ",")

	sparkURL := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/spark?symbols=%s&range=1d&interval=1d", url.QueryEscape(symbolsStr))
	if sparkResult := trySparkAPI(sparkURL); len(sparkResult) > 0 {
		return sparkResult
	}

	apiURL := fmt.Sprintf("https://query2.finance.yahoo.com/v7/finance/quote?symbols=%s", url.QueryEscape(symbolsStr))
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := httpClient.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var yahooResp YahooQuoteResponse
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return result
	}

	for _, q := range yahooResp.QuoteResponse.Result {
		result[q.Symbol] = QuoteData{
			Price:         q.RegularMarketPrice,
			Change:        q.RegularMarketChange,
			ChangePercent: q.RegularMarketChangePercent,
			PrevClose:     q.RegularMarketPreviousClose,
		}
	}

	return result
}

type SparkQuote struct {
	Symbol             string    `json:"symbol"`
	Timestamp          []int64   `json:"timestamp"`
	Close              []float64 `json:"close"`
	ChartPreviousClose float64   `json:"chartPreviousClose"`
	PreviousClose      float64   `json:"previousClose"`
}

func trySparkAPI(apiURL string) map[string]QuoteData {
	result := make(map[string]QuoteData)

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return result
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var sparkResp map[string]SparkQuote
	if err := json.Unmarshal(body, &sparkResp); err != nil {
		return result
	}

	for symbol, data := range sparkResp {
		if len(data.Close) > 0 {
			price := data.Close[len(data.Close)-1]
			prevClose := data.ChartPreviousClose
			if prevClose == 0 {
				prevClose = data.PreviousClose
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

	stock := Stock{
		Symbol:        symbol,
		Name:          name,
		AddedByUserID: userID.(uint),
		AddedByUser:   username,
	}

	db.Create(&stock)

	// Log activity
	logUserActivity(userID.(uint), username, "add_stock", fmt.Sprintf(`{"symbol":"%s"}`, symbol), c.ClientIP())

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
	"EUR": 0.92, // fallback
	"GBP": 0.79, // fallback
	"CHF": 0.88, // fallback
}
var exchangeRatesLastFetched time.Time
var exchangeRatesMutex = &sync.Mutex{}

// FrankfurterResponse represents the API response from frankfurter.app
type FrankfurterResponse struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

// Fetch live exchange rates from frankfurter.app (same API as frontend)
func fetchLiveExchangeRates() {
	exchangeRatesMutex.Lock()
	defer exchangeRatesMutex.Unlock()

	// Only fetch if rates are older than 1 hour
	if time.Since(exchangeRatesLastFetched) < time.Hour {
		return
	}

	apiURL := "https://api.frankfurter.app/latest?from=USD&to=EUR,GBP,CHF"
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "FlipperCapital/1.0")

	resp, err := httpClient.Do(req)
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

			// Calculate returns by converting current USD price to user's currency
			// This ensures comparison is in the same currency as the user entered their purchase price
			currentPriceInUserCurrency := convertFromUSD(q.Price, currency)

			if pos.AvgPrice > 0 {
				// Return in user's currency terms
				result[i].TotalReturn = currentPriceInUserCurrency - pos.AvgPrice
				result[i].TotalReturnPct = ((currentPriceInUserCurrency - pos.AvgPrice) / pos.AvgPrice) * 100
			}

			// Calculate values if quantity is set (in USD)
			if pos.Quantity != nil && *pos.Quantity > 0 {
				result[i].CurrentValue = q.Price * (*pos.Quantity)
				result[i].InvestedValue = avgPriceUSD * (*pos.Quantity)
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
				// Convert current USD price to user's currency for proper comparison
				currentPriceInUserCurrency := convertFromUSD(q.Price, currency)

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
				// Convert current USD price to user's currency for proper comparison
				currentPriceInUserCurrency := convertFromUSD(q.Price, currency)

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
					// Convert current USD price to user's currency for proper comparison
					currentPriceInUserCurrency := convertFromUSD(q.Price, currency)
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
		var totalReturnPct float64
		validCount := 0

		for _, pos := range positions {
			currency := pos.Currency
			if currency == "" {
				currency = "EUR"
			}
			avgPriceUSD := convertToUSD(pos.AvgPrice, currency)

			summary := PositionSummary{
				Symbol:      pos.Symbol,
				Name:        pos.Name,
				AvgPrice:    pos.AvgPrice,
				AvgPriceUSD: avgPriceUSD,
				Currency:    currency,
			}

			if q, ok := quotes[pos.Symbol]; ok {
				summary.CurrentPrice = q.Price
				summary.ChangePercent = q.ChangePercent
				// Convert current USD price to user's currency for proper return calculation
				currentPriceInUserCurrency := convertFromUSD(q.Price, currency)
				if pos.AvgPrice > 0 {
					summary.TotalReturnPct = ((currentPriceInUserCurrency - pos.AvgPrice) / pos.AvgPrice) * 100
					totalReturnPct += summary.TotalReturnPct
					validCount++
				}
			}

			posSummaries = append(posSummaries, summary)
		}

		avgReturn := 0.0
		if validCount > 0 {
			avgReturn = totalReturnPct / float64(validCount)
		}

		portfolios = append(portfolios, PortfolioSummary{
			UserID:         user.ID,
			Username:       user.Username,
			Positions:      posSummaries,
			TotalReturnPct: avgReturn,
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
						// Convert USD price to user's currency
						currency := pos.Currency
						if currency == "" {
							currency = "EUR"
						}
						priceInUserCurrency := convertFromUSD(price, currency)
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
					// Convert USD price to user's currency for proper comparison
					priceInUserCurrency := convertFromUSD(price, currency)
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

	logUserActivity(uid, username, req.Action, req.Details, c.ClientIP())
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
