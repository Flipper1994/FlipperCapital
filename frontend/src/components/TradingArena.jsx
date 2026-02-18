import { useState, useEffect, useRef, useCallback, useMemo, useTransition } from 'react'
import ArenaChart from './ArenaChart'
import ArenaBacktestPanel from './ArenaBacktestPanel'
import ArenaIndicatorChart from './ArenaIndicatorChart'
import { useCurrency } from '../context/CurrencyContext'

// Skeleton building blocks
const Sk = ({ className = '' }) => <div className={`bg-dark-700 rounded animate-pulse ${className}`} />

function ArenaSkeleton({ loadingInfo }) {
  return (
    <div className="relative space-y-4">
      {/* Spinner overlay */}
      <div className="absolute inset-0 z-10 flex items-center justify-center pointer-events-none">
        <div className="bg-dark-900/80 backdrop-blur-sm rounded-2xl px-8 py-6 flex flex-col items-center gap-3 shadow-xl border border-dark-600">
          <svg className="animate-spin h-10 w-10 text-accent-500" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" fill="none"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
          {loadingInfo && (
            <>
              <span className="text-sm text-white font-medium">{loadingInfo.label}</span>
              <span className="text-xs text-gray-400">{loadingInfo.detail}</span>
              <div className="w-48 bg-dark-700 rounded-full h-1.5 overflow-hidden">
                <div className="h-full bg-accent-500 rounded-full transition-all duration-300" style={{ width: `${Math.max(loadingInfo.pct, 2)}%` }} />
              </div>
            </>
          )}
        </div>
      </div>
      {/* Chart skeleton */}
      <div className="bg-dark-800 rounded-lg border border-dark-600 p-4">
        <div className="flex items-center gap-3 mb-3">
          <Sk className="h-5 w-32" />
          <Sk className="h-4 w-20" />
          <div className="ml-auto"><Sk className="h-4 w-12" /></div>
        </div>
        <Sk className="h-[400px] w-full rounded-lg" />
      </div>
      {/* Backtest results skeleton */}
      <div className="bg-dark-800 rounded-lg border border-dark-600 p-4">
        <div className="flex items-center gap-3 mb-3">
          <Sk className="h-4 w-40" />
          <Sk className="h-4 w-24 ml-auto" />
        </div>
        <div className="grid grid-cols-4 gap-3 mb-4">
          {[...Array(8)].map((_, i) => (
            <div key={i} className="space-y-1.5">
              <Sk className="h-3 w-16" />
              <Sk className="h-5 w-20" />
            </div>
          ))}
        </div>
        <div className="space-y-2">
          {[...Array(5)].map((_, i) => <Sk key={i} className="h-8 w-full" />)}
        </div>
      </div>
      {/* Batch / Watchlist results skeleton */}
      <div className="bg-dark-800 rounded-lg border border-dark-600 p-4">
        <div className="flex items-center gap-3 mb-3">
          <Sk className="h-4 w-48" />
          <Sk className="h-6 w-24 ml-auto rounded-lg" />
        </div>
        <div className="grid grid-cols-3 gap-3">
          {[...Array(6)].map((_, i) => (
            <div key={i} className="bg-dark-700/50 rounded-lg p-3 space-y-2">
              <Sk className="h-4 w-24" />
              <Sk className="h-3 w-16" />
              <Sk className="h-5 w-20" />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function SidebarSkeleton() {
  return (
    <div className="p-2 space-y-1">
      {[...Array(20)].map((_, i) => (
        <div key={i} className="flex items-center gap-2 px-2 py-1.5">
          <Sk className="h-4 w-14" />
          <Sk className="h-3 w-24" />
        </div>
      ))}
    </div>
  )
}

const INTERVALS = ['5m', '15m', '1h', '2h', '4h', '1D', '1W']
const INTERVAL_MAP = {
  '5m': '5m', '15m': '15m', '1h': '60m',
  '2h': '2h', '4h': '4h', '1D': '1d', '1W': '1wk',
}
const TV_INTERVAL_MAP = {
  '5m': '5', '15m': '15', '1h': '60',
  '2h': '120', '4h': '240', '1D': 'D', '1W': 'W',
}

const STRATEGIES = [
  { value: 'regression_scalping', label: 'Regression Scalping', beta: true },
  { value: 'hybrid_ai_trend', label: 'NW Bollinger Bands' },
  { value: 'diamond_signals', label: 'Diamond Signals', beta: true },
  { value: 'smart_money_flow', label: 'Smart Money Flow', beta: true },
  { value: 'hann_trend', label: 'Hann Trend (DMH + SAR)' },
]

const STRATEGY_INFO = {
  regression_scalping: {
    title: 'Regression Scalping',
    desc: 'Schnelle Mean-Reversion-Strategie. Nutzt polynomiale lineare Regression mit Standardabweichungs-Bändern — kauft bei Berührung des unteren Bands, verkauft am oberen. Bestätigung über Kerzenmuster optional.',
    indicators: 'LinReg-Kanal, Standardabweichungs-Bänder',
    timeframes: '5m, 15m',
  },
  hybrid_ai_trend: {
    title: 'NW Bollinger Bands',
    desc: '4-Level Bollinger Bänder mit Nadaraya-Watson Gaussian-Kernel Glättung (Flux Charts). BUY wenn Close unter Band 1 kreuzt, SELL wenn Close über Band 1 kreuzt. Quelle: HLC3, Smoothing h=6, Lookback 499 Bars.',
    indicators: 'BB Level 1 (20/3σ), Level 2 (75/3σ), Level 3 (100/4σ), Level 4 (100/4.25σ), NW-Smoothing',
    timeframes: '5m, 15m, 1h, 4h',
    tips: '↑ Win Rate: R/R senken (1.5), SL Buffer erhöhen (2.5-3%), BB1 StDev erhöhen (3.5-4.0), HybridFilter aktivieren, NW Smoothing erhöhen (10-12)',
    legend: [
      { symbol: '▲ LONG', color: '#22c55e', desc: 'Entry — Close kreuzt unter unteres Band 1' },
      { symbol: '▼ SHORT', color: '#ef4444', desc: 'Entry — Close kreuzt über oberes Band 1' },
      { symbol: '▼ SELL', color: '#f59e0b', desc: 'Long Only: Short-Signal schließt offene Long-Position' },
      { symbol: '▼ TP', color: '#22c55e', desc: 'Take Profit erreicht (Risk/Reward × Risiko)' },
      { symbol: '▼ SL', color: '#ef4444', desc: 'Stop Loss ausgelöst (SL Buffer % unter Entry)' },
      { symbol: 'SIGNAL', color: '#eab308', desc: 'Gegenläufiges Signal schließt offene Position' },
    ],
  },
  diamond_signals: {
    title: 'Diamond Signals',
    desc: 'Multi-Confluence-Strategie. Erkennt Chartmuster (Symmetrische Formationen) und bestätigt Signale über RSI-Extremzonen. Nur Trades mit mindestens N Konfluenz-Faktoren werden ausgeführt.',
    indicators: 'Pattern Detection, RSI, Konfluenz-Score',
    timeframes: '4h, 1D',
  },
  smart_money_flow: {
    title: 'Smart Money Flow Cloud',
    desc: 'Volumenfluss-basierte Trendstrategie (BOSWaves). Erkennt Regime über adaptiven Geldfluss-Indikator. Einstieg erst nach Struktur-Bestätigung: Regime-Wechsel → Swing-Punkt → Pullback zur Baseline → Structure Break.',
    indicators: 'Money Flow Cloud, Adaptive Bänder, Flow-Histogramm',
    timeframes: '1h, 4h, 1D',
    tips: '↑ Win Rate: Band Expansion erhöhen (2.5-3.0), Trend Length erhöhen (50+). ↑ Frequenz: Band Tightness senken (0.5-0.7), Flow Window kürzer (15-20)',
    legend: [
      { symbol: '▲ LONG', color: '#22c55e', desc: 'Entry — Structure Break über Swing-High nach Pullback' },
      { symbol: '▼ SHORT', color: '#ef4444', desc: 'Entry — Structure Break unter Swing-Low nach Pullback' },
      { symbol: '▼ TP', color: '#22c55e', desc: 'Take Profit (Risk × R/R)' },
      { symbol: '▼ SL', color: '#ef4444', desc: 'Stop Loss (Pullback-Extrem)' },
    ],
  },
  hann_trend: {
    title: 'Hann Trend (DMH + SAR)',
    desc: 'Trend+Pullback-Strategie nach Ehlers (TASC 2021.12). DMH-Oszillator bestimmt Trend-Richtung, Parabolic SAR erkennt Pullbacks. Nur erster Pullback nach DMH-Nulllinien-Kreuz wird gehandelt, Bestätigung über Swing-High/Low-Bruch.',
    indicators: 'DMH-Histogramm, Parabolic SAR',
    timeframes: '1h, 4h, 1D',
    tips: '↑ Frequenz: DMH Length senken (15-20), SAR Increment erhöhen (0.04-0.05). ↑ Qualität: DMH Length erhöhen (40-50), Swing Lookback erhöhen (8-10)',
    legend: [
      { symbol: '▲ LONG', color: '#22c55e', desc: 'Entry — Close über Swing-High nach SAR-Pullback im Aufwärtstrend' },
      { symbol: '▼ SHORT', color: '#ef4444', desc: 'Entry — Close unter Swing-Low nach SAR-Pullback im Abwärtstrend' },
      { symbol: '▼ TP', color: '#22c55e', desc: 'Take Profit (Risk × R/R)' },
      { symbol: '▼ SL', color: '#ef4444', desc: 'Stop Loss (Pullback-Extrem ± Buffer)' },
      { symbol: '█ gelb', color: '#FFCC00', desc: 'DMH > 0 — bullischer Trend' },
      { symbol: '█ blau', color: '#0055FF', desc: 'DMH < 0 — bärischer Trend' },
    ],
  },
}

const STRATEGY_PARAMS = {
  regression_scalping: [
    { key: 'degree', label: 'Degree', default: 2, min: 1, max: 5, step: 1 },
    { key: 'length', label: 'LinReg Length', default: 100, min: 20, max: 300, step: 10 },
    { key: 'multiplier', label: 'LinReg Multiplier', default: 3.0, min: 0.5, max: 5.0, step: 0.1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 2.5, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'sl_lookback', label: 'SL Lookback', default: 30, min: 5, max: 100, step: 5 },
    { key: 'confirmation_required', label: 'Confirmation', default: 1, min: 0, max: 1, step: 1, isToggle: true },
  ],
  hybrid_ai_trend: [
    { key: 'bb1_period', label: 'BB1 Period', default: 20, min: 5, max: 50, step: 1 },
    { key: 'bb1_stdev', label: 'BB1 StDev', default: 3.0, min: 1.0, max: 6.0, step: 0.25 },
    { key: 'bb2_period', label: 'BB2 Period', default: 75, min: 20, max: 200, step: 5 },
    { key: 'bb2_stdev', label: 'BB2 StDev', default: 3.0, min: 1.0, max: 6.0, step: 0.25 },
    { key: 'bb3_period', label: 'BB3 Period', default: 100, min: 50, max: 300, step: 5 },
    { key: 'bb3_stdev', label: 'BB3 StDev', default: 4.0, min: 1.0, max: 6.0, step: 0.25 },
    { key: 'bb4_period', label: 'BB4 Period', default: 100, min: 50, max: 300, step: 5 },
    { key: 'bb4_stdev', label: 'BB4 StDev', default: 4.25, min: 1.0, max: 6.0, step: 0.25 },
    { key: 'nw_bandwidth', label: 'NW Smoothing', default: 6.0, min: 1.0, max: 15.0, step: 0.5 },
    { key: 'nw_lookback', label: 'NW Lookback', default: 499, min: 50, max: 999, step: 10 },
    { key: 'sl_buffer', label: 'SL Buffer %', default: 1.5, min: 0, max: 5.0, step: 0.1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 2.0, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'hybrid_filter', label: 'mit Hybrid AlgoAI?', default: 0, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'hybrid_long_thresh', label: 'Threshold Long', default: 75, min: 0, max: 100, step: 1 },
    { key: 'hybrid_short_thresh', label: 'Threshold Short', default: 25, min: 0, max: 100, step: 1 },
    { key: 'confirm_candle', label: 'Bestätigungskerze?', default: 0, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'min_band_dist', label: 'Min Band-Abstand %', default: 0, min: 0, max: 3.0, step: 0.1 },
  ],
  diamond_signals: [
    { key: 'pattern_length', label: 'Pattern Length', default: 20, min: 5, max: 50, step: 1 },
    { key: 'rsi_period', label: 'RSI Period', default: 14, min: 5, max: 30, step: 1 },
    { key: 'confluence_min', label: 'Confluence Min', default: 3, min: 1, max: 5, step: 1 },
    { key: 'rsi_overbought', label: 'RSI Overbought', default: 65, min: 50, max: 90, step: 5 },
    { key: 'rsi_oversold', label: 'RSI Oversold', default: 35, min: 10, max: 50, step: 5 },
    { key: 'cooldown', label: 'Cooldown', default: 5, min: 1, max: 20, step: 1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 2.0, min: 1.0, max: 5.0, step: 0.1 },
  ],
  smart_money_flow: [
    { key: 'trend_length', label: 'Trend Length', default: 34, min: 10, max: 100, step: 1 },
    { key: 'basis_smooth', label: 'Trend Smoothing', default: 3, min: 1, max: 10, step: 1 },
    { key: 'flow_window', label: 'Flow Window', default: 24, min: 5, max: 60, step: 1 },
    { key: 'flow_smooth', label: 'Flow Smoothing', default: 5, min: 1, max: 15, step: 1 },
    { key: 'flow_boost', label: 'Flow Boost', default: 1.2, min: 0.5, max: 3.0, step: 0.1 },
    { key: 'atr_length', label: 'ATR Length', default: 14, min: 5, max: 50, step: 1 },
    { key: 'band_tightness', label: 'Band Tightness', default: 0.9, min: 0.1, max: 2.0, step: 0.1 },
    { key: 'band_expansion', label: 'Band Expansion', default: 2.2, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'dot_cooldown', label: 'Retest Cooldown', default: 12, min: 0, max: 30, step: 1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 2.0, min: 1.0, max: 5.0, step: 0.1 },
  ],
  hann_trend: [
    { key: 'dmh_length', label: 'DMH Length', default: 30, min: 5, max: 80, step: 1 },
    { key: 'sar_start', label: 'SAR Start', default: 0.02, min: 0.005, max: 0.1, step: 0.005 },
    { key: 'sar_increment', label: 'SAR Increment', default: 0.03, min: 0.005, max: 0.1, step: 0.005 },
    { key: 'sar_max', label: 'SAR Max', default: 0.3, min: 0.1, max: 0.5, step: 0.01 },
    { key: 'swing_lookback', label: 'Swing Lookback', default: 5, min: 2, max: 20, step: 1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 2.0, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'sl_buffer', label: 'SL Buffer %', default: 0.3, min: 0, max: 3.0, step: 0.1 },
  ],
}

const STRATEGY_DEFAULT_INTERVAL = {
  regression_scalping: '5m',
  hybrid_ai_trend: '5m',
  diamond_signals: '4h',
  smart_money_flow: '4h',
  hann_trend: '1h',
}

function getDefaultParams(strategy) {
  const defs = STRATEGY_PARAMS[strategy] || []
  const obj = {}
  defs.forEach(p => { obj[p.key] = p.default })
  return obj
}

function WatchlistBatchPanel({ batchResults, strategy, interval, longOnly, onSelectStock, filterSymbols, timeRange, formatTimeRange, tradeAmount, tradesFrom }) {
  const [sortCol, setSortCol] = useState('entry_time')
  const [sortDir, setSortDir] = useState('desc')
  const [stocksVisible, setStocksVisible] = useState(60)
  const [tradesVisible, setTradesVisible] = useState(100)
  const [filterUpdating, setFilterUpdating] = useState(false)

  // Reset pagination when filters change
  useEffect(() => {
    setStocksVisible(60)
    setTradesVisible(100)
    setFilterUpdating(true)
    const t = setTimeout(() => setFilterUpdating(false), 0)
    return () => clearTimeout(t)
  }, [longOnly, filterSymbols])

  const strategyLabel = STRATEGIES.find(s => s.value === strategy)?.label || strategy

  const toggleSort = (col) => {
    if (sortCol === col) {
      setSortDir(d => d === 'asc' ? 'desc' : 'asc')
    } else {
      setSortCol(col)
      setSortDir('desc')
    }
  }

  const sortIndicator = (col) => sortCol === col ? (sortDir === 'asc' ? ' ▲' : ' ▼') : ''

  const tradesFromUnix = useMemo(() => {
    if (!tradesFrom) return 0
    return Math.floor(new Date(tradesFrom).getTime() / 1000)
  }, [tradesFrom])

  const filteredBatchTrades = useMemo(() => {
    if (!batchResults?.trades) return []
    let trades = longOnly ? batchResults.trades.filter(t => t.direction === 'LONG') : batchResults.trades
    if (filterSymbols) {
      const set = new Set(filterSymbols)
      trades = trades.filter(t => set.has(t.symbol))
    }
    if (tradesFromUnix > 0) {
      trades = trades.filter(t => t.entry_time >= tradesFromUnix)
    }
    return [...trades].sort((a, b) => {
      let va = a[sortCol], vb = b[sortCol]
      if (typeof va === 'string') va = va.toLowerCase()
      if (typeof vb === 'string') vb = vb.toLowerCase()
      if (va < vb) return sortDir === 'asc' ? -1 : 1
      if (va > vb) return sortDir === 'asc' ? 1 : -1
      return 0
    })
  }, [batchResults?.trades, sortCol, sortDir, longOnly, filterSymbols, tradesFromUnix])

  const batchMetrics = useMemo(() => {
    if (!longOnly && !filterSymbols && !tradesFromUnix) return batchResults.metrics

    // Filter trades for recalculation
    let trades = batchResults?.trades || []
    if (longOnly) trades = trades.filter(t => t.direction === 'LONG')
    if (filterSymbols) { const set = new Set(filterSymbols); trades = trades.filter(t => set.has(t.symbol)) }
    if (tradesFromUnix > 0) trades = trades.filter(t => t.entry_time >= tradesFromUnix)
    trades = trades.filter(t => !t.is_open)
    if (trades.length === 0) return batchResults.metrics

    let wins = 0, losses = 0, totalReturn = 0, totalWinReturn = 0, totalLossReturn = 0
    let equity = 100, peak = 100, maxDD = 0
    for (const t of trades) {
      totalReturn += t.return_pct
      if (t.return_pct >= 0) { wins++; totalWinReturn += t.return_pct }
      else { losses++; totalLossReturn += Math.abs(t.return_pct) }
      equity *= (1 + t.return_pct / 100)
      if (equity > peak) peak = equity
      const dd = (peak - equity) / peak * 100
      if (dd > maxDD) maxDD = dd
    }
    const total = wins + losses
    const winRate = total > 0 ? (wins / total) * 100 : 0
    const avgWin = wins > 0 ? totalWinReturn / wins : 0
    const avgLoss = losses > 0 ? totalLossReturn / losses : 0
    const riskReward = avgLoss > 0 ? avgWin / avgLoss : 0
    return {
      win_rate: winRate, risk_reward: riskReward, total_return: totalReturn,
      avg_return: total > 0 ? totalReturn / total : 0, max_drawdown: maxDD,
      net_profit: equity - 100, total_trades: total, wins, losses,
    }
  }, [batchResults, longOnly, filterSymbols, tradesFromUnix])

  const batchPerStock = useMemo(() => {
    const ps = batchResults.per_stock || {}

    // No filters active → use backend metrics directly
    if (!longOnly && !filterSymbols && !tradesFromUnix) return ps

    // Filters active but NOT Long Only and no time filter → just filter per_stock by symbol list
    if (!longOnly && filterSymbols && !tradesFromUnix) {
      const symbolSet = new Set(filterSymbols)
      const result = {}
      for (const [sym, metrics] of Object.entries(ps)) {
        if (symbolSet.has(sym)) result[sym] = metrics
      }
      return result
    }

    // Must recalculate from trades (longOnly, tradesFrom, or both)
    const symbolSet = filterSymbols ? new Set(filterSymbols) : null
    const perStock = {}
    for (const t of (batchResults?.trades || [])) {
      if (longOnly && t.direction !== 'LONG') continue
      if (t.is_open) continue
      if (symbolSet && !symbolSet.has(t.symbol)) continue
      if (tradesFromUnix > 0 && t.entry_time < tradesFromUnix) continue
      if (!perStock[t.symbol]) perStock[t.symbol] = []
      perStock[t.symbol].push(t)
    }
    const result = {}
    for (const [sym, trades] of Object.entries(perStock)) {
      let wins = 0, losses = 0, totalReturn = 0, totalWinReturn = 0, totalLossReturn = 0
      for (const t of trades) {
        if (t.return_pct >= 0) { wins++; totalWinReturn += t.return_pct }
        else { losses++; totalLossReturn += Math.abs(t.return_pct) }
        totalReturn += t.return_pct
      }
      const total = wins + losses
      const avgWin = wins > 0 ? totalWinReturn / wins : 0
      const avgLoss = losses > 0 ? totalLossReturn / losses : 0
      result[sym] = {
        win_rate: total > 0 ? (wins / total) * 100 : 0,
        risk_reward: avgLoss > 0 ? avgWin / avgLoss : 0,
        total_return: totalReturn, total_trades: total,
      }
    }
    return result
  }, [batchResults, longOnly, filterSymbols, tradesFromUnix])

  const formatTime = (ts) => {
    if (!ts) return '-'
    const d = new Date(ts * 1000)
    return d.toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: '2-digit' }) +
      ' ' + d.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' })
  }
  const formatReturn = (v) => v == null ? '-' : `${v >= 0 ? '+' : ''}${v.toFixed(1)}%`

  const reasonColor = {
    TP: 'bg-green-500/20 text-green-400',
    SL: 'bg-red-500/20 text-red-400',
    SIGNAL: 'bg-yellow-500/20 text-yellow-400',
    END: 'bg-gray-500/20 text-gray-400',
  }

  const m = batchMetrics
  const perStock = batchPerStock

  // Portfolio return on invested capital
  const portfolioStats = useMemo(() => {
    const closedTrades = filteredBatchTrades.filter(t => !t.is_open)
    const sorted = [...closedTrades].sort((a, b) => a.entry_time - b.entry_time)

    // Max parallel positions for required capital
    const events = []
    sorted.forEach(t => {
      events.push({ time: t.entry_time, type: 1 })
      if (t.exit_time) events.push({ time: t.exit_time, type: -1 })
    })
    events.sort((a, b) => a.time - b.time || a.type - b.type)
    let open = 0, maxParallel = 0
    events.forEach(e => { open += e.type; if (open > maxParallel) maxParallel = open })

    const posSize = tradeAmount > 0 ? tradeAmount : 500
    const requiredCapital = maxParallel * posSize
    const totalProfit = sorted.reduce((s, t) => s + posSize * (t.return_pct / 100), 0)
    const portfolioReturn = requiredCapital > 0 ? (totalProfit / requiredCapital) * 100 : 0

    return { portfolioReturn, maxParallel, requiredCapital, totalProfit, posSize }
  }, [filteredBatchTrades, tradeAmount])

  return (
    <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mt-4 relative">
      {filterUpdating && (
        <div className="absolute inset-0 bg-dark-800/70 z-10 flex items-center justify-center rounded-lg">
          <div className="flex items-center gap-2 text-sm text-gray-400">
            <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
            Aktualisiere...
          </div>
        </div>
      )}
      <div className="flex items-center justify-between mb-3">
        <h3 className="text-sm font-medium text-white">
          Watchlist Performance — {strategyLabel} ({interval})
          {filterSymbols && <span className="text-accent-400 ml-2 text-xs">Filter aktiv ({filterSymbols.length} Aktien)</span>}
        </h3>
        {timeRange && <span className="text-[10px] text-gray-500">{formatTimeRange(timeRange)}</span>}
      </div>

      {/* Aggregated Metrics */}
      <div className="grid grid-cols-4 gap-2 mb-4">
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Win Rate</div>
          <div className={`text-base font-bold ${m.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
            {m.win_rate?.toFixed(0)}%
          </div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">R/R</div>
          <div className={`text-base font-bold ${m.risk_reward >= 1 ? 'text-green-400' : 'text-red-400'}`}>
            {m.risk_reward?.toFixed(1)}
          </div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Total</div>
          <div className={`text-base font-bold ${m.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {m.total_return >= 0 ? '+' : ''}{m.total_return?.toFixed(0)}%
          </div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">&Oslash;/Trade</div>
          <div className={`text-base font-bold ${m.avg_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {formatReturn(m.avg_return)}
          </div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Trades</div>
          <div className="text-base font-bold text-white">{m.total_trades}</div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Wins</div>
          <div className="text-base font-bold text-green-400">{m.wins}</div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Losses</div>
          <div className="text-base font-bold text-red-400">{m.losses}</div>
        </div>
        <div className="bg-dark-700 rounded p-2 text-center">
          <div className="text-xs text-gray-500">Max DD</div>
          <div className="text-base font-bold text-red-400">-{m.max_drawdown?.toFixed(1)}%</div>
        </div>
        <div className="bg-indigo-500/10 border border-indigo-500/30 rounded p-2 text-center col-span-2">
          <div className="text-xs text-indigo-300">Portfolio-Rendite</div>
          <div className={`text-lg font-bold ${portfolioStats.portfolioReturn >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {portfolioStats.portfolioReturn >= 0 ? '+' : ''}{portfolioStats.portfolioReturn.toFixed(1)}%
          </div>
        </div>
        <div className="bg-indigo-500/10 border border-indigo-500/30 rounded p-2 text-center">
          <div className="text-xs text-indigo-300">Startkapital</div>
          <div className="text-lg font-bold text-white">
            {portfolioStats.requiredCapital.toLocaleString('de-DE')} €
          </div>
          <div className="text-[10px] text-gray-500">{portfolioStats.maxParallel} Pos. × {portfolioStats.posSize}€</div>
        </div>
        <div className="bg-indigo-500/10 border border-indigo-500/30 rounded p-2 text-center">
          <div className="text-xs text-indigo-300">Gewinn ({portfolioStats.posSize}€/Trade)</div>
          <div className={`text-lg font-bold ${portfolioStats.totalProfit >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {portfolioStats.totalProfit >= 0 ? '+' : ''}{portfolioStats.totalProfit.toLocaleString('de-DE', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} €
          </div>
        </div>
      </div>

      {/* Per-Stock Grid */}
      {Object.keys(perStock).length > 0 && (() => {
        const sorted = Object.entries(perStock).sort((a, b) => b[1].total_return - a[1].total_return)
        const totalStocks = sorted.length
        const visible = sorted.slice(0, stocksVisible)
        return (
          <div className="mb-4">
            <div className="text-xs text-gray-500 mb-2">Performance pro Aktie ({totalStocks}/{Object.keys(batchResults?.per_stock || {}).length})</div>
            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-2">
              {visible.map(([sym, sm]) => (
                <div
                  key={sym}
                  className="bg-dark-700 rounded p-2 cursor-pointer hover:bg-dark-600 transition-colors border border-transparent hover:border-accent-500/30"
                  onClick={() => onSelectStock && onSelectStock(sym, null, true)}
                >
                  <div className="text-xs font-medium text-white truncate">{sym}</div>
                  <div className="flex justify-between mt-1 text-[10px]">
                    <span className={sm.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}>{sm.win_rate?.toFixed(0)}%</span>
                    <span className={sm.total_return >= 0 ? 'text-green-400' : 'text-red-400'}>{sm.total_return >= 0 ? '+' : ''}{sm.total_return?.toFixed(0)}%</span>
                    <span className={`${(sm.risk_reward || 0) >= 1 ? 'text-green-400' : 'text-red-400'}`}>R/R {sm.risk_reward?.toFixed(1) || '-'}</span>
                    <span className="text-gray-500">{sm.total_trades}T</span>
                  </div>
                </div>
              ))}
            </div>
            {totalStocks > stocksVisible && (
              <button
                onClick={() => setStocksVisible(v => v + 60)}
                className="w-full mt-2 py-1.5 text-xs text-gray-400 hover:text-white bg-dark-700 hover:bg-dark-600 rounded transition-colors"
              >
                Mehr anzeigen ({stocksVisible}/{totalStocks})
              </button>
            )}
          </div>
        )
      })()}

      {/* Sortable Trades Table */}
      <div className="text-xs text-gray-500 mb-2">Alle Trades ({filteredBatchTrades.length})</div>
      <div className="max-h-96 overflow-auto">
        <table className="w-full text-xs">
          <thead className="sticky top-0 bg-dark-800">
            <tr className="text-left text-gray-500 border-b border-dark-600">
              <th className="pb-1 pr-2 cursor-pointer hover:text-white select-none" onClick={() => toggleSort('symbol')}>
                Symbol{sortIndicator('symbol')}
              </th>
              <th className="pb-1 pr-2 cursor-pointer hover:text-white select-none" onClick={() => toggleSort('direction')}>
                Dir{sortIndicator('direction')}
              </th>
              <th className="pb-1 pr-2 cursor-pointer hover:text-white select-none" onClick={() => toggleSort('entry_time')}>
                Entry{sortIndicator('entry_time')}
              </th>
              <th className="pb-1 pr-2 cursor-pointer hover:text-white select-none" onClick={() => toggleSort('exit_time')}>
                Exit{sortIndicator('exit_time')}
              </th>
              <th className="pb-1 pr-2 text-right cursor-pointer hover:text-white select-none" onClick={() => toggleSort('return_pct')}>
                Return{sortIndicator('return_pct')}
              </th>
              <th className="pb-1 text-right cursor-pointer hover:text-white select-none" onClick={() => toggleSort('exit_reason')}>
                Reason{sortIndicator('exit_reason')}
              </th>
            </tr>
          </thead>
          <tbody>
            {filteredBatchTrades.slice(0, tradesVisible).map((t, i) => (
              <tr key={i} className="border-b border-dark-700/50 last:border-0">
                <td className="py-1.5 pr-2 font-medium text-accent-400">{t.symbol}</td>
                <td className="py-1.5 pr-2">
                  <span className={`font-medium ${t.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>
                    {t.direction}
                  </span>
                </td>
                <td className="py-1.5 pr-2 text-gray-400">
                  <div>${t.entry_price?.toFixed(2)}</div>
                  <div className="text-gray-600 text-[10px]">{formatTime(t.entry_time)}</div>
                </td>
                <td className="py-1.5 pr-2 text-gray-400">
                  <div>${t.exit_price?.toFixed(2)}</div>
                  <div className="text-gray-600 text-[10px]">{formatTime(t.exit_time)}</div>
                </td>
                <td className={`py-1.5 pr-2 text-right font-medium ${t.return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {formatReturn(t.return_pct)}
                </td>
                <td className="py-1.5 text-right">
                  <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${reasonColor[t.exit_reason] || ''}`}>
                    {t.exit_reason}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {filteredBatchTrades.length > tradesVisible && (
        <button
          onClick={() => setTradesVisible(v => v + 100)}
          className="w-full mt-2 py-1.5 text-xs text-gray-400 hover:text-white bg-dark-700 hover:bg-dark-600 rounded transition-colors"
        >
          Mehr Trades anzeigen ({tradesVisible}/{filteredBatchTrades.length})
        </button>
      )}
    </div>
  )
}

function TradingArena({ isAdmin, token }) {
  const [selectedSymbol, setSelectedSymbol] = useState('')
  const [selectedStock, setSelectedStock] = useState(null)
  const [interval, setInterval] = useState('5m')
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState([])
  const [searching, setSearching] = useState(false)
  const searchTimeout = useRef(null)
  const { formatPrice, currency } = useCurrency()

  // Trading Arena state
  const [backtestResults, setBacktestResults] = useState(null)
  const [backtestLoading, setBacktestLoading] = useState(false)
  const [backtestError, setBacktestError] = useState(null)
  const [backtestStrategy, setBacktestStrategy] = useState('hybrid_ai_trend')
  const [tradingWatchlist, setTradingWatchlist] = useState([])
  const [addSymbol, setAddSymbol] = useState('')
  const [importProgress, setImportProgress] = useState(null)
  const [allStrategyResults, setAllStrategyResults] = useState(null)
  const [showParams, setShowParams] = useState(false)
  const [showTradingView, setShowTradingView] = useState(false)
  const [batchResults, setBatchResults] = useState(null)
  const [batchLoading, setBatchLoading] = useState(false)
  const [batchProgress, setBatchProgress] = useState(null)
  const [noDataSymbols, setNoDataSymbols] = useState([])
  const [longOnly, setLongOnly] = useState(true)
  const [usOnly, setUsOnly] = useState(true)
  const [hideFiltered, setHideFiltered] = useState(true)
  const [isFilterPending, startFilterTransition] = useTransition()
  const [showSimulation, setShowSimulation] = useState(false)
  const [simTradeAmount, setSimTradeAmount] = useState(500)
  const [appliedFilters, setAppliedFilters] = useState(null)
  const [filtersActive, setFiltersActive] = useState(false)
  const [tradesFrom, setTradesFrom] = useState('')
  const filterFormRef = useRef(null)
  const [simResults, setSimResults] = useState(null)
  const [strategyParams, setStrategyParams] = useState(() => getDefaultParams('hybrid_ai_trend'))
  const [sessionName, setSessionName] = useState('')
  const [showSessionNameDialog, setShowSessionNameDialog] = useState(false)

  // When clicking from batch results, override single backtest with batch data
  const batchOverrideRef = useRef(null)

  // Prefetch state
  const [prefetchStatus, setPrefetchStatus] = useState(null) // null | 'fetching' | 'done'
  const [prefetchProgress, setPrefetchProgress] = useState(null)
  const prefetchAbortRef = useRef(null)

  // Global loading state — true when prefetch OR batch is running
  const isGlobalLoading = prefetchStatus === 'fetching' || batchLoading

  const skipLoading = useCallback(() => {
    if (prefetchAbortRef.current) prefetchAbortRef.current.abort()
    if (batchAbortRef.current) batchAbortRef.current.abort()
    setPrefetchStatus('done')
    setPrefetchProgress(null)
    setBatchLoading(false)
    setBatchProgress(null)
  }, [])

  // Debounce ref for param changes
  const paramDebounce = useRef(null)
  const settingsSaveDebounce = useRef(null)
  const savedSettingsRef = useRef({})
  const batchAbortRef = useRef(null) // also used by skipLoading + runBatchBacktest

  // Load saved strategy settings (global or per-symbol)
  const loadSettings = useCallback(async (symbol = '') => {
    try {
      const url = symbol
        ? `/api/trading/strategy-settings?symbol=${encodeURIComponent(symbol)}`
        : '/api/trading/strategy-settings'
      const res = await fetch(url, {
        headers: token ? { 'Authorization': `Bearer ${token}` } : {},
      })
      if (res.ok) {
        const data = await res.json()
        savedSettingsRef.current = data
        return data
      }
    } catch { /* ignore */ }
    return null
  }, [token])

  // Load global settings on mount
  useEffect(() => {
    // Non-admins: force allowed strategy
    if (!isAdmin && backtestStrategy !== 'hybrid_ai_trend') {
      setBacktestStrategy('hybrid_ai_trend')
      setStrategyParams(getDefaultParams('hybrid_ai_trend'))
    }
    loadSettings().then(data => {
      if (!data) return
      const strat = (!isAdmin && backtestStrategy !== 'hybrid_ai_trend') ? 'hybrid_ai_trend' : backtestStrategy
      const saved = data[strat]
      if (saved?.params) {
        setStrategyParams(prev => ({ ...prev, ...saved.params }))
      }
      if (saved?.interval && INTERVALS.includes(saved.interval)) {
        setInterval(saved.interval)
      }
    })
  }, [token])

  const saveSettings = (strategy, params, iv, symbol = '') => {
    if (settingsSaveDebounce.current) clearTimeout(settingsSaveDebounce.current)
    settingsSaveDebounce.current = setTimeout(() => {
      fetch('/api/trading/strategy-settings', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ symbol, strategy, params, interval: iv }),
      }).catch(() => {})
    }, 300)
  }

  // Load trading watchlist and auto-select first stock
  const tradingWatchlistLoaded = useRef(false)
  const fetchTradingWatchlist = async () => {
    try {
      const res = await fetch('/api/trading/watchlist', {
        headers: token ? { 'Authorization': `Bearer ${token}` } : {},
      })
      if (res.ok) {
        const data = await res.json()
        const list = data || []
        setTradingWatchlist(list)
        if (!tradingWatchlistLoaded.current && list.length > 0 && !selectedSymbol) {
          tradingWatchlistLoaded.current = true
          selectStock(list[0].symbol, list[0])
        }
      }
    } catch { /* ignore */ }
  }
  useEffect(() => { fetchTradingWatchlist() }, [token])

  // (scheduler status removed — now in Live Trading page)

  // Prefetch + Auto-Batch beim Laden (admin only)
  useEffect(() => {
    if (!isAdmin || !token) return
    runPrefetchAndBatch(backtestStrategy, interval, strategyParams, usOnly)
    return () => { if (prefetchAbortRef.current) prefetchAbortRef.current.abort() }
  }, [token, isAdmin]) // eslint-disable-line react-hooks/exhaustive-deps

  // Search with debounce
  useEffect(() => {
    if (!searchQuery.trim()) {
      setSearchResults([])
      return
    }
    if (searchTimeout.current) clearTimeout(searchTimeout.current)
    searchTimeout.current = setTimeout(async () => {
      setSearching(true)
      try {
        const res = await fetch(`/api/search?q=${encodeURIComponent(searchQuery)}`, {
          headers: token ? { 'Authorization': `Bearer ${token}` } : {},
        })
        if (res.ok) {
          const data = await res.json()
          setSearchResults(data || [])
        }
      } catch { /* ignore */ }
      setSearching(false)
    }, 300)
    return () => { if (searchTimeout.current) clearTimeout(searchTimeout.current) }
  }, [searchQuery, token])

  const runBacktestNow = useCallback(async (symbol, strategy, iv, params) => {
    if (!symbol) return
    setBacktestLoading(true)
    setBacktestResults(null)
    setBacktestError(null)
    try {
      const res = await fetch('/api/trading/backtest', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({
          symbol,
          strategy,
          interval: INTERVAL_MAP[iv] || '4h',
          params,
        }),
      })
      if (res.ok) {
        const data = await res.json()
        // When clicked from batch results: use batch trades+metrics, single backtest only for chart
        if (batchOverrideRef.current) {
          data.singleTrades = data.trades // preserve for marker filtering
          data.trades = batchOverrideRef.current.trades
          data.metrics = batchOverrideRef.current.metrics
          batchOverrideRef.current = null
        }
        setBacktestResults(data)
        setAllStrategyResults(prev => ({
          ...prev,
          [strategy]: {
            metrics: data.metrics,
            trades: data.trades,
            interval: INTERVAL_MAP[iv] || '4h',
            updated_at: new Date().toISOString(),
          }
        }))
      } else {
        const err = await res.json().catch(() => ({}))
        setBacktestError(err.error || `Backtest fehlgeschlagen (HTTP ${res.status})`)
      }
    } catch (err) {
      console.error('[TradingArena] Backtest error:', err)
      setBacktestError('Netzwerkfehler: ' + err.message)
    }
    setBacktestLoading(false)
  }, [token])

  // Long Only filter: only show markers belonging to LONG trades
  const filteredTrades = useMemo(() => {
    if (!backtestResults?.trades) return null
    if (!longOnly) return backtestResults.trades
    return backtestResults.trades.filter(t => t.direction === 'LONG')
  }, [backtestResults?.trades, longOnly])

  const filteredMarkers = useMemo(() => {
    if (!backtestResults?.markers) return null
    if (!longOnly) return backtestResults.markers

    // Use single backtest trades for timestamp matching (not batch-overridden trades)
    const sourceTrades = (backtestResults?.singleTrades || backtestResults?.trades || [])
      .filter(t => t.direction === 'LONG')

    const validEntryTimes = new Set()
    const validExitTimes = new Set()
    sourceTrades.forEach(t => {
      validEntryTimes.add(t.entry_time)
      if (t.exit_time) validExitTimes.add(t.exit_time)
    })

    return backtestResults.markers.filter(m => {
      // LONG entry markers
      if (m.text === 'LONG' || m.text === '◆ LONG') return validEntryTimes.has(m.time)
      // SHORT at LONG exit time = position close → keep
      if (m.text === 'SHORT' || m.text === '◆ SHORT') return validExitTimes.has(m.time)
      // SL/TP/SIGNAL markers — keep if they match a LONG trade exit time
      if (['SL', 'TP', 'SIGNAL'].includes(m.text)) return validExitTimes.has(m.time)
      return true
    }).map(m => {
      // Transform SHORT to SELL for clarity
      if (m.text === 'SHORT' || m.text === '◆ SHORT') {
        return { ...m, text: 'SELL', color: '#f59e0b', shape: 'arrowDown', position: 'aboveBar' }
      }
      return m
    })
  }, [backtestResults?.markers, backtestResults?.singleTrades, backtestResults?.trades, longOnly])

  const filteredMetrics = useMemo(() => {
    if (!filteredTrades || !longOnly) return backtestResults?.metrics || null
    const trades = filteredTrades
    if (trades.length === 0) return null
    let wins = 0, losses = 0, totalReturn = 0, totalWinReturn = 0, totalLossReturn = 0
    let equity = 100, peak = 100, maxDD = 0
    for (const t of trades) {
      totalReturn += t.return_pct
      if (t.return_pct >= 0) { wins++; totalWinReturn += t.return_pct }
      else { losses++; totalLossReturn += Math.abs(t.return_pct) }
      equity *= (1 + t.return_pct / 100)
      if (equity > peak) peak = equity
      const dd = (peak - equity) / peak * 100
      if (dd > maxDD) maxDD = dd
    }
    const total = wins + losses
    const winRate = total > 0 ? (wins / total) * 100 : 0
    const avgWin = wins > 0 ? totalWinReturn / wins : 0
    const avgLoss = losses > 0 ? totalLossReturn / losses : 0
    const riskReward = avgLoss > 0 ? avgWin / avgLoss : 0
    return {
      win_rate: winRate, risk_reward: riskReward, total_return: totalReturn,
      avg_return: total > 0 ? totalReturn / total : 0, max_drawdown: maxDD,
      net_profit: equity - 100, total_trades: total, wins, losses,
    }
  }, [filteredTrades, longOnly, backtestResults?.metrics])

  // Shared filtered symbols for Watchlist Performance + Simulation
  // Heavy per-stock metrics — only recompute when batch data or longOnly changes
  const tradesFromUnixMain = useMemo(() => {
    if (!tradesFrom) return 0
    return Math.floor(new Date(tradesFrom).getTime() / 1000)
  }, [tradesFrom])

  const perStockMetrics = useMemo(() => {
    if (!batchResults?.per_stock) return null
    if (!longOnly && !tradesFromUnixMain) return batchResults.per_stock
    const ps = {}
    let allTrades = batchResults.trades
    if (longOnly) allTrades = allTrades.filter(t => t.direction === 'LONG')
    if (tradesFromUnixMain > 0) allTrades = allTrades.filter(t => t.entry_time >= tradesFromUnixMain)
    const bySymbol = {}
    allTrades.forEach(t => { (bySymbol[t.symbol] = bySymbol[t.symbol] || []).push(t) })
    Object.entries(bySymbol).forEach(([sym, trades]) => {
      const wins = trades.filter(t => t.return_pct >= 0).length
      const totalReturn = trades.reduce((s, t) => s + t.return_pct, 0)
      const winReturns = trades.filter(t => t.return_pct >= 0).map(t => t.return_pct)
      const lossReturns = trades.filter(t => t.return_pct < 0).map(t => Math.abs(t.return_pct))
      const avgWin = winReturns.length ? winReturns.reduce((a, b) => a + b, 0) / winReturns.length : 0
      const avgLoss = lossReturns.length ? lossReturns.reduce((a, b) => a + b, 0) / lossReturns.length : 1
      let eq = 100, peak = 100, maxDD = 0
      trades.forEach(t => { eq *= (1 + t.return_pct / 100); if (eq > peak) peak = eq; const dd = (peak - eq) / peak * 100; if (dd > maxDD) maxDD = dd })
      ps[sym] = { win_rate: trades.length ? (wins / trades.length) * 100 : 0, risk_reward: avgLoss > 0 ? avgWin / avgLoss : 0, total_return: totalReturn, avg_return: trades.length ? totalReturn / trades.length : 0, net_profit: eq - 100, total_trades: trades.length }
    })
    return ps
  }, [batchResults, longOnly, tradesFromUnixMain])

  // Light filter — runs fast when usOnly/appliedFilters change
  const perfFilteredSymbols = useMemo(() => {
    if (!perStockMetrics) return null
    const hasPerformanceFilters = filtersActive && appliedFilters
    if (!hasPerformanceFilters && !usOnly) return null // null = show all
    const f = hasPerformanceFilters ? appliedFilters : null
    const mc = batchResults?.market_caps || {}
    return Object.entries(perStockMetrics).filter(([sym, m]) => {
      if (usOnly && sym.includes('.')) return false
      if (f) {
        if (f.minWinRate !== '' && m.win_rate < Number(f.minWinRate)) return false
        if (f.minRR !== '' && m.risk_reward < Number(f.minRR)) return false
        if (f.minTotalReturn !== '' && m.total_return < Number(f.minTotalReturn)) return false
        if (f.minAvgReturn !== '' && m.avg_return < Number(f.minAvgReturn)) return false
        if (f.minNetProfit !== '' && m.net_profit < Number(f.minNetProfit)) return false
        if (f.minMarketCap !== '' && (mc[sym] || 0) < Number(f.minMarketCap) * 1e9) return false
      }
      return true
    }).map(([sym]) => sym)
  }, [perStockMetrics, filtersActive, usOnly, appliedFilters, batchResults?.market_caps])

  // O(1) lookup sets for watchlist rendering
  const perfFilteredSet = useMemo(() => perfFilteredSymbols ? new Set(perfFilteredSymbols) : null, [perfFilteredSymbols])
  const noDataSet = useMemo(() => new Set(noDataSymbols), [noDataSymbols])

  // Compute timeframe from chart data or batch trades
  const backtestTimeRange = useMemo(() => {
    if (backtestResults?.chart_data?.length) {
      const d = backtestResults.chart_data
      return { start: d[0].time, end: d[d.length - 1].time }
    }
    return null
  }, [backtestResults?.chart_data])

  const batchTimeRange = useMemo(() => {
    if (!batchResults?.trades?.length) return null
    let min = Infinity, max = -Infinity
    batchResults.trades.forEach(t => {
      if (t.entry_time < min) min = t.entry_time
      if (t.exit_time > max) max = t.exit_time
      if (t.entry_time > max) max = t.entry_time
    })
    return min < Infinity ? { start: min, end: max } : null
  }, [batchResults?.trades])

  const formatTimeRange = (range) => {
    if (!range) return ''
    const fmt = (ts) => new Date(ts * 1000).toLocaleString('de-DE', { day: '2-digit', month: '2-digit', year: '2-digit', hour: '2-digit', minute: '2-digit' })
    return `${fmt(range.start)} — ${fmt(range.end)}`
  }

  // Auto-recalculate simulation when filters change (if already computed)
  useEffect(() => {
    if (!simResults || !batchResults?.trades) return
    const symbolSet = perfFilteredSymbols ? new Set(perfFilteredSymbols) : null
    let trades = batchResults.trades
    if (symbolSet) trades = trades.filter(t => symbolSet.has(t.symbol))
    if (longOnly) trades = trades.filter(t => t.direction === 'LONG')
    if (tradesFrom) {
      const cutoff = Math.floor(new Date(tradesFrom).getTime() / 1000)
      if (cutoff > 0) trades = trades.filter(t => t.entry_time >= cutoff)
    }
    const amt = simTradeAmount || 500
    const simTrades = trades.map(t => ({
      ...t, invested: amt, profitEUR: amt * (t.return_pct / 100),
    })).sort((a, b) => a.entry_time - b.entry_time)
    const events = []
    simTrades.forEach(t => { events.push({ time: t.entry_time, type: 1 }); events.push({ time: t.exit_time, type: -1 }) })
    events.sort((a, b) => a.time - b.time || a.type - b.type)
    let open = 0, maxParallel = 0
    events.forEach(e => { open += e.type; if (open > maxParallel) maxParallel = open })
    const totalProfit = simTrades.reduce((s, t) => s + t.profitEUR, 0)
    const wins = simTrades.filter(t => t.return_pct >= 0).length
    const openCount = simTrades.filter(t => t.is_open).length
    const requiredCapital = maxParallel * amt
    const uniqueSymbols = new Set(simTrades.map(t => t.symbol))
    setSimResults({
      trades: simTrades, filteredCount: uniqueSymbols.size,
      totalCount: Object.keys(batchResults.per_stock || {}).length,
      totalTrades: simTrades.length, totalProfit,
      winRate: simTrades.length ? (wins / simTrades.length) * 100 : 0,
      wins, losses: simTrades.length - wins, openCount, maxParallel, requiredCapital,
      roi: requiredCapital > 0 ? (totalProfit / requiredCapital) * 100 : 0,
    })
  }, [perfFilteredSymbols, filtersActive, longOnly, simTradeAmount, tradesFrom]) // eslint-disable-line react-hooks/exhaustive-deps

  const runBatchBacktest = useCallback(async (strategy, iv, params, usOnlyFlag = true) => {
    // Cancel previous batch request if still running
    if (batchAbortRef.current) batchAbortRef.current.abort()
    const controller = new AbortController()
    batchAbortRef.current = controller

    setBatchLoading(true)
    setBatchProgress(null)
    setBatchResults(null) // Reset old results → shows skeleton with spinner
    try {
      const res = await fetch('/api/trading/backtest-watchlist', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({
          strategy,
          interval: INTERVAL_MAP[iv] || '4h',
          params,
          us_only: usOnlyFlag,
        }),
        signal: controller.signal,
      })
      if (!res.ok || !res.body) throw new Error(`HTTP ${res.status}`)
      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      let latestProgress = null
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop()
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const msg = JSON.parse(line.slice(6))
              if (msg.type === 'prefetch') {
                latestProgress = { current: 0, total: msg.total, symbol: '', prefetching: true, uncached: msg.uncached }
              } else if (msg.type === 'prefetch_progress') {
                latestProgress = { current: 0, total: latestProgress?.total || msg.total, symbol: '', prefetching: true, prefetchCurrent: msg.current, prefetchTotal: msg.total }
              } else if (msg.type === 'progress') {
                latestProgress = { current: msg.current, total: msg.total, symbol: msg.symbol }
              } else if (msg.type === 'result') {
                setBatchResults(msg.data)
                setNoDataSymbols(msg.data.skipped_symbols || [])
              }
            } catch { /* ignore parse error */ }
          }
        }
        // Update once per network chunk
        if (latestProgress) setBatchProgress({ ...latestProgress })
      }
    } catch (err) {
      if (err.name !== 'AbortError') {
        console.error('[TradingArena] Batch backtest error:', err)
      }
    }
    setBatchProgress(null)
    setBatchLoading(false)
    batchAbortRef.current = null
  }, [token])

  // Prefetch OHLCV data for interval, then run batch backtest
  const runPrefetchAndBatch = useCallback(async (strategy, iv, params, usOnlyFlag = true) => {
    if (isAdmin && token) {
      // Cancel previous prefetch AND batch (may still be running from different strategy)
      if (prefetchAbortRef.current) prefetchAbortRef.current.abort()
      if (batchAbortRef.current) batchAbortRef.current.abort()
      const controller = new AbortController()
      prefetchAbortRef.current = controller

      setPrefetchStatus('fetching')
      setPrefetchProgress(null)
      try {
        const res = await fetch('/api/trading/arena/prefetch', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
          body: JSON.stringify({ interval: INTERVAL_MAP[iv] || '4h', us_only: usOnlyFlag }),
          signal: controller.signal,
        })
        if (!res.ok || !res.body) throw new Error('prefetch failed')
        const reader = res.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ''
        let latestProgress = null
        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          buffer += decoder.decode(value, { stream: true })
          const lines = buffer.split('\n')
          buffer = lines.pop()
          for (const line of lines) {
            if (line.startsWith('data: ')) {
              try {
                const msg = JSON.parse(line.slice(6))
                if (controller.signal.aborted) return
                if (msg.type === 'init') {
                  latestProgress = { current: 0, total: msg.fetch_total, symbol: '', source: 'cache', cached: msg.cached, fetchTotal: msg.fetch_total }
                } else if (msg.type === 'progress') {
                  latestProgress = { current: msg.current, total: msg.total, symbol: msg.symbol, source: msg.source }
                } else if (msg.type === 'complete') {
                  setPrefetchStatus('done')
                }
              } catch { /* ignore parse error */ }
            }
          }
          if (latestProgress) setPrefetchProgress({ ...latestProgress })
        }
      } catch {
        // Prefetch failed or aborted
      }
      if (controller.signal.aborted) return
      setPrefetchStatus('done')
    }
    runBatchBacktest(strategy, iv, params, usOnlyFlag)
  }, [token, isAdmin, runBatchBacktest])

  // Auto-backtest on strategy change
  const handleStrategyChange = (newStrategy) => {
    if (!isAdmin && newStrategy !== 'hybrid_ai_trend') return
    setBacktestStrategy(newStrategy)
    // Load saved params for this strategy, or use defaults
    const saved = savedSettingsRef.current[newStrategy]
    const newParams = saved?.params
      ? { ...getDefaultParams(newStrategy), ...saved.params }
      : getDefaultParams(newStrategy)
    setStrategyParams(newParams)
    // Set recommended interval for strategy (saved interval takes priority)
    const newIv = saved?.interval || STRATEGY_DEFAULT_INTERVAL[newStrategy] || '4h'
    setInterval(newIv)
    if (selectedSymbol) {
      runBacktestNow(selectedSymbol, newStrategy, newIv, newParams)
    }
    // Kill any running prefetch (e.g. initial pageload prefetch with old interval)
    if (prefetchAbortRef.current) prefetchAbortRef.current.abort()
    setPrefetchStatus('done')
    // Clear stale batch results immediately (may be from previous strategy/interval)
    setBatchResults(null)
    // Batch directly — no prefetch needed (batch handler fetches missing data itself)
    runBatchBacktest(newStrategy, newIv, newParams, usOnly)
  }

  // Auto-backtest on interval change
  const handleIntervalChange = (newIv) => {
    setInterval(newIv)
    saveSettings(backtestStrategy, strategyParams, newIv, selectedSymbol)
    if (selectedSymbol) {
      runBacktestNow(selectedSymbol, backtestStrategy, newIv, strategyParams)
    }
    // Kill any running prefetch
    if (prefetchAbortRef.current) prefetchAbortRef.current.abort()
    setPrefetchStatus('done')
    // Clear stale batch results immediately
    setBatchResults(null)
    // Batch directly — no prefetch needed (batch handler fetches missing data itself)
    runBatchBacktest(backtestStrategy, newIv, strategyParams, usOnly)
  }

  // Debounced auto-backtest on param change
  const handleParamChange = (key, value) => {
    const updated = { ...strategyParams, [key]: value }
    setStrategyParams(updated)
    saveSettings(backtestStrategy, updated, interval, selectedSymbol)
    if (paramDebounce.current) clearTimeout(paramDebounce.current)
    paramDebounce.current = setTimeout(() => {
      if (selectedSymbol) {
        runBacktestNow(selectedSymbol, backtestStrategy, interval, updated)
      }
    }, 500)
  }

  const resetParams = () => {
    const defs = getDefaultParams(backtestStrategy)
    setStrategyParams(defs)
    saveSettings(backtestStrategy, defs, interval, selectedSymbol)
    if (selectedSymbol) {
      runBacktestNow(selectedSymbol, backtestStrategy, interval, defs)
    }
  }

  const selectStock = async (symbol, stock = null, fromBatch = false) => {
    setSelectedSymbol(symbol)
    setSelectedStock(stock)
    setSearchQuery('')
    setSearchResults([])
    setBacktestResults(null)
    setAllStrategyResults(null)
    setBacktestLoading(true) // Sofort Loading zeigen, nicht erst nach den awaits

    let useParams = strategyParams
    let useInterval = interval

    // When clicking from batch: use batch trades+metrics (guaranteed consistent with cards)
    if (fromBatch && batchResults) {
      const symTrades = (batchResults.trades || []).filter(t => t.symbol === symbol)
      const symMetrics = batchResults.per_stock?.[symbol]
      if (symMetrics && symTrades.length > 0) {
        batchOverrideRef.current = { trades: symTrades, metrics: symMetrics }
      }
    } else {
      batchOverrideRef.current = null
    }

    // Load per-symbol settings only when NOT clicking from batch results
    // (batch results were calculated with current params/interval — keep them consistent)
    if (!fromBatch) {
      const symbolSettings = await loadSettings(symbol)
      if (symbolSettings) {
        const saved = symbolSettings[backtestStrategy]
        if (saved?.params) {
          useParams = { ...getDefaultParams(backtestStrategy), ...saved.params }
          setStrategyParams(useParams)
        }
        if (saved?.interval && INTERVALS.includes(saved.interval)) {
          useInterval = saved.interval
          setInterval(useInterval)
        }
      }
    }

    // Load stored results for ALL strategies (parallel zum Backtest)
    fetch(`/api/trading/backtest-results/${encodeURIComponent(symbol)}`, {
      headers: token ? { 'Authorization': `Bearer ${token}` } : {},
    }).then(res => res.ok ? res.json() : null).then(data => {
      if (data && Object.keys(data).length > 0) setAllStrategyResults(data)
    }).catch(() => {})

    // Auto-trigger backtest — loads chart data + indicators. Batch override merges in runBacktestNow.
    runBacktestNow(symbol, backtestStrategy, useInterval, useParams)
  }

  // Start Live Trading — opens name dialog first
  const handleStartLiveTrading = () => {
    const symbols = perfFilteredSymbols || Object.keys(batchResults?.per_stock || {})
    if (symbols.length === 0) {
      alert('Keine Symbole ausgewählt. Bitte erst Watchlist Backtest ausführen.')
      return
    }
    setShowSessionNameDialog(true)
  }

  const confirmStartLiveTrading = async () => {
    const symbols = perfFilteredSymbols || Object.keys(batchResults?.per_stock || {})
    try {
      const res = await fetch('/api/trading/live/config', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({
          strategy: backtestStrategy,
          interval,
          params: strategyParams,
          symbols,
          long_only: longOnly,
          us_only: usOnly,
          trade_amount: simTradeAmount,
          filters: appliedFilters || {},
          filters_active: filtersActive,
          currency: currency || 'EUR',
        }),
      })
      if (res.ok) {
        const startRes = await fetch('/api/trading/live/start', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
          },
          body: JSON.stringify({ name: sessionName.trim() || undefined }),
        })
        if (startRes.ok) {
          const data = await startRes.json()
          const sid = data.session?.ID || data.session?.id
          window.location.href = sid ? `/live-trading/${sid}` : '/live-trading'
        } else {
          const err = await startRes.json()
          alert(err.error || 'Session konnte nicht erstellt werden')
        }
      }
    } catch (err) {
      console.error('Failed to create live session:', err)
    } finally {
      setShowSessionNameDialog(false)
      setSessionName('')
    }
  }

  // Add to trading watchlist
  const addToTrading = async () => {
    if (!addSymbol.trim()) return
    try {
      const res = await fetch('/api/trading/watchlist', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
        },
        body: JSON.stringify({ symbol: addSymbol.trim().toUpperCase() }),
      })
      if (res.ok) {
        const item = await res.json()
        setTradingWatchlist(prev => [...prev, item].sort((a, b) => a.symbol.localeCompare(b.symbol)))
        setAddSymbol('')
      }
    } catch { /* ignore */ }
  }

  // Remove from trading watchlist
  const removeFromTrading = async (id) => {
    try {
      const res = await fetch(`/api/trading/watchlist/${id}`, {
        method: 'DELETE',
        headers: token ? { 'Authorization': `Bearer ${token}` } : {},
      })
      if (res.ok) {
        setTradingWatchlist(prev => prev.filter(i => i.id !== id))
      }
    } catch { /* ignore */ }
  }

  const handleImport = async () => {
    if (importProgress) return
    setImportProgress({ current: 0, total: 0, symbol: '', status: 'starting' })
    try {
      const res = await fetch('/api/trading/watchlist/import', {
        method: 'POST',
        headers: token ? { 'Authorization': `Bearer ${token}` } : {},
      })
      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop()
        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const data = JSON.parse(line.slice(6))
              if (data.done) {
                setImportProgress({ ...data, current: data.added + data.skipped + data.failed, total: data.added + data.skipped + data.failed })
                fetchTradingWatchlist()
                setTimeout(() => setImportProgress(null), 3000)
              } else {
                setImportProgress(data)
              }
            } catch { /* ignore parse error */ }
          }
        }
      }
    } catch {
      setImportProgress(null)
    }
  }

  const currentParams = STRATEGY_PARAMS[backtestStrategy] || []

  // Compute loading label and percentage for overlay
  const sourceLabel = (src) => {
    if (src === 'alpaca') return 'Alpaca'
    if (src === 'yahoo') return 'Yahoo'
    if (src === 'cache') return 'Cache'
    if (src === 'failed') return 'Fehler'
    return ''
  }
  const loadingInfo = useMemo(() => {
    if (prefetchStatus === 'fetching') {
      const p = prefetchProgress
      if (!p || p.total === 0) return { label: 'Aktualisiere Kursdaten', pct: 0, detail: 'Prüfe Cache...' }
      if (p.current === 0 && p.cached > 0) {
        return { label: 'Aktualisiere Kursdaten', pct: 0, detail: `${p.cached} im Cache, lade ${p.fetchTotal} verbleibende...` }
      }
      const pct = Math.round((p.current / p.total) * 100)
      const src = p.source ? ` via ${sourceLabel(p.source)}` : ''
      const detail = `${p.symbol || '...'}${src} (${p.current}/${p.total})`
      return { label: 'Aktualisiere Kursdaten', pct, detail }
    }
    if (batchLoading) {
      const p = batchProgress
      if (!p) return { label: 'Starte Watchlist Backtest', pct: 0, detail: 'Verbinde...' }
      if (p.prefetching) {
        if (p.prefetchCurrent != null) {
          const pct = Math.round((p.prefetchCurrent / p.prefetchTotal) * 100)
          return { label: 'Lade Kursdaten', pct, detail: `${p.prefetchCurrent}/${p.prefetchTotal} Aktien via Alpaca` }
        }
        return { label: 'Lade Kursdaten', pct: 0, detail: `${p.uncached || '?'} Aktien nicht im Cache` }
      }
      const pct = Math.round((p.current / p.total) * 100)
      return { label: 'Watchlist Backtest läuft', pct, detail: `Teste ${p.symbol || '...'} (${p.current}/${p.total})` }
    }
    return null
  }, [prefetchStatus, prefetchProgress, batchLoading, batchProgress])

  return (
    <div className="flex-1 flex flex-col md:flex-row gap-0 min-h-0 overflow-hidden relative">
      {/* Non-blocking loading banner */}
      {isGlobalLoading && loadingInfo && (
        <div className="absolute top-0 left-0 right-0 z-40 bg-dark-800/95 border-b border-dark-600 px-4 py-2">
          <div className="flex items-center gap-3 max-w-3xl mx-auto">
            <div className="w-4 h-4 shrink-0 border-2 border-accent-500 border-t-transparent rounded-full animate-spin" />
            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between mb-1">
                <span className="text-sm text-white font-medium truncate">{loadingInfo.label}</span>
                <span className="text-xs text-gray-400 ml-2 shrink-0">{loadingInfo.detail}</span>
              </div>
              <div className="w-full bg-dark-700 rounded-full h-1.5 overflow-hidden">
                <div
                  className={`h-full bg-accent-500 rounded-full transition-all duration-300 ${loadingInfo.pct === 0 ? 'animate-pulse' : ''}`}
                  style={{ width: `${Math.max(loadingInfo.pct, 2)}%` }}
                />
              </div>
            </div>
            <button
              onClick={skipLoading}
              className="text-xs text-gray-500 hover:text-white px-2 py-1 rounded hover:bg-dark-600 transition-colors shrink-0"
            >
              Überspringen
            </button>
          </div>
        </div>
      )}

      {/* Main content area */}
      <div className={`flex-1 flex flex-col p-4 min-w-0 overflow-y-auto ${isGlobalLoading ? 'pt-14' : ''}`}>
        {/* Filter transition indicator */}
        {isFilterPending && (
          <div className="mb-3 flex items-center gap-2 px-3 py-2 bg-dark-800 rounded-lg border border-accent-500/30">
            <svg className="animate-spin h-4 w-4 text-accent-400 shrink-0" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
            <span className="text-sm text-gray-300">Filter wird angewendet…</span>
          </div>
        )}
        {/* Top bar */}
        <div className="flex flex-wrap items-center gap-3 mb-4">
          {/* Search */}
          <div className="relative flex-1 min-w-[200px] max-w-md">
            <input
              type="text"
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
              placeholder="Aktie suchen..."
              className="w-full bg-dark-700 border border-dark-600 rounded-lg px-4 py-2 text-white text-sm placeholder-gray-500 focus:outline-none focus:border-accent-500"
            />
            {(searchResults.length > 0 || searching) && (
              <div className="absolute top-full left-0 right-0 mt-1 bg-dark-700 border border-dark-600 rounded-lg shadow-xl z-50 max-h-64 overflow-y-auto">
                {searching && <div className="px-4 py-2 text-gray-400 text-sm">Suche...</div>}
                {searchResults.map(r => (
                  <button
                    key={r.symbol}
                    onClick={() => selectStock(r.symbol, r)}
                    className="w-full text-left px-4 py-2 hover:bg-dark-600 transition-colors flex items-center gap-3"
                  >
                    <span className="text-white font-medium text-sm">{r.symbol}</span>
                    <span className="text-gray-400 text-xs truncate">{r.name}</span>
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Strategy selector */}
          <select
            value={backtestStrategy}
            onChange={e => handleStrategyChange(e.target.value)}
            className="bg-dark-700 border border-dark-600 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-accent-500"
          >
            {STRATEGIES.map(s => {
              const locked = !isAdmin && s.value !== 'hybrid_ai_trend'
              return (
                <option key={s.value} value={s.value} disabled={locked}>
                  {s.label}{s.beta ? ' [BETA]' : ''}{locked ? ' (gesperrt)' : ''}
                </option>
              )
            })}
          </select>

          {/* Params toggle */}
          <button
            onClick={() => setShowParams(!showParams)}
            className={`px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
              showParams
                ? 'bg-accent-500/20 text-accent-400 border border-accent-500/30'
                : 'bg-dark-700 text-gray-400 hover:text-white hover:bg-dark-600 border border-dark-600'
            }`}
            title="Parameter anzeigen"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
            </svg>
          </button>

          {/* Backtest button */}
          <button
            onClick={() => {
              if (selectedSymbol) runBacktestNow(selectedSymbol, backtestStrategy, interval, strategyParams)
              runBatchBacktest(backtestStrategy, interval, strategyParams, usOnly)
            }}
            disabled={!selectedSymbol || backtestLoading}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              selectedSymbol && !backtestLoading
                ? 'bg-accent-500 text-white hover:bg-accent-600'
                : 'bg-dark-700 text-gray-500 cursor-not-allowed'
            }`}
          >
            {backtestLoading ? 'Läuft...' : 'Backtest'}
          </button>

          {/* Long Only toggle */}
          <label className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-dark-700 border border-dark-600 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={longOnly}
              onChange={e => { const v = e.target.checked; startFilterTransition(() => setLongOnly(v)) }}
              className="accent-accent-500 w-3.5 h-3.5"
            />
            <span className={`text-xs font-medium ${longOnly ? 'text-accent-400' : 'text-gray-400'}`}>Long Only</span>
          </label>

          {/* US Only toggle */}
          <label className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-dark-700 border border-dark-600 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={usOnly}
              onChange={e => { const v = e.target.checked; startFilterTransition(() => setUsOnly(v)) }}
              className="accent-accent-500 w-3.5 h-3.5"
            />
            <span className={`text-xs font-medium ${usOnly ? 'text-accent-400' : 'text-gray-400'}`}>US Only</span>
          </label>

          {/* Trades ab Datum */}
          <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-dark-700 border border-dark-600">
            <span className="text-xs text-gray-400 whitespace-nowrap">Trades ab</span>
            <input
              type="datetime-local"
              value={tradesFrom}
              onChange={e => setTradesFrom(e.target.value)}
              className="bg-transparent border-none text-xs text-accent-400 focus:outline-none w-[145px] [&::-webkit-calendar-picker-indicator]:invert [&::-webkit-calendar-picker-indicator]:opacity-50"
            />
            {tradesFrom && (
              <button onClick={() => setTradesFrom('')} className="text-gray-500 hover:text-gray-300 text-sm leading-none" title="Filter zurücksetzen">×</button>
            )}
          </div>

          {/* TradingView toggle */}
          <button
            onClick={() => setShowTradingView(!showTradingView)}
            className={`px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
              showTradingView
                ? 'bg-blue-500/20 text-blue-400 border border-blue-500/30'
                : 'bg-dark-700 text-gray-400 hover:text-white hover:bg-dark-600 border border-dark-600'
            }`}
            title="TradingView Chart"
          >
            TV
          </button>

          {/* Start Live Trading */}
          {isAdmin ? (
            <button
              onClick={handleStartLiveTrading}
              disabled={!batchResults || batchLoading}
              className="flex items-center gap-2 px-4 py-2 bg-gradient-to-r from-green-600 to-emerald-600 hover:from-green-500 hover:to-emerald-500 text-white rounded-lg text-sm font-medium transition-all disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
              </svg>
              Neue Session starten
            </button>
          ) : (
            <span className="flex items-center gap-2 px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-sm text-gray-500 cursor-not-allowed">
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
              Neue Session starten
              <span className="px-1.5 py-0.5 bg-amber-500/20 text-amber-400 text-[10px] font-bold rounded">PRO</span>
            </span>
          )}
        </div>

        {/* Parameter Panel */}
        {showParams && (
          <div className="bg-dark-800 rounded-lg border border-dark-600 p-3 mb-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-medium text-gray-400">
                {STRATEGIES.find(s => s.value === backtestStrategy)?.label} Parameter
                {selectedSymbol && <span className="text-accent-400 ml-1">({selectedSymbol})</span>}
              </span>
              <button
                onClick={resetParams}
                className="text-[10px] px-2 py-0.5 rounded bg-dark-700 text-gray-400 hover:text-white transition-colors"
              >
                Reset
              </button>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-2">
              {currentParams.map(p => (
                <div key={p.key} className="flex flex-col gap-0.5">
                  <label className="text-[10px] text-gray-500">{p.label}</label>
                  {p.isToggle ? (
                    <button
                      onClick={() => handleParamChange(p.key, (strategyParams[p.key] ?? p.default) === 1 ? 0 : 1)}
                      className={`rounded px-2 py-1 text-xs font-medium transition-colors ${
                        (strategyParams[p.key] ?? p.default) === 1
                          ? 'bg-green-500/20 text-green-400 border border-green-500/30'
                          : 'bg-dark-700 text-gray-500 border border-dark-600'
                      }`}
                    >
                      {(strategyParams[p.key] ?? p.default) === 1 ? 'ON' : 'OFF'}
                    </button>
                  ) : (
                    <input
                      type="number"
                      value={strategyParams[p.key] ?? p.default}
                      onChange={e => handleParamChange(p.key, parseFloat(e.target.value) || 0)}
                      min={p.min}
                      max={p.max}
                      step={p.step}
                      className="bg-dark-700 border border-dark-600 rounded px-2 py-1 text-xs text-white w-full focus:outline-none focus:border-accent-500"
                    />
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Interval selector */}
        <div className="flex flex-wrap gap-1 mb-4">
          {INTERVALS.map(iv => (
            <button
              key={iv}
              onClick={() => handleIntervalChange(iv)}
              className={`px-3 py-1.5 rounded text-xs font-medium transition-colors ${
                interval === iv
                  ? 'bg-accent-500 text-white'
                  : 'bg-dark-700 text-gray-400 hover:text-white hover:bg-dark-600'
              }`}
            >
              {iv}
            </button>
          ))}
        </div>

        {/* TradingView Embed */}
        {selectedSymbol && showTradingView && (
          <div className="mb-4 rounded-lg overflow-hidden border border-dark-600">
            <iframe
              src={`https://s.tradingview.com/widgetembed/?symbol=${encodeURIComponent(selectedSymbol)}&interval=${TV_INTERVAL_MAP[interval] || '240'}&theme=dark&style=1&studies=BB%40tv-basicstudies&withdateranges=1&hide_side_toolbar=0`}
              width="100%"
              height="400"
              frameBorder="0"
              allowTransparency="true"
              allow="encrypted-media"
              title="TradingView Chart"
            />
          </div>
        )}

        {/* Strategy Info */}
        {(() => {
          const info = STRATEGY_INFO[backtestStrategy]
          if (!info) return null
          return (
            <div className="bg-dark-800/60 rounded-lg border border-dark-600 px-4 py-2.5 mb-4 flex flex-wrap items-start gap-x-6 gap-y-1">
              <div className="flex-1 min-w-[200px]">
                <span className="text-accent-400 text-sm font-semibold">{info.title}</span>
                <p className="text-gray-400 text-xs mt-0.5 leading-relaxed">{info.desc}</p>
              </div>
              <div className="flex flex-col gap-0.5 text-xs shrink-0">
                <div>
                  <span className="text-gray-500">Indikatoren: </span>
                  <span className="text-gray-300">{info.indicators}</span>
                </div>
                <div>
                  <span className="text-gray-500">Empfohlen: </span>
                  <span className="text-accent-400 font-medium">{info.timeframes}</span>
                </div>
              </div>
              {info.legend && (
                <div className="w-full border-t border-dark-600 pt-1.5 mt-1">
                  <div className="grid grid-cols-2 md:grid-cols-3 gap-x-4 gap-y-0.5 text-[11px]">
                    {info.legend.map((l, i) => (
                      <div key={i} className="flex items-center gap-1.5">
                        <span className="font-bold whitespace-nowrap" style={{ color: l.color }}>{l.symbol}</span>
                        <span className="text-gray-500">{l.desc}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
              {info.tips && (
                <div className="w-full border-t border-dark-600 pt-1.5 mt-1">
                  <span className="text-[11px] text-yellow-500/80">{info.tips}</span>
                </div>
              )}
            </div>
          )
        })()}

        {/* Chart */}
        {selectedSymbol ? (
          <div className="bg-dark-800 rounded-lg border border-dark-600 p-2 mb-4">
            <div className="flex items-center gap-3 mb-2 px-2">
              <h2 className="text-white font-bold text-lg">
                {selectedStock?.name ? (
                  <>{selectedStock.name}{' '}<span className="text-gray-400 font-normal text-base">({selectedSymbol})</span></>
                ) : selectedSymbol}
              </h2>
              <span className="text-gray-500 text-xs ml-auto">{INTERVAL_MAP[interval] || interval}</span>
            </div>
            <ArenaChart
              symbol={selectedSymbol}
              interval={INTERVAL_MAP[interval] || '4h'}
              token={token}
              markers={filteredMarkers}
              overlays={backtestResults?.overlays}
              customData={backtestResults?.chart_data}
              loading={backtestLoading}
            />
          </div>
        ) : (
          <div className="flex-1 flex items-center justify-center bg-dark-800 rounded-lg border border-dark-600 mb-4" style={{ minHeight: 450 }}>
            <div className="text-center text-gray-500">
              <svg className="w-16 h-16 mx-auto mb-3 opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M7 12l3-3 3 3 4-4M8 21l4-4 4 4M3 4h18M4 4h16v12a1 1 0 01-1 1H5a1 1 0 01-1-1V4z" />
              </svg>
              <p className="text-sm">Wähle eine Aktie aus der Liste oder nutze die Suche</p>
            </div>
          </div>
        )}

        {/* Indicator Sub-Chart */}
        {backtestResults?.indicators && (
          <ArenaIndicatorChart
            indicators={backtestResults.indicators}
            markers={filteredMarkers}
            strategyName={STRATEGIES.find(s => s.value === backtestStrategy)?.label}
          />
        )}

        {/* Backtest Results */}
        {backtestResults ? (
          <ArenaBacktestPanel
            metrics={filteredMetrics}
            trades={filteredTrades}
            formatPrice={formatPrice}
            symbol={selectedSymbol}
            timeRange={backtestTimeRange}
            tradeAmount={simTradeAmount}
          />
        ) : backtestLoading ? (
          <div className="bg-dark-800 rounded-lg border border-dark-600 p-4">
            <div className="flex items-center gap-3 mb-3">
              <Sk className="h-4 w-40" />
              <Sk className="h-4 w-24 ml-auto" />
            </div>
            <div className="grid grid-cols-4 gap-3 mb-4">
              {[...Array(8)].map((_, i) => (
                <div key={i} className="space-y-1.5">
                  <Sk className="h-3 w-16" />
                  <Sk className="h-5 w-20" />
                </div>
              ))}
            </div>
            <div className="space-y-2">
              {[...Array(3)].map((_, i) => <Sk key={i} className="h-8 w-full" />)}
            </div>
          </div>
        ) : backtestError ? (
          <div className="bg-dark-800 rounded-lg border border-red-500/30 p-4">
            <div className="flex items-center gap-2 text-red-400 text-sm">
              <span className="font-medium">Backtest-Fehler:</span> {backtestError}
            </div>
          </div>
        ) : (
          <div className="bg-dark-800 rounded-lg border border-dark-600 p-4">
            <div className="h-20 flex items-center justify-center text-gray-600 text-sm border border-dashed border-dark-600 rounded">
              Wähle eine Aktie um den Backtest zu starten
            </div>
          </div>
        )}

        {/* Performance aller Strategien */}
        {allStrategyResults && Object.keys(allStrategyResults).length > 0 && (
          <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mt-4">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-medium text-white">Performance aller Strategien</h3>
              {backtestTimeRange && <span className="text-[10px] text-gray-500">{formatTimeRange(backtestTimeRange)}</span>}
            </div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
              {STRATEGIES.map(strat => {
                const locked = !isAdmin && strat.value !== 'hybrid_ai_trend'
                const data = allStrategyResults[strat.value]
                if (locked) return (
                  <div key={strat.value} className="bg-dark-700 rounded-lg p-3 opacity-40 relative">
                    <div className="flex items-center gap-1.5 mb-1">
                      <svg className="w-3 h-3 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                      </svg>
                      <span className="text-xs text-gray-500">{strat.label}</span>
                    </div>
                    <div className="text-gray-600 text-[10px]">In Entwicklung</div>
                  </div>
                )
                if (!data?.metrics) return (
                  <div key={strat.value} className="bg-dark-700 rounded-lg p-3 opacity-50">
                    <div className="text-xs text-gray-500 mb-1">{strat.label}</div>
                    <div className="text-gray-600 text-xs">Keine Daten</div>
                  </div>
                )
                const m = data.metrics
                const isActive = strat.value === backtestStrategy
                return (
                  <div
                    key={strat.value}
                    className={`rounded-lg p-3 cursor-pointer transition-colors ${
                      isActive
                        ? 'bg-accent-500/10 border border-accent-500/30'
                        : 'bg-dark-700 hover:bg-dark-600 border border-transparent'
                    }`}
                    onClick={() => handleStrategyChange(strat.value)}
                  >
                    <div className="flex items-center justify-between mb-2">
                      <span className={`text-xs font-medium ${isActive ? 'text-accent-400' : 'text-gray-400'}`}>
                        {strat.label}
                        {strat.beta && <span className="ml-1 text-[8px] px-1 py-0.5 rounded bg-amber-500/20 text-amber-400 border border-amber-500/30 align-middle">BETA</span>}
                      </span>
                      {data.updated_at && (
                        <span className="text-[10px] text-gray-600">
                          {new Date(data.updated_at).toLocaleDateString('de-DE')}
                        </span>
                      )}
                    </div>
                    <div className="grid grid-cols-4 gap-1.5">
                      <div className="text-center">
                        <div className="text-[10px] text-gray-500">Win%</div>
                        <div className={`text-xs font-bold ${m.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                          {m.win_rate?.toFixed(0)}%
                        </div>
                      </div>
                      <div className="text-center">
                        <div className="text-[10px] text-gray-500">R/R</div>
                        <div className={`text-xs font-bold ${m.risk_reward >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                          {m.risk_reward?.toFixed(1)}
                        </div>
                      </div>
                      <div className="text-center">
                        <div className="text-[10px] text-gray-500">Total</div>
                        <div className={`text-xs font-bold ${m.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {m.total_return >= 0 ? '+' : ''}{m.total_return?.toFixed(0)}%
                        </div>
                      </div>
                      <div className="text-center">
                        <div className="text-[10px] text-gray-500">Trades</div>
                        <div className="text-xs font-bold text-white">{m.total_trades}</div>
                      </div>
                    </div>
                    {simTradeAmount > 0 && (
                      <div className="mt-1.5 text-center">
                        <div className="text-[10px] text-gray-500">Rendite ({simTradeAmount}€)</div>
                        <div className={`text-xs font-bold ${m.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {(simTradeAmount * m.total_return / 100) >= 0 ? '+' : ''}{(simTradeAmount * m.total_return / 100).toFixed(2)} €
                        </div>
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          </div>
        )}

        {/* Watchlist Batch Backtest Results — progress now shown in overlay */}

        {/* Shared Filters for Watchlist Performance + Simulation */}
        {batchResults && !batchLoading && (
          <div className="bg-dark-800 rounded-lg border border-dark-600 p-3 mt-4">
            <form ref={filterFormRef} className="flex items-center gap-3 flex-wrap" onSubmit={e => {
              e.preventDefault()
              const fd = new FormData(e.target)
              const filters = { minWinRate: fd.get('minWinRate'), minRR: fd.get('minRR'), minTotalReturn: fd.get('minTotalReturn'), minAvgReturn: fd.get('minAvgReturn'), minNetProfit: fd.get('minNetProfit'), minMarketCap: fd.get('minMarketCap') }
              startFilterTransition(() => { setAppliedFilters(filters); setFiltersActive(true) })
            }}>
              <span className="text-xs text-gray-400 font-medium whitespace-nowrap">Filter:</span>
              <div className="flex items-center gap-1">
                <label className="text-[10px] text-gray-500">Min Win%</label>
                <input type="number" name="minWinRate" defaultValue="50"
                  className="w-14 bg-dark-700 border border-dark-500 rounded px-1.5 py-1 text-xs text-white" />
              </div>
              <div className="flex items-center gap-1">
                <label className="text-[10px] text-gray-500">Min R/R</label>
                <input type="number" name="minRR" step="0.1" defaultValue="1"
                  className="w-14 bg-dark-700 border border-dark-500 rounded px-1.5 py-1 text-xs text-white" />
              </div>
              <div className="flex items-center gap-1">
                <label className="text-[10px] text-gray-500">Min Total%</label>
                <input type="number" name="minTotalReturn" defaultValue="0"
                  className="w-14 bg-dark-700 border border-dark-500 rounded px-1.5 py-1 text-xs text-white" />
              </div>
              <div className="flex items-center gap-1">
                <label className="text-[10px] text-gray-500">Min Avg%</label>
                <input type="number" name="minAvgReturn" defaultValue=""
                  className="w-14 bg-dark-700 border border-dark-500 rounded px-1.5 py-1 text-xs text-white" />
              </div>
              <div className="flex items-center gap-1">
                <label className="text-[10px] text-gray-500">Min Net%</label>
                <input type="number" name="minNetProfit" defaultValue="0"
                  className="w-14 bg-dark-700 border border-dark-500 rounded px-1.5 py-1 text-xs text-white" />
              </div>
              <div className="flex items-center gap-1">
                <label className="text-[10px] text-gray-500">Min MCap (Mrd)</label>
                <input type="number" name="minMarketCap" step="0.1" defaultValue=""
                  className="w-16 bg-dark-700 border border-dark-500 rounded px-1.5 py-1 text-xs text-white" placeholder="z.B. 10" />
              </div>
              <button
                type={filtersActive ? 'button' : 'submit'}
                onClick={filtersActive ? () => startFilterTransition(() => { setFiltersActive(false); setAppliedFilters(null) }) : undefined}
                className={`px-3 py-1 text-xs rounded font-medium transition-colors whitespace-nowrap ${
                  filtersActive
                    ? 'bg-accent-600 text-white hover:bg-accent-500'
                    : 'bg-dark-600 text-gray-300 hover:bg-dark-500 border border-dark-400'
                }`}
              >
                {filtersActive ? 'Filter aktiv' : 'Filter anwenden'}
              </button>
            </form>
          </div>
        )}

        {batchResults && !batchLoading && <WatchlistBatchPanel batchResults={batchResults} strategy={backtestStrategy} interval={interval} longOnly={longOnly} onSelectStock={selectStock} filterSymbols={perfFilteredSymbols} timeRange={batchTimeRange} formatTimeRange={formatTimeRange} tradeAmount={simTradeAmount} tradesFrom={tradesFrom} />}

        {/* Simulation Section */}
        {batchResults && !batchLoading && (
          <div className="bg-dark-800 rounded-lg border border-dark-600 mt-4">
            <button
              onClick={() => setShowSimulation(!showSimulation)}
              className="w-full px-4 py-3 flex items-center justify-between text-left hover:bg-dark-700/50 transition-colors"
            >
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-white">Simulation</span>
                {batchTimeRange && <span className="text-[10px] text-gray-500">{formatTimeRange(batchTimeRange)}</span>}
              </div>
              <svg className={`w-4 h-4 text-gray-400 transition-transform ${showSimulation ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" /></svg>
            </button>
            {showSimulation && (
              <div className="px-4 pb-4 border-t border-dark-600">
                <div className="flex items-center gap-3 mt-3 mb-3">
                  <div className="flex items-center gap-1">
                    <label className="text-[10px] text-gray-500">Betrag/Trade EUR</label>
                    <input type="number" value={simTradeAmount} onChange={e => setSimTradeAmount(Number(e.target.value) || 500)}
                      className="w-20 bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-xs text-white" />
                  </div>
                  <button
                    onClick={() => {
                      if (!batchResults?.trades) return
                      startFilterTransition(() => {
                        const symbolSet = perfFilteredSymbols ? new Set(perfFilteredSymbols) : null
                        let trades = batchResults.trades
                        if (symbolSet) trades = trades.filter(t => symbolSet.has(t.symbol))
                        if (longOnly) trades = trades.filter(t => t.direction === 'LONG')
                        const amt = simTradeAmount || 500
                        const simTrades = trades.map(t => ({
                          ...t, invested: amt, profitEUR: amt * (t.return_pct / 100),
                        })).sort((a, b) => a.entry_time - b.entry_time)
                        const events = []
                        simTrades.forEach(t => { events.push({ time: t.entry_time, type: 1 }); events.push({ time: t.exit_time, type: -1 }) })
                        events.sort((a, b) => a.time - b.time || a.type - b.type)
                        let open = 0, maxParallel = 0
                        events.forEach(e => { open += e.type; if (open > maxParallel) maxParallel = open })
                        const totalProfit = simTrades.reduce((s, t) => s + t.profitEUR, 0)
                        const wins = simTrades.filter(t => t.return_pct >= 0).length
                        const openCount = simTrades.filter(t => t.is_open).length
                        const requiredCapital = maxParallel * amt
                        const uniqueSymbols = new Set(simTrades.map(t => t.symbol))
                        setSimResults({
                          trades: simTrades, filteredCount: uniqueSymbols.size,
                          totalCount: Object.keys(batchResults.per_stock || {}).length,
                          totalTrades: simTrades.length, totalProfit,
                          winRate: simTrades.length ? (wins / simTrades.length) * 100 : 0,
                          wins, losses: simTrades.length - wins, openCount, maxParallel, requiredCapital,
                          roi: requiredCapital > 0 ? (totalProfit / requiredCapital) * 100 : 0,
                        })
                      })
                    }}
                    className="w-full px-3 py-1.5 bg-accent-600 hover:bg-accent-500 text-white text-xs rounded font-medium transition-colors"
                  >
                    {isFilterPending ? 'Berechne…' : 'Berechnen'}
                  </button>
                </div>

                {/* Simulation Results */}
                {simResults && (
                  <div className="mt-4">
                    {/* Metrics Summary */}
                    <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-8 gap-2 mb-4">
                      {[
                        { label: 'Aktien', value: `${simResults.filteredCount} / ${simResults.totalCount}` },
                        { label: 'Trades', value: simResults.totalTrades },
                        { label: 'Win Rate', value: `${simResults.winRate.toFixed(1)}%`, color: simResults.winRate >= 50 ? 'text-green-400' : 'text-red-400' },
                        { label: 'W / L / Offen', value: `${simResults.wins} / ${simResults.losses} / ${simResults.openCount}` },
                        { label: 'Gewinn/Verlust', value: `${simResults.totalProfit >= 0 ? '+' : ''}${simResults.totalProfit.toFixed(2)} EUR`, color: simResults.totalProfit >= 0 ? 'text-green-400' : 'text-red-400' },
                        { label: 'Max. Positionen', value: simResults.maxParallel },
                        { label: 'Kapital benötigt', value: `${simResults.requiredCapital.toFixed(0)} EUR` },
                        { label: 'ROI', value: `${simResults.roi >= 0 ? '+' : ''}${simResults.roi.toFixed(2)}%`, color: simResults.roi >= 0 ? 'text-green-400' : 'text-red-400' },
                      ].map((m, i) => (
                        <div key={i} className="bg-dark-700 rounded p-2">
                          <div className="text-[10px] text-gray-500">{m.label}</div>
                          <div className={`text-sm font-medium ${m.color || 'text-white'}`}>{m.value}</div>
                        </div>
                      ))}
                    </div>

                    {/* Trade History Table */}
                    {simResults.trades.length > 0 && (
                      <div className="overflow-auto max-h-96 rounded border border-dark-600">
                        <table className="w-full text-xs">
                          <thead className="bg-dark-700 sticky top-0">
                            <tr>
                              {['Symbol', 'Dir', 'Entry', 'Entry Zeit', 'Exit', 'Exit Zeit', 'Investiert', 'G/V EUR', 'Return %', 'Reason'].map(col => (
                                <th key={col} className="px-2 py-1.5 text-left text-gray-400 font-medium whitespace-nowrap">{col}</th>
                              ))}
                            </tr>
                          </thead>
                          <tbody>
                            {[...simResults.trades].sort((a, b) => b.entry_time - a.entry_time).map((t, i) => (
                              <tr key={i} className="border-t border-dark-600 hover:bg-dark-700/50">
                                <td className="px-2 py-1.5 text-white font-medium">{t.symbol}</td>
                                <td className={`px-2 py-1.5 ${t.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>{t.direction}</td>
                                <td className="px-2 py-1.5 text-gray-300">{t.entry_price?.toFixed(2)}</td>
                                <td className="px-2 py-1.5 text-gray-500 whitespace-nowrap">{new Date(t.entry_time * 1000).toLocaleString('de-DE', { day: '2-digit', month: '2-digit', year: '2-digit', hour: '2-digit', minute: '2-digit' })}</td>
                                <td className="px-2 py-1.5 text-gray-300">{t.exit_price?.toFixed(2)}</td>
                                <td className="px-2 py-1.5 text-gray-500 whitespace-nowrap">{new Date(t.exit_time * 1000).toLocaleString('de-DE', { day: '2-digit', month: '2-digit', year: '2-digit', hour: '2-digit', minute: '2-digit' })}</td>
                                <td className="px-2 py-1.5 text-gray-300">{t.invested?.toFixed(0)} EUR</td>
                                <td className={`px-2 py-1.5 font-medium ${t.profitEUR >= 0 ? 'text-green-400' : 'text-red-400'}`}>{t.profitEUR >= 0 ? '+' : ''}{t.profitEUR?.toFixed(2)}</td>
                                <td className={`px-2 py-1.5 ${t.return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>{t.return_pct >= 0 ? '+' : ''}{t.return_pct?.toFixed(2)}%</td>
                                <td className="px-2 py-1.5">
                                  {t.is_open ? (
                                    <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-blue-500/20 text-blue-400">OFFEN</span>
                                  ) : (
                                    <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                                      t.exit_reason === 'TP' ? 'bg-green-500/20 text-green-400' :
                                      t.exit_reason === 'SL' ? 'bg-red-500/20 text-red-400' :
                                      t.exit_reason === 'SIGNAL' ? 'bg-yellow-500/20 text-yellow-400' :
                                      'bg-gray-500/20 text-gray-400'
                                    }`}>{t.exit_reason}</span>
                                  )}
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>
        )}

      </div>

      {/* Sidebar */}
      <div className="md:w-80 bg-dark-800 border-l border-dark-600 flex flex-col overflow-hidden">
        <div className="px-3 py-2.5 border-b border-dark-600 text-sm font-medium text-white">
          Trading Watchlist
        </div>
        <div className="flex-1 flex flex-col overflow-hidden">
          {/* Add symbol input + Import */}
          {isAdmin && (
            <div className="p-2 border-b border-dark-600">
              <div className="flex gap-2">
                <input
                  type="text"
                  value={addSymbol}
                  onChange={e => setAddSymbol(e.target.value)}
                  onKeyDown={e => e.key === 'Enter' && addToTrading()}
                  placeholder="Symbol..."
                  className="flex-1 bg-dark-700 border border-dark-600 rounded px-3 py-1.5 text-white text-sm placeholder-gray-500 focus:outline-none focus:border-accent-500"
                />
                <button
                  onClick={addToTrading}
                  className="px-3 py-1.5 bg-accent-500 text-white rounded text-sm hover:bg-accent-600 transition-colors"
                >
                  +
                </button>
                <button
                  onClick={handleImport}
                  disabled={!!importProgress}
                  className={`px-3 py-1.5 rounded text-sm transition-colors ${
                    importProgress
                      ? 'bg-dark-600 text-gray-500 cursor-not-allowed'
                      : 'bg-dark-700 border border-dark-600 text-gray-300 hover:text-white hover:bg-dark-600'
                  }`}
                  title="Watchlist importieren"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                  </svg>
                </button>
              </div>
              {importProgress && (
                <div className="mt-2">
                  <div className="flex justify-between text-[10px] text-gray-400 mb-1">
                    <span>{importProgress.symbol || 'Starte...'}</span>
                    <span>
                      {importProgress.done
                        ? `+${importProgress.added} | ${importProgress.failed} fehlgeschlagen`
                        : `${importProgress.current}/${importProgress.total}`
                      }
                    </span>
                  </div>
                  <div className="w-full bg-dark-700 rounded-full h-1.5">
                    <div
                      className={`h-1.5 rounded-full transition-all ${importProgress.done ? 'bg-green-500' : 'bg-accent-500'}`}
                      style={{ width: importProgress.total > 0 ? `${(importProgress.current / importProgress.total) * 100}%` : '0%' }}
                    />
                  </div>
                </div>
              )}
            </div>
          )}
          {/* Hide filtered toggle */}
          {(filtersActive || usOnly) && perfFilteredSet && (
            <label className="flex items-center gap-1.5 px-3 py-1.5 border-b border-dark-600 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={hideFiltered}
                onChange={e => setHideFiltered(e.target.checked)}
                className="accent-accent-500 w-3 h-3"
              />
              <span className={`text-[11px] ${hideFiltered ? 'text-accent-400' : 'text-gray-500'}`}>Gefilterte ausblenden</span>
              {perfFilteredSet && <span className="text-[10px] text-gray-600 ml-auto">{perfFilteredSet.size}/{tradingWatchlist.length}</span>}
            </label>
          )}
          <div className="flex-1 overflow-y-auto p-2 relative">
            {isFilterPending && (
              <div className="absolute inset-0 z-10 flex items-center justify-center bg-dark-800/70 rounded">
                <div className="flex items-center gap-2 text-sm text-gray-300">
                  <svg className="animate-spin h-4 w-4 text-accent-400" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
                  Filter wird angewendet…
                </div>
              </div>
            )}
            {tradingWatchlist.map(item => {
              const excluded = (filtersActive || usOnly) && perfFilteredSet && !perfFilteredSet.has(item.symbol)
              if (hideFiltered && excluded) return null
              const hasNoData = noDataSet.has(item.symbol)
              return (
              <div
                key={item.id}
                className={`flex items-center justify-between px-2 py-1.5 rounded hover:bg-dark-700 group ${excluded ? 'opacity-40' : ''} ${hasNoData ? 'opacity-50' : ''}`}
                title={hasNoData ? 'Keine Yahoo-Finance-Daten verfügbar' : ''}
              >
                <button
                  onClick={() => selectStock(item.symbol)}
                  className={`flex-1 text-left text-sm ${
                    selectedSymbol === item.symbol ? 'text-white font-medium' : hasNoData ? 'text-yellow-600' : 'text-gray-300'
                  }`}
                >
                  <span className={`font-medium ${excluded ? 'line-through' : ''}`}>{item.symbol}</span>
                  {hasNoData && <span className="ml-1 text-yellow-500 text-xs" title="Keine Yahoo-Daten">⚠</span>}
                  {item.name && <span className={`text-xs ml-2 ${excluded ? 'text-gray-600 line-through' : hasNoData ? 'text-yellow-700' : 'text-gray-500'}`}>{item.name}</span>}
                  {item.is_live && !hasNoData && <span className="ml-2 w-1.5 h-1.5 rounded-full bg-green-400 inline-block" />}
                </button>
                {isAdmin && (
                  <button
                    onClick={() => removeFromTrading(item.id)}
                    className="text-gray-600 hover:text-red-400 text-xs opacity-0 group-hover:opacity-100 transition-opacity ml-2"
                  >
                    ✕
                  </button>
                )}
              </div>
            )})}
            {tradingWatchlist.length === 0 && (
              isGlobalLoading ? <SidebarSkeleton /> : <div className="text-gray-600 text-sm text-center py-8">Keine Trading-Symbole</div>
            )}
          </div>
        </div>
      </div>

      {/* Session Name Dialog */}
      {showSessionNameDialog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-dark-800 border border-dark-600 rounded-lg p-6 max-w-md w-full mx-4">
            <h3 className="text-lg font-medium text-white mb-4">Neue Live-Session erstellen</h3>
            <div className="mb-4">
              <label className="text-sm text-gray-400 block mb-2">Session-Name</label>
              <input
                type="text"
                value={sessionName}
                onChange={(e) => setSessionName(e.target.value)}
                placeholder={`z.B. "Scalping Q1 2026" (leer = Auto-Name)`}
                className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white text-sm focus:border-accent-500 focus:outline-none"
                autoFocus
                onKeyDown={(e) => e.key === 'Enter' && confirmStartLiveTrading()}
              />
            </div>
            <div className="flex gap-2">
              <button
                onClick={confirmStartLiveTrading}
                className="flex-1 px-4 py-2 bg-green-600 hover:bg-green-500 text-white rounded text-sm font-medium transition-colors"
              >
                Session erstellen
              </button>
              <button
                onClick={() => { setShowSessionNameDialog(false); setSessionName('') }}
                className="flex-1 px-4 py-2 bg-dark-600 hover:bg-dark-500 text-gray-300 rounded text-sm font-medium transition-colors"
              >
                Abbrechen
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default TradingArena
