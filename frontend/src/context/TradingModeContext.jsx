import { createContext, useContext, useState, useEffect } from 'react'

const TradingModeContext = createContext()

// Available trading modes
export const TRADING_MODES = {
  DEFENSIVE: 'defensive',
  AGGRESSIVE: 'aggressive',
  QUANT: 'quant'
}

export function TradingModeProvider({ children }) {
  const [mode, setMode] = useState(() => {
    const saved = localStorage.getItem('tradingMode')
    // Validate saved mode
    if (saved && Object.values(TRADING_MODES).includes(saved)) {
      return saved
    }
    return TRADING_MODES.DEFENSIVE
  })

  useEffect(() => {
    localStorage.setItem('tradingMode', mode)
    // Dispatch event for components that need to react
    window.dispatchEvent(new CustomEvent('tradingModeChanged', { detail: mode }))
  }, [mode])

  // Cycle through modes: defensive -> aggressive -> quant -> defensive
  const cycleMode = () => {
    setMode(prev => {
      switch (prev) {
        case TRADING_MODES.DEFENSIVE:
          return TRADING_MODES.AGGRESSIVE
        case TRADING_MODES.AGGRESSIVE:
          return TRADING_MODES.QUANT
        case TRADING_MODES.QUANT:
        default:
          return TRADING_MODES.DEFENSIVE
      }
    })
  }

  // Legacy toggle for backwards compatibility
  const toggleMode = cycleMode

  const isAggressive = mode === TRADING_MODES.AGGRESSIVE
  const isQuant = mode === TRADING_MODES.QUANT
  const isDefensive = mode === TRADING_MODES.DEFENSIVE

  return (
    <TradingModeContext.Provider value={{
      mode,
      setMode,
      toggleMode,
      cycleMode,
      isAggressive,
      isQuant,
      isDefensive,
      MODES: TRADING_MODES
    }}>
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
