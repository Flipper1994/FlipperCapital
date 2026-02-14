/**
 * Live Trading Calculation Tests
 * Tests P&L, session summary, symbol aggregation, and config payload
 * Run: node frontend/src/tests/live_trading.test.mjs
 */

import assert from 'node:assert/strict'

// ─── P&L Calculation (mirrors closeLivePosition logic) ───

function calcPnL(direction, entryPrice, closePrice, investedAmount) {
  const pct = direction === 'LONG'
    ? (closePrice - entryPrice) / entryPrice * 100
    : (entryPrice - closePrice) / entryPrice * 100
  const amt = investedAmount * pct / 100
  return { pct: Math.round(pct * 100) / 100, amt: Math.round(amt * 100) / 100 }
}

// ─── Session Summary ───

function calcSessionSummary(positions) {
  const closed = positions.filter(p => p.is_closed)
  const wins = closed.filter(p => p.profit_loss_pct > 0)
  const totalPnl = closed.reduce((sum, p) => sum + p.profit_loss_amt, 0)
  const winRate = closed.length > 0 ? (wins.length / closed.length * 100) : 0
  return {
    totalTrades: closed.length,
    wins: wins.length,
    losses: closed.length - wins.length,
    winRate: Math.round(winRate * 100) / 100,
    totalPnl: Math.round(totalPnl * 100) / 100,
  }
}

// ─── Symbol Aggregation ───

function aggregateBySymbol(positions) {
  const map = {}
  for (const p of positions) {
    if (!map[p.symbol]) map[p.symbol] = { symbol: p.symbol, trades: 0, totalPnlPct: 0, totalPnlAmt: 0 }
    map[p.symbol].trades++
    map[p.symbol].totalPnlPct += p.profit_loss_pct
    map[p.symbol].totalPnlAmt += p.profit_loss_amt
  }
  return Object.values(map).sort((a, b) => b.totalPnlAmt - a.totalPnlAmt)
}

// ─── Config Payload Builder ───

function buildConfigPayload({ strategy, interval, params, symbols, longOnly, tradeAmount, filters, filtersActive, currency }) {
  return {
    strategy,
    interval,
    params: params || {},
    symbols: symbols || [],
    long_only: longOnly !== undefined ? longOnly : true,
    trade_amount: tradeAmount || 500,
    filters: filters || {},
    filters_active: filtersActive || false,
    currency: currency || 'EUR',
  }
}

// ═══════════════════════════════════════════
// Tests
// ═══════════════════════════════════════════

let passed = 0
let failed = 0

function test(name, fn) {
  try {
    fn()
    console.log(`  ✓ ${name}`)
    passed++
  } catch (e) {
    console.log(`  ✗ ${name}: ${e.message}`)
    failed++
  }
}

console.log('\n=== Live Trading Tests ===\n')

// --- P&L Tests ---

test('P&L LONG profit: 500€ × +10% = +50€', () => {
  const { pct, amt } = calcPnL('LONG', 100, 110, 500)
  assert.equal(pct, 10)
  assert.equal(amt, 50)
})

test('P&L LONG loss: 500€ × -5% = -25€', () => {
  const { pct, amt } = calcPnL('LONG', 100, 95, 500)
  assert.equal(pct, -5)
  assert.equal(amt, -25)
})

test('P&L SHORT profit: 500€ × +10% = +50€', () => {
  const { pct, amt } = calcPnL('SHORT', 100, 90, 500)
  assert.equal(pct, 10)
  assert.equal(amt, 50)
})

test('P&L SHORT loss: 500€ × -3% = -15€', () => {
  const { pct, amt } = calcPnL('SHORT', 100, 103, 500)
  assert.equal(pct, -3)
  assert.equal(amt, -15)
})

test('P&L with different invest: 1000€ × +5% = +50€', () => {
  const { pct, amt } = calcPnL('LONG', 200, 210, 1000)
  assert.equal(pct, 5)
  assert.equal(amt, 50)
})

// --- Session Summary Tests ---

test('Session Summary: correct wins, losses, winRate, totalPnl', () => {
  const positions = [
    { is_closed: true, profit_loss_pct: 5, profit_loss_amt: 25 },
    { is_closed: true, profit_loss_pct: -3, profit_loss_amt: -15 },
    { is_closed: true, profit_loss_pct: 8, profit_loss_amt: 40 },
    { is_closed: true, profit_loss_pct: -2, profit_loss_amt: -10 },
    { is_closed: false, profit_loss_pct: 0, profit_loss_amt: 0 }, // open, ignored
  ]
  const s = calcSessionSummary(positions)
  assert.equal(s.totalTrades, 4)
  assert.equal(s.wins, 2)
  assert.equal(s.losses, 2)
  assert.equal(s.winRate, 50)
  assert.equal(s.totalPnl, 40)
})

test('Session Summary: all winners → 100% winRate', () => {
  const positions = [
    { is_closed: true, profit_loss_pct: 5, profit_loss_amt: 25 },
    { is_closed: true, profit_loss_pct: 10, profit_loss_amt: 50 },
  ]
  const s = calcSessionSummary(positions)
  assert.equal(s.winRate, 100)
  assert.equal(s.totalPnl, 75)
})

test('Session Summary: empty → 0 trades', () => {
  const s = calcSessionSummary([])
  assert.equal(s.totalTrades, 0)
  assert.equal(s.winRate, 0)
  assert.equal(s.totalPnl, 0)
})

// --- Symbol Aggregation Tests ---

test('Symbol Aggregation: groups by symbol, sorted by PnL', () => {
  const positions = [
    { symbol: 'AAPL', profit_loss_pct: 5, profit_loss_amt: 25 },
    { symbol: 'MSFT', profit_loss_pct: 10, profit_loss_amt: 50 },
    { symbol: 'AAPL', profit_loss_pct: -3, profit_loss_amt: -15 },
    { symbol: 'MSFT', profit_loss_pct: 2, profit_loss_amt: 10 },
  ]
  const result = aggregateBySymbol(positions)
  assert.equal(result.length, 2)
  // MSFT has higher total PnL (60) → first
  assert.equal(result[0].symbol, 'MSFT')
  assert.equal(result[0].trades, 2)
  assert.equal(result[0].totalPnlAmt, 60)
  assert.equal(result[0].totalPnlPct, 12)
  // AAPL
  assert.equal(result[1].symbol, 'AAPL')
  assert.equal(result[1].trades, 2)
  assert.equal(result[1].totalPnlAmt, 10)
})

// --- Config Payload Tests ---

test('Config Payload: correct JSON structure with defaults', () => {
  const payload = buildConfigPayload({
    strategy: 'hybrid_ai_trend',
    interval: '5m',
    symbols: ['AAPL', 'MSFT'],
  })
  assert.equal(payload.strategy, 'hybrid_ai_trend')
  assert.equal(payload.interval, '5m')
  assert.deepEqual(payload.symbols, ['AAPL', 'MSFT'])
  assert.equal(payload.long_only, true)
  assert.equal(payload.trade_amount, 500)
  assert.equal(payload.currency, 'EUR')
  assert.equal(payload.filters_active, false)
  assert.deepEqual(payload.params, {})
})

test('Config Payload: custom values override defaults', () => {
  const payload = buildConfigPayload({
    strategy: 'regression_scalping',
    interval: '15m',
    params: { bb1_period: 20 },
    symbols: ['TSLA'],
    longOnly: false,
    tradeAmount: 1000,
    filters: { minWinRate: '50' },
    filtersActive: true,
    currency: 'USD',
  })
  assert.equal(payload.strategy, 'regression_scalping')
  assert.equal(payload.long_only, false)
  assert.equal(payload.trade_amount, 1000)
  assert.equal(payload.currency, 'USD')
  assert.equal(payload.filters_active, true)
  assert.deepEqual(payload.params, { bb1_period: 20 })
})

// ═══════════════════════════════════════════
console.log(`\n${passed} passed, ${failed} failed\n`)
if (failed > 0) process.exit(1)
