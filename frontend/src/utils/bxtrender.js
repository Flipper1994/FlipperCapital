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

// Cache for Trader config
let traderConfigCache = null
let traderConfigPromise = null

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

// Fetch Trader config from backend
export async function fetchBXtrenderTraderConfig() {
  if (traderConfigCache) return traderConfigCache
  if (traderConfigPromise) return traderConfigPromise

  traderConfigPromise = fetch('/api/bxtrender-trader-config')
    .then(res => res.json())
    .then(data => {
      traderConfigCache = data
      return data
    })
    .catch(() => {
      return { ...DEFAULT_QUANT_CONFIG, maFilterOn: false }
    })

  return traderConfigPromise
}

// Clear trader config cache
export function clearTraderConfigCache() {
  traderConfigCache = null
  traderConfigPromise = null
}

export function calculateBXtrender(ohlcv, isAggressive = false, config = null, nextOpen = null) {
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
        if ((isLightRed && consecutiveLightRed === 1) || justTurnedGreen) {
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

    // SELL always takes priority over BUY (if both fire simultaneously, SELL wins)
    if (sellSignal) {
      // IMPORTANT: Signal is evaluated at month end, so trade happens at START of NEXT month
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
      } else if (nextOpen && nextOpen.open > 0 && entryPrice > 0) {
        // Last candle signal — sell at next month's open
        const exitPrice = nextOpen.open
        const returnPct = ((exitPrice - entryPrice) / entryPrice) * 100

        trades.push({
          entryDate: entryDate,
          entryPrice: entryPrice,
          exitDate: nextOpen.time,
          exitPrice: exitPrice,
          returnPct: returnPct,
          isOpen: false
        })

        const returnText = returnPct >= 0 ? `+${returnPct.toFixed(1)}%` : `${returnPct.toFixed(1)}%`
        markers.push({
          time: nextOpen.time,
          position: 'aboveBar',
          color: '#FF0000',
          shape: 'arrowDown',
          text: `SELL $${exitPrice.toFixed(2)} ${returnText}`
        })

        inPosition = false
        entryPrice = 0
        entryDate = null
      }
    } else if (buySignal) {
      // IMPORTANT: Signal is evaluated at month end, so trade happens at START of NEXT month
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
      } else if (nextOpen && nextOpen.open > 0) {
        // Last candle signal — use next month's open from current (stripped) data
        inPosition = true
        entryPrice = nextOpen.open
        entryDate = nextOpen.time

        markers.push({
          time: nextOpen.time,
          position: 'belowBar',
          color: '#00FF00',
          shape: 'arrowUp',
          text: `BUY $${nextOpen.open.toFixed(2)}`
        })
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
    // Use nextOpen's close (actual current price) if entry was via nextOpen
    const currentPrice = (nextOpen && nextOpen.close > 0) ? nextOpen.close : closes[lastIdx]
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
export function calculateBXtrenderQuant(ohlcv, config = null, mode = 'quant', nextOpen = null) {
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
  let highestPrice = 0
  const tslEnabled = cfg.tslEnabled !== undefined ? cfg.tslEnabled : cfg.tsl_enabled !== undefined ? cfg.tsl_enabled : true
  const tslPercent = cfg.tslPercent || cfg.tsl_percent || 20.0

  for (let i = startIdx; i < closes.length; i++) {
    const shortVal = shortTermXtrender[i]
    const shortPrev = shortTermXtrender[i - 1]
    const longVal = longTermXtrender[i]
    const longPrev = longTermXtrender[i - 1]
    const sigVal = signalLine[i]
    const sigPrev = signalLine[i - 1]
    const price = closes[i]
    const maVal = ma ? ma[i] : 0

    // Track highest price since entry for TSL
    if (inPosition && price > highestPrice) {
      highestPrice = price
    }

    // Check trailing stop loss
    let tslTriggered = false
    if (tslEnabled && inPosition && highestPrice > 0) {
      const stopPrice = highestPrice * (1 - tslPercent / 100)
      if (price <= stopPrice) {
        tslTriggered = true
      }
    }

    // Coloring logic
    let shortColor, longColor
    const bothPositive = shortVal > 0 && longVal > 0
    const bothNegative = shortVal < 0 && longVal < 0

    if (mode === 'trader') {
      // Trader mode: coloring based on signal line direction
      const sigRising = sigVal > sigPrev
      if (sigRising) {
        shortColor = shortVal > shortPrev ? '#00FF00' : '#228B22'
        longColor = longVal > longPrev ? '#00FF00' : '#228B22'
      } else {
        shortColor = shortVal > shortPrev ? '#FF0000' : '#8B0000'
        longColor = longVal > longPrev ? '#FF0000' : '#8B0000'
      }
    } else if (bothPositive) {
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

    let entrySignal, exitSignal

    if (mode === 'trader') {
      // Trader mode: based on T3 signal line direction changes
      const signalRising = sigVal > sigPrev
      const signalRisingPrev = sigPrev > (i >= 2 ? signalLine[i - 2] : sigPrev)

      // BUY when signal line turns from falling to rising (Red→Green)
      entrySignal = !inPosition && signalRising && !signalRisingPrev
      // SELL when signal line turns from rising to falling (Green→Red) OR TSL
      exitSignal = inPosition && ((!signalRising && signalRisingPrev) || tslTriggered)
    } else if (mode === 'ditz') {
      // Ditz mode: both indicators must be positive
      const bothPositivePrev = shortPrev > 0 && longPrev > 0

      // BUY when both turn positive
      entrySignal = !inPosition && bothPositive && (!bothPositivePrev || !inPosition) && maFilterLong
      // SELL when both negative OR trailing stop loss
      exitSignal = inPosition && (bothNegative || tslTriggered)
    } else {
      // Quant mode: both indicators alignment
      entrySignal = !inPosition &&
        shortVal > 0 && longVal > 0 &&
        maFilterLong &&
        (shortPrev <= 0 || longPrev <= 0)

      // SELL when either negative OR trailing stop loss
      exitSignal = inPosition && ((shortVal < 0 || longVal < 0) || tslTriggered)
    }

    // SELL always takes priority over BUY (if both fire simultaneously, SELL wins)
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
          isOpen: false,
          exitReason: tslTriggered ? 'TSL' : 'SIGNAL'
        })

        const returnText = returnPct >= 0 ? `+${returnPct.toFixed(1)}%` : `${returnPct.toFixed(1)}%`
        markers.push({
          time: times[i + 1],
          position: 'aboveBar',
          color: tslTriggered ? '#FFA500' : '#FF0000',
          shape: 'arrowDown',
          text: `${tslTriggered ? 'TSL' : 'SELL'} $${exitPrice.toFixed(2)} ${returnText}`
        })

        inPosition = false
        entryPrice = 0
        entryDate = null
        highestPrice = 0
      } else if (nextOpen && nextOpen.open > 0) {
        // Last candle signal — sell at next month's open
        const exitPrice = nextOpen.open
        const returnPct = ((exitPrice - entryPrice) / entryPrice) * 100

        trades.push({
          entryDate: entryDate,
          entryPrice: entryPrice,
          exitDate: nextOpen.time,
          exitPrice: exitPrice,
          returnPct: returnPct,
          isOpen: false,
          exitReason: tslTriggered ? 'TSL' : 'SIGNAL'
        })

        const returnText = returnPct >= 0 ? `+${returnPct.toFixed(1)}%` : `${returnPct.toFixed(1)}%`
        markers.push({
          time: nextOpen.time,
          position: 'aboveBar',
          color: tslTriggered ? '#FFA500' : '#FF0000',
          shape: 'arrowDown',
          text: `${tslTriggered ? 'TSL' : 'SELL'} $${exitPrice.toFixed(2)} ${returnText}`
        })

        inPosition = false
        entryPrice = 0
        entryDate = null
        highestPrice = 0
      }
    } else if (entrySignal && opens[i] > 0) {
      // Enter at next bar open (signal evaluated at bar close)
      if (i + 1 < closes.length && opens[i + 1] > 0) {
        inPosition = true
        entryPrice = opens[i + 1]
        entryDate = times[i + 1]
        highestPrice = opens[i + 1]

        markers.push({
          time: times[i + 1],
          position: 'belowBar',
          color: '#00FF00',
          shape: 'arrowUp',
          text: `BUY $${opens[i + 1].toFixed(2)}`
        })
      } else if (nextOpen && nextOpen.open > 0) {
        // Last candle signal — use next month's open from current (stripped) data
        inPosition = true
        entryPrice = nextOpen.open
        entryDate = nextOpen.time
        highestPrice = nextOpen.open

        markers.push({
          time: nextOpen.time,
          position: 'belowBar',
          color: '#00FF00',
          shape: 'arrowUp',
          text: `BUY $${nextOpen.open.toFixed(2)}`
        })
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
    const currentPrice = (nextOpen && nextOpen.close > 0) ? nextOpen.close : closes[lastIdx]
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
  } else if (!inPosition && closes.length > 0) {
    // Not in position - check if current conditions warrant a BUY signal
    const lastIdx = closes.length - 1
    const lastPrice = closes[lastIdx]
    const lastMA = ma ? ma[lastIdx] : 0
    const maConditionMet = !maFilterOn || lastPrice > lastMA

    let shouldBuy = false
    if (mode === 'trader') {
      // Signal line turning from falling to rising (Red→Green) at last bar
      const sigLast = signalLine[lastIdx]
      const sigPrev = lastIdx >= 1 ? signalLine[lastIdx - 1] : sigLast
      const sigPrevPrev = lastIdx >= 2 ? signalLine[lastIdx - 2] : sigPrev
      const sigRising = sigLast > sigPrev
      const sigRisingPrev = sigPrev > sigPrevPrev
      shouldBuy = sigRising && !sigRisingPrev
    } else if (mode === 'ditz') {
      // Both oscillators must be positive
      shouldBuy = shortTermXtrender[lastIdx] > 0 && longTermXtrender[lastIdx] > 0 && maConditionMet
    } else {
      shouldBuy = shortTermXtrender[lastIdx] > 0 && longTermXtrender[lastIdx] > 0 && maConditionMet
    }

    // Don't re-enter if we just exited via nextOpen on the same bar (SELL wins over BUY)
    if (shouldBuy && trades.length > 0 && nextOpen) {
      const lastTrade = trades[trades.length - 1]
      if (!lastTrade.isOpen && lastTrade.exitDate === nextOpen.time) {
        shouldBuy = false
      }
    }

    if (shouldBuy) {
      // Find the correct historical entry point instead of using last candle price
      // Search from after the last exit trade to find when entry conditions first appeared
      let searchStart = startIdx
      if (trades.length > 0) {
        const lastTrade = trades[trades.length - 1]
        if (lastTrade.exitDate) {
          for (let j = startIdx; j <= lastIdx; j++) {
            if (times[j] === lastTrade.exitDate) {
              searchStart = j
              break
            }
          }
        }
      }

      let entryIdx = -1
      for (let j = searchStart; j < lastIdx; j++) {
        let signalFound = false
        if (mode === 'trader') {
          // Signal line turning from falling to rising
          const sigJ = signalLine[j]
          const sigJPrev = j >= 1 ? signalLine[j - 1] : sigJ
          const sigJPrevPrev = j >= 2 ? signalLine[j - 2] : sigJPrev
          const rising = sigJ > sigJPrev
          const risingPrev = sigJPrev > sigJPrevPrev
          signalFound = rising && !risingPrev
        } else if (mode === 'ditz') {
          // Both oscillators must be positive
          const bothPos = shortTermXtrender[j] > 0 && longTermXtrender[j] > 0
          const bothPosPrev = shortTermXtrender[j - 1] > 0 && longTermXtrender[j - 1] > 0
          const maCond = !maFilterOn || closes[j] > (ma ? ma[j] : 0)
          signalFound = bothPos && !bothPosPrev && maCond
        } else {
          const bothPos = shortTermXtrender[j] > 0 && longTermXtrender[j] > 0
          const prevNotBoth = shortTermXtrender[j - 1] <= 0 || longTermXtrender[j - 1] <= 0
          const maCond = !maFilterOn || closes[j] > (ma ? ma[j] : 0)
          signalFound = bothPos && prevNotBoth && maCond
        }
        if (signalFound && j + 1 <= lastIdx && opens[j + 1] > 0) {
          entryIdx = j + 1
          break
        }
      }

      if (entryIdx >= 0) {
        // Signal found at historical candle — entry at next month's open (correct)
        const actualEntryPrice = opens[entryIdx] > 0 ? opens[entryIdx] : closes[entryIdx]
        const currentP = (nextOpen && nextOpen.close > 0) ? nextOpen.close : lastPrice
        const unrealizedReturn = actualEntryPrice > 0 ? ((currentP - actualEntryPrice) / actualEntryPrice) * 100 : 0

        trades.push({
          entryDate: times[entryIdx],
          entryPrice: actualEntryPrice,
          exitDate: null,
          exitPrice: null,
          currentPrice: currentP,
          returnPct: unrealizedReturn,
          isOpen: true
        })

        markers.push({
          time: times[entryIdx],
          position: 'belowBar',
          color: '#00FF00',
          shape: 'arrowUp',
          text: `BUY $${actualEntryPrice.toFixed(2)}`
        })
      } else if (nextOpen && nextOpen.open > 0) {
        // Signal first appeared on last candle — entry at next month's open
        const currentP = nextOpen.close > 0 ? nextOpen.close : lastPrice
        const unrealizedReturn = nextOpen.open > 0 ? ((currentP - nextOpen.open) / nextOpen.open) * 100 : 0

        trades.push({
          entryDate: nextOpen.time,
          entryPrice: nextOpen.open,
          exitDate: null,
          exitPrice: null,
          currentPrice: currentP,
          returnPct: unrealizedReturn,
          isOpen: true
        })

        markers.push({
          time: nextOpen.time,
          position: 'belowBar',
          color: '#00FF00',
          shape: 'arrowUp',
          text: `BUY $${nextOpen.open.toFixed(2)}`
        })
      }
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
  if (shortData.length < 4) return { signal: 'NO_DATA', bars: 0 }

  // Unified HOLD/WAIT logic based on trade history (all modes)
  const hasOpenPosition = trades.some(t => t.isOpen)

  if (hasOpenPosition) {
    // Find the most recent BUY entry and count bars since then
    const openTrade = trades.find(t => t.isOpen)
    if (openTrade && openTrade.entryDate) {
      // Count bars since entry
      let barsSinceBuy = 0
      for (let i = shortData.length - 1; i >= 0; i--) {
        if (shortData[i].time === openTrade.entryDate) {
          barsSinceBuy = shortData.length - 1 - i
          break
        }
      }
      if (barsSinceBuy <= 1) {
        return { signal: 'BUY', bars: barsSinceBuy }
      }
      return { signal: 'HOLD', bars: barsSinceBuy }
    }
    return { signal: 'HOLD', bars: 1 }
  } else {
    // No open position — find last SELL
    let barsSinceSell = 0
    let foundSell = false
    const closedTrades = trades.filter(t => !t.isOpen && t.exitDate)
    if (closedTrades.length > 0) {
      const lastSell = closedTrades[closedTrades.length - 1]
      for (let i = shortData.length - 1; i >= 0; i--) {
        if (shortData[i].time === lastSell.exitDate) {
          barsSinceSell = shortData.length - 1 - i
          foundSell = true
          break
        }
      }
    }
    if (foundSell && barsSinceSell <= 1) {
      return { signal: 'SELL', bars: barsSinceSell }
    }
    return { signal: 'WAIT', bars: barsSinceSell }
  }
}

// Save performance data to backend
export async function savePerformanceToBackend(symbol, name, metrics, trades, shortData, currentPrice, isAggressive = false, marketCap = 0) {
  try {
    // Pass trades to calculateSignal for aggressive mode (signal based on trade history)
    const { signal, bars } = calculateSignal(shortData, isAggressive, trades)
    if (signal === 'NO_DATA') return
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
    if (signal === 'NO_DATA') return

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

// Calculate signal for Ditz mode — unified HOLD/WAIT based on trade history
export function calculateDitzSignal(signalData, trades) {
  if (!signalData || signalData.length < 3) return { signal: 'NO_DATA', bars: 0 }

  const hasOpenPosition = trades && trades.some(t => t.isOpen)

  if (hasOpenPosition) {
    const openTrade = trades.find(t => t.isOpen)
    if (openTrade && openTrade.entryDate) {
      let barsSinceBuy = 0
      for (let i = signalData.length - 1; i >= 0; i--) {
        if (signalData[i].time === openTrade.entryDate) {
          barsSinceBuy = signalData.length - 1 - i
          break
        }
      }
      if (barsSinceBuy <= 1) {
        return { signal: 'BUY', bars: barsSinceBuy }
      }
      return { signal: 'HOLD', bars: barsSinceBuy }
    }
    return { signal: 'HOLD', bars: 1 }
  } else {
    let barsSinceSell = 0
    let foundSell = false
    const closedTrades = trades ? trades.filter(t => !t.isOpen && t.exitDate) : []
    if (closedTrades.length > 0) {
      const lastSell = closedTrades[closedTrades.length - 1]
      for (let i = signalData.length - 1; i >= 0; i--) {
        if (signalData[i].time === lastSell.exitDate) {
          barsSinceSell = signalData.length - 1 - i
          foundSell = true
          break
        }
      }
    }
    if (foundSell && barsSinceSell <= 1) {
      return { signal: 'SELL', bars: barsSinceSell }
    }
    return { signal: 'WAIT', bars: barsSinceSell }
  }
}

// Save Ditz mode performance data to backend
export async function saveDitzPerformanceToBackend(symbol, name, metrics, trades, signalData, currentPrice, marketCap = 0) {
  try {
    const { signal, bars } = calculateDitzSignal(signalData, trades)
    if (signal === 'NO_DATA') return

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

// Calculate signal for Trader mode (same as Ditz - based on signal line color changes)
export function calculateTraderSignal(signalData, trades) {
  return calculateDitzSignal(signalData, trades)
}

// Save Trader mode performance data to backend
export async function saveTraderPerformanceToBackend(symbol, name, metrics, trades, signalData, currentPrice, marketCap = 0) {
  try {
    const { signal, bars } = calculateTraderSignal(signalData, trades)
    if (signal === 'NO_DATA') return

    const res = await fetch('/api/performance/trader', {
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
      console.warn(`Failed to save trader performance for ${symbol}: ${res.status}`)
    }
  } catch (err) {
    console.warn('Failed to save trader performance data:', err)
  }
}

// Calculate signal for Quant mode — unified HOLD/WAIT based on trade history
export function calculateQuantSignal(shortData, longData, trades) {
  if (!shortData || shortData.length < 2 || !longData || longData.length < 2) {
    return { signal: 'NO_DATA', bars: 0 }
  }

  const hasOpenPosition = trades && trades.some(t => t.isOpen)

  if (hasOpenPosition) {
    const openTrade = trades.find(t => t.isOpen)
    if (openTrade && openTrade.entryDate) {
      let barsSinceBuy = 0
      for (let i = shortData.length - 1; i >= 0; i--) {
        if (shortData[i].time === openTrade.entryDate) {
          barsSinceBuy = shortData.length - 1 - i
          break
        }
      }
      if (barsSinceBuy <= 1) {
        return { signal: 'BUY', bars: barsSinceBuy }
      }
      return { signal: 'HOLD', bars: barsSinceBuy }
    }
    return { signal: 'HOLD', bars: 1 }
  } else {
    let barsSinceSell = 0
    let foundSell = false
    const closedTrades = trades ? trades.filter(t => !t.isOpen && t.exitDate) : []
    if (closedTrades.length > 0) {
      const lastSell = closedTrades[closedTrades.length - 1]
      for (let i = shortData.length - 1; i >= 0; i--) {
        if (shortData[i].time === lastSell.exitDate) {
          barsSinceSell = shortData.length - 1 - i
          foundSell = true
          break
        }
      }
    }
    if (foundSell && barsSinceSell <= 1) {
      return { signal: 'SELL', bars: barsSinceSell }
    }
    return { signal: 'WAIT', bars: barsSinceSell }
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
    const res = await fetch(`/api/test-marketcap/${symbol}`)
    if (!res.ok) return 0
    const data = await res.json()
    return data.market_cap_raw || 0
  } catch {
    return 0
  }
}

// Process a single stock with pre-fetched configs (for batch mode)
export async function processStockWithConfigs(symbol, name, configData, quantConfig, ditzConfig, traderConfig) {
  try {
    const [historyData, marketCap] = await Promise.all([
      fetchHistoricalData(symbol),
      fetchMarketCap(symbol)
    ])

    const data = historyData.data
    if (!data || data.length === 0) {
      return { success: false, error: 'No data' }
    }

    const currentPrice = data[data.length - 1].close

    const now = new Date()
    let monthlyData = data.filter(d => {
      const t = new Date(d.time * 1000)
      return !(t.getUTCFullYear() === now.getUTCFullYear() && t.getUTCMonth() === now.getUTCMonth())
    })

    const strippedCandles = data.filter(d => {
      const t = new Date(d.time * 1000)
      return t.getUTCFullYear() === now.getUTCFullYear() && t.getUTCMonth() === now.getUTCMonth()
    })
    const nextOpen = strippedCandles.length > 0
      ? { time: strippedCandles[0].time, open: strippedCandles[0].open, close: strippedCandles[strippedCandles.length - 1].close }
      : null

    const defensiveResult = calculateBXtrender(monthlyData, false, configData.defensive, nextOpen)
    const defensiveMetrics = calculateMetrics(defensiveResult.trades)
    await savePerformanceToBackend(symbol, name, defensiveMetrics, defensiveResult.trades, defensiveResult.short, currentPrice, false, marketCap)

    const aggressiveResult = calculateBXtrender(monthlyData, true, configData.aggressive, nextOpen)
    const aggressiveMetrics = calculateMetrics(aggressiveResult.trades)
    await savePerformanceToBackend(symbol, name, aggressiveMetrics, aggressiveResult.trades, aggressiveResult.short, currentPrice, true, marketCap)

    const quantResult = calculateBXtrenderQuant(monthlyData, quantConfig, 'quant', nextOpen)
    const quantMetrics = calculateMetrics(quantResult.trades)
    await saveQuantPerformanceToBackend(symbol, name, quantMetrics, quantResult.trades, quantResult.short, quantResult.long, currentPrice, marketCap)

    const ditzResult = calculateBXtrenderQuant(monthlyData, ditzConfig, 'ditz', nextOpen)
    const ditzMetrics = calculateMetrics(ditzResult.trades)
    await saveDitzPerformanceToBackend(symbol, name, ditzMetrics, ditzResult.trades, ditzResult.signal, currentPrice, marketCap)

    const traderResult = calculateBXtrenderQuant(monthlyData, traderConfig, 'trader', nextOpen)
    const traderMetrics = calculateMetrics(traderResult.trades)
    await saveTraderPerformanceToBackend(symbol, name, traderMetrics, traderResult.trades, traderResult.signal, currentPrice, marketCap)

    return { success: true }
  } catch (err) {
    return { success: false, error: err.message }
  }
}

// Process a single stock and save all three modes' performance
export async function processStock(symbol, name) {
  try {
    // Fetch config and data in parallel
    const [configData, quantConfig, ditzConfig, traderConfig, historyData, marketCap] = await Promise.all([
      fetchBXtrenderConfig(),
      fetchBXtrenderQuantConfig(),
      fetchBXtrenderDitzConfig(),
      fetchBXtrenderTraderConfig(),
      fetchHistoricalData(symbol),
      fetchMarketCap(symbol)
    ])

    const data = historyData.data
    if (!data || data.length === 0) {
      return { success: false, error: 'No data' }
    }

    const currentPrice = data[data.length - 1].close

    // Nur abgeschlossene Monatskerzen verwenden (aktuellen unvollständigen Monat entfernen)
    // Yahoo liefert manchmal mehrere Datenpunkte für den aktuellen Monat
    const now = new Date()
    let monthlyData = data.filter(d => {
      const t = new Date(d.time * 1000)
      return !(t.getUTCFullYear() === now.getUTCFullYear() && t.getUTCMonth() === now.getUTCMonth())
    })

    // Extract next month's open from stripped current-month data for signal execution
    const strippedCandles = data.filter(d => {
      const t = new Date(d.time * 1000)
      return t.getUTCFullYear() === now.getUTCFullYear() && t.getUTCMonth() === now.getUTCMonth()
    })
    const nextOpen = strippedCandles.length > 0
      ? { time: strippedCandles[0].time, open: strippedCandles[0].open, close: strippedCandles[strippedCandles.length - 1].close }
      : null

    // Calculate and save defensive mode
    const defensiveResult = calculateBXtrender(monthlyData, false, configData.defensive, nextOpen)
    const defensiveMetrics = calculateMetrics(defensiveResult.trades)
    await savePerformanceToBackend(symbol, name, defensiveMetrics, defensiveResult.trades, defensiveResult.short, currentPrice, false, marketCap)

    // Calculate and save aggressive mode
    const aggressiveResult = calculateBXtrender(monthlyData, true, configData.aggressive, nextOpen)
    const aggressiveMetrics = calculateMetrics(aggressiveResult.trades)
    await savePerformanceToBackend(symbol, name, aggressiveMetrics, aggressiveResult.trades, aggressiveResult.short, currentPrice, true, marketCap)

    // Calculate and save quant mode
    const quantResult = calculateBXtrenderQuant(monthlyData, quantConfig, 'quant', nextOpen)
    const quantMetrics = calculateMetrics(quantResult.trades)
    await saveQuantPerformanceToBackend(symbol, name, quantMetrics, quantResult.trades, quantResult.short, quantResult.long, currentPrice, marketCap)

    // Calculate and save ditz mode
    const ditzResult = calculateBXtrenderQuant(monthlyData, ditzConfig, 'ditz', nextOpen)
    const ditzMetrics = calculateMetrics(ditzResult.trades)
    await saveDitzPerformanceToBackend(symbol, name, ditzMetrics, ditzResult.trades, ditzResult.signal, currentPrice, marketCap)

    // Calculate and save trader mode
    const traderResult = calculateBXtrenderQuant(monthlyData, traderConfig, 'trader', nextOpen)
    const traderMetrics = calculateMetrics(traderResult.trades)
    await saveTraderPerformanceToBackend(symbol, name, traderMetrics, traderResult.trades, traderResult.signal, currentPrice, marketCap)

    return { success: true }
  } catch (err) {
    return { success: false, error: err.message }
  }
}
