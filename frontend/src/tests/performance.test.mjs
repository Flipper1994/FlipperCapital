/**
 * Performance Page Calculation Tests
 * Tests modeData stats and runSimForMode simulation for all 5 modes, 1y and 3y
 * Run: node frontend/src/tests/performance.test.mjs
 */

import assert from 'node:assert/strict'

// ─── Extracted calculation logic from Performance.jsx ───

const MODES = [
  { key: 'defensive', title: 'Defensiv' },
  { key: 'aggressive', title: 'Aggressiv' },
  { key: 'quant', title: 'Quant' },
  { key: 'ditz', title: 'Ditz' },
  { key: 'trader', title: 'Trader' },
]

function computeCutoffDate(timeRange, nowSec) {
  const y = 365 * 24 * 60 * 60
  const map = { '1m': 30 * 86400, '3m': 90 * 86400, '6m': 180 * 86400, '1y': y, '2y': 2 * y, '3y': 3 * y, '4y': 4 * y, '5y': 5 * y, '10y': 10 * y }
  return map[timeRange] ? nowSec - map[timeRange] : 0
}

function computeModeData(trades, cutoffDate, filters = {}) {
  const result = {}
  for (const m of MODES) {
    const filtered = trades
      .filter(t => t.mode === m.key && t.entry_date >= cutoffDate)
      .filter(t => {
        if (filters.minWinrate && t.win_rate < parseFloat(filters.minWinrate)) return false
        if (filters.maxWinrate && t.win_rate > parseFloat(filters.maxWinrate)) return false
        if (filters.minRR && t.risk_reward < parseFloat(filters.minRR)) return false
        if (filters.maxRR && t.risk_reward > parseFloat(filters.maxRR)) return false
        if (filters.minAvgReturn && t.avg_return < parseFloat(filters.minAvgReturn)) return false
        if (filters.maxAvgReturn && t.avg_return > parseFloat(filters.maxAvgReturn)) return false
        if (filters.minMarketCap && t.market_cap < parseFloat(filters.minMarketCap) * 1e9) return false
        return true
      })
    const wins = filtered.filter(t => (t.return_pct || 0) > 0)
    const losses = filtered.filter(t => (t.return_pct || 0) < 0)
    const totalReturn = filtered.reduce((s, t) => s + (t.return_pct || 0), 0)
    const aw = wins.length > 0 ? wins.reduce((s, t) => s + t.return_pct, 0) / wins.length : 0
    const al = losses.length > 0 ? Math.abs(losses.reduce((s, t) => s + t.return_pct, 0) / losses.length) : 0
    result[m.key] = {
      trades: filtered,
      stats: {
        tradeCount: filtered.length,
        winRate: filtered.length > 0 ? (wins.length / filtered.length) * 100 : 0,
        riskReward: al > 0 ? aw / al : aw > 0 ? Infinity : 0,
        totalReturn,
        avgReturn: filtered.length > 0 ? totalReturn / filtered.length : 0,
        wins: wins.length,
        losses: losses.length,
      }
    }
  }
  return result
}

function runSimForMode(filteredTrades, simAmount, nowSec) {
  const amount = parseFloat(simAmount)
  if (!amount || amount <= 0 || !filteredTrades.length) return null

  const simTrades = []
  let totalProfit = 0, openCount = 0
  for (const t of filteredTrades) {
    const pct = t.return_pct || 0
    const isOpen = t.status === 'OPEN' || !t.exit_date
    const profit = Math.round(amount * (pct / 100) * 100) / 100
    if (isOpen) openCount++
    totalProfit += profit
    simTrades.push({
      symbol: t.symbol, name: t.name,
      entryDate: t.entry_date, exitDate: isOpen ? null : t.exit_date,
      entryPrice: t.entry_price, exitPrice: isOpen ? t.current_price : t.exit_price,
      profit, received: Math.round((amount + profit) * 100) / 100,
      returnPct: pct, status: t.status,
    })
  }
  simTrades.sort((a, b) => a.entryDate - b.entryDate)

  // Max concurrent
  const events = []
  for (const t of simTrades) {
    events.push({ time: t.entryDate, type: 1 })
    events.push({ time: t.exitDate || nowSec, type: -1 })
  }
  events.sort((a, b) => a.time - b.time || a.type - b.type)
  let conc = 0, maxConc = 0
  for (const e of events) { conc += e.type; if (conc > maxConc) maxConc = conc }

  const ek = maxConc * amount
  const gew = Math.round(totalProfit * 100) / 100
  const endk = Math.round((ek + gew) * 100) / 100
  const rendite = ek > 0 ? (gew / ek) * 100 : 0

  // CAGR
  const first = simTrades[0]?.entryDate || 0
  const last = simTrades.reduce((m, t) => Math.max(m, t.exitDate || nowSec), 0)
  const years = first > 0 ? (last - first) / (365 * 86400) : 0
  let cagr = 0
  if (ek > 0 && endk > 0 && years >= 0.1) cagr = (Math.pow(endk / ek, 1 / years) - 1) * 100
  else if (ek > 0) cagr = rendite

  const allW = simTrades.filter(t => t.profit > 0)
  const allL = simTrades.filter(t => t.profit < 0)
  const wr = simTrades.length > 0 ? (allW.length / simTrades.length) * 100 : 0
  const avgW = allW.length > 0 ? allW.reduce((s, t) => s + t.profit, 0) / allW.length : 0
  const avgL = allL.length > 0 ? Math.abs(allL.reduce((s, t) => s + t.profit, 0) / allL.length) : 0
  const avgWP = allW.length > 0 ? allW.reduce((s, t) => s + t.returnPct, 0) / allW.length : 0
  const avgLP = allL.length > 0 ? Math.abs(allL.reduce((s, t) => s + t.returnPct, 0) / allL.length) : 0
  const rr = avgL > 0 ? avgW / avgL : avgW > 0 ? Infinity : 0

  // Equity curve
  const curve = []
  if (simTrades.length) {
    const start = simTrades[0].entryDate
    const end = simTrades.reduce((m, t) => Math.max(m, t.exitDate || nowSec), 0)
    const DAY = 86400
    const days = Math.ceil((end - start) / DAY)
    const step = days > 1000 ? Math.ceil(days / 1000) * DAY : DAY
    const calc = (ts) => {
      let real = 0, unreal = 0
      for (const t of simTrades) {
        const exit = t.exitDate || nowSec
        if (ts >= exit) real += t.profit
        else if (ts >= t.entryDate) {
          const dur = exit - t.entryDate
          unreal += amount * (t.returnPct / 100) * (dur > 0 ? (ts - t.entryDate) / dur : 0)
        }
      }
      return Math.round((ek + real + unreal) * 100) / 100
    }
    for (let ts = start; ts <= end; ts += step) curve.push({ time: ts, value: calc(ts) })
    if (!curve.length || curve[curve.length - 1].time < end) curve.push({ time: end, value: calc(end) })
  }

  return {
    trades: simTrades, eigenkapital: ek, endkapital: endk, gewinn: gew, rendite, cagr, years,
    maxConcurrent: maxConc, amount, tradeCount: simTrades.length, openCount,
    winRate: wr, riskReward: rr, wins: allW.length, losses: allL.length,
    avgWin: avgW, avgLoss: avgL, avgWinPct: avgWP, avgLossPct: avgLP, equityCurve: curve,
  }
}

// ─── Test helpers ───

const NOW = 1738886400 // 2025-02-07 00:00 UTC (fixed "now")
const DAY = 86400
const MONTH = 30 * DAY
const YEAR = 365 * DAY

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

// ─── Test data generators ───

function makeTrade(mode, symbol, entryDate, exitDate, entryPrice, exitPrice, returnPct, status = 'CLOSED', extra = {}) {
  return {
    mode, symbol, name: symbol + ' Inc',
    entry_date: entryDate, exit_date: exitDate,
    entry_price: entryPrice, exit_price: exitPrice,
    current_price: status === 'OPEN' ? exitPrice : exitPrice,
    return_pct: returnPct, status,
    win_rate: 50, risk_reward: 1, avg_return: 0, market_cap: 100e9,
    ...extra,
  }
}

// Create realistic trades for each mode spanning 3+ years
function generateTestTrades() {
  const trades = []
  const base = NOW - 3 * YEAR - 60 * DAY // ~3.16 years ago

  // ── DEFENSIVE: 8 trades over 3 years, 5 wins 3 losses ──
  trades.push(makeTrade('defensive', 'AAPL', base, base + 3 * MONTH, 150, 165, 10))            // +10%
  trades.push(makeTrade('defensive', 'MSFT', base + 2 * MONTH, base + 5 * MONTH, 300, 285, -5))  // -5%
  trades.push(makeTrade('defensive', 'GOOG', base + 4 * MONTH, base + 8 * MONTH, 120, 132, 10))  // +10%
  trades.push(makeTrade('defensive', 'AMZN', base + 6 * MONTH, base + 10 * MONTH, 140, 154, 10)) // +10%
  trades.push(makeTrade('defensive', 'META', base + 12 * MONTH, base + 15 * MONTH, 320, 304, -5)) // -5%
  // These 3 are within 1 year of NOW:
  trades.push(makeTrade('defensive', 'NVDA', NOW - 11 * MONTH, NOW - 8 * MONTH, 500, 575, 15))    // +15%
  trades.push(makeTrade('defensive', 'TSLA', NOW - 9 * MONTH, NOW - 6 * MONTH, 200, 190, -5))     // -5%
  trades.push(makeTrade('defensive', 'JPM', NOW - 5 * MONTH, NOW - 2 * MONTH, 180, 198, 10))      // +10%

  // ── AGGRESSIVE: 6 trades, 3 wins 3 losses ──
  trades.push(makeTrade('aggressive', 'AMD', base, base + 2 * MONTH, 100, 125, 25))               // +25%
  trades.push(makeTrade('aggressive', 'COIN', base + 3 * MONTH, base + 5 * MONTH, 80, 60, -25))   // -25%
  trades.push(makeTrade('aggressive', 'SQ', base + 8 * MONTH, base + 11 * MONTH, 60, 75, 25))     // +25%
  // Within 1 year:
  trades.push(makeTrade('aggressive', 'PLTR', NOW - 10 * MONTH, NOW - 7 * MONTH, 20, 30, 50))     // +50%
  trades.push(makeTrade('aggressive', 'SNAP', NOW - 6 * MONTH, NOW - 4 * MONTH, 12, 9, -25))      // -25%
  trades.push(makeTrade('aggressive', 'ROKU', NOW - 3 * MONTH, NOW - 1 * MONTH, 80, 100, 25))     // +25%

  // ── QUANT: 10 trades, 7 wins 3 losses ──
  trades.push(makeTrade('quant', 'SPY', base, base + 1 * MONTH, 400, 420, 5))                     // +5%
  trades.push(makeTrade('quant', 'QQQ', base + 1 * MONTH, base + 2 * MONTH, 350, 357, 2))         // +2%
  trades.push(makeTrade('quant', 'IWM', base + 2 * MONTH, base + 3 * MONTH, 200, 194, -3))        // -3%
  trades.push(makeTrade('quant', 'DIA', base + 5 * MONTH, base + 6 * MONTH, 340, 350.2, 3))       // +3%
  trades.push(makeTrade('quant', 'VTI', base + 8 * MONTH, base + 9 * MONTH, 220, 224.4, 2))       // +2%
  trades.push(makeTrade('quant', 'XLF', base + 11 * MONTH, base + 12 * MONTH, 35, 33.95, -3))     // -3%
  trades.push(makeTrade('quant', 'XLE', base + 14 * MONTH, base + 15 * MONTH, 80, 83.2, 4))       // +4%
  // Within 1 year:
  trades.push(makeTrade('quant', 'GLD', NOW - 8 * MONTH, NOW - 6 * MONTH, 180, 185.4, 3))         // +3%
  trades.push(makeTrade('quant', 'SLV', NOW - 5 * MONTH, NOW - 3 * MONTH, 22, 21.56, -2))         // -2%
  trades.push(makeTrade('quant', 'TLT', NOW - 2 * MONTH, NOW - 1 * MONTH, 95, 98.8, 4))           // +4%

  // ── DITZ: 4 trades, 2 wins 2 losses ──
  trades.push(makeTrade('ditz', 'BABA', base + 2 * MONTH, base + 6 * MONTH, 90, 108, 20))         // +20%
  trades.push(makeTrade('ditz', 'NIO', base + 8 * MONTH, base + 12 * MONTH, 10, 8, -20))          // -20%
  // Within 1 year:
  trades.push(makeTrade('ditz', 'RIVN', NOW - 8 * MONTH, NOW - 4 * MONTH, 15, 18, 20))            // +20%
  trades.push(makeTrade('ditz', 'LCID', NOW - 3 * MONTH, NOW - 1 * MONTH, 5, 4, -20))             // -20%

  // ── TRADER: 5 trades, 3 wins 1 loss 1 open ──
  trades.push(makeTrade('trader', 'V', base, base + 4 * MONTH, 240, 264, 10))                     // +10%
  trades.push(makeTrade('trader', 'MA', base + 6 * MONTH, base + 10 * MONTH, 380, 361, -5))       // -5%
  trades.push(makeTrade('trader', 'PYPL', base + 12 * MONTH, base + 16 * MONTH, 70, 84, 20))      // +20%
  // Within 1 year:
  trades.push(makeTrade('trader', 'AXP', NOW - 7 * MONTH, NOW - 3 * MONTH, 200, 220, 10))         // +10%
  trades.push(makeTrade('trader', 'WMT', NOW - 2 * MONTH, null, 160, 168, 5, 'OPEN'))             // +5% open

  return trades
}

// ─── TESTS ───

const allTrades = generateTestTrades()

console.log('\n═══ Performance Calculation Tests ═══\n')

// ────────────────────────────────────────
console.log('── 1. cutoffDate ──')
// ────────────────────────────────────────

test('1y cutoff = now - 365*86400', () => {
  const cutoff = computeCutoffDate('1y', NOW)
  assert.equal(cutoff, NOW - YEAR)
})

test('3y cutoff = now - 3*365*86400', () => {
  const cutoff = computeCutoffDate('3y', NOW)
  assert.equal(cutoff, NOW - 3 * YEAR)
})

test('all cutoff = 0', () => {
  const cutoff = computeCutoffDate('all', NOW)
  assert.equal(cutoff, 0)
})

// ────────────────────────────────────────
console.log('\n── 2. modeData: DEFENSIVE 1y ──')
// ────────────────────────────────────────

{
  const cutoff1y = computeCutoffDate('1y', NOW)
  const data = computeModeData(allTrades, cutoff1y)
  const def = data.defensive.stats

  // Defensive 1y trades: NVDA(+15%), TSLA(-5%), JPM(+10%) = 3 trades
  test('defensive 1y: tradeCount = 3', () => assert.equal(def.tradeCount, 3))
  test('defensive 1y: wins = 2', () => assert.equal(def.wins, 2))
  test('defensive 1y: losses = 1', () => assert.equal(def.losses, 1))
  test('defensive 1y: winRate = 66.67%', () => approx(def.winRate, (2 / 3) * 100, 0.01))
  test('defensive 1y: totalReturn = 20%', () => approx(def.totalReturn, 15 + (-5) + 10, 0.01))
  test('defensive 1y: avgReturn = 6.67%', () => approx(def.avgReturn, 20 / 3, 0.01))

  // riskReward: avgWin% / avgLoss%
  // avgWin = (15 + 10) / 2 = 12.5, avgLoss = |(-5)| / 1 = 5
  test('defensive 1y: riskReward = 2.5', () => approx(def.riskReward, 12.5 / 5, 0.01))
}

// ────────────────────────────────────────
console.log('\n── 3. modeData: DEFENSIVE 3y ──')
// ────────────────────────────────────────

{
  const cutoff3y = computeCutoffDate('3y', NOW)
  const data = computeModeData(allTrades, cutoff3y)
  const def = data.defensive.stats

  // 3y: NVDA, TSLA, JPM + META, AMZN, GOOG, MSFT, AAPL
  // But base = NOW - 3*YEAR - 60*DAY, cutoff3y = NOW - 3*YEAR
  // AAPL entry = base = NOW - 3*YEAR - 60*DAY < cutoff3y → excluded
  // MSFT entry = base + 2*MONTH = base + 60*DAY = NOW - 3*YEAR → included (>=)
  // GOOG entry = base + 4*MONTH → included
  // AMZN entry = base + 6*MONTH → included
  // META entry = base + 12*MONTH → included
  // NVDA, TSLA, JPM → included
  // Total: 7 trades (MSFT, GOOG, AMZN, META, NVDA, TSLA, JPM)
  // Wins: GOOG(+10), AMZN(+10), NVDA(+15), JPM(+10) = 4
  // Losses: MSFT(-5), META(-5), TSLA(-5) = 3
  test('defensive 3y: tradeCount = 7', () => assert.equal(def.tradeCount, 7))
  test('defensive 3y: wins = 4', () => assert.equal(def.wins, 4))
  test('defensive 3y: losses = 3', () => assert.equal(def.losses, 3))
  test('defensive 3y: winRate = 57.14%', () => approx(def.winRate, (4 / 7) * 100, 0.01))
  test('defensive 3y: totalReturn = 30%', () => approx(def.totalReturn, -5 + 10 + 10 - 5 + 15 - 5 + 10, 0.01))
  test('defensive 3y: avgReturn = 4.29%', () => approx(def.avgReturn, 30 / 7, 0.01))

  // avgWin = (10+10+15+10)/4 = 11.25, avgLoss = (5+5+5)/3 = 5
  test('defensive 3y: riskReward = 2.25', () => approx(def.riskReward, 11.25 / 5, 0.01))
}

// ────────────────────────────────────────
console.log('\n── 4. modeData: AGGRESSIVE 1y ──')
// ────────────────────────────────────────

{
  const cutoff1y = computeCutoffDate('1y', NOW)
  const data = computeModeData(allTrades, cutoff1y)
  const agg = data.aggressive.stats

  // 1y trades: PLTR(+50%), SNAP(-25%), ROKU(+25%) = 3
  test('aggressive 1y: tradeCount = 3', () => assert.equal(agg.tradeCount, 3))
  test('aggressive 1y: wins = 2', () => assert.equal(agg.wins, 2))
  test('aggressive 1y: losses = 1', () => assert.equal(agg.losses, 1))
  test('aggressive 1y: winRate = 66.67%', () => approx(agg.winRate, (2 / 3) * 100, 0.01))
  test('aggressive 1y: totalReturn = 50%', () => approx(agg.totalReturn, 50 - 25 + 25, 0.01))
  test('aggressive 1y: avgReturn = 16.67%', () => approx(agg.avgReturn, 50 / 3, 0.01))
  // avgWin = (50+25)/2 = 37.5, avgLoss = 25
  test('aggressive 1y: riskReward = 1.5', () => approx(agg.riskReward, 37.5 / 25, 0.01))
}

// ────────────────────────────────────────
console.log('\n── 5. modeData: QUANT 1y ──')
// ────────────────────────────────────────

{
  const cutoff1y = computeCutoffDate('1y', NOW)
  const data = computeModeData(allTrades, cutoff1y)
  const q = data.quant.stats

  // 1y: GLD(+3%), SLV(-2%), TLT(+4%) = 3
  test('quant 1y: tradeCount = 3', () => assert.equal(q.tradeCount, 3))
  test('quant 1y: wins = 2', () => assert.equal(q.wins, 2))
  test('quant 1y: losses = 1', () => assert.equal(q.losses, 1))
  test('quant 1y: totalReturn = 5%', () => approx(q.totalReturn, 3 - 2 + 4, 0.01))
  test('quant 1y: avgReturn = 1.67%', () => approx(q.avgReturn, 5 / 3, 0.01))
  // avgWin = (3+4)/2 = 3.5, avgLoss = 2
  test('quant 1y: riskReward = 1.75', () => approx(q.riskReward, 3.5 / 2, 0.01))
}

// ────────────────────────────────────────
console.log('\n── 6. modeData: DITZ 1y ──')
// ────────────────────────────────────────

{
  const cutoff1y = computeCutoffDate('1y', NOW)
  const data = computeModeData(allTrades, cutoff1y)
  const d = data.ditz.stats

  // 1y: RIVN(+20%), LCID(-20%) = 2
  test('ditz 1y: tradeCount = 2', () => assert.equal(d.tradeCount, 2))
  test('ditz 1y: wins = 1', () => assert.equal(d.wins, 1))
  test('ditz 1y: losses = 1', () => assert.equal(d.losses, 1))
  test('ditz 1y: winRate = 50%', () => approx(d.winRate, 50, 0.01))
  test('ditz 1y: totalReturn = 0%', () => approx(d.totalReturn, 0, 0.01))
  test('ditz 1y: avgReturn = 0%', () => approx(d.avgReturn, 0, 0.01))
  // avgWin = 20, avgLoss = 20 → rr = 1.0
  test('ditz 1y: riskReward = 1.0', () => approx(d.riskReward, 1.0, 0.01))
}

// ────────────────────────────────────────
console.log('\n── 7. modeData: TRADER 1y ──')
// ────────────────────────────────────────

{
  const cutoff1y = computeCutoffDate('1y', NOW)
  const data = computeModeData(allTrades, cutoff1y)
  const tr = data.trader.stats

  // 1y: AXP(+10%), WMT(+5% OPEN) = 2
  test('trader 1y: tradeCount = 2', () => assert.equal(tr.tradeCount, 2))
  test('trader 1y: wins = 2', () => assert.equal(tr.wins, 2))
  test('trader 1y: losses = 0', () => assert.equal(tr.losses, 0))
  test('trader 1y: winRate = 100%', () => approx(tr.winRate, 100, 0.01))
  test('trader 1y: totalReturn = 15%', () => approx(tr.totalReturn, 15, 0.01))
  test('trader 1y: riskReward = Infinity (no losses)', () => assert.equal(tr.riskReward, Infinity))
}

// ────────────────────────────────────────
console.log('\n── 8. runSimForMode: DEFENSIVE 1y, 100$ per trade ──')
// ────────────────────────────────────────

{
  const cutoff1y = computeCutoffDate('1y', NOW)
  const data = computeModeData(allTrades, cutoff1y)
  const sim = runSimForMode(data.defensive.trades, '100', NOW)

  // Trades: NVDA(+15%), TSLA(-5%), JPM(+10%)
  // Profits: +15, -5, +10 = 20 total
  test('sim defensive 1y: not null', () => assert.ok(sim))
  test('sim defensive 1y: tradeCount = 3', () => assert.equal(sim.tradeCount, 3))
  test('sim defensive 1y: gewinn = 20', () => approx(sim.gewinn, 20, 0.01))

  // Max concurrent: all 3 trades don't overlap (sequential windows)
  // NVDA: NOW-11M to NOW-8M, TSLA: NOW-9M to NOW-6M, JPM: NOW-5M to NOW-2M
  // NVDA and TSLA overlap (NVDA -11M to -8M, TSLA -9M to -8M → overlap at -9M to -8M)
  // So maxConc should be 2
  test('sim defensive 1y: maxConcurrent = 2', () => assert.equal(sim.maxConcurrent, 2))

  // eigenkapital = 2 * 100 = 200
  test('sim defensive 1y: eigenkapital = 200', () => approx(sim.eigenkapital, 200, 0.01))

  // endkapital = 200 + 20 = 220
  test('sim defensive 1y: endkapital = 220', () => approx(sim.endkapital, 220, 0.01))

  // rendite = 20/200 * 100 = 10%
  test('sim defensive 1y: rendite = 10%', () => approx(sim.rendite, 10, 0.01))

  // CAGR: years = (last exit - first entry) / (365*86400)
  // first entry = NOW - 11*MONTH, last exit = NOW - 2*MONTH
  // duration = 9 * MONTH = 9 * 30 * 86400 = 23328000 sec
  // years = 23328000 / (365*86400) = 23328000 / 31536000 ≈ 0.7397
  const expYears = (9 * MONTH) / YEAR
  test('sim defensive 1y: years ≈ 0.74', () => approx(sim.years, expYears, 0.01))

  // CAGR = (220/200)^(1/0.7397) - 1 = 1.1^(1.352) - 1
  const expCAGR = (Math.pow(220 / 200, 1 / expYears) - 1) * 100
  test('sim defensive 1y: cagr correct', () => approx(sim.cagr, expCAGR, 0.1))

  // Win/Loss stats
  test('sim defensive 1y: wins = 2', () => assert.equal(sim.wins, 2))
  test('sim defensive 1y: losses = 1', () => assert.equal(sim.losses, 1))
  test('sim defensive 1y: winRate = 66.67%', () => approx(sim.winRate, (2 / 3) * 100, 0.01))

  // avgWin (absolute $): (15 + 10) / 2 = 12.5
  test('sim defensive 1y: avgWin = 12.5', () => approx(sim.avgWin, 12.5, 0.01))
  // avgLoss: |-5| / 1 = 5
  test('sim defensive 1y: avgLoss = 5', () => approx(sim.avgLoss, 5, 0.01))
  // riskReward (absolute): 12.5 / 5 = 2.5
  test('sim defensive 1y: riskReward = 2.5', () => approx(sim.riskReward, 2.5, 0.01))

  // avgWinPct: (15 + 10) / 2 = 12.5%
  test('sim defensive 1y: avgWinPct = 12.5', () => approx(sim.avgWinPct, 12.5, 0.01))
  // avgLossPct: |-5| / 1 = 5%
  test('sim defensive 1y: avgLossPct = 5', () => approx(sim.avgLossPct, 5, 0.01))

  // Equity curve
  test('sim defensive 1y: equityCurve has entries', () => assert.ok(sim.equityCurve.length > 0))
  test('sim defensive 1y: equityCurve starts at eigenkapital', () => approx(sim.equityCurve[0].value, sim.eigenkapital, 0.01))
  test('sim defensive 1y: equityCurve ends at endkapital', () => approx(sim.equityCurve[sim.equityCurve.length - 1].value, sim.endkapital, 0.01))
}

// ────────────────────────────────────────
console.log('\n── 9. runSimForMode: AGGRESSIVE 3y, 200$ per trade ──')
// ────────────────────────────────────────

{
  const cutoff3y = computeCutoffDate('3y', NOW)
  const data = computeModeData(allTrades, cutoff3y)
  const sim = runSimForMode(data.aggressive.trades, '200', NOW)

  // 3y trades: AMD entry=base, cutoff=NOW-3y, base=NOW-3y-60d → AMD excluded
  // COIN entry=base+3M → base+3M = NOW-3y-60d+90d = NOW-3y+30d → included
  // SQ entry=base+8M → included
  // PLTR, SNAP, ROKU → included
  // So: COIN(-25%), SQ(+25%), PLTR(+50%), SNAP(-25%), ROKU(+25%) = 5 trades
  test('sim aggressive 3y: not null', () => assert.ok(sim))
  test('sim aggressive 3y: tradeCount = 5', () => assert.equal(sim.tradeCount, 5))

  // Profits at 200$: COIN -50, SQ +50, PLTR +100, SNAP -50, ROKU +50 = 100 total
  test('sim aggressive 3y: gewinn = 100', () => approx(sim.gewinn, 100, 0.01))

  // All 5 trades are sequential (no overlaps based on dates)
  test('sim aggressive 3y: maxConcurrent = 1', () => assert.equal(sim.maxConcurrent, 1))

  // eigenkapital = 1 * 200 = 200
  test('sim aggressive 3y: eigenkapital = 200', () => approx(sim.eigenkapital, 200, 0.01))
  // endkapital = 200 + 100 = 300
  test('sim aggressive 3y: endkapital = 300', () => approx(sim.endkapital, 300, 0.01))
  // rendite = 100/200 * 100 = 50%
  test('sim aggressive 3y: rendite = 50%', () => approx(sim.rendite, 50, 0.01))

  // wins: PLTR(100), SQ(50), ROKU(50) = 3 wins
  test('sim aggressive 3y: wins = 3', () => assert.equal(sim.wins, 3))
  // losses: COIN(-50), SNAP(-50) = 2 losses
  test('sim aggressive 3y: losses = 2', () => assert.equal(sim.losses, 2))
  test('sim aggressive 3y: winRate = 60%', () => approx(sim.winRate, 60, 0.01))

  // avgWin$ = (50+100+50)/3 = 66.67
  test('sim aggressive 3y: avgWin = 66.67', () => approx(sim.avgWin, 200 / 3, 0.01))
  // avgLoss$ = (50+50)/2 = 50
  test('sim aggressive 3y: avgLoss = 50', () => approx(sim.avgLoss, 50, 0.01))
  // rr = 66.67/50 = 1.333
  test('sim aggressive 3y: riskReward = 1.33', () => approx(sim.riskReward, (200 / 3) / 50, 0.01))
}

// ────────────────────────────────────────
console.log('\n── 10. runSimForMode: QUANT 3y, 500$ ──')
// ────────────────────────────────────────

{
  const cutoff3y = computeCutoffDate('3y', NOW)
  const data = computeModeData(allTrades, cutoff3y)
  const sim = runSimForMode(data.quant.trades, '500', NOW)

  // 3y trades: SPY excluded (entry=base < cutoff)
  // QQQ entry=base+1M=NOW-3y-30d → excluded
  // IWM entry=base+2M=NOW-3y → included (>=)
  // DIA, VTI, XLF, XLE + GLD, SLV, TLT → all included
  // 8 trades total: IWM(-3), DIA(+3), VTI(+2), XLF(-3), XLE(+4), GLD(+3), SLV(-2), TLT(+4)
  test('sim quant 3y: not null', () => assert.ok(sim))
  test('sim quant 3y: tradeCount = 8', () => assert.equal(sim.tradeCount, 8))

  // Profits at 500$: IWM -15, DIA +15, VTI +10, XLF -15, XLE +20, GLD +15, SLV -10, TLT +20
  const expectedGewinn = -15 + 15 + 10 - 15 + 20 + 15 - 10 + 20
  test('sim quant 3y: gewinn = 40', () => approx(sim.gewinn, expectedGewinn, 0.01))

  // All sequential (no overlaps) → maxConc = 1
  test('sim quant 3y: maxConcurrent = 1', () => assert.equal(sim.maxConcurrent, 1))
  test('sim quant 3y: eigenkapital = 500', () => approx(sim.eigenkapital, 500, 0.01))
  test('sim quant 3y: endkapital = 540', () => approx(sim.endkapital, 540, 0.01))
  test('sim quant 3y: rendite = 8%', () => approx(sim.rendite, 8, 0.01))

  // wins: DIA(15), VTI(10), XLE(20), GLD(15), TLT(20) = 5
  test('sim quant 3y: wins = 5', () => assert.equal(sim.wins, 5))
  // losses: IWM(-15), XLF(-15), SLV(-10) = 3
  test('sim quant 3y: losses = 3', () => assert.equal(sim.losses, 3))
}

// ────────────────────────────────────────
console.log('\n── 11. runSimForMode: TRADER 1y with OPEN position ──')
// ────────────────────────────────────────

{
  const cutoff1y = computeCutoffDate('1y', NOW)
  const data = computeModeData(allTrades, cutoff1y)
  const sim = runSimForMode(data.trader.trades, '100', NOW)

  // AXP(+10%), WMT(+5% OPEN)
  test('sim trader 1y: tradeCount = 2', () => assert.equal(sim.tradeCount, 2))
  test('sim trader 1y: openCount = 1', () => assert.equal(sim.openCount, 1))

  // Profits: AXP +10, WMT +5 = 15 total
  test('sim trader 1y: gewinn = 15', () => approx(sim.gewinn, 15, 0.01))

  // AXP: NOW-7M to NOW-3M, WMT: NOW-2M to NOW (open)
  // No overlap → maxConc = 1
  test('sim trader 1y: maxConcurrent = 1', () => assert.equal(sim.maxConcurrent, 1))
  test('sim trader 1y: eigenkapital = 100', () => approx(sim.eigenkapital, 100, 0.01))
  test('sim trader 1y: endkapital = 115', () => approx(sim.endkapital, 115, 0.01))
  test('sim trader 1y: rendite = 15%', () => approx(sim.rendite, 15, 0.01))

  // Only wins
  test('sim trader 1y: wins = 2, losses = 0', () => {
    assert.equal(sim.wins, 2)
    assert.equal(sim.losses, 0)
  })
  test('sim trader 1y: riskReward = Infinity', () => assert.equal(sim.riskReward, Infinity))
}

// ────────────────────────────────────────
console.log('\n── 12. runSimForMode: DITZ 1y, 1000$ ──')
// ────────────────────────────────────────

{
  const cutoff1y = computeCutoffDate('1y', NOW)
  const data = computeModeData(allTrades, cutoff1y)
  const sim = runSimForMode(data.ditz.trades, '1000', NOW)

  // RIVN(+20%), LCID(-20%)
  test('sim ditz 1y: tradeCount = 2', () => assert.equal(sim.tradeCount, 2))
  // Profits: +200, -200 = 0
  test('sim ditz 1y: gewinn = 0', () => approx(sim.gewinn, 0, 0.01))
  test('sim ditz 1y: rendite = 0%', () => approx(sim.rendite, 0, 0.01))
  test('sim ditz 1y: avgWin = 200', () => approx(sim.avgWin, 200, 0.01))
  test('sim ditz 1y: avgLoss = 200', () => approx(sim.avgLoss, 200, 0.01))
  test('sim ditz 1y: riskReward = 1.0', () => approx(sim.riskReward, 1.0, 0.01))
}

// ────────────────────────────────────────
console.log('\n── 13. Edge cases ──')
// ────────────────────────────────────────

test('empty trades → null sim', () => {
  assert.equal(runSimForMode([], '100', NOW), null)
})

test('zero amount → null sim', () => {
  const trades = [makeTrade('defensive', 'X', NOW - MONTH, NOW, 100, 110, 10)]
  assert.equal(runSimForMode(trades, '0', NOW), null)
})

test('negative amount → null sim', () => {
  const trades = [makeTrade('defensive', 'X', NOW - MONTH, NOW, 100, 110, 10)]
  assert.equal(runSimForMode(trades, '-50', NOW), null)
})

test('single trade sim is correct', () => {
  const trades = [makeTrade('defensive', 'X', NOW - 6 * MONTH, NOW - 3 * MONTH, 100, 120, 20)]
  const sim = runSimForMode(trades, '100', NOW)
  assert.equal(sim.tradeCount, 1)
  approx(sim.gewinn, 20, 0.01)
  approx(sim.eigenkapital, 100, 0.01)
  approx(sim.endkapital, 120, 0.01)
  approx(sim.rendite, 20, 0.01)
  assert.equal(sim.maxConcurrent, 1)
  assert.equal(sim.wins, 1)
  assert.equal(sim.losses, 0)
})

test('all losses scenario', () => {
  const trades = [
    makeTrade('defensive', 'A', NOW - 6 * MONTH, NOW - 5 * MONTH, 100, 90, -10),
    makeTrade('defensive', 'B', NOW - 4 * MONTH, NOW - 3 * MONTH, 100, 85, -15),
  ]
  const sim = runSimForMode(trades, '100', NOW)
  approx(sim.gewinn, -25, 0.01)
  assert.equal(sim.wins, 0)
  assert.equal(sim.losses, 2)
  approx(sim.winRate, 0, 0.01)
  approx(sim.riskReward, 0, 0.01) // no wins → rr = 0
})

test('concurrent positions increase eigenkapital', () => {
  // Two trades that fully overlap
  const trades = [
    makeTrade('defensive', 'A', NOW - 6 * MONTH, NOW - 3 * MONTH, 100, 110, 10),
    makeTrade('defensive', 'B', NOW - 6 * MONTH, NOW - 3 * MONTH, 50, 55, 10),
  ]
  const sim = runSimForMode(trades, '100', NOW)
  assert.equal(sim.maxConcurrent, 2)
  approx(sim.eigenkapital, 200, 0.01)
  approx(sim.gewinn, 20, 0.01) // 10 + 10
  approx(sim.endkapital, 220, 0.01)
  approx(sim.rendite, 10, 0.01) // 20/200 * 100
})

test('CAGR uses rendite when years < 0.1', () => {
  // Trade spanning only ~1 week
  const trades = [makeTrade('defensive', 'X', NOW - 7 * DAY, NOW, 100, 105, 5)]
  const sim = runSimForMode(trades, '100', NOW)
  // years = 7*DAY / YEAR ≈ 0.019 < 0.1
  approx(sim.years, 7 * DAY / YEAR, 0.01)
  approx(sim.cagr, sim.rendite, 0.01) // falls back to rendite
})

test('profit rounding: amount * pct / 100 rounded to 2 decimals', () => {
  // 100 * 7.3 / 100 = 7.3 → rounded = 7.3
  const trades = [makeTrade('defensive', 'X', NOW - MONTH, NOW, 100, 107.3, 7.3)]
  const sim = runSimForMode(trades, '100', NOW)
  assert.equal(sim.trades[0].profit, 7.3)
  assert.equal(sim.trades[0].received, 107.3)
})

test('trade with 0% return counts as neither win nor loss', () => {
  const trades = [makeTrade('defensive', 'X', NOW - MONTH, NOW, 100, 100, 0)]
  const data = computeModeData(trades, 0)
  assert.equal(data.defensive.stats.wins, 0)
  assert.equal(data.defensive.stats.losses, 0)
  assert.equal(data.defensive.stats.tradeCount, 1)

  const sim = runSimForMode(trades, '100', NOW)
  assert.equal(sim.wins, 0)
  assert.equal(sim.losses, 0)
  approx(sim.gewinn, 0, 0.01)
})

test('mode isolation: trades only appear in their mode', () => {
  const mixed = [
    makeTrade('defensive', 'A', NOW - MONTH, NOW, 100, 110, 10),
    makeTrade('aggressive', 'B', NOW - MONTH, NOW, 100, 120, 20),
    makeTrade('quant', 'C', NOW - MONTH, NOW, 100, 105, 5),
  ]
  const data = computeModeData(mixed, 0)
  assert.equal(data.defensive.stats.tradeCount, 1)
  assert.equal(data.aggressive.stats.tradeCount, 1)
  assert.equal(data.quant.stats.tradeCount, 1)
  assert.equal(data.ditz.stats.tradeCount, 0)
  assert.equal(data.trader.stats.tradeCount, 0)
})

// ────────────────────────────────────────
console.log('\n── 14. Filter tests ──')
// ────────────────────────────────────────

test('minWinrate filter excludes low win_rate trades', () => {
  const trades = [
    makeTrade('defensive', 'A', NOW - MONTH, NOW, 100, 110, 10, 'CLOSED', { win_rate: 60 }),
    makeTrade('defensive', 'B', NOW - MONTH, NOW, 100, 105, 5, 'CLOSED', { win_rate: 30 }),
  ]
  const data = computeModeData(trades, 0, { minWinrate: '50' })
  assert.equal(data.defensive.stats.tradeCount, 1)
  approx(data.defensive.stats.totalReturn, 10, 0.01)
})

test('minMarketCap filter (in Mrd/billions)', () => {
  const trades = [
    makeTrade('defensive', 'A', NOW - MONTH, NOW, 100, 110, 10, 'CLOSED', { market_cap: 50e9 }),
    makeTrade('defensive', 'B', NOW - MONTH, NOW, 100, 105, 5, 'CLOSED', { market_cap: 5e9 }),
  ]
  const data = computeModeData(trades, 0, { minMarketCap: '10' })
  assert.equal(data.defensive.stats.tradeCount, 1)
})

// ────────────────────────────────────────
console.log('\n── 15. Equity curve verification ──')
// ────────────────────────────────────────

test('equity curve monotonically sorted by time', () => {
  const cutoff1y = computeCutoffDate('1y', NOW)
  const data = computeModeData(allTrades, cutoff1y)
  const sim = runSimForMode(data.defensive.trades, '100', NOW)
  for (let i = 1; i < sim.equityCurve.length; i++) {
    assert.ok(sim.equityCurve[i].time >= sim.equityCurve[i - 1].time,
      `Curve not sorted at index ${i}`)
  }
})

test('equity curve final value = endkapital for closed trades', () => {
  // All closed, no open positions
  const trades = [
    makeTrade('defensive', 'A', NOW - 6 * MONTH, NOW - 4 * MONTH, 100, 110, 10),
    makeTrade('defensive', 'B', NOW - 3 * MONTH, NOW - 1 * MONTH, 100, 95, -5),
  ]
  const sim = runSimForMode(trades, '100', NOW)
  const lastVal = sim.equityCurve[sim.equityCurve.length - 1].value
  approx(lastVal, sim.endkapital, 0.01)
})

// ────────────────────────────────────────
console.log('\n── 16. CAGR calculation verification ──')
// ────────────────────────────────────────

test('CAGR for exactly 1 year, 10% return → CAGR = 10%', () => {
  const trades = [makeTrade('defensive', 'X', NOW - YEAR, NOW, 100, 110, 10)]
  const sim = runSimForMode(trades, '100', NOW)
  approx(sim.years, 1.0, 0.01)
  // CAGR = (110/100)^(1/1) - 1 = 10%
  approx(sim.cagr, 10, 0.1)
})

test('CAGR for 2 years, 21% total return', () => {
  const trades = [
    makeTrade('defensive', 'A', NOW - 2 * YEAR, NOW - YEAR, 100, 110, 10),
    makeTrade('defensive', 'B', NOW - YEAR + DAY, NOW, 100, 111, 11),
  ]
  const sim = runSimForMode(trades, '100', NOW)
  // maxConc could be 1 (sequential), ek = 100
  // gewinn = 10 + 11 = 21, endk = 121
  // years ≈ 2, CAGR = (121/100)^(1/2) - 1 = 1.1^1 - 1... wait
  // (121/100)^(0.5) - 1 = 1.1 - 1 = 0.1 = 10%
  approx(sim.cagr, 10, 0.5)
})

// ────────────────────────────────────────
// Summary
// ────────────────────────────────────────

console.log('\n═══════════════════════════════════════')
console.log(`Results: ${passed} passed, ${failed} failed out of ${passed + failed} tests`)
if (errors.length > 0) {
  console.log('\nFailed tests:')
  for (const e of errors) console.log(`  ✗ ${e.name}: ${e.error}`)
}
console.log('═══════════════════════════════════════\n')

process.exit(failed > 0 ? 1 : 0)
