import { createContext, useContext, useState, useEffect } from 'react'
import { CURRENCY_SYMBOLS, fetchExchangeRates, getStockCurrency } from '../utils/currency'

const CurrencyContext = createContext()

// Fallback rates (approximate values)
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

function getStoredRates() {
  try {
    const stored = localStorage.getItem('exchangeRates')
    if (stored) {
      const { rates } = JSON.parse(stored)
      return rates
    }
  } catch {
    // Ignore
  }
  return FALLBACK_RATES
}

export function CurrencyProvider({ children }) {
  const [currency, setCurrency] = useState(() => {
    return localStorage.getItem('currency') || 'EUR'
  })
  const [exchangeRates, setExchangeRates] = useState(getStoredRates)

  useEffect(() => {
    localStorage.setItem('currency', currency)
  }, [currency])

  // Fetch live rates on mount
  useEffect(() => {
    fetchExchangeRates().then(rates => {
      setExchangeRates(rates)
    })
  }, [])

  return (
    <CurrencyContext.Provider value={{ currency, setCurrency, exchangeRates }}>
      {children}
    </CurrencyContext.Provider>
  )
}

export function useCurrency() {
  const context = useContext(CurrencyContext)
  if (!context) {
    throw new Error('useCurrency must be used within a CurrencyProvider')
  }

  const { currency, setCurrency, exchangeRates } = context

  const getRate = () => exchangeRates[currency] || 1
  const getSymbol = () => CURRENCY_SYMBOLS[currency] || '$'

  return {
    currency,
    setCurrency,
    availableCurrencies: Object.keys(FALLBACK_RATES),

    get exchangeRate() { return getRate() },
    get currencySymbol() { return getSymbol() },

    formatPrice: (usdPrice, stockSymbol = null) => {
      if (usdPrice === null || usdPrice === undefined) return '--'
      const symbol = CURRENCY_SYMBOLS[currency] || '$'

      // Check if stock is traded in a non-USD currency
      const stockCurrency = getStockCurrency(stockSymbol)

      let converted = usdPrice
      if (stockCurrency) {
        // Stock is already in EUR/GBP/CHF, convert from that currency to target
        const stockRate = exchangeRates[stockCurrency] || 1
        const targetRate = exchangeRates[currency] || 1
        converted = (usdPrice / stockRate) * targetRate
      } else {
        // Stock is in USD, convert directly to target
        const rate = exchangeRates[currency] || 1
        converted = usdPrice * rate
      }

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

    convertPrice: (usdPrice, stockSymbol = null) => {
      if (usdPrice === null || usdPrice === undefined) return null

      const stockCurrency = getStockCurrency(stockSymbol)

      if (stockCurrency) {
        const stockRate = exchangeRates[stockCurrency] || 1
        const targetRate = exchangeRates[currency] || 1
        return (usdPrice / stockRate) * targetRate
      }

      const rate = exchangeRates[currency] || 1
      return usdPrice * rate
    },

    // Convert from current currency back to USD
    convertToUSD: (localPrice) => {
      if (localPrice === null || localPrice === undefined) return null
      const rate = exchangeRates[currency] || 1
      return localPrice / rate
    },

    formatChange: (usdChange, percent, stockSymbol = null) => {
      if (usdChange === undefined || usdChange === null) return null
      const symbol = CURRENCY_SYMBOLS[currency] || '$'

      const stockCurrency = getStockCurrency(stockSymbol)

      let converted = usdChange
      if (stockCurrency) {
        const stockRate = exchangeRates[stockCurrency] || 1
        const targetRate = exchangeRates[currency] || 1
        converted = (usdChange / stockRate) * targetRate
      } else {
        const rate = exchangeRates[currency] || 1
        converted = usdChange * rate
      }

      const isPositive = converted >= 0
      const sign = isPositive ? '+' : ''

      return {
        text: `${sign}${currency === 'CHF' ? 'CHF ' : symbol}${Math.abs(converted).toFixed(2)} (${sign}${percent?.toFixed(2) || '0.00'}%)`,
        isPositive
      }
    }
  }
}
