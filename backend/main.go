package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
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
	MarketCap     int64     `json:"market_cap" gorm:"default:0"`
	ISIN          string    `json:"isin"`
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
	ID              uint      `json:"id" gorm:"primaryKey"`
	Symbol          string    `json:"symbol" gorm:"uniqueIndex;not null"`
	Name            string    `json:"name"`
	WinRate         float64   `json:"win_rate"`
	RiskReward      float64   `json:"risk_reward"`
	TotalReturn     float64   `json:"total_return"`
	AvgReturn       float64   `json:"avg_return"`
	TotalTrades     int       `json:"total_trades"`
	Wins            int       `json:"wins"`
	Losses          int       `json:"losses"`
	Signal          string    `json:"signal"`
	SignalBars      int       `json:"signal_bars"`
	SignalSince     string    `json:"signal_since"`
	PrevSignal      string    `json:"prev_signal"`
	PrevSignalSince string    `json:"prev_signal_since"`
	TradesJSON      string    `json:"trades_json" gorm:"type:text"`
	CurrentPrice    float64   `json:"current_price"`
	MarketCap       int64     `json:"market_cap" gorm:"default:0"`
	ISIN            string    `json:"isin"`
	UpdatedAt       time.Time `json:"updated_at"`
	CreatedAt       time.Time `json:"created_at"`
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
	ID            uint       `json:"id" gorm:"primaryKey"`
	Symbol        string     `json:"symbol" gorm:"index;not null"`
	Name          string     `json:"name"`
	Action        string     `json:"action" gorm:"not null"` // BUY or SELL
	Quantity      float64    `json:"quantity" gorm:"default:1"`
	IsLive        bool       `json:"is_live" gorm:"default:false"`
	IsPending     bool       `json:"is_pending" gorm:"default:false"`
	IsDeleted     bool       `json:"is_deleted" gorm:"default:false"`
	IsRead        bool       `json:"is_read" gorm:"default:false"`
	IsAdminClosed bool       `json:"is_admin_closed" gorm:"default:false"`
	Price         float64    `json:"price" gorm:"not null"`
	SignalDate    time.Time  `json:"signal_date" gorm:"not null"`
	ExecutedAt    time.Time  `json:"executed_at" gorm:"not null"`
	ProfitLoss    *float64   `json:"profit_loss"`
	ProfitLossPct *float64   `json:"profit_loss_pct"`
	IsStopLoss        bool   `json:"is_stop_loss" gorm:"default:false"`
	IsFilterBlocked   bool   `json:"is_filter_blocked" gorm:"default:false"`
	FilterBlockReason string `json:"filter_block_reason" gorm:"type:text"`
	CreatedAt         time.Time `json:"created_at"`
}

// FlipperBotPosition tracks current open positions of the FlipperBot
type FlipperBotPosition struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Symbol        string     `json:"symbol" gorm:"index;not null"`
	Name          string     `json:"name"`
	Quantity      float64    `json:"quantity" gorm:"default:1"`
	AvgPrice      float64    `json:"avg_price" gorm:"not null"`
	InvestedEUR   float64    `json:"invested_eur" gorm:"default:0"`
	IsLive        bool       `json:"is_live" gorm:"default:false"`
	IsPending     bool       `json:"is_pending" gorm:"default:false"`
	IsClosed      bool       `json:"is_closed" gorm:"default:false"`
	SellPrice     float64    `json:"sell_price" gorm:"default:0"`
	SellDate      *time.Time `json:"sell_date"`
	ProfitLoss    *float64   `json:"profit_loss"`
	ProfitLossPct *float64   `json:"profit_loss_pct"`
	IsAdminClosed   bool       `json:"is_admin_closed" gorm:"default:false"`
	BuyDate         time.Time  `json:"buy_date" gorm:"not null"`
	StopLossPercent *float64   `json:"stop_loss_percent" gorm:"default:null"`
	StopLossType    string     `json:"stop_loss_type" gorm:"default:trailing"`
	HighestPrice    float64    `json:"highest_price" gorm:"default:0"`
	StopLossPrice   float64    `json:"stop_loss_price" gorm:"default:0"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

const FLIPPERBOT_START_DATE = "2026-01-01"
const FLIPPERBOT_USER_ID = 999999 // Special user ID for FlipperBot
const LUTZ_USER_ID = 999998       // Special user ID for Lutz (aggressive mode bot)
const QUANT_USER_ID = 999997      // Special user ID for Quant bot
const DITZ_USER_ID = 999996       // Special user ID for Ditz bot
const TRADER_USER_ID = 999995     // Special user ID for Trader bot

// AggressiveStockPerformance stores performance data for aggressive trading mode
type AggressiveStockPerformance struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	Symbol          string    `json:"symbol" gorm:"uniqueIndex;not null"`
	Name            string    `json:"name"`
	WinRate         float64   `json:"win_rate"`
	RiskReward      float64   `json:"risk_reward"`
	TotalReturn     float64   `json:"total_return"`
	AvgReturn       float64   `json:"avg_return"`
	TotalTrades     int       `json:"total_trades"`
	Wins            int       `json:"wins"`
	Losses          int       `json:"losses"`
	Signal          string    `json:"signal"`
	SignalBars      int       `json:"signal_bars"`
	SignalSince     string    `json:"signal_since"`
	PrevSignal      string    `json:"prev_signal"`
	PrevSignalSince string    `json:"prev_signal_since"`
	TradesJSON      string    `json:"trades_json" gorm:"type:text"`
	CurrentPrice    float64   `json:"current_price"`
	MarketCap       int64     `json:"market_cap" gorm:"default:0"`
	ISIN            string    `json:"isin"`
	UpdatedAt       time.Time `json:"updated_at"`
	CreatedAt       time.Time `json:"created_at"`
}

// LutzTrade tracks all trades made by the Lutz bot (aggressive mode)
type LutzTrade struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Symbol        string     `json:"symbol" gorm:"index;not null"`
	Name          string     `json:"name"`
	Action        string     `json:"action" gorm:"not null"` // BUY or SELL
	Quantity      float64    `json:"quantity" gorm:"default:1"`
	IsLive        bool       `json:"is_live" gorm:"default:false"`
	IsPending     bool       `json:"is_pending" gorm:"default:false"`
	IsDeleted     bool       `json:"is_deleted" gorm:"default:false"`
	IsRead        bool       `json:"is_read" gorm:"default:false"`
	IsAdminClosed bool       `json:"is_admin_closed" gorm:"default:false"`
	Price         float64    `json:"price" gorm:"not null"`
	SignalDate    time.Time  `json:"signal_date" gorm:"not null"`
	ExecutedAt    time.Time  `json:"executed_at" gorm:"not null"`
	ProfitLoss    *float64   `json:"profit_loss"`
	ProfitLossPct *float64   `json:"profit_loss_pct"`
	IsStopLoss        bool   `json:"is_stop_loss" gorm:"default:false"`
	IsFilterBlocked   bool   `json:"is_filter_blocked" gorm:"default:false"`
	FilterBlockReason string `json:"filter_block_reason" gorm:"type:text"`
	CreatedAt         time.Time `json:"created_at"`
}

// LutzPosition tracks current open positions of the Lutz bot
type LutzPosition struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Symbol        string     `json:"symbol" gorm:"index;not null"`
	Name          string     `json:"name"`
	Quantity      float64    `json:"quantity" gorm:"default:1"`
	AvgPrice      float64    `json:"avg_price" gorm:"not null"`
	InvestedEUR   float64    `json:"invested_eur" gorm:"default:0"`
	IsLive        bool       `json:"is_live" gorm:"default:false"`
	IsPending     bool       `json:"is_pending" gorm:"default:false"`
	IsClosed      bool       `json:"is_closed" gorm:"default:false"`
	SellPrice     float64    `json:"sell_price" gorm:"default:0"`
	SellDate      *time.Time `json:"sell_date"`
	ProfitLoss    *float64   `json:"profit_loss"`
	ProfitLossPct *float64   `json:"profit_loss_pct"`
	IsAdminClosed   bool       `json:"is_admin_closed" gorm:"default:false"`
	BuyDate         time.Time  `json:"buy_date" gorm:"not null"`
	StopLossPercent *float64   `json:"stop_loss_percent" gorm:"default:null"`
	StopLossType    string     `json:"stop_loss_type" gorm:"default:trailing"`
	HighestPrice    float64    `json:"highest_price" gorm:"default:0"`
	StopLossPrice   float64    `json:"stop_loss_price" gorm:"default:0"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
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

// BXtrenderConfig stores the configurable parameters for B-Xtrender indicator
type BXtrenderConfig struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Mode       string    `json:"mode" gorm:"uniqueIndex;not null"` // "defensive" or "aggressive"
	ShortL1    int       `json:"short_l1" gorm:"default:5"`
	ShortL2    int       `json:"short_l2" gorm:"default:20"`
	ShortL3    int       `json:"short_l3" gorm:"default:15"`
	LongL1     int       `json:"long_l1" gorm:"default:20"`
	LongL2     int       `json:"long_l2" gorm:"default:15"`
	TslPercent float64   `json:"tsl_percent" gorm:"default:20.0"`
	TslEnabled bool      `json:"tsl_enabled" gorm:"default:true"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// BXtrenderQuantConfig stores configuration for Quant mode (QuantTherapy algorithm)
type BXtrenderQuantConfig struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	ShortL1      int       `json:"short_l1" gorm:"default:5"`        // Short EMA fast period
	ShortL2      int       `json:"short_l2" gorm:"default:20"`       // Short EMA slow period
	ShortL3      int       `json:"short_l3" gorm:"default:15"`       // Short RSI period
	LongL1       int       `json:"long_l1" gorm:"default:20"`        // Long EMA period
	LongL2       int       `json:"long_l2" gorm:"default:15"`        // Long RSI period
	MaFilterOn   bool      `json:"ma_filter_on" gorm:"default:true"` // Enable MA filter
	MaLength     int       `json:"ma_length" gorm:"default:200"`     // MA filter length
	MaType       string    `json:"ma_type" gorm:"default:EMA"`       // MA type: "EMA" or "SMA"
	TslPercent   float64   `json:"tsl_percent" gorm:"default:20.0"`  // Trailing stop loss percentage
	TslEnabled   bool      `json:"tsl_enabled" gorm:"default:true"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// QuantStockPerformance stores performance data for Quant trading mode
type QuantStockPerformance struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	Symbol          string    `json:"symbol" gorm:"uniqueIndex;not null"`
	Name            string    `json:"name"`
	WinRate         float64   `json:"win_rate"`
	RiskReward      float64   `json:"risk_reward"`
	TotalReturn     float64   `json:"total_return"`
	AvgReturn       float64   `json:"avg_return"`
	TotalTrades     int       `json:"total_trades"`
	Wins            int       `json:"wins"`
	Losses          int       `json:"losses"`
	Signal          string    `json:"signal"`
	SignalBars      int       `json:"signal_bars"`
	SignalSince     string    `json:"signal_since"`
	PrevSignal      string    `json:"prev_signal"`
	PrevSignalSince string    `json:"prev_signal_since"`
	TradesJSON      string    `json:"trades_json" gorm:"type:text"`
	CurrentPrice    float64   `json:"current_price"`
	MarketCap       int64     `json:"market_cap" gorm:"default:0"`
	ISIN            string    `json:"isin"`
	UpdatedAt       time.Time `json:"updated_at"`
	CreatedAt       time.Time `json:"created_at"`
}

// QuantTrade tracks all trades made by the Quant bot
type QuantTrade struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Symbol        string     `json:"symbol" gorm:"index;not null"`
	Name          string     `json:"name"`
	Action        string     `json:"action" gorm:"not null"` // BUY or SELL
	Quantity      float64    `json:"quantity" gorm:"default:1"`
	IsLive        bool       `json:"is_live" gorm:"default:false"`
	IsPending     bool       `json:"is_pending" gorm:"default:false"` // Quant bot executes trades directly
	IsDeleted     bool       `json:"is_deleted" gorm:"default:false"`
	IsRead        bool       `json:"is_read" gorm:"default:false"`
	IsAdminClosed bool       `json:"is_admin_closed" gorm:"default:false"`
	Price         float64    `json:"price" gorm:"not null"`
	SignalDate    time.Time  `json:"signal_date" gorm:"not null"`
	ExecutedAt    time.Time  `json:"executed_at" gorm:"not null"`
	ProfitLoss    *float64   `json:"profit_loss"`
	ProfitLossPct *float64   `json:"profit_loss_pct"`
	IsStopLoss        bool   `json:"is_stop_loss" gorm:"default:false"`
	IsFilterBlocked   bool   `json:"is_filter_blocked" gorm:"default:false"`
	FilterBlockReason string `json:"filter_block_reason" gorm:"type:text"`
	CreatedAt         time.Time `json:"created_at"`
}

// QuantPosition tracks current open positions of the Quant bot
type QuantPosition struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Symbol        string     `json:"symbol" gorm:"index;not null"`
	Name          string     `json:"name"`
	Quantity      float64    `json:"quantity" gorm:"default:1"`
	AvgPrice      float64    `json:"avg_price" gorm:"not null"`
	InvestedEUR   float64    `json:"invested_eur" gorm:"default:0"`
	IsLive        bool       `json:"is_live" gorm:"default:false"`
	IsPending     bool       `json:"is_pending" gorm:"default:false"`
	IsClosed      bool       `json:"is_closed" gorm:"default:false"`
	SellPrice     float64    `json:"sell_price" gorm:"default:0"`
	SellDate      *time.Time `json:"sell_date"`
	ProfitLoss    *float64   `json:"profit_loss"`
	ProfitLossPct *float64   `json:"profit_loss_pct"`
	IsAdminClosed   bool       `json:"is_admin_closed" gorm:"default:false"`
	BuyDate         time.Time  `json:"buy_date" gorm:"not null"`
	StopLossPercent *float64   `json:"stop_loss_percent" gorm:"default:null"`
	StopLossType    string     `json:"stop_loss_type" gorm:"default:trailing"`
	HighestPrice    float64    `json:"highest_price" gorm:"default:0"`
	StopLossPrice   float64    `json:"stop_loss_price" gorm:"default:0"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// BXtrenderDitzConfig stores BXtrender configuration for Ditz mode
type BXtrenderDitzConfig struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	ShortL1      int       `json:"short_l1" gorm:"default:5"`
	ShortL2      int       `json:"short_l2" gorm:"default:20"`
	ShortL3      int       `json:"short_l3" gorm:"default:15"`
	LongL1       int       `json:"long_l1" gorm:"default:20"`
	LongL2       int       `json:"long_l2" gorm:"default:15"`
	MaFilterOn   bool      `json:"ma_filter_on" gorm:"default:true"`
	MaLength     int       `json:"ma_length" gorm:"default:200"`
	MaType       string    `json:"ma_type" gorm:"default:EMA"`
	TslPercent   float64   `json:"tsl_percent" gorm:"default:20.0"`
	TslEnabled   bool      `json:"tsl_enabled" gorm:"default:true"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DitzStockPerformance stores performance data for Ditz trading mode
type DitzStockPerformance struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	Symbol          string    `json:"symbol" gorm:"uniqueIndex;not null"`
	Name            string    `json:"name"`
	WinRate         float64   `json:"win_rate"`
	RiskReward      float64   `json:"risk_reward"`
	TotalReturn     float64   `json:"total_return"`
	AvgReturn       float64   `json:"avg_return"`
	TotalTrades     int       `json:"total_trades"`
	Wins            int       `json:"wins"`
	Losses          int       `json:"losses"`
	Signal          string    `json:"signal"`
	SignalBars      int       `json:"signal_bars"`
	SignalSince     string    `json:"signal_since"`
	PrevSignal      string    `json:"prev_signal"`
	PrevSignalSince string    `json:"prev_signal_since"`
	TradesJSON      string    `json:"trades_json" gorm:"type:text"`
	CurrentPrice    float64   `json:"current_price"`
	MarketCap       int64     `json:"market_cap" gorm:"default:0"`
	ISIN            string    `json:"isin"`
	UpdatedAt       time.Time `json:"updated_at"`
	CreatedAt       time.Time `json:"created_at"`
}

// DitzTrade tracks all trades made by the Ditz bot
type DitzTrade struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Symbol        string     `json:"symbol" gorm:"index;not null"`
	Name          string     `json:"name"`
	Action        string     `json:"action" gorm:"not null"`
	Quantity      float64    `json:"quantity" gorm:"default:1"`
	IsLive        bool       `json:"is_live" gorm:"default:false"`
	IsPending     bool       `json:"is_pending" gorm:"default:false"`
	IsDeleted     bool       `json:"is_deleted" gorm:"default:false"`
	IsRead        bool       `json:"is_read" gorm:"default:false"`
	IsAdminClosed bool       `json:"is_admin_closed" gorm:"default:false"`
	Price         float64    `json:"price" gorm:"not null"`
	SignalDate    time.Time  `json:"signal_date" gorm:"not null"`
	ExecutedAt    time.Time  `json:"executed_at" gorm:"not null"`
	ProfitLoss    *float64   `json:"profit_loss"`
	ProfitLossPct *float64   `json:"profit_loss_pct"`
	IsStopLoss        bool   `json:"is_stop_loss" gorm:"default:false"`
	IsFilterBlocked   bool   `json:"is_filter_blocked" gorm:"default:false"`
	FilterBlockReason string `json:"filter_block_reason" gorm:"type:text"`
	CreatedAt         time.Time `json:"created_at"`
}

// DitzPosition tracks current open positions of the Ditz bot
type DitzPosition struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Symbol        string     `json:"symbol" gorm:"index;not null"`
	Name          string     `json:"name"`
	Quantity      float64    `json:"quantity" gorm:"default:1"`
	AvgPrice      float64    `json:"avg_price" gorm:"not null"`
	InvestedEUR   float64    `json:"invested_eur" gorm:"default:0"`
	IsLive        bool       `json:"is_live" gorm:"default:false"`
	IsPending     bool       `json:"is_pending" gorm:"default:false"`
	IsClosed      bool       `json:"is_closed" gorm:"default:false"`
	SellPrice     float64    `json:"sell_price" gorm:"default:0"`
	SellDate      *time.Time `json:"sell_date"`
	ProfitLoss    *float64   `json:"profit_loss"`
	ProfitLossPct *float64   `json:"profit_loss_pct"`
	IsAdminClosed   bool       `json:"is_admin_closed" gorm:"default:false"`
	BuyDate         time.Time  `json:"buy_date" gorm:"not null"`
	StopLossPercent *float64   `json:"stop_loss_percent" gorm:"default:null"`
	StopLossType    string     `json:"stop_loss_type" gorm:"default:trailing"`
	HighestPrice    float64    `json:"highest_price" gorm:"default:0"`
	StopLossPrice   float64    `json:"stop_loss_price" gorm:"default:0"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// BXtrenderTraderConfig stores BXtrender configuration for Trader mode (like Ditz but MA filter off by default)
type BXtrenderTraderConfig struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	ShortL1      int       `json:"short_l1" gorm:"default:5"`
	ShortL2      int       `json:"short_l2" gorm:"default:20"`
	ShortL3      int       `json:"short_l3" gorm:"default:15"`
	LongL1       int       `json:"long_l1" gorm:"default:20"`
	LongL2       int       `json:"long_l2" gorm:"default:15"`
	MaFilterOn   bool      `json:"ma_filter_on" gorm:"default:false"`
	MaLength     int       `json:"ma_length" gorm:"default:200"`
	MaType       string    `json:"ma_type" gorm:"default:EMA"`
	TslPercent   float64   `json:"tsl_percent" gorm:"default:20.0"`
	TslEnabled   bool      `json:"tsl_enabled" gorm:"default:true"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TraderStockPerformance stores performance data for Trader trading mode
type TraderStockPerformance struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	Symbol          string    `json:"symbol" gorm:"uniqueIndex;not null"`
	Name            string    `json:"name"`
	WinRate         float64   `json:"win_rate"`
	RiskReward      float64   `json:"risk_reward"`
	TotalReturn     float64   `json:"total_return"`
	AvgReturn       float64   `json:"avg_return"`
	TotalTrades     int       `json:"total_trades"`
	Wins            int       `json:"wins"`
	Losses          int       `json:"losses"`
	Signal          string    `json:"signal"`
	SignalBars      int       `json:"signal_bars"`
	SignalSince     string    `json:"signal_since"`
	PrevSignal      string    `json:"prev_signal"`
	PrevSignalSince string    `json:"prev_signal_since"`
	TradesJSON      string    `json:"trades_json" gorm:"type:text"`
	CurrentPrice    float64   `json:"current_price"`
	MarketCap       int64     `json:"market_cap" gorm:"default:0"`
	ISIN            string    `json:"isin"`
	UpdatedAt       time.Time `json:"updated_at"`
	CreatedAt       time.Time `json:"created_at"`
}

// TraderTrade tracks all trades made by the Trader bot
type TraderTrade struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Symbol        string     `json:"symbol" gorm:"index;not null"`
	Name          string     `json:"name"`
	Action        string     `json:"action" gorm:"not null"`
	Quantity      float64    `json:"quantity" gorm:"default:1"`
	IsLive        bool       `json:"is_live" gorm:"default:false"`
	IsPending     bool       `json:"is_pending" gorm:"default:false"`
	IsDeleted     bool       `json:"is_deleted" gorm:"default:false"`
	IsRead        bool       `json:"is_read" gorm:"default:false"`
	IsAdminClosed bool       `json:"is_admin_closed" gorm:"default:false"`
	Price         float64    `json:"price" gorm:"not null"`
	SignalDate    time.Time  `json:"signal_date" gorm:"not null"`
	ExecutedAt    time.Time  `json:"executed_at" gorm:"not null"`
	ProfitLoss    *float64   `json:"profit_loss"`
	ProfitLossPct *float64   `json:"profit_loss_pct"`
	IsStopLoss        bool   `json:"is_stop_loss" gorm:"default:false"`
	IsFilterBlocked   bool   `json:"is_filter_blocked" gorm:"default:false"`
	FilterBlockReason string `json:"filter_block_reason" gorm:"type:text"`
	CreatedAt         time.Time `json:"created_at"`
}

// TraderPosition tracks current open positions of the Trader bot
type TraderPosition struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Symbol        string     `json:"symbol" gorm:"index;not null"`
	Name          string     `json:"name"`
	Quantity      float64    `json:"quantity" gorm:"default:1"`
	AvgPrice      float64    `json:"avg_price" gorm:"not null"`
	InvestedEUR   float64    `json:"invested_eur" gorm:"default:0"`
	IsLive        bool       `json:"is_live" gorm:"default:false"`
	IsPending     bool       `json:"is_pending" gorm:"default:false"`
	IsClosed      bool       `json:"is_closed" gorm:"default:false"`
	SellPrice     float64    `json:"sell_price" gorm:"default:0"`
	SellDate      *time.Time `json:"sell_date"`
	ProfitLoss    *float64   `json:"profit_loss"`
	ProfitLossPct *float64   `json:"profit_loss_pct"`
	IsAdminClosed   bool       `json:"is_admin_closed" gorm:"default:false"`
	BuyDate         time.Time  `json:"buy_date" gorm:"not null"`
	StopLossPercent *float64   `json:"stop_loss_percent" gorm:"default:null"`
	StopLossType    string     `json:"stop_loss_type" gorm:"default:trailing"`
	HighestPrice    float64    `json:"highest_price" gorm:"default:0"`
	StopLossPrice   float64    `json:"stop_loss_price" gorm:"default:0"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// SystemSetting stores system-wide settings and state
type SystemSetting struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Key       string    `json:"key" gorm:"uniqueIndex;not null"`
	Value     string    `json:"value" gorm:"type:text"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LastFullUpdate tracks the last full stock update
type LastFullUpdate struct {
	UpdatedAt   time.Time `json:"updated_at"`
	TriggeredBy string    `json:"triggered_by"` // username or "system"
	StocksCount int       `json:"stocks_count"`
	Success     int       `json:"success"`
	Failed      int       `json:"failed"`
}

// BotStockAllowlist controls which stocks each bot is allowed to trade
type BotStockAllowlist struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	BotName string `gorm:"index;not null" json:"bot_name"`
	Symbol  string `gorm:"index;not null" json:"symbol"`
	Allowed bool   `json:"allowed"`
}

// BotFilterConfig stores per-bot performance filter settings for BUY trade validation
type BotFilterConfig struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	BotName      string    `gorm:"uniqueIndex;not null" json:"bot_name"` // flipper, lutz, quant, ditz, trader
	MinWinrate   *float64  `json:"min_winrate"`
	MaxWinrate   *float64  `json:"max_winrate"`
	MinRR        *float64  `json:"min_rr"`
	MaxRR        *float64  `json:"max_rr"`
	MinAvgReturn *float64  `json:"min_avg_return"`
	MaxAvgReturn *float64  `json:"max_avg_return"`
	MinMarketCap *float64  `json:"min_market_cap"` // in Mrd (billions)
	Enabled      bool      `json:"enabled" gorm:"default:false"`
	UpdatedAt    time.Time `json:"updated_at"`
}

var db *gorm.DB
var latestPriceCache sync.Map // key: symbol (string), value: float64
var sessions = make(map[string]Session) // Legacy in-memory cache, DB is source of truth
var httpClient = &http.Client{Timeout: 10 * time.Second}
var twelveDataAPIKey string

// Yahoo Finance crumb-based auth client
var (
	yahooCrumb      string
	yahooCrumbMu    sync.Mutex
	yahooAuthClient *http.Client
)

func getYahooCrumbClient() (*http.Client, string, error) {
	yahooCrumbMu.Lock()
	defer yahooCrumbMu.Unlock()

	if yahooCrumb != "" && yahooAuthClient != nil {
		return yahooAuthClient, yahooCrumb, nil
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 10 * time.Second, Jar: jar}

	// Step 1: Get cookies
	req, _ := http.NewRequest("GET", "https://fc.yahoo.com/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("cookie fetch failed: %v", err)
	}
	resp.Body.Close()

	// Step 2: Get crumb
	req2, _ := http.NewRequest("GET", "https://query2.finance.yahoo.com/v1/test/getcrumb", nil)
	req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	resp2, err := client.Do(req2)
	if err != nil {
		return nil, "", fmt.Errorf("crumb fetch failed: %v", err)
	}
	defer resp2.Body.Close()

	body, _ := io.ReadAll(resp2.Body)
	crumb := strings.TrimSpace(string(body))
	if crumb == "" || resp2.StatusCode != 200 {
		return nil, "", fmt.Errorf("empty crumb, status: %d", resp2.StatusCode)
	}

	yahooAuthClient = client
	yahooCrumb = crumb
	fmt.Printf("[Yahoo] Got crumb: %s\n", crumb)
	return client, crumb, nil
}

func resetYahooCrumb() {
	yahooCrumbMu.Lock()
	defer yahooCrumbMu.Unlock()
	yahooCrumb = ""
	yahooAuthClient = nil
}

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

	twelveDataAPIKey = os.Getenv("TWELVE_DATA_API_KEY")
	if twelveDataAPIKey != "" {
		fmt.Println("[Config] Twelve Data API key configured - will use as fallback for monthly data")
	}

	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to database: " + err.Error())
	}

	// Drop unique indexes before AutoMigrate (changed to non-unique index)
	db.Exec("DROP INDEX IF EXISTS idx_flipper_bot_positions_symbol")
	db.Exec("DROP INDEX IF EXISTS idx_lutz_positions_symbol")

	db.AutoMigrate(&User{}, &Stock{}, &Category{}, &PortfolioPosition{}, &PortfolioTradeHistory{}, &StockPerformance{}, &ActivityLog{}, &FlipperBotTrade{}, &FlipperBotPosition{}, &AggressiveStockPerformance{}, &LutzTrade{}, &LutzPosition{}, &DBSession{}, &BotLog{}, &BotTodo{}, &BXtrenderConfig{}, &BXtrenderQuantConfig{}, &QuantStockPerformance{}, &QuantTrade{}, &QuantPosition{}, &BXtrenderDitzConfig{}, &DitzStockPerformance{}, &DitzTrade{}, &DitzPosition{}, &BXtrenderTraderConfig{}, &TraderStockPerformance{}, &TraderTrade{}, &TraderPosition{}, &SystemSetting{}, &BotStockAllowlist{}, &BotFilterConfig{})

	// Ensure "Sonstiges" category exists
	ensureSonstigesCategory()

	// Ensure is_live columns exist (SQLite doesn't always add new columns)
	db.Exec("ALTER TABLE flipper_bot_trades ADD COLUMN is_live BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE flipper_bot_positions ADD COLUMN is_live BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE lutz_trades ADD COLUMN is_live BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE lutz_positions ADD COLUMN is_live BOOLEAN DEFAULT 0")

	// Ensure is_pending columns exist (for admin approval workflow)
	db.Exec("ALTER TABLE flipper_bot_trades ADD COLUMN is_pending BOOLEAN DEFAULT 1")
	db.Exec("ALTER TABLE flipper_bot_positions ADD COLUMN is_pending BOOLEAN DEFAULT 1")
	db.Exec("ALTER TABLE lutz_trades ADD COLUMN is_pending BOOLEAN DEFAULT 1")
	db.Exec("ALTER TABLE lutz_positions ADD COLUMN is_pending BOOLEAN DEFAULT 1")
	// Set existing trades/positions to approved (not pending) so they remain visible
	db.Exec("UPDATE flipper_bot_trades SET is_pending = 0 WHERE is_pending IS NULL")
	db.Exec("UPDATE flipper_bot_positions SET is_pending = 0 WHERE is_pending IS NULL")
	db.Exec("UPDATE lutz_trades SET is_pending = 0 WHERE is_pending IS NULL")
	db.Exec("UPDATE lutz_positions SET is_pending = 0 WHERE is_pending IS NULL")

	// Ensure new columns exist for FlipperBot/Lutz automation
	db.Exec("ALTER TABLE flipper_bot_trades ADD COLUMN is_deleted BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE flipper_bot_trades ADD COLUMN is_read BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE flipper_bot_positions ADD COLUMN is_closed BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE flipper_bot_positions ADD COLUMN sell_price REAL DEFAULT 0")
	db.Exec("ALTER TABLE flipper_bot_positions ADD COLUMN sell_date DATETIME")
	db.Exec("ALTER TABLE flipper_bot_positions ADD COLUMN profit_loss REAL")
	db.Exec("ALTER TABLE flipper_bot_positions ADD COLUMN profit_loss_pct REAL")
	db.Exec("ALTER TABLE lutz_trades ADD COLUMN is_deleted BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE lutz_trades ADD COLUMN is_read BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE lutz_positions ADD COLUMN is_closed BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE lutz_positions ADD COLUMN sell_price REAL DEFAULT 0")
	db.Exec("ALTER TABLE lutz_positions ADD COLUMN sell_date DATETIME")
	db.Exec("ALTER TABLE lutz_positions ADD COLUMN profit_loss REAL")
	db.Exec("ALTER TABLE lutz_positions ADD COLUMN profit_loss_pct REAL")

	// Ensure is_admin_closed columns exist
	db.Exec("ALTER TABLE flipper_bot_trades ADD COLUMN is_admin_closed BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE flipper_bot_positions ADD COLUMN is_admin_closed BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE lutz_trades ADD COLUMN is_admin_closed BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE lutz_positions ADD COLUMN is_admin_closed BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE quant_trades ADD COLUMN is_admin_closed BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE quant_positions ADD COLUMN is_admin_closed BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE ditz_trades ADD COLUMN is_admin_closed BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE ditz_positions ADD COLUMN is_admin_closed BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE trader_trades ADD COLUMN is_admin_closed BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE trader_positions ADD COLUMN is_admin_closed BOOLEAN DEFAULT 0")

	// Ensure is_filter_blocked and filter_block_reason columns exist
	for _, table := range []string{"flipper_bot_trades", "lutz_trades", "quant_trades", "ditz_trades", "trader_trades"} {
		db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN is_filter_blocked BOOLEAN DEFAULT 0", table))
		db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN filter_block_reason TEXT DEFAULT ''", table))
	}

	// Clean up expired sessions on startup
	db.Where("expiry < ?", time.Now()).Delete(&DBSession{})

	// Ensure bot users exist for portfolio comparison
	ensureFlipperBotUser()
	ensureLutzUser()
	ensureQuantUser()
	ensureDitzUser()
	ensureTraderUser()

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
		api.GET("/isin/:symbol", getISIN)
		api.GET("/test-marketcap/:symbol", testMarketCap)
		api.POST("/update-marketcaps", updateMarketCaps)
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

		// Quant mode performance routes
		api.POST("/performance/quant", saveQuantStockPerformance)
		api.GET("/performance/quant", getQuantTrackedStocks)
		api.GET("/performance/quant/:symbol", getQuantStockPerformance)

		// Quant mode config routes
		api.GET("/bxtrender-quant-config", getBXtrenderQuantConfigPublic)
		api.GET("/admin/bxtrender-quant-config", authMiddleware(), adminOnly(), getBXtrenderQuantConfig)
		api.PUT("/admin/bxtrender-quant-config", authMiddleware(), adminOnly(), updateBXtrenderQuantConfig)

		// Ditz mode performance routes
		api.POST("/performance/ditz", saveDitzStockPerformance)
		api.GET("/performance/ditz", getDitzTrackedStocks)
		api.GET("/performance/ditz/:symbol", getDitzStockPerformance)

		// Ditz mode config routes
		api.GET("/bxtrender-ditz-config", getBXtrenderDitzConfigPublic)
		api.GET("/admin/bxtrender-ditz-config", authMiddleware(), adminOnly(), getBXtrenderDitzConfig)
		api.PUT("/admin/bxtrender-ditz-config", authMiddleware(), adminOnly(), updateBXtrenderDitzConfig)

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
		api.GET("/admin/bxtrender-config", authMiddleware(), adminOnly(), getBXtrenderConfig)
		api.PUT("/admin/bxtrender-config", authMiddleware(), adminOnly(), updateBXtrenderConfig)
		api.GET("/admin/last-full-update", authMiddleware(), adminOnly(), getLastFullUpdate)
		api.POST("/admin/record-full-update", authMiddleware(), adminOnly(), recordFullUpdate)
		api.POST("/admin/run-full-update", authMiddleware(), adminOnly(), runFullUpdateHandler)
		api.GET("/admin/scheduler-time", authMiddleware(), adminOnly(), getSchedulerTimeHandler)
		api.PUT("/admin/scheduler-time", authMiddleware(), adminOnly(), setSchedulerTimeHandler)
		api.GET("/admin/bot-allowlist", authMiddleware(), adminOnly(), getBotAllowlist)
		api.PUT("/admin/bot-allowlist", authMiddleware(), adminOnly(), updateBotAllowlist)
		api.GET("/admin/bot-filter-config", authMiddleware(), adminOnly(), getBotFilterConfig)
		api.PUT("/admin/bot-filter-config", authMiddleware(), adminOnly(), updateBotFilterConfig)
		api.GET("/admin/export-watchlist", authMiddleware(), adminOnly(), exportWatchlist)
		api.POST("/admin/import-watchlist", authMiddleware(), adminOnly(), importWatchlist)

		// Public endpoint for fetching BXtrender config (no auth required for frontend calculation)
		api.GET("/bxtrender-config", getBXtrenderConfigPublic)

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
		api.GET("/flipperbot/history", authMiddleware(), getFlipperBotHistory)
		api.POST("/flipperbot/backfill", authMiddleware(), adminOnly(), flipperBotBackfill)
		api.GET("/flipperbot/pending-trades", authMiddleware(), adminOnly(), getFlipperBotPendingTrades)
		api.POST("/flipperbot/trade/:id/accept", authMiddleware(), adminOnly(), acceptFlipperBotTrade)
		api.GET("/flipperbot/actions-all", authMiddleware(), adminOnly(), getFlipperBotActionsAll)
		api.GET("/flipperbot/simulated-portfolio", authMiddleware(), adminOnly(), getFlipperBotSimulatedPortfolio)
		api.GET("/flipperbot/simulated-performance", authMiddleware(), adminOnly(), getFlipperBotSimulatedPerformance)
		api.PUT("/flipperbot/trade/:id/read", authMiddleware(), adminOnly(), toggleFlipperTradeRead)
		api.PUT("/flipperbot/trades/read-all", authMiddleware(), adminOnly(), markAllFlipperTradesRead)
		api.PUT("/flipperbot/trades/unread-all", authMiddleware(), adminOnly(), markAllFlipperTradesUnread)
		api.GET("/flipperbot/trades/unread-count", authMiddleware(), adminOnly(), getFlipperUnreadCount)
		api.POST("/flipperbot/cleanup-pending", authMiddleware(), adminOnly(), cleanupFlipperPending)
		api.GET("/flipperbot/last-refresh", authMiddleware(), adminOnly(), getLastFlipperRefresh)

		// Lutz routes - Aggressive mode bot (view: all users, actions: admin only)
		api.GET("/lutz/update", authMiddleware(), adminOnly(), lutzUpdate)
		api.GET("/lutz/portfolio", authMiddleware(), getLutzPortfolio)
		api.GET("/lutz/actions", authMiddleware(), getLutzActions)
		api.GET("/lutz/performance", authMiddleware(), getLutzPerformance)
		api.POST("/lutz/reset", authMiddleware(), adminOnly(), resetLutz)
		api.POST("/lutz/backfill", authMiddleware(), adminOnly(), lutzBackfill)
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
		api.GET("/lutz/history", authMiddleware(), getLutzHistory)
		api.GET("/lutz/pending-trades", authMiddleware(), adminOnly(), getLutzPendingTrades)
		api.POST("/lutz/trade/:id/accept", authMiddleware(), adminOnly(), acceptLutzTrade)
		api.GET("/lutz/actions-all", authMiddleware(), adminOnly(), getLutzActionsAll)
		api.GET("/lutz/simulated-portfolio", authMiddleware(), adminOnly(), getLutzSimulatedPortfolio)
		api.GET("/lutz/simulated-performance", authMiddleware(), adminOnly(), getLutzSimulatedPerformance)
		api.PUT("/lutz/trade/:id/read", authMiddleware(), adminOnly(), toggleLutzTradeRead)
		api.PUT("/lutz/trades/read-all", authMiddleware(), adminOnly(), markAllLutzTradesRead)
		api.PUT("/lutz/trades/unread-all", authMiddleware(), adminOnly(), markAllLutzTradesUnread)
		api.GET("/lutz/trades/unread-count", authMiddleware(), adminOnly(), getLutzUnreadCount)
		api.POST("/lutz/cleanup-pending", authMiddleware(), adminOnly(), cleanupLutzPending)
		api.GET("/lutz/last-refresh", authMiddleware(), adminOnly(), getLastLutzRefresh)

		// Quant routes - Quant mode bot (view: all users, actions: admin only)
		api.GET("/quant/update", authMiddleware(), adminOnly(), quantUpdate)
		api.GET("/quant/portfolio", authMiddleware(), getQuantPortfolio)
		api.GET("/quant/actions", authMiddleware(), getQuantActions)
		api.GET("/quant/performance", authMiddleware(), getQuantPerformance)
		api.POST("/quant/reset", authMiddleware(), adminOnly(), resetQuant)
		api.POST("/quant/cleanup-pending", authMiddleware(), adminOnly(), cleanupQuantPending)
		api.GET("/quant/last-refresh", authMiddleware(), adminOnly(), getLastQuantRefresh)
		api.POST("/quant/backfill", authMiddleware(), adminOnly(), quantBackfill)
		api.PUT("/quant/position/:id", authMiddleware(), adminOnly(), updateQuantPosition)
		api.PUT("/quant/trade/:id", authMiddleware(), adminOnly(), updateQuantTrade)
		api.DELETE("/quant/trade/:id", authMiddleware(), adminOnly(), deleteQuantTrade)
		api.PUT("/quant/trade/:id/read", authMiddleware(), adminOnly(), toggleQuantTradeRead)
		api.PUT("/quant/trades/read-all", authMiddleware(), adminOnly(), markAllQuantTradesRead)
		api.PUT("/quant/trades/unread-all", authMiddleware(), adminOnly(), markAllQuantTradesUnread)
		api.GET("/quant/trades/unread-count", authMiddleware(), adminOnly(), getQuantUnreadCount)
		api.GET("/quant/pending", authMiddleware(), adminOnly(), getQuantPending)
		api.GET("/quant/logs", authMiddleware(), getQuantLogs)
		api.GET("/quant/todos", authMiddleware(), getQuantTodos)
		api.PUT("/quant/todos/:id/done", authMiddleware(), adminOnly(), markQuantTodoDone)
		api.PUT("/quant/todos/:id/reopen", authMiddleware(), adminOnly(), reopenQuantTodo)
		api.DELETE("/quant/todos/:id", authMiddleware(), adminOnly(), deleteQuantTodo)
		api.POST("/quant/todos/:id/execute", authMiddleware(), adminOnly(), executeQuantTodo)
		api.POST("/quant/sync", authMiddleware(), adminOnly(), syncQuant)
		api.GET("/quant/actions-all", authMiddleware(), adminOnly(), getQuantActionsAll)
		api.GET("/quant/completed-trades", authMiddleware(), getQuantCompletedTrades)
		api.GET("/quant/history", authMiddleware(), getQuantHistory)
		api.GET("/quant/pending-trades", authMiddleware(), adminOnly(), getQuantPendingTrades)
		api.POST("/quant/trade/:id/accept", authMiddleware(), adminOnly(), acceptQuantTrade)
		api.GET("/quant/simulated-portfolio", authMiddleware(), adminOnly(), getQuantSimulatedPortfolio)
		api.GET("/quant/simulated-performance", authMiddleware(), adminOnly(), getQuantSimulatedPerformance)
		api.POST("/quant/manual-trade", authMiddleware(), adminOnly(), createManualQuantTrade)

		// Ditz routes - Ditz mode bot (admin only)
		api.GET("/ditz/update", authMiddleware(), adminOnly(), ditzUpdate)
		api.GET("/ditz/portfolio", authMiddleware(), getDitzPortfolio)
		api.GET("/ditz/actions", authMiddleware(), getDitzActions)
		api.GET("/ditz/performance", authMiddleware(), getDitzPerformance)
		api.POST("/ditz/reset", authMiddleware(), adminOnly(), resetDitz)
		api.POST("/ditz/cleanup-pending", authMiddleware(), adminOnly(), cleanupDitzPending)
		api.GET("/ditz/last-refresh", authMiddleware(), adminOnly(), getLastDitzRefresh)
		api.POST("/ditz/backfill", authMiddleware(), adminOnly(), ditzBackfill)
		api.PUT("/ditz/position/:id", authMiddleware(), adminOnly(), updateDitzPosition)
		api.PUT("/ditz/trade/:id", authMiddleware(), adminOnly(), updateDitzTrade)
		api.DELETE("/ditz/trade/:id", authMiddleware(), adminOnly(), deleteDitzTrade)
		api.PUT("/ditz/trade/:id/read", authMiddleware(), adminOnly(), toggleDitzTradeRead)
		api.PUT("/ditz/trades/read-all", authMiddleware(), adminOnly(), markAllDitzTradesRead)
		api.PUT("/ditz/trades/unread-all", authMiddleware(), adminOnly(), markAllDitzTradesUnread)
		api.GET("/ditz/trades/unread-count", authMiddleware(), adminOnly(), getDitzUnreadCount)
		api.GET("/ditz/pending", authMiddleware(), adminOnly(), getDitzPending)
		api.GET("/ditz/logs", authMiddleware(), getDitzLogs)
		api.GET("/ditz/todos", authMiddleware(), getDitzTodos)
		api.PUT("/ditz/todos/:id/done", authMiddleware(), adminOnly(), markDitzTodoDone)
		api.PUT("/ditz/todos/:id/reopen", authMiddleware(), adminOnly(), reopenDitzTodo)
		api.DELETE("/ditz/todos/:id", authMiddleware(), adminOnly(), deleteDitzTodo)
		api.POST("/ditz/todos/:id/execute", authMiddleware(), adminOnly(), executeDitzTodo)
		api.POST("/ditz/sync", authMiddleware(), adminOnly(), syncDitz)
		api.GET("/ditz/actions-all", authMiddleware(), adminOnly(), getDitzActionsAll)
		api.GET("/ditz/completed-trades", authMiddleware(), getDitzCompletedTrades)
		api.GET("/ditz/history", authMiddleware(), getDitzHistory)
		api.GET("/ditz/pending-trades", authMiddleware(), adminOnly(), getDitzPendingTrades)
		api.POST("/ditz/trade/:id/accept", authMiddleware(), adminOnly(), acceptDitzTrade)
		api.GET("/ditz/simulated-portfolio", authMiddleware(), adminOnly(), getDitzSimulatedPortfolio)
		api.GET("/ditz/simulated-performance", authMiddleware(), adminOnly(), getDitzSimulatedPerformance)
		api.POST("/ditz/manual-trade", authMiddleware(), adminOnly(), createManualDitzTrade)

		// Trader mode performance routes
		api.POST("/performance/trader", saveTraderStockPerformance)
		api.GET("/performance/trader", getTraderTrackedStocks)
		api.GET("/performance/trader/:symbol", getTraderStockPerformance)

		// Trader mode config routes
		api.GET("/bxtrender-trader-config", getBXtrenderTraderConfigPublic)
		api.GET("/admin/bxtrender-trader-config", authMiddleware(), adminOnly(), getBXtrenderTraderConfig)
		api.PUT("/admin/bxtrender-trader-config", authMiddleware(), adminOnly(), updateBXtrenderTraderConfig)

		// Trader routes - Trader mode bot (admin only)
		api.GET("/trader/update", authMiddleware(), adminOnly(), traderUpdate)
		api.GET("/trader/portfolio", authMiddleware(), getTraderPortfolio)
		api.GET("/trader/actions", authMiddleware(), getTraderActions)
		api.GET("/trader/performance", authMiddleware(), getTraderPerformance)
		api.POST("/trader/reset", authMiddleware(), adminOnly(), resetTrader)
		api.POST("/trader/cleanup-pending", authMiddleware(), adminOnly(), cleanupTraderPending)
		api.GET("/trader/last-refresh", authMiddleware(), adminOnly(), getLastTraderRefresh)
		api.POST("/trader/backfill", authMiddleware(), adminOnly(), traderBackfill)
		api.PUT("/trader/position/:id", authMiddleware(), adminOnly(), updateTraderPosition)
		api.PUT("/trader/trade/:id", authMiddleware(), adminOnly(), updateTraderTrade)
		api.DELETE("/trader/trade/:id", authMiddleware(), adminOnly(), deleteTraderTrade)
		api.PUT("/trader/trade/:id/read", authMiddleware(), adminOnly(), toggleTraderTradeRead)
		api.PUT("/trader/trades/read-all", authMiddleware(), adminOnly(), markAllTraderTradesRead)
		api.PUT("/trader/trades/unread-all", authMiddleware(), adminOnly(), markAllTraderTradesUnread)
		api.GET("/trader/trades/unread-count", authMiddleware(), adminOnly(), getTraderUnreadCount)
		api.GET("/trader/pending", authMiddleware(), adminOnly(), getTraderPending)
		api.GET("/trader/logs", authMiddleware(), getTraderLogs)
		api.GET("/trader/todos", authMiddleware(), getTraderTodos)
		api.PUT("/trader/todos/:id/done", authMiddleware(), adminOnly(), markTraderTodoDone)
		api.PUT("/trader/todos/:id/reopen", authMiddleware(), adminOnly(), reopenTraderTodo)
		api.DELETE("/trader/todos/:id", authMiddleware(), adminOnly(), deleteTraderTodo)
		api.POST("/trader/todos/:id/execute", authMiddleware(), adminOnly(), executeTraderTodo)
		api.POST("/trader/sync", authMiddleware(), adminOnly(), syncTrader)
		api.GET("/trader/actions-all", authMiddleware(), adminOnly(), getTraderActionsAll)
		api.GET("/trader/completed-trades", authMiddleware(), getTraderCompletedTrades)
		api.GET("/trader/history", authMiddleware(), getTraderHistory)
		api.GET("/trader/pending-trades", authMiddleware(), adminOnly(), getTraderPendingTrades)
		api.POST("/trader/trade/:id/accept", authMiddleware(), adminOnly(), acceptTraderTrade)
		api.GET("/trader/simulated-portfolio", authMiddleware(), adminOnly(), getTraderSimulatedPortfolio)
		api.GET("/trader/simulated-performance", authMiddleware(), adminOnly(), getTraderSimulatedPerformance)
		api.POST("/trader/manual-trade", authMiddleware(), adminOnly(), createManualTraderTrade)

		// Performance page - combined view of both bots
		api.GET("/performance/history", authMiddleware(), getPerformanceHistory)
	}

	// Start the daily stock update scheduler
	go startDailyUpdateScheduler()

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

			// Update MarketCap in database if changed
			if q.MarketCap > 0 && q.MarketCap != stock.MarketCap {
				db.Model(&Stock{}).Where("id = ?", stock.ID).Update("market_cap", q.MarketCap)
			}
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

	// Also extract Market Cap from the nested spark structure (same response body)
	var fullResp struct {
		Spark struct {
			Result []struct {
				Symbol   string `json:"symbol"`
				Response []struct {
					Meta struct {
						MarketCap float64 `json:"marketCap"`
					} `json:"meta"`
				} `json:"response"`
			} `json:"result"`
		} `json:"spark"`
	}
	if err := json.Unmarshal(body, &fullResp); err == nil {
		for _, r := range fullResp.Spark.Result {
			if len(r.Response) > 0 && r.Response[0].Meta.MarketCap > 0 {
				sym := r.Symbol
				if q, ok := result[sym]; ok {
					q.MarketCap = int64(r.Response[0].Meta.MarketCap)
					result[sym] = q
				}
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
			"sector":         q.Sector,
			"market_cap":     q.MarketCap,
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
			Meta struct {
				DataGranularity string `json:"dataGranularity"`
				Range           string `json:"range"`
			} `json:"meta"`
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

	// Normalize monthly timestamps to 1st of month 00:00 UTC
	if interval == "1mo" {
		for i := range data {
			t := time.Unix(data[i].Time, 0).UTC()
			normalized := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
			data[i].Time = normalized.Unix()
		}
	}

	// Return actual data granularity from Yahoo Finance
	actualGranularity := interval
	if len(yahooResp.Chart.Result) > 0 && yahooResp.Chart.Result[0].Meta.DataGranularity != "" {
		actualGranularity = yahooResp.Chart.Result[0].Meta.DataGranularity
	}

	// Track data source
	dataSource := "yahoo"
	warnings := []string{}

	// Fallback: If monthly was requested but Yahoo returned 3mo, try Twelve Data then aggregate
	if interval == "1mo" && actualGranularity != "1mo" {
		fmt.Printf("[History] %s: Monthly not available from Yahoo (got %s)\n", symbol, actualGranularity)

		// Fallback 1: Twelve Data API
		if twelveDataAPIKey != "" {
			tdData, err := fetchMonthlyFromTwelveData(symbol)
			if err == nil && len(tdData) > 0 {
				fmt.Printf("[History] %s: Got %d monthly bars from Twelve Data\n", symbol, len(tdData))
				data = tdData
				actualGranularity = "1mo"
				dataSource = "twelvedata"
			} else {
				fmt.Printf("[History] %s: Twelve Data fallback failed: %v\n", symbol, err)
				if err != nil && strings.Contains(err.Error(), "TWELVE_DATA_RATE_LIMIT") {
					warnings = append(warnings, "Twelve Data API-Limit erreicht (800 Anfragen/Tag in der Testphase). Daten werden über Yahoo Finance aggregiert.")
				}
			}
		}

		// Fallback 2: Aggregate from daily/weekly (if Twelve Data didn't work)
		if actualGranularity != "1mo" {
			fallbackData, err := fetchWeeklyAndAggregateToMonthly(symbol)
			if err == nil && len(fallbackData) > 0 {
				data = fallbackData
				actualGranularity = "1mo"
				dataSource = "yahoo-aggregated"
			} else {
				fmt.Printf("[History] %s: Aggregation fallback also failed: %v\n", symbol, err)
			}
		}
	}

	respData := gin.H{
		"symbol":            symbol,
		"data":              data,
		"requestedInterval": interval,
		"actualInterval":    actualGranularity,
		"source":            dataSource,
	}
	if len(warnings) > 0 {
		respData["warnings"] = warnings
	}
	c.JSON(http.StatusOK, respData)
}

// fetchWeeklyAndAggregateToMonthly tries daily data first (accurate month-end closes),
// then falls back to weekly if daily is not available.
// Daily bars never cross month boundaries, unlike weekly bars where a bar starting Dec 29
// has its close from Jan 3, contaminating the monthly close prices.
func fetchWeeklyAndAggregateToMonthly(symbol string) ([]OHLCV, error) {
	// Try daily data first for accurate month-end close prices
	data, err := fetchIntervalAndAggregateToMonthly(symbol, "1d")
	if err == nil && len(data) > 35 {
		fmt.Printf("[History] %s: Aggregated %d monthly bars from daily data\n", symbol, len(data))
		return data, nil
	}
	if err != nil {
		fmt.Printf("[History] %s: Daily aggregation failed: %v, trying weekly\n", symbol, err)
	} else {
		fmt.Printf("[History] %s: Daily aggregation too few bars (%d), trying weekly\n", symbol, len(data))
	}

	// Fall back to weekly data
	data, err = fetchIntervalAndAggregateToMonthly(symbol, "1wk")
	if err != nil {
		return nil, err
	}
	fmt.Printf("[History] %s: Aggregated %d monthly bars from weekly data\n", symbol, len(data))
	return data, nil
}

// fetchIntervalAndAggregateToMonthly fetches data at the given interval and aggregates to monthly OHLCV bars
func fetchIntervalAndAggregateToMonthly(symbol, interval string) ([]OHLCV, error) {
	apiURL := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=max&interval=%s",
		url.QueryEscape(symbol), interval)

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var yahooResp YahooChartResponse
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return nil, err
	}

	if len(yahooResp.Chart.Result) == 0 || len(yahooResp.Chart.Result[0].Timestamp) == 0 {
		return nil, fmt.Errorf("no %s data found", interval)
	}

	result := yahooResp.Chart.Result[0]

	// Verify we got the requested granularity (reject if Yahoo returned a coarser interval)
	if result.Meta.DataGranularity != "" && result.Meta.DataGranularity != interval {
		return nil, fmt.Errorf("requested %s but got %s", interval, result.Meta.DataGranularity)
	}

	quotes := result.Indicators.Quote[0]

	// Group bars by year-month and aggregate
	type monthKey struct {
		Year  int
		Month int
	}
	type monthBar struct {
		Open   float64
		High   float64
		Low    float64
		Close  float64
		Volume float64
		Time   int64
		IsSet  bool
	}

	monthMap := make(map[monthKey]*monthBar)
	var monthOrder []monthKey

	for i, ts := range result.Timestamp {
		if i >= len(quotes.Open) || i >= len(quotes.Close) || quotes.Close[i] <= 0 {
			continue
		}

		t := time.Unix(ts, 0).UTC()
		key := monthKey{Year: t.Year(), Month: int(t.Month())}

		bar, exists := monthMap[key]
		if !exists {
			bar = &monthBar{
				Open:  quotes.Open[i],
				High:  quotes.High[i],
				Low:   quotes.Low[i],
				Time:  ts,
				IsSet: true,
			}
			monthMap[key] = bar
			monthOrder = append(monthOrder, key)
		} else {
			if quotes.High[i] > bar.High {
				bar.High = quotes.High[i]
			}
			if quotes.Low[i] < bar.Low || bar.Low == 0 {
				bar.Low = quotes.Low[i]
			}
		}
		bar.Close = quotes.Close[i]
		bar.Volume += quotes.Volume[i]
	}

	// Convert to OHLCV slice in chronological order
	data := make([]OHLCV, 0, len(monthOrder))
	for _, key := range monthOrder {
		bar := monthMap[key]
		if bar.IsSet {
			// Normalize timestamp to 1st of month 00:00 UTC
			normalized := time.Date(key.Year, time.Month(key.Month), 1, 0, 0, 0, 0, time.UTC)
			data = append(data, OHLCV{
				Time:   normalized.Unix(),
				Open:   bar.Open,
				High:   bar.High,
				Low:    bar.Low,
				Close:  bar.Close,
				Volume: bar.Volume,
			})
		}
	}

	return data, nil
}

// fetchMonthlyFromTwelveData fetches monthly OHLCV data from Twelve Data API
func fetchMonthlyFromTwelveData(symbol string) ([]OHLCV, error) {
	if twelveDataAPIKey == "" {
		return nil, fmt.Errorf("no Twelve Data API key configured")
	}

	apiURL := fmt.Sprintf("https://api.twelvedata.com/time_series?symbol=%s&interval=1month&outputsize=5000&apikey=%s",
		url.QueryEscape(symbol), twelveDataAPIKey)

	req, _ := http.NewRequest("GET", apiURL, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twelve data request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var tdResp struct {
		Status  string `json:"status"`
		Code    int    `json:"code"`
		Message string `json:"message"`
		Values  []struct {
			Datetime string `json:"datetime"`
			Open     string `json:"open"`
			High     string `json:"high"`
			Low      string `json:"low"`
			Close    string `json:"close"`
			Volume   string `json:"volume"`
		} `json:"values"`
	}

	if err := json.Unmarshal(body, &tdResp); err != nil {
		return nil, fmt.Errorf("twelve data parse error: %v", err)
	}

	if tdResp.Status == "error" {
		if tdResp.Code == 429 || strings.Contains(strings.ToLower(tdResp.Message), "api calls") ||
			strings.Contains(strings.ToLower(tdResp.Message), "rate limit") {
			return nil, fmt.Errorf("TWELVE_DATA_RATE_LIMIT: %s", tdResp.Message)
		}
		return nil, fmt.Errorf("twelve data API error (code %d): %s", tdResp.Code, tdResp.Message)
	}

	if len(tdResp.Values) == 0 {
		return nil, fmt.Errorf("no monthly data from Twelve Data")
	}

	data := make([]OHLCV, 0, len(tdResp.Values))
	for _, v := range tdResp.Values {
		t, err := time.Parse("2006-01-02", v.Datetime)
		if err != nil {
			continue
		}
		open, _ := strconv.ParseFloat(v.Open, 64)
		high, _ := strconv.ParseFloat(v.High, 64)
		low, _ := strconv.ParseFloat(v.Low, 64)
		cl, _ := strconv.ParseFloat(v.Close, 64)
		vol, _ := strconv.ParseFloat(v.Volume, 64)

		if cl <= 0 {
			continue
		}

		// Normalize timestamp to 1st of month 00:00 UTC
		normalized := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		data = append(data, OHLCV{
			Time:   normalized.Unix(),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  cl,
			Volume: vol,
		})
	}

	// Twelve Data returns newest first, reverse to chronological order
	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}

	return data, nil
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
			} else if user.ID == QUANT_USER_ID {
				var botPos QuantPosition
				if db.Where("symbol = ?", pos.Symbol).First(&botPos).Error == nil {
					summary.IsLive = botPos.IsLive
				}
			} else if user.ID == DITZ_USER_ID {
				var botPos DitzPosition
				if db.Where("symbol = ?", pos.Symbol).First(&botPos).Error == nil {
					summary.IsLive = botPos.IsLive
				}
			} else if user.ID == TRADER_USER_ID {
				var botPos TraderPosition
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

	// Determine start time for period-based calculations
	now := time.Now()
	var startTime time.Time
	switch period {
	case "1d":
		startTime = now.AddDate(0, 0, -1)
	case "1w":
		startTime = now.AddDate(0, 0, -7)
	case "1m":
		startTime = now.AddDate(0, -1, 0)
	case "3m":
		startTime = now.AddDate(0, -3, 0)
	case "6m":
		startTime = now.AddDate(0, -6, 0)
	case "1y":
		startTime = now.AddDate(-1, 0, 0)
	case "ytd":
		startTime = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	case "5y":
		startTime = now.AddDate(-5, 0, 0)
	default:
		startTime = now.AddDate(0, -1, 0)
	}

	// Get all users with positions
	var users []User
	db.Find(&users)

	type PortfolioHistory struct {
		UserID          uint                     `json:"user_id"`
		Username        string                   `json:"username"`
		History         []map[string]interface{} `json:"history"`
		PeriodReturnPct float64                  `json:"period_return_pct"`
	}

	var result []PortfolioHistory
	var resultMu sync.Mutex
	var userWg sync.WaitGroup

	for _, user := range users {
		userWg.Add(1)
		go func(u User) {
			defer userWg.Done()

			var posCount int64
			db.Model(&PortfolioPosition{}).Where("user_id = ?", u.ID).Count(&posCount)

			var closedCount int64
			db.Model(&PortfolioTradeHistory{}).Where("user_id = ? AND sell_date >= ?", u.ID, startTime).Count(&closedCount)

			if posCount == 0 && closedCount == 0 {
				return
			}

			var history []map[string]interface{}
			if posCount > 0 {
				history = calculatePortfolioHistoryForUser(u.ID, period)
			}

			openReturnPct := 0.0
			startValue := 0.0
			if len(history) >= 2 {
				startValue = history[0]["value"].(float64)
				openReturnPct = history[len(history)-1]["pct"].(float64)
			}

			var closedTrades []PortfolioTradeHistory
			db.Where("user_id = ? AND sell_date >= ?", u.ID, startTime).Find(&closedTrades)
			closedGain := 0.0
			closedInvested := 0.0
			for _, t := range closedTrades {
				closedGain += t.ProfitLoss
				closedInvested += t.BuyPrice * t.Quantity
			}

			periodReturn := openReturnPct
			if len(closedTrades) > 0 {
				openGain := openReturnPct / 100 * startValue
				totalCapital := startValue + closedInvested
				if totalCapital > 0 {
					periodReturn = ((openGain + closedGain) / totalCapital) * 100
				}
			}

			if len(history) > 0 || len(closedTrades) > 0 {
				resultMu.Lock()
				result = append(result, PortfolioHistory{
					UserID:          u.ID,
					Username:        u.Username,
					History:         history,
					PeriodReturnPct: periodReturn,
				})
				resultMu.Unlock()
			}
		}(user)
	}
	userWg.Wait()

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
	case "1d":
		yahooRange = "5d"
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

	// Collect symbols and fetch historical data in parallel
	symbolData := make(map[string][]OHLCV)
	var fetchMu sync.Mutex
	var fetchWg sync.WaitGroup
	for _, pos := range positions {
		fetchWg.Add(1)
		go func(symbol string) {
			defer fetchWg.Done()
			data := fetchHistoricalData(symbol, yahooRange)
			if len(data) > 0 {
				fetchMu.Lock()
				symbolData[symbol] = data
				fetchMu.Unlock()
			}
		}(pos.Symbol)
	}
	fetchWg.Wait()

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
	// Key idea: each position only contributes from its PurchaseDate onward.
	// When a new position joins mid-period, we rebase the portfolio value so the
	// new position starts at 0% return (its buy price), preventing a fake jump.
	result := make([]map[string]interface{}, 0)

	if len(allTimes) == 0 {
		return result
	}

	hasQuantities := false
	for _, pos := range positions {
		if pos.Quantity != nil && *pos.Quantity > 0 {
			hasQuantities = true
			break
		}
	}

	// Track last known prices for each symbol (for filling gaps)
	lastPrices := make(map[string]float64)

	// Pre-fill with first available price per symbol
	for _, pos := range positions {
		if data, ok := symbolData[pos.Symbol]; ok && len(data) > 0 {
			lastPrices[pos.Symbol] = data[0].Close
		}
	}

	// For each position, determine the "base price" at time of entry into the chart.
	// If PurchaseDate is before the chart period, the base price is the first price in the period.
	// If PurchaseDate is during the chart period, the base price is the price at PurchaseDate (AvgPrice).
	type posEntry struct {
		pos       PortfolioPosition
		basePrice float64 // price at chart entry (in position's currency)
		weight    float64 // invested amount at base
	}

	prevActiveCount := 0
	var baseValue float64 // tracks the "rebased" base for pct calculation
	var prevPct float64   // tracks accumulated pct before rebase

	for _, t := range allTimes {
		prices := timeValues[t]

		// Update last known prices
		for symbol, price := range prices {
			lastPrices[symbol] = price
		}

		// Determine which positions are active at this time point
		var activeEntries []posEntry
		for _, pos := range positions {
			if pos.PurchaseDate != nil && pos.PurchaseDate.Unix() > t {
				continue
			}

			currency := pos.Currency
			if currency == "" {
				currency = "EUR"
			}

			// Base price: if position existed before chart start, use first chart price
			// Otherwise use the purchase price (AvgPrice)
			bp := pos.AvgPrice
			if pos.PurchaseDate == nil || pos.PurchaseDate.Unix() <= allTimes[0] {
				// Position pre-dates the chart → base is first available price in chart
				if data, ok := symbolData[pos.Symbol]; ok && len(data) > 0 {
					bp = convertStockPrice(data[0].Close, pos.Symbol, currency)
				}
			}

			qty := 1.0
			if hasQuantities {
				if pos.Quantity != nil && *pos.Quantity > 0 {
					qty = *pos.Quantity
				} else {
					continue
				}
			}

			w := bp * qty
			if !hasQuantities {
				w = 1000.0 // equal weight per position when no quantities
			}

			activeEntries = append(activeEntries, posEntry{pos: pos, basePrice: bp, weight: w})
		}

		if len(activeEntries) == 0 {
			continue
		}

		// Calculate portfolio value at this time
		var portfolioValue float64
		if hasQuantities {
			for _, e := range activeEntries {
				if price, ok := lastPrices[e.pos.Symbol]; ok {
					currency := e.pos.Currency
					if currency == "" {
						currency = "EUR"
					}
					priceInUserCurrency := convertStockPrice(price, e.pos.Symbol, currency)
					qty := 1.0
					if e.pos.Quantity != nil {
						qty = *e.pos.Quantity
					}
					portfolioValue += priceInUserCurrency * qty
				}
			}
		} else {
			for _, e := range activeEntries {
				if price, ok := lastPrices[e.pos.Symbol]; ok {
					currency := e.pos.Currency
					if currency == "" {
						currency = "EUR"
					}
					priceInUserCurrency := convertStockPrice(price, e.pos.Symbol, currency)
					if e.basePrice > 0 {
						portfolioValue += 1000 * (priceInUserCurrency / e.basePrice)
					}
				}
			}
		}

		if portfolioValue <= 0 {
			continue
		}

		// Detect when new positions join: rebase to prevent fake jump
		if len(activeEntries) != prevActiveCount {
			if len(result) > 0 {
				prevPct = result[len(result)-1]["pct"].(float64)
			}
			baseValue = portfolioValue
			prevActiveCount = len(activeEntries)
		}

		// Set initial base if first data point
		if baseValue == 0 {
			baseValue = portfolioValue
		}

		pct := prevPct
		if baseValue > 0 {
			pct = prevPct + ((portfolioValue-baseValue)/baseValue)*100
		}

		result = append(result, map[string]interface{}{
			"time":  t,
			"value": portfolioValue,
			"pct":   pct,
		})
	}

	return result
}

// Cache for Yahoo historical data to avoid repeated API calls
var (
	histCacheMu sync.RWMutex
	histCache   = make(map[string]histCacheEntry)
)

type histCacheEntry struct {
	Data      []OHLCV
	FetchedAt time.Time
}

const histCacheTTL = 10 * time.Minute

func fetchHistoricalData(symbol string, period string) []OHLCV {
	cacheKey := symbol + ":" + period

	// Check cache first
	histCacheMu.RLock()
	if entry, ok := histCache[cacheKey]; ok && time.Since(entry.FetchedAt) < histCacheTTL {
		histCacheMu.RUnlock()
		return entry.Data
	}
	histCacheMu.RUnlock()

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

	// Store in cache
	histCacheMu.Lock()
	histCache[cacheKey] = histCacheEntry{Data: data, FetchedAt: time.Now()}
	histCacheMu.Unlock()

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
		AvgReturn    float64     `json:"avg_return"`
		TotalTrades  int         `json:"total_trades"`
		Wins         int         `json:"wins"`
		Losses       int         `json:"losses"`
		Signal       string      `json:"signal"`
		SignalBars   int         `json:"signal_bars"`
		Trades       []TradeData `json:"trades"`
		CurrentPrice float64     `json:"current_price"`
		MarketCap    int64       `json:"market_cap"`
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

	// Compute signal_since from trades
	newSignalSince := calcSignalSinceFromRequest(req.Trades, req.SignalBars)

	if result.Error == nil {
		// Update existing
		ss, ps, pss := updateSignalHistory(existing.Signal, existing.SignalSince, req.Signal, newSignalSince)
		existing.Name = req.Name
		existing.WinRate = req.WinRate
		existing.RiskReward = req.RiskReward
		existing.TotalReturn = req.TotalReturn
		existing.AvgReturn = req.AvgReturn
		existing.TotalTrades = req.TotalTrades
		existing.Wins = req.Wins
		existing.Losses = req.Losses
		existing.Signal = req.Signal
		existing.SignalBars = req.SignalBars
		existing.SignalSince = ss
		if ps != "" {
			existing.PrevSignal = ps
			existing.PrevSignalSince = pss
		}
		existing.TradesJSON = string(tradesJSON)
		existing.CurrentPrice = req.CurrentPrice
		if req.MarketCap > 0 {
			existing.MarketCap = req.MarketCap
		}
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
			AvgReturn:    req.AvgReturn,
			TotalTrades:  req.TotalTrades,
			Wins:         req.Wins,
			Losses:       req.Losses,
			Signal:       req.Signal,
			SignalBars:   req.SignalBars,
			SignalSince:  newSignalSince,
			TradesJSON:   string(tradesJSON),
			CurrentPrice: req.CurrentPrice,
			MarketCap:    req.MarketCap,
		}
		db.Create(&perf)
		c.JSON(http.StatusCreated, perf)
	}

	// Sync MarketCap to stocks table
	if req.MarketCap > 0 {
		db.Model(&Stock{}).Where("symbol = ?", symbol).Update("market_cap", req.MarketCap)
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
		AvgReturn    float64     `json:"avg_return"`
		TotalTrades  int         `json:"total_trades"`
		Wins         int         `json:"wins"`
		Losses       int         `json:"losses"`
		Signal       string      `json:"signal"`
		SignalBars   int         `json:"signal_bars"`
		Trades       []TradeData `json:"trades"`
		CurrentPrice float64     `json:"current_price"`
		MarketCap    int64       `json:"market_cap"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	symbol := strings.ToUpper(req.Symbol)
	tradesJSON, _ := json.Marshal(req.Trades)

	newSignalSince := calcSignalSinceFromRequest(req.Trades, req.SignalBars)

	var existing AggressiveStockPerformance
	result := db.Where("symbol = ?", symbol).First(&existing)

	if result.Error == nil {
		ss, ps, pss := updateSignalHistory(existing.Signal, existing.SignalSince, req.Signal, newSignalSince)
		existing.Name = req.Name
		existing.WinRate = req.WinRate
		existing.RiskReward = req.RiskReward
		existing.TotalReturn = req.TotalReturn
		existing.AvgReturn = req.AvgReturn
		existing.TotalTrades = req.TotalTrades
		existing.Wins = req.Wins
		existing.Losses = req.Losses
		existing.Signal = req.Signal
		existing.SignalBars = req.SignalBars
		existing.SignalSince = ss
		if ps != "" {
			existing.PrevSignal = ps
			existing.PrevSignalSince = pss
		}
		existing.TradesJSON = string(tradesJSON)
		existing.CurrentPrice = req.CurrentPrice
		if req.MarketCap > 0 {
			existing.MarketCap = req.MarketCap
		}
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
			AvgReturn:    req.AvgReturn,
			TotalTrades:  req.TotalTrades,
			Wins:         req.Wins,
			Losses:       req.Losses,
			Signal:       req.Signal,
			SignalBars:   req.SignalBars,
			SignalSince:  newSignalSince,
			TradesJSON:   string(tradesJSON),
			CurrentPrice: req.CurrentPrice,
			MarketCap:    req.MarketCap,
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

// Quant mode performance handlers
func saveQuantStockPerformance(c *gin.Context) {
	var req struct {
		Symbol       string      `json:"symbol" binding:"required"`
		Name         string      `json:"name"`
		WinRate      float64     `json:"win_rate"`
		RiskReward   float64     `json:"risk_reward"`
		TotalReturn  float64     `json:"total_return"`
		AvgReturn    float64     `json:"avg_return"`
		TotalTrades  int         `json:"total_trades"`
		Wins         int         `json:"wins"`
		Losses       int         `json:"losses"`
		Signal       string      `json:"signal"`
		SignalBars   int         `json:"signal_bars"`
		Trades       []TradeData `json:"trades"`
		CurrentPrice float64     `json:"current_price"`
		MarketCap    int64       `json:"market_cap"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	symbol := strings.ToUpper(req.Symbol)
	tradesJSON, _ := json.Marshal(req.Trades)

	newSignalSince := calcSignalSinceFromRequest(req.Trades, req.SignalBars)

	var existing QuantStockPerformance
	result := db.Where("symbol = ?", symbol).First(&existing)

	if result.Error == nil {
		ss, ps, pss := updateSignalHistory(existing.Signal, existing.SignalSince, req.Signal, newSignalSince)
		existing.Name = req.Name
		existing.WinRate = req.WinRate
		existing.RiskReward = req.RiskReward
		existing.TotalReturn = req.TotalReturn
		existing.AvgReturn = req.AvgReturn
		existing.TotalTrades = req.TotalTrades
		existing.Wins = req.Wins
		existing.Losses = req.Losses
		existing.Signal = req.Signal
		existing.SignalBars = req.SignalBars
		existing.SignalSince = ss
		if ps != "" {
			existing.PrevSignal = ps
			existing.PrevSignalSince = pss
		}
		existing.TradesJSON = string(tradesJSON)
		existing.CurrentPrice = req.CurrentPrice
		if req.MarketCap > 0 {
			existing.MarketCap = req.MarketCap
		}
		existing.UpdatedAt = time.Now()
		db.Save(&existing)
		c.JSON(http.StatusOK, existing)
	} else {
		perf := QuantStockPerformance{
			Symbol:       symbol,
			Name:         req.Name,
			WinRate:      req.WinRate,
			RiskReward:   req.RiskReward,
			TotalReturn:  req.TotalReturn,
			AvgReturn:    req.AvgReturn,
			TotalTrades:  req.TotalTrades,
			Wins:         req.Wins,
			Losses:       req.Losses,
			Signal:       req.Signal,
			SignalBars:   req.SignalBars,
			SignalSince:  newSignalSince,
			TradesJSON:   string(tradesJSON),
			CurrentPrice: req.CurrentPrice,
			MarketCap:    req.MarketCap,
		}
		db.Create(&perf)
		c.JSON(http.StatusCreated, perf)
	}
}

func getQuantTrackedStocks(c *gin.Context) {
	var performances []QuantStockPerformance
	db.Order("updated_at desc").Find(&performances)

	type PerformanceWithTrades struct {
		QuantStockPerformance
		Trades []TradeData `json:"trades"`
	}

	result := make([]PerformanceWithTrades, len(performances))
	for i, p := range performances {
		result[i].QuantStockPerformance = p
		if p.TradesJSON != "" {
			json.Unmarshal([]byte(p.TradesJSON), &result[i].Trades)
		}
	}

	c.JSON(http.StatusOK, result)
}

func getQuantStockPerformance(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))

	var perf QuantStockPerformance
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

// Quant config handlers
func getBXtrenderQuantConfigPublic(c *gin.Context) {
	var config BXtrenderQuantConfig
	result := db.First(&config)

	if result.Error != nil {
		// Return default config
		config = BXtrenderQuantConfig{
			ShortL1:    5,
			ShortL2:    20,
			ShortL3:    15,
			LongL1:     20,
			LongL2:     15,
			MaFilterOn: true,
			MaLength:   200,
			MaType:     "EMA",
			TslPercent: 20.0,
		}
	}

	c.JSON(http.StatusOK, config)
}

func getBXtrenderQuantConfig(c *gin.Context) {
	var config BXtrenderQuantConfig
	result := db.First(&config)

	if result.Error != nil {
		// Return default config
		config = BXtrenderQuantConfig{
			ShortL1:    5,
			ShortL2:    20,
			ShortL3:    15,
			LongL1:     20,
			LongL2:     15,
			MaFilterOn: true,
			MaLength:   200,
			MaType:     "EMA",
			TslPercent: 20.0,
		}
	}

	c.JSON(http.StatusOK, config)
}

func updateBXtrenderQuantConfig(c *gin.Context) {
	var req struct {
		ShortL1    int     `json:"short_l1"`
		ShortL2    int     `json:"short_l2"`
		ShortL3    int     `json:"short_l3"`
		LongL1     int     `json:"long_l1"`
		LongL2     int     `json:"long_l2"`
		MaFilterOn bool    `json:"ma_filter_on"`
		MaLength   int     `json:"ma_length"`
		MaType     string  `json:"ma_type"`
		TslPercent float64 `json:"tsl_percent"`
		TslEnabled *bool   `json:"tsl_enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var config BXtrenderQuantConfig
	result := db.First(&config)

	if result.Error != nil {
		tslEnabled := true
		if req.TslEnabled != nil {
			tslEnabled = *req.TslEnabled
		}
		config = BXtrenderQuantConfig{
			ShortL1:    req.ShortL1,
			ShortL2:    req.ShortL2,
			ShortL3:    req.ShortL3,
			LongL1:     req.LongL1,
			LongL2:     req.LongL2,
			MaFilterOn: req.MaFilterOn,
			MaLength:   req.MaLength,
			MaType:     req.MaType,
			TslPercent: req.TslPercent,
			TslEnabled: tslEnabled,
			UpdatedAt:  time.Now(),
		}
		db.Create(&config)
	} else {
		config.ShortL1 = req.ShortL1
		config.ShortL2 = req.ShortL2
		config.ShortL3 = req.ShortL3
		config.LongL1 = req.LongL1
		config.LongL2 = req.LongL2
		config.MaFilterOn = req.MaFilterOn
		config.MaLength = req.MaLength
		config.MaType = req.MaType
		config.TslPercent = req.TslPercent
		if req.TslEnabled != nil {
			config.TslEnabled = *req.TslEnabled
		}
		config.UpdatedAt = time.Now()
		db.Save(&config)
	}

	c.JSON(http.StatusOK, config)
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

// getBXtrenderConfig returns the BXtrender configuration for admin
func getBXtrenderConfig(c *gin.Context) {
	var configs []BXtrenderConfig
	db.Find(&configs)

	// Create default configs if they don't exist
	if len(configs) == 0 {
		defensive := BXtrenderConfig{Mode: "defensive", ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15}
		aggressive := BXtrenderConfig{Mode: "aggressive", ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15}
		db.Create(&defensive)
		db.Create(&aggressive)
		configs = []BXtrenderConfig{defensive, aggressive}
	}

	c.JSON(http.StatusOK, configs)
}

// getBXtrenderConfigPublic returns the BXtrender configuration for frontend (no auth)
func getBXtrenderConfigPublic(c *gin.Context) {
	var configs []BXtrenderConfig
	db.Find(&configs)

	// Create default configs if they don't exist
	if len(configs) == 0 {
		defensive := BXtrenderConfig{Mode: "defensive", ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15}
		aggressive := BXtrenderConfig{Mode: "aggressive", ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15}
		db.Create(&defensive)
		db.Create(&aggressive)
		configs = []BXtrenderConfig{defensive, aggressive}
	}

	// Return as a map for easier frontend access
	result := make(map[string]BXtrenderConfig)
	for _, cfg := range configs {
		result[cfg.Mode] = cfg
	}

	c.JSON(http.StatusOK, result)
}

// updateBXtrenderConfig updates the BXtrender configuration
func updateBXtrenderConfig(c *gin.Context) {
	var req struct {
		Mode       string  `json:"mode" binding:"required"`
		ShortL1    int     `json:"short_l1"`
		ShortL2    int     `json:"short_l2"`
		ShortL3    int     `json:"short_l3"`
		LongL1     int     `json:"long_l1"`
		LongL2     int     `json:"long_l2"`
		TslPercent float64 `json:"tsl_percent"`
		TslEnabled *bool   `json:"tsl_enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var config BXtrenderConfig
	if err := db.Where("mode = ?", req.Mode).First(&config).Error; err != nil {
		// Create new config
		tslEnabled := true
		if req.TslEnabled != nil {
			tslEnabled = *req.TslEnabled
		}
		config = BXtrenderConfig{
			Mode:       req.Mode,
			ShortL1:    req.ShortL1,
			ShortL2:    req.ShortL2,
			ShortL3:    req.ShortL3,
			LongL1:     req.LongL1,
			LongL2:     req.LongL2,
			TslPercent: req.TslPercent,
			TslEnabled: tslEnabled,
		}
		db.Create(&config)
	} else {
		// Update existing config
		config.ShortL1 = req.ShortL1
		config.ShortL2 = req.ShortL2
		config.ShortL3 = req.ShortL3
		config.LongL1 = req.LongL1
		config.LongL2 = req.LongL2
		if req.TslPercent > 0 {
			config.TslPercent = req.TslPercent
		}
		if req.TslEnabled != nil {
			config.TslEnabled = *req.TslEnabled
		}
		db.Save(&config)
	}

	c.JSON(http.StatusOK, config)
}

// updateAllWatchlistStocks returns all watchlist stocks for bulk update
// The actual BX-Trender calculation happens in the frontend
func updateAllWatchlistStocks(c *gin.Context) {
	mode := c.DefaultQuery("mode", "defensive")

	// Get all stocks from watchlist, largest market cap first
	var stocks []Stock
	db.Order("market_cap desc").Find(&stocks)

	// Get last performance update per symbol (newest across all 5 mode tables)
	type SymUpdate struct {
		Symbol    string
		UpdatedAt time.Time
	}
	lastUpdates := make(map[string]time.Time)

	tables := []string{"stock_performances", "aggressive_stock_performances", "quant_stock_performances", "ditz_stock_performances", "trader_stock_performances"}
	for _, table := range tables {
		var rows []SymUpdate
		db.Table(table).Select("symbol, updated_at").Find(&rows)
		for _, r := range rows {
			if existing, ok := lastUpdates[r.Symbol]; !ok || r.UpdatedAt.After(existing) {
				lastUpdates[r.Symbol] = r.UpdatedAt
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"mode":         mode,
		"stocks":       stocks,
		"total":        len(stocks),
		"last_updates": lastUpdates,
	})
}

func exportWatchlist(c *gin.Context) {
	var stocks []Stock
	db.Order("symbol asc").Find(&stocks)

	var categories []Category
	db.Find(&categories)
	catMap := make(map[uint]string)
	for _, cat := range categories {
		catMap[cat.ID] = cat.Name
	}

	type ExportEntry struct {
		Symbol    string `json:"symbol"`
		Name      string `json:"name"`
		Category  string `json:"category"`
		ISIN      string `json:"isin"`
		MarketCap int64  `json:"market_cap"`
	}

	var entries []ExportEntry
	for _, s := range stocks {
		catName := ""
		if s.CategoryID != nil {
			catName = catMap[*s.CategoryID]
		}
		entries = append(entries, ExportEntry{
			Symbol:    s.Symbol,
			Name:      s.Name,
			Category:  catName,
			ISIN:      s.ISIN,
			MarketCap: s.MarketCap,
		})
	}

	c.Header("Content-Disposition", "attachment; filename=watchlist_export.json")
	c.JSON(http.StatusOK, entries)
}

func importWatchlist(c *gin.Context) {
	type ImportEntry struct {
		Symbol    string `json:"symbol"`
		Name      string `json:"name"`
		Category  string `json:"category"`
		ISIN      string `json:"isin"`
		MarketCap int64  `json:"market_cap"`
	}

	var entries []ImportEntry
	if err := c.ShouldBindJSON(&entries); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültiges JSON: " + err.Error()})
		return
	}

	// Load existing stocks and categories
	var existingStocks []Stock
	db.Find(&existingStocks)
	stockMap := make(map[string]*Stock)
	for i := range existingStocks {
		stockMap[existingStocks[i].Symbol] = &existingStocks[i]
	}

	var existingCategories []Category
	db.Find(&existingCategories)
	catByName := make(map[string]*Category)
	maxSort := 0
	for i := range existingCategories {
		catByName[existingCategories[i].Name] = &existingCategories[i]
		if existingCategories[i].SortOrder > maxSort {
			maxSort = existingCategories[i].SortOrder
		}
	}

	type ResultEntry struct {
		Symbol string `json:"symbol"`
		Name   string `json:"name"`
		Action string `json:"action"`
	}

	var results []ResultEntry
	var newStocks []ResultEntry
	created := 0
	updated := 0

	// Get admin username from context
	adminUser := "admin"
	if userObj, exists := c.Get("user"); exists {
		if u, ok := userObj.(User); ok {
			adminUser = u.Username
		}
	}

	for _, entry := range entries {
		if entry.Symbol == "" {
			continue
		}

		// Resolve category
		var catID *uint
		if entry.Category != "" {
			cat, exists := catByName[entry.Category]
			if !exists {
				maxSort++
				newCat := Category{Name: entry.Category, SortOrder: maxSort}
				db.Create(&newCat)
				catByName[entry.Category] = &newCat
				cat = &newCat
			}
			catID = &cat.ID
		}

		if existing, exists := stockMap[entry.Symbol]; exists {
			// Update existing stock
			updates := map[string]interface{}{"category_id": catID}
			if entry.ISIN != "" {
				updates["isin"] = entry.ISIN
			}
			db.Model(existing).Updates(updates)
			results = append(results, ResultEntry{Symbol: entry.Symbol, Name: existing.Name, Action: "updated"})
			updated++
		} else {
			// Create new stock
			newStock := Stock{
				Symbol:      entry.Symbol,
				Name:        entry.Name,
				CategoryID:  catID,
				ISIN:        entry.ISIN,
				MarketCap:   entry.MarketCap,
				AddedByUser: adminUser,
			}
			db.Create(&newStock)
			results = append(results, ResultEntry{Symbol: entry.Symbol, Name: entry.Name, Action: "created"})
			newStocks = append(newStocks, ResultEntry{Symbol: entry.Symbol, Name: entry.Name})
			stockMap[entry.Symbol] = &newStock
			created++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"results":    results,
		"total":      len(results),
		"created":    created,
		"updated":    updated,
		"new_stocks": newStocks,
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

func ensureQuantUser() {
	// Create Quant user if not exists (for portfolio comparison visibility)
	var user User
	result := db.Where("id = ?", QUANT_USER_ID).First(&user)
	if result.Error != nil {
		hashedPassword, _ := hashPassword("quant-system-user-no-login")
		botUser := User{
			ID:       QUANT_USER_ID,
			Email:    "quant@system.local",
			Username: "Quant",
			Password: hashedPassword,
			IsAdmin:  false,
		}
		db.Create(&botUser)
	}
}

func isStockAllowedForBot(botName, symbol string) bool {
	var entry BotStockAllowlist
	if err := db.Where("bot_name = ? AND symbol = ?", botName, symbol).First(&entry).Error; err != nil {
		return true // No entry = allowed
	}
	return entry.Allowed
}

// checkBotFilterConfig checks if a stock passes the bot's performance filter criteria.
// Returns (blocked bool, reason string). If blocked=true, the trade should be recorded but not executed.
func checkBotFilterConfig(botName string, winRate, riskReward, avgReturn float64, marketCap int64) (bool, string) {
	var config BotFilterConfig
	if err := db.Where("bot_name = ?", botName).First(&config).Error; err != nil {
		return false, "" // No config = no filter = allow
	}
	if !config.Enabled {
		return false, "" // Filter disabled = allow
	}

	var reasons []string

	if config.MinWinrate != nil && *config.MinWinrate != 0 && winRate < *config.MinWinrate {
		reasons = append(reasons, fmt.Sprintf("WinRate %.1f%% < Min %.1f%%", winRate, *config.MinWinrate))
	}
	if config.MaxWinrate != nil && *config.MaxWinrate != 0 && winRate > *config.MaxWinrate {
		reasons = append(reasons, fmt.Sprintf("WinRate %.1f%% > Max %.1f%%", winRate, *config.MaxWinrate))
	}
	if config.MinRR != nil && *config.MinRR != 0 && riskReward < *config.MinRR {
		reasons = append(reasons, fmt.Sprintf("R/R %.2f < Min %.2f", riskReward, *config.MinRR))
	}
	if config.MaxRR != nil && *config.MaxRR != 0 && riskReward > *config.MaxRR {
		reasons = append(reasons, fmt.Sprintf("R/R %.2f > Max %.2f", riskReward, *config.MaxRR))
	}
	if config.MinAvgReturn != nil && *config.MinAvgReturn != 0 && avgReturn < *config.MinAvgReturn {
		reasons = append(reasons, fmt.Sprintf("AvgReturn %.1f%% < Min %.1f%%", avgReturn, *config.MinAvgReturn))
	}
	if config.MaxAvgReturn != nil && *config.MaxAvgReturn != 0 && avgReturn > *config.MaxAvgReturn {
		reasons = append(reasons, fmt.Sprintf("AvgReturn %.1f%% > Max %.1f%%", avgReturn, *config.MaxAvgReturn))
	}
	if config.MinMarketCap != nil && *config.MinMarketCap != 0 {
		minCapValue := *config.MinMarketCap * 1e9
		if float64(marketCap) < minCapValue {
			mcapBillions := float64(marketCap) / 1e9
			reasons = append(reasons, fmt.Sprintf("MarketCap %.1f Mrd < Min %.1f Mrd", mcapBillions, *config.MinMarketCap))
		}
	}

	if len(reasons) > 0 {
		return true, strings.Join(reasons, "; ")
	}
	return false, ""
}

func closePositionForBot(botName, symbol string) bool {
	now := time.Now()

	// Fetch current price
	quotes := fetchQuotes([]string{symbol})
	currentPrice := quotes[symbol].Price
	if currentPrice <= 0 {
		return false
	}

	switch botName {
	case "flipper":
		var pos FlipperBotPosition
		if err := db.Where("symbol = ? AND is_closed = ? AND is_pending = ?", symbol, false, false).First(&pos).Error; err != nil {
			return false
		}
		pnl := (currentPrice - pos.AvgPrice) * pos.Quantity
		pnlPct := ((currentPrice - pos.AvgPrice) / pos.AvgPrice) * 100
		sellTrade := FlipperBotTrade{
			Symbol: pos.Symbol, Name: pos.Name, Action: "SELL", Quantity: pos.Quantity,
			Price: currentPrice, SignalDate: now, ExecutedAt: now,
			IsPending: false, IsLive: pos.IsLive, IsAdminClosed: true,
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
		db.Where("user_id = ? AND symbol = ?", FLIPPERBOT_USER_ID, symbol).Delete(&PortfolioPosition{})
		return true

	case "lutz":
		var pos LutzPosition
		if err := db.Where("symbol = ? AND is_closed = ? AND is_pending = ?", symbol, false, false).First(&pos).Error; err != nil {
			return false
		}
		pnl := (currentPrice - pos.AvgPrice) * pos.Quantity
		pnlPct := ((currentPrice - pos.AvgPrice) / pos.AvgPrice) * 100
		sellTrade := LutzTrade{
			Symbol: pos.Symbol, Name: pos.Name, Action: "SELL", Quantity: pos.Quantity,
			Price: currentPrice, SignalDate: now, ExecutedAt: now,
			IsPending: false, IsLive: pos.IsLive, IsAdminClosed: true,
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
		db.Where("user_id = ? AND symbol = ?", LUTZ_USER_ID, symbol).Delete(&PortfolioPosition{})
		return true

	case "quant":
		var pos QuantPosition
		if err := db.Where("symbol = ? AND is_closed = ? AND is_pending = ?", symbol, false, false).First(&pos).Error; err != nil {
			return false
		}
		pnl := (currentPrice - pos.AvgPrice) * pos.Quantity
		pnlPct := ((currentPrice - pos.AvgPrice) / pos.AvgPrice) * 100
		sellTrade := QuantTrade{
			Symbol: pos.Symbol, Name: pos.Name, Action: "SELL", Quantity: pos.Quantity,
			Price: currentPrice, SignalDate: now, ExecutedAt: now,
			IsPending: false, IsLive: pos.IsLive, IsAdminClosed: true,
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
		db.Where("user_id = ? AND symbol = ?", QUANT_USER_ID, symbol).Delete(&PortfolioPosition{})
		return true

	case "ditz":
		var pos DitzPosition
		if err := db.Where("symbol = ? AND is_closed = ? AND is_pending = ?", symbol, false, false).First(&pos).Error; err != nil {
			return false
		}
		pnl := (currentPrice - pos.AvgPrice) * pos.Quantity
		pnlPct := ((currentPrice - pos.AvgPrice) / pos.AvgPrice) * 100
		sellTrade := DitzTrade{
			Symbol: pos.Symbol, Name: pos.Name, Action: "SELL", Quantity: pos.Quantity,
			Price: currentPrice, SignalDate: now, ExecutedAt: now,
			IsPending: false, IsLive: pos.IsLive, IsAdminClosed: true,
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
		db.Where("user_id = ? AND symbol = ?", DITZ_USER_ID, symbol).Delete(&PortfolioPosition{})
		return true

	case "trader":
		var pos TraderPosition
		if err := db.Where("symbol = ? AND is_closed = ? AND is_pending = ?", symbol, false, false).First(&pos).Error; err != nil {
			return false
		}
		pnl := (currentPrice - pos.AvgPrice) * pos.Quantity
		pnlPct := ((currentPrice - pos.AvgPrice) / pos.AvgPrice) * 100
		sellTrade := TraderTrade{
			Symbol: pos.Symbol, Name: pos.Name, Action: "SELL", Quantity: pos.Quantity,
			Price: currentPrice, SignalDate: now, ExecutedAt: now,
			IsPending: false, IsLive: pos.IsLive, IsAdminClosed: true,
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
		db.Where("user_id = ? AND symbol = ?", TRADER_USER_ID, symbol).Delete(&PortfolioPosition{})
		return true
	}
	return false
}

func getBotAllowlist(c *gin.Context) {
	botConfigs := map[string]interface{}{}
	botNames := []string{"flipper", "lutz", "quant", "ditz", "trader"}

	for _, botName := range botNames {
		var symbols []string
		switch botName {
		case "flipper":
			var stocks []StockPerformance
			db.Select("symbol").Find(&stocks)
			for _, s := range stocks {
				symbols = append(symbols, s.Symbol)
			}
		case "lutz":
			var stocks []AggressiveStockPerformance
			db.Select("symbol").Find(&stocks)
			for _, s := range stocks {
				symbols = append(symbols, s.Symbol)
			}
		case "quant":
			var stocks []QuantStockPerformance
			db.Select("symbol").Find(&stocks)
			for _, s := range stocks {
				symbols = append(symbols, s.Symbol)
			}
		case "ditz":
			var stocks []DitzStockPerformance
			db.Select("symbol").Find(&stocks)
			for _, s := range stocks {
				symbols = append(symbols, s.Symbol)
			}
		case "trader":
			var stocks []TraderStockPerformance
			db.Select("symbol").Find(&stocks)
			for _, s := range stocks {
				symbols = append(symbols, s.Symbol)
			}
		}

		var allowlistEntries []BotStockAllowlist
		db.Where("bot_name = ?", botName).Find(&allowlistEntries)
		blockedMap := map[string]bool{}
		for _, e := range allowlistEntries {
			if !e.Allowed {
				blockedMap[e.Symbol] = true
			}
		}

		var entries []map[string]interface{}
		for _, sym := range symbols {
			entries = append(entries, map[string]interface{}{
				"symbol":  sym,
				"allowed": !blockedMap[sym],
			})
		}
		botConfigs[botName] = entries
	}

	c.JSON(http.StatusOK, botConfigs)
}

func updateBotAllowlist(c *gin.Context) {
	var req struct {
		BotName string `json:"bot_name" binding:"required"`
		Symbol  string `json:"symbol" binding:"required"`
		Allowed bool   `json:"allowed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bot_name and symbol required"})
		return
	}

	validBots := map[string]bool{"flipper": true, "lutz": true, "quant": true, "ditz": true, "trader": true}
	if !validBots[req.BotName] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid bot_name"})
		return
	}

	var entry BotStockAllowlist
	result := db.Where("bot_name = ? AND symbol = ?", req.BotName, req.Symbol).First(&entry)
	if result.Error != nil {
		entry = BotStockAllowlist{BotName: req.BotName, Symbol: req.Symbol, Allowed: req.Allowed}
		db.Create(&entry)
	} else {
		entry.Allowed = req.Allowed
		db.Save(&entry)
	}

	closedPosition := false
	if !req.Allowed {
		closedPosition = closePositionForBot(req.BotName, req.Symbol)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Updated", "closed_position": closedPosition})
}

func getBotFilterConfig(c *gin.Context) {
	var configs []BotFilterConfig
	db.Find(&configs)

	result := make(map[string]BotFilterConfig)
	for _, config := range configs {
		result[config.BotName] = config
	}

	for _, botName := range []string{"flipper", "lutz", "quant", "ditz", "trader"} {
		if _, exists := result[botName]; !exists {
			result[botName] = BotFilterConfig{BotName: botName, Enabled: false}
		}
	}

	c.JSON(http.StatusOK, result)
}

func updateBotFilterConfig(c *gin.Context) {
	var req BotFilterConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	validBots := map[string]bool{"flipper": true, "lutz": true, "quant": true, "ditz": true, "trader": true}
	if !validBots[req.BotName] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid bot_name"})
		return
	}

	var config BotFilterConfig
	result := db.Where("bot_name = ?", req.BotName).First(&config)
	if result.Error != nil {
		req.UpdatedAt = time.Now()
		db.Create(&req)
		c.JSON(http.StatusOK, req)
	} else {
		config.MinWinrate = req.MinWinrate
		config.MaxWinrate = req.MaxWinrate
		config.MinRR = req.MinRR
		config.MaxRR = req.MaxRR
		config.MinAvgReturn = req.MinAvgReturn
		config.MaxAvgReturn = req.MaxAvgReturn
		config.MinMarketCap = req.MinMarketCap
		config.Enabled = req.Enabled
		config.UpdatedAt = time.Now()
		db.Save(&config)
		c.JSON(http.StatusOK, config)
	}
}

// checkFlipperStopLoss checks all open Flipper positions against their stop loss
func checkFlipperStopLoss() {
	var config BXtrenderConfig
	if err := db.Where("mode = ?", "defensive").First(&config).Error; err != nil {
		return
	}
	if !config.TslEnabled {
		return
	}

	var positions []FlipperBotPosition
	db.Where("is_closed = ? AND is_pending = ?", false, false).Find(&positions)

	now := time.Now()
	for _, pos := range positions {
		priceVal, ok := latestPriceCache.Load(pos.Symbol)
		if !ok {
			continue
		}
		currentPrice := priceVal.(float64)

		slPercent := config.TslPercent
		if pos.StopLossPercent != nil {
			slPercent = *pos.StopLossPercent
		}
		if slPercent <= 0 {
			continue
		}

		if currentPrice > pos.HighestPrice {
			pos.HighestPrice = currentPrice
		}

		if pos.StopLossType == "fixed" {
			pos.StopLossPrice = pos.AvgPrice * (1 - slPercent/100)
		} else {
			pos.StopLossPrice = pos.HighestPrice * (1 - slPercent/100)
		}

		if currentPrice <= pos.StopLossPrice && pos.StopLossPrice > 0 {
			sellPrice := currentPrice
			pnl := (sellPrice - pos.AvgPrice) * pos.Quantity
			pnlPct := ((sellPrice - pos.AvgPrice) / pos.AvgPrice) * 100

			sellTrade := FlipperBotTrade{
				Symbol:     pos.Symbol,
				Name:       pos.Name,
				Action:     "SELL",
				Quantity:   pos.Quantity,
				Price:      sellPrice,
				SignalDate: now,
				ExecutedAt: now,
				IsPending:  false,
				IsLive:     pos.IsLive,
				IsStopLoss: true,
			}
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct
			db.Create(&sellTrade)

			pos.IsClosed = true
			pos.SellPrice = sellPrice
			pos.SellDate = &now
			pos.ProfitLoss = &pnl
			pos.ProfitLossPct = &pnlPct
			pos.UpdatedAt = now
			db.Save(&pos)
			db.Where("user_id = ? AND symbol = ?", FLIPPERBOT_USER_ID, pos.Symbol).Delete(&PortfolioPosition{})

			fmt.Printf("[FLIPPER SL] %s Stop Loss ausgelöst bei $%.2f (SL: $%.2f, P/L: %.2f%%)\n", pos.Symbol, currentPrice, pos.StopLossPrice, pnlPct)
		} else {
			db.Save(&pos)
		}
	}
}

// checkLutzStopLoss checks all open Lutz positions against their stop loss
func checkLutzStopLoss() {
	var config BXtrenderConfig
	if err := db.Where("mode = ?", "aggressive").First(&config).Error; err != nil {
		return
	}
	if !config.TslEnabled {
		return
	}

	var positions []LutzPosition
	db.Where("is_closed = ? AND is_pending = ?", false, false).Find(&positions)

	now := time.Now()
	for _, pos := range positions {
		priceVal, ok := latestPriceCache.Load(pos.Symbol)
		if !ok {
			continue
		}
		currentPrice := priceVal.(float64)

		slPercent := config.TslPercent
		if pos.StopLossPercent != nil {
			slPercent = *pos.StopLossPercent
		}
		if slPercent <= 0 {
			continue
		}

		if currentPrice > pos.HighestPrice {
			pos.HighestPrice = currentPrice
		}

		if pos.StopLossType == "fixed" {
			pos.StopLossPrice = pos.AvgPrice * (1 - slPercent/100)
		} else {
			pos.StopLossPrice = pos.HighestPrice * (1 - slPercent/100)
		}

		if currentPrice <= pos.StopLossPrice && pos.StopLossPrice > 0 {
			sellPrice := currentPrice
			pnl := (sellPrice - pos.AvgPrice) * pos.Quantity
			pnlPct := ((sellPrice - pos.AvgPrice) / pos.AvgPrice) * 100

			sellTrade := LutzTrade{
				Symbol:     pos.Symbol,
				Name:       pos.Name,
				Action:     "SELL",
				Quantity:   pos.Quantity,
				Price:      sellPrice,
				SignalDate: now,
				ExecutedAt: now,
				IsPending:  false,
				IsLive:     pos.IsLive,
				IsStopLoss: true,
			}
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct
			db.Create(&sellTrade)

			pos.IsClosed = true
			pos.SellPrice = sellPrice
			pos.SellDate = &now
			pos.ProfitLoss = &pnl
			pos.ProfitLossPct = &pnlPct
			pos.UpdatedAt = now
			db.Save(&pos)
			db.Where("user_id = ? AND symbol = ?", LUTZ_USER_ID, pos.Symbol).Delete(&PortfolioPosition{})

			fmt.Printf("[LUTZ SL] %s Stop Loss ausgelöst bei $%.2f (SL: $%.2f, P/L: %.2f%%)\n", pos.Symbol, currentPrice, pos.StopLossPrice, pnlPct)
		} else {
			db.Save(&pos)
		}
	}
}

// checkQuantStopLoss checks all open Quant positions against their stop loss
func checkQuantStopLoss() {
	var config BXtrenderQuantConfig
	if err := db.First(&config).Error; err != nil {
		return
	}
	if !config.TslEnabled {
		return
	}

	var positions []QuantPosition
	db.Where("is_closed = ? AND is_pending = ?", false, false).Find(&positions)

	now := time.Now()
	for _, pos := range positions {
		priceVal, ok := latestPriceCache.Load(pos.Symbol)
		if !ok {
			continue
		}
		currentPrice := priceVal.(float64)

		slPercent := config.TslPercent
		if pos.StopLossPercent != nil {
			slPercent = *pos.StopLossPercent
		}
		if slPercent <= 0 {
			continue
		}

		if currentPrice > pos.HighestPrice {
			pos.HighestPrice = currentPrice
		}

		if pos.StopLossType == "fixed" {
			pos.StopLossPrice = pos.AvgPrice * (1 - slPercent/100)
		} else {
			pos.StopLossPrice = pos.HighestPrice * (1 - slPercent/100)
		}

		if currentPrice <= pos.StopLossPrice && pos.StopLossPrice > 0 {
			sellPrice := currentPrice
			pnl := (sellPrice - pos.AvgPrice) * pos.Quantity
			pnlPct := ((sellPrice - pos.AvgPrice) / pos.AvgPrice) * 100

			sellTrade := QuantTrade{
				Symbol:     pos.Symbol,
				Name:       pos.Name,
				Action:     "SELL",
				Quantity:   pos.Quantity,
				Price:      sellPrice,
				SignalDate: now,
				ExecutedAt: now,
				IsPending:  false,
				IsLive:     pos.IsLive,
				IsStopLoss: true,
			}
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct
			db.Create(&sellTrade)

			pos.IsClosed = true
			pos.SellPrice = sellPrice
			pos.SellDate = &now
			pos.ProfitLoss = &pnl
			pos.ProfitLossPct = &pnlPct
			pos.UpdatedAt = now
			db.Save(&pos)
			db.Where("user_id = ? AND symbol = ?", QUANT_USER_ID, pos.Symbol).Delete(&PortfolioPosition{})

			fmt.Printf("[QUANT SL] %s Stop Loss ausgelöst bei $%.2f (SL: $%.2f, P/L: %.2f%%)\n", pos.Symbol, currentPrice, pos.StopLossPrice, pnlPct)
		} else {
			db.Save(&pos)
		}
	}
}

// checkDitzStopLoss checks all open Ditz positions against their stop loss
func checkDitzStopLoss() {
	var config BXtrenderDitzConfig
	if err := db.First(&config).Error; err != nil {
		return
	}
	if !config.TslEnabled {
		return
	}

	var positions []DitzPosition
	db.Where("is_closed = ? AND is_pending = ?", false, false).Find(&positions)

	now := time.Now()
	for _, pos := range positions {
		priceVal, ok := latestPriceCache.Load(pos.Symbol)
		if !ok {
			continue
		}
		currentPrice := priceVal.(float64)

		slPercent := config.TslPercent
		if pos.StopLossPercent != nil {
			slPercent = *pos.StopLossPercent
		}
		if slPercent <= 0 {
			continue
		}

		if currentPrice > pos.HighestPrice {
			pos.HighestPrice = currentPrice
		}

		if pos.StopLossType == "fixed" {
			pos.StopLossPrice = pos.AvgPrice * (1 - slPercent/100)
		} else {
			pos.StopLossPrice = pos.HighestPrice * (1 - slPercent/100)
		}

		if currentPrice <= pos.StopLossPrice && pos.StopLossPrice > 0 {
			sellPrice := currentPrice
			pnl := (sellPrice - pos.AvgPrice) * pos.Quantity
			pnlPct := ((sellPrice - pos.AvgPrice) / pos.AvgPrice) * 100

			sellTrade := DitzTrade{
				Symbol:     pos.Symbol,
				Name:       pos.Name,
				Action:     "SELL",
				Quantity:   pos.Quantity,
				Price:      sellPrice,
				SignalDate: now,
				ExecutedAt: now,
				IsPending:  false,
				IsLive:     pos.IsLive,
				IsStopLoss: true,
			}
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct
			db.Create(&sellTrade)

			pos.IsClosed = true
			pos.SellPrice = sellPrice
			pos.SellDate = &now
			pos.ProfitLoss = &pnl
			pos.ProfitLossPct = &pnlPct
			pos.UpdatedAt = now
			db.Save(&pos)
			db.Where("user_id = ? AND symbol = ?", DITZ_USER_ID, pos.Symbol).Delete(&PortfolioPosition{})

			fmt.Printf("[DITZ SL] %s Stop Loss ausgelöst bei $%.2f (SL: $%.2f, P/L: %.2f%%)\n", pos.Symbol, currentPrice, pos.StopLossPrice, pnlPct)
		} else {
			db.Save(&pos)
		}
	}
}

// checkTraderStopLoss checks all open Trader positions against their stop loss
func checkTraderStopLoss() {
	var config BXtrenderTraderConfig
	if err := db.First(&config).Error; err != nil {
		return
	}
	if !config.TslEnabled {
		return
	}

	var positions []TraderPosition
	db.Where("is_closed = ? AND is_pending = ?", false, false).Find(&positions)

	now := time.Now()
	for _, pos := range positions {
		priceVal, ok := latestPriceCache.Load(pos.Symbol)
		if !ok {
			continue
		}
		currentPrice := priceVal.(float64)

		slPercent := config.TslPercent
		if pos.StopLossPercent != nil {
			slPercent = *pos.StopLossPercent
		}
		if slPercent <= 0 {
			continue
		}

		if currentPrice > pos.HighestPrice {
			pos.HighestPrice = currentPrice
		}

		if pos.StopLossType == "fixed" {
			pos.StopLossPrice = pos.AvgPrice * (1 - slPercent/100)
		} else {
			pos.StopLossPrice = pos.HighestPrice * (1 - slPercent/100)
		}

		if currentPrice <= pos.StopLossPrice && pos.StopLossPrice > 0 {
			sellPrice := currentPrice
			pnl := (sellPrice - pos.AvgPrice) * pos.Quantity
			pnlPct := ((sellPrice - pos.AvgPrice) / pos.AvgPrice) * 100

			sellTrade := TraderTrade{
				Symbol:     pos.Symbol,
				Name:       pos.Name,
				Action:     "SELL",
				Quantity:   pos.Quantity,
				Price:      sellPrice,
				SignalDate: now,
				ExecutedAt: now,
				IsPending:  false,
				IsLive:     pos.IsLive,
				IsStopLoss: true,
			}
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct
			db.Create(&sellTrade)

			pos.IsClosed = true
			pos.SellPrice = sellPrice
			pos.SellDate = &now
			pos.ProfitLoss = &pnl
			pos.ProfitLossPct = &pnlPct
			pos.UpdatedAt = now
			db.Save(&pos)
			db.Where("user_id = ? AND symbol = ?", TRADER_USER_ID, pos.Symbol).Delete(&PortfolioPosition{})

			fmt.Printf("[TRADER SL] %s Stop Loss ausgelöst bei $%.2f (SL: $%.2f, P/L: %.2f%%)\n", pos.Symbol, currentPrice, pos.StopLossPrice, pnlPct)
		} else {
			db.Save(&pos)
		}
	}
}

// runFlipperUpdateInternal performs the FlipperBot update without HTTP context
func runFlipperUpdateInternal(triggeredBy string) {
	checkFlipperStopLoss()

	now := time.Now()
	sessionID := uuid.New().String()

	var logs []map[string]interface{}
	addLog := func(level, msg string) {
		logs = append(logs, map[string]interface{}{"level": level, "message": msg, "time": time.Now().Format("15:04:05")})
		db.Create(&BotLog{Bot: "flipperbot", Level: level, Message: msg, SessionID: sessionID, CreatedAt: time.Now()})
	}

	addLog("INFO", fmt.Sprintf("FlipperBot Update gestartet um %s (von: %s)", now.Format("15:04:05"), triggeredBy))

	var flipperConfig BXtrenderConfig
	db.Where("mode = ?", "defensive").First(&flipperConfig)

	var perfData []StockPerformance
	if err := db.Find(&perfData).Error; err != nil {
		addLog("ERROR", fmt.Sprintf("Fehler beim Laden der Performance Daten: %v", err))
		return
	}

	addLog("INFO", fmt.Sprintf("%d Aktien geladen", len(perfData)))

	// Phase 1: Validate existing positions against current BXTrender data
	var existingPositions []FlipperBotPosition
	db.Where("is_live = ? AND is_closed = ?", false, false).Find(&existingPositions)

	for _, pos := range existingPositions {
		var stockPerf *StockPerformance
		for i := range perfData {
			if perfData[i].Symbol == pos.Symbol {
				stockPerf = &perfData[i]
				break
			}
		}

		if stockPerf == nil {
			addLog("WARN", fmt.Sprintf("%s: Position vorhanden aber keine Performance-Daten - überspringe Validierung", pos.Symbol))
			continue
		}

		if stockPerf.Signal == "NO_DATA" {
			addLog("SKIP", fmt.Sprintf("%s: Nicht genug Daten für Berechnung - überspringe", pos.Symbol))
			continue
		}

		if stockPerf.Signal == "SELL" || stockPerf.Signal == "WAIT" {
			addLog("KORREKTUR", fmt.Sprintf("%s: Signal ist jetzt %s, aber Position vorhanden - schließe Position", pos.Symbol, stockPerf.Signal))

			sellPrice := stockPerf.CurrentPrice
			sellDate := now

			sellTrade := FlipperBotTrade{
				Symbol:     pos.Symbol,
				Name:       pos.Name,
				Action:     "SELL",
				Quantity:   pos.Quantity,
				Price:      sellPrice,
				SignalDate: sellDate,
				ExecutedAt: sellDate,
				IsPending:  false,
				IsLive:     pos.IsLive,
			}
			pnl := (sellPrice - pos.AvgPrice) * pos.Quantity
			pnlPct := ((sellPrice - pos.AvgPrice) / pos.AvgPrice) * 100
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct

			db.Create(&sellTrade)

			pos.IsClosed = true
			pos.SellPrice = sellPrice
			pos.SellDate = &sellDate
			pos.ProfitLoss = &pnl
			pos.ProfitLossPct = &pnlPct
			pos.UpdatedAt = time.Now()
			db.Save(&pos)
			db.Where("user_id = ? AND symbol = ?", FLIPPERBOT_USER_ID, pos.Symbol).Delete(&PortfolioPosition{})

			addLog("KORREKTUR", fmt.Sprintf("%s: Position geschlossen @ $%.2f (P/L: %.2f%%)", pos.Symbol, sellPrice, pnlPct))
			continue
		}
	}

	// Phase 2: Process new signals (BUY/SELL)
	for _, stock := range perfData {
		if !isStockAllowedForBot("flipper", stock.Symbol) {
			continue
		}
		if stock.Signal == "BUY" {
			var existingPos FlipperBotPosition
			if err := db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error; err == nil {
				addLog("SKIP", fmt.Sprintf("%s: Position bereits vorhanden", stock.Symbol))
				continue
			}

			var deletedBuy FlipperBotTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ?", stock.Symbol, "BUY", true).Order("executed_at desc").First(&deletedBuy).Error; err == nil {
				var sellAfterDeleted FlipperBotTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, deletedBuy.ExecutedAt).First(&sellAfterDeleted).Error; err != nil {
					addLog("SKIP", fmt.Sprintf("%s: Soft-deleted BUY vorhanden - überspringe", stock.Symbol))
					continue
				}
			}

			var existingBuy FlipperBotTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND is_filter_blocked = ?", stock.Symbol, "BUY", false, false).Order("executed_at desc").First(&existingBuy).Error; err == nil {
				var sellAfter FlipperBotTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, existingBuy.ExecutedAt).First(&sellAfter).Error; err != nil {
					addLog("SKIP", fmt.Sprintf("%s: Bereits gekauft am %s", stock.Symbol, existingBuy.ExecutedAt.Format("02.01.2006")))
					continue
				}
			}

			// Check if there's already a filter-blocked BUY (don't create duplicates)
			var blockedBuy FlipperBotTrade
			if err := db.Where("symbol = ? AND action = ? AND is_filter_blocked = ? AND is_deleted = ?", stock.Symbol, "BUY", true, false).Order("executed_at desc").First(&blockedBuy).Error; err == nil {
				var sellAfterBlocked FlipperBotTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, blockedBuy.ExecutedAt).First(&sellAfterBlocked).Error; err != nil {
					continue
				}
			}

			// Use current price and today's date (no retroactive trades)
			signalPrice := stock.CurrentPrice
			signalDate := now

			investmentEUR := 100.0
			investmentUSD := convertToUSD(investmentEUR, "EUR")
			qty := math.Round((investmentUSD/signalPrice)*1000000) / 1000000
			if qty <= 0 {
				addLog("SKIP", fmt.Sprintf("%s: Ungültige Menge berechnet", stock.Symbol))
				continue
			}

			// Check bot filter config
			filterBlocked, filterReason := checkBotFilterConfig("flipper", stock.WinRate, stock.RiskReward, stock.AvgReturn, stock.MarketCap)
			if filterBlocked {
				blockedTrade := FlipperBotTrade{
					Symbol:            stock.Symbol,
					Name:              stock.Name,
					Action:            "BUY",
					Quantity:          qty,
					Price:             signalPrice,
					SignalDate:        signalDate,
					ExecutedAt:        signalDate,
					IsPending:         false,
					IsLive:            false,
					IsFilterBlocked:   true,
					FilterBlockReason: filterReason,
				}
				db.Create(&blockedTrade)
				addLog("FILTER", fmt.Sprintf("%s: BUY blockiert durch Filter (%s)", stock.Symbol, filterReason))
				continue
			}

			buyTrade := FlipperBotTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      signalPrice,
				SignalDate: signalDate,
				ExecutedAt: signalDate,
				IsPending:  false,
				IsLive:     false,
			}
			db.Create(&buyTrade)

			newPos := FlipperBotPosition{
				Symbol:        stock.Symbol,
				Name:          stock.Name,
				Quantity:      qty,
				AvgPrice:      signalPrice,
				InvestedEUR:   investmentEUR,
				BuyDate:       signalDate,
				IsPending:     false,
				IsLive:        false,
				HighestPrice:  signalPrice,
				StopLossPrice: signalPrice * (1 - flipperConfig.TslPercent/100),
				StopLossType:  "trailing",
			}
			db.Create(&newPos)

			portfolioPos := PortfolioPosition{
				UserID:       FLIPPERBOT_USER_ID,
				Symbol:       stock.Symbol,
				Name:         stock.Name,
				PurchaseDate: &signalDate,
				AvgPrice:     signalPrice,
				Currency:     "USD",
				Quantity:     &qty,
			}
			db.Create(&portfolioPos)

			addLog("ACTION", fmt.Sprintf("BUY ausgeführt: %s %.6f @ $%.2f (Signal: %s)", stock.Symbol, qty, signalPrice, signalDate.Format("02.01.2006")))

		} else if stock.Signal == "SELL" {
			var deletedSell FlipperBotTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ?", stock.Symbol, "SELL", true).Order("executed_at desc").First(&deletedSell).Error; err == nil {
				addLog("SKIP", fmt.Sprintf("%s: Soft-deleted SELL vorhanden - überspringe", stock.Symbol))
				continue
			}

			var existingPos FlipperBotPosition
			if err := db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error; err != nil {
				addLog("SKIP", fmt.Sprintf("%s: SELL Signal aber keine offene Position", stock.Symbol))
				continue
			}

			// Use current price and today's date (no retroactive trades)
			sellPrice := stock.CurrentPrice
			sellDate := now

			sellTrade := FlipperBotTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "SELL",
				Quantity:   existingPos.Quantity,
				Price:      sellPrice,
				SignalDate: sellDate,
				ExecutedAt: sellDate,
				IsPending:  false,
				IsLive:     existingPos.IsLive,
			}

			pnl := (sellPrice - existingPos.AvgPrice) * existingPos.Quantity
			pnlPct := ((sellPrice - existingPos.AvgPrice) / existingPos.AvgPrice) * 100
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct

			db.Create(&sellTrade)

			existingPos.IsClosed = true
			existingPos.SellPrice = sellPrice
			existingPos.SellDate = &sellDate
			existingPos.ProfitLoss = &pnl
			existingPos.ProfitLossPct = &pnlPct
			existingPos.UpdatedAt = time.Now()
			db.Save(&existingPos)
			db.Where("user_id = ? AND symbol = ?", FLIPPERBOT_USER_ID, stock.Symbol).Delete(&PortfolioPosition{})

			addLog("ACTION", fmt.Sprintf("SELL ausgeführt: %s @ $%.2f (Signal: %s, P/L: %.2f%%)", stock.Symbol, sellPrice, sellDate.Format("02.01.2006"), pnlPct))
		}
	}

	addLog("INFO", "FlipperBot Update abgeschlossen")

	lastRefresh := map[string]interface{}{
		"updated_at":   now,
		"triggered_by": triggeredBy,
		"logs":         logs,
	}
	lastRefreshJSON, _ := json.Marshal(lastRefresh)

	var setting SystemSetting
	if err := db.Where("key = ?", "last_flipper_refresh").First(&setting).Error; err != nil {
		setting = SystemSetting{
			Key:       "last_flipper_refresh",
			Value:     string(lastRefreshJSON),
			UpdatedAt: now,
		}
		db.Create(&setting)
	} else {
		setting.Value = string(lastRefreshJSON)
		setting.UpdatedAt = now
		db.Save(&setting)
	}
}

func flipperBotUpdate(c *gin.Context) {
	triggeredBy := "system"
	if userID, exists := c.Get("userID"); exists {
		var user User
		if err := db.First(&user, userID).Error; err == nil {
			triggeredBy = user.Username
		}
	}

	runFlipperUpdateInternal(triggeredBy)

	c.JSON(http.StatusOK, gin.H{
		"message": "FlipperBot update completed",
	})
}

// flipperBotBackfill allows admin to create retroactive trades from a specified date until today
// This uses the historical trade data stored in StockPerformance.TradesJSON
func flipperBotBackfill(c *gin.Context) {
	var req struct {
		UntilDate string `json:"until_date"` // Format: 2026-01-15 - this is actually the START date
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "until_date required"})
		return
	}

	fromDate, err := time.Parse("2006-01-02", req.UntilDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format (use YYYY-MM-DD)"})
		return
	}

	now := time.Now()
	sessionID := uuid.New().String()
	var logs []map[string]interface{}
	addLog := func(level, message string) {
		entry := map[string]interface{}{
			"level":   level,
			"message": message,
			"time":    time.Now().Format("15:04:05"),
		}
		logs = append(logs, entry)
		saveBotLog("flipperbot", level, message, sessionID)
	}

	addLog("INFO", fmt.Sprintf("Backfill gestartet ab %s bis heute", req.UntilDate))

	// Get all tracked stocks with their performance data
	var trackedStocks []StockPerformance
	db.Find(&trackedStocks)

	if len(trackedStocks) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message": "No tracked stocks found",
			"trades_created": 0,
			"logs": logs,
		})
		return
	}

	var tradesCreated int
	var positionsCreated int

	for _, stock := range trackedStocks {
		if stock.TradesJSON == "" {
			continue
		}

		// Check allowlist
		if !isStockAllowedForBot("flipper", stock.Symbol) {
			addLog("SKIP", fmt.Sprintf("%s: Nicht in Allowlist — übersprungen", stock.Symbol))
			continue
		}

		// Parse the historical trades from TradesJSON
		var historicalTrades []TradeData
		if err := json.Unmarshal([]byte(stock.TradesJSON), &historicalTrades); err != nil {
			addLog("ERROR", fmt.Sprintf("%s: Fehler beim Parsen der Trades: %v", stock.Symbol, err))
			continue
		}

		// Check if there's already an open position from BEFORE the backfill start date
		// If so, the stock is in HOLD status and we should not open a new position
		hasOpenPositionBefore := false
		for _, t := range historicalTrades {
			entryT := time.Unix(t.EntryDate, 0)
			if t.IsOpen && entryT.Before(fromDate) {
				hasOpenPositionBefore = true
				break
			}
		}
		if hasOpenPositionBefore {
			addLog("SKIP", fmt.Sprintf("%s: Offene Position vor Startdatum (HOLD) — übersprungen", stock.Symbol))
			continue
		}

		for _, trade := range historicalTrades {
			// Convert entryDate from seconds to time (timestamps are in seconds, not milliseconds)
			entryTime := time.Unix(trade.EntryDate, 0)

			// Sanity check: skip invalid dates (before 2020 or after 2030)
			if entryTime.Year() < 2020 || entryTime.Year() > 2030 {
				continue
			}

			// Skip trades that are before the from_date (user selected start date)
			if entryTime.Before(fromDate) {
				continue
			}

			// Skip trades in the future
			if entryTime.After(now) {
				continue
			}

			// Check if we already have a buy trade for this date
			var existingBuy FlipperBotTrade
			dateStart := entryTime.Truncate(24 * time.Hour)
			dateEnd := dateStart.Add(24 * time.Hour)
			alreadyExists := db.Where("symbol = ? AND action = ? AND signal_date >= ? AND signal_date < ?",
				stock.Symbol, "BUY", dateStart, dateEnd).First(&existingBuy).Error == nil
			if alreadyExists {
				continue
			}

			// Calculate quantity: invest 100 EUR worth
			investmentEUR := 100.0
			investmentUSD := convertToUSD(investmentEUR, "EUR")
			qty := math.Round((investmentUSD/trade.EntryPrice)*1000000) / 1000000
			if qty <= 0 || trade.EntryPrice <= 0 {
				continue
			}

			// Create BUY trade
			buyTrade := FlipperBotTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      trade.EntryPrice,
				SignalDate: entryTime,
				ExecutedAt: now,
			}
			db.Create(&buyTrade)
			tradesCreated++
			addLog("ACTION", fmt.Sprintf("%s: BUY erstellt @ $%.2f am %s", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))

			// Handle exit (SELL) if exists and is not in the future
			if trade.ExitDate != nil && trade.ExitPrice != nil {
				exitTime := time.Unix(*trade.ExitDate, 0)

				if !exitTime.After(now) {
					// Calculate profit/loss
					profitLoss := (*trade.ExitPrice - trade.EntryPrice) * qty
					profitLossPct := trade.ReturnPct

					// Create SELL trade
					sellTrade := FlipperBotTrade{
						Symbol:        stock.Symbol,
						Name:          stock.Name,
						Action:        "SELL",
						Quantity:      qty,
						Price:         *trade.ExitPrice,
						SignalDate:    exitTime,
						ExecutedAt:    now,
						ProfitLoss:    &profitLoss,
						ProfitLossPct: &profitLossPct,
					}
					db.Create(&sellTrade)
					tradesCreated++
					addLog("ACTION", fmt.Sprintf("%s: SELL erstellt @ $%.2f am %s (%.2f%%)", stock.Symbol, *trade.ExitPrice, exitTime.Format("2006-01-02"), profitLossPct))
				} else {
					// Exit is in the future - create open position
					var existingPos FlipperBotPosition
					if db.Where("symbol = ?", stock.Symbol).First(&existingPos).Error != nil {
						newPos := FlipperBotPosition{
							Symbol:      stock.Symbol,
							Name:        stock.Name,
							Quantity:    qty,
							AvgPrice:    trade.EntryPrice,
							InvestedEUR: investmentEUR,
							BuyDate:     entryTime,
						}
						db.Create(&newPos)
						positionsCreated++

						// Add to portfolio comparison
						portfolioPos := PortfolioPosition{
							UserID:       FLIPPERBOT_USER_ID,
							Symbol:       stock.Symbol,
							Name:         stock.Name,
							PurchaseDate: &entryTime,
							AvgPrice:     trade.EntryPrice,
							Currency:     "USD",
							Quantity:     &qty,
						}
						db.Create(&portfolioPos)
						addLog("ACTION", fmt.Sprintf("%s: Position erstellt (offen)", stock.Symbol))
					}
				}
			} else if trade.IsOpen {
				// Trade is open with no exit - create position
				var existingPos FlipperBotPosition
				if db.Where("symbol = ?", stock.Symbol).First(&existingPos).Error != nil {
					newPos := FlipperBotPosition{
						Symbol:      stock.Symbol,
						Name:        stock.Name,
						Quantity:    qty,
						AvgPrice:    trade.EntryPrice,
						InvestedEUR: investmentEUR,
						BuyDate:     entryTime,
					}
					db.Create(&newPos)
					positionsCreated++

					// Add to portfolio comparison
					portfolioPos := PortfolioPosition{
						UserID:       FLIPPERBOT_USER_ID,
						Symbol:       stock.Symbol,
						Name:         stock.Name,
						PurchaseDate: &entryTime,
						AvgPrice:     trade.EntryPrice,
						Currency:     "USD",
						Quantity:     &qty,
					}
					db.Create(&portfolioPos)
					addLog("ACTION", fmt.Sprintf("%s: Position erstellt (offen)", stock.Symbol))
				}
			}
		}
	}

	addLog("INFO", fmt.Sprintf("Backfill abgeschlossen: %d Trades, %d Positionen erstellt", tradesCreated, positionsCreated))

	c.JSON(http.StatusOK, gin.H{
		"message":           "Backfill completed",
		"trades_created":    tradesCreated,
		"positions_created": positionsCreated,
		"until_date":        req.UntilDate,
		"logs":              logs,
	})
}

// lutzBackfill allows admin to create retroactive trades for Lutz (aggressive mode) from a specified date until today
// This uses the historical trade data stored in AggressiveStockPerformance.TradesJSON
func lutzBackfill(c *gin.Context) {
	var req struct {
		UntilDate string `json:"until_date"` // Format: 2026-01-15 - this is actually the START date
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "until_date required"})
		return
	}

	fromDate, err := time.Parse("2006-01-02", req.UntilDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format (use YYYY-MM-DD)"})
		return
	}

	now := time.Now()
	sessionID := uuid.New().String()
	var logs []map[string]interface{}
	addLog := func(level, message string) {
		entry := map[string]interface{}{
			"level":   level,
			"message": message,
			"time":    time.Now().Format("15:04:05"),
		}
		logs = append(logs, entry)
		saveBotLog("lutz", level, message, sessionID)
	}

	addLog("INFO", fmt.Sprintf("Lutz Backfill gestartet ab %s bis heute", req.UntilDate))

	// Get all tracked stocks with their aggressive performance data
	var trackedStocks []AggressiveStockPerformance
	db.Find(&trackedStocks)

	if len(trackedStocks) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message":        "No tracked stocks found",
			"trades_created": 0,
			"logs":           logs,
		})
		return
	}

	var tradesCreated int
	var positionsCreated int

	for _, stock := range trackedStocks {
		if stock.TradesJSON == "" {
			continue
		}

		// Check allowlist
		if !isStockAllowedForBot("lutz", stock.Symbol) {
			addLog("SKIP", fmt.Sprintf("%s: Nicht in Allowlist — übersprungen", stock.Symbol))
			continue
		}

		// Parse the historical trades from TradesJSON
		var historicalTrades []TradeData
		if err := json.Unmarshal([]byte(stock.TradesJSON), &historicalTrades); err != nil {
			addLog("ERROR", fmt.Sprintf("%s: Fehler beim Parsen der Trades: %v", stock.Symbol, err))
			continue
		}

		// Check if there's already an open position from BEFORE the backfill start date
		hasOpenPositionBefore := false
		for _, t := range historicalTrades {
			entryT := time.Unix(t.EntryDate, 0)
			if t.IsOpen && entryT.Before(fromDate) {
				hasOpenPositionBefore = true
				break
			}
		}
		if hasOpenPositionBefore {
			addLog("SKIP", fmt.Sprintf("%s: Offene Position vor Startdatum (HOLD) — übersprungen", stock.Symbol))
			continue
		}

		for _, trade := range historicalTrades {
			// Convert entryDate from seconds to time
			entryTime := time.Unix(trade.EntryDate, 0)

			// Sanity check: skip invalid dates (before 2020 or after 2030)
			if entryTime.Year() < 2020 || entryTime.Year() > 2030 {
				continue
			}

			// Skip trades that are before the from_date (user selected start date)
			if entryTime.Before(fromDate) {
				continue
			}

			// Skip trades in the future
			if entryTime.After(now) {
				continue
			}

			// Check if we already have a buy trade for this date
			var existingBuy LutzTrade
			dateStart := entryTime.Truncate(24 * time.Hour)
			dateEnd := dateStart.Add(24 * time.Hour)
			alreadyExists := db.Where("symbol = ? AND action = ? AND signal_date >= ? AND signal_date < ?",
				stock.Symbol, "BUY", dateStart, dateEnd).First(&existingBuy).Error == nil
			if alreadyExists {
				continue
			}

			// Calculate quantity: invest 100 EUR worth
			investmentEUR := 100.0
			investmentUSD := convertToUSD(investmentEUR, "EUR")
			qty := math.Round((investmentUSD/trade.EntryPrice)*1000000) / 1000000
			if qty <= 0 || trade.EntryPrice <= 0 {
				continue
			}

			// Create BUY trade
			buyTrade := LutzTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      trade.EntryPrice,
				SignalDate: entryTime,
				ExecutedAt: now,
			}
			db.Create(&buyTrade)
			tradesCreated++
			addLog("ACTION", fmt.Sprintf("%s: BUY erstellt @ $%.2f am %s", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))

			// Handle exit (SELL) if exists and is not in the future
			if trade.ExitDate != nil && trade.ExitPrice != nil {
				exitTime := time.Unix(*trade.ExitDate, 0)

				if !exitTime.After(now) {
					// Calculate profit/loss
					profitLoss := (*trade.ExitPrice - trade.EntryPrice) * qty
					profitLossPct := trade.ReturnPct

					// Create SELL trade
					sellTrade := LutzTrade{
						Symbol:        stock.Symbol,
						Name:          stock.Name,
						Action:        "SELL",
						Quantity:      qty,
						Price:         *trade.ExitPrice,
						SignalDate:    exitTime,
						ExecutedAt:    now,
						ProfitLoss:    &profitLoss,
						ProfitLossPct: &profitLossPct,
					}
					db.Create(&sellTrade)
					tradesCreated++
					addLog("ACTION", fmt.Sprintf("%s: SELL erstellt @ $%.2f am %s (%.2f%%)", stock.Symbol, *trade.ExitPrice, exitTime.Format("2006-01-02"), profitLossPct))
				} else {
					// Exit is in the future - create open position
					var existingPos LutzPosition
					if db.Where("symbol = ?", stock.Symbol).First(&existingPos).Error != nil {
						newPos := LutzPosition{
							Symbol:      stock.Symbol,
							Name:        stock.Name,
							Quantity:    qty,
							AvgPrice:    trade.EntryPrice,
							InvestedEUR: investmentEUR,
							BuyDate:     entryTime,
						}
						db.Create(&newPos)
						positionsCreated++

						// Add to portfolio comparison
						portfolioPos := PortfolioPosition{
							UserID:       LUTZ_USER_ID,
							Symbol:       stock.Symbol,
							Name:         stock.Name,
							PurchaseDate: &entryTime,
							AvgPrice:     trade.EntryPrice,
							Currency:     "USD",
							Quantity:     &qty,
						}
						db.Create(&portfolioPos)
						addLog("ACTION", fmt.Sprintf("%s: Position erstellt (offen)", stock.Symbol))
					}
				}
			} else if trade.IsOpen {
				// Trade is open with no exit - create position
				var existingPos LutzPosition
				if db.Where("symbol = ?", stock.Symbol).First(&existingPos).Error != nil {
					newPos := LutzPosition{
						Symbol:      stock.Symbol,
						Name:        stock.Name,
						Quantity:    qty,
						AvgPrice:    trade.EntryPrice,
						InvestedEUR: investmentEUR,
						BuyDate:     entryTime,
					}
					db.Create(&newPos)
					positionsCreated++

					// Add to portfolio comparison
					portfolioPos := PortfolioPosition{
						UserID:       LUTZ_USER_ID,
						Symbol:       stock.Symbol,
						Name:         stock.Name,
						PurchaseDate: &entryTime,
						AvgPrice:     trade.EntryPrice,
						Currency:     "USD",
						Quantity:     &qty,
					}
					db.Create(&portfolioPos)
					addLog("ACTION", fmt.Sprintf("%s: Position erstellt (offen)", stock.Symbol))
				}
			}
		}
	}

	addLog("INFO", fmt.Sprintf("Lutz Backfill abgeschlossen: %d Trades, %d Positionen erstellt", tradesCreated, positionsCreated))

	c.JSON(http.StatusOK, gin.H{
		"message":           "Backfill completed",
		"trades_created":    tradesCreated,
		"positions_created": positionsCreated,
		"until_date":        req.UntilDate,
		"logs":              logs,
	})
}

// getFlipperBotPendingTrades returns all FlipperBot trades that are pending admin approval
func getFlipperBotPendingTrades(c *gin.Context) {
	var trades []FlipperBotTrade
	db.Where("is_pending = ?", true).Order("signal_date desc").Find(&trades)
	c.JSON(http.StatusOK, trades)
}

// acceptFlipperBotTrade accepts a pending FlipperBot trade (sets is_pending = false)
func acceptFlipperBotTrade(c *gin.Context) {
	id := c.Param("id")
	var trade FlipperBotTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	trade.IsPending = false
	db.Save(&trade)

	// If this is a BUY trade, also accept the corresponding position
	if trade.Action == "BUY" {
		db.Model(&FlipperBotPosition{}).Where("symbol = ? AND is_pending = ?", trade.Symbol, true).Update("is_pending", false)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Trade accepted", "trade": trade})
}

// getLutzPendingTrades returns all Lutz trades that are pending admin approval
func getLutzPendingTrades(c *gin.Context) {
	var trades []LutzTrade
	db.Where("is_pending = ?", true).Order("signal_date desc").Find(&trades)
	c.JSON(http.StatusOK, trades)
}

// acceptLutzTrade accepts a pending Lutz trade (sets is_pending = false)
func acceptLutzTrade(c *gin.Context) {
	id := c.Param("id")
	var trade LutzTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	trade.IsPending = false
	db.Save(&trade)

	// If this is a BUY trade, also accept the corresponding position
	if trade.Action == "BUY" {
		db.Model(&LutzPosition{}).Where("symbol = ? AND is_pending = ?", trade.Symbol, true).Update("is_pending", false)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Trade accepted", "trade": trade})
}

func getFlipperBotPortfolio(c *gin.Context) {
	var positions []FlipperBotPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Order("buy_date desc").Find(&positions)

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
	var trades []FlipperBotTrade
	db.Where("is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false).Order("signal_date desc, executed_at desc").Limit(50).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getFlipperBotActionsAll(c *gin.Context) {
	var trades []FlipperBotTrade
	db.Where("is_pending = ?", false).Order("signal_date desc").Limit(100).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getFlipperBotPerformance(c *gin.Context) {
	var sellTrades []FlipperBotTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false, false).Find(&sellTrades)

	var buyTrades []FlipperBotTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "BUY", false, false, false).Find(&buyTrades)

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

	// Calculate average return per closed trade
	totalReturnPctClosed := 0.0
	for _, trade := range sellTrades {
		if trade.ProfitLossPct != nil {
			totalReturnPctClosed += *trade.ProfitLossPct
		}
	}
	avgReturnPerTrade := 0.0
	if len(sellTrades) > 0 {
		avgReturnPerTrade = totalReturnPctClosed / float64(len(sellTrades))
	}

	// Calculate current unrealized gains - only open, non-deleted positions
	var positions []FlipperBotPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Find(&positions)

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

	// Calculate overall performance (including realized)
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
		"realized_return_pct":   totalReturnPctClosed,
		"avg_return_per_trade":  avgReturnPerTrade,
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

func getFlipperBotSimulatedPortfolio(c *gin.Context) {
	var positions []FlipperBotPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Order("buy_date desc").Find(&positions)

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
		InvestedEUR    float64   `json:"invested_eur"`
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
			InvestedEUR:    pos.InvestedEUR,
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

	var closedSellTrades []FlipperBotTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ?", "SELL", false, false).Find(&closedSellTrades)

	realizedPL := 0.0
	totalClosedInvested := 0.0
	for _, trade := range closedSellTrades {
		if trade.ProfitLoss != nil {
			realizedPL += *trade.ProfitLoss
			totalClosedInvested += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}

	overallReturn := totalReturn + realizedPL
	overallInvested := totalInvested + totalClosedInvested
	overallReturnPct := 0.0
	if overallInvested > 0 {
		overallReturnPct = (overallReturn / overallInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"positions":          result,
		"total_value":        totalValue,
		"total_invested":     totalInvested,
		"total_return":       totalReturn,
		"total_return_pct":   overallReturnPct,
		"realized_pl":        realizedPL,
		"overall_return":     overallReturn,
		"overall_invested":   overallInvested,
	})
}

func getFlipperBotSimulatedPerformance(c *gin.Context) {
	var sellTrades []FlipperBotTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_admin_closed = ? AND is_filter_blocked = ?", "SELL", false, false, false, false).Find(&sellTrades)

	var buyTrades []FlipperBotTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_admin_closed = ? AND is_filter_blocked = ?", "BUY", false, false, false, false).Find(&buyTrades)

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

	totalReturnPctClosed := 0.0
	for _, trade := range sellTrades {
		if trade.ProfitLossPct != nil {
			totalReturnPctClosed += *trade.ProfitLossPct
		}
	}
	avgReturnPerTrade := 0.0
	if len(sellTrades) > 0 {
		avgReturnPerTrade = totalReturnPctClosed / float64(len(sellTrades))
	}

	var positions []FlipperBotPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	unrealizedGain := 0.0
	investedInPositions := 0.0
	currentValue := 0.0

	for _, pos := range positions {
		quote := quotes[pos.Symbol]
		currentPrice := quote.Price
		if currentPrice <= 0 {
			currentPrice = pos.AvgPrice
		}
		investedInPositions += pos.AvgPrice * pos.Quantity
		currentValue += currentPrice * pos.Quantity
		unrealizedGain += (currentPrice - pos.AvgPrice) * pos.Quantity
	}

	totalReturnPct := 0.0
	if investedInPositions > 0 {
		totalReturnPct = (unrealizedGain / investedInPositions) * 100
	}

	totalGain := totalProfitLoss + unrealizedGain
	totalInvestedAll := investedInPositions
	for _, trade := range sellTrades {
		if trade.ProfitLoss != nil {
			totalInvestedAll += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}
	overallReturnPct := 0.0
	if totalInvestedAll > 0 {
		overallReturnPct = (totalGain / totalInvestedAll) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_trades":          len(buyTrades) + len(sellTrades),
		"total_buys":            len(buyTrades),
		"open_positions":        len(positions),
		"closed_trades":         len(sellTrades),
		"wins":                  wins,
		"losses":                losses,
		"win_rate":              winRate,
		"realized_profit":       totalProfitLoss,
		"avg_return_per_trade":  avgReturnPerTrade,
		"unrealized_gain":       unrealizedGain,
		"invested_in_positions": investedInPositions,
		"current_value":         currentValue,
		"total_gain":            totalGain,
		"total_return_pct":      totalReturnPct,
		"overall_return_pct":    overallReturnPct,
	})
}

func toggleFlipperTradeRead(c *gin.Context) {
	id := c.Param("id")
	var trade FlipperBotTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}
	trade.IsRead = !trade.IsRead
	db.Save(&trade)
	c.JSON(http.StatusOK, gin.H{"trade": trade})
}

func markAllFlipperTradesRead(c *gin.Context) {
	db.Model(&FlipperBotTrade{}).Where("is_read = ? AND is_pending = ?", false, false).Update("is_read", true)
	c.JSON(http.StatusOK, gin.H{"message": "All trades marked as read"})
}

func markAllFlipperTradesUnread(c *gin.Context) {
	db.Model(&FlipperBotTrade{}).Where("is_read = ? AND is_pending = ?", true, false).Update("is_read", false)
	c.JSON(http.StatusOK, gin.H{"message": "All trades marked as unread"})
}

func getFlipperUnreadCount(c *gin.Context) {
	var count int64
	db.Model(&FlipperBotTrade{}).Where("is_read = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false, false).Count(&count)

	var unreadTrades []FlipperBotTrade
	db.Where("is_read = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false, false).Order("executed_at desc").Limit(10).Find(&unreadTrades)

	c.JSON(http.StatusOK, gin.H{"count": count, "trades": unreadTrades})
}

func cleanupFlipperPending(c *gin.Context) {
	db.Where("is_pending = ?", true).Delete(&FlipperBotTrade{})
	db.Where("is_pending = ?", true).Delete(&FlipperBotPosition{})
	c.JSON(http.StatusOK, gin.H{"message": "Pending trades cleaned up"})
}

func getLastFlipperRefresh(c *gin.Context) {
	var setting SystemSetting
	if err := db.Where("key = ?", "last_flipper_refresh").First(&setting).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"updated_at":   nil,
			"triggered_by": nil,
			"logs":         []interface{}{},
		})
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(setting.Value), &result); err != nil {
		c.JSON(http.StatusOK, gin.H{"updated_at": setting.UpdatedAt})
		return
	}
	c.JSON(http.StatusOK, result)
}

// getFlipperBotHistory returns historical performance data for the FlipperBot chart
func getFlipperBotHistory(c *gin.Context) {
	period := c.DefaultQuery("period", "1m")
	live := c.DefaultQuery("live", "true")
	botType := "flipperbot-live"
	if live == "false" {
		botType = "flipperbot-sim"
	}
	history := calculateBotHistory(botType, period)
	c.JSON(http.StatusOK, history)
}

// getLutzHistory returns historical performance data for the Lutz bot chart
func getLutzHistory(c *gin.Context) {
	period := c.DefaultQuery("period", "1m")
	live := c.DefaultQuery("live", "true")
	botType := "lutz-live"
	if live == "false" {
		botType = "lutz-sim"
	}
	history := calculateBotHistory(botType, period)
	c.JSON(http.StatusOK, history)
}

// calculateBotHistory calculates historical performance for a bot
func calculateBotHistory(botType string, period string) []map[string]interface{} {
	// Get positions based on bot type
	var symbols []string
	var positionData []struct {
		Symbol   string
		Quantity float64
		AvgPrice float64
		BuyDate  time.Time
	}

	if botType == "flipperbot-live" || botType == "flipperbot-sim" {
		isLive := botType == "flipperbot-live"
		var positions []FlipperBotPosition
		db.Where("is_pending = ? AND is_live = ? AND is_closed = ?", false, isLive, false).Find(&positions)
		for _, p := range positions {
			symbols = append(symbols, p.Symbol)
			positionData = append(positionData, struct {
				Symbol   string
				Quantity float64
				AvgPrice float64
				BuyDate  time.Time
			}{p.Symbol, p.Quantity, p.AvgPrice, p.BuyDate})
		}
	} else if botType == "quant-live" || botType == "quant-sim" {
		isLive := botType == "quant-live"
		var positions []QuantPosition
		db.Where("is_pending = ? AND is_live = ? AND is_closed = ?", false, isLive, false).Find(&positions)
		for _, p := range positions {
			symbols = append(symbols, p.Symbol)
			positionData = append(positionData, struct {
				Symbol   string
				Quantity float64
				AvgPrice float64
				BuyDate  time.Time
			}{p.Symbol, p.Quantity, p.AvgPrice, p.BuyDate})
		}
	} else if botType == "ditz-live" || botType == "ditz-sim" {
		isLive := botType == "ditz-live"
		var positions []DitzPosition
		db.Where("is_pending = ? AND is_live = ? AND is_closed = ?", false, isLive, false).Find(&positions)
		for _, p := range positions {
			symbols = append(symbols, p.Symbol)
			positionData = append(positionData, struct {
				Symbol   string
				Quantity float64
				AvgPrice float64
				BuyDate  time.Time
			}{p.Symbol, p.Quantity, p.AvgPrice, p.BuyDate})
		}
	} else if botType == "trader-live" || botType == "trader-sim" {
		isLive := botType == "trader-live"
		var positions []TraderPosition
		db.Where("is_pending = ? AND is_live = ? AND is_closed = ?", false, isLive, false).Find(&positions)
		for _, p := range positions {
			symbols = append(symbols, p.Symbol)
			positionData = append(positionData, struct {
				Symbol   string
				Quantity float64
				AvgPrice float64
				BuyDate  time.Time
			}{p.Symbol, p.Quantity, p.AvgPrice, p.BuyDate})
		}
	} else if botType == "lutz-live" || botType == "lutz-sim" {
		isLive := botType == "lutz-live"
		var positions []LutzPosition
		db.Where("is_pending = ? AND is_live = ? AND is_closed = ?", false, isLive, false).Find(&positions)
		for _, p := range positions {
			symbols = append(symbols, p.Symbol)
			positionData = append(positionData, struct {
				Symbol   string
				Quantity float64
				AvgPrice float64
				BuyDate  time.Time
			}{p.Symbol, p.Quantity, p.AvgPrice, p.BuyDate})
		}
	}

	if len(symbols) == 0 {
		return []map[string]interface{}{}
	}

	// Map period to Yahoo Finance range
	yahooRange := "1mo"
	switch period {
	case "1d":
		yahooRange = "5d"
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

	// Fetch historical data for all symbols in parallel
	symbolData := make(map[string][]OHLCV)
	var fetchMu sync.Mutex
	var fetchWg sync.WaitGroup
	for _, symbol := range symbols {
		fetchWg.Add(1)
		go func(sym string) {
			defer fetchWg.Done()
			data := fetchHistoricalData(sym, yahooRange)
			if len(data) > 0 {
				fetchMu.Lock()
				symbolData[sym] = data
				fetchMu.Unlock()
			}
		}(symbol)
	}
	fetchWg.Wait()

	if len(symbolData) == 0 {
		return []map[string]interface{}{}
	}

	// Collect all timestamps
	var allTimes []int64
	timeValues := make(map[int64]map[string]float64)

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

	if len(allTimes) == 0 {
		return []map[string]interface{}{}
	}

	// Only include positions that existed BEFORE the period start
	periodStart := allTimes[0]
	var activePositions []struct {
		Symbol   string
		Quantity float64
		AvgPrice float64
		BuyDate  time.Time
	}
	for _, p := range positionData {
		if p.BuyDate.IsZero() || p.BuyDate.Unix() <= periodStart {
			activePositions = append(activePositions, p)
		}
	}

	// Fallback: if no positions existed before the period, use all
	if len(activePositions) == 0 {
		activePositions = positionData
	}

	// Track last known prices
	lastPrices := make(map[string]float64)
	result := make([]map[string]interface{}, 0)

	// Pre-fill with first available price so ALL active positions contribute from the start
	for _, p := range activePositions {
		if data, ok := symbolData[p.Symbol]; ok && len(data) > 0 {
			lastPrices[p.Symbol] = data[0].Close
		}
	}

	for _, t := range allTimes {
		prices := timeValues[t]

		for symbol, price := range prices {
			lastPrices[symbol] = price
		}

		var portfolioValue float64
		for _, p := range activePositions {
			if price, ok := lastPrices[p.Symbol]; ok {
				portfolioValue += price * p.Quantity
			}
		}

		if portfolioValue > 0 {
			result = append(result, map[string]interface{}{
				"time":  t,
				"value": portfolioValue,
				"pct":   0.0,
			})
		}
	}

	// Normalize pct: first data point = 0%, all others relative to it
	if len(result) > 0 {
		baseValue := result[0]["value"].(float64)
		for i := range result {
			val := result[i]["value"].(float64)
			if baseValue > 0 {
				result[i]["pct"] = ((val - baseValue) / baseValue) * 100
			}
		}
	}

	return result
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
			if _, ok := openBuys[trade.Symbol]; ok {
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
	db.Where("action = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false).Order("signal_date desc").Find(&trades)

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
		// Find the matching BUY (also exclude deleted)
		var buy FlipperBotTrade
		if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND is_filter_blocked = ? AND signal_date < ?",
			sell.Symbol, "BUY", false, false, sell.SignalDate).
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
		Mode         string  `json:"mode"` // "defensive", "aggressive", or "quant"
		Symbol       string  `json:"symbol"`
		Name         string  `json:"name"`
		EntryPrice   float64 `json:"entry_price"`
		ExitPrice    float64 `json:"exit_price"`
		CurrentPrice float64 `json:"current_price"`
		EntryDate    int64   `json:"entry_date"`
		ExitDate     int64   `json:"exit_date"`
		Status       string  `json:"status"` // "OPEN" or "CLOSED"
		ReturnPct    float64 `json:"return_pct"`
		// Stock-level metrics for filtering
		WinRate    float64 `json:"win_rate"`
		RiskReward float64 `json:"risk_reward"`
		AvgReturn  float64 `json:"avg_return"`
		MarketCap  int64   `json:"market_cap"`
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
				WinRate:    stock.WinRate,
				RiskReward: stock.RiskReward,
				AvgReturn:  stock.AvgReturn,
				MarketCap:  stock.MarketCap,
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
				WinRate:    stock.WinRate,
				RiskReward: stock.RiskReward,
				AvgReturn:  stock.AvgReturn,
				MarketCap:  stock.MarketCap,
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

	// Get quant stock performances
	var quantStocks []QuantStockPerformance
	db.Find(&quantStocks)

	for _, stock := range quantStocks {
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
				Mode:       "quant",
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				EntryPrice: trade.EntryPrice,
				EntryDate:  trade.EntryDate,
				ReturnPct:  trade.ReturnPct,
				WinRate:    stock.WinRate,
				RiskReward: stock.RiskReward,
				AvgReturn:  stock.AvgReturn,
				MarketCap:  stock.MarketCap,
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

	// Get ditz stock performances
	var ditzStocks []DitzStockPerformance
	db.Find(&ditzStocks)

	for _, stock := range ditzStocks {
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
				Mode:       "ditz",
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				EntryPrice: trade.EntryPrice,
				EntryDate:  trade.EntryDate,
				ReturnPct:  trade.ReturnPct,
				WinRate:    stock.WinRate,
				RiskReward: stock.RiskReward,
				AvgReturn:  stock.AvgReturn,
				MarketCap:  stock.MarketCap,
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

	// Get trader stock performances
	var traderStocks []TraderStockPerformance
	db.Find(&traderStocks)

	for _, stock := range traderStocks {
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
				Mode:       "trader",
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				EntryPrice: trade.EntryPrice,
				EntryDate:  trade.EntryDate,
				ReturnPct:  trade.ReturnPct,
				WinRate:    stock.WinRate,
				RiskReward: stock.RiskReward,
				AvgReturn:  stock.AvgReturn,
				MarketCap:  stock.MarketCap,
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
		Quantity        float64  `json:"quantity"`
		AvgPrice        float64  `json:"avg_price"`
		IsLive          bool     `json:"is_live"`
		StopLossPercent *float64 `json:"stop_loss_percent"`
		StopLossType    *string  `json:"stop_loss_type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Update position
	position.Quantity = req.Quantity
	position.AvgPrice = req.AvgPrice
	position.IsLive = req.IsLive
	if req.StopLossPercent != nil {
		if *req.StopLossPercent <= 0 {
			position.StopLossPercent = nil
		} else {
			position.StopLossPercent = req.StopLossPercent
		}
	}
	if req.StopLossType != nil {
		position.StopLossType = *req.StopLossType
	}
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
	} else if trade.Action == "SELL" {
		// Update closed position with corrected sell price
		var position FlipperBotPosition
		if err := db.Where("symbol = ? AND is_closed = ? AND is_live = ?", trade.Symbol, true, trade.IsLive).Order("updated_at desc").First(&position).Error; err == nil {
			position.SellPrice = req.Price
			pnl := (req.Price - position.AvgPrice) * position.Quantity
			pnlPct := ((req.Price - position.AvgPrice) / position.AvgPrice) * 100
			position.ProfitLoss = &pnl
			position.ProfitLossPct = &pnlPct
			db.Save(&position)
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
	wasDeleted := trade.IsDeleted

	// Toggle soft-delete
	trade.IsDeleted = !wasDeleted
	db.Save(&trade)

	if trade.Action == "BUY" {
		if !wasDeleted {
			// Soft-deleting a BUY → also soft-delete matching SELL, hard-delete position + portfolio
			var sellTrade FlipperBotTrade
			if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "SELL", false).
				Order("signal_date desc").First(&sellTrade).Error; err == nil {
				sellTrade.IsDeleted = true
				db.Save(&sellTrade)
			}
			db.Where("symbol = ? AND is_live = ?", symbol, trade.IsLive).Delete(&FlipperBotPosition{})
			db.Where("user_id = ? AND symbol = ?", FLIPPERBOT_USER_ID, symbol).Delete(&PortfolioPosition{})
		} else {
			// Restoring a BUY → also restore matching SELL, recreate position
			var sellTrade FlipperBotTrade
			hasSell := false
			if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "SELL", true).
				Order("signal_date desc").First(&sellTrade).Error; err == nil {
				sellTrade.IsDeleted = false
				db.Save(&sellTrade)
				hasSell = true
			}

			qty := trade.Quantity
			buyDate := trade.SignalDate
			newPos := FlipperBotPosition{
				Symbol:   symbol,
				Name:     trade.Name,
				Quantity: qty,
				AvgPrice: trade.Price,
				IsLive:   trade.IsLive,
				BuyDate:  buyDate,
			}

			if hasSell {
				// SELL exists → position is closed, no portfolio entry
				newPos.IsClosed = true
				newPos.SellPrice = sellTrade.Price
				sellDate := sellTrade.SignalDate
				newPos.SellDate = &sellDate
				newPos.ProfitLoss = sellTrade.ProfitLoss
				newPos.ProfitLossPct = sellTrade.ProfitLossPct
				db.Create(&newPos)
			} else {
				// No SELL → position is open, create portfolio entry
				db.Create(&newPos)
				portfolioPos := PortfolioPosition{
					UserID:       FLIPPERBOT_USER_ID,
					Symbol:       symbol,
					Name:         trade.Name,
					AvgPrice:     trade.Price,
					PurchaseDate: &buyDate,
					Quantity:     &qty,
				}
				db.Create(&portfolioPos)
			}
		}
	} else if trade.Action == "SELL" {
		// Check if the corresponding BUY is deleted
		var buyTrade FlipperBotTrade
		buyDeleted := false
		if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "BUY", true).
			Order("signal_date desc").First(&buyTrade).Error; err == nil {
			buyDeleted = true
		}

		if buyDeleted {
			// BUY is deleted → just toggle SELL, no position changes
		} else if !wasDeleted {
			// Soft-deleting a SELL (BUY active) → reopen position
			var pos FlipperBotPosition
			if err := db.Where("symbol = ? AND is_live = ?", symbol, trade.IsLive).Order("updated_at desc").First(&pos).Error; err == nil {
				pos.IsClosed = false
				pos.SellPrice = 0
				pos.SellDate = nil
				pos.ProfitLoss = nil
				pos.ProfitLossPct = nil
				pos.UpdatedAt = time.Now()
				db.Save(&pos)

				qty := pos.Quantity
				buyDate := pos.BuyDate
				portfolioPos := PortfolioPosition{
					UserID:       FLIPPERBOT_USER_ID,
					Symbol:       pos.Symbol,
					Name:         pos.Name,
					AvgPrice:     pos.AvgPrice,
					PurchaseDate: &buyDate,
					Quantity:     &qty,
				}
				db.Create(&portfolioPos)
			}
		} else {
			// Restoring a SELL (BUY active) → re-close position
			var pos FlipperBotPosition
			if err := db.Where("symbol = ? AND is_live = ? AND is_closed = ?", symbol, trade.IsLive, false).First(&pos).Error; err == nil {
				pos.IsClosed = true
				pos.SellPrice = trade.Price
				sellDate := trade.SignalDate
				pos.SellDate = &sellDate
				pos.ProfitLoss = trade.ProfitLoss
				pos.ProfitLossPct = trade.ProfitLossPct
				pos.UpdatedAt = time.Now()
				db.Save(&pos)
			}
			db.Where("user_id = ? AND symbol = ?", FLIPPERBOT_USER_ID, symbol).Delete(&PortfolioPosition{})
		}
	}

	action := "deleted"
	if wasDeleted {
		action = "restored"
	}
	c.JSON(http.StatusOK, gin.H{"message": "Trade " + action, "trade": trade})
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

// runLutzUpdateInternal performs the Lutz bot update without HTTP context
func runLutzUpdateInternal(triggeredBy string) {
	checkLutzStopLoss()

	now := time.Now()
	sessionID := uuid.New().String()

	var logs []map[string]interface{}
	addLog := func(level, msg string) {
		logs = append(logs, map[string]interface{}{"level": level, "message": msg, "time": time.Now().Format("15:04:05")})
		db.Create(&BotLog{Bot: "lutz", Level: level, Message: msg, SessionID: sessionID, CreatedAt: time.Now()})
	}

	addLog("INFO", fmt.Sprintf("Lutz Update gestartet um %s (von: %s)", now.Format("15:04:05"), triggeredBy))

	var lutzConfig BXtrenderConfig
	db.Where("mode = ?", "aggressive").First(&lutzConfig)

	var perfData []AggressiveStockPerformance
	if err := db.Find(&perfData).Error; err != nil {
		addLog("ERROR", fmt.Sprintf("Fehler beim Laden der Performance Daten: %v", err))
		return
	}

	addLog("INFO", fmt.Sprintf("%d Aktien geladen", len(perfData)))

	// Phase 1: Validate existing positions against current BXTrender data
	var existingPositions []LutzPosition
	db.Where("is_live = ? AND is_closed = ?", false, false).Find(&existingPositions)

	for _, pos := range existingPositions {
		var stockPerf *AggressiveStockPerformance
		for i := range perfData {
			if perfData[i].Symbol == pos.Symbol {
				stockPerf = &perfData[i]
				break
			}
		}

		if stockPerf == nil {
			addLog("WARN", fmt.Sprintf("%s: Position vorhanden aber keine Performance-Daten - überspringe Validierung", pos.Symbol))
			continue
		}

		if stockPerf.Signal == "NO_DATA" {
			addLog("SKIP", fmt.Sprintf("%s: Nicht genug Daten für Berechnung - überspringe", pos.Symbol))
			continue
		}

		if stockPerf.Signal == "SELL" || stockPerf.Signal == "WAIT" {
			addLog("KORREKTUR", fmt.Sprintf("%s: Signal ist jetzt %s, aber Position vorhanden - schließe Position", pos.Symbol, stockPerf.Signal))

			sellPrice := stockPerf.CurrentPrice
			sellDate := now

			sellTrade := LutzTrade{
				Symbol:     pos.Symbol,
				Name:       pos.Name,
				Action:     "SELL",
				Quantity:   pos.Quantity,
				Price:      sellPrice,
				SignalDate: sellDate,
				ExecutedAt: sellDate,
				IsPending:  false,
				IsLive:     pos.IsLive,
			}
			pnl := (sellPrice - pos.AvgPrice) * pos.Quantity
			pnlPct := ((sellPrice - pos.AvgPrice) / pos.AvgPrice) * 100
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct

			db.Create(&sellTrade)

			pos.IsClosed = true
			pos.SellPrice = sellPrice
			pos.SellDate = &sellDate
			pos.ProfitLoss = &pnl
			pos.ProfitLossPct = &pnlPct
			pos.UpdatedAt = time.Now()
			db.Save(&pos)
			db.Where("user_id = ? AND symbol = ?", LUTZ_USER_ID, pos.Symbol).Delete(&PortfolioPosition{})

			addLog("KORREKTUR", fmt.Sprintf("%s: Position geschlossen @ $%.2f (P/L: %.2f%%)", pos.Symbol, sellPrice, pnlPct))
			continue
		}
	}

	// Phase 2: Process new signals (BUY/SELL)
	for _, stock := range perfData {
		if !isStockAllowedForBot("lutz", stock.Symbol) {
			continue
		}
		if stock.Signal == "BUY" {
			var existingPos LutzPosition
			if err := db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error; err == nil {
				addLog("SKIP", fmt.Sprintf("%s: Position bereits vorhanden", stock.Symbol))
				continue
			}

			var deletedBuy LutzTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ?", stock.Symbol, "BUY", true).Order("executed_at desc").First(&deletedBuy).Error; err == nil {
				var sellAfterDeleted LutzTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, deletedBuy.ExecutedAt).First(&sellAfterDeleted).Error; err != nil {
					addLog("SKIP", fmt.Sprintf("%s: Soft-deleted BUY vorhanden - überspringe", stock.Symbol))
					continue
				}
			}

			var existingBuy LutzTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND is_filter_blocked = ?", stock.Symbol, "BUY", false, false).Order("executed_at desc").First(&existingBuy).Error; err == nil {
				var sellAfter LutzTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, existingBuy.ExecutedAt).First(&sellAfter).Error; err != nil {
					addLog("SKIP", fmt.Sprintf("%s: Bereits gekauft am %s", stock.Symbol, existingBuy.ExecutedAt.Format("02.01.2006")))
					continue
				}
			}

			// Check if there's already a filter-blocked BUY (don't create duplicates)
			var blockedBuy LutzTrade
			if err := db.Where("symbol = ? AND action = ? AND is_filter_blocked = ? AND is_deleted = ?", stock.Symbol, "BUY", true, false).Order("executed_at desc").First(&blockedBuy).Error; err == nil {
				var sellAfterBlocked LutzTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, blockedBuy.ExecutedAt).First(&sellAfterBlocked).Error; err != nil {
					continue
				}
			}

			// Use current price and today's date (no retroactive trades)
			signalPrice := stock.CurrentPrice
			signalDate := now

			investmentEUR := 100.0
			investmentUSD := convertToUSD(investmentEUR, "EUR")
			qty := math.Round((investmentUSD/signalPrice)*1000000) / 1000000
			if qty <= 0 {
				addLog("SKIP", fmt.Sprintf("%s: Ungültige Menge berechnet", stock.Symbol))
				continue
			}

			// Check bot filter config
			filterBlocked, filterReason := checkBotFilterConfig("lutz", stock.WinRate, stock.RiskReward, stock.AvgReturn, stock.MarketCap)
			if filterBlocked {
				blockedTrade := LutzTrade{
					Symbol:            stock.Symbol,
					Name:              stock.Name,
					Action:            "BUY",
					Quantity:          qty,
					Price:             signalPrice,
					SignalDate:        signalDate,
					ExecutedAt:        signalDate,
					IsPending:         false,
					IsLive:            false,
					IsFilterBlocked:   true,
					FilterBlockReason: filterReason,
				}
				db.Create(&blockedTrade)
				addLog("FILTER", fmt.Sprintf("%s: BUY blockiert durch Filter (%s)", stock.Symbol, filterReason))
				continue
			}

			buyTrade := LutzTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      signalPrice,
				SignalDate: signalDate,
				ExecutedAt: signalDate,
				IsPending:  false,
				IsLive:     false,
			}
			db.Create(&buyTrade)

			newPos := LutzPosition{
				Symbol:        stock.Symbol,
				Name:          stock.Name,
				Quantity:      qty,
				AvgPrice:      signalPrice,
				InvestedEUR:   investmentEUR,
				BuyDate:       signalDate,
				IsPending:     false,
				IsLive:        false,
				HighestPrice:  signalPrice,
				StopLossPrice: signalPrice * (1 - lutzConfig.TslPercent/100),
				StopLossType:  "trailing",
			}
			db.Create(&newPos)

			portfolioPos := PortfolioPosition{
				UserID:       LUTZ_USER_ID,
				Symbol:       stock.Symbol,
				Name:         stock.Name,
				PurchaseDate: &signalDate,
				AvgPrice:     signalPrice,
				Currency:     "USD",
				Quantity:     &qty,
			}
			db.Create(&portfolioPos)

			addLog("ACTION", fmt.Sprintf("BUY ausgeführt: %s %.6f @ $%.2f (Signal: %s)", stock.Symbol, qty, signalPrice, signalDate.Format("02.01.2006")))

		} else if stock.Signal == "SELL" {
			var deletedSell LutzTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ?", stock.Symbol, "SELL", true).Order("executed_at desc").First(&deletedSell).Error; err == nil {
				addLog("SKIP", fmt.Sprintf("%s: Soft-deleted SELL vorhanden - überspringe", stock.Symbol))
				continue
			}

			var existingPos LutzPosition
			if err := db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error; err != nil {
				addLog("SKIP", fmt.Sprintf("%s: SELL Signal aber keine offene Position", stock.Symbol))
				continue
			}

			// Use current price and today's date (no retroactive trades)
			sellPrice := stock.CurrentPrice
			sellDate := now

			sellTrade := LutzTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "SELL",
				Quantity:   existingPos.Quantity,
				Price:      sellPrice,
				SignalDate: sellDate,
				ExecutedAt: sellDate,
				IsPending:  false,
				IsLive:     existingPos.IsLive,
			}

			pnl := (sellPrice - existingPos.AvgPrice) * existingPos.Quantity
			pnlPct := ((sellPrice - existingPos.AvgPrice) / existingPos.AvgPrice) * 100
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct

			db.Create(&sellTrade)

			existingPos.IsClosed = true
			existingPos.SellPrice = sellPrice
			existingPos.SellDate = &sellDate
			existingPos.ProfitLoss = &pnl
			existingPos.ProfitLossPct = &pnlPct
			existingPos.UpdatedAt = time.Now()
			db.Save(&existingPos)
			db.Where("user_id = ? AND symbol = ?", LUTZ_USER_ID, stock.Symbol).Delete(&PortfolioPosition{})

			addLog("ACTION", fmt.Sprintf("SELL ausgeführt: %s @ $%.2f (Signal: %s, P/L: %.2f%%)", stock.Symbol, sellPrice, sellDate.Format("02.01.2006"), pnlPct))
		}
	}

	addLog("INFO", "Lutz Update abgeschlossen")

	lastRefresh := map[string]interface{}{
		"updated_at":   now,
		"triggered_by": triggeredBy,
		"logs":         logs,
	}
	lastRefreshJSON, _ := json.Marshal(lastRefresh)

	var setting SystemSetting
	if err := db.Where("key = ?", "last_lutz_refresh").First(&setting).Error; err != nil {
		setting = SystemSetting{
			Key:       "last_lutz_refresh",
			Value:     string(lastRefreshJSON),
			UpdatedAt: now,
		}
		db.Create(&setting)
	} else {
		setting.Value = string(lastRefreshJSON)
		setting.UpdatedAt = now
		db.Save(&setting)
	}
}

func lutzUpdate(c *gin.Context) {
	triggeredBy := "system"
	if userID, exists := c.Get("userID"); exists {
		var user User
		if err := db.First(&user, userID).Error; err == nil {
			triggeredBy = user.Username
		}
	}

	runLutzUpdateInternal(triggeredBy)

	c.JSON(http.StatusOK, gin.H{
		"message": "Lutz update completed",
	})
}

func getLutzPortfolio(c *gin.Context) {
	var positions []LutzPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Order("buy_date desc").Find(&positions)

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
	db.Where("is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false).Order("signal_date desc").Limit(50).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getLutzActionsAll(c *gin.Context) {
	var trades []LutzTrade
	db.Where("is_pending = ?", false).Order("signal_date desc").Limit(100).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getLutzPerformance(c *gin.Context) {
	var sellTrades []LutzTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false, false).Find(&sellTrades)

	var buyTrades []LutzTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "BUY", false, false, false).Find(&buyTrades)

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

	totalReturnPctClosed := 0.0
	for _, trade := range sellTrades {
		if trade.ProfitLossPct != nil {
			totalReturnPctClosed += *trade.ProfitLossPct
		}
	}
	avgReturnPerTrade := 0.0
	if len(sellTrades) > 0 {
		avgReturnPerTrade = totalReturnPctClosed / float64(len(sellTrades))
	}

	var positions []LutzPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Find(&positions)

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
		"realized_return_pct":   totalReturnPctClosed,
		"avg_return_per_trade":  avgReturnPerTrade,
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

func getLutzSimulatedPortfolio(c *gin.Context) {
	var positions []LutzPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Order("buy_date desc").Find(&positions)

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
		InvestedEUR    float64   `json:"invested_eur"`
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
			InvestedEUR:    pos.InvestedEUR,
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

	var closedSellTrades []LutzTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ?", "SELL", false, false).Find(&closedSellTrades)

	realizedPL := 0.0
	totalClosedInvested := 0.0
	for _, trade := range closedSellTrades {
		if trade.ProfitLoss != nil {
			realizedPL += *trade.ProfitLoss
			totalClosedInvested += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}

	overallReturn := totalReturn + realizedPL
	overallInvested := totalInvested + totalClosedInvested
	overallReturnPct := 0.0
	if overallInvested > 0 {
		overallReturnPct = (overallReturn / overallInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"positions":          result,
		"total_value":        totalValue,
		"total_invested":     totalInvested,
		"total_return":       totalReturn,
		"total_return_pct":   overallReturnPct,
		"realized_pl":        realizedPL,
		"overall_return":     overallReturn,
		"overall_invested":   overallInvested,
	})
}

func getLutzSimulatedPerformance(c *gin.Context) {
	var sellTrades []LutzTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_admin_closed = ? AND is_filter_blocked = ?", "SELL", false, false, false, false).Find(&sellTrades)

	var buyTrades []LutzTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_admin_closed = ? AND is_filter_blocked = ?", "BUY", false, false, false, false).Find(&buyTrades)

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

	totalReturnPctClosed := 0.0
	for _, trade := range sellTrades {
		if trade.ProfitLossPct != nil {
			totalReturnPctClosed += *trade.ProfitLossPct
		}
	}
	avgReturnPerTrade := 0.0
	if len(sellTrades) > 0 {
		avgReturnPerTrade = totalReturnPctClosed / float64(len(sellTrades))
	}

	var positions []LutzPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	unrealizedGain := 0.0
	investedInPositions := 0.0
	currentValue := 0.0

	for _, pos := range positions {
		quote := quotes[pos.Symbol]
		currentPrice := quote.Price
		if currentPrice <= 0 {
			currentPrice = pos.AvgPrice
		}
		investedInPositions += pos.AvgPrice * pos.Quantity
		currentValue += currentPrice * pos.Quantity
		unrealizedGain += (currentPrice - pos.AvgPrice) * pos.Quantity
	}

	totalReturnPct := 0.0
	if investedInPositions > 0 {
		totalReturnPct = (unrealizedGain / investedInPositions) * 100
	}

	totalGain := totalProfitLoss + unrealizedGain
	totalInvestedAll := investedInPositions
	for _, trade := range sellTrades {
		if trade.ProfitLoss != nil {
			totalInvestedAll += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}
	overallReturnPct := 0.0
	if totalInvestedAll > 0 {
		overallReturnPct = (totalGain / totalInvestedAll) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_trades":          len(buyTrades) + len(sellTrades),
		"total_buys":            len(buyTrades),
		"open_positions":        len(positions),
		"closed_trades":         len(sellTrades),
		"wins":                  wins,
		"losses":                losses,
		"win_rate":              winRate,
		"realized_profit":       totalProfitLoss,
		"avg_return_per_trade":  avgReturnPerTrade,
		"unrealized_gain":       unrealizedGain,
		"invested_in_positions": investedInPositions,
		"current_value":         currentValue,
		"total_gain":            totalGain,
		"total_return_pct":      totalReturnPct,
		"overall_return_pct":    overallReturnPct,
	})
}

func toggleLutzTradeRead(c *gin.Context) {
	id := c.Param("id")
	var trade LutzTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}
	trade.IsRead = !trade.IsRead
	db.Save(&trade)
	c.JSON(http.StatusOK, gin.H{"trade": trade})
}

func markAllLutzTradesRead(c *gin.Context) {
	db.Model(&LutzTrade{}).Where("is_read = ? AND is_pending = ?", false, false).Update("is_read", true)
	c.JSON(http.StatusOK, gin.H{"message": "All trades marked as read"})
}

func markAllLutzTradesUnread(c *gin.Context) {
	db.Model(&LutzTrade{}).Where("is_read = ? AND is_pending = ?", true, false).Update("is_read", false)
	c.JSON(http.StatusOK, gin.H{"message": "All trades marked as unread"})
}

func getLutzUnreadCount(c *gin.Context) {
	var count int64
	db.Model(&LutzTrade{}).Where("is_read = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false, false).Count(&count)

	var unreadTrades []LutzTrade
	db.Where("is_read = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false, false).Order("executed_at desc").Limit(10).Find(&unreadTrades)

	c.JSON(http.StatusOK, gin.H{"count": count, "trades": unreadTrades})
}

func cleanupLutzPending(c *gin.Context) {
	db.Where("is_pending = ?", true).Delete(&LutzTrade{})
	db.Where("is_pending = ?", true).Delete(&LutzPosition{})
	c.JSON(http.StatusOK, gin.H{"message": "Pending trades cleaned up"})
}

func getLastLutzRefresh(c *gin.Context) {
	var setting SystemSetting
	if err := db.Where("key = ?", "last_lutz_refresh").First(&setting).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"updated_at":   nil,
			"triggered_by": nil,
			"logs":         []interface{}{},
		})
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(setting.Value), &result); err != nil {
		c.JSON(http.StatusOK, gin.H{"updated_at": setting.UpdatedAt})
		return
	}
	c.JSON(http.StatusOK, result)
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
	db.Where("action = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false).Order("signal_date desc").Find(&trades)

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
		// Find the matching BUY (also exclude deleted)
		var buy LutzTrade
		if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND is_filter_blocked = ? AND signal_date < ?",
			sell.Symbol, "BUY", false, false, sell.SignalDate).
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
		Quantity        float64  `json:"quantity"`
		AvgPrice        float64  `json:"avg_price"`
		IsLive          bool     `json:"is_live"`
		StopLossPercent *float64 `json:"stop_loss_percent"`
		StopLossType    *string  `json:"stop_loss_type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Update position
	position.Quantity = req.Quantity
	position.AvgPrice = req.AvgPrice
	position.IsLive = req.IsLive
	if req.StopLossPercent != nil {
		if *req.StopLossPercent <= 0 {
			position.StopLossPercent = nil
		} else {
			position.StopLossPercent = req.StopLossPercent
		}
	}
	if req.StopLossType != nil {
		position.StopLossType = *req.StopLossType
	}
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
	} else if trade.Action == "SELL" {
		var position LutzPosition
		if err := db.Where("symbol = ? AND is_closed = ? AND is_live = ?", trade.Symbol, true, trade.IsLive).Order("updated_at desc").First(&position).Error; err == nil {
			position.SellPrice = req.Price
			pnl := (req.Price - position.AvgPrice) * position.Quantity
			pnlPct := ((req.Price - position.AvgPrice) / position.AvgPrice) * 100
			position.ProfitLoss = &pnl
			position.ProfitLossPct = &pnlPct
			db.Save(&position)
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
	wasDeleted := trade.IsDeleted

	// Toggle soft-delete
	trade.IsDeleted = !wasDeleted
	db.Save(&trade)

	if trade.Action == "BUY" {
		if !wasDeleted {
			// Soft-deleting a BUY → also soft-delete matching SELL, hard-delete position + portfolio
			var sellTrade LutzTrade
			if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "SELL", false).
				Order("signal_date desc").First(&sellTrade).Error; err == nil {
				sellTrade.IsDeleted = true
				db.Save(&sellTrade)
			}
			db.Where("symbol = ? AND is_live = ?", symbol, trade.IsLive).Delete(&LutzPosition{})
			db.Where("user_id = ? AND symbol = ?", LUTZ_USER_ID, symbol).Delete(&PortfolioPosition{})
		} else {
			// Restoring a BUY → also restore matching SELL, recreate position
			var sellTrade LutzTrade
			hasSell := false
			if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "SELL", true).
				Order("signal_date desc").First(&sellTrade).Error; err == nil {
				sellTrade.IsDeleted = false
				db.Save(&sellTrade)
				hasSell = true
			}

			qty := trade.Quantity
			buyDate := trade.SignalDate
			newPos := LutzPosition{
				Symbol:   symbol,
				Name:     trade.Name,
				Quantity: qty,
				AvgPrice: trade.Price,
				IsLive:   trade.IsLive,
				BuyDate:  buyDate,
			}

			if hasSell {
				newPos.IsClosed = true
				newPos.SellPrice = sellTrade.Price
				sellDate := sellTrade.SignalDate
				newPos.SellDate = &sellDate
				newPos.ProfitLoss = sellTrade.ProfitLoss
				newPos.ProfitLossPct = sellTrade.ProfitLossPct
				db.Create(&newPos)
			} else {
				db.Create(&newPos)
				portfolioPos := PortfolioPosition{
					UserID:       LUTZ_USER_ID,
					Symbol:       symbol,
					Name:         trade.Name,
					AvgPrice:     trade.Price,
					PurchaseDate: &buyDate,
					Quantity:     &qty,
				}
				db.Create(&portfolioPos)
			}
		}
	} else if trade.Action == "SELL" {
		// Check if the corresponding BUY is deleted
		var buyTrade LutzTrade
		buyDeleted := false
		if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "BUY", true).
			Order("signal_date desc").First(&buyTrade).Error; err == nil {
			buyDeleted = true
		}

		if buyDeleted {
			// BUY is deleted → just toggle SELL, no position changes
		} else if !wasDeleted {
			// Soft-deleting a SELL (BUY active) → reopen position
			var pos LutzPosition
			if err := db.Where("symbol = ? AND is_live = ?", symbol, trade.IsLive).Order("updated_at desc").First(&pos).Error; err == nil {
				pos.IsClosed = false
				pos.SellPrice = 0
				pos.SellDate = nil
				pos.ProfitLoss = nil
				pos.ProfitLossPct = nil
				pos.UpdatedAt = time.Now()
				db.Save(&pos)

				qty := pos.Quantity
				buyDate := pos.BuyDate
				portfolioPos := PortfolioPosition{
					UserID:       LUTZ_USER_ID,
					Symbol:       pos.Symbol,
					Name:         pos.Name,
					AvgPrice:     pos.AvgPrice,
					PurchaseDate: &buyDate,
					Quantity:     &qty,
				}
				db.Create(&portfolioPos)
			}
		} else {
			// Restoring a SELL (BUY active) → re-close position
			var pos LutzPosition
			if err := db.Where("symbol = ? AND is_live = ? AND is_closed = ?", symbol, trade.IsLive, false).First(&pos).Error; err == nil {
				pos.IsClosed = true
				pos.SellPrice = trade.Price
				sellDate := trade.SignalDate
				pos.SellDate = &sellDate
				pos.ProfitLoss = trade.ProfitLoss
				pos.ProfitLossPct = trade.ProfitLossPct
				pos.UpdatedAt = time.Now()
				db.Save(&pos)
			}
			db.Where("user_id = ? AND symbol = ?", LUTZ_USER_ID, symbol).Delete(&PortfolioPosition{})
		}
	}

	action := "deleted"
	if wasDeleted {
		action = "restored"
	}
	c.JSON(http.StatusOK, gin.H{"message": "Trade " + action, "trade": trade})
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

// ========================================
// Quant Bot Functions
// ========================================

// runQuantUpdateInternal performs the Quant bot update without HTTP context
func runQuantUpdateInternal(triggeredBy string) {
	checkQuantStopLoss()

	now := time.Now()
	sessionID := uuid.New().String()

	var logs []map[string]interface{}
	addLog := func(level, msg string) {
		logs = append(logs, map[string]interface{}{"level": level, "message": msg, "time": time.Now().Format("15:04:05")})
		db.Create(&BotLog{Bot: "quant", Level: level, Message: msg, SessionID: sessionID, CreatedAt: time.Now()})
	}

	addLog("INFO", fmt.Sprintf("Quant Update gestartet um %s (von: %s)", now.Format("15:04:05"), triggeredBy))

	var quantBotConfig BXtrenderQuantConfig
	db.First(&quantBotConfig)

	var perfData []QuantStockPerformance
	if err := db.Find(&perfData).Error; err != nil {
		addLog("ERROR", fmt.Sprintf("Fehler beim Laden der Performance Daten: %v", err))
		return
	}

	addLog("INFO", fmt.Sprintf("%d Aktien geladen", len(perfData)))

	// Phase 1: Validate existing positions and trades against current BXTrender data
	// This catches cases where BXTrender settings changed, or trades were created with wrong data
	var existingPositions []QuantPosition
	db.Where("is_live = ? AND is_closed = ?", false, false).Find(&existingPositions)

	for _, pos := range existingPositions {
		// Find matching performance data
		var stockPerf *QuantStockPerformance
		for i := range perfData {
			if perfData[i].Symbol == pos.Symbol {
				stockPerf = &perfData[i]
				break
			}
		}

		if stockPerf == nil {
			addLog("WARN", fmt.Sprintf("%s: Position vorhanden aber keine Performance-Daten - überspringe Validierung", pos.Symbol))
			continue
		}

		// Parse TradesJSON to find the matching open BUY trade
		if stockPerf.TradesJSON == "" {
			continue
		}
		var serverTrades []ServerTrade
		if err := json.Unmarshal([]byte(stockPerf.TradesJSON), &serverTrades); err != nil {
			continue
		}

		// Find the last open BUY trade in TradesJSON (one without a following SELL)
		var lastBuyTrade *ServerTrade
		for i := len(serverTrades) - 1; i >= 0; i-- {
			if serverTrades[i].Type == "BUY" {
				lastBuyTrade = &serverTrades[i]
				break
			}
		}

		if stockPerf.Signal == "NO_DATA" {
			addLog("SKIP", fmt.Sprintf("%s: Nicht genug Daten für Berechnung - überspringe", pos.Symbol))
			continue
		}

		if stockPerf.Signal == "SELL" || stockPerf.Signal == "WAIT" {
			// BXTrender says no position should be open - but we have one
			// This means settings changed and the BUY signal no longer exists
			addLog("KORREKTUR", fmt.Sprintf("%s: Signal ist jetzt %s, aber Position vorhanden - schließe Position", pos.Symbol, stockPerf.Signal))

			// Find the last SELL in TradesJSON for the correct close price/date
			sellPrice := stockPerf.CurrentPrice
			sellDate := now
			for i := len(serverTrades) - 1; i >= 0; i-- {
				if serverTrades[i].Type == "SELL" {
					sellPrice = serverTrades[i].Price
					sellDate = time.Unix(serverTrades[i].Time, 0)
					break
				}
			}

			sellTrade := QuantTrade{
				Symbol:     pos.Symbol,
				Name:       pos.Name,
				Action:     "SELL",
				Quantity:   pos.Quantity,
				Price:      sellPrice,
				SignalDate: sellDate,
				ExecutedAt: sellDate,
				IsPending:  false,
				IsLive:     pos.IsLive,
			}
			pnl := (sellPrice - pos.AvgPrice) * pos.Quantity
			pnlPct := ((sellPrice - pos.AvgPrice) / pos.AvgPrice) * 100
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct

			db.Create(&sellTrade)

			// Close position instead of deleting
			pos.IsClosed = true
			pos.SellPrice = sellPrice
			pos.SellDate = &sellDate
			pos.ProfitLoss = &pnl
			pos.ProfitLossPct = &pnlPct
			pos.UpdatedAt = time.Now()
			db.Save(&pos)
			db.Where("user_id = ? AND symbol = ?", QUANT_USER_ID, pos.Symbol).Delete(&PortfolioPosition{})

			addLog("KORREKTUR", fmt.Sprintf("%s: Position geschlossen @ $%.2f (P/L: %.2f%%)", pos.Symbol, sellPrice, pnlPct))
			continue
		}

		if lastBuyTrade != nil {
			// Validate price and date of existing position against TradesJSON
			expectedPrice := lastBuyTrade.Price
			expectedDate := time.Unix(lastBuyTrade.Time, 0)

			priceDiff := math.Abs(pos.AvgPrice-expectedPrice) / expectedPrice * 100
			dateDiff := pos.BuyDate.Sub(expectedDate).Hours()

			if priceDiff > 1.0 || math.Abs(dateDiff) > 48 {
				// Significant difference - correct the trade and position
				addLog("KORREKTUR", fmt.Sprintf("%s: Position korrigiert - Alt: $%.2f am %s, Neu: $%.2f am %s",
					pos.Symbol, pos.AvgPrice, pos.BuyDate.Format("02.01.2006"),
					expectedPrice, expectedDate.Format("02.01.2006")))

				// Update position
				investmentEUR := pos.InvestedEUR
				if investmentEUR == 0 {
					investmentEUR = 100.0
				}
				investmentUSD := convertToUSD(investmentEUR, "EUR")
				newQty := math.Round((investmentUSD/expectedPrice)*1000000) / 1000000

				db.Model(&pos).Updates(map[string]interface{}{
					"avg_price": expectedPrice,
					"buy_date":  expectedDate,
					"quantity":  newQty,
				})

				// Update matching BUY trade
				var buyTrade QuantTrade
				if err := db.Where("symbol = ? AND action = ? AND is_live = ?", pos.Symbol, "BUY", false).
					Order("created_at desc").First(&buyTrade).Error; err == nil {
					db.Model(&buyTrade).Updates(map[string]interface{}{
						"price":       expectedPrice,
						"signal_date": expectedDate,
						"executed_at": expectedDate,
						"quantity":    newQty,
					})
				}

				// Update portfolio position
				db.Model(&PortfolioPosition{}).
					Where("user_id = ? AND symbol = ?", QUANT_USER_ID, pos.Symbol).
					Updates(map[string]interface{}{
						"avg_price":     expectedPrice,
						"purchase_date": expectedDate,
						"quantity":      newQty,
					})
			}
		}
	}

	// Phase 2: Process new signals (BUY/SELL)
	for _, stock := range perfData {
		if !isStockAllowedForBot("quant", stock.Symbol) {
			continue
		}
		if stock.Signal == "BUY" {
			// Check if we already have an open position
			var existingPos QuantPosition
			if err := db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error; err == nil {
				addLog("SKIP", fmt.Sprintf("%s: Position bereits vorhanden", stock.Symbol))
				continue
			}

			// Check if there's a soft-deleted BUY (admin struck it out) - don't recreate
			var deletedBuy QuantTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ?", stock.Symbol, "BUY", true).Order("executed_at desc").First(&deletedBuy).Error; err == nil {
				var sellAfterDeleted QuantTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, deletedBuy.ExecutedAt).First(&sellAfterDeleted).Error; err != nil {
					addLog("SKIP", fmt.Sprintf("%s: Soft-deleted BUY vorhanden - überspringe", stock.Symbol))
					continue
				}
			}

			// Check if there's a recent BUY without a SELL
			var existingBuy QuantTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND is_filter_blocked = ?", stock.Symbol, "BUY", false, false).Order("executed_at desc").First(&existingBuy).Error; err == nil {
				var sellAfter QuantTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, existingBuy.ExecutedAt).First(&sellAfter).Error; err != nil {
					addLog("SKIP", fmt.Sprintf("%s: Bereits gekauft am %s", stock.Symbol, existingBuy.ExecutedAt.Format("02.01.2006")))
					continue
				}
			}

			// Check if there's already a filter-blocked BUY (don't create duplicates)
			var blockedBuy QuantTrade
			if err := db.Where("symbol = ? AND action = ? AND is_filter_blocked = ? AND is_deleted = ?", stock.Symbol, "BUY", true, false).Order("executed_at desc").First(&blockedBuy).Error; err == nil {
				var sellAfterBlocked QuantTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, blockedBuy.ExecutedAt).First(&sellAfterBlocked).Error; err != nil {
					continue
				}
			}

			// Extract signal date and price from TradesJSON (last BUY trade)
			signalPrice := stock.CurrentPrice
			signalDate := now
			if stock.TradesJSON != "" {
				var serverTrades []ServerTrade
				if err := json.Unmarshal([]byte(stock.TradesJSON), &serverTrades); err == nil {
					for i := len(serverTrades) - 1; i >= 0; i-- {
						if serverTrades[i].Type == "BUY" {
							signalPrice = serverTrades[i].Price
							signalDate = time.Unix(serverTrades[i].Time, 0)
							addLog("DEBUG", fmt.Sprintf("%s: Signal-Datum aus TradesJSON: %s, Preis: $%.2f",
								stock.Symbol, signalDate.Format("02.01.2006"), signalPrice))
							break
						}
					}
				}
			}

			// Calculate quantity based on 100 EUR investment
			investmentEUR := 100.0
			investmentUSD := convertToUSD(investmentEUR, "EUR")
			qty := math.Round((investmentUSD/signalPrice)*1000000) / 1000000
			if qty <= 0 {
				addLog("SKIP", fmt.Sprintf("%s: Ungültige Menge berechnet", stock.Symbol))
				continue
			}

			// Check bot filter config
			filterBlocked, filterReason := checkBotFilterConfig("quant", stock.WinRate, stock.RiskReward, stock.AvgReturn, stock.MarketCap)
			if filterBlocked {
				blockedTrade := QuantTrade{
					Symbol:            stock.Symbol,
					Name:              stock.Name,
					Action:            "BUY",
					Quantity:          qty,
					Price:             signalPrice,
					SignalDate:        signalDate,
					ExecutedAt:        signalDate,
					IsPending:         false,
					IsLive:            false,
					IsFilterBlocked:   true,
					FilterBlockReason: filterReason,
				}
				db.Create(&blockedTrade)
				addLog("FILTER", fmt.Sprintf("%s: BUY blockiert durch Filter (%s)", stock.Symbol, filterReason))
				continue
			}

			buyTrade := QuantTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      signalPrice,
				SignalDate: signalDate,
				ExecutedAt: signalDate,
				IsPending:  false,
				IsLive:     false,
			}
			db.Create(&buyTrade)

			newPos := QuantPosition{
				Symbol:        stock.Symbol,
				Name:          stock.Name,
				Quantity:      qty,
				AvgPrice:      signalPrice,
				InvestedEUR:   investmentEUR,
				BuyDate:       signalDate,
				IsPending:     false,
				IsLive:        false,
				HighestPrice:  signalPrice,
				StopLossPrice: signalPrice * (1 - quantBotConfig.TslPercent/100),
				StopLossType:  "trailing",
			}
			db.Create(&newPos)

			portfolioPos := PortfolioPosition{
				UserID:       QUANT_USER_ID,
				Symbol:       stock.Symbol,
				Name:         stock.Name,
				PurchaseDate: &signalDate,
				AvgPrice:     signalPrice,
				Currency:     "USD",
				Quantity:     &qty,
			}
			db.Create(&portfolioPos)

			addLog("ACTION", fmt.Sprintf("BUY ausgeführt: %s %.6f @ $%.2f (Signal: %s)", stock.Symbol, qty, signalPrice, signalDate.Format("02.01.2006")))

		} else if stock.Signal == "SELL" {
			// Check if there's a soft-deleted SELL (admin struck it out) - don't recreate
			var deletedSell QuantTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ?", stock.Symbol, "SELL", true).Order("executed_at desc").First(&deletedSell).Error; err == nil {
				addLog("SKIP", fmt.Sprintf("%s: Soft-deleted SELL vorhanden - überspringe", stock.Symbol))
				continue
			}

			var existingPos QuantPosition
			if err := db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error; err != nil {
				addLog("SKIP", fmt.Sprintf("%s: SELL Signal aber keine offene Position", stock.Symbol))
				continue
			}

			sellPrice := stock.CurrentPrice
			sellDate := now
			if stock.TradesJSON != "" {
				var serverTrades []ServerTrade
				if err := json.Unmarshal([]byte(stock.TradesJSON), &serverTrades); err == nil {
					for i := len(serverTrades) - 1; i >= 0; i-- {
						if serverTrades[i].Type == "SELL" {
							sellPrice = serverTrades[i].Price
							sellDate = time.Unix(serverTrades[i].Time, 0)
							break
						}
					}
				}
			}

			sellTrade := QuantTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "SELL",
				Quantity:   existingPos.Quantity,
				Price:      sellPrice,
				SignalDate: sellDate,
				ExecutedAt: sellDate,
				IsPending:  false,
				IsLive:     existingPos.IsLive,
			}

			pnl := (sellPrice - existingPos.AvgPrice) * existingPos.Quantity
			pnlPct := ((sellPrice - existingPos.AvgPrice) / existingPos.AvgPrice) * 100
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct

			db.Create(&sellTrade)

			// Close position instead of deleting
			existingPos.IsClosed = true
			existingPos.SellPrice = sellPrice
			existingPos.SellDate = &sellDate
			existingPos.ProfitLoss = &pnl
			existingPos.ProfitLossPct = &pnlPct
			existingPos.UpdatedAt = time.Now()
			db.Save(&existingPos)
			db.Where("user_id = ? AND symbol = ?", QUANT_USER_ID, stock.Symbol).Delete(&PortfolioPosition{})

			addLog("ACTION", fmt.Sprintf("SELL ausgeführt: %s @ $%.2f (Signal: %s, P/L: %.2f%%)", stock.Symbol, sellPrice, sellDate.Format("02.01.2006"), pnlPct))
		}
	}

	addLog("INFO", "Quant Update abgeschlossen")

	lastRefresh := map[string]interface{}{
		"updated_at":   now,
		"triggered_by": triggeredBy,
		"logs":         logs,
	}
	lastRefreshJSON, _ := json.Marshal(lastRefresh)

	var setting SystemSetting
	if err := db.Where("key = ?", "last_quant_refresh").First(&setting).Error; err != nil {
		setting = SystemSetting{
			Key:       "last_quant_refresh",
			Value:     string(lastRefreshJSON),
			UpdatedAt: now,
		}
		db.Create(&setting)
	} else {
		setting.Value = string(lastRefreshJSON)
		setting.UpdatedAt = now
		db.Save(&setting)
	}
}

func quantUpdate(c *gin.Context) {
	// Get username from session
	triggeredBy := "system"
	if userID, exists := c.Get("userID"); exists {
		var user User
		if err := db.First(&user, userID).Error; err == nil {
			triggeredBy = user.Username
		}
	}

	runQuantUpdateInternal(triggeredBy)

	// Read back the logs from the last refresh
	var setting SystemSetting
	if err := db.Where("key = ?", "last_quant_refresh").First(&setting).Error; err == nil {
		var lastRefresh map[string]interface{}
		if err := json.Unmarshal([]byte(setting.Value), &lastRefresh); err == nil {
			c.JSON(http.StatusOK, gin.H{"message": "Quant update completed", "logs": lastRefresh["logs"], "triggered_by": triggeredBy, "updated_at": lastRefresh["updated_at"]})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Quant update completed", "triggered_by": triggeredBy})
}

func getQuantPortfolio(c *gin.Context) {
	// Return all open positions (live + simulated) - frontend filters by is_live
	var positions []QuantPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Order("buy_date desc").Find(&positions)

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

	// Realisierte Gewinne aus geschlossenen Trades einrechnen
	var closedSellTrades []QuantTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ?", "SELL", false, false).Find(&closedSellTrades)

	realizedPL := 0.0
	totalClosedInvested := 0.0
	for _, trade := range closedSellTrades {
		if trade.ProfitLoss != nil {
			realizedPL += *trade.ProfitLoss
			totalClosedInvested += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}

	overallReturn := totalReturn + realizedPL
	overallInvested := totalInvested + totalClosedInvested
	overallReturnPct := 0.0
	if overallInvested > 0 {
		overallReturnPct = (overallReturn / overallInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"positions":          result,
		"total_value":        totalValue,
		"total_invested":     totalInvested,
		"total_return":       totalReturn,
		"total_return_pct":   overallReturnPct,
		"realized_pl":        realizedPL,
		"overall_return":     overallReturn,
		"overall_invested":   overallInvested,
	})
}

func getQuantActions(c *gin.Context) {
	// Return all trades (live + simulated) - frontend filters by is_live
	var trades []QuantTrade
	db.Where("is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false).Order("signal_date desc").Limit(50).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getQuantActionsAll(c *gin.Context) {
	// Admin view: return ALL trades (live + simulated)
	var trades []QuantTrade
	db.Where("is_pending = ?", false).Order("signal_date desc").Limit(100).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getQuantPerformance(c *gin.Context) {
	// Return all trades (live + simulated) - frontend filters by is_live
	var sellTrades []QuantTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false, false).Find(&sellTrades)

	var buyTrades []QuantTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "BUY", false, false, false).Find(&buyTrades)

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

	totalReturnPctClosed := 0.0
	for _, trade := range sellTrades {
		if trade.ProfitLossPct != nil {
			totalReturnPctClosed += *trade.ProfitLossPct
		}
	}
	avgReturnPerTrade := 0.0
	if len(sellTrades) > 0 {
		avgReturnPerTrade = totalReturnPctClosed / float64(len(sellTrades))
	}

	var positions []QuantPosition
	db.Where("is_pending = ? AND is_live = ? AND is_closed = ?", false, true, false).Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	unrealizedGain := 0.0
	investedInPositions := 0.0
	currentValue := 0.0
	liveCount := 0

	for _, pos := range positions {
		if pos.IsLive {
			liveCount++
		}
		investedInPositions += pos.AvgPrice * pos.Quantity
		quote := quotes[pos.Symbol]
		if quote.Price > 0 {
			currentValue += quote.Price * pos.Quantity
			unrealizedGain += (quote.Price - pos.AvgPrice) * pos.Quantity
		} else {
			currentValue += pos.AvgPrice * pos.Quantity
		}
	}

	unrealizedGainPct := 0.0
	if investedInPositions > 0 {
		unrealizedGainPct = (unrealizedGain / investedInPositions) * 100
	}

	totalGain := totalProfitLoss + unrealizedGain
	// Total invested = current open positions + closed positions (sell price - profit = original cost)
	totalInvested := investedInPositions
	for _, trade := range sellTrades {
		if trade.ProfitLoss != nil {
			totalInvested += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}
	overallReturnPct := 0.0
	if totalInvested > 0 {
		overallReturnPct = (totalGain / totalInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_trades":         len(buyTrades) + len(sellTrades),
		"total_buys":           len(buyTrades),
		"completed_trades":     len(sellTrades),
		"open_positions":       len(positions),
		"live_positions":       liveCount,
		"wins":                 wins,
		"losses":               losses,
		"win_rate":             winRate,
		"realized_profit":      totalProfitLoss,
		"total_gain":           totalGain,
		"overall_return_pct":   overallReturnPct,
		"avg_return_per_trade": avgReturnPerTrade,
		"unrealized_gain":      unrealizedGain,
		"total_return_pct":     unrealizedGainPct,
		"invested_in_positions": investedInPositions,
		"current_value":        currentValue,
	})
}

func resetQuant(c *gin.Context) {
	db.Where("1 = 1").Delete(&QuantTrade{})
	db.Where("1 = 1").Delete(&QuantPosition{})
	db.Where("user_id = ?", QUANT_USER_ID).Delete(&PortfolioPosition{})
	db.Where("bot = ?", "quant").Delete(&BotTodo{})
	db.Where("bot = ?", "quant").Delete(&BotLog{})
	c.JSON(http.StatusOK, gin.H{"message": "Quant reset complete"})
}

// getLastQuantRefresh returns the last quant refresh info with logs
func getLastQuantRefresh(c *gin.Context) {
	var setting SystemSetting
	if err := db.Where("key = ?", "last_quant_refresh").First(&setting).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"updated_at":   nil,
			"triggered_by": nil,
			"logs":         []interface{}{},
		})
		return
	}

	var lastRefresh map[string]interface{}
	if err := json.Unmarshal([]byte(setting.Value), &lastRefresh); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"updated_at":   setting.UpdatedAt,
			"triggered_by": "unknown",
			"logs":         []interface{}{},
		})
		return
	}

	c.JSON(http.StatusOK, lastRefresh)
}

// cleanupQuantPending deletes all pending trades and positions, and all todos
func cleanupQuantPending(c *gin.Context) {
	// Delete pending trades
	result1 := db.Where("is_pending = ?", true).Delete(&QuantTrade{})
	// Delete pending positions
	result2 := db.Where("is_pending = ?", true).Delete(&QuantPosition{})
	// Delete all todos for quant bot
	result3 := db.Where("bot = ?", "quant").Delete(&BotTodo{})

	c.JSON(http.StatusOK, gin.H{
		"message":             "Cleanup complete",
		"deleted_trades":      result1.RowsAffected,
		"deleted_positions":   result2.RowsAffected,
		"deleted_todos":       result3.RowsAffected,
	})
}

func quantBackfill(c *gin.Context) {
	var req struct {
		UntilDate string `json:"until_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "until_date required"})
		return
	}

	fromDate, err := time.Parse("2006-01-02", req.UntilDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format (use YYYY-MM-DD)"})
		return
	}

	now := time.Now()
	sessionID := uuid.New().String()
	var logs []map[string]interface{}
	addLog := func(level, message string) {
		entry := map[string]interface{}{
			"level":   level,
			"message": message,
			"time":    time.Now().Format("15:04:05"),
		}
		logs = append(logs, entry)
		saveBotLog("quant", level, message, sessionID)
	}

	addLog("INFO", fmt.Sprintf("Quant Backfill gestartet ab %s bis heute", req.UntilDate))

	// Get all tracked stocks with their quant performance data
	var trackedStocks []QuantStockPerformance
	db.Find(&trackedStocks)

	if len(trackedStocks) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message":        "No tracked stocks found",
			"trades_created": 0,
			"logs":           logs,
		})
		return
	}

	var tradesCreated int
	var positionsCreated int

	for _, stock := range trackedStocks {
		if stock.TradesJSON == "" {
			continue
		}

		// Check allowlist
		if !isStockAllowedForBot("quant", stock.Symbol) {
			addLog("SKIP", fmt.Sprintf("%s: Nicht in Allowlist — übersprungen", stock.Symbol))
			continue
		}

		var historicalTrades []TradeData
		if err := json.Unmarshal([]byte(stock.TradesJSON), &historicalTrades); err != nil {
			addLog("ERROR", fmt.Sprintf("%s: Fehler beim Parsen der Trades: %v", stock.Symbol, err))
			continue
		}

		// Check if there's already an open position from BEFORE the backfill start date
		hasOpenPositionBefore := false
		for _, t := range historicalTrades {
			entryT := time.Unix(t.EntryDate, 0)
			if t.IsOpen && entryT.Before(fromDate) {
				hasOpenPositionBefore = true
				break
			}
		}
		if hasOpenPositionBefore {
			addLog("SKIP", fmt.Sprintf("%s: Offene Position vor Startdatum (HOLD) — übersprungen", stock.Symbol))
			continue
		}

		for _, trade := range historicalTrades {
			entryTime := time.Unix(trade.EntryDate, 0).UTC()
			entryTime = time.Date(entryTime.Year(), entryTime.Month(), 1, 0, 0, 0, 0, time.UTC)

			if entryTime.Year() < 2020 || entryTime.Year() > 2030 {
				continue
			}
			if entryTime.Before(fromDate) {
				continue
			}
			if entryTime.After(now) {
				continue
			}

			var existingBuy QuantTrade
			dateStart := entryTime.Truncate(24 * time.Hour)
			dateEnd := dateStart.Add(24 * time.Hour)
			alreadyExists := db.Where("symbol = ? AND action = ? AND signal_date >= ? AND signal_date < ?",
				stock.Symbol, "BUY", dateStart, dateEnd).First(&existingBuy).Error == nil
			if alreadyExists {
				continue
			}

			investmentEUR := 100.0
			investmentUSD := convertToUSD(investmentEUR, "EUR")
			qty := math.Round((investmentUSD/trade.EntryPrice)*1000000) / 1000000
			if qty <= 0 || trade.EntryPrice <= 0 {
				continue
			}

			buyTrade := QuantTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      trade.EntryPrice,
				SignalDate: entryTime,
				ExecutedAt: entryTime,
				IsPending:  false,
			}
			db.Create(&buyTrade)
			tradesCreated++
			addLog("ACTION", fmt.Sprintf("%s: BUY erstellt @ $%.2f am %s", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))

			if trade.ExitDate != nil && trade.ExitPrice != nil {
				exitTime := time.Unix(*trade.ExitDate, 0).UTC()
				exitTime = time.Date(exitTime.Year(), exitTime.Month(), 1, 0, 0, 0, 0, time.UTC)

				if !exitTime.After(now) {
					profitLoss := (*trade.ExitPrice - trade.EntryPrice) * qty
					profitLossPct := trade.ReturnPct

					sellTrade := QuantTrade{
						Symbol:        stock.Symbol,
						Name:          stock.Name,
						Action:        "SELL",
						Quantity:      qty,
						Price:         *trade.ExitPrice,
						SignalDate:    exitTime,
						ExecutedAt:    exitTime,
						IsPending:     false,
						ProfitLoss:    &profitLoss,
						ProfitLossPct: &profitLossPct,
					}
					db.Create(&sellTrade)
					tradesCreated++
					addLog("ACTION", fmt.Sprintf("%s: SELL erstellt @ $%.2f am %s (%.2f%%)", stock.Symbol, *trade.ExitPrice, exitTime.Format("2006-01-02"), profitLossPct))
				} else {
					var existingPos QuantPosition
					if db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error != nil {
						newPos := QuantPosition{
							Symbol:      stock.Symbol,
							Name:        stock.Name,
							Quantity:    qty,
							AvgPrice:    trade.EntryPrice,
							InvestedEUR: investmentEUR,
							BuyDate:     entryTime,
							IsPending:   false,
						}
						db.Create(&newPos)
						positionsCreated++

						portfolioPos := PortfolioPosition{
							UserID:       QUANT_USER_ID,
							Symbol:       stock.Symbol,
							Name:         stock.Name,
							PurchaseDate: &entryTime,
							AvgPrice:     trade.EntryPrice,
							Currency:     "USD",
							Quantity:     &qty,
						}
						db.Create(&portfolioPos)
						addLog("ACTION", fmt.Sprintf("%s: Position erstellt (offen)", stock.Symbol))
					}
				}
			} else if trade.IsOpen {
				var existingPos QuantPosition
				if db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error != nil {
					newPos := QuantPosition{
						Symbol:      stock.Symbol,
						Name:        stock.Name,
						Quantity:    qty,
						AvgPrice:    trade.EntryPrice,
						InvestedEUR: investmentEUR,
						BuyDate:     entryTime,
						IsPending:   false,
					}
					db.Create(&newPos)
					positionsCreated++

					portfolioPos := PortfolioPosition{
						UserID:       QUANT_USER_ID,
						Symbol:       stock.Symbol,
						Name:         stock.Name,
						PurchaseDate: &entryTime,
						AvgPrice:     trade.EntryPrice,
						Currency:     "USD",
						Quantity:     &qty,
					}
					db.Create(&portfolioPos)
					addLog("ACTION", fmt.Sprintf("%s: Position erstellt (offen)", stock.Symbol))
				}
			}
		}
	}

	addLog("INFO", fmt.Sprintf("Quant Backfill abgeschlossen: %d Trades, %d Positionen erstellt", tradesCreated, positionsCreated))

	c.JSON(http.StatusOK, gin.H{
		"message":           "Backfill completed",
		"trades_created":    tradesCreated,
		"positions_created": positionsCreated,
		"until_date":        req.UntilDate,
		"logs":              logs,
	})
}

func getQuantCompletedTrades(c *gin.Context) {
	var trades []QuantTrade
	db.Where("action = ? AND profit_loss IS NOT NULL AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false).Order("executed_at desc").Find(&trades)

	type CompletedTrade struct {
		Symbol        string     `json:"symbol"`
		Name          string     `json:"name"`
		BuyPrice      float64    `json:"buy_price"`
		SellPrice     float64    `json:"sell_price"`
		BuyDate       time.Time  `json:"buy_date"`
		SellDate      time.Time  `json:"sell_date"`
		ProfitLoss    float64    `json:"profit_loss"`
		ProfitLossPct float64    `json:"profit_loss_pct"`
		IsLive        bool       `json:"is_live"`
	}

	var result []CompletedTrade
	for _, sell := range trades {
		var buy QuantTrade
		if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND is_filter_blocked = ? AND executed_at < ?", sell.Symbol, "BUY", false, false, sell.ExecutedAt).Order("executed_at desc").First(&buy).Error; err != nil {
			continue
		}

		ct := CompletedTrade{
			Symbol:    sell.Symbol,
			Name:      sell.Name,
			BuyPrice:  buy.Price,
			SellPrice: sell.Price,
			BuyDate:   buy.ExecutedAt,
			SellDate:  sell.ExecutedAt,
			IsLive:    sell.IsLive,
		}
		if sell.ProfitLoss != nil {
			ct.ProfitLoss = *sell.ProfitLoss
		}
		if sell.ProfitLossPct != nil {
			ct.ProfitLossPct = *sell.ProfitLossPct
		}
		result = append(result, ct)
	}

	c.JSON(http.StatusOK, result)
}

func updateQuantPosition(c *gin.Context) {
	id := c.Param("id")

	var position QuantPosition
	if err := db.First(&position, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found"})
		return
	}

	var req struct {
		IsLive          *bool    `json:"is_live"`
		AvgPrice        *float64 `json:"avg_price"`
		InvestedEUR     *float64 `json:"invested_eur"`
		StopLossPercent *float64 `json:"stop_loss_percent"`
		StopLossType    *string  `json:"stop_loss_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.IsLive != nil {
		position.IsLive = *req.IsLive
	}
	if req.AvgPrice != nil {
		position.AvgPrice = *req.AvgPrice
	}
	if req.InvestedEUR != nil {
		position.InvestedEUR = *req.InvestedEUR
	}
	if req.StopLossPercent != nil {
		if *req.StopLossPercent <= 0 {
			position.StopLossPercent = nil
		} else {
			position.StopLossPercent = req.StopLossPercent
		}
	}
	if req.StopLossType != nil {
		position.StopLossType = *req.StopLossType
	}

	db.Save(&position)

	var portfolioPos PortfolioPosition
	if err := db.Where("user_id = ? AND symbol = ?", QUANT_USER_ID, position.Symbol).First(&portfolioPos).Error; err == nil {
		portfolioPos.AvgPrice = position.AvgPrice
		db.Save(&portfolioPos)
	}

	c.JSON(http.StatusOK, position)
}

func updateQuantTrade(c *gin.Context) {
	id := c.Param("id")

	var trade QuantTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	var req struct {
		IsLive     *bool      `json:"is_live"`
		Price      *float64   `json:"price"`
		Quantity   *float64   `json:"quantity"`
		SignalDate *time.Time `json:"signal_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.IsLive != nil {
		trade.IsLive = *req.IsLive
	}
	if req.Price != nil {
		trade.Price = *req.Price
	}
	if req.Quantity != nil {
		trade.Quantity = *req.Quantity
	}
	if req.SignalDate != nil {
		trade.SignalDate = *req.SignalDate
		trade.ExecutedAt = *req.SignalDate
	}

	db.Save(&trade)

	// Sync changes to matching position and portfolio entry
	if trade.Action == "BUY" {
		var position QuantPosition
		if err := db.Where("symbol = ? AND is_closed = ?", trade.Symbol, false).First(&position).Error; err == nil {
			if req.IsLive != nil {
				position.IsLive = *req.IsLive
			}
			if req.Price != nil {
				position.AvgPrice = *req.Price
			}
			if req.Quantity != nil {
				position.Quantity = *req.Quantity
			}
			if req.SignalDate != nil {
				position.BuyDate = *req.SignalDate
			}
			db.Save(&position)

			// Also update portfolio position
			var portfolioPos PortfolioPosition
			if err := db.Where("user_id = ? AND symbol = ?", QUANT_USER_ID, trade.Symbol).First(&portfolioPos).Error; err == nil {
				if req.Price != nil {
					portfolioPos.AvgPrice = *req.Price
				}
				if req.Quantity != nil {
					portfolioPos.Quantity = req.Quantity
				}
				if req.SignalDate != nil {
					portfolioPos.PurchaseDate = req.SignalDate
				}
				db.Save(&portfolioPos)
			}
		}
	} else if trade.Action == "SELL" && req.Price != nil {
		var position QuantPosition
		if err := db.Where("symbol = ? AND is_closed = ? AND is_live = ?", trade.Symbol, true, trade.IsLive).Order("updated_at desc").First(&position).Error; err == nil {
			position.SellPrice = *req.Price
			pnl := (*req.Price - position.AvgPrice) * position.Quantity
			pnlPct := ((*req.Price - position.AvgPrice) / position.AvgPrice) * 100
			position.ProfitLoss = &pnl
			position.ProfitLossPct = &pnlPct
			db.Save(&position)
		}
	}

	c.JSON(http.StatusOK, trade)
}

func createManualQuantTrade(c *gin.Context) {
	var req struct {
		Symbol   string  `json:"symbol" binding:"required"`
		Name     string  `json:"name"`
		Action   string  `json:"action" binding:"required"` // BUY or SELL
		Price    float64 `json:"price" binding:"required"`
		Quantity float64 `json:"quantity"`
		Date     string  `json:"date"` // YYYY-MM-DD
		IsLive   bool    `json:"is_live"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Symbol, Action und Price sind Pflichtfelder"})
		return
	}

	if req.Action != "BUY" && req.Action != "SELL" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Action muss BUY oder SELL sein"})
		return
	}

	// Parse date or use now
	signalDate := time.Now()
	if req.Date != "" {
		parsed, err := time.Parse("2006-01-02", req.Date)
		if err == nil {
			signalDate = parsed
		}
	}

	// Default quantity: 100 EUR worth
	qty := req.Quantity
	if qty <= 0 {
		investmentEUR := 100.0
		investmentUSD := convertToUSD(investmentEUR, "EUR")
		qty = math.Round((investmentUSD/req.Price)*1000000) / 1000000
		if qty <= 0 {
			qty = 1
		}
	}

	// Resolve name if not provided
	name := req.Name
	if name == "" {
		name = req.Symbol
	}

	if req.Action == "BUY" {
		// Check for existing open position
		var existingPos QuantPosition
		if err := db.Where("symbol = ? AND is_closed = ?", req.Symbol, false).First(&existingPos).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Offene Position für %s existiert bereits", req.Symbol)})
			return
		}

		trade := QuantTrade{
			Symbol:     req.Symbol,
			Name:       name,
			Action:     "BUY",
			Quantity:   qty,
			Price:      req.Price,
			SignalDate: signalDate,
			ExecutedAt: signalDate,
			IsPending:  false,
			IsLive:     req.IsLive,
		}
		db.Create(&trade)

		investmentEUR := 100.0
		if req.Quantity > 0 {
			investmentEUR = req.Price * req.Quantity / convertToUSD(1.0, "EUR")
		}

		pos := QuantPosition{
			Symbol:      req.Symbol,
			Name:        name,
			Quantity:    qty,
			AvgPrice:    req.Price,
			InvestedEUR: investmentEUR,
			BuyDate:     signalDate,
			IsPending:   false,
			IsLive:      req.IsLive,
		}
		db.Create(&pos)

		portfolioPos := PortfolioPosition{
			UserID:       QUANT_USER_ID,
			Symbol:       req.Symbol,
			Name:         name,
			PurchaseDate: &signalDate,
			AvgPrice:     req.Price,
			Currency:     "USD",
			Quantity:     &qty,
		}
		db.Create(&portfolioPos)

		c.JSON(http.StatusOK, gin.H{"trade": trade, "position": pos})

	} else {
		// SELL - must have existing open position
		var existingPos QuantPosition
		if err := db.Where("symbol = ? AND is_closed = ?", req.Symbol, false).First(&existingPos).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Keine offene Position für %s vorhanden", req.Symbol)})
			return
		}

		sellQty := qty
		if req.Quantity <= 0 {
			sellQty = existingPos.Quantity
		}

		trade := QuantTrade{
			Symbol:     req.Symbol,
			Name:       name,
			Action:     "SELL",
			Quantity:   sellQty,
			Price:      req.Price,
			SignalDate: signalDate,
			ExecutedAt: signalDate,
			IsPending:  false,
			IsLive:     existingPos.IsLive,
		}

		pnl := (req.Price - existingPos.AvgPrice) * sellQty
		pnlPct := ((req.Price - existingPos.AvgPrice) / existingPos.AvgPrice) * 100
		trade.ProfitLoss = &pnl
		trade.ProfitLossPct = &pnlPct
		db.Create(&trade)

		// Close position instead of deleting
		existingPos.IsClosed = true
		existingPos.SellPrice = req.Price
		existingPos.SellDate = &signalDate
		existingPos.ProfitLoss = &pnl
		existingPos.ProfitLossPct = &pnlPct
		existingPos.UpdatedAt = time.Now()
		db.Save(&existingPos)
		db.Where("user_id = ? AND symbol = ?", QUANT_USER_ID, req.Symbol).Delete(&PortfolioPosition{})

		c.JSON(http.StatusOK, gin.H{"trade": trade})
	}
}

func deleteQuantTrade(c *gin.Context) {
	id := c.Param("id")

	var trade QuantTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	symbol := trade.Symbol
	wasDeleted := trade.IsDeleted

	// Toggle soft-delete
	trade.IsDeleted = !wasDeleted
	db.Save(&trade)

	if trade.Action == "BUY" {
		if !wasDeleted {
			// Soft-deleting a BUY → also soft-delete matching SELL, hard-delete position + portfolio
			var sellTrade QuantTrade
			if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "SELL", false).
				Order("signal_date desc").First(&sellTrade).Error; err == nil {
				sellTrade.IsDeleted = true
				db.Save(&sellTrade)
			}
			db.Where("symbol = ? AND is_live = ?", symbol, trade.IsLive).Delete(&QuantPosition{})
			db.Where("user_id = ? AND symbol = ?", QUANT_USER_ID, symbol).Delete(&PortfolioPosition{})
		} else {
			// Restoring a BUY → also restore matching SELL, recreate position
			var sellTrade QuantTrade
			hasSell := false
			if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "SELL", true).
				Order("signal_date desc").First(&sellTrade).Error; err == nil {
				sellTrade.IsDeleted = false
				db.Save(&sellTrade)
				hasSell = true
			}

			qty := trade.Quantity
			buyDate := trade.SignalDate
			newPos := QuantPosition{
				Symbol:   symbol,
				Name:     trade.Name,
				Quantity: qty,
				AvgPrice: trade.Price,
				IsLive:   trade.IsLive,
				BuyDate:  buyDate,
			}

			if hasSell {
				newPos.IsClosed = true
				newPos.SellPrice = sellTrade.Price
				sellDate := sellTrade.SignalDate
				newPos.SellDate = &sellDate
				newPos.ProfitLoss = sellTrade.ProfitLoss
				newPos.ProfitLossPct = sellTrade.ProfitLossPct
				db.Create(&newPos)
			} else {
				db.Create(&newPos)
				portfolioPos := PortfolioPosition{
					UserID:       QUANT_USER_ID,
					Symbol:       symbol,
					Name:         trade.Name,
					AvgPrice:     trade.Price,
					PurchaseDate: &buyDate,
					Quantity:     &qty,
				}
				db.Create(&portfolioPos)
			}
		}
	} else if trade.Action == "SELL" {
		// Check if the corresponding BUY is deleted
		var buyTrade QuantTrade
		buyDeleted := false
		if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "BUY", true).
			Order("signal_date desc").First(&buyTrade).Error; err == nil {
			buyDeleted = true
		}

		if buyDeleted {
			// BUY is deleted → just toggle SELL, no position changes
		} else if !wasDeleted {
			// Soft-deleting a SELL (BUY active) → reopen position
			var pos QuantPosition
			if err := db.Where("symbol = ? AND is_live = ?", symbol, trade.IsLive).Order("updated_at desc").First(&pos).Error; err == nil {
				pos.IsClosed = false
				pos.SellPrice = 0
				pos.SellDate = nil
				pos.ProfitLoss = nil
				pos.ProfitLossPct = nil
				pos.UpdatedAt = time.Now()
				db.Save(&pos)

				qty := pos.Quantity
				buyDate := pos.BuyDate
				portfolioPos := PortfolioPosition{
					UserID:       QUANT_USER_ID,
					Symbol:       pos.Symbol,
					Name:         pos.Name,
					AvgPrice:     pos.AvgPrice,
					PurchaseDate: &buyDate,
					Quantity:     &qty,
				}
				db.Create(&portfolioPos)
			}
		} else {
			// Restoring a SELL (BUY active) → re-close position
			var pos QuantPosition
			if err := db.Where("symbol = ? AND is_live = ? AND is_closed = ?", symbol, trade.IsLive, false).First(&pos).Error; err == nil {
				pos.IsClosed = true
				pos.SellPrice = trade.Price
				sellDate := trade.SignalDate
				pos.SellDate = &sellDate
				pos.ProfitLoss = trade.ProfitLoss
				pos.ProfitLossPct = trade.ProfitLossPct
				pos.UpdatedAt = time.Now()
				db.Save(&pos)
			}
			db.Where("user_id = ? AND symbol = ?", QUANT_USER_ID, symbol).Delete(&PortfolioPosition{})
		}
	}

	action := "deleted"
	if wasDeleted {
		action = "restored"
	}
	c.JSON(http.StatusOK, gin.H{"message": "Trade " + action, "trade": trade})
}

func toggleQuantTradeRead(c *gin.Context) {
	id := c.Param("id")
	var trade QuantTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}
	trade.IsRead = !trade.IsRead
	db.Save(&trade)
	c.JSON(http.StatusOK, gin.H{"trade": trade})
}

func markAllQuantTradesRead(c *gin.Context) {
	db.Model(&QuantTrade{}).Where("is_read = ? AND is_pending = ?", false, false).Update("is_read", true)
	c.JSON(http.StatusOK, gin.H{"message": "All trades marked as read"})
}

func markAllQuantTradesUnread(c *gin.Context) {
	db.Model(&QuantTrade{}).Where("is_read = ? AND is_pending = ?", true, false).Update("is_read", false)
	c.JSON(http.StatusOK, gin.H{"message": "All trades marked as unread"})
}

func getQuantUnreadCount(c *gin.Context) {
	var count int64
	db.Model(&QuantTrade{}).Where("is_read = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false, false).Count(&count)

	// Also get the unread trades for notification details
	var unreadTrades []QuantTrade
	db.Where("is_read = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false, false).Order("executed_at desc").Limit(10).Find(&unreadTrades)

	c.JSON(http.StatusOK, gin.H{"count": count, "trades": unreadTrades})
}

func getQuantPending(c *gin.Context) {
	var positions []QuantPosition
	db.Where("is_pending = ?", true).Find(&positions)

	type PendingPosition struct {
		QuantPosition
		CurrentPrice   float64 `json:"current_price"`
		TotalReturnPct float64 `json:"total_return_pct"`
	}

	symbols := make([]string, 0, len(positions))
	for _, p := range positions {
		symbols = append(symbols, p.Symbol)
	}
	quotes := fetchQuotes(symbols)

	var result []PendingPosition
	for _, pos := range positions {
		pp := PendingPosition{QuantPosition: pos}
		if quote, ok := quotes[pos.Symbol]; ok {
			pp.CurrentPrice = quote.Price
			if pos.AvgPrice > 0 {
				pp.TotalReturnPct = ((quote.Price - pos.AvgPrice) / pos.AvgPrice) * 100
			}
		}
		result = append(result, pp)
	}

	c.JSON(http.StatusOK, result)
}

func getQuantLogs(c *gin.Context) {
	var logs []BotLog
	db.Where("bot = ?", "quant").Order("created_at desc").Limit(200).Find(&logs)
	c.JSON(http.StatusOK, logs)
}

func getQuantTodos(c *gin.Context) {
	var todos []BotTodo
	db.Where("bot = ? AND done = ?", "quant", false).Order("created_at desc").Find(&todos)
	c.JSON(http.StatusOK, todos)
}

func markQuantTodoDone(c *gin.Context) {
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

func reopenQuantTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	todo.Done = false
	todo.Decision = ""
	todo.DoneAt = nil
	db.Save(&todo)
	c.JSON(http.StatusOK, todo)
}

func deleteQuantTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	db.Delete(&todo)
	c.JSON(http.StatusOK, gin.H{"message": "Todo deleted"})
}

func executeQuantTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	var req struct {
		IsLive      bool     `json:"is_live"`
		Price       *float64 `json:"price"`
		InvestedEUR *float64 `json:"invested_eur"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	now := time.Now()
	price := todo.Price
	if req.Price != nil {
		price = *req.Price
	}

	if todo.Type == "BUY" {
		var existingPos QuantPosition
		if err := db.Where("symbol = ? AND is_closed = ?", todo.Symbol, false).First(&existingPos).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Position already exists"})
			return
		}

		// Calculate quantity based on invested EUR (default 100 EUR)
		investmentEUR := 100.0
		if req.InvestedEUR != nil && *req.InvestedEUR > 0 {
			investmentEUR = *req.InvestedEUR
		}
		investmentUSD := convertToUSD(investmentEUR, "EUR")
		qty := math.Round((investmentUSD/price)*1000000) / 1000000
		if qty <= 0 {
			qty = 1
		}

		newTrade := QuantTrade{
			Symbol:     todo.Symbol,
			Name:       todo.Name,
			Action:     "BUY",
			Quantity:   qty,
			Price:      price,
			SignalDate: todo.CreatedAt,
			ExecutedAt: now,
			IsLive:     req.IsLive,
			IsPending:  false,
		}
		db.Create(&newTrade)

		newPosition := QuantPosition{
			Symbol:      todo.Symbol,
			Name:        todo.Name,
			Quantity:    qty,
			AvgPrice:    price,
			IsLive:      req.IsLive,
			IsPending:   false,
			BuyDate:     now,
			InvestedEUR: investmentEUR,
		}
		db.Create(&newPosition)

		portfolioPos := PortfolioPosition{
			UserID:       QUANT_USER_ID,
			Symbol:       todo.Symbol,
			Name:         todo.Name,
			AvgPrice:     price,
			PurchaseDate: &now,
			Quantity:     &qty,
		}
		db.Create(&portfolioPos)

	} else if todo.Type == "SELL" {
		var position QuantPosition
		if err := db.Where("symbol = ? AND is_closed = ?", todo.Symbol, false).First(&position).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Position not found"})
			return
		}

		pnl := price - position.AvgPrice
		pnlPct := (pnl / position.AvgPrice) * 100

		newTrade := QuantTrade{
			Symbol:        todo.Symbol,
			Name:          todo.Name,
			Action:        "SELL",
			Price:         price,
			SignalDate:    todo.CreatedAt,
			ExecutedAt:    now,
			IsLive:        position.IsLive,
			IsPending:     false,
			ProfitLoss:    &pnl,
			ProfitLossPct: &pnlPct,
		}
		db.Create(&newTrade)

		// Close position instead of deleting
		position.IsClosed = true
		position.SellPrice = price
		position.SellDate = &now
		position.ProfitLoss = &pnl
		position.ProfitLossPct = &pnlPct
		position.UpdatedAt = time.Now()
		db.Save(&position)
		db.Where("user_id = ? AND symbol = ?", QUANT_USER_ID, todo.Symbol).Delete(&PortfolioPosition{})
	}

	todo.Done = true
	todo.Decision = "executed"
	todo.DoneAt = &now
	db.Save(&todo)

	c.JSON(http.StatusOK, gin.H{"message": "Todo executed", "todo": todo})
}

func syncQuant(c *gin.Context) {
	var positions []QuantPosition
	db.Where("is_closed = ?", false).Find(&positions)

	for _, pos := range positions {
		var existingPosition QuantPosition
		if err := db.Where("symbol = ?", pos.Symbol).First(&existingPosition).Error; err != nil {
			continue
		}

		if pos.AvgPrice > 0 {
			var existingBuy QuantTrade
			if err := db.Where("symbol = ? AND action = ?", pos.Symbol, "BUY").Order("executed_at desc").First(&existingBuy).Error; err == nil {
				var lastSell QuantTrade
				if err := db.Where("symbol = ? AND action = ? AND executed_at > ?", pos.Symbol, "SELL", existingBuy.ExecutedAt).First(&lastSell).Error; err != nil {
					if existingBuy.Price != pos.AvgPrice {
						existingBuy.Price = pos.AvgPrice
						db.Save(&existingBuy)
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Quant sync complete"})
}

func getQuantHistory(c *gin.Context) {
	period := c.DefaultQuery("period", "1m")
	live := c.DefaultQuery("live", "true")

	botType := "quant-live"
	if live == "false" {
		botType = "quant-sim"
	}

	history := calculateBotHistory(botType, period)
	c.JSON(http.StatusOK, history)
}

func getQuantPendingTrades(c *gin.Context) {
	var trades []QuantTrade
	db.Where("is_pending = ?", true).Order("executed_at desc").Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func acceptQuantTrade(c *gin.Context) {
	id := c.Param("id")
	var trade QuantTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	trade.IsPending = false
	db.Save(&trade)

	if trade.Action == "BUY" {
		db.Model(&QuantPosition{}).Where("symbol = ? AND is_pending = ?", trade.Symbol, true).Update("is_pending", false)
	}

	c.JSON(http.StatusOK, trade)
}

// getQuantPrivatePortfolio returns only live/private positions (is_live = true)
// getQuantSimulatedPortfolio returns simulated/test positions (is_live = false) for Admin view
func getQuantSimulatedPortfolio(c *gin.Context) {
	// Show ALL open positions (both live and simulated) - live ones are marked with is_live badge
	var positions []QuantPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Order("buy_date desc").Find(&positions)

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

	// Realisierte Gewinne aus geschlossenen Trades einrechnen
	var closedSellTrades []QuantTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ?", "SELL", false, false).Find(&closedSellTrades)

	realizedPL := 0.0
	totalClosedInvested := 0.0
	for _, trade := range closedSellTrades {
		if trade.ProfitLoss != nil {
			realizedPL += *trade.ProfitLoss
			totalClosedInvested += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}

	overallReturn := totalReturn + realizedPL
	overallInvested := totalInvested + totalClosedInvested
	overallReturnPct := 0.0
	if overallInvested > 0 {
		overallReturnPct = (overallReturn / overallInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"positions":          result,
		"total_value":        totalValue,
		"total_invested":     totalInvested,
		"total_return":       totalReturn,
		"total_return_pct":   overallReturnPct,
		"realized_pl":        realizedPL,
		"overall_return":     overallReturn,
		"overall_invested":   overallInvested,
	})
}

// getQuantPrivatePerformance returns performance stats for only live/private trades
// getQuantSimulatedPerformance returns performance stats for simulated/test trades (is_live = false) for Admin view
func getQuantSimulatedPerformance(c *gin.Context) {
	var sellTrades []QuantTrade
	db.Where("action = ? AND is_pending = ? AND is_live = ? AND is_deleted = ? AND is_admin_closed = ? AND is_filter_blocked = ?", "SELL", false, false, false, false, false).Find(&sellTrades)

	var buyTrades []QuantTrade
	db.Where("action = ? AND is_pending = ? AND is_live = ? AND is_deleted = ? AND is_admin_closed = ? AND is_filter_blocked = ?", "BUY", false, false, false, false, false).Find(&buyTrades)

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

	totalReturnPctClosed := 0.0
	for _, trade := range sellTrades {
		if trade.ProfitLossPct != nil {
			totalReturnPctClosed += *trade.ProfitLossPct
		}
	}
	avgReturnPerTrade := 0.0
	if len(sellTrades) > 0 {
		avgReturnPerTrade = totalReturnPctClosed / float64(len(sellTrades))
	}

	// Get open positions for unrealized P/L (simulated trades)
	var positions []QuantPosition
	db.Where("is_pending = ? AND is_live = ? AND is_closed = ?", false, false, false).Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	unrealizedGain := 0.0
	investedInPositions := 0.0
	currentValue := 0.0

	for _, pos := range positions {
		quote := quotes[pos.Symbol]
		currentPrice := quote.Price
		if currentPrice <= 0 {
			currentPrice = pos.AvgPrice
		}
		investedInPositions += pos.AvgPrice * pos.Quantity
		currentValue += currentPrice * pos.Quantity
		unrealizedGain += (currentPrice - pos.AvgPrice) * pos.Quantity
	}

	totalReturnPct := 0.0
	if investedInPositions > 0 {
		totalReturnPct = (unrealizedGain / investedInPositions) * 100
	}

	totalGain := totalProfitLoss + unrealizedGain
	totalInvestedAll := investedInPositions
	for _, trade := range sellTrades {
		if trade.ProfitLoss != nil {
			totalInvestedAll += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}
	overallReturnPct := 0.0
	if totalInvestedAll > 0 {
		overallReturnPct = (totalGain / totalInvestedAll) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_trades":          len(buyTrades) + len(sellTrades),
		"total_buys":            len(buyTrades),
		"open_positions":        len(positions),
		"closed_trades":         len(sellTrades),
		"wins":                  wins,
		"losses":                losses,
		"win_rate":              winRate,
		"realized_profit":       totalProfitLoss,
		"avg_return_per_trade":  avgReturnPerTrade,
		"unrealized_gain":       unrealizedGain,
		"invested_in_positions": investedInPositions,
		"current_value":         currentValue,
		"total_gain":            totalGain,
		"total_return_pct":      totalReturnPct,
		"overall_return_pct":    overallReturnPct,
	})
}

// ==================== Full Stock Update System ====================

// getLastFullUpdate returns the last full update info
func getLastFullUpdate(c *gin.Context) {
	var setting SystemSetting
	if err := db.Where("key = ?", "last_full_update").First(&setting).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"updated_at":   nil,
			"triggered_by": nil,
			"stocks_count": 0,
			"success":      0,
			"failed":       0,
		})
		return
	}

	var lastUpdate LastFullUpdate
	if err := json.Unmarshal([]byte(setting.Value), &lastUpdate); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"updated_at":   setting.UpdatedAt,
			"triggered_by": "unknown",
			"stocks_count": 0,
			"success":      0,
			"failed":       0,
		})
		return
	}

	c.JSON(http.StatusOK, lastUpdate)
}

// recordFullUpdate records that a full update was completed (called by frontend)
func recordFullUpdate(c *gin.Context) {
	var req struct {
		TriggeredBy string `json:"triggered_by"`
		StocksCount int    `json:"stocks_count"`
		Success     int    `json:"success"`
		Failed      int    `json:"failed"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Get username from session if triggered_by is empty
	triggeredBy := req.TriggeredBy
	if triggeredBy == "" {
		if userID, exists := c.Get("userID"); exists {
			var user User
			if err := db.First(&user, userID).Error; err == nil {
				triggeredBy = user.Username
			}
		}
	}
	if triggeredBy == "" {
		triggeredBy = "admin"
	}

	lastUpdate := LastFullUpdate{
		UpdatedAt:   time.Now(),
		TriggeredBy: triggeredBy,
		StocksCount: req.StocksCount,
		Success:     req.Success,
		Failed:      req.Failed,
	}

	valueJSON, _ := json.Marshal(lastUpdate)

	var setting SystemSetting
	if err := db.Where("key = ?", "last_full_update").First(&setting).Error; err != nil {
		setting = SystemSetting{
			Key:       "last_full_update",
			Value:     string(valueJSON),
			UpdatedAt: time.Now(),
		}
		db.Create(&setting)
	} else {
		setting.Value = string(valueJSON)
		setting.UpdatedAt = time.Now()
		db.Save(&setting)
	}

	c.JSON(http.StatusOK, lastUpdate)
}

// runFullUpdateHandler triggers a server-side full stock update
func runFullUpdateHandler(c *gin.Context) {
	// This endpoint starts the update process
	go runFullStockUpdate("system")
	c.JSON(http.StatusOK, gin.H{"status": "started", "message": "Full update started in background"})
}

// getSchedulerTime reads the configured scheduler time from DB, default "00:00"
func getSchedulerTime() (int, int) {
	var setting SystemSetting
	if err := db.Where("key = ?", "scheduler_time").First(&setting).Error; err == nil {
		parts := strings.Split(setting.Value, ":")
		if len(parts) == 2 {
			h, err1 := strconv.Atoi(parts[0])
			m, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil && h >= 0 && h <= 23 && m >= 0 && m <= 59 {
				return h, m
			}
		}
	}
	return 0, 0 // Default: midnight
}

// schedulerResetChan is used to signal the scheduler to recalculate the next run time
var schedulerResetChan = make(chan struct{}, 1)

// startDailyUpdateScheduler starts a goroutine that runs the full update daily at the configured time
func startDailyUpdateScheduler() {
	// Set timezone to Europe/Berlin
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		fmt.Printf("[Scheduler] WARNING: Could not load Europe/Berlin timezone: %v, using UTC\n", err)
		loc = time.UTC
	}

	fmt.Println("[Scheduler] Daily stock update scheduler started (TZ: Europe/Berlin)")

	for {
		hour, minute := getSchedulerTime()
		now := time.Now().In(loc)

		// Calculate next run time
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, loc)
		if now.After(nextRun) {
			nextRun = nextRun.AddDate(0, 0, 1)
		}
		duration := nextRun.Sub(now)

		fmt.Printf("[Scheduler] Next update scheduled at %s (in %v)\n", nextRun.Format("2006-01-02 15:04:05 MST"), duration)

		// Wait until scheduled time or reset signal
		select {
		case <-time.After(duration):
			fmt.Println("[Scheduler] Starting daily full stock update...")
			runFullStockUpdate("scheduler")
		case <-schedulerResetChan:
			fmt.Println("[Scheduler] Schedule time changed, recalculating...")
			continue
		}
	}
}

// getSchedulerTimeHandler returns the current scheduler time setting
func getSchedulerTimeHandler(c *gin.Context) {
	var setting SystemSetting
	schedulerTime := "00:00"
	if err := db.Where("key = ?", "scheduler_time").First(&setting).Error; err == nil {
		schedulerTime = setting.Value
	}
	c.JSON(http.StatusOK, gin.H{"time": schedulerTime})
}

// setSchedulerTimeHandler updates the scheduler time setting
func setSchedulerTimeHandler(c *gin.Context) {
	var req struct {
		Time string `json:"time"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Validate format HH:MM
	parts := strings.Split(req.Time, ":")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid time format, use HH:MM"})
		return
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid time, must be 00:00 - 23:59"})
		return
	}

	// Save to DB
	var setting SystemSetting
	if err := db.Where("key = ?", "scheduler_time").First(&setting).Error; err != nil {
		setting = SystemSetting{
			Key:       "scheduler_time",
			Value:     req.Time,
			UpdatedAt: time.Now(),
		}
		db.Create(&setting)
	} else {
		setting.Value = req.Time
		setting.UpdatedAt = time.Now()
		db.Save(&setting)
	}

	// Signal scheduler to recalculate
	select {
	case schedulerResetChan <- struct{}{}:
	default:
	}

	c.JSON(http.StatusOK, gin.H{"time": req.Time, "message": "Scheduler time updated"})
}

// runFullStockUpdate performs the full stock update for all watchlist stocks
func runFullStockUpdate(triggeredBy string) {
	fmt.Printf("[FullUpdate] Starting full stock update triggered by: %s\n", triggeredBy)

	// Get all stocks from watchlist, largest market cap first
	var stocks []Stock
	db.Order("market_cap desc").Find(&stocks)

	if len(stocks) == 0 {
		fmt.Println("[FullUpdate] No stocks in watchlist")
		return
	}

	// Get BXtrender configs
	var defensiveConfig, aggressiveConfig BXtrenderConfig
	db.Where("mode = ?", "defensive").First(&defensiveConfig)
	db.Where("mode = ?", "aggressive").First(&aggressiveConfig)

	// Get Quant config
	var quantConfig BXtrenderQuantConfig
	db.First(&quantConfig)

	// Get Ditz config
	var ditzConfig BXtrenderDitzConfig
	db.First(&ditzConfig)

	// Get Trader config
	var traderConfig BXtrenderTraderConfig
	db.First(&traderConfig)

	// Set defaults if not found
	if defensiveConfig.ID == 0 {
		defensiveConfig = BXtrenderConfig{ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15}
	}
	if aggressiveConfig.ID == 0 {
		aggressiveConfig = BXtrenderConfig{ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15}
	}
	if quantConfig.ID == 0 {
		quantConfig = BXtrenderQuantConfig{ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15, MaFilterOn: true, MaLength: 200, MaType: "EMA", TslPercent: 20.0}
	}
	if ditzConfig.ID == 0 {
		ditzConfig = BXtrenderDitzConfig{ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15, MaFilterOn: true, MaLength: 200, MaType: "EMA", TslPercent: 20.0}
	}
	if traderConfig.ID == 0 {
		traderConfig = BXtrenderTraderConfig{ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15, MaFilterOn: false, MaLength: 200, MaType: "EMA", TslPercent: 20.0}
	}

	successCount := 0
	failedCount := 0

	for i, stock := range stocks {
		fmt.Printf("[FullUpdate] Processing %d/%d: %s\n", i+1, len(stocks), stock.Symbol)

		err := processStockServer(stock.Symbol, stock.Name, defensiveConfig, aggressiveConfig, quantConfig, ditzConfig, traderConfig)
		if err != nil {
			fmt.Printf("[FullUpdate] Failed to process %s: %v\n", stock.Symbol, err)
			failedCount++
		} else {
			successCount++
		}

		// Rate limiting - wait 1.5 seconds between requests
		time.Sleep(1500 * time.Millisecond)
	}

	// Record the update
	lastUpdate := LastFullUpdate{
		UpdatedAt:   time.Now(),
		TriggeredBy: triggeredBy,
		StocksCount: len(stocks),
		Success:     successCount,
		Failed:      failedCount,
	}

	valueJSON, _ := json.Marshal(lastUpdate)

	var setting SystemSetting
	if err := db.Where("key = ?", "last_full_update").First(&setting).Error; err != nil {
		setting = SystemSetting{
			Key:       "last_full_update",
			Value:     string(valueJSON),
			UpdatedAt: time.Now(),
		}
		db.Create(&setting)
	} else {
		setting.Value = string(valueJSON)
		setting.UpdatedAt = time.Now()
		db.Save(&setting)
	}

	fmt.Printf("[FullUpdate] Completed! Success: %d, Failed: %d\n", successCount, failedCount)

	// After updating all stock performance data, run all bots to process new signals
	fmt.Println("[FullUpdate] Running FlipperBot update to process new signals...")
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[FullUpdate] FlipperBot update panicked: %v\n", r)
			}
		}()
		runFlipperUpdateInternal(triggeredBy)
	}()
	fmt.Println("[FullUpdate] FlipperBot update completed")

	fmt.Println("[FullUpdate] Running Lutz bot update to process new signals...")
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[FullUpdate] Lutz bot update panicked: %v\n", r)
			}
		}()
		runLutzUpdateInternal(triggeredBy)
	}()
	fmt.Println("[FullUpdate] Lutz bot update completed")

	fmt.Println("[FullUpdate] Running Quant bot update to process new signals...")
	runQuantUpdateInternal(triggeredBy)
	fmt.Println("[FullUpdate] Quant bot update completed")

	// Also run the Ditz bot to process new signals
	fmt.Println("[FullUpdate] Running Ditz bot update to process new signals...")
	runDitzUpdateInternal(triggeredBy)
	fmt.Println("[FullUpdate] Ditz bot update completed")

	// Run the Trader bot to process new signals
	fmt.Println("[FullUpdate] Running Trader bot update to process new signals...")
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[FullUpdate] Trader bot update panicked: %v\n", r)
			}
		}()
		runTraderUpdateInternal(triggeredBy)
	}()
	fmt.Println("[FullUpdate] Trader bot update completed")
}

// processStockServer processes a single stock and saves performance data
func processStockServer(symbol, name string, defensiveConfig, aggressiveConfig BXtrenderConfig, quantConfig BXtrenderQuantConfig, ditzConfig BXtrenderDitzConfig, traderConfig BXtrenderTraderConfig) error {
	// Fetch historical data
	data, err := fetchHistoricalDataServer(symbol)
	if err != nil {
		return fmt.Errorf("failed to fetch historical data: %v", err)
	}

	if len(data) < 50 {
		return fmt.Errorf("not enough data points: %d", len(data))
	}

	currentPrice := data[len(data)-1].Close
	latestPriceCache.Store(symbol, currentPrice)

	// Fetch market cap
	marketCap, _ := fetchMarketCapServer(symbol)

	// Nur abgeschlossene Monatskerzen verwenden (aktuellen unvollständigen Monat entfernen)
	monthlyData := data
	now := time.Now().UTC()
	// Vor dem Strippen: Open-Preis des aktuellen Monats erfassen (= Ausfuehrungspreis fuer Signale auf letzter Kerze)
	var nextBarOpen float64
	var nextBarTime int64
	// Strip ALL bars from the current month (Yahoo can return multiple bars for the current month)
	for len(monthlyData) > 0 {
		lastBar := time.Unix(monthlyData[len(monthlyData)-1].Time, 0).UTC()
		if lastBar.Year() == now.Year() && lastBar.Month() == now.Month() {
			// Immer ueberschreiben - die letzte gestripte Bar ist die frueheste im aktuellen Monat
			nextBarOpen = monthlyData[len(monthlyData)-1].Open
			nextBarTime = monthlyData[len(monthlyData)-1].Time
			monthlyData = monthlyData[:len(monthlyData)-1]
		} else {
			break
		}
	}

	// Calculate and save defensive mode
	defensiveResult := calculateBXtrenderServer(monthlyData, false, defensiveConfig, nextBarOpen, nextBarTime)
	defensiveMetrics := calculateMetricsServer(defensiveResult.Trades)
	savePerformanceServer(symbol, name, defensiveMetrics, defensiveResult, currentPrice, marketCap, false)

	// Calculate and save aggressive mode
	aggressiveResult := calculateBXtrenderServer(monthlyData, true, aggressiveConfig, nextBarOpen, nextBarTime)
	aggressiveMetrics := calculateMetricsServer(aggressiveResult.Trades)
	savePerformanceServer(symbol, name, aggressiveMetrics, aggressiveResult, currentPrice, marketCap, true)

	// Calculate and save quant mode
	quantResult := calculateBXtrenderQuantServer(monthlyData, quantConfig, nextBarOpen, nextBarTime)
	quantMetrics := calculateMetricsServer(quantResult.Trades)
	saveQuantPerformanceServer(symbol, name, quantMetrics, quantResult, currentPrice, marketCap)

	// Calculate and save ditz mode
	ditzResult := calculateBXtrenderDitzServer(monthlyData, ditzConfig, nextBarOpen, nextBarTime)
	ditzMetrics := calculateMetricsServer(ditzResult.Trades)
	saveDitzPerformanceServer(symbol, name, ditzMetrics, ditzResult, currentPrice, marketCap)

	// Calculate and save trader mode
	traderResult := calculateBXtrenderTraderServer(monthlyData, traderConfig, nextBarOpen, nextBarTime)
	traderMetrics := calculateMetricsServer(traderResult.Trades)
	saveTraderPerformanceServer(symbol, name, traderMetrics, traderResult, currentPrice, marketCap)

	return nil
}

// fetchHistoricalDataServer fetches historical OHLCV data from Yahoo Finance
func fetchHistoricalDataServer(symbol string) ([]OHLCV, error) {
	apiURL := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=max&interval=1mo",
		url.QueryEscape(symbol))

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var yahooResp YahooChartResponse
	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return nil, err
	}

	if len(yahooResp.Chart.Result) == 0 || len(yahooResp.Chart.Result[0].Timestamp) == 0 {
		return nil, fmt.Errorf("no data found")
	}

	// Check if Yahoo returned monthly data or something else
	actualGranularity := yahooResp.Chart.Result[0].Meta.DataGranularity
	if actualGranularity != "" && actualGranularity != "1mo" {
		fmt.Printf("[HistoryServer] %s: Monthly not available (got %s)\n", symbol, actualGranularity)

		// Fallback 1: Twelve Data API
		if twelveDataAPIKey != "" {
			tdData, err := fetchMonthlyFromTwelveData(symbol)
			if err == nil && len(tdData) > 0 {
				fmt.Printf("[HistoryServer] %s: Got %d monthly bars from Twelve Data\n", symbol, len(tdData))
				return tdData, nil
			}
			fmt.Printf("[HistoryServer] %s: Twelve Data fallback failed: %v\n", symbol, err)
		}

		// Fallback 2: Aggregate from daily/weekly
		fallbackData, err := fetchWeeklyAndAggregateToMonthly(symbol)
		if err == nil && len(fallbackData) > 0 {
			return fallbackData, nil
		}
		fmt.Printf("[HistoryServer] %s: Aggregation fallback also failed: %v, using original data\n", symbol, err)
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

	// Normalize Yahoo monthly timestamps to 1st of month 00:00 UTC
	for i := range data {
		t := time.Unix(data[i].Time, 0).UTC()
		normalized := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		data[i].Time = normalized.Unix()
	}

	return data, nil
}

// fetchMarketCapServer fetches market cap using Yahoo Quote API with crumb auth
func fetchMarketCapServer(symbol string) (int64, string) {
	// Try with cached crumb first, retry once if it fails
	for attempt := 0; attempt < 2; attempt++ {
		client, crumb, err := getYahooCrumbClient()
		if err != nil {
			fmt.Printf("[MarketCap] Crumb error for %s: %v\n", symbol, err)
			return 0, ""
		}

		quoteURL := fmt.Sprintf("https://query1.finance.yahoo.com/v7/finance/quote?symbols=%s&crumb=%s",
			url.QueryEscape(symbol), url.QueryEscape(crumb))

		req, _ := http.NewRequest("GET", quoteURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("[MarketCap] Request error for %s: %v\n", symbol, err)
			return 0, ""
		}
		defer resp.Body.Close()

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			// Crumb expired, reset and retry
			resetYahooCrumb()
			continue
		}

		body, _ := io.ReadAll(resp.Body)

		var quoteData map[string]interface{}
		if json.Unmarshal(body, &quoteData) == nil {
			if qr, ok := quoteData["quoteResponse"].(map[string]interface{}); ok {
				if result, ok := qr["result"].([]interface{}); ok && len(result) > 0 {
					if r, ok := result[0].(map[string]interface{}); ok {
						if mc, ok := r["marketCap"].(float64); ok && int64(mc) > 0 {
							return int64(mc), "quote-v7"
						}
					}
				}
			}
		}

		// If parsing failed but status was OK, don't retry
		if resp.StatusCode == 200 {
			fmt.Printf("[MarketCap] No marketCap in response for %s\n", symbol)
			return 0, ""
		}

		resetYahooCrumb()
	}

	return 0, ""
}

// testMarketCap handles GET /api/test-marketcap/:symbol
func testMarketCap(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Symbol required"})
		return
	}
	mc, source := fetchMarketCapServer(symbol)
	c.JSON(http.StatusOK, gin.H{
		"symbol":             symbol,
		"market_cap_raw":     mc,
		"market_cap_billions": float64(mc) / 1e9,
		"source":             source,
	})
}

// updateMarketCaps handles POST /api/update-marketcaps
func updateMarketCaps(c *gin.Context) {
	var symbols []string
	db.Table("stock_performances").Pluck("symbol", &symbols)

	tables := []string{
		"stock_performances",
		"aggressive_stock_performances",
		"quant_stock_performances",
		"ditz_stock_performances",
		"trader_stock_performances",
	}

	type detail struct {
		Symbol    string `json:"symbol"`
		MarketCap int64  `json:"market_cap"`
		Source    string `json:"source"`
	}

	updated := 0
	failed := 0
	details := []detail{}

	for _, sym := range symbols {
		mc, source := fetchMarketCapServer(sym)
		if mc > 0 {
			for _, table := range tables {
				db.Table(table).Where("symbol = ?", sym).Update("market_cap", mc)
			}
			db.Model(&Stock{}).Where("symbol = ?", sym).Update("market_cap", mc)
			updated++
		} else {
			failed++
		}
		details = append(details, detail{Symbol: sym, MarketCap: mc, Source: source})
		time.Sleep(200 * time.Millisecond) // Rate limiting
	}

	c.JSON(http.StatusOK, gin.H{
		"updated": updated,
		"failed":  failed,
		"total":   len(symbols),
		"details": details,
	})
}

func fetchISIN(symbol string) string {
	apiURL := fmt.Sprintf("https://query1.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=quoteType",
		url.QueryEscape(symbol))

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var data struct {
		QuoteSummary struct {
			Result []struct {
				QuoteType struct {
					ISIN string `json:"isin"`
				} `json:"quoteType"`
			} `json:"result"`
		} `json:"quoteSummary"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return ""
	}
	if len(data.QuoteSummary.Result) > 0 {
		return data.QuoteSummary.Result[0].QuoteType.ISIN
	}
	return ""
}

func getISIN(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Symbol required"})
		return
	}

	// Check all performance tables for cached ISIN
	tables := []string{
		"stock_performances",
		"aggressive_stock_performances",
		"quant_stock_performances",
		"ditz_stock_performances",
		"trader_stock_performances",
	}
	for _, table := range tables {
		var isin string
		row := db.Table(table).Select("isin").Where("symbol = ? AND isin != ''", symbol).Row()
		if row.Scan(&isin) == nil && isin != "" {
			c.JSON(http.StatusOK, gin.H{"symbol": symbol, "isin": isin})
			return
		}
	}

	// Also check stocks table
	var stock Stock
	if db.Where("symbol = ? AND isin != ''", symbol).First(&stock).Error == nil && stock.ISIN != "" {
		c.JSON(http.StatusOK, gin.H{"symbol": symbol, "isin": stock.ISIN})
		return
	}

	// Fetch from Yahoo Finance
	isin := fetchISIN(symbol)
	if isin != "" {
		// Cache in stocks table and all performance tables
		db.Model(&Stock{}).Where("symbol = ?", symbol).Update("isin", isin)
		for _, table := range tables {
			db.Table(table).Where("symbol = ?", symbol).Update("isin", isin)
		}
	}

	c.JSON(http.StatusOK, gin.H{"symbol": symbol, "isin": isin})
}

// BXtrender calculation structures
type BXtrenderResult struct {
	Short  []float64
	Long   []float64
	Signal string
	Bars   int
	Trades []ServerTrade
}

type ServerTrade struct {
	Type      string  `json:"type"` // BUY or SELL
	Time      int64   `json:"time"`
	Price     float64 `json:"price"`
	PrevPrice float64 `json:"prev_price,omitempty"`
	Return    float64 `json:"return,omitempty"`
}

type MetricsResult struct {
	WinRate     float64
	RiskReward  float64
	TotalReturn float64
	AvgReturn   float64
	TotalTrades int
	Wins        int
	Losses      int
}

// calculateEMAServer calculates Exponential Moving Average
func calculateEMAServer(data []float64, period int) []float64 {
	if len(data) < period {
		return make([]float64, len(data))
	}

	ema := make([]float64, len(data))

	// Initialize with SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += data[i]
	}
	ema[period-1] = sum / float64(period)

	// Calculate EMA
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(data); i++ {
		ema[i] = (data[i]-ema[i-1])*multiplier + ema[i-1]
	}

	// Fill initial values
	for i := 0; i < period-1; i++ {
		ema[i] = ema[period-1]
	}

	return ema
}

// calculateT3Server calculates Tillson T3 Moving Average (identical to frontend calculateT3)
func calculateT3Server(data []float64, period int) []float64 {
	b := 0.7
	c1 := -b * b * b
	c2 := 3*b*b + 3*b*b*b
	c3 := -6*b*b - 3*b - 3*b*b*b
	c4 := 1 + 3*b + b*b*b + 3*b*b

	e1 := calculateEMAServer(data, period)
	e2 := calculateEMAServer(e1, period)
	e3 := calculateEMAServer(e2, period)
	e4 := calculateEMAServer(e3, period)
	e5 := calculateEMAServer(e4, period)
	e6 := calculateEMAServer(e5, period)

	result := make([]float64, len(data))
	for i := range data {
		result[i] = c1*e6[i] + c2*e5[i] + c3*e4[i] + c4*e3[i]
	}
	return result
}

// calculateRMAServer calculates Wilder's smoothing (RMA) for RSI
func calculateRMAServer(data []float64, period int) []float64 {
	if len(data) < period {
		return make([]float64, len(data))
	}

	rma := make([]float64, len(data))

	// Initialize with SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += data[i]
	}
	rma[period-1] = sum / float64(period)

	// Calculate RMA
	alpha := 1.0 / float64(period)
	for i := period; i < len(data); i++ {
		rma[i] = alpha*data[i] + (1-alpha)*rma[i-1]
	}

	return rma
}

// calculateRSIServer calculates RSI using RMA
func calculateRSIServer(data []float64, period int) []float64 {
	if len(data) < period+1 {
		result := make([]float64, len(data))
		for i := range result {
			result[i] = 50
		}
		return result
	}

	gains := make([]float64, len(data))
	losses := make([]float64, len(data))

	for i := 1; i < len(data); i++ {
		change := data[i] - data[i-1]
		if change > 0 {
			gains[i] = change
		} else {
			losses[i] = math.Abs(change)
		}
	}

	avgGain := calculateRMAServer(gains[1:], period)
	avgLoss := calculateRMAServer(losses[1:], period)

	result := make([]float64, len(data))
	for i := range result {
		result[i] = 50
	}

	for i := period; i < len(data); i++ {
		ag := avgGain[i-1]
		al := avgLoss[i-1]
		if al == 0 {
			if ag == 0 {
				result[i] = 50
			} else {
				result[i] = 100
			}
		} else {
			rs := ag / al
			result[i] = 100 - 100/(1+rs)
		}
	}

	return result
}

// calculateSMAServer calculates Simple Moving Average
func calculateSMAServer(data []float64, period int) []float64 {
	if len(data) < period {
		return make([]float64, len(data))
	}

	sma := make([]float64, len(data))
	sum := 0.0

	for i := 0; i < period; i++ {
		sum += data[i]
	}
	sma[period-1] = sum / float64(period)

	for i := period; i < len(data); i++ {
		sum = sum - data[i-period] + data[i]
		sma[i] = sum / float64(period)
	}

	return sma
}

// calculateBXtrenderServer calculates BXtrender indicators and generates signals
func calculateBXtrenderServer(ohlcv []OHLCV, isAggressive bool, config BXtrenderConfig, nextBarOpen float64, nextBarTime int64) BXtrenderResult {
	shortL1 := config.ShortL1
	shortL2 := config.ShortL2
	shortL3 := config.ShortL3
	longL1 := config.LongL1
	longL2 := config.LongL2

	if shortL1 == 0 {
		shortL1 = 5
	}
	if shortL2 == 0 {
		shortL2 = 20
	}
	if shortL3 == 0 {
		shortL3 = 15
	}
	if longL1 == 0 {
		longL1 = 20
	}
	if longL2 == 0 {
		longL2 = 15
	}

	minLen := shortL2
	if longL1 > minLen {
		minLen = longL1
	}
	minLen += shortL3 + 10

	if len(ohlcv) < minLen {
		return BXtrenderResult{Signal: "NO_DATA", Bars: 0}
	}

	// Extract close prices
	closes := make([]float64, len(ohlcv))
	for i, bar := range ohlcv {
		closes[i] = bar.Close
	}

	// Calculate EMAs
	ema1 := calculateEMAServer(closes, shortL1)
	ema2 := calculateEMAServer(closes, shortL2)
	emaLong := calculateEMAServer(closes, longL1)

	// Calculate difference for short term
	diff := make([]float64, len(closes))
	for i := range diff {
		diff[i] = ema1[i] - ema2[i]
	}

	// Calculate RSI of difference (short term xtrender)
	shortXtrender := calculateRSIServer(diff, shortL3)
	for i := range shortXtrender {
		shortXtrender[i] -= 50
	}

	// Calculate RSI of long EMA (long term xtrender)
	longXtrender := calculateRSIServer(emaLong, longL2)
	for i := range longXtrender {
		longXtrender[i] -= 50
	}

	// Generate trades using color-based logic (matching frontend)
	trades := []ServerTrade{}
	inPosition := false
	var lastBuyPrice float64
	lastBuySignalIdx := -1
	lastSellSignalIdx := -1

	for i := 1; i < len(ohlcv); i++ {
		shortPrev := shortXtrender[i-1]
		shortCurr := shortXtrender[i]

		// Color definitions (matching frontend)
		isBullish := shortCurr > 0
		wasBullish := shortPrev > 0
		isLightRed := shortCurr < 0 && shortCurr > shortPrev  // negative but rising
		isDarkRed := shortCurr < 0 && shortCurr <= shortPrev   // negative and falling

		// Count consecutive light red bars
		consecutiveLightRed := 0
		if isLightRed {
			consecutiveLightRed = 1
			for j := i - 1; j >= 1; j-- {
				if shortXtrender[j] < 0 && shortXtrender[j] > shortXtrender[j-1] {
					consecutiveLightRed++
				} else {
					break
				}
			}
		}

		justTurnedGreen := isBullish && !wasBullish
		buySignal := false
		sellSignal := false

		if isAggressive {
			// Aggressive: 1st light-red bar OR red→green transition
			buySignal = !inPosition && ((isLightRed && consecutiveLightRed == 1) || justTurnedGreen)
		} else {
			// Defensive: red→green OR 4th consecutive light-red bar
			buySignal = !inPosition && (justTurnedGreen || (isLightRed && consecutiveLightRed == 4))
		}

		// SELL: First dark red bar (both modes)
		sellSignal = isDarkRed && inPosition

		// Execute at Open of NEXT bar (or nextBarOpen for last bar)
		if buySignal {
			var execPrice float64
			var execTime int64
			if i+1 < len(ohlcv) && ohlcv[i+1].Open > 0 {
				execPrice = ohlcv[i+1].Open
				execTime = ohlcv[i+1].Time
			} else if i == len(ohlcv)-1 && nextBarOpen > 0 {
				execPrice = nextBarOpen
				execTime = nextBarTime
			}
			if execPrice > 0 {
				trades = append(trades, ServerTrade{
					Type:  "BUY",
					Time:  execTime,
					Price: execPrice,
				})
				lastBuyPrice = execPrice
				inPosition = true
				lastBuySignalIdx = i
			}
		} else if sellSignal {
			var execPrice float64
			var execTime int64
			if i+1 < len(ohlcv) && ohlcv[i+1].Open > 0 {
				execPrice = ohlcv[i+1].Open
				execTime = ohlcv[i+1].Time
			} else if i == len(ohlcv)-1 && nextBarOpen > 0 {
				execPrice = nextBarOpen
				execTime = nextBarTime
			}
			if execPrice > 0 {
				returnPct := ((execPrice - lastBuyPrice) / lastBuyPrice) * 100
				trades = append(trades, ServerTrade{
					Type:      "SELL",
					Time:      execTime,
					Price:     execPrice,
					PrevPrice: lastBuyPrice,
					Return:    returnPct,
				})
				inPosition = false
				lastBuyPrice = 0
				lastSellSignalIdx = i
			}
		}
	}

	// Signal basiert auf dem SIGNAL-Bar-Index (nicht dem Trade-Ausfuehrungsbar)
	// BUY: Signal auf letztem Bar, HOLD: Position offen aber Signal aelter
	// SELL: Signal auf letztem Bar, WAIT: keine Position und Signal aelter
	signal := "WAIT"
	bars := 0
	lastIdx := len(ohlcv) - 1

	if inPosition {
		if lastBuySignalIdx == lastIdx {
			signal = "BUY"
		} else {
			signal = "HOLD"
		}
		if lastBuySignalIdx >= 0 {
			bars = lastIdx - lastBuySignalIdx
		}
	} else {
		if lastSellSignalIdx == lastIdx && len(trades) > 0 {
			signal = "SELL"
		} else {
			signal = "WAIT"
		}
		if lastSellSignalIdx >= 0 {
			bars = lastIdx - lastSellSignalIdx
		}
	}

	return BXtrenderResult{
		Short:  shortXtrender,
		Long:   longXtrender,
		Signal: signal,
		Bars:   bars,
		Trades: trades,
	}
}

// calculateBXtrenderQuantServer calculates BXtrender for Quant mode with trailing stop loss
func calculateBXtrenderQuantServer(ohlcv []OHLCV, config BXtrenderQuantConfig, nextBarOpen float64, nextBarTime int64) BXtrenderResult {
	shortL1 := config.ShortL1
	shortL2 := config.ShortL2
	shortL3 := config.ShortL3
	longL1 := config.LongL1
	longL2 := config.LongL2
	maLength := config.MaLength
	maFilterOn := config.MaFilterOn
	tslPercent := config.TslPercent

	if shortL1 == 0 {
		shortL1 = 5
	}
	if shortL2 == 0 {
		shortL2 = 20
	}
	if shortL3 == 0 {
		shortL3 = 15
	}
	if longL1 == 0 {
		longL1 = 20
	}
	if longL2 == 0 {
		longL2 = 15
	}
	if maLength == 0 {
		maLength = 200
	}
	if tslPercent == 0 {
		tslPercent = 20.0
	}

	minLen := shortL2
	if longL1 > minLen {
		minLen = longL1
	}
	if maLength > minLen {
		minLen = maLength
	}
	minLen += shortL3 + 10

	if len(ohlcv) < minLen {
		return BXtrenderResult{Signal: "NO_DATA", Bars: 0}
	}

	// Extract close prices
	closes := make([]float64, len(ohlcv))
	for i, bar := range ohlcv {
		closes[i] = bar.Close
	}

	// Calculate EMAs
	ema1 := calculateEMAServer(closes, shortL1)
	ema2 := calculateEMAServer(closes, shortL2)
	emaLong := calculateEMAServer(closes, longL1)

	// Calculate MA filter
	var maFilter []float64
	if config.MaType == "SMA" {
		maFilter = calculateSMAServer(closes, maLength)
	} else {
		maFilter = calculateEMAServer(closes, maLength)
	}

	// Calculate difference for short term
	diff := make([]float64, len(closes))
	for i := range diff {
		diff[i] = ema1[i] - ema2[i]
	}

	// Calculate RSI of difference (short term xtrender)
	shortXtrender := calculateRSIServer(diff, shortL3)
	for i := range shortXtrender {
		shortXtrender[i] -= 50
	}

	// Calculate RSI of long EMA (long term xtrender)
	longXtrender := calculateRSIServer(emaLong, longL2)
	for i := range longXtrender {
		longXtrender[i] -= 50
	}

	// Generate trades with trailing stop loss
	trades := []ServerTrade{}
	inPosition := false
	var lastBuyPrice, highestPrice float64
	lastBuySignalIdx := -1
	lastSellSignalIdx := -1

	for i := 1; i < len(ohlcv); i++ {
		shortPrev := shortXtrender[i-1]
		shortCurr := shortXtrender[i]
		longPrev := longXtrender[i-1]
		longCurr := longXtrender[i]
		price := ohlcv[i].Close

		// Update highest price if in position
		if inPosition && price > highestPrice {
			highestPrice = price
		}

		// Check trailing stop loss
		tslTriggered := false
		if inPosition && highestPrice > 0 {
			stopPrice := highestPrice * (1 - tslPercent/100)
			if price <= stopPrice {
				tslTriggered = true
			}
		}

		// MA filter condition
		maCondition := !maFilterOn || price > maFilter[i]

		// Both indicators alignment
		bothPositiveNow := shortCurr > 0 && longCurr > 0
		bothPositivePrev := shortPrev > 0 && longPrev > 0

		// Buy signal: both positive AND at least one was negative before AND MA filter
		// Also allow re-entry when both are still positive but we got stopped out by TSL
		buySignal := bothPositiveNow && (!bothPositivePrev || !inPosition) && maCondition

		// Sell signal: EITHER indicator negative OR TSL triggered
		sellSignal := (shortCurr < 0 || longCurr < 0) || tslTriggered

		if buySignal && !inPosition {
			var execPrice float64
			var execTime int64
			if i+1 < len(ohlcv) && ohlcv[i+1].Open > 0 {
				execPrice = ohlcv[i+1].Open
				execTime = ohlcv[i+1].Time
			} else if i == len(ohlcv)-1 && nextBarOpen > 0 {
				execPrice = nextBarOpen
				execTime = nextBarTime
			}
			if execPrice > 0 {
				trades = append(trades, ServerTrade{
					Type:  "BUY",
					Time:  execTime,
					Price: execPrice,
				})
				lastBuyPrice = execPrice
				highestPrice = execPrice
				inPosition = true
				lastBuySignalIdx = i
			}
		} else if sellSignal && inPosition {
			var execPrice float64
			var execTime int64
			if i+1 < len(ohlcv) && ohlcv[i+1].Open > 0 {
				execPrice = ohlcv[i+1].Open
				execTime = ohlcv[i+1].Time
			} else if i == len(ohlcv)-1 && nextBarOpen > 0 {
				execPrice = nextBarOpen
				execTime = nextBarTime
			}
			if execPrice > 0 {
				returnPct := ((execPrice - lastBuyPrice) / lastBuyPrice) * 100
				trades = append(trades, ServerTrade{
					Type:      "SELL",
					Time:      execTime,
					Price:     execPrice,
					PrevPrice: lastBuyPrice,
					Return:    returnPct,
				})
				inPosition = false
				lastBuyPrice = 0
				highestPrice = 0
				lastSellSignalIdx = i
			}
		}
	}

	// Signal basiert auf dem SIGNAL-Bar-Index (nicht dem Trade-Ausfuehrungsbar)
	signal := "WAIT"
	bars := 0
	lastIdx := len(ohlcv) - 1

	if inPosition {
		if lastBuySignalIdx == lastIdx {
			signal = "BUY"
		} else {
			signal = "HOLD"
		}
		if lastBuySignalIdx >= 0 {
			bars = lastIdx - lastBuySignalIdx
		}
	} else {
		if lastSellSignalIdx == lastIdx && len(trades) > 0 {
			signal = "SELL"
		} else {
			signal = "WAIT"
		}
		if lastSellSignalIdx >= 0 {
			bars = lastIdx - lastSellSignalIdx
		}
	}

	return BXtrenderResult{
		Short:  shortXtrender,
		Long:   longXtrender,
		Signal: signal,
		Bars:   bars,
		Trades: trades,
	}
}

// calculateMetricsServer calculates trading metrics from trades
func calculateMetricsServer(trades []ServerTrade) MetricsResult {
	wins := 0
	losses := 0
	totalReturn := 0.0
	totalWinReturn := 0.0
	totalLossReturn := 0.0

	for _, trade := range trades {
		if trade.Type == "SELL" {
			totalReturn += trade.Return
			if trade.Return > 0 {
				wins++
				totalWinReturn += trade.Return
			} else {
				losses++
				totalLossReturn += math.Abs(trade.Return)
			}
		}
	}

	totalTrades := wins + losses
	winRate := 0.0
	avgReturn := 0.0
	riskReward := 0.0

	if totalTrades > 0 {
		winRate = float64(wins) / float64(totalTrades) * 100
		avgReturn = totalReturn / float64(totalTrades)
	}

	if losses > 0 && wins > 0 {
		avgWin := totalWinReturn / float64(wins)
		avgLoss := totalLossReturn / float64(losses)
		if avgLoss > 0 {
			riskReward = avgWin / avgLoss
		}
	}

	return MetricsResult{
		WinRate:     winRate,
		RiskReward:  riskReward,
		TotalReturn: totalReturn,
		AvgReturn:   avgReturn,
		TotalTrades: totalTrades,
		Wins:        wins,
		Losses:      losses,
	}
}

// calcSignalSince computes signal_since from SignalBars and the OHLCV timestamps in trades
// calcSignalSinceFromRequest computes signal_since from HTTP request trade data
func calcSignalSinceFromRequest(trades []TradeData, signalBars int) string {
	if len(trades) > 0 {
		lastTrade := trades[len(trades)-1]
		ts := lastTrade.EntryDate
		if lastTrade.ExitDate != nil && *lastTrade.ExitDate > 0 {
			ts = *lastTrade.ExitDate
		}
		if ts > 0 {
			// Normalize to 1st of the month (monthly candle timestamps can be mid-month)
			t := time.Unix(ts, 0).UTC()
			normalized := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
			return normalized.Format("2006-01-02")
		}
	}
	if signalBars > 0 {
		return time.Now().AddDate(0, -signalBars, 0).Format("2006-01-02")
	}
	return time.Now().Format("2006-01-02")
}

func calcSignalSince(result BXtrenderResult) string {
	// Use the last trade's timestamp if possible for accuracy
	if len(result.Trades) > 0 {
		lastTrade := result.Trades[len(result.Trades)-1]
		if lastTrade.Time > 0 {
			// Normalize to 1st of the month (monthly candle timestamps can be mid-month)
			t := time.Unix(lastTrade.Time, 0).UTC()
			normalized := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
			return normalized.Format("2006-01-02")
		}
	}
	// Fallback: approximate from bars
	if result.Bars > 0 {
		return time.Now().AddDate(0, -result.Bars, 0).Format("2006-01-02")
	}
	return time.Now().Format("2006-01-02")
}

// updateSignalHistory updates prev_signal fields when the signal changes
func updateSignalHistory(oldSignal, oldSignalSince, newSignal, newSignalSince string) (signalSince, prevSignal, prevSignalSince string) {
	if oldSignal != "" && oldSignal != newSignal {
		// Signal changed → old becomes prev
		return newSignalSince, oldSignal, oldSignalSince
	}
	// Signal unchanged → keep existing since/prev
	if oldSignalSince != "" {
		return oldSignalSince, oldSignal, ""
	}
	return newSignalSince, "", ""
}

// savePerformanceServer saves performance data for defensive or aggressive mode
// convertServerTradesToTradeData converts ServerTrade pairs (BUY/SELL) to TradeData format
// so that getPerformanceHistory can read them correctly
func convertServerTradesToTradeData(serverTrades []ServerTrade, currentPrice float64) []TradeData {
	var result []TradeData
	for i := 0; i < len(serverTrades); i++ {
		t := serverTrades[i]
		if t.Type != "BUY" {
			continue
		}
		td := TradeData{
			EntryDate:  t.Time,
			EntryPrice: t.Price,
		}
		// Check if next trade is a matching SELL
		if i+1 < len(serverTrades) && serverTrades[i+1].Type == "SELL" {
			sell := serverTrades[i+1]
			td.ExitDate = &sell.Time
			td.ExitPrice = &sell.Price
			td.ReturnPct = sell.Return
			td.IsOpen = false
			i++ // skip the SELL
		} else {
			// Open position
			td.IsOpen = true
			cp := currentPrice
			td.CurrentPrice = &cp
			if t.Price > 0 {
				td.ReturnPct = ((currentPrice - t.Price) / t.Price) * 100
			}
		}
		result = append(result, td)
	}
	return result
}

func savePerformanceServer(symbol, name string, metrics MetricsResult, result BXtrenderResult, currentPrice float64, marketCap int64, isAggressive bool) {
	if result.Signal == "NO_DATA" {
		return // Don't overwrite existing data with no-data result
	}
	tradeData := convertServerTradesToTradeData(result.Trades, currentPrice)
	tradesJSON, _ := json.Marshal(tradeData)

	newSignalSince := calcSignalSince(result)

	if isAggressive {
		var existing AggressiveStockPerformance
		if err := db.Where("symbol = ?", symbol).First(&existing).Error; err != nil {
			existing = AggressiveStockPerformance{
				Symbol:       symbol,
				Name:         name,
				WinRate:      metrics.WinRate,
				RiskReward:   metrics.RiskReward,
				TotalReturn:  metrics.TotalReturn,
				AvgReturn:    metrics.AvgReturn,
				TotalTrades:  metrics.TotalTrades,
				Wins:         metrics.Wins,
				Losses:       metrics.Losses,
				Signal:       result.Signal,
				SignalBars:   result.Bars,
				SignalSince:  newSignalSince,
				TradesJSON:   string(tradesJSON),
				CurrentPrice: currentPrice,
				MarketCap:    marketCap,
				UpdatedAt:    time.Now(),
				CreatedAt:    time.Now(),
			}
			db.Create(&existing)
		} else {
			ss, ps, pss := updateSignalHistory(existing.Signal, existing.SignalSince, result.Signal, newSignalSince)
			existing.Name = name
			existing.WinRate = metrics.WinRate
			existing.RiskReward = metrics.RiskReward
			existing.TotalReturn = metrics.TotalReturn
			existing.AvgReturn = metrics.AvgReturn
			existing.TotalTrades = metrics.TotalTrades
			existing.Wins = metrics.Wins
			existing.Losses = metrics.Losses
			existing.Signal = result.Signal
			existing.SignalBars = result.Bars
			existing.SignalSince = ss
			if ps != "" {
				existing.PrevSignal = ps
				existing.PrevSignalSince = pss
			}
			existing.TradesJSON = string(tradesJSON)
			existing.CurrentPrice = currentPrice
			if marketCap > 0 {
				existing.MarketCap = marketCap
			}
			existing.UpdatedAt = time.Now()
			db.Save(&existing)
		}
	} else {
		var existing StockPerformance
		if err := db.Where("symbol = ?", symbol).First(&existing).Error; err != nil {
			existing = StockPerformance{
				Symbol:       symbol,
				Name:         name,
				WinRate:      metrics.WinRate,
				RiskReward:   metrics.RiskReward,
				TotalReturn:  metrics.TotalReturn,
				AvgReturn:    metrics.AvgReturn,
				TotalTrades:  metrics.TotalTrades,
				Wins:         metrics.Wins,
				Losses:       metrics.Losses,
				Signal:       result.Signal,
				SignalBars:   result.Bars,
				SignalSince:  newSignalSince,
				TradesJSON:   string(tradesJSON),
				CurrentPrice: currentPrice,
				MarketCap:    marketCap,
				UpdatedAt:    time.Now(),
				CreatedAt:    time.Now(),
			}
			db.Create(&existing)
		} else {
			ss, ps, pss := updateSignalHistory(existing.Signal, existing.SignalSince, result.Signal, newSignalSince)
			existing.Name = name
			existing.WinRate = metrics.WinRate
			existing.RiskReward = metrics.RiskReward
			existing.TotalReturn = metrics.TotalReturn
			existing.AvgReturn = metrics.AvgReturn
			existing.TotalTrades = metrics.TotalTrades
			existing.Wins = metrics.Wins
			existing.Losses = metrics.Losses
			existing.Signal = result.Signal
			existing.SignalBars = result.Bars
			existing.SignalSince = ss
			if ps != "" {
				existing.PrevSignal = ps
				existing.PrevSignalSince = pss
			}
			existing.TradesJSON = string(tradesJSON)
			existing.CurrentPrice = currentPrice
			if marketCap > 0 {
				existing.MarketCap = marketCap
			}
			existing.UpdatedAt = time.Now()
			db.Save(&existing)
		}
	}
}

// saveQuantPerformanceServer saves performance data for quant mode
func saveQuantPerformanceServer(symbol, name string, metrics MetricsResult, result BXtrenderResult, currentPrice float64, marketCap int64) {
	if result.Signal == "NO_DATA" {
		return
	}
	tradeData := convertServerTradesToTradeData(result.Trades, currentPrice)
	tradesJSON, _ := json.Marshal(tradeData)
	newSignalSince := calcSignalSince(result)

	var existing QuantStockPerformance
	if err := db.Where("symbol = ?", symbol).First(&existing).Error; err != nil {
		existing = QuantStockPerformance{
			Symbol:       symbol,
			Name:         name,
			WinRate:      metrics.WinRate,
			RiskReward:   metrics.RiskReward,
			TotalReturn:  metrics.TotalReturn,
			AvgReturn:    metrics.AvgReturn,
			TotalTrades:  metrics.TotalTrades,
			Wins:         metrics.Wins,
			Losses:       metrics.Losses,
			Signal:       result.Signal,
			SignalBars:   result.Bars,
			SignalSince:  newSignalSince,
			TradesJSON:   string(tradesJSON),
			CurrentPrice: currentPrice,
			MarketCap:    marketCap,
			UpdatedAt:    time.Now(),
			CreatedAt:    time.Now(),
		}
		db.Create(&existing)
	} else {
		ss, ps, pss := updateSignalHistory(existing.Signal, existing.SignalSince, result.Signal, newSignalSince)
		existing.Name = name
		existing.WinRate = metrics.WinRate
		existing.RiskReward = metrics.RiskReward
		existing.TotalReturn = metrics.TotalReturn
		existing.AvgReturn = metrics.AvgReturn
		existing.TotalTrades = metrics.TotalTrades
		existing.Wins = metrics.Wins
		existing.Losses = metrics.Losses
		existing.Signal = result.Signal
		existing.SignalBars = result.Bars
		existing.SignalSince = ss
		if ps != "" {
			existing.PrevSignal = ps
			existing.PrevSignalSince = pss
		}
		existing.TradesJSON = string(tradesJSON)
		existing.CurrentPrice = currentPrice
		if marketCap > 0 {
			existing.MarketCap = marketCap
		}
		existing.UpdatedAt = time.Now()
		db.Save(&existing)
	}
}

// ========================================
// Ditz Bot Functions
// ========================================

func ensureDitzUser() {
	// Create Ditz user if not exists (for portfolio comparison visibility)
	var user User
	result := db.Where("id = ?", DITZ_USER_ID).First(&user)
	if result.Error != nil {
		hashedPassword, _ := hashPassword("ditz-system-user-no-login")
		botUser := User{
			ID:       DITZ_USER_ID,
			Email:    "ditz@system.local",
			Username: "Ditz",
			Password: hashedPassword,
			IsAdmin:  false,
		}
		db.Create(&botUser)
	}
}

func ensureTraderUser() {
	// Create Trader user if not exists (for portfolio comparison visibility)
	var user User
	result := db.Where("id = ?", TRADER_USER_ID).First(&user)
	if result.Error != nil {
		hashedPassword, _ := hashPassword("trader-system-user-no-login")
		botUser := User{
			ID:       TRADER_USER_ID,
			Email:    "trader@system.local",
			Username: "Trader",
			Password: hashedPassword,
			IsAdmin:  false,
		}
		db.Create(&botUser)
	}
}

// Ditz mode performance handlers
func saveDitzStockPerformance(c *gin.Context) {
	var req struct {
		Symbol       string      `json:"symbol" binding:"required"`
		Name         string      `json:"name"`
		WinRate      float64     `json:"win_rate"`
		RiskReward   float64     `json:"risk_reward"`
		TotalReturn  float64     `json:"total_return"`
		AvgReturn    float64     `json:"avg_return"`
		TotalTrades  int         `json:"total_trades"`
		Wins         int         `json:"wins"`
		Losses       int         `json:"losses"`
		Signal       string      `json:"signal"`
		SignalBars   int         `json:"signal_bars"`
		Trades       []TradeData `json:"trades"`
		CurrentPrice float64     `json:"current_price"`
		MarketCap    int64       `json:"market_cap"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	symbol := strings.ToUpper(req.Symbol)
	tradesJSON, _ := json.Marshal(req.Trades)

	newSignalSince := calcSignalSinceFromRequest(req.Trades, req.SignalBars)

	var existing DitzStockPerformance
	result := db.Where("symbol = ?", symbol).First(&existing)

	if result.Error == nil {
		ss, ps, pss := updateSignalHistory(existing.Signal, existing.SignalSince, req.Signal, newSignalSince)
		existing.Name = req.Name
		existing.WinRate = req.WinRate
		existing.RiskReward = req.RiskReward
		existing.TotalReturn = req.TotalReturn
		existing.AvgReturn = req.AvgReturn
		existing.TotalTrades = req.TotalTrades
		existing.Wins = req.Wins
		existing.Losses = req.Losses
		existing.Signal = req.Signal
		existing.SignalBars = req.SignalBars
		existing.SignalSince = ss
		if ps != "" {
			existing.PrevSignal = ps
			existing.PrevSignalSince = pss
		}
		existing.TradesJSON = string(tradesJSON)
		existing.CurrentPrice = req.CurrentPrice
		if req.MarketCap > 0 {
			existing.MarketCap = req.MarketCap
		}
		existing.UpdatedAt = time.Now()
		db.Save(&existing)
		c.JSON(http.StatusOK, existing)
	} else {
		perf := DitzStockPerformance{
			Symbol:       symbol,
			Name:         req.Name,
			WinRate:      req.WinRate,
			RiskReward:   req.RiskReward,
			TotalReturn:  req.TotalReturn,
			AvgReturn:    req.AvgReturn,
			TotalTrades:  req.TotalTrades,
			Wins:         req.Wins,
			Losses:       req.Losses,
			Signal:       req.Signal,
			SignalBars:   req.SignalBars,
			SignalSince:  newSignalSince,
			TradesJSON:   string(tradesJSON),
			CurrentPrice: req.CurrentPrice,
			MarketCap:    req.MarketCap,
		}
		db.Create(&perf)
		c.JSON(http.StatusCreated, perf)
	}
}

func getDitzTrackedStocks(c *gin.Context) {
	var performances []DitzStockPerformance
	db.Order("updated_at desc").Find(&performances)

	type PerformanceWithTrades struct {
		DitzStockPerformance
		Trades []TradeData `json:"trades"`
	}

	result := make([]PerformanceWithTrades, len(performances))
	for i, p := range performances {
		result[i].DitzStockPerformance = p
		if p.TradesJSON != "" {
			json.Unmarshal([]byte(p.TradesJSON), &result[i].Trades)
		}
	}

	c.JSON(http.StatusOK, result)
}

func getDitzStockPerformance(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))

	var perf DitzStockPerformance
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

// Ditz config handlers
func getBXtrenderDitzConfigPublic(c *gin.Context) {
	var config BXtrenderDitzConfig
	result := db.First(&config)

	if result.Error != nil {
		// Return default config
		config = BXtrenderDitzConfig{
			ShortL1:    5,
			ShortL2:    20,
			ShortL3:    15,
			LongL1:     20,
			LongL2:     15,
			MaFilterOn: true,
			MaLength:   200,
			MaType:     "EMA",
			TslPercent: 20.0,
		}
	}

	c.JSON(http.StatusOK, config)
}

func getBXtrenderDitzConfig(c *gin.Context) {
	var config BXtrenderDitzConfig
	result := db.First(&config)

	if result.Error != nil {
		// Return default config
		config = BXtrenderDitzConfig{
			ShortL1:    5,
			ShortL2:    20,
			ShortL3:    15,
			LongL1:     20,
			LongL2:     15,
			MaFilterOn: true,
			MaLength:   200,
			MaType:     "EMA",
			TslPercent: 20.0,
		}
	}

	c.JSON(http.StatusOK, config)
}

func updateBXtrenderDitzConfig(c *gin.Context) {
	var req struct {
		ShortL1    int     `json:"short_l1"`
		ShortL2    int     `json:"short_l2"`
		ShortL3    int     `json:"short_l3"`
		LongL1     int     `json:"long_l1"`
		LongL2     int     `json:"long_l2"`
		MaFilterOn bool    `json:"ma_filter_on"`
		MaLength   int     `json:"ma_length"`
		MaType     string  `json:"ma_type"`
		TslPercent float64 `json:"tsl_percent"`
		TslEnabled *bool   `json:"tsl_enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var config BXtrenderDitzConfig
	result := db.First(&config)

	if result.Error != nil {
		tslEnabled := true
		if req.TslEnabled != nil {
			tslEnabled = *req.TslEnabled
		}
		config = BXtrenderDitzConfig{
			ShortL1:    req.ShortL1,
			ShortL2:    req.ShortL2,
			ShortL3:    req.ShortL3,
			LongL1:     req.LongL1,
			LongL2:     req.LongL2,
			MaFilterOn: req.MaFilterOn,
			MaLength:   req.MaLength,
			MaType:     req.MaType,
			TslPercent: req.TslPercent,
			TslEnabled: tslEnabled,
			UpdatedAt:  time.Now(),
		}
		db.Create(&config)
	} else {
		config.ShortL1 = req.ShortL1
		config.ShortL2 = req.ShortL2
		config.ShortL3 = req.ShortL3
		config.LongL1 = req.LongL1
		config.LongL2 = req.LongL2
		config.MaFilterOn = req.MaFilterOn
		config.MaLength = req.MaLength
		config.MaType = req.MaType
		config.TslPercent = req.TslPercent
		if req.TslEnabled != nil {
			config.TslEnabled = *req.TslEnabled
		}
		config.UpdatedAt = time.Now()
		db.Save(&config)
	}

	c.JSON(http.StatusOK, config)
}

// runDitzUpdateInternal performs the Ditz bot update without HTTP context
func runDitzUpdateInternal(triggeredBy string) {
	checkDitzStopLoss()

	now := time.Now()
	sessionID := uuid.New().String()

	var logs []map[string]interface{}
	addLog := func(level, msg string) {
		logs = append(logs, map[string]interface{}{"level": level, "message": msg, "time": time.Now().Format("15:04:05")})
		db.Create(&BotLog{Bot: "ditz", Level: level, Message: msg, SessionID: sessionID, CreatedAt: time.Now()})
	}

	addLog("INFO", fmt.Sprintf("Ditz Update gestartet um %s (von: %s)", now.Format("15:04:05"), triggeredBy))

	var ditzBotConfig BXtrenderDitzConfig
	db.First(&ditzBotConfig)

	var perfData []DitzStockPerformance
	if err := db.Find(&perfData).Error; err != nil {
		addLog("ERROR", fmt.Sprintf("Fehler beim Laden der Performance Daten: %v", err))
		return
	}

	addLog("INFO", fmt.Sprintf("%d Aktien geladen", len(perfData)))

	// Phase 1: Validate existing positions and trades against current BXTrender data
	var existingPositions []DitzPosition
	db.Where("is_live = ? AND is_closed = ?", false, false).Find(&existingPositions)

	for _, pos := range existingPositions {
		// Find matching performance data
		var stockPerf *DitzStockPerformance
		for i := range perfData {
			if perfData[i].Symbol == pos.Symbol {
				stockPerf = &perfData[i]
				break
			}
		}

		if stockPerf == nil {
			addLog("WARN", fmt.Sprintf("%s: Position vorhanden aber keine Performance-Daten - überspringe Validierung", pos.Symbol))
			continue
		}

		// Parse TradesJSON to find the matching open BUY trade
		if stockPerf.TradesJSON == "" {
			continue
		}
		var serverTrades []ServerTrade
		if err := json.Unmarshal([]byte(stockPerf.TradesJSON), &serverTrades); err != nil {
			continue
		}

		// Find the last open BUY trade in TradesJSON (one without a following SELL)
		var lastBuyTrade *ServerTrade
		for i := len(serverTrades) - 1; i >= 0; i-- {
			if serverTrades[i].Type == "BUY" {
				lastBuyTrade = &serverTrades[i]
				break
			}
		}

		if stockPerf.Signal == "NO_DATA" {
			addLog("SKIP", fmt.Sprintf("%s: Nicht genug Daten für Berechnung - überspringe", pos.Symbol))
			continue
		}

		if stockPerf.Signal == "SELL" || stockPerf.Signal == "WAIT" {
			// BXTrender says no position should be open - but we have one
			addLog("KORREKTUR", fmt.Sprintf("%s: Signal ist jetzt %s, aber Position vorhanden - schließe Position", pos.Symbol, stockPerf.Signal))

			// Find the last SELL in TradesJSON for the correct close price/date
			sellPrice := stockPerf.CurrentPrice
			sellDate := now
			for i := len(serverTrades) - 1; i >= 0; i-- {
				if serverTrades[i].Type == "SELL" {
					sellPrice = serverTrades[i].Price
					sellDate = time.Unix(serverTrades[i].Time, 0)
					break
				}
			}

			sellTrade := DitzTrade{
				Symbol:     pos.Symbol,
				Name:       pos.Name,
				Action:     "SELL",
				Quantity:   pos.Quantity,
				Price:      sellPrice,
				SignalDate: sellDate,
				ExecutedAt: sellDate,
				IsPending:  false,
				IsLive:     pos.IsLive,
			}
			pnl := (sellPrice - pos.AvgPrice) * pos.Quantity
			pnlPct := ((sellPrice - pos.AvgPrice) / pos.AvgPrice) * 100
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct

			db.Create(&sellTrade)

			// Close position instead of deleting
			pos.IsClosed = true
			pos.SellPrice = sellPrice
			pos.SellDate = &sellDate
			pos.ProfitLoss = &pnl
			pos.ProfitLossPct = &pnlPct
			pos.UpdatedAt = time.Now()
			db.Save(&pos)
			db.Where("user_id = ? AND symbol = ?", DITZ_USER_ID, pos.Symbol).Delete(&PortfolioPosition{})

			addLog("KORREKTUR", fmt.Sprintf("%s: Position geschlossen @ $%.2f (P/L: %.2f%%)", pos.Symbol, sellPrice, pnlPct))
			continue
		}

		if lastBuyTrade != nil {
			// Validate price and date of existing position against TradesJSON
			expectedPrice := lastBuyTrade.Price
			expectedDate := time.Unix(lastBuyTrade.Time, 0)

			priceDiff := math.Abs(pos.AvgPrice-expectedPrice) / expectedPrice * 100
			dateDiff := pos.BuyDate.Sub(expectedDate).Hours()

			if priceDiff > 1.0 || math.Abs(dateDiff) > 48 {
				addLog("KORREKTUR", fmt.Sprintf("%s: Position korrigiert - Alt: $%.2f am %s, Neu: $%.2f am %s",
					pos.Symbol, pos.AvgPrice, pos.BuyDate.Format("02.01.2006"),
					expectedPrice, expectedDate.Format("02.01.2006")))

				// Update position
				investmentEUR := pos.InvestedEUR
				if investmentEUR == 0 {
					investmentEUR = 100.0
				}
				investmentUSD := convertToUSD(investmentEUR, "EUR")
				newQty := math.Round((investmentUSD/expectedPrice)*1000000) / 1000000

				db.Model(&pos).Updates(map[string]interface{}{
					"avg_price": expectedPrice,
					"buy_date":  expectedDate,
					"quantity":  newQty,
				})

				// Update matching BUY trade
				var buyTrade DitzTrade
				if err := db.Where("symbol = ? AND action = ? AND is_live = ?", pos.Symbol, "BUY", false).
					Order("created_at desc").First(&buyTrade).Error; err == nil {
					db.Model(&buyTrade).Updates(map[string]interface{}{
						"price":       expectedPrice,
						"signal_date": expectedDate,
						"executed_at": expectedDate,
						"quantity":    newQty,
					})
				}

				// Update portfolio position
				db.Model(&PortfolioPosition{}).
					Where("user_id = ? AND symbol = ?", DITZ_USER_ID, pos.Symbol).
					Updates(map[string]interface{}{
						"avg_price":     expectedPrice,
						"purchase_date": expectedDate,
						"quantity":      newQty,
					})
			}
		}
	}

	// Phase 2: Process new signals (BUY/SELL)
	for _, stock := range perfData {
		if !isStockAllowedForBot("ditz", stock.Symbol) {
			continue
		}
		if stock.Signal == "BUY" {
			// Check if we already have an open position
			var existingPos DitzPosition
			if err := db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error; err == nil {
				addLog("SKIP", fmt.Sprintf("%s: Position bereits vorhanden", stock.Symbol))
				continue
			}

			// Check if there's a soft-deleted BUY (admin struck it out) - don't recreate
			var deletedBuy DitzTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ?", stock.Symbol, "BUY", true).Order("executed_at desc").First(&deletedBuy).Error; err == nil {
				var sellAfterDeleted DitzTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, deletedBuy.ExecutedAt).First(&sellAfterDeleted).Error; err != nil {
					addLog("SKIP", fmt.Sprintf("%s: Soft-deleted BUY vorhanden - überspringe", stock.Symbol))
					continue
				}
			}

			// Check if there's a recent BUY without a SELL
			var existingBuy DitzTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND is_filter_blocked = ?", stock.Symbol, "BUY", false, false).Order("executed_at desc").First(&existingBuy).Error; err == nil {
				var sellAfter DitzTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, existingBuy.ExecutedAt).First(&sellAfter).Error; err != nil {
					addLog("SKIP", fmt.Sprintf("%s: Bereits gekauft am %s", stock.Symbol, existingBuy.ExecutedAt.Format("02.01.2006")))
					continue
				}
			}

			// Check if there's already a filter-blocked BUY (don't create duplicates)
			var blockedBuy DitzTrade
			if err := db.Where("symbol = ? AND action = ? AND is_filter_blocked = ? AND is_deleted = ?", stock.Symbol, "BUY", true, false).Order("executed_at desc").First(&blockedBuy).Error; err == nil {
				var sellAfterBlocked DitzTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, blockedBuy.ExecutedAt).First(&sellAfterBlocked).Error; err != nil {
					continue
				}
			}

			// Extract signal date and price from TradesJSON (last BUY trade)
			signalPrice := stock.CurrentPrice
			signalDate := now
			if stock.TradesJSON != "" {
				var serverTrades []ServerTrade
				if err := json.Unmarshal([]byte(stock.TradesJSON), &serverTrades); err == nil {
					for i := len(serverTrades) - 1; i >= 0; i-- {
						if serverTrades[i].Type == "BUY" {
							signalPrice = serverTrades[i].Price
							signalDate = time.Unix(serverTrades[i].Time, 0)
							addLog("DEBUG", fmt.Sprintf("%s: Signal-Datum aus TradesJSON: %s, Preis: $%.2f",
								stock.Symbol, signalDate.Format("02.01.2006"), signalPrice))
							break
						}
					}
				}
			}

			// Calculate quantity based on 100 EUR investment
			investmentEUR := 100.0
			investmentUSD := convertToUSD(investmentEUR, "EUR")
			qty := math.Round((investmentUSD/signalPrice)*1000000) / 1000000
			if qty <= 0 {
				addLog("SKIP", fmt.Sprintf("%s: Ungültige Menge berechnet", stock.Symbol))
				continue
			}

			// Check bot filter config
			filterBlocked, filterReason := checkBotFilterConfig("ditz", stock.WinRate, stock.RiskReward, stock.AvgReturn, stock.MarketCap)
			if filterBlocked {
				blockedTrade := DitzTrade{
					Symbol:            stock.Symbol,
					Name:              stock.Name,
					Action:            "BUY",
					Quantity:          qty,
					Price:             signalPrice,
					SignalDate:        signalDate,
					ExecutedAt:        signalDate,
					IsPending:         false,
					IsLive:            false,
					IsFilterBlocked:   true,
					FilterBlockReason: filterReason,
				}
				db.Create(&blockedTrade)
				addLog("FILTER", fmt.Sprintf("%s: BUY blockiert durch Filter (%s)", stock.Symbol, filterReason))
				continue
			}

			buyTrade := DitzTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      signalPrice,
				SignalDate: signalDate,
				ExecutedAt: signalDate,
				IsPending:  false,
				IsLive:     false,
			}
			db.Create(&buyTrade)

			newPos := DitzPosition{
				Symbol:        stock.Symbol,
				Name:          stock.Name,
				Quantity:      qty,
				AvgPrice:      signalPrice,
				InvestedEUR:   investmentEUR,
				BuyDate:       signalDate,
				IsPending:     false,
				IsLive:        false,
				HighestPrice:  signalPrice,
				StopLossPrice: signalPrice * (1 - ditzBotConfig.TslPercent/100),
				StopLossType:  "trailing",
			}
			db.Create(&newPos)

			portfolioPos := PortfolioPosition{
				UserID:       DITZ_USER_ID,
				Symbol:       stock.Symbol,
				Name:         stock.Name,
				PurchaseDate: &signalDate,
				AvgPrice:     signalPrice,
				Currency:     "USD",
				Quantity:     &qty,
			}
			db.Create(&portfolioPos)

			addLog("ACTION", fmt.Sprintf("BUY ausgeführt: %s %.6f @ $%.2f (Signal: %s)", stock.Symbol, qty, signalPrice, signalDate.Format("02.01.2006")))

		} else if stock.Signal == "SELL" {
			// Check if there's a soft-deleted SELL (admin struck it out) - don't recreate
			var deletedSell DitzTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ?", stock.Symbol, "SELL", true).Order("executed_at desc").First(&deletedSell).Error; err == nil {
				addLog("SKIP", fmt.Sprintf("%s: Soft-deleted SELL vorhanden - überspringe", stock.Symbol))
				continue
			}

			var existingPos DitzPosition
			if err := db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error; err != nil {
				addLog("SKIP", fmt.Sprintf("%s: SELL Signal aber keine offene Position", stock.Symbol))
				continue
			}

			sellPrice := stock.CurrentPrice
			sellDate := now
			if stock.TradesJSON != "" {
				var serverTrades []ServerTrade
				if err := json.Unmarshal([]byte(stock.TradesJSON), &serverTrades); err == nil {
					for i := len(serverTrades) - 1; i >= 0; i-- {
						if serverTrades[i].Type == "SELL" {
							sellPrice = serverTrades[i].Price
							sellDate = time.Unix(serverTrades[i].Time, 0)
							break
						}
					}
				}
			}

			sellTrade := DitzTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "SELL",
				Quantity:   existingPos.Quantity,
				Price:      sellPrice,
				SignalDate: sellDate,
				ExecutedAt: sellDate,
				IsPending:  false,
				IsLive:     existingPos.IsLive,
			}

			pnl := (sellPrice - existingPos.AvgPrice) * existingPos.Quantity
			pnlPct := ((sellPrice - existingPos.AvgPrice) / existingPos.AvgPrice) * 100
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct

			db.Create(&sellTrade)

			// Close position instead of deleting
			existingPos.IsClosed = true
			existingPos.SellPrice = sellPrice
			existingPos.SellDate = &sellDate
			existingPos.ProfitLoss = &pnl
			existingPos.ProfitLossPct = &pnlPct
			existingPos.UpdatedAt = time.Now()
			db.Save(&existingPos)
			db.Where("user_id = ? AND symbol = ?", DITZ_USER_ID, stock.Symbol).Delete(&PortfolioPosition{})

			addLog("ACTION", fmt.Sprintf("SELL ausgeführt: %s @ $%.2f (Signal: %s, P/L: %.2f%%)", stock.Symbol, sellPrice, sellDate.Format("02.01.2006"), pnlPct))
		}
	}

	addLog("INFO", "Ditz Update abgeschlossen")

	lastRefresh := map[string]interface{}{
		"updated_at":   now,
		"triggered_by": triggeredBy,
		"logs":         logs,
	}
	lastRefreshJSON, _ := json.Marshal(lastRefresh)

	var setting SystemSetting
	if err := db.Where("key = ?", "last_ditz_refresh").First(&setting).Error; err != nil {
		setting = SystemSetting{
			Key:       "last_ditz_refresh",
			Value:     string(lastRefreshJSON),
			UpdatedAt: now,
		}
		db.Create(&setting)
	} else {
		setting.Value = string(lastRefreshJSON)
		setting.UpdatedAt = now
		db.Save(&setting)
	}
}

func ditzUpdate(c *gin.Context) {
	// Get username from session
	triggeredBy := "system"
	if userID, exists := c.Get("userID"); exists {
		var user User
		if err := db.First(&user, userID).Error; err == nil {
			triggeredBy = user.Username
		}
	}

	runDitzUpdateInternal(triggeredBy)

	// Read back the logs from the last refresh
	var setting SystemSetting
	if err := db.Where("key = ?", "last_ditz_refresh").First(&setting).Error; err == nil {
		var lastRefresh map[string]interface{}
		if err := json.Unmarshal([]byte(setting.Value), &lastRefresh); err == nil {
			c.JSON(http.StatusOK, gin.H{"message": "Ditz update completed", "logs": lastRefresh["logs"], "triggered_by": triggeredBy, "updated_at": lastRefresh["updated_at"]})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ditz update completed", "triggered_by": triggeredBy})
}

func getDitzPortfolio(c *gin.Context) {
	// Return all open positions (live + simulated) - frontend filters by is_live
	var positions []DitzPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Order("buy_date desc").Find(&positions)

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

	// Realisierte Gewinne aus geschlossenen Trades einrechnen
	var closedSellTrades []DitzTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ?", "SELL", false, false).Find(&closedSellTrades)

	realizedPL := 0.0
	totalClosedInvested := 0.0
	for _, trade := range closedSellTrades {
		if trade.ProfitLoss != nil {
			realizedPL += *trade.ProfitLoss
			totalClosedInvested += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}

	overallReturn := totalReturn + realizedPL
	overallInvested := totalInvested + totalClosedInvested
	overallReturnPct := 0.0
	if overallInvested > 0 {
		overallReturnPct = (overallReturn / overallInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"positions":          result,
		"total_value":        totalValue,
		"total_invested":     totalInvested,
		"total_return":       totalReturn,
		"total_return_pct":   overallReturnPct,
		"realized_pl":        realizedPL,
		"overall_return":     overallReturn,
		"overall_invested":   overallInvested,
	})
}

func getDitzActions(c *gin.Context) {
	// Return all trades (live + simulated) - frontend filters by is_live
	var trades []DitzTrade
	db.Where("is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false).Order("signal_date desc").Limit(50).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getDitzActionsAll(c *gin.Context) {
	// Admin view: return ALL trades (live + simulated)
	var trades []DitzTrade
	db.Where("is_pending = ?", false).Order("signal_date desc").Limit(100).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getDitzPerformance(c *gin.Context) {
	// Return all trades (live + simulated) - frontend filters by is_live
	var sellTrades []DitzTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false, false).Find(&sellTrades)

	var buyTrades []DitzTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "BUY", false, false, false).Find(&buyTrades)

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

	totalReturnPctClosed := 0.0
	for _, trade := range sellTrades {
		if trade.ProfitLossPct != nil {
			totalReturnPctClosed += *trade.ProfitLossPct
		}
	}
	avgReturnPerTrade := 0.0
	if len(sellTrades) > 0 {
		avgReturnPerTrade = totalReturnPctClosed / float64(len(sellTrades))
	}

	var positions []DitzPosition
	db.Where("is_pending = ? AND is_live = ? AND is_closed = ?", false, true, false).Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	unrealizedGain := 0.0
	investedInPositions := 0.0
	currentValue := 0.0
	liveCount := 0

	for _, pos := range positions {
		if pos.IsLive {
			liveCount++
		}
		investedInPositions += pos.AvgPrice * pos.Quantity
		quote := quotes[pos.Symbol]
		if quote.Price > 0 {
			currentValue += quote.Price * pos.Quantity
			unrealizedGain += (quote.Price - pos.AvgPrice) * pos.Quantity
		} else {
			currentValue += pos.AvgPrice * pos.Quantity
		}
	}

	unrealizedGainPct := 0.0
	if investedInPositions > 0 {
		unrealizedGainPct = (unrealizedGain / investedInPositions) * 100
	}

	totalGain := totalProfitLoss + unrealizedGain
	totalInvested := investedInPositions
	for _, trade := range sellTrades {
		if trade.ProfitLoss != nil {
			totalInvested += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}
	overallReturnPct := 0.0
	if totalInvested > 0 {
		overallReturnPct = (totalGain / totalInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_trades":         len(buyTrades) + len(sellTrades),
		"total_buys":           len(buyTrades),
		"completed_trades":     len(sellTrades),
		"open_positions":       len(positions),
		"live_positions":       liveCount,
		"wins":                 wins,
		"losses":               losses,
		"win_rate":             winRate,
		"realized_profit":      totalProfitLoss,
		"total_gain":           totalGain,
		"overall_return_pct":   overallReturnPct,
		"avg_return_per_trade": avgReturnPerTrade,
		"unrealized_gain":      unrealizedGain,
		"total_return_pct":     unrealizedGainPct,
		"invested_in_positions": investedInPositions,
		"current_value":        currentValue,
	})
}

func resetDitz(c *gin.Context) {
	db.Where("1 = 1").Delete(&DitzTrade{})
	db.Where("1 = 1").Delete(&DitzPosition{})
	db.Where("user_id = ?", DITZ_USER_ID).Delete(&PortfolioPosition{})
	db.Where("bot = ?", "ditz").Delete(&BotTodo{})
	db.Where("bot = ?", "ditz").Delete(&BotLog{})
	c.JSON(http.StatusOK, gin.H{"message": "Ditz reset complete"})
}

// getLastDitzRefresh returns the last ditz refresh info with logs
func getLastDitzRefresh(c *gin.Context) {
	var setting SystemSetting
	if err := db.Where("key = ?", "last_ditz_refresh").First(&setting).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"updated_at":   nil,
			"triggered_by": nil,
			"logs":         []interface{}{},
		})
		return
	}

	var lastRefresh map[string]interface{}
	if err := json.Unmarshal([]byte(setting.Value), &lastRefresh); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"updated_at":   setting.UpdatedAt,
			"triggered_by": "unknown",
			"logs":         []interface{}{},
		})
		return
	}

	c.JSON(http.StatusOK, lastRefresh)
}

// cleanupDitzPending deletes all pending trades and positions, and all todos
func cleanupDitzPending(c *gin.Context) {
	// Delete pending trades
	result1 := db.Where("is_pending = ?", true).Delete(&DitzTrade{})
	// Delete pending positions
	result2 := db.Where("is_pending = ?", true).Delete(&DitzPosition{})
	// Delete all todos for ditz bot
	result3 := db.Where("bot = ?", "ditz").Delete(&BotTodo{})

	c.JSON(http.StatusOK, gin.H{
		"message":             "Cleanup complete",
		"deleted_trades":      result1.RowsAffected,
		"deleted_positions":   result2.RowsAffected,
		"deleted_todos":       result3.RowsAffected,
	})
}

func ditzBackfill(c *gin.Context) {
	var req struct {
		UntilDate string `json:"until_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "until_date required"})
		return
	}

	fromDate, err := time.Parse("2006-01-02", req.UntilDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format (use YYYY-MM-DD)"})
		return
	}

	now := time.Now()
	sessionID := uuid.New().String()
	var logs []map[string]interface{}
	addLog := func(level, message string) {
		entry := map[string]interface{}{
			"level":   level,
			"message": message,
			"time":    time.Now().Format("15:04:05"),
		}
		logs = append(logs, entry)
		saveBotLog("ditz", level, message, sessionID)
	}

	addLog("INFO", fmt.Sprintf("Ditz Backfill gestartet ab %s bis heute", req.UntilDate))

	// Get all tracked stocks with their ditz performance data
	var trackedStocks []DitzStockPerformance
	db.Find(&trackedStocks)

	if len(trackedStocks) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message":        "No tracked stocks found",
			"trades_created": 0,
			"logs":           logs,
		})
		return
	}

	var tradesCreated int
	var positionsCreated int

	for _, stock := range trackedStocks {
		if stock.TradesJSON == "" {
			continue
		}

		// Check allowlist
		if !isStockAllowedForBot("ditz", stock.Symbol) {
			addLog("SKIP", fmt.Sprintf("%s: Nicht in Allowlist — übersprungen", stock.Symbol))
			continue
		}

		var historicalTrades []TradeData
		if err := json.Unmarshal([]byte(stock.TradesJSON), &historicalTrades); err != nil {
			addLog("ERROR", fmt.Sprintf("%s: Fehler beim Parsen der Trades: %v", stock.Symbol, err))
			continue
		}

		// Check if there's already an open position from BEFORE the backfill start date
		hasOpenPositionBefore := false
		for _, t := range historicalTrades {
			entryT := time.Unix(t.EntryDate, 0)
			if t.IsOpen && entryT.Before(fromDate) {
				hasOpenPositionBefore = true
				break
			}
		}
		if hasOpenPositionBefore {
			addLog("SKIP", fmt.Sprintf("%s: Offene Position vor Startdatum (HOLD) — übersprungen", stock.Symbol))
			continue
		}

		for _, trade := range historicalTrades {
			entryTime := time.Unix(trade.EntryDate, 0).UTC()
			entryTime = time.Date(entryTime.Year(), entryTime.Month(), 1, 0, 0, 0, 0, time.UTC)

			if entryTime.Year() < 2020 || entryTime.Year() > 2030 {
				continue
			}
			if entryTime.Before(fromDate) {
				continue
			}
			if entryTime.After(now) {
				continue
			}

			var existingBuy DitzTrade
			dateStart := entryTime.Truncate(24 * time.Hour)
			dateEnd := dateStart.Add(24 * time.Hour)
			alreadyExists := db.Where("symbol = ? AND action = ? AND signal_date >= ? AND signal_date < ?",
				stock.Symbol, "BUY", dateStart, dateEnd).First(&existingBuy).Error == nil
			if alreadyExists {
				continue
			}

			investmentEUR := 100.0
			investmentUSD := convertToUSD(investmentEUR, "EUR")
			qty := math.Round((investmentUSD/trade.EntryPrice)*1000000) / 1000000
			if qty <= 0 || trade.EntryPrice <= 0 {
				continue
			}

			buyTrade := DitzTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      trade.EntryPrice,
				SignalDate: entryTime,
				ExecutedAt: entryTime,
				IsPending:  false,
			}
			db.Create(&buyTrade)
			tradesCreated++
			addLog("ACTION", fmt.Sprintf("%s: BUY erstellt @ $%.2f am %s", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))

			if trade.ExitDate != nil && trade.ExitPrice != nil {
				exitTime := time.Unix(*trade.ExitDate, 0).UTC()
				exitTime = time.Date(exitTime.Year(), exitTime.Month(), 1, 0, 0, 0, 0, time.UTC)

				if !exitTime.After(now) {
					profitLoss := (*trade.ExitPrice - trade.EntryPrice) * qty
					profitLossPct := trade.ReturnPct

					sellTrade := DitzTrade{
						Symbol:        stock.Symbol,
						Name:          stock.Name,
						Action:        "SELL",
						Quantity:      qty,
						Price:         *trade.ExitPrice,
						SignalDate:    exitTime,
						ExecutedAt:    exitTime,
						IsPending:     false,
						ProfitLoss:    &profitLoss,
						ProfitLossPct: &profitLossPct,
					}
					db.Create(&sellTrade)
					tradesCreated++
					addLog("ACTION", fmt.Sprintf("%s: SELL erstellt @ $%.2f am %s (%.2f%%)", stock.Symbol, *trade.ExitPrice, exitTime.Format("2006-01-02"), profitLossPct))
				} else {
					var existingPos DitzPosition
					if db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error != nil {
						newPos := DitzPosition{
							Symbol:      stock.Symbol,
							Name:        stock.Name,
							Quantity:    qty,
							AvgPrice:    trade.EntryPrice,
							InvestedEUR: investmentEUR,
							BuyDate:     entryTime,
							IsPending:   false,
						}
						db.Create(&newPos)
						positionsCreated++

						portfolioPos := PortfolioPosition{
							UserID:       DITZ_USER_ID,
							Symbol:       stock.Symbol,
							Name:         stock.Name,
							PurchaseDate: &entryTime,
							AvgPrice:     trade.EntryPrice,
							Currency:     "USD",
							Quantity:     &qty,
						}
						db.Create(&portfolioPos)
						addLog("ACTION", fmt.Sprintf("%s: Position erstellt (offen)", stock.Symbol))
					}
				}
			} else if trade.IsOpen {
				var existingPos DitzPosition
				if db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error != nil {
					newPos := DitzPosition{
						Symbol:      stock.Symbol,
						Name:        stock.Name,
						Quantity:    qty,
						AvgPrice:    trade.EntryPrice,
						InvestedEUR: investmentEUR,
						BuyDate:     entryTime,
						IsPending:   false,
					}
					db.Create(&newPos)
					positionsCreated++

					portfolioPos := PortfolioPosition{
						UserID:       DITZ_USER_ID,
						Symbol:       stock.Symbol,
						Name:         stock.Name,
						PurchaseDate: &entryTime,
						AvgPrice:     trade.EntryPrice,
						Currency:     "USD",
						Quantity:     &qty,
					}
					db.Create(&portfolioPos)
					addLog("ACTION", fmt.Sprintf("%s: Position erstellt (offen)", stock.Symbol))
				}
			}
		}
	}

	addLog("INFO", fmt.Sprintf("Ditz Backfill abgeschlossen: %d Trades, %d Positionen erstellt", tradesCreated, positionsCreated))

	c.JSON(http.StatusOK, gin.H{
		"message":           "Backfill completed",
		"trades_created":    tradesCreated,
		"positions_created": positionsCreated,
		"until_date":        req.UntilDate,
		"logs":              logs,
	})
}

func getDitzCompletedTrades(c *gin.Context) {
	var trades []DitzTrade
	db.Where("action = ? AND profit_loss IS NOT NULL AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false).Order("executed_at desc").Find(&trades)

	type CompletedTrade struct {
		Symbol        string     `json:"symbol"`
		Name          string     `json:"name"`
		BuyPrice      float64    `json:"buy_price"`
		SellPrice     float64    `json:"sell_price"`
		BuyDate       time.Time  `json:"buy_date"`
		SellDate      time.Time  `json:"sell_date"`
		ProfitLoss    float64    `json:"profit_loss"`
		ProfitLossPct float64    `json:"profit_loss_pct"`
		IsLive        bool       `json:"is_live"`
	}

	var result []CompletedTrade
	for _, sell := range trades {
		var buy DitzTrade
		if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND is_filter_blocked = ? AND executed_at < ?", sell.Symbol, "BUY", false, false, sell.ExecutedAt).Order("executed_at desc").First(&buy).Error; err != nil {
			continue
		}

		ct := CompletedTrade{
			Symbol:    sell.Symbol,
			Name:      sell.Name,
			BuyPrice:  buy.Price,
			SellPrice: sell.Price,
			BuyDate:   buy.ExecutedAt,
			SellDate:  sell.ExecutedAt,
			IsLive:    sell.IsLive,
		}
		if sell.ProfitLoss != nil {
			ct.ProfitLoss = *sell.ProfitLoss
		}
		if sell.ProfitLossPct != nil {
			ct.ProfitLossPct = *sell.ProfitLossPct
		}
		result = append(result, ct)
	}

	c.JSON(http.StatusOK, result)
}

func updateDitzPosition(c *gin.Context) {
	id := c.Param("id")

	var position DitzPosition
	if err := db.First(&position, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found"})
		return
	}

	var req struct {
		IsLive          *bool    `json:"is_live"`
		AvgPrice        *float64 `json:"avg_price"`
		InvestedEUR     *float64 `json:"invested_eur"`
		StopLossPercent *float64 `json:"stop_loss_percent"`
		StopLossType    *string  `json:"stop_loss_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.IsLive != nil {
		position.IsLive = *req.IsLive
	}
	if req.AvgPrice != nil {
		position.AvgPrice = *req.AvgPrice
	}
	if req.InvestedEUR != nil {
		position.InvestedEUR = *req.InvestedEUR
	}
	if req.StopLossPercent != nil {
		if *req.StopLossPercent <= 0 {
			position.StopLossPercent = nil
		} else {
			position.StopLossPercent = req.StopLossPercent
		}
	}
	if req.StopLossType != nil {
		position.StopLossType = *req.StopLossType
	}

	db.Save(&position)

	var portfolioPos PortfolioPosition
	if err := db.Where("user_id = ? AND symbol = ?", DITZ_USER_ID, position.Symbol).First(&portfolioPos).Error; err == nil {
		portfolioPos.AvgPrice = position.AvgPrice
		db.Save(&portfolioPos)
	}

	c.JSON(http.StatusOK, position)
}

func updateDitzTrade(c *gin.Context) {
	id := c.Param("id")

	var trade DitzTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	var req struct {
		IsLive     *bool      `json:"is_live"`
		Price      *float64   `json:"price"`
		Quantity   *float64   `json:"quantity"`
		SignalDate *time.Time `json:"signal_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.IsLive != nil {
		trade.IsLive = *req.IsLive
	}
	if req.Price != nil {
		trade.Price = *req.Price
	}
	if req.Quantity != nil {
		trade.Quantity = *req.Quantity
	}
	if req.SignalDate != nil {
		trade.SignalDate = *req.SignalDate
		trade.ExecutedAt = *req.SignalDate
	}

	db.Save(&trade)

	// Sync changes to matching position and portfolio entry
	if trade.Action == "BUY" {
		var position DitzPosition
		if err := db.Where("symbol = ? AND is_closed = ?", trade.Symbol, false).First(&position).Error; err == nil {
			if req.IsLive != nil {
				position.IsLive = *req.IsLive
			}
			if req.Price != nil {
				position.AvgPrice = *req.Price
			}
			if req.Quantity != nil {
				position.Quantity = *req.Quantity
			}
			if req.SignalDate != nil {
				position.BuyDate = *req.SignalDate
			}
			db.Save(&position)

			// Also update portfolio position
			var portfolioPos PortfolioPosition
			if err := db.Where("user_id = ? AND symbol = ?", DITZ_USER_ID, trade.Symbol).First(&portfolioPos).Error; err == nil {
				if req.Price != nil {
					portfolioPos.AvgPrice = *req.Price
				}
				if req.Quantity != nil {
					portfolioPos.Quantity = req.Quantity
				}
				if req.SignalDate != nil {
					portfolioPos.PurchaseDate = req.SignalDate
				}
				db.Save(&portfolioPos)
			}
		}
	} else if trade.Action == "SELL" && req.Price != nil {
		var position DitzPosition
		if err := db.Where("symbol = ? AND is_closed = ? AND is_live = ?", trade.Symbol, true, trade.IsLive).Order("updated_at desc").First(&position).Error; err == nil {
			position.SellPrice = *req.Price
			pnl := (*req.Price - position.AvgPrice) * position.Quantity
			pnlPct := ((*req.Price - position.AvgPrice) / position.AvgPrice) * 100
			position.ProfitLoss = &pnl
			position.ProfitLossPct = &pnlPct
			db.Save(&position)
		}
	}

	c.JSON(http.StatusOK, trade)
}

func createManualDitzTrade(c *gin.Context) {
	var req struct {
		Symbol   string  `json:"symbol" binding:"required"`
		Name     string  `json:"name"`
		Action   string  `json:"action" binding:"required"` // BUY or SELL
		Price    float64 `json:"price" binding:"required"`
		Quantity float64 `json:"quantity"`
		Date     string  `json:"date"` // YYYY-MM-DD
		IsLive   bool    `json:"is_live"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Symbol, Action und Price sind Pflichtfelder"})
		return
	}

	if req.Action != "BUY" && req.Action != "SELL" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Action muss BUY oder SELL sein"})
		return
	}

	// Parse date or use now
	signalDate := time.Now()
	if req.Date != "" {
		parsed, err := time.Parse("2006-01-02", req.Date)
		if err == nil {
			signalDate = parsed
		}
	}

	// Default quantity: 100 EUR worth
	qty := req.Quantity
	if qty <= 0 {
		investmentEUR := 100.0
		investmentUSD := convertToUSD(investmentEUR, "EUR")
		qty = math.Round((investmentUSD/req.Price)*1000000) / 1000000
		if qty <= 0 {
			qty = 1
		}
	}

	// Resolve name if not provided
	name := req.Name
	if name == "" {
		name = req.Symbol
	}

	if req.Action == "BUY" {
		// Check for existing open position
		var existingPos DitzPosition
		if err := db.Where("symbol = ? AND is_closed = ?", req.Symbol, false).First(&existingPos).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Offene Position für %s existiert bereits", req.Symbol)})
			return
		}

		trade := DitzTrade{
			Symbol:     req.Symbol,
			Name:       name,
			Action:     "BUY",
			Quantity:   qty,
			Price:      req.Price,
			SignalDate: signalDate,
			ExecutedAt: signalDate,
			IsPending:  false,
			IsLive:     req.IsLive,
		}
		db.Create(&trade)

		investmentEUR := 100.0
		if req.Quantity > 0 {
			investmentEUR = req.Price * req.Quantity / convertToUSD(1.0, "EUR")
		}

		pos := DitzPosition{
			Symbol:      req.Symbol,
			Name:        name,
			Quantity:    qty,
			AvgPrice:    req.Price,
			InvestedEUR: investmentEUR,
			BuyDate:     signalDate,
			IsPending:   false,
			IsLive:      req.IsLive,
		}
		db.Create(&pos)

		portfolioPos := PortfolioPosition{
			UserID:       DITZ_USER_ID,
			Symbol:       req.Symbol,
			Name:         name,
			PurchaseDate: &signalDate,
			AvgPrice:     req.Price,
			Currency:     "USD",
			Quantity:     &qty,
		}
		db.Create(&portfolioPos)

		c.JSON(http.StatusOK, gin.H{"trade": trade, "position": pos})

	} else {
		// SELL - must have existing open position
		var existingPos DitzPosition
		if err := db.Where("symbol = ? AND is_closed = ?", req.Symbol, false).First(&existingPos).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Keine offene Position für %s vorhanden", req.Symbol)})
			return
		}

		sellQty := qty
		if req.Quantity <= 0 {
			sellQty = existingPos.Quantity
		}

		trade := DitzTrade{
			Symbol:     req.Symbol,
			Name:       name,
			Action:     "SELL",
			Quantity:   sellQty,
			Price:      req.Price,
			SignalDate: signalDate,
			ExecutedAt: signalDate,
			IsPending:  false,
			IsLive:     existingPos.IsLive,
		}

		pnl := (req.Price - existingPos.AvgPrice) * sellQty
		pnlPct := ((req.Price - existingPos.AvgPrice) / existingPos.AvgPrice) * 100
		trade.ProfitLoss = &pnl
		trade.ProfitLossPct = &pnlPct
		db.Create(&trade)

		// Close position instead of deleting
		existingPos.IsClosed = true
		existingPos.SellPrice = req.Price
		existingPos.SellDate = &signalDate
		existingPos.ProfitLoss = &pnl
		existingPos.ProfitLossPct = &pnlPct
		existingPos.UpdatedAt = time.Now()
		db.Save(&existingPos)
		db.Where("user_id = ? AND symbol = ?", DITZ_USER_ID, req.Symbol).Delete(&PortfolioPosition{})

		c.JSON(http.StatusOK, gin.H{"trade": trade})
	}
}

func deleteDitzTrade(c *gin.Context) {
	id := c.Param("id")

	var trade DitzTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	symbol := trade.Symbol
	wasDeleted := trade.IsDeleted

	// Toggle soft-delete
	trade.IsDeleted = !wasDeleted
	db.Save(&trade)

	if trade.Action == "BUY" {
		if !wasDeleted {
			// Soft-deleting a BUY → also soft-delete matching SELL, hard-delete position + portfolio
			var sellTrade DitzTrade
			if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "SELL", false).
				Order("signal_date desc").First(&sellTrade).Error; err == nil {
				sellTrade.IsDeleted = true
				db.Save(&sellTrade)
			}
			db.Where("symbol = ? AND is_live = ?", symbol, trade.IsLive).Delete(&DitzPosition{})
			db.Where("user_id = ? AND symbol = ?", DITZ_USER_ID, symbol).Delete(&PortfolioPosition{})
		} else {
			// Restoring a BUY → also restore matching SELL, recreate position
			var sellTrade DitzTrade
			hasSell := false
			if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "SELL", true).
				Order("signal_date desc").First(&sellTrade).Error; err == nil {
				sellTrade.IsDeleted = false
				db.Save(&sellTrade)
				hasSell = true
			}

			qty := trade.Quantity
			buyDate := trade.SignalDate
			newPos := DitzPosition{
				Symbol:   symbol,
				Name:     trade.Name,
				Quantity: qty,
				AvgPrice: trade.Price,
				IsLive:   trade.IsLive,
				BuyDate:  buyDate,
			}

			if hasSell {
				newPos.IsClosed = true
				newPos.SellPrice = sellTrade.Price
				sellDate := sellTrade.SignalDate
				newPos.SellDate = &sellDate
				newPos.ProfitLoss = sellTrade.ProfitLoss
				newPos.ProfitLossPct = sellTrade.ProfitLossPct
				db.Create(&newPos)
			} else {
				db.Create(&newPos)
				portfolioPos := PortfolioPosition{
					UserID:       DITZ_USER_ID,
					Symbol:       symbol,
					Name:         trade.Name,
					AvgPrice:     trade.Price,
					PurchaseDate: &buyDate,
					Quantity:     &qty,
				}
				db.Create(&portfolioPos)
			}
		}
	} else if trade.Action == "SELL" {
		// Check if the corresponding BUY is deleted
		var buyTrade DitzTrade
		buyDeleted := false
		if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "BUY", true).
			Order("signal_date desc").First(&buyTrade).Error; err == nil {
			buyDeleted = true
		}

		if buyDeleted {
			// BUY is deleted → just toggle SELL, no position changes
		} else if !wasDeleted {
			// Soft-deleting a SELL (BUY active) → reopen position
			var pos DitzPosition
			if err := db.Where("symbol = ? AND is_live = ?", symbol, trade.IsLive).Order("updated_at desc").First(&pos).Error; err == nil {
				pos.IsClosed = false
				pos.SellPrice = 0
				pos.SellDate = nil
				pos.ProfitLoss = nil
				pos.ProfitLossPct = nil
				pos.UpdatedAt = time.Now()
				db.Save(&pos)

				qty := pos.Quantity
				buyDate := pos.BuyDate
				portfolioPos := PortfolioPosition{
					UserID:       DITZ_USER_ID,
					Symbol:       pos.Symbol,
					Name:         pos.Name,
					AvgPrice:     pos.AvgPrice,
					PurchaseDate: &buyDate,
					Quantity:     &qty,
				}
				db.Create(&portfolioPos)
			}
		} else {
			// Restoring a SELL (BUY active) → re-close position
			var pos DitzPosition
			if err := db.Where("symbol = ? AND is_live = ? AND is_closed = ?", symbol, trade.IsLive, false).First(&pos).Error; err == nil {
				pos.IsClosed = true
				pos.SellPrice = trade.Price
				sellDate := trade.SignalDate
				pos.SellDate = &sellDate
				pos.ProfitLoss = trade.ProfitLoss
				pos.ProfitLossPct = trade.ProfitLossPct
				pos.UpdatedAt = time.Now()
				db.Save(&pos)
			}
			db.Where("user_id = ? AND symbol = ?", DITZ_USER_ID, symbol).Delete(&PortfolioPosition{})
		}
	}

	action := "deleted"
	if wasDeleted {
		action = "restored"
	}
	c.JSON(http.StatusOK, gin.H{"message": "Trade " + action, "trade": trade})
}

func toggleDitzTradeRead(c *gin.Context) {
	id := c.Param("id")
	var trade DitzTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}
	trade.IsRead = !trade.IsRead
	db.Save(&trade)
	c.JSON(http.StatusOK, gin.H{"trade": trade})
}

func markAllDitzTradesRead(c *gin.Context) {
	db.Model(&DitzTrade{}).Where("is_read = ? AND is_pending = ?", false, false).Update("is_read", true)
	c.JSON(http.StatusOK, gin.H{"message": "All trades marked as read"})
}

func markAllDitzTradesUnread(c *gin.Context) {
	db.Model(&DitzTrade{}).Where("is_read = ? AND is_pending = ?", true, false).Update("is_read", false)
	c.JSON(http.StatusOK, gin.H{"message": "All trades marked as unread"})
}

func getDitzUnreadCount(c *gin.Context) {
	var count int64
	db.Model(&DitzTrade{}).Where("is_read = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false, false).Count(&count)

	// Also get the unread trades for notification details
	var unreadTrades []DitzTrade
	db.Where("is_read = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false, false).Order("executed_at desc").Limit(10).Find(&unreadTrades)

	c.JSON(http.StatusOK, gin.H{"count": count, "trades": unreadTrades})
}

func getDitzPending(c *gin.Context) {
	var positions []DitzPosition
	db.Where("is_pending = ?", true).Find(&positions)

	type PendingPosition struct {
		DitzPosition
		CurrentPrice   float64 `json:"current_price"`
		TotalReturnPct float64 `json:"total_return_pct"`
	}

	symbols := make([]string, 0, len(positions))
	for _, p := range positions {
		symbols = append(symbols, p.Symbol)
	}
	quotes := fetchQuotes(symbols)

	var result []PendingPosition
	for _, pos := range positions {
		pp := PendingPosition{DitzPosition: pos}
		if quote, ok := quotes[pos.Symbol]; ok {
			pp.CurrentPrice = quote.Price
			if pos.AvgPrice > 0 {
				pp.TotalReturnPct = ((quote.Price - pos.AvgPrice) / pos.AvgPrice) * 100
			}
		}
		result = append(result, pp)
	}

	c.JSON(http.StatusOK, result)
}

func getDitzLogs(c *gin.Context) {
	var logs []BotLog
	db.Where("bot = ?", "ditz").Order("created_at desc").Limit(200).Find(&logs)
	c.JSON(http.StatusOK, logs)
}

func getDitzTodos(c *gin.Context) {
	var todos []BotTodo
	db.Where("bot = ? AND done = ?", "ditz", false).Order("created_at desc").Find(&todos)
	c.JSON(http.StatusOK, todos)
}

func markDitzTodoDone(c *gin.Context) {
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

func reopenDitzTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	todo.Done = false
	todo.Decision = ""
	todo.DoneAt = nil
	db.Save(&todo)
	c.JSON(http.StatusOK, todo)
}

func deleteDitzTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	db.Delete(&todo)
	c.JSON(http.StatusOK, gin.H{"message": "Todo deleted"})
}

func executeDitzTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	var req struct {
		IsLive      bool     `json:"is_live"`
		Price       *float64 `json:"price"`
		InvestedEUR *float64 `json:"invested_eur"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	now := time.Now()
	price := todo.Price
	if req.Price != nil {
		price = *req.Price
	}

	if todo.Type == "BUY" {
		var existingPos DitzPosition
		if err := db.Where("symbol = ? AND is_closed = ?", todo.Symbol, false).First(&existingPos).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Position already exists"})
			return
		}

		// Calculate quantity based on invested EUR (default 100 EUR)
		investmentEUR := 100.0
		if req.InvestedEUR != nil && *req.InvestedEUR > 0 {
			investmentEUR = *req.InvestedEUR
		}
		investmentUSD := convertToUSD(investmentEUR, "EUR")
		qty := math.Round((investmentUSD/price)*1000000) / 1000000
		if qty <= 0 {
			qty = 1
		}

		newTrade := DitzTrade{
			Symbol:     todo.Symbol,
			Name:       todo.Name,
			Action:     "BUY",
			Quantity:   qty,
			Price:      price,
			SignalDate: todo.CreatedAt,
			ExecutedAt: now,
			IsLive:     req.IsLive,
			IsPending:  false,
		}
		db.Create(&newTrade)

		newPosition := DitzPosition{
			Symbol:      todo.Symbol,
			Name:        todo.Name,
			Quantity:    qty,
			AvgPrice:    price,
			IsLive:      req.IsLive,
			IsPending:   false,
			BuyDate:     now,
			InvestedEUR: investmentEUR,
		}
		db.Create(&newPosition)

		portfolioPos := PortfolioPosition{
			UserID:       DITZ_USER_ID,
			Symbol:       todo.Symbol,
			Name:         todo.Name,
			AvgPrice:     price,
			PurchaseDate: &now,
			Quantity:     &qty,
		}
		db.Create(&portfolioPos)

	} else if todo.Type == "SELL" {
		var position DitzPosition
		if err := db.Where("symbol = ? AND is_closed = ?", todo.Symbol, false).First(&position).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Position not found"})
			return
		}

		pnl := price - position.AvgPrice
		pnlPct := (pnl / position.AvgPrice) * 100

		newTrade := DitzTrade{
			Symbol:        todo.Symbol,
			Name:          todo.Name,
			Action:        "SELL",
			Price:         price,
			SignalDate:    todo.CreatedAt,
			ExecutedAt:    now,
			IsLive:        position.IsLive,
			IsPending:     false,
			ProfitLoss:    &pnl,
			ProfitLossPct: &pnlPct,
		}
		db.Create(&newTrade)

		// Close position instead of deleting
		position.IsClosed = true
		position.SellPrice = price
		position.SellDate = &now
		position.ProfitLoss = &pnl
		position.ProfitLossPct = &pnlPct
		position.UpdatedAt = time.Now()
		db.Save(&position)
		db.Where("user_id = ? AND symbol = ?", DITZ_USER_ID, todo.Symbol).Delete(&PortfolioPosition{})
	}

	todo.Done = true
	todo.Decision = "executed"
	todo.DoneAt = &now
	db.Save(&todo)

	c.JSON(http.StatusOK, gin.H{"message": "Todo executed", "todo": todo})
}

func syncDitz(c *gin.Context) {
	var positions []DitzPosition
	db.Where("is_closed = ?", false).Find(&positions)

	for _, pos := range positions {
		var existingPosition DitzPosition
		if err := db.Where("symbol = ?", pos.Symbol).First(&existingPosition).Error; err != nil {
			continue
		}

		if pos.AvgPrice > 0 {
			var existingBuy DitzTrade
			if err := db.Where("symbol = ? AND action = ?", pos.Symbol, "BUY").Order("executed_at desc").First(&existingBuy).Error; err == nil {
				var lastSell DitzTrade
				if err := db.Where("symbol = ? AND action = ? AND executed_at > ?", pos.Symbol, "SELL", existingBuy.ExecutedAt).First(&lastSell).Error; err != nil {
					if existingBuy.Price != pos.AvgPrice {
						existingBuy.Price = pos.AvgPrice
						db.Save(&existingBuy)
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ditz sync complete"})
}

func getDitzHistory(c *gin.Context) {
	period := c.DefaultQuery("period", "1m")
	live := c.DefaultQuery("live", "true")

	botType := "ditz-live"
	if live == "false" {
		botType = "ditz-sim"
	}

	history := calculateBotHistory(botType, period)
	c.JSON(http.StatusOK, history)
}

func getDitzPendingTrades(c *gin.Context) {
	var trades []DitzTrade
	db.Where("is_pending = ?", true).Order("executed_at desc").Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func acceptDitzTrade(c *gin.Context) {
	id := c.Param("id")
	var trade DitzTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	trade.IsPending = false
	db.Save(&trade)

	if trade.Action == "BUY" {
		db.Model(&DitzPosition{}).Where("symbol = ? AND is_pending = ?", trade.Symbol, true).Update("is_pending", false)
	}

	c.JSON(http.StatusOK, trade)
}

// getDitzSimulatedPortfolio returns simulated/test positions (is_live = false) for Admin view
func getDitzSimulatedPortfolio(c *gin.Context) {
	// Show ALL open positions (both live and simulated) - live ones are marked with is_live badge
	var positions []DitzPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Order("buy_date desc").Find(&positions)

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

	// Realisierte Gewinne aus geschlossenen Trades einrechnen
	var closedSellTrades []DitzTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ?", "SELL", false, false).Find(&closedSellTrades)

	realizedPL := 0.0
	totalClosedInvested := 0.0
	for _, trade := range closedSellTrades {
		if trade.ProfitLoss != nil {
			realizedPL += *trade.ProfitLoss
			totalClosedInvested += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}

	overallReturn := totalReturn + realizedPL
	overallInvested := totalInvested + totalClosedInvested
	overallReturnPct := 0.0
	if overallInvested > 0 {
		overallReturnPct = (overallReturn / overallInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"positions":          result,
		"total_value":        totalValue,
		"total_invested":     totalInvested,
		"total_return":       totalReturn,
		"total_return_pct":   overallReturnPct,
		"realized_pl":        realizedPL,
		"overall_return":     overallReturn,
		"overall_invested":   overallInvested,
	})
}

// getDitzSimulatedPerformance returns performance stats for simulated/test trades (is_live = false) for Admin view
func getDitzSimulatedPerformance(c *gin.Context) {
	var sellTrades []DitzTrade
	db.Where("action = ? AND is_pending = ? AND is_live = ? AND is_deleted = ? AND is_admin_closed = ? AND is_filter_blocked = ?", "SELL", false, false, false, false, false).Find(&sellTrades)

	var buyTrades []DitzTrade
	db.Where("action = ? AND is_pending = ? AND is_live = ? AND is_deleted = ? AND is_admin_closed = ? AND is_filter_blocked = ?", "BUY", false, false, false, false, false).Find(&buyTrades)

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

	totalReturnPctClosed := 0.0
	for _, trade := range sellTrades {
		if trade.ProfitLossPct != nil {
			totalReturnPctClosed += *trade.ProfitLossPct
		}
	}
	avgReturnPerTrade := 0.0
	if len(sellTrades) > 0 {
		avgReturnPerTrade = totalReturnPctClosed / float64(len(sellTrades))
	}

	// Get open positions for unrealized P/L (simulated trades)
	var positions []DitzPosition
	db.Where("is_pending = ? AND is_live = ? AND is_closed = ?", false, false, false).Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	unrealizedGain := 0.0
	investedInPositions := 0.0
	currentValue := 0.0

	for _, pos := range positions {
		quote := quotes[pos.Symbol]
		currentPrice := quote.Price
		if currentPrice <= 0 {
			currentPrice = pos.AvgPrice
		}
		investedInPositions += pos.AvgPrice * pos.Quantity
		currentValue += currentPrice * pos.Quantity
		unrealizedGain += (currentPrice - pos.AvgPrice) * pos.Quantity
	}

	totalReturnPct := 0.0
	if investedInPositions > 0 {
		totalReturnPct = (unrealizedGain / investedInPositions) * 100
	}

	totalGain := totalProfitLoss + unrealizedGain
	totalInvestedAll := investedInPositions
	for _, trade := range sellTrades {
		if trade.ProfitLoss != nil {
			totalInvestedAll += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}
	overallReturnPct := 0.0
	if totalInvestedAll > 0 {
		overallReturnPct = (totalGain / totalInvestedAll) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_trades":          len(buyTrades) + len(sellTrades),
		"total_buys":            len(buyTrades),
		"open_positions":        len(positions),
		"closed_trades":         len(sellTrades),
		"wins":                  wins,
		"losses":                losses,
		"win_rate":              winRate,
		"realized_profit":       totalProfitLoss,
		"avg_return_per_trade":  avgReturnPerTrade,
		"unrealized_gain":       unrealizedGain,
		"invested_in_positions": investedInPositions,
		"current_value":         currentValue,
		"total_gain":            totalGain,
		"total_return_pct":      totalReturnPct,
		"overall_return_pct":    overallReturnPct,
	})
}


func saveTraderStockPerformance(c *gin.Context) {
	var req struct {
		Symbol       string      `json:"symbol" binding:"required"`
		Name         string      `json:"name"`
		WinRate      float64     `json:"win_rate"`
		RiskReward   float64     `json:"risk_reward"`
		TotalReturn  float64     `json:"total_return"`
		AvgReturn    float64     `json:"avg_return"`
		TotalTrades  int         `json:"total_trades"`
		Wins         int         `json:"wins"`
		Losses       int         `json:"losses"`
		Signal       string      `json:"signal"`
		SignalBars   int         `json:"signal_bars"`
		Trades       []TradeData `json:"trades"`
		CurrentPrice float64     `json:"current_price"`
		MarketCap    int64       `json:"market_cap"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	symbol := strings.ToUpper(req.Symbol)
	tradesJSON, _ := json.Marshal(req.Trades)

	newSignalSince := calcSignalSinceFromRequest(req.Trades, req.SignalBars)

	var existing TraderStockPerformance
	result := db.Where("symbol = ?", symbol).First(&existing)

	if result.Error == nil {
		ss, ps, pss := updateSignalHistory(existing.Signal, existing.SignalSince, req.Signal, newSignalSince)
		existing.Name = req.Name
		existing.WinRate = req.WinRate
		existing.RiskReward = req.RiskReward
		existing.TotalReturn = req.TotalReturn
		existing.AvgReturn = req.AvgReturn
		existing.TotalTrades = req.TotalTrades
		existing.Wins = req.Wins
		existing.Losses = req.Losses
		existing.Signal = req.Signal
		existing.SignalBars = req.SignalBars
		existing.SignalSince = ss
		if ps != "" {
			existing.PrevSignal = ps
			existing.PrevSignalSince = pss
		}
		existing.TradesJSON = string(tradesJSON)
		existing.CurrentPrice = req.CurrentPrice
		if req.MarketCap > 0 {
			existing.MarketCap = req.MarketCap
		}
		existing.UpdatedAt = time.Now()
		db.Save(&existing)
		c.JSON(http.StatusOK, existing)
	} else {
		perf := TraderStockPerformance{
			Symbol:       symbol,
			Name:         req.Name,
			WinRate:      req.WinRate,
			RiskReward:   req.RiskReward,
			TotalReturn:  req.TotalReturn,
			AvgReturn:    req.AvgReturn,
			TotalTrades:  req.TotalTrades,
			Wins:         req.Wins,
			Losses:       req.Losses,
			Signal:       req.Signal,
			SignalBars:   req.SignalBars,
			SignalSince:  newSignalSince,
			TradesJSON:   string(tradesJSON),
			CurrentPrice: req.CurrentPrice,
			MarketCap:    req.MarketCap,
		}
		db.Create(&perf)
		c.JSON(http.StatusCreated, perf)
	}
}

func getTraderTrackedStocks(c *gin.Context) {
	var performances []TraderStockPerformance
	db.Order("updated_at desc").Find(&performances)

	type PerformanceWithTrades struct {
		TraderStockPerformance
		Trades []TradeData `json:"trades"`
	}

	result := make([]PerformanceWithTrades, len(performances))
	for i, p := range performances {
		result[i].TraderStockPerformance = p
		if p.TradesJSON != "" {
			json.Unmarshal([]byte(p.TradesJSON), &result[i].Trades)
		}
	}

	c.JSON(http.StatusOK, result)
}

func getTraderStockPerformance(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))

	var perf TraderStockPerformance
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

// Trader config handlers
func getBXtrenderTraderConfigPublic(c *gin.Context) {
	var config BXtrenderTraderConfig
	result := db.First(&config)

	if result.Error != nil {
		// Return default config
		config = BXtrenderTraderConfig{
			ShortL1:    5,
			ShortL2:    20,
			ShortL3:    15,
			LongL1:     20,
			LongL2:     15,
			MaFilterOn: false,
			MaLength:   200,
			MaType:     "EMA",
			TslPercent: 20.0,
		}
	}

	c.JSON(http.StatusOK, config)
}

func getBXtrenderTraderConfig(c *gin.Context) {
	var config BXtrenderTraderConfig
	result := db.First(&config)

	if result.Error != nil {
		// Return default config
		config = BXtrenderTraderConfig{
			ShortL1:    5,
			ShortL2:    20,
			ShortL3:    15,
			LongL1:     20,
			LongL2:     15,
			MaFilterOn: false,
			MaLength:   200,
			MaType:     "EMA",
			TslPercent: 20.0,
		}
	}

	c.JSON(http.StatusOK, config)
}

func updateBXtrenderTraderConfig(c *gin.Context) {
	var req struct {
		ShortL1    int     `json:"short_l1"`
		ShortL2    int     `json:"short_l2"`
		ShortL3    int     `json:"short_l3"`
		LongL1     int     `json:"long_l1"`
		LongL2     int     `json:"long_l2"`
		MaFilterOn bool    `json:"ma_filter_on"`
		MaLength   int     `json:"ma_length"`
		MaType     string  `json:"ma_type"`
		TslPercent float64 `json:"tsl_percent"`
		TslEnabled *bool   `json:"tsl_enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var config BXtrenderTraderConfig
	result := db.First(&config)

	if result.Error != nil {
		tslEnabled := true
		if req.TslEnabled != nil {
			tslEnabled = *req.TslEnabled
		}
		config = BXtrenderTraderConfig{
			ShortL1:    req.ShortL1,
			ShortL2:    req.ShortL2,
			ShortL3:    req.ShortL3,
			LongL1:     req.LongL1,
			LongL2:     req.LongL2,
			MaFilterOn: req.MaFilterOn,
			MaLength:   req.MaLength,
			MaType:     req.MaType,
			TslPercent: req.TslPercent,
			TslEnabled: tslEnabled,
			UpdatedAt:  time.Now(),
		}
		db.Create(&config)
	} else {
		config.ShortL1 = req.ShortL1
		config.ShortL2 = req.ShortL2
		config.ShortL3 = req.ShortL3
		config.LongL1 = req.LongL1
		config.LongL2 = req.LongL2
		config.MaFilterOn = req.MaFilterOn
		config.MaLength = req.MaLength
		config.MaType = req.MaType
		config.TslPercent = req.TslPercent
		if req.TslEnabled != nil {
			config.TslEnabled = *req.TslEnabled
		}
		config.UpdatedAt = time.Now()
		db.Save(&config)
	}

	c.JSON(http.StatusOK, config)
}

// runTraderUpdateInternal performs the Trader bot update without HTTP context
func runTraderUpdateInternal(triggeredBy string) {
	checkTraderStopLoss()

	now := time.Now()
	sessionID := uuid.New().String()

	var logs []map[string]interface{}
	addLog := func(level, msg string) {
		logs = append(logs, map[string]interface{}{"level": level, "message": msg, "time": time.Now().Format("15:04:05")})
		db.Create(&BotLog{Bot: "trader", Level: level, Message: msg, SessionID: sessionID, CreatedAt: time.Now()})
	}

	addLog("INFO", fmt.Sprintf("Trader Update gestartet um %s (von: %s)", now.Format("15:04:05"), triggeredBy))

	var traderBotConfig BXtrenderTraderConfig
	db.First(&traderBotConfig)

	var perfData []TraderStockPerformance
	if err := db.Find(&perfData).Error; err != nil {
		addLog("ERROR", fmt.Sprintf("Fehler beim Laden der Performance Daten: %v", err))
		return
	}

	addLog("INFO", fmt.Sprintf("%d Aktien geladen", len(perfData)))

	// Phase 1: Validate existing positions and trades against current BXTrender data
	var existingPositions []TraderPosition
	db.Where("is_live = ? AND is_closed = ?", false, false).Find(&existingPositions)

	for _, pos := range existingPositions {
		// Find matching performance data
		var stockPerf *TraderStockPerformance
		for i := range perfData {
			if perfData[i].Symbol == pos.Symbol {
				stockPerf = &perfData[i]
				break
			}
		}

		if stockPerf == nil {
			addLog("WARN", fmt.Sprintf("%s: Position vorhanden aber keine Performance-Daten - überspringe Validierung", pos.Symbol))
			continue
		}

		// Parse TradesJSON to find the matching open BUY trade
		if stockPerf.TradesJSON == "" {
			continue
		}
		var serverTrades []ServerTrade
		if err := json.Unmarshal([]byte(stockPerf.TradesJSON), &serverTrades); err != nil {
			continue
		}

		// Find the last open BUY trade in TradesJSON (one without a following SELL)
		var lastBuyTrade *ServerTrade
		for i := len(serverTrades) - 1; i >= 0; i-- {
			if serverTrades[i].Type == "BUY" {
				lastBuyTrade = &serverTrades[i]
				break
			}
		}

		if stockPerf.Signal == "NO_DATA" {
			addLog("SKIP", fmt.Sprintf("%s: Nicht genug Daten für Berechnung - überspringe", pos.Symbol))
			continue
		}

		if stockPerf.Signal == "SELL" || stockPerf.Signal == "WAIT" {
			// BXTrender says no position should be open - but we have one
			addLog("KORREKTUR", fmt.Sprintf("%s: Signal ist jetzt %s, aber Position vorhanden - schließe Position", pos.Symbol, stockPerf.Signal))

			// Find the last SELL in TradesJSON for the correct close price/date
			sellPrice := stockPerf.CurrentPrice
			sellDate := now
			for i := len(serverTrades) - 1; i >= 0; i-- {
				if serverTrades[i].Type == "SELL" {
					sellPrice = serverTrades[i].Price
					sellDate = time.Unix(serverTrades[i].Time, 0)
					break
				}
			}

			sellTrade := TraderTrade{
				Symbol:     pos.Symbol,
				Name:       pos.Name,
				Action:     "SELL",
				Quantity:   pos.Quantity,
				Price:      sellPrice,
				SignalDate: sellDate,
				ExecutedAt: sellDate,
				IsPending:  false,
				IsLive:     pos.IsLive,
			}
			pnl := (sellPrice - pos.AvgPrice) * pos.Quantity
			pnlPct := ((sellPrice - pos.AvgPrice) / pos.AvgPrice) * 100
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct

			db.Create(&sellTrade)

			// Close position instead of deleting
			pos.IsClosed = true
			pos.SellPrice = sellPrice
			pos.SellDate = &sellDate
			pos.ProfitLoss = &pnl
			pos.ProfitLossPct = &pnlPct
			pos.UpdatedAt = time.Now()
			db.Save(&pos)
			db.Where("user_id = ? AND symbol = ?", TRADER_USER_ID, pos.Symbol).Delete(&PortfolioPosition{})

			addLog("KORREKTUR", fmt.Sprintf("%s: Position geschlossen @ $%.2f (P/L: %.2f%%)", pos.Symbol, sellPrice, pnlPct))
			continue
		}

		if lastBuyTrade != nil {
			// Validate price and date of existing position against TradesJSON
			expectedPrice := lastBuyTrade.Price
			expectedDate := time.Unix(lastBuyTrade.Time, 0)

			priceDiff := math.Abs(pos.AvgPrice-expectedPrice) / expectedPrice * 100
			dateDiff := pos.BuyDate.Sub(expectedDate).Hours()

			if priceDiff > 1.0 || math.Abs(dateDiff) > 48 {
				addLog("KORREKTUR", fmt.Sprintf("%s: Position korrigiert - Alt: $%.2f am %s, Neu: $%.2f am %s",
					pos.Symbol, pos.AvgPrice, pos.BuyDate.Format("02.01.2006"),
					expectedPrice, expectedDate.Format("02.01.2006")))

				// Update position
				investmentEUR := pos.InvestedEUR
				if investmentEUR == 0 {
					investmentEUR = 100.0
				}
				investmentUSD := convertToUSD(investmentEUR, "EUR")
				newQty := math.Round((investmentUSD/expectedPrice)*1000000) / 1000000

				db.Model(&pos).Updates(map[string]interface{}{
					"avg_price": expectedPrice,
					"buy_date":  expectedDate,
					"quantity":  newQty,
				})

				// Update matching BUY trade
				var buyTrade TraderTrade
				if err := db.Where("symbol = ? AND action = ? AND is_live = ?", pos.Symbol, "BUY", false).
					Order("created_at desc").First(&buyTrade).Error; err == nil {
					db.Model(&buyTrade).Updates(map[string]interface{}{
						"price":       expectedPrice,
						"signal_date": expectedDate,
						"executed_at": expectedDate,
						"quantity":    newQty,
					})
				}

				// Update portfolio position
				db.Model(&PortfolioPosition{}).
					Where("user_id = ? AND symbol = ?", TRADER_USER_ID, pos.Symbol).
					Updates(map[string]interface{}{
						"avg_price":     expectedPrice,
						"purchase_date": expectedDate,
						"quantity":      newQty,
					})
			}
		}
	}

	// Phase 2: Process new signals (BUY/SELL)
	for _, stock := range perfData {
		if !isStockAllowedForBot("trader", stock.Symbol) {
			continue
		}
		if stock.Signal == "BUY" {
			// Check if we already have an open position
			var existingPos TraderPosition
			if err := db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error; err == nil {
				addLog("SKIP", fmt.Sprintf("%s: Position bereits vorhanden", stock.Symbol))
				continue
			}

			// Check if there's a soft-deleted BUY (admin struck it out) - don't recreate
			var deletedBuy TraderTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ?", stock.Symbol, "BUY", true).Order("executed_at desc").First(&deletedBuy).Error; err == nil {
				var sellAfterDeleted TraderTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, deletedBuy.ExecutedAt).First(&sellAfterDeleted).Error; err != nil {
					addLog("SKIP", fmt.Sprintf("%s: Soft-deleted BUY vorhanden - überspringe", stock.Symbol))
					continue
				}
			}

			// Check if there's a recent BUY without a SELL
			var existingBuy TraderTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND is_filter_blocked = ?", stock.Symbol, "BUY", false, false).Order("executed_at desc").First(&existingBuy).Error; err == nil {
				var sellAfter TraderTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, existingBuy.ExecutedAt).First(&sellAfter).Error; err != nil {
					addLog("SKIP", fmt.Sprintf("%s: Bereits gekauft am %s", stock.Symbol, existingBuy.ExecutedAt.Format("02.01.2006")))
					continue
				}
			}

			// Check if there's already a filter-blocked BUY (don't create duplicates)
			var blockedBuy TraderTrade
			if err := db.Where("symbol = ? AND action = ? AND is_filter_blocked = ? AND is_deleted = ?", stock.Symbol, "BUY", true, false).Order("executed_at desc").First(&blockedBuy).Error; err == nil {
				var sellAfterBlocked TraderTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND executed_at > ?", stock.Symbol, "SELL", false, blockedBuy.ExecutedAt).First(&sellAfterBlocked).Error; err != nil {
					continue
				}
			}

			// Extract signal date and price from TradesJSON (last BUY trade)
			signalPrice := stock.CurrentPrice
			signalDate := now
			if stock.TradesJSON != "" {
				var serverTrades []ServerTrade
				if err := json.Unmarshal([]byte(stock.TradesJSON), &serverTrades); err == nil {
					for i := len(serverTrades) - 1; i >= 0; i-- {
						if serverTrades[i].Type == "BUY" {
							signalPrice = serverTrades[i].Price
							signalDate = time.Unix(serverTrades[i].Time, 0)
							addLog("DEBUG", fmt.Sprintf("%s: Signal-Datum aus TradesJSON: %s, Preis: $%.2f",
								stock.Symbol, signalDate.Format("02.01.2006"), signalPrice))
							break
						}
					}
				}
			}

			// Calculate quantity based on 100 EUR investment
			investmentEUR := 100.0
			investmentUSD := convertToUSD(investmentEUR, "EUR")
			qty := math.Round((investmentUSD/signalPrice)*1000000) / 1000000
			if qty <= 0 {
				addLog("SKIP", fmt.Sprintf("%s: Ungültige Menge berechnet", stock.Symbol))
				continue
			}

			// Check bot filter config
			filterBlocked, filterReason := checkBotFilterConfig("trader", stock.WinRate, stock.RiskReward, stock.AvgReturn, stock.MarketCap)
			if filterBlocked {
				blockedTrade := TraderTrade{
					Symbol:            stock.Symbol,
					Name:              stock.Name,
					Action:            "BUY",
					Quantity:          qty,
					Price:             signalPrice,
					SignalDate:        signalDate,
					ExecutedAt:        signalDate,
					IsPending:         false,
					IsLive:            false,
					IsFilterBlocked:   true,
					FilterBlockReason: filterReason,
				}
				db.Create(&blockedTrade)
				addLog("FILTER", fmt.Sprintf("%s: BUY blockiert durch Filter (%s)", stock.Symbol, filterReason))
				continue
			}

			buyTrade := TraderTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      signalPrice,
				SignalDate: signalDate,
				ExecutedAt: signalDate,
				IsPending:  false,
				IsLive:     false,
			}
			db.Create(&buyTrade)

			newPos := TraderPosition{
				Symbol:        stock.Symbol,
				Name:          stock.Name,
				Quantity:      qty,
				AvgPrice:      signalPrice,
				InvestedEUR:   investmentEUR,
				BuyDate:       signalDate,
				IsPending:     false,
				IsLive:        false,
				HighestPrice:  signalPrice,
				StopLossPrice: signalPrice * (1 - traderBotConfig.TslPercent/100),
				StopLossType:  "trailing",
			}
			db.Create(&newPos)

			portfolioPos := PortfolioPosition{
				UserID:       TRADER_USER_ID,
				Symbol:       stock.Symbol,
				Name:         stock.Name,
				PurchaseDate: &signalDate,
				AvgPrice:     signalPrice,
				Currency:     "USD",
				Quantity:     &qty,
			}
			db.Create(&portfolioPos)

			addLog("ACTION", fmt.Sprintf("BUY ausgeführt: %s %.6f @ $%.2f (Signal: %s)", stock.Symbol, qty, signalPrice, signalDate.Format("02.01.2006")))

		} else if stock.Signal == "SELL" {
			// Check if there's a soft-deleted SELL (admin struck it out) - don't recreate
			var deletedSell TraderTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ?", stock.Symbol, "SELL", true).Order("executed_at desc").First(&deletedSell).Error; err == nil {
				addLog("SKIP", fmt.Sprintf("%s: Soft-deleted SELL vorhanden - überspringe", stock.Symbol))
				continue
			}

			var existingPos TraderPosition
			if err := db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error; err != nil {
				addLog("SKIP", fmt.Sprintf("%s: SELL Signal aber keine offene Position", stock.Symbol))
				continue
			}

			sellPrice := stock.CurrentPrice
			sellDate := now
			if stock.TradesJSON != "" {
				var serverTrades []ServerTrade
				if err := json.Unmarshal([]byte(stock.TradesJSON), &serverTrades); err == nil {
					for i := len(serverTrades) - 1; i >= 0; i-- {
						if serverTrades[i].Type == "SELL" {
							sellPrice = serverTrades[i].Price
							sellDate = time.Unix(serverTrades[i].Time, 0)
							break
						}
					}
				}
			}

			sellTrade := TraderTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "SELL",
				Quantity:   existingPos.Quantity,
				Price:      sellPrice,
				SignalDate: sellDate,
				ExecutedAt: sellDate,
				IsPending:  false,
				IsLive:     existingPos.IsLive,
			}

			pnl := (sellPrice - existingPos.AvgPrice) * existingPos.Quantity
			pnlPct := ((sellPrice - existingPos.AvgPrice) / existingPos.AvgPrice) * 100
			sellTrade.ProfitLoss = &pnl
			sellTrade.ProfitLossPct = &pnlPct

			db.Create(&sellTrade)

			// Close position instead of deleting
			existingPos.IsClosed = true
			existingPos.SellPrice = sellPrice
			existingPos.SellDate = &sellDate
			existingPos.ProfitLoss = &pnl
			existingPos.ProfitLossPct = &pnlPct
			existingPos.UpdatedAt = time.Now()
			db.Save(&existingPos)
			db.Where("user_id = ? AND symbol = ?", TRADER_USER_ID, stock.Symbol).Delete(&PortfolioPosition{})

			addLog("ACTION", fmt.Sprintf("SELL ausgeführt: %s @ $%.2f (Signal: %s, P/L: %.2f%%)", stock.Symbol, sellPrice, sellDate.Format("02.01.2006"), pnlPct))
		}
	}

	addLog("INFO", "Trader Update abgeschlossen")

	lastRefresh := map[string]interface{}{
		"updated_at":   now,
		"triggered_by": triggeredBy,
		"logs":         logs,
	}
	lastRefreshJSON, _ := json.Marshal(lastRefresh)

	var setting SystemSetting
	if err := db.Where("key = ?", "last_trader_refresh").First(&setting).Error; err != nil {
		setting = SystemSetting{
			Key:       "last_trader_refresh",
			Value:     string(lastRefreshJSON),
			UpdatedAt: now,
		}
		db.Create(&setting)
	} else {
		setting.Value = string(lastRefreshJSON)
		setting.UpdatedAt = now
		db.Save(&setting)
	}
}

func traderUpdate(c *gin.Context) {
	// Get username from session
	triggeredBy := "system"
	if userID, exists := c.Get("userID"); exists {
		var user User
		if err := db.First(&user, userID).Error; err == nil {
			triggeredBy = user.Username
		}
	}

	runTraderUpdateInternal(triggeredBy)

	// Read back the logs from the last refresh
	var setting SystemSetting
	if err := db.Where("key = ?", "last_trader_refresh").First(&setting).Error; err == nil {
		var lastRefresh map[string]interface{}
		if err := json.Unmarshal([]byte(setting.Value), &lastRefresh); err == nil {
			c.JSON(http.StatusOK, gin.H{"message": "Trader update completed", "logs": lastRefresh["logs"], "triggered_by": triggeredBy, "updated_at": lastRefresh["updated_at"]})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Trader update completed", "triggered_by": triggeredBy})
}

func getTraderPortfolio(c *gin.Context) {
	// Return all trades (live + simulated) - frontend filters by is_live
	var positions []TraderPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Order("buy_date desc").Find(&positions)

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

	// Realisierte Gewinne aus geschlossenen Trades einrechnen
	var closedSellTrades []TraderTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ?", "SELL", false, false).Find(&closedSellTrades)

	realizedPL := 0.0
	totalClosedInvested := 0.0
	for _, trade := range closedSellTrades {
		if trade.ProfitLoss != nil {
			realizedPL += *trade.ProfitLoss
			totalClosedInvested += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}

	overallReturn := totalReturn + realizedPL
	overallInvested := totalInvested + totalClosedInvested
	overallReturnPct := 0.0
	if overallInvested > 0 {
		overallReturnPct = (overallReturn / overallInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"positions":          result,
		"total_value":        totalValue,
		"total_invested":     totalInvested,
		"total_return":       totalReturn,
		"total_return_pct":   overallReturnPct,
		"realized_pl":        realizedPL,
		"overall_return":     overallReturn,
		"overall_invested":   overallInvested,
	})
}

func getTraderActions(c *gin.Context) {
	// Return all trades (live + simulated) - frontend filters by is_live
	var trades []TraderTrade
	db.Where("is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false).Order("signal_date desc").Limit(50).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getTraderActionsAll(c *gin.Context) {
	// Admin view: return ALL trades (live + simulated)
	var trades []TraderTrade
	db.Where("is_pending = ?", false).Order("signal_date desc").Limit(100).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getTraderPerformance(c *gin.Context) {
	// Return all trades (live + simulated) - frontend filters by is_live
	var sellTrades []TraderTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false, false).Find(&sellTrades)

	var buyTrades []TraderTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "BUY", false, false, false).Find(&buyTrades)

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

	totalReturnPctClosed := 0.0
	for _, trade := range sellTrades {
		if trade.ProfitLossPct != nil {
			totalReturnPctClosed += *trade.ProfitLossPct
		}
	}
	avgReturnPerTrade := 0.0
	if len(sellTrades) > 0 {
		avgReturnPerTrade = totalReturnPctClosed / float64(len(sellTrades))
	}

	var positions []TraderPosition
	db.Where("is_pending = ? AND is_live = ? AND is_closed = ?", false, true, false).Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	unrealizedGain := 0.0
	investedInPositions := 0.0
	currentValue := 0.0
	liveCount := 0

	for _, pos := range positions {
		if pos.IsLive {
			liveCount++
		}
		investedInPositions += pos.AvgPrice * pos.Quantity
		quote := quotes[pos.Symbol]
		if quote.Price > 0 {
			currentValue += quote.Price * pos.Quantity
			unrealizedGain += (quote.Price - pos.AvgPrice) * pos.Quantity
		} else {
			currentValue += pos.AvgPrice * pos.Quantity
		}
	}

	unrealizedGainPct := 0.0
	if investedInPositions > 0 {
		unrealizedGainPct = (unrealizedGain / investedInPositions) * 100
	}

	totalGain := totalProfitLoss + unrealizedGain
	totalInvested := investedInPositions
	for _, trade := range sellTrades {
		if trade.ProfitLoss != nil {
			totalInvested += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}
	overallReturnPct := 0.0
	if totalInvested > 0 {
		overallReturnPct = (totalGain / totalInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_trades":         len(buyTrades) + len(sellTrades),
		"total_buys":           len(buyTrades),
		"completed_trades":     len(sellTrades),
		"open_positions":       len(positions),
		"live_positions":       liveCount,
		"wins":                 wins,
		"losses":               losses,
		"win_rate":             winRate,
		"realized_profit":      totalProfitLoss,
		"total_gain":           totalGain,
		"overall_return_pct":   overallReturnPct,
		"avg_return_per_trade": avgReturnPerTrade,
		"unrealized_gain":      unrealizedGain,
		"total_return_pct":     unrealizedGainPct,
		"invested_in_positions": investedInPositions,
		"current_value":        currentValue,
	})
}

func resetTrader(c *gin.Context) {
	db.Where("1 = 1").Delete(&TraderTrade{})
	db.Where("1 = 1").Delete(&TraderPosition{})
	db.Where("user_id = ?", TRADER_USER_ID).Delete(&PortfolioPosition{})
	db.Where("bot = ?", "trader").Delete(&BotTodo{})
	db.Where("bot = ?", "trader").Delete(&BotLog{})
	c.JSON(http.StatusOK, gin.H{"message": "Trader reset complete"})
}

// getLastTraderRefresh returns the last trader refresh info with logs
func getLastTraderRefresh(c *gin.Context) {
	var setting SystemSetting
	if err := db.Where("key = ?", "last_trader_refresh").First(&setting).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"updated_at":   nil,
			"triggered_by": nil,
			"logs":         []interface{}{},
		})
		return
	}

	var lastRefresh map[string]interface{}
	if err := json.Unmarshal([]byte(setting.Value), &lastRefresh); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"updated_at":   setting.UpdatedAt,
			"triggered_by": "unknown",
			"logs":         []interface{}{},
		})
		return
	}

	c.JSON(http.StatusOK, lastRefresh)
}

// cleanupTraderPending deletes all pending trades and positions, and all todos
func cleanupTraderPending(c *gin.Context) {
	// Delete pending trades
	result1 := db.Where("is_pending = ?", true).Delete(&TraderTrade{})
	// Delete pending positions
	result2 := db.Where("is_pending = ?", true).Delete(&TraderPosition{})
	// Delete all todos for trader bot
	result3 := db.Where("bot = ?", "trader").Delete(&BotTodo{})

	c.JSON(http.StatusOK, gin.H{
		"message":             "Cleanup complete",
		"deleted_trades":      result1.RowsAffected,
		"deleted_positions":   result2.RowsAffected,
		"deleted_todos":       result3.RowsAffected,
	})
}

func traderBackfill(c *gin.Context) {
	var req struct {
		UntilDate string `json:"until_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "until_date required"})
		return
	}

	fromDate, err := time.Parse("2006-01-02", req.UntilDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format (use YYYY-MM-DD)"})
		return
	}

	now := time.Now()
	sessionID := uuid.New().String()
	var logs []map[string]interface{}
	addLog := func(level, message string) {
		entry := map[string]interface{}{
			"level":   level,
			"message": message,
			"time":    time.Now().Format("15:04:05"),
		}
		logs = append(logs, entry)
		saveBotLog("trader", level, message, sessionID)
	}

	addLog("INFO", fmt.Sprintf("Trader Backfill gestartet ab %s bis heute", req.UntilDate))

	// Get all tracked stocks with their trader performance data
	var trackedStocks []TraderStockPerformance
	db.Find(&trackedStocks)

	if len(trackedStocks) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message":        "No tracked stocks found",
			"trades_created": 0,
			"logs":           logs,
		})
		return
	}

	var tradesCreated int
	var positionsCreated int

	for _, stock := range trackedStocks {
		if stock.TradesJSON == "" {
			continue
		}

		// Check allowlist
		if !isStockAllowedForBot("trader", stock.Symbol) {
			addLog("SKIP", fmt.Sprintf("%s: Nicht in Allowlist — übersprungen", stock.Symbol))
			continue
		}

		var historicalTrades []TradeData
		if err := json.Unmarshal([]byte(stock.TradesJSON), &historicalTrades); err != nil {
			addLog("ERROR", fmt.Sprintf("%s: Fehler beim Parsen der Trades: %v", stock.Symbol, err))
			continue
		}

		// Check if there's already an open position from BEFORE the backfill start date
		hasOpenPositionBefore := false
		for _, t := range historicalTrades {
			entryT := time.Unix(t.EntryDate, 0)
			if t.IsOpen && entryT.Before(fromDate) {
				hasOpenPositionBefore = true
				break
			}
		}
		if hasOpenPositionBefore {
			addLog("SKIP", fmt.Sprintf("%s: Offene Position vor Startdatum (HOLD) — übersprungen", stock.Symbol))
			continue
		}

		for _, trade := range historicalTrades {
			entryTime := time.Unix(trade.EntryDate, 0).UTC()
			entryTime = time.Date(entryTime.Year(), entryTime.Month(), 1, 0, 0, 0, 0, time.UTC)

			if entryTime.Year() < 2020 || entryTime.Year() > 2030 {
				continue
			}
			if entryTime.Before(fromDate) {
				continue
			}
			if entryTime.After(now) {
				continue
			}

			var existingBuy TraderTrade
			dateStart := entryTime.Truncate(24 * time.Hour)
			dateEnd := dateStart.Add(24 * time.Hour)
			alreadyExists := db.Where("symbol = ? AND action = ? AND signal_date >= ? AND signal_date < ?",
				stock.Symbol, "BUY", dateStart, dateEnd).First(&existingBuy).Error == nil
			if alreadyExists {
				continue
			}

			investmentEUR := 100.0
			investmentUSD := convertToUSD(investmentEUR, "EUR")
			qty := math.Round((investmentUSD/trade.EntryPrice)*1000000) / 1000000
			if qty <= 0 || trade.EntryPrice <= 0 {
				continue
			}

			buyTrade := TraderTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      trade.EntryPrice,
				SignalDate: entryTime,
				ExecutedAt: entryTime,
				IsPending:  false,
			}
			db.Create(&buyTrade)
			tradesCreated++
			addLog("ACTION", fmt.Sprintf("%s: BUY erstellt @ $%.2f am %s", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))

			if trade.ExitDate != nil && trade.ExitPrice != nil {
				exitTime := time.Unix(*trade.ExitDate, 0).UTC()
				exitTime = time.Date(exitTime.Year(), exitTime.Month(), 1, 0, 0, 0, 0, time.UTC)

				if !exitTime.After(now) {
					profitLoss := (*trade.ExitPrice - trade.EntryPrice) * qty
					profitLossPct := trade.ReturnPct

					sellTrade := TraderTrade{
						Symbol:        stock.Symbol,
						Name:          stock.Name,
						Action:        "SELL",
						Quantity:      qty,
						Price:         *trade.ExitPrice,
						SignalDate:    exitTime,
						ExecutedAt:    exitTime,
						IsPending:     false,
						ProfitLoss:    &profitLoss,
						ProfitLossPct: &profitLossPct,
					}
					db.Create(&sellTrade)
					tradesCreated++
					addLog("ACTION", fmt.Sprintf("%s: SELL erstellt @ $%.2f am %s (%.2f%%)", stock.Symbol, *trade.ExitPrice, exitTime.Format("2006-01-02"), profitLossPct))
				} else {
					var existingPos TraderPosition
					if db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error != nil {
						newPos := TraderPosition{
							Symbol:      stock.Symbol,
							Name:        stock.Name,
							Quantity:    qty,
							AvgPrice:    trade.EntryPrice,
							InvestedEUR: investmentEUR,
							BuyDate:     entryTime,
							IsPending:   false,
						}
						db.Create(&newPos)
						positionsCreated++

						portfolioPos := PortfolioPosition{
							UserID:       TRADER_USER_ID,
							Symbol:       stock.Symbol,
							Name:         stock.Name,
							PurchaseDate: &entryTime,
							AvgPrice:     trade.EntryPrice,
							Currency:     "USD",
							Quantity:     &qty,
						}
						db.Create(&portfolioPos)
						addLog("ACTION", fmt.Sprintf("%s: Position erstellt (offen)", stock.Symbol))
					}
				}
			} else if trade.IsOpen {
				var existingPos TraderPosition
				if db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingPos).Error != nil {
					newPos := TraderPosition{
						Symbol:      stock.Symbol,
						Name:        stock.Name,
						Quantity:    qty,
						AvgPrice:    trade.EntryPrice,
						InvestedEUR: investmentEUR,
						BuyDate:     entryTime,
						IsPending:   false,
					}
					db.Create(&newPos)
					positionsCreated++

					portfolioPos := PortfolioPosition{
						UserID:       TRADER_USER_ID,
						Symbol:       stock.Symbol,
						Name:         stock.Name,
						PurchaseDate: &entryTime,
						AvgPrice:     trade.EntryPrice,
						Currency:     "USD",
						Quantity:     &qty,
					}
					db.Create(&portfolioPos)
					addLog("ACTION", fmt.Sprintf("%s: Position erstellt (offen)", stock.Symbol))
				}
			}
		}
	}

	addLog("INFO", fmt.Sprintf("Trader Backfill abgeschlossen: %d Trades, %d Positionen erstellt", tradesCreated, positionsCreated))

	c.JSON(http.StatusOK, gin.H{
		"message":           "Backfill completed",
		"trades_created":    tradesCreated,
		"positions_created": positionsCreated,
		"until_date":        req.UntilDate,
		"logs":              logs,
	})
}

func getTraderCompletedTrades(c *gin.Context) {
	var trades []TraderTrade
	db.Where("action = ? AND profit_loss IS NOT NULL AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false).Order("executed_at desc").Find(&trades)

	type CompletedTrade struct {
		Symbol        string     `json:"symbol"`
		Name          string     `json:"name"`
		BuyPrice      float64    `json:"buy_price"`
		SellPrice     float64    `json:"sell_price"`
		BuyDate       time.Time  `json:"buy_date"`
		SellDate      time.Time  `json:"sell_date"`
		ProfitLoss    float64    `json:"profit_loss"`
		ProfitLossPct float64    `json:"profit_loss_pct"`
		IsLive        bool       `json:"is_live"`
	}

	var result []CompletedTrade
	for _, sell := range trades {
		var buy TraderTrade
		if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND is_filter_blocked = ? AND executed_at < ?", sell.Symbol, "BUY", false, false, sell.ExecutedAt).Order("executed_at desc").First(&buy).Error; err != nil {
			continue
		}

		ct := CompletedTrade{
			Symbol:    sell.Symbol,
			Name:      sell.Name,
			BuyPrice:  buy.Price,
			SellPrice: sell.Price,
			BuyDate:   buy.ExecutedAt,
			SellDate:  sell.ExecutedAt,
			IsLive:    sell.IsLive,
		}
		if sell.ProfitLoss != nil {
			ct.ProfitLoss = *sell.ProfitLoss
		}
		if sell.ProfitLossPct != nil {
			ct.ProfitLossPct = *sell.ProfitLossPct
		}
		result = append(result, ct)
	}

	c.JSON(http.StatusOK, result)
}

func updateTraderPosition(c *gin.Context) {
	id := c.Param("id")

	var position TraderPosition
	if err := db.First(&position, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Position not found"})
		return
	}

	var req struct {
		IsLive          *bool    `json:"is_live"`
		AvgPrice        *float64 `json:"avg_price"`
		InvestedEUR     *float64 `json:"invested_eur"`
		StopLossPercent *float64 `json:"stop_loss_percent"`
		StopLossType    *string  `json:"stop_loss_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.IsLive != nil {
		position.IsLive = *req.IsLive
	}
	if req.AvgPrice != nil {
		position.AvgPrice = *req.AvgPrice
	}
	if req.InvestedEUR != nil {
		position.InvestedEUR = *req.InvestedEUR
	}
	if req.StopLossPercent != nil {
		if *req.StopLossPercent <= 0 {
			position.StopLossPercent = nil
		} else {
			position.StopLossPercent = req.StopLossPercent
		}
	}
	if req.StopLossType != nil {
		position.StopLossType = *req.StopLossType
	}

	db.Save(&position)

	var portfolioPos PortfolioPosition
	if err := db.Where("user_id = ? AND symbol = ?", TRADER_USER_ID, position.Symbol).First(&portfolioPos).Error; err == nil {
		portfolioPos.AvgPrice = position.AvgPrice
		db.Save(&portfolioPos)
	}

	c.JSON(http.StatusOK, position)
}

func updateTraderTrade(c *gin.Context) {
	id := c.Param("id")

	var trade TraderTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	var req struct {
		IsLive     *bool      `json:"is_live"`
		Price      *float64   `json:"price"`
		Quantity   *float64   `json:"quantity"`
		SignalDate *time.Time `json:"signal_date"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.IsLive != nil {
		trade.IsLive = *req.IsLive
	}
	if req.Price != nil {
		trade.Price = *req.Price
	}
	if req.Quantity != nil {
		trade.Quantity = *req.Quantity
	}
	if req.SignalDate != nil {
		trade.SignalDate = *req.SignalDate
		trade.ExecutedAt = *req.SignalDate
	}

	db.Save(&trade)

	// Sync changes to matching position and portfolio entry
	if trade.Action == "BUY" {
		var position TraderPosition
		if err := db.Where("symbol = ? AND is_closed = ?", trade.Symbol, false).First(&position).Error; err == nil {
			if req.IsLive != nil {
				position.IsLive = *req.IsLive
			}
			if req.Price != nil {
				position.AvgPrice = *req.Price
			}
			if req.Quantity != nil {
				position.Quantity = *req.Quantity
			}
			if req.SignalDate != nil {
				position.BuyDate = *req.SignalDate
			}
			db.Save(&position)

			// Also update portfolio position
			var portfolioPos PortfolioPosition
			if err := db.Where("user_id = ? AND symbol = ?", TRADER_USER_ID, trade.Symbol).First(&portfolioPos).Error; err == nil {
				if req.Price != nil {
					portfolioPos.AvgPrice = *req.Price
				}
				if req.Quantity != nil {
					portfolioPos.Quantity = req.Quantity
				}
				if req.SignalDate != nil {
					portfolioPos.PurchaseDate = req.SignalDate
				}
				db.Save(&portfolioPos)
			}
		}
	} else if trade.Action == "SELL" && req.Price != nil {
		var position TraderPosition
		if err := db.Where("symbol = ? AND is_closed = ? AND is_live = ?", trade.Symbol, true, trade.IsLive).Order("updated_at desc").First(&position).Error; err == nil {
			position.SellPrice = *req.Price
			pnl := (*req.Price - position.AvgPrice) * position.Quantity
			pnlPct := ((*req.Price - position.AvgPrice) / position.AvgPrice) * 100
			position.ProfitLoss = &pnl
			position.ProfitLossPct = &pnlPct
			db.Save(&position)
		}
	}

	c.JSON(http.StatusOK, trade)
}

func createManualTraderTrade(c *gin.Context) {
	var req struct {
		Symbol   string  `json:"symbol" binding:"required"`
		Name     string  `json:"name"`
		Action   string  `json:"action" binding:"required"` // BUY or SELL
		Price    float64 `json:"price" binding:"required"`
		Quantity float64 `json:"quantity"`
		Date     string  `json:"date"` // YYYY-MM-DD
		IsLive   bool    `json:"is_live"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Symbol, Action und Price sind Pflichtfelder"})
		return
	}

	if req.Action != "BUY" && req.Action != "SELL" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Action muss BUY oder SELL sein"})
		return
	}

	// Parse date or use now
	signalDate := time.Now()
	if req.Date != "" {
		parsed, err := time.Parse("2006-01-02", req.Date)
		if err == nil {
			signalDate = parsed
		}
	}

	// Default quantity: 100 EUR worth
	qty := req.Quantity
	if qty <= 0 {
		investmentEUR := 100.0
		investmentUSD := convertToUSD(investmentEUR, "EUR")
		qty = math.Round((investmentUSD/req.Price)*1000000) / 1000000
		if qty <= 0 {
			qty = 1
		}
	}

	// Resolve name if not provided
	name := req.Name
	if name == "" {
		name = req.Symbol
	}

	if req.Action == "BUY" {
		// Check for existing open position
		var existingPos TraderPosition
		if err := db.Where("symbol = ? AND is_closed = ?", req.Symbol, false).First(&existingPos).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("Offene Position für %s existiert bereits", req.Symbol)})
			return
		}

		trade := TraderTrade{
			Symbol:     req.Symbol,
			Name:       name,
			Action:     "BUY",
			Quantity:   qty,
			Price:      req.Price,
			SignalDate: signalDate,
			ExecutedAt: signalDate,
			IsPending:  false,
			IsLive:     req.IsLive,
		}
		db.Create(&trade)

		investmentEUR := 100.0
		if req.Quantity > 0 {
			investmentEUR = req.Price * req.Quantity / convertToUSD(1.0, "EUR")
		}

		pos := TraderPosition{
			Symbol:      req.Symbol,
			Name:        name,
			Quantity:    qty,
			AvgPrice:    req.Price,
			InvestedEUR: investmentEUR,
			BuyDate:     signalDate,
			IsPending:   false,
			IsLive:      req.IsLive,
		}
		db.Create(&pos)

		portfolioPos := PortfolioPosition{
			UserID:       TRADER_USER_ID,
			Symbol:       req.Symbol,
			Name:         name,
			PurchaseDate: &signalDate,
			AvgPrice:     req.Price,
			Currency:     "USD",
			Quantity:     &qty,
		}
		db.Create(&portfolioPos)

		c.JSON(http.StatusOK, gin.H{"trade": trade, "position": pos})

	} else {
		// SELL - must have existing open position
		var existingPos TraderPosition
		if err := db.Where("symbol = ? AND is_closed = ?", req.Symbol, false).First(&existingPos).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Keine offene Position für %s vorhanden", req.Symbol)})
			return
		}

		sellQty := qty
		if req.Quantity <= 0 {
			sellQty = existingPos.Quantity
		}

		trade := TraderTrade{
			Symbol:     req.Symbol,
			Name:       name,
			Action:     "SELL",
			Quantity:   sellQty,
			Price:      req.Price,
			SignalDate: signalDate,
			ExecutedAt: signalDate,
			IsPending:  false,
			IsLive:     existingPos.IsLive,
		}

		pnl := (req.Price - existingPos.AvgPrice) * sellQty
		pnlPct := ((req.Price - existingPos.AvgPrice) / existingPos.AvgPrice) * 100
		trade.ProfitLoss = &pnl
		trade.ProfitLossPct = &pnlPct
		db.Create(&trade)

		// Close position instead of deleting
		existingPos.IsClosed = true
		existingPos.SellPrice = req.Price
		existingPos.SellDate = &signalDate
		existingPos.ProfitLoss = &pnl
		existingPos.ProfitLossPct = &pnlPct
		existingPos.UpdatedAt = time.Now()
		db.Save(&existingPos)
		db.Where("user_id = ? AND symbol = ?", TRADER_USER_ID, req.Symbol).Delete(&PortfolioPosition{})

		c.JSON(http.StatusOK, gin.H{"trade": trade})
	}
}

func deleteTraderTrade(c *gin.Context) {
	id := c.Param("id")

	var trade TraderTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	symbol := trade.Symbol
	wasDeleted := trade.IsDeleted

	// Toggle soft-delete
	trade.IsDeleted = !wasDeleted
	db.Save(&trade)

	if trade.Action == "BUY" {
		if !wasDeleted {
			// Soft-deleting a BUY → also soft-delete matching SELL, hard-delete position + portfolio
			var sellTrade TraderTrade
			if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "SELL", false).
				Order("signal_date desc").First(&sellTrade).Error; err == nil {
				sellTrade.IsDeleted = true
				db.Save(&sellTrade)
			}
			db.Where("symbol = ? AND is_live = ?", symbol, trade.IsLive).Delete(&TraderPosition{})
			db.Where("user_id = ? AND symbol = ?", TRADER_USER_ID, symbol).Delete(&PortfolioPosition{})
		} else {
			// Restoring a BUY → also restore matching SELL, recreate position
			var sellTrade TraderTrade
			hasSell := false
			if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "SELL", true).
				Order("signal_date desc").First(&sellTrade).Error; err == nil {
				sellTrade.IsDeleted = false
				db.Save(&sellTrade)
				hasSell = true
			}

			qty := trade.Quantity
			buyDate := trade.SignalDate
			newPos := TraderPosition{
				Symbol:   symbol,
				Name:     trade.Name,
				Quantity: qty,
				AvgPrice: trade.Price,
				IsLive:   trade.IsLive,
				BuyDate:  buyDate,
			}

			if hasSell {
				newPos.IsClosed = true
				newPos.SellPrice = sellTrade.Price
				sellDate := sellTrade.SignalDate
				newPos.SellDate = &sellDate
				newPos.ProfitLoss = sellTrade.ProfitLoss
				newPos.ProfitLossPct = sellTrade.ProfitLossPct
				db.Create(&newPos)
			} else {
				db.Create(&newPos)
				portfolioPos := PortfolioPosition{
					UserID:       TRADER_USER_ID,
					Symbol:       symbol,
					Name:         trade.Name,
					AvgPrice:     trade.Price,
					PurchaseDate: &buyDate,
					Quantity:     &qty,
				}
				db.Create(&portfolioPos)
			}
		}
	} else if trade.Action == "SELL" {
		// Check if the corresponding BUY is deleted
		var buyTrade TraderTrade
		buyDeleted := false
		if err := db.Where("symbol = ? AND is_live = ? AND action = ? AND is_deleted = ?", symbol, trade.IsLive, "BUY", true).
			Order("signal_date desc").First(&buyTrade).Error; err == nil {
			buyDeleted = true
		}

		if buyDeleted {
			// BUY is deleted → just toggle SELL, no position changes
		} else if !wasDeleted {
			// Soft-deleting a SELL (BUY active) → reopen position
			var pos TraderPosition
			if err := db.Where("symbol = ? AND is_live = ?", symbol, trade.IsLive).Order("updated_at desc").First(&pos).Error; err == nil {
				pos.IsClosed = false
				pos.SellPrice = 0
				pos.SellDate = nil
				pos.ProfitLoss = nil
				pos.ProfitLossPct = nil
				pos.UpdatedAt = time.Now()
				db.Save(&pos)

				qty := pos.Quantity
				buyDate := pos.BuyDate
				portfolioPos := PortfolioPosition{
					UserID:       TRADER_USER_ID,
					Symbol:       pos.Symbol,
					Name:         pos.Name,
					AvgPrice:     pos.AvgPrice,
					PurchaseDate: &buyDate,
					Quantity:     &qty,
				}
				db.Create(&portfolioPos)
			}
		} else {
			// Restoring a SELL (BUY active) → re-close position
			var pos TraderPosition
			if err := db.Where("symbol = ? AND is_live = ? AND is_closed = ?", symbol, trade.IsLive, false).First(&pos).Error; err == nil {
				pos.IsClosed = true
				pos.SellPrice = trade.Price
				sellDate := trade.SignalDate
				pos.SellDate = &sellDate
				pos.ProfitLoss = trade.ProfitLoss
				pos.ProfitLossPct = trade.ProfitLossPct
				pos.UpdatedAt = time.Now()
				db.Save(&pos)
			}
			db.Where("user_id = ? AND symbol = ?", TRADER_USER_ID, symbol).Delete(&PortfolioPosition{})
		}
	}

	action := "deleted"
	if wasDeleted {
		action = "restored"
	}
	c.JSON(http.StatusOK, gin.H{"message": "Trade " + action, "trade": trade})
}

func toggleTraderTradeRead(c *gin.Context) {
	id := c.Param("id")
	var trade TraderTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}
	trade.IsRead = !trade.IsRead
	db.Save(&trade)
	c.JSON(http.StatusOK, gin.H{"trade": trade})
}

func markAllTraderTradesRead(c *gin.Context) {
	db.Model(&TraderTrade{}).Where("is_read = ? AND is_pending = ?", false, false).Update("is_read", true)
	c.JSON(http.StatusOK, gin.H{"message": "All trades marked as read"})
}

func markAllTraderTradesUnread(c *gin.Context) {
	db.Model(&TraderTrade{}).Where("is_read = ? AND is_pending = ?", true, false).Update("is_read", false)
	c.JSON(http.StatusOK, gin.H{"message": "All trades marked as unread"})
}

func getTraderUnreadCount(c *gin.Context) {
	var count int64
	db.Model(&TraderTrade{}).Where("is_read = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false, false).Count(&count)

	// Also get the unread trades for notification details
	var unreadTrades []TraderTrade
	db.Where("is_read = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false, false).Order("executed_at desc").Limit(10).Find(&unreadTrades)

	c.JSON(http.StatusOK, gin.H{"count": count, "trades": unreadTrades})
}

func getTraderPending(c *gin.Context) {
	var positions []TraderPosition
	db.Where("is_pending = ?", true).Find(&positions)

	type PendingPosition struct {
		TraderPosition
		CurrentPrice   float64 `json:"current_price"`
		TotalReturnPct float64 `json:"total_return_pct"`
	}

	symbols := make([]string, 0, len(positions))
	for _, p := range positions {
		symbols = append(symbols, p.Symbol)
	}
	quotes := fetchQuotes(symbols)

	var result []PendingPosition
	for _, pos := range positions {
		pp := PendingPosition{TraderPosition: pos}
		if quote, ok := quotes[pos.Symbol]; ok {
			pp.CurrentPrice = quote.Price
			if pos.AvgPrice > 0 {
				pp.TotalReturnPct = ((quote.Price - pos.AvgPrice) / pos.AvgPrice) * 100
			}
		}
		result = append(result, pp)
	}

	c.JSON(http.StatusOK, result)
}

func getTraderLogs(c *gin.Context) {
	var logs []BotLog
	db.Where("bot = ?", "trader").Order("created_at desc").Limit(200).Find(&logs)
	c.JSON(http.StatusOK, logs)
}

func getTraderTodos(c *gin.Context) {
	var todos []BotTodo
	db.Where("bot = ? AND done = ?", "trader", false).Order("created_at desc").Find(&todos)
	c.JSON(http.StatusOK, todos)
}

func markTraderTodoDone(c *gin.Context) {
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

func reopenTraderTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	todo.Done = false
	todo.Decision = ""
	todo.DoneAt = nil
	db.Save(&todo)
	c.JSON(http.StatusOK, todo)
}

func deleteTraderTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	db.Delete(&todo)
	c.JSON(http.StatusOK, gin.H{"message": "Todo deleted"})
}

func executeTraderTodo(c *gin.Context) {
	id := c.Param("id")
	var todo BotTodo
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	var req struct {
		IsLive      bool     `json:"is_live"`
		Price       *float64 `json:"price"`
		InvestedEUR *float64 `json:"invested_eur"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	now := time.Now()
	price := todo.Price
	if req.Price != nil {
		price = *req.Price
	}

	if todo.Type == "BUY" {
		var existingPos TraderPosition
		if err := db.Where("symbol = ? AND is_closed = ?", todo.Symbol, false).First(&existingPos).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Position already exists"})
			return
		}

		// Calculate quantity based on invested EUR (default 100 EUR)
		investmentEUR := 100.0
		if req.InvestedEUR != nil && *req.InvestedEUR > 0 {
			investmentEUR = *req.InvestedEUR
		}
		investmentUSD := convertToUSD(investmentEUR, "EUR")
		qty := math.Round((investmentUSD/price)*1000000) / 1000000
		if qty <= 0 {
			qty = 1
		}

		newTrade := TraderTrade{
			Symbol:     todo.Symbol,
			Name:       todo.Name,
			Action:     "BUY",
			Quantity:   qty,
			Price:      price,
			SignalDate: todo.CreatedAt,
			ExecutedAt: now,
			IsLive:     req.IsLive,
			IsPending:  false,
		}
		db.Create(&newTrade)

		newPosition := TraderPosition{
			Symbol:      todo.Symbol,
			Name:        todo.Name,
			Quantity:    qty,
			AvgPrice:    price,
			IsLive:      req.IsLive,
			IsPending:   false,
			BuyDate:     now,
			InvestedEUR: investmentEUR,
		}
		db.Create(&newPosition)

		portfolioPos := PortfolioPosition{
			UserID:       TRADER_USER_ID,
			Symbol:       todo.Symbol,
			Name:         todo.Name,
			AvgPrice:     price,
			PurchaseDate: &now,
			Quantity:     &qty,
		}
		db.Create(&portfolioPos)

	} else if todo.Type == "SELL" {
		var position TraderPosition
		if err := db.Where("symbol = ? AND is_closed = ?", todo.Symbol, false).First(&position).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Position not found"})
			return
		}

		pnl := price - position.AvgPrice
		pnlPct := (pnl / position.AvgPrice) * 100

		newTrade := TraderTrade{
			Symbol:        todo.Symbol,
			Name:          todo.Name,
			Action:        "SELL",
			Price:         price,
			SignalDate:    todo.CreatedAt,
			ExecutedAt:    now,
			IsLive:        position.IsLive,
			IsPending:     false,
			ProfitLoss:    &pnl,
			ProfitLossPct: &pnlPct,
		}
		db.Create(&newTrade)

		// Close position instead of deleting
		position.IsClosed = true
		position.SellPrice = price
		position.SellDate = &now
		position.ProfitLoss = &pnl
		position.ProfitLossPct = &pnlPct
		position.UpdatedAt = time.Now()
		db.Save(&position)
		db.Where("user_id = ? AND symbol = ?", TRADER_USER_ID, todo.Symbol).Delete(&PortfolioPosition{})
	}

	todo.Done = true
	todo.Decision = "executed"
	todo.DoneAt = &now
	db.Save(&todo)

	c.JSON(http.StatusOK, gin.H{"message": "Todo executed", "todo": todo})
}

func syncTrader(c *gin.Context) {
	var positions []TraderPosition
	db.Where("is_closed = ?", false).Find(&positions)

	for _, pos := range positions {
		var existingPosition TraderPosition
		if err := db.Where("symbol = ?", pos.Symbol).First(&existingPosition).Error; err != nil {
			continue
		}

		if pos.AvgPrice > 0 {
			var existingBuy TraderTrade
			if err := db.Where("symbol = ? AND action = ?", pos.Symbol, "BUY").Order("executed_at desc").First(&existingBuy).Error; err == nil {
				var lastSell TraderTrade
				if err := db.Where("symbol = ? AND action = ? AND executed_at > ?", pos.Symbol, "SELL", existingBuy.ExecutedAt).First(&lastSell).Error; err != nil {
					if existingBuy.Price != pos.AvgPrice {
						existingBuy.Price = pos.AvgPrice
						db.Save(&existingBuy)
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Trader sync complete"})
}

func getTraderHistory(c *gin.Context) {
	period := c.DefaultQuery("period", "1m")
	live := c.DefaultQuery("live", "true")

	botType := "trader-live"
	if live == "false" {
		botType = "trader-sim"
	}

	history := calculateBotHistory(botType, period)
	c.JSON(http.StatusOK, history)
}

func getTraderPendingTrades(c *gin.Context) {
	var trades []TraderTrade
	db.Where("is_pending = ?", true).Order("executed_at desc").Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func acceptTraderTrade(c *gin.Context) {
	id := c.Param("id")
	var trade TraderTrade
	if err := db.First(&trade, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trade not found"})
		return
	}

	trade.IsPending = false
	db.Save(&trade)

	if trade.Action == "BUY" {
		db.Model(&TraderPosition{}).Where("symbol = ? AND is_pending = ?", trade.Symbol, true).Update("is_pending", false)
	}

	c.JSON(http.StatusOK, trade)
}

// getTraderSimulatedPortfolio returns simulated/test positions (is_live = false) for Admin view
func getTraderSimulatedPortfolio(c *gin.Context) {
	// Show ALL open positions (both live and simulated) - live ones are marked with is_live badge
	var positions []TraderPosition
	db.Where("is_pending = ? AND is_closed = ?", false, false).Order("buy_date desc").Find(&positions)

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

	// Realisierte Gewinne aus geschlossenen Trades einrechnen
	var closedSellTrades []TraderTrade
	db.Where("action = ? AND is_pending = ? AND is_deleted = ?", "SELL", false, false).Find(&closedSellTrades)

	realizedPL := 0.0
	totalClosedInvested := 0.0
	for _, trade := range closedSellTrades {
		if trade.ProfitLoss != nil {
			realizedPL += *trade.ProfitLoss
			totalClosedInvested += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}

	overallReturn := totalReturn + realizedPL
	overallInvested := totalInvested + totalClosedInvested
	overallReturnPct := 0.0
	if overallInvested > 0 {
		overallReturnPct = (overallReturn / overallInvested) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"positions":          result,
		"total_value":        totalValue,
		"total_invested":     totalInvested,
		"total_return":       totalReturn,
		"total_return_pct":   overallReturnPct,
		"realized_pl":        realizedPL,
		"overall_return":     overallReturn,
		"overall_invested":   overallInvested,
	})
}

// getTraderSimulatedPerformance returns performance stats for simulated/test trades (is_live = false) for Admin view
func getTraderSimulatedPerformance(c *gin.Context) {
	var sellTrades []TraderTrade
	db.Where("action = ? AND is_pending = ? AND is_live = ? AND is_deleted = ? AND is_admin_closed = ? AND is_filter_blocked = ?", "SELL", false, false, false, false, false).Find(&sellTrades)

	var buyTrades []TraderTrade
	db.Where("action = ? AND is_pending = ? AND is_live = ? AND is_deleted = ? AND is_admin_closed = ? AND is_filter_blocked = ?", "BUY", false, false, false, false, false).Find(&buyTrades)

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

	totalReturnPctClosed := 0.0
	for _, trade := range sellTrades {
		if trade.ProfitLossPct != nil {
			totalReturnPctClosed += *trade.ProfitLossPct
		}
	}
	avgReturnPerTrade := 0.0
	if len(sellTrades) > 0 {
		avgReturnPerTrade = totalReturnPctClosed / float64(len(sellTrades))
	}

	// Get open positions for unrealized P/L (simulated trades)
	var positions []TraderPosition
	db.Where("is_pending = ? AND is_live = ? AND is_closed = ?", false, false, false).Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	unrealizedGain := 0.0
	investedInPositions := 0.0
	currentValue := 0.0

	for _, pos := range positions {
		quote := quotes[pos.Symbol]
		currentPrice := quote.Price
		if currentPrice <= 0 {
			currentPrice = pos.AvgPrice
		}
		investedInPositions += pos.AvgPrice * pos.Quantity
		currentValue += currentPrice * pos.Quantity
		unrealizedGain += (currentPrice - pos.AvgPrice) * pos.Quantity
	}

	totalReturnPct := 0.0
	if investedInPositions > 0 {
		totalReturnPct = (unrealizedGain / investedInPositions) * 100
	}

	totalGain := totalProfitLoss + unrealizedGain
	totalInvestedAll := investedInPositions
	for _, trade := range sellTrades {
		if trade.ProfitLoss != nil {
			totalInvestedAll += (trade.Price * trade.Quantity) - *trade.ProfitLoss
		}
	}
	overallReturnPct := 0.0
	if totalInvestedAll > 0 {
		overallReturnPct = (totalGain / totalInvestedAll) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"total_trades":          len(buyTrades) + len(sellTrades),
		"total_buys":            len(buyTrades),
		"open_positions":        len(positions),
		"closed_trades":         len(sellTrades),
		"wins":                  wins,
		"losses":                losses,
		"win_rate":              winRate,
		"realized_profit":       totalProfitLoss,
		"avg_return_per_trade":  avgReturnPerTrade,
		"unrealized_gain":       unrealizedGain,
		"invested_in_positions": investedInPositions,
		"current_value":         currentValue,
		"total_gain":            totalGain,
		"total_return_pct":      totalReturnPct,
		"overall_return_pct":    overallReturnPct,
	})
}


// calculateBXtrenderDitzServer calculates BXtrender for Ditz mode
// BUY when line turns green (both indicators positive), SELL when line turns red (both negative)
func calculateBXtrenderDitzServer(ohlcv []OHLCV, config BXtrenderDitzConfig, nextBarOpen float64, nextBarTime int64) BXtrenderResult {
	shortL1 := config.ShortL1
	shortL2 := config.ShortL2
	shortL3 := config.ShortL3
	longL1 := config.LongL1
	longL2 := config.LongL2
	maLength := config.MaLength
	maFilterOn := config.MaFilterOn
	tslPercent := config.TslPercent

	if shortL1 == 0 {
		shortL1 = 5
	}
	if shortL2 == 0 {
		shortL2 = 20
	}
	if shortL3 == 0 {
		shortL3 = 15
	}
	if longL1 == 0 {
		longL1 = 20
	}
	if longL2 == 0 {
		longL2 = 15
	}
	if maLength == 0 {
		maLength = 200
	}
	if tslPercent == 0 {
		tslPercent = 20.0
	}

	minLen := shortL2
	if longL1 > minLen {
		minLen = longL1
	}
	if maLength > minLen {
		minLen = maLength
	}
	minLen += shortL3 + 10

	if len(ohlcv) < minLen {
		return BXtrenderResult{Signal: "NO_DATA", Bars: 0}
	}

	// Extract close prices
	closes := make([]float64, len(ohlcv))
	for i, bar := range ohlcv {
		closes[i] = bar.Close
	}

	// Calculate EMAs
	ema1 := calculateEMAServer(closes, shortL1)
	ema2 := calculateEMAServer(closes, shortL2)
	emaLong := calculateEMAServer(closes, longL1)

	// Calculate MA filter
	var maFilter []float64
	if config.MaType == "SMA" {
		maFilter = calculateSMAServer(closes, maLength)
	} else {
		maFilter = calculateEMAServer(closes, maLength)
	}

	// Calculate difference for short term
	diff := make([]float64, len(closes))
	for i := range diff {
		diff[i] = ema1[i] - ema2[i]
	}

	// Calculate RSI of difference (short term xtrender)
	shortXtrender := calculateRSIServer(diff, shortL3)
	for i := range shortXtrender {
		shortXtrender[i] -= 50
	}

	// Calculate RSI of long EMA (long term xtrender)
	longXtrender := calculateRSIServer(emaLong, longL2)
	for i := range longXtrender {
		longXtrender[i] -= 50
	}

	// Generate Ditz trades - buy when green, sell when red
	trades := []ServerTrade{}
	inPosition := false
	var lastBuyPrice, highestPrice float64
	lastBuySignalIdx := -1
	lastSellSignalIdx := -1

	for i := 1; i < len(ohlcv); i++ {
		shortPrev := shortXtrender[i-1]
		shortCurr := shortXtrender[i]
		longPrev := longXtrender[i-1]
		longCurr := longXtrender[i]
		price := ohlcv[i].Close

		// Update highest price if in position
		if inPosition && price > highestPrice {
			highestPrice = price
		}

		// Check trailing stop loss
		tslTriggered := false
		if inPosition && highestPrice > 0 {
			stopPrice := highestPrice * (1 - tslPercent/100)
			if price <= stopPrice {
				tslTriggered = true
			}
		}

		// MA filter condition
		maCondition := !maFilterOn || price > maFilter[i]

		// Both indicators alignment
		bothPositiveNow := shortCurr > 0 && longCurr > 0
		bothPositivePrev := shortPrev > 0 && longPrev > 0
		bothNegativeNow := shortCurr < 0 && longCurr < 0

		// Buy signal: both turn positive (line turns green) AND MA filter
		// Also allow re-entry when both are still positive but we got stopped out by TSL
		buySignal := bothPositiveNow && (!bothPositivePrev || !inPosition) && maCondition

		// Sell signal: both turn negative (line turns red) OR TSL triggered
		sellSignal := bothNegativeNow || tslTriggered

		if buySignal && !inPosition {
			var execPrice float64
			var execTime int64
			if i+1 < len(ohlcv) && ohlcv[i+1].Open > 0 {
				execPrice = ohlcv[i+1].Open
				execTime = ohlcv[i+1].Time
			} else if i == len(ohlcv)-1 && nextBarOpen > 0 {
				execPrice = nextBarOpen
				execTime = nextBarTime
			}
			if execPrice > 0 {
				trades = append(trades, ServerTrade{
					Type:  "BUY",
					Time:  execTime,
					Price: execPrice,
				})
				lastBuyPrice = execPrice
				highestPrice = execPrice
				inPosition = true
				lastBuySignalIdx = i
			}
		} else if sellSignal && inPosition {
			var execPrice float64
			var execTime int64
			if i+1 < len(ohlcv) && ohlcv[i+1].Open > 0 {
				execPrice = ohlcv[i+1].Open
				execTime = ohlcv[i+1].Time
			} else if i == len(ohlcv)-1 && nextBarOpen > 0 {
				execPrice = nextBarOpen
				execTime = nextBarTime
			}
			if execPrice > 0 {
				returnPct := ((execPrice - lastBuyPrice) / lastBuyPrice) * 100
				trades = append(trades, ServerTrade{
					Type:      "SELL",
					Time:      execTime,
					Price:     execPrice,
					PrevPrice: lastBuyPrice,
					Return:    returnPct,
				})
				inPosition = false
				lastBuyPrice = 0
				highestPrice = 0
				lastSellSignalIdx = i
			}
		}
	}

	// Signal basiert auf dem SIGNAL-Bar-Index (nicht dem Trade-Ausfuehrungsbar)
	signal := "WAIT"
	bars := 0
	lastIdx := len(ohlcv) - 1

	if inPosition {
		if lastBuySignalIdx == lastIdx {
			signal = "BUY"
		} else {
			signal = "HOLD"
		}
		if lastBuySignalIdx >= 0 {
			bars = lastIdx - lastBuySignalIdx
		}
	} else {
		if lastSellSignalIdx == lastIdx && len(trades) > 0 {
			signal = "SELL"
		} else {
			signal = "WAIT"
		}
		if lastSellSignalIdx >= 0 {
			bars = lastIdx - lastSellSignalIdx
		}
	}

	return BXtrenderResult{
		Short:  shortXtrender,
		Long:   longXtrender,
		Signal: signal,
		Bars:   bars,
		Trades: trades,
	}
}

func saveDitzPerformanceServer(symbol, name string, metrics MetricsResult, result BXtrenderResult, currentPrice float64, marketCap int64) {
	if result.Signal == "NO_DATA" {
		return
	}
	tradeData := convertServerTradesToTradeData(result.Trades, currentPrice)
	tradesJSON, _ := json.Marshal(tradeData)
	newSignalSince := calcSignalSince(result)

	var existing DitzStockPerformance
	if err := db.Where("symbol = ?", symbol).First(&existing).Error; err != nil {
		existing = DitzStockPerformance{
			Symbol:       symbol,
			Name:         name,
			WinRate:      metrics.WinRate,
			RiskReward:   metrics.RiskReward,
			TotalReturn:  metrics.TotalReturn,
			AvgReturn:    metrics.AvgReturn,
			TotalTrades:  metrics.TotalTrades,
			Wins:         metrics.Wins,
			Losses:       metrics.Losses,
			Signal:       result.Signal,
			SignalBars:   result.Bars,
			SignalSince:  newSignalSince,
			TradesJSON:   string(tradesJSON),
			CurrentPrice: currentPrice,
			MarketCap:    marketCap,
			UpdatedAt:    time.Now(),
			CreatedAt:    time.Now(),
		}
		db.Create(&existing)
	} else {
		ss, ps, pss := updateSignalHistory(existing.Signal, existing.SignalSince, result.Signal, newSignalSince)
		existing.Name = name
		existing.WinRate = metrics.WinRate
		existing.RiskReward = metrics.RiskReward
		existing.TotalReturn = metrics.TotalReturn
		existing.AvgReturn = metrics.AvgReturn
		existing.TotalTrades = metrics.TotalTrades
		existing.Wins = metrics.Wins
		existing.Losses = metrics.Losses
		existing.Signal = result.Signal
		existing.SignalBars = result.Bars
		existing.SignalSince = ss
		if ps != "" {
			existing.PrevSignal = ps
			existing.PrevSignalSince = pss
		}
		existing.TradesJSON = string(tradesJSON)
		existing.CurrentPrice = currentPrice
		if marketCap > 0 {
			existing.MarketCap = marketCap
		}
		existing.UpdatedAt = time.Now()
		db.Save(&existing)
	}
}

// calculateBXtrenderTraderServer calculates BXtrender for Trader mode (like Ditz but MA filter always off)
func calculateBXtrenderTraderServer(ohlcv []OHLCV, config BXtrenderTraderConfig, nextBarOpen float64, nextBarTime int64) BXtrenderResult {
	shortL1 := config.ShortL1
	shortL2 := config.ShortL2
	shortL3 := config.ShortL3
	longL1 := config.LongL1
	longL2 := config.LongL2
	tslPercent := config.TslPercent

	if shortL1 == 0 {
		shortL1 = 5
	}
	if shortL2 == 0 {
		shortL2 = 20
	}
	if shortL3 == 0 {
		shortL3 = 15
	}
	if longL1 == 0 {
		longL1 = 20
	}
	if longL2 == 0 {
		longL2 = 15
	}
	if tslPercent == 0 {
		tslPercent = 20.0
	}

	minLen := shortL2
	if longL1 > minLen {
		minLen = longL1
	}
	minLen += shortL3 + 10

	if len(ohlcv) < minLen {
		return BXtrenderResult{Signal: "NO_DATA", Bars: 0}
	}

	// Extract close prices
	closes := make([]float64, len(ohlcv))
	for i, bar := range ohlcv {
		closes[i] = bar.Close
	}

	// Calculate EMAs
	ema1 := calculateEMAServer(closes, shortL1)
	ema2 := calculateEMAServer(closes, shortL2)
	emaLong := calculateEMAServer(closes, longL1)

	// Calculate difference for short term
	diff := make([]float64, len(closes))
	for i := range diff {
		diff[i] = ema1[i] - ema2[i]
	}

	// Calculate RSI of difference (short term xtrender)
	shortXtrender := calculateRSIServer(diff, shortL3)
	for i := range shortXtrender {
		shortXtrender[i] -= 50
	}

	// Calculate RSI of long EMA (long term xtrender)
	longXtrender := calculateRSIServer(emaLong, longL2)
	for i := range longXtrender {
		longXtrender[i] -= 50
	}

	// Calculate T3 signal line from short xtrender
	signalLine := calculateT3Server(shortXtrender, 5)

	// Generate Trader trades - based on T3 signal line direction changes
	trades := []ServerTrade{}
	inPosition := false
	var lastBuyPrice, highestPrice float64
	lastBuySignalIdx := -1
	lastSellSignalIdx := -1

	for i := 2; i < len(ohlcv); i++ {
		price := ohlcv[i].Close

		// Update highest price if in position
		if inPosition && price > highestPrice {
			highestPrice = price
		}

		// Check trailing stop loss
		tslTriggered := false
		if inPosition && highestPrice > 0 {
			stopPrice := highestPrice * (1 - tslPercent/100)
			if price <= stopPrice {
				tslTriggered = true
			}
		}

		// Signal line direction
		signalRising := signalLine[i] > signalLine[i-1]
		signalRisingPrev := signalLine[i-1] > signalLine[i-2]

		// Buy signal: signal line turns from falling to rising (Red→Green)
		buySignal := signalRising && !signalRisingPrev

		// Sell signal: signal line turns from rising to falling (Green→Red) OR TSL
		sellSignal := (!signalRising && signalRisingPrev) || tslTriggered

		if buySignal && !inPosition {
			var execPrice float64
			var execTime int64
			if i+1 < len(ohlcv) && ohlcv[i+1].Open > 0 {
				execPrice = ohlcv[i+1].Open
				execTime = ohlcv[i+1].Time
			} else if i == len(ohlcv)-1 && nextBarOpen > 0 {
				execPrice = nextBarOpen
				execTime = nextBarTime
			}
			if execPrice > 0 {
				trades = append(trades, ServerTrade{
					Type:  "BUY",
					Time:  execTime,
					Price: execPrice,
				})
				lastBuyPrice = execPrice
				highestPrice = execPrice
				inPosition = true
				lastBuySignalIdx = i
			}
		} else if sellSignal && inPosition {
			var execPrice float64
			var execTime int64
			if i+1 < len(ohlcv) && ohlcv[i+1].Open > 0 {
				execPrice = ohlcv[i+1].Open
				execTime = ohlcv[i+1].Time
			} else if i == len(ohlcv)-1 && nextBarOpen > 0 {
				execPrice = nextBarOpen
				execTime = nextBarTime
			}
			if execPrice > 0 {
				returnPct := ((execPrice - lastBuyPrice) / lastBuyPrice) * 100
				trades = append(trades, ServerTrade{
					Type:      "SELL",
					Time:      execTime,
					Price:     execPrice,
					PrevPrice: lastBuyPrice,
					Return:    returnPct,
				})
				inPosition = false
				lastBuyPrice = 0
				highestPrice = 0
				lastSellSignalIdx = i
			}
		}
	}

	// Signal basiert auf dem SIGNAL-Bar-Index (nicht dem Trade-Ausfuehrungsbar)
	signal := "WAIT"
	bars := 0
	lastIdx := len(ohlcv) - 1

	if inPosition {
		if lastBuySignalIdx == lastIdx {
			signal = "BUY"
		} else {
			signal = "HOLD"
		}
		if lastBuySignalIdx >= 0 {
			bars = lastIdx - lastBuySignalIdx
		}
	} else {
		if lastSellSignalIdx == lastIdx && len(trades) > 0 {
			signal = "SELL"
		} else {
			signal = "WAIT"
		}
		if lastSellSignalIdx >= 0 {
			bars = lastIdx - lastSellSignalIdx
		}
	}

	return BXtrenderResult{
		Short:  shortXtrender,
		Long:   longXtrender,
		Signal: signal,
		Bars:   bars,
		Trades: trades,
	}
}

// saveTraderPerformanceServer saves performance data for trader mode (server-side batch)
func saveTraderPerformanceServer(symbol, name string, metrics MetricsResult, result BXtrenderResult, currentPrice float64, marketCap int64) {
	if result.Signal == "NO_DATA" {
		return
	}
	tradeData := convertServerTradesToTradeData(result.Trades, currentPrice)
	tradesJSON, _ := json.Marshal(tradeData)
	newSignalSince := calcSignalSince(result)

	var existing TraderStockPerformance
	if err := db.Where("symbol = ?", symbol).First(&existing).Error; err != nil {
		existing = TraderStockPerformance{
			Symbol:       symbol,
			Name:         name,
			WinRate:      metrics.WinRate,
			RiskReward:   metrics.RiskReward,
			TotalReturn:  metrics.TotalReturn,
			AvgReturn:    metrics.AvgReturn,
			TotalTrades:  metrics.TotalTrades,
			Wins:         metrics.Wins,
			Losses:       metrics.Losses,
			Signal:       result.Signal,
			SignalBars:   result.Bars,
			SignalSince:  newSignalSince,
			TradesJSON:   string(tradesJSON),
			CurrentPrice: currentPrice,
			MarketCap:    marketCap,
			UpdatedAt:    time.Now(),
			CreatedAt:    time.Now(),
		}
		db.Create(&existing)
	} else {
		ss, ps, pss := updateSignalHistory(existing.Signal, existing.SignalSince, result.Signal, newSignalSince)
		existing.Name = name
		existing.WinRate = metrics.WinRate
		existing.RiskReward = metrics.RiskReward
		existing.TotalReturn = metrics.TotalReturn
		existing.AvgReturn = metrics.AvgReturn
		existing.TotalTrades = metrics.TotalTrades
		existing.Wins = metrics.Wins
		existing.Losses = metrics.Losses
		existing.Signal = result.Signal
		existing.SignalBars = result.Bars
		existing.SignalSince = ss
		if ps != "" {
			existing.PrevSignal = ps
			existing.PrevSignalSince = pss
		}
		existing.TradesJSON = string(tradesJSON)
		existing.CurrentPrice = currentPrice
		if marketCap > 0 {
			existing.MarketCap = marketCap
		}
		existing.UpdatedAt = time.Now()
		db.Save(&existing)
	}
}
