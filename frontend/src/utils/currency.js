// Direct currency conversion utilities - no React context
// Fallback rates if API fails
const FALLBACK_RATES = {
  USD: 1,
  EUR: 0.92,
  GBP: 0.79,
  CHF: 0.88
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
    const res = await fetch('https://api.frankfurter.app/latest?from=USD&to=EUR,GBP,CHF')
    if (!res.ok) throw new Error('API error')
    const data = await res.json()

    const rates = {
      USD: 1,
      EUR: data.rates.EUR,
      GBP: data.rates.GBP,
      CHF: data.rates.CHF
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
