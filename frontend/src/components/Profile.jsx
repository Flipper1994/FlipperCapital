import { useState, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'

const TABS = [
  { label: 'Info', short: 'Info', icon: 'M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z' },
  { label: 'Passwort', short: 'Passwort', icon: 'M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z' },
  { label: 'Benachrichtigungen', short: 'Meldungen', icon: 'M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9' },
  { label: 'Aktivität', short: 'Aktivität', icon: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z' },
]

function Profile() {
  const [searchParams] = useSearchParams()
  const tabParam = searchParams.get('tab')
  const [activeTab, setActiveTab] = useState(tabParam !== null ? parseInt(tabParam) : 0)
  const [profile, setProfile] = useState(null)
  const [notifications, setNotifications] = useState([])
  const [activity, setActivity] = useState([])
  const [loading, setLoading] = useState(true)
  const [pwForm, setPwForm] = useState({ old_password: '', new_password: '', confirm_password: '' })
  const [pwMsg, setPwMsg] = useState(null)
  const [pwLoading, setPwLoading] = useState(false)
  const [initialTabResolved, setInitialTabResolved] = useState(tabParam !== null)

  const token = localStorage.getItem('authToken')
  const headers = { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' }

  useEffect(() => {
    fetchProfile()
    // Auto-switch to notifications tab if unread notifications exist (and no explicit tab param)
    if (tabParam === null) {
      fetch('/api/notifications/unread-count', { headers })
        .then(r => r.ok ? r.json() : { count: 0 })
        .then(data => {
          if (data.count > 0) setActiveTab(2)
          setInitialTabResolved(true)
        })
        .catch(() => setInitialTabResolved(true))
    }
  }, [])

  useEffect(() => {
    if (activeTab === 2) fetchNotifications()
    if (activeTab === 3) fetchActivity()
  }, [activeTab])

  const fetchProfile = async () => {
    try {
      const res = await fetch('/api/profile', { headers })
      if (res.ok) setProfile(await res.json())
    } catch { /* ignore */ }
    setLoading(false)
  }

  const fetchNotifications = async () => {
    try {
      const res = await fetch('/api/notifications', { headers })
      if (res.ok) setNotifications(await res.json())
    } catch { /* ignore */ }
  }

  const fetchActivity = async () => {
    try {
      const res = await fetch('/api/profile/activity', { headers })
      if (res.ok) setActivity(await res.json())
    } catch { /* ignore */ }
  }

  const changePassword = async (e) => {
    e.preventDefault()
    setPwMsg(null)
    if (pwForm.new_password.length < 6) {
      setPwMsg({ type: 'error', text: 'Mindestens 6 Zeichen' })
      return
    }
    if (pwForm.new_password !== pwForm.confirm_password) {
      setPwMsg({ type: 'error', text: 'Passwörter stimmen nicht überein' })
      return
    }
    setPwLoading(true)
    try {
      const res = await fetch('/api/profile/password', {
        method: 'PUT', headers, body: JSON.stringify(pwForm)
      })
      const data = await res.json()
      if (res.ok) {
        setPwMsg({ type: 'success', text: 'Passwort erfolgreich geändert' })
        setPwForm({ old_password: '', new_password: '', confirm_password: '' })
      } else {
        setPwMsg({ type: 'error', text: data.error || 'Fehler' })
      }
    } catch {
      setPwMsg({ type: 'error', text: 'Netzwerkfehler' })
    }
    setPwLoading(false)
  }

  const markRead = async (id) => {
    await fetch(`/api/notifications/${id}/read`, { method: 'PUT', headers })
    setNotifications(prev => prev.map(n => n.id === id ? { ...n, is_read: true } : n))
  }

  const markAllRead = async () => {
    await fetch('/api/notifications/read-all', { method: 'PUT', headers })
    setNotifications(prev => prev.map(n => ({ ...n, is_read: true })))
  }

  const updateRankingVisibility = async (visible) => {
    try {
      await fetch('/api/user/ranking-visibility', {
        method: 'PUT', headers, body: JSON.stringify({ visible })
      })
      setProfile(prev => ({ ...prev, visible_in_ranking: visible }))
    } catch { /* ignore */ }
  }

  const formatDate = (d) => {
    if (!d) return '-'
    return new Date(d).toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' })
  }

  const formatDateShort = (d) => {
    if (!d) return '-'
    return new Date(d).toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: 'numeric' })
  }

  const timeAgo = (d) => {
    if (!d) return '-'
    const diff = Date.now() - new Date(d).getTime()
    const mins = Math.floor(diff / 60000)
    if (mins < 60) return `vor ${mins}m`
    const hrs = Math.floor(mins / 60)
    if (hrs < 24) return `vor ${hrs}h`
    const days = Math.floor(hrs / 24)
    return `vor ${days}d`
  }

  const signalColor = (signal) => {
    if (!signal) return 'text-gray-400'
    const s = signal.toUpperCase()
    if (s === 'BUY') return 'text-green-400'
    if (s === 'SELL') return 'text-red-400'
    if (s === 'HOLD') return 'text-blue-400'
    return 'text-yellow-400'
  }

  const activityIcon = (action) => {
    switch (action) {
      case 'login': return 'M11 16l-4-4m0 0l4-4m-4 4h14m-5 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h7a3 3 0 013 3v1'
      case 'password_change': return 'M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z'
      case 'add_stock': return 'M12 6v6m0 0v6m0-6h6m-6 0H6'
      case 'search': return 'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z'
      case 'page_view': return 'M15 12a3 3 0 11-6 0 3 3 0 016 0z'
      default: return 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z'
    }
  }

  if (loading) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-accent-500"></div>
      </div>
    )
  }

  return (
    <div className="flex-1 p-4 md:p-6 max-w-4xl">
      <h1 className="text-2xl font-bold text-white mb-6">Mein Profil</h1>

      {/* Tabs */}
      <div className="flex gap-1 mb-6 bg-dark-800 rounded-lg p-1">
        {TABS.map((tab, i) => (
          <button
            key={tab.label}
            onClick={() => setActiveTab(i)}
            className={`flex-1 flex items-center justify-center gap-1.5 px-2 sm:px-3 py-2 text-xs sm:text-sm font-medium rounded-md transition-colors ${
              activeTab === i
                ? 'bg-accent-500 text-white'
                : 'text-gray-400 hover:text-white hover:bg-dark-700'
            }`}
          >
            <svg className="w-4 h-4 shrink-0 sm:hidden" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d={tab.icon} />
            </svg>
            <span className="hidden sm:inline">{tab.label}</span>
            <span className="sm:hidden">{tab.short}</span>
          </button>
        ))}
      </div>

      {/* Tab: Info */}
      {activeTab === 0 && profile && (
        <div className="space-y-6">
          <div className="bg-dark-800 rounded-xl p-6 border border-dark-600">
            <div className="flex items-center gap-4 mb-6">
              <div className="w-16 h-16 bg-accent-500 rounded-full flex items-center justify-center">
                <span className="text-white text-2xl font-bold">
                  {profile.username?.charAt(0).toUpperCase() || 'U'}
                </span>
              </div>
              <div>
                <h2 className="text-xl font-bold text-white">{profile.username}</h2>
                <p className="text-gray-400">{profile.email}</p>
              </div>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
              <div>
                <p className="text-xs text-gray-500 uppercase">Mitglied seit</p>
                <p className="text-sm text-white">{formatDateShort(profile.created_at)}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 uppercase">Logins</p>
                <p className="text-sm text-white">{profile.login_count}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 uppercase">Letzte Aktivität</p>
                <p className="text-sm text-white">{timeAgo(profile.last_active)}</p>
              </div>
            </div>
            <div className="mt-4 pt-4 border-t border-dark-600">
              <label className="flex items-center gap-3 cursor-pointer">
                <div
                  onClick={() => updateRankingVisibility(!profile.visible_in_ranking)}
                  className={`relative w-10 h-5 rounded-full transition-colors ${
                    profile.visible_in_ranking ? 'bg-accent-500' : 'bg-dark-600'
                  }`}
                >
                  <div className={`absolute top-0.5 left-0.5 w-4 h-4 bg-white rounded-full transition-transform ${
                    profile.visible_in_ranking ? 'translate-x-5' : ''
                  }`} />
                </div>
                <span className="text-sm text-gray-300">Im Ranking sichtbar</span>
              </label>
            </div>
          </div>

          {/* Portfolio Stats */}
          <div className="bg-dark-800 rounded-xl p-6 border border-dark-600">
            <h3 className="text-lg font-semibold text-white mb-4">Portfolio-Statistiken</h3>
            <div className="grid grid-cols-3 sm:grid-cols-5 gap-2 sm:gap-4">
              <div className="bg-dark-700 rounded-lg p-2 sm:p-3 text-center">
                <p className="text-lg sm:text-2xl font-bold text-white">{profile.portfolio_stats?.open_positions || 0}</p>
                <p className="text-[10px] sm:text-xs text-gray-400">Offene Pos.</p>
              </div>
              <div className="bg-dark-700 rounded-lg p-2 sm:p-3 text-center">
                <p className="text-lg sm:text-2xl font-bold text-white">{profile.portfolio_stats?.closed_trades || 0}</p>
                <p className="text-[10px] sm:text-xs text-gray-400">Abgeschlossen</p>
              </div>
              <div className="bg-dark-700 rounded-lg p-2 sm:p-3 text-center">
                <p className={`text-lg sm:text-2xl font-bold ${(profile.portfolio_stats?.win_rate || 0) >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                  {(profile.portfolio_stats?.win_rate || 0).toFixed(1)}%
                </p>
                <p className="text-[10px] sm:text-xs text-gray-400">Gewinnrate</p>
              </div>
              <div className="bg-dark-700 rounded-lg p-2 sm:p-3 text-center">
                <p className="text-lg sm:text-2xl font-bold text-green-400">
                  {(profile.portfolio_stats?.best_trade || 0) > 0 ? '+' : ''}{(profile.portfolio_stats?.best_trade || 0).toFixed(1)}%
                </p>
                <p className="text-[10px] sm:text-xs text-gray-400">Bester Trade</p>
              </div>
              <div className="bg-dark-700 rounded-lg p-2 sm:p-3 text-center">
                <p className="text-lg sm:text-2xl font-bold text-red-400">
                  {(profile.portfolio_stats?.worst_trade || 0).toFixed(1)}%
                </p>
                <p className="text-[10px] sm:text-xs text-gray-400">Schlechtester</p>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Tab: Passwort */}
      {activeTab === 1 && (
        <div className="bg-dark-800 rounded-xl p-6 border border-dark-600 max-w-md">
          <h3 className="text-lg font-semibold text-white mb-4">Passwort ändern</h3>
          <form onSubmit={changePassword} className="space-y-4">
            <div>
              <label className="block text-sm text-gray-400 mb-1">Altes Passwort</label>
              <input
                type="password"
                value={pwForm.old_password}
                onChange={e => setPwForm(f => ({ ...f, old_password: e.target.value }))}
                className="w-full px-3 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white text-sm focus:outline-none focus:border-accent-500"
                required
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Neues Passwort</label>
              <input
                type="password"
                value={pwForm.new_password}
                onChange={e => setPwForm(f => ({ ...f, new_password: e.target.value }))}
                className="w-full px-3 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white text-sm focus:outline-none focus:border-accent-500"
                required
                minLength={6}
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Passwort bestätigen</label>
              <input
                type="password"
                value={pwForm.confirm_password}
                onChange={e => setPwForm(f => ({ ...f, confirm_password: e.target.value }))}
                className="w-full px-3 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white text-sm focus:outline-none focus:border-accent-500"
                required
              />
            </div>
            {pwMsg && (
              <div className={`text-sm px-3 py-2 rounded-lg ${
                pwMsg.type === 'success' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
              }`}>
                {pwMsg.text}
              </div>
            )}
            <button
              type="submit"
              disabled={pwLoading}
              className="w-full py-2 bg-accent-500 text-white rounded-lg text-sm font-medium hover:bg-accent-600 transition-colors disabled:opacity-50"
            >
              {pwLoading ? 'Wird geändert...' : 'Passwort ändern'}
            </button>
          </form>
        </div>
      )}

      {/* Tab: Benachrichtigungen */}
      {activeTab === 2 && (
        <div className="space-y-3">
          {notifications.length > 0 && (
            <div className="flex justify-end">
              <button
                onClick={markAllRead}
                className="text-xs text-accent-400 hover:text-accent-300 transition-colors"
              >
                Alle als gelesen markieren
              </button>
            </div>
          )}
          {notifications.length === 0 ? (
            <div className="bg-dark-800 rounded-xl p-8 border border-dark-600 text-center">
              <svg className="w-12 h-12 mx-auto text-gray-600 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
              </svg>
              <p className="text-gray-400">Keine Benachrichtigungen</p>
              <p className="text-gray-500 text-xs mt-1">Signal-Änderungen deiner Portfolio-Aktien erscheinen hier</p>
            </div>
          ) : (
            notifications.map(n => (
              <div
                key={n.id}
                onClick={() => !n.is_read && markRead(n.id)}
                className={`bg-dark-800 rounded-lg p-4 border transition-colors cursor-pointer ${
                  n.is_read
                    ? 'border-dark-600'
                    : 'border-l-4 border-l-accent-500 border-dark-600 bg-dark-750'
                }`}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-white font-medium text-sm">{n.symbol}</span>
                    {n.name && <span className="text-gray-500 text-xs hidden sm:inline">{n.name}</span>}
                    <span className="text-gray-500 text-xs px-1.5 py-0.5 bg-dark-700 rounded">{n.mode}</span>
                  </div>
                  <span className="text-gray-500 text-xs">{timeAgo(n.created_at)}</span>
                </div>
                <div className="mt-1 flex items-center gap-2 text-sm">
                  <span className={signalColor(n.old_signal)}>{n.old_signal}</span>
                  <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7l5 5m0 0l-5 5m5-5H6" />
                  </svg>
                  <span className={signalColor(n.new_signal)}>{n.new_signal}</span>
                </div>
              </div>
            ))
          )}
        </div>
      )}

      {/* Tab: Aktivität */}
      {activeTab === 3 && (
        <div>
          {activity.length === 0 ? (
            <div className="bg-dark-800 rounded-xl border border-dark-600 p-8 text-center">
              <p className="text-gray-400">Keine Aktivitäten</p>
            </div>
          ) : (
            <>
              {/* Desktop: Tabelle */}
              <div className="hidden sm:block bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-dark-600">
                      <th className="text-left px-4 py-3 text-gray-400 font-medium">Aktion</th>
                      <th className="text-left px-4 py-3 text-gray-400 font-medium">Details</th>
                      <th className="text-right px-4 py-3 text-gray-400 font-medium">Zeitpunkt</th>
                    </tr>
                  </thead>
                  <tbody>
                    {activity.map(a => (
                      <tr key={a.id} className="border-b border-dark-700 hover:bg-dark-700/50">
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-2">
                            <svg className="w-4 h-4 text-gray-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d={activityIcon(a.action)} />
                            </svg>
                            <span className="text-white capitalize">{a.action?.replace(/_/g, ' ')}</span>
                          </div>
                        </td>
                        <td className="px-4 py-3 text-gray-400 max-w-xs truncate">{a.details || '-'}</td>
                        <td className="px-4 py-3 text-gray-500 text-right whitespace-nowrap">{formatDate(a.created_at)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {/* Mobil: Cards */}
              <div className="sm:hidden space-y-2">
                {activity.map(a => (
                  <div key={a.id} className="bg-dark-800 rounded-lg p-3 border border-dark-600">
                    <div className="flex items-center justify-between mb-1">
                      <div className="flex items-center gap-2">
                        <svg className="w-4 h-4 text-gray-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d={activityIcon(a.action)} />
                        </svg>
                        <span className="text-white text-sm capitalize">{a.action?.replace(/_/g, ' ')}</span>
                      </div>
                      <span className="text-gray-500 text-xs">{timeAgo(a.created_at)}</span>
                    </div>
                    {a.details && <p className="text-gray-400 text-xs truncate pl-6">{a.details}</p>}
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      )}
    </div>
  )
}

export default Profile
