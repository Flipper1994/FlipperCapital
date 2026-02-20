/**
 * Trading Arena Backtest Tests
 * Tests: Metriken-Berechnung, Portfolio-Rendite, Simulation, Filter, Strategie-Config
 * Run: node frontend/src/tests/arena_backtest.test.mjs
 */

import assert from 'node:assert/strict'

// ─── Test framework ───

let passed = 0, failed = 0, errors = []

function test(name, fn) {
  try {
    fn()
    passed++
    console.log(`  ✓ ${name}`)
  } catch (e) {
    failed++
    errors.push({ name, error: e.message })
    console.log(`  ✗ ${name}`)
    console.log(`    ${e.message}`)
  }
}

function approx(actual, expected, tolerance = 0.01, msg = '') {
  const diff = Math.abs(actual - expected)
  if (diff > tolerance) {
    throw new Error(`${msg}Expected ~${expected}, got ${actual} (diff ${diff.toFixed(6)}, tol ${tolerance})`)
  }
}

// ─── Replizierte Logik aus arenaConfig.js ───

const STRATEGIES = [
  { value: 'regression_scalping', label: 'Regression Scalping', beta: true },
  { value: 'hybrid_ai_trend', label: 'NW Bollinger Bands' },
  { value: 'smart_money_flow', label: 'Smart Money Flow', beta: true },
  { value: 'hann_trend', label: 'Hann Trend (DMH + SAR)' },
  { value: 'gmma_pullback', label: 'GMMA Pullback', disabled: true, disabledReason: 'Nicht profitabel' },
  { value: 'macd_sr', label: 'MACD + S/R' },
  { value: 'trippa_trade', label: 'TrippaTrade RSO', beta: true },
]

const STRATEGY_PARAMS = {
  regression_scalping: [
    { key: 'degree', label: 'Degree', default: 2, min: 1, max: 5, step: 1 },
    { key: 'length', label: 'LinReg Length', default: 100, min: 20, max: 300, step: 10 },
    { key: 'multiplier', label: 'LinReg Multiplier', default: 3.0, min: 0.5, max: 5.0, step: 0.1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 1.5, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'sl_lookback', label: 'SL Lookback', default: 30, min: 5, max: 100, step: 5 },
    { key: 'confirmation_required', label: 'Confirmation', default: 1, min: 0, max: 1, step: 1, isToggle: true },
  ],
  hybrid_ai_trend: [
    { key: 'bb1_period', default: 20 }, { key: 'bb1_stdev', default: 3.0 },
    { key: 'bb2_period', default: 75 }, { key: 'bb2_stdev', default: 3.0 },
    { key: 'bb3_period', default: 100 }, { key: 'bb3_stdev', default: 4.0 },
    { key: 'bb4_period', default: 100 }, { key: 'bb4_stdev', default: 4.25 },
    { key: 'nw_bandwidth', default: 6.0 }, { key: 'nw_lookback', default: 499 },
    { key: 'sl_buffer', default: 1.5 }, { key: 'risk_reward', default: 2.0 },
    { key: 'hybrid_filter', default: 1, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'hybrid_long_thresh', default: 75 }, { key: 'hybrid_short_thresh', default: 25 },
    { key: 'confirm_candle', default: 0, min: 0, max: 1, step: 1, isToggle: true }, { key: 'min_band_dist', default: 0 },
  ],
  smart_money_flow: [
    { key: 'trend_length', default: 34 }, { key: 'basis_smooth', default: 3 },
    { key: 'flow_window', default: 24 }, { key: 'flow_smooth', default: 5 },
    { key: 'flow_boost', default: 1.2 }, { key: 'atr_length', default: 14 },
    { key: 'band_tightness', default: 0.9 }, { key: 'band_expansion', default: 2.2 },
    { key: 'dot_cooldown', default: 12 }, { key: 'risk_reward', default: 2.0 },
  ],
  hann_trend: [
    { key: 'dmh_length', default: 30 }, { key: 'sar_start', default: 0.02 },
    { key: 'sar_increment', default: 0.03 }, { key: 'sar_max', default: 0.3 },
    { key: 'swing_lookback', default: 5 }, { key: 'risk_reward', default: 2.0 },
    { key: 'sl_buffer', default: 0.3 },
  ],
  gmma_pullback: [
    { key: 'signal_len', default: 9 }, { key: 'smooth_len', default: 3 },
    { key: 'fractal_periods', default: 5 }, { key: 'zone_count', default: 5 },
    { key: 'risk_reward', default: 2.0 }, { key: 'sl_lookback', default: 10 },
    { key: 'sl_buffer', default: 0.3 },
  ],
  macd_sr: [
    { key: 'macd_fast', default: 12 }, { key: 'macd_slow', default: 26 },
    { key: 'macd_signal', default: 9 }, { key: 'ema_period', default: 200 },
    { key: 'sl_buffer', default: 1.5 }, { key: 'risk_reward', default: 1.5 },
    { key: 'sr_filter', default: 1, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'fractal_periods', default: 5 }, { key: 'zone_count', default: 5 },
    { key: 'sr_tolerance', default: 1.5 },
    { key: 'hybrid_filter', default: 0, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'hybrid_long_thresh', default: 75 }, { key: 'hybrid_short_thresh', default: 25 },
  ],
  trippa_trade: [
    { key: 'max_range', default: 100 }, { key: 'min_range', default: 10 },
    { key: 'reg_step', default: 5 }, { key: 'signal_len', default: 7 },
    { key: 'ema_fast', default: 5 }, { key: 'ema_slow', default: 13 },
    { key: 'risk_reward', default: 2.0 }, { key: 'sl_buffer', default: 0.5 },
    { key: 'min_trend_bars', default: 3 },
  ],
}

const STRATEGY_DEFAULT_INTERVAL = {
  regression_scalping: '5m',
  hybrid_ai_trend: '5m',
  smart_money_flow: '4h',
  hann_trend: '1h',
  gmma_pullback: '1h',
  macd_sr: '1h',
  trippa_trade: '1h',
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

function getDefaultParams(strategy) {
  const defs = STRATEGY_PARAMS[strategy] || []
  const obj = {}
  defs.forEach(p => { obj[p.key] = p.default })
  return obj
}

// ─── Replizierte Metriken-Berechnung (aus TradingArena.jsx batchMetrics useMemo) ───

function calculateBatchMetrics(trades, { longOnly = false, filterSymbols = null, tradesFromUnix = 0 } = {}) {
  let filtered = [...trades]
  if (longOnly) filtered = filtered.filter(t => t.direction === 'LONG')
  if (filterSymbols) { const set = new Set(filterSymbols); filtered = filtered.filter(t => set.has(t.symbol)) }
  if (tradesFromUnix > 0) filtered = filtered.filter(t => t.entry_time >= tradesFromUnix)
  filtered = filtered.filter(t => !t.is_open)

  if (filtered.length === 0) return null

  let wins = 0, losses = 0, totalReturn = 0, totalWinReturn = 0, totalLossReturn = 0
  let equity = 100, peak = 100, maxDD = 0
  for (const t of filtered) {
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
}

// ─── Replizierte Portfolio-Rendite (aus ArenaBacktestPanel.jsx) ───

function calculateSingleBacktestPortfolio(trades, tradeAmount = 100) {
  const closedTrades = trades.filter(t => !t.is_open)
  const sortedClosed = [...closedTrades].sort((a, b) => a.entry_time - b.entry_time)
  let equity = 1.0
  for (const t of sortedClosed) {
    equity *= (1 + t.return_pct / 100)
  }
  const portfolioReturn = (equity - 1) * 100
  const positionSize = tradeAmount > 0 ? tradeAmount : 100
  const portfolioProfit = positionSize * (equity - 1)
  return { portfolioReturn, portfolioProfit, positionSize }
}

// ─── Replizierte Simulation (aus TradingArena.jsx useEffect recalc) ───

function calculateSimulation(trades, { longOnly = false, filterSymbols = null, tradesFrom = null, tradeAmount = 500 } = {}) {
  let filtered = [...trades]
  if (filterSymbols) { const set = new Set(filterSymbols); filtered = filtered.filter(t => set.has(t.symbol)) }
  if (longOnly) filtered = filtered.filter(t => t.direction === 'LONG')
  if (tradesFrom) {
    const cutoff = Math.floor(new Date(tradesFrom).getTime() / 1000)
    if (cutoff > 0) filtered = filtered.filter(t => t.entry_time >= cutoff)
  }
  const amt = tradeAmount || 500
  const simTrades = filtered.map(t => ({
    ...t, invested: amt, profitEUR: amt * (t.return_pct / 100),
  })).sort((a, b) => a.entry_time - b.entry_time)

  const events = []
  simTrades.forEach(t => {
    events.push({ time: t.entry_time, type: 1 })
    if (t.exit_time) events.push({ time: t.exit_time, type: -1 })
  })
  events.sort((a, b) => a.time - b.time || a.type - b.type)
  let open = 0, maxParallel = 0
  events.forEach(e => { open += e.type; if (open > maxParallel) maxParallel = open })

  const totalProfit = simTrades.reduce((s, t) => s + t.profitEUR, 0)
  const wins = simTrades.filter(t => t.return_pct >= 0).length
  const openCount = simTrades.filter(t => t.is_open).length
  const requiredCapital = maxParallel * amt
  const uniqueSymbols = new Set(simTrades.map(t => t.symbol))

  return {
    trades: simTrades, filteredCount: uniqueSymbols.size,
    totalTrades: simTrades.length, totalProfit,
    winRate: simTrades.length ? (wins / simTrades.length) * 100 : 0,
    wins, losses: simTrades.length - wins, openCount, maxParallel, requiredCapital,
    roi: requiredCapital > 0 ? (totalProfit / requiredCapital) * 100 : 0,
  }
}

// ─── Replizierte WatchlistBatchPanel portfolioStats ───

function calculateWatchlistPortfolio(trades, tradeAmount = 500) {
  const closedTrades = trades.filter(t => !t.is_open)
  const sorted = [...closedTrades].sort((a, b) => a.entry_time - b.entry_time)
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
}

// ─── Test-Daten ───

function makeTrades(specs) {
  // specs: [{ symbol, direction, entry_time, exit_time, entry_price, exit_price, return_pct, exit_reason, is_open }]
  return specs.map((s, i) => ({
    symbol: s.symbol || 'AAPL',
    direction: s.direction || 'LONG',
    entry_time: s.entry_time || (1000 + i * 100),
    exit_time: s.exit_time || (s.is_open ? null : (1050 + i * 100)),
    entry_price: s.entry_price || 100,
    exit_price: s.exit_price || (s.is_open ? null : 100 * (1 + (s.return_pct || 0) / 100)),
    return_pct: s.return_pct || 0,
    exit_reason: s.exit_reason || (s.is_open ? null : 'TP'),
    is_open: s.is_open || false,
  }))
}

// ═══════════════════════════════════════
console.log('\n═══ Trading Arena Backtest Tests ═══\n')
// ═══════════════════════════════════════

// ────────────────────────────────────────
console.log('── 1. Strategie-Konfiguration ──')
// ────────────────────────────────────────

test('Alle 7 Strategien definiert', () => {
  assert.equal(STRATEGIES.length, 7)
})

test('Jede Strategie hat value und label', () => {
  for (const s of STRATEGIES) {
    assert.ok(s.value, `Strategie fehlt value: ${JSON.stringify(s)}`)
    assert.ok(s.label, `Strategie fehlt label: ${JSON.stringify(s)}`)
  }
})

test('Jede Strategie hat Parameter-Definition', () => {
  for (const s of STRATEGIES) {
    assert.ok(STRATEGY_PARAMS[s.value], `Keine Parameter für ${s.value}`)
    assert.ok(STRATEGY_PARAMS[s.value].length > 0, `Leere Parameter für ${s.value}`)
  }
})

test('Jede Strategie hat Default-Interval', () => {
  for (const s of STRATEGIES) {
    const interval = STRATEGY_DEFAULT_INTERVAL[s.value]
    assert.ok(interval, `Kein Default-Interval für ${s.value}`)
    assert.ok(INTERVALS.includes(interval), `Default-Interval ${interval} für ${s.value} nicht in INTERVALS`)
  }
})

test('GMMA Pullback ist disabled', () => {
  const gmma = STRATEGIES.find(s => s.value === 'gmma_pullback')
  assert.ok(gmma.disabled)
  assert.ok(gmma.disabledReason)
})

test('Aktive Strategien sind nicht disabled', () => {
  const active = STRATEGIES.filter(s => !s.disabled)
  assert.equal(active.length, 6)
  for (const s of active) {
    assert.ok(!s.disabled, `${s.value} sollte nicht disabled sein`)
  }
})

test('Alle Default-Intervals sind gültig', () => {
  for (const [strat, interval] of Object.entries(STRATEGY_DEFAULT_INTERVAL)) {
    assert.ok(INTERVALS.includes(interval), `${strat}: ${interval} nicht in INTERVALS`)
    assert.ok(INTERVAL_MAP[interval], `${strat}: ${interval} nicht in INTERVAL_MAP`)
    assert.ok(TV_INTERVAL_MAP[interval], `${strat}: ${interval} nicht in TV_INTERVAL_MAP`)
  }
})

// ────────────────────────────────────────
console.log('\n── 2. getDefaultParams ──')
// ────────────────────────────────────────

for (const s of STRATEGIES) {
  test(`getDefaultParams(${s.value}) liefert korrekte Keys`, () => {
    const params = getDefaultParams(s.value)
    const expected = STRATEGY_PARAMS[s.value].map(p => p.key)
    assert.deepEqual(Object.keys(params).sort(), expected.sort())
  })

  test(`getDefaultParams(${s.value}) — alle Werte innerhalb min/max`, () => {
    const params = getDefaultParams(s.value)
    for (const def of STRATEGY_PARAMS[s.value]) {
      const val = params[def.key]
      if (def.min !== undefined) assert.ok(val >= def.min, `${s.value}.${def.key}: ${val} < min ${def.min}`)
      if (def.max !== undefined) assert.ok(val <= def.max, `${s.value}.${def.key}: ${val} > max ${def.max}`)
    }
  })
}

test('getDefaultParams mit unbekannter Strategie → leeres Objekt', () => {
  const params = getDefaultParams('nicht_existent')
  assert.deepEqual(params, {})
})

// ────────────────────────────────────────
console.log('\n── 3. Strategie-Parameter Validierung ──')
// ────────────────────────────────────────

for (const s of STRATEGIES) {
  test(`${s.value}: Jeder Parameter hat risk_reward`, () => {
    const hasRR = STRATEGY_PARAMS[s.value].some(p => p.key === 'risk_reward')
    assert.ok(hasRR, `${s.value} hat keinen risk_reward Parameter`)
  })

  test(`${s.value}: min < max für alle Parameter`, () => {
    for (const p of STRATEGY_PARAMS[s.value]) {
      if (p.min !== undefined && p.max !== undefined) {
        assert.ok(p.min < p.max, `${s.value}.${p.key}: min(${p.min}) >= max(${p.max})`)
      }
    }
  })

  test(`${s.value}: step > 0 für alle Parameter`, () => {
    for (const p of STRATEGY_PARAMS[s.value]) {
      if (p.step !== undefined) {
        assert.ok(p.step > 0, `${s.value}.${p.key}: step(${p.step}) <= 0`)
      }
    }
  })

  test(`${s.value}: Toggle-Parameter haben min=0, max=1`, () => {
    for (const p of STRATEGY_PARAMS[s.value]) {
      if (p.isToggle) {
        assert.equal(p.min, 0, `${s.value}.${p.key}: Toggle min != 0`)
        assert.equal(p.max, 1, `${s.value}.${p.key}: Toggle max != 1`)
      }
    }
  })
}

// ────────────────────────────────────────
console.log('\n── 4. Batch-Metriken Berechnung ──')
// ────────────────────────────────────────

{
  const trades = makeTrades([
    { return_pct: 10, direction: 'LONG' },
    { return_pct: -5, direction: 'LONG' },
    { return_pct: 8, direction: 'SHORT' },
    { return_pct: -3, direction: 'SHORT' },
    { return_pct: 15, direction: 'LONG' },
  ])

  test('Metriken: WinRate korrekt', () => {
    const m = calculateBatchMetrics(trades)
    // 3 wins (10, 8, 15), 2 losses (-5, -3) → 60%
    approx(m.win_rate, 60, 0.01)
  })

  test('Metriken: Total Trades korrekt', () => {
    const m = calculateBatchMetrics(trades)
    assert.equal(m.total_trades, 5)
    assert.equal(m.wins, 3)
    assert.equal(m.losses, 2)
  })

  test('Metriken: Total Return = Summe', () => {
    const m = calculateBatchMetrics(trades)
    approx(m.total_return, 10 + (-5) + 8 + (-3) + 15, 0.01)
  })

  test('Metriken: Avg Return = Summe / Anzahl', () => {
    const m = calculateBatchMetrics(trades)
    approx(m.avg_return, 25 / 5, 0.01)
  })

  test('Metriken: Risk/Reward korrekt', () => {
    const m = calculateBatchMetrics(trades)
    const avgWin = (10 + 8 + 15) / 3
    const avgLoss = (5 + 3) / 2
    approx(m.risk_reward, avgWin / avgLoss, 0.01)
  })

  test('Metriken: Max Drawdown >= 0', () => {
    const m = calculateBatchMetrics(trades)
    assert.ok(m.max_drawdown >= 0)
  })
}

// ────────────────────────────────────────
console.log('\n── 5. Metriken mit Filtern ──')
// ────────────────────────────────────────

{
  const trades = makeTrades([
    { symbol: 'AAPL', return_pct: 10, direction: 'LONG', entry_time: 1000 },
    { symbol: 'MSFT', return_pct: -5, direction: 'SHORT', entry_time: 2000 },
    { symbol: 'AAPL', return_pct: 8, direction: 'LONG', entry_time: 3000 },
    { symbol: 'GOOG', return_pct: -3, direction: 'LONG', entry_time: 4000 },
  ])

  test('Filter: Long Only', () => {
    const m = calculateBatchMetrics(trades, { longOnly: true })
    assert.equal(m.total_trades, 3) // AAPL+10, AAPL+8, GOOG-3
    approx(m.total_return, 10 + 8 + (-3), 0.01)
  })

  test('Filter: Symbol-Filter', () => {
    const m = calculateBatchMetrics(trades, { filterSymbols: ['AAPL'] })
    assert.equal(m.total_trades, 2)
    approx(m.total_return, 18, 0.01)
  })

  test('Filter: Zeitfilter', () => {
    const m = calculateBatchMetrics(trades, { tradesFromUnix: 2500 })
    assert.equal(m.total_trades, 2) // entry_time 3000 und 4000
    approx(m.total_return, 8 + (-3), 0.01)
  })

  test('Filter: Kombination Long Only + Symbol', () => {
    const m = calculateBatchMetrics(trades, { longOnly: true, filterSymbols: ['AAPL'] })
    assert.equal(m.total_trades, 2)
    approx(m.total_return, 18, 0.01)
  })

  test('Filter: Keine Treffer → null', () => {
    const m = calculateBatchMetrics(trades, { filterSymbols: ['TSLA'] })
    assert.equal(m, null)
  })
}

// ────────────────────────────────────────
console.log('\n── 6. Offene Trades in Metriken ──')
// ────────────────────────────────────────

{
  const trades = makeTrades([
    { return_pct: 10, is_open: false },
    { return_pct: 50, is_open: true }, // offen — wird ignoriert
    { return_pct: -5, is_open: false },
  ])

  test('Offene Trades werden bei Metriken ignoriert', () => {
    const m = calculateBatchMetrics(trades)
    assert.equal(m.total_trades, 2)
    approx(m.total_return, 5, 0.01) // 10 + (-5)
  })
}

// ────────────────────────────────────────
console.log('\n── 7. Max Drawdown Berechnung ──')
// ────────────────────────────────────────

{
  // Sequenz: +10%, -20%, +5% → peak bei 110, trough bei 110*0.80=88, DD = (110-88)/110 = 20%
  const trades = makeTrades([
    { return_pct: 10 },
    { return_pct: -20 },
    { return_pct: 5 },
  ])

  test('MaxDD: korrekt berechnet', () => {
    const m = calculateBatchMetrics(trades)
    // equity: 100 → 110 → 88 → 92.4
    // peak: 100 → 110 → 110 → 110
    // dd:    0%    0%    20%   16%
    approx(m.max_drawdown, 20, 0.01)
  })
}

{
  // Nur Gewinne → kein Drawdown
  const trades = makeTrades([
    { return_pct: 5 }, { return_pct: 10 }, { return_pct: 3 },
  ])

  test('MaxDD: nur Gewinne → DD = 0', () => {
    const m = calculateBatchMetrics(trades)
    approx(m.max_drawdown, 0, 0.01)
  })
}

{
  // Nur Verluste
  const trades = makeTrades([
    { return_pct: -10 }, { return_pct: -15 },
  ])

  test('MaxDD: nur Verluste → kumulierter DD', () => {
    const m = calculateBatchMetrics(trades)
    // equity: 100 → 90 → 76.5, peak bleibt 100
    // dd: 0%, 10%, 23.5%
    approx(m.max_drawdown, 23.5, 0.01)
  })
}

// ────────────────────────────────────────
console.log('\n── 8. Einzel-Backtest Portfolio-Rendite ──')
// ────────────────────────────────────────

{
  const trades = makeTrades([
    { return_pct: 10 }, { return_pct: -5 }, { return_pct: 8 },
  ])

  test('Portfolio-Rendite: Compound-Formel', () => {
    const r = calculateSingleBacktestPortfolio(trades, 100)
    // equity = 1 * 1.10 * 0.95 * 1.08 = 1.1286
    const expected = (1 * 1.10 * 0.95 * 1.08 - 1) * 100
    approx(r.portfolioReturn, expected, 0.01)
  })

  test('Portfolio-Profit: Compound auf Positionsgröße', () => {
    const r = calculateSingleBacktestPortfolio(trades, 200)
    const equity = 1 * 1.10 * 0.95 * 1.08
    approx(r.portfolioProfit, 200 * (equity - 1), 0.01)
  })

  test('Portfolio-Rendite: Default 100€', () => {
    const r = calculateSingleBacktestPortfolio(trades, 0)
    assert.equal(r.positionSize, 100)
  })
}

// ────────────────────────────────────────
console.log('\n── 9. Simulation ──')
// ────────────────────────────────────────

{
  // 3 sequentielle Trades
  const trades = makeTrades([
    { symbol: 'AAPL', return_pct: 10, entry_time: 100, exit_time: 200, direction: 'LONG' },
    { symbol: 'MSFT', return_pct: -5, entry_time: 300, exit_time: 400, direction: 'SHORT' },
    { symbol: 'GOOG', return_pct: 8, entry_time: 500, exit_time: 600, direction: 'LONG' },
  ])

  test('Simulation: Sequentielle Trades → maxParallel=1', () => {
    const sim = calculateSimulation(trades, { tradeAmount: 500 })
    assert.equal(sim.maxParallel, 1)
    assert.equal(sim.requiredCapital, 500)
  })

  test('Simulation: Total Profit korrekt', () => {
    const sim = calculateSimulation(trades, { tradeAmount: 500 })
    approx(sim.totalProfit, 500 * (0.10 + (-0.05) + 0.08), 0.01)
  })

  test('Simulation: ROI korrekt', () => {
    const sim = calculateSimulation(trades, { tradeAmount: 500 })
    approx(sim.roi, (65 / 500) * 100, 0.01)
  })

  test('Simulation: WinRate korrekt', () => {
    const sim = calculateSimulation(trades, { tradeAmount: 500 })
    approx(sim.winRate, (2 / 3) * 100, 0.1) // 2 wins, 1 loss
  })
}

{
  // Parallele Trades
  const trades = makeTrades([
    { symbol: 'AAPL', return_pct: 10, entry_time: 100, exit_time: 500 },
    { symbol: 'MSFT', return_pct: -5, entry_time: 100, exit_time: 500 },
    { symbol: 'GOOG', return_pct: 8, entry_time: 100, exit_time: 500 },
  ])

  test('Simulation: Parallele Trades → maxParallel=3', () => {
    const sim = calculateSimulation(trades, { tradeAmount: 500 })
    assert.equal(sim.maxParallel, 3)
    assert.equal(sim.requiredCapital, 1500)
  })

  test('Simulation: Parallele ROI = Profit/Kapital', () => {
    const sim = calculateSimulation(trades, { tradeAmount: 500 })
    approx(sim.roi, (65 / 1500) * 100, 0.01) // 65€ Profit / 1500€ Kapital
  })
}

// ────────────────────────────────────────
console.log('\n── 10. Simulation mit Filtern ──')
// ────────────────────────────────────────

{
  const trades = makeTrades([
    { symbol: 'AAPL', return_pct: 10, direction: 'LONG', entry_time: 1000, exit_time: 2000 },
    { symbol: 'MSFT', return_pct: -5, direction: 'SHORT', entry_time: 1000, exit_time: 2000 },
    { symbol: 'GOOG', return_pct: 8, direction: 'LONG', entry_time: 3000, exit_time: 4000 },
  ])

  test('Simulation: Long Only Filter', () => {
    const sim = calculateSimulation(trades, { longOnly: true, tradeAmount: 500 })
    assert.equal(sim.totalTrades, 2) // nur AAPL und GOOG
  })

  test('Simulation: Symbol Filter', () => {
    const sim = calculateSimulation(trades, { filterSymbols: ['AAPL', 'MSFT'], tradeAmount: 500 })
    assert.equal(sim.totalTrades, 2) // AAPL und MSFT
    assert.equal(sim.filteredCount, 2)
  })
}

// ────────────────────────────────────────
console.log('\n── 11. Simulation mit offenen Trades ──')
// ────────────────────────────────────────

{
  const trades = makeTrades([
    { symbol: 'AAPL', return_pct: 10, entry_time: 100, exit_time: 200, is_open: false },
    { symbol: 'MSFT', return_pct: 50, entry_time: 100, exit_time: null, is_open: true },
  ])

  test('Simulation: Offene Trades korrekt gezählt', () => {
    const sim = calculateSimulation(trades, { tradeAmount: 500 })
    assert.equal(sim.openCount, 1)
    assert.equal(sim.totalTrades, 2)
  })

  test('Simulation: Offene Trades ohne exit_time → maxParallel korrekt', () => {
    const sim = calculateSimulation(trades, { tradeAmount: 500 })
    // AAPL: entry=100, exit=200 → open=1 then open=0
    // MSFT: entry=100, no exit → open stays +1
    // Events: (100,+1), (100,+1), (200,-1)  → max at 100: 2, then at 200: 1
    assert.equal(sim.maxParallel, 2)
  })
}

// ────────────────────────────────────────
console.log('\n── 12. Watchlist-Batch Portfolio ──')
// ────────────────────────────────────────

{
  const trades = makeTrades([
    { return_pct: 10, entry_time: 100, exit_time: 200 },
    { return_pct: -5, entry_time: 100, exit_time: 200 },
    { return_pct: 8, entry_time: 300, exit_time: 400 },
  ])

  test('Watchlist: maxParallel bei überlappenden Trades', () => {
    const r = calculateWatchlistPortfolio(trades, 500)
    // Trade 1 und 2 parallel (100-200), Trade 3 allein (300-400)
    assert.equal(r.maxParallel, 2)
  })

  test('Watchlist: requiredCapital = maxParallel × posSize', () => {
    const r = calculateWatchlistPortfolio(trades, 500)
    assert.equal(r.requiredCapital, 1000)
  })

  test('Watchlist: ROI = totalProfit / requiredCapital', () => {
    const r = calculateWatchlistPortfolio(trades, 500)
    const expectedProfit = 500 * (0.10 + (-0.05) + 0.08)
    approx(r.totalProfit, expectedProfit, 0.01)
    approx(r.portfolioReturn, (expectedProfit / 1000) * 100, 0.01)
  })
}

// ────────────────────────────────────────
console.log('\n── 13. Edge Cases ──')
// ────────────────────────────────────────

test('Leere Trade-Liste → null Metriken', () => {
  const m = calculateBatchMetrics([])
  assert.equal(m, null)
})

test('Nur offene Trades → null Metriken', () => {
  const trades = makeTrades([
    { return_pct: 10, is_open: true },
    { return_pct: -5, is_open: true },
  ])
  const m = calculateBatchMetrics(trades)
  assert.equal(m, null)
})

test('Ein Trade → korrekte Metriken', () => {
  const trades = makeTrades([{ return_pct: 15 }])
  const m = calculateBatchMetrics(trades)
  assert.equal(m.total_trades, 1)
  assert.equal(m.wins, 1)
  assert.equal(m.losses, 0)
  approx(m.win_rate, 100, 0.01)
  approx(m.total_return, 15, 0.01)
  approx(m.avg_return, 15, 0.01)
  approx(m.max_drawdown, 0, 0.01)
})

test('Trade mit 0% Return → zählt als Win', () => {
  const trades = makeTrades([{ return_pct: 0 }])
  const m = calculateBatchMetrics(trades)
  assert.equal(m.wins, 1)
  assert.equal(m.losses, 0)
})

test('Sehr großer Verlust: -99% → Drawdown nahe 99%', () => {
  const trades = makeTrades([{ return_pct: -99 }])
  const m = calculateBatchMetrics(trades)
  approx(m.max_drawdown, 99, 0.01)
})

test('Simulation: Leere Trades → 0 ROI', () => {
  const sim = calculateSimulation([], { tradeAmount: 500 })
  assert.equal(sim.totalTrades, 0)
  approx(sim.roi, 0, 0.01)
  assert.equal(sim.maxParallel, 0)
})

test('Simulation: tradeAmount 0 → Default 500', () => {
  const trades = makeTrades([{ return_pct: 10, entry_time: 100, exit_time: 200 }])
  const sim = calculateSimulation(trades, { tradeAmount: 0 })
  assert.equal(sim.requiredCapital, 500) // maxParallel=1 × 500
})

test('Watchlist: Keine Trades → 0% Rendite', () => {
  const r = calculateWatchlistPortfolio([], 500)
  approx(r.portfolioReturn, 0, 0.01)
  assert.equal(r.maxParallel, 0)
})

// ────────────────────────────────────────
console.log('\n── 14. NaN/Infinity Schutz ──')
// ────────────────────────────────────────

test('Metriken: Risk/Reward bei 0 Verlusten → 0 (nicht Infinity)', () => {
  const trades = makeTrades([{ return_pct: 10 }, { return_pct: 5 }])
  const m = calculateBatchMetrics(trades)
  // avgLoss = 0 → riskReward = 0 (weil avgLoss > 0 ist false)
  assert.ok(isFinite(m.risk_reward), `risk_reward is not finite: ${m.risk_reward}`)
  assert.ok(!isNaN(m.risk_reward), `risk_reward is NaN`)
})

test('Metriken: Alle Werte sind finite', () => {
  const trades = makeTrades([
    { return_pct: 10 }, { return_pct: -5 }, { return_pct: 0 },
  ])
  const m = calculateBatchMetrics(trades)
  for (const [key, val] of Object.entries(m)) {
    assert.ok(isFinite(val), `${key} is not finite: ${val}`)
    assert.ok(!isNaN(val), `${key} is NaN`)
  }
})

test('Simulation: Alle Werte sind finite', () => {
  const trades = makeTrades([
    { return_pct: 10, entry_time: 100, exit_time: 200 },
    { return_pct: -5, entry_time: 100, exit_time: 200 },
  ])
  const sim = calculateSimulation(trades, { tradeAmount: 500 })
  for (const key of ['totalProfit', 'winRate', 'roi', 'maxParallel', 'requiredCapital']) {
    assert.ok(isFinite(sim[key]), `${key} is not finite: ${sim[key]}`)
    assert.ok(!isNaN(sim[key]), `${key} is NaN`)
  }
})

// ────────────────────────────────────────
console.log('\n── 15. Konsistenz: Batch vs Single Backtest ──')
// ────────────────────────────────────────

{
  // Wenn man die gleichen Trades einmal als Batch (batchMetrics) und einmal als Single (ArenaBacktestPanel) berechnet,
  // sollten WinRate, TotalReturn etc. übereinstimmen
  const trades = makeTrades([
    { return_pct: 10 }, { return_pct: -5 }, { return_pct: 8 }, { return_pct: -3 }, { return_pct: 12 },
  ])

  test('Konsistenz: WinRate gleich', () => {
    const batch = calculateBatchMetrics(trades)
    // Single berechnet WinRate nicht direkt, aber: 3 wins / 5 total = 60%
    approx(batch.win_rate, 60, 0.01)
  })

  test('Konsistenz: Portfolio-Rendite vs. Batch-Return', () => {
    const batch = calculateBatchMetrics(trades)
    const single = calculateSingleBacktestPortfolio(trades)
    // batch.total_return = Summe = 22%
    // single.portfolioReturn = Compound
    // Bei kleinen Returns sollten beide ähnlich sein, aber Compound ist genauer
    approx(batch.total_return, 22, 0.01) // Summe
    // Compound: 1 * 1.10 * 0.95 * 1.08 * 0.97 * 1.12 ≈ 22.36%
    const compound = (1 * 1.10 * 0.95 * 1.08 * 0.97 * 1.12 - 1) * 100
    approx(single.portfolioReturn, compound, 0.01)
  })
}

// ────────────────────────────────────────
console.log('\n── 16. Strategie-spezifische Default-Tests ──')
// ────────────────────────────────────────

test('regression_scalping: Default degree=2, length=100', () => {
  const p = getDefaultParams('regression_scalping')
  assert.equal(p.degree, 2)
  assert.equal(p.length, 100)
  assert.equal(p.multiplier, 3.0)
  assert.equal(p.risk_reward, 1.5)
})

test('hybrid_ai_trend: Default bb1_period=20, 4 BB-Level', () => {
  const p = getDefaultParams('hybrid_ai_trend')
  assert.equal(p.bb1_period, 20)
  assert.equal(p.bb2_period, 75)
  assert.equal(p.bb3_period, 100)
  assert.equal(p.bb4_period, 100)
  assert.equal(p.hybrid_filter, 1)
})

test('smart_money_flow: Default trend_length=34, risk_reward=2.0', () => {
  const p = getDefaultParams('smart_money_flow')
  assert.equal(p.trend_length, 34)
  assert.equal(p.risk_reward, 2.0)
  assert.equal(p.flow_boost, 1.2)
})

test('hann_trend: Default dmh_length=30, SAR-Params', () => {
  const p = getDefaultParams('hann_trend')
  assert.equal(p.dmh_length, 30)
  assert.equal(p.sar_start, 0.02)
  assert.equal(p.sar_increment, 0.03)
  assert.equal(p.sar_max, 0.3)
})

test('macd_sr: Default MACD 12/26/9, S/R Filter an', () => {
  const p = getDefaultParams('macd_sr')
  assert.equal(p.macd_fast, 12)
  assert.equal(p.macd_slow, 26)
  assert.equal(p.macd_signal, 9)
  assert.equal(p.sr_filter, 1)
  assert.equal(p.ema_period, 200)
})

test('trippa_trade: Default Dual-MACD params', () => {
  const p = getDefaultParams('trippa_trade')
  assert.equal(p.max_range, 100)
  assert.equal(p.min_range, 10)
  assert.equal(p.ema_fast, 5)
  assert.equal(p.ema_slow, 13)
  assert.equal(p.min_trend_bars, 3)
})

test('gmma_pullback: Default signal_len=9, fractal_periods=5', () => {
  const p = getDefaultParams('gmma_pullback')
  assert.equal(p.signal_len, 9)
  assert.equal(p.fractal_periods, 5)
  assert.equal(p.zone_count, 5)
})

// ────────────────────────────────────────
console.log('\n── 17. Realistische Multi-Symbol Simulation ──')
// ────────────────────────────────────────

{
  // 10 Aktien mit realistischen Returns, verschiedene Laufzeiten
  const DAY = 86400
  const NOW = 1739491200
  const trades = [
    // Batch 1: 5 Trades gleichzeitig
    { symbol: 'AAPL', return_pct: 3.2, entry_time: NOW - 30*DAY, exit_time: NOW - 25*DAY, direction: 'LONG' },
    { symbol: 'MSFT', return_pct: -1.5, entry_time: NOW - 30*DAY, exit_time: NOW - 25*DAY, direction: 'LONG' },
    { symbol: 'GOOG', return_pct: 5.1, entry_time: NOW - 30*DAY, exit_time: NOW - 25*DAY, direction: 'LONG' },
    { symbol: 'AMZN', return_pct: -2.8, entry_time: NOW - 30*DAY, exit_time: NOW - 25*DAY, direction: 'SHORT' },
    { symbol: 'TSLA', return_pct: 8.0, entry_time: NOW - 30*DAY, exit_time: NOW - 25*DAY, direction: 'LONG' },
    // Batch 2: 3 Trades danach
    { symbol: 'NVDA', return_pct: 12.5, entry_time: NOW - 20*DAY, exit_time: NOW - 15*DAY, direction: 'LONG' },
    { symbol: 'META', return_pct: -4.0, entry_time: NOW - 20*DAY, exit_time: NOW - 15*DAY, direction: 'SHORT' },
    { symbol: 'NFLX', return_pct: 2.1, entry_time: NOW - 20*DAY, exit_time: NOW - 15*DAY, direction: 'LONG' },
    // Aktuell offen
    { symbol: 'AAPL', return_pct: 1.5, entry_time: NOW - 5*DAY, exit_time: null, is_open: true, direction: 'LONG' },
  ].map(t => ({ ...t, entry_price: 100, exit_price: t.is_open ? null : 100 * (1 + t.return_pct/100), exit_reason: t.is_open ? null : (t.return_pct >= 0 ? 'TP' : 'SL'), is_open: t.is_open || false }))

  test('Realistische Simulation: Metriken korrekt', () => {
    const m = calculateBatchMetrics(trades)
    // 8 geschlossene Trades: 5 wins (3.2, 5.1, 8.0, 12.5, 2.1), 3 losses (-1.5, -2.8, -4.0)
    assert.equal(m.total_trades, 8)
    assert.equal(m.wins, 5)
    assert.equal(m.losses, 3)
    approx(m.win_rate, (5/8)*100, 0.01)
  })

  test('Realistische Simulation: maxParallel korrekt', () => {
    const sim = calculateSimulation(trades, { tradeAmount: 500 })
    // Batch 1: 5 parallel, Batch 2: 3 parallel, AAPL offen: 1
    // max = 5
    assert.equal(sim.maxParallel, 5)
  })

  test('Realistische Simulation: Long Only Filter', () => {
    const sim = calculateSimulation(trades, { longOnly: true, tradeAmount: 500 })
    // AMZN (SHORT) und META (SHORT) rausgefiltert → 7 verbleibend
    assert.equal(sim.totalTrades, 7)
  })

  test('Realistische Simulation: Symbol Filter', () => {
    const sim = calculateSimulation(trades, { filterSymbols: ['AAPL', 'MSFT'], tradeAmount: 500 })
    // AAPL x2 + MSFT x1 = 3
    assert.equal(sim.totalTrades, 3)
    assert.equal(sim.filteredCount, 2) // 2 unique symbols
  })

  test('Realistische Simulation: Offene Trades gezählt', () => {
    const sim = calculateSimulation(trades, { tradeAmount: 500 })
    assert.equal(sim.openCount, 1)
  })
}

// ═══════════════════════════════════════
// Summary
// ═══════════════════════════════════════

console.log('\n═══════════════════════════════════════')
console.log(`Results: ${passed} passed, ${failed} failed out of ${passed + failed} tests`)
if (errors.length > 0) {
  console.log('\nFailed tests:')
  for (const e of errors) console.log(`  ✗ ${e.name}: ${e.error}`)
}
console.log('═══════════════════════════════════════\n')

process.exit(failed > 0 ? 1 : 0)
