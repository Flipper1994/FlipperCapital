import { useState, useEffect } from 'react'
import { NavLink } from 'react-router-dom'

function Sidebar({ isLoggedIn, isAdmin, user, isOpen, onClose }) {
  const [unreadCount, setUnreadCount] = useState(0)
  const [liveSessionActive, setLiveSessionActive] = useState(false)
  const [alpacaWS, setAlpacaWS] = useState(null) // null=unknown, true=connected, false=disconnected
  const [alpacaConfigured, setAlpacaConfigured] = useState(false)

  useEffect(() => {
    if (!isLoggedIn) { setUnreadCount(0); return }
    const token = localStorage.getItem('authToken')
    const fetchCount = async () => {
      try {
        const res = await fetch('/api/notifications/unread-count', {
          headers: { 'Authorization': `Bearer ${token}` }
        })
        if (res.ok) {
          const data = await res.json()
          setUnreadCount(data.count || 0)
        }
      } catch { /* ignore */ }
    }
    fetchCount()
    const interval = setInterval(fetchCount, 60000)
    return () => clearInterval(interval)
  }, [isLoggedIn])

  useEffect(() => {
    if (!isLoggedIn) { setLiveSessionActive(false); return }
    const token = localStorage.getItem('authToken')
    const fetchLiveStatus = async () => {
      try {
        const res = await fetch('/api/trading/live/status', {
          headers: { 'Authorization': `Bearer ${token}` }
        })
        if (res.ok) {
          const data = await res.json()
          setLiveSessionActive(!!data.is_running)
          setAlpacaConfigured(!!data.alpaca_configured)
          setAlpacaWS(!!data.alpaca_ws)
        }
      } catch { /* ignore */ }
    }
    fetchLiveStatus()
    const interval = setInterval(fetchLiveStatus, 15000)
    return () => clearInterval(interval)
  }, [isLoggedIn])

  const navItems = [
    { path: '/', label: 'Dashboard', icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6' },
    { path: '/tracker', label: 'Aktien Tracker', icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z' },
    { path: '/performance', label: 'Performance', icon: 'M13 7h8m0 0v8m0-8l-8 8-4-4-6 6' },
    { path: '/signal-liste', label: 'Signal Liste', icon: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4' },
    { path: '/flipperbot', label: 'FlipperBot', icon: 'M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z', premium: true },
    { path: '/flipperbot-lab', label: 'FlipperBot Lab', icon: 'M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z', bot: 'from-purple-500 to-pink-500' },
    { path: '/lutz-lab', label: 'Lutz Lab', icon: 'M17.657 18.657A8 8 0 016.343 7.343S7 9 9 10c0-2 .5-5 2.986-7C14 5 16.09 5.777 17.656 7.343A7.975 7.975 0 0120 13a7.975 7.975 0 01-2.343 5.657z', bot: 'from-orange-500 to-red-500' },
    { path: '/quant-lab', label: 'Quant Lab', icon: 'M9 7h6m0 10v-3m-3 3h.01M9 17h.01M9 14h.01M12 14h.01M15 11h.01M12 11h.01M9 11h.01M7 21h10a2 2 0 002-2V5a2 2 0 00-2-2H7a2 2 0 00-2 2v14a2 2 0 002 2z', bot: 'from-violet-500 to-purple-500' },
    { path: '/ditz-lab', label: 'Ditz Lab', icon: 'M9 7h6m0 10v-3m-3 3h.01M9 17h.01M9 14h.01M12 14h.01M15 11h.01M12 11h.01M9 11h.01M7 21h10a2 2 0 002-2V5a2 2 0 00-2-2H7a2 2 0 00-2 2v14a2 2 0 002 2z', bot: 'from-cyan-500 to-teal-500' },
    { path: '/trader-lab', label: 'Trader Lab', icon: 'M13 7h8m0 0v8m0-8l-8 8-4-4-6 6', bot: 'from-emerald-500 to-green-500' },
    { path: '/backtest-lab', label: 'Backtest Lab', icon: 'M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z', redirectIfNotAuth: '/login', bot: 'from-blue-500 to-indigo-500' },
    { path: '/portfolio', label: 'Mein Portfolio', icon: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01', redirectIfNotAuth: '/login' },
    { path: '/profile', label: 'Mein Profil', icon: 'M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z', redirectIfNotAuth: '/login', showNotifications: true },
    { path: '/compare', label: 'Portfolio vergleich', icon: 'M16 8v8m-4-5v5m-4-2v2m-2 4h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z', redirectIfNotAuth: '/login' },
    { path: '/help', label: 'Hilfe', icon: 'M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z' },
    { path: '/trading-arena', label: 'Trading Arena', icon: 'M7 12l3-3 3 3 4-4M8 21l4-4 4 4M3 4h18M4 4h16v12a1 1 0 01-1 1H5a1 1 0 01-1-1V4z', redirectIfNotAuth: '/login' },
    { path: '/live-trading', label: 'Live Trading', icon: 'M13 10V3L4 14h7v7l9-11h-7z', badge: 'live', redirectIfNotAuth: '/login' },
    { path: '/daytrading-stats', label: 'Trading Stats', icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z', badge: 'new', redirectIfNotAuth: '/login' },
    { path: '/admin', label: 'Admin Panel', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z', requiresAdmin: true },
  ]

  const authItems = isLoggedIn
    ? []
    : [
        { path: '/login', label: 'Login', icon: 'M11 16l-4-4m0 0l4-4m-4 4h14m-5 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h7a3 3 0 013 3v1' },
        { path: '/register', label: 'Registrieren', icon: 'M18 9v3m0 0v3m0-3h3m-3 0h-3m-2-5a4 4 0 11-8 0 4 4 0 018 0zM3 20a6 6 0 0112 0v1H3v-1z' },
      ]

  const handleNavClick = () => {
    // Close sidebar on mobile after navigation
    if (window.innerWidth < 768) {
      onClose()
    }
  }

  return (
    <>
      {/* Backdrop for mobile */}
      {isOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-40 md:hidden"
          onClick={onClose}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`
          fixed md:sticky inset-y-0 left-0 z-50
          w-56 bg-dark-800 border-r border-dark-600 flex flex-col
          transform transition-transform duration-300 ease-in-out
          ${isOpen ? 'translate-x-0' : '-translate-x-full'}
          md:translate-x-0 md:top-0 md:h-[calc(100vh-64px)] md:shrink-0 md:self-start
        `}
      >
        {/* Mobile close button */}
        <div className="md:hidden flex items-center justify-between p-4 border-b border-dark-600">
          <span className="text-white font-medium">Menu</span>
          <button
            onClick={onClose}
            className="p-1 text-gray-400 hover:text-white transition-colors"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <nav className="flex-1 p-4">
          <ul className="space-y-2">
            {navItems.map((item) => {
              // Only Admin Panel is hidden for non-admins
              if (item.requiresAdmin && !isAdmin) return null

              return (
                <li key={item.path}>
                    <NavLink
                      to={item.path}
                      onClick={handleNavClick}
                      className={({ isActive }) =>
                        `flex items-center gap-3 px-3 py-2.5 rounded-lg transition-colors ${
                          isActive
                            ? 'bg-accent-500 text-white'
                            : 'text-gray-400 hover:text-white hover:bg-dark-700'
                        }`
                      }
                    >
                      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d={item.icon} />
                      </svg>
                      <span className="text-sm font-medium">{item.label}</span>
                      {item.premium && (
                        <span className="ml-auto px-1.5 py-0.5 bg-gradient-to-r from-amber-500 to-yellow-500 text-dark-900 text-[10px] font-bold rounded">
                          PRO
                        </span>
                      )}
                      {item.bot && (
                        <span className={`ml-auto px-1.5 py-0.5 bg-gradient-to-r ${item.bot} text-white text-[10px] font-bold rounded flex items-center gap-1`}>
                          <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                          </svg>
                          FREE
                        </span>
                      )}
                      {item.showNotifications && unreadCount > 0 && (
                        <span className="ml-auto min-w-[20px] h-5 flex items-center justify-center px-1.5 bg-red-500 text-white text-[10px] font-bold rounded-full animate-pulse">
                          {unreadCount > 9 ? '9+' : unreadCount}
                        </span>
                      )}
                      {item.badge === 'new' && (
                        <span className="ml-auto px-1.5 py-0.5 bg-gradient-to-r from-blue-500 to-cyan-500 text-white text-[10px] font-bold rounded">
                          NEW
                        </span>
                      )}
                      {item.badge === 'live' && (
                        <span className={`ml-auto px-1.5 py-0.5 text-[10px] font-bold rounded ${
                          liveSessionActive
                            ? 'bg-green-500/20 text-green-400 border border-green-500/30 animate-pulse'
                            : 'bg-dark-600 text-gray-500'
                        }`}>
                          {liveSessionActive ? 'LIVE' : 'OFF'}
                        </span>
                      )}
                    </NavLink>
                </li>
              )
            })}
          </ul>

          {authItems.length > 0 && (
            <>
              <div className="my-4 border-t border-dark-600"></div>
              <ul className="space-y-2">
                {authItems.map((item) => (
                  <li key={item.path}>
                    <NavLink
                      to={item.path}
                      onClick={handleNavClick}
                      className={({ isActive }) =>
                        `flex items-center gap-3 px-3 py-2.5 rounded-lg transition-colors ${
                          isActive
                            ? 'bg-accent-500 text-white'
                            : 'text-gray-400 hover:text-white hover:bg-dark-700'
                        }`
                      }
                    >
                      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d={item.icon} />
                      </svg>
                      <span className="text-sm font-medium">{item.label}</span>
                    </NavLink>
                  </li>
                ))}
              </ul>
            </>
          )}
        </nav>

        {isLoggedIn && (
          <div className="px-4 py-2 border-t border-dark-600">
            <div className="flex items-center gap-2">
              <span className={`w-2 h-2 rounded-full shrink-0 ${
                alpacaWS ? 'bg-green-400' : alpacaConfigured ? 'bg-red-400 animate-pulse' : 'bg-gray-600'
              }`} />
              <span className="text-[11px] text-gray-500">
                Alpaca WS {alpacaWS ? 'verbunden' : alpacaConfigured ? 'getrennt' : 'nicht konfiguriert'}
              </span>
            </div>
          </div>
        )}

        {isLoggedIn && user && (
          <div className="p-4 border-t border-dark-600">
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 bg-accent-500 rounded-full flex items-center justify-center">
                <span className="text-white text-sm font-bold">
                  {user.username?.charAt(0).toUpperCase() || 'U'}
                </span>
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-white truncate">{user.username}</p>
                <p className="text-xs text-gray-500 truncate">{user.email}</p>
              </div>
              {isAdmin && (
                <span className="px-2 py-0.5 bg-green-500/20 text-green-400 text-xs rounded-full">
                  Admin
                </span>
              )}
            </div>
          </div>
        )}
      </aside>
    </>
  )
}

export default Sidebar
