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

export function formatPrice(usdPrice) {
  if (usdPrice === null || usdPrice === undefined) return '--'
  const currency = getCurrency()
  const rates = getExchangeRates()
  const rate = rates[currency] || 1
  const symbol = CURRENCY_SYMBOLS[currency] || '$'
  const converted = usdPrice * rate

  if (currency === 'CHF') {
    return `${symbol} ${converted.toLocaleString('de-CH', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
  }
  return `${symbol}${converted.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

export function formatChange(usdChange, percent) {
  if (usdChange === undefined || usdChange === null) return null
  const currency = getCurrency()
  const rates = getExchangeRates()
  const rate = rates[currency] || 1
  const symbol = CURRENCY_SYMBOLS[currency] || '$'
  const converted = usdChange * rate
  const isPositive = converted >= 0
  const sign = isPositive ? '+' : ''

  return {
    text: `${sign}${currency === 'CHF' ? 'CHF ' : symbol}${Math.abs(converted).toFixed(2)} (${sign}${percent?.toFixed(2) || '0.00'}%)`,
    isPositive
  }
}
