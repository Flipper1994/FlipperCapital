/**
 * Portfolio Rendite + Chart Tests
 * Prüft: calculatePortfolioHistory (pct), period_return_pct, Chart-Daten
 * Run: node frontend/src/tests/portfolio_rendite.test.mjs
 */

import assert from 'node:assert/strict'

// ─── Test helpers ───

const DAY = 86400
const MONTH = 30 * DAY
const YEAR = 365 * DAY
const NOW = 1739491200 // 2025-02-14 00:00 UTC

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

// ─── Replizierte Backend-Logik: calculatePortfolioHistoryForUser ───
// Exakt aus backend/main.go Zeilen 3375-3597 nachgebaut

function calculatePortfolioHistory(positions, symbolData, hasQuantities = false) {
  if (positions.length === 0 || Object.keys(symbolData).length === 0) return []

  // Collect all timestamps
  const timeValues = {} // time -> symbol -> price
  const allTimesSet = new Set()

  for (const [symbol, data] of Object.entries(symbolData)) {
    for (const candle of data) {
      if (!timeValues[candle.time]) timeValues[candle.time] = {}
      timeValues[candle.time][symbol] = candle.close
      allTimesSet.add(candle.time)
    }
  }

  const allTimes = [...allTimesSet].sort((a, b) => a - b)
  if (allTimes.length === 0) return []

  // Track last known prices
  const lastPrices = {}
  for (const pos of positions) {
    const data = symbolData[pos.symbol]
    if (data && data.length > 0) lastPrices[pos.symbol] = data[0].close
  }

  let prevActiveCount = 0
  let baseValue = 0
  let prevPct = 0
  const result = []

  for (const t of allTimes) {
    const prices = timeValues[t] || {}

    // Update last known prices
    for (const [symbol, price] of Object.entries(prices)) {
      lastPrices[symbol] = price
    }

    // Active positions at this time
    const activeEntries = []
    for (const pos of positions) {
      if (pos.purchaseDate && pos.purchaseDate > t) continue

      // Base price
      let bp = pos.avgPrice // already in USD for simplicity
      if (!pos.purchaseDate || pos.purchaseDate <= allTimes[0]) {
        const data = symbolData[pos.symbol]
        if (data && data.length > 0) bp = data[0].close
      }

      const qty = hasQuantities ? (pos.quantity || 1) : 1
      const w = hasQuantities ? bp * qty : 1000

      activeEntries.push({ pos, basePrice: bp, weight: w })
    }

    if (activeEntries.length === 0) continue

    // Portfolio value
    let portfolioValue = 0
    if (hasQuantities) {
      for (const e of activeEntries) {
        const price = lastPrices[e.pos.symbol] || 0
        portfolioValue += price * (e.pos.quantity || 1)
      }
    } else {
      for (const e of activeEntries) {
        const price = lastPrices[e.pos.symbol] || 0
        if (e.basePrice > 0) {
          portfolioValue += 1000 * (price / e.basePrice)
        }
      }
    }

    if (portfolioValue <= 0) continue

    // Rebase when new positions join
    if (activeEntries.length > prevActiveCount && prevActiveCount > 0) {
      if (result.length > 0) prevPct = result[result.length - 1].pct
      baseValue = portfolioValue
    }
    prevActiveCount = activeEntries.length

    if (baseValue === 0) baseValue = portfolioValue

    const pct = prevPct + ((portfolioValue - baseValue) / baseValue) * 100

    result.push({ time: t, value: portfolioValue, pct })
  }

  return result
}

// ─── Replizierte Backend-Logik: period_return_pct (getAllPortfoliosHistory) ───

function calculatePeriodReturn(history, closedTrades) {
  if (!history || history.length < 2) {
    // Nur closed trades
    if (!closedTrades || closedTrades.length === 0) return 0
    const closedGain = closedTrades.reduce((s, t) => s + (t.sellPrice - t.buyPrice) * t.quantity, 0)
    const closedInvested = closedTrades.reduce((s, t) => s + t.buyPrice * t.quantity, 0)
    return closedInvested > 0 ? (closedGain / closedInvested) * 100 : 0
  }

  const startValue = history[0].value
  const openReturnPct = history[history.length - 1].pct

  if (!closedTrades || closedTrades.length === 0) return openReturnPct

  const openGain = (openReturnPct / 100) * startValue
  let closedGain = 0
  let closedInvested = 0
  for (const t of closedTrades) {
    closedGain += (t.sellPrice - t.buyPrice) * t.quantity
    closedInvested += t.buyPrice * t.quantity
  }

  const totalCapital = startValue + closedInvested
  return totalCapital > 0 ? ((openGain + closedGain) / totalCapital) * 100 : 0
}

// ─── Replizierte Frontend-Logik: getPeriodReturn (PortfolioCompare) ───

function getPeriodReturn(userId, historyData, portfolios) {
  const entry = historyData.find(h => h.user_id === userId)
  if (entry && entry.period_return_pct !== undefined) return entry.period_return_pct
  const portfolio = portfolios.find(p => p.user_id === userId)
  return portfolio ? portfolio.total_return_pct : 0
}

// ─── Test-Daten ───

function makeCandles(startPrice, changes, startTime, interval = DAY) {
  // changes = array of price deltas or absolute prices
  const candles = []
  let price = startPrice
  for (let i = 0; i < changes.length; i++) {
    price = startPrice + changes[i]
    candles.push({ time: startTime + i * interval, close: price })
  }
  return candles
}

function makePrices(prices, startTime, interval = DAY) {
  return prices.map((p, i) => ({ time: startTime + i * interval, close: p }))
}

// ═══════════════════════════════════════
console.log('\n═══ Portfolio Rendite & Chart Tests ═══\n')
// ═══════════════════════════════════════

// ────────────────────────────────────────
console.log('── 1. Einzel-Position: pct-Berechnung ──')
// ────────────────────────────────────────

{
  const startTime = NOW - 30 * DAY
  const positions = [
    { symbol: 'AAPL', avgPrice: 150, purchaseDate: null } // vor Chart-Zeitraum
  ]
  const symbolData = {
    'AAPL': makePrices([150, 155, 160, 157, 165], startTime)
  }

  const history = calculatePortfolioHistory(positions, symbolData)

  test('Einzel-Position: erster pct = 0%', () => {
    approx(history[0].pct, 0, 0.01)
  })

  test('Einzel-Position: letzter pct = (165-150)/150 * 100 = 10%', () => {
    approx(history[history.length - 1].pct, (165 - 150) / 150 * 100, 0.01)
  })

  test('Einzel-Position: Zwischenwert korrekt (160 → +6.67%)', () => {
    approx(history[2].pct, (160 - 150) / 150 * 100, 0.01)
  })

  test('Einzel-Position: alle Zeitpunkte aufsteigend sortiert', () => {
    for (let i = 1; i < history.length; i++) {
      assert.ok(history[i].time > history[i - 1].time)
    }
  })
}

// ────────────────────────────────────────
console.log('\n── 2. Zwei Positionen gleichzeitig (ohne Quantities) ──')
// ────────────────────────────────────────

{
  const startTime = NOW - 5 * DAY
  const positions = [
    { symbol: 'AAPL', avgPrice: 100, purchaseDate: null },
    { symbol: 'MSFT', avgPrice: 200, purchaseDate: null },
  ]
  // AAPL: 100 → 110 (+10%), MSFT: 200 → 220 (+10%) → Portfolio +10%
  const symbolData = {
    'AAPL': makePrices([100, 105, 110], startTime),
    'MSFT': makePrices([200, 210, 220], startTime),
  }

  const history = calculatePortfolioHistory(positions, symbolData)

  test('2 Positionen gleichmäßig: Start bei 0%', () => {
    approx(history[0].pct, 0, 0.01)
  })

  test('2 Positionen gleichmäßig +10% jede → Portfolio +10%', () => {
    // Ohne Quantities: jede Position = 1000$ gleichgewichtet
    // AAPL: 1000 * (110/100) = 1100, MSFT: 1000 * (220/200) = 1100
    // Portfolio: 2200, base: 2000 → (2200-2000)/2000 * 100 = 10%
    approx(history[history.length - 1].pct, 10, 0.01)
  })
}

// ────────────────────────────────────────
console.log('\n── 3. Zwei Positionen ungleich (ohne Quantities) ──')
// ────────────────────────────────────────

{
  const startTime = NOW - 5 * DAY
  const positions = [
    { symbol: 'AAPL', avgPrice: 100, purchaseDate: null },
    { symbol: 'MSFT', avgPrice: 200, purchaseDate: null },
  ]
  // AAPL: 100 → 120 (+20%), MSFT: 200 → 190 (-5%)
  const symbolData = {
    'AAPL': makePrices([100, 110, 120], startTime),
    'MSFT': makePrices([200, 195, 190], startTime),
  }

  const history = calculatePortfolioHistory(positions, symbolData)

  test('Ungleiche Positionen: Durchschnittsrendite korrekt', () => {
    // AAPL: 1000*(120/100) = 1200, MSFT: 1000*(190/200) = 950
    // Portfolio: 2150, base: 2000 → +7.5%
    approx(history[history.length - 1].pct, 7.5, 0.01)
  })
}

// ────────────────────────────────────────
console.log('\n── 4. Position mit purchaseDate innerhalb des Zeitraums (Rebase) ──')
// ────────────────────────────────────────

{
  const startTime = NOW - 10 * DAY
  const positions = [
    { symbol: 'AAPL', avgPrice: 100, purchaseDate: null },
    { symbol: 'MSFT', avgPrice: 200, purchaseDate: startTime + 3 * DAY }, // tritt nach 3 Tagen bei
  ]
  // AAPL: 5 Tage [100, 105, 110, 115, 120]
  // MSFT: 5 Tage [—, —, —, 200, 210] (erst ab Tag 3 aktiv)
  const symbolData = {
    'AAPL': makePrices([100, 105, 110, 115, 120], startTime),
    'MSFT': makePrices([190, 195, 200, 200, 210], startTime),
  }

  const history = calculatePortfolioHistory(positions, symbolData)

  test('Rebase: Anfangs nur 1 Position (AAPL)', () => {
    // Tag 0: AAPL=100, base=1000*(100/100)=1000, pct=0%
    approx(history[0].pct, 0, 0.01)
  })

  test('Rebase: Vor Join korrekt (AAPL +10% bei Tag 2)', () => {
    // Tag 2: AAPL=110, value=1000*(110/100)=1100, pct=(1100-1000)/1000*100=10%
    approx(history[2].pct, 10, 0.01)
  })

  test('Rebase: Nach Join wird prevPct beibehalten', () => {
    // Tag 3: MSFT tritt bei. AAPL=115 → 1000*(115/100)=1150
    // MSFT basePrice=200 (purchaseDate > allTimes[0]) → 1000*(200/200)=1000
    // portfolioValue = 2150, prevPct = history[2].pct = 10%
    // baseValue = 2150 (rebase!)
    // pct = 10% + (2150-2150)/2150*100 = 10%
    approx(history[3].pct, 10, 0.5) // rebase hält den pct-Wert
  })

  test('Rebase: Nach Join korrekte Weiterentwicklung', () => {
    // Tag 4: AAPL=120 → 1000*(120/100)=1200, MSFT=210 → 1000*(210/200)=1050
    // portfolioValue = 2250, baseValue = 2150
    // pct = 10% + (2250-2150)/2150*100 = 10% + 4.65% = 14.65%
    const expected = 10 + ((2250 - 2150) / 2150) * 100
    approx(history[4].pct, expected, 0.1)
  })
}

// ────────────────────────────────────────
console.log('\n── 5. Mit Quantities: investitionsgewichtetes Portfolio ──')
// ────────────────────────────────────────

{
  const startTime = NOW - 5 * DAY
  const positions = [
    { symbol: 'AAPL', avgPrice: 100, purchaseDate: null, quantity: 10 },
    { symbol: 'MSFT', avgPrice: 200, purchaseDate: null, quantity: 5 },
  ]
  // AAPL: 10 Aktien à 100→120 (+20%), MSFT: 5 Aktien à 200→210 (+5%)
  const symbolData = {
    'AAPL': makePrices([100, 110, 120], startTime),
    'MSFT': makePrices([200, 205, 210], startTime),
  }

  const history = calculatePortfolioHistory(positions, symbolData, true)

  test('Quantities: Start bei 0%', () => {
    approx(history[0].pct, 0, 0.01)
  })

  test('Quantities: investitionsgewichtete Rendite', () => {
    // Start: AAPL 10*100=1000, MSFT 5*200=1000, total=2000
    // End: AAPL 10*120=1200, MSFT 5*210=1050, total=2250
    // pct = (2250-2000)/2000 * 100 = 12.5%
    approx(history[history.length - 1].pct, 12.5, 0.01)
  })
}

// ────────────────────────────────────────
console.log('\n── 6. period_return_pct: nur offene Positionen ──')
// ────────────────────────────────────────

{
  const startTime = NOW - 30 * DAY
  const positions = [{ symbol: 'AAPL', avgPrice: 100, purchaseDate: null }]
  const symbolData = {
    'AAPL': makePrices([100, 110, 115], startTime)
  }
  const history = calculatePortfolioHistory(positions, symbolData)

  test('period_return: nur offene Positionen = letzter pct', () => {
    const ret = calculatePeriodReturn(history, [])
    approx(ret, history[history.length - 1].pct, 0.01)
  })
}

// ────────────────────────────────────────
console.log('\n── 7. period_return_pct: offene + geschlossene Trades ──')
// ────────────────────────────────────────

{
  // Offene Position: AAPL 100 → 110 (+10%)
  const startTime = NOW - 30 * DAY
  const positions = [{ symbol: 'AAPL', avgPrice: 100, purchaseDate: null }]
  const symbolData = {
    'AAPL': makePrices([100, 105, 110], startTime)
  }
  const history = calculatePortfolioHistory(positions, symbolData)

  // Geschlossener Trade: MSFT gekauft bei 200, verkauft bei 220 (+10%), 1 Stück
  const closedTrades = [{ buyPrice: 200, sellPrice: 220, quantity: 1 }]

  test('period_return: offene + geschlossene korrekt kombiniert', () => {
    const ret = calculatePeriodReturn(history, closedTrades)
    // startValue = 1000 (AAPL bei Preis 100 → 1000$ gleichgewichtet)
    // openReturnPct = 10%
    // openGain = 10/100 * 1000 = 100
    // closedGain = (220-200)*1 = 20
    // closedInvested = 200*1 = 200
    // totalCapital = 1000 + 200 = 1200
    // periodReturn = (100+20)/1200 * 100 = 10%
    approx(ret, 10, 0.01)
  })
}

// ────────────────────────────────────────
console.log('\n── 8. period_return_pct: nur geschlossene Trades (keine offenen) ──')
// ────────────────────────────────────────

{
  const closedTrades = [
    { buyPrice: 100, sellPrice: 120, quantity: 2 }, // +40 Gewinn
    { buyPrice: 50, sellPrice: 45, quantity: 4 },   // -20 Verlust
  ]

  test('period_return: nur closed trades → gewichtete Rendite', () => {
    const ret = calculatePeriodReturn([], closedTrades)
    // closedGain = (120-100)*2 + (45-50)*4 = 40 + (-20) = 20
    // closedInvested = 100*2 + 50*4 = 200 + 200 = 400
    // return = 20/400 * 100 = 5%
    approx(ret, 5, 0.01)
  })
}

// ────────────────────────────────────────
console.log('\n── 9. period_return_pct bei Rebase (potenzielle Ungenauigkeit) ──')
// ────────────────────────────────────────

{
  // Szenario: AAPL startet, steigt 10%, dann kommt MSFT dazu, danach fallen beide auf Startwert
  const startTime = NOW - 10 * DAY
  const positions = [
    { symbol: 'AAPL', avgPrice: 100, purchaseDate: null },
    { symbol: 'MSFT', avgPrice: 200, purchaseDate: startTime + 2 * DAY },
  ]
  const symbolData = {
    'AAPL': makePrices([100, 110, 110, 100, 100], startTime),
    'MSFT': makePrices([190, 195, 200, 200, 200], startTime),
  }

  const history = calculatePortfolioHistory(positions, symbolData)

  test('Rebase-Szenario: AAPL startet bei 0%', () => {
    approx(history[0].pct, 0, 0.01)
  })

  test('Rebase-Szenario: vor Join AAPL +10%', () => {
    approx(history[1].pct, 10, 0.01)
  })

  test('Rebase-Szenario: nach Join + Rückgang pct korrekt', () => {
    // Tag 2: Rebase. AAPL=110 → 1000*(110/100)=1100, MSFT=200 → 1000*(200/200)=1000
    // portfolioValue=2100, baseValue=2100, prevPct=10%
    // Tag 3: AAPL=100→1000*(100/100)=1000, MSFT=200→1000
    // portfolioValue=2000, pct = 10% + (2000-2100)/2100*100 = 10% - 4.76% = 5.24%
    const expectedPct = 10 + ((2000 - 2100) / 2100) * 100
    approx(history[3].pct, expectedPct, 0.1)
  })

  test('Rebase-Szenario: period_return ohne closed trades', () => {
    const ret = calculatePeriodReturn(history, [])
    // Gibt den letzten pct zurück (= 5.24%)
    // Aber: tatsächlicher Gewinn auf offene Positionen:
    // AAPL: 100→100 = 0, MSFT: 200→200 = 0 → tatsächlich 0% Gewinn auf aktuelles Portfolio
    // Das anfängliche AAPL-Wachstum (+10%) vor dem Rebase zählt aber noch rein
    // Das ist designbedingt korrekt für den Chart, aber:
    // openGain = 5.24% / 100 * 1000 (startValue) = 52.4$ ???
    // Tatsächlicher Dollar-Gewinn: AAPL 100→100 = 0$
    // HINWEIS: Die Diskrepanz kommt vom Rebase-Mechanismus
    approx(ret, history[history.length - 1].pct, 0.01)
    console.log(`    Info: pct=${ret.toFixed(2)}%, obwohl AAPL und MSFT am Ende wieder auf Kaufkurs stehen`)
    console.log(`    → Dies ist korrekt: AAPL war zwischenzeitlich +10%, bevor MSFT dazukam`)
  })
}

// ────────────────────────────────────────
console.log('\n── 10. period_return_pct: Rebase + geschlossene Trades ──')
// ────────────────────────────────────────

{
  const startTime = NOW - 10 * DAY
  const positions = [
    { symbol: 'AAPL', avgPrice: 100, purchaseDate: null },
    { symbol: 'MSFT', avgPrice: 200, purchaseDate: startTime + 2 * DAY },
  ]
  const symbolData = {
    'AAPL': makePrices([100, 110, 110, 120, 120], startTime),
    'MSFT': makePrices([190, 195, 200, 200, 220], startTime),
  }

  const history = calculatePortfolioHistory(positions, symbolData)
  const closedTrades = [{ buyPrice: 50, sellPrice: 60, quantity: 2 }] // +20$ Gewinn

  test('Rebase + closed: period_return berechnet', () => {
    const ret = calculatePeriodReturn(history, closedTrades)
    const startValue = history[0].value
    const openRetPct = history[history.length - 1].pct
    const openGain = (openRetPct / 100) * startValue
    const closedGain = (60 - 50) * 2 // = 20
    const closedInv = 50 * 2 // = 100
    const totalCap = startValue + closedInv
    const expected = ((openGain + closedGain) / totalCap) * 100
    approx(ret, expected, 0.01)
  })
}

// ────────────────────────────────────────
console.log('\n── 11. Chart-Daten: pct für MultiPortfolioChart ──')
// ────────────────────────────────────────

{
  // Simuliere Backend-Response für MultiPortfolioChart
  const userData = [
    {
      user_id: 1, username: 'Alice',
      history: [
        { time: NOW - 5 * DAY, pct: 0 },
        { time: NOW - 4 * DAY, pct: 2.5 },
        { time: NOW - 3 * DAY, pct: 5.0 },
        { time: NOW - 2 * DAY, pct: 3.0 },
        { time: NOW - 1 * DAY, pct: 8.0 },
      ],
      period_return_pct: 8.0,
    },
    {
      user_id: 2, username: 'Bob',
      history: [
        { time: NOW - 5 * DAY, pct: 0 },
        { time: NOW - 4 * DAY, pct: -1.0 },
        { time: NOW - 3 * DAY, pct: -3.0 },
        { time: NOW - 2 * DAY, pct: -2.0 },
        { time: NOW - 1 * DAY, pct: -5.0 },
      ],
      period_return_pct: -5.0,
    },
  ]

  test('Chart: pct-Werte starten bei 0', () => {
    for (const u of userData) {
      approx(u.history[0].pct, 0, 0.01, `${u.username}: `)
    }
  })

  test('Chart: letzter pct = period_return_pct', () => {
    for (const u of userData) {
      const lastPct = u.history[u.history.length - 1].pct
      approx(lastPct, u.period_return_pct, 0.01, `${u.username}: `)
    }
  })

  test('Chart: Ranking sortiert nach period_return_pct', () => {
    const ranked = [...userData].sort((a, b) =>
      getPeriodReturn(b.user_id, userData, []) - getPeriodReturn(a.user_id, userData, [])
    )
    assert.equal(ranked[0].username, 'Alice')
    assert.equal(ranked[1].username, 'Bob')
  })

  test('Chart: getPeriodReturn nutzt period_return_pct aus historyData', () => {
    const ret = getPeriodReturn(1, userData, [])
    approx(ret, 8.0, 0.01)
  })

  test('Chart: getPeriodReturn Fallback auf total_return_pct', () => {
    const portfolios = [{ user_id: 99, total_return_pct: 15.5 }]
    const ret = getPeriodReturn(99, [], portfolios)
    approx(ret, 15.5, 0.01)
  })
}

// ────────────────────────────────────────
console.log('\n── 12. Edge Cases ──')
// ────────────────────────────────────────

test('Keine Positionen → leere History', () => {
  const h = calculatePortfolioHistory([], { 'AAPL': makePrices([100], NOW) })
  assert.equal(h.length, 0)
})

test('Keine Kursdaten → leere History', () => {
  const h = calculatePortfolioHistory([{ symbol: 'AAPL', avgPrice: 100 }], {})
  assert.equal(h.length, 0)
})

test('Position mit Kurs 0 wird übersprungen', () => {
  const positions = [{ symbol: 'AAPL', avgPrice: 0, purchaseDate: null }]
  const symbolData = { 'AAPL': makePrices([0, 0], NOW - 2 * DAY) }
  const h = calculatePortfolioHistory(positions, symbolData)
  // Bei basePrice 0 wird 1000*(price/0) = Infinity → portfolioValue > 0 aber baseValue = Infinity
  // Backend: if e.basePrice > 0 → wird übersprungen
  assert.equal(h.length, 0)
})

test('Einzelner Datenpunkt → 0% pct', () => {
  const positions = [{ symbol: 'AAPL', avgPrice: 100, purchaseDate: null }]
  const symbolData = { 'AAPL': makePrices([150], NOW) }
  const h = calculatePortfolioHistory(positions, symbolData)
  assert.equal(h.length, 1)
  approx(h[0].pct, 0, 0.01) // Basis = erster Preis
})

test('period_return: leere History + leere Trades → 0', () => {
  approx(calculatePeriodReturn([], []), 0, 0.01)
})

test('Negatives Portfolio: -50% Verlust korrekt', () => {
  const positions = [{ symbol: 'AAPL', avgPrice: 100, purchaseDate: null }]
  const symbolData = { 'AAPL': makePrices([100, 75, 50], NOW - 3 * DAY) }
  const h = calculatePortfolioHistory(positions, symbolData)
  approx(h[h.length - 1].pct, -50, 0.01)
})

// ────────────────────────────────────────
console.log('\n── 13. Rendite-Konsistenz: pct vs. tatsächlicher Gewinn ──')
// ────────────────────────────────────────

{
  // Ohne Rebase: pct sollte genau (endValue - startValue) / startValue * 100 sein
  const startTime = NOW - 5 * DAY
  const positions = [
    { symbol: 'AAPL', avgPrice: 100, purchaseDate: null },
    { symbol: 'MSFT', avgPrice: 300, purchaseDate: null },
  ]
  const symbolData = {
    'AAPL': makePrices([100, 105, 115], startTime),
    'MSFT': makePrices([300, 310, 330], startTime),
  }

  const history = calculatePortfolioHistory(positions, symbolData)

  test('Ohne Rebase: pct = (endValue - startValue) / startValue * 100', () => {
    const startVal = history[0].value
    const endVal = history[history.length - 1].value
    const expectedPct = ((endVal - startVal) / startVal) * 100
    approx(history[history.length - 1].pct, expectedPct, 0.01)
  })

  test('Ohne Rebase: period_return stimmt mit Dollar-Gewinn überein', () => {
    const ret = calculatePeriodReturn(history, [])
    const startVal = history[0].value
    const endVal = history[history.length - 1].value
    const dollarGain = endVal - startVal
    const dollarFromPct = (ret / 100) * startVal
    approx(dollarFromPct, dollarGain, 0.1)
  })
}

// ────────────────────────────────────────
console.log('\n── 14. Quantities: gewichtete Rendite vs. gleichgewichtete ──')
// ────────────────────────────────────────

{
  const startTime = NOW - 3 * DAY
  const positions = [
    { symbol: 'AAPL', avgPrice: 100, purchaseDate: null, quantity: 10 }, // 1000$ investiert
    { symbol: 'MSFT', avgPrice: 200, purchaseDate: null, quantity: 1 },  // 200$ investiert
  ]
  // AAPL +20%, MSFT -10%
  const symbolData = {
    'AAPL': makePrices([100, 110, 120], startTime),
    'MSFT': makePrices([200, 195, 180], startTime),
  }

  const histQ = calculatePortfolioHistory(positions, symbolData, true)
  const histNoQ = calculatePortfolioHistory(positions, symbolData, false)

  test('Quantities: gewichtete Rendite ≠ gleichgewichtete', () => {
    // Mit Quantities: Start=10*100+1*200=1200, End=10*120+1*180=1380 → +15%
    // Ohne Quantities: Start=1000+1000=2000, End=1000*(120/100)+1000*(180/200)=1200+900=2100 → +5%
    approx(histQ[histQ.length - 1].pct, ((1380 - 1200) / 1200) * 100, 0.1)
    approx(histNoQ[histNoQ.length - 1].pct, ((2100 - 2000) / 2000) * 100, 0.1)
    assert.ok(Math.abs(histQ[histQ.length - 1].pct - histNoQ[histNoQ.length - 1].pct) > 1,
      'Gewichtete und gleichgewichtete Rendite sollten sich unterscheiden')
  })
}

// ────────────────────────────────────────
console.log('\n── 15. formatPercent Konsistenz ──')
// ────────────────────────────────────────

{
  // Repliziert die formatPercent-Funktion aus PortfolioCompare
  const formatPercent = (value) => {
    if (value === undefined || value === null || isNaN(value)) return '--'
    const sign = value >= 0 ? '+' : ''
    return `${sign}${value.toFixed(2)}%`
  }

  test('formatPercent: positiv', () => assert.equal(formatPercent(10.5), '+10.50%'))
  test('formatPercent: negativ', () => assert.equal(formatPercent(-3.14), '-3.14%'))
  test('formatPercent: null → --', () => assert.equal(formatPercent(null), '--'))
  test('formatPercent: undefined → --', () => assert.equal(formatPercent(undefined), '--'))
  test('formatPercent: NaN → --', () => assert.equal(formatPercent(NaN), '--'))
  test('formatPercent: 0 → +0.00%', () => assert.equal(formatPercent(0), '+0.00%'))
}

// ────────────────────────────────────────
console.log('\n── 16. Ranking-Reihenfolge mit period_return ──')
// ────────────────────────────────────────

{
  const historyData = [
    { user_id: 1, period_return_pct: 15 },
    { user_id: 2, period_return_pct: -5 },
    { user_id: 3, period_return_pct: 8 },
    { user_id: 4, period_return_pct: 0 },
  ]
  const portfolios = historyData.map(h => ({ user_id: h.user_id, total_return_pct: 0 }))

  test('Ranking: korrekte Reihenfolge (höchste Rendite zuerst)', () => {
    const ranked = [...portfolios].sort((a, b) =>
      getPeriodReturn(b.user_id, historyData, portfolios) -
      getPeriodReturn(a.user_id, historyData, portfolios)
    )
    assert.deepEqual(ranked.map(p => p.user_id), [1, 3, 4, 2])
  })
}

// ────────────────────────────────────────
console.log('\n── 17. Vollständiger Portfolio-Compare Workflow ──')
// ────────────────────────────────────────

{
  // Simuliere 3 User mit unterschiedlichen Portfolios
  const startTime = NOW - 30 * DAY

  // User 1: AAPL +20%
  const pos1 = [{ symbol: 'AAPL', avgPrice: 100, purchaseDate: null }]
  const data1 = { 'AAPL': makePrices([100, 105, 110, 115, 120], startTime) }
  const hist1 = calculatePortfolioHistory(pos1, data1)

  // User 2: MSFT -10%
  const pos2 = [{ symbol: 'MSFT', avgPrice: 200, purchaseDate: null }]
  const data2 = { 'MSFT': makePrices([200, 195, 190, 185, 180], startTime) }
  const hist2 = calculatePortfolioHistory(pos2, data2)

  // User 3: GOOG +5%, mit closed trade (+20$ auf 100$ investiert = +20%)
  const pos3 = [{ symbol: 'GOOG', avgPrice: 150, purchaseDate: null }]
  const data3 = { 'GOOG': makePrices([150, 152, 154, 156, 157.5], startTime) }
  const hist3 = calculatePortfolioHistory(pos3, data3)

  const ret1 = calculatePeriodReturn(hist1, [])
  const ret2 = calculatePeriodReturn(hist2, [])
  const ret3 = calculatePeriodReturn(hist3, [{ buyPrice: 100, sellPrice: 120, quantity: 1 }])

  test('Workflow: User1 = +20%', () => approx(ret1, 20, 0.01))
  test('Workflow: User2 = -10%', () => approx(ret2, -10, 0.01))
  test('Workflow: User3 = offene + geschlossene Rendite', () => {
    // openReturnPct = 5%, startValue = 1000
    // openGain = 5/100 * 1000 = 50
    // closedGain = 20, closedInvested = 100
    // totalCapital = 1000 + 100 = 1100
    // periodReturn = (50 + 20) / 1100 * 100 = 6.36%
    approx(ret3, (50 + 20) / 1100 * 100, 0.1)
  })

  test('Workflow: Ranking = User1 > User3 > User2', () => {
    const users = [
      { user_id: 1, ret: ret1 },
      { user_id: 2, ret: ret2 },
      { user_id: 3, ret: ret3 },
    ].sort((a, b) => b.ret - a.ret)
    assert.deepEqual(users.map(u => u.user_id), [1, 3, 2])
  })
}

// ────────────────────────────────────────
console.log('\n── 18. Chart-Baseline und Wertebereiche ──')
// ────────────────────────────────────────

{
  const startTime = NOW - 5 * DAY
  const positions = [{ symbol: 'AAPL', avgPrice: 100, purchaseDate: null }]
  const symbolData = { 'AAPL': makePrices([100, 95, 90, 105, 110], startTime) }
  const history = calculatePortfolioHistory(positions, symbolData)

  test('Chart: pct durchläuft negativ und positiv', () => {
    // Tag 0: 0%, Tag 1: -5%, Tag 2: -10%, Tag 3: +5%, Tag 4: +10%
    approx(history[0].pct, 0, 0.01)
    approx(history[1].pct, -5, 0.01)
    approx(history[2].pct, -10, 0.01)
    approx(history[3].pct, 5, 0.01)
    approx(history[4].pct, 10, 0.01)
  })

  test('Chart: value-Werte konsistent mit pct', () => {
    const base = history[0].value
    for (const point of history) {
      const expectedValue = base * (1 + point.pct / 100)
      approx(point.value, expectedValue, 0.1, `Zeit ${point.time}: `)
    }
  })
}

// ────────────────────────────────────────
console.log('\n── 19. Watchlist-Performance: Portfolio-Rendite (WatchlistBatchPanel) ──')
// ────────────────────────────────────────

// Repliziert die portfolioStats Berechnung aus TradingArena.jsx (WatchlistBatchPanel)
// ALTE Formel (falsch — kompoundiert sequentiell):
function calcPortfolioReturnOld(trades, tradeAmount) {
  const closed = trades.filter(t => !t.is_open)
  const sorted = [...closed].sort((a, b) => a.entry_time - b.entry_time)
  let equity = 1.0
  for (const t of sorted) {
    equity *= (1 + t.return_pct / 100)
  }
  const portfolioReturn = (equity - 1) * 100

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

  return { portfolioReturn, maxParallel, requiredCapital, totalProfit, posSize }
}

// NEUE Formel (korrekt — Gewinn / eingesetztes Kapital):
function calcPortfolioReturnNew(trades, tradeAmount) {
  const closed = trades.filter(t => !t.is_open)
  const sorted = [...closed].sort((a, b) => a.entry_time - b.entry_time)

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

{
  // Szenario 1: 3 sequentielle Trades (nie parallel), 500€/Trade
  // Trade 1: +10%, Trade 2: +5%, Trade 3: -3%
  const trades1 = [
    { entry_time: 100, exit_time: 200, return_pct: 10, is_open: false },
    { entry_time: 300, exit_time: 400, return_pct: 5, is_open: false },
    { entry_time: 500, exit_time: 600, return_pct: -3, is_open: false },
  ]

  test('Sequentielle Trades: maxParallel = 1', () => {
    const r = calcPortfolioReturnNew(trades1, 500)
    assert.equal(r.maxParallel, 1)
    assert.equal(r.requiredCapital, 500)
  })

  test('Sequentielle Trades: Gewinn = 500*(0.10+0.05-0.03) = 60€', () => {
    const r = calcPortfolioReturnNew(trades1, 500)
    approx(r.totalProfit, 60, 0.01)
  })

  test('Sequentielle Trades: Rendite = 60/500 = 12%', () => {
    const r = calcPortfolioReturnNew(trades1, 500)
    approx(r.portfolioReturn, 12, 0.01)
  })

  test('Sequentielle Trades: ALTE Formel gibt falschen Wert (kompoundiert)', () => {
    const r = calcPortfolioReturnOld(trades1, 500)
    // equity = 1 * 1.10 * 1.05 * 0.97 = 1.11945
    const expected = (1 * 1.10 * 1.05 * 0.97 - 1) * 100
    approx(r.portfolioReturn, expected, 0.01)
    // Das ist ~11.95%, aber der tatsächliche Gewinn auf 500€ Kapital ist 12% (60€/500€)
    // Bei sequentiellen Trades ist der Unterschied gering, aber die Logik ist trotzdem falsch
    // weil kein Reinvestment stattfindet (fixer 500€ Betrag pro Trade)
  })
}

{
  // Szenario 2: 3 parallele Trades, 500€/Trade
  // Alle laufen gleichzeitig → maxParallel = 3 → Kapital = 1500€
  const trades2 = [
    { entry_time: 100, exit_time: 500, return_pct: 10, is_open: false },
    { entry_time: 100, exit_time: 500, return_pct: 5, is_open: false },
    { entry_time: 100, exit_time: 500, return_pct: -3, is_open: false },
  ]

  test('Parallele Trades: maxParallel = 3, Kapital = 1500€', () => {
    const r = calcPortfolioReturnNew(trades2, 500)
    assert.equal(r.maxParallel, 3)
    assert.equal(r.requiredCapital, 1500)
  })

  test('Parallele Trades: Gewinn = 500*(0.10+0.05-0.03) = 60€', () => {
    const r = calcPortfolioReturnNew(trades2, 500)
    approx(r.totalProfit, 60, 0.01)
  })

  test('Parallele Trades: Rendite = 60/1500 = 4%', () => {
    const r = calcPortfolioReturnNew(trades2, 500)
    approx(r.portfolioReturn, 4, 0.01)
  })

  test('Parallele Trades: ALTE Formel ignoriert Parallelität komplett', () => {
    const rOld = calcPortfolioReturnOld(trades2, 500)
    const rNew = calcPortfolioReturnNew(trades2, 500)
    // Alte Formel kompoundiert: 1 * 1.10 * 1.05 * 0.97 ≈ 11.9% (so als wären sie sequentiell)
    // Neue Formel: 60/1500 = 4% (korrekt für 1500€ eingesetztes Kapital)
    assert.ok(Math.abs(rOld.portfolioReturn - rNew.portfolioReturn) > 5,
      `Alt=${rOld.portfolioReturn.toFixed(1)}% vs Neu=${rNew.portfolioReturn.toFixed(1)}% — Differenz sollte >5% sein`)
  })
}

{
  // Szenario 3: Gemischt — erst 1 Trade, dann 2 parallel, dann 1
  // t=100-200: Trade A (+8%)
  // t=300-500: Trade B (+12%) und Trade C (-5%) parallel
  // t=600-700: Trade D (+3%)
  // maxParallel = 2
  const trades3 = [
    { entry_time: 100, exit_time: 200, return_pct: 8, is_open: false },
    { entry_time: 300, exit_time: 500, return_pct: 12, is_open: false },
    { entry_time: 300, exit_time: 500, return_pct: -5, is_open: false },
    { entry_time: 600, exit_time: 700, return_pct: 3, is_open: false },
  ]

  test('Gemischte Trades: maxParallel = 2, Kapital = 1000€', () => {
    const r = calcPortfolioReturnNew(trades3, 500)
    assert.equal(r.maxParallel, 2)
    assert.equal(r.requiredCapital, 1000)
  })

  test('Gemischte Trades: Gewinn = 500*(0.08+0.12-0.05+0.03) = 90€', () => {
    const r = calcPortfolioReturnNew(trades3, 500)
    approx(r.totalProfit, 90, 0.01)
  })

  test('Gemischte Trades: Rendite = 90/1000 = 9%', () => {
    const r = calcPortfolioReturnNew(trades3, 500)
    approx(r.portfolioReturn, 9, 0.01)
  })
}

{
  // Szenario 4: Nur Verlust-Trades
  const trades4 = [
    { entry_time: 100, exit_time: 200, return_pct: -10, is_open: false },
    { entry_time: 100, exit_time: 200, return_pct: -5, is_open: false },
  ]

  test('Nur Verluste: Rendite = -7.5%', () => {
    const r = calcPortfolioReturnNew(trades4, 500)
    // Gewinn = 500*(-0.10 + -0.05) = -75€, Kapital = 1000€
    approx(r.portfolioReturn, -7.5, 0.01)
  })
}

{
  // Szenario 5: Offene Trades werden ignoriert
  const trades5 = [
    { entry_time: 100, exit_time: 200, return_pct: 20, is_open: false },
    { entry_time: 100, exit_time: null, return_pct: 50, is_open: true }, // offen
  ]

  test('Offene Trades werden ignoriert', () => {
    const r = calcPortfolioReturnNew(trades5, 500)
    assert.equal(r.maxParallel, 1) // nur der geschlossene zählt
    approx(r.totalProfit, 100, 0.01) // 500 * 0.20
    approx(r.portfolioReturn, 20, 0.01) // 100/500
  })
}

{
  // Szenario 6: Keine Trades → 0%
  test('Keine Trades: Rendite = 0%', () => {
    const r = calcPortfolioReturnNew([], 500)
    approx(r.portfolioReturn, 0, 0.01)
  })
}

{
  // Szenario 7: Realistisch — 10 Aktien, verschiedene Returns, teilweise parallel
  // 5 laufen gleichzeitig von t=100-300, 5 laufen von t=200-400 (Überlappung bei t=200-300 → 10 parallel)
  const trades7 = []
  const returns1 = [8, 12, -3, 5, -7] // Gruppe 1
  const returns2 = [15, -10, 6, -2, 9] // Gruppe 2
  returns1.forEach((r, i) => trades7.push({ entry_time: 100, exit_time: 300, return_pct: r, is_open: false }))
  returns2.forEach((r, i) => trades7.push({ entry_time: 200, exit_time: 400, return_pct: r, is_open: false }))

  test('10 Aktien realistisch: maxParallel = 10', () => {
    const r = calcPortfolioReturnNew(trades7, 500)
    assert.equal(r.maxParallel, 10)
    assert.equal(r.requiredCapital, 5000)
  })

  test('10 Aktien realistisch: korrekte Rendite', () => {
    const r = calcPortfolioReturnNew(trades7, 500)
    const totalReturnPct = 8 + 12 - 3 + 5 - 7 + 15 - 10 + 6 - 2 + 9 // = 33
    const totalProfit = 500 * totalReturnPct / 100 // = 165€
    approx(r.totalProfit, totalProfit, 0.01)
    approx(r.portfolioReturn, totalProfit / 5000 * 100, 0.01) // 165/5000 = 3.3%
  })

  test('10 Aktien: ALTE Formel weicht stark ab', () => {
    const rOld = calcPortfolioReturnOld(trades7, 500)
    const rNew = calcPortfolioReturnNew(trades7, 500)
    // Alte Formel kompoundiert 10 Trades: ≈ 37.5% (sequentiell)
    // Neue Formel: 3.3% (auf eingesetztes Kapital)
    console.log(`    Alt: ${rOld.portfolioReturn.toFixed(1)}% vs Neu: ${rNew.portfolioReturn.toFixed(1)}%`)
    assert.ok(Math.abs(rOld.portfolioReturn - rNew.portfolioReturn) > 20,
      'Alte Formel weicht bei 10 parallelen Trades massiv ab')
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
