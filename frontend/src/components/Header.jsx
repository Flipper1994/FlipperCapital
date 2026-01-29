import { useCurrency } from '../context/CurrencyContext'

function Header({ isLoggedIn, isAdmin, user, onLogout }) {
  const { currency, setCurrency, availableCurrencies } = useCurrency()

  return (
    <header className="relative border-b border-dark-600 px-6 py-4 overflow-hidden">
      {/* Banner Background */}
      <img
        src="/banner.png"
        alt=""
        className="absolute inset-0 w-full h-full object-cover"
      />
      {/* Dark overlay for better text readability */}
      <div className="absolute inset-0 bg-dark-900/60"></div>

      <div className="relative flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 bg-accent-500 rounded-lg flex items-center justify-center">
            <span className="text-white font-bold text-xl">F</span>
          </div>
          <h1 className="text-2xl font-bold text-white">
            Flipper<span className="text-accent-400">Capital</span>
          </h1>
        </div>

        <div className="flex items-center gap-4">
          {/* Currency Selector */}
          <div className="flex items-center bg-dark-700/80 rounded-lg p-1 backdrop-blur-sm">
            {availableCurrencies.map((curr) => (
              <button
                key={curr}
                onClick={() => setCurrency(curr)}
                className={`px-3 py-1.5 text-sm font-medium rounded-md transition-all ${
                  currency === curr
                    ? 'bg-accent-500 text-white'
                    : 'text-gray-300 hover:text-white'
                }`}
              >
                {curr}
              </button>
            ))}
          </div>

          {isLoggedIn && (
            <div className="flex items-center gap-3">
              {isAdmin && (
                <span className="px-3 py-1 bg-green-500/20 text-green-400 text-sm rounded-full backdrop-blur-sm">
                  Admin
                </span>
              )}
              <button
                onClick={onLogout}
                className="px-4 py-2 text-sm text-gray-300 hover:text-white transition-colors"
              >
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
