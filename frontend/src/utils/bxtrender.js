// B-Xtrender Implementation based on original Pine Script by @Puppytherapy
// shortTermXtrender = rsi(ema(close, 5) - ema(close, 20), 15) - 50
// longTermXtrender = rsi(ema(close, 20), 15) - 50

// Pine Script compatible EMA - initializes with SMA of first 'period' values
function calculateEMA(data, period) {
  const ema = new Array(data.length).fill(0)

  if (data.length < period) return ema

  // Initialize with SMA of first 'period' values
  let sum = 0
  for (let i = 0; i < period; i++) {
    sum += data[i]
  }
  ema[period - 1] = sum / period

  // Calculate EMA for remaining values
  const multiplier = 2 / (period + 1)
  for (let i = period; i < data.length; i++) {
    ema[i] = (data[i] - ema[i - 1]) * multiplier + ema[i - 1]
  }

  // Fill initial values with first valid EMA
  for (let i = 0; i < period - 1; i++) {
    ema[i] = ema[period - 1]
  }

  return ema
}

// Pine Script compatible RMA (Wilder's smoothing) for RSI
function calculateRMA(data, period) {
  const rma = new Array(data.length).fill(0)

  if (data.length < period) return rma

  // Initialize with SMA
  let sum = 0
  for (let i = 0; i < period; i++) {
    sum += data[i]
  }
  rma[period - 1] = sum / period

  // Calculate RMA (Wilder's smoothing: alpha = 1/period)
  const alpha = 1 / period
  for (let i = period; i < data.length; i++) {
    rma[i] = alpha * data[i] + (1 - alpha) * rma[i - 1]
  }

  return rma
}

// Pine Script compatible RSI using RMA
function calcRSI(data, period) {
  const result = new Array(data.length).fill(50)

  if (data.length < period + 1) return result

  // Calculate gains and losses
  const gains = new Array(data.length).fill(0)
  const losses = new Array(data.length).fill(0)

  for (let i = 1; i < data.length; i++) {
    const change = data[i] - data[i - 1]
    gains[i] = change > 0 ? change : 0
    losses[i] = change < 0 ? Math.abs(change) : 0
  }

  // Calculate RMA of gains and losses
  const avgGain = calculateRMA(gains.slice(1), period)
  const avgLoss = calculateRMA(losses.slice(1), period)

  // Calculate RSI
  for (let i = period; i < data.length; i++) {
    const ag = avgGain[i - 1]
    const al = avgLoss[i - 1]
    if (al === 0) {
      result[i] = ag === 0 ? 50 : 100
    } else {
      const rs = ag / al
      result[i] = 100 - 100 / (1 + rs)
    }
  }

  return result
}

// T3 Moving Average (Tillson T3)
function calculateT3(data, period) {
  const b = 0.7
  const c1 = -b * b * b
  const c2 = 3 * b * b + 3 * b * b * b
  const c3 = -6 * b * b - 3 * b - 3 * b * b * b
  const c4 = 1 + 3 * b + b * b * b + 3 * b * b

  const e1 = calculateEMA(data, period)
  const e2 = calculateEMA(e1, period)
  const e3 = calculateEMA(e2, period)
  const e4 = calculateEMA(e3, period)
  const e5 = calculateEMA(e4, period)
  const e6 = calculateEMA(e5, period)

  return data.map((_, i) => c1 * e6[i] + c2 * e5[i] + c3 * e4[i] + c4 * e3[i])
}

// Default config values
const DEFAULT_CONFIG = {
  shortL1: 5,
  shortL2: 20,
  shortL3: 15,
  longL1: 20,
  longL2: 15
}

// Default Quant config values (QuantTherapy algorithm)
const DEFAULT_QUANT_CONFIG = {
  shortL1: 5,
  shortL2: 20,
  shortL3: 15,
  longL1: 20,
  longL2: 15,
  maFilterOn: true,
  maLength: 200,
  maType: 'EMA',
  tslPercent: 20.0
}

// Cache for BXtrender config
let configCache = null
let configPromise = null

// Cache for Quant config
let quantConfigCache = null
let quantConfigPromise = null

// Cache for Ditz config
let ditzConfigCache = null
let ditzConfigPromise = null

// Fetch BXtrender config from backend
export async function fetchBXtrenderConfig() {
  if (configCache) return configCache
  if (configPromise) return configPromise

  configPromise = fetch('/api/bxtrender-config')
    .then(res => res.json())
    .then(data => {
      configCache = data
      return data
    })
    .catch(() => {
      // Return defaults on error
      return {
        defensive: DEFAULT_CONFIG,
        aggressive: DEFAULT_CONFIG
      }
    })

  return configPromise
}

// Fetch Quant config from backend
export async function fetchBXtrenderQuantConfig() {
  if (quantConfigCache) return quantConfigCache
  if (quantConfigPromise) return quantConfigPromise

  quantConfigPromise = fetch('/api/bxtrender-quant-config')
    .then(res => res.json())
    .then(data => {
      quantConfigCache = data
      return data
    })
    .catch(() => {
      return DEFAULT_QUANT_CONFIG
    })

  return quantConfigPromise
}

// Clear config cache (call when config is updated)
export function clearConfigCache() {
  configCache = null
  configPromise = null
}

// Clear quant config cache
export function clearQuantConfigCache() {
  quantConfigCache = null
  quantConfigPromise = null
}

// Fetch Ditz config from backend
export async function fetchBXtrenderDitzConfig() {
  if (ditzConfigCache) return ditzConfigCache
  if (ditzConfigPromise) return ditzConfigPromise

  ditzConfigPromise = fetch('/api/bxtrender-ditz-config')
    .then(res => res.json())
    .then(data => {
      ditzConfigCache = data
      return data
    })
    .catch(() => {
      return DEFAULT_QUANT_CONFIG
    })

  return ditzConfigPromise
}

// Clear ditz config cache
export function clearDitzConfigCache() {
  ditzConfigCache = null
  ditzConfigPromise = null
}

export function calculateBXtrender(ohlcv, isAggressive = false, config = null) {
  // Use provided config or defaults
  const cfg = config || DEFAULT_CONFIG
  const shortL1 = cfg.shortL1 || cfg.short_l1 || 5
  const shortL2 = cfg.shortL2 || cfg.short_l2 || 20
  const shortL3 = cfg.shortL3 || cfg.short_l3 || 15
  const longL1 = cfg.longL1 || cfg.long_l1 || 20
  const longL2 = cfg.longL2 || cfg.long_l2 || 15

  if (!ohlcv || ohlcv.length < Math.max(shortL2, longL1) + shortL3 + 10) {
    return { short: [], long: [], signal: [], trades: [], markers: [] }
  }

  // Filter out invalid data points
  const validData = ohlcv.filter(d => d && d.close != null && !isNaN(d.close))
  if (validData.length < Math.max(shortL2, longL1) + shortL3 + 10) {
    return { short: [], long: [], signal: [], trades: [], markers: [] }
  }

  const closes = validData.map(d => d.close)
  const opens = validData.map(d => d.open != null ? d.open : d.close)
  const times = validData.map(d => d.time)

  // Short-term: RSI(EMA(close, 5) - EMA(close, 20), 15) - 50
  const emaShort1 = calculateEMA(closes, shortL1)
  const emaShort2 = calculateEMA(closes, shortL2)
  const emaDiff = emaShort1.map((v, i) => v - emaShort2[i])
  const rsiShort = calcRSI(emaDiff, shortL3)
  const shortTermXtrender = rsiShort.map(v => v - 50)

  // Long-term: RSI(EMA(close, 20), 15) - 50
  const emaLong = calculateEMA(closes, longL1)
  const rsiLong = calcRSI(emaLong, longL2)
  const longTermXtrender = rsiLong.map(v => v - 50)

  // T3 Signal line of short-term
  const signalLine = calculateT3(shortTermXtrender, 5)

  // Build result with colors
  const shortData = []
  const longData = []
  const signalData = []
  const trades = []
  const markers = []

  const startIdx = Math.max(shortL2, longL1) + shortL3

  // Track trading state
  let inPosition = false
  let entryPrice = 0
  let entryDate = null

  for (let i = startIdx; i < closes.length; i++) {
    const shortVal = shortTermXtrender[i]
    const shortPrev = shortTermXtrender[i - 1]
    const longVal = longTermXtrender[i]
    const longPrev = longTermXtrender[i - 1]
    const sigVal = signalLine[i]
    const sigPrev = signalLine[i - 1]

    // Determine if bullish (green) or bearish (red)
    const isBullish = shortVal > 0
    const wasBullish = shortPrev > 0

    // Aggressive mode specific states
    const isLightRed = shortVal < 0 && shortVal > shortPrev
    const isDarkRed = shortVal < 0 && shortVal <= shortPrev
    const wasLightRed = shortPrev < 0 && i > startIdx && shortPrev > shortTermXtrender[i - 2]
    const wasDarkRed = shortPrev < 0 && i > startIdx && shortPrev <= shortTermXtrender[i - 2]

    // Short-term histogram colors
    let shortColor
    if (shortVal > 0) {
      shortColor = shortVal > shortPrev ? '#00FF00' : '#228B22'
    } else {
      shortColor = shortVal > shortPrev ? '#FF0000' : '#8B0000'
    }

    // Long-term histogram colors
    let longColor
    if (longVal > 0) {
      longColor = longVal > longPrev ? '#00FF00' : '#228B22'
    } else {
      longColor = longVal > longPrev ? '#FF0000' : '#8B0000'
    }

    // Signal line color
    const signalColor = sigVal > sigPrev ? '#00FF00' : '#FF0000'

    let buySignal = false
    let sellSignal = false

    // Count consecutive light red bars (used by both modes)
    let consecutiveLightRed = 0
    if (isLightRed) {
      consecutiveLightRed = 1
      for (let j = i - 1; j >= startIdx; j--) {
        const v = shortTermXtrender[j]
        const vPrev = shortTermXtrender[j - 1]
        if (v < 0 && v > vPrev) {
          consecutiveLightRed++
        } else {
          break
        }
      }
    }

    if (isAggressive) {
      // Aggressive BUY: First or second light red bar, OR red->green transition
      const justTurnedGreen = isBullish && !wasBullish
      if (!inPosition && opens[i] > 0) {
        if ((isLightRed && consecutiveLightRed <= 2) || justTurnedGreen) {
          buySignal = true
        }
      }
      // Aggressive SELL: First dark red bar immediately
      if (isDarkRed && inPosition && opens[i] > 0 && entryPrice > 0) {
        sellSignal = true
      }
    } else {
      // Defensive BUY:
      // 1. Enter when crossing from red to green (rot->grün)
      // 2. OR after 3 consecutive light red bars, buy at the 4th light red
      if (!inPosition && opens[i] > 0) {
        const justTurnedGreen = isBullish && !wasBullish
        const fourthLightRed = isLightRed && consecutiveLightRed === 4
        if (justTurnedGreen || fourthLightRed) {
          buySignal = true
        }
      }
      // Defensive SELL: Exit immediately at first dark red bar
      if (isDarkRed && inPosition && opens[i] > 0 && entryPrice > 0) {
        sellSignal = true
      }
    }

    if (buySignal) {
      // IMPORTANT: Signal is evaluated at month end, so trade happens at START of NEXT month
      // Check if next month data exists
      if (i + 1 < closes.length && opens[i + 1] > 0) {
        inPosition = true
        entryPrice = opens[i + 1]  // Open price of NEXT month
        entryDate = times[i + 1]   // First trading day of NEXT month

        markers.push({
          time: times[i + 1],
          position: 'belowBar',
          color: '#00FF00',
          shape: 'arrowUp',
          text: `BUY $${opens[i + 1].toFixed(2)}`
        })
      }
      // If no next month data, skip this signal (can't trade yet)
    }

    if (sellSignal) {
      // IMPORTANT: Signal is evaluated at month end, so trade happens at START of NEXT month
      // Check if next month data exists
      if (i + 1 < closes.length && opens[i + 1] > 0) {
        const exitPrice = opens[i + 1]  // Open price of NEXT month
        const returnPct = ((exitPrice - entryPrice) / entryPrice) * 100

        trades.push({
          entryDate: entryDate,
          entryPrice: entryPrice,
          exitDate: times[i + 1],   // First trading day of NEXT month
          exitPrice: exitPrice,
          returnPct: returnPct,
          isOpen: false
        })

        const returnText = returnPct >= 0 ? `+${returnPct.toFixed(1)}%` : `${returnPct.toFixed(1)}%`
        markers.push({
          time: times[i + 1],
          position: 'aboveBar',
          color: '#FF0000',
          shape: 'arrowDown',
          text: `SELL $${exitPrice.toFixed(2)} ${returnText}`
        })

        inPosition = false
        entryPrice = 0
        entryDate = null
      }
      // If no next month data, position stays open
    }

    shortData.push({
      time: times[i],
      value: shortVal,
      color: shortColor
    })

    longData.push({
      time: times[i],
      value: longVal,
      color: longColor
    })

    signalData.push({
      time: times[i],
      value: sigVal,
      color: signalColor
    })
  }

  // If still in position, add open trade info
  if (inPosition && entryPrice > 0) {
    const lastIdx = closes.length - 1
    const currentPrice = closes[lastIdx]
    const unrealizedReturn = ((currentPrice - entryPrice) / entryPrice) * 100

    trades.push({
      entryDate: entryDate,
      entryPrice: entryPrice,
      exitDate: null,
      exitPrice: null,
      currentPrice: currentPrice,
      returnPct: unrealizedReturn,
      isOpen: true
    })
  }

  return { short: shortData, long: longData, signal: signalData, trades, markers }
}

/**
 * Calculate B-Xtrender using QuantTherapy algorithm
 * Entry: Both short-term AND long-term indicators positive (with optional MA filter)
 * Exit: Either short-term OR long-term indicator turns negative
 */
export function calculateBXtrenderQuant(ohlcv, config = null, mode = 'quant') {
  const cfg = config || DEFAULT_QUANT_CONFIG
  const shortL1 = cfg.shortL1 || cfg.short_l1 || 5
  const shortL2 = cfg.shortL2 || cfg.short_l2 || 20
  const shortL3 = cfg.shortL3 || cfg.short_l3 || 15
  const longL1 = cfg.longL1 || cfg.long_l1 || 20
  const longL2 = cfg.longL2 || cfg.long_l2 || 15
  let maFilterOn = cfg.maFilterOn !== undefined ? cfg.maFilterOn : cfg.ma_filter_on !== undefined ? cfg.ma_filter_on : true
  const maLength = cfg.maLength || cfg.ma_length || 200
  const maType = cfg.maType || cfg.ma_type || 'EMA'

  // Minimum required for basic B-Xtrender calculation (without MA filter)
  const minDataRequired = Math.max(shortL2, longL1) + shortL3 + 10

  if (!ohlcv || ohlcv.length < minDataRequired) {
    return { short: [], long: [], signal: [], trades: [], markers: [] }
  }

  // Filter out invalid data points
  const validData = ohlcv.filter(d => d && d.close != null && !isNaN(d.close))
  if (validData.length < minDataRequired) {
    return { short: [], long: [], signal: [], trades: [], markers: [] }
  }

  // If not enough data for MA filter, disable it automatically
  if (validData.length < maLength + shortL3 + 10) {
    maFilterOn = false
  }

  const closes = validData.map(d => d.close)
  const opens = validData.map(d => d.open != null ? d.open : d.close)
  const times = validData.map(d => d.time)

  // Short-term Xtrender: RSI(EMA(close, shortL1) - EMA(close, shortL2), shortL3) - 50
  const emaShort1 = calculateEMA(closes, shortL1)
  const emaShort2 = calculateEMA(closes, shortL2)
  const emaDiff = emaShort1.map((v, i) => v - emaShort2[i])
  const rsiShort = calcRSI(emaDiff, shortL3)
  const shortTermXtrender = rsiShort.map(v => v - 50)

  // Long-term Xtrender: RSI(EMA(close, longL1), longL2) - 50
  const emaLong = calculateEMA(closes, longL1)
  const rsiLong = calcRSI(emaLong, longL2)
  const longTermXtrender = rsiLong.map(v => v - 50)

  // Moving Average filter (only calculate if enabled and enough data)
  const ma = maFilterOn
    ? (maType === 'SMA' ? calculateSMA(closes, maLength) : calculateEMA(closes, maLength))
    : null

  // T3 Signal line
  const signalLine = calculateT3(shortTermXtrender, 5)

  const shortData = []
  const longData = []
  const signalData = []
  const trades = []
  const markers = []

  // Start index depends on whether MA filter is active
  const startIdx = maFilterOn
    ? Math.max(shortL2, longL1, maLength) + shortL3
    : Math.max(shortL2, longL1) + shortL3

  let inPosition = false
  let entryPrice = 0
  let entryDate = null

  for (let i = startIdx; i < closes.length; i++) {
    const shortVal = shortTermXtrender[i]
    const shortPrev = shortTermXtrender[i - 1]
    const longVal = longTermXtrender[i]
    const longPrev = longTermXtrender[i - 1]
    const sigVal = signalLine[i]
    const sigPrev = signalLine[i - 1]
    const price = closes[i]
    const maVal = ma ? ma[i] : 0

    // Quant coloring: Both positive = green shades, both negative = red shades, mixed = gray
    let shortColor, longColor
    const bothPositive = shortVal > 0 && longVal > 0
    const bothNegative = shortVal < 0 && longVal < 0

    if (bothPositive) {
      // Both positive - green gradient
      shortColor = shortVal > shortPrev ? '#00FF00' : '#228B22'
      longColor = longVal > longPrev ? '#00FF00' : '#228B22'
    } else if (bothNegative) {
      // Both negative - red gradient
      shortColor = shortVal > shortPrev ? '#FF0000' : '#8B0000'
      longColor = longVal > longPrev ? '#FF0000' : '#8B0000'
    } else {
      // Mixed - use original coloring
      shortColor = shortVal > 0
        ? (shortVal > shortPrev ? '#00FF00' : '#228B22')
        : (shortVal > shortPrev ? '#FF0000' : '#8B0000')
      longColor = longVal > 0
        ? (longVal > longPrev ? '#00FF00' : '#228B22')
        : (longVal > longPrev ? '#FF0000' : '#8B0000')
    }

    const signalColor = sigVal > sigPrev ? '#00FF00' : '#FF0000'

    // MA Filter: for long entry, price must be above MA (if filter enabled)
    const maFilterLong = !maFilterOn || price > maVal

    // Entry condition: BOTH indicators positive AND MA filter passes
    // Check if this is the first bar where both turn positive
    const entrySignal = !inPosition &&
      shortVal > 0 && longVal > 0 &&
      maFilterLong &&
      (shortPrev <= 0 || longPrev <= 0) // At least one was negative before

    // Exit condition: Ditz = BOTH negative (line turns red), Quant = EITHER negative
    const exitSignal = inPosition && (mode === 'ditz'
      ? (shortVal < 0 && longVal < 0)
      : (shortVal < 0 || longVal < 0))

    if (entrySignal && opens[i] > 0) {
      // Enter at next bar open (signal evaluated at bar close)
      if (i + 1 < closes.length && opens[i + 1] > 0) {
        inPosition = true
        entryPrice = opens[i + 1]
        entryDate = times[i + 1]

        markers.push({
          time: times[i + 1],
          position: 'belowBar',
          color: '#00FF00',
          shape: 'arrowUp',
          text: `BUY $${opens[i + 1].toFixed(2)}`
        })
      }
    }

    if (exitSignal && entryPrice > 0) {
      // Exit at next bar open
      if (i + 1 < closes.length && opens[i + 1] > 0) {
        const exitPrice = opens[i + 1]
        const returnPct = ((exitPrice - entryPrice) / entryPrice) * 100

        trades.push({
          entryDate: entryDate,
          entryPrice: entryPrice,
          exitDate: times[i + 1],
          exitPrice: exitPrice,
          returnPct: returnPct,
          isOpen: false
        })

        const returnText = returnPct >= 0 ? `+${returnPct.toFixed(1)}%` : `${returnPct.toFixed(1)}%`
        markers.push({
          time: times[i + 1],
          position: 'aboveBar',
          color: '#FF0000',
          shape: 'arrowDown',
          text: `SELL $${exitPrice.toFixed(2)} ${returnText}`
        })

        inPosition = false
        entryPrice = 0
        entryDate = null
      }
    }

    shortData.push({
      time: times[i],
      value: shortVal,
      color: shortColor
    })

    longData.push({
      time: times[i],
      value: longVal,
      color: longColor
    })

    signalData.push({
      time: times[i],
      value: sigVal,
      color: signalColor
    })
  }

  // If still in position, add open trade info
  if (inPosition && entryPrice > 0) {
    const lastIdx = closes.length - 1
    const currentPrice = closes[lastIdx]
    const unrealizedReturn = ((currentPrice - entryPrice) / entryPrice) * 100

    trades.push({
      entryDate: entryDate,
      entryPrice: entryPrice,
      exitDate: null,
      exitPrice: null,
      currentPrice: currentPrice,
      returnPct: unrealizedReturn,
      isOpen: true
    })
  } else if (!inPosition && closes.length > 0 && shortTermXtrender.length > 0 && longTermXtrender.length > 0) {
    // Not in position but check if current conditions warrant a BUY signal
    // If both indicators are positive now, add an open trade for consistency
    const lastIdx = closes.length - 1
    const lastShort = shortTermXtrender[lastIdx]
    const lastLong = longTermXtrender[lastIdx]
    const lastPrice = closes[lastIdx]
    const lastMA = ma ? ma[lastIdx] : 0
    const maConditionMet = !maFilterOn || lastPrice > lastMA

    if (lastShort > 0 && lastLong > 0 && maConditionMet) {
      // Conditions for BUY are met now - add open trade at current price
      trades.push({
        entryDate: times[lastIdx],
        entryPrice: lastPrice,
        exitDate: null,
        exitPrice: null,
        currentPrice: lastPrice,
        returnPct: 0,
        isOpen: true
      })

      markers.push({
        time: times[lastIdx],
        position: 'belowBar',
        color: '#00FF00',
        shape: 'arrowUp',
        text: `BUY $${lastPrice.toFixed(2)}`
      })
    }
  }

  return { short: shortData, long: longData, signal: signalData, trades, markers }
}

// Simple Moving Average calculation
function calculateSMA(data, period) {
  const sma = new Array(data.length).fill(0)
  if (data.length < period) return sma

  let sum = 0
  for (let i = 0; i < period; i++) {
    sum += data[i]
  }
  sma[period - 1] = sum / period

  for (let i = period; i < data.length; i++) {
    sum = sum - data[i - period] + data[i]
    sma[i] = sum / period
  }

  // Fill initial values
  for (let i = 0; i < period - 1; i++) {
    sma[i] = sma[period - 1]
  }

  return sma
}

// Calculate performance metrics from trades
export function calculateMetrics(trades) {
  const completedTrades = trades.filter(t => !t.isOpen)

  if (completedTrades.length === 0) {
    return {
      winRate: 0,
      riskReward: 0,
      totalReturn: 0,
      avgReturn: 0,
      totalTrades: 0,
      wins: 0,
      losses: 0
    }
  }

  const wins = completedTrades.filter(t => t.returnPct > 0)
  const losses = completedTrades.filter(t => t.returnPct <= 0)

  const winRate = (wins.length / completedTrades.length) * 100

  const avgWin = wins.length > 0
    ? wins.reduce((sum, t) => sum + t.returnPct, 0) / wins.length
    : 0

  const avgLoss = losses.length > 0
    ? Math.abs(losses.reduce((sum, t) => sum + t.returnPct, 0) / losses.length)
    : 1

  const riskReward = avgLoss > 0 ? avgWin / avgLoss : avgWin

  // Compounded total return
  const totalReturn = completedTrades.reduce((acc, t) => acc * (1 + t.returnPct / 100), 1) - 1

  // Average return per trade (simple average)
  const avgReturn = completedTrades.reduce((sum, t) => sum + t.returnPct, 0) / completedTrades.length

  return {
    winRate,
    riskReward,
    totalReturn: totalReturn * 100,
    avgReturn,
    totalTrades: completedTrades.length,
    wins: wins.length,
    losses: losses.length
  }
}

// Calculate signal based on BX Trender bars (for defensive mode)
// or based on trade history (for aggressive mode)
// IMPORTANT: Signal is based on PREVIOUS MONTH (last completed bar), not current month
export function calculateSignal(shortData, isAggressive = false, trades = []) {
  if (shortData.length < 4) return { signal: 'WAIT', bars: 0 }

  // Use second-to-last bar (previous month = last completed month)
  // Last bar is current month (still open/changing)
  const prevMonthIdx = shortData.length - 2
  const prevMonthValue = shortData[prevMonthIdx].value
  const prevPrevValue = shortData[prevMonthIdx - 1].value
  const isPositive = prevMonthValue > 0

  // Count consecutive bars of the same sign from the previous month backwards
  let consecutiveBars = 1
  for (let i = prevMonthIdx - 1; i >= 0; i--) {
    const val = shortData[i].value
    if ((val > 0) === isPositive) {
      consecutiveBars++
    } else {
      break
    }
  }

  if (isAggressive) {
    // AGGRESSIVE MODE SIGNALS (based on trade history):
    // Signal is based on PREVIOUS MONTH (last completed month)
    // BUY: BUY triggered last/prev month AND open position exists
    // SELL: SELL triggered last/prev month AND no open position
    // HOLD: No recent BUY but open position exists
    // WAIT: No open position and no recent SELL

    const now = new Date()
    // We look at last month and month before (since current month is ignored)
    const lastMonth = now.getMonth() === 0 ? 11 : now.getMonth() - 1
    const lastMonthYear = now.getMonth() === 0 ? now.getFullYear() - 1 : now.getFullYear()
    const prevMonth = lastMonth === 0 ? 11 : lastMonth - 1
    const prevMonthYear = lastMonth === 0 ? lastMonthYear - 1 : lastMonthYear

    // Check for open position
    const hasOpenPosition = trades.some(t => t.isOpen)

    // Check for recent BUY/SELL (last month or month before)
    let recentBuy = false
    let recentSell = false
    let buyBars = 0
    let sellBars = 0

    for (const trade of trades) {
      if (trade.entryDate) {
        const entryDate = new Date(trade.entryDate * 1000)
        const entryMonth = entryDate.getMonth()
        const entryYear = entryDate.getFullYear()

        const isLastMonth = entryMonth === lastMonth && entryYear === lastMonthYear
        const isPrevMonth = entryMonth === prevMonth && entryYear === prevMonthYear

        if ((isLastMonth || isPrevMonth) && trade.isOpen) {
          recentBuy = true
          buyBars = isLastMonth ? 1 : 2
        }
      }

      if (trade.exitDate && !trade.isOpen) {
        const exitDate = new Date(trade.exitDate * 1000)
        const exitMonth = exitDate.getMonth()
        const exitYear = exitDate.getFullYear()

        const isLastMonth = exitMonth === lastMonth && exitYear === lastMonthYear
        const isPrevMonth = exitMonth === prevMonth && exitYear === prevMonthYear

        if (isLastMonth || isPrevMonth) {
          recentSell = true
          sellBars = isLastMonth ? 1 : 2
        }
      }
    }

    // BUY: Recent buy with open position
    if (recentBuy && hasOpenPosition) {
      return { signal: 'BUY', bars: buyBars }
    }

    // SELL: Recent sell and no open position
    if (recentSell && !hasOpenPosition) {
      return { signal: 'SELL', bars: sellBars }
    }

    // HOLD: Open position but no recent buy
    if (hasOpenPosition) {
      return { signal: 'HOLD', bars: consecutiveBars }
    }

    // WAIT: No open position and no recent sell
    return { signal: 'WAIT', bars: consecutiveBars }
  } else {
    // DEFENSIVE MODE SIGNALS (original logic):
    // BUY: First 1-2 green bars
    // HOLD: Green continuation (3+ bars)
    // SELL: First 1-2 red bars
    // WAIT: Red continuation (3+ bars)

    if (isPositive) {
      if (consecutiveBars <= 2) {
        return { signal: 'BUY', bars: consecutiveBars }
      } else {
        return { signal: 'HOLD', bars: consecutiveBars }
      }
    } else {
      if (consecutiveBars <= 2) {
        return { signal: 'SELL', bars: consecutiveBars }
      } else {
        return { signal: 'WAIT', bars: consecutiveBars }
      }
    }
  }
}

// Save performance data to backend
export async function savePerformanceToBackend(symbol, name, metrics, trades, shortData, currentPrice, isAggressive = false, marketCap = 0) {
  try {
    // Pass trades to calculateSignal for aggressive mode (signal based on trade history)
    const { signal, bars } = calculateSignal(shortData, isAggressive, trades)
    const endpoint = isAggressive ? '/api/performance/aggressive' : '/api/performance'

    const res = await fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        symbol,
        name,
        win_rate: metrics.winRate,
        risk_reward: metrics.riskReward,
        total_return: metrics.totalReturn,
        avg_return: metrics.avgReturn,
        total_trades: metrics.totalTrades,
        wins: metrics.wins,
        losses: metrics.losses,
        signal,
        signal_bars: bars,
        trades,
        current_price: currentPrice,
        market_cap: marketCap
      })
    })

    if (!res.ok) {
      console.warn(`Failed to save ${isAggressive ? 'aggressive' : 'defensive'} performance for ${symbol}: ${res.status}`)
    }
  } catch (err) {
    console.warn('Failed to save performance data:', err)
  }
}

// Save Quant mode performance data to backend
export async function saveQuantPerformanceToBackend(symbol, name, metrics, trades, shortData, longData, currentPrice, marketCap = 0) {
  try {
    // Calculate signal for Quant mode: based on alignment of both indicators
    const { signal, bars } = calculateQuantSignal(shortData, longData, trades)

    const res = await fetch('/api/performance/quant', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        symbol,
        name,
        win_rate: metrics.winRate,
        risk_reward: metrics.riskReward,
        total_return: metrics.totalReturn,
        avg_return: metrics.avgReturn,
        total_trades: metrics.totalTrades,
        wins: metrics.wins,
        losses: metrics.losses,
        signal,
        signal_bars: bars,
        trades,
        current_price: currentPrice,
        market_cap: marketCap
      })
    })

    if (!res.ok) {
      console.warn(`Failed to save quant performance for ${symbol}: ${res.status}`)
    }
  } catch (err) {
    console.warn('Failed to save quant performance data:', err)
  }
}

// Calculate signal for Ditz mode (hold through mixed signals)
function calculateDitzSignal(shortData, longData, trades) {
  if (!shortData || shortData.length < 2 || !longData || longData.length < 2) {
    return { signal: 'WAIT', bars: 0 }
  }

  const idx = shortData.length - 2
  const shortVal = shortData[idx].value
  const longVal = longData[idx].value

  const hasOpenPosition = trades && trades.some(t => t.isOpen)

  let consecutiveBars = 1
  const bothPositive = shortVal > 0 && longVal > 0
  const bothNegative = shortVal < 0 && longVal < 0

  for (let i = idx - 1; i >= 0; i--) {
    const sv = shortData[i].value
    const lv = longData[i].value
    const wasPositive = sv > 0 && lv > 0
    const wasNegative = sv < 0 && lv < 0

    if ((bothPositive && wasPositive) || (bothNegative && wasNegative)) {
      consecutiveBars++
    } else {
      break
    }
  }

  if (bothPositive) {
    return consecutiveBars <= 2
      ? { signal: 'BUY', bars: consecutiveBars }
      : { signal: 'HOLD', bars: consecutiveBars }
  } else if (bothNegative) {
    return consecutiveBars <= 2
      ? { signal: 'SELL', bars: consecutiveBars }
      : { signal: 'WAIT', bars: consecutiveBars }
  } else {
    // Mixed signals - HOLD position (don't exit until both negative)
    if (hasOpenPosition) {
      return { signal: 'HOLD', bars: 1 }
    }
    return { signal: 'WAIT', bars: 1 }
  }
}

// Save Ditz mode performance data to backend
export async function saveDitzPerformanceToBackend(symbol, name, metrics, trades, shortData, longData, currentPrice, marketCap = 0) {
  try {
    const { signal, bars } = calculateDitzSignal(shortData, longData, trades)

    const res = await fetch('/api/performance/ditz', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        symbol,
        name,
        win_rate: metrics.winRate,
        risk_reward: metrics.riskReward,
        total_return: metrics.totalReturn,
        avg_return: metrics.avgReturn,
        total_trades: metrics.totalTrades,
        wins: metrics.wins,
        losses: metrics.losses,
        signal,
        signal_bars: bars,
        trades,
        current_price: currentPrice,
        market_cap: marketCap
      })
    })

    if (!res.ok) {
      console.warn(`Failed to save ditz performance for ${symbol}: ${res.status}`)
    }
  } catch (err) {
    console.warn('Failed to save ditz performance data:', err)
  }
}

// Calculate signal for Quant mode (based on both indicators alignment)
function calculateQuantSignal(shortData, longData, trades) {
  if (!shortData || shortData.length < 2 || !longData || longData.length < 2) {
    return { signal: 'WAIT', bars: 0 }
  }

  // Use second-to-last bar (previous month = last completed)
  const idx = shortData.length - 2
  const shortVal = shortData[idx].value
  const longVal = longData[idx].value

  // Check if we have an open position
  const hasOpenPosition = trades && trades.some(t => t.isOpen)

  // Count consecutive aligned bars
  let consecutiveBars = 1
  const bothPositive = shortVal > 0 && longVal > 0
  const bothNegative = shortVal < 0 && longVal < 0

  for (let i = idx - 1; i >= 0; i--) {
    const sv = shortData[i].value
    const lv = longData[i].value
    const wasPositive = sv > 0 && lv > 0
    const wasNegative = sv < 0 && lv < 0

    if ((bothPositive && wasPositive) || (bothNegative && wasNegative)) {
      consecutiveBars++
    } else {
      break
    }
  }

  if (bothPositive) {
    // Both indicators positive - BUY for first 2 bars, then HOLD
    return consecutiveBars <= 2
      ? { signal: 'BUY', bars: consecutiveBars }
      : { signal: 'HOLD', bars: consecutiveBars }
  } else if (bothNegative) {
    // Both indicators negative
    return consecutiveBars <= 2
      ? { signal: 'SELL', bars: consecutiveBars }
      : { signal: 'WAIT', bars: consecutiveBars }
  } else {
    // Mixed signals - potential exit
    if (hasOpenPosition) {
      return { signal: 'SELL', bars: 1 }
    }
    return { signal: 'WAIT', bars: 1 }
  }
}

// Fetch historical data from backend
export async function fetchHistoricalData(symbol) {
  const url = `/api/history/${symbol}?period=max&interval=1mo`

  const res = await fetch(url)
  if (!res.ok) {
    const errorText = await res.text().catch(() => 'Unknown error')
    throw new Error(`Failed to fetch data for ${symbol}: ${res.status} - ${errorText}`)
  }

  const json = await res.json()
  if (!json.data) {
    throw new Error(`No data field in response for ${symbol}`)
  }
  return json
}

// Fetch market cap for a single stock
export async function fetchMarketCap(symbol) {
  try {
    const res = await fetch(`/api/quote/${symbol}`)
    if (!res.ok) return 0
    const data = await res.json()
    return data.market_cap || 0
  } catch {
    return 0
  }
}

// Process a single stock and save all three modes' performance
export async function processStock(symbol, name) {
  try {
    // Fetch config and data in parallel
    const [configData, quantConfig, ditzConfig, historyData, marketCap] = await Promise.all([
      fetchBXtrenderConfig(),
      fetchBXtrenderQuantConfig(),
      fetchBXtrenderDitzConfig(),
      fetchHistoricalData(symbol),
      fetchMarketCap(symbol)
    ])

    const data = historyData.data
    if (!data || data.length === 0) {
      return { success: false, error: 'No data' }
    }

    const currentPrice = data[data.length - 1].close

    // Nur abgeschlossene Monatskerzen verwenden (aktuellen unvollständigen Monat entfernen)
    let monthlyData = data
    const now = new Date()
    if (monthlyData.length > 0) {
      const lastTime = new Date(monthlyData[monthlyData.length - 1].time * 1000)
      if (lastTime.getUTCFullYear() === now.getUTCFullYear() && lastTime.getUTCMonth() === now.getUTCMonth()) {
        monthlyData = monthlyData.slice(0, -1)
      }
    }

    // Calculate and save defensive mode
    const defensiveResult = calculateBXtrender(monthlyData, false, configData.defensive)
    const defensiveMetrics = calculateMetrics(defensiveResult.trades)
    await savePerformanceToBackend(symbol, name, defensiveMetrics, defensiveResult.trades, defensiveResult.short, currentPrice, false, marketCap)

    // Calculate and save aggressive mode
    const aggressiveResult = calculateBXtrender(monthlyData, true, configData.aggressive)
    const aggressiveMetrics = calculateMetrics(aggressiveResult.trades)
    await savePerformanceToBackend(symbol, name, aggressiveMetrics, aggressiveResult.trades, aggressiveResult.short, currentPrice, true, marketCap)

    // Calculate and save quant mode
    const quantResult = calculateBXtrenderQuant(monthlyData, quantConfig)
    const quantMetrics = calculateMetrics(quantResult.trades)
    await saveQuantPerformanceToBackend(symbol, name, quantMetrics, quantResult.trades, quantResult.short, quantResult.long, currentPrice, marketCap)

    // Calculate and save ditz mode
    const ditzResult = calculateBXtrenderQuant(monthlyData, ditzConfig, 'ditz')
    const ditzMetrics = calculateMetrics(ditzResult.trades)
    await saveDitzPerformanceToBackend(symbol, name, ditzMetrics, ditzResult.trades, ditzResult.short, ditzResult.long, currentPrice, marketCap)

    return { success: true }
  } catch (err) {
    return { success: false, error: err.message }
  }
}
