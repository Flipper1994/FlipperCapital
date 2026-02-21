/**
 * DaytradingStats Calculation Tests
 * Tests all KPI calculations, date filters, sorting, and edge cases
 * Run: node frontend/src/tests/daytradingStats.test.mjs
 */

import assert from 'node:assert/strict'

// ─── Extracted calculation logic from DaytradingStats.jsx ───

function computeStats(positions) {
  if (positions.length === 0) return null
  const closedPositions = positions.filter(p => p.is_closed)
  const closed = closedPositions
  const totalPnl = closed.reduce((s, p) => s + (p.profit_loss_amt || 0), 0)
  const totalInvested = closed.reduce((s, p) => s + (p.invested_amount || 0), 0)
  const wins = closed.filter(p => (p.profit_loss_pct || 0) > 0)
  const losses = closed.filter(p => (p.profit_loss_pct || 0) <= 0)
  const winRate = closed.length > 0 ? (wins.length / closed.length) * 100 : 0
  const avgWin = wins.length > 0 ? wins.reduce((s, p) => s + p.profit_loss_pct, 0) / wins.length : 0
  const avgLoss = losses.length > 0 ? losses.reduce((s, p) => s + p.profit_loss_pct, 0) / losses.length : 0
  const rr = avgLoss !== 0 ? Math.abs(avgWin / avgLoss) : 0
  const grossWin = wins.reduce((s, p) => s + (p.profit_loss_amt || 0), 0)
  const grossLoss = Math.abs(losses.reduce((s, p) => s + (p.profit_loss_amt || 0), 0))
  const profitFactor = grossLoss > 0 ? grossWin / grossLoss : grossWin > 0 ? Infinity : 0
  const avgReturn = closed.length > 0 ? closed.reduce((s, p) => s + (p.profit_loss_pct || 0), 0) / closed.length : 0
  const best = closed.length > 0 ? Math.max(...closed.map(p => p.profit_loss_pct || 0)) : 0
  const worst = closed.length > 0 ? Math.min(...closed.map(p => p.profit_loss_pct || 0)) : 0

  // Additive equity curve + max drawdown
  const sorted = [...closed].sort((a, b) => new Date(a.close_time) - new Date(b.close_time))
  let eq = 100, peak = 100, maxDD = 0
  sorted.forEach(t => {
    eq += (t.profit_loss_pct || 0)
    if (eq > peak) peak = eq
    const dd = peak > 0 ? (peak - eq) / peak * 100 : 0
    if (dd > maxDD) maxDD = dd
  })
  const rendite = eq - 100

  // Avg holding duration
  const durations = closed.filter(p => p.entry_time && p.close_time).map(p =>
    (new Date(p.close_time) - new Date(p.entry_time)) / (1000 * 60)
  )
  const avgDuration = durations.length > 0 ? durations.reduce((a, b) => a + b, 0) / durations.length : 0

  // Streaks
  let winStreak = 0, lossStreak = 0, maxWinStreak = 0, maxLossStreak = 0
  sorted.forEach(t => {
    if ((t.profit_loss_pct || 0) > 0) {
      winStreak++; lossStreak = 0
      if (winStreak > maxWinStreak) maxWinStreak = winStreak
    } else {
      lossStreak++; winStreak = 0
      if (lossStreak > maxLossStreak) maxLossStreak = lossStreak
    }
  })

  // Max concurrent open positions
  const events = []
  positions.forEach(p => {
    events.push({ time: new Date(p.entry_time).getTime(), delta: 1 })
    if (p.is_closed && p.close_time) events.push({ time: new Date(p.close_time).getTime(), delta: -1 })
  })
  events.sort((a, b) => a.time - b.time || a.delta - b.delta)
  let concurrent = 0, maxConcurrent = 0
  events.forEach(e => {
    concurrent += e.delta
    if (concurrent > maxConcurrent) maxConcurrent = concurrent
  })
  const tradeAmount = positions.length > 0 ? (positions[0].invested_amount || 100) : 100
  const minCapital = maxConcurrent * tradeAmount

  return {
    totalPnl, rendite, winRate, rr, profitFactor, avgReturn,
    totalTrades: positions.length, totalClosed: closed.length,
    wins: wins.length, losses: losses.length,
    avgWin, avgLoss, best, worst, maxDD, avgDuration,
    maxWinStreak, maxLossStreak, totalInvested, equity: eq,
    maxConcurrent, minCapital
  }
}

function computeSymbolBreakdown(positions) {
  const map = {}
  positions.forEach(p => {
    if (!map[p.symbol]) map[p.symbol] = { symbol: p.symbol, trades: 0, wins: 0, totalReturn: 0, pnl: 0 }
    map[p.symbol].trades++
    map[p.symbol].totalReturn += p.profit_loss_pct || 0
    map[p.symbol].pnl += p.profit_loss_amt || 0
    if ((p.profit_loss_pct || 0) > 0) map[p.symbol].wins++
  })
  return Object.values(map).map(s => ({
    ...s,
    winRate: s.trades > 0 ? (s.wins / s.trades) * 100 : 0,
    avgReturn: s.trades > 0 ? s.totalReturn / s.trades : 0,
  }))
}

function computeISOWeekRange(customWeek) {
  const [y, w] = customWeek.split('-W').map(Number)
  const jan4 = new Date(y, 0, 4)
  const dow = jan4.getDay() || 7
  const week1Monday = new Date(jan4)
  week1Monday.setDate(jan4.getDate() - dow + 1)
  const start = new Date(week1Monday)
  start.setDate(week1Monday.getDate() + (w - 1) * 7)
  start.setHours(0, 0, 0, 0)
  const end = new Date(start)
  end.setDate(end.getDate() + 6)
  end.setHours(23, 59, 59, 999)
  return { start, end }
}

function sortSymbols(breakdown, sortField, sortDir) {
  return [...breakdown].sort((a, b) => {
    const mul = sortDir === 'asc' ? 1 : -1
    if (sortField === 'symbol') return a.symbol.localeCompare(b.symbol) * mul
    return (a[sortField] - b[sortField]) * mul
  })
}

function formatDuration(mins) {
  if (mins < 60) return `${Math.round(mins)}m`
  if (mins < 1440) return `${Math.floor(mins / 60)}h ${Math.round(mins % 60)}m`
  return `${Math.floor(mins / 1440)}d ${Math.floor((mins % 1440) / 60)}h`
}

// ─── Test Helpers ───

function mkPos({ symbol = 'AAPL', pct = 0, amt = null, invested = 100, entry = '2025-01-10T10:00:00Z', close = '2025-01-10T11:00:00Z', closed = true, direction = 'LONG', reason = 'SIGNAL' }) {
  return {
    id: Math.random(),
    symbol,
    direction,
    profit_loss_pct: pct,
    profit_loss_amt: amt !== null ? amt : invested * pct / 100,
    invested_amount: invested,
    entry_time: entry,
    close_time: closed ? close : null,
    is_closed: closed,
    close_reason: closed ? reason : 'OPEN',
  }
}

let passed = 0, failed = 0, errors = []
function test(name, fn) {
  try {
    fn()
    passed++
    console.log(`  ✓ ${name}`)
  } catch (e) {
    failed++
    errors.push({ name, error: e })
    console.log(`  ✗ ${name}: ${e.message}`)
  }
}

function approx(a, b, eps = 0.01) {
  assert.ok(Math.abs(a - b) < eps, `Expected ~${b}, got ${a}`)
}

// ═══════════════════════════════════════════
// 1. BASIC STATS
// ═══════════════════════════════════════════
console.log('\n═══ 1. Basic Stats ═══')

test('totalPnl sums profit_loss_amt of closed positions', () => {
  const pos = [
    mkPos({ pct: 10, amt: 10 }),
    mkPos({ pct: -5, amt: -5 }),
    mkPos({ pct: 3, amt: 3 }),
  ]
  const s = computeStats(pos)
  approx(s.totalPnl, 8)
})

test('totalPnl ignores open positions', () => {
  const pos = [
    mkPos({ pct: 10, amt: 10 }),
    mkPos({ pct: 50, amt: 50, closed: false }), // open — should be ignored
  ]
  const s = computeStats(pos)
  approx(s.totalPnl, 10)
})

test('winRate is based on closed positions only', () => {
  const pos = [
    mkPos({ pct: 10 }),  // win
    mkPos({ pct: -5 }),  // loss
    mkPos({ pct: 3 }),   // win
    mkPos({ pct: 20, closed: false }), // open — not counted
  ]
  const s = computeStats(pos)
  approx(s.winRate, 66.67)
  assert.equal(s.wins, 2)
  assert.equal(s.losses, 1)
})

test('totalTrades includes open, totalClosed only closed', () => {
  const pos = [
    mkPos({ pct: 10 }),
    mkPos({ pct: -5 }),
    mkPos({ pct: 0, closed: false }),
  ]
  const s = computeStats(pos)
  assert.equal(s.totalTrades, 3)
  assert.equal(s.totalClosed, 2)
})

test('avgWin and avgLoss calculations', () => {
  const pos = [
    mkPos({ pct: 10 }),
    mkPos({ pct: 6 }),
    mkPos({ pct: -4 }),
    mkPos({ pct: -8 }),
  ]
  const s = computeStats(pos)
  approx(s.avgWin, 8)    // (10+6)/2
  approx(s.avgLoss, -6)  // (-4+-8)/2
})

test('rr = |avgWin / avgLoss|', () => {
  const pos = [
    mkPos({ pct: 12 }),
    mkPos({ pct: -4 }),
  ]
  const s = computeStats(pos)
  approx(s.rr, 3.0)  // |12 / -4| = 3
})

test('profitFactor = grossWin / grossLoss', () => {
  const pos = [
    mkPos({ pct: 10, amt: 10 }),
    mkPos({ pct: 5, amt: 5 }),
    mkPos({ pct: -3, amt: -3 }),
  ]
  const s = computeStats(pos)
  approx(s.profitFactor, 5.0) // 15 / 3
})

test('avgReturn is average of all closed positions', () => {
  const pos = [
    mkPos({ pct: 10 }),
    mkPos({ pct: -4 }),
    mkPos({ pct: 6 }),
  ]
  const s = computeStats(pos)
  approx(s.avgReturn, 4.0)  // (10 + -4 + 6) / 3
})

test('best and worst trades', () => {
  const pos = [
    mkPos({ pct: 10 }),
    mkPos({ pct: -7 }),
    mkPos({ pct: 3 }),
  ]
  const s = computeStats(pos)
  approx(s.best, 10)
  approx(s.worst, -7)
})

// ═══════════════════════════════════════════
// 2. RENDITE (ADDITIVE EQUITY)
// ═══════════════════════════════════════════
console.log('\n═══ 2. Rendite (Additive Equity) ═══')

test('rendite = sum of profit_loss_pct (additive)', () => {
  const pos = [
    mkPos({ pct: 10, close: '2025-01-10T11:00:00Z' }),
    mkPos({ pct: -5, close: '2025-01-10T12:00:00Z' }),
    mkPos({ pct: 3, close: '2025-01-10T13:00:00Z' }),
  ]
  const s = computeStats(pos)
  approx(s.rendite, 8.0) // 10 + (-5) + 3 = 8
  approx(s.equity, 108.0) // 100 + 8
})

test('rendite with all losses', () => {
  const pos = [
    mkPos({ pct: -5, close: '2025-01-10T11:00:00Z' }),
    mkPos({ pct: -3, close: '2025-01-10T12:00:00Z' }),
    mkPos({ pct: -2, close: '2025-01-10T13:00:00Z' }),
  ]
  const s = computeStats(pos)
  approx(s.rendite, -10.0)
  approx(s.equity, 90.0)
})

test('equity is additive, not multiplicative', () => {
  // Example: 3 trades of +10%, -10%, +10%
  // Additive: 100 + 10 - 10 + 10 = 110
  // Multiplicative (old bug): 100 * 1.10 * 0.90 * 1.10 = 108.9
  const pos = [
    mkPos({ pct: 10, close: '2025-01-10T11:00:00Z' }),
    mkPos({ pct: -10, close: '2025-01-10T12:00:00Z' }),
    mkPos({ pct: 10, close: '2025-01-10T13:00:00Z' }),
  ]
  const s = computeStats(pos)
  approx(s.equity, 110.0) // NOT 108.9
  approx(s.rendite, 10.0) // NOT 8.9
})

test('rendite is NOT totalPnl/totalInvested (old bug)', () => {
  // Old formula: totalPnl(8) / totalInvested(300) * 100 = 2.67% — WRONG
  // New formula: sum of pct = 10 + (-5) + 3 = 8%
  const pos = [
    mkPos({ pct: 10, amt: 10, invested: 100, close: '2025-01-10T11:00:00Z' }),
    mkPos({ pct: -5, amt: -5, invested: 100, close: '2025-01-10T12:00:00Z' }),
    mkPos({ pct: 3, amt: 3, invested: 100, close: '2025-01-10T13:00:00Z' }),
  ]
  const s = computeStats(pos)
  const oldRendite = s.totalPnl / s.totalInvested * 100 // = 2.67%
  assert.notEqual(Math.round(s.rendite * 100), Math.round(oldRendite * 100),
    'rendite should NOT equal totalPnl/totalInvested')
  approx(s.rendite, 8.0)
})

// ═══════════════════════════════════════════
// 3. MAX DRAWDOWN
// ═══════════════════════════════════════════
console.log('\n═══ 3. Max Drawdown ═══')

test('maxDD basic case', () => {
  // Equity: 100 → 110 → 105 → 108
  // Peak: 110. DD after trade 2: (110-105)/110 = 4.55%
  const pos = [
    mkPos({ pct: 10, close: '2025-01-10T11:00:00Z' }),
    mkPos({ pct: -5, close: '2025-01-10T12:00:00Z' }),
    mkPos({ pct: 3, close: '2025-01-10T13:00:00Z' }),
  ]
  const s = computeStats(pos)
  approx(s.maxDD, (110 - 105) / 110 * 100, 0.01) // 4.55%
})

test('maxDD with consecutive losses', () => {
  // Equity: 100 → 95 → 88 → 91
  // Peak: 100. DD after trade 2: (100-88)/100 = 12%
  const pos = [
    mkPos({ pct: -5, close: '2025-01-10T11:00:00Z' }),
    mkPos({ pct: -7, close: '2025-01-10T12:00:00Z' }),
    mkPos({ pct: 3, close: '2025-01-10T13:00:00Z' }),
  ]
  const s = computeStats(pos)
  approx(s.maxDD, 12.0)
})

test('maxDD zero when all wins', () => {
  const pos = [
    mkPos({ pct: 5, close: '2025-01-10T11:00:00Z' }),
    mkPos({ pct: 3, close: '2025-01-10T12:00:00Z' }),
    mkPos({ pct: 8, close: '2025-01-10T13:00:00Z' }),
  ]
  const s = computeStats(pos)
  approx(s.maxDD, 0)
})

test('maxDD uses additive equity, not multiplicative', () => {
  // Trade: +50%, -30%
  // Additive: 100 → 150 → 120, DD = (150-120)/150 = 20%
  // Multiplicative: 100 → 150 → 105, DD = (150-105)/150 = 30%
  const pos = [
    mkPos({ pct: 50, close: '2025-01-10T11:00:00Z' }),
    mkPos({ pct: -30, close: '2025-01-10T12:00:00Z' }),
  ]
  const s = computeStats(pos)
  approx(s.maxDD, 20.0) // NOT 30%
})

// ═══════════════════════════════════════════
// 4. STREAKS
// ═══════════════════════════════════════════
console.log('\n═══ 4. Streaks ═══')

test('win streak', () => {
  const pos = [
    mkPos({ pct: 5, close: '2025-01-10T11:00:00Z' }),
    mkPos({ pct: 3, close: '2025-01-10T12:00:00Z' }),
    mkPos({ pct: 8, close: '2025-01-10T13:00:00Z' }),
    mkPos({ pct: -2, close: '2025-01-10T14:00:00Z' }),
    mkPos({ pct: 1, close: '2025-01-10T15:00:00Z' }),
  ]
  const s = computeStats(pos)
  assert.equal(s.maxWinStreak, 3)
  assert.equal(s.maxLossStreak, 1)
})

test('loss streak', () => {
  const pos = [
    mkPos({ pct: 5, close: '2025-01-10T11:00:00Z' }),
    mkPos({ pct: -3, close: '2025-01-10T12:00:00Z' }),
    mkPos({ pct: -8, close: '2025-01-10T13:00:00Z' }),
    mkPos({ pct: -2, close: '2025-01-10T14:00:00Z' }),
    mkPos({ pct: 1, close: '2025-01-10T15:00:00Z' }),
  ]
  const s = computeStats(pos)
  assert.equal(s.maxWinStreak, 1)
  assert.equal(s.maxLossStreak, 3)
})

test('streak at end of array is counted', () => {
  const pos = [
    mkPos({ pct: -1, close: '2025-01-10T11:00:00Z' }),
    mkPos({ pct: 5, close: '2025-01-10T12:00:00Z' }),
    mkPos({ pct: 3, close: '2025-01-10T13:00:00Z' }),
    mkPos({ pct: 8, close: '2025-01-10T14:00:00Z' }),
  ]
  const s = computeStats(pos)
  assert.equal(s.maxWinStreak, 3)
})

// ═══════════════════════════════════════════
// 5. DURATION
// ═══════════════════════════════════════════
console.log('\n═══ 5. Duration ═══')

test('avgDuration in minutes', () => {
  const pos = [
    mkPos({ entry: '2025-01-10T10:00:00Z', close: '2025-01-10T11:00:00Z', pct: 5 }), // 60 min
    mkPos({ entry: '2025-01-10T10:00:00Z', close: '2025-01-10T10:30:00Z', pct: 3 }), // 30 min
  ]
  const s = computeStats(pos)
  approx(s.avgDuration, 45) // (60+30)/2
})

test('formatDuration minutes', () => {
  assert.equal(formatDuration(45), '45m')
})

test('formatDuration hours', () => {
  assert.equal(formatDuration(90), '1h 30m')
})

test('formatDuration days', () => {
  assert.equal(formatDuration(1500), '1d 1h')
})

// ═══════════════════════════════════════════
// 6. CONCURRENT POSITIONS
// ═══════════════════════════════════════════
console.log('\n═══ 6. Concurrent Positions ═══')

test('maxConcurrent with overlapping positions', () => {
  const pos = [
    mkPos({ entry: '2025-01-10T10:00:00Z', close: '2025-01-10T12:00:00Z', pct: 5 }),
    mkPos({ entry: '2025-01-10T10:30:00Z', close: '2025-01-10T11:00:00Z', pct: 3 }),
    mkPos({ entry: '2025-01-10T10:45:00Z', close: '2025-01-10T11:30:00Z', pct: -2 }),
  ]
  const s = computeStats(pos)
  assert.equal(s.maxConcurrent, 3)
  approx(s.minCapital, 300)
})

test('maxConcurrent sequential (no overlap)', () => {
  const pos = [
    mkPos({ entry: '2025-01-10T10:00:00Z', close: '2025-01-10T11:00:00Z', pct: 5 }),
    mkPos({ entry: '2025-01-10T11:00:00Z', close: '2025-01-10T12:00:00Z', pct: 3 }),
  ]
  const s = computeStats(pos)
  // Close at same time as open: close comes first (-1 before +1), so max = 1
  assert.equal(s.maxConcurrent, 1)
})

test('maxConcurrent includes open positions', () => {
  const pos = [
    mkPos({ entry: '2025-01-10T10:00:00Z', close: '2025-01-10T12:00:00Z', pct: 5 }),
    mkPos({ entry: '2025-01-10T10:30:00Z', closed: false, pct: 0 }),
  ]
  const s = computeStats(pos)
  assert.equal(s.maxConcurrent, 2)
})

// ═══════════════════════════════════════════
// 7. SYMBOL BREAKDOWN
// ═══════════════════════════════════════════
console.log('\n═══ 7. Symbol Breakdown ═══')

test('groups by symbol correctly', () => {
  const pos = [
    mkPos({ symbol: 'AAPL', pct: 10, amt: 10 }),
    mkPos({ symbol: 'AAPL', pct: -5, amt: -5 }),
    mkPos({ symbol: 'MSFT', pct: 8, amt: 8 }),
  ]
  const bd = computeSymbolBreakdown(pos)
  assert.equal(bd.length, 2)
  const aapl = bd.find(s => s.symbol === 'AAPL')
  const msft = bd.find(s => s.symbol === 'MSFT')
  assert.equal(aapl.trades, 2)
  assert.equal(aapl.wins, 1)
  approx(aapl.winRate, 50)
  approx(aapl.totalReturn, 5) // 10 + (-5)
  approx(aapl.pnl, 5)
  approx(aapl.avgReturn, 2.5)
  assert.equal(msft.trades, 1)
  approx(msft.winRate, 100)
})

test('symbol sorting works for string column', () => {
  const bd = [
    { symbol: 'MSFT', trades: 3, pnl: 10 },
    { symbol: 'AAPL', trades: 5, pnl: -5 },
    { symbol: 'TSLA', trades: 1, pnl: 20 },
  ]
  const sorted = sortSymbols(bd, 'symbol', 'asc')
  assert.equal(sorted[0].symbol, 'AAPL')
  assert.equal(sorted[1].symbol, 'MSFT')
  assert.equal(sorted[2].symbol, 'TSLA')
})

test('symbol sorting works for numeric column', () => {
  const bd = [
    { symbol: 'MSFT', trades: 3, pnl: 10 },
    { symbol: 'AAPL', trades: 5, pnl: -5 },
    { symbol: 'TSLA', trades: 1, pnl: 20 },
  ]
  const sorted = sortSymbols(bd, 'pnl', 'desc')
  assert.equal(sorted[0].symbol, 'TSLA') // 20
  assert.equal(sorted[1].symbol, 'MSFT') // 10
  assert.equal(sorted[2].symbol, 'AAPL') // -5
})

test('symbol sorting desc for string', () => {
  const bd = [
    { symbol: 'AAPL' },
    { symbol: 'TSLA' },
    { symbol: 'MSFT' },
  ]
  const sorted = sortSymbols(bd, 'symbol', 'desc')
  assert.equal(sorted[0].symbol, 'TSLA')
  assert.equal(sorted[2].symbol, 'AAPL')
})

// ═══════════════════════════════════════════
// 8. ISO WEEK DATE RANGE
// ═══════════════════════════════════════════
console.log('\n═══ 8. ISO Week Date Range ═══')

test('2024 week 1 (Jan 1 is Monday)', () => {
  const { start, end } = computeISOWeekRange('2024-W01')
  assert.equal(start.getFullYear(), 2024)
  assert.equal(start.getMonth(), 0) // January
  assert.equal(start.getDate(), 1)  // Monday Jan 1
  assert.equal(end.getDate(), 7)    // Sunday Jan 7
})

test('2024 week 2', () => {
  const { start, end } = computeISOWeekRange('2024-W02')
  assert.equal(start.getDate(), 8)  // Monday Jan 8
  assert.equal(end.getDate(), 14)   // Sunday Jan 14
})

test('2025 week 1 (Jan 1 is Wednesday → week 1 starts Dec 30)', () => {
  const { start, end } = computeISOWeekRange('2025-W01')
  // ISO: Jan 1 2025 is Wednesday. Week 1 contains first Thursday (Jan 2).
  // Week 1 Monday = Dec 30, 2024
  assert.equal(start.getFullYear(), 2024)
  assert.equal(start.getMonth(), 11)  // December
  assert.equal(start.getDate(), 30)
  assert.equal(end.getFullYear(), 2025)
  assert.equal(end.getMonth(), 0)     // January
  assert.equal(end.getDate(), 5)
})

test('2023 week 1 (Jan 1 is Sunday → week 1 starts Jan 2)', () => {
  const { start, end } = computeISOWeekRange('2023-W01')
  // ISO: Jan 1 2023 is Sunday. First Thursday = Jan 5.
  // Week 1 Monday = Jan 2
  assert.equal(start.getFullYear(), 2023)
  assert.equal(start.getMonth(), 0)
  assert.equal(start.getDate(), 2)
  assert.equal(end.getDate(), 8)
})

test('2022 week 1 (Jan 1 is Saturday → week 1 starts Jan 3)', () => {
  const { start, end } = computeISOWeekRange('2022-W01')
  // ISO: Jan 1 2022 is Saturday. First Thursday = Jan 6.
  // Week 1 Monday = Jan 3
  assert.equal(start.getFullYear(), 2022)
  assert.equal(start.getMonth(), 0)
  assert.equal(start.getDate(), 3)
  assert.equal(end.getDate(), 9)
})

test('2021 week 1 (Jan 1 is Friday → week 1 starts Jan 4)', () => {
  const { start, end } = computeISOWeekRange('2021-W01')
  // ISO: Jan 1 2021 is Friday. First Thursday = Jan 7.
  // Week 1 Monday = Jan 4
  assert.equal(start.getFullYear(), 2021)
  assert.equal(start.getMonth(), 0)
  assert.equal(start.getDate(), 4)
  assert.equal(end.getDate(), 10)
})

test('2026 week 1 (Jan 1 is Thursday → week 1 starts Dec 29)', () => {
  const { start, end } = computeISOWeekRange('2026-W01')
  // ISO: Jan 1 2026 is Thursday. Week 1 contains Jan 1.
  // Week 1 Monday = Dec 29, 2025
  assert.equal(start.getFullYear(), 2025)
  assert.equal(start.getMonth(), 11)
  assert.equal(start.getDate(), 29)
  assert.equal(end.getFullYear(), 2026)
  assert.equal(end.getMonth(), 0)
  assert.equal(end.getDate(), 4)
})

test('week range is Monday to Sunday', () => {
  const { start, end } = computeISOWeekRange('2025-W10')
  assert.equal(start.getDay(), 1) // Monday
  assert.equal(end.getDay(), 0)   // Sunday
})

// ═══════════════════════════════════════════
// 9. EDGE CASES
// ═══════════════════════════════════════════
console.log('\n═══ 9. Edge Cases ═══')

test('empty positions returns null', () => {
  const s = computeStats([])
  assert.equal(s, null)
})

test('only open positions — stats based on 0 closed', () => {
  const pos = [
    mkPos({ pct: 10, closed: false }),
    mkPos({ pct: -5, closed: false }),
  ]
  const s = computeStats(pos)
  // totalTrades = 2 (including open), but all KPIs = 0 because no closed
  assert.equal(s.totalTrades, 2)
  assert.equal(s.totalClosed, 0)
  approx(s.totalPnl, 0)
  approx(s.winRate, 0)
  approx(s.rendite, 0)
  approx(s.equity, 100)
})

test('single closed position', () => {
  const pos = [mkPos({ pct: 5, amt: 5 })]
  const s = computeStats(pos)
  assert.equal(s.totalTrades, 1)
  assert.equal(s.totalClosed, 1)
  approx(s.rendite, 5)
  approx(s.winRate, 100)
  approx(s.maxDD, 0)
  approx(s.best, 5)
  approx(s.worst, 5)
})

test('all winning trades', () => {
  const pos = [
    mkPos({ pct: 5, close: '2025-01-10T11:00:00Z' }),
    mkPos({ pct: 3, close: '2025-01-10T12:00:00Z' }),
    mkPos({ pct: 8, close: '2025-01-10T13:00:00Z' }),
  ]
  const s = computeStats(pos)
  approx(s.winRate, 100)
  approx(s.losses, 0)
  approx(s.avgLoss, 0)
  approx(s.maxDD, 0)
  assert.equal(s.profitFactor, Infinity)
  assert.equal(s.maxLossStreak, 0)
})

test('all losing trades', () => {
  const pos = [
    mkPos({ pct: -5, amt: -5, close: '2025-01-10T11:00:00Z' }),
    mkPos({ pct: -3, amt: -3, close: '2025-01-10T12:00:00Z' }),
    mkPos({ pct: -8, amt: -8, close: '2025-01-10T13:00:00Z' }),
  ]
  const s = computeStats(pos)
  approx(s.winRate, 0)
  approx(s.wins, 0)
  approx(s.avgWin, 0)
  approx(s.profitFactor, 0)
  assert.equal(s.maxWinStreak, 0)
  assert.ok(s.best <= 0, 'best should be ≤ 0')
})

test('trade with exactly 0% pct counts as loss', () => {
  const pos = [mkPos({ pct: 0, amt: 0 })]
  const s = computeStats(pos)
  assert.equal(s.wins, 0)
  assert.equal(s.losses, 1)
  approx(s.winRate, 0)
})

test('profitFactor all wins no losses = Infinity', () => {
  const pos = [
    mkPos({ pct: 5, amt: 5 }),
    mkPos({ pct: 3, amt: 3 }),
  ]
  const s = computeStats(pos)
  assert.equal(s.profitFactor, Infinity)
})

test('rr with no losses = 0 (avgLoss is 0)', () => {
  const pos = [mkPos({ pct: 5 })]
  const s = computeStats(pos)
  assert.equal(s.rr, 0) // avgLoss = 0, so rr = 0
})

// ═══════════════════════════════════════════
// 10. REALISTIC SCENARIO
// ═══════════════════════════════════════════
console.log('\n═══ 10. Realistic Scenario ═══')

test('10-trade session with mixed results', () => {
  const trades = [
    mkPos({ symbol: 'AAPL', pct: 3.5, amt: 3.5, close: '2025-01-10T11:00:00Z' }),
    mkPos({ symbol: 'MSFT', pct: -2.1, amt: -2.1, close: '2025-01-10T12:00:00Z' }),
    mkPos({ symbol: 'AAPL', pct: 1.8, amt: 1.8, close: '2025-01-10T13:00:00Z' }),
    mkPos({ symbol: 'TSLA', pct: -4.5, amt: -4.5, close: '2025-01-10T14:00:00Z' }),
    mkPos({ symbol: 'MSFT', pct: 2.3, amt: 2.3, close: '2025-01-10T15:00:00Z' }),
    mkPos({ symbol: 'AAPL', pct: -1.0, amt: -1.0, close: '2025-01-11T10:00:00Z' }),
    mkPos({ symbol: 'TSLA', pct: 5.2, amt: 5.2, close: '2025-01-11T11:00:00Z' }),
    mkPos({ symbol: 'MSFT', pct: 0.8, amt: 0.8, close: '2025-01-11T12:00:00Z' }),
    mkPos({ symbol: 'AAPL', pct: -3.2, amt: -3.2, close: '2025-01-11T13:00:00Z' }),
    mkPos({ symbol: 'TSLA', pct: 4.1, amt: 4.1, close: '2025-01-11T14:00:00Z' }),
  ]
  const s = computeStats(trades)

  // Total P&L: 3.5 - 2.1 + 1.8 - 4.5 + 2.3 - 1.0 + 5.2 + 0.8 - 3.2 + 4.1 = 6.9
  approx(s.totalPnl, 6.9)

  // Rendite (additive) = sum of pct = 6.9
  approx(s.rendite, 6.9)

  // Wins: 3.5, 1.8, 2.3, 5.2, 0.8, 4.1 = 6 wins
  assert.equal(s.wins, 6)
  assert.equal(s.losses, 4)
  approx(s.winRate, 60)

  // avgWin = (3.5+1.8+2.3+5.2+0.8+4.1)/6 = 17.7/6 = 2.95
  approx(s.avgWin, 2.95)

  // avgLoss = (-2.1 + -4.5 + -1.0 + -3.2)/4 = -10.8/4 = -2.7
  approx(s.avgLoss, -2.7)

  // RR = |2.95 / -2.7| = 1.093
  approx(s.rr, 1.093, 0.01)

  // Equity curve: 100 → 103.5 → 101.4 → 103.2 → 98.7 → 101.0 → 100.0 → 105.2 → 106.0 → 102.8 → 106.9
  // Peak after trade 1: 103.5, DD after trade 4: (103.5 - 98.7)/103.5 = 4.64%
  // Peak after trade 8: 106.0, DD after trade 9: (106.0 - 102.8)/106.0 = 3.02%
  // Max DD = 4.64%
  approx(s.maxDD, 4.64, 0.1)

  // Symbol breakdown
  const bd = computeSymbolBreakdown(trades)
  const aapl = bd.find(s => s.symbol === 'AAPL')
  assert.equal(aapl.trades, 4)
  assert.equal(aapl.wins, 2) // 3.5, 1.8
  approx(aapl.winRate, 50)
  approx(aapl.totalReturn, 1.1) // 3.5+1.8-1.0-3.2
})

// ═══════════════════════════════════════════
// SUMMARY
// ═══════════════════════════════════════════
console.log('\n' + '═'.repeat(40))
console.log(`Results: ${passed} passed, ${failed} failed`)
if (errors.length > 0) {
  console.log('\nFailed tests:')
  errors.forEach(({ name, error }) => {
    console.log(`  ✗ ${name}`)
    console.log(`    ${error.message}`)
  })
}
console.log('═'.repeat(40))
process.exit(failed > 0 ? 1 : 0)
