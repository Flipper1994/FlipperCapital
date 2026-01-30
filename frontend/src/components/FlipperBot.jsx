import { useState } from 'react'
import { Link } from 'react-router-dom'

function FlipperBot() {
  const [email, setEmail] = useState('')
  const [subscribed, setSubscribed] = useState(false)

  const handleNotify = (e) => {
    e.preventDefault()
    if (email) {
      setSubscribed(true)
      setEmail('')
    }
  }

  return (
    <div className="flex-1 p-4 md:p-6 overflow-auto">
      <div className="max-w-4xl mx-auto">
        {/* Hero Section */}
        <div className="relative rounded-2xl overflow-hidden mb-8">
          <img
            src="/images/flipperbot-hero.png"
            alt="FlipperBot"
            className="w-full h-48 md:h-64 lg:h-80 object-cover"
          />
          <div className="absolute inset-0 bg-gradient-to-t from-dark-900 via-dark-900/60 to-transparent"></div>
          <div className="absolute bottom-0 left-0 right-0 p-6 md:p-8">
            <div className="flex items-center gap-3 mb-2">
              <div className="w-10 h-10 md:w-12 md:h-12 bg-accent-500 rounded-xl flex items-center justify-center">
                <svg className="w-6 h-6 md:w-7 md:h-7 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                </svg>
              </div>
              <h1 className="text-2xl md:text-3xl lg:text-4xl font-bold text-white">FlipperBot</h1>
            </div>
            <p className="text-gray-300 text-sm md:text-base max-w-xl">
              Automatisierte Trading-Signale basierend auf dem BX Trender Algorithmus
            </p>
          </div>
        </div>

        {/* Premium Badge */}
        <div className="bg-gradient-to-r from-amber-500/20 via-yellow-500/20 to-amber-500/20 border border-amber-500/30 rounded-xl p-4 md:p-6 mb-8">
          <div className="flex flex-col md:flex-row md:items-center gap-4">
            <div className="flex items-center gap-3">
              <div className="w-12 h-12 bg-gradient-to-br from-amber-400 to-yellow-600 rounded-xl flex items-center justify-center shrink-0">
                <svg className="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z"/>
                </svg>
              </div>
              <div>
                <h3 className="text-amber-300 font-semibold text-lg">Premium Feature</h3>
                <p className="text-amber-200/70 text-sm">Diese Funktion ist nur mit einem Premium-Abo verfügbar</p>
              </div>
            </div>
            <button
              disabled
              className="md:ml-auto px-6 py-2.5 bg-gradient-to-r from-amber-500 to-yellow-500 text-dark-900 font-semibold rounded-lg opacity-50 cursor-not-allowed"
            >
              Coming Soon
            </button>
          </div>
        </div>

        {/* Features Preview */}
        <div className="grid md:grid-cols-2 gap-4 md:gap-6 mb-8">
          <div className="bg-dark-800 rounded-xl border border-dark-600 p-5 md:p-6">
            <div className="w-10 h-10 bg-green-500/20 rounded-lg flex items-center justify-center mb-4">
              <svg className="w-5 h-5 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
              </svg>
            </div>
            <h3 className="text-white font-semibold mb-2">Automatische Signale</h3>
            <p className="text-gray-500 text-sm">
              Erhalte BUY, SELL, HOLD und WAIT Signale automatisch berechnet durch unseren BX Trender Algorithmus.
            </p>
          </div>

          <div className="bg-dark-800 rounded-xl border border-dark-600 p-5 md:p-6">
            <div className="w-10 h-10 bg-blue-500/20 rounded-lg flex items-center justify-center mb-4">
              <svg className="w-5 h-5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
              </svg>
            </div>
            <h3 className="text-white font-semibold mb-2">Push-Benachrichtigungen</h3>
            <p className="text-gray-500 text-sm">
              Werde sofort benachrichtigt, wenn sich ein neues Trading-Signal für deine Watchlist-Aktien ergibt.
            </p>
          </div>

          <div className="bg-dark-800 rounded-xl border border-dark-600 p-5 md:p-6">
            <div className="w-10 h-10 bg-purple-500/20 rounded-lg flex items-center justify-center mb-4">
              <svg className="w-5 h-5 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              </svg>
            </div>
            <h3 className="text-white font-semibold mb-2">Performance-Tracking</h3>
            <p className="text-gray-500 text-sm">
              Verfolge die theoretische Performance aller Bot-Trades mit detaillierten Statistiken und Analysen.
            </p>
          </div>

          <div className="bg-dark-800 rounded-xl border border-dark-600 p-5 md:p-6">
            <div className="w-10 h-10 bg-red-500/20 rounded-lg flex items-center justify-center mb-4">
              <svg className="w-5 h-5 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            </div>
            <h3 className="text-white font-semibold mb-2">Risiko-Management</h3>
            <p className="text-gray-500 text-sm">
              Stop-Loss und Take-Profit Level werden automatisch berechnet und angezeigt.
            </p>
          </div>
        </div>

        {/* Example Signal Card (Blurred/Locked) */}
        <div className="bg-dark-800 rounded-xl border border-dark-600 p-5 md:p-6 mb-8 relative overflow-hidden">
          <div className="absolute inset-0 backdrop-blur-sm bg-dark-900/60 z-10 flex items-center justify-center">
            <div className="text-center p-6">
              <div className="w-16 h-16 mx-auto mb-4 bg-dark-700 rounded-full flex items-center justify-center">
                <svg className="w-8 h-8 text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                </svg>
              </div>
              <p className="text-white font-medium mb-1">Premium erforderlich</p>
              <p className="text-gray-500 text-sm">Upgrade auf Premium um Live-Signale zu sehen</p>
            </div>
          </div>

          <h3 className="text-white font-semibold mb-4">Aktuelle Signale</h3>
          <div className="space-y-3">
            {['AAPL', 'MSFT', 'GOOGL'].map((symbol) => (
              <div key={symbol} className="flex items-center justify-between p-3 bg-dark-700 rounded-lg">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 bg-dark-600 rounded-lg"></div>
                  <div>
                    <div className="font-medium text-white">{symbol}</div>
                    <div className="text-xs text-gray-500">Signal aktiv</div>
                  </div>
                </div>
                <div className="px-3 py-1 bg-green-500/20 text-green-400 text-sm font-medium rounded-full">
                  BUY
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Notify Me Section */}
        <div className="bg-dark-800 rounded-xl border border-dark-600 p-5 md:p-6">
          <div className="text-center max-w-md mx-auto">
            <h3 className="text-white font-semibold text-lg mb-2">Benachrichtigt werden</h3>
            <p className="text-gray-500 text-sm mb-4">
              Melde dich an, um als Erster zu erfahren, wenn FlipperBot verfügbar ist.
            </p>

            {subscribed ? (
              <div className="flex items-center justify-center gap-2 text-green-400">
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
                <span>Du wirst benachrichtigt!</span>
              </div>
            ) : (
              <form onSubmit={handleNotify} className="flex flex-col sm:flex-row gap-3">
                <input
                  type="email"
                  placeholder="Deine E-Mail Adresse"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="flex-1 px-4 py-2.5 bg-dark-700 border border-dark-600 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-accent-500"
                  required
                />
                <button
                  type="submit"
                  className="px-6 py-2.5 bg-accent-500 text-white rounded-lg hover:bg-accent-400 transition-colors font-medium whitespace-nowrap"
                >
                  Benachrichtigen
                </button>
              </form>
            )}
          </div>
        </div>

        {/* Disclaimer */}
        <p className="text-center text-gray-600 text-xs mt-6 px-4">
          FlipperBot zeigt theoretische Trading-Signale basierend auf technischen Indikatoren.
          Dies stellt keine Anlageberatung dar. Investitionen bergen Risiken.
        </p>
      </div>
    </div>
  )
}

export default FlipperBot
