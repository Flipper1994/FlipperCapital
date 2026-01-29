import { createContext, useContext, useState, useEffect } from 'react'

const CurrencyContext = createContext()

// Exchange rates: How many units of this currency per 1 USD
export const EXCHANGE_RATES = {
  USD: 1,
  EUR: 0.92,   // 1 USD = 0.92 EUR
  GBP: 0.79,   // 1 USD = 0.79 GBP
  CHF: 0.88    // 1 USD = 0.88 CHF
}

export const CURRENCY_SYMBOLS = {
  USD: '$',
  EUR: '€',
  GBP: '£',
  CHF: 'CHF'
}

export function CurrencyProvider({ children }) {
  const [currency, setCurrency] = useState(() => {
    return localStorage.getItem('currency') || 'EUR'
  })

  useEffect(() => {
    localStorage.setItem('currency', currency)
  }, [currency])

  return (
    <CurrencyContext.Provider value={{ currency, setCurrency }}>
      {children}
    </CurrencyContext.Provider>
  )
}

export function useCurrency() {
  const context = useContext(CurrencyContext)
  if (!context) {
    throw new Error('useCurrency must be used within a CurrencyProvider')
  }

  const { currency, setCurrency } = context

  // Calculate rate and symbol directly from current currency - no closures
  const getRate = () => EXCHANGE_RATES[currency] || 1
  const getSymbol = () => CURRENCY_SYMBOLS[currency] || '$'

  return {
    currency,
    setCurrency,
    availableCurrencies: Object.keys(EXCHANGE_RATES),

    // Get current values
    get exchangeRate() { return getRate() },
    get currencySymbol() { return getSymbol() },

    // Format USD price to current currency
    formatPrice: (usdPrice) => {
      if (usdPrice === null || usdPrice === undefined) return '--'
      const rate = EXCHANGE_RATES[currency] || 1
      const symbol = CURRENCY_SYMBOLS[currency] || '$'
      const converted = usdPrice * rate

      if (currency === 'CHF') {
        return `${symbol} ${converted.toLocaleString('de-CH', {
          minimumFractionDigits: 2,
          maximumFractionDigits: 2
        })}`
      }

      return `${symbol}${converted.toLocaleString('en-US', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2
      })}`
    },

    // Convert USD to current currency (returns number)
    convertPrice: (usdPrice) => {
      if (usdPrice === null || usdPrice === undefined) return null
      const rate = EXCHANGE_RATES[currency] || 1
      return usdPrice * rate
    },

    // Format price change
    formatChange: (usdChange, percent) => {
      if (usdChange === undefined || usdChange === null) return null
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
  }
}
