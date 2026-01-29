// Direct currency conversion utilities - no React context
export const EXCHANGE_RATES = {
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
  const rate = EXCHANGE_RATES[currency] || 1
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
  const rate = EXCHANGE_RATES[currency] || 1
  const symbol = CURRENCY_SYMBOLS[currency] || '$'
  const converted = usdChange * rate
  const isPositive = converted >= 0
  const sign = isPositive ? '+' : ''

  return {
    text: `${sign}${currency === 'CHF' ? 'CHF ' : symbol}${Math.abs(converted).toFixed(2)} (${sign}${percent?.toFixed(2) || '0.00'}%)`,
    isPositive
  }
}
