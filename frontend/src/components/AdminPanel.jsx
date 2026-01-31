import { useState, useEffect } from 'react'
import { Link, Navigate } from 'react-router-dom'
import { useTradingMode } from '../context/TradingModeContext'
import { useCurrency } from '../context/CurrencyContext'
import { processStock } from '../utils/bxtrender'

function AdminPanel() {
  const token = localStorage.getItem('authToken')
  const [isAdmin, setIsAdmin] = useState(null)
  const [activeTab, setActiveTab] = useState('dashboard')
  const [stats, setStats] = useState(null)
  const [users, setUsers] = useState([])
  const [activities, setActivities] = useState([])
  const [traffic, setTraffic] = useState(null)
  const [loading, setLoading] = useState(true)
  const [selectedUser, setSelectedUser] = useState(null)
  const [activityFilter, setActivityFilter] = useState('')
  const [updatingStocks, setUpdatingStocks] = useState(false)
  const [updateProgress, setUpdateProgress] = useState(null)
  const { mode, isAggressive } = useTradingMode()
  const { formatPrice, convertPrice, convertToUSD, currencySymbol } = useCurrency()

  // Bots state
  const [flipperPositions, setFlipperPositions] = useState([])
  const [flipperTrades, setFlipperTrades] = useState([])
  const [lutzPositions, setLutzPositions] = useState([])
  const [lutzTrades, setLutzTrades] = useState([])
  const [botTab, setBotTab] = useState('flipper')
  const [editingItem, setEditingItem] = useState(null)

  useEffect(() => {
    checkAdmin()
  }, [])

  useEffect(() => {
    if (isAdmin) {
      if (activeTab === 'dashboard') fetchStats()
      if (activeTab === 'users') fetchUsers()
      if (activeTab === 'activity') fetchActivity()
      if (activeTab === 'traffic') fetchTraffic()
      if (activeTab === 'bots') fetchBotData()
    }
  }, [isAdmin, activeTab, activityFilter])

  const checkAdmin = async () => {
    if (!token) {
      setIsAdmin(false)
      return
    }
    try {
      const res = await fetch('/api/verify', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      setIsAdmin(data.valid && data.user?.is_admin)
    } catch {
      setIsAdmin(false)
    }
  }

  const fetchStats = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/admin/stats', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      setStats(data)
    } catch (err) {
      console.error('Failed to fetch stats:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchUsers = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/admin/users', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      setUsers(data)
    } catch (err) {
      console.error('Failed to fetch users:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchActivity = async () => {
    setLoading(true)
    try {
      let url = '/api/admin/activity?limit=200'
      if (activityFilter) url += `&action=${activityFilter}`
      const res = await fetch(url, {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      setActivities(data)
    } catch (err) {
      console.error('Failed to fetch activity:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchTraffic = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/admin/traffic', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      setTraffic(data)
    } catch (err) {
      console.error('Failed to fetch traffic:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchBotData = async () => {
    setLoading(true)
    try {
      const [fpRes, ftRes, lpRes, ltRes] = await Promise.all([
        fetch('/api/flipperbot/portfolio', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/flipperbot/actions', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/lutz/portfolio', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/lutz/actions', { headers: { 'Authorization': `Bearer ${token}` } })
      ])
      const [fp, ft, lp, lt] = await Promise.all([
        fpRes.json(), ftRes.json(), lpRes.json(), ltRes.json()
      ])
      setFlipperPositions(fp?.positions || [])
      setFlipperTrades(ft || [])
      setLutzPositions(lp?.positions || [])
      setLutzTrades(lt || [])
    } catch (err) {
      console.error('Failed to fetch bot data:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleUpdatePosition = async (bot, position) => {
    try {
      const res = await fetch(`/api/${bot}/position/${position.id}`, {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          quantity: parseFloat(position.quantity),
          avg_price: parseFloat(position.avg_price),
          is_live: position.is_live
        })
      })
      if (res.ok) {
        setEditingItem(null)
        fetchBotData()
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Speichern')
      }
    } catch (err) {
      console.error('Failed to update position:', err)
    }
  }

  const handleUpdateTrade = async (bot, trade) => {
    try {
      // Convert price back to USD for storage
      const priceInUSD = convertToUSD(parseFloat(trade.price))
      const res = await fetch(`/api/${bot}/trade/${trade.id}`, {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          quantity: parseFloat(trade.quantity),
          price: priceInUSD,
          is_live: trade.is_live
        })
      })
      if (res.ok) {
        setEditingItem(null)
        fetchBotData()
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Speichern')
      }
    } catch (err) {
      console.error('Failed to update trade:', err)
    }
  }

  const handleDeleteUser = async (userId) => {
    if (!confirm('Nutzer wirklich löschen? Alle Portfolio-Daten werden ebenfalls gelöscht.')) return
    try {
      const res = await fetch(`/api/admin/users/${userId}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
      })
      if (res.ok) {
        fetchUsers()
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Löschen')
      }
    } catch (err) {
      console.error('Failed to delete user:', err)
    }
  }

  const handleToggleAdmin = async (user) => {
    try {
      const res = await fetch(`/api/admin/users/${user.id}`, {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ is_admin: !user.is_admin })
      })
      if (res.ok) {
        fetchUsers()
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Aktualisieren')
      }
    } catch (err) {
      console.error('Failed to update user:', err)
    }
  }

  const formatDate = (dateStr) => {
    if (!dateStr) return '-'
    const date = new Date(dateStr)
    return date.toLocaleDateString('de-DE', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    })
  }

  const formatRelative = (dateStr) => {
    if (!dateStr) return 'Nie'
    const date = new Date(dateStr)
    const now = new Date()
    const diff = now - date

    if (diff < 60000) return 'Gerade eben'
    if (diff < 3600000) return `vor ${Math.floor(diff / 60000)} Min`
    if (diff < 86400000) return `vor ${Math.floor(diff / 3600000)} Std`
    if (diff < 604800000) return `vor ${Math.floor(diff / 86400000)} Tagen`
    return formatDate(dateStr)
  }

  const handleUpdateAllStocks = async () => {
    if (!confirm(`Alle Watchlist-Aktien aktualisieren? Das speichert BX-Trender Daten für BEIDE Modi (defensiv & aggressiv). Das kann mehrere Minuten dauern.`)) {
      return
    }

    setUpdatingStocks(true)
    setUpdateProgress({ current: 0, total: 0, status: 'Lade Aktien-Liste...' })

    try {
      // Get all stocks from watchlist
      const res = await fetch(`/api/admin/update-all-stocks?mode=${mode}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()

      if (!data.stocks || data.stocks.length === 0) {
        setUpdateProgress({ current: 0, total: 0, status: 'Keine Aktien in der Watchlist' })
        setUpdatingStocks(false)
        return
      }

      const stocks = data.stocks
      const total = stocks.length
      let successCount = 0
      let errorCount = 0

      setUpdateProgress({ current: 0, total, status: 'Verarbeite Aktien...' })

      // Process each stock
      for (let i = 0; i < stocks.length; i++) {
        const stock = stocks[i]
        setUpdateProgress({
          current: i,
          total,
          status: `Verarbeite ${stock.symbol}...`,
          currentStock: `${stock.symbol} - ${stock.name}`
        })

        const result = await processStock(stock.symbol, stock.name)

        if (result.success) {
          successCount++
        } else {
          errorCount++
          console.warn(`Failed to process ${stock.symbol}:`, result.error)
        }

        // Delay between stocks to avoid Yahoo Finance rate limiting
        await new Promise(r => setTimeout(r, 1000))
      }

      setUpdateProgress({
        current: total,
        total,
        status: `Fertig! ${successCount} erfolgreich, ${errorCount} fehlgeschlagen`,
        currentStock: null
      })

      // Refresh stats
      fetchStats()
    } catch (err) {
      console.error('Failed to update stocks:', err)
      setUpdateProgress({ status: 'Fehler: ' + err.message, current: 0, total: 0 })
    } finally {
      setUpdatingStocks(false)
    }
  }

  if (isAdmin === null) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <div className="w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full animate-spin"></div>
      </div>
    )
  }

  if (!isAdmin) {
    return <Navigate to="/" replace />
  }

  return (
    <div className="flex-1 p-4 md:p-6 overflow-auto">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-white">Admin Panel</h1>
          <p className="text-gray-500 text-sm">Nutzerverwaltung und Statistiken</p>
        </div>

        {/* Tabs */}
        <div className="flex gap-2 mb-6 border-b border-dark-600 pb-2 overflow-x-auto">
          {[
            { key: 'dashboard', label: 'Dashboard' },
            { key: 'users', label: 'Nutzer' },
            { key: 'activity', label: 'Aktivitäten' },
            { key: 'traffic', label: 'Traffic' },
            { key: 'bots', label: 'Bots' }
          ].map(tab => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
                activeTab === tab.key
                  ? 'bg-accent-500 text-white'
                  : 'text-gray-400 hover:text-white hover:bg-dark-700'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {loading ? (
          <div className="text-center py-12">
            <div className="w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full animate-spin mx-auto"></div>
          </div>
        ) : (
          <>
            {/* Dashboard Tab */}
            {activeTab === 'dashboard' && stats && (
              <div className="space-y-6">
                {/* Bulk Update Section */}
                <div className={`rounded-xl border p-4 ${
                  isAggressive
                    ? 'bg-orange-500/10 border-orange-500/30'
                    : 'bg-blue-500/10 border-blue-500/30'
                }`}>
                  <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                    <div>
                      <h2 className="text-lg font-semibold text-white flex items-center gap-2">
                        Watchlist Migration
                        <span className={`px-2 py-0.5 text-xs font-bold rounded ${
                          isAggressive
                            ? 'bg-orange-500/20 text-orange-400'
                            : 'bg-blue-500/20 text-blue-400'
                        }`}>
                          {isAggressive ? 'AGGRESSIV' : 'DEFENSIV'}
                        </span>
                      </h2>
                      <p className="text-sm text-gray-400 mt-1">
                        Aktualisiere alle Aktien der Watchlist im aktuell gewählten Modus.
                        Dies speichert die BX-Trender Performance-Daten für jede Aktie.
                      </p>
                    </div>
                    <button
                      onClick={handleUpdateAllStocks}
                      disabled={updatingStocks}
                      className={`flex items-center gap-2 px-4 py-2.5 rounded-lg font-medium transition-colors whitespace-nowrap ${
                        isAggressive
                          ? 'bg-orange-500 text-white hover:bg-orange-400 disabled:bg-orange-500/50'
                          : 'bg-blue-500 text-white hover:bg-blue-400 disabled:bg-blue-500/50'
                      } disabled:cursor-not-allowed`}
                    >
                      {updatingStocks ? (
                        <>
                          <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                          Aktualisiere...
                        </>
                      ) : (
                        <>
                          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                          </svg>
                          Alle Aktien aktualisieren
                        </>
                      )}
                    </button>
                  </div>
                  {updateProgress && (
                    <div className="mt-4 p-3 bg-dark-800 rounded-lg">
                      <div className="flex items-center justify-between text-sm mb-2">
                        <span className="text-gray-400">{updateProgress.status}</span>
                        {updateProgress.total > 0 && (
                          <span className="text-white font-medium">
                            {updateProgress.current} / {updateProgress.total}
                          </span>
                        )}
                      </div>
                      {updateProgress.total > 0 && (
                        <div className="w-full h-2 bg-dark-700 rounded-full overflow-hidden">
                          <div
                            className={`h-full transition-all duration-300 ${
                              isAggressive ? 'bg-orange-500' : 'bg-blue-500'
                            }`}
                            style={{ width: `${(updateProgress.current / updateProgress.total) * 100}%` }}
                          />
                        </div>
                      )}
                      {updateProgress.currentStock && (
                        <div className="text-xs text-gray-500 mt-2">
                          Aktuell: {updateProgress.currentStock}
                        </div>
                      )}
                    </div>
                  )}
                </div>

                {/* Main Stats */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                  <div className="bg-dark-800 rounded-xl border border-dark-600 p-4">
                    <div className="text-xs text-gray-500 mb-1">Nutzer gesamt</div>
                    <div className="text-3xl font-bold text-white">{stats.users}</div>
                  </div>
                  <div className="bg-dark-800 rounded-xl border border-dark-600 p-4">
                    <div className="text-xs text-gray-500 mb-1">Watchlist Aktien</div>
                    <div className="text-3xl font-bold text-white">{stats.stocks}</div>
                  </div>
                  <div className="bg-dark-800 rounded-xl border border-dark-600 p-4">
                    <div className="text-xs text-gray-500 mb-1">Portfolio Positionen</div>
                    <div className="text-3xl font-bold text-white">{stats.positions}</div>
                  </div>
                  <div className="bg-dark-800 rounded-xl border border-dark-600 p-4">
                    <div className="text-xs text-gray-500 mb-1">Getrackte Aktien</div>
                    <div className="text-3xl font-bold text-white">{stats.tracked_stocks}</div>
                  </div>
                </div>

                {/* Week Stats */}
                <div className="bg-dark-800 rounded-xl border border-dark-600 p-6">
                  <h2 className="text-lg font-semibold text-white mb-4">Letzte 7 Tage</h2>
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                    <div className="bg-dark-700 rounded-lg p-4">
                      <div className="text-xs text-gray-500 mb-1">Logins</div>
                      <div className="text-2xl font-bold text-accent-400">{stats.week_stats?.logins || 0}</div>
                    </div>
                    <div className="bg-dark-700 rounded-lg p-4">
                      <div className="text-xs text-gray-500 mb-1">Suchen</div>
                      <div className="text-2xl font-bold text-accent-400">{stats.week_stats?.searches || 0}</div>
                    </div>
                    <div className="bg-dark-700 rounded-lg p-4">
                      <div className="text-xs text-gray-500 mb-1">Seitenaufrufe</div>
                      <div className="text-2xl font-bold text-accent-400">{stats.week_stats?.page_views || 0}</div>
                    </div>
                    <div className="bg-dark-700 rounded-lg p-4">
                      <div className="text-xs text-gray-500 mb-1">Neue Nutzer</div>
                      <div className="text-2xl font-bold text-green-400">{stats.week_stats?.new_users || 0}</div>
                    </div>
                  </div>
                </div>

                <div className="grid md:grid-cols-2 gap-6">
                  {/* Most Active Users */}
                  <div className="bg-dark-800 rounded-xl border border-dark-600 p-6">
                    <h2 className="text-lg font-semibold text-white mb-4">Aktivste Nutzer (7 Tage)</h2>
                    {stats.most_active?.length > 0 ? (
                      <div className="space-y-2">
                        {stats.most_active.map((u, i) => (
                          <div key={u.user_id} className="flex items-center justify-between py-2 border-b border-dark-700 last:border-0">
                            <div className="flex items-center gap-3">
                              <span className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold ${
                                i === 0 ? 'bg-yellow-500/20 text-yellow-400' :
                                i === 1 ? 'bg-gray-400/20 text-gray-300' :
                                i === 2 ? 'bg-orange-500/20 text-orange-400' :
                                'bg-dark-700 text-gray-500'
                              }`}>{i + 1}</span>
                              <span className="text-white">{u.username || 'Anonym'}</span>
                            </div>
                            <span className="text-gray-400">{u.count} Aktionen</span>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-gray-500">Keine Daten</p>
                    )}
                  </div>

                  {/* Recent Stocks */}
                  <div className="bg-dark-800 rounded-xl border border-dark-600 p-6">
                    <h2 className="text-lg font-semibold text-white mb-4">Zuletzt hinzugefügte Aktien</h2>
                    {stats.recent_stocks?.length > 0 ? (
                      <div className="space-y-2">
                        {stats.recent_stocks.map((s) => (
                          <div key={s.id} className="flex items-center justify-between py-2 border-b border-dark-700 last:border-0">
                            <div>
                              <span className="font-medium text-white">{s.symbol}</span>
                              <span className="text-gray-500 text-sm ml-2">{s.name}</span>
                            </div>
                            <div className="text-right text-xs text-gray-500">
                              <div>{s.added_by_user || '-'}</div>
                              <div>{formatRelative(s.created_at)}</div>
                            </div>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-gray-500">Keine Aktien</p>
                    )}
                  </div>
                </div>
              </div>
            )}

            {/* Users Tab */}
            {activeTab === 'users' && (
              <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
                <div className="overflow-x-auto">
                  <table className="w-full">
                    <thead>
                      <tr className="text-left text-xs text-gray-500 border-b border-dark-600 bg-dark-900/50">
                        <th className="p-4">Nutzer</th>
                        <th className="p-4">Email</th>
                        <th className="p-4">Rolle</th>
                        <th className="p-4 text-right">Portfolio</th>
                        <th className="p-4 text-right">Logins</th>
                        <th className="p-4">Zuletzt aktiv</th>
                        <th className="p-4">Registriert</th>
                        <th className="p-4"></th>
                      </tr>
                    </thead>
                    <tbody>
                      {users.map((user) => (
                        <tr key={user.id} className="border-b border-dark-700/50 hover:bg-dark-700/30">
                          <td className="p-4">
                            <div className="flex items-center gap-3">
                              <div className="w-8 h-8 bg-accent-500/20 rounded-full flex items-center justify-center">
                                <span className="text-accent-400 font-bold text-sm">
                                  {user.username?.charAt(0).toUpperCase()}
                                </span>
                              </div>
                              <span className="font-medium text-white">{user.username}</span>
                            </div>
                          </td>
                          <td className="p-4 text-gray-400">{user.email}</td>
                          <td className="p-4">
                            {user.is_admin ? (
                              <span className="px-2 py-1 bg-green-500/20 text-green-400 text-xs rounded">Admin</span>
                            ) : (
                              <span className="px-2 py-1 bg-dark-700 text-gray-400 text-xs rounded">User</span>
                            )}
                          </td>
                          <td className="p-4 text-right text-white">{user.portfolio_count}</td>
                          <td className="p-4 text-right text-white">{user.login_count}</td>
                          <td className="p-4 text-gray-500 text-sm">{formatRelative(user.last_active)}</td>
                          <td className="p-4 text-gray-500 text-sm">{formatDate(user.created_at)}</td>
                          <td className="p-4">
                            <div className="flex items-center gap-2">
                              <button
                                onClick={() => handleToggleAdmin(user)}
                                className="p-1.5 text-gray-400 hover:text-accent-400 transition-colors"
                                title={user.is_admin ? 'Admin entfernen' : 'Zum Admin machen'}
                              >
                                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                                </svg>
                              </button>
                              <button
                                onClick={() => handleDeleteUser(user.id)}
                                className="p-1.5 text-gray-400 hover:text-red-400 transition-colors"
                                title="Nutzer löschen"
                              >
                                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                </svg>
                              </button>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {/* Activity Tab */}
            {activeTab === 'activity' && (
              <div className="space-y-4">
                {/* Filter */}
                <div className="flex gap-2">
                  {['', 'login', 'add_stock', 'search', 'page_view'].map(filter => (
                    <button
                      key={filter}
                      onClick={() => setActivityFilter(filter)}
                      className={`px-3 py-1.5 text-sm rounded-lg transition-colors ${
                        activityFilter === filter
                          ? 'bg-accent-500 text-white'
                          : 'bg-dark-700 text-gray-400 hover:text-white'
                      }`}
                    >
                      {filter === '' ? 'Alle' :
                       filter === 'login' ? 'Logins' :
                       filter === 'add_stock' ? 'Aktien hinzugefügt' :
                       filter === 'search' ? 'Suchen' :
                       'Seitenaufrufe'}
                    </button>
                  ))}
                </div>

                <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
                  <div className="overflow-x-auto max-h-[600px] overflow-y-auto">
                    <table className="w-full">
                      <thead className="sticky top-0 bg-dark-900">
                        <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                          <th className="p-4">Zeit</th>
                          <th className="p-4">Nutzer</th>
                          <th className="p-4">Aktion</th>
                          <th className="p-4">Details</th>
                          <th className="p-4">IP</th>
                        </tr>
                      </thead>
                      <tbody>
                        {activities.map((log) => (
                          <tr key={log.id} className="border-b border-dark-700/50 hover:bg-dark-700/30">
                            <td className="p-4 text-gray-500 text-sm whitespace-nowrap">{formatDate(log.created_at)}</td>
                            <td className="p-4">
                              <span className="text-white">{log.username || 'Anonym'}</span>
                            </td>
                            <td className="p-4">
                              <span className={`px-2 py-1 text-xs rounded ${
                                log.action === 'login' ? 'bg-green-500/20 text-green-400' :
                                log.action === 'add_stock' ? 'bg-blue-500/20 text-blue-400' :
                                log.action === 'search' ? 'bg-yellow-500/20 text-yellow-400' :
                                'bg-dark-700 text-gray-400'
                              }`}>
                                {log.action}
                              </span>
                            </td>
                            <td className="p-4 text-gray-400 text-sm max-w-[200px] truncate">{log.details}</td>
                            <td className="p-4 text-gray-500 text-sm">{log.ip_address}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>
            )}

            {/* Traffic Tab */}
            {activeTab === 'traffic' && traffic && (
              <div className="space-y-6">
                {/* Today's Stats */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                  <div className="bg-dark-800 rounded-xl border border-dark-600 p-4">
                    <div className="text-xs text-gray-500 mb-1">Besucher heute</div>
                    <div className="text-3xl font-bold text-accent-400">{traffic.unique_today || 0}</div>
                    <div className="text-xs text-gray-500">Unique IPs</div>
                  </div>
                  <div className="bg-dark-800 rounded-xl border border-dark-600 p-4">
                    <div className="text-xs text-gray-500 mb-1">Aufrufe heute</div>
                    <div className="text-3xl font-bold text-white">{traffic.views_today || 0}</div>
                    <div className="text-xs text-gray-500">Gesamt</div>
                  </div>
                </div>

                {/* Daily Traffic Chart */}
                {traffic.daily && traffic.daily.length > 0 && (
                  <div className="bg-dark-800 rounded-xl border border-dark-600 p-6">
                    <h2 className="text-lg font-semibold text-white mb-4">Traffic letzte 7 Tage</h2>
                    <div className="flex items-end gap-2 h-32">
                      {traffic.daily.slice().reverse().map((day, idx) => {
                        const maxCount = Math.max(...traffic.daily.map(d => d.count))
                        const height = maxCount > 0 ? (day.count / maxCount) * 100 : 0
                        return (
                          <div key={idx} className="flex-1 flex flex-col items-center gap-1">
                            <div
                              className="w-full bg-accent-500 rounded-t transition-all"
                              style={{ height: `${height}%`, minHeight: day.count > 0 ? '4px' : '0' }}
                              title={`${day.count} Aufrufe`}
                            />
                            <span className="text-[10px] text-gray-500">{day.date?.slice(5) || ''}</span>
                            <span className="text-xs text-gray-400">{day.count}</span>
                          </div>
                        )
                      })}
                    </div>
                  </div>
                )}

                <div className="grid md:grid-cols-2 gap-6">
                  {/* Traffic by IP */}
                  <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
                    <div className="p-4 border-b border-dark-600">
                      <h2 className="text-lg font-semibold text-white">Traffic nach IP</h2>
                      <p className="text-xs text-gray-500">Top 50 IP-Adressen</p>
                    </div>
                    <div className="max-h-[400px] overflow-auto">
                      <table className="w-full">
                        <thead className="sticky top-0 bg-dark-900">
                          <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                            <th className="p-3">IP-Adresse</th>
                            <th className="p-3 text-right">Aufrufe</th>
                            <th className="p-3">Letzter Besuch</th>
                          </tr>
                        </thead>
                        <tbody>
                          {traffic.by_ip?.map((ip, idx) => (
                            <tr key={idx} className="border-b border-dark-700/50 hover:bg-dark-700/30">
                              <td className="p-3 font-mono text-sm text-white">{ip.ip_address}</td>
                              <td className="p-3 text-right">
                                <span className="px-2 py-1 bg-accent-500/20 text-accent-400 rounded text-sm font-medium">
                                  {ip.count}
                                </span>
                              </td>
                              <td className="p-3 text-gray-500 text-xs">
                                {ip.last_visit ? new Date(ip.last_visit).toLocaleString('de-DE') : '-'}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>

                  {/* Traffic by Device */}
                  <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
                    <div className="p-4 border-b border-dark-600">
                      <h2 className="text-lg font-semibold text-white">Traffic nach Gerät</h2>
                      <p className="text-xs text-gray-500">User-Agent Analyse</p>
                    </div>
                    <div className="max-h-[400px] overflow-auto">
                      <table className="w-full">
                        <thead className="sticky top-0 bg-dark-900">
                          <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                            <th className="p-3">Gerät</th>
                            <th className="p-3">User-Agent</th>
                            <th className="p-3 text-right">Aufrufe</th>
                          </tr>
                        </thead>
                        <tbody>
                          {traffic.by_device?.map((device, idx) => (
                            <tr key={idx} className="border-b border-dark-700/50 hover:bg-dark-700/30">
                              <td className="p-3">
                                <span className={`px-2 py-1 text-xs rounded ${
                                  device.device === 'Mobile' ? 'bg-blue-500/20 text-blue-400' :
                                  device.device === 'Tablet' ? 'bg-purple-500/20 text-purple-400' :
                                  device.device === 'Bot' ? 'bg-yellow-500/20 text-yellow-400' :
                                  'bg-green-500/20 text-green-400'
                                }`}>
                                  {device.device}
                                </span>
                              </td>
                              <td className="p-3 text-gray-400 text-xs max-w-[200px] truncate" title={device.user_agent}>
                                {device.user_agent}
                              </td>
                              <td className="p-3 text-right">
                                <span className="px-2 py-1 bg-dark-700 text-white rounded text-sm font-medium">
                                  {device.count}
                                </span>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                </div>
              </div>
            )}

            {/* Bots Tab */}
            {activeTab === 'bots' && (
              <div className="space-y-6">
                {/* Bot Selector */}
                <div className="flex gap-2">
                  <button
                    onClick={() => setBotTab('flipper')}
                    className={`px-4 py-2 rounded-lg font-medium transition-colors ${
                      botTab === 'flipper'
                        ? 'bg-blue-500 text-white'
                        : 'bg-dark-700 text-gray-400 hover:text-white'
                    }`}
                  >
                    FlipperBot (Defensiv)
                  </button>
                  <button
                    onClick={() => setBotTab('lutz')}
                    className={`px-4 py-2 rounded-lg font-medium transition-colors ${
                      botTab === 'lutz'
                        ? 'bg-orange-500 text-white'
                        : 'bg-dark-700 text-gray-400 hover:text-white'
                    }`}
                  >
                    Lutz (Aggressiv)
                  </button>
                </div>

                {/* Info */}
                <div className="p-4 bg-blue-500/10 border border-blue-500/30 rounded-xl text-sm text-blue-300">
                  <p>Trades bearbeiten um echte Werte einzutragen. Positionen werden automatisch aktualisiert.</p>
                </div>

                {/* Trades Table */}
                <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
                  <div className="p-4 border-b border-dark-600">
                    <h2 className="text-lg font-semibold text-white">
                      {botTab === 'flipper' ? 'FlipperBot' : 'Lutz'} Trades
                    </h2>
                    <p className="text-xs text-gray-500">BUY-Trades bearbeiten um Position zu aktualisieren</p>
                  </div>
                  <div className="overflow-x-auto max-h-[500px] overflow-y-auto">
                    <table className="w-full">
                      <thead className="sticky top-0 bg-dark-900">
                        <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                          <th className="p-3">Datum</th>
                          <th className="p-3">Symbol</th>
                          <th className="p-3">Typ</th>
                          <th className="p-3 text-right">Anzahl</th>
                          <th className="p-3 text-right">Preis</th>
                          <th className="p-3 text-center">LIVE</th>
                          <th className="p-3"></th>
                        </tr>
                      </thead>
                      <tbody>
                        {(botTab === 'flipper' ? flipperTrades : lutzTrades).slice(0, 50).map((trade) => (
                          <tr key={trade.id} className={`border-b border-dark-700/50 hover:bg-dark-700/30 ${trade.is_live ? 'bg-green-500/5' : ''}`}>
                            {editingItem?.type === 'trade' && editingItem?.id === trade.id ? (
                              <>
                                <td className="p-3 text-gray-400 text-sm">
                                  {new Date(trade.signal_date || trade.created_at).toLocaleDateString('de-DE')}
                                </td>
                                <td className="p-3 font-medium text-white">{trade.symbol}</td>
                                <td className="p-3">
                                  <span className={`px-2 py-1 text-xs rounded ${
                                    trade.action === 'BUY' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
                                  }`}>
                                    {trade.action}
                                  </span>
                                </td>
                                <td className="p-3">
                                  <input
                                    type="number"
                                    step="0.0001"
                                    value={editingItem.quantity}
                                    onChange={(e) => setEditingItem({...editingItem, quantity: e.target.value})}
                                    className="w-24 bg-dark-700 border border-dark-500 rounded px-2 py-1 text-white text-right"
                                  />
                                </td>
                                <td className="p-3">
                                  <div className="flex items-center gap-1">
                                    <span className="text-gray-400 text-sm">{currencySymbol}</span>
                                    <input
                                      type="number"
                                      step="0.01"
                                      value={editingItem.price}
                                      onChange={(e) => setEditingItem({...editingItem, price: e.target.value})}
                                      className="w-24 bg-dark-700 border border-dark-500 rounded px-2 py-1 text-white text-right"
                                    />
                                  </div>
                                </td>
                                <td className="p-3 text-center">
                                  <button
                                    onClick={() => setEditingItem({...editingItem, is_live: !editingItem.is_live})}
                                    className={`px-3 py-1 rounded text-xs font-bold transition-colors ${
                                      editingItem.is_live
                                        ? 'bg-green-500 text-white'
                                        : 'bg-dark-600 text-gray-400'
                                    }`}
                                  >
                                    {editingItem.is_live ? 'LIVE' : 'SIM'}
                                  </button>
                                </td>
                                <td className="p-3">
                                  <div className="flex gap-2">
                                    <button
                                      onClick={() => handleUpdateTrade(botTab === 'flipper' ? 'flipperbot' : 'lutz', editingItem)}
                                      className="px-3 py-1 bg-green-500 text-white rounded text-xs font-medium"
                                    >
                                      Speichern
                                    </button>
                                    <button
                                      onClick={() => setEditingItem(null)}
                                      className="px-3 py-1 bg-dark-600 text-gray-400 rounded text-xs"
                                    >
                                      Abbrechen
                                    </button>
                                  </div>
                                </td>
                              </>
                            ) : (
                              <>
                                <td className="p-3 text-gray-400 text-sm">
                                  {new Date(trade.signal_date || trade.created_at).toLocaleDateString('de-DE')}
                                </td>
                                <td className="p-3 font-medium text-white">{trade.symbol}</td>
                                <td className="p-3">
                                  <span className={`px-2 py-1 text-xs rounded ${
                                    trade.action === 'BUY' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
                                  }`}>
                                    {trade.action}
                                  </span>
                                </td>
                                <td className="p-3 text-right text-white">{trade.quantity}</td>
                                <td className="p-3 text-right text-white">{formatPrice(trade.price, trade.symbol)}</td>
                                <td className="p-3 text-center">
                                  {trade.is_live && (
                                    <span className="px-2 py-1 bg-green-500/20 text-green-400 rounded text-xs font-bold">
                                      LIVE
                                    </span>
                                  )}
                                </td>
                                <td className="p-3">
                                  <button
                                    onClick={() => setEditingItem({
                                      type: 'trade',
                                      id: trade.id,
                                      quantity: trade.quantity,
                                      price: convertPrice(trade.price)?.toFixed(2) || trade.price,
                                      is_live: trade.is_live || false
                                    })}
                                    className="p-1.5 text-gray-400 hover:text-accent-400 transition-colors"
                                  >
                                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                                    </svg>
                                  </button>
                                </td>
                              </>
                            )}
                          </tr>
                        ))}
                        {(botTab === 'flipper' ? flipperTrades : lutzTrades).length === 0 && (
                          <tr>
                            <td colSpan={7} className="p-8 text-center text-gray-500">
                              Keine Trades vorhanden
                            </td>
                          </tr>
                        )}
                      </tbody>
                    </table>
                  </div>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}

export default AdminPanel
