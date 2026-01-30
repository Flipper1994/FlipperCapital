import { useState, useEffect } from 'react'
import { Link, Navigate } from 'react-router-dom'

function AdminPanel() {
  const token = localStorage.getItem('authToken')
  const [isAdmin, setIsAdmin] = useState(null)
  const [activeTab, setActiveTab] = useState('dashboard')
  const [stats, setStats] = useState(null)
  const [users, setUsers] = useState([])
  const [activities, setActivities] = useState([])
  const [loading, setLoading] = useState(true)
  const [selectedUser, setSelectedUser] = useState(null)
  const [activityFilter, setActivityFilter] = useState('')

  useEffect(() => {
    checkAdmin()
  }, [])

  useEffect(() => {
    if (isAdmin) {
      if (activeTab === 'dashboard') fetchStats()
      if (activeTab === 'users') fetchUsers()
      if (activeTab === 'activity') fetchActivity()
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
        <div className="flex gap-2 mb-6 border-b border-dark-600 pb-2">
          {[
            { key: 'dashboard', label: 'Dashboard' },
            { key: 'users', label: 'Nutzer' },
            { key: 'activity', label: 'Aktivitäten' }
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
          </>
        )}
      </div>
    </div>
  )
}

export default AdminPanel
