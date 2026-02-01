import { useState, useEffect, useRef } from 'react'
import { getCurrency, setCurrency, EXCHANGE_RATES } from '../utils/currency'
import { useTradingMode } from '../context/TradingModeContext'

function Header({ isLoggedIn, isAdmin, user, onLogout, sidebarOpen, setSidebarOpen }) {
  const [currency, setCurrencyState] = useState(getCurrency())
  const [showMoreCurrencies, setShowMoreCurrencies] = useState(false)
  const dropdownRef = useRef(null)
  const allCurrencies = Object.keys(EXCHANGE_RATES)
  const primaryCurrencies = ['USD', 'EUR']
  const otherCurrencies = allCurrencies.filter(c => !primaryCurrencies.includes(c))
  const { mode, toggleMode, isAggressive } = useTradingMode()

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target)) {
        setShowMoreCurrencies(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  useEffect(() => {
    const handleChange = () => setCurrencyState(getCurrency())
    window.addEventListener('currencyChanged', handleChange)
    return () => window.removeEventListener('currencyChanged', handleChange)
  }, [])

  const handleCurrencyChange = (curr) => {
    setCurrency(curr)
    setCurrencyState(curr)
  }

  return (
    <header className="relative border-b border-dark-600 px-4 md:px-6 py-3 md:py-4 overflow-hidden">
      <img src="/banner.png" alt="" className="absolute inset-0 w-full h-full object-cover" />
      <div className="absolute inset-0 bg-dark-900/60"></div>
      <div className="relative flex items-center justify-between">
        <div className="flex items-center gap-2 md:gap-3">
          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            className="md:hidden p-2 -ml-2 text-gray-300 hover:text-white transition-colors"
          >
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              {sidebarOpen ? (
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              ) : (
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
              )}
            </svg>
          </button>
          <div className="w-8 h-8 md:w-10 md:h-10 bg-accent-500 rounded-lg flex items-center justify-center">
            <span className="text-white font-bold text-lg md:text-xl">F</span>
          </div>
          <h1 className="text-lg md:text-2xl font-bold text-white">
            Flipper<span className="text-accent-400">Capital</span>
          </h1>
        </div>
        <div className="flex items-center gap-2 md:gap-4">
          {/* Trading Mode Toggle */}
          <button
            onClick={toggleMode}
            className={`flex items-center gap-1.5 md:gap-2 px-2 md:px-3 py-1.5 md:py-2 rounded-lg backdrop-blur-sm transition-all ${
              isAggressive
                ? 'bg-orange-500/20 border border-orange-500/50 text-orange-400 hover:bg-orange-500/30'
                : 'bg-blue-500/20 border border-blue-500/50 text-blue-400 hover:bg-blue-500/30'
            }`}
            title={isAggressive ? 'Aggressiver Modus aktiv' : 'Defensiver Modus aktiv'}
          >
            {isAggressive ? (
              <>
                {/* Fire/Lightning icon for aggressive */}
                <svg className="w-4 h-4 md:w-5 md:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17.657 18.657A8 8 0 016.343 7.343S7 9 9 10c0-2 .5-5 2.986-7C14 5 16.09 5.777 17.656 7.343A7.975 7.975 0 0120 13a7.975 7.975 0 01-2.343 5.657z" />
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.879 16.121A3 3 0 1012.015 11L11 14H9c0 .768.293 1.536.879 2.121z" />
                </svg>
                <span className="hidden sm:inline text-xs md:text-sm font-medium">Aggressiv</span>
              </>
            ) : (
              <>
                {/* Shield icon for defensive */}
                <svg className="w-4 h-4 md:w-5 md:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                </svg>
                <span className="hidden sm:inline text-xs md:text-sm font-medium">Defensiv</span>
              </>
            )}
          </button>
          <div className="flex items-center bg-dark-700/80 rounded-lg p-0.5 md:p-1 backdrop-blur-sm">
            {primaryCurrencies.map((curr) => (
              <button
                key={curr}
                onClick={() => handleCurrencyChange(curr)}
                className={`px-2 md:px-3 py-1 md:py-1.5 text-xs md:text-sm font-medium rounded-md transition-all ${
                  currency === curr ? 'bg-accent-500 text-white' : 'text-gray-300 hover:text-white'
                }`}
              >
                {curr}
              </button>
            ))}
            <div className="relative" ref={dropdownRef}>
              <button
                onClick={() => setShowMoreCurrencies(!showMoreCurrencies)}
                className={`px-2 md:px-3 py-1 md:py-1.5 text-xs md:text-sm font-medium rounded-md transition-all ${
                  otherCurrencies.includes(currency) ? 'bg-accent-500 text-white' : 'text-gray-300 hover:text-white'
                }`}
              >
                {otherCurrencies.includes(currency) ? currency : '...'}
              </button>
              {showMoreCurrencies && (
                <div className="absolute right-0 top-full mt-1 bg-dark-800 border border-dark-600 rounded-lg shadow-xl z-[100] min-w-[80px]">
                  {otherCurrencies.map((curr) => (
                    <button
                      key={curr}
                      onClick={() => { handleCurrencyChange(curr); setShowMoreCurrencies(false) }}
                      className={`block w-full px-3 py-2 text-xs md:text-sm text-left transition-colors ${
                        currency === curr ? 'bg-accent-500 text-white' : 'text-gray-300 hover:bg-dark-700 hover:text-white'
                      }`}
                    >
                      {curr}
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>
          {isLoggedIn && (
            <div className="flex items-center gap-2 md:gap-3">
              {isAdmin && (
                <span className="hidden sm:inline-block px-2 md:px-3 py-1 bg-green-500/20 text-green-400 text-xs md:text-sm rounded-full backdrop-blur-sm">
                  Admin
                </span>
              )}
              <button onClick={onLogout} className="px-2 md:px-4 py-1.5 md:py-2 text-xs md:text-sm text-gray-300 hover:text-white transition-colors">
                Logout
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  )
}

export default Header
