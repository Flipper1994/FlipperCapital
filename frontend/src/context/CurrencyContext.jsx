import { createContext, useContext, useState, useEffect } from 'react'

const CurrencyContext = createContext()

const EXCHANGE_RATES = {
  USD: 1,
  EUR: 0.92,
  GBP: 0.79,
  CHF: 0.88
}

const CURRENCY_SYMBOLS = {
  USD: '$',
  EUR: '€',
  GBP: '£',
  CHF: 'CHF'
}

export function CurrencyProvider({ children }) {
  const [currency, setCurrency] = useState(() => {
    return localStorage.getItem('currency') || 'EUR'
  })
  const [exchangeRate, setExchangeRate] = useState(EXCHANGE_RATES[currency])

  useEffect(() => {
    localStorage.setItem('currency', currency)
    setExchangeRate(EXCHANGE_RATES[currency] || 1)
  }, [currency])

  const convertPrice = (usdPrice) => {
    if (!usdPrice) return null
    return usdPrice * exchangeRate
  }

  const formatPrice = (usdPrice) => {
    if (!usdPrice && usdPrice !== 0) return '--'
    const converted = convertPrice(usdPrice)
    const symbol = CURRENCY_SYMBOLS[currency]

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
  }

  const formatChange = (usdChange, percent) => {
    if (usdChange === undefined || usdChange === null) return null
    const converted = usdChange * exchangeRate
    const isPositive = converted >= 0
    const sign = isPositive ? '+' : ''
    const symbol = CURRENCY_SYMBOLS[currency]

    return {
      text: `${sign}${symbol === 'CHF' ? 'CHF ' : symbol}${Math.abs(converted).toFixed(2)} (${sign}${percent.toFixed(2)}%)`,
      isPositive
    }
  }

  return (
    <CurrencyContext.Provider value={{
      currency,
      setCurrency,
      convertPrice,
      formatPrice,
      formatChange,
      availableCurrencies: Object.keys(EXCHANGE_RATES),
      currencySymbol: CURRENCY_SYMBOLS[currency]
    }}>
      {children}
    </CurrencyContext.Provider>
  )
}

export function useCurrency() {
  const context = useContext(CurrencyContext)
  if (!context) {
    throw new Error('useCurrency must be used within a CurrencyProvider')
  }
  return context
}
