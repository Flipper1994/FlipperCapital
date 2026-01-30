import { createContext, useContext, useState, useEffect } from 'react'

const TradingModeContext = createContext()

export function TradingModeProvider({ children }) {
  const [mode, setMode] = useState(() => {
    const saved = localStorage.getItem('tradingMode')
    return saved || 'defensive'
  })

  useEffect(() => {
    localStorage.setItem('tradingMode', mode)
    // Dispatch event for components that need to react
    window.dispatchEvent(new CustomEvent('tradingModeChanged', { detail: mode }))
  }, [mode])

  const toggleMode = () => {
    setMode(prev => prev === 'defensive' ? 'aggressive' : 'defensive')
  }

  const isAggressive = mode === 'aggressive'

  return (
    <TradingModeContext.Provider value={{ mode, setMode, toggleMode, isAggressive }}>
      {children}
    </TradingModeContext.Provider>
  )
}

export function useTradingMode() {
  const context = useContext(TradingModeContext)
  if (!context) {
    throw new Error('useTradingMode must be used within a TradingModeProvider')
  }
  return context
}
