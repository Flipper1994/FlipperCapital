// Debug script: Verify BXtrender Ditz/Trader fix for CTSH
// Compares OLD (T3) vs NEW (bothPositive) frontend logic against backend

function calculateEMA(data, period) {
  const result = new Array(data.length).fill(0)
  const multiplier = 2 / (period + 1)
  let sum = 0
  for (let i = 0; i < period && i < data.length; i++) sum += data[i]
  result[period - 1] = sum / period
  for (let i = period; i < data.length; i++) {
    result[i] = (data[i] - result[i - 1]) * multiplier + result[i - 1]
  }
  return result
}

function calculateRSI(data, period) {
  const result = new Array(data.length).fill(50)
  if (data.length < period + 1) return result
  let gainSum = 0, lossSum = 0
  for (let i = 1; i <= period; i++) {
    const change = data[i] - data[i - 1]
    if (change > 0) gainSum += change
    else lossSum -= change
  }
  let avgGain = gainSum / period
  let avgLoss = lossSum / period
  result[period] = avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss)
  for (let i = period + 1; i < data.length; i++) {
    const change = data[i] - data[i - 1]
    avgGain = (avgGain * (period - 1) + (change > 0 ? change : 0)) / period
    avgLoss = (avgLoss * (period - 1) + (change < 0 ? -change : 0)) / period
    result[i] = avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss)
  }
  return result
}

function calculateT3(data, period) {
  const e1 = calculateEMA(data, period)
  const e2 = calculateEMA(e1, period)
  const e3 = calculateEMA(e2, period)
  const e4 = calculateEMA(e3, period)
  const e5 = calculateEMA(e4, period)
  const e6 = calculateEMA(e5, period)
  const b = 0.7
  const c1 = -b * b * b
  const c2 = 3 * b * b + 3 * b * b * b
  const c3 = -6 * b * b - 3 * b - 3 * b * b * b
  const c4 = 1 + 3 * b + b * b * b + 3 * b * b
  return data.map((_, i) => c1 * e6[i] + c2 * e5[i] + c3 * e4[i] + c4 * e3[i])
}

async function fetchMonthlyData(symbol) {
  const url = `https://query1.finance.yahoo.com/v8/finance/chart/${symbol}?range=max&interval=1mo`
  const resp = await fetch(url, {
    headers: { 'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)' }
  })
  const json = await resp.json()
  const result = json.chart.result[0]
  const timestamps = result.timestamp
  const quotes = result.indicators.quote[0]
  const data = []
  for (let i = 0; i < timestamps.length; i++) {
    if (quotes.close[i] && quotes.open[i]) {
      data.push({ time: timestamps[i], open: quotes.open[i], close: quotes.close[i], high: quotes.high[i], low: quotes.low[i] })
    }
  }
  return data
}

function simulate(label, ohlcv, shortTermXtrender, longTermXtrender, signalLine, opens, times, startIdx, mode) {
  const tslPercent = 20.0
  let inPosition = false
  let entryPrice = 0, highestPrice = 0
  const trades = []

  for (let i = Math.max(startIdx, 1); i < ohlcv.length; i++) {
    const shortCurr = shortTermXtrender[i]
    const shortPrev = shortTermXtrender[i - 1]
    const longCurr = longTermXtrender[i]
    const longPrev = longTermXtrender[i - 1]
    const price = ohlcv[i].close

    if (inPosition && price > highestPrice) highestPrice = price

    let tslTriggered = false
    if (inPosition && highestPrice > 0) {
      if (price <= highestPrice * (1 - tslPercent / 100)) tslTriggered = true
    }

    const bothPositiveNow = shortCurr > 0 && longCurr > 0
    const bothPositivePrev = shortPrev > 0 && longPrev > 0
    const bothNegativeNow = shortCurr < 0 && longCurr < 0

    let buySignal, sellSignal

    if (mode === 'old_t3') {
      // OLD Frontend logic (T3 signal line)
      const sigVal = signalLine[i]
      const sigPrev = signalLine[i - 1]
      const lineIsGreen = sigVal > sigPrev
      const lineWasGreen = i > startIdx ? sigPrev > signalLine[i - 2] : false
      buySignal = !inPosition && lineIsGreen && !lineWasGreen
      sellSignal = inPosition && !lineIsGreen && lineWasGreen
    } else if (mode === 'new_frontend') {
      // NEW Frontend logic (bothPositive + TSL, matching backend)
      buySignal = !inPosition && bothPositiveNow && (!bothPositivePrev || !inPosition)
      sellSignal = inPosition && (bothNegativeNow || tslTriggered)
    } else {
      // Backend logic (reference)
      buySignal = !inPosition && bothPositiveNow && (!bothPositivePrev || !inPosition)
      sellSignal = inPosition && (bothNegativeNow || tslTriggered)
    }

    const d = new Date(times[i] * 1000)
    const month = d.toLocaleDateString('de-DE', { month: 'short', year: 'numeric' })

    if (buySignal && !inPosition && i + 1 < ohlcv.length) {
      entryPrice = opens[i + 1]
      highestPrice = entryPrice
      inPosition = true
      const execDate = new Date(times[i + 1] * 1000)
      trades.push({ type: 'BUY', signal: month, exec: execDate.toLocaleDateString('de-DE', { month: 'short', year: 'numeric' }), price: entryPrice })
    } else if (sellSignal && inPosition && i + 1 < ohlcv.length) {
      const exitPrice = opens[i + 1]
      const ret = ((exitPrice - entryPrice) / entryPrice) * 100
      const execDate = new Date(times[i + 1] * 1000)
      trades.push({ type: tslTriggered ? 'SELL(TSL)' : 'SELL', signal: month, exec: execDate.toLocaleDateString('de-DE', { month: 'short', year: 'numeric' }), price: exitPrice, ret })
      inPosition = false
      entryPrice = 0
      highestPrice = 0
    }
  }

  console.log(`\n--- ${label} ---`)
  if (trades.length === 0) {
    console.log('  Keine Trades in den letzten 24 Monaten')
  }
  for (const t of trades) {
    if (t.type === 'BUY') {
      console.log(`  ${t.type.padEnd(10)} Signal @ ${t.signal.padEnd(12)} → Exec @ ${t.exec} ($${t.price.toFixed(2)})`)
    } else {
      console.log(`  ${t.type.padEnd(10)} Signal @ ${t.signal.padEnd(12)} → Exec @ ${t.exec} ($${t.price.toFixed(2)}) Return: ${t.ret >= 0 ? '+' : ''}${t.ret.toFixed(1)}%`)
    }
  }
  console.log(`  Status: ${inPosition ? 'IN POSITION' : 'FLAT'}`)
  return trades
}

async function main() {
  console.log('=== CTSH: Vergleich ALT vs NEU vs Backend ===\n')

  const ohlcv = await fetchMonthlyData('CTSH')
  const closes = ohlcv.map(b => b.close)
  const opens = ohlcv.map(b => b.open)
  const times = ohlcv.map(b => b.time)

  const shortL1 = 5, shortL2 = 20, shortL3 = 15, longL1 = 20, longL2 = 15

  const ema1 = calculateEMA(closes, shortL1)
  const ema2 = calculateEMA(closes, shortL2)
  const diff = closes.map((_, i) => ema1[i] - ema2[i])
  const shortTermXtrender = calculateRSI(diff, shortL3).map(v => v - 50)
  const emaLong = calculateEMA(closes, longL1)
  const longTermXtrender = calculateRSI(emaLong, longL2).map(v => v - 50)
  const signalLine = calculateT3(shortTermXtrender, 5)

  const startIdx = Math.max(shortL2, longL1) + shortL3

  // Only show last 24 months
  const cutoff = ohlcv.length - 24

  console.log('BXtrender Werte (letzte 8 Monate):')
  console.log('Monat          | Short  | Long   | Beide>0 | T3-Signal | T3-Richtung')
  console.log('---------------|--------|--------|---------|-----------|------------')
  for (let i = Math.max(0, ohlcv.length - 8); i < ohlcv.length; i++) {
    const d = new Date(times[i] * 1000)
    const month = d.toLocaleDateString('de-DE', { month: 'short', year: 'numeric' }).padEnd(14)
    const s = shortTermXtrender[i].toFixed(2).padStart(6)
    const l = longTermXtrender[i].toFixed(2).padStart(6)
    const bp = (shortTermXtrender[i] > 0 && longTermXtrender[i] > 0) ? '  JA  ' : ' NEIN '
    const sig = signalLine[i].toFixed(2).padStart(9)
    const dir = i > 0 && signalLine[i] > signalLine[i - 1] ? 'steigend' : 'fallend'
    console.log(`${month} | ${s} | ${l} | ${bp}  | ${sig} | ${dir}`)
  }

  // Simulate old start from 24 months ago
  const oldOhlcv = ohlcv.slice(cutoff)
  const oldShort = shortTermXtrender.slice(cutoff)
  const oldLong = longTermXtrender.slice(cutoff)
  const oldSignal = signalLine.slice(cutoff)
  const oldOpens = opens.slice(cutoff)
  const oldTimes = times.slice(cutoff)

  simulate('ALT: Frontend T3-Linie (Trader, KEIN TSL)', oldOhlcv, oldShort, oldLong, oldSignal, oldOpens, oldTimes, 1, 'old_t3')
  simulate('NEU: Frontend bothPositive + TSL (Trader)', oldOhlcv, oldShort, oldLong, oldSignal, oldOpens, oldTimes, 1, 'new_frontend')
  simulate('Backend: bothPositive + TSL (Referenz)', oldOhlcv, oldShort, oldLong, oldSignal, oldOpens, oldTimes, 1, 'backend')

  // Verify match
  const newTrades = simulate('', oldOhlcv, oldShort, oldLong, oldSignal, oldOpens, oldTimes, 1, 'new_frontend')
  const backendTrades = simulate('', oldOhlcv, oldShort, oldLong, oldSignal, oldOpens, oldTimes, 1, 'backend')

  console.log('\n=== ERGEBNIS ===')
  const match = JSON.stringify(newTrades) === JSON.stringify(backendTrades)
  console.log(`Frontend NEU === Backend: ${match ? 'MATCH ✓' : 'MISMATCH ✗'}`)
  if (!match) {
    console.log('NEU:', JSON.stringify(newTrades, null, 2))
    console.log('BE:', JSON.stringify(backendTrades, null, 2))
  }
}

main().catch(console.error)
