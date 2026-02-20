package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"sort"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	Email            string    `json:"email" gorm:"uniqueIndex;not null"`
	Username         string    `json:"username" gorm:"uniqueIndex;not null"`
	Password         string    `json:"-" gorm:"not null"`
	IsAdmin          bool      `json:"is_admin" gorm:"default:false"`
	VisibleInRanking bool      `json:"visible_in_ranking" gorm:"default:true"`
	LoginCount       int       `json:"login_count" gorm:"default:0"`
	LastActive       time.Time `json:"last_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
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

// UserNotification stores signal change notifications per user
type UserNotification struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index;not null"`
	Symbol    string    `json:"symbol" gorm:"not null"`
	Name      string    `json:"name"`
	Mode      string    `json:"mode" gorm:"not null"`
	OldSignal string    `json:"old_signal" gorm:"not null"`
	NewSignal string    `json:"new_signal" gorm:"not null"`
	IsRead    bool      `json:"is_read" gorm:"default:false;index"`
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

type SignalListFilterConfig struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	MinWinrate   *float64  `json:"min_winrate"`
	MaxWinrate   *float64  `json:"max_winrate"`
	MinRR        *float64  `json:"min_rr"`
	MaxRR        *float64  `json:"max_rr"`
	MinAvgReturn *float64  `json:"min_avg_return"`
	MaxAvgReturn *float64  `json:"max_avg_return"`
	MinMarketCap *float64  `json:"min_market_cap"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SignalListVisibility struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	Symbol  string `gorm:"uniqueIndex:idx_signal_vis_sym_month;not null" json:"symbol"`
	Month   string `gorm:"uniqueIndex:idx_signal_vis_sym_month;not null" json:"month"` // "2026-02"
	Visible bool   `gorm:"default:true" json:"visible"`
}

type TradingWatchlistItem struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Symbol       string    `json:"symbol" gorm:"uniqueIndex;not null"`
	Name         string    `json:"name"`
	AddedBy      uint      `json:"added_by"`
	IsLive       bool      `json:"is_live" gorm:"default:false"`
	Fractionable bool      `json:"fractionable" gorm:"default:true"`
	CreatedAt    time.Time `json:"created_at"`
}

type GlobalSetting struct {
	Key   string `json:"key" gorm:"primaryKey"`
	Value string `json:"value"`
}

type TradingVirtualPosition struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	Symbol        string     `json:"symbol" gorm:"index"`
	Strategy      string     `json:"strategy"`
	Direction     string     `json:"direction"`
	EntryPrice    float64    `json:"entry_price"`
	EntryTime     time.Time  `json:"entry_time"`
	StopLoss      float64    `json:"stop_loss"`
	TakeProfit    float64    `json:"take_profit"`
	CurrentPrice  float64    `json:"current_price"`
	IsClosed      bool       `json:"is_closed" gorm:"default:false"`
	ClosePrice    float64    `json:"close_price"`
	CloseTime     *time.Time `json:"close_time"`
	CloseReason   string     `json:"close_reason"`
	ProfitLossPct float64    `json:"profit_loss_pct"`
	CreatedAt     time.Time  `json:"created_at"`
}

type ArenaBacktestHistory struct {
	ID          uint      `gorm:"primaryKey"`
	Symbol      string    `gorm:"index;not null"`
	Strategy    string    `gorm:"not null"`
	Interval    string    `gorm:"not null"`
	MetricsJSON string    `gorm:"type:text"`
	TradesJSON  string    `gorm:"type:text"`
	MarkersJSON string    `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ArenaStrategySettings struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Symbol     string    `json:"symbol" gorm:"uniqueIndex:idx_sym_strat;default:''"`
	Strategy   string    `json:"strategy" gorm:"uniqueIndex:idx_sym_strat;not null"`
	ParamsJSON string    `json:"params_json" gorm:"type:text"`
	Interval   string    `json:"interval"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ArenaV2BatchResult struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Symbol      string    `json:"symbol" gorm:"uniqueIndex:idx_v2_sym_strat_iv"`
	Strategy    string    `json:"strategy" gorm:"uniqueIndex:idx_v2_sym_strat_iv"`
	Interval    string    `json:"interval" gorm:"uniqueIndex:idx_v2_sym_strat_iv"`
	MetricsJSON string    `json:"metrics_json" gorm:"type:text"`
	TradesJSON  string    `json:"trades_json" gorm:"type:text"`
	MarketCap   int64     `json:"market_cap"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Weekly OHLCV cache table (legacy — migrated to OHLCVCache)
type WeeklyOHLCVCache struct {
	ID        uint      `gorm:"primaryKey"`
	Symbol    string    `gorm:"uniqueIndex;not null"`
	DataJSON  string    `gorm:"type:text"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// Universal OHLCV cache — replaces WeeklyOHLCVCache (interval-agnostic)
type OHLCVCache struct {
	ID        uint      `gorm:"primaryKey"`
	Symbol    string    `gorm:"uniqueIndex:idx_ohlcv_sym_iv;not null"`
	Interval  string    `gorm:"uniqueIndex:idx_ohlcv_sym_iv;not null"` // 5m, 15m, 60m, 1d, 1wk, 1mo
	DataJSON  string    `gorm:"type:text"`
	BarCount  int       `gorm:"default:0"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// Backtest Lab History
type BacktestLabHistory struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	UserID           uint      `json:"user_id" gorm:"index"`
	Name             string    `json:"name"`
	BaseMode         string    `json:"base_mode"`
	RulesJSON        string    `json:"-" gorm:"type:text"`
	TSL              float64   `json:"tsl"`
	TimeRange        string    `json:"time_range"`
	FiltersJSON      string    `json:"-" gorm:"type:text"`
	MetricsJSON      string    `json:"-" gorm:"type:text"`
	StockSummaryJSON string    `json:"-" gorm:"type:text"`
	TestedStocks     int       `json:"tested_stocks"`
	SkippedCount     int       `json:"skipped_count"`
	TotalStocks      int       `json:"total_stocks"`
	FilteredStocks   int       `json:"filtered_stocks"`
	CreatedAt        time.Time `json:"created_at"`
}

// Alpaca Account Management
type AlpacaAccount struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"not null"`
	ApiKey    string    `json:"-" gorm:"type:text"`
	SecretKey string    `json:"-" gorm:"type:text"`
	IsPaper   bool      `json:"is_paper" gorm:"default:true"`
	IsDefault bool      `json:"is_default" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Live Trading
type LiveTradingConfig struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	UserID        uint      `json:"user_id" gorm:"index;not null"`
	Strategy      string    `json:"strategy"`
	Interval      string    `json:"interval"`
	ParamsJSON    string    `json:"params_json" gorm:"type:text"`
	Symbols       string    `json:"symbols" gorm:"type:text"`
	LongOnly      bool      `json:"long_only" gorm:"default:true"`
	TradeAmount   float64   `json:"trade_amount" gorm:"default:500"`
	FiltersJSON   string    `json:"filters_json" gorm:"type:text"`
	FiltersActive   bool      `json:"filters_active"`
	Currency        string    `json:"currency" gorm:"default:'USD'"`
	AlpacaAccountID uint      `json:"alpaca_account_id" gorm:"default:0"`
	AlpacaApiKey    string    `json:"alpaca_api_key" gorm:"type:text"`
	AlpacaSecretKey string    `json:"alpaca_secret_key" gorm:"type:text"`
	AlpacaEnabled   bool      `json:"alpaca_enabled" gorm:"default:false"`
	AlpacaPaper     bool      `json:"alpaca_paper" gorm:"default:true"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type LiveTradingSession struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	UserID      uint       `json:"user_id" gorm:"index"`
	ConfigID    uint       `json:"config_id"`
	Name        string     `json:"name"`
	Strategy    string     `json:"strategy"`
	Interval    string     `json:"interval"`
	ParamsJSON  string     `json:"params_json" gorm:"type:text"`
	Symbols     string     `json:"symbols" gorm:"type:text"`
	LongOnly    bool       `json:"long_only"`
	TradeAmount float64    `json:"trade_amount"`
	Currency    string     `json:"currency" gorm:"default:'USD'"`
	IsActive    bool       `json:"is_active" gorm:"index"`
	StartedAt   time.Time  `json:"started_at"`
	StoppedAt   *time.Time `json:"stopped_at"`
	LastPollAt  *time.Time `json:"last_poll_at"`
	NextPollAt  *time.Time `json:"next_poll_at"`
	TotalPolls       int        `json:"total_polls" gorm:"default:0"`
	SymbolPricesJSON string     `json:"-" gorm:"type:text"`
	CreatedAt        time.Time  `json:"created_at"`
}

type LiveTradingPosition struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	SessionID      uint       `json:"session_id" gorm:"index"`
	StrategyID     uint       `json:"strategy_id" gorm:"index;default:0"`
	Symbol         string     `json:"symbol" gorm:"index"`
	Direction      string     `json:"direction"`
	EntryPrice     float64    `json:"entry_price"`
	EntryPriceUSD  float64    `json:"entry_price_usd"`
	EntryTime      time.Time  `json:"entry_time"`
	StopLoss       float64    `json:"stop_loss"`
	TakeProfit     float64    `json:"take_profit"`
	CurrentPrice   float64    `json:"current_price"`
	IsClosed       bool       `json:"is_closed" gorm:"default:false;index"`
	ClosePrice     float64    `json:"close_price"`
	ClosePriceUSD  float64    `json:"close_price_usd"`
	CloseTime      *time.Time `json:"close_time"`
	CloseReason    string     `json:"close_reason"`
	ProfitLossPct  float64    `json:"profit_loss_pct"`
	InvestedAmount float64    `json:"invested_amount"`
	ProfitLossAmt  float64    `json:"profit_loss_amt"`
	NativeCurrency string     `json:"native_currency"`
	Quantity       float64    `json:"quantity"`
	SignalIndex    int        `json:"signal_index"`
	AlpacaOrderID  string     `json:"alpaca_order_id"`
	CreatedAt      time.Time  `json:"created_at"`
}

type LiveTradingLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	SessionID uint      `json:"session_id" gorm:"index"`
	Level     string    `json:"level"`
	Symbol    string    `json:"symbol"`
	Message   string    `json:"message"`
	Strategy  string    `json:"strategy"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

type LiveSessionStrategy struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	SessionID  uint      `json:"session_id" gorm:"index"`
	Name       string    `json:"name"`
	ParamsJSON string    `json:"params_json" gorm:"type:text"`
	Symbols    string    `json:"symbols" gorm:"type:text"`
	IsEnabled  bool      `json:"is_enabled" gorm:"default:false"`
	LongOnly   bool      `json:"long_only" gorm:"default:true"`
	CreatedAt  time.Time `json:"created_at"`
}

type BacktestLabHistoryStockSummary struct {
	Symbol      string  `json:"symbol"`
	Name        string  `json:"name"`
	WinRate     float64 `json:"win_rate"`
	TotalReturn float64 `json:"total_return"`
	AvgReturn   float64 `json:"avg_return"`
	RiskReward  float64 `json:"risk_reward"`
	TotalTrades int     `json:"total_trades"`
}

// Backtest Lab types
type BacktestLabRule struct {
	Type             string `json:"type"`              // "entry" or "exit"
	MonthlyCondition string `json:"monthly_condition"` // "BUY","SELL","HOLD","WAIT","FIRST_LIGHT_RED","ANY"
	WeeklyCondition  string `json:"weekly_condition"`  // "BUY","SELL","HOLD","WAIT","BUY_TO_HOLD","ANY"
	Operator         string `json:"operator"`          // "AND", "OR"
}

type BacktestLabRequest struct {
	Symbol   string            `json:"symbol"`
	BaseMode string            `json:"base_mode"` // "defensive","aggressive","quant","ditz","trader"
	Rules    []BacktestLabRule `json:"rules"`
	TSL      float64           `json:"tsl"` // 0 = default 20%
}

type BacktestLabBatchRequest struct {
	BaseMode     string            `json:"base_mode"`
	Rules        []BacktestLabRule `json:"rules"`
	TSL          float64           `json:"tsl"`
	TimeRange    string            `json:"time_range"` // "1y","2y","3y","5y","10y","all"
	MinWinrate   *float64          `json:"min_winrate"`
	MaxWinrate   *float64          `json:"max_winrate"`
	MinRR        *float64          `json:"min_rr"`
	MaxRR        *float64          `json:"max_rr"`
	MinAvgReturn *float64          `json:"min_avg_return"`
	MaxAvgReturn *float64          `json:"max_avg_return"`
	MinMarketCap *float64          `json:"min_market_cap"` // in Mrd
}

type BacktestLabBatchStockResult struct {
	Symbol  string               `json:"symbol"`
	Name    string               `json:"name"`
	Metrics ArenaBacktestMetrics `json:"metrics"`
	Trades  []ArenaBacktestTrade `json:"trades"`
}

type BacktestLabBatchResponse struct {
	TotalMetrics   ArenaBacktestMetrics          `json:"total_metrics"`
	StockResults   []BacktestLabBatchStockResult  `json:"stock_results"`
	SkippedStocks  []BacktestLabSkippedStock      `json:"skipped_stocks"`
	TotalStocks    int                            `json:"total_stocks"`
	TestedStocks   int                            `json:"tested_stocks"`
	FilteredStocks int                            `json:"filtered_stocks"`
}

type BacktestLabSkippedStock struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type BacktestLabTimeValue struct {
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
}

type BacktestLabOHLCV struct {
	Time   int64   `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

type BacktestLabResponse struct {
	Metrics      ArenaBacktestMetrics   `json:"metrics"`
	Trades       []ArenaBacktestTrade   `json:"trades"`
	Markers      []ChartMarker          `json:"markers"`
	MonthlyBars  []BacktestLabOHLCV     `json:"monthly_bars"`
	MonthlyShort []BacktestLabTimeValue `json:"monthly_short"`
	MonthlyLong  []BacktestLabTimeValue `json:"monthly_long"`
	WeeklyBars   []BacktestLabOHLCV     `json:"weekly_bars"`
	WeeklyShort  []BacktestLabTimeValue `json:"weekly_short"`
	WeeklyLong   []BacktestLabTimeValue `json:"weekly_long"`
}

var db *gorm.DB
var latestPriceCache sync.Map // key: symbol (string), value: float64
var sessions = make(map[string]Session) // Legacy in-memory cache, DB is source of truth
var sessionsMu sync.RWMutex
var httpClient = &http.Client{Timeout: 10 * time.Second}
var twelveDataAPIKey string

// Alpaca Data API keys — loaded from default AlpacaAccount in DB
var (
	alpacaDataKey    string
	alpacaDataSecret string
	alpacaRateTicker *time.Ticker
)

// Alpaca tradable assets cache (for watchlist validation)
type AlpacaAssetInfo struct {
	Tradable     bool
	Fractionable bool
}

var (
	tradableAssetsCache   map[string]AlpacaAssetInfo
	tradableAssetsCacheMu sync.RWMutex
)

// File-based OHLCV Cache with lazy-loading LRU memory cache
// Files: data/ohlcv/SYMBOL_INTERVAL.json.gz
// Memory: only loaded on demand, evicted after 30 min of no access
var (
	ohlcvMemCache   map[string]map[string]*ohlcvCacheEntry
	ohlcvMemCacheMu sync.RWMutex
	ohlcvCacheDir   string // set in initDB
)

type ohlcvCacheEntry struct {
	Bars       []OHLCV
	LastAccess time.Time
}

// Live-Trading: serialized position writes to avoid SQLite lock contention
var livePositionWriteCh = make(chan func(), 256)

// Live-Trading: in-memory guard to prevent duplicate position opens (race condition with async writes)
// Key: "sessionID:strategyID:symbol" → true if position is open
var liveOpenPosGuard sync.Map

// Live-Trading: separate Alpaca rate limiter (doesn't interfere with prefetch)
var (
	liveAlpacaLimiterMu   sync.Mutex
	liveAlpacaLimiterLast time.Time
)

func liveAlpacaThrottle() {
	liveAlpacaLimiterMu.Lock()
	defer liveAlpacaLimiterMu.Unlock()
	elapsed := time.Since(liveAlpacaLimiterLast)
	if elapsed < 350*time.Millisecond {
		time.Sleep(350*time.Millisecond - elapsed)
	}
	liveAlpacaLimiterLast = time.Now()
}

func livePositionWriter() {
	for fn := range livePositionWriteCh {
		fn()
	}
}

// initOpenPosGuard loads all open positions for a session into the in-memory guard
func initOpenPosGuard(sessionID uint) {
	var openPos []LiveTradingPosition
	db.Where("session_id = ? AND is_closed = ?", sessionID, false).Find(&openPos)
	for _, p := range openPos {
		key := fmt.Sprintf("%d:%d:%s", p.SessionID, p.StrategyID, p.Symbol)
		liveOpenPosGuard.Store(key, true)
	}
	if len(openPos) > 0 {
		log.Printf("[LiveTrading] OpenPosGuard: %d offene Positionen für Session #%d geladen", len(openPos), sessionID)
	}
}

// openPosGuardKey generates the key for the in-memory guard
func openPosGuardKey(sessionID, strategyID uint, symbol string) string {
	return fmt.Sprintf("%d:%d:%s", sessionID, strategyID, symbol)
}

// initOHLCVFileCache sets up the file cache directory and migrates DB→files if needed
func initOHLCVFileCache() {
	ohlcvMemCacheMu.Lock()
	ohlcvMemCache = make(map[string]map[string]*ohlcvCacheEntry)
	ohlcvMemCacheMu.Unlock()

	// Ensure cache dir exists
	os.MkdirAll(ohlcvCacheDir, 0755)

	// One-time migration: if DB has entries but files don't exist yet, export to files
	var dbCount int64
	db.Model(&OHLCVCache{}).Count(&dbCount)
	files, _ := filepath.Glob(filepath.Join(ohlcvCacheDir, "*.json.gz"))
	if dbCount > 0 && len(files) == 0 {
		log.Printf("[OHLCVCache] Migrating %d DB entries to gzip files...", dbCount)
		var entries []OHLCVCache
		db.Find(&entries)
		migrated := 0
		for _, e := range entries {
			var bars []OHLCV
			if json.Unmarshal([]byte(e.DataJSON), &bars) == nil && len(bars) > 0 {
				writeOHLCVFile(e.Symbol, e.Interval, bars)
				migrated++
			}
		}
		log.Printf("[OHLCVCache] Migrated %d entries to files", migrated)
		// Clear DB cache to free space (files are the source of truth now)
		db.Exec("DELETE FROM ohlcv_caches")
		log.Println("[OHLCVCache] Cleared ohlcv_caches table")
	} else if dbCount > 0 && len(files) > 0 {
		// Files exist, clear remaining DB entries
		db.Exec("DELETE FROM ohlcv_caches")
	}

	fileCount := 0
	files, _ = filepath.Glob(filepath.Join(ohlcvCacheDir, "*.json.gz"))
	fileCount = len(files)
	log.Printf("[OHLCVCache] File cache ready: %d files in %s (lazy-load, nothing in RAM yet)", fileCount, ohlcvCacheDir)

	// Start LRU evictor
	go ohlcvLRUEvictor()
}

// ohlcvLRUEvictor removes entries not accessed for 30 minutes
func ohlcvLRUEvictor() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ohlcvMemCacheMu.Lock()
		evicted := 0
		cutoff := time.Now().Add(-30 * time.Minute)
		for sym, intervals := range ohlcvMemCache {
			for iv, entry := range intervals {
				if entry.LastAccess.Before(cutoff) {
					delete(intervals, iv)
					evicted++
				}
			}
			if len(intervals) == 0 {
				delete(ohlcvMemCache, sym)
			}
		}
		ohlcvMemCacheMu.Unlock()
		if evicted > 0 {
			log.Printf("[OHLCVCache] LRU evicted %d entries", evicted)
		}
	}
}

// ohlcvFilePath returns the gzip file path for a symbol+interval
func ohlcvFilePath(symbol, interval string) string {
	// Sanitize symbol for filesystem (e.g., BRK.B → BRK_B)
	safe := strings.ReplaceAll(symbol, "/", "_")
	safe = strings.ReplaceAll(safe, "\\", "_")
	return filepath.Join(ohlcvCacheDir, safe+"_"+interval+".json.gz")
}

// writeOHLCVFile saves OHLCV data as a gzip JSON file
func writeOHLCVFile(symbol, interval string, bars []OHLCV) error {
	data, err := json.Marshal(bars)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(data)
	gz.Close()
	return os.WriteFile(ohlcvFilePath(symbol, interval), buf.Bytes(), 0644)
}

// readOHLCVFile loads OHLCV data from a gzip JSON file
func readOHLCVFile(symbol, interval string) ([]OHLCV, time.Time, error) {
	path := ohlcvFilePath(symbol, interval)
	fi, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, time.Time{}, err
	}
	defer gz.Close()
	data, err := io.ReadAll(gz)
	if err != nil {
		return nil, time.Time{}, err
	}
	var bars []OHLCV
	if err := json.Unmarshal(data, &bars); err != nil {
		return nil, time.Time{}, err
	}
	return bars, fi.ModTime(), nil
}

// getOHLCVFromMemCache reads from memory, lazy-loads from file on miss
func getOHLCVFromMemCache(symbol, interval string) ([]OHLCV, bool) {
	// Fast path: check memory
	ohlcvMemCacheMu.RLock()
	if m, ok := ohlcvMemCache[symbol]; ok {
		if entry, ok := m[interval]; ok && len(entry.Bars) > 0 {
			entry.LastAccess = time.Now()
			ohlcvMemCacheMu.RUnlock()
			return entry.Bars, true
		}
	}
	ohlcvMemCacheMu.RUnlock()

	// Slow path: load from file
	bars, _, err := readOHLCVFile(symbol, interval)
	if err != nil || len(bars) == 0 {
		return nil, false
	}

	// Store in memory cache
	ohlcvMemCacheMu.Lock()
	if ohlcvMemCache[symbol] == nil {
		ohlcvMemCache[symbol] = make(map[string]*ohlcvCacheEntry)
	}
	ohlcvMemCache[symbol][interval] = &ohlcvCacheEntry{Bars: bars, LastAccess: time.Now()}
	ohlcvMemCacheMu.Unlock()
	return bars, true
}

// setOHLCVInMemCache writes to memory cache + gzip file
func setOHLCVInMemCache(symbol, interval string, bars []OHLCV) {
	ohlcvMemCacheMu.Lock()
	if ohlcvMemCache == nil {
		ohlcvMemCache = make(map[string]map[string]*ohlcvCacheEntry)
	}
	if ohlcvMemCache[symbol] == nil {
		ohlcvMemCache[symbol] = make(map[string]*ohlcvCacheEntry)
	}
	ohlcvMemCache[symbol][interval] = &ohlcvCacheEntry{Bars: bars, LastAccess: time.Now()}
	ohlcvMemCacheMu.Unlock()

	// Persist to file (async, don't block caller)
	go writeOHLCVFile(symbol, interval, bars)
}

// getOHLCVMemCacheSymbols returns all symbols that have cached data for a given interval
// Checks both memory and files on disk
func getOHLCVMemCacheSymbols(interval string) map[string]bool {
	result := make(map[string]bool)
	// Memory
	ohlcvMemCacheMu.RLock()
	for sym, intervals := range ohlcvMemCache {
		if _, ok := intervals[interval]; ok {
			result[sym] = true
		}
	}
	ohlcvMemCacheMu.RUnlock()
	// Files on disk
	pattern := filepath.Join(ohlcvCacheDir, "*_"+interval+".json.gz")
	files, _ := filepath.Glob(pattern)
	for _, f := range files {
		base := filepath.Base(f)
		sym := strings.TrimSuffix(base, "_"+interval+".json.gz")
		sym = strings.ReplaceAll(sym, "_", ".") // BRK_B → BRK.B (reverse sanitize — only for dots)
		result[sym] = true
	}
	return result
}

func getGlobalSetting(key string) string {
	var s GlobalSetting
	if db.Where("`key` = ?", key).First(&s).Error == nil {
		return s.Value
	}
	return ""
}

func setGlobalSetting(key, value string) {
	var s GlobalSetting
	if db.Where("`key` = ?", key).First(&s).Error == nil {
		db.Model(&s).Update("value", value)
	} else {
		db.Create(&GlobalSetting{Key: key, Value: value})
	}
}

func alpacaFetchTradableAssets() (map[string]AlpacaAssetInfo, error) {
	if alpacaDataKey == "" || alpacaDataSecret == "" {
		return nil, fmt.Errorf("alpaca keys not configured")
	}

	req, err := http.NewRequest("GET", "https://paper-api.alpaca.markets/v2/assets?status=active&asset_class=us_equity", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", alpacaDataKey)
	req.Header.Set("APCA-API-SECRET-KEY", alpacaDataSecret)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca assets request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("alpaca assets error %d: %s", resp.StatusCode, string(body))
	}

	var assets []struct {
		Symbol       string `json:"symbol"`
		Tradable     bool   `json:"tradable"`
		Fractionable bool   `json:"fractionable"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil {
		return nil, fmt.Errorf("alpaca assets decode error: %v", err)
	}

	result := make(map[string]AlpacaAssetInfo, len(assets))
	for _, a := range assets {
		if a.Tradable {
			result[a.Symbol] = AlpacaAssetInfo{Tradable: true, Fractionable: a.Fractionable}
		}
	}
	log.Printf("[Alpaca] Loaded %d tradable assets from Alpaca", len(result))
	return result, nil
}

func refreshTradableAssetsCache() error {
	assets, err := alpacaFetchTradableAssets()
	if err != nil {
		return err
	}
	tradableAssetsCacheMu.Lock()
	tradableAssetsCache = assets
	tradableAssetsCacheMu.Unlock()
	return nil
}

func isAlpacaTradable(symbol string) (AlpacaAssetInfo, bool) {
	tradableAssetsCacheMu.RLock()
	defer tradableAssetsCacheMu.RUnlock()
	info, ok := tradableAssetsCache[symbol]
	return info, ok
}

// TwelveData daily call counter (800/day limit)
var (
	twelveDataCallsToday  int64
	twelveDataCounterDate string
	twelveDataCounterMu   sync.Mutex
)

func checkTwelveDataBudget() bool {
	twelveDataCounterMu.Lock()
	defer twelveDataCounterMu.Unlock()
	today := time.Now().Format("2006-01-02")
	if twelveDataCounterDate != today {
		twelveDataCallsToday = 0
		twelveDataCounterDate = today
	}
	if twelveDataCallsToday >= 750 {
		return false
	}
	twelveDataCallsToday++
	return true
}

// ==================== Alpaca Broker Integration ====================

func alpacaBaseURL(paper bool) string {
	if paper {
		return "https://paper-api.alpaca.markets"
	}
	return "https://api.alpaca.markets"
}

func alpacaRequest(method, path string, body interface{}, config LiveTradingConfig) (map[string]interface{}, error) {
	baseURL := alpacaBaseURL(config.AlpacaPaper)
	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("alpaca marshal error: %v", err)
		}
		reqBody = bytes.NewReader(jsonBytes)
	}
	req, err := http.NewRequest(method, baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("alpaca request error: %v", err)
	}
	req.Header.Set("APCA-API-KEY-ID", config.AlpacaApiKey)
	req.Header.Set("APCA-API-SECRET-KEY", config.AlpacaSecretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("alpaca read error: %v", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("alpaca error %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("alpaca unmarshal error: %v", err)
	}
	return result, nil
}

func alpacaGetAccount(config LiveTradingConfig) (map[string]interface{}, error) {
	return alpacaRequest("GET", "/v2/account", nil, config)
}

type AlpacaOrderResult struct {
	OrderID        string
	FilledAvgPrice float64
	Status         string
	OrderClass     string
	Legs           []AlpacaOrderLeg
}

type AlpacaOrderLeg struct {
	ID     string
	Type   string // "stop" or "limit"
	Status string
}

func alpacaPlaceOrder(symbol string, qty float64, side string, config LiveTradingConfig, opts ...map[string]float64) (*AlpacaOrderResult, error) {
	liveAlpacaThrottle() // Rate-limit live-trading Alpaca calls
	if qty <= 0 {
		return nil, fmt.Errorf("alpaca: qty must be > 0, got %.6f", qty)
	}
	qtyStr := fmt.Sprintf("%g", qty)
	isFractional := qty != float64(int(qty))
	if !isFractional {
		qtyStr = fmt.Sprintf("%d", int(qty))
	}
	// Alpaca requires "day" TIF for fractional orders, "gtc" for whole shares
	tif := "gtc"
	if isFractional {
		tif = "day"
	}
	orderBody := map[string]interface{}{
		"symbol":        symbol,
		"qty":           qtyStr,
		"side":          side,
		"type":          "market",
		"time_in_force": tif,
	}
	// SL/TP is now managed server-side — no bracket/oto orders needed

	result, err := alpacaRequest("POST", "/v2/orders", orderBody, config)
	if err != nil {
		return nil, err
	}

	orderID, _ := result["id"].(string)
	status, _ := result["status"].(string)
	orderClass, _ := result["order_class"].(string)
	filledPrice := 0.0
	if fp, ok := result["filled_avg_price"].(string); ok && fp != "" {
		filledPrice, _ = strconv.ParseFloat(fp, 64)
	}

	var legs []AlpacaOrderLeg
	if rawLegs, ok := result["legs"].([]interface{}); ok {
		for _, rl := range rawLegs {
			if leg, ok := rl.(map[string]interface{}); ok {
				legType := ""
				if t, ok := leg["type"].(string); ok {
					legType = t
				}
				legID, _ := leg["id"].(string)
				legStatus, _ := leg["status"].(string)
				legs = append(legs, AlpacaOrderLeg{ID: legID, Type: legType, Status: legStatus})
			}
		}
	}

	return &AlpacaOrderResult{
		OrderID:        orderID,
		FilledAvgPrice: filledPrice,
		Status:         status,
		OrderClass:     orderClass,
		Legs:           legs,
	}, nil
}

func alpacaGetPositions(config LiveTradingConfig) ([]map[string]interface{}, error) {
	baseURL := alpacaBaseURL(config.AlpacaPaper)
	req, err := http.NewRequest("GET", baseURL+"/v2/positions", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", config.AlpacaApiKey)
	req.Header.Set("APCA-API-SECRET-KEY", config.AlpacaSecretKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("alpaca error %d: %s", resp.StatusCode, string(body))
	}

	var positions []map[string]interface{}
	json.Unmarshal(body, &positions)
	return positions, nil
}

func alpacaGetOrders(config LiveTradingConfig) ([]map[string]interface{}, error) {
	baseURL := alpacaBaseURL(config.AlpacaPaper)
	req, err := http.NewRequest("GET", baseURL+"/v2/orders?status=all&limit=500&direction=desc&nested=true", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", config.AlpacaApiKey)
	req.Header.Set("APCA-API-SECRET-KEY", config.AlpacaSecretKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("alpaca error %d: %s", resp.StatusCode, string(body))
	}

	var orders []map[string]interface{}
	json.Unmarshal(body, &orders)
	return orders, nil
}

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
	sessionsMu.Lock()
	sessions[token] = Session{
		UserID:  userID,
		IsAdmin: isAdmin,
		Expiry:  expiry,
	}
	sessionsMu.Unlock()

	return token
}

func getSession(token string) (*Session, bool) {
	// Try memory cache first
	sessionsMu.RLock()
	session, exists := sessions[token]
	sessionsMu.RUnlock()
	if exists {
		if time.Now().Before(session.Expiry) {
			return &session, true
		}
		// Expired in cache, remove it
		sessionsMu.Lock()
		delete(sessions, token)
		sessionsMu.Unlock()
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
	sess := Session{
		UserID:  dbSession.UserID,
		IsAdmin: dbSession.IsAdmin,
		Expiry:  newExpiry,
	}
	sessionsMu.Lock()
	sessions[token] = sess
	sessionsMu.Unlock()

	return &sess, true
}

func deleteSession(token string) {
	sessionsMu.Lock()
	delete(sessions, token)
	sessionsMu.Unlock()
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

	// Alpaca Data API keys are loaded from DB after AutoMigrate (see loadAlpacaWSKeys)

	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to database: " + err.Error())
	}

	// SQLite WAL mode: enables concurrent reads + background writes without "database is locked"
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")
	db.Exec("PRAGMA synchronous=NORMAL")
	// DB health check on startup
	if fi, err := os.Stat(dbPath); err == nil {
		sizeMB := float64(fi.Size()) / 1024 / 1024
		var freelistCount int64
		db.Raw("PRAGMA freelist_count").Scan(&freelistCount)
		if sizeMB > 500 {
			log.Printf("[DB] WARNING: Database is %.0f MB (freelist: %d pages) — consider VACUUM", sizeMB, freelistCount)
		} else {
			log.Printf("[DB] Size: %.0f MB, freelist: %d pages", sizeMB, freelistCount)
		}
	}
	// incremental_vacuum: reclaim freelist pages if auto_vacuum is already enabled
	db.Exec("PRAGMA incremental_vacuum(500)")

	// Drop unique indexes before AutoMigrate to prevent table recreation.
	// GORM SQLite migrator's parseDDL marks columns with UNIQUE INDEX as Unique=true,
	// but field.Unique is false for uniqueIndex tags → MigrateColumnUnique sees mismatch
	// → triggers recreateTable (very slow for large tables). Dropping indexes first avoids
	// the mismatch; AutoMigrate recreates them quickly via CREATE INDEX.
	db.Exec("DROP INDEX IF EXISTS idx_flipper_bot_positions_symbol")
	db.Exec("DROP INDEX IF EXISTS idx_lutz_positions_symbol")
	db.Exec("DROP INDEX IF EXISTS idx_arena_strategy_settings_strategy")
	db.Exec("DROP INDEX IF EXISTS idx_ohlcv_sym_iv")
	db.Exec("DROP INDEX IF EXISTS idx_weekly_ohlcv_caches_symbol")
	db.Exec("DROP INDEX IF EXISTS idx_stock_performances_symbol")
	db.Exec("DROP INDEX IF EXISTS idx_aggressive_stock_performances_symbol")
	db.Exec("DROP INDEX IF EXISTS idx_quant_stock_performances_symbol")
	db.Exec("DROP INDEX IF EXISTS idx_ditz_stock_performances_symbol")
	db.Exec("DROP INDEX IF EXISTS idx_trader_stock_performances_symbol")
	db.Exec("DROP INDEX IF EXISTS idx_trading_watchlist_items_symbol")
	db.Exec("DROP INDEX IF EXISTS idx_sym_strat")
	db.Exec("DROP INDEX IF EXISTS idx_signal_vis_sym_month")
	db.Exec("DROP INDEX IF EXISTS idx_bot_filter_configs_bot_name")
	db.Exec("DROP INDEX IF EXISTS idx_system_settings_key")
	db.Exec("DROP INDEX IF EXISTS idx_b_xtrender_configs_mode")
	db.Exec("DROP INDEX IF EXISTS idx_v2_sym_strat_iv")

	db.AutoMigrate(&User{}, &Stock{}, &Category{}, &PortfolioPosition{}, &PortfolioTradeHistory{}, &StockPerformance{}, &ActivityLog{}, &FlipperBotTrade{}, &FlipperBotPosition{}, &AggressiveStockPerformance{}, &LutzTrade{}, &LutzPosition{}, &DBSession{}, &BotLog{}, &BotTodo{}, &BXtrenderConfig{}, &BXtrenderQuantConfig{}, &QuantStockPerformance{}, &QuantTrade{}, &QuantPosition{}, &BXtrenderDitzConfig{}, &DitzStockPerformance{}, &DitzTrade{}, &DitzPosition{}, &BXtrenderTraderConfig{}, &TraderStockPerformance{}, &TraderTrade{}, &TraderPosition{}, &SystemSetting{}, &BotStockAllowlist{}, &BotFilterConfig{}, &SignalListFilterConfig{}, &SignalListVisibility{}, &UserNotification{}, &TradingWatchlistItem{}, &TradingVirtualPosition{}, &ArenaBacktestHistory{}, &ArenaStrategySettings{}, &WeeklyOHLCVCache{}, &OHLCVCache{}, &BacktestLabHistory{}, &LiveTradingConfig{}, &LiveTradingSession{}, &LiveTradingPosition{}, &LiveTradingLog{}, &LiveSessionStrategy{}, &ArenaV2BatchResult{}, &GlobalSetting{}, &AlpacaAccount{})

	// Migrate existing Alpaca keys from LiveTradingConfig to AlpacaAccount (one-time)
	var alpacaAccountCount int64
	db.Model(&AlpacaAccount{}).Count(&alpacaAccountCount)
	if alpacaAccountCount == 0 {
		var configs []LiveTradingConfig
		db.Where("alpaca_api_key != '' AND alpaca_api_key IS NOT NULL").Find(&configs)
		seen := map[string]uint{}
		for _, cfg := range configs {
			if cfg.AlpacaApiKey == "" {
				continue
			}
			if existingID, ok := seen[cfg.AlpacaApiKey]; ok {
				db.Model(&LiveTradingConfig{}).Where("id = ?", cfg.ID).Update("alpaca_account_id", existingID)
				continue
			}
			acc := AlpacaAccount{
				Name:      fmt.Sprintf("Account %d", len(seen)+1),
				ApiKey:    cfg.AlpacaApiKey,
				SecretKey: cfg.AlpacaSecretKey,
				IsPaper:   cfg.AlpacaPaper,
				IsDefault: len(seen) == 0,
			}
			db.Create(&acc)
			seen[cfg.AlpacaApiKey] = acc.ID
			db.Model(&LiveTradingConfig{}).Where("id = ?", cfg.ID).Update("alpaca_account_id", acc.ID)
		}
		if len(seen) > 0 {
			log.Printf("[AlpacaAccount] Migrated %d existing key(s) to alpaca_accounts", len(seen))
		}
	}

	// Migrate WeeklyOHLCVCache → OHLCVCache (one-time)
	var weeklyCount int64
	db.Model(&WeeklyOHLCVCache{}).Count(&weeklyCount)
	var ohlcvWeeklyCount int64
	db.Model(&OHLCVCache{}).Where("interval = ?", "1wk").Count(&ohlcvWeeklyCount)
	if weeklyCount > 0 && ohlcvWeeklyCount == 0 {
		log.Printf("[OHLCVCache] Migrating %d WeeklyOHLCVCache entries...", weeklyCount)
		var weeklyCaches []WeeklyOHLCVCache
		db.Find(&weeklyCaches)
		for _, wc := range weeklyCaches {
			var ohlcv []OHLCV
			barCount := 0
			if json.Unmarshal([]byte(wc.DataJSON), &ohlcv) == nil {
				barCount = len(ohlcv)
			}
			db.Create(&OHLCVCache{
				Symbol:   wc.Symbol,
				Interval: "1wk",
				DataJSON: wc.DataJSON,
				BarCount: barCount,
			})
		}
		log.Printf("[OHLCVCache] Migration done: %d entries migrated", weeklyCount)
	}

	// Ensure existing users are visible in ranking (new column defaults to false in SQLite)
	db.Exec("UPDATE users SET visible_in_ranking = 1 WHERE visible_in_ranking = 0 OR visible_in_ranking IS NULL")

	// Ensure default invite code exists
	var inviteSetting SystemSetting
	if db.Where("key = ?", "invite_code").First(&inviteSetting).Error != nil {
		db.Create(&SystemSetting{Key: "invite_code", Value: "KommInDieGruppe"})
	}

	// Load Alpaca WS/Data keys from default AlpacaAccount
	loadAlpacaWSKeys()

	// Startup: Load Alpaca tradable assets + clean trading watchlist
	if err := refreshTradableAssetsCache(); err != nil {
		log.Printf("[Alpaca] Warning: Could not load tradable assets: %v", err)
	} else {
		var allItems []TradingWatchlistItem
		db.Find(&allItems)
		var removeIDs []uint
		for _, item := range allItems {
			asset, ok := isAlpacaTradable(item.Symbol)
			if !ok {
				removeIDs = append(removeIDs, item.ID)
			} else {
				db.Model(&item).Update("fractionable", asset.Fractionable)
			}
		}
		if len(removeIDs) > 0 {
			db.Where("id IN ?", removeIDs).Delete(&TradingWatchlistItem{})
			log.Printf("[Alpaca] Removed %d non-Alpaca symbols from trading watchlist", len(removeIDs))
		}
		log.Printf("[Alpaca] Trading watchlist: %d symbols remaining", len(allItems)-len(removeIDs))
	}

	// Init file-based OHLCV cache (lazy-load, no startup RAM usage)
	ohlcvCacheDir = filepath.Join(filepath.Dir(dbPath), "ohlcv")
	initOHLCVFileCache()

	// Start live-trading position writer (serialized DB writes)
	go livePositionWriter()

	// Ensure "Sonstiges" category exists
	ensureSonstigesCategory()

	// Migrate existing LiveTradingSessions without LiveSessionStrategy entries (one-time)
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
		log.Printf("[Migration] LiveSessionStrategy erstellt für Session #%d (%s)", s.ID, s.Name)
	}

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

	// Drop old unique index on live_trading_configs.user_id (now supports multiple configs per user)
	db.Exec("DROP INDEX IF EXISTS idx_live_trading_configs_user_id")

	// Migrate old sessions without per-session config: assign global user config
	var legacySessions []LiveTradingSession
	db.Where("config_id = 0 OR config_id IS NULL").Find(&legacySessions)
	for _, ls := range legacySessions {
		var userConfig LiveTradingConfig
		if db.Where("user_id = ?", ls.UserID).Order("updated_at DESC").First(&userConfig).Error == nil {
			db.Model(&ls).Update("config_id", userConfig.ID)
			log.Printf("[Migration] Session #%d (%s): ConfigID → %d", ls.ID, ls.Name, userConfig.ID)
		}
	}

	// Initialize shared WebSocket for live trading (before auto-resume)
	initSharedWS()

	// Auto-resume orphaned live trading sessions (scheduler lost on restart)
	var orphanedSessions []LiveTradingSession
	db.Where("is_active = ?", true).Find(&orphanedSessions)
	if len(orphanedSessions) > 0 {
		log.Printf("[LiveTrading] Auto-Resume: %d aktive Sessions gefunden", len(orphanedSessions))
		for _, session := range orphanedSessions {
			// Sessions that were never truly started (ghost sessions from GORM default:true bug) → deactivate
			if session.StartedAt.IsZero() {
				log.Printf("[LiveTrading] Session #%d '%s' war nie gestartet (ghost) — deaktiviere", session.ID, session.Name)
				db.Model(&session).Update("is_active", false)
				continue
			}
			var symbols []string
			json.Unmarshal([]byte(session.Symbols), &symbols)
			// Initialize in-memory open position guard from DB
			initOpenPosGuard(session.ID)
			state := &liveSessionState{
				StopChan: make(chan struct{}),
			}
			liveSchedulerMu.Lock()
			liveSchedulers[session.ID] = state
			liveSchedulerMu.Unlock()

			// Check if session has Alpaca WS capability
			var liveConfig LiveTradingConfig
			if session.ConfigID > 0 {
				db.First(&liveConfig, session.ConfigID)
			} else {
				db.Where("user_id = ?", session.UserID).Order("updated_at DESC").First(&liveConfig)
			}
			if liveConfig.AlpacaEnabled && liveConfig.AlpacaApiKey != "" {
				state.Mode = "websocket"
				// Sofortiger Alpaca-Health-Check beim Resume
				if acct, err := alpacaGetAccount(liveConfig); err == nil {
					state.AlpacaActive = true
					state.AlpacaLastChecked = time.Now()
					_ = acct
				} else {
					state.AlpacaActive = false
					state.AlpacaError = err.Error()
					state.AlpacaLastChecked = time.Now()
				}
				// WS-Status sofort setzen wenn SharedWS aktiv
				if sharedWS != nil {
					state.UsesSharedWS = true
					state.WSConnected = sharedWS.IsConnected()
				}
				go runLiveWebSocket(state, session.ID, liveConfig)
			} else {
				state.Mode = "polling"
				go runLiveScheduler(state, session.ID)
			}
			log.Printf("[LiveTrading] Auto-Resume: Session #%d '%s' (%s %s, %d Symbole, mode: %s) gestartet",
				session.ID, session.Name, session.Strategy, session.Interval, len(symbols), state.Mode)
		}
	}

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
		api.PUT("/user/ranking-visibility", authMiddleware(), updateRankingVisibility)
		api.GET("/bot-blocked-stocks", authMiddleware(), getBlockedStocksForUser)

		// Profile & Notification routes
		api.GET("/profile", authMiddleware(), getProfile)
		api.PUT("/profile/password", authMiddleware(), changePassword)
		api.GET("/profile/activity", authMiddleware(), getProfileActivity)
		api.GET("/notifications", authMiddleware(), getNotifications)
		api.GET("/notifications/unread-count", authMiddleware(), getUnreadNotificationCount)
		api.PUT("/notifications/:id/read", authMiddleware(), markNotificationRead)
		api.PUT("/notifications/read-all", authMiddleware(), markAllNotificationsRead)

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
		api.GET("/admin/invite-code", authMiddleware(), adminOnly(), getInviteCodeHandler)
		api.PUT("/admin/invite-code", authMiddleware(), adminOnly(), setInviteCodeHandler)
		api.GET("/admin/bot-allowlist", authMiddleware(), adminOnly(), getBotAllowlist)
		api.PUT("/admin/bot-allowlist", authMiddleware(), adminOnly(), updateBotAllowlist)
		api.GET("/admin/bot-filter-config", authMiddleware(), adminOnly(), getBotFilterConfig)
		api.PUT("/admin/bot-filter-config", authMiddleware(), adminOnly(), updateBotFilterConfig)
		api.GET("/admin/export-watchlist", authMiddleware(), adminOnly(), exportWatchlist)
		api.POST("/admin/import-watchlist", authMiddleware(), adminOnly(), importWatchlist)

		// Trading Arena routes
		api.GET("/trading/watchlist", authMiddleware(), getTradingWatchlist)
		api.POST("/trading/watchlist", authMiddleware(), adminOnly(), addToTradingWatchlist)
		api.POST("/trading/watchlist/import", authMiddleware(), adminOnly(), importWatchlistToTrading)
		api.DELETE("/trading/watchlist/:id", authMiddleware(), adminOnly(), removeFromTradingWatchlist)
		api.POST("/trading/backtest", authMiddleware(), runBacktestHandler)
		api.POST("/trading/backtest-debug", authMiddleware(), backtestDebugHandler)
		api.GET("/trading/backtest-results/:symbol", authMiddleware(), getBacktestResultsHandler)
		api.POST("/trading/backtest-batch", authMiddleware(), adminOnly(), backtestBatchHandler)
		api.POST("/trading/backtest-watchlist", authMiddleware(), backtestWatchlistHandler)
		api.POST("/trading/arena/prefetch", authMiddleware(), arenaPrefetchHandler)
		api.GET("/trading/strategy-settings", authMiddleware(), getStrategySettings)
		api.POST("/trading/strategy-settings", authMiddleware(), saveStrategySettings)
		api.GET("/trading/positions", authMiddleware(), getTradingPositions)
		api.GET("/trading/scheduler/status", authMiddleware(), adminOnly(), getTradingSchedulerStatus)
		api.POST("/trading/scheduler/toggle", authMiddleware(), adminOnly(), toggleTradingScheduler)

		// Alpaca Account Management (admin only)
		api.GET("/admin/alpaca-accounts", authMiddleware(), adminOnly(), getAlpacaAccounts)
		api.POST("/admin/alpaca-accounts", authMiddleware(), adminOnly(), createAlpacaAccount)
		api.PUT("/admin/alpaca-accounts/:id", authMiddleware(), adminOnly(), updateAlpacaAccount)
		api.DELETE("/admin/alpaca-accounts/:id", authMiddleware(), adminOnly(), deleteAlpacaAccount)
		api.POST("/admin/alpaca-accounts/:id/validate", authMiddleware(), adminOnly(), validateAlpacaAccount)
		api.POST("/admin/alpaca-accounts/:id/default", authMiddleware(), adminOnly(), setDefaultAlpacaAccount)

		// Live Trading — write actions: admin only, read actions: all authenticated users
		api.POST("/trading/live/config", authMiddleware(), adminOnly(), saveLiveTradingConfig)
		api.GET("/trading/live/config", authMiddleware(), getLiveTradingConfig)
		api.POST("/trading/live/start", authMiddleware(), adminOnly(), startLiveTrading)
		api.POST("/trading/live/stop", authMiddleware(), adminOnly(), stopLiveTrading)
		api.GET("/trading/live/status", authMiddleware(), getLiveTradingStatus)
		api.GET("/trading/live/sessions", authMiddleware(), getLiveTradingSessions)
		api.GET("/trading/live/session/:id", authMiddleware(), getLiveTradingSession)
		api.POST("/trading/live/session/:id/resume", authMiddleware(), adminOnly(), resumeLiveTrading)
		api.PATCH("/trading/live/session/:id/name", authMiddleware(), adminOnly(), renameLiveSession)
		api.DELETE("/trading/live/session/:id", authMiddleware(), adminOnly(), deleteLiveSession)
		api.POST("/trading/live/session/:id/reset", authMiddleware(), adminOnly(), resetLiveSession)
		api.GET("/trading/live/session/:id/strategies", authMiddleware(), getLiveSessionStrategies)
		api.POST("/trading/live/session/:id/strategy", authMiddleware(), adminOnly(), addLiveSessionStrategy)
		api.PUT("/trading/live/session/:id/strategy/:strategyId", authMiddleware(), adminOnly(), toggleLiveSessionStrategy)
		api.GET("/trading/live/logs/:sessionId", authMiddleware(), getLiveTradingLogs)
		api.POST("/trading/live/analyze", authMiddleware(), analyzeLiveSymbolHandler)
		api.POST("/trading/live/alpaca/validate", authMiddleware(), adminOnly(), validateAlpacaKeys)
		api.POST("/trading/live/alpaca/test-order", authMiddleware(), adminOnly(), alpacaTestOrder)
		api.GET("/trading/live/alpaca/portfolio", authMiddleware(), getAlpacaPortfolio)

		// Arena v2
		api.POST("/trading/arena/v2/batch", authMiddleware(), arenaV2BatchHandler)
		api.GET("/trading/arena/v2/watchlist-grid", authMiddleware(), arenaV2WatchlistGrid)
		api.POST("/trading/arena/v2/start-session", authMiddleware(), adminOnly(), arenaV2StartSession)

		// Backtest Lab
		api.POST("/backtest-lab", authMiddleware(), runBacktestLabHandler)
		api.POST("/backtest-lab/batch", authMiddleware(), runBacktestLabBatchHandler)
		api.GET("/backtest-lab/history", authMiddleware(), getBacktestLabHistory)
		api.DELETE("/backtest-lab/history/:id", authMiddleware(), deleteBacktestLabHistory)

		// Public endpoint for fetching BXtrender config (no auth required for frontend calculation)
		api.GET("/bxtrender-config", getBXtrenderConfigPublic)

		// FlipperBot routes - Defensive mode (view: all users, actions: admin only)
		api.GET("/flipperbot/update", authMiddleware(), adminOnly(), flipperBotUpdate)
		api.GET("/flipperbot/portfolio", authMiddleware(), getFlipperBotPortfolio)
		api.GET("/flipperbot/actions", authMiddleware(), getFlipperBotActions)
		api.GET("/flipperbot/performance", authMiddleware(), getFlipperBotPerformance)
		api.POST("/flipperbot/reset", authMiddleware(), adminOnly(), resetFlipperBot)
		api.POST("/bots/reset-all", authMiddleware(), adminOnly(), resetAllBots)
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
		api.GET("/performance/history", optionalAuthMiddleware(), getPerformanceHistory)

		// Signal Liste routes
		api.GET("/signal-list", optionalAuthMiddleware(), getSignalList)
		api.GET("/signal-list/filter-config", getSignalListFilterConfig)
		api.PUT("/admin/signal-list/filter-config", authMiddleware(), adminOnly(), updateSignalListFilterConfig)
		api.PUT("/admin/signal-list/visibility", authMiddleware(), adminOnly(), toggleSignalListVisibility)

		// Global Settings (Alpaca Broker Keys etc.)
		api.GET("/admin/settings", authMiddleware(), adminOnly(), getGlobalSettings)
		api.POST("/admin/settings", authMiddleware(), adminOnly(), saveGlobalSettings)

		// DB maintenance
		api.POST("/admin/db-vacuum", authMiddleware(), adminOnly(), runDBVacuum)
	}

	// Start the daily stock update scheduler
	go startDailyUpdateScheduler()

	// Start persistent OHLCV cache background worker (Yahoo-based refresh)
	go startOHLCVCacheWorker()

	// Start SL/TP monitor (checks open positions every 2 min, independent of strategy interval)
	go startSLTPMonitor()

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
		Email      string `json:"email" binding:"required"`
		Username   string `json:"username" binding:"required"`
		Password   string `json:"password" binding:"required"`
		InviteCode string `json:"invite_code"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email, username and password are required"})
		return
	}

	// Validate invite code
	var inviteSetting SystemSetting
	if db.Where("key = ?", "invite_code").First(&inviteSetting).Error == nil && inviteSetting.Value != "" {
		if req.InviteCode != inviteSetting.Value {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültiger Invite-Code"})
			return
		}
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
		"user":  gin.H{"id": user.ID, "email": user.Email, "username": user.Username, "is_admin": user.IsAdmin, "visible_in_ranking": user.VisibleInRanking},
	})
}

func getCurrentUser(c *gin.Context) {
	userID, _ := c.Get("userID")
	var user User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": user.ID, "email": user.Email, "username": user.Username, "is_admin": user.IsAdmin, "visible_in_ranking": user.VisibleInRanking})
}

func updateRankingVisibility(c *gin.Context) {
	userID, _ := c.Get("userID")
	var req struct {
		Visible bool `json:"visible"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	db.Model(&User{}).Where("id = ?", userID).Update("visible_in_ranking", req.Visible)
	c.JSON(http.StatusOK, gin.H{"visible_in_ranking": req.Visible})
}

func getProfile(c *gin.Context) {
	userID, _ := c.Get("userID")
	var user User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Portfolio stats
	var openPositions int64
	db.Model(&PortfolioPosition{}).Where("user_id = ?", userID).Count(&openPositions)

	var trades []PortfolioTradeHistory
	db.Where("user_id = ?", userID).Find(&trades)

	closedTrades := len(trades)
	var wins int
	var bestTrade, worstTrade float64
	for _, t := range trades {
		if t.ProfitLossPct > 0 {
			wins++
		}
		if t.ProfitLossPct > bestTrade {
			bestTrade = t.ProfitLossPct
		}
		if t.ProfitLossPct < worstTrade {
			worstTrade = t.ProfitLossPct
		}
	}
	var winRate float64
	if closedTrades > 0 {
		winRate = float64(wins) / float64(closedTrades) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"username":           user.Username,
		"email":              user.Email,
		"created_at":         user.CreatedAt,
		"login_count":        user.LoginCount,
		"last_active":        user.LastActive,
		"visible_in_ranking": user.VisibleInRanking,
		"is_admin":           user.IsAdmin,
		"portfolio_stats": gin.H{
			"open_positions": openPositions,
			"closed_trades":  closedTrades,
			"win_rate":       winRate,
			"best_trade":     bestTrade,
			"worst_trade":    worstTrade,
		},
	})
}

func changePassword(c *gin.Context) {
	userID, _ := c.Get("userID")
	var req struct {
		OldPassword     string `json:"old_password"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Anfrage"})
		return
	}
	if len(req.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Neues Passwort muss mindestens 6 Zeichen lang sein"})
		return
	}
	if req.NewPassword != req.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Passwörter stimmen nicht überein"})
		return
	}

	var user User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User nicht gefunden"})
		return
	}
	if !checkPassword(req.OldPassword, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Altes Passwort ist falsch"})
		return
	}

	hashed, err := hashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Fehler beim Hashen"})
		return
	}
	db.Model(&user).Update("password", hashed)

	logUserActivity(user.ID, user.Username, "password_change", "Passwort geändert", c.ClientIP(), c.GetHeader("User-Agent"))
	c.JSON(http.StatusOK, gin.H{"message": "Passwort erfolgreich geändert"})
}

func getProfileActivity(c *gin.Context) {
	userID, _ := c.Get("userID")
	var logs []ActivityLog
	db.Where("user_id = ?", userID).Order("created_at desc").Limit(50).Find(&logs)
	c.JSON(http.StatusOK, logs)
}

func getNotifications(c *gin.Context) {
	userID, _ := c.Get("userID")
	var notifications []UserNotification
	db.Where("user_id = ?", userID).Order("created_at desc").Limit(100).Find(&notifications)
	c.JSON(http.StatusOK, notifications)
}

func getUnreadNotificationCount(c *gin.Context) {
	userID, _ := c.Get("userID")
	var count int64
	db.Model(&UserNotification{}).Where("user_id = ? AND is_read = ?", userID, false).Count(&count)
	c.JSON(http.StatusOK, gin.H{"count": count})
}

func markNotificationRead(c *gin.Context) {
	userID, _ := c.Get("userID")
	id := c.Param("id")
	result := db.Model(&UserNotification{}).Where("id = ? AND user_id = ?", id, userID).Update("is_read", true)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Benachrichtigung nicht gefunden"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func markAllNotificationsRead(c *gin.Context) {
	userID, _ := c.Get("userID")
	db.Model(&UserNotification{}).Where("user_id = ? AND is_read = ?", userID, false).Update("is_read", true)
	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

func getBlockedStocksForUser(c *gin.Context) {
	var entries []BotStockAllowlist
	db.Where("allowed = ?", false).Find(&entries)

	blocked := make(map[string][]string)
	for _, e := range entries {
		blocked[e.Symbol] = append(blocked[e.Symbol], e.BotName)
	}
	c.JSON(http.StatusOK, blocked)
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

// liveOwnerUID returns the user ID to use for live trading read-access.
// Admins see their own data; regular users see the first admin's data.
func liveOwnerUID(c *gin.Context) uint {
	userID, _ := c.Get("userID")
	uid := userID.(uint)
	isAdmin, _ := c.Get("isAdmin")
	if isAdminBool, ok := isAdmin.(bool); ok && isAdminBool {
		return uid
	}
	// Find admin who has a live trading config
	var config LiveTradingConfig
	if db.Joins("JOIN users ON users.id = live_trading_configs.user_id AND users.is_admin = ?", true).First(&config).Error == nil {
		return config.UserID
	}
	return uid
}

func maskKey(key string) string {
	if len(key) > 4 {
		return "****" + key[len(key)-4:]
	} else if key != "" {
		return "****"
	}
	return ""
}

func adminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("isAdmin")
		isAdminBool, ok := isAdmin.(bool)
		if !exists || !ok || !isAdminBool {
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

// aggregateOHLCV combines consecutive OHLCV candles by the given factor.
// O=first.Open, H=max(High), L=min(Low), C=last.Close, V=sum(Volume)
func aggregateOHLCV(data []OHLCV, factor int) []OHLCV {
	if factor <= 1 || len(data) == 0 {
		return data
	}
	result := make([]OHLCV, 0, len(data)/factor+1)
	for i := 0; i < len(data); i += factor {
		end := i + factor
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]
		agg := OHLCV{
			Time:   chunk[0].Time,
			Open:   chunk[0].Open,
			High:   chunk[0].High,
			Low:    chunk[0].Low,
			Close:  chunk[len(chunk)-1].Close,
			Volume: 0,
		}
		for _, bar := range chunk {
			if bar.High > agg.High {
				agg.High = bar.High
			}
			if bar.Low < agg.Low {
				agg.Low = bar.Low
			}
			agg.Volume += bar.Volume
		}
		result = append(result, agg)
	}
	return result
}

func getHistory(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))
	period := c.DefaultQuery("period", "6mo")
	interval := c.DefaultQuery("interval", "1d")

	// For monthly interval: use persistent cache (saves Yahoo + TwelveData calls)
	if interval == "1mo" {
		cached, err := getMonthlyOHLCVCached(symbol, 12*time.Hour)
		if err == nil && len(cached) > 0 {
			c.JSON(http.StatusOK, gin.H{
				"symbol":            symbol,
				"data":              cached,
				"requestedInterval": "1mo",
				"actualInterval":    "1mo",
				"source":            "cache",
			})
			return
		}
	}

	// For non-monthly intervals: check OHLCV cache first (getOHLCVCached handles 2h/4h aggregation)
	if interval != "1mo" {
		cached, err := getOHLCVCached(symbol, interval, 15*time.Minute)
		if err == nil && len(cached) > 0 {
			cutoff := periodToTime(period)
			filtered := filterOHLCVAfter(cached, cutoff)
			if len(filtered) > 0 {
				c.JSON(http.StatusOK, gin.H{
					"symbol":            symbol,
					"data":              filtered,
					"requestedInterval": interval,
					"actualInterval":    interval,
					"source":            "cache",
				})
				return
			}
		}
	}

	// Map 2h/4h to 60m fetch + aggregation (Yahoo doesn't support 2h/4h natively)
	requestedInterval := interval
	aggregateFactor := 0
	switch interval {
	case "2h":
		interval = "60m"
		aggregateFactor = 2
	case "4h":
		interval = "60m"
		aggregateFactor = 4
	}

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

	// Aggregate 60m candles into 2h/4h if requested
	if aggregateFactor > 0 {
		data = aggregateOHLCV(data, aggregateFactor)
		interval = requestedInterval
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

	if !checkTwelveDataBudget() {
		return nil, fmt.Errorf("TWELVE_DATA_RATE_LIMIT: daily budget exhausted (750/800)")
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
	data := fetchHistoricalData(symbol, period)
	if len(data) == 0 {
		return 0
	}
	for _, d := range data {
		if d.Close > 0 {
			return d.Close
		}
	}
	return 0
}

// Get all portfolios for comparison (public view)
func getAllPortfoliosForComparison(c *gin.Context) {
	// Get all users with positions — filter by ranking visibility (admin sees all)
	isAdmin, _ := c.Get("isAdmin")
	var users []User
	if isAdmin != nil && isAdmin.(bool) {
		db.Find(&users)
	} else {
		db.Where("visible_in_ranking = ?", true).Find(&users)
	}

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
		UserID           uint              `json:"user_id"`
		Username         string            `json:"username"`
		Positions        []PositionSummary `json:"positions"`
		TotalReturnPct   float64           `json:"total_return_pct"`
		PositionCount    int               `json:"position_count"`
		VisibleInRanking bool              `json:"visible_in_ranking"`
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
			UserID:           user.ID,
			Username:         user.Username,
			Positions:        posSummaries,
			TotalReturnPct:   weightedReturn,
			PositionCount:    len(positions),
			VisibleInRanking: user.VisibleInRanking,
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

	// Get all users with positions — filter by ranking visibility (admin sees all)
	isAdmin, _ := c.Get("isAdmin")
	var users []User
	if isAdmin != nil && isAdmin.(bool) {
		db.Find(&users)
	} else {
		db.Where("visible_in_ranking = ?", true).Find(&users)
	}

	type PortfolioHistory struct {
		UserID           uint                     `json:"user_id"`
		Username         string                   `json:"username"`
		History          []map[string]interface{} `json:"history"`
		PeriodReturnPct  float64                  `json:"period_return_pct"`
		VisibleInRanking bool                     `json:"visible_in_ranking"`
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
				cur := t.Currency
				if cur == "" {
					cur = "EUR"
				}
				buyUSD := convertToUSD(t.BuyPrice, cur)
				sellUSD := convertToUSD(t.SellPrice, cur)
				closedGain += (sellUSD - buyUSD) * t.Quantity
				closedInvested += buyUSD * t.Quantity
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
					UserID:           u.ID,
					Username:         u.Username,
					History:          history,
					PeriodReturnPct:  periodReturn,
					VisibleInRanking: u.VisibleInRanking,
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
			// Otherwise use the purchase price (AvgPrice) — all normalized to USD
			bp := convertToUSD(pos.AvgPrice, currency)
			if pos.PurchaseDate == nil || pos.PurchaseDate.Unix() <= allTimes[0] {
				// Position pre-dates the chart → base is first available price in chart
				if data, ok := symbolData[pos.Symbol]; ok && len(data) > 0 {
					bp = convertStockPrice(data[0].Close, pos.Symbol, "USD")
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

		// Calculate portfolio value at this time — all in USD for consistency
		var portfolioValue float64
		if hasQuantities {
			for _, e := range activeEntries {
				if price, ok := lastPrices[e.pos.Symbol]; ok {
					priceUSD := convertStockPrice(price, e.pos.Symbol, "USD")
					qty := 1.0
					if e.pos.Quantity != nil {
						qty = *e.pos.Quantity
					}
					portfolioValue += priceUSD * qty
				}
			}
		} else {
			for _, e := range activeEntries {
				if price, ok := lastPrices[e.pos.Symbol]; ok {
					priceUSD := convertStockPrice(price, e.pos.Symbol, "USD")
					if e.basePrice > 0 {
						portfolioValue += 1000 * (priceUSD / e.basePrice)
					}
				}
			}
		}

		if portfolioValue <= 0 {
			continue
		}

		// Detect when new positions join: rebase to prevent fake jump
		// Only rebase when positions INCREASE (not when sold/removed)
		if len(activeEntries) > prevActiveCount && prevActiveCount > 0 {
			if len(result) > 0 {
				prevPct = result[len(result)-1]["pct"].(float64)
			}
			baseValue = portfolioValue
		}
		prevActiveCount = len(activeEntries)

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

	// 1. In-memory cache (ultra short-term dedup)
	histCacheMu.RLock()
	if entry, ok := histCache[cacheKey]; ok && time.Since(entry.FetchedAt) < histCacheTTL {
		histCacheMu.RUnlock()
		return entry.Data
	}
	histCacheMu.RUnlock()

	// 2. Persistent cache (OHLCVCache "1d")
	var dbCache OHLCVCache
	if db.Where("symbol = ? AND interval = ?", symbol, "1d").First(&dbCache).Error == nil {
		if time.Since(dbCache.UpdatedAt) < 12*time.Hour {
			var fullData []OHLCV
			if json.Unmarshal([]byte(dbCache.DataJSON), &fullData) == nil && len(fullData) > 0 {
				cutoff := periodToTime(period)
				filtered := filterOHLCVAfter(fullData, cutoff)
				if len(filtered) > 0 {
					histCacheMu.Lock()
					histCache[cacheKey] = histCacheEntry{Data: filtered, FetchedAt: time.Now()}
					histCacheMu.Unlock()
					return filtered
				}
			}
		}
	}

	// 3. Yahoo fetch (existing logic)
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

	// 4. Store in both caches
	saveOHLCVCache(symbol, "1d", data)
	histCacheMu.Lock()
	histCache[cacheKey] = histCacheEntry{Data: data, FetchedAt: time.Now()}
	histCacheMu.Unlock()

	return data
}

// periodToTime converts a Yahoo-style period string to a cutoff time
func periodToTime(period string) time.Time {
	now := time.Now()
	switch period {
	case "1d":
		return now.AddDate(0, 0, -1)
	case "5d":
		return now.AddDate(0, 0, -5)
	case "1mo":
		return now.AddDate(0, -1, 0)
	case "3mo":
		return now.AddDate(0, -3, 0)
	case "6mo":
		return now.AddDate(0, -6, 0)
	case "1y":
		return now.AddDate(-1, 0, 0)
	case "2y":
		return now.AddDate(-2, 0, 0)
	case "5y":
		return now.AddDate(-5, 0, 0)
	case "10y":
		return now.AddDate(-10, 0, 0)
	case "max":
		return time.Time{} // zero time = no filter
	default:
		return now.AddDate(0, -6, 0) // default 6 months
	}
}

// filterOHLCVAfter returns only OHLCV bars after the cutoff time
func filterOHLCVAfter(data []OHLCV, cutoff time.Time) []OHLCV {
	if cutoff.IsZero() {
		return data
	}
	cutoffUnix := cutoff.Unix()
	for i, d := range data {
		if d.Time >= cutoffUnix {
			return data[i:]
		}
	}
	return nil
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

func isFirstOfMonth() bool {
	return time.Now().Day() == 1
}

func isStockDataStale(updatedAt time.Time) bool {
	return time.Since(updatedAt).Hours() > 48
}

// getWarmupEndDate checks if a stock needs warmup filtering based on its trade history.
// It only fetches OHLCV from Yahoo if the stock might be affected (first trade is recent).
// Returns 0 if no warmup filtering needed, math.MaxInt64 if ALL bars are warmup.
func getWarmupEndDate(symbol string, minBars int, trades []TradeData) int64 {
	if len(trades) == 0 {
		return 0
	}
	// Quick check: if the earliest trade is old enough, the stock definitely has enough data.
	// minBars months before the first trade = data must have started well before that.
	// If first trade is before 2019, the stock has 7+ years of monthly data (84+ bars) — no warmup issue.
	earliestTrade := trades[0].EntryDate
	for _, t := range trades[1:] {
		if t.EntryDate < earliestTrade {
			earliestTrade = t.EntryDate
		}
	}
	warmupMonths := int64(minBars + 12) // minBars + 1 year safety margin
	cutoff := time.Now().AddDate(0, -int(warmupMonths), 0).Unix()
	if earliestTrade < cutoff {
		// First trade is old enough — stock definitely had enough data, skip API call
		return 0
	}

	// Stock might be new — fetch OHLCV to check actual bar count
	ohlcv, err := fetchHistoricalDataServer(symbol)
	if err != nil || len(ohlcv) == 0 {
		return 0
	}
	if len(ohlcv) < minBars {
		return math.MaxInt64
	}
	return ohlcv[minBars-1].Time
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

	// DB health info
	dbHealth := gin.H{}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/watchlist.db"
	}
	if fi, err := os.Stat(dbPath); err == nil {
		dbHealth["size_bytes"] = fi.Size()
		dbHealth["size_mb"] = float64(fi.Size()) / 1024 / 1024
	}
	var pageCount, freelistCount int64
	db.Raw("PRAGMA page_count").Scan(&pageCount)
	db.Raw("PRAGMA freelist_count").Scan(&freelistCount)
	var pageSize int64
	db.Raw("PRAGMA page_size").Scan(&pageSize)
	dbHealth["page_count"] = pageCount
	dbHealth["freelist_count"] = freelistCount
	dbHealth["fragmentation_pct"] = 0.0
	if pageCount > 0 {
		dbHealth["fragmentation_pct"] = float64(freelistCount) / float64(pageCount) * 100
	}
	// All tables: name, row count, size
	type TableInfo struct {
		Name   string  `json:"name"`
		Count  int64   `json:"count"`
		SizeMB float64 `json:"size_mb"`
	}
	var tableNames []string
	db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").Scan(&tableNames)
	var tables []TableInfo
	for _, tbl := range tableNames {
		var cnt int64
		db.Raw("SELECT COUNT(*) FROM `" + tbl + "`").Scan(&cnt)
		// Get column names, then estimate size via SUM(LENGTH()) on a sample
		var sizeMB float64
		if cnt > 0 {
			var colNames []string
			rows, err := db.Raw("PRAGMA table_info(`" + tbl + "`)").Rows()
			if err == nil {
				for rows.Next() {
					var cid int
					var name, ctype string
					var notnull int
					var dfltValue *string
					var pk int
					rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk)
					colNames = append(colNames, name)
				}
				rows.Close()
			}
			// Build SUM(COALESCE(LENGTH(col1),0) + COALESCE(LENGTH(col2),0) + ...) query
			if len(colNames) > 0 {
				lenExprs := ""
				for i, col := range colNames {
					if i > 0 {
						lenExprs += "+"
					}
					lenExprs += "COALESCE(LENGTH(`" + col + "`),0)"
				}
				var sampleAvg float64
				db.Raw("SELECT AVG(row_len) FROM (SELECT " + lenExprs + " AS row_len FROM `" + tbl + "` LIMIT 200)").Scan(&sampleAvg)
				sizeMB = float64(cnt) * sampleAvg / 1024 / 1024
			}
		}
		tables = append(tables, TableInfo{Name: tbl, Count: cnt, SizeMB: sizeMB})
	}
	dbHealth["tables"] = tables

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
		"db_health":      dbHealth,
	})
}

// runDBVacuum runs VACUUM to compact the database and enable auto_vacuum
func runDBVacuum(c *gin.Context) {
	log.Println("[DB] Admin triggered VACUUM...")
	db.Exec("PRAGMA auto_vacuum=INCREMENTAL")
	if err := db.Exec("VACUUM").Error; err != nil {
		log.Printf("[DB] VACUUM failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Println("[DB] VACUUM complete")
	// Return new size
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/watchlist.db"
	}
	var sizeMB float64
	if fi, err := os.Stat(dbPath); err == nil {
		sizeMB = float64(fi.Size()) / 1024 / 1024
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "size_mb": sizeMB})
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

func getBlockedSymbolsForBot(botName string) []string {
	var entries []BotStockAllowlist
	db.Where("bot_name = ? AND allowed = ?", botName, false).Find(&entries)
	symbols := make([]string, len(entries))
	for i, e := range entries {
		symbols[i] = e.Symbol
	}
	return symbols
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

	if config.MinWinrate != nil && winRate < *config.MinWinrate {
		reasons = append(reasons, fmt.Sprintf("WinRate %.1f%% < Min %.1f%%", winRate, *config.MinWinrate))
	}
	if config.MaxWinrate != nil && winRate > *config.MaxWinrate {
		reasons = append(reasons, fmt.Sprintf("WinRate %.1f%% > Max %.1f%%", winRate, *config.MaxWinrate))
	}
	if config.MinRR != nil && riskReward < *config.MinRR {
		reasons = append(reasons, fmt.Sprintf("R/R %.2f < Min %.2f", riskReward, *config.MinRR))
	}
	if config.MaxRR != nil && riskReward > *config.MaxRR {
		reasons = append(reasons, fmt.Sprintf("R/R %.2f > Max %.2f", riskReward, *config.MaxRR))
	}
	if config.MinAvgReturn != nil && avgReturn < *config.MinAvgReturn {
		reasons = append(reasons, fmt.Sprintf("AvgReturn %.1f%% < Min %.1f%%", avgReturn, *config.MinAvgReturn))
	}
	if config.MaxAvgReturn != nil && avgReturn > *config.MaxAvgReturn {
		reasons = append(reasons, fmt.Sprintf("AvgReturn %.1f%% > Max %.1f%%", avgReturn, *config.MaxAvgReturn))
	}
	if config.MinMarketCap != nil {
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
		BotName         string `json:"bot_name" binding:"required"`
		Symbol          string `json:"symbol" binding:"required"`
		Allowed         bool   `json:"allowed"`
		RetroactiveScan bool   `json:"retroactive_scan"`
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
	retroactiveDeleted := 0

	if !req.Allowed {
		if req.RetroactiveScan {
			// Retroactive scan: soft-delete all trades + remove all positions
			retroactiveDeleted = retroactiveDeleteForBot(req.BotName, req.Symbol)
		} else {
			closedPosition = closePositionForBot(req.BotName, req.Symbol)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":              "Updated",
		"closed_position":      closedPosition,
		"retroactive_deleted":  retroactiveDeleted,
	})
}

func retroactiveDeleteForBot(botName, symbol string) int {
	var count int64
	switch botName {
	case "flipper":
		db.Model(&FlipperBotTrade{}).Where("symbol = ? AND is_deleted = ?", symbol, false).Count(&count)
		db.Model(&FlipperBotTrade{}).Where("symbol = ?", symbol).Update("is_deleted", true)
		db.Where("symbol = ?", symbol).Delete(&FlipperBotPosition{})
		db.Where("user_id = ? AND symbol = ?", FLIPPERBOT_USER_ID, symbol).Delete(&PortfolioPosition{})
	case "lutz":
		db.Model(&LutzTrade{}).Where("symbol = ? AND is_deleted = ?", symbol, false).Count(&count)
		db.Model(&LutzTrade{}).Where("symbol = ?", symbol).Update("is_deleted", true)
		db.Where("symbol = ?", symbol).Delete(&LutzPosition{})
		db.Where("user_id = ? AND symbol = ?", LUTZ_USER_ID, symbol).Delete(&PortfolioPosition{})
	case "quant":
		db.Model(&QuantTrade{}).Where("symbol = ? AND is_deleted = ?", symbol, false).Count(&count)
		db.Model(&QuantTrade{}).Where("symbol = ?", symbol).Update("is_deleted", true)
		db.Where("symbol = ?", symbol).Delete(&QuantPosition{})
		db.Where("user_id = ? AND symbol = ?", QUANT_USER_ID, symbol).Delete(&PortfolioPosition{})
	case "ditz":
		db.Model(&DitzTrade{}).Where("symbol = ? AND is_deleted = ?", symbol, false).Count(&count)
		db.Model(&DitzTrade{}).Where("symbol = ?", symbol).Update("is_deleted", true)
		db.Where("symbol = ?", symbol).Delete(&DitzPosition{})
		db.Where("user_id = ? AND symbol = ?", DITZ_USER_ID, symbol).Delete(&PortfolioPosition{})
	case "trader":
		db.Model(&TraderTrade{}).Where("symbol = ? AND is_deleted = ?", symbol, false).Count(&count)
		db.Model(&TraderTrade{}).Where("symbol = ?", symbol).Update("is_deleted", true)
		db.Where("symbol = ?", symbol).Delete(&TraderPosition{})
		db.Where("user_id = ? AND symbol = ?", TRADER_USER_ID, symbol).Delete(&PortfolioPosition{})
	}
	return int(count)
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

	// Auto-enable filter when any filter value is set
	hasAnyFilter := req.MinWinrate != nil || req.MaxWinrate != nil ||
		req.MinRR != nil || req.MaxRR != nil ||
		req.MinAvgReturn != nil || req.MaxAvgReturn != nil ||
		req.MinMarketCap != nil
	if hasAnyFilter {
		req.Enabled = true
	}

	var config BotFilterConfig
	result := db.Where("bot_name = ?", req.BotName).First(&config)
	if result.Error != nil {
		// CREATE: no existing config for this bot
		req.UpdatedAt = time.Now()
		if err := db.Create(&req).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create config"})
			return
		}
		c.JSON(http.StatusOK, req)
	} else {
		// UPDATE: use explicit map to ensure NULL values are written correctly
		updates := map[string]interface{}{
			"min_winrate":    req.MinWinrate,
			"max_winrate":    req.MaxWinrate,
			"min_rr":         req.MinRR,
			"max_rr":         req.MaxRR,
			"min_avg_return": req.MinAvgReturn,
			"max_avg_return": req.MaxAvgReturn,
			"min_market_cap": req.MinMarketCap,
			"enabled":        req.Enabled,
			"updated_at":     time.Now(),
		}
		db.Model(&config).Updates(updates)
		// Reload from DB to return the actual saved values
		db.Where("bot_name = ?", req.BotName).First(&config)
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

	// Only process signals on the 1st of the month to match calculated trade history
	if !isFirstOfMonth() {
		return
	}

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

		if isStockDataStale(stockPerf.UpdatedAt) {
			addLog("SKIP", fmt.Sprintf("%s: Daten älter als 48h (letztes Update: %s) - überspringe", pos.Symbol, stockPerf.UpdatedAt.Format("02.01.2006 15:04")))
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
		if isStockDataStale(stock.UpdatedAt) {
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

	// Set up streaming response for progress updates
	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	sendProgress := func(current, total int, symbol, message string) {
		line, _ := json.Marshal(gin.H{"type": "progress", "current": current, "total": total, "symbol": symbol, "message": message})
		c.Writer.Write(append(line, '\n'))
		c.Writer.Flush()
	}

	// Get all tracked stocks with their performance data
	var trackedStocks []StockPerformance
	db.Find(&trackedStocks)

	if len(trackedStocks) == 0 {
		line, _ := json.Marshal(gin.H{"type": "done", "trades_created": 0, "positions_created": 0, "logs": logs})
		c.Writer.Write(append(line, '\n'))
		c.Writer.Flush()
		return
	}

	var tradesCreated int
	var positionsCreated int

	for stockIdx, stock := range trackedStocks {
		sendProgress(stockIdx+1, len(trackedStocks), stock.Symbol, fmt.Sprintf("Verarbeite %s (%d/%d)", stock.Symbol, stockIdx+1, len(trackedStocks)))
		if stock.TradesJSON == "" {
			continue
		}

		// Check allowlist
		if !isStockAllowedForBot("flipper", stock.Symbol) {
			addLog("SKIP", fmt.Sprintf("%s: Nicht in Allowlist — übersprungen", stock.Symbol))
			continue
		}

		// Check bot filter config
		if filterBlocked, filterReason := checkBotFilterConfig("flipper", stock.WinRate, stock.RiskReward, stock.AvgReturn, stock.MarketCap); filterBlocked {
			addLog("FILTER", fmt.Sprintf("%s: Übersprungen durch Filter (%s)", stock.Symbol, filterReason))
			continue
		}

		// Check if bot already has an open position for this stock
		var existingBotPos FlipperBotPosition
		if db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingBotPos).Error == nil {
			addLog("SKIP", fmt.Sprintf("%s: Bot hat bereits offene Position — übersprungen", stock.Symbol))
			continue
		}

		// Parse the historical trades from TradesJSON
		var historicalTrades []TradeData
		if err := json.Unmarshal([]byte(stock.TradesJSON), &historicalTrades); err != nil {
			addLog("ERROR", fmt.Sprintf("%s: Fehler beim Parsen der Trades: %v", stock.Symbol, err))
			continue
		}

		// Check if there's already an open position from BEFORE or AT the backfill start date
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

		// Warmup detection: check if indicator has enough data for stable signals
		warmupEnd := getWarmupEndDate(stock.Symbol, 45, historicalTrades)

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

			// Check if trade is in warmup period (indicator not yet stable)
			isWarmup := warmupEnd > 0 && trade.EntryDate <= warmupEnd

			// Create BUY trade
			buyTrade := FlipperBotTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      trade.EntryPrice,
				SignalDate: entryTime,
				ExecutedAt: now,
				IsDeleted:  isWarmup,
			}
			db.Create(&buyTrade)
			tradesCreated++
			if isWarmup {
				addLog("WARMUP", fmt.Sprintf("%s: BUY @ $%.2f am %s — Indikator nicht eingeschwungen (45 Bars nötig)", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))
			} else {
				addLog("ACTION", fmt.Sprintf("%s: BUY erstellt @ $%.2f am %s", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))
			}

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
						IsDeleted:     isWarmup,
					}
					db.Create(&sellTrade)
					tradesCreated++
					if !isWarmup {
						addLog("ACTION", fmt.Sprintf("%s: SELL erstellt @ $%.2f am %s (%.2f%%)", stock.Symbol, *trade.ExitPrice, exitTime.Format("2006-01-02"), profitLossPct))
					}
				} else if !isWarmup {
					// Exit is in the future - create open position (skip for warmup trades)
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
			} else if trade.IsOpen && !isWarmup {
				// Trade is open with no exit - create position (skip for warmup trades)
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

	line, _ := json.Marshal(gin.H{"type": "done", "trades_created": tradesCreated, "positions_created": positionsCreated, "until_date": req.UntilDate, "logs": logs})
	c.Writer.Write(append(line, '\n'))
	c.Writer.Flush()
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

	// Set up streaming response for progress updates
	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	sendProgress := func(current, total int, symbol, message string) {
		line, _ := json.Marshal(gin.H{"type": "progress", "current": current, "total": total, "symbol": symbol, "message": message})
		c.Writer.Write(append(line, '\n'))
		c.Writer.Flush()
	}

	// Get all tracked stocks with their aggressive performance data
	var trackedStocks []AggressiveStockPerformance
	db.Find(&trackedStocks)

	if len(trackedStocks) == 0 {
		line, _ := json.Marshal(gin.H{"type": "done", "trades_created": 0, "positions_created": 0, "logs": logs})
		c.Writer.Write(append(line, '\n'))
		c.Writer.Flush()
		return
	}

	var tradesCreated int
	var positionsCreated int

	for stockIdx, stock := range trackedStocks {
		sendProgress(stockIdx+1, len(trackedStocks), stock.Symbol, fmt.Sprintf("Verarbeite %s (%d/%d)", stock.Symbol, stockIdx+1, len(trackedStocks)))
		if stock.TradesJSON == "" {
			continue
		}

		// Check allowlist
		if !isStockAllowedForBot("lutz", stock.Symbol) {
			addLog("SKIP", fmt.Sprintf("%s: Nicht in Allowlist — übersprungen", stock.Symbol))
			continue
		}

		// Check bot filter config
		if filterBlocked, filterReason := checkBotFilterConfig("lutz", stock.WinRate, stock.RiskReward, stock.AvgReturn, stock.MarketCap); filterBlocked {
			addLog("FILTER", fmt.Sprintf("%s: Übersprungen durch Filter (%s)", stock.Symbol, filterReason))
			continue
		}

		// Check if bot already has an open position for this stock
		var existingBotPos LutzPosition
		if db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingBotPos).Error == nil {
			addLog("SKIP", fmt.Sprintf("%s: Bot hat bereits offene Position — übersprungen", stock.Symbol))
			continue
		}

		// Parse the historical trades from TradesJSON
		var historicalTrades []TradeData
		if err := json.Unmarshal([]byte(stock.TradesJSON), &historicalTrades); err != nil {
			addLog("ERROR", fmt.Sprintf("%s: Fehler beim Parsen der Trades: %v", stock.Symbol, err))
			continue
		}

		// Warmup detection: check if indicator has enough data for stable signals
		warmupEnd := getWarmupEndDate(stock.Symbol, 45, historicalTrades)

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

			// Check if trade is in warmup period (indicator not yet stable)
			isWarmup := warmupEnd > 0 && trade.EntryDate <= warmupEnd

			// Create BUY trade
			buyTrade := LutzTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      trade.EntryPrice,
				SignalDate: entryTime,
				ExecutedAt: now,
				IsDeleted:  isWarmup,
			}
			db.Create(&buyTrade)
			tradesCreated++
			if isWarmup {
				addLog("WARMUP", fmt.Sprintf("%s: BUY @ $%.2f am %s — Indikator nicht eingeschwungen (45 Bars nötig)", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))
			} else {
				addLog("ACTION", fmt.Sprintf("%s: BUY erstellt @ $%.2f am %s", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))
			}

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
						IsDeleted:     isWarmup,
					}
					db.Create(&sellTrade)
					tradesCreated++
					if !isWarmup {
						addLog("ACTION", fmt.Sprintf("%s: SELL erstellt @ $%.2f am %s (%.2f%%)", stock.Symbol, *trade.ExitPrice, exitTime.Format("2006-01-02"), profitLossPct))
					}
				} else if !isWarmup {
					// Exit is in the future - create open position (skip for warmup trades)
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
			} else if trade.IsOpen && !isWarmup {
				// Trade is open with no exit - create position (skip for warmup trades)
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

	line, _ := json.Marshal(gin.H{"type": "done", "trades_created": tradesCreated, "positions_created": positionsCreated, "until_date": req.UntilDate, "logs": logs})
	c.Writer.Write(append(line, '\n'))
	c.Writer.Flush()
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
	q := db.Where("is_pending = ? AND is_closed = ?", false, false)
	if blocked := getBlockedSymbolsForBot("flipper"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("buy_date desc").Find(&positions)

	// Fetch current quotes
	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	// Fetch market caps from stocks table
	marketCaps := make(map[string]int64)
	if len(symbols) > 0 {
		var mcStocks []Stock
		db.Select("symbol, market_cap").Where("symbol IN ? AND market_cap > 0", symbols).Find(&mcStocks)
		for _, s := range mcStocks {
			marketCaps[s.Symbol] = s.MarketCap
		}
	}

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
		MarketCap     int64     `json:"market_cap"`
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
			MarketCap:     marketCaps[pos.Symbol],
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
	q := db.Where("is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false)
	if blocked := getBlockedSymbolsForBot("flipper"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("signal_date desc, executed_at desc").Limit(50).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getFlipperBotActionsAll(c *gin.Context) {
	var trades []FlipperBotTrade
	db.Where("is_pending = ?", false).Order("signal_date desc").Limit(100).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getFlipperBotPerformance(c *gin.Context) {
	blocked := getBlockedSymbolsForBot("flipper")

	var sellTrades []FlipperBotTrade
	sq := db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false, false)
	if len(blocked) > 0 {
		sq = sq.Where("symbol NOT IN ?", blocked)
	}
	sq.Find(&sellTrades)

	var buyTrades []FlipperBotTrade
	bq := db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "BUY", false, false, false)
	if len(blocked) > 0 {
		bq = bq.Where("symbol NOT IN ?", blocked)
	}
	bq.Find(&buyTrades)

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

	// Fetch market caps from stocks table
	marketCaps := make(map[string]int64)
	if len(symbols) > 0 {
		var mcStocks []Stock
		db.Select("symbol, market_cap").Where("symbol IN ? AND market_cap > 0", symbols).Find(&mcStocks)
		for _, s := range mcStocks {
			marketCaps[s.Symbol] = s.MarketCap
		}
	}

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
		MarketCap      int64     `json:"market_cap"`
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
			MarketCap:      marketCaps[pos.Symbol],
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
	type posInfo struct {
		Symbol     string
		Quantity   float64
		AvgPrice   float64
		BuyDate    time.Time
		SellDate   *time.Time
		IsClosed   bool
		ProfitLoss float64
	}

	var allPositions []posInfo

	// Load ALL positions (open + closed) for the bot
	if botType == "flipperbot-live" || botType == "flipperbot-sim" {
		isLive := botType == "flipperbot-live"
		var positions []FlipperBotPosition
		db.Where("is_pending = ? AND is_live = ?", false, isLive).Find(&positions)
		for _, p := range positions {
			pl := 0.0
			if p.ProfitLoss != nil {
				pl = *p.ProfitLoss
			}
			allPositions = append(allPositions, posInfo{p.Symbol, p.Quantity, p.AvgPrice, p.BuyDate, p.SellDate, p.IsClosed, pl})
		}
	} else if botType == "quant-live" || botType == "quant-sim" {
		isLive := botType == "quant-live"
		var positions []QuantPosition
		db.Where("is_pending = ? AND is_live = ?", false, isLive).Find(&positions)
		for _, p := range positions {
			pl := 0.0
			if p.ProfitLoss != nil {
				pl = *p.ProfitLoss
			}
			allPositions = append(allPositions, posInfo{p.Symbol, p.Quantity, p.AvgPrice, p.BuyDate, p.SellDate, p.IsClosed, pl})
		}
	} else if botType == "ditz-live" || botType == "ditz-sim" {
		isLive := botType == "ditz-live"
		var positions []DitzPosition
		db.Where("is_pending = ? AND is_live = ?", false, isLive).Find(&positions)
		for _, p := range positions {
			pl := 0.0
			if p.ProfitLoss != nil {
				pl = *p.ProfitLoss
			}
			allPositions = append(allPositions, posInfo{p.Symbol, p.Quantity, p.AvgPrice, p.BuyDate, p.SellDate, p.IsClosed, pl})
		}
	} else if botType == "trader-live" || botType == "trader-sim" {
		isLive := botType == "trader-live"
		var positions []TraderPosition
		db.Where("is_pending = ? AND is_live = ?", false, isLive).Find(&positions)
		for _, p := range positions {
			pl := 0.0
			if p.ProfitLoss != nil {
				pl = *p.ProfitLoss
			}
			allPositions = append(allPositions, posInfo{p.Symbol, p.Quantity, p.AvgPrice, p.BuyDate, p.SellDate, p.IsClosed, pl})
		}
	} else if botType == "lutz-live" || botType == "lutz-sim" {
		isLive := botType == "lutz-live"
		var positions []LutzPosition
		db.Where("is_pending = ? AND is_live = ?", false, isLive).Find(&positions)
		for _, p := range positions {
			pl := 0.0
			if p.ProfitLoss != nil {
				pl = *p.ProfitLoss
			}
			allPositions = append(allPositions, posInfo{p.Symbol, p.Quantity, p.AvgPrice, p.BuyDate, p.SellDate, p.IsClosed, pl})
		}
	}

	if len(allPositions) == 0 {
		return []map[string]interface{}{}
	}

	// Collect unique symbols and find earliest BuyDate
	symbolSet := make(map[string]bool)
	var earliestBuy time.Time
	for _, p := range allPositions {
		symbolSet[p.Symbol] = true
		if earliestBuy.IsZero() || p.BuyDate.Before(earliestBuy) {
			earliestBuy = p.BuyDate
		}
	}
	var symbols []string
	for sym := range symbolSet {
		symbols = append(symbols, sym)
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

	// Filter timestamps: only from earliest BuyDate onwards
	earliestUnix := earliestBuy.Unix()
	var filteredTimes []int64
	for _, t := range allTimes {
		if t >= earliestUnix {
			filteredTimes = append(filteredTimes, t)
		}
	}
	if len(filteredTimes) == 0 {
		filteredTimes = allTimes // fallback: use all if no match
	}

	// Pre-fill last known prices from OHLCV data
	lastPrices := make(map[string]float64)
	for sym, data := range symbolData {
		for _, candle := range data {
			if candle.Time <= filteredTimes[0] {
				lastPrices[sym] = candle.Close
			}
		}
	}
	// Also seed with avgPrice for positions without OHLCV data at start
	for _, p := range allPositions {
		if _, ok := lastPrices[p.Symbol]; !ok {
			lastPrices[p.Symbol] = p.AvgPrice
		}
	}

	result := make([]map[string]interface{}, 0)

	for _, t := range filteredTimes {
		// Update prices
		if prices, ok := timeValues[t]; ok {
			for sym, price := range prices {
				lastPrices[sym] = price
			}
		}

		var unrealized float64
		var realized float64
		var invested float64

		for _, p := range allPositions {
			if p.BuyDate.Unix() > t {
				continue // not yet opened
			}

			cost := p.AvgPrice * p.Quantity

			if p.IsClosed && p.SellDate != nil && p.SellDate.Unix() <= t {
				// Closed by this time — count realized P&L
				invested += cost
				realized += p.ProfitLoss
			} else {
				// Still open at this time
				invested += cost
				if price, ok := lastPrices[p.Symbol]; ok {
					unrealized += (price - p.AvgPrice) * p.Quantity
				}
			}
		}

		if invested > 0 {
			pct := ((unrealized + realized) / invested) * 100
			result = append(result, map[string]interface{}{
				"time": t,
				"pct":  pct,
			})
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

func resetAllBots(c *gin.Context) {
	// FlipperBot
	db.Where("1 = 1").Delete(&FlipperBotTrade{})
	db.Where("1 = 1").Delete(&FlipperBotPosition{})
	db.Where("user_id = ?", FLIPPERBOT_USER_ID).Delete(&PortfolioPosition{})
	// Lutz
	db.Where("1 = 1").Delete(&LutzTrade{})
	db.Where("1 = 1").Delete(&LutzPosition{})
	db.Where("user_id = ?", LUTZ_USER_ID).Delete(&PortfolioPosition{})
	// Quant
	db.Where("1 = 1").Delete(&QuantTrade{})
	db.Where("1 = 1").Delete(&QuantPosition{})
	db.Where("user_id = ?", QUANT_USER_ID).Delete(&PortfolioPosition{})
	db.Where("bot = ?", "quant").Delete(&BotTodo{})
	db.Where("bot = ?", "quant").Delete(&BotLog{})
	// Ditz
	db.Where("1 = 1").Delete(&DitzTrade{})
	db.Where("1 = 1").Delete(&DitzPosition{})
	db.Where("user_id = ?", DITZ_USER_ID).Delete(&PortfolioPosition{})
	db.Where("bot = ?", "ditz").Delete(&BotTodo{})
	db.Where("bot = ?", "ditz").Delete(&BotLog{})
	// Trader
	db.Where("1 = 1").Delete(&TraderTrade{})
	db.Where("1 = 1").Delete(&TraderPosition{})
	db.Where("user_id = ?", TRADER_USER_ID).Delete(&PortfolioPosition{})
	db.Where("bot = ?", "trader").Delete(&BotTodo{})
	db.Where("bot = ?", "trader").Delete(&BotLog{})

	c.JSON(http.StatusOK, gin.H{"message": "All bots reset completed"})
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
	q := db.Where("action = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false)
	if blocked := getBlockedSymbolsForBot("flipper"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("signal_date desc").Find(&trades)

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

	// Only process signals on the 1st of the month to match calculated trade history
	if !isFirstOfMonth() {
		return
	}

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

		if isStockDataStale(stockPerf.UpdatedAt) {
			addLog("SKIP", fmt.Sprintf("%s: Daten älter als 48h (letztes Update: %s) - überspringe", pos.Symbol, stockPerf.UpdatedAt.Format("02.01.2006 15:04")))
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
		if isStockDataStale(stock.UpdatedAt) {
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
	q := db.Where("is_pending = ? AND is_closed = ?", false, false)
	if blocked := getBlockedSymbolsForBot("lutz"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("buy_date desc").Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	// Fetch market caps from stocks table
	marketCaps := make(map[string]int64)
	if len(symbols) > 0 {
		var mcStocks []Stock
		db.Select("symbol, market_cap").Where("symbol IN ? AND market_cap > 0", symbols).Find(&mcStocks)
		for _, s := range mcStocks {
			marketCaps[s.Symbol] = s.MarketCap
		}
	}

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
		MarketCap      int64     `json:"market_cap"`
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
			MarketCap:      marketCaps[pos.Symbol],
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
	q := db.Where("is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false)
	if blocked := getBlockedSymbolsForBot("lutz"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("signal_date desc").Limit(50).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getLutzActionsAll(c *gin.Context) {
	var trades []LutzTrade
	db.Where("is_pending = ?", false).Order("signal_date desc").Limit(100).Find(&trades)
	c.JSON(http.StatusOK, trades)
}

func getLutzPerformance(c *gin.Context) {
	blocked := getBlockedSymbolsForBot("lutz")

	var sellTrades []LutzTrade
	sq := db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false, false)
	if len(blocked) > 0 {
		sq = sq.Where("symbol NOT IN ?", blocked)
	}
	sq.Find(&sellTrades)

	var buyTrades []LutzTrade
	bq := db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "BUY", false, false, false)
	if len(blocked) > 0 {
		bq = bq.Where("symbol NOT IN ?", blocked)
	}
	bq.Find(&buyTrades)

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

	// Fetch market caps from stocks table
	marketCaps := make(map[string]int64)
	if len(symbols) > 0 {
		var mcStocks []Stock
		db.Select("symbol, market_cap").Where("symbol IN ? AND market_cap > 0", symbols).Find(&mcStocks)
		for _, s := range mcStocks {
			marketCaps[s.Symbol] = s.MarketCap
		}
	}

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
		MarketCap      int64     `json:"market_cap"`
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
			MarketCap:      marketCaps[pos.Symbol],
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
	q := db.Where("action = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false)
	if blocked := getBlockedSymbolsForBot("lutz"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("signal_date desc").Find(&trades)

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
			// Check 1: Soft-deleted BUY without subsequent SELL blocks re-entry
			var deletedBuy LutzTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ?", stock.Symbol, "BUY", true).Order("signal_date desc").First(&deletedBuy).Error; err == nil {
				var sellAfterDeleted LutzTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND signal_date > ?", stock.Symbol, "SELL", false, deletedBuy.SignalDate).First(&sellAfterDeleted).Error; err != nil {
					continue // Soft-deleted BUY without sell — skip
				}
			}

			// Check 2: Active BUY (not deleted, not filter-blocked) without subsequent SELL blocks re-entry
			var existingBuy LutzTrade
			if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND is_filter_blocked = ?", stock.Symbol, "BUY", false, false).Order("signal_date desc").First(&existingBuy).Error; err == nil {
				var sellAfter LutzTrade
				if err := db.Where("symbol = ? AND action = ? AND is_deleted = ? AND signal_date > ?", stock.Symbol, "SELL", false, existingBuy.SignalDate).First(&sellAfter).Error; err != nil {
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

	// Only process signals on the 1st of the month to match calculated trade history
	if !isFirstOfMonth() {
		return
	}

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

		if isStockDataStale(stockPerf.UpdatedAt) {
			addLog("SKIP", fmt.Sprintf("%s: Daten älter als 48h (letztes Update: %s) - überspringe", pos.Symbol, stockPerf.UpdatedAt.Format("02.01.2006 15:04")))
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
		if isStockDataStale(stock.UpdatedAt) {
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
	q := db.Where("is_pending = ? AND is_closed = ?", false, false)
	if blocked := getBlockedSymbolsForBot("quant"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("buy_date desc").Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	// Fetch market caps from stocks table
	marketCaps := make(map[string]int64)
	if len(symbols) > 0 {
		var mcStocks []Stock
		db.Select("symbol, market_cap").Where("symbol IN ? AND market_cap > 0", symbols).Find(&mcStocks)
		for _, s := range mcStocks {
			marketCaps[s.Symbol] = s.MarketCap
		}
	}

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
		MarketCap      int64     `json:"market_cap"`
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
			MarketCap:      marketCaps[pos.Symbol],
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
	q := db.Where("is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false)
	if blocked := getBlockedSymbolsForBot("quant"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("signal_date desc").Limit(50).Find(&trades)
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
	blocked := getBlockedSymbolsForBot("quant")

	var sellTrades []QuantTrade
	sq := db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false, false)
	if len(blocked) > 0 {
		sq = sq.Where("symbol NOT IN ?", blocked)
	}
	sq.Find(&sellTrades)

	var buyTrades []QuantTrade
	bq := db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "BUY", false, false, false)
	if len(blocked) > 0 {
		bq = bq.Where("symbol NOT IN ?", blocked)
	}
	bq.Find(&buyTrades)

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

	// Set up streaming response for progress updates
	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	sendProgress := func(current, total int, symbol, message string) {
		line, _ := json.Marshal(gin.H{"type": "progress", "current": current, "total": total, "symbol": symbol, "message": message})
		c.Writer.Write(append(line, '\n'))
		c.Writer.Flush()
	}

	// Get all tracked stocks with their quant performance data
	var trackedStocks []QuantStockPerformance
	db.Find(&trackedStocks)

	if len(trackedStocks) == 0 {
		line, _ := json.Marshal(gin.H{"type": "done", "trades_created": 0, "positions_created": 0, "logs": logs})
		c.Writer.Write(append(line, '\n'))
		c.Writer.Flush()
		return
	}

	var tradesCreated int
	var positionsCreated int

	for stockIdx, stock := range trackedStocks {
		sendProgress(stockIdx+1, len(trackedStocks), stock.Symbol, fmt.Sprintf("Verarbeite %s (%d/%d)", stock.Symbol, stockIdx+1, len(trackedStocks)))
		if stock.TradesJSON == "" {
			continue
		}

		// Check allowlist
		if !isStockAllowedForBot("quant", stock.Symbol) {
			addLog("SKIP", fmt.Sprintf("%s: Nicht in Allowlist — übersprungen", stock.Symbol))
			continue
		}

		// Check bot filter config
		if filterBlocked, filterReason := checkBotFilterConfig("quant", stock.WinRate, stock.RiskReward, stock.AvgReturn, stock.MarketCap); filterBlocked {
			addLog("FILTER", fmt.Sprintf("%s: Übersprungen durch Filter (%s)", stock.Symbol, filterReason))
			continue
		}

		// Check if bot already has an open position for this stock
		var existingBotPos QuantPosition
		if db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingBotPos).Error == nil {
			addLog("SKIP", fmt.Sprintf("%s: Bot hat bereits offene Position — übersprungen", stock.Symbol))
			continue
		}

		var historicalTrades []TradeData
		if err := json.Unmarshal([]byte(stock.TradesJSON), &historicalTrades); err != nil {
			addLog("ERROR", fmt.Sprintf("%s: Fehler beim Parsen der Trades: %v", stock.Symbol, err))
			continue
		}

		// Check if there's already an open position from BEFORE or AT the backfill start date
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

		// Warmup detection: check if indicator has enough data for stable signals
		warmupEnd := getWarmupEndDate(stock.Symbol, 225, historicalTrades)

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
			// Check if trade is in warmup period (indicator not yet stable)
			isWarmup := warmupEnd > 0 && trade.EntryDate <= warmupEnd


			buyTrade := QuantTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      trade.EntryPrice,
				SignalDate: entryTime,
				ExecutedAt: entryTime,
				IsPending:  false,
				IsDeleted:  isWarmup,
			}
			db.Create(&buyTrade)
			tradesCreated++
			if isWarmup {
				addLog("WARMUP", fmt.Sprintf("%s: BUY @ $%.2f am %s — Indikator nicht eingeschwungen (225 Bars nötig)", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))
			} else {
				addLog("ACTION", fmt.Sprintf("%s: BUY erstellt @ $%.2f am %s", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))
			}

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
						IsDeleted:     isWarmup,
					}
					db.Create(&sellTrade)
					tradesCreated++
					if !isWarmup {
						addLog("ACTION", fmt.Sprintf("%s: SELL erstellt @ $%.2f am %s (%.2f%%)", stock.Symbol, *trade.ExitPrice, exitTime.Format("2006-01-02"), profitLossPct))
					}
				} else if !isWarmup {
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
			} else if trade.IsOpen && !isWarmup {
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

	line, _ := json.Marshal(gin.H{"type": "done", "trades_created": tradesCreated, "positions_created": positionsCreated, "until_date": req.UntilDate, "logs": logs})
	c.Writer.Write(append(line, '\n'))
	c.Writer.Flush()
}

func getQuantCompletedTrades(c *gin.Context) {
	var trades []QuantTrade
	q := db.Where("action = ? AND profit_loss IS NOT NULL AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false)
	if blocked := getBlockedSymbolsForBot("quant"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("executed_at desc").Find(&trades)

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

	// Fetch market caps from stocks table
	marketCaps := make(map[string]int64)
	if len(symbols) > 0 {
		var mcStocks []Stock
		db.Select("symbol, market_cap").Where("symbol IN ? AND market_cap > 0", symbols).Find(&mcStocks)
		for _, s := range mcStocks {
			marketCaps[s.Symbol] = s.MarketCap
		}
	}

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
		MarketCap      int64     `json:"market_cap"`
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
			MarketCap:      marketCaps[pos.Symbol],
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

// getInviteCodeHandler returns the current invite code
func getInviteCodeHandler(c *gin.Context) {
	var setting SystemSetting
	code := "KommInDieGruppe"
	if err := db.Where("key = ?", "invite_code").First(&setting).Error; err == nil {
		code = setting.Value
	}
	c.JSON(http.StatusOK, gin.H{"code": code})
}

// setInviteCodeHandler updates the invite code
func setInviteCodeHandler(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invite-Code darf nicht leer sein"})
		return
	}

	var setting SystemSetting
	if err := db.Where("key = ?", "invite_code").First(&setting).Error; err != nil {
		setting = SystemSetting{Key: "invite_code", Value: req.Code, UpdatedAt: time.Now()}
		db.Create(&setting)
	} else {
		setting.Value = req.Code
		setting.UpdatedAt = time.Now()
		db.Save(&setting)
	}

	c.JSON(http.StatusOK, gin.H{"code": req.Code, "message": "Invite-Code aktualisiert"})
}

// runFullStockUpdate performs the full stock update for all watchlist stocks
func runFullStockUpdate(triggeredBy string) {
	fmt.Printf("[FullUpdate] Starting full stock update triggered by: %s\n", triggeredBy)

	// Capture signal snapshot BEFORE update for notification generation
	preSignals := captureSignalSnapshot()

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

	// --- Phase 1: Batch-fetch all market caps upfront (saves ~2000 individual API calls) ---
	allSymbols := make([]string, len(stocks))
	for i, s := range stocks {
		allSymbols[i] = s.Symbol
	}
	fmt.Printf("[FullUpdate] Batch-fetching market caps for %d symbols...\n", len(allSymbols))
	marketCapMap := fetchMarketCapBatch(allSymbols)
	fmt.Printf("[FullUpdate] Got market caps for %d/%d symbols\n", len(marketCapMap), len(allSymbols))

	// --- Phase 2: Smart cache freshness ---
	// Monthly OHLCV data only changes at month boundaries.
	// Days 1-3: use 20h freshness (new month bar just appeared)
	// Days 4+:  use 7 days freshness (data unchanged, save Yahoo calls)
	now := time.Now()
	cacheFreshness := 7 * 24 * time.Hour
	if now.Day() <= 3 {
		cacheFreshness = 20 * time.Hour
	}
	fmt.Printf("[FullUpdate] Day %d of month → cache freshness: %v\n", now.Day(), cacheFreshness)

	// --- Phase 3: Process stocks sequentially, sleep only on cache miss ---
	successCount := 0
	failedCount := 0
	cacheMisses := 0

	for i, stock := range stocks {
		if (i+1)%100 == 0 || i == 0 {
			fmt.Printf("[FullUpdate] Processing %d/%d (cache misses so far: %d)...\n", i+1, len(stocks), cacheMisses)
		}

		// Check if monthly OHLCV cache is still fresh (to decide on rate limiting)
		needsFetch := false
		var cache OHLCVCache
		if db.Where("symbol = ? AND interval = ?", stock.Symbol, "1mo").First(&cache).Error != nil {
			needsFetch = true // no cache entry
		} else if time.Since(cache.UpdatedAt) >= cacheFreshness {
			needsFetch = true // cache stale
		}

		// Rate limit only when we'll actually hit Yahoo
		if needsFetch {
			cacheMisses++
			time.Sleep(1500 * time.Millisecond)
		}

		mc := marketCapMap[stock.Symbol]
		err := processStockServer(stock.Symbol, stock.Name, defensiveConfig, aggressiveConfig, quantConfig, ditzConfig, traderConfig, mc, cacheFreshness)
		if err != nil {
			fmt.Printf("[FullUpdate] Failed to process %s: %v\n", stock.Symbol, err)
			failedCount++
		} else {
			successCount++
		}
	}
	fmt.Printf("[FullUpdate] Cache misses: %d/%d (%.0f%% from cache)\n", cacheMisses, len(stocks), float64(len(stocks)-cacheMisses)/float64(len(stocks))*100)

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

	// Generate signal change notifications for portfolio holders
	generateSignalNotifications(preSignals)
}

func captureSignalSnapshot() map[string]string {
	snapshot := map[string]string{}
	type symbolSignal struct {
		Symbol string
		Signal string
	}
	tables := []struct {
		model interface{}
		mode  string
	}{
		{&StockPerformance{}, "Defensiv"},
		{&AggressiveStockPerformance{}, "Aggressiv"},
		{&QuantStockPerformance{}, "Quant"},
		{&DitzStockPerformance{}, "Ditz"},
		{&TraderStockPerformance{}, "Trader"},
	}
	for _, t := range tables {
		var rows []symbolSignal
		db.Model(t.model).Select("symbol, signal").Find(&rows)
		for _, r := range rows {
			snapshot[t.mode+"_"+r.Symbol] = r.Signal
		}
	}
	return snapshot
}

func generateSignalNotifications(preSignals map[string]string) {
	// Delete old notifications > 90 days
	db.Where("created_at < ?", time.Now().AddDate(0, -3, 0)).Delete(&UserNotification{})

	postSignals := captureSignalSnapshot()

	// Load all users with portfolio positions
	var positions []PortfolioPosition
	db.Select("DISTINCT user_id, symbol, name").Find(&positions)
	userSymbols := map[uint][]PortfolioPosition{}
	for _, p := range positions {
		userSymbols[p.UserID] = append(userSymbols[p.UserID], p)
	}

	modes := []string{"Defensiv", "Aggressiv", "Quant", "Ditz", "Trader"}
	var notifications []UserNotification
	for userID, poss := range userSymbols {
		for _, pos := range poss {
			for _, mode := range modes {
				key := mode + "_" + pos.Symbol
				oldSig, newSig := preSignals[key], postSignals[key]
				if oldSig != "" && newSig != "" && oldSig != newSig {
					notifications = append(notifications, UserNotification{
						UserID:    userID,
						Symbol:    pos.Symbol,
						Name:      pos.Name,
						Mode:      mode,
						OldSignal: oldSig,
						NewSignal: newSig,
					})
				}
			}
		}
	}
	if len(notifications) > 0 {
		db.CreateInBatches(notifications, 100)
		fmt.Printf("[FullUpdate] Created %d signal change notifications\n", len(notifications))
	}
}

// processStockServer processes a single stock and saves performance data.
// marketCap is passed in from the caller (batch-fetched upfront).
// cacheFreshness controls how long the monthly OHLCV cache is considered fresh.
func processStockServer(symbol, name string, defensiveConfig, aggressiveConfig BXtrenderConfig, quantConfig BXtrenderQuantConfig, ditzConfig BXtrenderDitzConfig, traderConfig BXtrenderTraderConfig, marketCap int64, cacheFreshness time.Duration) error {
	data, err := getMonthlyOHLCVCached(symbol, cacheFreshness)
	if err != nil {
		return fmt.Errorf("failed to fetch historical data: %v", err)
	}

	if len(data) < 50 {
		return fmt.Errorf("not enough data points: %d", len(data))
	}

	currentPrice := data[len(data)-1].Close
	latestPriceCache.Store(symbol, currentPrice)

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

// fetchMarketCapBatch fetches market caps for many symbols using Yahoo v7/quote API
// in batches of 50. Returns map[symbol]marketCap. Much more efficient than per-symbol calls.
func fetchMarketCapBatch(symbols []string) map[string]int64 {
	result := make(map[string]int64, len(symbols))
	if len(symbols) == 0 {
		return result
	}

	const batchSize = 50
	for i := 0; i < len(symbols); i += batchSize {
		end := i + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batch := symbols[i:end]

		// Rate limit between batches
		if i > 0 {
			time.Sleep(500 * time.Millisecond)
		}

		batchResult := fetchMarketCapBatchSingle(batch)
		for sym, mc := range batchResult {
			result[sym] = mc
		}
	}

	return result
}

// fetchMarketCapBatchSingle fetches market caps for up to 50 symbols in one Yahoo v7/quote request.
func fetchMarketCapBatchSingle(symbols []string) map[string]int64 {
	result := make(map[string]int64, len(symbols))

	for attempt := 0; attempt < 2; attempt++ {
		client, crumb, err := getYahooCrumbClient()
		if err != nil {
			fmt.Printf("[MarketCapBatch] Crumb error: %v\n", err)
			return result
		}

		encodedSymbols := make([]string, len(symbols))
		for i, s := range symbols {
			encodedSymbols[i] = url.QueryEscape(s)
		}
		symbolsStr := strings.Join(encodedSymbols, ",")

		quoteURL := fmt.Sprintf("https://query1.finance.yahoo.com/v7/finance/quote?symbols=%s&crumb=%s",
			symbolsStr, url.QueryEscape(crumb))

		req, _ := http.NewRequest("GET", quoteURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("[MarketCapBatch] Request error: %v\n", err)
			return result
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			resetYahooCrumb()
			continue
		}

		var quoteData map[string]interface{}
		if json.Unmarshal(body, &quoteData) == nil {
			if qr, ok := quoteData["quoteResponse"].(map[string]interface{}); ok {
				if results, ok := qr["result"].([]interface{}); ok {
					for _, item := range results {
						if r, ok := item.(map[string]interface{}); ok {
							sym, _ := r["symbol"].(string)
							if mc, ok := r["marketCap"].(float64); ok && sym != "" && int64(mc) > 0 {
								result[sym] = int64(mc)
							}
						}
					}
				}
			}
		}

		if resp.StatusCode == 200 {
			break
		}
		resetYahooCrumb()
	}

	return result
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

	// Batch-fetch all market caps
	mcMap := fetchMarketCapBatch(symbols)

	updated := 0
	failed := 0
	type detail struct {
		Symbol    string `json:"symbol"`
		MarketCap int64  `json:"market_cap"`
	}
	details := []detail{}

	for _, sym := range symbols {
		mc := mcMap[sym]
		if mc > 0 {
			for _, table := range tables {
				db.Table(table).Where("symbol = ?", sym).Update("market_cap", mc)
			}
			db.Model(&Stock{}).Where("symbol = ?", sym).Update("market_cap", mc)
			updated++
		} else {
			failed++
		}
		details = append(details, detail{Symbol: sym, MarketCap: mc})
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

// ========== Trading Arena Indicators ==========

// extractCloses extracts close prices from OHLCV data
func extractCloses(ohlcv []OHLCV) []float64 {
	closes := make([]float64, len(ohlcv))
	for i, bar := range ohlcv {
		closes[i] = bar.Close
	}
	return closes
}

// calculateAwesomeOscillator calculates AO = SMA(5, midpoints) - SMA(34, midpoints)
func calculateAwesomeOscillator(ohlcv []OHLCV) []float64 {
	midpoints := make([]float64, len(ohlcv))
	for i, bar := range ohlcv {
		midpoints[i] = (bar.High + bar.Low) / 2
	}
	sma5 := calculateSMAServer(midpoints, 5)
	sma34 := calculateSMAServer(midpoints, 34)
	ao := make([]float64, len(ohlcv))
	for i := range ohlcv {
		ao[i] = sma5[i] - sma34[i]
	}
	return ao
}

// calculateHeikinAshi converts OHLCV data to Heikin Ashi candles
func calculateHeikinAshi(ohlcv []OHLCV) []OHLCV {
	if len(ohlcv) == 0 {
		return nil
	}
	ha := make([]OHLCV, len(ohlcv))
	// First candle
	ha[0].Time = ohlcv[0].Time
	ha[0].Close = (ohlcv[0].Open + ohlcv[0].High + ohlcv[0].Low + ohlcv[0].Close) / 4
	ha[0].Open = (ohlcv[0].Open + ohlcv[0].Close) / 2
	ha[0].High = math.Max(ohlcv[0].High, math.Max(ha[0].Open, ha[0].Close))
	ha[0].Low = math.Min(ohlcv[0].Low, math.Min(ha[0].Open, ha[0].Close))
	ha[0].Volume = ohlcv[0].Volume

	for i := 1; i < len(ohlcv); i++ {
		ha[i].Time = ohlcv[i].Time
		ha[i].Close = (ohlcv[i].Open + ohlcv[i].High + ohlcv[i].Low + ohlcv[i].Close) / 4
		ha[i].Open = (ha[i-1].Open + ha[i-1].Close) / 2
		ha[i].High = math.Max(ohlcv[i].High, math.Max(ha[i].Open, ha[i].Close))
		ha[i].Low = math.Min(ohlcv[i].Low, math.Min(ha[i].Open, ha[i].Close))
		ha[i].Volume = ohlcv[i].Volume
	}
	return ha
}

// solveLinearSystem solves Ax=b using Gaussian elimination with partial pivoting
func solveLinearSystem(A [][]float64, b []float64) []float64 {
	n := len(b)
	// Augmented matrix
	aug := make([][]float64, n)
	for i := 0; i < n; i++ {
		aug[i] = make([]float64, n+1)
		copy(aug[i], A[i])
		aug[i][n] = b[i]
	}
	// Forward elimination with partial pivoting
	for col := 0; col < n; col++ {
		maxRow := col
		maxVal := math.Abs(aug[col][col])
		for row := col + 1; row < n; row++ {
			if math.Abs(aug[row][col]) > maxVal {
				maxVal = math.Abs(aug[row][col])
				maxRow = row
			}
		}
		aug[col], aug[maxRow] = aug[maxRow], aug[col]
		if math.Abs(aug[col][col]) < 1e-12 {
			return make([]float64, n)
		}
		for row := col + 1; row < n; row++ {
			factor := aug[row][col] / aug[col][col]
			for j := col; j <= n; j++ {
				aug[row][j] -= factor * aug[col][j]
			}
		}
	}
	// Back substitution
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		x[i] = aug[i][n]
		for j := i + 1; j < n; j++ {
			x[i] -= aug[i][j] * x[j]
		}
		x[i] /= aug[i][i]
	}
	return x
}

// calculatePolyRegressionBands calculates Moving Regression Prediction Bands
// Source: TradingView Script zOaMXJ65 (tbtkg)
func calculatePolyRegressionBands(closes []float64, degree, length int, multiplier float64) (upper, middle, lower []float64) {
	n := len(closes)
	upper = make([]float64, n)
	middle = make([]float64, n)
	lower = make([]float64, n)

	if n < length+1 {
		return
	}

	prevPrediction := 0.0
	prevRMSE := 0.0
	hasPrev := false

	for i := length - 1; i < n; i++ {
		// Window: closes[i-length+1 .. i]
		window := closes[i-length+1 : i+1]
		dim := degree + 1

		// Build normal equations: X^T X beta = X^T y
		// X columns: [1, x, x², ...], x = 0..length-1
		XtX := make([][]float64, dim)
		for r := 0; r < dim; r++ {
			XtX[r] = make([]float64, dim)
		}
		Xty := make([]float64, dim)

		for j := 0; j < length; j++ {
			xj := float64(j)
			xPow := 1.0
			for r := 0; r < dim; r++ {
				xPow2 := 1.0
				for c := 0; c < dim; c++ {
					XtX[r][c] += xPow * xPow2
					xPow2 *= xj
				}
				Xty[r] += xPow * window[j]
				xPow *= xj
			}
		}

		beta := solveLinearSystem(XtX, Xty)

		// Prediction at x = length (extrapolation to next point)
		prediction := 0.0
		xPow := 1.0
		xVal := float64(length)
		for d := 0; d < dim; d++ {
			prediction += beta[d] * xPow
			xPow *= xVal
		}

		// RMSE
		sumSqErr := 0.0
		for j := 0; j < length; j++ {
			xj := float64(j)
			fitted := 0.0
			xp := 1.0
			for d := 0; d < dim; d++ {
				fitted += beta[d] * xp
				xp *= xj
			}
			diff := window[j] - fitted
			sumSqErr += diff * diff
		}
		rmse := math.Sqrt(sumSqErr / float64(length))

		// Bands use PREVIOUS prediction + RMSE
		if hasPrev {
			middle[i] = prevPrediction
			upper[i] = prevPrediction + multiplier*prevRMSE
			lower[i] = prevPrediction - multiplier*prevRMSE
		} else {
			middle[i] = prediction
			upper[i] = prediction + multiplier*rmse
			lower[i] = prediction - multiplier*rmse
		}

		prevPrediction = prediction
		prevRMSE = rmse
		hasPrev = true
	}

	return
}

// nadarayaWatsonSmooth applies Nadaraya-Watson kernel smoothing with Gaussian kernel.
// Skips zero values (warmup period) to match Pine Script behavior with na values.
func nadarayaWatsonSmooth(data []float64, bandwidth float64, lookback int) []float64 {
	n := len(data)
	smoothed := make([]float64, n)

	for i := 0; i < n; i++ {
		if data[i] == 0 {
			continue // skip warmup bars
		}
		sumWeighted := 0.0
		sumWeights := 0.0
		start := i - lookback
		if start < 0 {
			start = 0
		}
		for j := start; j <= i; j++ {
			if data[j] == 0 {
				continue // skip na/warmup values
			}
			offset := float64(i - j)
			weight := math.Exp(-(offset * offset) / (2 * bandwidth * bandwidth))
			sumWeighted += weight * data[j]
			sumWeights += weight
		}
		if sumWeights > 0 {
			smoothed[i] = sumWeighted / sumWeights
		}
	}
	return smoothed
}

// calculateSingleBBLevel computes one Bollinger Band level from HLC3 with NW smoothing.
// Source: TradingView Script LUoxSDKw (Flux Charts) — "Bollinger Bands (Nadaraya Smoothed)"
func calculateSingleBBLevel(ohlcv []OHLCV, period int, stdevMult, nwBandwidth float64, nwLookback int) (smoothedUpper, smoothedLower []float64) {
	n := len(ohlcv)
	smoothedUpper = make([]float64, n)
	smoothedLower = make([]float64, n)

	if n < period {
		return
	}

	// Source: hlc3 = (high + low + close) / 3
	src := make([]float64, n)
	for i, bar := range ohlcv {
		src[i] = (bar.High + bar.Low + bar.Close) / 3
	}

	// Standard Bollinger Bands on HLC3
	bbBasis := calculateSMAServer(src, period)
	bbUpper := make([]float64, n)
	bbLower := make([]float64, n)

	for i := period - 1; i < n; i++ {
		sumSq := 0.0
		for j := i - period + 1; j <= i; j++ {
			diff := src[j] - bbBasis[i]
			sumSq += diff * diff
		}
		stdev := math.Sqrt(sumSq / float64(period))
		bbUpper[i] = bbBasis[i] + stdevMult*stdev
		bbLower[i] = bbBasis[i] - stdevMult*stdev
	}

	// Apply Nadaraya-Watson Gaussian kernel smoothing (non-repainting mode)
	smoothedUpper = nadarayaWatsonSmooth(bbUpper, nwBandwidth, nwLookback)
	smoothedLower = nadarayaWatsonSmooth(bbLower, nwBandwidth, nwLookback)

	return
}

// calculateHybridEMA calculates the Hybrid EMA AlgoLearner oscillator (k-NN weighted)
// Source: TradingView Script 4jhuhtMN (Uldisbebris)
func calculateHybridEMA(closes []float64, shortP, longP, k, lookback, normLookback int) []float64 {
	n := len(closes)
	scaled := make([]float64, n)
	if n < longP+lookback {
		return scaled
	}

	shortEMA := calculateEMAServer(closes, shortP)
	longEMA := calculateEMAServer(closes, longP)

	// k-NN inspired weighted EMA
	weightEMA := make([]float64, n)
	for i := 0; i < n; i++ {
		if i < lookback {
			weightEMA[i] = shortEMA[i]
			continue
		}
		type distPair struct {
			dist float64
			idx  int
		}
		distances := make([]distPair, 0, lookback)
		for j := 1; j <= lookback; j++ {
			d := math.Abs(shortEMA[i] - longEMA[i-j])
			distances = append(distances, distPair{d, i - j})
		}
		sort.Slice(distances, func(a, b int) bool {
			return distances[a].dist < distances[b].dist
		})
		if len(distances) > k {
			distances = distances[:k]
		}
		sumWeighted := 0.0
		sumDist := 0.0
		for _, dp := range distances {
			w := 1.0 / (dp.dist + 1e-10)
			sumWeighted += shortEMA[dp.idx] * w
			sumDist += w
		}
		if sumDist > 0 {
			weightEMA[i] = sumWeighted / sumDist
		} else {
			weightEMA[i] = shortEMA[i]
		}
	}

	// Normalize to 0-100 scale
	for i := 0; i < n; i++ {
		start := i - normLookback
		if start < 0 {
			start = 0
		}
		minVal := weightEMA[start]
		maxVal := weightEMA[start]
		for j := start + 1; j <= i; j++ {
			if weightEMA[j] < minVal {
				minVal = weightEMA[j]
			}
			if weightEMA[j] > maxVal {
				maxVal = weightEMA[j]
			}
		}
		if maxVal-minVal > 1e-10 {
			scaled[i] = (weightEMA[i] - minVal) / (maxVal - minVal) * 100
		} else {
			scaled[i] = 50
		}
	}

	return scaled
}

// ========== Diamond Signals Indicators ==========

// findLocalExtrema identifies local peaks and troughs in a data series.
// A point at index i is a peak if it is the highest within `order` bars on both sides.
func findLocalExtrema(data []float64, order int) (peaks []int, troughs []int) {
	if len(data) < 2*order+1 {
		return
	}
	for i := order; i < len(data)-order; i++ {
		isPeak := true
		isTrough := true
		for j := 1; j <= order; j++ {
			if data[i] <= data[i-j] || data[i] <= data[i+j] {
				isPeak = false
			}
			if data[i] >= data[i-j] || data[i] >= data[i+j] {
				isTrough = false
			}
			if !isPeak && !isTrough {
				break
			}
		}
		if isPeak {
			peaks = append(peaks, i)
		}
		if isTrough {
			troughs = append(troughs, i)
		}
	}
	return
}

// detectRSIDivergence detects classic RSI divergences.
// Bullish: price lower low + RSI higher low. Bearish: price higher high + RSI lower high.
func detectRSIDivergence(closes []float64, rsiPeriod, lookback int) (bullDiv, bearDiv []int) {
	if len(closes) < rsiPeriod+20 {
		return
	}
	rsi := calculateRSIServer(closes, rsiPeriod)
	pricePeaks, priceTroughs := findLocalExtrema(closes, 5)
	rsiPeaks, rsiTroughs := findLocalExtrema(rsi, 5)

	// Helper: find closest RSI extremum to a price extremum (within tolerance)
	findClosest := func(rsiExtrema []int, target int, tolerance int) (int, bool) {
		bestIdx := -1
		bestDist := tolerance + 1
		for _, idx := range rsiExtrema {
			dist := target - idx
			if dist < 0 {
				dist = -dist
			}
			if dist <= tolerance && dist < bestDist {
				bestDist = dist
				bestIdx = idx
			}
		}
		return bestIdx, bestIdx >= 0
	}

	// Bullish divergence: consecutive price troughs
	for i := 1; i < len(priceTroughs); i++ {
		t2 := priceTroughs[i]
		t1 := priceTroughs[i-1]
		if t2-t1 > lookback {
			continue
		}
		if closes[t2] >= closes[t1] {
			continue // not lower low
		}
		r1, ok1 := findClosest(rsiTroughs, t1, 3)
		r2, ok2 := findClosest(rsiTroughs, t2, 3)
		if ok1 && ok2 && rsi[r2] > rsi[r1] {
			bullDiv = append(bullDiv, t2)
		}
	}

	// Bearish divergence: consecutive price peaks
	for i := 1; i < len(pricePeaks); i++ {
		p2 := pricePeaks[i]
		p1 := pricePeaks[i-1]
		if p2-p1 > lookback {
			continue
		}
		if closes[p2] <= closes[p1] {
			continue // not higher high
		}
		r1, ok1 := findClosest(rsiPeaks, p1, 3)
		r2, ok2 := findClosest(rsiPeaks, p2, 3)
		if ok1 && ok2 && rsi[r2] < rsi[r1] {
			bearDiv = append(bearDiv, p2)
		}
	}
	return
}

// detectVolumeDivergence detects volume-price disagreement.
// Bearish: price new high + volume declining. Bullish: price new low + volume declining.
func detectVolumeDivergence(ohlcv []OHLCV, lookback int) (bullVolDiv, bearVolDiv []int) {
	if len(ohlcv) < 20 {
		return
	}
	closes := extractCloses(ohlcv)
	volumes := make([]float64, len(ohlcv))
	for i, bar := range ohlcv {
		volumes[i] = bar.Volume
	}

	pricePeaks, priceTroughs := findLocalExtrema(closes, 5)

	// Bearish: price higher high, volume lower
	for i := 1; i < len(pricePeaks); i++ {
		p2 := pricePeaks[i]
		p1 := pricePeaks[i-1]
		if p2-p1 > lookback {
			continue
		}
		if closes[p2] > closes[p1] && volumes[p2] < volumes[p1] {
			bearVolDiv = append(bearVolDiv, p2)
		}
	}

	// Bullish: price lower low, volume lower
	for i := 1; i < len(priceTroughs); i++ {
		t2 := priceTroughs[i]
		t1 := priceTroughs[i-1]
		if t2-t1 > lookback {
			continue
		}
		if closes[t2] < closes[t1] && volumes[t2] < volumes[t1] {
			bullVolDiv = append(bullVolDiv, t2)
		}
	}
	return
}

// detectDiamondPattern detects diamond chart patterns (expansion then contraction).
// bullDiamond = bottom reversal (breakout down from diamond). bearDiamond = top reversal.
func detectDiamondPattern(ohlcv []OHLCV, length int) (bullDiamond, bearDiamond []int) {
	if len(ohlcv) < length || length < 4 {
		return
	}
	halfLen := length / 2

	for i := length - 1; i < len(ohlcv); i++ {
		// First half: check range expansion
		expandCount := 0
		for j := 1; j < halfLen; j++ {
			idx := i - length + 1 + j
			prevIdx := idx - 1
			rangeJ := ohlcv[idx].High - ohlcv[idx].Low
			rangePrev := ohlcv[prevIdx].High - ohlcv[prevIdx].Low
			if rangeJ > rangePrev {
				expandCount++
			}
		}

		// Second half: check range contraction
		contractCount := 0
		for j := 1; j < halfLen; j++ {
			idx := i - halfLen + j
			prevIdx := idx - 1
			rangeJ := ohlcv[idx].High - ohlcv[idx].Low
			rangePrev := ohlcv[prevIdx].High - ohlcv[prevIdx].Low
			if rangeJ < rangePrev {
				contractCount++
			}
		}

		// Find max range bar position within window
		maxRange := 0.0
		maxRangePos := 0
		for j := 0; j < length; j++ {
			idx := i - length + 1 + j
			r := ohlcv[idx].High - ohlcv[idx].Low
			if r > maxRange {
				maxRange = r
				maxRangePos = j
			}
		}
		relPos := float64(maxRangePos) / float64(length)

		expandThresh := int(float64(halfLen-1) * 0.6)
		contractThresh := int(float64(halfLen-1) * 0.6)

		if expandCount >= expandThresh && contractCount >= contractThresh && relPos > 0.25 && relPos < 0.75 {
			midIdx := i - halfLen
			midPrice := (ohlcv[midIdx].High + ohlcv[midIdx].Low) / 2
			exitPrice := ohlcv[i].Close

			if exitPrice < midPrice {
				bullDiamond = append(bullDiamond, i) // broke down → bottom reversal
			} else {
				bearDiamond = append(bearDiamond, i) // broke up → top reversal
			}
		}
	}
	return
}

// detectOrderBlocks identifies supply/demand zones from high-volume impulse candles.
// Returns zones as [low, high] price ranges.
func detectOrderBlocks(ohlcv []OHLCV, lookback int) (demandZones, supplyZones [][2]float64) {
	if len(ohlcv) < 21 {
		return
	}

	for i := 20; i < len(ohlcv); i++ {
		// Calculate 20-bar average volume and body
		avgVol := 0.0
		avgBody := 0.0
		for j := i - 20; j < i; j++ {
			avgVol += ohlcv[j].Volume
			avgBody += math.Abs(ohlcv[j].Close - ohlcv[j].Open)
		}
		avgVol /= 20
		avgBody /= 20

		body := math.Abs(ohlcv[i].Close - ohlcv[i].Open)
		isBullishImpulse := ohlcv[i].Close > ohlcv[i].Open && ohlcv[i].Volume > avgVol*1.5 && body > avgBody*1.5
		isBearishImpulse := ohlcv[i].Close < ohlcv[i].Open && ohlcv[i].Volume > avgVol*1.5 && body > avgBody*1.5

		if isBullishImpulse {
			// Demand zone = last bearish candle before impulse
			for j := i - 1; j >= i-10 && j >= 0; j-- {
				if ohlcv[j].Close < ohlcv[j].Open {
					demandZones = append(demandZones, [2]float64{ohlcv[j].Low, ohlcv[j].High})
					break
				}
			}
		}
		if isBearishImpulse {
			// Supply zone = last bullish candle before impulse
			for j := i - 1; j >= i-10 && j >= 0; j-- {
				if ohlcv[j].Close > ohlcv[j].Open {
					supplyZones = append(supplyZones, [2]float64{ohlcv[j].Low, ohlcv[j].High})
					break
				}
			}
		}
	}

	// Only keep zones from recent bars
	if lookback > 0 && len(ohlcv) > lookback {
		threshold := len(ohlcv) - lookback
		_ = threshold // zones are added chronologically, keep all for now (proximity check handles relevance)
	}
	return
}

// ========== Diamond Signals Utility Helpers ==========

// toIndexMap converts a slice of indices to a map for O(1) lookup
func toIndexMap(indices []int) map[int]bool {
	m := make(map[int]bool, len(indices))
	for _, idx := range indices {
		m[idx] = true
	}
	return m
}

// expandAggMap maps aggregated-timeframe indices back to base-timeframe index ranges
func expandAggMap(aggIndices []int, factor int, maxLen int) map[int]bool {
	m := make(map[int]bool)
	for _, aggIdx := range aggIndices {
		for j := 0; j < factor; j++ {
			baseIdx := aggIdx*factor + j
			if baseIdx < maxLen {
				m[baseIdx] = true
			}
		}
	}
	return m
}

// withinRange checks if any index within [center-radius, center+radius] exists in map
func withinRange(m map[int]bool, center int, radius int) bool {
	for i := center - radius; i <= center+radius; i++ {
		if m[i] {
			return true
		}
	}
	return false
}

// isNearZone checks if price is within tolerance of any zone's range
func isNearZone(price float64, zones [][2]float64, tolerance float64) bool {
	for _, zone := range zones {
		zoneMid := (zone[0] + zone[1]) / 2
		if math.Abs(price-zoneMid)/price <= tolerance {
			return true
		}
	}
	return false
}

// findRecentSwingLow finds the lowest low in the past N bars
func findRecentSwingLow(ohlcv []OHLCV, currentIdx int, lookback int) float64 {
	low := ohlcv[currentIdx].Low
	start := currentIdx - lookback
	if start < 0 {
		start = 0
	}
	for j := start; j < currentIdx; j++ {
		if ohlcv[j].Low < low {
			low = ohlcv[j].Low
		}
	}
	return low
}

// findRecentSwingHigh finds the highest high in the past N bars
func findRecentSwingHigh(ohlcv []OHLCV, currentIdx int, lookback int) float64 {
	high := ohlcv[currentIdx].High
	start := currentIdx - lookback
	if start < 0 {
		start = 0
	}
	for j := start; j < currentIdx; j++ {
		if ohlcv[j].High > high {
			high = ohlcv[j].High
		}
	}
	return high
}

// ========== Trading Arena Strategies & Backtest Engine ==========

type ChartMarker struct {
	Time     int64  `json:"time"`
	Position string `json:"position"`
	Color    string `json:"color"`
	Shape    string `json:"shape"`
	Text     string `json:"text"`
}

type StrategySignal struct {
	Index      int
	Direction  string // "LONG" | "SHORT"
	EntryPrice float64
	StopLoss   float64
	TakeProfit float64
	Shape      string // optional, default "" → arrowUp/Down
	Text       string // optional, default "" → "LONG"/"SHORT"
	Color      string // optional, default "" → standard colors
}

type IndicatorSeries struct {
	Name  string           `json:"name"`
	Type  string           `json:"type"` // "histogram", "line"
	Color string           `json:"color"`
	Data  []IndicatorPoint `json:"data"`
}

type IndicatorPoint struct {
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
	Color string  `json:"color,omitempty"`
}

type IndicatorProvider interface {
	ComputeIndicators(ohlcv []OHLCV) []IndicatorSeries
}

type TradingStrategy interface {
	Name() string
	RequiredBars() int
	Analyze(ohlcv []OHLCV) []StrategySignal
}

// --- Strategy A: Moving Regression Scalping ---
type RegressionScalpingStrategy struct {
	Degree               int     `json:"degree"`
	Length               int     `json:"length"`
	Multiplier           float64 `json:"multiplier"`
	RiskReward           float64 `json:"risk_reward"`
	SLLookback           int     `json:"sl_lookback"`
	ConfirmationRequired int     `json:"confirmation_required"` // 1=on (default), 0=off
	// computation cache
	cachedUpper, cachedMiddle, cachedLower []float64
	cachedAO                               []float64
	cachedHA                               []OHLCV
	cachedLen                              int
}

func (s *RegressionScalpingStrategy) defaults() {
	if s.Degree <= 0 { s.Degree = 2 }
	if s.Length <= 0 { s.Length = 100 }
	if s.Multiplier <= 0 { s.Multiplier = 3.0 }
	if s.RiskReward <= 0 { s.RiskReward = 1.5 }
	if s.SLLookback <= 0 { s.SLLookback = 30 }
	if s.ConfirmationRequired < 0 { s.ConfirmationRequired = 1 }
}

func (s *RegressionScalpingStrategy) Name() string      { return "regression_scalping" }
func (s *RegressionScalpingStrategy) RequiredBars() int  { s.defaults(); return s.Length + 20 }

func (s *RegressionScalpingStrategy) compute(ohlcv []OHLCV) (upper, middle, lower, ao []float64, ha []OHLCV) {
	if s.cachedLen == len(ohlcv) {
		return s.cachedUpper, s.cachedMiddle, s.cachedLower, s.cachedAO, s.cachedHA
	}
	closes := extractCloses(ohlcv)
	upper, middle, lower = calculatePolyRegressionBands(closes, s.Degree, s.Length, s.Multiplier)
	ao = calculateAwesomeOscillator(ohlcv)
	ha = calculateHeikinAshi(ohlcv)
	s.cachedUpper, s.cachedMiddle, s.cachedLower, s.cachedAO, s.cachedHA = upper, middle, lower, ao, ha
	s.cachedLen = len(ohlcv)
	return
}

func (s *RegressionScalpingStrategy) Analyze(ohlcv []OHLCV) []StrategySignal {
	s.defaults()
	var signals []StrategySignal
	minBars := s.Length + 20
	if len(ohlcv) < minBars {
		return signals
	}

	upper, _, lower, ao, ha := s.compute(ohlcv)
	needConfirm := s.ConfirmationRequired == 1

	// Three-step confirmation: Setup → AO flip → Candle color
	type setupState struct {
		active    bool
		dir       string
		confirmed bool
	}
	setup := setupState{}

	for i := 35; i < len(ohlcv); i++ {
		// Reset setup if price returns between bands (without confirmation)
		if ohlcv[i].Close > lower[i] && ohlcv[i].Close < upper[i] && !setup.confirmed {
			setup = setupState{}
		}

		// Step A: LONG setup — price closes under Lower Band
		if !setup.active && ohlcv[i].Close < lower[i] && lower[i] > 0 {
			setup = setupState{active: true, dir: "LONG", confirmed: !needConfirm}
			if needConfirm { continue }
		}
		// Step A: SHORT setup — price closes above Upper Band
		if !setup.active && ohlcv[i].Close > upper[i] && upper[i] > 0 {
			setup = setupState{active: true, dir: "SHORT", confirmed: !needConfirm}
			if needConfirm { continue }
		}

		if !setup.active {
			continue
		}

		// Step B: AO color flip (Red→Green for Long, Green→Red for Short)
		if needConfirm && !setup.confirmed && i > 1 {
			aoRising := ao[i] > ao[i-1]   // Green bar (rising)
			aoFalling := ao[i] < ao[i-1]  // Red bar (falling)
			aoPrevFalling := ao[i-1] <= ao[i-2] // Previous was red
			aoPrevRising := ao[i-1] >= ao[i-2]  // Previous was green

			if setup.dir == "LONG" && aoRising && aoPrevFalling {
				setup.confirmed = true
			} else if setup.dir == "SHORT" && aoFalling && aoPrevRising {
				setup.confirmed = true
			}
		}

		if !setup.confirmed {
			continue
		}

		// Step C: Heikin Ashi candle color confirms direction (HA Close > HA Open = bullish)
		candleBullish := ha[i].Close > ha[i].Open
		candleBearish := ha[i].Close < ha[i].Open
		if needConfirm && setup.dir == "LONG" && !candleBullish { continue }
		if needConfirm && setup.dir == "SHORT" && !candleBearish { continue }

		if setup.dir == "LONG" {
			if i+1 >= len(ohlcv) {
				setup = setupState{}
				continue
			}
			entryPrice := ohlcv[i+1].Open
			swingLow := ohlcv[i].Low
			lookStart := i - s.SLLookback
			if lookStart < 0 { lookStart = 0 }
			for j := lookStart; j < i; j++ {
				if ohlcv[j].Low < swingLow { swingLow = ohlcv[j].Low }
			}
			risk := entryPrice - swingLow
			if risk > 0 {
				signals = append(signals, StrategySignal{
					Index: i + 1, Direction: "LONG", EntryPrice: entryPrice,
					StopLoss: swingLow, TakeProfit: entryPrice + s.RiskReward*risk,
				})
			}
			setup = setupState{}
		} else if setup.dir == "SHORT" {
			if i+1 >= len(ohlcv) {
				setup = setupState{}
				continue
			}
			entryPrice := ohlcv[i+1].Open
			swingHigh := ohlcv[i].High
			lookStart := i - s.SLLookback
			if lookStart < 0 { lookStart = 0 }
			for j := lookStart; j < i; j++ {
				if ohlcv[j].High > swingHigh { swingHigh = ohlcv[j].High }
			}
			risk := swingHigh - entryPrice
			if risk > 0 {
				signals = append(signals, StrategySignal{
					Index: i + 1, Direction: "SHORT", EntryPrice: entryPrice,
					StopLoss: swingHigh, TakeProfit: entryPrice - s.RiskReward*risk,
				})
			}
			setup = setupState{}
		}
	}
	return signals
}

// --- Strategy B: NW Bollinger Bands (Flux Charts LUoxSDKw) ---
type HybridAITrendStrategy struct {
	BB1Period   int     `json:"bb1_period"`    // Level 1 (inner, signal line) — default 20
	BB1Stdev    float64 `json:"bb1_stdev"`     // default 3.0
	BB2Period   int     `json:"bb2_period"`    // Level 2 — default 75
	BB2Stdev    float64 `json:"bb2_stdev"`     // default 3.0 (short_stdev in Pine)
	BB3Period   int     `json:"bb3_period"`    // Level 3 — default 100
	BB3Stdev    float64 `json:"bb3_stdev"`     // default 4.0
	BB4Period   int     `json:"bb4_period"`    // Level 4 (outermost) — default 100
	BB4Stdev    float64 `json:"bb4_stdev"`     // default 4.25
	NWBandwidth float64 `json:"nw_bandwidth"`  // Nadaraya-Watson smoothing factor h — default 6.0
	NWLookback  int     `json:"nw_lookback"`   // NW kernel lookback — default 499
	SLBuffer    float64 `json:"sl_buffer"`     // Stop Loss buffer % — default 1.5
	RiskReward  float64 `json:"risk_reward"`   // Risk/Reward ratio — default 2.0
	HybridFilter      bool    `json:"hybrid_filter"`       // filter signals by Hybrid EMA AlgoLearner
	HybridLongThresh  float64 `json:"hybrid_long_thresh"`  // LONG only when oscillator >= this — default 75
	HybridShortThresh float64 `json:"hybrid_short_thresh"` // SHORT only when oscillator <= this — default 25
	ConfirmCandle     bool    `json:"confirm_candle"`      // require bullish/bearish confirmation candle before entry
	MinBandDist       float64 `json:"min_band_dist"`       // min % distance below/above band for entry (0 = disabled)
}

func (s *HybridAITrendStrategy) defaults() {
	if s.BB1Period <= 0 { s.BB1Period = 20 }
	if s.BB1Stdev <= 0 { s.BB1Stdev = 3.0 }
	if s.BB2Period <= 0 { s.BB2Period = 75 }
	if s.BB2Stdev <= 0 { s.BB2Stdev = 3.0 }
	if s.BB3Period <= 0 { s.BB3Period = 100 }
	if s.BB3Stdev <= 0 { s.BB3Stdev = 4.0 }
	if s.BB4Period <= 0 { s.BB4Period = 100 }
	if s.BB4Stdev <= 0 { s.BB4Stdev = 4.25 }
	if s.NWBandwidth <= 0 { s.NWBandwidth = 6.0 }
	if s.NWLookback <= 0 { s.NWLookback = 499 }
	if s.SLBuffer < 0 { s.SLBuffer = 1.5 }
	if s.RiskReward <= 0 { s.RiskReward = 2.0 }
	if s.HybridLongThresh <= 0 { s.HybridLongThresh = 75.0 }
	if s.HybridShortThresh <= 0 { s.HybridShortThresh = 25.0 }
}

func (s *HybridAITrendStrategy) Name() string     { return "hybrid_ai_trend" }
func (s *HybridAITrendStrategy) RequiredBars() int { s.defaults(); return s.NWLookback + 100 }

func (s *HybridAITrendStrategy) Analyze(ohlcv []OHLCV) []StrategySignal {
	s.defaults()
	var signals []StrategySignal
	if len(ohlcv) < s.BB1Period+2 {
		return signals
	}

	// Compute Level 1 NW-smoothed Bollinger Bands (signal line)
	upper1, lower1 := calculateSingleBBLevel(ohlcv, s.BB1Period, s.BB1Stdev, s.NWBandwidth, s.NWLookback)
	closes := extractCloses(ohlcv)

	// Optional: Hybrid EMA AlgoLearner filter
	var oscillator []float64
	if s.HybridFilter {
		oscillator = calculateHybridEMA(closes, 50, 200, 5, 100, 400)
	}

	for i := 1; i < len(ohlcv); i++ {
		if upper1[i] == 0 || lower1[i] == 0 || upper1[i-1] == 0 || lower1[i-1] == 0 {
			continue
		}

		// BUY (LONG): close crosses below lower band 1
		// Signal fires at bar i close → entry at bar i+1 open (no look-ahead bias)
		if closes[i] <= lower1[i] && closes[i-1] > lower1[i-1] {
			if s.HybridFilter && (i >= len(oscillator) || oscillator[i] < s.HybridLongThresh) {
				continue // oscillator below long threshold → skip
			}
			// Min band distance filter: close must be at least X% below lower band
			if s.MinBandDist > 0 {
				dist := (lower1[i] - closes[i]) / lower1[i] * 100
				if dist < s.MinBandDist {
					continue
				}
			}
			// Confirmation candle: wait for next candle to be bullish (Close > Open)
			entryIdx := i + 1
			if s.ConfirmCandle {
				if i+1 >= len(ohlcv) {
					continue
				}
				if ohlcv[i+1].Close <= ohlcv[i+1].Open {
					continue // next candle not bullish → skip
				}
				entryIdx = i + 2 // enter at bar after the confirmation candle
			} else {
				entryIdx = i + 1
			}
			if entryIdx >= len(ohlcv) {
				continue
			}
			entryPrice := ohlcv[entryIdx].Open
			slPrice := entryPrice * (1 - s.SLBuffer/100)
			risk := entryPrice - slPrice
			signals = append(signals, StrategySignal{
				Index: i + 1, Direction: "LONG", EntryPrice: entryPrice,
				StopLoss: slPrice, TakeProfit: entryPrice + s.RiskReward*risk,
			})
		}

		// SELL (SHORT): close crosses above upper band 1
		// Signal fires at bar i close → entry at bar i+1 open (no look-ahead bias)
		if closes[i] >= upper1[i] && closes[i-1] < upper1[i-1] {
			if s.HybridFilter && (i >= len(oscillator) || oscillator[i] > s.HybridShortThresh) {
				continue // oscillator above short threshold → skip
			}
			// Min band distance filter: close must be at least X% above upper band
			if s.MinBandDist > 0 {
				dist := (closes[i] - upper1[i]) / upper1[i] * 100
				if dist < s.MinBandDist {
					continue
				}
			}
			// Confirmation candle: wait for next candle to be bearish (Close < Open)
			entryIdx := i + 1
			if s.ConfirmCandle {
				if i+1 >= len(ohlcv) {
					continue
				}
				if ohlcv[i+1].Close >= ohlcv[i+1].Open {
					continue // next candle not bearish → skip
				}
				entryIdx = i + 2
			} else {
				entryIdx = i + 1
			}
			if entryIdx >= len(ohlcv) {
				continue
			}
			entryPrice := ohlcv[entryIdx].Open
			slPrice := entryPrice * (1 + s.SLBuffer/100)
			risk := slPrice - entryPrice
			signals = append(signals, StrategySignal{
				Index: i + 1, Direction: "SHORT", EntryPrice: entryPrice,
				StopLoss: slPrice, TakeProfit: entryPrice - s.RiskReward*risk,
			})
		}
	}
	return signals
}

// --- Strategy C: Diamond Signals ---
type DiamondSignalsStrategy struct {
	PatternLength int     `json:"pattern_length"`
	RSIPeriod     int     `json:"rsi_period"`
	ConfluenceMin int     `json:"confluence_min"`
	RSIOverbought float64 `json:"rsi_overbought"`
	RSIOversold   float64 `json:"rsi_oversold"`
	Cooldown      int     `json:"cooldown"`
	RiskReward    float64 `json:"risk_reward"`
}

func (s *DiamondSignalsStrategy) defaults() {
	if s.PatternLength <= 0 { s.PatternLength = 20 }
	if s.RSIPeriod <= 0 { s.RSIPeriod = 14 }
	if s.ConfluenceMin <= 0 { s.ConfluenceMin = 3 }
	if s.RSIOverbought <= 0 { s.RSIOverbought = 65 }
	if s.RSIOversold <= 0 { s.RSIOversold = 35 }
	if s.Cooldown <= 0 { s.Cooldown = 5 }
	if s.RiskReward <= 0 { s.RiskReward = 2.0 }
}

func (s *DiamondSignalsStrategy) Name() string     { return "diamond_signals" }
func (s *DiamondSignalsStrategy) RequiredBars() int { return 200 }

func (s *DiamondSignalsStrategy) Analyze(ohlcv []OHLCV) []StrategySignal {
	s.defaults()
	var signals []StrategySignal
	if len(ohlcv) < 200 {
		return signals
	}

	closes := extractCloses(ohlcv)
	rsi := calculateRSIServer(closes, s.RSIPeriod)

	bullDiamond, bearDiamond := detectDiamondPattern(ohlcv, s.PatternLength)
	rsiBullDiv, rsiBearDiv := detectRSIDivergence(closes, s.RSIPeriod, 30)
	volBullDiv, volBearDiv := detectVolumeDivergence(ohlcv, 30)
	demandZones, supplyZones := detectOrderBlocks(ohlcv, 50)

	aggOHLCV := aggregateOHLCV(ohlcv, 4)
	var aggBullDiamondBase, aggBearDiamondBase map[int]bool
	var aggRSIBullBase, aggRSIBearBase map[int]bool
	if len(aggOHLCV) >= 50 {
		aggBullD, aggBearD := detectDiamondPattern(aggOHLCV, s.PatternLength)
		aggCloses := extractCloses(aggOHLCV)
		aggRSIBull, aggRSIBear := detectRSIDivergence(aggCloses, s.RSIPeriod, 15)
		aggBullDiamondBase = expandAggMap(aggBullD, 4, len(ohlcv))
		aggBearDiamondBase = expandAggMap(aggBearD, 4, len(ohlcv))
		aggRSIBullBase = expandAggMap(aggRSIBull, 4, len(ohlcv))
		aggRSIBearBase = expandAggMap(aggRSIBear, 4, len(ohlcv))
	} else {
		aggBullDiamondBase = make(map[int]bool)
		aggBearDiamondBase = make(map[int]bool)
		aggRSIBullBase = make(map[int]bool)
		aggRSIBearBase = make(map[int]bool)
	}

	bullDiamondMap := toIndexMap(bullDiamond)
	bearDiamondMap := toIndexMap(bearDiamond)
	rsiBullMap := toIndexMap(rsiBullDiv)
	rsiBearMap := toIndexMap(rsiBearDiv)
	volBullMap := toIndexMap(volBullDiv)
	volBearMap := toIndexMap(volBearDiv)

	cooldown := 0

	for i := 50; i < len(ohlcv); i++ {
		if cooldown > 0 {
			cooldown--
			continue
		}

		longScore := 0
		if withinRange(bullDiamondMap, i, 3) { longScore++ }
		if withinRange(rsiBullMap, i, 3) { longScore++ }
		if withinRange(volBullMap, i, 3) { longScore++ }
		if isNearZone(closes[i], demandZones, 0.02) { longScore++ }
		if aggBullDiamondBase[i] || aggRSIBullBase[i] { longScore++ }

		shortScore := 0
		if withinRange(bearDiamondMap, i, 3) { shortScore++ }
		if withinRange(rsiBearMap, i, 3) { shortScore++ }
		if withinRange(volBearMap, i, 3) { shortScore++ }
		if isNearZone(closes[i], supplyZones, 0.02) { shortScore++ }
		if aggBearDiamondBase[i] || aggRSIBearBase[i] { shortScore++ }

		if longScore >= s.ConfluenceMin && rsi[i] < s.RSIOverbought {
			if i+1 >= len(ohlcv) {
				continue
			}
			entryPrice := ohlcv[i+1].Open
			swingLow := findRecentSwingLow(ohlcv, i, 20)
			risk := entryPrice - swingLow
			if risk > 0 && risk/entryPrice > 0.002 {
				signals = append(signals, StrategySignal{
					Index: i + 1, Direction: "LONG", EntryPrice: entryPrice,
					StopLoss: swingLow, TakeProfit: entryPrice + s.RiskReward*risk,
					Shape: "square", Text: "◆ LONG", Color: "#3b82f6",
				})
				cooldown = s.Cooldown
			}
		}

		if shortScore >= s.ConfluenceMin && rsi[i] > s.RSIOversold {
			if i+1 >= len(ohlcv) {
				continue
			}
			entryPrice := ohlcv[i+1].Open
			swingHigh := findRecentSwingHigh(ohlcv, i, 20)
			risk := swingHigh - entryPrice
			if risk > 0 && risk/entryPrice > 0.002 {
				signals = append(signals, StrategySignal{
					Index: i + 1, Direction: "SHORT", EntryPrice: entryPrice,
					StopLoss: swingHigh, TakeProfit: entryPrice - s.RiskReward*risk,
					Shape: "square", Text: "◆ SHORT", Color: "#ec4899",
				})
				cooldown = s.Cooldown
			}
		}
	}
	return signals
}

// --- Strategy D: Smart Money Flow Cloud [BOSWaves] ---
// Indicator: Volume-weighted flow → adaptive bands → regime detection
// Strategy: Regime change → swing point → pullback retest → structure break entry

type SmartMoneyFlowStrategy struct {
	TrendLength   int     `json:"trend_length"`   // EMA baseline length (Pine: len=34)
	BasisSmooth   int     `json:"basis_smooth"`   // Extra EMA smoothing on baseline (Pine: basisSmooth=3)
	FlowWindow    int     `json:"flow_window"`    // Money flow lookback (Pine: mfLen=24)
	FlowSmooth    int     `json:"flow_smooth"`    // EMA smoothing on flow (Pine: mfSmooth=5)
	FlowBoost     float64 `json:"flow_boost"`     // Nonlinear flow power (Pine: mfPower=1.2)
	ATRLength     int     `json:"atr_length"`     // ATR period for bands (Pine: atrLen=14)
	BandTightness float64 `json:"band_tightness"` // Min band multiplier (Pine: minMult=0.9)
	BandExpansion float64 `json:"band_expansion"` // Max band multiplier (Pine: maxMult=2.2)
	DotCooldown   int     `json:"dot_cooldown"`   // Retest cooldown bars (Pine: dotCooldown=12)
	RiskReward    float64 `json:"risk_reward"`
	cachedData    *smfData // cached compute() result per request
	cachedLen     int      // len(ohlcv) for cache invalidation
}

func (s *SmartMoneyFlowStrategy) defaults() {
	if s.TrendLength <= 0 { s.TrendLength = 34 }
	if s.BasisSmooth <= 0 { s.BasisSmooth = 3 }
	if s.FlowWindow <= 0 { s.FlowWindow = 24 }
	if s.FlowSmooth <= 0 { s.FlowSmooth = 5 }
	if s.FlowBoost <= 0 { s.FlowBoost = 1.2 }
	if s.ATRLength <= 0 { s.ATRLength = 14 }
	if s.BandTightness <= 0 { s.BandTightness = 0.9 }
	if s.BandExpansion <= 0 { s.BandExpansion = 2.2 }
	if s.DotCooldown < 0 { s.DotCooldown = 12 }
	if s.RiskReward <= 0 { s.RiskReward = 2.0 }
}

func (s *SmartMoneyFlowStrategy) Name() string     { return "smart_money_flow" }
func (s *SmartMoneyFlowStrategy) RequiredBars() int { s.defaults(); return s.TrendLength + s.FlowWindow + 50 }

type smfData struct {
	basisOpen  []float64
	basisClose []float64
	upper      []float64
	lower      []float64
	regime     []int
	strength   []float64
	mfSm       []float64
	atr        []float64
}

func smfEMA(data []float64, period int) []float64 {
	n := len(data)
	result := make([]float64, n)
	if n == 0 || period <= 1 {
		copy(result, data)
		return result
	}
	k := 2.0 / float64(period+1)
	result[0] = data[0]
	for i := 1; i < n; i++ {
		result[i] = data[i]*k + result[i-1]*(1-k)
	}
	return result
}

func (s *SmartMoneyFlowStrategy) compute(ohlcv []OHLCV) *smfData {
	s.defaults()
	n := len(ohlcv)

	// Return cached result if available (Analyze, Overlays, Indicators share same data)
	if s.cachedData != nil && s.cachedLen == n {
		return s.cachedData
	}

	opens := make([]float64, n)
	closes := make([]float64, n)
	for i, bar := range ohlcv {
		opens[i] = bar.Open
		closes[i] = bar.Close
	}

	// Basis: EMA(EMA(x, trendLength), basisSmooth)
	bO := smfEMA(smfEMA(opens, s.TrendLength), s.BasisSmooth)
	bC := smfEMA(smfEMA(closes, s.TrendLength), s.BasisSmooth)

	// CLV: ((close-low) - (high-close)) / (high-low)
	clv := make([]float64, n)
	for i, bar := range ohlcv {
		if bar.High == bar.Low {
			clv[i] = 0
		} else {
			clv[i] = ((bar.Close - bar.Low) - (bar.High - bar.Close)) / (bar.High - bar.Low)
		}
	}

	// Money flow: sliding window O(n) instead of naive O(n*FlowWindow)
	raw := make([]float64, n)
	rawAbs := make([]float64, n)
	for i := range ohlcv {
		raw[i] = clv[i] * ohlcv[i].Volume
		rawAbs[i] = math.Abs(raw[i])
	}
	mfRatio := make([]float64, n)
	num, den := 0.0, 0.0
	for i := 0; i < n; i++ {
		num += raw[i]
		den += rawAbs[i]
		if i >= s.FlowWindow {
			num -= raw[i-s.FlowWindow]
			den -= rawAbs[i-s.FlowWindow]
		}
		if den == 0 {
			mfRatio[i] = 0
		} else {
			mfRatio[i] = num / den
		}
	}

	mfSm := smfEMA(mfRatio, s.FlowSmooth)

	// strength = clamp(pow(|mfSm|, flowBoost), 0, 1)
	strength := make([]float64, n)
	mult := make([]float64, n)
	for i := range mfSm {
		str := math.Pow(math.Abs(mfSm[i]), s.FlowBoost)
		if str > 1 { str = 1 }
		strength[i] = str
		mult[i] = s.BandTightness + (s.BandExpansion-s.BandTightness)*str
	}

	// ATR (Wilder's RMA: alpha = 1/period)
	tr := make([]float64, n)
	tr[0] = ohlcv[0].High - ohlcv[0].Low
	for i := 1; i < n; i++ {
		hl := ohlcv[i].High - ohlcv[i].Low
		hc := math.Abs(ohlcv[i].High - ohlcv[i-1].Close)
		lc := math.Abs(ohlcv[i].Low - ohlcv[i-1].Close)
		tr[i] = math.Max(hl, math.Max(hc, lc))
	}
	al := 1.0 / float64(s.ATRLength)
	atr := make([]float64, n)
	atr[0] = tr[0]
	for i := 1; i < n; i++ {
		atr[i] = al*tr[i] + (1-al)*atr[i-1]
	}

	// Adaptive bands
	upper := make([]float64, n)
	lower := make([]float64, n)
	for i := range ohlcv {
		upper[i] = bC[i] + atr[i]*mult[i]
		lower[i] = bC[i] - atr[i]*mult[i]
	}

	// Regime: crossover(close, upper) → bull, crossunder(close, lower) → bear
	regime := make([]int, n)
	if closes[0] >= bC[0] { regime[0] = 1 } else { regime[0] = -1 }
	for i := 1; i < n; i++ {
		if closes[i] > upper[i] && closes[i-1] <= upper[i-1] {
			regime[i] = 1
		} else if closes[i] < lower[i] && closes[i-1] >= lower[i-1] {
			regime[i] = -1
		} else {
			regime[i] = regime[i-1]
		}
	}

	result := &smfData{basisOpen: bO, basisClose: bC, upper: upper, lower: lower, regime: regime, strength: strength, mfSm: mfSm, atr: atr}
	s.cachedData = result
	s.cachedLen = n
	return result
}

func (s *SmartMoneyFlowStrategy) Analyze(ohlcv []OHLCV) []StrategySignal {
	s.defaults()
	n := len(ohlcv)
	if n < s.RequiredBars() { return nil }

	d := s.compute(ohlcv)
	var signals []StrategySignal

	// State machine phases
	const (
		phWait     = 0 // waiting for regime change
		phTracking = 1 // tracking swing point, waiting for retest
		phArmed    = 2 // retest happened, waiting for structure break
	)
	phase := phWait
	dir := 0        // 1=bull, -1=bear
	swingRef := 0.0 // swing high (bull) or swing low (bear)
	pbExt := 0.0    // pullback low (bull SL) or pullback high (bear SL)

	warmup := s.TrendLength + s.BasisSmooth + s.FlowWindow
	if warmup >= n-1 { return nil }

	for i := warmup; i < n-1; i++ {
		// Regime change → start tracking in new direction
		if i > 0 && d.regime[i] != d.regime[i-1] {
			phase = phTracking
			dir = d.regime[i]
			if dir == 1 {
				swingRef = ohlcv[i].High
			} else {
				swingRef = ohlcv[i].Low
			}
			pbExt = 0
			continue
		}

		if phase == phWait { continue }

		if dir == 1 { // BULL regime → Long setups
			if phase == phTracking {
				if ohlcv[i].High > swingRef { swingRef = ohlcv[i].High }
				// Retest: price dips below baseline
				if ohlcv[i].Low < d.basisClose[i] {
					phase = phArmed
					pbExt = ohlcv[i].Low
				}
			} else if phase == phArmed {
				if ohlcv[i].Low < pbExt { pbExt = ohlcv[i].Low }
				// Structure break: close above swing high
				if ohlcv[i].Close > swingRef {
					entry := ohlcv[i+1].Open
					risk := entry - pbExt
					if risk > 0 && risk/entry > 0.001 {
						signals = append(signals, StrategySignal{
							Index: i + 1, Direction: "LONG", EntryPrice: entry,
							StopLoss: pbExt, TakeProfit: entry + s.RiskReward*risk,
						})
					}
					phase = phTracking
					swingRef = ohlcv[i].High
					pbExt = 0
				}
			}
		} else { // BEAR regime → Short setups
			if phase == phTracking {
				if ohlcv[i].Low < swingRef { swingRef = ohlcv[i].Low }
				// Retest: price rises above baseline
				if ohlcv[i].High > d.basisClose[i] {
					phase = phArmed
					pbExt = ohlcv[i].High
				}
			} else if phase == phArmed {
				if ohlcv[i].High > pbExt { pbExt = ohlcv[i].High }
				// Structure break: close below swing low
				if ohlcv[i].Close < swingRef {
					entry := ohlcv[i+1].Open
					risk := pbExt - entry
					if risk > 0 && risk/entry > 0.001 {
						signals = append(signals, StrategySignal{
							Index: i + 1, Direction: "SHORT", EntryPrice: entry,
							StopLoss: pbExt, TakeProfit: entry - s.RiskReward*risk,
						})
					}
					phase = phTracking
					swingRef = ohlcv[i].Low
					pbExt = 0
				}
			}
		}
	}
	return signals
}

func (s *SmartMoneyFlowStrategy) ComputeOverlays(ohlcv []OHLCV) []OverlaySeries {
	s.defaults()
	d := s.compute(ohlcv)
	n := len(ohlcv)

	bOpenPts := make([]OverlayPoint, n)
	bClosePts := make([]OverlayPoint, n)
	upperPts := make([]OverlayPoint, n)
	lowerPts := make([]OverlayPoint, n)
	for i, bar := range ohlcv {
		bOpenPts[i] = OverlayPoint{Time: bar.Time, Value: d.basisOpen[i]}
		bClosePts[i] = OverlayPoint{Time: bar.Time, Value: d.basisClose[i]}
		upperPts[i] = OverlayPoint{Time: bar.Time, Value: d.upper[i]}
		lowerPts[i] = OverlayPoint{Time: bar.Time, Value: d.lower[i]}
	}

	return []OverlaySeries{
		{Name: "Basis Open", Type: "line", Color: "rgba(100,180,255,0.5)", Data: bOpenPts, Style: 0},
		{Name: "Basis Close", Type: "line", Color: "rgba(100,180,255,0.9)", Data: bClosePts, Style: 0, FillColor: "rgba(100,180,255,0.12)"},
		{Name: "Upper Band", Type: "line", Color: "rgba(255,255,255,0.25)", Data: upperPts, Style: 2},
		{Name: "Lower Band", Type: "line", Color: "rgba(255,255,255,0.25)", Data: lowerPts, Style: 2, InvertFill: true},
	}
}

func (s *SmartMoneyFlowStrategy) ComputeIndicators(ohlcv []OHLCV) []IndicatorSeries {
	s.defaults()
	d := s.compute(ohlcv)
	n := len(ohlcv)

	pts := make([]IndicatorPoint, n)
	for i, bar := range ohlcv {
		val := d.mfSm[i] * 100
		col := "#ef4444"
		if val >= 0 { col = "#22c55e" }
		pts[i] = IndicatorPoint{Time: bar.Time, Value: val, Color: col}
	}

	return []IndicatorSeries{
		{Name: "Smart Money Flow", Type: "histogram", Color: "#22c55e", Data: pts},
	}
}

// ========== Strategy: Hann Trend (DMH + Parabolic SAR) ==========

type HannTrendStrategy struct {
	DMHLength     int     `json:"dmh_length"`
	SARStart      float64 `json:"sar_start"`
	SARIncrement  float64 `json:"sar_increment"`
	SARMax        float64 `json:"sar_max"`
	SwingLookback int     `json:"swing_lookback"`
	RiskReward    float64 `json:"risk_reward"`
	SLBuffer      float64 `json:"sl_buffer"` // percent
	HybridFilter      bool    `json:"hybrid_filter"`
	HybridLongThresh  float64 `json:"hybrid_long_thresh"`
	HybridShortThresh float64 `json:"hybrid_short_thresh"`
}

func (s *HannTrendStrategy) defaults() {
	if s.DMHLength <= 0 { s.DMHLength = 30 }
	if s.SARStart <= 0 { s.SARStart = 0.02 }
	if s.SARIncrement <= 0 { s.SARIncrement = 0.03 }
	if s.SARMax <= 0 { s.SARMax = 0.3 }
	if s.SwingLookback <= 0 { s.SwingLookback = 5 }
	if s.RiskReward <= 0 { s.RiskReward = 2.0 }
	if s.SLBuffer <= 0 { s.SLBuffer = 0.3 }
	if s.HybridLongThresh <= 0 { s.HybridLongThresh = 75 }
	if s.HybridShortThresh <= 0 { s.HybridShortThresh = 25 }
}

func (s *HannTrendStrategy) Name() string     { return "hann_trend" }
func (s *HannTrendStrategy) RequiredBars() int { s.defaults(); return s.DMHLength*2 + 10 }

// hannRMA computes Wilder's RMA (alpha = 1/period)
func hannRMA(src []float64, period int) []float64 {
	n := len(src)
	result := make([]float64, n)
	if n == 0 { return result }
	alpha := 1.0 / float64(period)
	result[0] = src[0]
	for i := 1; i < n; i++ {
		result[i] = alpha*src[i] + (1-alpha)*result[i-1]
	}
	return result
}

// hannFilter applies a Hann FIR filter (TASC 2021.12 Ehlers)
func hannFilter(src []float64, period int) []float64 {
	n := len(src)
	result := make([]float64, n)
	pix2 := 2.0 * math.Pi / float64(period+1)
	for i := 0; i < n; i++ {
		sumH, sumC := 0.0, 0.0
		for count := 1; count <= period; count++ {
			idx := i - (count - 1)
			if idx < 0 { break }
			coef := 1.0 - math.Cos(float64(count)*pix2)
			sumH += coef * src[idx]
			sumC += coef
		}
		if sumC != 0 {
			result[i] = sumH / sumC
		}
	}
	return result
}

// computeDMH computes the Directional Movement with Hann filter
func computeDMH(ohlcv []OHLCV, period int) []float64 {
	n := len(ohlcv)
	netDM := make([]float64, n)
	for i := 1; i < n; i++ {
		upMove := ohlcv[i].High - ohlcv[i-1].High
		dnMove := ohlcv[i-1].Low - ohlcv[i].Low
		pDM, mDM := 0.0, 0.0
		if upMove > dnMove && upMove > 0 { pDM = upMove }
		if dnMove > upMove && dnMove > 0 { mDM = dnMove }
		netDM[i] = pDM - mDM
	}
	return hannFilter(hannRMA(netDM, period), period)
}

type sarPoint struct {
	Value float64
	IsUp  bool // true = SAR below price (bullish)
}

// computeParabolicSAR computes Wilder's Parabolic SAR
func computeParabolicSAR(ohlcv []OHLCV, start, increment, maximum float64) []sarPoint {
	n := len(ohlcv)
	if n == 0 { return nil }
	sar := make([]sarPoint, n)

	isUp := true
	sarVal := ohlcv[0].Low
	ep := ohlcv[0].High
	af := start
	sar[0] = sarPoint{Value: sarVal, IsUp: true}

	for i := 1; i < n; i++ {
		sarVal = sarVal + af*(ep-sarVal)

		if isUp {
			if sarVal > ohlcv[i-1].Low { sarVal = ohlcv[i-1].Low }
			if i >= 2 && sarVal > ohlcv[i-2].Low { sarVal = ohlcv[i-2].Low }

			if ohlcv[i].Low < sarVal {
				isUp = false
				sarVal = ep
				ep = ohlcv[i].Low
				af = start
			} else {
				if ohlcv[i].High > ep {
					ep = ohlcv[i].High
					af += increment
					if af > maximum { af = maximum }
				}
			}
		} else {
			if sarVal < ohlcv[i-1].High { sarVal = ohlcv[i-1].High }
			if i >= 2 && sarVal < ohlcv[i-2].High { sarVal = ohlcv[i-2].High }

			if ohlcv[i].High > sarVal {
				isUp = true
				sarVal = ep
				ep = ohlcv[i].High
				af = start
			} else {
				if ohlcv[i].Low < ep {
					ep = ohlcv[i].Low
					af += increment
					if af > maximum { af = maximum }
				}
			}
		}
		sar[i] = sarPoint{Value: sarVal, IsUp: isUp}
	}
	return sar
}

func (s *HannTrendStrategy) Analyze(ohlcv []OHLCV) []StrategySignal {
	s.defaults()
	n := len(ohlcv)
	if n < s.RequiredBars() { return nil }

	dmh := computeDMH(ohlcv, s.DMHLength)
	sar := computeParabolicSAR(ohlcv, s.SARStart, s.SARIncrement, s.SARMax)

	var oscillator []float64
	if s.HybridFilter {
		closes := extractCloses(ohlcv)
		oscillator = calculateHybridEMA(closes, 50, 200, 5, 100, 400)
	}

	var signals []StrategySignal

	const (
		phWait         = 0
		phWaitPullback = 1
		phInPullback   = 2
		phWaitConfirm  = 3
	)

	phase := phWait
	bullish := false
	pullbackAllowed := false
	swingHigh := 0.0
	swingLow := math.MaxFloat64
	pullbackExtreme := 0.0

	warmup := s.DMHLength + 1
	if warmup >= n-1 { return nil }

	for i := warmup; i < n-1; i++ {
		// DMH zero-line cross detection
		if dmh[i-1] <= 0 && dmh[i] > 0 {
			bullish = true
			pullbackAllowed = true
			phase = phWaitPullback
			swingHigh = ohlcv[i].High
			swingLow = ohlcv[i].Low
			pullbackExtreme = 0
		} else if dmh[i-1] >= 0 && dmh[i] < 0 {
			bullish = false
			pullbackAllowed = true
			phase = phWaitPullback
			swingHigh = ohlcv[i].High
			swingLow = ohlcv[i].Low
			pullbackExtreme = 0
		}

		if !pullbackAllowed || phase == phWait { continue }

		// Track running swing high/low only before pullback starts
		if phase == phWaitPullback {
			if ohlcv[i].High > swingHigh { swingHigh = ohlcv[i].High }
			if ohlcv[i].Low < swingLow { swingLow = ohlcv[i].Low }
		}

		sarFlipped := sar[i].IsUp != sar[i-1].IsUp

		if bullish {
			switch phase {
			case phWaitPullback:
				if sarFlipped && !sar[i].IsUp {
					phase = phInPullback
					pullbackExtreme = ohlcv[i].Low
					// Freeze swing high from lookback
					for j := i - s.SwingLookback; j < i; j++ {
						if j >= 0 && ohlcv[j].High > swingHigh { swingHigh = ohlcv[j].High }
					}
				}
			case phInPullback:
				if ohlcv[i].Low < pullbackExtreme { pullbackExtreme = ohlcv[i].Low }
				if sarFlipped && sar[i].IsUp {
					phase = phWaitConfirm
				}
			case phWaitConfirm:
				if ohlcv[i].Close > swingHigh {
					if s.HybridFilter && (i >= len(oscillator) || oscillator[i] < s.HybridLongThresh) {
						break
					}
					entry := ohlcv[i+1].Open
					sl := pullbackExtreme * (1 - s.SLBuffer/100)
					risk := entry - sl
					if risk > 0 && risk/entry > 0.001 {
						signals = append(signals, StrategySignal{
							Index: i + 1, Direction: "LONG", EntryPrice: entry,
							StopLoss: sl, TakeProfit: entry + risk*s.RiskReward,
						})
					}
					pullbackAllowed = false
					phase = phWait
				}
			}
		} else {
			switch phase {
			case phWaitPullback:
				if sarFlipped && sar[i].IsUp {
					phase = phInPullback
					pullbackExtreme = ohlcv[i].High
					for j := i - s.SwingLookback; j < i; j++ {
						if j >= 0 && ohlcv[j].Low < swingLow { swingLow = ohlcv[j].Low }
					}
				}
			case phInPullback:
				if ohlcv[i].High > pullbackExtreme { pullbackExtreme = ohlcv[i].High }
				if sarFlipped && !sar[i].IsUp {
					phase = phWaitConfirm
				}
			case phWaitConfirm:
				if ohlcv[i].Close < swingLow {
					if s.HybridFilter && (i >= len(oscillator) || oscillator[i] > s.HybridShortThresh) {
						break
					}
					entry := ohlcv[i+1].Open
					sl := pullbackExtreme * (1 + s.SLBuffer/100)
					risk := sl - entry
					if risk > 0 && risk/entry > 0.001 {
						signals = append(signals, StrategySignal{
							Index: i + 1, Direction: "SHORT", EntryPrice: entry,
							StopLoss: sl, TakeProfit: entry - risk*s.RiskReward,
						})
					}
					pullbackAllowed = false
					phase = phWait
				}
			}
		}
	}
	return signals
}

func (s *HannTrendStrategy) ComputeIndicators(ohlcv []OHLCV) []IndicatorSeries {
	s.defaults()
	if len(ohlcv) < 2 {
		return nil
	}
	dmh := computeDMH(ohlcv, s.DMHLength)
	n := len(ohlcv)

	pts := make([]IndicatorPoint, n)
	for i, bar := range ohlcv {
		col := "#0055FF"
		if dmh[i] > 0 { col = "#FFCC00" }
		pts[i] = IndicatorPoint{Time: bar.Time, Value: dmh[i], Color: col}
	}
	return []IndicatorSeries{
		{Name: "DMH", Type: "histogram", Color: "#FFCC00", Data: pts},
	}
}

func (s *HannTrendStrategy) ComputeOverlays(ohlcv []OHLCV) []OverlaySeries {
	s.defaults()
	if len(ohlcv) < 2 {
		return nil
	}
	sar := computeParabolicSAR(ohlcv, s.SARStart, s.SARIncrement, s.SARMax)
	n := len(ohlcv)

	allPts := make([]OverlayPoint, n)
	for i, bar := range ohlcv {
		allPts[i] = OverlayPoint{Time: bar.Time, Value: sar[i].Value}
	}
	return []OverlaySeries{
		{Name: "Parabolic SAR", Type: "line", Color: "rgba(255,255,255,0.5)", Data: allPts, Style: 2},
	}
}

// ─── GMMA Pullback Strategy ───────────────────────────────────────────────────

type GMMAPullbackStrategy struct {
	SignalLen      int     `json:"signal_len"`
	SmoothLen      int     `json:"smooth_len"`
	FractalPeriods int     `json:"fractal_periods"`
	ZoneCount      int     `json:"zone_count"`
	RiskReward     float64 `json:"risk_reward"`
	SLLookback     int     `json:"sl_lookback"`
	SLBuffer       float64 `json:"sl_buffer"`
}

func (s *GMMAPullbackStrategy) defaults() {
	if s.SignalLen <= 0 {
		s.SignalLen = 9
	}
	if s.SmoothLen <= 0 {
		s.SmoothLen = 3
	}
	if s.FractalPeriods <= 0 {
		s.FractalPeriods = 5
	}
	if s.ZoneCount <= 0 {
		s.ZoneCount = 5
	}
	if s.RiskReward <= 0 {
		s.RiskReward = 2.0
	}
	if s.SLLookback <= 0 {
		s.SLLookback = 10
	}
	if s.SLBuffer <= 0 {
		s.SLBuffer = 0.3
	}
}

func (s *GMMAPullbackStrategy) Name() string      { return "gmma_pullback" }
func (s *GMMAPullbackStrategy) RequiredBars() int { s.defaults(); return 80 }

type gmmaResult struct {
	main   []float64
	signal []float64
}

func computeGMMAOscillator(closes []float64, smoothLen, signalLen int) gmmaResult {
	n := len(closes)
	fastPeriods := []int{3, 5, 8, 10, 12, 15}
	slowPeriods := []int{30, 35, 40, 45, 50, 60}

	// Compute fast & slow group sums
	fastSum := make([]float64, n)
	slowSum := make([]float64, n)
	for _, p := range fastPeriods {
		ema := calculateEMAServer(closes, p)
		for i := range ema {
			fastSum[i] += ema[i]
		}
	}
	for _, p := range slowPeriods {
		ema := calculateEMAServer(closes, p)
		for i := range ema {
			slowSum[i] += ema[i]
		}
	}

	// Oscillator: percent difference
	osc := make([]float64, n)
	for i := 0; i < n; i++ {
		if slowSum[i] != 0 {
			osc[i] = ((fastSum[i] - slowSum[i]) / slowSum[i]) * 100
		}
	}

	// SMA smoothing → main line
	mainLine := calculateSMAServer(osc, smoothLen)

	// EMA of oscillator → signal line
	sigLine := calculateEMAServer(osc, signalLen)

	return gmmaResult{main: mainLine, signal: sigLine}
}

type srZone struct {
	top    float64
	bottom float64
	time   int64
}

func computeSRZones(ohlcv []OHLCV, fractalPeriods, zoneCount int, zoneBreach float64) (supports []srZone, resistances []srZone) {
	n := len(ohlcv)
	if n < fractalPeriods*2+1 {
		return
	}

	for i := fractalPeriods; i < n-fractalPeriods; i++ {
		// Pivot High check
		isPivotHigh := true
		for j := 1; j <= fractalPeriods; j++ {
			if ohlcv[i].High <= ohlcv[i-j].High || ohlcv[i].High <= ohlcv[i+j].High {
				isPivotHigh = false
				break
			}
		}
		if isPivotHigh {
			body := ohlcv[i].Open
			if ohlcv[i].Close > body {
				body = ohlcv[i].Close
			}
			resistances = append(resistances, srZone{
				top:    ohlcv[i].High,
				bottom: body,
				time:   ohlcv[i].Time,
			})
		}

		// Pivot Low check
		isPivotLow := true
		for j := 1; j <= fractalPeriods; j++ {
			if ohlcv[i].Low >= ohlcv[i-j].Low || ohlcv[i].Low >= ohlcv[i+j].Low {
				isPivotLow = false
				break
			}
		}
		if isPivotLow {
			body := ohlcv[i].Open
			if ohlcv[i].Close < body {
				body = ohlcv[i].Close
			}
			supports = append(supports, srZone{
				top:    body,
				bottom: ohlcv[i].Low,
				time:   ohlcv[i].Time,
			})
		}
	}

	// Keep only last zoneCount per type
	if len(supports) > zoneCount {
		supports = supports[len(supports)-zoneCount:]
	}
	if len(resistances) > zoneCount {
		resistances = resistances[len(resistances)-zoneCount:]
	}
	return
}

func (s *GMMAPullbackStrategy) Analyze(ohlcv []OHLCV) []StrategySignal {
	s.defaults()
	var signals []StrategySignal
	n := len(ohlcv)
	if n < 80 {
		return signals
	}

	closes := make([]float64, n)
	for i, bar := range ohlcv {
		closes[i] = bar.Close
	}

	gmma := computeGMMAOscillator(closes, s.SmoothLen, s.SignalLen)
	zoneBreach := 0.5 // 0.5% breach threshold

	for i := 60; i < n-1; i++ {
		// Recompute active zones up to bar i (only consider pivots confirmed before bar i)
		maxPivotIdx := i - s.FractalPeriods
		if maxPivotIdx < s.FractalPeriods {
			continue
		}

		// Collect zones from confirmed pivots
		var activeSup, activeRes []srZone
		for pi := s.FractalPeriods; pi <= maxPivotIdx; pi++ {
			// Pivot High
			isPH := true
			for j := 1; j <= s.FractalPeriods; j++ {
				if ohlcv[pi].High <= ohlcv[pi-j].High || ohlcv[pi].High <= ohlcv[pi+j].High {
					isPH = false
					break
				}
			}
			if isPH {
				body := ohlcv[pi].Open
				if ohlcv[pi].Close > body {
					body = ohlcv[pi].Close
				}
				// Check if zone has been breached
				breached := false
				threshold := ohlcv[pi].High * (1 + zoneBreach/100)
				for bi := pi + s.FractalPeriods + 1; bi <= i; bi++ {
					if ohlcv[bi].Close > threshold {
						breached = true
						break
					}
				}
				if !breached {
					activeRes = append(activeRes, srZone{top: ohlcv[pi].High, bottom: body, time: ohlcv[pi].Time})
				}
			}

			// Pivot Low
			isPL := true
			for j := 1; j <= s.FractalPeriods; j++ {
				if ohlcv[pi].Low >= ohlcv[pi-j].Low || ohlcv[pi].Low >= ohlcv[pi+j].Low {
					isPL = false
					break
				}
			}
			if isPL {
				body := ohlcv[pi].Open
				if ohlcv[pi].Close < body {
					body = ohlcv[pi].Close
				}
				breached := false
				threshold := ohlcv[pi].Low * (1 - zoneBreach/100)
				for bi := pi + s.FractalPeriods + 1; bi <= i; bi++ {
					if ohlcv[bi].Close < threshold {
						breached = true
						break
					}
				}
				if !breached {
					activeSup = append(activeSup, srZone{top: body, bottom: ohlcv[pi].Low, time: ohlcv[pi].Time})
				}
			}
		}

		// Keep last zoneCount
		if len(activeSup) > s.ZoneCount {
			activeSup = activeSup[len(activeSup)-s.ZoneCount:]
		}
		if len(activeRes) > s.ZoneCount {
			activeRes = activeRes[len(activeRes)-s.ZoneCount:]
		}

		// GMMA crossover/crossunder check
		if i < 1 {
			continue
		}
		mainNow := gmma.main[i]
		mainPrev := gmma.main[i-1]
		sigNow := gmma.signal[i]
		sigPrev := gmma.signal[i-1]

		crossover := mainPrev <= sigPrev && mainNow > sigNow   // main crosses above signal
		crossunder := mainPrev >= sigPrev && mainNow < sigNow  // main crosses below signal

		cl := ohlcv[i].Close

		// LONG: Close in support zone + GMMA crossover + both > 0
		if crossover && mainNow > 0 && sigNow > 0 {
			inSupport := false
			for _, z := range activeSup {
				if cl >= z.bottom && cl <= z.top {
					inSupport = true
					break
				}
			}
			if inSupport {
				entryPrice := ohlcv[i+1].Open
				swingLow := ohlcv[i].Low
				lookStart := i - s.SLLookback
				if lookStart < 0 {
					lookStart = 0
				}
				for j := lookStart; j <= i; j++ {
					if ohlcv[j].Low < swingLow {
						swingLow = ohlcv[j].Low
					}
				}
				sl := swingLow * (1 - s.SLBuffer/100)
				risk := entryPrice - sl
				if risk > 0 {
					signals = append(signals, StrategySignal{
						Index: i + 1, Direction: "LONG", EntryPrice: entryPrice,
						StopLoss: sl, TakeProfit: entryPrice + s.RiskReward*risk,
					})
				}
			}
		}

		// SHORT: Close in resistance zone + GMMA crossunder + both < 0
		if crossunder && mainNow < 0 && sigNow < 0 {
			inResistance := false
			for _, z := range activeRes {
				if cl >= z.bottom && cl <= z.top {
					inResistance = true
					break
				}
			}
			if inResistance {
				entryPrice := ohlcv[i+1].Open
				swingHigh := ohlcv[i].High
				lookStart := i - s.SLLookback
				if lookStart < 0 {
					lookStart = 0
				}
				for j := lookStart; j <= i; j++ {
					if ohlcv[j].High > swingHigh {
						swingHigh = ohlcv[j].High
					}
				}
				sl := swingHigh * (1 + s.SLBuffer/100)
				risk := sl - entryPrice
				if risk > 0 {
					signals = append(signals, StrategySignal{
						Index: i + 1, Direction: "SHORT", EntryPrice: entryPrice,
						StopLoss: sl, TakeProfit: entryPrice - s.RiskReward*risk,
					})
				}
			}
		}
	}
	return signals
}

func (s *GMMAPullbackStrategy) ComputeIndicators(ohlcv []OHLCV) []IndicatorSeries {
	s.defaults()
	n := len(ohlcv)
	if n < 80 {
		return nil
	}

	closes := make([]float64, n)
	for i, bar := range ohlcv {
		closes[i] = bar.Close
	}
	gmma := computeGMMAOscillator(closes, s.SmoothLen, s.SignalLen)

	mainData := make([]IndicatorPoint, n)
	sigData := make([]IndicatorPoint, n)
	for i, bar := range ohlcv {
		color := "#22c55e" // green when main > signal
		if gmma.main[i] < gmma.signal[i] {
			color = "#ef4444" // red
		}
		mainData[i] = IndicatorPoint{Time: bar.Time, Value: gmma.main[i], Color: color}
		sigData[i] = IndicatorPoint{Time: bar.Time, Value: gmma.signal[i]}
	}

	return []IndicatorSeries{
		{Name: "GMMA Main", Type: "histogram", Color: "#22c55e", Data: mainData},
		{Name: "GMMA Signal", Type: "line", Color: "#f59e0b", Data: sigData},
	}
}

func (s *GMMAPullbackStrategy) ComputeOverlays(ohlcv []OHLCV) []OverlaySeries {
	s.defaults()
	n := len(ohlcv)
	if n < 80 {
		return nil
	}

	sup, res := computeSRZones(ohlcv, s.FractalPeriods, s.ZoneCount, 0.5)

	var overlays []OverlaySeries
	for idx, z := range sup {
		topPts := []OverlayPoint{{Time: z.time, Value: z.top}, {Time: ohlcv[n-1].Time, Value: z.top}}
		botPts := []OverlayPoint{{Time: z.time, Value: z.bottom}, {Time: ohlcv[n-1].Time, Value: z.bottom}}
		name := "S" + fmt.Sprintf("%d", idx+1)
		overlays = append(overlays,
			OverlaySeries{Name: name + " Top", Type: "line", Color: "rgba(34,197,94,0.4)", Data: topPts, Style: 2,
				FillColor: "rgba(34,197,94,0.06)"},
			OverlaySeries{Name: name + " Bot", Type: "line", Color: "rgba(34,197,94,0.4)", Data: botPts, Style: 2,
				InvertFill: true, FillColor: "rgba(34,197,94,0.06)"},
		)
	}
	for idx, z := range res {
		topPts := []OverlayPoint{{Time: z.time, Value: z.top}, {Time: ohlcv[n-1].Time, Value: z.top}}
		botPts := []OverlayPoint{{Time: z.time, Value: z.bottom}, {Time: ohlcv[n-1].Time, Value: z.bottom}}
		name := "R" + fmt.Sprintf("%d", idx+1)
		overlays = append(overlays,
			OverlaySeries{Name: name + " Top", Type: "line", Color: "rgba(239,68,68,0.4)", Data: topPts, Style: 2,
				FillColor: "rgba(239,68,68,0.06)"},
			OverlaySeries{Name: name + " Bot", Type: "line", Color: "rgba(239,68,68,0.4)", Data: botPts, Style: 2,
				InvertFill: true, FillColor: "rgba(239,68,68,0.06)"},
		)
	}
	return overlays
}

// ========== MACD + S/R Strategy ==========

type MACDSRStrategy struct {
	MACDFast          int
	MACDSlow          int
	MACDSignal        int
	EMAPeriod         int
	SLBuffer          float64
	RiskReward        float64
	SRFilter          bool
	FractalPeriods    int
	ZoneCount         int
	SRTolerance       float64
	HybridFilter      bool
	HybridLongThresh  float64
	HybridShortThresh float64
}

func (s *MACDSRStrategy) defaults() {
	if s.MACDFast <= 0 {
		s.MACDFast = 12
	}
	if s.MACDSlow <= 0 {
		s.MACDSlow = 26
	}
	if s.MACDSignal <= 0 {
		s.MACDSignal = 9
	}
	if s.EMAPeriod <= 0 {
		s.EMAPeriod = 200
	}
	if s.SLBuffer <= 0 {
		s.SLBuffer = 1.5
	}
	if s.RiskReward <= 0 {
		s.RiskReward = 1.5
	}
	if s.FractalPeriods <= 0 {
		s.FractalPeriods = 5
	}
	if s.ZoneCount <= 0 {
		s.ZoneCount = 5
	}
	if s.SRTolerance <= 0 {
		s.SRTolerance = 1.5
	}
	if s.HybridLongThresh <= 0 {
		s.HybridLongThresh = 75
	}
	if s.HybridShortThresh <= 0 {
		s.HybridShortThresh = 25
	}
}

func (s *MACDSRStrategy) Name() string      { return "macd_sr" }
func (s *MACDSRStrategy) RequiredBars() int  { s.defaults(); return s.EMAPeriod + s.MACDSignal + 50 }

func (s *MACDSRStrategy) Analyze(ohlcv []OHLCV) []StrategySignal {
	s.defaults()
	var signals []StrategySignal
	n := len(ohlcv)
	minBars := s.EMAPeriod + s.MACDSignal + 10
	if n < minBars {
		return signals
	}

	closes := extractCloses(ohlcv)

	// MACD calculation
	emaFast := calculateEMAServer(closes, s.MACDFast)
	emaSlow := calculateEMAServer(closes, s.MACDSlow)
	macdLine := make([]float64, n)
	for i := 0; i < n; i++ {
		macdLine[i] = emaFast[i] - emaSlow[i]
	}
	signalLine := calculateEMAServer(macdLine, s.MACDSignal)

	// 200 EMA trend filter
	ema200 := calculateEMAServer(closes, s.EMAPeriod)

	// Optional Hybrid AlgoAI filter
	var hybridOsc []float64
	if s.HybridFilter {
		hybridOsc = calculateHybridEMA(closes, 50, 200, 5, 100, 400)
	}

	startBar := s.EMAPeriod + s.MACDSignal
	if startBar < s.MACDSlow+s.MACDSignal {
		startBar = s.MACDSlow + s.MACDSignal
	}

	zoneBreach := 0.5

	for i := startBar; i < n-1; i++ {
		// MACD crossover detection — cross must happen below zero (LONG) / above zero (SHORT)
		bullishCross := macdLine[i-1] <= signalLine[i-1] && macdLine[i] > signalLine[i] && macdLine[i-1] < 0 && signalLine[i-1] < 0
		bearishCross := macdLine[i-1] >= signalLine[i-1] && macdLine[i] < signalLine[i] && macdLine[i-1] > 0 && signalLine[i-1] > 0

		if !bullishCross && !bearishCross {
			continue
		}

		cl := ohlcv[i].Close

		// EMA(200) trend filter
		isAboveEMA := cl > ema200[i]
		isBelowEMA := cl < ema200[i]

		// Hybrid AlgoAI filter
		if s.HybridFilter && hybridOsc != nil {
			if bullishCross && hybridOsc[i] < s.HybridLongThresh {
				bullishCross = false
			}
			if bearishCross && hybridOsc[i] > s.HybridShortThresh {
				bearishCross = false
			}
		}

		// S/R zones (computed on-the-fly)
		var activeSup, activeRes []srZone
		if s.SRFilter {
			maxPivotIdx := i - s.FractalPeriods
			if maxPivotIdx < s.FractalPeriods {
				continue
			}
			for pi := s.FractalPeriods; pi <= maxPivotIdx; pi++ {
				// Pivot Low → Support
				isPL := true
				for j := 1; j <= s.FractalPeriods; j++ {
					if ohlcv[pi].Low >= ohlcv[pi-j].Low || ohlcv[pi].Low >= ohlcv[pi+j].Low {
						isPL = false
						break
					}
				}
				if isPL {
					body := ohlcv[pi].Open
					if ohlcv[pi].Close < body {
						body = ohlcv[pi].Close
					}
					breached := false
					threshold := ohlcv[pi].Low * (1 - zoneBreach/100)
					for bi := pi + s.FractalPeriods + 1; bi <= i; bi++ {
						if ohlcv[bi].Close < threshold {
							breached = true
							break
						}
					}
					if !breached {
						activeSup = append(activeSup, srZone{top: body, bottom: ohlcv[pi].Low, time: ohlcv[pi].Time})
					}
				}

				// Pivot High → Resistance
				isPH := true
				for j := 1; j <= s.FractalPeriods; j++ {
					if ohlcv[pi].High <= ohlcv[pi-j].High || ohlcv[pi].High <= ohlcv[pi+j].High {
						isPH = false
						break
					}
				}
				if isPH {
					body := ohlcv[pi].Open
					if ohlcv[pi].Close > body {
						body = ohlcv[pi].Close
					}
					breached := false
					threshold := ohlcv[pi].High * (1 + zoneBreach/100)
					for bi := pi + s.FractalPeriods + 1; bi <= i; bi++ {
						if ohlcv[bi].Close > threshold {
							breached = true
							break
						}
					}
					if !breached {
						activeRes = append(activeRes, srZone{top: ohlcv[pi].High, bottom: body, time: ohlcv[pi].Time})
					}
				}
			}
			if len(activeSup) > s.ZoneCount {
				activeSup = activeSup[len(activeSup)-s.ZoneCount:]
			}
			if len(activeRes) > s.ZoneCount {
				activeRes = activeRes[len(activeRes)-s.ZoneCount:]
			}
		}

		// LONG signal
		if bullishCross && isAboveEMA {
			passedSR := true
			if s.SRFilter {
				passedSR = false
				tol := cl * s.SRTolerance / 100
				for _, z := range activeSup {
					if cl >= z.bottom-tol && cl <= z.top+tol {
						passedSR = true
						break
					}
				}
			}
			if passedSR {
				entryPrice := ohlcv[i+1].Open
				// SL below 200 EMA — the EMA acts as a wall
				sl := ema200[i] * (1 - s.SLBuffer/100)
				risk := entryPrice - sl
				if risk > 0 {
					signals = append(signals, StrategySignal{
						Index: i + 1, Direction: "LONG", EntryPrice: entryPrice,
						StopLoss: sl, TakeProfit: entryPrice + s.RiskReward*risk,
					})
				}
			}
		}

		// SHORT signal
		if bearishCross && isBelowEMA {
			passedSR := true
			if s.SRFilter {
				passedSR = false
				tol := cl * s.SRTolerance / 100
				for _, z := range activeRes {
					if cl >= z.bottom-tol && cl <= z.top+tol {
						passedSR = true
						break
					}
				}
			}
			if passedSR {
				entryPrice := ohlcv[i+1].Open
				// SL above 200 EMA — the EMA acts as a wall
				sl := ema200[i] * (1 + s.SLBuffer/100)
				risk := sl - entryPrice
				if risk > 0 {
					signals = append(signals, StrategySignal{
						Index: i + 1, Direction: "SHORT", EntryPrice: entryPrice,
						StopLoss: sl, TakeProfit: entryPrice - s.RiskReward*risk,
					})
				}
			}
		}
	}
	return signals
}

func (s *MACDSRStrategy) ComputeIndicators(ohlcv []OHLCV) []IndicatorSeries {
	s.defaults()
	n := len(ohlcv)
	if n < s.MACDSlow+s.MACDSignal {
		return nil
	}

	closes := extractCloses(ohlcv)
	emaFast := calculateEMAServer(closes, s.MACDFast)
	emaSlow := calculateEMAServer(closes, s.MACDSlow)
	macdLine := make([]float64, n)
	for i := 0; i < n; i++ {
		macdLine[i] = emaFast[i] - emaSlow[i]
	}
	signalLine := calculateEMAServer(macdLine, s.MACDSignal)

	macdData := make([]IndicatorPoint, n)
	sigData := make([]IndicatorPoint, n)
	histData := make([]IndicatorPoint, n)
	for i, bar := range ohlcv {
		hist := macdLine[i] - signalLine[i]
		color := "#22c55e"
		if hist < 0 {
			color = "#ef4444"
		}
		macdData[i] = IndicatorPoint{Time: bar.Time, Value: macdLine[i]}
		sigData[i] = IndicatorPoint{Time: bar.Time, Value: signalLine[i]}
		histData[i] = IndicatorPoint{Time: bar.Time, Value: hist, Color: color}
	}

	series := []IndicatorSeries{
		{Name: "MACD Histogram", Type: "histogram", Color: "#22c55e", Data: histData},
		{Name: "MACD", Type: "line", Color: "#3b82f6", Data: macdData},
		{Name: "Signal", Type: "line", Color: "#f59e0b", Data: sigData},
	}

	if s.HybridFilter {
		hybridOsc := calculateHybridEMA(closes, 50, 200, 5, 100, 400)
		hybData := make([]IndicatorPoint, n)
		for i, bar := range ohlcv {
			hybData[i] = IndicatorPoint{Time: bar.Time, Value: hybridOsc[i]}
		}
		series = append(series, IndicatorSeries{Name: "Hybrid AlgoAI", Type: "line", Color: "#a855f7", Data: hybData})
	}

	return series
}

func (s *MACDSRStrategy) ComputeOverlays(ohlcv []OHLCV) []OverlaySeries {
	s.defaults()
	n := len(ohlcv)
	if n < s.EMAPeriod {
		return nil
	}

	closes := extractCloses(ohlcv)
	ema200 := calculateEMAServer(closes, s.EMAPeriod)

	// EMA(200) overlay
	emaPoints := make([]OverlayPoint, n)
	for i, bar := range ohlcv {
		emaPoints[i] = OverlayPoint{Time: bar.Time, Value: ema200[i]}
	}
	overlays := []OverlaySeries{
		{Name: fmt.Sprintf("EMA(%d)", s.EMAPeriod), Type: "line", Color: "#f59e0b", Data: emaPoints},
	}

	// S/R zones
	if s.SRFilter {
		sup, res := computeSRZones(ohlcv, s.FractalPeriods, s.ZoneCount, 0.5)
		for idx, z := range sup {
			topPts := []OverlayPoint{{Time: z.time, Value: z.top}, {Time: ohlcv[n-1].Time, Value: z.top}}
			botPts := []OverlayPoint{{Time: z.time, Value: z.bottom}, {Time: ohlcv[n-1].Time, Value: z.bottom}}
			name := "S" + fmt.Sprintf("%d", idx+1)
			overlays = append(overlays,
				OverlaySeries{Name: name + " Top", Type: "line", Color: "rgba(34,197,94,0.4)", Data: topPts, Style: 2,
					FillColor: "rgba(34,197,94,0.06)"},
				OverlaySeries{Name: name + " Bot", Type: "line", Color: "rgba(34,197,94,0.4)", Data: botPts, Style: 2,
					InvertFill: true, FillColor: "rgba(34,197,94,0.06)"},
			)
		}
		for idx, z := range res {
			topPts := []OverlayPoint{{Time: z.time, Value: z.top}, {Time: ohlcv[n-1].Time, Value: z.top}}
			botPts := []OverlayPoint{{Time: z.time, Value: z.bottom}, {Time: ohlcv[n-1].Time, Value: z.bottom}}
			name := "R" + fmt.Sprintf("%d", idx+1)
			overlays = append(overlays,
				OverlaySeries{Name: name + " Top", Type: "line", Color: "rgba(239,68,68,0.4)", Data: topPts, Style: 2,
					FillColor: "rgba(239,68,68,0.06)"},
				OverlaySeries{Name: name + " Bot", Type: "line", Color: "rgba(239,68,68,0.4)", Data: botPts, Style: 2,
					InvertFill: true, FillColor: "rgba(239,68,68,0.06)"},
			)
		}
	}

	return overlays
}

// TrippaTrade — Regression Slope Oscillator + EMA Pullback (BigBeluga RSO)
type TrippaTrade struct {
	MaxRange     int
	MinRange     int
	Step         int
	SignalLen    int
	EMAFast      int
	EMASlow      int
	RiskReward   float64
	SLBuffer     float64
	MinTrendBars int
}

func (s *TrippaTrade) Name() string     { return "trippa_trade" }
func (s *TrippaTrade) RequiredBars() int { s.defaults(); return s.MaxRange + s.SignalLen + 20 }

func (s *TrippaTrade) defaults() {
	if s.MaxRange <= 0 {
		s.MaxRange = 100
	}
	if s.MinRange <= 0 {
		s.MinRange = 10
	}
	if s.Step <= 0 {
		s.Step = 5
	}
	if s.SignalLen <= 0 {
		s.SignalLen = 7
	}
	if s.EMAFast <= 0 {
		s.EMAFast = 5
	}
	if s.EMASlow <= 0 {
		s.EMASlow = 13
	}
	if s.RiskReward <= 0 {
		s.RiskReward = 2.0
	}
	if s.SLBuffer < 0 {
		s.SLBuffer = 0.5
	}
	if s.MinTrendBars <= 0 {
		s.MinTrendBars = 3
	}
}

// calcRegressionSlopeOscillator computes the averaged log-regression slope oscillator
// based on BigBeluga's Regression Slope Oscillator indicator.
func calcRegressionSlopeOscillator(closes []float64, minRange, maxRange, step int) []float64 {
	n := len(closes)
	osc := make([]float64, n)
	if n == 0 || minRange < 2 {
		return osc
	}

	// Collect all lengths
	var lengths []int
	for l := minRange; l <= maxRange; l += step {
		lengths = append(lengths, l)
	}
	if len(lengths) == 0 {
		return osc
	}

	for i := 0; i < n; i++ {
		sumSlope := 0.0
		count := 0
		for _, length := range lengths {
			if i < length-1 {
				continue
			}
			// Log-regression slope over [i-length+1 .. i]
			// f_log_regression from BigBeluga: sum_x, sum_y (log), sum_x_sqr, sum_xy → slope * -1
			sumX := 0.0
			sumY := 0.0
			sumXSqr := 0.0
			sumXY := 0.0
			fLen := float64(length)
			for j := 0; j < length; j++ {
				idx := i - length + 1 + j
				x := float64(j)
				y := 0.0
				if closes[idx] > 0 {
					y = math.Log(closes[idx])
				}
				sumX += x
				sumY += y
				sumXSqr += x * x
				sumXY += x * y
			}
			denom := fLen*sumXSqr - sumX*sumX
			if denom != 0 {
				slope := (fLen*sumXY - sumX*sumY) / denom
				sumSlope += slope * -1
				count++
			}
		}
		if count > 0 {
			osc[i] = sumSlope / float64(count)
		}
	}
	return osc
}

func (s *TrippaTrade) Analyze(ohlcv []OHLCV) []StrategySignal {
	s.defaults()
	var signals []StrategySignal
	n := len(ohlcv)
	minBars := s.MaxRange + s.SignalLen + 5
	if n < minBars {
		return signals
	}

	closes := extractCloses(ohlcv)

	// Regression Slope Oscillator
	oscillator := calcRegressionSlopeOscillator(closes, s.MinRange, s.MaxRange, s.Step)
	// Signal line = SMA of oscillator
	signalLine := calculateSMAServer(oscillator, s.SignalLen)
	// EMA ribbon
	emaFast := calculateEMAServer(closes, s.EMAFast)
	emaSlow := calculateEMAServer(closes, s.EMASlow)

	// State machine: trend → pullback → confirmation → entry
	startBar := s.MaxRange + s.SignalLen
	trendDir := 0   // +1 bullish, -1 bearish
	trendBars := 0
	phase := 0       // 0=waiting for trend, 1=trend established, 2=pullback detected
	pullbackLow := 0.0
	pullbackHigh := 0.0

	for i := startBar; i < n-1; i++ {
		// Track trend: oscillator vs zero line
		if oscillator[i] > 0 {
			if trendDir == 1 {
				trendBars++
			} else {
				trendDir = 1
				trendBars = 1
				phase = 0
			}
		} else if oscillator[i] < 0 {
			if trendDir == -1 {
				trendBars++
			} else {
				trendDir = -1
				trendBars = 1
				phase = 0
			}
		} else {
			trendDir = 0
			trendBars = 0
			phase = 0
			continue
		}

		// Phase 0→1: trend established after MinTrendBars
		if phase == 0 && trendBars >= s.MinTrendBars {
			phase = 1
		}

		if phase < 1 {
			continue
		}

		// Phase 1→2: detect pullback
		// LONG pullback: EMA5 < EMA13 AND osc < signal
		// SHORT pullback: EMA5 > EMA13 AND osc > signal
		if phase == 1 {
			if trendDir == 1 && emaFast[i] < emaSlow[i] && oscillator[i] < signalLine[i] {
				phase = 2
				pullbackLow = ohlcv[i].Low
			} else if trendDir == -1 && emaFast[i] > emaSlow[i] && oscillator[i] > signalLine[i] {
				phase = 2
				pullbackHigh = ohlcv[i].High
			}
		}

		// Track pullback extremes
		if phase == 2 && trendDir == 1 {
			if ohlcv[i].Low < pullbackLow {
				pullbackLow = ohlcv[i].Low
			}
		} else if phase == 2 && trendDir == -1 {
			if ohlcv[i].High > pullbackHigh {
				pullbackHigh = ohlcv[i].High
			}
		}

		// Phase 2→entry: confirmation
		// LONG: EMA5 > EMA13 AND osc > signal (back in trend direction)
		// SHORT: EMA5 < EMA13 AND osc < signal
		if phase == 2 {
			if trendDir == 1 && emaFast[i] > emaSlow[i] && oscillator[i] > signalLine[i] {
				entryPrice := ohlcv[i+1].Open
				sl := pullbackLow * (1 - s.SLBuffer/100)
				risk := entryPrice - sl
				if risk > 0 {
					signals = append(signals, StrategySignal{
						Index: i + 1, Direction: "LONG", EntryPrice: entryPrice,
						StopLoss: sl, TakeProfit: entryPrice + s.RiskReward*risk,
					})
				}
				phase = 1
			} else if trendDir == -1 && emaFast[i] < emaSlow[i] && oscillator[i] < signalLine[i] {
				entryPrice := ohlcv[i+1].Open
				sl := pullbackHigh * (1 + s.SLBuffer/100)
				risk := sl - entryPrice
				if risk > 0 {
					signals = append(signals, StrategySignal{
						Index: i + 1, Direction: "SHORT", EntryPrice: entryPrice,
						StopLoss: sl, TakeProfit: entryPrice - s.RiskReward*risk,
					})
				}
				phase = 1
			}
		}
	}

	return signals
}

func (s *TrippaTrade) ComputeIndicators(ohlcv []OHLCV) []IndicatorSeries {
	s.defaults()
	n := len(ohlcv)
	if n < s.MaxRange+s.SignalLen {
		return nil
	}

	closes := extractCloses(ohlcv)
	oscillator := calcRegressionSlopeOscillator(closes, s.MinRange, s.MaxRange, s.Step)
	signalLine := calculateSMAServer(oscillator, s.SignalLen)

	histData := make([]IndicatorPoint, n)
	sigData := make([]IndicatorPoint, n)
	for i, bar := range ohlcv {
		color := "#22c55e"
		if oscillator[i] < 0 {
			color = "#ef4444"
		}
		histData[i] = IndicatorPoint{Time: bar.Time, Value: oscillator[i], Color: color}
		sigData[i] = IndicatorPoint{Time: bar.Time, Value: signalLine[i]}
	}

	return []IndicatorSeries{
		{Name: "RSO", Type: "histogram", Color: "#22c55e", Data: histData},
		{Name: "Signal", Type: "line", Color: "#f97316", Data: sigData},
	}
}

func (s *TrippaTrade) ComputeOverlays(ohlcv []OHLCV) []OverlaySeries {
	s.defaults()
	n := len(ohlcv)
	if n < s.EMASlow {
		return nil
	}

	closes := extractCloses(ohlcv)
	emaFast := calculateEMAServer(closes, s.EMAFast)
	emaSlow := calculateEMAServer(closes, s.EMASlow)

	fastData := make([]OverlayPoint, n)
	slowData := make([]OverlayPoint, n)
	for i, bar := range ohlcv {
		fastData[i] = OverlayPoint{Time: bar.Time, Value: emaFast[i]}
		slowData[i] = OverlayPoint{Time: bar.Time, Value: emaSlow[i]}
	}

	return []OverlaySeries{
		{Name: fmt.Sprintf("EMA(%d)", s.EMAFast), Type: "line", Color: "#60a5fa", Data: fastData},
		{Name: fmt.Sprintf("EMA(%d)", s.EMASlow), Type: "line", Color: "#f97316", Data: slowData},
	}
}

type OverlaySeries struct {
	Name       string         `json:"name"`
	Type       string         `json:"type"` // "line"
	Color      string         `json:"color"`
	Data       []OverlayPoint `json:"data"`
	Style      int            `json:"style"` // 0=solid, 2=dashed
	FillColor  string         `json:"fill_color,omitempty"`  // RGBA for area fill
	InvertFill bool           `json:"invert_fill,omitempty"` // true = fill upward (for lower bands)
}

type OverlayPoint struct {
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
}

type OverlayProvider interface {
	ComputeOverlays(ohlcv []OHLCV) []OverlaySeries
}

// BacktestResult holds all results of a backtest run
type ArenaBacktestResult struct {
	Metrics    ArenaBacktestMetrics `json:"metrics"`
	Trades     []ArenaBacktestTrade `json:"trades"`
	Markers    []ChartMarker        `json:"markers"`
	Indicators []IndicatorSeries    `json:"indicators,omitempty"`
	Overlays   []OverlaySeries      `json:"overlays,omitempty"`
	ChartData  []OHLCV              `json:"chart_data,omitempty"`
}

type ArenaBacktestMetrics struct {
	WinRate     float64 `json:"win_rate"`
	RiskReward  float64 `json:"risk_reward"`
	TotalReturn float64 `json:"total_return"`
	AvgReturn   float64 `json:"avg_return"`
	MaxDrawdown float64 `json:"max_drawdown"`
	NetProfit   float64 `json:"net_profit"`
	TotalTrades int     `json:"total_trades"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
}

type ArenaBacktestTrade struct {
	Direction  string  `json:"direction"`
	EntryPrice float64 `json:"entry_price"`
	EntryTime  int64   `json:"entry_time"`
	ExitPrice  float64 `json:"exit_price"`
	ExitTime   int64   `json:"exit_time"`
	ReturnPct  float64 `json:"return_pct"`
	ExitReason string  `json:"exit_reason"` // "TP", "SL", "SIGNAL", "END"
	IsOpen     bool    `json:"is_open"`
}

// runArenaBacktest runs a bar-by-bar backtest simulation
func runArenaBacktest(ohlcv []OHLCV, strategy TradingStrategy) ArenaBacktestResult {
	signals := strategy.Analyze(ohlcv)
	var trades []ArenaBacktestTrade
	var markers []ChartMarker

	signalMap := make(map[int]StrategySignal)
	for _, sig := range signals {
		signalMap[sig.Index] = sig
	}

	var activeTrade *ArenaBacktestTrade
	var activeSL, activeTP float64
	var activeDir string

	for i := 0; i < len(ohlcv); i++ {
		bar := ohlcv[i]

		// Phase 1: Execute signals at bar open (entry/exit happen at open price)
		// Signals have Index set to the bar AFTER detection, so EntryPrice = this bar's Open
		if sig, ok := signalMap[i]; ok {
			// Close opposing trade at bar open
			if activeTrade != nil && activeDir != sig.Direction {
				if activeDir == "LONG" {
					activeTrade.ReturnPct = (bar.Open - activeTrade.EntryPrice) / activeTrade.EntryPrice * 100
				} else {
					activeTrade.ReturnPct = (activeTrade.EntryPrice - bar.Open) / activeTrade.EntryPrice * 100
				}
				activeTrade.ExitPrice = bar.Open
				activeTrade.ExitTime = bar.Time
				activeTrade.ExitReason = "SIGNAL"
				trades = append(trades, *activeTrade)
				activeTrade = nil
			}
			if activeTrade == nil && sig.EntryPrice > 0 {
				activeTrade = &ArenaBacktestTrade{
					Direction:  sig.Direction,
					EntryPrice: sig.EntryPrice,
					EntryTime:  bar.Time,
				}
				activeSL = sig.StopLoss
				activeTP = sig.TakeProfit
				activeDir = sig.Direction

				if sig.Direction == "LONG" {
					shape, text, color := "arrowUp", "LONG", "#22c55e"
					if sig.Shape != "" { shape = sig.Shape }
					if sig.Text != "" { text = sig.Text }
					if sig.Color != "" { color = sig.Color }
					markers = append(markers, ChartMarker{Time: bar.Time, Position: "belowBar", Color: color, Shape: shape, Text: text})
				} else {
					shape, text, color := "arrowDown", "SHORT", "#ef4444"
					if sig.Shape != "" { shape = sig.Shape }
					if sig.Text != "" { text = sig.Text }
					if sig.Color != "" { color = sig.Color }
					markers = append(markers, ChartMarker{Time: bar.Time, Position: "aboveBar", Color: color, Shape: shape, Text: text})
				}
			}
		}

		// Phase 2: Check SL/TP during bar (intrabar stop/limit order execution)
		if activeTrade != nil {
			closed := false
			if activeDir == "LONG" {
				if bar.Low <= activeSL {
					activeTrade.ExitPrice = activeSL
					activeTrade.ExitTime = bar.Time
					activeTrade.ReturnPct = (activeSL - activeTrade.EntryPrice) / activeTrade.EntryPrice * 100
					activeTrade.ExitReason = "SL"
					closed = true
					markers = append(markers, ChartMarker{Time: bar.Time, Position: "aboveBar", Color: "#ef4444", Shape: "arrowDown", Text: "SL"})
				} else if bar.High >= activeTP {
					activeTrade.ExitPrice = activeTP
					activeTrade.ExitTime = bar.Time
					activeTrade.ReturnPct = (activeTP - activeTrade.EntryPrice) / activeTrade.EntryPrice * 100
					activeTrade.ExitReason = "TP"
					closed = true
					markers = append(markers, ChartMarker{Time: bar.Time, Position: "aboveBar", Color: "#22c55e", Shape: "arrowDown", Text: "TP"})
				}
			} else { // SHORT
				if bar.High >= activeSL {
					activeTrade.ExitPrice = activeSL
					activeTrade.ExitTime = bar.Time
					activeTrade.ReturnPct = (activeTrade.EntryPrice - activeSL) / activeTrade.EntryPrice * 100
					activeTrade.ExitReason = "SL"
					closed = true
					markers = append(markers, ChartMarker{Time: bar.Time, Position: "belowBar", Color: "#ef4444", Shape: "arrowUp", Text: "SL"})
				} else if bar.Low <= activeTP {
					activeTrade.ExitPrice = activeTP
					activeTrade.ExitTime = bar.Time
					activeTrade.ReturnPct = (activeTrade.EntryPrice - activeTP) / activeTrade.EntryPrice * 100
					activeTrade.ExitReason = "TP"
					closed = true
					markers = append(markers, ChartMarker{Time: bar.Time, Position: "belowBar", Color: "#22c55e", Shape: "arrowUp", Text: "TP"})
				}
			}
			if closed {
				trades = append(trades, *activeTrade)
				activeTrade = nil
			}
		}
	}

	// Close open trade at end
	if activeTrade != nil && len(ohlcv) > 0 {
		lastBar := ohlcv[len(ohlcv)-1]
		activeTrade.ExitPrice = lastBar.Close
		activeTrade.ExitTime = lastBar.Time
		activeTrade.ExitReason = "END"
		activeTrade.IsOpen = true
		if activeDir == "LONG" {
			activeTrade.ReturnPct = (lastBar.Close - activeTrade.EntryPrice) / activeTrade.EntryPrice * 100
		} else {
			activeTrade.ReturnPct = (activeTrade.EntryPrice - lastBar.Close) / activeTrade.EntryPrice * 100
		}
		trades = append(trades, *activeTrade)
	}

	// Calculate metrics
	metrics := ArenaBacktestMetrics{TotalTrades: len(trades)}
	if len(trades) > 0 {
		totalReturn := 0.0
		var winReturns, lossReturns []float64
		equity := 100.0
		peak := equity
		maxDD := 0.0

		for _, t := range trades {
			totalReturn += t.ReturnPct
			if t.ReturnPct >= 0 {
				metrics.Wins++
				winReturns = append(winReturns, t.ReturnPct)
			} else {
				metrics.Losses++
				lossReturns = append(lossReturns, t.ReturnPct)
			}
			equity *= (1 + t.ReturnPct/100)
			if equity > peak {
				peak = equity
			}
			dd := (peak - equity) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}

		metrics.WinRate = float64(metrics.Wins) / float64(metrics.TotalTrades) * 100
		metrics.TotalReturn = totalReturn
		metrics.AvgReturn = totalReturn / float64(metrics.TotalTrades)
		metrics.MaxDrawdown = maxDD
		metrics.NetProfit = equity - 100 // Profit/Loss in % of starting capital

		if len(winReturns) > 0 && len(lossReturns) > 0 {
			avgWin := 0.0
			for _, w := range winReturns {
				avgWin += w
			}
			avgWin /= float64(len(winReturns))
			avgLoss := 0.0
			for _, l := range lossReturns {
				avgLoss += math.Abs(l)
			}
			avgLoss /= float64(len(lossReturns))
			if avgLoss > 0 {
				metrics.RiskReward = avgWin / avgLoss
			}
		}
	}

	return ArenaBacktestResult{Metrics: metrics, Trades: trades, Markers: markers}
}

// --- IndicatorProvider implementations ---

// --- Regression Scalping: Overlay = Bands, Sub-Chart = AO ---

func (s *RegressionScalpingStrategy) ComputeOverlays(ohlcv []OHLCV) []OverlaySeries {
	s.defaults()
	if len(ohlcv) < s.Length+20 { return nil }
	upper, middle, lower, _, _ := s.compute(ohlcv)
	toData := func(vals []float64) []OverlayPoint {
		pts := make([]OverlayPoint, 0, len(ohlcv))
		for i, v := range vals {
			if v != 0 { pts = append(pts, OverlayPoint{Time: ohlcv[i].Time, Value: v}) }
		}
		return pts
	}
	return []OverlaySeries{
		{Name: "Upper Band", Type: "line", Color: "#ef4444", Data: toData(upper), Style: 2,
			FillColor: "rgba(239,68,68,0.08)"},
		{Name: "Prediction", Type: "line", Color: "#9ca3af", Data: toData(middle), Style: 0},
		{Name: "Lower Band", Type: "line", Color: "#22c55e", Data: toData(lower), Style: 2,
			FillColor: "rgba(34,197,94,0.08)", InvertFill: true},
	}
}

func (s *RegressionScalpingStrategy) ComputeIndicators(ohlcv []OHLCV) []IndicatorSeries {
	s.defaults()
	if len(ohlcv) < 34 { return nil }
	_, _, _, ao, _ := s.compute(ohlcv)
	data := make([]IndicatorPoint, len(ohlcv))
	for i := range ohlcv {
		color := "#22c55e"
		if i > 0 && ao[i] < ao[i-1] { color = "#ef4444" }
		data[i] = IndicatorPoint{Time: ohlcv[i].Time, Value: ao[i], Color: color}
	}
	return []IndicatorSeries{{Name: "Awesome Oscillator", Type: "histogram", Color: "#f59e0b", Data: data}}
}

// --- NW Bollinger Bands: Overlay = 4-Level NW-BB, no Sub-Chart ---

func (s *HybridAITrendStrategy) ComputeOverlays(ohlcv []OHLCV) []OverlaySeries {
	s.defaults()
	if len(ohlcv) < s.BB1Period+10 { return nil }

	toData := func(vals []float64) []OverlayPoint {
		pts := make([]OverlayPoint, 0, len(ohlcv))
		for i, v := range vals {
			if v != 0 { pts = append(pts, OverlayPoint{Time: ohlcv[i].Time, Value: v}) }
		}
		return pts
	}

	// Compute all 4 BB levels with NW smoothing
	u1, l1 := calculateSingleBBLevel(ohlcv, s.BB1Period, s.BB1Stdev, s.NWBandwidth, s.NWLookback)
	u2, l2 := calculateSingleBBLevel(ohlcv, s.BB2Period, s.BB2Stdev, s.NWBandwidth, s.NWLookback)
	u3, l3 := calculateSingleBBLevel(ohlcv, s.BB3Period, s.BB3Stdev, s.NWBandwidth, s.NWLookback)
	u4, l4 := calculateSingleBBLevel(ohlcv, s.BB4Period, s.BB4Stdev, s.NWBandwidth, s.NWLookback)

	return []OverlaySeries{
		// Upper bands (red) — innermost to outermost
		{Name: "NW-BB Upper 1", Type: "line", Color: "#ef4444", Data: toData(u1), Style: 0,
			FillColor: "rgba(239,68,68,0.10)"},
		{Name: "NW-BB Upper 2", Type: "line", Color: "rgba(239,68,68,0.5)", Data: toData(u2), Style: 2,
			FillColor: "rgba(239,68,68,0.12)"},
		{Name: "NW-BB Upper 3", Type: "line", Color: "rgba(239,68,68,0.35)", Data: toData(u3), Style: 2,
			FillColor: "rgba(239,68,68,0.15)"},
		{Name: "NW-BB Upper 4", Type: "line", Color: "rgba(239,68,68,0.25)", Data: toData(u4), Style: 2},
		// Lower bands (green) — innermost to outermost
		{Name: "NW-BB Lower 1", Type: "line", Color: "#22c55e", Data: toData(l1), Style: 0,
			FillColor: "rgba(34,197,94,0.10)", InvertFill: true},
		{Name: "NW-BB Lower 2", Type: "line", Color: "rgba(34,197,94,0.5)", Data: toData(l2), Style: 2,
			FillColor: "rgba(34,197,94,0.12)", InvertFill: true},
		{Name: "NW-BB Lower 3", Type: "line", Color: "rgba(34,197,94,0.35)", Data: toData(l3), Style: 2,
			FillColor: "rgba(34,197,94,0.15)", InvertFill: true},
		{Name: "NW-BB Lower 4", Type: "line", Color: "rgba(34,197,94,0.25)", Data: toData(l4), Style: 2},
	}
}

func (s *HybridAITrendStrategy) ComputeIndicators(ohlcv []OHLCV) []IndicatorSeries {
	// Hybrid EMA AlgoLearner oscillator (display only, no signal influence)
	// Hardcoded defaults matching TradingView Script 4jhuhtMN
	const (
		shortEMA     = 50
		longEMA      = 200
		kNearest     = 5
		lookback     = 100
		normLookback = 400
		oscHigh      = 75.0
		oscLow       = 25.0
	)
	minBars := normLookback + 50
	if len(ohlcv) < minBars {
		return nil
	}
	closes := extractCloses(ohlcv)
	oscillator := calculateHybridEMA(closes, shortEMA, longEMA, kNearest, lookback, normLookback)

	oscData := make([]IndicatorPoint, len(ohlcv))
	for i := range ohlcv {
		color := "#6366f1" // indigo default
		if oscillator[i] >= oscHigh {
			color = "#22c55e" // green = bullish
		} else if oscillator[i] <= oscLow {
			color = "#ef4444" // red = bearish
		}
		oscData[i] = IndicatorPoint{Time: ohlcv[i].Time, Value: oscillator[i], Color: color}
	}

	refHigh := make([]IndicatorPoint, len(ohlcv))
	refMid := make([]IndicatorPoint, len(ohlcv))
	refLow := make([]IndicatorPoint, len(ohlcv))
	for i := range ohlcv {
		refHigh[i] = IndicatorPoint{Time: ohlcv[i].Time, Value: oscHigh}
		refMid[i] = IndicatorPoint{Time: ohlcv[i].Time, Value: 50}
		refLow[i] = IndicatorPoint{Time: ohlcv[i].Time, Value: oscLow}
	}
	return []IndicatorSeries{
		{Name: "Hybrid EMA AlgoLearner", Type: "line", Color: "#6366f1", Data: oscData},
		{Name: "75", Type: "reference_line", Color: "#22c55e40", Data: refHigh},
		{Name: "50", Type: "reference_line", Color: "#4b556340", Data: refMid},
		{Name: "25", Type: "reference_line", Color: "#ef444440", Data: refLow},
	}
}

// --- Diamond Signals: Sub-Chart = Confluence Score ---

func (s *DiamondSignalsStrategy) ComputeIndicators(ohlcv []OHLCV) []IndicatorSeries {
	s.defaults()
	if len(ohlcv) < 200 { return nil }

	closes := extractCloses(ohlcv)
	rsi := calculateRSIServer(closes, s.RSIPeriod)

	bullDiamond, bearDiamond := detectDiamondPattern(ohlcv, s.PatternLength)
	rsiBullDiv, rsiBearDiv := detectRSIDivergence(closes, s.RSIPeriod, 30)
	volBullDiv, volBearDiv := detectVolumeDivergence(ohlcv, 30)
	demandZones, supplyZones := detectOrderBlocks(ohlcv, 50)

	aggOHLCV := aggregateOHLCV(ohlcv, 4)
	var aggBullDiamondBase, aggBearDiamondBase map[int]bool
	var aggRSIBullBase, aggRSIBearBase map[int]bool
	if len(aggOHLCV) >= 50 {
		aggBullD, aggBearD := detectDiamondPattern(aggOHLCV, s.PatternLength)
		aggCloses := extractCloses(aggOHLCV)
		aggRSIBull, aggRSIBear := detectRSIDivergence(aggCloses, s.RSIPeriod, 15)
		aggBullDiamondBase = expandAggMap(aggBullD, 4, len(ohlcv))
		aggBearDiamondBase = expandAggMap(aggBearD, 4, len(ohlcv))
		aggRSIBullBase = expandAggMap(aggRSIBull, 4, len(ohlcv))
		aggRSIBearBase = expandAggMap(aggRSIBear, 4, len(ohlcv))
	} else {
		aggBullDiamondBase = make(map[int]bool)
		aggBearDiamondBase = make(map[int]bool)
		aggRSIBullBase = make(map[int]bool)
		aggRSIBearBase = make(map[int]bool)
	}

	bullDiamondMap := toIndexMap(bullDiamond)
	bearDiamondMap := toIndexMap(bearDiamond)
	rsiBullMap := toIndexMap(rsiBullDiv)
	rsiBearMap := toIndexMap(rsiBearDiv)
	volBullMap := toIndexMap(volBullDiv)
	volBearMap := toIndexMap(volBearDiv)

	data := make([]IndicatorPoint, len(ohlcv))
	for i := range ohlcv {
		if i < 50 {
			data[i] = IndicatorPoint{Time: ohlcv[i].Time, Value: 0}
			continue
		}
		longScore := 0
		if withinRange(bullDiamondMap, i, 3) { longScore++ }
		if withinRange(rsiBullMap, i, 3) { longScore++ }
		if withinRange(volBullMap, i, 3) { longScore++ }
		if isNearZone(closes[i], demandZones, 0.02) { longScore++ }
		if aggBullDiamondBase[i] || aggRSIBullBase[i] { longScore++ }

		shortScore := 0
		if withinRange(bearDiamondMap, i, 3) { shortScore++ }
		if withinRange(rsiBearMap, i, 3) { shortScore++ }
		if withinRange(volBearMap, i, 3) { shortScore++ }
		if isNearZone(closes[i], supplyZones, 0.02) { shortScore++ }
		if aggBearDiamondBase[i] || aggRSIBearBase[i] { shortScore++ }

		score := float64(longScore - shortScore)
		if rsi[i] >= s.RSIOverbought { score -= 0.5 }
		if rsi[i] <= s.RSIOversold { score += 0.5 }

		color := "#6b7280"
		if score >= 2 { color = "#22c55e" } else if score >= 1 { color = "#86efac" } else if score <= -2 { color = "#ef4444" } else if score <= -1 { color = "#fca5a5" }
		data[i] = IndicatorPoint{Time: ohlcv[i].Time, Value: score, Color: color}
	}
	return []IndicatorSeries{{Name: "Confluence Score", Type: "histogram", Color: "#8b5cf6", Data: data}}
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

	// Skip warmup period — indicators not stable before minLen bars
	for i := minLen; i < len(ohlcv); i++ {
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

	// Skip warmup period — indicators not stable before minLen bars
	for i := minLen; i < len(ohlcv); i++ {
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

	// Only process signals on the 1st of the month to match calculated trade history
	if !isFirstOfMonth() {
		return
	}

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

		if isStockDataStale(stockPerf.UpdatedAt) {
			addLog("SKIP", fmt.Sprintf("%s: Daten älter als 48h (letztes Update: %s) - überspringe", pos.Symbol, stockPerf.UpdatedAt.Format("02.01.2006 15:04")))
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
		if isStockDataStale(stock.UpdatedAt) {
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
	q := db.Where("is_pending = ? AND is_closed = ?", false, false)
	if blocked := getBlockedSymbolsForBot("ditz"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("buy_date desc").Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	// Fetch market caps from stocks table
	marketCaps := make(map[string]int64)
	if len(symbols) > 0 {
		var mcStocks []Stock
		db.Select("symbol, market_cap").Where("symbol IN ? AND market_cap > 0", symbols).Find(&mcStocks)
		for _, s := range mcStocks {
			marketCaps[s.Symbol] = s.MarketCap
		}
	}

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
		MarketCap      int64     `json:"market_cap"`
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
			MarketCap:      marketCaps[pos.Symbol],
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
	q := db.Where("is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false)
	if blocked := getBlockedSymbolsForBot("ditz"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("signal_date desc").Limit(50).Find(&trades)
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
	blocked := getBlockedSymbolsForBot("ditz")

	var sellTrades []DitzTrade
	sq := db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false, false)
	if len(blocked) > 0 {
		sq = sq.Where("symbol NOT IN ?", blocked)
	}
	sq.Find(&sellTrades)

	var buyTrades []DitzTrade
	bq := db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "BUY", false, false, false)
	if len(blocked) > 0 {
		bq = bq.Where("symbol NOT IN ?", blocked)
	}
	bq.Find(&buyTrades)

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

	// Set up streaming response for progress updates
	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	sendProgress := func(current, total int, symbol, message string) {
		line, _ := json.Marshal(gin.H{"type": "progress", "current": current, "total": total, "symbol": symbol, "message": message})
		c.Writer.Write(append(line, '\n'))
		c.Writer.Flush()
	}

	// Get all tracked stocks with their ditz performance data
	var trackedStocks []DitzStockPerformance
	db.Find(&trackedStocks)

	if len(trackedStocks) == 0 {
		line, _ := json.Marshal(gin.H{"type": "done", "trades_created": 0, "positions_created": 0, "logs": logs})
		c.Writer.Write(append(line, '\n'))
		c.Writer.Flush()
		return
	}

	var tradesCreated int
	var positionsCreated int

	for stockIdx, stock := range trackedStocks {
		sendProgress(stockIdx+1, len(trackedStocks), stock.Symbol, fmt.Sprintf("Verarbeite %s (%d/%d)", stock.Symbol, stockIdx+1, len(trackedStocks)))
		if stock.TradesJSON == "" {
			continue
		}

		// Check allowlist
		if !isStockAllowedForBot("ditz", stock.Symbol) {
			addLog("SKIP", fmt.Sprintf("%s: Nicht in Allowlist — übersprungen", stock.Symbol))
			continue
		}

		// Check bot filter config
		if filterBlocked, filterReason := checkBotFilterConfig("ditz", stock.WinRate, stock.RiskReward, stock.AvgReturn, stock.MarketCap); filterBlocked {
			addLog("FILTER", fmt.Sprintf("%s: Übersprungen durch Filter (%s)", stock.Symbol, filterReason))
			continue
		}

		// Check if bot already has an open position for this stock
		var existingBotPos DitzPosition
		if db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingBotPos).Error == nil {
			addLog("SKIP", fmt.Sprintf("%s: Bot hat bereits offene Position — übersprungen", stock.Symbol))
			continue
		}

		var historicalTrades []TradeData
		if err := json.Unmarshal([]byte(stock.TradesJSON), &historicalTrades); err != nil {
			addLog("ERROR", fmt.Sprintf("%s: Fehler beim Parsen der Trades: %v", stock.Symbol, err))
			continue
		}

		// Check if there's already an open position from BEFORE or AT the backfill start date
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

		// Warmup detection: check if indicator has enough data for stable signals
		warmupEnd := getWarmupEndDate(stock.Symbol, 225, historicalTrades)

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
			// Check if trade is in warmup period (indicator not yet stable)
			isWarmup := warmupEnd > 0 && trade.EntryDate <= warmupEnd


			buyTrade := DitzTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      trade.EntryPrice,
				SignalDate: entryTime,
				ExecutedAt: entryTime,
				IsPending:  false,
				IsDeleted:  isWarmup,
			}
			db.Create(&buyTrade)
			tradesCreated++
			if isWarmup {
				addLog("WARMUP", fmt.Sprintf("%s: BUY @ $%.2f am %s — Indikator nicht eingeschwungen (225 Bars nötig)", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))
			} else {
				addLog("ACTION", fmt.Sprintf("%s: BUY erstellt @ $%.2f am %s", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))
			}

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
						IsDeleted:     isWarmup,
					}
					db.Create(&sellTrade)
					tradesCreated++
					if !isWarmup {
						addLog("ACTION", fmt.Sprintf("%s: SELL erstellt @ $%.2f am %s (%.2f%%)", stock.Symbol, *trade.ExitPrice, exitTime.Format("2006-01-02"), profitLossPct))
					}
				} else if !isWarmup {
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
			} else if trade.IsOpen && !isWarmup {
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

	line, _ := json.Marshal(gin.H{"type": "done", "trades_created": tradesCreated, "positions_created": positionsCreated, "until_date": req.UntilDate, "logs": logs})
	c.Writer.Write(append(line, '\n'))
	c.Writer.Flush()
}

func getDitzCompletedTrades(c *gin.Context) {
	var trades []DitzTrade
	q := db.Where("action = ? AND profit_loss IS NOT NULL AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false)
	if blocked := getBlockedSymbolsForBot("ditz"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("executed_at desc").Find(&trades)

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

	// Fetch market caps from stocks table
	marketCaps := make(map[string]int64)
	if len(symbols) > 0 {
		var mcStocks []Stock
		db.Select("symbol, market_cap").Where("symbol IN ? AND market_cap > 0", symbols).Find(&mcStocks)
		for _, s := range mcStocks {
			marketCaps[s.Symbol] = s.MarketCap
		}
	}

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
		MarketCap      int64     `json:"market_cap"`
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
			MarketCap:      marketCaps[pos.Symbol],
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

	// Only process signals on the 1st of the month to match calculated trade history
	if !isFirstOfMonth() {
		return
	}

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

		if isStockDataStale(stockPerf.UpdatedAt) {
			addLog("SKIP", fmt.Sprintf("%s: Daten älter als 48h (letztes Update: %s) - überspringe", pos.Symbol, stockPerf.UpdatedAt.Format("02.01.2006 15:04")))
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
		if isStockDataStale(stock.UpdatedAt) {
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
	q := db.Where("is_pending = ? AND is_closed = ?", false, false)
	if blocked := getBlockedSymbolsForBot("trader"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("buy_date desc").Find(&positions)

	symbols := make([]string, len(positions))
	for i, p := range positions {
		symbols[i] = p.Symbol
	}
	quotes := fetchQuotes(symbols)

	// Fetch market caps from stocks table
	marketCaps := make(map[string]int64)
	if len(symbols) > 0 {
		var mcStocks []Stock
		db.Select("symbol, market_cap").Where("symbol IN ? AND market_cap > 0", symbols).Find(&mcStocks)
		for _, s := range mcStocks {
			marketCaps[s.Symbol] = s.MarketCap
		}
	}

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
		MarketCap      int64     `json:"market_cap"`
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
			MarketCap:      marketCaps[pos.Symbol],
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
	q := db.Where("is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", false, false, false)
	if blocked := getBlockedSymbolsForBot("trader"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("signal_date desc").Limit(50).Find(&trades)
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
	blocked := getBlockedSymbolsForBot("trader")

	var sellTrades []TraderTrade
	sq := db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false, false)
	if len(blocked) > 0 {
		sq = sq.Where("symbol NOT IN ?", blocked)
	}
	sq.Find(&sellTrades)

	var buyTrades []TraderTrade
	bq := db.Where("action = ? AND is_pending = ? AND is_deleted = ? AND is_filter_blocked = ?", "BUY", false, false, false)
	if len(blocked) > 0 {
		bq = bq.Where("symbol NOT IN ?", blocked)
	}
	bq.Find(&buyTrades)

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

	// Set up streaming response for progress updates
	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	sendProgress := func(current, total int, symbol, message string) {
		line, _ := json.Marshal(gin.H{"type": "progress", "current": current, "total": total, "symbol": symbol, "message": message})
		c.Writer.Write(append(line, '\n'))
		c.Writer.Flush()
	}

	// Get all tracked stocks with their trader performance data
	var trackedStocks []TraderStockPerformance
	db.Find(&trackedStocks)

	if len(trackedStocks) == 0 {
		line, _ := json.Marshal(gin.H{"type": "done", "trades_created": 0, "positions_created": 0, "logs": logs})
		c.Writer.Write(append(line, '\n'))
		c.Writer.Flush()
		return
	}

	var tradesCreated int
	var positionsCreated int

	for stockIdx, stock := range trackedStocks {
		sendProgress(stockIdx+1, len(trackedStocks), stock.Symbol, fmt.Sprintf("Verarbeite %s (%d/%d)", stock.Symbol, stockIdx+1, len(trackedStocks)))
		if stock.TradesJSON == "" {
			continue
		}

		// Check allowlist
		if !isStockAllowedForBot("trader", stock.Symbol) {
			addLog("SKIP", fmt.Sprintf("%s: Nicht in Allowlist — übersprungen", stock.Symbol))
			continue
		}

		// Check bot filter config
		if filterBlocked, filterReason := checkBotFilterConfig("trader", stock.WinRate, stock.RiskReward, stock.AvgReturn, stock.MarketCap); filterBlocked {
			addLog("FILTER", fmt.Sprintf("%s: Übersprungen durch Filter (%s)", stock.Symbol, filterReason))
			continue
		}

		// Check if bot already has an open position for this stock
		var existingBotPos TraderPosition
		if db.Where("symbol = ? AND is_closed = ?", stock.Symbol, false).First(&existingBotPos).Error == nil {
			addLog("SKIP", fmt.Sprintf("%s: Bot hat bereits offene Position — übersprungen", stock.Symbol))
			continue
		}

		var historicalTrades []TradeData
		if err := json.Unmarshal([]byte(stock.TradesJSON), &historicalTrades); err != nil {
			addLog("ERROR", fmt.Sprintf("%s: Fehler beim Parsen der Trades: %v", stock.Symbol, err))
			continue
		}

		// Check if there's already an open position from BEFORE or AT the backfill start date
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

		// Warmup detection: check if indicator has enough data for stable signals
		warmupEnd := getWarmupEndDate(stock.Symbol, 45, historicalTrades)

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
			// Check if trade is in warmup period (indicator not yet stable)
			isWarmup := warmupEnd > 0 && trade.EntryDate <= warmupEnd


			buyTrade := TraderTrade{
				Symbol:     stock.Symbol,
				Name:       stock.Name,
				Action:     "BUY",
				Quantity:   qty,
				Price:      trade.EntryPrice,
				SignalDate: entryTime,
				ExecutedAt: entryTime,
				IsPending:  false,
				IsDeleted:  isWarmup,
			}
			db.Create(&buyTrade)
			tradesCreated++
			if isWarmup {
				addLog("WARMUP", fmt.Sprintf("%s: BUY @ $%.2f am %s — Indikator nicht eingeschwungen (45 Bars nötig)", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))
			} else {
				addLog("ACTION", fmt.Sprintf("%s: BUY erstellt @ $%.2f am %s", stock.Symbol, trade.EntryPrice, entryTime.Format("2006-01-02")))
			}

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
						IsDeleted:     isWarmup,
					}
					db.Create(&sellTrade)
					tradesCreated++
					if !isWarmup {
						addLog("ACTION", fmt.Sprintf("%s: SELL erstellt @ $%.2f am %s (%.2f%%)", stock.Symbol, *trade.ExitPrice, exitTime.Format("2006-01-02"), profitLossPct))
					}
				} else if !isWarmup {
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
			} else if trade.IsOpen && !isWarmup {
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

	line, _ := json.Marshal(gin.H{"type": "done", "trades_created": tradesCreated, "positions_created": positionsCreated, "until_date": req.UntilDate, "logs": logs})
	c.Writer.Write(append(line, '\n'))
	c.Writer.Flush()
}

func getTraderCompletedTrades(c *gin.Context) {
	var trades []TraderTrade
	q := db.Where("action = ? AND profit_loss IS NOT NULL AND is_deleted = ? AND is_filter_blocked = ?", "SELL", false, false)
	if blocked := getBlockedSymbolsForBot("trader"); len(blocked) > 0 {
		q = q.Where("symbol NOT IN ?", blocked)
	}
	q.Order("executed_at desc").Find(&trades)

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

	// Fetch market caps from stocks table
	marketCaps := make(map[string]int64)
	if len(symbols) > 0 {
		var mcStocks []Stock
		db.Select("symbol, market_cap").Where("symbol IN ? AND market_cap > 0", symbols).Find(&mcStocks)
		for _, s := range mcStocks {
			marketCaps[s.Symbol] = s.MarketCap
		}
	}

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
		MarketCap      int64     `json:"market_cap"`
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
			MarketCap:      marketCaps[pos.Symbol],
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

	// Skip warmup period — indicators not stable before minLen bars
	for i := minLen; i < len(ohlcv); i++ {
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

	// Skip warmup period — indicators not stable before minLen bars
	for i := minLen; i < len(ohlcv); i++ {
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
// ==================== Signal Liste ====================

type signalListModeData struct {
	Mode                string   `json:"mode"`
	Signal              string   `json:"signal"`
	SignalSince         string   `json:"signal_since"`
	WinRate             float64  `json:"win_rate"`
	RiskReward          float64  `json:"risk_reward"`
	TotalReturn         float64  `json:"total_return"`
	AvgReturn           float64  `json:"avg_return"`
	TotalTrades         int      `json:"total_trades"`
	Wins                int      `json:"wins"`
	Losses              int      `json:"losses"`
	TradeReturnPct      *float64 `json:"trade_return_pct"`
	TradeDurationMonths *int     `json:"trade_duration_months"`
}

type signalListEntry struct {
	Symbol         string               `json:"symbol"`
	Name           string               `json:"name"`
	Signal         string               `json:"signal"`
	SignalSince    string               `json:"signal_since"`
	CurrentPrice   float64              `json:"current_price"`
	MarketCap      int64                `json:"market_cap"`
	TradeReturnPct      *float64             `json:"trade_return_pct"`
	TradeDurationMonths *int                 `json:"trade_duration_months"`
	Modes               []signalListModeData `json:"modes"`
	ModeCount      int                  `json:"mode_count"`
	BuyModeCount   int                  `json:"buy_mode_count"`
	SellModeCount  int                  `json:"sell_mode_count"`
	Visible        bool                 `json:"visible"`
	WinRate        float64              `json:"win_rate"`
	RiskReward     float64              `json:"risk_reward"`
	TotalReturn    float64              `json:"total_return"`
	AvgReturn      float64              `json:"avg_return"`
	TotalTrades    int                  `json:"total_trades"`
}

func signalForMonth(tradesJSON string, targetYear int, targetMonth int) string {
	var trades []TradeData
	if err := json.Unmarshal([]byte(tradesJSON), &trades); err != nil || len(trades) == 0 {
		return ""
	}

	monthStart := time.Date(targetYear, time.Month(targetMonth), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Second)

	hasSell := false
	hasBuy := false
	hasHold := false

	for _, trade := range trades {
		entryDate := time.Unix(trade.EntryDate, 0).UTC()
		hasExit := trade.ExitDate != nil && *trade.ExitDate > 0
		var exitDate time.Time
		if hasExit {
			exitDate = time.Unix(*trade.ExitDate, 0).UTC()
		}

		openAtMonthEnd := !hasExit || exitDate.After(monthEnd)

		// SELL: exit in target month
		if hasExit && !exitDate.Before(monthStart) && !exitDate.After(monthEnd) {
			hasSell = true
		}
		// BUY: entry in target month and still open at month end
		if !entryDate.Before(monthStart) && !entryDate.After(monthEnd) && openAtMonthEnd {
			hasBuy = true
		}
		// HOLD: entry before month and still open at month end
		if entryDate.Before(monthStart) && openAtMonthEnd {
			hasHold = true
		}
	}

	if hasSell {
		return "SELL"
	}
	if hasBuy {
		return "BUY"
	}
	if hasHold {
		return "HOLD"
	}
	return "WAIT"
}

func getSignalList(c *gin.Context) {
	now := time.Now()
	monthParam := c.DefaultQuery("month", fmt.Sprintf("%d-%02d", now.Year(), now.Month()))

	// Parse month parameter
	parts := strings.Split(monthParam, "-")
	if len(parts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid month format, use YYYY-MM"})
		return
	}
	targetYear := 0
	targetMonth := 0
	fmt.Sscanf(parts[0], "%d", &targetYear)
	fmt.Sscanf(parts[1], "%d", &targetMonth)
	if targetYear == 0 || targetMonth < 1 || targetMonth > 12 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid month"})
		return
	}

	isCurrentMonth := targetYear == now.Year() && targetMonth == int(now.Month())
	isAdmin := false
	if v, exists := c.Get("isAdmin"); exists {
		isAdmin = v.(bool)
	}

	type perfRow struct {
		Symbol       string
		Name         string
		Signal       string
		SignalSince  string
		CurrentPrice float64
		MarketCap    int64
		WinRate      float64
		RiskReward   float64
		TotalReturn  float64
		AvgReturn    float64
		TotalTrades  int
		Wins         int
		Losses       int
		TradesJSON   string
	}

	symbolMap := make(map[string]*signalListEntry)

	// Extract trade return + duration from TradesJSON
	type tradeReturnInfo struct {
		ReturnPct      float64
		DurationMonths int
	}
	getTradeReturn := func(tradesJSON string, sig string, signalSince string, tYear int, tMonth int) *tradeReturnInfo {
		var trades []TradeData
		if err := json.Unmarshal([]byte(tradesJSON), &trades); err != nil || len(trades) == 0 {
			return nil
		}
		mStart := time.Date(tYear, time.Month(tMonth), 1, 0, 0, 0, 0, time.UTC)
		mEnd := mStart.AddDate(0, 1, 0).Add(-time.Second)
		calcMonths := func(entry, exit time.Time) int {
			m := (exit.Year()-entry.Year())*12 + int(exit.Month()) - int(entry.Month())
			if m < 1 {
				m = 1
			}
			return m
		}
		if sig == "BUY" {
			for i := len(trades) - 1; i >= 0; i-- {
				t := trades[i]
				entryDate := time.Unix(t.EntryDate, 0).UTC()
				hasExit := t.ExitDate != nil && *t.ExitDate > 0
				openAtEnd := !hasExit || time.Unix(*t.ExitDate, 0).UTC().After(mEnd)
				if !entryDate.After(mEnd) && openAtEnd {
					dur := calcMonths(entryDate, time.Now())
					return &tradeReturnInfo{ReturnPct: t.ReturnPct, DurationMonths: dur}
				}
			}
		} else if sig == "SELL" {
			// For current month: find last closed trade matching signal_since or just the latest closed
			// For historical: find trade closed in that month
			for i := len(trades) - 1; i >= 0; i-- {
				t := trades[i]
				if t.ExitDate == nil || *t.ExitDate <= 0 {
					continue
				}
				exitDate := time.Unix(*t.ExitDate, 0).UTC()
				entryDate := time.Unix(t.EntryDate, 0).UTC()
				if isCurrentMonth {
					// For current month: match by signal_since date or latest closed trade
					if signalSince != "" {
						ssParsed, err := time.Parse("2006-01-02", signalSince)
						if err == nil {
							// Exit date should be in the same month as signalSince
							if exitDate.Year() == ssParsed.Year() && exitDate.Month() == ssParsed.Month() {
								dur := calcMonths(entryDate, exitDate)
								return &tradeReturnInfo{ReturnPct: t.ReturnPct, DurationMonths: dur}
							}
							continue
						}
					}
					// Fallback: latest closed trade
					dur := calcMonths(entryDate, exitDate)
					return &tradeReturnInfo{ReturnPct: t.ReturnPct, DurationMonths: dur}
				} else {
					if !exitDate.Before(mStart) && !exitDate.After(mEnd) {
						dur := calcMonths(entryDate, exitDate)
						return &tradeReturnInfo{ReturnPct: t.ReturnPct, DurationMonths: dur}
					}
				}
			}
		}
		return nil
	}


	processRows := func(rows []perfRow, modeName string) {
		for _, row := range rows {
			signal := ""
			signalSince := row.SignalSince

			if isCurrentMonth {
				if row.Signal == "BUY" || row.Signal == "SELL" {
					signal = row.Signal
				}
				// Fallback: derive signalSince from TradesJSON if empty
				if signal != "" && signalSince == "" && row.TradesJSON != "" {
					var tmpTrades []TradeData
					if err := json.Unmarshal([]byte(row.TradesJSON), &tmpTrades); err == nil && len(tmpTrades) > 0 {
						lastTrade := tmpTrades[len(tmpTrades)-1]
						if signal == "BUY" {
							entryDate := time.Unix(lastTrade.EntryDate, 0).UTC()
							signalSince = entryDate.Format("2006-01-02")
						} else if signal == "SELL" && lastTrade.ExitDate != nil && *lastTrade.ExitDate > 0 {
							exitDate := time.Unix(*lastTrade.ExitDate, 0).UTC()
							signalSince = exitDate.Format("2006-01-02")
						}
					}
				}
			} else {
				sig := signalForMonth(row.TradesJSON, targetYear, targetMonth)
				if sig == "BUY" || sig == "SELL" {
					signal = sig
					signalSince = ""
					// Derive date from TradesJSON for historical months
					var tmpTrades []TradeData
					if err := json.Unmarshal([]byte(row.TradesJSON), &tmpTrades); err == nil {
						mStart := time.Date(targetYear, time.Month(targetMonth), 1, 0, 0, 0, 0, time.UTC)
						mEnd := mStart.AddDate(0, 1, 0).Add(-time.Second)
						for i := len(tmpTrades) - 1; i >= 0; i-- {
							t := tmpTrades[i]
							if sig == "SELL" && t.ExitDate != nil && *t.ExitDate > 0 {
								exitDate := time.Unix(*t.ExitDate, 0).UTC()
								if !exitDate.Before(mStart) && !exitDate.After(mEnd) {
									signalSince = exitDate.Format("2006-01-02")
									break
								}
							} else if sig == "BUY" {
								entryDate := time.Unix(t.EntryDate, 0).UTC()
								if !entryDate.Before(mStart) && !entryDate.After(mEnd) {
									signalSince = entryDate.Format("2006-01-02")
									break
								}
							}
						}
					}
				}
			}

			if signal == "" {
				continue
			}

			modeData := signalListModeData{
				Mode:        modeName,
				Signal:      signal,
				SignalSince: signalSince,
				WinRate:     row.WinRate,
				RiskReward:  row.RiskReward,
				TotalReturn: row.TotalReturn,
				AvgReturn:   row.AvgReturn,
				TotalTrades: row.TotalTrades,
				Wins:        row.Wins,
				Losses:      row.Losses,
			}

			entry, exists := symbolMap[row.Symbol]
			if !exists {
				entry = &signalListEntry{
					Symbol:       row.Symbol,
					Name:         row.Name,
					CurrentPrice: row.CurrentPrice,
					MarketCap:    row.MarketCap,
					Visible:      true,
				}
				symbolMap[row.Symbol] = entry
			}
			// Calculate trade return per mode
			if info := getTradeReturn(row.TradesJSON, signal, signalSince, targetYear, targetMonth); info != nil {
				modeData.TradeReturnPct = &info.ReturnPct
				modeData.TradeDurationMonths = &info.DurationMonths
			}
			entry.Modes = append(entry.Modes, modeData)

			if signal == "BUY" {
				entry.BuyModeCount++
			} else {
				entry.SellModeCount++
			}
		}
	}

	// Query all 5 performance tables
	var defRows []perfRow
	db.Model(&StockPerformance{}).Select("symbol, name, signal, signal_since, current_price, market_cap, win_rate, risk_reward, total_return, avg_return, total_trades, wins, losses, trades_json").Find(&defRows)
	processRows(defRows, "defensive")

	var aggRows []perfRow
	db.Model(&AggressiveStockPerformance{}).Select("symbol, name, signal, signal_since, current_price, market_cap, win_rate, risk_reward, total_return, avg_return, total_trades, wins, losses, trades_json").Find(&aggRows)
	processRows(aggRows, "aggressive")

	var quantRows []perfRow
	db.Model(&QuantStockPerformance{}).Select("symbol, name, signal, signal_since, current_price, market_cap, win_rate, risk_reward, total_return, avg_return, total_trades, wins, losses, trades_json").Find(&quantRows)
	processRows(quantRows, "quant")

	var ditzRows []perfRow
	db.Model(&DitzStockPerformance{}).Select("symbol, name, signal, signal_since, current_price, market_cap, win_rate, risk_reward, total_return, avg_return, total_trades, wins, losses, trades_json").Find(&ditzRows)
	processRows(ditzRows, "ditz")

	var traderRows []perfRow
	db.Model(&TraderStockPerformance{}).Select("symbol, name, signal, signal_since, current_price, market_cap, win_rate, risk_reward, total_return, avg_return, total_trades, wins, losses, trades_json").Find(&traderRows)
	processRows(traderRows, "trader")

	// Load visibility
	var hidden []SignalListVisibility
	db.Where("month = ? AND visible = ?", monthParam, false).Find(&hidden)
	hiddenSet := make(map[string]bool)
	for _, h := range hidden {
		hiddenSet[h.Symbol] = true
	}

	// Build result
	var results []signalListEntry
	for _, entry := range symbolMap {
		entry.ModeCount = len(entry.Modes)

		// Determine dominant signal
		if entry.BuyModeCount >= entry.SellModeCount {
			entry.Signal = "BUY"
		} else {
			entry.Signal = "SELL"
		}

		// Find earliest signal_since
		earliest := ""
		for _, m := range entry.Modes {
			if m.SignalSince != "" {
				if earliest == "" || m.SignalSince < earliest {
					earliest = m.SignalSince
				}
			}
		}
		entry.SignalSince = earliest

		// Aggregate metrics: average across modes
		totalWR := 0.0
		totalRR := 0.0
		totalTR := 0.0
		totalAR := 0.0
		totalT := 0
		for _, m := range entry.Modes {
			totalWR += m.WinRate
			totalRR += m.RiskReward
			totalTR += m.TotalReturn
			totalAR += m.AvgReturn
			totalT += m.TotalTrades
		}
		n := float64(len(entry.Modes))
		entry.WinRate = totalWR / n
		entry.RiskReward = totalRR / n
		entry.TotalReturn = totalTR / n
		entry.AvgReturn = totalAR / n
		entry.TotalTrades = totalT

		// Aggregate trade return for sorting (average of mode returns)
		var returnSum float64
		var returnCount int
		for _, m := range entry.Modes {
			if m.TradeReturnPct != nil {
				returnSum += *m.TradeReturnPct
				returnCount++
			}
		}
		if returnCount > 0 {
			avg := returnSum / float64(returnCount)
			entry.TradeReturnPct = &avg
		}

		// Visibility
		if hiddenSet[entry.Symbol] {
			entry.Visible = false
		}
		if !isAdmin && !entry.Visible {
			continue
		}

		results = append(results, *entry)
	}

	// Sort: BUY first, then by mode_count desc, then market_cap desc
	sort.Slice(results, func(i, j int) bool {
		// Primary: BUY before SELL
		if results[i].Signal != results[j].Signal {
			return results[i].Signal == "BUY"
		}
		// Secondary: more total modes = higher rank
		if results[i].ModeCount != results[j].ModeCount {
			return results[i].ModeCount > results[j].ModeCount
		}
		// Tertiary: higher market cap
		return results[i].MarketCap > results[j].MarketCap
	})

	c.JSON(http.StatusOK, gin.H{
		"month":   monthParam,
		"entries": results,
		"total":   len(results),
	})
}

func getSignalListFilterConfig(c *gin.Context) {
	var config SignalListFilterConfig
	result := db.First(&config)
	if result.Error != nil {
		c.JSON(http.StatusOK, SignalListFilterConfig{})
		return
	}
	c.JSON(http.StatusOK, config)
}

func updateSignalListFilterConfig(c *gin.Context) {
	var req SignalListFilterConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var config SignalListFilterConfig
	result := db.First(&config)
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
		config.UpdatedAt = time.Now()
		db.Save(&config)
		c.JSON(http.StatusOK, config)
	}
}

func toggleSignalListVisibility(c *gin.Context) {
	var req struct {
		Symbol  string `json:"symbol"`
		Month   string `json:"month"`
		Visible bool   `json:"visible"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var vis SignalListVisibility
	result := db.Where("symbol = ? AND month = ?", req.Symbol, req.Month).First(&vis)
	if result.Error != nil {
		vis = SignalListVisibility{Symbol: req.Symbol, Month: req.Month, Visible: req.Visible}
		db.Create(&vis)
	} else {
		vis.Visible = req.Visible
		db.Save(&vis)
	}
	c.JSON(http.StatusOK, vis)
}

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


// ========== Trading Arena API Handlers ==========

func getTradingWatchlist(c *gin.Context) {
	var items []TradingWatchlistItem
	db.Order("symbol ASC").Find(&items)
	c.JSON(200, items)
}

func getGlobalSettings(c *gin.Context) {
	var settings []GlobalSetting
	db.Find(&settings)
	result := make(map[string]string, len(settings))
	for _, s := range settings {
		val := s.Value
		// Mask secrets — only show last 4 chars
		if strings.Contains(s.Key, "secret") && len(val) > 4 {
			val = "****" + val[len(val)-4:]
		}
		result[s.Key] = val
	}
	c.JSON(200, result)
}

func saveGlobalSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Ungültige Daten"})
		return
	}
	for key, value := range req {
		// Don't overwrite with masked value
		if strings.HasPrefix(value, "****") {
			continue
		}
		setGlobalSetting(key, value)
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func addToTradingWatchlist(c *gin.Context) {
	var req struct {
		Symbol string `json:"symbol"`
		Name   string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Symbol == "" {
		c.JSON(400, gin.H{"error": "Symbol erforderlich"})
		return
	}
	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))

	// Validate symbol is tradable on Alpaca
	asset, ok := isAlpacaTradable(symbol)
	if !ok || !asset.Tradable {
		c.JSON(400, gin.H{"error": "Symbol nicht über Alpaca handelbar"})
		return
	}

	var existingItem TradingWatchlistItem
	if db.Where("symbol = ?", symbol).First(&existingItem).Error == nil {
		c.JSON(409, gin.H{"error": "Symbol bereits in Trading-Watchlist"})
		return
	}

	// Try to get name from main watchlist if not provided
	name := strings.TrimSpace(req.Name)
	if name == "" {
		var stock Stock
		if db.Where("symbol = ?", symbol).First(&stock).Error == nil {
			name = stock.Name
		} else {
			name = symbol
		}
	}

	item := TradingWatchlistItem{
		Symbol:       symbol,
		Name:         name,
		Fractionable: asset.Fractionable,
		CreatedAt:    time.Now(),
	}
	if err := db.Create(&item).Error; err != nil {
		c.JSON(500, gin.H{"error": "Fehler beim Speichern"})
		return
	}
	c.JSON(201, item)
}

func removeFromTradingWatchlist(c *gin.Context) {
	id := c.Param("id")
	if err := db.Delete(&TradingWatchlistItem{}, id).Error; err != nil {
		c.JSON(500, gin.H{"error": "Fehler beim Löschen"})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func importWatchlistToTrading(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)

	// SSE streaming
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Flush()

	// Refresh Alpaca cache for fresh data
	if err := refreshTradableAssetsCache(); err != nil {
		msg := fmt.Sprintf(`{"done":true,"added":0,"skipped":0,"failed":0,"not_tradable":0,"error":"%s"}`, err.Error())
		fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
		c.Writer.Flush()
		return
	}

	// Load all stocks from main watchlist
	var allStocks []Stock
	db.Find(&allStocks)

	// Load existing trading watchlist
	var existing []TradingWatchlistItem
	db.Find(&existing)
	existingSet := map[string]bool{}
	for _, e := range existing {
		existingSet[e.Symbol] = true
	}

	// Filter to only stocks not already in trading watchlist
	type candidate struct {
		Symbol string
		Name   string
	}
	var candidates []candidate
	for _, s := range allStocks {
		if !existingSet[s.Symbol] {
			candidates = append(candidates, candidate{Symbol: s.Symbol, Name: s.Name})
		}
	}

	total := len(candidates)
	added := 0
	skipped := len(allStocks) - total // already existing
	notTradable := 0

	for i, cand := range candidates {
		progress := i + 1
		asset, ok := isAlpacaTradable(cand.Symbol)
		if !ok {
			notTradable++
			msg := fmt.Sprintf(`{"current":%d,"total":%d,"symbol":"%s","status":"not_tradable"}`, progress, total, cand.Symbol)
			fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
			c.Writer.Flush()
			continue
		}
		name := cand.Name
		if name == "" {
			name = cand.Symbol
		}
		db.Create(&TradingWatchlistItem{
			Symbol:       cand.Symbol,
			Name:         name,
			AddedBy:      uid,
			IsLive:       true,
			Fractionable: asset.Fractionable,
		})
		added++
		msg := fmt.Sprintf(`{"current":%d,"total":%d,"symbol":"%s","status":"added"}`, progress, total, cand.Symbol)
		fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
		c.Writer.Flush()
	}

	// Final event
	msg := fmt.Sprintf(`{"done":true,"added":%d,"skipped":%d,"failed":0,"not_tradable":%d}`, added, skipped, notTradable)
	fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
	c.Writer.Flush()
}

var backtestPeriodMap = map[string]string{
	"5m": "60d", "15m": "60d", "60m": "2y", "1h": "2y",
	"2h": "2y", "4h": "2y", "1d": "2y", "1wk": "10y",
}

func instantiateStrategy(strategyName string, params map[string]interface{}) (TradingStrategy, error) {
	pFloat := func(key string, def float64) float64 {
		if v, ok := params[key]; ok {
			switch val := v.(type) {
			case float64:
				return val
			case int:
				return float64(val)
			}
		}
		return def
	}
	pInt := func(key string, def int) int {
		if v, ok := params[key]; ok {
			switch val := v.(type) {
			case float64:
				return int(val)
			case int:
				return val
			}
		}
		return def
	}
	pBool := func(key string) bool {
		if v, ok := params[key]; ok {
			switch val := v.(type) {
			case bool:
				return val
			case float64:
				return val != 0
			}
		}
		return false
	}

	switch strategyName {
	case "regression_scalping":
		return &RegressionScalpingStrategy{
			Degree: pInt("degree", 0), Length: pInt("length", 0), Multiplier: pFloat("multiplier", 0),
			RiskReward: pFloat("risk_reward", 0), SLLookback: pInt("sl_lookback", 0),
			ConfirmationRequired: pInt("confirmation_required", -1),
		}, nil
	case "hybrid_ai_trend":
		return &HybridAITrendStrategy{
			BB1Period: pInt("bb1_period", 0), BB1Stdev: pFloat("bb1_stdev", 0),
			BB2Period: pInt("bb2_period", 0), BB2Stdev: pFloat("bb2_stdev", 0),
			BB3Period: pInt("bb3_period", 0), BB3Stdev: pFloat("bb3_stdev", 0),
			BB4Period: pInt("bb4_period", 0), BB4Stdev: pFloat("bb4_stdev", 0),
			NWBandwidth: pFloat("nw_bandwidth", 0), NWLookback: pInt("nw_lookback", 0),
			SLBuffer: pFloat("sl_buffer", 0), RiskReward: pFloat("risk_reward", 0),
			HybridFilter: pBool("hybrid_filter"),
			HybridLongThresh: pFloat("hybrid_long_thresh", 0), HybridShortThresh: pFloat("hybrid_short_thresh", 0),
			ConfirmCandle: pBool("confirm_candle"), MinBandDist: pFloat("min_band_dist", 0),
		}, nil
	case "diamond_signals":
		return &DiamondSignalsStrategy{
			PatternLength: pInt("pattern_length", 0), RSIPeriod: pInt("rsi_period", 0),
			ConfluenceMin: pInt("confluence_min", 0), RSIOverbought: pFloat("rsi_overbought", 0),
			RSIOversold: pFloat("rsi_oversold", 0), Cooldown: pInt("cooldown", 0), RiskReward: pFloat("risk_reward", 0),
		}, nil
	case "smart_money_flow":
		return &SmartMoneyFlowStrategy{
			TrendLength: pInt("trend_length", 0), BasisSmooth: pInt("basis_smooth", 0),
			FlowWindow: pInt("flow_window", 0), FlowSmooth: pInt("flow_smooth", 0),
			FlowBoost: pFloat("flow_boost", 0), ATRLength: pInt("atr_length", 0),
			BandTightness: pFloat("band_tightness", 0), BandExpansion: pFloat("band_expansion", 0),
			DotCooldown: pInt("dot_cooldown", 0), RiskReward: pFloat("risk_reward", 0),
		}, nil
	case "hann_trend":
		return &HannTrendStrategy{
			DMHLength: pInt("dmh_length", 0), SARStart: pFloat("sar_start", 0),
			SARIncrement: pFloat("sar_increment", 0), SARMax: pFloat("sar_max", 0),
			SwingLookback: pInt("swing_lookback", 0), RiskReward: pFloat("risk_reward", 0),
			SLBuffer: pFloat("sl_buffer", 0),
			HybridFilter: pBool("hybrid_filter"),
			HybridLongThresh: pFloat("hybrid_long_thresh", 0),
			HybridShortThresh: pFloat("hybrid_short_thresh", 0),
		}, nil
	case "gmma_pullback":
		return &GMMAPullbackStrategy{
			SignalLen: pInt("signal_len", 0), SmoothLen: pInt("smooth_len", 0),
			FractalPeriods: pInt("fractal_periods", 0), ZoneCount: pInt("zone_count", 0),
			RiskReward: pFloat("risk_reward", 0), SLLookback: pInt("sl_lookback", 0),
			SLBuffer: pFloat("sl_buffer", 0),
		}, nil
	case "macd_sr":
		return &MACDSRStrategy{
			MACDFast: pInt("macd_fast", 0), MACDSlow: pInt("macd_slow", 0),
			MACDSignal: pInt("macd_signal", 0), EMAPeriod: pInt("ema_period", 0),
			SLBuffer: pFloat("sl_buffer", 0), RiskReward: pFloat("risk_reward", 0),
			SRFilter: pInt("sr_filter", 1) == 1, FractalPeriods: pInt("fractal_periods", 0),
			ZoneCount: pInt("zone_count", 0), SRTolerance: pFloat("sr_tolerance", 0),
			HybridFilter: pInt("hybrid_filter", 0) == 1,
			HybridLongThresh: pFloat("hybrid_long_thresh", 0),
			HybridShortThresh: pFloat("hybrid_short_thresh", 0),
		}, nil
	case "trippa_trade":
		return &TrippaTrade{
			MaxRange: pInt("max_range", 0), MinRange: pInt("min_range", 0),
			Step: pInt("reg_step", 0), SignalLen: pInt("signal_len", 0),
			EMAFast: pInt("ema_fast", 0), EMASlow: pInt("ema_slow", 0),
			RiskReward: pFloat("risk_reward", 0), SLBuffer: pFloat("sl_buffer", 0),
			MinTrendBars: pInt("min_trend_bars", 0),
		}, nil
	default:
		return nil, fmt.Errorf("unbekannte Strategie: %s", strategyName)
	}
}

func runBacktestHandler(c *gin.Context) {
	var req struct {
		Symbol   string                 `json:"symbol"`
		Strategy string                 `json:"strategy"`
		Interval string                 `json:"interval"`
		Params   map[string]interface{} `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Ungültige Anfrage"})
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if symbol == "" {
		c.JSON(400, gin.H{"error": "Symbol erforderlich"})
		return
	}

	interval := req.Interval
	if interval == "" {
		interval = "4h"
	}
	if _, ok := backtestPeriodMap[interval]; !ok {
		c.JSON(400, gin.H{"error": "Ungültiges Interval"})
		return
	}

	ohlcv, err := getOHLCVCached(symbol, interval, 24*time.Hour)
	if err != nil {
		c.JSON(500, gin.H{"error": "Daten konnten nicht geladen werden: " + err.Error()})
		return
	}
	if len(ohlcv) < 50 {
		c.JSON(400, gin.H{"error": "Nicht genug Daten für Backtest"})
		return
	}

	strategy, err := instantiateStrategy(req.Strategy, req.Params)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result := runArenaBacktest(ohlcv, strategy)
	// Recalculate metrics from closed trades only (open trades still shown in chart)
	closedTrades := make([]ArenaBacktestTrade, 0)
	for _, t := range result.Trades {
		if !t.IsOpen {
			closedTrades = append(closedTrades, t)
		}
	}
	result.Metrics = recalcMetrics(closedTrades)
	log.Printf("[Arena-Single] %s: bars=%d trades=%d closed=%d winrate=%.1f%% interval=%s", symbol, len(ohlcv), len(result.Trades), len(closedTrades), result.Metrics.WinRate, interval)
	result.ChartData = ohlcv

	// Compute indicators if strategy supports it
	if provider, ok := strategy.(IndicatorProvider); ok {
		result.Indicators = provider.ComputeIndicators(ohlcv)
	}
	// Compute overlays (bands on price chart)
	if provider, ok := strategy.(OverlayProvider); ok {
		result.Overlays = provider.ComputeOverlays(ohlcv)
	}

	// Persist result in DB
	metricsJSON, _ := json.Marshal(result.Metrics)
	tradesJSON, _ := json.Marshal(result.Trades)
	markersJSON, _ := json.Marshal(result.Markers)

	var existing ArenaBacktestHistory
	if db.Where("symbol = ? AND strategy = ? AND interval = ?", symbol, req.Strategy, interval).First(&existing).Error == nil {
		existing.MetricsJSON = string(metricsJSON)
		existing.TradesJSON = string(tradesJSON)
		existing.MarkersJSON = string(markersJSON)
		existing.UpdatedAt = time.Now()
		db.Save(&existing)
	} else {
		db.Create(&ArenaBacktestHistory{
			Symbol:      symbol,
			Strategy:    req.Strategy,
			Interval:    interval,
			MetricsJSON: string(metricsJSON),
			TradesJSON:  string(tradesJSON),
			MarkersJSON: string(markersJSON),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		})
	}

	c.JSON(200, result)
}

func getBacktestResultsHandler(c *gin.Context) {
	symbol := strings.ToUpper(c.Param("symbol"))
	if symbol == "" {
		c.JSON(400, gin.H{"error": "Symbol erforderlich"})
		return
	}

	var results []ArenaBacktestHistory
	db.Where("symbol = ?", symbol).Find(&results)

	out := make(map[string]interface{})
	for _, r := range results {
		var metrics ArenaBacktestMetrics
		var trades []ArenaBacktestTrade
		json.Unmarshal([]byte(r.MetricsJSON), &metrics)
		json.Unmarshal([]byte(r.TradesJSON), &trades)
		out[r.Strategy] = gin.H{
			"metrics":    metrics,
			"trades":     trades,
			"interval":   r.Interval,
			"updated_at": r.UpdatedAt.Format(time.RFC3339),
		}
	}
	c.JSON(200, out)
}

var arenaBatchMutex sync.Mutex
var arenaBatchRunning bool

// Watchlist batch: cancel-previous-run mechanism (used by backtestWatchlistHandler)
var wlBatchMu sync.Mutex
var wlBatchCancelFn context.CancelFunc
var wlBatchGen uint64

// V2 batch: cancel-previous-run mechanism
var arenaV2BatchMu sync.Mutex
var arenaV2CancelFn context.CancelFunc
var arenaV2BatchGen uint64

func backtestBatchHandler(c *gin.Context) {
	arenaBatchMutex.Lock()
	if arenaBatchRunning {
		arenaBatchMutex.Unlock()
		c.JSON(429, gin.H{"error": "Batch bereits aktiv"})
		return
	}
	arenaBatchRunning = true
	arenaBatchMutex.Unlock()

	var stocks []Stock
	db.Select("symbol").Find(&stocks)

	strategies := []string{"regression_scalping", "hybrid_ai_trend", "smart_money_flow", "hann_trend", "macd_sr", "trippa_trade"}
	count := len(stocks) * len(strategies)

	go func() {
		defer func() {
			arenaBatchMutex.Lock()
			arenaBatchRunning = false
			arenaBatchMutex.Unlock()
		}()

		interval := "4h"

		for _, stock := range stocks {
			ohlcv, err := getOHLCVCached(stock.Symbol, interval, 24*time.Hour)
			if err != nil || len(ohlcv) < 50 {
				continue
			}

			for _, stratName := range strategies {
				var strategy TradingStrategy
				switch stratName {
				case "regression_scalping":
					strategy = &RegressionScalpingStrategy{}
				case "hybrid_ai_trend":
					strategy = &HybridAITrendStrategy{}
				case "smart_money_flow":
					strategy = &SmartMoneyFlowStrategy{}
				case "hann_trend":
					strategy = &HannTrendStrategy{}
				case "gmma_pullback":
					strategy = &GMMAPullbackStrategy{}
				case "macd_sr":
					strategy = &MACDSRStrategy{}
				case "trippa_trade":
					strategy = &TrippaTrade{}
				}

				result := runArenaBacktest(ohlcv, strategy)
				if provider, ok := strategy.(IndicatorProvider); ok {
					result.Indicators = provider.ComputeIndicators(ohlcv)
				}

				metricsJSON, _ := json.Marshal(result.Metrics)
				tradesJSON, _ := json.Marshal(result.Trades)
				markersJSON, _ := json.Marshal(result.Markers)

				var existing ArenaBacktestHistory
				if db.Where("symbol = ? AND strategy = ? AND interval = ?", stock.Symbol, stratName, interval).First(&existing).Error == nil {
					existing.MetricsJSON = string(metricsJSON)
					existing.TradesJSON = string(tradesJSON)
					existing.MarkersJSON = string(markersJSON)
					existing.UpdatedAt = time.Now()
					db.Save(&existing)
				} else {
					db.Create(&ArenaBacktestHistory{
						Symbol:      stock.Symbol,
						Strategy:    stratName,
						Interval:    interval,
						MetricsJSON: string(metricsJSON),
						TradesJSON:  string(tradesJSON),
						MarkersJSON: string(markersJSON),
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					})
				}
			}
		}
		log.Printf("[Arena] Batch backtest completed for %d stocks", len(stocks))
	}()

	c.JSON(200, gin.H{"status": "started", "count": count})
}

// --- Watchlist Batch Backtest ---

type WatchlistBacktestTrade struct {
	Symbol     string  `json:"symbol"`
	Direction  string  `json:"direction"`
	EntryPrice float64 `json:"entry_price"`
	EntryTime  int64   `json:"entry_time"`
	ExitPrice  float64 `json:"exit_price"`
	ExitTime   int64   `json:"exit_time"`
	ReturnPct  float64 `json:"return_pct"`
	ExitReason string  `json:"exit_reason"`
	IsOpen     bool    `json:"is_open"`
}

type WatchlistBacktestResult struct {
	Metrics        ArenaBacktestMetrics            `json:"metrics"`
	Trades         []WatchlistBacktestTrade        `json:"trades"`
	PerStock       map[string]ArenaBacktestMetrics `json:"per_stock"`
	MarketCaps     map[string]int64                `json:"market_caps,omitempty"`
	SkippedSymbols []string                        `json:"skipped_symbols,omitempty"`
}

// backtestDebugHandler runs the same backtest via both the single and batch code paths
// and compares results to diagnose discrepancies.
func backtestDebugHandler(c *gin.Context) {
	var req struct {
		Symbol   string                 `json:"symbol"`
		Strategy string                 `json:"strategy"`
		Interval string                 `json:"interval"`
		Params   map[string]interface{} `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	interval := req.Interval
	if interval == "" {
		interval = "4h"
	}

	// Path A: exactly like runBacktestHandler (single backtest)
	ohlcvSingle, errA := getOHLCVCached(symbol, interval, 24*time.Hour)
	var singleTrades int
	var singleClosed int
	var singleMetrics ArenaBacktestMetrics
	var singleBars int
	var singleFirstBar, singleLastBar int64
	if errA == nil {
		singleBars = len(ohlcvSingle)
		if singleBars > 0 {
			singleFirstBar = ohlcvSingle[0].Time
			singleLastBar = ohlcvSingle[singleBars-1].Time
		}
		stratA, _ := instantiateStrategy(req.Strategy, req.Params)
		resultA := runArenaBacktest(ohlcvSingle, stratA)
		singleTrades = len(resultA.Trades)
		for _, t := range resultA.Trades {
			if !t.IsOpen {
				singleClosed++
			}
		}
		closedOnly := make([]ArenaBacktestTrade, 0)
		for _, t := range resultA.Trades {
			if !t.IsOpen {
				closedOnly = append(closedOnly, t)
			}
		}
		singleMetrics = recalcMetrics(closedOnly)
	}

	// Path B: exactly like backtestWatchlistHandler (batch backtest)
	ohlcvBatch, errB := getOHLCVCached(symbol, interval, 24*time.Hour)
	var batchTrades int
	var batchClosed int
	var batchMetrics ArenaBacktestMetrics
	var batchBars int
	var batchFirstBar, batchLastBar int64
	if errB == nil {
		batchBars = len(ohlcvBatch)
		if batchBars > 0 {
			batchFirstBar = ohlcvBatch[0].Time
			batchLastBar = ohlcvBatch[batchBars-1].Time
		}
		stratB, _ := instantiateStrategy(req.Strategy, req.Params)
		resultB := runArenaBacktest(ohlcvBatch, stratB)
		batchTrades = len(resultB.Trades)
		for _, t := range resultB.Trades {
			if !t.IsOpen {
				batchClosed++
			}
		}
		closedOnly := make([]ArenaBacktestTrade, 0)
		for _, t := range resultB.Trades {
			if !t.IsOpen {
				closedOnly = append(closedOnly, t)
			}
		}
		batchMetrics = recalcMetrics(closedOnly)
	}

	// Check for duplicate timestamps in OHLCV data
	dupes := 0
	if errA == nil {
		seen := make(map[int64]bool)
		for _, bar := range ohlcvSingle {
			if seen[bar.Time] {
				dupes++
			}
			seen[bar.Time] = true
		}
	}

	c.JSON(200, gin.H{
		"symbol":   symbol,
		"interval": interval,
		"strategy": req.Strategy,
		"single": gin.H{
			"error":      fmt.Sprintf("%v", errA),
			"bars":       singleBars,
			"first_bar":  singleFirstBar,
			"last_bar":   singleLastBar,
			"all_trades": singleTrades,
			"closed":     singleClosed,
			"metrics":    singleMetrics,
		},
		"batch": gin.H{
			"error":      fmt.Sprintf("%v", errB),
			"bars":       batchBars,
			"first_bar":  batchFirstBar,
			"last_bar":   batchLastBar,
			"all_trades": batchTrades,
			"closed":     batchClosed,
			"metrics":    batchMetrics,
		},
		"ohlcv_duplicate_timestamps": dupes,
		"data_identical":             fmt.Sprintf("%v", singleBars == batchBars && singleFirstBar == batchFirstBar && singleLastBar == batchLastBar),
	})
}

// recalcMetrics recalculates backtest metrics from a filtered trades slice.
func recalcMetrics(trades []ArenaBacktestTrade) ArenaBacktestMetrics {
	m := ArenaBacktestMetrics{TotalTrades: len(trades)}
	if len(trades) == 0 {
		return m
	}
	totalReturn := 0.0
	var winReturns, lossReturns []float64
	equity := 100.0
	peak := equity
	maxDD := 0.0
	for _, t := range trades {
		totalReturn += t.ReturnPct
		if t.ReturnPct >= 0 {
			m.Wins++
			winReturns = append(winReturns, t.ReturnPct)
		} else {
			m.Losses++
			lossReturns = append(lossReturns, t.ReturnPct)
		}
		equity *= (1 + t.ReturnPct/100)
		if equity > peak {
			peak = equity
		}
		dd := (peak - equity) / peak * 100
		if dd > maxDD {
			maxDD = dd
		}
	}
	m.WinRate = float64(m.Wins) / float64(m.TotalTrades) * 100
	m.TotalReturn = totalReturn
	m.AvgReturn = totalReturn / float64(m.TotalTrades)
	m.MaxDrawdown = maxDD
	m.NetProfit = equity - 100
	if len(winReturns) > 0 && len(lossReturns) > 0 {
		avgWin := 0.0
		for _, w := range winReturns {
			avgWin += w
		}
		avgWin /= float64(len(winReturns))
		avgLoss := 0.0
		for _, l := range lossReturns {
			avgLoss += math.Abs(l)
		}
		avgLoss /= float64(len(lossReturns))
		if avgLoss > 0 {
			m.RiskReward = avgWin / avgLoss
		}
	}
	// Sanitize NaN/Inf (can occur with bad data e.g. EntryPrice=0)
	sanitize := func(v float64) float64 {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0
		}
		return v
	}
	m.WinRate = sanitize(m.WinRate)
	m.TotalReturn = sanitize(m.TotalReturn)
	m.AvgReturn = sanitize(m.AvgReturn)
	m.MaxDrawdown = sanitize(m.MaxDrawdown)
	m.NetProfit = sanitize(m.NetProfit)
	m.RiskReward = sanitize(m.RiskReward)
	return m
}

func backtestWatchlistHandler(c *gin.Context) {
	// Cancel any previous running batch (prevents parallel Yahoo fetches / 429 errors)
	wlBatchMu.Lock()
	if wlBatchCancelFn != nil {
		wlBatchCancelFn()
	}
	ctx, cancel := context.WithCancel(c.Request.Context())
	wlBatchCancelFn = cancel
	wlBatchGen++
	myGen := wlBatchGen
	wlBatchMu.Unlock()
	defer func() {
		wlBatchMu.Lock()
		if wlBatchGen == myGen {
			wlBatchCancelFn = nil
		}
		wlBatchMu.Unlock()
		cancel()
	}()

	var req struct {
		Strategy   string                 `json:"strategy"`
		Interval   string                 `json:"interval"`
		Params     map[string]interface{} `json:"params"`
		USOnly     bool                   `json:"us_only"`
		DataSource string                 `json:"data_source"` // "alpaca" (default) or "yahoo"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Ungültige Anfrage"})
		return
	}

	periodMap := map[string]string{
		"5m": "60d", "15m": "60d", "60m": "2y", "1h": "2y",
		"2h": "2y", "4h": "2y", "1d": "2y", "1wk": "10y",
	}
	interval := req.Interval
	if interval == "" {
		interval = "4h"
	}
	if _, ok := periodMap[interval]; !ok {
		c.JSON(400, gin.H{"error": "Ungültiges Interval"})
		return
	}

	// Validate strategy
	if _, err := instantiateStrategy(req.Strategy, req.Params); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Get trading watchlist symbols
	var allWatchlist []TradingWatchlistItem
	db.Find(&allWatchlist)
	var watchlist []TradingWatchlistItem
	for _, w := range allWatchlist {
		if req.USOnly && isNonUSStock(w.Symbol) {
			continue
		}
		watchlist = append(watchlist, w)
	}
	if len(watchlist) == 0 {
		c.JSON(400, gin.H{"error": "Trading Watchlist ist leer"})
		return
	}

	type stockResult struct {
		Symbol  string
		Trades  []ArenaBacktestTrade
		Metrics ArenaBacktestMetrics
	}

	// SSE streaming for progress
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	total := len(watchlist)

	// Send init event immediately
	initJSON, _ := json.Marshal(gin.H{
		"type": "init", "total": total,
	})
	fmt.Fprintf(c.Writer, "data: %s\n\n", initJSON)
	c.Writer.Flush()

	// === Pre-warm cache: fetch uncached symbols via Yahoo (fast, no rate-limit) ===
	cacheInterval := interval
	switch interval {
	case "1h":
		cacheInterval = "60m"
	case "1D":
		cacheInterval = "1d"
	case "1W":
		cacheInterval = "1wk"
	}

	// Bulk-check which symbols are already cached (single query)
	var cacheEntries []OHLCVCache
	db.Select("symbol, updated_at").Where("interval = ?", cacheInterval).Find(&cacheEntries)
	cacheMap := make(map[string]time.Time, len(cacheEntries))
	for _, ce := range cacheEntries {
		cacheMap[ce.Symbol] = ce.UpdatedAt
	}

	var uncached []string
	for _, w := range watchlist {
		if updatedAt, ok := cacheMap[w.Symbol]; ok && time.Since(updatedAt) < 24*time.Hour {
			continue
		}
		uncached = append(uncached, w.Symbol)
	}

	if len(uncached) > 0 {
		log.Printf("[Arena-Batch] Pre-warming %d uncached symbols via Yahoo", len(uncached))
		prefetchJSON, _ := json.Marshal(gin.H{
			"type": "prefetch", "uncached": len(uncached), "total": total, "source": "Yahoo",
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", prefetchJSON)
		c.Writer.Flush()

		yahooPeriod := backtestPeriodMap[cacheInterval]
		if yahooPeriod == "" {
			yahooPeriod = "60d"
		}
		var ywg sync.WaitGroup
		ysem := make(chan struct{}, 20)
		var yahooDone int64
		var yahooFailed int64
		type yahooProgress struct{ Symbol string }
		yCh := make(chan yahooProgress, len(uncached))
		for _, sym := range uncached {
			if ctx.Err() != nil {
				break
			}
			ywg.Add(1)
			go func(s string) {
				defer ywg.Done()
				select {
				case ysem <- struct{}{}:
				case <-ctx.Done():
					return
				}
				defer func() { <-ysem }()
				if ctx.Err() != nil {
					return
				}
				var ohlcv []OHLCV
				var lastErr error
				for attempt := 0; attempt < 3; attempt++ {
					if ctx.Err() != nil {
						return
					}
					if attempt > 0 {
						time.Sleep(time.Duration(attempt) * 2 * time.Second)
					}
					ohlcv, lastErr = fetchOHLCVFromYahoo(s, yahooPeriod, cacheInterval)
					if lastErr == nil && len(ohlcv) > 0 {
						break
					}
					if lastErr != nil && !strings.Contains(lastErr.Error(), "status 4") {
						break
					}
				}
				if lastErr == nil && len(ohlcv) > 0 {
					saveOHLCVCache(s, cacheInterval, ohlcv)
				} else {
					atomic.AddInt64(&yahooFailed, 1)
				}
				yCh <- yahooProgress{Symbol: s}
			}(sym)
		}
		go func() { ywg.Wait(); close(yCh) }()
		for range yCh {
			done := atomic.AddInt64(&yahooDone, 1)
			pJSON, _ := json.Marshal(gin.H{
				"type": "prefetch_progress", "current": done, "total": len(uncached), "source": "Yahoo",
			})
			fmt.Fprintf(c.Writer, "data: %s\n\n", pJSON)
			c.Writer.Flush()
		}
		if yahooFailed > 0 {
			log.Printf("[Arena-Batch] Yahoo Prefetch: %d/%d fehlgeschlagen", yahooFailed, len(uncached))
			failEvt, _ := json.Marshal(gin.H{"type": "prefetch_warning", "yahoo_failed": yahooFailed, "yahoo_total": len(uncached)})
			fmt.Fprintf(c.Writer, "data: %s\n\n", failEvt)
			c.Writer.Flush()
		}
		log.Printf("[Arena-Batch] Pre-warm complete")
	}

	var completed int64
	results := make([]stockResult, total)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 50) // CPU-only with in-memory cache

	// Progress channel
	type progressMsg struct {
		Index   int
		Symbol  string
		Skipped bool
	}
	progressCh := make(chan progressMsg, total)

	for idx, item := range watchlist {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(i int, symbol string) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Arena] Batch panic for %s: %v", symbol, r)
					progressCh <- progressMsg{Index: i, Symbol: symbol, Skipped: true}
				}
			}()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}

			ohlcv, err := getOHLCVCached(symbol, interval, 0) // cache only — pre-warm already fetched
			if err != nil || len(ohlcv) < 50 {
				progressCh <- progressMsg{Index: i, Symbol: symbol, Skipped: true}
				return
			}
			strategy, _ := instantiateStrategy(req.Strategy, req.Params)
			result := runArenaBacktest(ohlcv, strategy)
			closedTrades := make([]ArenaBacktestTrade, 0)
			for _, t := range result.Trades {
				if !t.IsOpen {
					closedTrades = append(closedTrades, t)
				}
			}
			// Recalculate metrics from closed trades only (consistent with single backtest display)
			metrics := recalcMetrics(closedTrades)
			log.Printf("[Arena-Batch] %s: bars=%d trades=%d closed=%d winrate=%.1f%% interval=%s", symbol, len(ohlcv), len(result.Trades), len(closedTrades), metrics.WinRate, interval)
			results[i] = stockResult{Symbol: symbol, Trades: closedTrades, Metrics: metrics}
			progressCh <- progressMsg{Index: i, Symbol: symbol}
		}(idx, item.Symbol)
	}

	// Drain progress in background, close channel when all done
	go func() {
		wg.Wait()
		close(progressCh)
	}()

	var skippedSymbols []string
	for msg := range progressCh {
		if ctx.Err() != nil {
			continue // drain without writing
		}
		completed++
		if msg.Skipped {
			skippedSymbols = append(skippedSymbols, msg.Symbol)
		}
		progressJSON, _ := json.Marshal(gin.H{
			"type":    "progress",
			"current": completed,
			"total":   total,
			"symbol":  msg.Symbol,
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", progressJSON)
		c.Writer.Flush()
	}

	if ctx.Err() != nil {
		return
	}

	// Aggregate
	var allTrades []WatchlistBacktestTrade
	var allArena []ArenaBacktestTrade
	perStock := make(map[string]ArenaBacktestMetrics)

	for _, sr := range results {
		if sr.Symbol == "" {
			continue
		}
		for _, t := range sr.Trades {
			allTrades = append(allTrades, WatchlistBacktestTrade{
				Symbol: sr.Symbol, Direction: t.Direction,
				EntryPrice: t.EntryPrice, EntryTime: t.EntryTime,
				ExitPrice: t.ExitPrice, ExitTime: t.ExitTime,
				ReturnPct: t.ReturnPct, ExitReason: t.ExitReason, IsOpen: t.IsOpen,
			})
			allArena = append(allArena, t)
		}
		// Use metrics directly from runArenaBacktest (identical to single backtest)
		perStock[sr.Symbol] = sr.Metrics
	}

	aggregated := calculateBacktestLabMetrics(allArena)

	// Fetch market caps for all symbols
	symbols := make([]string, 0, len(perStock))
	for sym := range perStock {
		symbols = append(symbols, sym)
	}
	marketCaps := make(map[string]int64)
	if len(symbols) > 0 {
		var stocks []Stock
		db.Where("symbol IN ?", symbols).Select("symbol, market_cap").Find(&stocks)
		for _, s := range stocks {
			if s.MarketCap > 0 {
				marketCaps[s.Symbol] = s.MarketCap
			}
		}
	}

	resultJSON, _ := json.Marshal(gin.H{
		"type": "result",
		"data": WatchlistBacktestResult{
			Metrics:        aggregated,
			Trades:         allTrades,
			PerStock:       perStock,
			MarketCaps:     marketCaps,
			SkippedSymbols: skippedSymbols,
		},
	})
	fmt.Fprintf(c.Writer, "data: %s\n\n", resultJSON)
	c.Writer.Flush()
}

// ==================== Arena v2 Handlers ====================

func arenaV2BatchHandler(c *gin.Context) {
	// Cancel any previous running batch (prevents parallel Yahoo fetches / 429 errors)
	arenaV2BatchMu.Lock()
	if arenaV2CancelFn != nil {
		arenaV2CancelFn()
	}
	ctx, cancel := context.WithCancel(c.Request.Context())
	arenaV2CancelFn = cancel
	arenaV2BatchGen++
	myGen := arenaV2BatchGen
	arenaV2BatchMu.Unlock()
	defer func() {
		arenaV2BatchMu.Lock()
		if arenaV2BatchGen == myGen {
			arenaV2CancelFn = nil
		}
		arenaV2BatchMu.Unlock()
		cancel()
	}()

	var req struct {
		Strategy     string                 `json:"strategy"`
		Interval     string                 `json:"interval"`
		Params       map[string]interface{} `json:"params"`
		USOnly       bool                   `json:"us_only"`
		LongOnly     bool                   `json:"long_only"`
		TradesFrom   int64                  `json:"trades_from"`
		MinWinrate   float64                `json:"min_winrate"`
		MinRR        float64                `json:"min_rr"`
		MinAvgReturn float64                `json:"min_avg_return"`
		MinMarketCap int64                  `json:"min_market_cap"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Ungültige Anfrage"})
		return
	}

	periodMap := map[string]string{
		"5m": "60d", "15m": "60d", "60m": "2y", "1h": "2y",
		"2h": "2y", "4h": "2y", "1d": "2y", "1wk": "10y",
	}
	interval := req.Interval
	if interval == "" {
		interval = "4h"
	}
	if _, ok := periodMap[interval]; !ok {
		c.JSON(400, gin.H{"error": "Ungültiges Interval"})
		return
	}
	if _, err := instantiateStrategy(req.Strategy, req.Params); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var allWatchlist []TradingWatchlistItem
	db.Find(&allWatchlist)
	var watchlist []TradingWatchlistItem
	for _, w := range allWatchlist {
		if req.USOnly && isNonUSStock(w.Symbol) {
			continue
		}
		watchlist = append(watchlist, w)
	}
	if len(watchlist) == 0 {
		c.JSON(400, gin.H{"error": "Trading Watchlist ist leer"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	total := len(watchlist)

	bulkCacheInterval := interval
	switch interval {
	case "1h":
		bulkCacheInterval = "60m"
	case "1D":
		bulkCacheInterval = "1d"
	case "1W":
		bulkCacheInterval = "1wk"
	}

	// Use in-memory cache to determine cached vs uncached symbols
	// Validate that cached data has enough bars (≥50), otherwise re-fetch
	cachedSymbols := getOHLCVMemCacheSymbols(bulkCacheInterval)

	var uncachedSymbols []string
	validCached := 0
	for _, w := range watchlist {
		if cachedSymbols[w.Symbol] {
			if bars, ok := getOHLCVFromMemCache(w.Symbol, bulkCacheInterval); ok && len(bars) >= 50 {
				validCached++
			} else {
				uncachedSymbols = append(uncachedSymbols, w.Symbol)
			}
		} else {
			uncachedSymbols = append(uncachedSymbols, w.Symbol)
		}
	}

	cacheEvt, _ := json.Marshal(gin.H{
		"type": "cache_loaded", "cached": validCached, "total": total, "uncached": len(uncachedSymbols),
	})
	fmt.Fprintf(c.Writer, "data: %s\n\n", cacheEvt)
	c.Writer.Flush()

	if len(uncachedSymbols) > 0 {
		var sseMu sync.Mutex
		var prewarmDone int64

		prefetchEvt, _ := json.Marshal(gin.H{"type": "prefetch", "uncached": len(uncachedSymbols)})
		fmt.Fprintf(c.Writer, "data: %s\n\n", prefetchEvt)
		c.Writer.Flush()

		sendProgress := func(done int64) {
			sseMu.Lock()
			pJSON, _ := json.Marshal(gin.H{"type": "prefetch_progress", "current": done, "total": len(uncachedSymbols)})
			fmt.Fprintf(c.Writer, "data: %s\n\n", pJSON)
			c.Writer.Flush()
			sseMu.Unlock()
		}

		// Yahoo-only prefetch (fast, no rate-limit, 20 concurrent)
		var yahooWg sync.WaitGroup
		yahooSem := make(chan struct{}, 20)
		var yahooFailed int64
		for _, sym := range uncachedSymbols {
			if ctx.Err() != nil {
				break
			}
			yahooWg.Add(1)
			go func(s string) {
				defer yahooWg.Done()
				select {
				case yahooSem <- struct{}{}:
				case <-ctx.Done():
					return
				}
				defer func() { <-yahooSem }()
				if ctx.Err() != nil {
					return
				}
				yahooIv := bulkCacheInterval
				period := backtestPeriodMap[yahooIv]
				if period == "" {
					period = "60d"
				}
				var ohlcv []OHLCV
				var lastErr error
				for attempt := 0; attempt < 3; attempt++ {
					if ctx.Err() != nil {
						return
					}
					if attempt > 0 {
						time.Sleep(time.Duration(attempt) * 2 * time.Second)
					}
					ohlcv, lastErr = fetchOHLCVFromYahoo(s, period, yahooIv)
					if lastErr == nil && len(ohlcv) > 0 {
						break
					}
					if lastErr != nil && !strings.Contains(lastErr.Error(), "status 4") {
						break
					}
				}
				if lastErr == nil && len(ohlcv) > 0 {
					saveOHLCVCache(s, bulkCacheInterval, ohlcv)
				} else {
					atomic.AddInt64(&yahooFailed, 1)
				}
				done := atomic.AddInt64(&prewarmDone, 1)
				sendProgress(done)
			}(sym)
		}
		yahooWg.Wait()
		if yahooFailed > 0 {
			log.Printf("[Arena Batch] Yahoo Prefetch: %d/%d fehlgeschlagen", yahooFailed, len(uncachedSymbols))
			sseMu.Lock()
			failEvt, _ := json.Marshal(gin.H{"type": "prefetch_warning", "yahoo_failed": yahooFailed, "yahoo_total": len(uncachedSymbols)})
			fmt.Fprintf(c.Writer, "data: %s\n\n", failEvt)
			c.Writer.Flush()
			sseMu.Unlock()
		}
	}

	// Fetch market caps
	v2AllSymbols := make([]string, 0, len(watchlist))
	for _, w := range watchlist {
		v2AllSymbols = append(v2AllSymbols, w.Symbol)
	}
	marketCaps := make(map[string]int64)
	if len(v2AllSymbols) > 0 {
		var stocks []Stock
		db.Where("symbol IN ?", v2AllSymbols).Select("symbol, market_cap").Find(&stocks)
		for _, s := range stocks {
			if s.MarketCap > 0 {
				marketCaps[s.Symbol] = s.MarketCap
			}
		}
	}

	type stockResult struct {
		Symbol  string
		Trades  []ArenaBacktestTrade
		Metrics ArenaBacktestMetrics
	}

	var completed int64
	results := make([]stockResult, total)
	var wg sync.WaitGroup
	sem := make(chan struct{}, 50) // CPU-only with in-memory cache

	type progressMsg struct {
		Index   int
		Symbol  string
		Skipped bool
	}
	progressCh := make(chan progressMsg, total)

	for idx, item := range watchlist {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(i int, symbol string) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					progressCh <- progressMsg{Index: i, Symbol: symbol, Skipped: true}
				}
			}()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}

			// Read directly from in-memory cache (no JSON unmarshal, no DB hit)
			ohlcv, ok := getOHLCVFromMemCache(symbol, bulkCacheInterval)
			if !ok || len(ohlcv) < 50 {
				progressCh <- progressMsg{Index: i, Symbol: symbol, Skipped: true}
				return
			}
			strategy, _ := instantiateStrategy(req.Strategy, req.Params)
			result := runArenaBacktest(ohlcv, strategy)

			closedTrades := make([]ArenaBacktestTrade, 0)
			for _, t := range result.Trades {
				if !t.IsOpen {
					if req.TradesFrom > 0 && t.EntryTime < req.TradesFrom {
						continue
					}
					closedTrades = append(closedTrades, t)
				}
			}

			if req.LongOnly {
				filtered := make([]ArenaBacktestTrade, 0, len(closedTrades))
				for _, t := range closedTrades {
					if t.Direction == "long" {
						filtered = append(filtered, t)
					}
				}
				closedTrades = filtered
			}

			metrics := recalcMetrics(closedTrades)
			results[i] = stockResult{Symbol: symbol, Trades: closedTrades, Metrics: metrics}
			progressCh <- progressMsg{Index: i, Symbol: symbol}
		}(idx, item.Symbol)
	}

	go func() {
		wg.Wait()
		close(progressCh)
	}()

	var skippedSymbols []string
	for msg := range progressCh {
		if ctx.Err() != nil {
			// Drain remaining messages without writing to cancelled client
			continue
		}
		completed++
		if msg.Skipped {
			skippedSymbols = append(skippedSymbols, msg.Symbol)
		}
		progressJSON, _ := json.Marshal(gin.H{
			"type": "progress", "current": completed, "total": total, "symbol": msg.Symbol,
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", progressJSON)
		c.Writer.Flush()
	}

	// If cancelled (strategy switch or client disconnect), skip result/DB write
	if ctx.Err() != nil {
		return
	}

	var allTrades []WatchlistBacktestTrade
	var allArena []ArenaBacktestTrade
	perStock := make(map[string]ArenaBacktestMetrics)
	var filteredCount int
	var totalCount int

	var batchInserts []ArenaV2BatchResult
	for _, sr := range results {
		if sr.Symbol == "" {
			continue
		}
		totalCount++

		if req.MinWinrate > 0 && sr.Metrics.WinRate < req.MinWinrate {
			continue
		}
		if req.MinRR > 0 && sr.Metrics.RiskReward < req.MinRR {
			continue
		}
		if req.MinAvgReturn > 0 && sr.Metrics.AvgReturn < req.MinAvgReturn {
			continue
		}
		if req.MinMarketCap > 0 {
			if mc, ok := marketCaps[sr.Symbol]; !ok || mc < req.MinMarketCap {
				continue
			}
		}
		filteredCount++

		metricsJSON, _ := json.Marshal(sr.Metrics)
		tradesJSON, _ := json.Marshal(sr.Trades)
		mc := marketCaps[sr.Symbol]
		batchInserts = append(batchInserts, ArenaV2BatchResult{
			Symbol: sr.Symbol, Strategy: req.Strategy, Interval: interval,
			MetricsJSON: string(metricsJSON), TradesJSON: string(tradesJSON),
			MarketCap: mc, UpdatedAt: time.Now(),
		})

		for _, t := range sr.Trades {
			allTrades = append(allTrades, WatchlistBacktestTrade{
				Symbol: sr.Symbol, Direction: t.Direction,
				EntryPrice: t.EntryPrice, EntryTime: t.EntryTime,
				ExitPrice: t.ExitPrice, ExitTime: t.ExitTime,
				ReturnPct: t.ReturnPct, ExitReason: t.ExitReason, IsOpen: t.IsOpen,
			})
			allArena = append(allArena, t)
		}
		perStock[sr.Symbol] = sr.Metrics
	}

	// Batch-INSERT: delete old results for this strategy+interval, then insert all at once
	if len(batchInserts) > 0 {
		db.Transaction(func(tx *gorm.DB) error {
			tx.Where("strategy = ? AND interval = ?", req.Strategy, interval).Delete(&ArenaV2BatchResult{})
			// Insert in chunks of 100 to stay within SQLite variable limits
			for i := 0; i < len(batchInserts); i += 100 {
				end := i + 100
				if end > len(batchInserts) {
					end = len(batchInserts)
				}
				tx.Create(batchInserts[i:end])
			}
			return nil
		})
	}

	aggregated := calculateBacktestLabMetrics(allArena)

	resultJSON, _ := json.Marshal(gin.H{
		"type": "result",
		"data": gin.H{
			"metrics":         aggregated,
			"trades":          allTrades,
			"per_stock":       perStock,
			"market_caps":     marketCaps,
			"skipped_symbols": skippedSymbols,
			"filtered_count":  filteredCount,
			"total_count":     totalCount,
		},
	})
	fmt.Fprintf(c.Writer, "data: %s\n\n", resultJSON)
	c.Writer.Flush()
}

func arenaV2WatchlistGrid(c *gin.Context) {
	strategy := c.Query("strategy")
	interval := c.Query("interval")
	if strategy == "" || interval == "" {
		c.JSON(400, gin.H{"error": "strategy und interval sind erforderlich"})
		return
	}

	var results []ArenaV2BatchResult
	db.Where("strategy = ? AND interval = ?", strategy, interval).Find(&results)

	type gridItem struct {
		Symbol    string               `json:"symbol"`
		Metrics   ArenaBacktestMetrics `json:"metrics"`
		MarketCap int64                `json:"market_cap"`
		UpdatedAt time.Time            `json:"updated_at"`
	}

	items := make([]gridItem, 0, len(results))
	for _, r := range results {
		var metrics ArenaBacktestMetrics
		json.Unmarshal([]byte(r.MetricsJSON), &metrics)
		items = append(items, gridItem{
			Symbol:    r.Symbol,
			Metrics:   metrics,
			MarketCap: r.MarketCap,
			UpdatedAt: r.UpdatedAt,
		})
	}

	c.JSON(200, gin.H{"items": items})
}

func arenaV2StartSession(c *gin.Context) {
	uid := liveOwnerUID(c)

	var req struct {
		Name        string   `json:"name"`
		Strategy    string   `json:"strategy"`
		Interval    string   `json:"interval"`
		ParamsJSON  string   `json:"params_json"`
		ConfigID    uint     `json:"config_id"`
		LongOnly    bool     `json:"long_only"`
		TradeAmount float64  `json:"trade_amount"`
		Symbols     []string `json:"symbols"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Ungültige Anfrage"})
		return
	}

	if len(req.Symbols) == 0 {
		c.JSON(400, gin.H{"error": "Mindestens ein Symbol erforderlich"})
		return
	}

	symbolsJSON, _ := json.Marshal(req.Symbols)

	newConfig := LiveTradingConfig{
		UserID:      uid,
		Strategy:    req.Strategy,
		Interval:    req.Interval,
		ParamsJSON:  req.ParamsJSON,
		Symbols:     string(symbolsJSON),
		LongOnly:    req.LongOnly,
		TradeAmount: req.TradeAmount,
		AlpacaPaper: true,
		UpdatedAt:   time.Now(),
	}

	if req.ConfigID > 0 {
		var existingConfig LiveTradingConfig
		if db.First(&existingConfig, req.ConfigID).Error == nil {
			newConfig.AlpacaAccountID = existingConfig.AlpacaAccountID
			newConfig.AlpacaApiKey = existingConfig.AlpacaApiKey
			newConfig.AlpacaSecretKey = existingConfig.AlpacaSecretKey
			newConfig.AlpacaEnabled = existingConfig.AlpacaEnabled
			newConfig.AlpacaPaper = existingConfig.AlpacaPaper
			newConfig.Currency = existingConfig.Currency
		}
	}
	db.Create(&newConfig)

	sessionName := strings.TrimSpace(req.Name)
	if sessionName == "" {
		var sessionCount int64
		db.Model(&LiveTradingSession{}).Where("user_id = ?", uid).Count(&sessionCount)
		sessionName = fmt.Sprintf("#%d %s (%s)", sessionCount+1, req.Strategy, req.Interval)
	}

	now := time.Now()
	session := LiveTradingSession{
		UserID:      uid,
		ConfigID:    newConfig.ID,
		Name:        sessionName,
		Strategy:    req.Strategy,
		Interval:    req.Interval,
		ParamsJSON:  req.ParamsJSON,
		Symbols:     string(symbolsJSON),
		LongOnly:    req.LongOnly,
		TradeAmount: req.TradeAmount,
		Currency:    newConfig.Currency,
		IsActive:    false,
		CreatedAt:   now,
	}
	db.Create(&session)
	db.Model(&session).Update("is_active", false)

	logLiveEvent(session.ID, "INFO", "-", fmt.Sprintf("Arena v2 Session '%s' erstellt — %d Symbole, %s %s", session.Name, len(req.Symbols), req.Strategy, req.Interval))

	c.JSON(200, gin.H{"session": session, "status": "created"})
}

func getStrategySettings(c *gin.Context) {
	symbol := c.Query("symbol") // optional: per-symbol settings

	// Load global settings (symbol="")
	var globalSettings []ArenaStrategySettings
	db.Where("symbol = ?", "").Find(&globalSettings)

	out := make(map[string]interface{})
	for _, s := range globalSettings {
		var params map[string]interface{}
		json.Unmarshal([]byte(s.ParamsJSON), &params)
		out[s.Strategy] = gin.H{
			"params":   params,
			"interval": s.Interval,
			"symbol":   "",
		}
	}

	// Overlay with per-symbol settings if requested
	if symbol != "" {
		var symbolSettings []ArenaStrategySettings
		db.Where("symbol = ?", symbol).Find(&symbolSettings)
		for _, s := range symbolSettings {
			var params map[string]interface{}
			json.Unmarshal([]byte(s.ParamsJSON), &params)
			// Merge: start from global, overlay symbol-specific
			if existing, ok := out[s.Strategy]; ok {
				if existMap, ok := existing.(gin.H); ok {
					if globalParams, ok := existMap["params"].(map[string]interface{}); ok {
						for k, v := range globalParams {
							if _, has := params[k]; !has {
								params[k] = v
							}
						}
					}
				}
			}
			out[s.Strategy] = gin.H{
				"params":   params,
				"interval": s.Interval,
				"symbol":   symbol,
			}
		}
	}
	c.JSON(200, out)
}

func saveStrategySettings(c *gin.Context) {
	var req struct {
		Symbol   string                 `json:"symbol"`
		Strategy string                 `json:"strategy"`
		Params   map[string]interface{} `json:"params"`
		Interval string                 `json:"interval"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Strategy == "" {
		c.JSON(400, gin.H{"error": "Strategie erforderlich"})
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol)) // "" = global
	paramsJSON, _ := json.Marshal(req.Params)

	var existing ArenaStrategySettings
	if db.Where("symbol = ? AND strategy = ?", symbol, req.Strategy).First(&existing).Error == nil {
		existing.ParamsJSON = string(paramsJSON)
		if req.Interval != "" {
			existing.Interval = req.Interval
		}
		existing.UpdatedAt = time.Now()
		db.Save(&existing)
	} else {
		db.Create(&ArenaStrategySettings{
			Symbol:     symbol,
			Strategy:   req.Strategy,
			ParamsJSON: string(paramsJSON),
			Interval:   req.Interval,
			UpdatedAt:  time.Now(),
		})
	}
	c.JSON(200, gin.H{"status": "ok"})
}

// fetchOHLCVFromYahoo fetches OHLCV data from Yahoo Finance
func fetchOHLCVFromYahoo(symbol, period, interval string) ([]OHLCV, error) {
	client, crumb, err := getYahooCrumbClient()
	if err != nil {
		return nil, err
	}

	yahooURL := fmt.Sprintf("https://query2.finance.yahoo.com/v8/finance/chart/%s?range=%s&interval=%s&crumb=%s",
		symbol, period, interval, crumb)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", yahooURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			yahooCrumbMu.Lock()
			yahooCrumb = ""
			yahooAuthClient = nil
			yahooCrumbMu.Unlock()
		}
		return nil, fmt.Errorf("yahoo %s status %d", symbol, resp.StatusCode)
	}

	var chartResp YahooChartResponse
	if err := json.Unmarshal(body, &chartResp); err != nil {
		return nil, err
	}

	if len(chartResp.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data for %s", symbol)
	}

	chartResult := chartResp.Chart.Result[0]
	timestamps := chartResult.Timestamp
	if len(timestamps) == 0 || chartResult.Indicators.Quote == nil || len(chartResult.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("empty data for %s", symbol)
	}

	quote := chartResult.Indicators.Quote[0]
	var ohlcv []OHLCV

	for i := range timestamps {
		if i >= len(quote.Close) || quote.Close[i] == 0 {
			continue
		}
		bar := OHLCV{
			Time:   timestamps[i],
			Open:   quote.Open[i],
			High:   quote.High[i],
			Low:    quote.Low[i],
			Close:  quote.Close[i],
			Volume: quote.Volume[i],
		}
		ohlcv = append(ohlcv, bar)
	}

	if interval == "2h" {
		ohlcv = aggregateOHLCV(ohlcv, 2)
	} else if interval == "4h" {
		ohlcv = aggregateOHLCV(ohlcv, 4)
	}

	return ohlcv, nil
}

// ==================== Alpaca Data API ====================

type AlpacaBar struct {
	T  string  `json:"t"`  // RFC3339 timestamp
	O  float64 `json:"o"`
	H  float64 `json:"h"`
	L  float64 `json:"l"`
	C  float64 `json:"c"`
	V  float64 `json:"v"`
	N  int     `json:"n"`
	VW float64 `json:"vw"`
}

type AlpacaBarsResponse struct {
	Bars          []AlpacaBar `json:"bars"`
	NextPageToken string      `json:"next_page_token"`
}

var alpacaIntervalMap = map[string]string{
	"5m": "5Min", "15m": "15Min", "1h": "1Hour", "60m": "1Hour",
	"2h": "2Hour", "4h": "4Hour",
	"1d": "1Day", "1D": "1Day", "1wk": "1Week", "1W": "1Week",
}

var alpacaLookbackMap = map[string]time.Duration{
	"5Min": 60 * 24 * time.Hour, "15Min": 60 * 24 * time.Hour,
	"1Hour": 2 * 365 * 24 * time.Hour, "2Hour": 2 * 365 * 24 * time.Hour, "4Hour": 2 * 365 * 24 * time.Hour,
	"1Day": 5 * 365 * 24 * time.Hour, "1Week": 10 * 365 * 24 * time.Hour,
}

func fetchOHLCVFromAlpaca(symbol, interval string) ([]OHLCV, error) {
	if alpacaDataKey == "" || alpacaDataSecret == "" {
		return nil, fmt.Errorf("alpaca data keys not configured")
	}

	alpacaInterval, ok := alpacaIntervalMap[interval]
	if !ok {
		return nil, fmt.Errorf("unsupported alpaca interval: %s", interval)
	}

	lookback := alpacaLookbackMap[alpacaInterval]
	if lookback == 0 {
		lookback = 60 * 24 * time.Hour
	}
	start := time.Now().Add(-lookback).Format(time.RFC3339)

	var allBars []OHLCV
	pageToken := ""

	for {
		if alpacaRateTicker != nil {
			<-alpacaRateTicker.C
		}

		reqURL := fmt.Sprintf("https://data.alpaca.markets/v2/stocks/%s/bars?timeframe=%s&start=%s&limit=10000&feed=iex&adjustment=split",
			symbol, alpacaInterval, url.QueryEscape(start))
		if pageToken != "" {
			reqURL += "&page_token=" + url.QueryEscape(pageToken)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		req.Header.Set("APCA-API-KEY-ID", alpacaDataKey)
		req.Header.Set("APCA-API-SECRET-KEY", alpacaDataSecret)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("alpaca request failed: %v", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("alpaca returned status %d: %s", resp.StatusCode, string(body))
		}

		var barsResp AlpacaBarsResponse
		if err := json.Unmarshal(body, &barsResp); err != nil {
			return nil, fmt.Errorf("alpaca parse error: %v", err)
		}

		for _, bar := range barsResp.Bars {
			t, err := time.Parse(time.RFC3339, bar.T)
			if err != nil {
				continue
			}
			allBars = append(allBars, OHLCV{
				Time: t.Unix(), Open: bar.O, High: bar.H,
				Low: bar.L, Close: bar.C, Volume: bar.V,
			})
		}

		if barsResp.NextPageToken == "" {
			break
		}
		pageToken = barsResp.NextPageToken
	}

	if len(allBars) == 0 {
		return nil, fmt.Errorf("no alpaca data for %s/%s", symbol, interval)
	}

	return allBars, nil
}

// fetchOHLCVBatchFromAlpaca fetches bars for multiple symbols in one request using
// the multi-symbol endpoint. Returns a map of symbol → []OHLCV.
// Alpaca allows up to ~50 symbols per request on the multi-bar endpoint.
func fetchOHLCVBatchFromAlpaca(symbols []string, interval string) (map[string][]OHLCV, error) {
	if alpacaDataKey == "" || alpacaDataSecret == "" {
		return nil, fmt.Errorf("alpaca data keys not configured")
	}
	alpacaInterval, ok := alpacaIntervalMap[interval]
	if !ok {
		return nil, fmt.Errorf("unsupported alpaca interval: %s", interval)
	}
	lookback := alpacaLookbackMap[alpacaInterval]
	if lookback == 0 {
		lookback = 60 * 24 * time.Hour
	}
	start := time.Now().Add(-lookback).Format(time.RFC3339)

	result := make(map[string][]OHLCV)
	symbolsParam := strings.Join(symbols, ",")
	pageToken := ""

	for {
		reqURL := fmt.Sprintf("https://data.alpaca.markets/v2/stocks/bars?symbols=%s&timeframe=%s&start=%s&limit=10000&feed=iex&adjustment=split",
			url.QueryEscape(symbolsParam), alpacaInterval, url.QueryEscape(start))
		if pageToken != "" {
			reqURL += "&page_token=" + url.QueryEscape(pageToken)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		req.Header.Set("APCA-API-KEY-ID", alpacaDataKey)
		req.Header.Set("APCA-API-SECRET-KEY", alpacaDataSecret)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("alpaca batch request failed: %v", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if resp.StatusCode == 429 {
			time.Sleep(2 * time.Second) // rate limited — back off and retry
			continue
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("alpaca batch returned status %d: %s", resp.StatusCode, string(body))
		}

		// Multi-symbol response: { "bars": { "AAPL": [...], "MSFT": [...] }, "next_page_token": "..." }
		var raw struct {
			Bars          map[string][]AlpacaBar `json:"bars"`
			NextPageToken string                 `json:"next_page_token"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("alpaca batch parse error: %v", err)
		}

		for sym, bars := range raw.Bars {
			for _, bar := range bars {
				t, err := time.Parse(time.RFC3339, bar.T)
				if err != nil {
					continue
				}
				result[sym] = append(result[sym], OHLCV{
					Time: t.Unix(), Open: bar.O, High: bar.H,
					Low: bar.L, Close: bar.C, Volume: bar.V,
				})
			}
		}

		if raw.NextPageToken == "" {
			break
		}
		pageToken = raw.NextPageToken
	}

	return result, nil
}

func isNonUSStock(symbol string) bool {
	return strings.Contains(symbol, ".")
}

// ==================== Alpaca WebSocket Client ====================

type AlpacaWSBar struct {
	T  string  `json:"t"`
	S  string  `json:"S"`
	O  float64 `json:"o"`
	H  float64 `json:"h"`
	L  float64 `json:"l"`
	C  float64 `json:"c"`
	V  float64 `json:"v"`
}

// alpacaWSMessage is used for direct unmarshaling of Alpaca WebSocket messages (avoids double-parse)
type alpacaWSMessage struct {
	T string  `json:"T"` // Message type: "b" = bar
	Ts string `json:"t"` // Timestamp (RFC3339)
	S string  `json:"S"` // Symbol
	O float64 `json:"o"`
	H float64 `json:"h"`
	L float64 `json:"l"`
	C float64 `json:"c"`
	V float64 `json:"v"`
}

type AlpacaWSClient struct {
	conn          *websocket.Conn
	mu            sync.Mutex
	apiKey        string
	secretKey     string
	subscriptions map[string]bool
	barHandlers   map[string][]func(AlpacaWSBar)
	handlerMu     sync.RWMutex
	stopChan      chan struct{}
	isConnected   bool
	reconnectWait time.Duration
}

func newAlpacaWSClient(key, secret string) (*AlpacaWSClient, error) {
	client := &AlpacaWSClient{
		apiKey:        key,
		secretKey:     secret,
		subscriptions: make(map[string]bool),
		barHandlers:   make(map[string][]func(AlpacaWSBar)),
		stopChan:      make(chan struct{}),
		reconnectWait: 1 * time.Second,
	}
	if err := client.connect(); err != nil {
		return nil, err
	}
	go client.readPump()
	return client, nil
}

func (c *AlpacaWSClient) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.Dial("wss://stream.data.alpaca.markets/v2/iex", nil)
	if err != nil {
		return fmt.Errorf("alpaca ws dial failed: %v", err)
	}
	c.conn = conn

	// Read welcome
	_, _, err = conn.ReadMessage()
	if err != nil {
		conn.Close()
		return fmt.Errorf("alpaca ws welcome read failed: %v", err)
	}

	// Auth
	authMsg := map[string]string{"action": "auth", "key": c.apiKey, "secret": c.secretKey}
	if err := conn.WriteJSON(authMsg); err != nil {
		conn.Close()
		return fmt.Errorf("alpaca ws auth write failed: %v", err)
	}

	_, authResp, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return fmt.Errorf("alpaca ws auth read failed: %v", err)
	}

	var authMsgs []map[string]interface{}
	if json.Unmarshal(authResp, &authMsgs) == nil && len(authMsgs) > 0 {
		if msg, ok := authMsgs[0]["msg"]; ok && msg == "authenticated" {
			c.isConnected = true
			c.reconnectWait = 1 * time.Second
			log.Println("[AlpacaWS] Authenticated successfully")
			return nil
		}
	}

	conn.Close()
	return fmt.Errorf("alpaca ws auth failed: %s", string(authResp))
}

func (c *AlpacaWSClient) Subscribe(symbols []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil || !c.isConnected {
		return fmt.Errorf("not connected")
	}

	var newSymbols []string
	for _, s := range symbols {
		if !c.subscriptions[s] {
			newSymbols = append(newSymbols, s)
			c.subscriptions[s] = true
		}
	}
	if len(newSymbols) == 0 {
		return nil
	}

	msg := map[string]interface{}{"action": "subscribe", "bars": newSymbols}
	if err := c.conn.WriteJSON(msg); err != nil {
		return err
	}
	log.Printf("[AlpacaWS] Subscribed to %d symbols (total: %d)", len(newSymbols), len(c.subscriptions))
	return nil
}

func (c *AlpacaWSClient) Unsubscribe(symbols []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	for _, s := range symbols {
		delete(c.subscriptions, s)
	}
	return c.conn.WriteJSON(map[string]interface{}{"action": "unsubscribe", "bars": symbols})
}

func (c *AlpacaWSClient) OnBar(symbol string, handler func(AlpacaWSBar)) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.barHandlers[symbol] = append(c.barHandlers[symbol], handler)
}

func (c *AlpacaWSClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isConnected
}

func (c *AlpacaWSClient) Close() {
	select {
	case <-c.stopChan:
		return
	default:
		close(c.stopChan)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.isConnected = false
	}
}

// ==================== Shared WebSocket Manager ====================

type SharedWSManager struct {
	mu              sync.Mutex
	client          *AlpacaWSClient
	refCount        map[string]int                        // symbol → number of sessions needing it
	sessionHandlers map[uint]map[string]func(AlpacaWSBar) // sessionID → symbol → handler
}

var sharedWS *SharedWSManager

func initSharedWS() {
	if alpacaDataKey == "" || alpacaDataSecret == "" {
		return
	}
	if os.Getenv("ALPACA_WS_DISABLED") == "true" {
		log.Println("[SharedWS] WebSocket deaktiviert (ALPACA_WS_DISABLED=true) — nur REST-API für Kursdaten")
		return
	}
	client, err := newAlpacaWSClient(alpacaDataKey, alpacaDataSecret)
	if err != nil {
		log.Printf("[SharedWS] Verbindung fehlgeschlagen: %v — Sessions nutzen eigene Clients", err)
		return
	}
	sharedWS = &SharedWSManager{
		client:          client,
		refCount:        make(map[string]int),
		sessionHandlers: make(map[uint]map[string]func(AlpacaWSBar)),
	}
	log.Println("[SharedWS] Globaler WebSocket connected")
}

// loadAlpacaWSKeys reads the default AlpacaAccount from DB and sets the global data keys.
func loadAlpacaWSKeys() {
	var acc AlpacaAccount
	if db.Where("is_default = ?", true).First(&acc).Error != nil {
		log.Println("[Alpaca] Kein Default-Account in DB — Alpaca Data API deaktiviert")
		return
	}
	alpacaDataKey = acc.ApiKey
	alpacaDataSecret = acc.SecretKey
	if alpacaDataKey != "" && alpacaDataSecret != "" {
		alpacaRateTicker = time.NewTicker(350 * time.Millisecond)
		log.Printf("[Alpaca] Data API Keys geladen von Account '%s' (ID %d)", acc.Name, acc.ID)
	}
}

func (m *SharedWSManager) IsConnected() bool {
	if m == nil || m.client == nil {
		return false
	}
	return m.client.IsConnected()
}

func (m *SharedWSManager) AddSession(sessionID uint, symbols []string, handler func(string, AlpacaWSBar)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sessionHandlers[sessionID] != nil {
		return // already registered
	}

	handlers := make(map[string]func(AlpacaWSBar), len(symbols))
	var newSymbols []string

	for _, sym := range symbols {
		symbol := sym
		handlers[symbol] = func(bar AlpacaWSBar) {
			handler(symbol, bar)
		}
		prev := m.refCount[symbol]
		m.refCount[symbol] = prev + 1
		if prev == 0 {
			newSymbols = append(newSymbols, symbol)
			// Register dispatch handler for this symbol on the WSClient
			m.client.OnBar(symbol, func(bar AlpacaWSBar) {
				m.dispatch(symbol, bar)
			})
		}
	}

	m.sessionHandlers[sessionID] = handlers

	if len(newSymbols) > 0 {
		if err := m.client.Subscribe(newSymbols); err != nil {
			log.Printf("[SharedWS] Subscribe fehlgeschlagen für %d Symbole: %v", len(newSymbols), err)
		}
	}

	log.Printf("[SharedWS] AddSession #%d: subscribed %d new symbols (total: %d)", sessionID, len(newSymbols), len(m.refCount))
}

func (m *SharedWSManager) RemoveSession(sessionID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	handlers, ok := m.sessionHandlers[sessionID]
	if !ok {
		return
	}
	delete(m.sessionHandlers, sessionID)

	var unsubSymbols []string
	for symbol := range handlers {
		m.refCount[symbol]--
		if m.refCount[symbol] <= 0 {
			delete(m.refCount, symbol)
			unsubSymbols = append(unsubSymbols, symbol)
		}
	}

	if len(unsubSymbols) > 0 {
		if err := m.client.Unsubscribe(unsubSymbols); err != nil {
			log.Printf("[SharedWS] Unsubscribe fehlgeschlagen für %d Symbole: %v", len(unsubSymbols), err)
		}
		// Clean up bar handlers for symbols no longer needed
		m.client.handlerMu.Lock()
		for _, sym := range unsubSymbols {
			delete(m.client.barHandlers, sym)
		}
		m.client.handlerMu.Unlock()
	}

	log.Printf("[SharedWS] RemoveSession #%d: unsubscribed %d symbols (total: %d)", sessionID, len(unsubSymbols), len(m.refCount))
}

// AddSymbols subscribes new symbols on the shared WebSocket (for hot-add of strategies)
func (m *SharedWSManager) AddSymbols(symbols []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var newSymbols []string
	for _, sym := range symbols {
		symbol := sym
		prev := m.refCount[symbol]
		m.refCount[symbol] = prev + 1
		if prev == 0 {
			newSymbols = append(newSymbols, symbol)
			m.client.OnBar(symbol, func(bar AlpacaWSBar) {
				m.dispatch(symbol, bar)
			})
		}
	}
	if len(newSymbols) > 0 {
		if err := m.client.Subscribe(newSymbols); err != nil {
			log.Printf("[SharedWS] AddSymbols Subscribe fehlgeschlagen: %v", err)
		} else {
			log.Printf("[SharedWS] AddSymbols: %d neue Symbole subscribed", len(newSymbols))
		}
	}
}

func (m *SharedWSManager) dispatch(symbol string, bar AlpacaWSBar) {
	m.mu.Lock()
	handlers := make([]func(AlpacaWSBar), 0)
	for _, symHandlers := range m.sessionHandlers {
		if h, ok := symHandlers[symbol]; ok {
			handlers = append(handlers, h)
		}
	}
	m.mu.Unlock()

	for _, h := range handlers {
		h(bar)
	}
}

func (c *AlpacaWSClient) readPump() {
	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			c.reconnect()
			continue
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[AlpacaWS] Read error: %v — reconnecting", err)
			c.mu.Lock()
			c.isConnected = false
			c.mu.Unlock()
			c.reconnect()
			continue
		}

		var msgs []alpacaWSMessage
		if json.Unmarshal(message, &msgs) != nil {
			continue
		}

		for _, msg := range msgs {
			if msg.T != "b" {
				continue
			}
			bar := AlpacaWSBar{T: msg.Ts, S: msg.S, O: msg.O, H: msg.H, L: msg.L, C: msg.C, V: msg.V}
			c.handlerMu.RLock()
			handlers := c.barHandlers[bar.S]
			c.handlerMu.RUnlock()
			for _, h := range handlers {
				h(bar)
			}
		}
	}
}

func (c *AlpacaWSClient) reconnect() {
	select {
	case <-c.stopChan:
		return
	default:
	}
	time.Sleep(c.reconnectWait)
	if c.reconnectWait < 60*time.Second {
		c.reconnectWait *= 2
	}
	log.Printf("[AlpacaWS] Reconnecting (backoff: %v)...", c.reconnectWait)
	if err := c.connect(); err != nil {
		log.Printf("[AlpacaWS] Reconnect failed: %v", err)
		return
	}
	// Re-subscribe
	c.mu.Lock()
	symbols := make([]string, 0, len(c.subscriptions))
	for s := range c.subscriptions {
		symbols = append(symbols, s)
	}
	if len(symbols) > 0 && c.conn != nil {
		c.conn.WriteJSON(map[string]interface{}{"action": "subscribe", "bars": symbols})
	}
	c.mu.Unlock()
	log.Printf("[AlpacaWS] Reconnected, re-subscribed to %d symbols", len(symbols))
}

// ==================== Bar Aggregator ====================

type BarAggregator struct {
	interval      time.Duration
	currentCandle *OHLCV
	candleStart   int64
	mu            sync.Mutex
	onComplete    func(OHLCV)
}

func newBarAggregator(interval time.Duration, onComplete func(OHLCV)) *BarAggregator {
	return &BarAggregator{interval: interval, onComplete: onComplete}
}

func (a *BarAggregator) AddBar(bar OHLCV) {
	var completedCandle *OHLCV

	a.mu.Lock()
	intervalSec := int64(a.interval.Seconds())
	bucketStart := (bar.Time / intervalSec) * intervalSec

	if a.currentCandle == nil || bucketStart != a.candleStart {
		if a.currentCandle != nil {
			cc := *a.currentCandle
			completedCandle = &cc
		}
		a.candleStart = bucketStart
		a.currentCandle = &OHLCV{
			Time: bucketStart, Open: bar.Open, High: bar.High,
			Low: bar.Low, Close: bar.Close, Volume: bar.Volume,
		}
	} else {
		if bar.High > a.currentCandle.High {
			a.currentCandle.High = bar.High
		}
		if bar.Low < a.currentCandle.Low {
			a.currentCandle.Low = bar.Low
		}
		a.currentCandle.Close = bar.Close
		a.currentCandle.Volume += bar.Volume
	}
	a.mu.Unlock()

	// Fire callback OUTSIDE the lock to avoid blocking other AddBar calls
	if completedCandle != nil && a.onComplete != nil {
		a.onComplete(*completedCandle)
	}
}

func (a *BarAggregator) FlushIfExpired(buffer time.Duration) {
	a.mu.Lock()
	if a.currentCandle == nil {
		a.mu.Unlock()
		return
	}
	bucketEnd := a.candleStart + int64(a.interval.Seconds())
	if time.Now().Unix() < bucketEnd+int64(buffer.Seconds()) {
		a.mu.Unlock()
		return
	}
	cc := *a.currentCandle
	a.currentCandle = nil
	a.candleStart = 0
	a.mu.Unlock()
	if a.onComplete != nil {
		a.onComplete(cc)
	}
}

func getTradingPositions(c *gin.Context) {
	var positions []TradingVirtualPosition
	db.Where("is_closed = ?", false).Find(&positions)
	c.JSON(200, positions)
}

// ========== Trading Scheduler ==========

var (
	tradingSchedulerRunning  bool
	tradingSchedulerMu       sync.Mutex
	tradingSchedulerStopChan chan struct{}
)

func getTradingSchedulerStatus(c *gin.Context) {
	tradingSchedulerMu.Lock()
	running := tradingSchedulerRunning
	tradingSchedulerMu.Unlock()
	c.JSON(200, gin.H{"running": running})
}

func toggleTradingScheduler(c *gin.Context) {
	tradingSchedulerMu.Lock()
	defer tradingSchedulerMu.Unlock()

	if tradingSchedulerRunning {
		if tradingSchedulerStopChan != nil {
			close(tradingSchedulerStopChan)
		}
		tradingSchedulerRunning = false
		fmt.Println("[TradingScheduler] Stopped")
		c.JSON(200, gin.H{"running": false})
	} else {
		tradingSchedulerStopChan = make(chan struct{})
		tradingSchedulerRunning = true
		go runTradingScheduler(tradingSchedulerStopChan)
		fmt.Println("[TradingScheduler] Started")
		c.JSON(200, gin.H{"running": true})
	}
}

func runTradingScheduler(stopChan chan struct{}) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	runTradingScan()

	for {
		select {
		case <-ticker.C:
			runTradingScan()
		case <-stopChan:
			return
		}
	}
}

func runTradingScan() {
	var items []TradingWatchlistItem
	db.Where("is_live = ?", true).Find(&items)
	if len(items) == 0 {
		return
	}

	fmt.Printf("[TradingScheduler] Scanning %d live symbols...\n", len(items))

	strategies := []TradingStrategy{
		&RegressionScalpingStrategy{},
		&HybridAITrendStrategy{},
	}

	for _, item := range items {
		ohlcv, err := getOHLCVCached(item.Symbol, "5m", 10*time.Minute)
		if err != nil {
			fmt.Printf("[TradingScheduler] Error fetching %s: %v\n", item.Symbol, err)
			continue
		}

		for _, strategy := range strategies {
			signals := strategy.Analyze(ohlcv)
			if len(signals) == 0 {
				continue
			}

			lastSignal := signals[len(signals)-1]
			if lastSignal.Index < len(ohlcv)-2 {
				continue
			}

			var existingPos TradingVirtualPosition
			if db.Where("symbol = ? AND strategy = ? AND is_closed = ?", item.Symbol, strategy.Name(), false).First(&existingPos).Error == nil {
				continue
			}

			pos := TradingVirtualPosition{
				Symbol:       item.Symbol,
				Strategy:     strategy.Name(),
				Direction:    lastSignal.Direction,
				EntryPrice:   lastSignal.EntryPrice,
				EntryTime:    time.Now(),
				StopLoss:     lastSignal.StopLoss,
				TakeProfit:   lastSignal.TakeProfit,
				CurrentPrice: lastSignal.EntryPrice,
				CreatedAt:    time.Now(),
			}
			db.Create(&pos)
			fmt.Printf("[TradingScheduler] Opened %s %s on %s @ %.2f\n", lastSignal.Direction, strategy.Name(), item.Symbol, lastSignal.EntryPrice)
		}

		var openPositions []TradingVirtualPosition
		db.Where("symbol = ? AND is_closed = ?", item.Symbol, false).Find(&openPositions)
		if len(ohlcv) > 0 {
			lastBar := ohlcv[len(ohlcv)-1]
			for _, pos := range openPositions {
				pos.CurrentPrice = lastBar.Close
				shouldClose := false
				reason := ""
				if pos.Direction == "LONG" {
					if lastBar.Low <= pos.StopLoss {
						shouldClose = true
						reason = "SL"
						pos.ClosePrice = pos.StopLoss
					} else if lastBar.High >= pos.TakeProfit {
						shouldClose = true
						reason = "TP"
						pos.ClosePrice = pos.TakeProfit
					}
					pos.ProfitLossPct = (pos.CurrentPrice - pos.EntryPrice) / pos.EntryPrice * 100
				} else {
					if lastBar.High >= pos.StopLoss {
						shouldClose = true
						reason = "SL"
						pos.ClosePrice = pos.StopLoss
					} else if lastBar.Low <= pos.TakeProfit {
						shouldClose = true
						reason = "TP"
						pos.ClosePrice = pos.TakeProfit
					}
					pos.ProfitLossPct = (pos.EntryPrice - pos.CurrentPrice) / pos.EntryPrice * 100
				}
				if shouldClose {
					now := time.Now()
					pos.IsClosed = true
					pos.CloseTime = &now
					pos.CloseReason = reason
					if pos.Direction == "LONG" {
						pos.ProfitLossPct = (pos.ClosePrice - pos.EntryPrice) / pos.EntryPrice * 100
					} else {
						pos.ProfitLossPct = (pos.EntryPrice - pos.ClosePrice) / pos.EntryPrice * 100
					}
					fmt.Printf("[TradingScheduler] Closed %s %s on %s: %s (%.1f%%)\n", pos.Direction, pos.Strategy, pos.Symbol, reason, pos.ProfitLossPct)
				}
				db.Save(&pos)
			}
		}
	}
}

// ==================== Live Trading ====================

// Per-session scheduler state
// candleEvent is sent from aggregator callbacks to the worker pool for async processing
type candleEvent struct {
	symbol        string
	candle        OHLCV
	cacheInterval string
}

type liveSessionState struct {
	StopChan          chan struct{}
	IsPolling         bool
	ScanTotal         int
	ScanProgress      int
	CurrentSymbol     string
	AlpacaActive      bool
	AlpacaLastChecked time.Time
	AlpacaError       string
	// WebSocket fields
	WSClient        *AlpacaWSClient
	UsesSharedWS    bool
	Aggregators     map[string]*BarAggregator
	WSConnected     bool
	LastBarReceived time.Time
	Mode            string // "polling" or "websocket"
	// Worker pool channel for async candle processing (Fix 1)
	candleChan chan candleEvent
	// In-memory session/strategy cache (Fix 2)
	cachedSession  LiveTradingSession
	cachedStrats   []LiveSessionStrategy
	cachedSymSets  map[uint]map[string]bool // strategyID → symbol set
	cacheMu        sync.RWMutex
	cacheReady     bool
	// In-memory OHLCV data (Fix 3)
	ohlcvData  map[string][]OHLCV
	ohlcvDirty map[string]bool
	ohlcvMu    sync.RWMutex
}

var (
	liveSchedulers   = map[uint]*liveSessionState{} // sessionID -> state
	liveSchedulerMu  sync.Mutex
)

func getLiveSessionState(sessionID uint) *liveSessionState {
	liveSchedulerMu.Lock()
	defer liveSchedulerMu.Unlock()
	return liveSchedulers[sessionID]
}

// refreshSessionCache loads session + strategies + symbol sets from DB into memory cache
func refreshSessionCache(state *liveSessionState, sessionID uint) {
	var sess LiveTradingSession
	if db.First(&sess, sessionID).Error != nil {
		return
	}
	var strats []LiveSessionStrategy
	db.Where("session_id = ? AND is_enabled = ?", sessionID, true).Find(&strats)

	symSets := make(map[uint]map[string]bool, len(strats))
	for _, s := range strats {
		var syms []string
		json.Unmarshal([]byte(s.Symbols), &syms)
		set := make(map[string]bool, len(syms))
		for _, sym := range syms {
			set[sym] = true
		}
		symSets[s.ID] = set
	}

	state.cacheMu.Lock()
	state.cachedSession = sess
	state.cachedStrats = strats
	state.cachedSymSets = symSets
	state.cacheReady = true
	state.cacheMu.Unlock()
}

// loadOHLCVIntoMemory preloads all OHLCV data from DB into the in-memory map
func loadOHLCVIntoMemory(state *liveSessionState, symbols []string, cacheInterval string) {
	state.ohlcvMu.Lock()
	state.ohlcvData = make(map[string][]OHLCV, len(symbols))
	state.ohlcvDirty = make(map[string]bool)
	state.ohlcvMu.Unlock()

	for _, sym := range symbols {
		var cache OHLCVCache
		if db.Where("symbol = ? AND interval = ?", sym, cacheInterval).First(&cache).Error == nil {
			var ohlcv []OHLCV
			if json.Unmarshal([]byte(cache.DataJSON), &ohlcv) == nil {
				state.ohlcvMu.Lock()
				state.ohlcvData[sym] = ohlcv
				state.ohlcvMu.Unlock()
			}
		}
	}
}

// flushDirtyOHLCV writes changed symbols from memory back to DB
func flushDirtyOHLCV(state *liveSessionState, cacheInterval string) {
	state.ohlcvMu.Lock()
	dirtySymbols := make([]string, 0, len(state.ohlcvDirty))
	for sym := range state.ohlcvDirty {
		dirtySymbols = append(dirtySymbols, sym)
	}
	// Copy data for dirty symbols and clear dirty set
	toFlush := make(map[string][]OHLCV, len(dirtySymbols))
	for _, sym := range dirtySymbols {
		data := state.ohlcvData[sym]
		cp := make([]OHLCV, len(data))
		copy(cp, data)
		toFlush[sym] = cp
	}
	state.ohlcvDirty = make(map[string]bool)
	state.ohlcvMu.Unlock()

	for sym, data := range toFlush {
		saveOHLCVCache(sym, cacheInterval, data)
	}
	if len(toFlush) > 0 {
		log.Printf("[LiveWS] Flushed OHLCV cache for %d dirty symbols", len(toFlush))
	}
}

// processCandleEvent handles a single completed candle using in-memory caches
func processCandleEvent(state *liveSessionState, evt candleEvent, config LiveTradingConfig) {
	// Append candle to in-memory OHLCV data
	state.ohlcvMu.Lock()
	data := state.ohlcvData[evt.symbol]
	data = append(data, evt.candle)
	state.ohlcvData[evt.symbol] = data
	state.ohlcvDirty[evt.symbol] = true
	// Make a copy for strategy analysis (avoid holding lock during processing)
	ohlcvCopy := make([]OHLCV, len(data))
	copy(ohlcvCopy, data)
	state.ohlcvMu.Unlock()

	// Read cached session + strategies
	state.cacheMu.RLock()
	if !state.cacheReady {
		state.cacheMu.RUnlock()
		return
	}
	sess := state.cachedSession
	strats := state.cachedStrats
	symSets := state.cachedSymSets
	state.cacheMu.RUnlock()

	// Run each strategy for this symbol
	for _, s := range strats {
		if !symSets[s.ID][evt.symbol] {
			continue
		}
		// Create fresh engine per call (strategies have internal state, not thread-safe)
		engine := createStrategyFromJSON(s.Name, s.ParamsJSON)
		if engine == nil {
			continue
		}
		if len(ohlcvCopy) >= engine.RequiredBars() {
			processLiveSymbolWithData(sess, evt.symbol, engine, ohlcvCopy, config, s)
		}
	}
}

func mergeOHLCV(cached, fresh []OHLCV) []OHLCV {
	if len(cached) == 0 {
		return fresh
	}
	if len(fresh) == 0 {
		return cached
	}
	freshStart := fresh[0].Time
	cutIdx := len(cached)
	for i, bar := range cached {
		if bar.Time >= freshStart {
			cutIdx = i
			break
		}
	}
	return append(cached[:cutIdx], fresh...)
}

// triggerPriorityRefresh fills the persistent OHLCV cache for symbols missing data.
// Uses 5 parallel workers with 500ms delay (~7 min for 2000 symbols).
func triggerPriorityRefresh(symbols []string, yahooInterval string, sessionID uint) {
	period := backtestPeriodMap[yahooInterval]
	if period == "" {
		period = "60d"
	}

	// Find symbols without cache data
	var missing []string
	for _, sym := range symbols {
		var cache OHLCVCache
		if db.Where("symbol = ? AND interval = ?", sym, yahooInterval).First(&cache).Error != nil {
			missing = append(missing, sym)
		}
	}

	if len(missing) == 0 {
		logLiveEvent(sessionID, "INFO", "-", fmt.Sprintf("Cache OK — alle %d Symbole vorhanden (%s)", len(symbols), yahooInterval))
		return
	}

	logLiveEvent(sessionID, "INFO", "-", fmt.Sprintf("Priority-Prefetch: %d/%d Symbole fehlen im Cache (%s)", len(missing), len(symbols), yahooInterval))
	start := time.Now()

	var fetched int64

	// Try Alpaca batch first (50 symbols/request, fast for live-trading)
	if alpacaDataKey != "" {
		alpacaIv := yahooInterval
		ivMap := map[string]string{"60m": "1h", "1d": "1D", "1wk": "1W"}
		if mapped, ok := ivMap[alpacaIv]; ok {
			alpacaIv = mapped
		}
		const batchSize = 50
		for i := 0; i < len(missing); i += batchSize {
			end := i + batchSize
			if end > len(missing) {
				end = len(missing)
			}
			batch := missing[i:end]
			batchResult, err := fetchOHLCVBatchFromAlpaca(batch, alpacaIv)
			if err == nil {
				for _, sym := range batch {
					if bars, ok := batchResult[sym]; ok && len(bars) > 0 {
						saveOHLCVCache(sym, yahooInterval, bars)
						atomic.AddInt64(&fetched, 1)
					}
				}
			}
		}
		// Check what's still missing after Alpaca
		var stillMissing []string
		for _, sym := range missing {
			if _, ok := getOHLCVFromMemCache(sym, yahooInterval); !ok {
				stillMissing = append(stillMissing, sym)
			}
		}
		missing = stillMissing
	}

	// Yahoo fallback for remaining (20 concurrent, no throttle)
	const maxWorkers = 20
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for _, sym := range missing {
		wg.Add(1)
		sem <- struct{}{}
		go func(symbol string) {
			defer wg.Done()
			defer func() { <-sem }()
			_, err := getOHLCVCached(symbol, yahooInterval, 1*time.Millisecond)
			if err != nil {
				return
			}
			atomic.AddInt64(&fetched, 1)
		}(sym)
	}
	wg.Wait()
	logLiveEvent(sessionID, "INFO", "-", fmt.Sprintf("Priority-Prefetch abgeschlossen — %d Symbole in %.1fs", fetched, time.Since(start).Seconds()))
}

func logLiveEvent(sessionID uint, level, symbol, message string, strategyName ...string) {
	sn := ""
	if len(strategyName) > 0 {
		sn = strategyName[0]
	}
	db.Create(&LiveTradingLog{
		SessionID: sessionID,
		Level:     level,
		Symbol:    symbol,
		Message:   message,
		Strategy:  sn,
		CreatedAt: time.Now(),
	})
	fmt.Printf("[LiveTrading] [%s] %s: %s\n", level, symbol, message)
}

func intervalToDuration(iv string) time.Duration {
	switch iv {
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "1h", "60m":
		return 1 * time.Hour
	case "2h":
		return 2 * time.Hour
	case "4h":
		return 4 * time.Hour
	case "1d", "1D":
		return 24 * time.Hour
	case "1wk", "1W":
		return 7 * 24 * time.Hour
	default:
		return 5 * time.Minute
	}
}

func createStrategyFromJSON(strategyName, paramsJSON string) TradingStrategy {
	var params map[string]interface{}
	if paramsJSON != "" {
		json.Unmarshal([]byte(paramsJSON), &params)
	}
	if params == nil {
		params = map[string]interface{}{}
	}

	pFloat := func(key string, def float64) float64 {
		if v, ok := params[key]; ok {
			switch val := v.(type) {
			case float64:
				return val
			case int:
				return float64(val)
			}
		}
		return def
	}
	pInt := func(key string, def int) int {
		if v, ok := params[key]; ok {
			switch val := v.(type) {
			case float64:
				return int(val)
			case int:
				return val
			}
		}
		return def
	}
	pBool := func(key string) bool {
		if v, ok := params[key]; ok {
			switch val := v.(type) {
			case bool:
				return val
			case float64:
				return val != 0
			}
		}
		return false
	}

	switch strategyName {
	case "regression_scalping":
		return &RegressionScalpingStrategy{
			Degree: pInt("degree", 2), Length: pInt("length", 100), Multiplier: pFloat("multiplier", 3.0),
			RiskReward: pFloat("risk_reward", 2.5), SLLookback: pInt("sl_lookback", 30),
			ConfirmationRequired: pInt("confirmation_required", 1),
		}
	case "hybrid_ai_trend":
		return &HybridAITrendStrategy{
			BB1Period: pInt("bb1_period", 20), BB1Stdev: pFloat("bb1_stdev", 3.0),
			BB2Period: pInt("bb2_period", 75), BB2Stdev: pFloat("bb2_stdev", 3.0),
			BB3Period: pInt("bb3_period", 100), BB3Stdev: pFloat("bb3_stdev", 4.0),
			BB4Period: pInt("bb4_period", 100), BB4Stdev: pFloat("bb4_stdev", 4.25),
			NWBandwidth: pFloat("nw_bandwidth", 6.0), NWLookback: pInt("nw_lookback", 499),
			SLBuffer: pFloat("sl_buffer", 1.5), RiskReward: pFloat("risk_reward", 2.0),
			HybridFilter: pBool("hybrid_filter"),
			HybridLongThresh: pFloat("hybrid_long_thresh", 75), HybridShortThresh: pFloat("hybrid_short_thresh", 25),
			ConfirmCandle: pBool("confirm_candle"), MinBandDist: pFloat("min_band_dist", 0),
		}
	case "diamond_signals":
		return &DiamondSignalsStrategy{
			PatternLength: pInt("pattern_length", 20), RSIPeriod: pInt("rsi_period", 14),
			ConfluenceMin: pInt("confluence_min", 3), RSIOverbought: pFloat("rsi_overbought", 65),
			RSIOversold: pFloat("rsi_oversold", 35), Cooldown: pInt("cooldown", 5), RiskReward: pFloat("risk_reward", 2.0),
		}
	case "smart_money_flow":
		return &SmartMoneyFlowStrategy{
			TrendLength: pInt("trend_length", 34), BasisSmooth: pInt("basis_smooth", 3),
			FlowWindow: pInt("flow_window", 24), FlowSmooth: pInt("flow_smooth", 5),
			FlowBoost: pFloat("flow_boost", 1.2), ATRLength: pInt("atr_length", 14),
			BandTightness: pFloat("band_tightness", 0.9), BandExpansion: pFloat("band_expansion", 2.2),
			DotCooldown: pInt("dot_cooldown", 12), RiskReward: pFloat("risk_reward", 2.0),
		}
	case "hann_trend":
		return &HannTrendStrategy{
			DMHLength: pInt("dmh_length", 30), SARStart: pFloat("sar_start", 0.02),
			SARIncrement: pFloat("sar_increment", 0.03), SARMax: pFloat("sar_max", 0.3),
			SwingLookback: pInt("swing_lookback", 5), RiskReward: pFloat("risk_reward", 2.0),
			SLBuffer: pFloat("sl_buffer", 0.3),
			HybridFilter: pBool("hybrid_filter"),
			HybridLongThresh: pFloat("hybrid_long_thresh", 75),
			HybridShortThresh: pFloat("hybrid_short_thresh", 25),
		}
	case "gmma_pullback":
		return &GMMAPullbackStrategy{
			SignalLen: pInt("signal_len", 9), SmoothLen: pInt("smooth_len", 3),
			FractalPeriods: pInt("fractal_periods", 5), ZoneCount: pInt("zone_count", 5),
			RiskReward: pFloat("risk_reward", 2.0), SLLookback: pInt("sl_lookback", 10),
			SLBuffer: pFloat("sl_buffer", 0.3),
		}
	case "macd_sr":
		return &MACDSRStrategy{
			MACDFast: pInt("macd_fast", 12), MACDSlow: pInt("macd_slow", 26),
			MACDSignal: pInt("macd_signal", 9), EMAPeriod: pInt("ema_period", 200),
			SLBuffer: pFloat("sl_buffer", 1.5), RiskReward: pFloat("risk_reward", 1.5),
			SRFilter: pInt("sr_filter", 1) == 1, FractalPeriods: pInt("fractal_periods", 5),
			ZoneCount: pInt("zone_count", 5), SRTolerance: pFloat("sr_tolerance", 1.5),
			HybridFilter: pInt("hybrid_filter", 0) == 1,
			HybridLongThresh: pFloat("hybrid_long_thresh", 75),
			HybridShortThresh: pFloat("hybrid_short_thresh", 25),
		}
	case "trippa_trade":
		return &TrippaTrade{
			MaxRange: pInt("max_range", 100), MinRange: pInt("min_range", 10),
			Step: pInt("reg_step", 5), SignalLen: pInt("signal_len", 7),
			EMAFast: pInt("ema_fast", 5), EMASlow: pInt("ema_slow", 13),
			RiskReward: pFloat("risk_reward", 2.0), SLBuffer: pFloat("sl_buffer", 0.5),
			MinTrendBars: pInt("min_trend_bars", 3),
		}
	default:
		return nil
	}
}

func saveLiveTradingConfig(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)

	var req struct {
		Strategy        string                 `json:"strategy"`
		Interval        string                 `json:"interval"`
		Params          map[string]interface{} `json:"params"`
		Symbols         []string               `json:"symbols"`
		LongOnly        bool                   `json:"long_only"`
		TradeAmount     float64                `json:"trade_amount"`
		Filters         map[string]interface{} `json:"filters"`
		FiltersActive   bool                   `json:"filters_active"`
		Currency        string                 `json:"currency"`
		AlpacaAccountID *uint                  `json:"alpaca_account_id"`
		AlpacaApiKey    *string                `json:"alpaca_api_key"`
		AlpacaSecretKey *string                `json:"alpaca_secret_key"`
		AlpacaEnabled   *bool                  `json:"alpaca_enabled"`
		AlpacaPaper     *bool                  `json:"alpaca_paper"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Ungültige Anfrage"})
		return
	}

	paramsBytes, _ := json.Marshal(req.Params)
	symbolsBytes, _ := json.Marshal(req.Symbols)
	filtersBytes, _ := json.Marshal(req.Filters)

	currency := req.Currency
	if currency == "" {
		currency = "EUR"
	}

	var config LiveTradingConfig
	if configIDStr := c.Query("config_id"); configIDStr != "" {
		if db.First(&config, configIDStr).Error != nil || config.UserID != uid {
			c.JSON(400, gin.H{"error": "Config nicht gefunden"})
			return
		}
	} else {
		db.Where("user_id = ?", uid).FirstOrCreate(&config, LiveTradingConfig{UserID: uid})
	}
	config.Strategy = req.Strategy
	config.Interval = req.Interval
	config.ParamsJSON = string(paramsBytes)
	config.Symbols = string(symbolsBytes)
	config.LongOnly = req.LongOnly
	config.TradeAmount = req.TradeAmount
	config.FiltersJSON = string(filtersBytes)
	config.FiltersActive = req.FiltersActive
	config.Currency = currency

	// Resolve Alpaca Account if ID provided
	if req.AlpacaAccountID != nil && *req.AlpacaAccountID > 0 {
		var acc AlpacaAccount
		if db.First(&acc, *req.AlpacaAccountID).Error == nil {
			config.AlpacaAccountID = acc.ID
			config.AlpacaApiKey = acc.ApiKey
			config.AlpacaSecretKey = acc.SecretKey
			config.AlpacaPaper = acc.IsPaper
		}
	} else if req.AlpacaAccountID != nil && *req.AlpacaAccountID == 0 {
		config.AlpacaAccountID = 0
		config.AlpacaApiKey = ""
		config.AlpacaSecretKey = ""
	} else {
		// Legacy: direct key input
		if req.AlpacaApiKey != nil && !strings.HasPrefix(*req.AlpacaApiKey, "****") {
			config.AlpacaApiKey = *req.AlpacaApiKey
		}
		if req.AlpacaSecretKey != nil && !strings.HasPrefix(*req.AlpacaSecretKey, "****") {
			config.AlpacaSecretKey = *req.AlpacaSecretKey
		}
	}
	if req.AlpacaEnabled != nil {
		config.AlpacaEnabled = *req.AlpacaEnabled
	}
	if req.AlpacaPaper != nil && (req.AlpacaAccountID == nil || *req.AlpacaAccountID == 0) {
		config.AlpacaPaper = *req.AlpacaPaper
	}
	config.UpdatedAt = time.Now()
	db.Save(&config)

	c.JSON(200, gin.H{
		"id":                config.ID,
		"strategy":          config.Strategy,
		"interval":          config.Interval,
		"params":            req.Params,
		"symbols":           req.Symbols,
		"long_only":         config.LongOnly,
		"trade_amount":      config.TradeAmount,
		"filters":           req.Filters,
		"filters_active":    config.FiltersActive,
		"currency":          config.Currency,
		"alpaca_account_id": config.AlpacaAccountID,
		"updated_at":        config.UpdatedAt,
	})
}

// --- Alpaca Account CRUD ---

func getAlpacaAccounts(c *gin.Context) {
	var accounts []AlpacaAccount
	db.Order("is_default DESC, name ASC").Find(&accounts)

	result := make([]gin.H, len(accounts))
	for i, a := range accounts {
		result[i] = gin.H{
			"id":         a.ID,
			"name":       a.Name,
			"api_key":    maskKey(a.ApiKey),
			"is_paper":   a.IsPaper,
			"is_default": a.IsDefault,
			"created_at": a.CreatedAt,
			"updated_at": a.UpdatedAt,
		}
	}
	c.JSON(200, result)
}

func createAlpacaAccount(c *gin.Context) {
	var req struct {
		Name      string `json:"name"`
		ApiKey    string `json:"api_key"`
		SecretKey string `json:"secret_key"`
		IsPaper   bool   `json:"is_paper"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.ApiKey == "" || req.SecretKey == "" {
		c.JSON(400, gin.H{"error": "Name, API Key und Secret Key erforderlich"})
		return
	}

	account := AlpacaAccount{
		Name:      req.Name,
		ApiKey:    req.ApiKey,
		SecretKey: req.SecretKey,
		IsPaper:   req.IsPaper,
	}
	db.Create(&account)
	c.JSON(200, gin.H{
		"id":         account.ID,
		"name":       account.Name,
		"api_key":    maskKey(account.ApiKey),
		"is_paper":   account.IsPaper,
		"is_default": account.IsDefault,
		"created_at": account.CreatedAt,
	})
}

func updateAlpacaAccount(c *gin.Context) {
	id := c.Param("id")
	var account AlpacaAccount
	if db.First(&account, id).Error != nil {
		c.JSON(404, gin.H{"error": "Account nicht gefunden"})
		return
	}

	var req struct {
		Name      *string `json:"name"`
		ApiKey    *string `json:"api_key"`
		SecretKey *string `json:"secret_key"`
		IsPaper   *bool   `json:"is_paper"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Ungültige Anfrage"})
		return
	}

	if req.Name != nil {
		account.Name = *req.Name
	}
	if req.ApiKey != nil && !strings.HasPrefix(*req.ApiKey, "****") {
		account.ApiKey = *req.ApiKey
	}
	if req.SecretKey != nil && !strings.HasPrefix(*req.SecretKey, "****") {
		account.SecretKey = *req.SecretKey
	}
	if req.IsPaper != nil {
		account.IsPaper = *req.IsPaper
	}
	account.UpdatedAt = time.Now()
	db.Save(&account)

	c.JSON(200, gin.H{
		"id":         account.ID,
		"name":       account.Name,
		"api_key":    maskKey(account.ApiKey),
		"is_paper":   account.IsPaper,
		"is_default": account.IsDefault,
		"updated_at": account.UpdatedAt,
	})
}

func deleteAlpacaAccount(c *gin.Context) {
	id := c.Param("id")
	var account AlpacaAccount
	if db.First(&account, id).Error != nil {
		c.JSON(404, gin.H{"error": "Account nicht gefunden"})
		return
	}

	// Check if any active session uses this account
	var activeCount int64
	db.Model(&LiveTradingSession{}).
		Joins("JOIN live_trading_configs ON live_trading_configs.id = live_trading_sessions.config_id").
		Where("live_trading_sessions.is_active = ? AND live_trading_configs.alpaca_account_id = ?", true, account.ID).
		Count(&activeCount)
	if activeCount > 0 {
		c.JSON(400, gin.H{"error": "Account wird von aktiven Sessions verwendet"})
		return
	}

	db.Delete(&account)
	c.JSON(200, gin.H{"ok": true})
}

func validateAlpacaAccount(c *gin.Context) {
	id := c.Param("id")
	var account AlpacaAccount
	if db.First(&account, id).Error != nil {
		c.JSON(404, gin.H{"error": "Account nicht gefunden"})
		return
	}

	tempConfig := LiveTradingConfig{
		AlpacaApiKey:    account.ApiKey,
		AlpacaSecretKey: account.SecretKey,
		AlpacaPaper:     account.IsPaper,
	}
	acc, err := alpacaGetAccount(tempConfig)
	if err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("Verbindung fehlgeschlagen: %v", err)})
		return
	}

	c.JSON(200, gin.H{
		"status":         acc["status"],
		"buying_power":   acc["buying_power"],
		"cash":           acc["cash"],
		"account_number": acc["account_number"],
		"paper":          account.IsPaper,
	})
}

func setDefaultAlpacaAccount(c *gin.Context) {
	id := c.Param("id")
	var account AlpacaAccount
	if db.First(&account, id).Error != nil {
		c.JSON(404, gin.H{"error": "Account nicht gefunden"})
		return
	}

	// Clear all defaults, then set new one
	db.Model(&AlpacaAccount{}).Where("is_default = ?", true).Update("is_default", false)
	db.Model(&account).Update("is_default", true)

	// Update global data keys (used for WS + REST data API)
	alpacaDataKey = account.ApiKey
	alpacaDataSecret = account.SecretKey
	if alpacaRateTicker == nil {
		alpacaRateTicker = time.NewTicker(350 * time.Millisecond)
	}
	log.Printf("[Alpaca] Default Account gewechselt → '%s' (ID %d)", account.Name, account.ID)

	// (Re-)initialize SharedWS if not yet connected
	if sharedWS == nil || !sharedWS.IsConnected() {
		initSharedWS()
	}

	wsStatus := "disconnected"
	if sharedWS != nil && sharedWS.IsConnected() {
		wsStatus = "connected"
	}
	c.JSON(200, gin.H{"ok": true, "ws_status": wsStatus})
}

func validateAlpacaKeys(c *gin.Context) {
	var req struct {
		ApiKey    string `json:"api_key"`
		SecretKey string `json:"secret_key"`
		Paper     bool   `json:"paper"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ApiKey == "" || req.SecretKey == "" {
		c.JSON(400, gin.H{"error": "API Key und Secret Key erforderlich"})
		return
	}

	tempConfig := LiveTradingConfig{
		AlpacaApiKey:    req.ApiKey,
		AlpacaSecretKey: req.SecretKey,
		AlpacaPaper:     req.Paper,
	}
	account, err := alpacaGetAccount(tempConfig)
	if err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("Verbindung fehlgeschlagen: %v", err)})
		return
	}

	c.JSON(200, gin.H{
		"status":        account["status"],
		"buying_power":  account["buying_power"],
		"cash":          account["cash"],
		"account_number": account["account_number"],
		"paper":         req.Paper,
	})
}

func alpacaGetLatestPrice(symbol string, config LiveTradingConfig) (float64, error) {
	dataURL := "https://data.alpaca.markets"
	if config.AlpacaPaper {
		dataURL = "https://data.alpaca.markets"
	}
	req, err := http.NewRequest("GET", dataURL+"/v2/stocks/"+symbol+"/quotes/latest", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("APCA-API-KEY-ID", config.AlpacaApiKey)
	req.Header.Set("APCA-API-SECRET-KEY", config.AlpacaSecretKey)
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if quote, ok := result["quote"].(map[string]interface{}); ok {
		if ap, ok := quote["ap"].(float64); ok && ap > 0 {
			return ap, nil // ask price
		}
		if bp, ok := quote["bp"].(float64); ok && bp > 0 {
			return bp, nil // bid price
		}
	}
	return 0, fmt.Errorf("kein Kurs für %s verfügbar", symbol)
}

// alpacaGetLatestPrices fetches latest quotes for multiple symbols in one batch call.
// Returns map[symbol]price. Uses Alpaca's multi-quote endpoint.
func alpacaGetLatestPrices(symbols []string, config LiveTradingConfig) map[string]float64 {
	prices := make(map[string]float64, len(symbols))
	if len(symbols) == 0 {
		return prices
	}
	dataURL := "https://data.alpaca.markets"
	// Alpaca batch: max ~100 symbols per call
	for i := 0; i < len(symbols); i += 100 {
		end := i + 100
		if end > len(symbols) {
			end = len(symbols)
		}
		batch := symbols[i:end]
		symParam := strings.Join(batch, ",")
		req, err := http.NewRequest("GET", dataURL+"/v2/stocks/quotes/latest?symbols="+symParam, nil)
		if err != nil {
			continue
		}
		req.Header.Set("APCA-API-KEY-ID", config.AlpacaApiKey)
		req.Header.Set("APCA-API-SECRET-KEY", config.AlpacaSecretKey)
		resp, err := httpClient.Do(req)
		if err != nil {
			continue
		}
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if quotes, ok := result["quotes"].(map[string]interface{}); ok {
			for sym, q := range quotes {
				if qm, ok := q.(map[string]interface{}); ok {
					if ap, ok := qm["ap"].(float64); ok && ap > 0 {
						prices[sym] = ap
					} else if bp, ok := qm["bp"].(float64); ok && bp > 0 {
						prices[sym] = bp
					}
				}
			}
		}
	}
	return prices
}

// ==================== SL/TP Monitor ====================
// Runs every 2 minutes. Checks all open positions with SL/TP against live quotes.
// Uses each session's own Alpaca config (separate broker account per session).

func startSLTPMonitor() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	log.Println("[SLTPMonitor] Gestartet — prüft alle 2 Min offene Positionen")
	for range ticker.C {
		checkOpenPositionsSLTP()
	}
}

func checkOpenPositionsSLTP() {
	// Get all open positions that have SL or TP set
	var positions []LiveTradingPosition
	db.Where("is_closed = ? AND (stop_loss > 0 OR take_profit > 0)", false).Find(&positions)
	if len(positions) == 0 {
		return
	}

	// Group positions by session
	sessionPositions := map[uint][]LiveTradingPosition{}
	for i := range positions {
		sid := positions[i].SessionID
		sessionPositions[sid] = append(sessionPositions[sid], positions[i])
	}

	for sessionID, posGroup := range sessionPositions {
		// Load session + config
		var session LiveTradingSession
		if db.First(&session, sessionID).Error != nil || !session.IsActive {
			continue
		}
		var config LiveTradingConfig
		if session.ConfigID == 0 || db.First(&config, session.ConfigID).Error != nil {
			continue
		}
		if !config.AlpacaEnabled || config.AlpacaApiKey == "" {
			continue
		}

		// Collect symbols for batch quote
		symbols := make([]string, 0, len(posGroup))
		for _, p := range posGroup {
			symbols = append(symbols, p.Symbol)
		}

		// Batch fetch current prices via this session's Alpaca account
		prices := alpacaGetLatestPrices(symbols, config)

		// Check SL/TP for each position
		for i := range posGroup {
			pos := &posGroup[i]
			price, ok := prices[pos.Symbol]
			if !ok || price <= 0 {
				continue
			}

			hit := false
			reason := ""

			if pos.Direction == "LONG" {
				if pos.StopLoss > 0 && price <= pos.StopLoss {
					hit = true
					reason = "SL"
				} else if pos.TakeProfit > 0 && price >= pos.TakeProfit {
					hit = true
					reason = "TP"
				}
			} else { // SHORT
				if pos.StopLoss > 0 && price >= pos.StopLoss {
					hit = true
					reason = "SL"
				} else if pos.TakeProfit > 0 && price <= pos.TakeProfit {
					hit = true
					reason = "TP"
				}
			}

			if hit {
				// Re-fetch from DB to avoid stale data (could have been closed by scan in the meantime)
				var fresh LiveTradingPosition
				if db.First(&fresh, pos.ID).Error != nil || fresh.IsClosed {
					continue
				}
				nativeCurrency := fresh.NativeCurrency
				if nativeCurrency == "" {
					nativeCurrency = "USD"
				}
				logLiveEvent(session.ID, "SLTP_MONITOR", fresh.Symbol, fmt.Sprintf("%s ausgelöst @ %.4f (Entry: %.4f, %s: %.4f)", reason, price, fresh.EntryPrice, reason, func() float64 {
					if reason == "SL" {
						return fresh.StopLoss
					}
					return fresh.TakeProfit
				}()))
				closeLivePosition(&fresh, price, reason, nativeCurrency, config)
			}
		}
	}
}

func alpacaTestOrder(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid := userID.(uint)

	var req struct {
		Symbol    string  `json:"symbol"`
		Qty       int     `json:"qty"`
		Side      string  `json:"side"`
		StopLoss  float64 `json:"stop_loss"`
		TakeProfit float64 `json:"take_profit"`
		SessionID  uint   `json:"session_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Ungültige Anfrage"})
		return
	}
	if req.Symbol == "" {
		c.JSON(400, gin.H{"error": "Symbol erforderlich"})
		return
	}
	if req.Side != "buy" && req.Side != "sell" {
		c.JSON(400, gin.H{"error": "Side muss 'buy' oder 'sell' sein"})
		return
	}

	var config LiveTradingConfig
	if req.SessionID > 0 {
		var session LiveTradingSession
		if db.Where("id = ? AND user_id = ?", req.SessionID, uid).First(&session).Error != nil {
			c.JSON(400, gin.H{"error": "Session nicht gefunden"})
			return
		}
		if db.First(&config, session.ConfigID).Error != nil || config.AlpacaApiKey == "" {
			c.JSON(400, gin.H{"error": "Alpaca nicht konfiguriert"})
			return
		}
	} else if db.Where("user_id = ?", uid).First(&config).Error != nil || config.AlpacaApiKey == "" {
		c.JSON(400, gin.H{"error": "Alpaca nicht konfiguriert"})
		return
	}
	if !config.AlpacaPaper {
		c.JSON(403, gin.H{"error": "Test-Orders nur im Paper-Modus erlaubt"})
		return
	}

	// Auto-calculate qty from TradeAmount if not provided
	qty := float64(req.Qty)
	currentPrice := 0.0
	if qty <= 0 && req.Side == "buy" {
		price, err := alpacaGetLatestPrice(req.Symbol, config)
		if err != nil || price <= 0 {
			c.JSON(400, gin.H{"error": fmt.Sprintf("Kurs für %s nicht verfügbar: %v", req.Symbol, err)})
			return
		}
		currentPrice = price
		tradeAmountUSD := config.TradeAmount // TradeAmount is always in USD
		qty = math.Round(tradeAmountUSD/price*1000000) / 1000000
		if qty <= 0 {
			qty = 1
		}
	} else if qty <= 0 {
		qty = 1
	}

	// Build bracket opts from SL/TP
	bracketOpts := map[string]float64{}
	if req.StopLoss > 0 {
		bracketOpts["stop_loss"] = req.StopLoss
	}
	if req.TakeProfit > 0 {
		bracketOpts["take_profit"] = req.TakeProfit
	}

	orderResult, err := alpacaPlaceOrder(req.Symbol, qty, req.Side, config, bracketOpts)
	if err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("Order fehlgeschlagen: %v", err)})
		return
	}

	// Find active session to track position + log
	var session LiveTradingSession
	hasSession := db.Where("user_id = ? AND is_active = ?", uid, true).First(&session).Error == nil
	if !hasSession {
		hasSession = db.Where("user_id = ?", uid).Order("started_at DESC").First(&session).Error == nil
	}

	estimatedPrice := orderResult.FilledAvgPrice
	if estimatedPrice == 0 && currentPrice > 0 {
		estimatedPrice = currentPrice
	} else if estimatedPrice == 0 {
		estimatedPrice = config.TradeAmount / qty
	}

	if hasSession && req.Side == "buy" {
		pos := LiveTradingPosition{
			SessionID:      session.ID,
			Symbol:         req.Symbol,
			Direction:      "LONG",
			EntryPrice:     estimatedPrice,
			EntryPriceUSD:  estimatedPrice,
			EntryTime:      time.Now(),
			CurrentPrice:   estimatedPrice,
			Quantity:       qty,
			StopLoss:       req.StopLoss,
			TakeProfit:     req.TakeProfit,
			InvestedAmount: estimatedPrice * qty, // USD — no currency conversion
			NativeCurrency: "USD",
			AlpacaOrderID:  orderResult.OrderID,
			CreatedAt:      time.Now(),
		}
		db.Create(&pos)
		bracketInfo := ""
		if orderResult.OrderClass == "bracket" {
			bracketInfo = fmt.Sprintf(" [BRACKET SL:%.2f TP:%.2f]", req.StopLoss, req.TakeProfit)
		}
		logLiveEvent(session.ID, "TRADE", req.Symbol, fmt.Sprintf("TEST-BUY %g x %s @ $%.2f (Einsatz: $%.0f)%s → Alpaca OrderID: %s, Status: %s", qty, req.Symbol, estimatedPrice, config.TradeAmount, bracketInfo, orderResult.OrderID, orderResult.Status))
	} else if hasSession && req.Side == "sell" {
		var openPos LiveTradingPosition
		if db.Where("session_id = ? AND symbol = ? AND is_closed = ?", session.ID, req.Symbol, false).First(&openPos).Error == nil {
			closeLivePosition(&openPos, openPos.CurrentPrice, "MANUAL", openPos.NativeCurrency, config)
			logLiveEvent(session.ID, "TRADE", req.Symbol, fmt.Sprintf("TEST-SELL %s geschlossen @ %.4f (%.2f%%) → Alpaca OrderID: %s", req.Symbol, openPos.ClosePrice, openPos.ProfitLossPct, orderResult.OrderID))
		} else {
			logLiveEvent(session.ID, "TRADE", req.Symbol, fmt.Sprintf("TEST-SELL %g x %s → Alpaca OrderID: %s, Status: %s (keine offene Position)", qty, req.Symbol, orderResult.OrderID, orderResult.Status))
		}
	}

	legInfo := []gin.H{}
	for _, leg := range orderResult.Legs {
		legInfo = append(legInfo, gin.H{"id": leg.ID, "type": leg.Type, "status": leg.Status})
	}

	c.JSON(200, gin.H{
		"order_id":         orderResult.OrderID,
		"status":           orderResult.Status,
		"order_class":      orderResult.OrderClass,
		"filled_avg_price": orderResult.FilledAvgPrice,
		"symbol":           req.Symbol,
		"qty":              qty,
		"side":             req.Side,
		"stop_loss":        req.StopLoss,
		"take_profit":      req.TakeProfit,
		"trade_amount":     config.TradeAmount,
		"currency":         config.Currency,
		"legs":             legInfo,
	})
}

func renameLiveSession(c *gin.Context) {
	uid := liveOwnerUID(c)
	id := c.Param("id")

	var req struct {
		Name string `json:"name"`
	}
	if c.BindJSON(&req) != nil || req.Name == "" {
		c.JSON(400, gin.H{"error": "Name darf nicht leer sein"})
		return
	}

	var session LiveTradingSession
	if db.Where("id = ? AND user_id = ?", id, uid).First(&session).Error != nil {
		c.JSON(404, gin.H{"error": "Session nicht gefunden"})
		return
	}

	db.Model(&session).Update("name", req.Name)
	c.JSON(200, gin.H{"name": req.Name})
}

func deleteLiveSession(c *gin.Context) {
	uid := liveOwnerUID(c)
	id := c.Param("id")

	var session LiveTradingSession
	if db.Where("id = ? AND user_id = ?", id, uid).First(&session).Error != nil {
		c.JSON(404, gin.H{"error": "Session nicht gefunden"})
		return
	}

	if session.IsActive {
		// Check if scheduler is actually running
		liveSchedulerMu.Lock()
		state, hasScheduler := liveSchedulers[session.ID]
		if hasScheduler {
			close(state.StopChan)
			delete(liveSchedulers, session.ID)
		}
		liveSchedulerMu.Unlock()

		// Deactivate the session so it can be deleted
		now := time.Now()
		db.Model(&session).Updates(map[string]interface{}{"is_active": false, "stopped_at": &now})
		if hasScheduler {
			logLiveEvent(session.ID, "INFO", "-", "Session gestoppt (vor Löschung)")
		}
	}

	// Delete positions, logs, then session
	db.Where("session_id = ?", session.ID).Delete(&LiveTradingPosition{})
	db.Where("session_id = ?", session.ID).Delete(&LiveTradingLog{})
	db.Delete(&session)

	log.Printf("[LiveTrading] Session #%d gelöscht von User %d", session.ID, uid)
	c.JSON(200, gin.H{"message": fmt.Sprintf("Session #%d gelöscht", session.ID)})
}

func resetLiveSession(c *gin.Context) {
	uid := liveOwnerUID(c)
	id := c.Param("id")

	var session LiveTradingSession
	if db.Where("id = ? AND user_id = ?", id, uid).First(&session).Error != nil {
		c.JSON(404, gin.H{"error": "Session nicht gefunden"})
		return
	}

	// Stop if active
	if session.IsActive {
		liveSchedulerMu.Lock()
		state, hasScheduler := liveSchedulers[session.ID]
		if hasScheduler {
			close(state.StopChan)
			delete(liveSchedulers, session.ID)
		}
		liveSchedulerMu.Unlock()
	}

	// Clear liveOpenPosGuard entries for this session's positions
	var positions []LiveTradingPosition
	db.Where("session_id = ?", session.ID).Find(&positions)
	for _, pos := range positions {
		liveOpenPosGuard.Delete(openPosGuardKey(session.ID, pos.StrategyID, pos.Symbol))
	}

	// Delete positions and logs
	db.Where("session_id = ?", session.ID).Delete(&LiveTradingPosition{})
	db.Where("session_id = ?", session.ID).Delete(&LiveTradingLog{})

	// Reset session fields
	db.Model(&session).Updates(map[string]interface{}{
		"is_active":        false,
		"started_at":       nil,
		"stopped_at":       nil,
		"last_poll_at":     nil,
		"next_poll_at":     nil,
		"total_polls":      0,
		"symbol_prices_json": nil,
	})

	logLiveEvent(session.ID, "INFO", "-", "Session zurückgesetzt")
	log.Printf("[LiveTrading] Session #%d zurückgesetzt von User %d", session.ID, uid)
	c.JSON(200, gin.H{"message": fmt.Sprintf("Session #%d zurückgesetzt", session.ID)})
}

func getAlpacaPortfolio(c *gin.Context) {
	uid := liveOwnerUID(c)

	var config LiveTradingConfig
	if sessionIDStr := c.Query("session_id"); sessionIDStr != "" {
		var session LiveTradingSession
		if db.Where("id = ? AND user_id = ?", sessionIDStr, uid).First(&session).Error != nil {
			c.JSON(400, gin.H{"error": "Session nicht gefunden"})
			return
		}
		if db.First(&config, session.ConfigID).Error != nil || !config.AlpacaEnabled || config.AlpacaApiKey == "" {
			c.JSON(400, gin.H{"error": "Alpaca nicht konfiguriert"})
			return
		}
	} else if db.Where("user_id = ?", uid).First(&config).Error != nil || !config.AlpacaEnabled || config.AlpacaApiKey == "" {
		c.JSON(400, gin.H{"error": "Alpaca nicht konfiguriert"})
		return
	}

	account, err := alpacaGetAccount(config)
	if err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("Account-Abfrage fehlgeschlagen: %v", err)})
		return
	}

	positions, err := alpacaGetPositions(config)
	if err != nil {
		positions = []map[string]interface{}{}
	}

	orders, err := alpacaGetOrders(config)
	if err != nil {
		orders = []map[string]interface{}{}
	}

	// Build clean position list
	cleanPositions := []gin.H{}
	for _, p := range positions {
		avgEntry, _ := strconv.ParseFloat(fmt.Sprintf("%v", p["avg_entry_price"]), 64)
		currentPrice, _ := strconv.ParseFloat(fmt.Sprintf("%v", p["current_price"]), 64)
		marketValue, _ := strconv.ParseFloat(fmt.Sprintf("%v", p["market_value"]), 64)
		unrealizedPL, _ := strconv.ParseFloat(fmt.Sprintf("%v", p["unrealized_pl"]), 64)
		unrealizedPLPct, _ := strconv.ParseFloat(fmt.Sprintf("%v", p["unrealized_plpc"]), 64)
		qty, _ := strconv.ParseFloat(fmt.Sprintf("%v", p["qty"]), 64)
		costBasis, _ := strconv.ParseFloat(fmt.Sprintf("%v", p["cost_basis"]), 64)

		symbol := fmt.Sprintf("%v", p["symbol"])
		stockName := symbol
		var stock Stock
		if db.Where("symbol = ?", symbol).First(&stock).Error == nil && stock.Name != "" {
			stockName = stock.Name
		}

		cleanPositions = append(cleanPositions, gin.H{
			"symbol":            symbol,
			"name":              stockName,
			"qty":               qty,
			"side":              p["side"],
			"avg_entry_price":   avgEntry,
			"current_price":     currentPrice,
			"market_value":      marketValue,
			"cost_basis":        costBasis,
			"unrealized_pl":     unrealizedPL,
			"unrealized_pl_pct": unrealizedPLPct * 100,
		})
	}

	// Build clean order list (all orders) and calculate realized P&L
	cleanOrders := []gin.H{}
	// Track BUY fills per symbol to match with SELLs for realized P&L
	type fillInfo struct {
		qty   float64
		price float64
	}
	buyFills := map[string][]fillInfo{}  // symbol -> []fillInfo (chronological)
	realizedPL := 0.0
	realizedCount := 0
	realizedInvested := 0.0

	// First pass: collect all filled BUYs (chronological = ascending)
	for i := len(orders) - 1; i >= 0; i-- {
		o := orders[i]
		status, _ := o["status"].(string)
		side, _ := o["side"].(string)
		if status != "filled" || side != "buy" {
			continue
		}
		symbol, _ := o["symbol"].(string)
		fp := 0.0
		if fpStr, ok := o["filled_avg_price"].(string); ok && fpStr != "" {
			fp, _ = strconv.ParseFloat(fpStr, 64)
		}
		fq := 0.0
		if fqStr, ok := o["filled_qty"].(string); ok && fqStr != "" {
			fq, _ = strconv.ParseFloat(fqStr, 64)
		}
		if fp > 0 && fq > 0 {
			buyFills[symbol] = append(buyFills[symbol], fillInfo{qty: fq, price: fp})
		}
	}

	// Second pass: match SELLs to BUYs for realized P&L
	sellMatches := map[string][]fillInfo{} // symbol -> matched buy prices for sells
	for i := len(orders) - 1; i >= 0; i-- {
		o := orders[i]
		status, _ := o["status"].(string)
		side, _ := o["side"].(string)
		if status != "filled" || side != "sell" {
			continue
		}
		symbol, _ := o["symbol"].(string)
		sp := 0.0
		if spStr, ok := o["filled_avg_price"].(string); ok && spStr != "" {
			sp, _ = strconv.ParseFloat(spStr, 64)
		}
		sq := 0.0
		if sqStr, ok := o["filled_qty"].(string); ok && sqStr != "" {
			sq, _ = strconv.ParseFloat(sqStr, 64)
		}
		if sp > 0 && sq > 0 {
			if fills, ok := buyFills[symbol]; ok && len(fills) > 0 {
				buyPrice := fills[0].price
				buyQty := fills[0].qty
				matchQty := sq
				if matchQty > buyQty {
					matchQty = buyQty
				}
				pnl := (sp - buyPrice) * matchQty
				realizedPL += pnl
				realizedInvested += buyPrice * matchQty
				realizedCount++
				sellMatches[symbol] = append(sellMatches[symbol], fillInfo{qty: matchQty, price: buyPrice})
				// Remove matched buy
				if sq >= buyQty {
					buyFills[symbol] = fills[1:]
				} else {
					buyFills[symbol][0] = fillInfo{qty: buyQty - sq, price: buyPrice}
				}
			}
		}
	}

	// Build position lookup for unrealized P&L on open BUYs
	posLookup := map[string]float64{} // symbol -> current_price
	for _, p := range positions {
		sym, _ := p["symbol"].(string)
		cp := 0.0
		if cpStr, ok := p["current_price"].(string); ok && cpStr != "" {
			cp, _ = strconv.ParseFloat(cpStr, 64)
		}
		posLookup[sym] = cp
	}

	// Third pass: build clean order list with trade P&L
	sellMatchIdx := map[string]int{} // track which sell match we're on per symbol
	for _, o := range orders {
		filledPrice := 0.0
		if fp, ok := o["filled_avg_price"].(string); ok && fp != "" {
			filledPrice, _ = strconv.ParseFloat(fp, 64)
		}
		filledQty := 0.0
		if fq, ok := o["filled_qty"].(string); ok && fq != "" {
			filledQty, _ = strconv.ParseFloat(fq, 64)
		}
		rawQty := ""
		if q, ok := o["qty"].(string); ok {
			rawQty = q
		}
		stopPrice := 0.0
		if sp, ok := o["stop_price"].(string); ok && sp != "" {
			stopPrice, _ = strconv.ParseFloat(sp, 64)
		}
		limitPrice := 0.0
		if lp, ok := o["limit_price"].(string); ok && lp != "" {
			limitPrice, _ = strconv.ParseFloat(lp, 64)
		}
		orderClass, _ := o["order_class"].(string)
		orderType, _ := o["type"].(string)
		status, _ := o["status"].(string)
		side, _ := o["side"].(string)
		symbol, _ := o["symbol"].(string)

		// Stock name lookup
		stockName := symbol
		var stock Stock
		if db.Where("symbol = ?", symbol).First(&stock).Error == nil && stock.Name != "" {
			stockName = stock.Name
		}

		// Parse legs
		var legs []gin.H
		if rawLegs, ok := o["legs"].([]interface{}); ok {
			for _, rl := range rawLegs {
				if leg, ok := rl.(map[string]interface{}); ok {
					legSP := 0.0
					if sp, ok := leg["stop_price"].(string); ok && sp != "" {
						legSP, _ = strconv.ParseFloat(sp, 64)
					}
					legLP := 0.0
					if lp, ok := leg["limit_price"].(string); ok && lp != "" {
						legLP, _ = strconv.ParseFloat(lp, 64)
					}
					legStatus, _ := leg["status"].(string)
					legType, _ := leg["type"].(string)
					legSide, _ := leg["side"].(string)
					legs = append(legs, gin.H{
						"type": legType, "side": legSide,
						"stop_price": legSP, "limit_price": legLP, "status": legStatus,
					})
				}
			}
		}

		// Calculate trade P&L
		tradePL := 0.0
		tradePLPct := 0.0
		invested := 0.0
		if filledPrice > 0 && filledQty > 0 {
			invested = filledPrice * filledQty
		}
		if status == "filled" && side == "sell" && filledPrice > 0 {
			// Realized: match with buy
			idx := sellMatchIdx[symbol]
			if matches, ok := sellMatches[symbol]; ok && idx < len(matches) {
				buyPrice := matches[idx].price
				tradePL = (filledPrice - buyPrice) * filledQty
				if buyPrice > 0 {
					tradePLPct = (filledPrice - buyPrice) / buyPrice * 100
				}
				invested = buyPrice * filledQty
				sellMatchIdx[symbol] = idx + 1
			}
		}

		cleanOrders = append(cleanOrders, gin.H{
			"id": o["id"], "symbol": symbol, "name": stockName,
			"side": side, "qty": rawQty, "filled_qty": filledQty,
			"filled_avg_price": filledPrice, "status": status,
			"order_class": orderClass, "order_type": orderType,
			"stop_price": stopPrice, "limit_price": limitPrice,
			"legs": legs, "created_at": o["created_at"], "filled_at": o["filled_at"],
			"invested": invested, "trade_pl": tradePL, "trade_pl_pct": tradePLPct,
		})
	}

	equity, _ := strconv.ParseFloat(fmt.Sprintf("%v", account["equity"]), 64)
	buyingPower, _ := strconv.ParseFloat(fmt.Sprintf("%v", account["buying_power"]), 64)
	cash, _ := strconv.ParseFloat(fmt.Sprintf("%v", account["cash"]), 64)
	portfolioValue, _ := strconv.ParseFloat(fmt.Sprintf("%v", account["portfolio_value"]), 64)
	lastEquity, _ := strconv.ParseFloat(fmt.Sprintf("%v", account["last_equity"]), 64)
	dayChange := equity - lastEquity
	dayChangePct := 0.0
	if lastEquity > 0 {
		dayChangePct = dayChange / lastEquity * 100
	}

	c.JSON(200, gin.H{
		"account": gin.H{
			"equity":            equity,
			"buying_power":      buyingPower,
			"cash":              cash,
			"portfolio_value":   portfolioValue,
			"last_equity":       lastEquity,
			"day_change":        dayChange,
			"day_change_pct":    dayChangePct,
			"realized_pl":       realizedPL,
			"realized_count":    realizedCount,
			"realized_invested": realizedInvested,
			"status":            account["status"],
			"paper":             config.AlpacaPaper,
		},
		"positions": cleanPositions,
		"orders":    cleanOrders,
	})
}

func getLiveTradingConfig(c *gin.Context) {
	uid := liveOwnerUID(c)
	isAdmin, _ := c.Get("isAdmin")
	isAdminBool, _ := isAdmin.(bool)

	var config LiveTradingConfig
	if sessionIDStr := c.Query("session_id"); sessionIDStr != "" {
		var session LiveTradingSession
		if db.Where("id = ? AND user_id = ?", sessionIDStr, uid).First(&session).Error != nil {
			c.JSON(200, gin.H{"config": nil})
			return
		}
		if db.First(&config, session.ConfigID).Error != nil {
			c.JSON(200, gin.H{"config": nil})
			return
		}
	} else if db.Where("user_id = ?", uid).First(&config).Error != nil {
		c.JSON(200, gin.H{"config": nil})
		return
	}

	var params map[string]interface{}
	json.Unmarshal([]byte(config.ParamsJSON), &params)
	var symbols []string
	json.Unmarshal([]byte(config.Symbols), &symbols)
	var filters map[string]interface{}
	json.Unmarshal([]byte(config.FiltersJSON), &filters)

	result := gin.H{
		"id":                config.ID,
		"strategy":          config.Strategy,
		"interval":          config.Interval,
		"params":            params,
		"symbols":           symbols,
		"long_only":         config.LongOnly,
		"trade_amount":      config.TradeAmount,
		"filters":           filters,
		"filters_active":    config.FiltersActive,
		"currency":          config.Currency,
		"updated_at":        config.UpdatedAt,
		"alpaca_enabled":    config.AlpacaEnabled,
		"alpaca_paper":      config.AlpacaPaper,
		"alpaca_account_id": config.AlpacaAccountID,
	}

	// Only admins see API keys (masked)
	if isAdminBool {
		result["alpaca_api_key"] = maskKey(config.AlpacaApiKey)
		result["alpaca_secret_key"] = maskKey(config.AlpacaSecretKey)
	}

	c.JSON(200, result)
}

func startLiveTrading(c *gin.Context) {
	uid := liveOwnerUID(c)

	// Accept optional name
	var req struct {
		Name string `json:"name"`
	}
	c.ShouldBindJSON(&req)

	// Load template config
	var templateConfig LiveTradingConfig
	if db.Where("user_id = ?", uid).Order("updated_at DESC").First(&templateConfig).Error != nil {
		c.JSON(400, gin.H{"error": "Keine Konfiguration gefunden. Bitte zuerst in der Trading Arena 'Neue Session starten' drücken."})
		return
	}

	// Create NEW config for this session (copy strategy/symbols, blank Alpaca)
	newConfig := LiveTradingConfig{
		UserID:        uid,
		Strategy:      templateConfig.Strategy,
		Interval:      templateConfig.Interval,
		ParamsJSON:    templateConfig.ParamsJSON,
		Symbols:       templateConfig.Symbols,
		LongOnly:      templateConfig.LongOnly,
		TradeAmount:   templateConfig.TradeAmount,
		FiltersJSON:   templateConfig.FiltersJSON,
		FiltersActive: templateConfig.FiltersActive,
		Currency:      templateConfig.Currency,
		AlpacaPaper:   true,
		UpdatedAt:     time.Now(),
	}
	db.Create(&newConfig)

	// Session name: use provided or auto-generate
	sessionName := strings.TrimSpace(req.Name)
	if sessionName == "" {
		var sessionCount int64
		db.Model(&LiveTradingSession{}).Where("user_id = ?", uid).Count(&sessionCount)
		stratLabel := newConfig.Strategy
		labels := map[string]string{"regression_scalping": "Regression", "hybrid_ai_trend": "NW Bollinger", "macd_sr": "MACD S/R", "trippa_trade": "TrippaTrade"}
		if l, ok := labels[newConfig.Strategy]; ok {
			stratLabel = l
		}
		sessionName = fmt.Sprintf("#%d %s (%s)", sessionCount+1, stratLabel, newConfig.Interval)
	}

	now := time.Now()
	session := LiveTradingSession{
		UserID:      uid,
		ConfigID:    newConfig.ID,
		Name:        sessionName,
		Strategy:    newConfig.Strategy,
		Interval:    newConfig.Interval,
		ParamsJSON:  newConfig.ParamsJSON,
		Symbols:     newConfig.Symbols,
		LongOnly:    newConfig.LongOnly,
		TradeAmount: newConfig.TradeAmount,
		Currency:    newConfig.Currency,
		IsActive:    false,
		CreatedAt:   now,
	}
	db.Create(&session)
	// Explicit update: GORM ignores false bool on Create (zero value)
	db.Model(&session).Update("is_active", false)

	// Create initial LiveSessionStrategy (enabled)
	initialStrat := LiveSessionStrategy{
		SessionID:  session.ID,
		Name:       session.Strategy,
		ParamsJSON: session.ParamsJSON,
		Symbols:    session.Symbols,
		IsEnabled:  true,
		LongOnly:   session.LongOnly,
		CreatedAt:  time.Now(),
	}
	db.Create(&initialStrat)

	logLiveEvent(session.ID, "INFO", "-", fmt.Sprintf("Session '%s' erstellt (inaktiv) — Strategie: %s, Intervall: %s", session.Name, session.Strategy, session.Interval))

	c.JSON(200, gin.H{"session": session, "status": "created"})
}

func stopLiveTrading(c *gin.Context) {
	uid := liveOwnerUID(c)

	// Support optional session_id param, otherwise stop first active
	sessionIDParam := c.Query("session_id")
	var session LiveTradingSession
	if sessionIDParam != "" {
		if db.Where("id = ? AND user_id = ? AND is_active = ?", sessionIDParam, uid, true).First(&session).Error != nil {
			c.JSON(400, gin.H{"error": "Session nicht gefunden oder nicht aktiv"})
			return
		}
	} else {
		if db.Where("user_id = ? AND is_active = ?", uid, true).First(&session).Error != nil {
			c.JSON(400, gin.H{"error": "Keine aktive Session gefunden"})
			return
		}
	}

	// Stop scheduler for this session
	liveSchedulerMu.Lock()
	if state, ok := liveSchedulers[session.ID]; ok {
		if state.UsesSharedWS && sharedWS != nil {
			sharedWS.RemoveSession(session.ID)
		} else if state.WSClient != nil {
			state.WSClient.Close()
		}
		close(state.StopChan)
		delete(liveSchedulers, session.ID)
	}
	liveSchedulerMu.Unlock()

	// Load config for Alpaca close orders
	var stopConfig LiveTradingConfig
	if session.ConfigID > 0 {
		db.First(&stopConfig, session.ConfigID)
	} else {
		db.Where("user_id = ?", session.UserID).First(&stopConfig)
	}

	// Close all open positions and clear in-memory guards
	var openPositions []LiveTradingPosition
	db.Where("session_id = ? AND is_closed = ?", session.ID, false).Find(&openPositions)
	now := time.Now()
	for _, pos := range openPositions {
		liveOpenPosGuard.Delete(openPosGuardKey(session.ID, pos.StrategyID, pos.Symbol))
		pos.IsClosed = true
		pos.ClosePrice = pos.CurrentPrice
		pos.CloseTime = &now
		pos.CloseReason = "MANUAL"
		if pos.NativeCurrency != "USD" {
			pos.ClosePriceUSD = convertToUSD(pos.CurrentPrice, pos.NativeCurrency)
		} else {
			pos.ClosePriceUSD = pos.CurrentPrice
		}
		if pos.Direction == "LONG" {
			pos.ProfitLossPct = (pos.ClosePrice - pos.EntryPrice) / pos.EntryPrice * 100
		} else {
			pos.ProfitLossPct = (pos.EntryPrice - pos.ClosePrice) / pos.EntryPrice * 100
		}
		pos.ProfitLossAmt = pos.InvestedAmount * pos.ProfitLossPct / 100
		db.Save(&pos)
		logLiveEvent(session.ID, "CLOSE", pos.Symbol, fmt.Sprintf("MANUAL geschlossen %s @ %.4f (%.2f%%, %.2f EUR)", pos.Direction, pos.ClosePrice, pos.ProfitLossPct, pos.ProfitLossAmt))
		// Alpaca: Sell-Order with specific quantity (not DELETE which closes ALL positions for symbol)
		if pos.AlpacaOrderID != "" && stopConfig.AlpacaEnabled {
			side := "sell"
			if pos.Direction == "SHORT" {
				side = "buy"
			}
			if _, err := alpacaPlaceOrder(pos.Symbol, pos.Quantity, side, stopConfig); err != nil {
				logLiveEvent(session.ID, "ERROR", pos.Symbol, fmt.Sprintf("Alpaca Close fehlgeschlagen: %v", err))
			} else {
				logLiveEvent(session.ID, "ALPACA", pos.Symbol, fmt.Sprintf("Position geschlossen via Alpaca: %s %gx %s", side, pos.Quantity, pos.Symbol))
			}
		}
	}

	db.Model(&session).Updates(map[string]interface{}{"is_active": false, "stopped_at": &now})
	session.IsActive = false
	session.StoppedAt = &now

	logLiveEvent(session.ID, "INFO", "-", "Session gestoppt")
	c.JSON(200, gin.H{"session": session, "status": "stopped"})
}

// ==================== Live Session Strategy Management ====================

func getLiveSessionStrategies(c *gin.Context) {
	id := c.Param("id")
	uid := liveOwnerUID(c)

	var session LiveTradingSession
	if db.Where("id = ? AND user_id = ?", id, uid).First(&session).Error != nil {
		c.JSON(404, gin.H{"error": "Session nicht gefunden"})
		return
	}

	var strategies []LiveSessionStrategy
	db.Where("session_id = ?", session.ID).Order("id ASC").Find(&strategies)
	c.JSON(200, gin.H{"strategies": strategies})
}

func addLiveSessionStrategy(c *gin.Context) {
	id := c.Param("id")
	uid := liveOwnerUID(c)

	var session LiveTradingSession
	if db.Where("id = ? AND user_id = ?", id, uid).First(&session).Error != nil {
		c.JSON(404, gin.H{"error": "Session nicht gefunden"})
		return
	}

	var req struct {
		Strategy string   `json:"strategy"`
		Params   string   `json:"params"`
		Symbols  []string `json:"symbols"`
		LongOnly bool     `json:"long_only"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Ungültige Anfrage"})
		return
	}

	// Check if same strategy+params already exists for this session
	var existingStrat LiveSessionStrategy
	if db.Where("session_id = ? AND name = ? AND params_json = ?", session.ID, req.Strategy, req.Params).First(&existingStrat).Error == nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("Strategie '%s' mit gleichen Parametern existiert bereits in dieser Session", req.Strategy)})
		return
	}

	symbolsJSON, _ := json.Marshal(req.Symbols)
	strat := LiveSessionStrategy{
		SessionID:  session.ID,
		Name:       req.Strategy,
		ParamsJSON: req.Params,
		Symbols:    string(symbolsJSON),
		IsEnabled:  false, // Hot-add: starts disabled
		LongOnly:   req.LongOnly,
		CreatedAt:  time.Now(),
	}
	db.Create(&strat)
	// GORM treats false as zero-value, so explicit update if needed
	if !req.LongOnly {
		db.Model(&strat).Update("long_only", false)
	}

	// Merge symbols into session (union, no duplicates)
	var existingSymbols []string
	json.Unmarshal([]byte(session.Symbols), &existingSymbols)
	symbolSet := map[string]bool{}
	for _, s := range existingSymbols {
		symbolSet[s] = true
	}
	newSymbols := []string{}
	for _, s := range req.Symbols {
		if !symbolSet[s] {
			newSymbols = append(newSymbols, s)
			symbolSet[s] = true
		}
	}
	if len(newSymbols) > 0 {
		merged := append(existingSymbols, newSymbols...)
		mergedJSON, _ := json.Marshal(merged)
		db.Model(&session).Update("symbols", string(mergedJSON))

		// If session is active and using shared WS, subscribe new symbols
		liveSchedulerMu.Lock()
		if state, ok := liveSchedulers[session.ID]; ok && state.UsesSharedWS && sharedWS != nil {
			sharedWS.AddSymbols(newSymbols)
		}
		liveSchedulerMu.Unlock()
	}

	logLiveEvent(session.ID, "INFO", "-", fmt.Sprintf("Strategie '%s' hinzugefügt (deaktiviert) — %d Symbole", req.Strategy, len(req.Symbols)))
	c.JSON(200, gin.H{"strategy": strat, "status": "added"})
}

func toggleLiveSessionStrategy(c *gin.Context) {
	id := c.Param("id")
	strategyId := c.Param("strategyId")
	uid := liveOwnerUID(c)

	var session LiveTradingSession
	if db.Where("id = ? AND user_id = ?", id, uid).First(&session).Error != nil {
		c.JSON(404, gin.H{"error": "Session nicht gefunden"})
		return
	}

	// Only allow toggle when session is NOT active
	if session.IsActive {
		c.JSON(400, gin.H{"error": "Strategien können nur bei gestoppter Session aktiviert/deaktiviert werden"})
		return
	}

	var strat LiveSessionStrategy
	if db.Where("id = ? AND session_id = ?", strategyId, session.ID).First(&strat).Error != nil {
		c.JSON(404, gin.H{"error": "Strategie nicht gefunden"})
		return
	}

	strat.IsEnabled = !strat.IsEnabled
	db.Save(&strat)

	status := "deaktiviert"
	if strat.IsEnabled {
		status = "aktiviert"
	}
	logLiveEvent(session.ID, "INFO", "-", fmt.Sprintf("Strategie '%s' %s", strat.Name, status))
	c.JSON(200, gin.H{"strategy": strat, "status": status})
}

func getLiveTradingStatus(c *gin.Context) {
	uid := liveOwnerUID(c)

	// Collect all active sessions for this user
	var activeSessions []LiveTradingSession
	db.Where("user_id = ? AND is_active = ?", uid, true).Find(&activeSessions)

	liveSchedulerMu.Lock()
	hasRunning := len(liveSchedulers) > 0
	liveSchedulerMu.Unlock()

	// Build per-session status array
	sessionsStatus := []gin.H{}
	for _, session := range activeSessions {
		var symbols []string
		json.Unmarshal([]byte(session.Symbols), &symbols)

		var openCount, closedCount int64
		var totalPnl float64
		db.Model(&LiveTradingPosition{}).Where("session_id = ? AND is_closed = ?", session.ID, false).Count(&openCount)
		db.Model(&LiveTradingPosition{}).Where("session_id = ? AND is_closed = ?", session.ID, true).Count(&closedCount)
		db.Model(&LiveTradingPosition{}).Where("session_id = ?", session.ID).Select("COALESCE(SUM(profit_loss_amt), 0)").Row().Scan(&totalPnl)

		sStatus := gin.H{
			"session_id":       session.ID,
			"name":             session.Name,
			"interval":         session.Interval,
			"strategy":         session.Strategy,
			"started_at":       session.StartedAt,
			"last_poll_at":     session.LastPollAt,
			"next_poll_at":     session.NextPollAt,
			"total_polls":      session.TotalPolls,
			"symbols_count":    len(symbols),
			"open_positions":   openCount,
			"closed_positions": closedCount,
			"total_pnl":        totalPnl,
			"currency":         session.Currency,
		}

		liveSchedulerMu.Lock()
		if state, ok := liveSchedulers[session.ID]; ok {
			sStatus["is_polling"] = state.IsPolling
			sStatus["scan_progress_current"] = state.ScanProgress
			sStatus["scan_progress_total"] = state.ScanTotal
			sStatus["current_symbol"] = state.CurrentSymbol
			sStatus["alpaca_active"] = state.AlpacaActive
			if !state.AlpacaLastChecked.IsZero() {
				sStatus["alpaca_last_checked"] = state.AlpacaLastChecked
			}
			if state.AlpacaError != "" {
				sStatus["alpaca_error"] = state.AlpacaError
			}
			// WebSocket mode info
			if state.Mode != "" {
				sStatus["mode"] = state.Mode
			}
			sStatus["ws_connected"] = state.WSConnected
			if !state.LastBarReceived.IsZero() {
				sStatus["last_bar_received"] = state.LastBarReceived
			}
		}
		liveSchedulerMu.Unlock()

		var symbolPrices map[string]float64
		if json.Unmarshal([]byte(session.SymbolPricesJSON), &symbolPrices) == nil && symbolPrices != nil {
			sStatus["symbol_prices"] = symbolPrices
		}

		sessionsStatus = append(sessionsStatus, sStatus)
	}

	result := gin.H{
		"is_running":        hasRunning || len(activeSessions) > 0,
		"active_sessions":   sessionsStatus,
		"alpaca_ws":         sharedWS != nil && sharedWS.IsConnected(),
		"alpaca_configured": alpacaDataKey != "",
		"market_open":       isUSMarketOpen(),
	}

	// Backwards compat: if exactly 1 active session, flatten into top-level
	if len(sessionsStatus) == 1 {
		for k, v := range sessionsStatus[0] {
			result[k] = v
		}
	}

	// If not running, check for last resumable session (admin only)
	isAdmin, _ := c.Get("isAdmin")
	isAdminBool, _ := isAdmin.(bool)
	if len(activeSessions) == 0 && isAdminBool {
		var lastSession LiveTradingSession
		if db.Where("user_id = ? AND is_active = ?", uid, false).Order("stopped_at DESC").First(&lastSession).Error == nil {
			var config LiveTradingConfig
			canResume := true // Sessions can always be resumed now
			if db.First(&config, lastSession.ConfigID).Error != nil {
				canResume = false
			}

			var symbols []string
			json.Unmarshal([]byte(lastSession.Symbols), &symbols)

			var openCount int64
			db.Model(&LiveTradingPosition{}).Where("session_id = ? AND is_closed = ?", lastSession.ID, false).Count(&openCount)

			result["last_session"] = gin.H{
				"id":             lastSession.ID,
				"name":           lastSession.Name,
				"strategy":       lastSession.Strategy,
				"interval":       lastSession.Interval,
				"started_at":     lastSession.StartedAt,
				"stopped_at":     lastSession.StoppedAt,
				"total_polls":    lastSession.TotalPolls,
				"symbols_count":  len(symbols),
				"open_positions": openCount,
				"can_resume":     canResume,
			}
		}
	}

	c.JSON(200, result)
}

func getLiveTradingSessions(c *gin.Context) {
	uid := liveOwnerUID(c)

	var sessions []LiveTradingSession
	db.Where("user_id = ?", uid).Order("id DESC").Find(&sessions)

	// Resume check is now per-session (each session has its own config)

	results := []gin.H{}
	for _, s := range sessions {
		var symbols []string
		json.Unmarshal([]byte(s.Symbols), &symbols)

		var totalTrades int64
		var totalPnl float64
		var wins int64
		db.Model(&LiveTradingPosition{}).Where("session_id = ?", s.ID).Count(&totalTrades)
		db.Model(&LiveTradingPosition{}).Where("session_id = ?", s.ID).Select("COALESCE(SUM(profit_loss_amt), 0)").Row().Scan(&totalPnl)
		db.Model(&LiveTradingPosition{}).Where("session_id = ? AND profit_loss_pct > 0", s.ID).Count(&wins)

		winRate := 0.0
		if totalTrades > 0 {
			winRate = float64(wins) / float64(totalTrades) * 100
		}

		// Stopped sessions can be resumed if they have a config
		var sessionConfig LiveTradingConfig
		canResume := !s.IsActive && db.First(&sessionConfig, s.ConfigID).Error == nil

		var strategiesCount int64
		db.Model(&LiveSessionStrategy{}).Where("session_id = ?", s.ID).Count(&strategiesCount)

		results = append(results, gin.H{
			"id":               s.ID,
			"name":             s.Name,
			"strategy":         s.Strategy,
			"interval":         s.Interval,
			"symbols_count":    len(symbols),
			"is_active":        s.IsActive,
			"started_at":       s.StartedAt,
			"stopped_at":       s.StoppedAt,
			"total_polls":      s.TotalPolls,
			"total_trades":     totalTrades,
			"total_pnl":        totalPnl,
			"win_rate":         winRate,
			"currency":         s.Currency,
			"can_resume":       canResume,
			"strategies_count": strategiesCount,
		})
	}

	c.JSON(200, gin.H{"sessions": results})
}

func getLiveTradingSession(c *gin.Context) {
	uid := liveOwnerUID(c)
	id := c.Param("id")

	var session LiveTradingSession
	if db.Where("id = ? AND user_id = ?", id, uid).First(&session).Error != nil {
		c.JSON(404, gin.H{"error": "Session nicht gefunden"})
		return
	}

	var symbols []string
	json.Unmarshal([]byte(session.Symbols), &symbols)
	var params map[string]interface{}
	json.Unmarshal([]byte(session.ParamsJSON), &params)

	var positions []LiveTradingPosition
	db.Where("session_id = ?", session.ID).Order("created_at DESC").Find(&positions)

	var strategies []LiveSessionStrategy
	db.Where("session_id = ?", session.ID).Order("id ASC").Find(&strategies)

	// Build strategy name map for positions
	stratNameMap := map[uint]string{}
	for _, st := range strategies {
		stratNameMap[st.ID] = st.Name
	}
	// Enrich positions with strategy_name
	type posWithStrategy struct {
		LiveTradingPosition
		StrategyName string `json:"strategy_name"`
	}
	enrichedPositions := make([]posWithStrategy, len(positions))
	for i, p := range positions {
		enrichedPositions[i] = posWithStrategy{LiveTradingPosition: p, StrategyName: stratNameMap[p.StrategyID]}
	}

	result := gin.H{
		"session":    session,
		"symbols":    symbols,
		"params":     params,
		"positions":  enrichedPositions,
		"strategies": strategies,
	}

	var symbolPrices map[string]float64
	if json.Unmarshal([]byte(session.SymbolPricesJSON), &symbolPrices) == nil && symbolPrices != nil {
		result["symbol_prices"] = symbolPrices
	}

	c.JSON(200, result)
}

func resumeLiveTrading(c *gin.Context) {
	uid := liveOwnerUID(c)
	id := c.Param("id")

	// Load session
	var session LiveTradingSession
	if db.Where("id = ? AND user_id = ?", id, uid).First(&session).Error != nil {
		c.JSON(404, gin.H{"error": "Session nicht gefunden"})
		return
	}

	if session.IsActive {
		c.JSON(400, gin.H{"error": "Session ist bereits aktiv"})
		return
	}

	// Reactivate session
	updates := map[string]interface{}{
		"is_active":  true,
		"stopped_at": nil,
	}
	if session.StartedAt.IsZero() {
		updates["started_at"] = time.Now()
	}
	db.Model(&session).Updates(updates)

	// Load config to check for Alpaca WS capability
	var config LiveTradingConfig
	if session.ConfigID > 0 {
		db.First(&config, session.ConfigID)
	} else {
		db.Where("user_id = ?", session.UserID).Order("updated_at DESC").First(&config)
	}

	// Initialize in-memory open position guard from DB
	initOpenPosGuard(session.ID)

	// Start scheduler
	state := &liveSessionState{
		StopChan: make(chan struct{}),
	}
	liveSchedulerMu.Lock()
	liveSchedulers[session.ID] = state
	liveSchedulerMu.Unlock()

	if config.AlpacaEnabled && config.AlpacaApiKey != "" {
		state.Mode = "websocket"
		go runLiveWebSocket(state, session.ID, config)
	} else {
		state.Mode = "polling"
		go runLiveScheduler(state, session.ID)
	}

	logLiveEvent(session.ID, "INFO", "-", fmt.Sprintf("Session '%s' fortgesetzt (mode: %s)", session.Name, state.Mode))
	c.JSON(200, gin.H{"session": session, "status": "resumed"})
}

// runLiveWebSocket runs a live trading session using Alpaca WebSocket for real-time bars.
func runLiveWebSocket(state *liveSessionState, sessionID uint, config LiveTradingConfig) {
	var session LiveTradingSession
	if db.First(&session, sessionID).Error != nil {
		return
	}

	var symbols []string
	json.Unmarshal([]byte(session.Symbols), &symbols)
	dur := intervalToDuration(session.Interval)

	logLiveEvent(sessionID, "INFO", "-", fmt.Sprintf("WebSocket-Modus: %d Symbole, Interval %s", len(symbols), session.Interval))

	// 1. Prefetch historical bars for strategy warmup
	cacheInterval := session.Interval
	ivMap := map[string]string{"1h": "60m", "1D": "1d", "1W": "1wk"}
	if mapped, ok := ivMap[cacheInterval]; ok {
		cacheInterval = mapped
	}
	triggerPriorityRefresh(symbols, cacheInterval, sessionID)

	// Run initial scan from cache
	runLiveScan(sessionID)

	// === Fix 2: Initialize in-memory session/strategy cache ===
	refreshSessionCache(state, sessionID)

	// === Fix 3: Load OHLCV data into memory ===
	loadOHLCVIntoMemory(state, symbols, cacheInterval)
	logLiveEvent(sessionID, "INFO", "-", fmt.Sprintf("OHLCV in-memory geladen: %d Symbole", len(state.ohlcvData)))

	// === Fix 1b: Create candle event channel ===
	state.candleChan = make(chan candleEvent, 4096)

	// === Fix 1d: Start worker goroutines ===
	const numWorkers = 20
	var workerWg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		workerWg.Add(1)
		go func() {
			defer workerWg.Done()
			for {
				select {
				case <-state.StopChan:
					return
				case evt, ok := <-state.candleChan:
					if !ok {
						return
					}
					processCandleEvent(state, evt, config)
				}
			}
		}()
	}
	logLiveEvent(sessionID, "INFO", "-", fmt.Sprintf("Worker-Pool gestartet: %d Worker", numWorkers))

	// 2. Create aggregators per symbol — onComplete sends to channel (Fix 1c)
	state.Aggregators = make(map[string]*BarAggregator)

	for _, sym := range symbols {
		symbol := sym
		agg := newBarAggregator(dur, func(candle OHLCV) {
			state.LastBarReceived = time.Now()
			// Non-blocking send to worker pool
			select {
			case state.candleChan <- candleEvent{symbol: symbol, candle: candle, cacheInterval: cacheInterval}:
			default:
				log.Printf("[LiveWS] WARNING: candleChan full, dropping candle for %s", symbol)
			}
		})
		state.Aggregators[symbol] = agg
	}

	// 3. Connect: shared WS or per-session fallback
	if sharedWS != nil {
		// Use shared WebSocket (single global connection)
		state.UsesSharedWS = true
		state.WSConnected = sharedWS.IsConnected()

		sharedWS.AddSession(sessionID, symbols, func(symbol string, bar AlpacaWSBar) {
			t, err := time.Parse(time.RFC3339, bar.T)
			if err != nil {
				return
			}
			state.LastBarReceived = time.Now()
			state.WSConnected = sharedWS.IsConnected()
			if agg, ok := state.Aggregators[symbol]; ok {
				agg.AddBar(OHLCV{
					Time: t.Unix(), Open: bar.O, High: bar.H,
					Low: bar.L, Close: bar.C, Volume: bar.V,
				})
			}
		})

		logLiveEvent(sessionID, "INFO", "-", fmt.Sprintf("[SharedWS] Session registriert — %d Symbole", len(symbols)))
	} else {
		// Fallback: eigener WSClient mit per-session keys
		wsClient, err := newAlpacaWSClient(config.AlpacaApiKey, config.AlpacaSecretKey)
		if err != nil {
			logLiveEvent(sessionID, "ERROR", "-", fmt.Sprintf("WebSocket-Verbindung fehlgeschlagen: %v — Fallback auf Polling", err))
			state.Mode = "polling"
			close(state.candleChan)
			workerWg.Wait()
			runLiveScheduler(state, sessionID)
			return
		}
		state.WSClient = wsClient
		state.WSConnected = true

		for _, sym := range symbols {
			symbol := sym
			agg := state.Aggregators[symbol]
			wsClient.OnBar(symbol, func(bar AlpacaWSBar) {
				t, err := time.Parse(time.RFC3339, bar.T)
				if err != nil {
					return
				}
				state.LastBarReceived = time.Now()
				state.WSConnected = wsClient.IsConnected()
				agg.AddBar(OHLCV{
					Time: t.Unix(), Open: bar.O, High: bar.H,
					Low: bar.L, Close: bar.C, Volume: bar.V,
				})
			})
		}

		if err := wsClient.Subscribe(symbols); err != nil {
			logLiveEvent(sessionID, "ERROR", "-", fmt.Sprintf("WebSocket subscribe fehlgeschlagen: %v", err))
		}

		logLiveEvent(sessionID, "INFO", "-", fmt.Sprintf("WebSocket connected (eigener Client) — subscribed to %d symbols", len(symbols)))
	}

	// 3b. Sofortiger Alpaca-Health-Check nach WS-Setup
	if _, err := alpacaGetAccount(config); err == nil {
		state.AlpacaActive = true
		state.AlpacaLastChecked = time.Now()
		state.AlpacaError = ""
	} else {
		state.AlpacaActive = false
		state.AlpacaError = err.Error()
		state.AlpacaLastChecked = time.Now()
	}

	// 4. Wait for stop signal, periodically update state + flush caches
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	var lastCacheRefresh time.Time
	var lastOHLCVFlush time.Time
	for {
		select {
		case <-state.StopChan:
			// Cleanup: close channel, wait for workers, flush dirty OHLCV
			close(state.candleChan)
			workerWg.Wait()
			flushDirtyOHLCV(state, cacheInterval)
			if state.UsesSharedWS {
				sharedWS.RemoveSession(sessionID)
			} else if state.WSClient != nil {
				state.WSClient.Close()
			}
			return
		case <-ticker.C:
			if state.UsesSharedWS {
				state.WSConnected = sharedWS.IsConnected()
			} else if state.WSClient != nil {
				state.WSConnected = state.WSClient.IsConnected()
			}
			// Alpaca broker health check every 60s
			if time.Since(state.AlpacaLastChecked) > 60*time.Second {
				_, err := alpacaGetAccount(config)
				state.AlpacaLastChecked = time.Now()
				if err != nil {
					state.AlpacaActive = false
					state.AlpacaError = err.Error()
				} else {
					state.AlpacaActive = true
					state.AlpacaError = ""
				}
			}
			// Fix 2: Refresh session/strategy cache every 30s
			if time.Since(lastCacheRefresh) > 30*time.Second {
				refreshSessionCache(state, sessionID)
				lastCacheRefresh = time.Now()
			}
			// Fix 3: Flush dirty OHLCV data to DB every 60s
			if time.Since(lastOHLCVFlush) > 60*time.Second {
				go flushDirtyOHLCV(state, cacheInterval)
				lastOHLCVFlush = time.Now()
			}
			// Flush expired aggregator candles (prevents stale candles when no new bars arrive)
			for _, agg := range state.Aggregators {
				agg.FlushIfExpired(3 * time.Second)
			}
		}
	}
}

// Live Scheduler
func runLiveScheduler(state *liveSessionState, sessionID uint) {
	var session LiveTradingSession
	if db.First(&session, sessionID).Error != nil {
		return
	}

	dur := intervalToDuration(session.Interval)
	const buffer = 1500 * time.Millisecond // wait 1.5s after candle close for Yahoo to finalize

	// Ensure persistent cache has data for all symbols
	var symbols []string
	json.Unmarshal([]byte(session.Symbols), &symbols)

	yahooInterval := session.Interval
	intervalMap := map[string]string{"1h": "60m", "1D": "1d", "1W": "1wk"}
	if mapped, ok := intervalMap[yahooInterval]; ok {
		yahooInterval = mapped
	}

	triggerPriorityRefresh(symbols, yahooInterval, sessionID)

	// First scan immediately (reads from persistent cache)
	runLiveScan(sessionID)

	for {
		now := time.Now()
		durSec := int64(dur.Seconds())
		nowUnix := now.Unix()
		nextAligned := ((nowUnix / durSec) + 1) * durSec
		waitUntil := time.Unix(nextAligned, 0).Add(buffer)
		waitDur := waitUntil.Sub(now)
		if waitDur <= 0 {
			waitDur = dur
		}

		select {
		case <-time.After(waitDur):
			runLiveScan(sessionID)
		case <-state.StopChan:
			return
		}
	}
}

// refreshLiveSymbols fetches fresh OHLCV data for all session symbols right after candle close.
// Uses 20 parallel workers with 200ms stagger to complete quickly (~8-15s for 760 symbols).
// This replaces the background worker's responsibility for these symbols during active sessions.
func refreshLiveSymbols(symbols []string, interval string, sessionID uint) {
	yahooInterval := interval
	intervalMap := map[string]string{"1h": "60m", "1D": "1d", "1W": "1wk"}
	if mapped, ok := intervalMap[yahooInterval]; ok {
		yahooInterval = mapped
	}

	deltaPeriod := getOHLCVDeltaPeriod(yahooInterval)
	start := time.Now()
	logLiveEvent(sessionID, "REFRESH", "-", fmt.Sprintf("Cache-Refresh gestartet — %d Symbole (%s)", len(symbols), yahooInterval))

	const maxWorkers = 20
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup
	var refreshed int64
	var failed int64

	for _, sym := range symbols {
		wg.Add(1)
		sem <- struct{}{}
		go func(symbol string) {
			defer wg.Done()
			defer func() { <-sem }()
			time.Sleep(200 * time.Millisecond)

			// Try Alpaca first for US stocks, then Yahoo fallback (consistent with backtest)
			var freshBars []OHLCV
			var err error
			useAlpaca := alpacaDataKey != "" && !isNonUSStock(symbol)
			if useAlpaca {
				freshBars, err = fetchOHLCVFromAlpaca(symbol, interval)
			}
			if !useAlpaca || err != nil || len(freshBars) == 0 {
				freshBars, err = fetchOHLCVFromYahoo(symbol, deltaPeriod, yahooInterval)
			}
			if err != nil || len(freshBars) == 0 {
				atomic.AddInt64(&failed, 1)
				return
			}

			var cache OHLCVCache
			if db.Where("symbol = ? AND interval = ?", symbol, yahooInterval).First(&cache).Error == nil && cache.DataJSON != "" {
				var cached []OHLCV
				if json.Unmarshal([]byte(cache.DataJSON), &cached) == nil && len(cached) > 0 {
					freshBars = mergeOHLCV(cached, freshBars)
				}
			}

			saveOHLCVCache(symbol, yahooInterval, freshBars)
			atomic.AddInt64(&refreshed, 1)
		}(sym)
	}
	wg.Wait()

	logLiveEvent(sessionID, "REFRESH", "-", fmt.Sprintf("Cache-Refresh abgeschlossen — %d/%d in %.1fs (fehlgeschlagen: %d)", refreshed, len(symbols), time.Since(start).Seconds(), failed))
}

func runLiveScan(sessionID uint) {
	state := getLiveSessionState(sessionID)
	if state == nil {
		return
	}

	state.IsPolling = true
	defer func() { state.IsPolling = false }()

	var session LiveTradingSession
	if db.First(&session, sessionID).Error != nil || !session.IsActive {
		return
	}

	// Guard: session must have a valid StartedAt to prevent retroactive trading
	if session.StartedAt.IsZero() || session.StartedAt.Year() < 2000 {
		logLiveEvent(sessionID, "WARN", "-", "Scan übersprungen: Session hat kein gültiges StartedAt")
		return
	}

	var symbols []string
	json.Unmarshal([]byte(session.Symbols), &symbols)

	// Targeted refresh: only in polling mode (WS mode gets bars via WebSocket)
	if state.WSClient == nil && !state.UsesSharedWS {
		refreshLiveSymbols(symbols, session.Interval, sessionID)
	}

	// Load config for Alpaca integration (per-session config)
	var liveConfig LiveTradingConfig
	if session.ConfigID == 0 {
		// Legacy fallback: old sessions without per-session config
		db.Where("user_id = ?", session.UserID).First(&liveConfig)
	} else {
		db.First(&liveConfig, session.ConfigID)
	}

	// Alpaca connection check before each poll
	if liveConfig.AlpacaEnabled && liveConfig.AlpacaApiKey != "" {
		_, err := alpacaGetAccount(liveConfig)
		state.AlpacaLastChecked = time.Now()
		if err != nil {
			state.AlpacaActive = false
			state.AlpacaError = err.Error()
			logLiveEvent(sessionID, "ERROR", "-", fmt.Sprintf("Alpaca Verbindung fehlgeschlagen: %v", err))
		} else {
			state.AlpacaActive = true
			state.AlpacaError = ""
		}
	}

	// Load all enabled strategies for this session
	var strategies []LiveSessionStrategy
	db.Where("session_id = ? AND is_enabled = ?", sessionID, true).Find(&strategies)
	if len(strategies) == 0 {
		logLiveEvent(sessionID, "SKIP", "-", "Keine aktiven Strategien")
		return
	}

	// Build strategy instances + symbol sets
	type stratInstance struct {
		strat    LiveSessionStrategy
		engine   TradingStrategy
		symbols  map[string]bool
	}
	var activeStrats []stratInstance
	for _, s := range strategies {
		engine := createStrategyFromJSON(s.Name, s.ParamsJSON)
		if engine == nil {
			logLiveEvent(sessionID, "SKIP", "-", fmt.Sprintf("Unbekannte Strategie: %s (ID %d)", s.Name, s.ID))
			continue
		}
		symSet := map[string]bool{}
		var stratSymbols []string
		json.Unmarshal([]byte(s.Symbols), &stratSymbols)
		for _, sym := range stratSymbols {
			symSet[sym] = true
		}
		activeStrats = append(activeStrats, stratInstance{strat: s, engine: engine, symbols: symSet})
	}

	state.ScanTotal = len(symbols)
	state.ScanProgress = 0
	state.CurrentSymbol = ""

	scanStart := time.Now()
	logLiveEvent(sessionID, "SCAN", "-", fmt.Sprintf("Poll gestartet — %d Aktien, %d Strategien (aus Cache)", len(symbols), len(activeStrats)))

	var priceMapMu sync.Mutex
	var progressDone int
	var progressMu sync.Mutex
	priceMap := map[string]float64{}
	skipped := 0

	// Map interval to cache key
	liveCacheInterval := session.Interval
	switch liveCacheInterval {
	case "1h":
		liveCacheInterval = "60m"
	case "1D":
		liveCacheInterval = "1d"
	case "1W":
		liveCacheInterval = "1wk"
	}

	for i, sym := range symbols {
		// Read from in-memory OHLCV cache first, fallback to DB cache
		ohlcv, ok := getOHLCVFromMemCache(sym, liveCacheInterval)
		if !ok || len(ohlcv) == 0 {
			var err error
			ohlcv, err = getOHLCVCached(sym, session.Interval, 0)
			if err != nil || len(ohlcv) == 0 {
				skipped++
				continue
			}
		}

		// Run each active strategy for this symbol
		for _, si := range activeStrats {
			if !si.symbols[sym] {
				continue // symbol not in this strategy's list
			}
			minBars := si.engine.RequiredBars()
			if len(ohlcv) >= minBars {
				if price, ok := processLiveSymbolWithData(session, sym, si.engine, ohlcv, liveConfig, si.strat); ok {
					priceMapMu.Lock()
					priceMap[sym] = price
					priceMapMu.Unlock()
				}
			}
		}

		progressMu.Lock()
		progressDone = i + 1
		progressMu.Unlock()

		state.ScanProgress = progressDone
		state.CurrentSymbol = sym
	}

	state.CurrentSymbol = ""

	scanDuration := time.Since(scanStart)
	logMsg := fmt.Sprintf("Poll abgeschlossen — %d Aktien, %d Strategien in %.1fs", len(symbols), len(activeStrats), scanDuration.Seconds())
	if skipped > 0 {
		logMsg += fmt.Sprintf(" (%d ohne Cache übersprungen)", skipped)
	}
	logLiveEvent(sessionID, "SCAN", "-", logMsg)

	// Update session poll stats + symbol prices
	now := time.Now()
	dur := intervalToDuration(session.Interval)
	durSec := int64(dur.Seconds())
	nextAligned := ((now.Unix() / durSec) + 1) * durSec
	next := time.Unix(nextAligned, 0).Add(1500 * time.Millisecond)
	pricesJSON, _ := json.Marshal(priceMap)
	db.Model(&session).Updates(map[string]interface{}{
		"last_poll_at":       now,
		"next_poll_at":       next,
		"total_polls":        gorm.Expr("total_polls + 1"),
		"symbol_prices_json": string(pricesJSON),
	})
}

func processLiveSymbolWithData(session LiveTradingSession, symbol string, strategy TradingStrategy, ohlcv []OHLCV, config LiveTradingConfig, strat ...LiveSessionStrategy) (float64, bool) {

	// Resolve strategy scope (for multi-strategy sessions)
	var strategyID uint
	var strategyName string
	longOnly := session.LongOnly
	if len(strat) > 0 {
		strategyID = strat[0].ID
		strategyName = strat[0].Name
		longOnly = strat[0].LongOnly
	}

	// Strip incomplete (still open) candle — only analyze fully closed bars
	// Keep lastPrice from the latest bar (even if open) for P&L updates
	lastPrice := ohlcv[len(ohlcv)-1].Close
	ivDur := intervalToDuration(session.Interval)
	if len(ohlcv) > 1 {
		lastBar := ohlcv[len(ohlcv)-1]
		candleEnd := lastBar.Time + int64(ivDur.Seconds())
		if candleEnd > time.Now().Unix() {
			// Last candle is still open → remove it for signal analysis
			ohlcv = ohlcv[:len(ohlcv)-1]
		}
	}

	nativeCurrency := getStockCurrency(symbol)
	if nativeCurrency == "" {
		nativeCurrency = "USD"
	}

	signals := strategy.Analyze(ohlcv)

	// Log signal detection (visible by default) vs. no-signal (DEBUG)
	if len(signals) > 0 {
		lastSig := signals[len(signals)-1]
		sigBarTime := time.Unix(ohlcv[lastSig.Index].Time, 0).Format("15:04")
		sigPrice := ohlcv[lastSig.Index].Close
		logLiveEvent(session.ID, "SIGNAL", symbol, fmt.Sprintf("%s erkannt @ %s (Kerze %s, Kurs: %.4f)", lastSig.Direction, time.Now().Format("15:04:05"), sigBarTime, sigPrice), strategyName)
	} else {
		logLiveEvent(session.ID, "DEBUG", symbol, fmt.Sprintf("Kein Signal @ %s (Kerzen: %d)", time.Now().Format("15:04:05"), len(ohlcv)), strategyName)
	}

	// Guard: if session has no StartedAt, skip ALL signal processing
	// (prevents retroactive trading on sessions that haven't been started yet)
	if session.StartedAt.IsZero() || session.StartedAt.Year() < 2000 {
		return lastPrice, false
	}
	sessionStartUnix := session.StartedAt.Unix()

	// Get existing open position for this symbol in this session+strategy
	posKey := openPosGuardKey(session.ID, strategyID, symbol)
	var existingPos LiveTradingPosition
	hasOpenPos := db.Where("session_id = ? AND strategy_id = ? AND symbol = ? AND is_closed = ?", session.ID, strategyID, symbol, false).First(&existingPos).Error == nil
	// Also check in-memory guard (catches positions where async DB write is still pending)
	if !hasOpenPos {
		if _, guardOpen := liveOpenPosGuard.Load(posKey); guardOpen {
			hasOpenPos = true
		}
	}

	// Process new signals (only after session start)
	for _, sig := range signals {
		if sig.Index < 0 || sig.Index >= len(ohlcv) {
			continue
		}
		signalBarTime := ohlcv[sig.Index].Time
		if signalBarTime < sessionStartUnix {
			continue
		}

		// Check if this signal was already processed (DB check)
		var duplicate LiveTradingPosition
		if db.Where("session_id = ? AND strategy_id = ? AND symbol = ? AND signal_index = ?", session.ID, strategyID, symbol, sig.Index).First(&duplicate).Error == nil {
			continue
		}

		if !hasOpenPos {
			// Skip new entries outside US market hours
			if !isUSMarketOpen() {
				logLiveEvent(session.ID, "SKIP", symbol, fmt.Sprintf("%s Signal übersprungen (Markt geschlossen)", sig.Direction), strategyName)
				continue
			}

			// LongOnly filter
			if longOnly && sig.Direction == "SHORT" {
				logLiveEvent(session.ID, "SKIP", symbol, "SHORT Signal übersprungen (Long Only)", strategyName)
				continue
			}

			// Use current market price (last closed candle's close) instead of backtest signal price
			entryPriceNative := ohlcv[len(ohlcv)-1].Close
			entryPriceUSD := entryPriceNative
			if nativeCurrency != "USD" {
				entryPriceUSD = convertToUSD(entryPriceNative, nativeCurrency)
			}

			tradeAmountUSD := session.TradeAmount // TradeAmount is always in USD
			posQty := 0.0
			if entryPriceUSD > 0 {
				posQty = math.Round(tradeAmountUSD/entryPriceUSD*1000000) / 1000000
			}
			if posQty <= 0 {
				posQty = 1
			}
			// Check if asset supports fractional shares — if not, round down to whole shares
			var watchlistItem TradingWatchlistItem
			if db.Where("symbol = ?", symbol).First(&watchlistItem).Error == nil && !watchlistItem.Fractionable {
				posQty = math.Floor(posQty)
				if posQty < 1 {
					logLiveEvent(session.ID, "SKIP", symbol, fmt.Sprintf("Nicht fractionable, Preis $%.2f > Trade-Amount $%.0f — übersprungen", entryPriceUSD, tradeAmountUSD), strategyName)
					continue
				}
			}

			// Scale SL/TP from signal entry price to actual entry price (proportional)
			actualSL := sig.StopLoss
			actualTP := sig.TakeProfit
			if sig.EntryPrice > 0 && entryPriceNative != sig.EntryPrice {
				ratio := entryPriceNative / sig.EntryPrice
				if actualSL > 0 {
					actualSL = math.Round(actualSL*ratio*100) / 100
				}
				if actualTP > 0 {
					actualTP = math.Round(actualTP*ratio*100) / 100
				}
			}
			// Alpaca guard: TP must be > entry + 0.01 (LONG) or < entry - 0.01 (SHORT)
			if sig.Direction == "LONG" {
				if actualTP > 0 && actualTP <= entryPriceNative+0.01 {
					actualTP = math.Round((entryPriceNative*1.005)*100) / 100 // fallback: +0.5%
				}
				if actualSL > 0 && actualSL >= entryPriceNative-0.01 {
					actualSL = math.Round((entryPriceNative*0.995)*100) / 100 // fallback: -0.5%
				}
			} else {
				if actualTP > 0 && actualTP >= entryPriceNative-0.01 {
					actualTP = math.Round((entryPriceNative*0.995)*100) / 100
				}
				if actualSL > 0 && actualSL <= entryPriceNative+0.01 {
					actualSL = math.Round((entryPriceNative*1.005)*100) / 100
				}
			}

			// Alpaca: Place simple fractional market order (SL/TP managed server-side)
			alpacaOrderID := ""
			if config.AlpacaEnabled && config.AlpacaApiKey != "" {
				side := "buy"
				if sig.Direction == "SHORT" {
					side = "sell"
				}
				orderResult, err := alpacaPlaceOrder(symbol, posQty, side, config)
				if err != nil {
					logLiveEvent(session.ID, "ERROR", symbol, fmt.Sprintf("Alpaca Order fehlgeschlagen: %v — Position wird NICHT eröffnet", err), strategyName)
					continue // Skip DB entry if Alpaca order failed
				}
				alpacaOrderID = orderResult.OrderID
				slInfo := ""
				tpInfo := ""
				if actualSL > 0 {
					slInfo = fmt.Sprintf(" SL:%.2f", actualSL)
				}
				if actualTP > 0 {
					tpInfo = fmt.Sprintf(" TP:%.2f", actualTP)
				}
				logLiveEvent(session.ID, "ALPACA", symbol, fmt.Sprintf("Order platziert: %s %gx %s [FRACTIONAL%s%s] (ID: %s, Status: %s)", side, posQty, symbol, slInfo, tpInfo, orderResult.OrderID, orderResult.Status), strategyName)
			}

			// DB: Create position only after successful Alpaca order (or if Alpaca disabled)
			pos := LiveTradingPosition{
				SessionID:      session.ID,
				StrategyID:     strategyID,
				Symbol:         symbol,
				Direction:      sig.Direction,
				EntryPrice:     entryPriceNative,
				EntryPriceUSD:  entryPriceUSD,
				EntryTime:      time.Now(),
				StopLoss:       actualSL,
				TakeProfit:     actualTP,
				CurrentPrice:   entryPriceNative,
				NativeCurrency: nativeCurrency,
				InvestedAmount: posQty * entryPriceUSD, // USD — no currency conversion
				Quantity:       posQty,
				SignalIndex:    sig.Index,
				AlpacaOrderID:  alpacaOrderID,
				CreatedAt:      time.Now(),
			}
			// Set in-memory guard BEFORE async DB write to prevent race condition
			liveOpenPosGuard.Store(posKey, true)
			livePositionWriteCh <- func() { db.Create(&pos) }

			hasOpenPos = true
			existingPos = pos
			slInfo := ""
			tpInfo := ""
			if actualSL > 0 {
				slInfo = fmt.Sprintf(", SL: %.2f", actualSL)
			}
			if actualTP > 0 {
				tpInfo = fmt.Sprintf(", TP: %.2f", actualTP)
			}
			logLiveEvent(session.ID, "OPEN", symbol, fmt.Sprintf("%s eröffnet @ %.4f %s%s%s", sig.Direction, entryPriceNative, nativeCurrency, slInfo, tpInfo), strategyName)

		} else if hasOpenPos && existingPos.Direction != sig.Direction {
			// Close on opposing signal
			closePriceNative := ohlcv[sig.Index].Open
			closeLivePosition(&existingPos, closePriceNative, "SIGNAL", nativeCurrency, config)
			liveOpenPosGuard.Delete(posKey)
			logLiveEvent(session.ID, "CLOSE", symbol, fmt.Sprintf("Gegensignal — %s geschlossen @ %.4f (%.2f%%, %.2f EUR)", existingPos.Direction, closePriceNative, existingPos.ProfitLossPct, existingPos.ProfitLossAmt), strategyName)
			hasOpenPos = false
		}
	}

	// SL/TP intrabar check for open position
	if hasOpenPos {
		entryUnix := existingPos.EntryTime.Unix()
		for _, bar := range ohlcv {
			if bar.Time <= entryUnix {
				continue
			}
			closed := false
			if existingPos.Direction == "LONG" {
				// SL checked BEFORE TP (matches backtest engine)
				if existingPos.StopLoss > 0 && bar.Low <= existingPos.StopLoss {
					closeLivePosition(&existingPos, existingPos.StopLoss, "SL", nativeCurrency, config)
					closed = true
				} else if existingPos.TakeProfit > 0 && bar.High >= existingPos.TakeProfit {
					closeLivePosition(&existingPos, existingPos.TakeProfit, "TP", nativeCurrency, config)
					closed = true
				}
			} else {
				if existingPos.StopLoss > 0 && bar.High >= existingPos.StopLoss {
					closeLivePosition(&existingPos, existingPos.StopLoss, "SL", nativeCurrency, config)
					closed = true
				} else if existingPos.TakeProfit > 0 && bar.Low <= existingPos.TakeProfit {
					closeLivePosition(&existingPos, existingPos.TakeProfit, "TP", nativeCurrency, config)
					closed = true
				}
			}
			if closed {
				liveOpenPosGuard.Delete(posKey)
				hasOpenPos = false
				break
			}
		}

		// Update current price if still open
		if hasOpenPos && len(ohlcv) > 0 {
			lastBar := ohlcv[len(ohlcv)-1]
			existingPos.CurrentPrice = lastBar.Close
			if existingPos.Direction == "LONG" {
				existingPos.ProfitLossPct = (lastBar.Close - existingPos.EntryPrice) / existingPos.EntryPrice * 100
			} else {
				existingPos.ProfitLossPct = (existingPos.EntryPrice - lastBar.Close) / existingPos.EntryPrice * 100
			}
			existingPos.ProfitLossAmt = existingPos.InvestedAmount * existingPos.ProfitLossPct / 100
			posCopy := existingPos
			livePositionWriteCh <- func() { db.Save(&posCopy) }
		}
	}
	return lastPrice, true
}

func closeLivePosition(pos *LiveTradingPosition, closePriceNative float64, reason, nativeCurrency string, config ...LiveTradingConfig) {
	now := time.Now()
	pos.IsClosed = true
	pos.ClosePrice = closePriceNative
	pos.CloseTime = &now
	pos.CloseReason = reason
	pos.CurrentPrice = closePriceNative

	if nativeCurrency != "USD" {
		pos.ClosePriceUSD = convertToUSD(closePriceNative, nativeCurrency)
	} else {
		pos.ClosePriceUSD = closePriceNative
	}

	if pos.Direction == "LONG" {
		pos.ProfitLossPct = (closePriceNative - pos.EntryPrice) / pos.EntryPrice * 100
	} else {
		pos.ProfitLossPct = (pos.EntryPrice - closePriceNative) / pos.EntryPrice * 100
	}
	pos.ProfitLossAmt = pos.InvestedAmount * pos.ProfitLossPct / 100

	// Serialize DB write via channel to avoid lock contention
	posCopy := *pos
	livePositionWriteCh <- func() { db.Save(&posCopy) }
	if reason == "SL" || reason == "TP" {
		logLiveEvent(pos.SessionID, reason, pos.Symbol, fmt.Sprintf("%s ausgelöst — %s geschlossen @ %.4f (%.2f%%, %.2f EUR)", reason, pos.Direction, closePriceNative, pos.ProfitLossPct, pos.ProfitLossAmt))
	}

	// Alpaca: Close position via sell/buy order with specific quantity (not DELETE which closes ALL)
	if len(config) > 0 && pos.AlpacaOrderID != "" && config[0].AlpacaEnabled {
		side := "sell"
		if pos.Direction == "SHORT" {
			side = "buy" // Cover short
		}
		_, err := alpacaPlaceOrder(pos.Symbol, pos.Quantity, side, config[0])
		if err != nil {
			logLiveEvent(pos.SessionID, "ERROR", pos.Symbol, fmt.Sprintf("Alpaca Position-Close fehlgeschlagen: %v", err))
		} else {
			logLiveEvent(pos.SessionID, "ALPACA", pos.Symbol, fmt.Sprintf("Position geschlossen via Alpaca (%s): %s %gx %s @ %.4f (P&L: %.2f%%)", reason, pos.Direction, pos.Quantity, pos.Symbol, closePriceNative, pos.ProfitLossPct))
		}
	}
}

// ==================== Live Symbol Analysis ====================

type ComparisonDetail struct {
	Type          string  `json:"type"`
	Message       string  `json:"message"`
	BacktestEntry float64 `json:"backtest_entry,omitempty"`
	LiveEntry     float64 `json:"live_entry,omitempty"`
	DiffPct       float64 `json:"diff_pct,omitempty"`
	Time          string  `json:"time,omitempty"`
}

type ComparisonResult struct {
	Matches    int                `json:"matches"`
	Mismatches int                `json:"mismatches"`
	Details    []ComparisonDetail `json:"details"`
}

func compareTradesWithPositions(backtestTrades []ArenaBacktestTrade, livePositions []LiveTradingPosition, sessionID uint, symbol string, sessionStart time.Time, longOnly bool, intervalSec float64) ComparisonResult {
	result := ComparisonResult{Details: []ComparisonDetail{}}

	// Filter backtest trades to only those after session start
	var relevantTrades []ArenaBacktestTrade
	for _, bt := range backtestTrades {
		if bt.EntryTime >= sessionStart.Unix() {
			if longOnly && bt.Direction == "SHORT" {
				continue
			}
			relevantTrades = append(relevantTrades, bt)
		}
	}

	matched := make(map[int]bool)
	maxTimeDiff := intervalSec * 2

	for _, bt := range relevantTrades {
		btTime := time.Unix(bt.EntryTime, 0)
		found := false

		for i, lp := range livePositions {
			if matched[i] {
				continue
			}

			timeDiff := math.Abs(float64(btTime.Unix() - lp.EntryTime.Unix()))
			if bt.Direction == lp.Direction && timeDiff <= maxTimeDiff {
				matched[i] = true
				found = true

				// Compare entry prices (use USD for consistency)
				refPrice := lp.EntryPriceUSD
				if refPrice == 0 {
					refPrice = lp.EntryPrice
				}
				if refPrice > 0 {
					priceDiff := math.Abs(bt.EntryPrice-refPrice) / refPrice * 100
					if priceDiff > 0.5 {
						detail := ComparisonDetail{
							Type:          "ENTRY_PRICE_DIFF",
							Message:       fmt.Sprintf("Entry-Preis Abweichung: Backtest %.4f vs Live %.4f (%.2f%%)", bt.EntryPrice, refPrice, priceDiff),
							BacktestEntry: bt.EntryPrice,
							LiveEntry:     refPrice,
							DiffPct:       priceDiff,
							Time:          btTime.Format(time.RFC3339),
						}
						result.Details = append(result.Details, detail)
						result.Mismatches++
						logLiveEvent(sessionID, "DATA_MISMATCH", symbol, fmt.Sprintf("⚠ %s", detail.Message))
					} else {
						result.Matches++
					}
				} else {
					result.Matches++
				}
				break
			}
		}

		if !found {
			msg := fmt.Sprintf("Backtest zeigt %s Signal bei %s (%.4f), aber keine passende Live-Position", bt.Direction, btTime.Format("02.01.2006 15:04"), bt.EntryPrice)
			result.Details = append(result.Details, ComparisonDetail{Type: "MISSING_POSITION", Message: msg, Time: btTime.Format(time.RFC3339)})
			result.Mismatches++
			logLiveEvent(sessionID, "DATA_MISMATCH", symbol, fmt.Sprintf("⚠ %s", msg))
		}
	}

	for i, lp := range livePositions {
		if !matched[i] {
			refPrice := lp.EntryPriceUSD
			if refPrice == 0 {
				refPrice = lp.EntryPrice
			}
			msg := fmt.Sprintf("Live-Position %s bei %s (%.4f) hat kein Backtest-Signal", lp.Direction, lp.EntryTime.Format("02.01.2006 15:04"), refPrice)
			result.Details = append(result.Details, ComparisonDetail{Type: "EXTRA_POSITION", Message: msg, Time: lp.EntryTime.Format(time.RFC3339)})
			result.Mismatches++
			logLiveEvent(sessionID, "DATA_MISMATCH", symbol, fmt.Sprintf("⚠ %s", msg))
		}
	}

	return result
}

func intervalToSeconds(interval string) float64 {
	switch interval {
	case "5m":
		return 300
	case "15m":
		return 900
	case "60m", "1h":
		return 3600
	case "2h":
		return 7200
	case "4h":
		return 14400
	case "1d":
		return 86400
	case "1wk":
		return 604800
	default:
		return 14400
	}
}

func analyzeLiveSymbolHandler(c *gin.Context) {
	var req struct {
		SessionID  uint   `json:"session_id"`
		Symbol     string `json:"symbol"`
		StrategyID uint   `json:"strategy_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Ungültige Anfrage"})
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if symbol == "" || req.SessionID == 0 {
		c.JSON(400, gin.H{"error": "session_id und symbol erforderlich"})
		return
	}

	uid := liveOwnerUID(c)

	var session LiveTradingSession
	if err := db.Where("id = ? AND user_id = ?", req.SessionID, uid).First(&session).Error; err != nil {
		c.JSON(404, gin.H{"error": "Session nicht gefunden"})
		return
	}

	// Load all strategies for this session
	var sessionStrategies []LiveSessionStrategy
	db.Where("session_id = ?", session.ID).Order("id ASC").Find(&sessionStrategies)

	// Build strategies list for response
	strategiesList := []gin.H{}
	for _, s := range sessionStrategies {
		strategiesList = append(strategiesList, gin.H{
			"id":         s.ID,
			"name":       s.Name,
			"is_enabled": s.IsEnabled,
		})
	}

	// Determine which strategy to use
	strategyName := session.Strategy
	paramsJSON := session.ParamsJSON
	longOnly := session.LongOnly
	var activeStrategyID uint

	if req.StrategyID > 0 {
		// Specific strategy requested
		for _, s := range sessionStrategies {
			if s.ID == req.StrategyID {
				strategyName = s.Name
				paramsJSON = s.ParamsJSON
				longOnly = s.LongOnly
				activeStrategyID = s.ID
				break
			}
		}
	} else if len(sessionStrategies) > 0 {
		// Use first enabled strategy
		for _, s := range sessionStrategies {
			if s.IsEnabled {
				strategyName = s.Name
				paramsJSON = s.ParamsJSON
				longOnly = s.LongOnly
				activeStrategyID = s.ID
				break
			}
		}
		// Fallback to first strategy if none enabled
		if activeStrategyID == 0 {
			s := sessionStrategies[0]
			strategyName = s.Name
			paramsJSON = s.ParamsJSON
			longOnly = s.LongOnly
			activeStrategyID = s.ID
		}
	}

	// Parse strategy params
	var params map[string]interface{}
	if paramsJSON != "" {
		json.Unmarshal([]byte(paramsJSON), &params)
	}
	if params == nil {
		params = map[string]interface{}{}
	}

	strategy, err := instantiateStrategy(strategyName, params)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	interval := session.Interval
	if _, ok := backtestPeriodMap[interval]; !ok {
		c.JSON(400, gin.H{"error": "Ungültiges Interval: " + interval})
		return
	}

	// Fetch OHLCV from cache (fresh within 24h)
	ohlcv, err := getOHLCVCached(symbol, interval, 24*time.Hour)
	if err != nil {
		c.JSON(500, gin.H{"error": "Daten konnten nicht geladen werden: " + err.Error()})
		return
	}
	if len(ohlcv) < 50 {
		c.JSON(400, gin.H{"error": "Nicht genug Daten für Analyse"})
		return
	}

	// Run fresh backtest
	result := runArenaBacktest(ohlcv, strategy)
	result.ChartData = ohlcv

	if provider, ok := strategy.(IndicatorProvider); ok {
		result.Indicators = provider.ComputeIndicators(ohlcv)
	}
	if provider, ok := strategy.(OverlayProvider); ok {
		result.Overlays = provider.ComputeOverlays(ohlcv)
	}

	// Load live positions for comparison — filter by strategy_id if available
	var livePositions []LiveTradingPosition
	if activeStrategyID > 0 {
		db.Where("session_id = ? AND strategy_id = ? AND symbol = ?", session.ID, activeStrategyID, symbol).Order("entry_time ASC").Find(&livePositions)
	} else {
		db.Where("session_id = ? AND symbol = ?", session.ID, symbol).Order("entry_time ASC").Find(&livePositions)
	}

	// Compare fresh backtest with live positions
	comparison := compareTradesWithPositions(
		result.Trades,
		livePositions,
		session.ID,
		symbol,
		session.StartedAt,
		longOnly,
		intervalToSeconds(session.Interval),
	)

	c.JSON(200, gin.H{
		"metrics":         result.Metrics,
		"trades":          result.Trades,
		"markers":         result.Markers,
		"indicators":      result.Indicators,
		"overlays":        result.Overlays,
		"chart_data":      result.ChartData,
		"comparison":      comparison,
		"strategies":      strategiesList,
		"active_strategy": gin.H{"id": activeStrategyID, "name": strategyName},
	})
}

func getLiveTradingLogs(c *gin.Context) {
	sessionID := c.Param("sessionId")

	afterID := uint(0)
	if v := c.Query("after_id"); v != "" {
		if parsed, err := strconv.ParseUint(v, 10, 32); err == nil {
			afterID = uint(parsed)
		}
	}

	strategyFilter := c.Query("strategy")

	var logs []LiveTradingLog
	q := db.Where("session_id = ?", sessionID)
	if afterID > 0 {
		q = q.Where("id > ?", afterID)
	}
	if strategyFilter != "" {
		q = q.Where("strategy = ?", strategyFilter)
	}
	q.Order("created_at DESC, id DESC").Limit(500).Find(&logs)

	var total int64
	tq := db.Model(&LiveTradingLog{}).Where("session_id = ?", sessionID)
	if strategyFilter != "" {
		tq = tq.Where("strategy = ?", strategyFilter)
	}
	tq.Count(&total)

	c.JSON(200, gin.H{"logs": logs, "total": total})
}

// ==================== Backtest Lab ====================

func runBacktestLabHandler(c *gin.Context) {
	var req BacktestLabRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Ungültige Anfrage: " + err.Error()})
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if symbol == "" {
		c.JSON(400, gin.H{"error": "Symbol erforderlich"})
		return
	}

	tslPercent := req.TSL // 0 = kein TSL

	// Load config for base mode
	var defConfig, aggConfig BXtrenderConfig
	var quantConfig BXtrenderQuantConfig
	var ditzConfig BXtrenderDitzConfig
	var traderConfig BXtrenderTraderConfig

	switch req.BaseMode {
	case "defensive":
		db.Where("mode = ?", "defensive").First(&defConfig)
		if defConfig.ID == 0 {
			defConfig = BXtrenderConfig{ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15}
		}
	case "aggressive":
		db.Where("mode = ?", "aggressive").First(&aggConfig)
		if aggConfig.ID == 0 {
			aggConfig = BXtrenderConfig{ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15}
		}
	case "quant":
		db.First(&quantConfig)
		if quantConfig.ID == 0 {
			quantConfig = BXtrenderQuantConfig{ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15, MaFilterOn: true, MaLength: 200, MaType: "EMA", TslPercent: 20.0}
		}
	case "ditz":
		db.First(&ditzConfig)
		if ditzConfig.ID == 0 {
			ditzConfig = BXtrenderDitzConfig{ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15, MaFilterOn: true, MaLength: 200, MaType: "EMA", TslPercent: 20.0}
		}
	case "trader":
		db.First(&traderConfig)
		if traderConfig.ID == 0 {
			traderConfig = BXtrenderTraderConfig{ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15, MaFilterOn: false, MaLength: 200, MaType: "EMA", TslPercent: 20.0}
		}
	default:
		c.JSON(400, gin.H{"error": "Unbekannter Base Mode: " + req.BaseMode})
		return
	}

	// Fetch monthly OHLCV
	monthlyOHLCV, err := fetchHistoricalDataServer(symbol)
	if err != nil || len(monthlyOHLCV) < 50 {
		c.JSON(500, gin.H{"error": "Monthly-Daten konnten nicht geladen werden"})
		return
	}

	// Fetch weekly OHLCV from cache
	weeklyOHLCV, err := getOHLCVCached(symbol, "1wk", 24*time.Hour)
	if err != nil || len(weeklyOHLCV) < 50 {
		c.JSON(500, gin.H{"error": "Weekly-Daten konnten nicht geladen werden"})
		return
	}

	// Calculate BXtrender on both timeframes
	var monthlyResult, weeklyResult BXtrenderResult
	switch req.BaseMode {
	case "defensive":
		monthlyResult = calculateBXtrenderServer(monthlyOHLCV, false, defConfig, 0, 0)
		weeklyResult = calculateBXtrenderServer(weeklyOHLCV, false, defConfig, 0, 0)
	case "aggressive":
		monthlyResult = calculateBXtrenderServer(monthlyOHLCV, true, aggConfig, 0, 0)
		weeklyResult = calculateBXtrenderServer(weeklyOHLCV, true, aggConfig, 0, 0)
	case "quant":
		monthlyResult = calculateBXtrenderQuantServer(monthlyOHLCV, quantConfig, 0, 0)
		weeklyResult = calculateBXtrenderQuantServer(weeklyOHLCV, quantConfig, 0, 0)
	case "ditz":
		monthlyResult = calculateBXtrenderDitzServer(monthlyOHLCV, ditzConfig, 0, 0)
		weeklyResult = calculateBXtrenderDitzServer(weeklyOHLCV, ditzConfig, 0, 0)
	case "trader":
		monthlyResult = calculateBXtrenderTraderServer(monthlyOHLCV, traderConfig, 0, 0)
		weeklyResult = calculateBXtrenderTraderServer(weeklyOHLCV, traderConfig, 0, 0)
	}

	if monthlyResult.Signal == "NO_DATA" {
		c.JSON(500, gin.H{"error": "Nicht genug Monthly-Daten für BXtrender-Berechnung"})
		return
	}

	// Build response data
	monthlyBars := make([]BacktestLabOHLCV, len(monthlyOHLCV))
	for i, bar := range monthlyOHLCV {
		monthlyBars[i] = BacktestLabOHLCV{Time: bar.Time, Open: bar.Open, High: bar.High, Low: bar.Low, Close: bar.Close, Volume: bar.Volume}
	}
	weeklyBars := make([]BacktestLabOHLCV, len(weeklyOHLCV))
	for i, bar := range weeklyOHLCV {
		weeklyBars[i] = BacktestLabOHLCV{Time: bar.Time, Open: bar.Open, High: bar.High, Low: bar.Low, Close: bar.Close, Volume: bar.Volume}
	}

	monthlyShortTV := buildTimeValues(monthlyOHLCV, monthlyResult.Short)
	monthlyLongTV := buildTimeValues(monthlyOHLCV, monthlyResult.Long)
	weeklyShortTV := buildTimeValues(weeklyOHLCV, weeklyResult.Short)
	weeklyLongTV := buildTimeValues(weeklyOHLCV, weeklyResult.Long)

	// If no custom rules, return base mode results
	if len(req.Rules) == 0 {
		trades, markers := convertServerTradesToArena(monthlyResult.Trades)
		metrics := calculateBacktestLabMetrics(trades)
		c.JSON(200, BacktestLabResponse{
			Metrics:      metrics,
			Trades:       trades,
			Markers:      markers,
			MonthlyBars:  monthlyBars,
			MonthlyShort: monthlyShortTV,
			MonthlyLong:  monthlyLongTV,
			WeeklyBars:   weeklyBars,
			WeeklyShort:  weeklyShortTV,
			WeeklyLong:   weeklyLongTV,
		})
		return
	}

	// Run custom rule evaluation
	trades, markers := evaluateBacktestLabRules(
		monthlyOHLCV, weeklyOHLCV,
		monthlyResult, weeklyResult,
		req.BaseMode, req.Rules, tslPercent,
	)
	metrics := calculateBacktestLabMetrics(trades)

	c.JSON(200, BacktestLabResponse{
		Metrics:      metrics,
		Trades:       trades,
		Markers:      markers,
		MonthlyBars:  monthlyBars,
		MonthlyShort: monthlyShortTV,
		MonthlyLong:  monthlyLongTV,
		WeeklyBars:   weeklyBars,
		WeeklyShort:  weeklyShortTV,
		WeeklyLong:   weeklyLongTV,
	})
}

func buildTimeValues(ohlcv []OHLCV, values []float64) []BacktestLabTimeValue {
	result := make([]BacktestLabTimeValue, 0, len(ohlcv))
	for i, bar := range ohlcv {
		if i < len(values) {
			result = append(result, BacktestLabTimeValue{Time: bar.Time, Value: values[i]})
		}
	}
	return result
}

func convertServerTradesToArena(serverTrades []ServerTrade) ([]ArenaBacktestTrade, []ChartMarker) {
	trades := []ArenaBacktestTrade{}
	markers := []ChartMarker{}
	var currentBuy *ServerTrade

	for _, st := range serverTrades {
		if st.Type == "BUY" {
			currentBuy = &ServerTrade{Type: st.Type, Time: st.Time, Price: st.Price}
			markers = append(markers, ChartMarker{
				Time: st.Time, Position: "belowBar", Color: "#22c55e", Shape: "arrowUp", Text: "BUY",
			})
		} else if st.Type == "SELL" && currentBuy != nil {
			returnPct := st.Return
			trades = append(trades, ArenaBacktestTrade{
				Direction: "LONG", EntryPrice: currentBuy.Price, EntryTime: currentBuy.Time,
				ExitPrice: st.Price, ExitTime: st.Time, ReturnPct: returnPct,
				ExitReason: "SIGNAL", IsOpen: false,
			})
			color := "#22c55e"
			if returnPct < 0 {
				color = "#ef4444"
			}
			markers = append(markers, ChartMarker{
				Time: st.Time, Position: "aboveBar", Color: color, Shape: "arrowDown", Text: "SELL",
			})
			currentBuy = nil
		}
	}
	// Open position
	if currentBuy != nil {
		trades = append(trades, ArenaBacktestTrade{
			Direction: "LONG", EntryPrice: currentBuy.Price, EntryTime: currentBuy.Time,
			IsOpen: true,
		})
	}
	return trades, markers
}

func calculateBacktestLabMetrics(trades []ArenaBacktestTrade) ArenaBacktestMetrics {
	wins, losses := 0, 0
	totalReturn := 0.0
	var winReturns, lossReturns []float64
	equity := 100.0
	peak := equity
	maxDD := 0.0

	for _, t := range trades {
		// Include open trades (same as runArenaBacktest)
		totalReturn += t.ReturnPct
		if t.ReturnPct >= 0 {
			wins++
			winReturns = append(winReturns, t.ReturnPct)
		} else {
			losses++
			lossReturns = append(lossReturns, t.ReturnPct)
		}
		equity *= (1 + t.ReturnPct/100)
		if equity > peak {
			peak = equity
		}
		dd := (peak - equity) / peak * 100
		if dd > maxDD {
			maxDD = dd
		}
	}

	totalTrades := wins + losses
	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(wins) / float64(totalTrades) * 100
	}
	riskReward := 0.0
	if len(winReturns) > 0 && len(lossReturns) > 0 {
		avgWin := 0.0
		for _, w := range winReturns {
			avgWin += w
		}
		avgWin /= float64(len(winReturns))
		avgLoss := 0.0
		for _, l := range lossReturns {
			avgLoss += math.Abs(l)
		}
		avgLoss /= float64(len(lossReturns))
		riskReward = avgWin / avgLoss
	}
	avgReturn := 0.0
	if totalTrades > 0 {
		avgReturn = totalReturn / float64(totalTrades)
	}

	sanitize := func(v float64) float64 {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0
		}
		return v
	}
	return ArenaBacktestMetrics{
		WinRate: sanitize(winRate), RiskReward: sanitize(riskReward), TotalReturn: sanitize(totalReturn),
		AvgReturn: sanitize(avgReturn), MaxDrawdown: sanitize(maxDD), NetProfit: sanitize(equity - 100),
		TotalTrades: totalTrades, Wins: wins, Losses: losses,
	}
}

// getBarSignalState determines the signal state at a given bar index using BXtrender values
func getBarSignalState(idx int, short, long []float64, baseMode string, inPosition bool) string {
	if idx < 1 || idx >= len(short) {
		return "WAIT"
	}
	shortCurr := short[idx]
	shortPrev := short[idx-1]
	longCurr := long[idx]
	longPrev := long[idx-1]

	switch baseMode {
	case "defensive":
		isBullish := shortCurr > 0
		wasBullish := shortPrev > 0
		isLightRed := shortCurr < 0 && shortCurr > shortPrev
		isDarkRed := shortCurr < 0 && shortCurr <= shortPrev

		// Count consecutive light red bars
		consecutiveLightRed := 0
		if isLightRed {
			consecutiveLightRed = 1
			for j := idx - 1; j >= 1; j-- {
				if short[j] < 0 && short[j] > short[j-1] {
					consecutiveLightRed++
				} else {
					break
				}
			}
		}
		justTurnedGreen := isBullish && !wasBullish

		if !inPosition && (justTurnedGreen || (isLightRed && consecutiveLightRed == 4)) {
			return "BUY"
		}
		if inPosition && isDarkRed {
			return "SELL"
		}
		if inPosition {
			return "HOLD"
		}
		return "WAIT"

	case "aggressive":
		isBullish := shortCurr > 0
		wasBullish := shortPrev > 0
		isLightRed := shortCurr < 0 && shortCurr > shortPrev
		isDarkRed := shortCurr < 0 && shortCurr <= shortPrev

		consecutiveLightRed := 0
		if isLightRed {
			consecutiveLightRed = 1
			for j := idx - 1; j >= 1; j-- {
				if short[j] < 0 && short[j] > short[j-1] {
					consecutiveLightRed++
				} else {
					break
				}
			}
		}
		justTurnedGreen := isBullish && !wasBullish
		_ = consecutiveLightRed

		if !inPosition && ((isLightRed && consecutiveLightRed == 1) || justTurnedGreen) {
			return "BUY"
		}
		if inPosition && isDarkRed {
			return "SELL"
		}
		if inPosition {
			return "HOLD"
		}
		return "WAIT"

	case "quant":
		bothPositiveNow := shortCurr > 0 && longCurr > 0
		bothPositivePrev := shortPrev > 0 && longPrev > 0

		if !inPosition && bothPositiveNow && !bothPositivePrev {
			return "BUY"
		}
		if inPosition && (shortCurr < 0 || longCurr < 0) {
			return "SELL"
		}
		if inPosition {
			return "HOLD"
		}
		return "WAIT"

	case "ditz":
		bothPositiveNow := shortCurr > 0 && longCurr > 0
		bothPositivePrev := shortPrev > 0 && longPrev > 0
		bothNegativeNow := shortCurr < 0 && longCurr < 0

		if !inPosition && bothPositiveNow && !bothPositivePrev {
			return "BUY"
		}
		if inPosition && bothNegativeNow {
			return "SELL"
		}
		if inPosition {
			return "HOLD"
		}
		return "WAIT"

	case "trader":
		// T3 signal line direction not available here directly from short/long arrays
		// For trader, we check short xtrender trend
		if !inPosition && shortCurr > shortPrev && shortPrev <= short[max(0, idx-2)] {
			return "BUY"
		}
		if inPosition && shortCurr < shortPrev && shortPrev >= short[max(0, idx-2)] {
			return "SELL"
		}
		if inPosition {
			return "HOLD"
		}
		return "WAIT"
	}
	return "WAIT"
}

// getBarConditionState checks a specific condition against BXtrender values
func getBarConditionState(condition string, idx int, short, long []float64, baseMode string, inPosition bool) bool {
	if condition == "ANY" {
		return true
	}

	shortCurr := short[idx]
	shortPrev := 0.0
	if idx > 0 {
		shortPrev = short[idx-1]
	}

	switch condition {
	case "FIRST_LIGHT_RED":
		if idx < 1 {
			return false
		}
		isLightRed := shortCurr < 0 && shortCurr > shortPrev
		if !isLightRed {
			return false
		}
		// Check it's the first light red (previous was not light red)
		if idx >= 2 {
			prevWasLightRed := short[idx-1] < 0 && short[idx-1] > short[idx-2]
			return !prevWasLightRed
		}
		return true

	case "BUY_TO_HOLD":
		// Previous bar was BUY, current bar is HOLD
		if idx < 2 {
			return false
		}
		prevSignal := getBarSignalState(idx-1, short, long, baseMode, false)
		currSignal := getBarSignalState(idx, short, long, baseMode, true) // assume in position for HOLD check
		return prevSignal == "BUY" && currSignal == "HOLD"

	case "BUY":
		// Check if bar would trigger BUY (needs inPosition=false)
		return getBarSignalState(idx, short, long, baseMode, false) == "BUY"

	case "SELL":
		// Check if bar would trigger SELL (needs inPosition=true)
		return getBarSignalState(idx, short, long, baseMode, true) == "SELL"

	case "HOLD":
		// Check if bar would be HOLD (in position, no sell signal)
		return getBarSignalState(idx, short, long, baseMode, true) == "HOLD"

	case "WAIT":
		// Check if bar would be WAIT (no position, no buy signal)
		return getBarSignalState(idx, short, long, baseMode, false) == "WAIT"

	default:
		return false
	}
}

// findWeeklyIndexForMonthlyBar finds the last weekly bar that ended before or at the monthly bar time
func findWeeklyIndexForMonthlyBar(monthlyTime int64, weeklyOHLCV []OHLCV) int {
	bestIdx := -1
	for i, bar := range weeklyOHLCV {
		if bar.Time <= monthlyTime {
			bestIdx = i
		} else {
			break
		}
	}
	return bestIdx
}

// findMonthlyIndexForWeeklyBar finds the last monthly bar with Time <= weeklyTime
func findMonthlyIndexForWeeklyBar(weeklyTime int64, monthlyOHLCV []OHLCV) int {
	bestIdx := -1
	for i, bar := range monthlyOHLCV {
		if bar.Time <= weeklyTime {
			bestIdx = i
		} else {
			break
		}
	}
	return bestIdx
}

// evaluateBacktestLabRules iterates WEEKLY bars as primary timeframe.
// For each weekly bar, the monthly signal state is looked up from the last completed monthly bar.
// Trades execute at the NEXT WEEKLY bar's open price.
func evaluateBacktestLabRules(
	monthlyOHLCV, weeklyOHLCV []OHLCV,
	monthlyResult, weeklyResult BXtrenderResult,
	baseMode string,
	rules []BacktestLabRule,
	tslPercent float64,
) ([]ArenaBacktestTrade, []ChartMarker) {
	trades := []ArenaBacktestTrade{}
	markers := []ChartMarker{}

	inPosition := false
	var entryPrice, highestPrice float64
	var entryTime int64

	// Separate entry and exit rules
	entryRules := []BacktestLabRule{}
	exitRules := []BacktestLabRule{}
	for _, r := range rules {
		if r.Type == "exit" {
			exitRules = append(exitRules, r)
		} else {
			entryRules = append(entryRules, r)
		}
	}

	// Determine start index (skip warmup for weekly bars)
	startIdx := 50
	if startIdx >= len(weeklyOHLCV) {
		return trades, markers
	}

	for i := startIdx; i < len(weeklyOHLCV); i++ {
		bar := weeklyOHLCV[i]
		price := bar.Close

		// Update highest price for TSL (checked every week)
		if inPosition && price > highestPrice {
			highestPrice = price
		}

		// Check TSL (disabled when tslPercent == 0)
		tslTriggered := false
		if tslPercent > 0 && inPosition && highestPrice > 0 {
			stopPrice := highestPrice * (1 - tslPercent/100)
			if price <= stopPrice {
				tslTriggered = true
			}
		}

		// Find corresponding monthly index for this weekly bar
		monthlyIdx := findMonthlyIndexForWeeklyBar(bar.Time, monthlyOHLCV)

		// Check entry rules (weekly is primary, monthly is state lookup)
		if !inPosition && len(entryRules) > 0 {
			for _, rule := range entryRules {
				// Monthly condition: check against last completed monthly bar
				monthlyMatch := true
				if monthlyIdx >= 1 && monthlyIdx < len(monthlyResult.Short) {
					monthlyMatch = getBarConditionState(rule.MonthlyCondition, monthlyIdx, monthlyResult.Short, monthlyResult.Long, baseMode, false)
				} else if rule.MonthlyCondition != "ANY" {
					monthlyMatch = false
				}

				// Weekly condition: check against current weekly bar
				weeklyMatch := getBarConditionState(rule.WeeklyCondition, i, weeklyResult.Short, weeklyResult.Long, baseMode, false)

				triggered := false
				if rule.Operator == "OR" {
					triggered = monthlyMatch || weeklyMatch
				} else {
					triggered = monthlyMatch && weeklyMatch
				}

				if triggered {
					// Execute at next weekly bar's open
					var execPrice float64
					var execTime int64
					if i+1 < len(weeklyOHLCV) && weeklyOHLCV[i+1].Open > 0 {
						execPrice = weeklyOHLCV[i+1].Open
						execTime = weeklyOHLCV[i+1].Time
					}
					if execPrice > 0 {
						entryPrice = execPrice
						entryTime = execTime
						highestPrice = execPrice
						inPosition = true
						markers = append(markers, ChartMarker{
							Time: execTime, Position: "belowBar", Color: "#22c55e", Shape: "arrowUp", Text: "BUY",
						})
						break
					}
				}
			}
		}

		// Check exit rules + TSL
		if inPosition {
			shouldExit := false
			exitReason := ""

			if tslTriggered {
				shouldExit = true
				exitReason = "TSL"
			}

			if !shouldExit && len(exitRules) > 0 {
				for _, rule := range exitRules {
					// Monthly condition: check against last completed monthly bar
					monthlyMatch := true
					if monthlyIdx >= 1 && monthlyIdx < len(monthlyResult.Short) {
						monthlyMatch = getBarConditionState(rule.MonthlyCondition, monthlyIdx, monthlyResult.Short, monthlyResult.Long, baseMode, true)
					} else if rule.MonthlyCondition != "ANY" {
						monthlyMatch = false
					}

					// Weekly condition: check against current weekly bar
					weeklyMatch := getBarConditionState(rule.WeeklyCondition, i, weeklyResult.Short, weeklyResult.Long, baseMode, true)

					triggered := false
					if rule.Operator == "OR" {
						triggered = monthlyMatch || weeklyMatch
					} else {
						triggered = monthlyMatch && weeklyMatch
					}

					if triggered {
						shouldExit = true
						exitReason = "SIGNAL"
						break
					}
				}
			}

			// If no exit rules defined, use base mode's sell signal on weekly
			if !shouldExit && len(exitRules) == 0 {
				baseSignal := getBarSignalState(i, weeklyResult.Short, weeklyResult.Long, baseMode, true)
				if baseSignal == "SELL" {
					shouldExit = true
					exitReason = "SIGNAL"
				}
			}

			if shouldExit {
				var execPrice float64
				var execTime int64
				if exitReason == "TSL" {
					// TSL triggers at stop price
					execPrice = highestPrice * (1 - tslPercent/100)
					execTime = bar.Time
				} else if i+1 < len(weeklyOHLCV) && weeklyOHLCV[i+1].Open > 0 {
					execPrice = weeklyOHLCV[i+1].Open
					execTime = weeklyOHLCV[i+1].Time
				}
				if execPrice > 0 {
					returnPct := (execPrice - entryPrice) / entryPrice * 100
					trades = append(trades, ArenaBacktestTrade{
						Direction: "LONG", EntryPrice: entryPrice, EntryTime: entryTime,
						ExitPrice: execPrice, ExitTime: execTime, ReturnPct: returnPct,
						ExitReason: exitReason, IsOpen: false,
					})
					color := "#22c55e"
					if returnPct < 0 {
						color = "#ef4444"
					}
					markers = append(markers, ChartMarker{
						Time: execTime, Position: "aboveBar", Color: color, Shape: "arrowDown", Text: exitReason,
					})
					inPosition = false
					entryPrice = 0
					highestPrice = 0
				}
			}
		}
	}

	// Open position at end
	if inPosition {
		trades = append(trades, ArenaBacktestTrade{
			Direction: "LONG", EntryPrice: entryPrice, EntryTime: entryTime,
			IsOpen: true,
		})
		markers = append(markers, ChartMarker{
			Time: entryTime, Position: "belowBar", Color: "#22c55e", Shape: "arrowUp", Text: "BUY",
		})
	}

	return trades, markers
}

// ==================== Backtest Lab Batch ====================

// getOHLCVCached returns OHLCV data from in-memory cache (fast path) or persistent DB cache.
// freshness=0: always from cache (no Yahoo call). freshness>0: fetch from Yahoo if cache older than freshness.
// For 2h/4h: reads 60m cache and aggregates via aggregateOHLCV.
func getOHLCVCached(symbol, interval string, freshness time.Duration) ([]OHLCV, error) {
	// With Alpaca: 2h/4h are fetched natively (no aggregation needed)
	// Without Alpaca: aggregate from 60m cache
	if (interval == "2h" || interval == "4h") && (alpacaDataKey == "" || isNonUSStock(symbol)) {
		factor := 2
		if interval == "4h" {
			factor = 4
		}
		hourly, err := getOHLCVCached(symbol, "60m", freshness)
		if err != nil {
			return nil, err
		}
		return aggregateOHLCV(hourly, factor), nil
	}

	// Map display intervals to cache key intervals
	cacheInterval := interval
	switch interval {
	case "1h":
		cacheInterval = "60m"
	case "1D":
		cacheInterval = "1d"
	case "1W":
		cacheInterval = "1wk"
	}

	// Fast path: memory cache (lazy-loads from file on miss)
	if bars, ok := getOHLCVFromMemCache(symbol, cacheInterval); ok {
		if freshness == 0 {
			return bars, nil
		}
		// Check file freshness
		if _, modTime, err := readOHLCVFile(symbol, cacheInterval); err == nil {
			if time.Since(modTime) < freshness {
				return bars, nil
			}
		}
	}

	// freshness=0 means "cache only, no fetch calls"
	if freshness == 0 {
		return nil, fmt.Errorf("no cache for %s/%s", symbol, interval)
	}

	// Cache miss or stale — Yahoo primary (fast, no rate-limit), Alpaca only for live-trading
	yahooInterval := cacheInterval
	period := backtestPeriodMap[yahooInterval]
	if period == "" {
		period = "60d"
	}

	// If we have cached data, try delta-fetch first
	if cached, ok := getOHLCVFromMemCache(symbol, cacheInterval); ok && len(cached) > 0 {
		deltaPeriod := getOHLCVDeltaPeriod(yahooInterval)
		freshBars, err := fetchOHLCVFromYahoo(symbol, deltaPeriod, yahooInterval)
		if err == nil && len(freshBars) > 0 {
			merged := mergeOHLCV(cached, freshBars)
			saveOHLCVCache(symbol, yahooInterval, merged)
			return merged, nil
		}
		return cached, nil
	}

	// Full Yahoo fetch
	ohlcv, err := fetchOHLCVFromYahoo(symbol, period, yahooInterval)
	if err != nil {
		return nil, err
	}
	if len(ohlcv) == 0 {
		return nil, fmt.Errorf("no data for %s/%s", symbol, interval)
	}

	saveOHLCVCache(symbol, yahooInterval, ohlcv)
	return ohlcv, nil
}

// saveOHLCVCache writes to memory cache + gzip file (no DB)
func saveOHLCVCache(symbol, interval string, ohlcv []OHLCV) {
	setOHLCVInMemCache(symbol, interval, ohlcv)
}

// getMonthlyOHLCVCached returns monthly OHLCV data with persistent caching.
// Uses fetchHistoricalDataServer (Yahoo→TwelveData→Aggregate fallback) on cache miss.
func getMonthlyOHLCVCached(symbol string, freshness time.Duration) ([]OHLCV, error) {
	// 1. Check file/memory cache
	if bars, ok := getOHLCVFromMemCache(symbol, "1mo"); ok {
		if freshness <= 0 {
			return bars, nil
		}
		if _, modTime, err := readOHLCVFile(symbol, "1mo"); err == nil && time.Since(modTime) < freshness {
			return bars, nil
		}
	}

	// 2. Cache miss/stale → full fallback chain
	data, err := fetchHistoricalDataServer(symbol)
	if err != nil {
		return nil, err
	}

	// 3. Cache the result
	if len(data) > 0 {
		saveOHLCVCache(symbol, "1mo", data)
	}

	return data, nil
}

// getOHLCVDeltaPeriod returns how far back to fetch for a delta update
func getOHLCVDeltaPeriod(interval string) string {
	switch interval {
	case "5m":
		return "1d"
	case "15m":
		return "5d"
	case "60m":
		return "5d"
	case "1d":
		return "3mo"
	case "1wk":
		return "6mo"
	case "1mo":
		return "2y"
	default:
		return "5d"
	}
}

// isUSMarketOpen returns true during US market hours (9:30-16:00 ET, weekdays)
func isUSMarketOpen() bool {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return true // assume open if timezone fails
	}
	now := time.Now().In(loc)
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}
	hour, min := now.Hour(), now.Minute()
	minutes := hour*60 + min
	return minutes >= 9*60+30 && minutes <= 16*60
}

// arenaPrefetchHandler prefetches OHLCV data for all trading watchlist symbols via SSE progress.
// Supports force=true to ignore cache freshness, and intervals=["5m","60m"] to fetch multiple at once.
func arenaPrefetchHandler(c *gin.Context) {
	var req struct {
		Interval  string   `json:"interval"`
		Intervals []string `json:"intervals"`
		USOnly    bool     `json:"us_only"`
		Force     bool     `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Interval == "" && len(req.Intervals) == 0) {
		req.Interval = "4h"
	}

	// Build list of cache intervals to fetch
	intervalsToPrefetch := req.Intervals
	if len(intervalsToPrefetch) == 0 {
		intervalsToPrefetch = []string{req.Interval}
	}
	type ivPair struct {
		raw   string // as sent (e.g. "1h", "60m")
		cache string // cache key (e.g. "60m")
	}
	var ivPairs []ivPair
	for _, iv := range intervalsToPrefetch {
		ci := iv
		switch iv {
		case "1h":
			ci = "60m"
		case "1D":
			ci = "1d"
		case "1W":
			ci = "1wk"
		}
		ivPairs = append(ivPairs, ivPair{raw: iv, cache: ci})
	}

	// SSE headers — flush IMMEDIATELY so browser gets response
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()

	// Get watchlist (fast, small table)
	var watchlist []TradingWatchlistItem
	db.Find(&watchlist)
	var symbols []string
	for _, w := range watchlist {
		if req.USOnly && isNonUSStock(w.Symbol) {
			continue
		}
		symbols = append(symbols, w.Symbol)
	}

	// Build jobs — skip cache check entirely when force=true
	type fetchJob struct {
		Symbol   string
		CacheIv  string
		YahooIv  string
		AlpacaIv string
	}
	var jobs []fetchJob
	var totalCached int

	if req.Force {
		// Force: refresh everything, no DB queries needed
		for _, ivp := range ivPairs {
			for _, sym := range symbols {
				jobs = append(jobs, fetchJob{Symbol: sym, CacheIv: ivp.cache, YahooIv: ivp.cache, AlpacaIv: ivp.raw})
			}
		}
	} else {
		for _, ivp := range ivPairs {
			var cacheEntries []OHLCVCache
			db.Select("symbol, updated_at").Where("interval = ?", ivp.cache).Find(&cacheEntries)
			cacheMap := make(map[string]time.Time, len(cacheEntries))
			for _, ce := range cacheEntries {
				cacheMap[ce.Symbol] = ce.UpdatedAt
			}
			for _, sym := range symbols {
				if updatedAt, ok := cacheMap[sym]; ok && time.Since(updatedAt) < 24*time.Hour {
					totalCached++
					continue
				}
				jobs = append(jobs, fetchJob{Symbol: sym, CacheIv: ivp.cache, YahooIv: ivp.cache, AlpacaIv: ivp.raw})
			}
		}
	}

	fetchTotal := len(jobs)

	// Send init IMMEDIATELY — this is what the user sees first
	initJSON, _ := json.Marshal(gin.H{
		"type": "init", "cached": totalCached, "fetch_total": fetchTotal, "total": len(symbols) * len(ivPairs),
	})
	fmt.Fprintf(c.Writer, "data: %s\n\n", initJSON)
	c.Writer.Flush()

	if fetchTotal == 0 {
		completeJSON, _ := json.Marshal(gin.H{
			"type": "complete", "fetched": 0, "cached": totalCached, "failed": 0,
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", completeJSON)
		c.Writer.Flush()
		return
	}

	var fetched, failed int64
	type prefetchProgress struct {
		Symbol string
		Source string
	}
	progressCh := make(chan prefetchProgress, fetchTotal+1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Prefetch] panic recovered: %v", r)
			}
			close(progressCh)
		}()

		// Yahoo-only parallel fetch (fast, no rate-limit, 20 concurrent)
		if len(jobs) > 0 {
			var wg sync.WaitGroup
			sem := make(chan struct{}, 20)
			for _, job := range jobs {
				wg.Add(1)
				go func(j fetchJob) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					period := backtestPeriodMap[j.YahooIv]
					if period == "" {
						period = "60d"
					}
					ohlcv, err := fetchOHLCVFromYahoo(j.Symbol, period, j.YahooIv)
					if err == nil && len(ohlcv) > 0 {
						saveOHLCVCache(j.Symbol, j.CacheIv, ohlcv)
						atomic.AddInt64(&fetched, 1)
						progressCh <- prefetchProgress{Symbol: j.Symbol, Source: "yahoo"}
					} else {
						atomic.AddInt64(&failed, 1)
						progressCh <- prefetchProgress{Symbol: j.Symbol, Source: "failed"}
					}
				}(job)
			}
			wg.Wait()
		}
	}()

	// SSE writer — stop if client disconnects
	clientGone := c.Request.Context().Done()
	var current int64
	for msg := range progressCh {
		select {
		case <-clientGone:
			return
		default:
		}
		current++
		progressJSON, _ := json.Marshal(gin.H{
			"type":    "progress",
			"current": current,
			"total":   fetchTotal,
			"symbol":  msg.Symbol,
			"source":  msg.Source,
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", progressJSON)
		c.Writer.Flush()
	}

	completeJSON, _ := json.Marshal(gin.H{
		"type":    "complete",
		"fetched": atomic.LoadInt64(&fetched),
		"cached":  totalCached,
		"failed":  atomic.LoadInt64(&failed),
	})
	fmt.Fprintf(c.Writer, "data: %s\n\n", completeJSON)
	c.Writer.Flush()
}

// startOHLCVCacheWorker runs a background goroutine that keeps the OHLCV cache fresh.
// Rate-limited to 1 request / 2 seconds (~1800/h, under Yahoo's ~2000/h limit).
func startOHLCVCacheWorker() {
	log.Println("[OHLCVCache] Background worker started")
	for {
		// Collect all symbols that need updating
		var stocks []Stock
		db.Select("symbol").Find(&stocks)
		if len(stocks) == 0 {
			time.Sleep(5 * time.Minute)
			continue
		}

		// Determine which intervals to refresh
		// Intraday (5m, 15m, 60m) only during market hours
		// Daily/weekly always
		marketOpen := isUSMarketOpen()
		intervals := []string{"1d", "1wk"}
		if marketOpen {
			intervals = append([]string{"5m", "15m", "60m"}, intervals...)
		}

		// Collect live session symbols + their intervals (refreshed by runLiveScan, skip here)
		liveSymbolIntervals := map[string]map[string]bool{} // symbol -> set of yahoo intervals
		liveSchedulerMu.Lock()
		for sid := range liveSchedulers {
			var session LiveTradingSession
			if db.First(&session, sid).Error == nil && session.IsActive {
				var syms []string
				json.Unmarshal([]byte(session.Symbols), &syms)
				yahooIv := session.Interval
				ivMap := map[string]string{"1h": "60m", "1D": "1d", "1W": "1wk"}
				if mapped, ok := ivMap[yahooIv]; ok {
					yahooIv = mapped
				}
				for _, s := range syms {
					if liveSymbolIntervals[s] == nil {
						liveSymbolIntervals[s] = map[string]bool{}
					}
					liveSymbolIntervals[s][yahooIv] = true
				}
			}
		}
		liveSchedulerMu.Unlock()

		// Build symbol list (no priority ordering needed — live symbols are skipped per-interval)
		symbolOrder := make([]string, 0, len(stocks))
		seen := map[string]bool{}
		for _, s := range stocks {
			if !seen[s.Symbol] {
				symbolOrder = append(symbolOrder, s.Symbol)
				seen[s.Symbol] = true
			}
		}

		updated := 0
		for _, iv := range intervals {
			maxAge := getOHLCVCacheMaxAge(iv)
			for _, symbol := range symbolOrder {
				// Skip symbols whose interval is handled by live session refresh
				if ivSet, ok := liveSymbolIntervals[symbol]; ok && ivSet[iv] {
					continue
				}

				// Check if cache is still fresh
				var cache OHLCVCache
				if db.Where("symbol = ? AND interval = ?", symbol, iv).First(&cache).Error == nil {
					if time.Since(cache.UpdatedAt) < maxAge {
						continue // still fresh
					}
				}

				// Rate limit: 1 request / 2 seconds
				time.Sleep(2 * time.Second)

				// Delta fetch
				deltaPeriod := getOHLCVDeltaPeriod(iv)
				freshBars, err := fetchOHLCVFromYahoo(symbol, deltaPeriod, iv)
				if err != nil {
					continue
				}

				if cache.DataJSON != "" {
					var cached []OHLCV
					if json.Unmarshal([]byte(cache.DataJSON), &cached) == nil && len(cached) > 0 {
						freshBars = mergeOHLCV(cached, freshBars)
					}
				}

				if len(freshBars) > 0 {
					saveOHLCVCache(symbol, iv, freshBars)
					updated++
				}
			}
		}

		if updated > 0 {
			log.Printf("[OHLCVCache] Cycle done: %d symbols updated", updated)
		}

		// Wait 5 minutes before next cycle
		time.Sleep(5 * time.Minute)
	}
}

// getOHLCVCacheMaxAge returns how long data for a given interval stays "fresh" before the worker refreshes it
func getOHLCVCacheMaxAge(interval string) time.Duration {
	switch interval {
	case "5m":
		return 10 * time.Minute
	case "15m":
		return 30 * time.Minute
	case "60m":
		return 2 * time.Hour
	case "1d":
		return 12 * time.Hour
	case "1wk":
		return 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

func runBacktestLabBatchHandler(c *gin.Context) {
	var req BacktestLabBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Ungültige Anfrage: " + err.Error()})
		return
	}

	validModes := map[string]bool{"defensive": true, "aggressive": true, "quant": true, "ditz": true, "trader": true}
	if !validModes[req.BaseMode] {
		c.JSON(400, gin.H{"error": "Unbekannter Base Mode: " + req.BaseMode})
		return
	}

	tslPercent := req.TSL // 0 = kein TSL

	// Get cutoff timestamp from time_range
	cutoffTime := int64(0)
	if req.TimeRange != "" && req.TimeRange != "all" {
		now := time.Now().Unix()
		year := int64(365 * 24 * 60 * 60)
		switch req.TimeRange {
		case "1y":
			cutoffTime = now - year
		case "2y":
			cutoffTime = now - 2*year
		case "3y":
			cutoffTime = now - 3*year
		case "5y":
			cutoffTime = now - 5*year
		case "10y":
			cutoffTime = now - 10*year
		}
	}

	// Load all stocks
	var stocks []Stock
	db.Find(&stocks)
	if len(stocks) == 0 {
		c.JSON(200, BacktestLabBatchResponse{})
		return
	}

	// Load performance data for filtering (based on mode)
	perfMap := loadPerformanceMapForMode(req.BaseMode)

	// Load config for base mode
	defConfig, aggConfig, quantConfig, ditzConfig, traderConfig := loadAllConfigs()

	// SSE streaming for progress
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Flush()

	sendProgress := func(current, total int, symbol, status string) {
		msg := fmt.Sprintf(`{"current":%d,"total":%d,"symbol":"%s","status":"%s"}`, current, total, symbol, status)
		fmt.Fprintf(c.Writer, "event: progress\ndata: %s\n\n", msg)
		c.Writer.Flush()
	}

	// Pre-filter stocks
	type stockCandidate struct {
		Symbol string
		Name   string
	}
	var candidates []stockCandidate
	filteredCount := 0

	for _, stock := range stocks {
		symbol := stock.Symbol
		if perf, ok := perfMap[symbol]; ok {
			if req.MinWinrate != nil && perf.WinRate < *req.MinWinrate {
				filteredCount++
				continue
			}
			if req.MaxWinrate != nil && perf.WinRate > *req.MaxWinrate {
				filteredCount++
				continue
			}
			if req.MinRR != nil && perf.RiskReward < *req.MinRR {
				filteredCount++
				continue
			}
			if req.MaxRR != nil && perf.RiskReward > *req.MaxRR {
				filteredCount++
				continue
			}
			if req.MinAvgReturn != nil && perf.AvgReturn < *req.MinAvgReturn {
				filteredCount++
				continue
			}
			if req.MaxAvgReturn != nil && perf.AvgReturn > *req.MaxAvgReturn {
				filteredCount++
				continue
			}
			if req.MinMarketCap != nil {
				minCapValue := *req.MinMarketCap * 1e9
				if float64(perf.MarketCap) < minCapValue {
					filteredCount++
					continue
				}
			}
		} else if req.MinWinrate != nil || req.MinRR != nil || req.MinAvgReturn != nil || req.MinMarketCap != nil {
			filteredCount++
			continue
		}
		candidates = append(candidates, stockCandidate{Symbol: symbol, Name: stock.Name})
	}

	totalCandidates := len(candidates)
	sendProgress(0, totalCandidates, "", "Starte Batch-Backtest...")

	var stockResults []BacktestLabBatchStockResult
	var skippedStocks []BacktestLabSkippedStock

	for i, cand := range candidates {
		symbol := cand.Symbol
		name := cand.Name

		sendProgress(i+1, totalCandidates, symbol, "Verarbeite...")

		// Fetch monthly OHLCV
		monthlyOHLCV, err := fetchHistoricalDataServer(symbol)
		if err != nil || len(monthlyOHLCV) < 50 {
			skippedStocks = append(skippedStocks, BacktestLabSkippedStock{
				Symbol: symbol, Name: name, Reason: "Keine ausreichenden Monthly-Daten",
			})
			continue
		}

		// Fetch weekly OHLCV from persistent cache
		weeklyOHLCV, err := getOHLCVCached(symbol, "1wk", 24*time.Hour)
		if err != nil {
			reason := "Weekly-Daten nicht verfügbar: " + err.Error()
			skippedStocks = append(skippedStocks, BacktestLabSkippedStock{
				Symbol: symbol, Name: name, Reason: reason,
			})
			continue
		}

		// Calculate BXtrender on both timeframes
		var monthlyResult, weeklyResult BXtrenderResult
		switch req.BaseMode {
		case "defensive":
			monthlyResult = calculateBXtrenderServer(monthlyOHLCV, false, defConfig, 0, 0)
			weeklyResult = calculateBXtrenderServer(weeklyOHLCV, false, defConfig, 0, 0)
		case "aggressive":
			monthlyResult = calculateBXtrenderServer(monthlyOHLCV, true, aggConfig, 0, 0)
			weeklyResult = calculateBXtrenderServer(weeklyOHLCV, true, aggConfig, 0, 0)
		case "quant":
			monthlyResult = calculateBXtrenderQuantServer(monthlyOHLCV, quantConfig, 0, 0)
			weeklyResult = calculateBXtrenderQuantServer(weeklyOHLCV, quantConfig, 0, 0)
		case "ditz":
			monthlyResult = calculateBXtrenderDitzServer(monthlyOHLCV, ditzConfig, 0, 0)
			weeklyResult = calculateBXtrenderDitzServer(weeklyOHLCV, ditzConfig, 0, 0)
		case "trader":
			monthlyResult = calculateBXtrenderTraderServer(monthlyOHLCV, traderConfig, 0, 0)
			weeklyResult = calculateBXtrenderTraderServer(weeklyOHLCV, traderConfig, 0, 0)
		}

		if monthlyResult.Signal == "NO_DATA" {
			skippedStocks = append(skippedStocks, BacktestLabSkippedStock{
				Symbol: symbol, Name: name, Reason: "BXtrender-Berechnung fehlgeschlagen (zu wenig Daten)",
			})
			continue
		}

		// Run backtest
		var trades []ArenaBacktestTrade
		if len(req.Rules) == 0 {
			trades, _ = convertServerTradesToArena(monthlyResult.Trades)
		} else {
			trades, _ = evaluateBacktestLabRules(
				monthlyOHLCV, weeklyOHLCV,
				monthlyResult, weeklyResult,
				req.BaseMode, req.Rules, tslPercent,
			)
		}

		// Filter trades by time range
		if cutoffTime > 0 {
			var filteredTrades []ArenaBacktestTrade
			for _, t := range trades {
				if t.EntryTime >= cutoffTime || t.IsOpen {
					filteredTrades = append(filteredTrades, t)
				}
			}
			trades = filteredTrades
		}

		// Only include stocks that had trades
		closedTrades := 0
		for _, t := range trades {
			if !t.IsOpen {
				closedTrades++
			}
		}
		if closedTrades == 0 {
			skippedStocks = append(skippedStocks, BacktestLabSkippedStock{
				Symbol: symbol, Name: name, Reason: "Keine Trades im gewählten Zeitraum/Regelset",
			})
			continue
		}

		metrics := calculateBacktestLabMetrics(trades)
		stockResults = append(stockResults, BacktestLabBatchStockResult{
			Symbol:  symbol,
			Name:    name,
			Metrics: metrics,
			Trades:  trades,
		})

	}

	// Calculate total aggregated metrics
	totalMetrics := aggregateBatchMetrics(stockResults)

	// Send final result as SSE event
	resultData := BacktestLabBatchResponse{
		TotalMetrics:   totalMetrics,
		StockResults:   stockResults,
		SkippedStocks:  skippedStocks,
		TotalStocks:    len(stocks),
		TestedStocks:   len(stockResults),
		FilteredStocks: filteredCount,
	}
	resultJSON, _ := json.Marshal(resultData)
	fmt.Fprintf(c.Writer, "event: result\ndata: %s\n\n", string(resultJSON))
	c.Writer.Flush()

	// Auto-save to history
	userID, _ := c.Get("userID")
	if uid, ok := userID.(uint); ok && len(stockResults) > 0 {
		rulesJSON, _ := json.Marshal(req.Rules)
		filtersJSON, _ := json.Marshal(map[string]interface{}{
			"min_winrate": req.MinWinrate, "max_winrate": req.MaxWinrate,
			"min_rr": req.MinRR, "max_rr": req.MaxRR,
			"min_avg_return": req.MinAvgReturn, "max_avg_return": req.MaxAvgReturn,
			"min_market_cap": req.MinMarketCap,
		})
		metricsJSON, _ := json.Marshal(totalMetrics)
		var stockSummaries []BacktestLabHistoryStockSummary
		for _, sr := range stockResults {
			stockSummaries = append(stockSummaries, BacktestLabHistoryStockSummary{
				Symbol: sr.Symbol, Name: sr.Name,
				WinRate: sr.Metrics.WinRate, TotalReturn: sr.Metrics.TotalReturn,
				AvgReturn: sr.Metrics.AvgReturn, RiskReward: sr.Metrics.RiskReward,
				TotalTrades: sr.Metrics.TotalTrades,
			})
		}
		stockSummaryJSON, _ := json.Marshal(stockSummaries)

		db.Create(&BacktestLabHistory{
			UserID:           uid,
			Name:             fmt.Sprintf("%s — %s", strings.ToUpper(req.BaseMode[:1])+req.BaseMode[1:], req.TimeRange),
			BaseMode:         req.BaseMode,
			RulesJSON:        string(rulesJSON),
			TSL:              req.TSL,
			TimeRange:        req.TimeRange,
			FiltersJSON:      string(filtersJSON),
			MetricsJSON:      string(metricsJSON),
			StockSummaryJSON: string(stockSummaryJSON),
			TestedStocks:     len(stockResults),
			SkippedCount:     len(skippedStocks),
			TotalStocks:      len(stocks),
			FilteredStocks:   filteredCount,
		})
	}
}

func getBacktestLabHistory(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid, _ := userID.(uint)
	isAdmin, _ := c.Get("isAdmin")
	admin, _ := isAdmin.(bool)

	var histories []BacktestLabHistory
	if admin {
		db.Order("created_at DESC").Limit(100).Find(&histories)
	} else {
		db.Where("user_id = ?", uid).Order("created_at DESC").Limit(50).Find(&histories)
	}

	type HistoryItem struct {
		ID             uint                             `json:"id"`
		Name           string                           `json:"name"`
		BaseMode       string                           `json:"base_mode"`
		Rules          []BacktestLabRule                `json:"rules"`
		TSL            float64                          `json:"tsl"`
		TimeRange      string                           `json:"time_range"`
		Metrics        ArenaBacktestMetrics             `json:"metrics"`
		StockSummary   []BacktestLabHistoryStockSummary `json:"stock_summary"`
		TestedStocks   int                              `json:"tested_stocks"`
		SkippedCount   int                              `json:"skipped_count"`
		TotalStocks    int                              `json:"total_stocks"`
		FilteredStocks int                              `json:"filtered_stocks"`
		CreatedAt      time.Time                        `json:"created_at"`
	}

	var items []HistoryItem
	for _, h := range histories {
		item := HistoryItem{
			ID: h.ID, Name: h.Name, BaseMode: h.BaseMode, TSL: h.TSL,
			TimeRange: h.TimeRange, TestedStocks: h.TestedStocks,
			SkippedCount: h.SkippedCount, TotalStocks: h.TotalStocks,
			FilteredStocks: h.FilteredStocks, CreatedAt: h.CreatedAt,
		}
		json.Unmarshal([]byte(h.RulesJSON), &item.Rules)
		json.Unmarshal([]byte(h.MetricsJSON), &item.Metrics)
		json.Unmarshal([]byte(h.StockSummaryJSON), &item.StockSummary)
		items = append(items, item)
	}

	c.JSON(200, items)
}

func deleteBacktestLabHistory(c *gin.Context) {
	userID, _ := c.Get("userID")
	uid, _ := userID.(uint)
	isAdmin, _ := c.Get("isAdmin")
	admin, _ := isAdmin.(bool)
	id := c.Param("id")

	var result *gorm.DB
	if admin {
		result = db.Where("id = ?", id).Delete(&BacktestLabHistory{})
	} else {
		result = db.Where("id = ? AND user_id = ?", id, uid).Delete(&BacktestLabHistory{})
	}
	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "Nicht gefunden"})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func loadPerformanceMapForMode(mode string) map[string]struct {
	WinRate    float64
	RiskReward float64
	AvgReturn  float64
	MarketCap  int64
} {
	result := make(map[string]struct {
		WinRate    float64
		RiskReward float64
		AvgReturn  float64
		MarketCap  int64
	})

	switch mode {
	case "defensive":
		var perfs []StockPerformance
		db.Find(&perfs)
		for _, p := range perfs {
			result[p.Symbol] = struct {
				WinRate    float64
				RiskReward float64
				AvgReturn  float64
				MarketCap  int64
			}{p.WinRate, p.RiskReward, p.AvgReturn, p.MarketCap}
		}
	case "aggressive":
		var perfs []AggressiveStockPerformance
		db.Find(&perfs)
		for _, p := range perfs {
			result[p.Symbol] = struct {
				WinRate    float64
				RiskReward float64
				AvgReturn  float64
				MarketCap  int64
			}{p.WinRate, p.RiskReward, p.AvgReturn, p.MarketCap}
		}
	case "quant":
		var perfs []QuantStockPerformance
		db.Find(&perfs)
		for _, p := range perfs {
			result[p.Symbol] = struct {
				WinRate    float64
				RiskReward float64
				AvgReturn  float64
				MarketCap  int64
			}{p.WinRate, p.RiskReward, p.AvgReturn, p.MarketCap}
		}
	case "ditz":
		var perfs []DitzStockPerformance
		db.Find(&perfs)
		for _, p := range perfs {
			result[p.Symbol] = struct {
				WinRate    float64
				RiskReward float64
				AvgReturn  float64
				MarketCap  int64
			}{p.WinRate, p.RiskReward, p.AvgReturn, p.MarketCap}
		}
	case "trader":
		var perfs []TraderStockPerformance
		db.Find(&perfs)
		for _, p := range perfs {
			result[p.Symbol] = struct {
				WinRate    float64
				RiskReward float64
				AvgReturn  float64
				MarketCap  int64
			}{p.WinRate, p.RiskReward, p.AvgReturn, p.MarketCap}
		}
	}
	return result
}

func loadAllConfigs() (BXtrenderConfig, BXtrenderConfig, BXtrenderQuantConfig, BXtrenderDitzConfig, BXtrenderTraderConfig) {
	var defConfig, aggConfig BXtrenderConfig
	db.Where("mode = ?", "defensive").First(&defConfig)
	db.Where("mode = ?", "aggressive").First(&aggConfig)
	var quantConfig BXtrenderQuantConfig
	db.First(&quantConfig)
	var ditzConfig BXtrenderDitzConfig
	db.First(&ditzConfig)
	var traderConfig BXtrenderTraderConfig
	db.First(&traderConfig)

	if defConfig.ID == 0 {
		defConfig = BXtrenderConfig{ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15}
	}
	if aggConfig.ID == 0 {
		aggConfig = BXtrenderConfig{ShortL1: 5, ShortL2: 20, ShortL3: 15, LongL1: 20, LongL2: 15}
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
	return defConfig, aggConfig, quantConfig, ditzConfig, traderConfig
}

func aggregateBatchMetrics(results []BacktestLabBatchStockResult) ArenaBacktestMetrics {
	totalWins, totalLosses := 0, 0
	totalReturn := 0.0
	totalWinReturn := 0.0
	totalLossReturn := 0.0
	maxDrawdown := 0.0

	for _, r := range results {
		totalWins += r.Metrics.Wins
		totalLosses += r.Metrics.Losses
		for _, t := range r.Trades {
			if t.IsOpen {
				continue
			}
			totalReturn += t.ReturnPct
			if t.ReturnPct > 0 {
				totalWinReturn += t.ReturnPct
			} else {
				totalLossReturn += math.Abs(t.ReturnPct)
			}
		}
		if r.Metrics.MaxDrawdown > maxDrawdown {
			maxDrawdown = r.Metrics.MaxDrawdown
		}
	}

	totalTrades := totalWins + totalLosses
	winRate := 0.0
	if totalTrades > 0 {
		winRate = float64(totalWins) / float64(totalTrades) * 100
	}
	riskReward := 0.0
	if totalLosses > 0 && totalLossReturn > 0 {
		avgWin := totalWinReturn / math.Max(float64(totalWins), 1)
		avgLoss := totalLossReturn / float64(totalLosses)
		riskReward = avgWin / avgLoss
	}
	avgReturn := 0.0
	if totalTrades > 0 {
		avgReturn = totalReturn / float64(totalTrades)
	}

	return ArenaBacktestMetrics{
		WinRate: winRate, RiskReward: riskReward, TotalReturn: totalReturn,
		AvgReturn: avgReturn, MaxDrawdown: maxDrawdown, NetProfit: totalReturn,
		TotalTrades: totalTrades, Wins: totalWins, Losses: totalLosses,
	}
}
