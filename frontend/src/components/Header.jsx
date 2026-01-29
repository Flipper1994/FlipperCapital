import { useState, useEffect } from 'react'
import { getCurrency, setCurrency, EXCHANGE_RATES } from '../utils/currency'

function Header({ isLoggedIn, isAdmin, user, onLogout, sidebarOpen, setSidebarOpen }) {
  const [currency, setCurrencyState] = useState(getCurrency())
  const availableCurrencies = Object.keys(EXCHANGE_RATES)

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
          <div className="flex items-center bg-dark-700/80 rounded-lg p-0.5 md:p-1 backdrop-blur-sm">
            {availableCurrencies.map((curr) => (
              <button
                key={curr}
                onClick={() => handleCurrencyChange(curr)}
                className={`${curr === 'USD' || curr === 'EUR' ? '' : 'hidden md:block'} px-2 md:px-3 py-1 md:py-1.5 text-xs md:text-sm font-medium rounded-md transition-all ${
                  currency === curr ? 'bg-accent-500 text-white' : 'text-gray-300 hover:text-white'
                }`}
              >
                {curr}
              </button>
            ))}
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
