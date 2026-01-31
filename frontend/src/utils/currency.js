// Direct currency conversion utilities - no React context
// Fallback rates if API fails (approximate values)
const FALLBACK_RATES = {
  USD: 1,
  EUR: 0.92,
  GBP: 0.79,
  CHF: 0.88,
  HKD: 7.8,
  JPY: 150,
  CNY: 7.2,
  KRW: 1350,
  TWD: 32,
  INR: 83,
  AUD: 1.55,
  CAD: 1.36
}

export const CURRENCY_SYMBOLS = {
  USD: '$',
  EUR: '€',
  GBP: '£',
  CHF: 'CHF'
}

// Detect stock's native currency based on exchange suffix
// Returns null if USD (no conversion needed as that's the base)
export function getStockCurrency(symbol) {
  if (!symbol) return null
  const s = symbol.toUpperCase()

  // European exchanges - EUR
  if (s.endsWith('.PA') || s.endsWith('.DE') || s.endsWith('.F') ||
      s.endsWith('.AS') || s.endsWith('.BR') || s.endsWith('.MI') ||
      s.endsWith('.MC') || s.endsWith('.VI') || s.endsWith('.HE') ||
      s.endsWith('.LS') || s.endsWith('.IR')) {
    return 'EUR'
  }
  // London - GBP (pence, but Yahoo converts to GBP)
  if (s.endsWith('.L')) {
    return 'GBP'
  }
  // Swiss - CHF
  if (s.endsWith('.SW') || s.endsWith('.VX')) {
    return 'CHF'
  }
  // Hong Kong - HKD
  if (s.endsWith('.HK')) {
    return 'HKD'
  }
  // Japan - JPY
  if (s.endsWith('.T') || s.endsWith('.TYO')) {
    return 'JPY'
  }
  // China - CNY
  if (s.endsWith('.SS') || s.endsWith('.SZ')) {
    return 'CNY'
  }
  // Korea - KRW
  if (s.endsWith('.KS') || s.endsWith('.KQ')) {
    return 'KRW'
  }
  // Taiwan - TWD
  if (s.endsWith('.TW') || s.endsWith('.TWO')) {
    return 'TWD'
  }
  // India - INR
  if (s.endsWith('.NS') || s.endsWith('.BO')) {
    return 'INR'
  }
  // Australia - AUD
  if (s.endsWith('.AX')) {
    return 'AUD'
  }
  // Canada - CAD
  if (s.endsWith('.TO') || s.endsWith('.V')) {
    return 'CAD'
  }
  // US exchanges or no suffix = USD
  return null
}

// Get exchange rates from localStorage or use fallback
function getExchangeRates() {
  try {
    const stored = localStorage.getItem('exchangeRates')
    if (stored) {
      const { rates, timestamp } = JSON.parse(stored)
      // Use cached rates if less than 1 hour old
      if (Date.now() - timestamp < 3600000) {
        return rates
      }
    }
  } catch {
    // Ignore parse errors
  }
  return FALLBACK_RATES
}

// Fetch live exchange rates from API
export async function fetchExchangeRates() {
  try {
    // Frankfurter API for major currencies
    const res = await fetch('https://api.frankfurter.app/latest?from=USD&to=EUR,GBP,CHF,JPY,AUD,CAD')
    if (!res.ok) throw new Error('Frankfurter API error')
    const data = await res.json()

    const rates = {
      USD: 1,
      EUR: data.rates.EUR,
      GBP: data.rates.GBP,
      CHF: data.rates.CHF,
      JPY: data.rates.JPY,
      AUD: data.rates.AUD,
      CAD: data.rates.CAD,
      // Frankfurter doesn't have Asian currencies, use fallbacks initially
      HKD: FALLBACK_RATES.HKD,
      CNY: FALLBACK_RATES.CNY,
      KRW: FALLBACK_RATES.KRW,
      TWD: FALLBACK_RATES.TWD,
      INR: FALLBACK_RATES.INR
    }

    // Try to get Asian currencies from exchangerate-api (free tier)
    try {
      const asianRes = await fetch('https://open.er-api.com/v6/latest/USD')
      if (asianRes.ok) {
        const asianData = await asianRes.json()
        if (asianData.rates) {
          if (asianData.rates.HKD) rates.HKD = asianData.rates.HKD
          if (asianData.rates.CNY) rates.CNY = asianData.rates.CNY
          if (asianData.rates.KRW) rates.KRW = asianData.rates.KRW
          if (asianData.rates.TWD) rates.TWD = asianData.rates.TWD
          if (asianData.rates.INR) rates.INR = asianData.rates.INR
        }
      }
    } catch {
      // Use fallbacks for Asian currencies
    }

    // Cache rates with timestamp
    localStorage.setItem('exchangeRates', JSON.stringify({
      rates,
      timestamp: Date.now()
    }))

    // Notify components of rate update
    window.dispatchEvent(new Event('currencyChanged'))

    return rates
  } catch (err) {
    console.warn('Failed to fetch exchange rates, using fallback:', err)
    return FALLBACK_RATES
  }
}

// Initialize rates on load
fetchExchangeRates()

export const EXCHANGE_RATES = getExchangeRates()

export function getCurrency() {
  return localStorage.getItem('currency') || 'EUR'
}

export function setCurrency(currency) {
  localStorage.setItem('currency', currency)
  window.dispatchEvent(new Event('currencyChanged'))
}

export function formatPrice(usdPrice, stockSymbol = null) {
  if (usdPrice === null || usdPrice === undefined) return '--'
  const targetCurrency = getCurrency()
  const rates = getExchangeRates()
  const symbol = CURRENCY_SYMBOLS[targetCurrency] || '$'

  // Check if stock is traded in a non-USD currency
  const stockCurrency = getStockCurrency(stockSymbol)

  let converted = usdPrice
  if (stockCurrency) {
    // Stock is already in EUR/GBP/CHF, convert from that currency to target
    const stockRate = rates[stockCurrency] || 1
    const targetRate = rates[targetCurrency] || 1
    // First convert to USD (divide by stock rate), then to target (multiply by target rate)
    converted = (usdPrice / stockRate) * targetRate
  } else {
    // Stock is in USD, convert directly to target
    const rate = rates[targetCurrency] || 1
    converted = usdPrice * rate
  }

  if (targetCurrency === 'CHF') {
    return `${symbol} ${converted.toLocaleString('de-CH', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
  }
  return `${symbol}${converted.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

export function formatChange(usdChange, percent, stockSymbol = null) {
  if (usdChange === undefined || usdChange === null) return null
  const targetCurrency = getCurrency()
  const rates = getExchangeRates()
  const symbol = CURRENCY_SYMBOLS[targetCurrency] || '$'

  // Check if stock is traded in a non-USD currency
  const stockCurrency = getStockCurrency(stockSymbol)

  let converted = usdChange
  if (stockCurrency) {
    // Stock is already in EUR/GBP/CHF, convert from that currency to target
    const stockRate = rates[stockCurrency] || 1
    const targetRate = rates[targetCurrency] || 1
    converted = (usdChange / stockRate) * targetRate
  } else {
    // Stock is in USD, convert directly to target
    const rate = rates[targetCurrency] || 1
    converted = usdChange * rate
  }

  const isPositive = converted >= 0
  const sign = isPositive ? '+' : ''

  return {
    text: `${sign}${targetCurrency === 'CHF' ? 'CHF ' : symbol}${Math.abs(converted).toFixed(2)} (${sign}${percent?.toFixed(2) || '0.00'}%)`,
    isPositive
  }
}
