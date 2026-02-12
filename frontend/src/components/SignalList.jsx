import { useState, useEffect, useMemo } from 'react'
import { useCurrency } from '../context/CurrencyContext'

const MODE_COLORS = {
  defensive:  { bg: 'bg-blue-500/20', text: 'text-blue-400', border: 'border-blue-500/30', label: 'Defensiv' },
  aggressive: { bg: 'bg-orange-500/20', text: 'text-orange-400', border: 'border-orange-500/30', label: 'Aggressiv' },
  quant:      { bg: 'bg-violet-500/20', text: 'text-violet-400', border: 'border-violet-500/30', label: 'Quant' },
  ditz:       { bg: 'bg-cyan-500/20', text: 'text-cyan-400', border: 'border-cyan-500/30', label: 'Ditz' },
  trader:     { bg: 'bg-emerald-500/20', text: 'text-emerald-400', border: 'border-emerald-500/30', label: 'Trader' },
}

function generateMonthOptions() {
  const options = []
  const now = new Date()
  for (let i = 0; i < 36; i++) {
    const date = new Date(now.getFullYear(), now.getMonth() - i, 1)
    options.push({
      value: `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`,
      label: date.toLocaleDateString('de-DE', { month: 'long', year: 'numeric' })
    })
  }
  return options
}

function SignalList({ token, isAdmin }) {
  const { formatPrice } = useCurrency()
  const monthOptions = useMemo(() => generateMonthOptions(), [])

  const [entries, setEntries] = useState([])
  const [loading, setLoading] = useState(true)
  const [selectedMonth, setSelectedMonth] = useState(() => {
    const now = new Date()
    return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
  })
  const [searchQuery, setSearchQuery] = useState('')
  const [filters, setFilters] = useState({
    minWinrate: '', maxWinrate: '', minRR: '', maxRR: '',
    minAvgReturn: '', maxAvgReturn: '', minMarketCap: ''
  })
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [sortField, setSortField] = useState('mode_count')
  const [sortDir, setSortDir] = useState('desc')
  const [signalFilter, setSignalFilter] = useState(null)

  // Load admin default filters on mount
  useEffect(() => {
    fetch('/api/signal-list/filter-config')
      .then(r => r.json())
      .then(config => {
        if (config && config.id) {
          setFilters({
            minWinrate: config.min_winrate ?? '',
            maxWinrate: config.max_winrate ?? '',
            minRR: config.min_rr ?? '',
            maxRR: config.max_rr ?? '',
            minAvgReturn: config.min_avg_return ?? '',
            maxAvgReturn: config.max_avg_return ?? '',
            minMarketCap: config.min_market_cap ?? '',
          })
        }
      })
      .catch(() => {})
  }, [])

  // Fetch signal list data
  useEffect(() => {
    setLoading(true)
    const headers = token ? { 'Authorization': `Bearer ${token}` } : {}
    fetch(`/api/signal-list?month=${selectedMonth}`, { headers })
      .then(r => r.json())
      .then(data => setEntries(data.entries || []))
      .catch(() => setEntries([]))
      .finally(() => setLoading(false))
  }, [selectedMonth, token])

  const handleFilterChange = (key, value) => {
    setFilters(prev => ({ ...prev, [key]: value }))
  }

  const hasActiveFilters = Object.values(filters).some(v => v !== '')

  const clearFilters = () => {
    setFilters({ minWinrate: '', maxWinrate: '', minRR: '', maxRR: '', minAvgReturn: '', maxAvgReturn: '', minMarketCap: '' })
  }

  const saveFiltersAsDefault = async () => {
    try {
      await fetch('/api/admin/signal-list/filter-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({
          min_winrate: filters.minWinrate !== '' ? parseFloat(filters.minWinrate) : null,
          max_winrate: filters.maxWinrate !== '' ? parseFloat(filters.maxWinrate) : null,
          min_rr: filters.minRR !== '' ? parseFloat(filters.minRR) : null,
          max_rr: filters.maxRR !== '' ? parseFloat(filters.maxRR) : null,
          min_avg_return: filters.minAvgReturn !== '' ? parseFloat(filters.minAvgReturn) : null,
          max_avg_return: filters.maxAvgReturn !== '' ? parseFloat(filters.maxAvgReturn) : null,
          min_market_cap: filters.minMarketCap !== '' ? parseFloat(filters.minMarketCap) : null,
        })
      })
    } catch (err) {
      console.error('Failed to save filters:', err)
    }
  }

  const toggleVisibility = async (symbol, currentVisible) => {
    if (!isAdmin) return
    try {
      await fetch('/api/admin/signal-list/visibility', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ symbol, month: selectedMonth, visible: !currentVisible })
      })
      setEntries(prev => prev.map(e =>
        e.symbol === symbol ? { ...e, visible: !currentVisible } : e
      ))
    } catch (err) {
      console.error('Failed to toggle visibility:', err)
    }
  }

  // Navigate months
  const navigateMonth = (direction) => {
    const idx = monthOptions.findIndex(o => o.value === selectedMonth)
    const newIdx = idx + direction
    if (newIdx >= 0 && newIdx < monthOptions.length) {
      setSelectedMonth(monthOptions[newIdx].value)
    }
  }

  const currentPageIdx = monthOptions.findIndex(o => o.value === selectedMonth)
  const currentPage = currentPageIdx + 1
  const totalPages = monthOptions.length
  const selectedMonthLabel = monthOptions[currentPageIdx]?.label || selectedMonth

  // Filter & sort
  const filteredEntries = useMemo(() => {
    return entries.filter(entry => {
      if (searchQuery) {
        const q = searchQuery.toLowerCase()
        if (!entry.symbol.toLowerCase().includes(q) && !(entry.name || '').toLowerCase().includes(q)) return false
      }
      if (signalFilter && entry.signal !== signalFilter) return false
      if (filters.minWinrate !== '' && entry.win_rate < parseFloat(filters.minWinrate)) return false
      if (filters.maxWinrate !== '' && entry.win_rate > parseFloat(filters.maxWinrate)) return false
      if (filters.minRR !== '' && entry.risk_reward < parseFloat(filters.minRR)) return false
      if (filters.maxRR !== '' && entry.risk_reward > parseFloat(filters.maxRR)) return false
      if (filters.minAvgReturn !== '' && entry.avg_return < parseFloat(filters.minAvgReturn)) return false
      if (filters.maxAvgReturn !== '' && entry.avg_return > parseFloat(filters.maxAvgReturn)) return false
      if (filters.minMarketCap !== '' && entry.market_cap < parseFloat(filters.minMarketCap) * 1e9) return false
      return true
    })
  }, [entries, searchQuery, signalFilter, filters])

  const sortedEntries = useMemo(() => {
    return [...filteredEntries].sort((a, b) => {
      if (sortField === 'mode_count') {
        // Default: BUY first, then by total mode_count desc, then market_cap desc
        if (a.signal !== b.signal) return a.signal === 'BUY' ? -1 : 1
        if (a.mode_count !== b.mode_count) return sortDir === 'desc' ? b.mode_count - a.mode_count : a.mode_count - b.mode_count
        return (b.market_cap || 0) - (a.market_cap || 0)
      }
      let aVal = a[sortField], bVal = b[sortField]
      if (typeof aVal === 'string') {
        aVal = (aVal || '').toLowerCase()
        bVal = (bVal || '').toLowerCase()
      }
      if (aVal < bVal) return sortDir === 'asc' ? -1 : 1
      if (aVal > bVal) return sortDir === 'asc' ? 1 : -1
      return 0
    })
  }, [filteredEntries, sortField, sortDir])

  const handleSort = (field) => {
    if (sortField === field) {
      setSortDir(d => d === 'asc' ? 'desc' : 'asc')
    } else {
      setSortField(field)
      setSortDir('desc')
    }
  }

  const SortIcon = ({ field }) => {
    if (sortField !== field) return null
    return (
      <svg className="w-3 h-3 inline ml-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        {sortDir === 'asc'
          ? <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
          : <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        }
      </svg>
    )
  }

  // Helpers
  const getSignalStyle = (signal) => {
    switch (signal) {
      case 'BUY': return 'bg-green-500/20 text-green-400 border-green-500/30'
      case 'SELL': return 'bg-red-500/20 text-red-400 border-red-500/30'
      default: return 'bg-gray-500/20 text-gray-400 border-gray-500/30'
    }
  }

  const formatPercent = (value) => {
    if (value === undefined || value === null || isNaN(value)) return '--'
    const sign = value >= 0 ? '+' : ''
    return `${sign}${value.toFixed(1)}%`
  }

  const formatModeReturn = (mode) => {
    if (mode.trade_return_pct == null) return null
    const sign = mode.trade_return_pct >= 0 ? '+' : ''
    const pct = `${sign}${mode.trade_return_pct.toFixed(1)}%`
    const dur = mode.trade_duration_months != null ? ` (${mode.trade_duration_months}M)` : ''
    return pct + dur
  }

  const formatMarketCap = (mc) => {
    if (!mc || mc <= 0) return '-'
    if (mc >= 1e12) return `${(mc / 1e12).toFixed(2)}T`
    if (mc >= 1e9) return `${(mc / 1e9).toFixed(1)}B`
    if (mc >= 1e6) return `${(mc / 1e6).toFixed(0)}M`
    return `${mc.toLocaleString('de-DE')} $`
  }

  const formatSignalDate = (dateStr) => {
    if (!dateStr) return '-'
    const date = new Date(dateStr)
    return date.toLocaleDateString('de-DE', { day: '2-digit', month: 'short', year: '2-digit' })
  }

  // Stats
  const buyCount = filteredEntries.filter(e => e.signal === 'BUY').length
  const sellCount = filteredEntries.filter(e => e.signal === 'SELL').length

  return (
    <div className="flex-1 p-4 md:p-6 overflow-auto">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-4 md:mb-6">
          <div className="flex items-center justify-between flex-wrap gap-3">
            <div>
              <h1 className="text-xl md:text-2xl font-bold text-white">Signal Liste</h1>
              <p className="text-gray-500 text-sm mt-0.5">BUY und SELL Signale aller Trading-Modi</p>
            </div>

            {/* Page Navigation */}
            <div className="flex items-center gap-3">
              <button
                onClick={() => navigateMonth(-1)}
                disabled={currentPageIdx <= 0}
                className="p-2 bg-dark-700 border border-dark-600 rounded-lg text-white hover:bg-dark-600 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                title="Neuerer Monat"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                </svg>
              </button>
              <div className="text-center min-w-[140px]">
                <div className="text-white font-semibold text-sm">{selectedMonthLabel}</div>
                <div className="text-gray-500 text-xs">Seite {currentPage} / {totalPages}</div>
              </div>
              <button
                onClick={() => navigateMonth(1)}
                disabled={currentPageIdx >= totalPages - 1}
                className="p-2 bg-dark-700 border border-dark-600 rounded-lg text-white hover:bg-dark-600 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                title="Älterer Monat"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                </svg>
              </button>
            </div>
          </div>

          {/* Search */}
          <div className="mt-3 relative">
            <input
              type="text"
              placeholder="Suche nach Ticker oder Name..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full md:w-80 px-3 py-2 pl-9 bg-dark-700 border border-dark-600 rounded-lg text-sm text-white placeholder-gray-500 focus:outline-none focus:border-accent-500"
            />
            <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            {searchQuery && (
              <button onClick={() => setSearchQuery('')} className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-white">
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            )}
          </div>
        </div>

        {/* Filters */}
        <div className="mb-3 bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
          <button onClick={() => setFiltersOpen(!filtersOpen)}
            className="w-full px-4 py-2.5 flex items-center justify-between text-left hover:bg-dark-700/50 transition-colors">
            <div className="flex items-center gap-2">
              <svg className="w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
              </svg>
              <span className="text-white font-medium text-sm">Filter</span>
              {hasActiveFilters && <span className="px-1.5 py-0.5 text-xs bg-accent-500 text-white rounded-full">Aktiv</span>}
            </div>
            <svg className={`w-4 h-4 text-gray-400 transition-transform ${filtersOpen ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>
          {filtersOpen && (
            <div className="px-4 pb-3 border-t border-dark-600">
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mt-3">
                <div>
                  <label className="block text-xs text-gray-400 mb-1">Winrate (%)</label>
                  <div className="flex gap-2">
                    <input type="number" placeholder="Min" value={filters.minWinrate} onChange={e => handleFilterChange('minWinrate', e.target.value)}
                      className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
                    <input type="number" placeholder="Max" value={filters.maxWinrate} onChange={e => handleFilterChange('maxWinrate', e.target.value)}
                      className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
                  </div>
                </div>
                <div>
                  <label className="block text-xs text-gray-400 mb-1">Risk/Reward</label>
                  <div className="flex gap-2">
                    <input type="number" step="0.1" placeholder="Min" value={filters.minRR} onChange={e => handleFilterChange('minRR', e.target.value)}
                      className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
                    <input type="number" step="0.1" placeholder="Max" value={filters.maxRR} onChange={e => handleFilterChange('maxRR', e.target.value)}
                      className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
                  </div>
                </div>
                <div>
                  <label className="block text-xs text-gray-400 mb-1">&Oslash; Rendite (%)</label>
                  <div className="flex gap-2">
                    <input type="number" step="0.1" placeholder="Min" value={filters.minAvgReturn} onChange={e => handleFilterChange('minAvgReturn', e.target.value)}
                      className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
                    <input type="number" step="0.1" placeholder="Max" value={filters.maxAvgReturn} onChange={e => handleFilterChange('maxAvgReturn', e.target.value)}
                      className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
                  </div>
                </div>
                <div>
                  <label className="block text-xs text-gray-400 mb-1">Min Market Cap (Mrd)</label>
                  <input type="number" step="0.1" placeholder="z.B. 10" value={filters.minMarketCap} onChange={e => handleFilterChange('minMarketCap', e.target.value)}
                    className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500" />
                </div>
              </div>
              <div className="mt-2 flex justify-end gap-2">
                {hasActiveFilters && (
                  <button onClick={clearFilters} className="px-3 py-1 text-xs text-gray-400 hover:text-white transition-colors">Filter zur&uuml;cksetzen</button>
                )}
                {isAdmin && (
                  <button onClick={saveFiltersAsDefault} className="px-3 py-1 text-xs bg-accent-500 text-white rounded hover:bg-accent-600 transition-colors">Filter speichern</button>
                )}
              </div>
            </div>
          )}
        </div>

        {/* Stats */}
        <div className="grid grid-cols-3 gap-3 mb-4">
          <button onClick={() => setSignalFilter(null)}
            className={`p-3 rounded-xl border transition-colors ${!signalFilter ? 'bg-dark-700 border-accent-500' : 'bg-dark-800 border-dark-600 hover:border-dark-500'}`}>
            <div className="text-xs text-gray-500">Gesamt</div>
            <div className="text-xl font-bold text-white">{filteredEntries.length}</div>
          </button>
          <button onClick={() => setSignalFilter(signalFilter === 'BUY' ? null : 'BUY')}
            className={`p-3 rounded-xl border transition-colors ${signalFilter === 'BUY' ? 'bg-green-500/10 border-green-500/50' : 'bg-dark-800 border-dark-600 hover:border-dark-500'}`}>
            <div className="text-xs text-gray-500">BUY Signale</div>
            <div className="text-xl font-bold text-green-400">{buyCount}</div>
          </button>
          <button onClick={() => setSignalFilter(signalFilter === 'SELL' ? null : 'SELL')}
            className={`p-3 rounded-xl border transition-colors ${signalFilter === 'SELL' ? 'bg-red-500/10 border-red-500/50' : 'bg-dark-800 border-dark-600 hover:border-dark-500'}`}>
            <div className="text-xs text-gray-500">SELL Signale</div>
            <div className="text-xl font-bold text-red-400">{sellCount}</div>
          </button>
        </div>

        {loading ? (
          <div className="text-center py-12">
            <div className="w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full animate-spin mx-auto"></div>
            <p className="text-gray-500 mt-4">Lade Signale...</p>
          </div>
        ) : sortedEntries.length === 0 ? (
          <div className="bg-dark-800 rounded-xl border border-dark-600 p-12 text-center">
            <div className="w-20 h-20 mx-auto mb-6 bg-dark-700 rounded-full flex items-center justify-center">
              <svg className="w-10 h-10 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
              </svg>
            </div>
            <h2 className="text-xl font-semibold text-gray-400 mb-3">Keine Signale f&uuml;r diesen Monat</h2>
            <p className="text-gray-600 max-w-md mx-auto">
              Es gibt aktuell keine BUY oder SELL Signale f&uuml;r {selectedMonthLabel}.
            </p>
          </div>
        ) : (
          <>
            {/* Mobile Cards */}
            <div className="md:hidden space-y-2">
              {sortedEntries.map((entry) => (
                <div
                  key={entry.symbol}
                  onClick={() => isAdmin && toggleVisibility(entry.symbol, entry.visible)}
                  className={`bg-dark-800 rounded-xl border border-dark-600 p-3 ${isAdmin ? 'cursor-pointer' : ''} ${!entry.visible ? 'opacity-50' : ''}`}
                >
                  <div className="flex items-center justify-between mb-2">
                    <div>
                      <div className={`font-semibold text-white ${!entry.visible ? 'line-through' : ''}`}>{entry.name} <span className="text-gray-500 text-xs font-normal">({entry.symbol})</span></div>
                    </div>
                    <span className={`px-2 py-1 text-xs font-bold rounded border ${getSignalStyle(entry.signal)}`}>
                      {entry.signal}
                    </span>
                  </div>
                  <div className="flex flex-wrap gap-1 mb-2">
                    {entry.modes.map(m => {
                      const mc = MODE_COLORS[m.mode]
                      return (
                        <span key={m.mode} className={`px-1.5 py-0.5 text-[10px] font-bold rounded border ${mc.bg} ${mc.text} ${mc.border}`}>
                          {mc.label} ({m.signal})
                        </span>
                      )
                    })}
                  </div>
                  <div className="grid grid-cols-5 gap-2 text-sm">
                    <div>
                      <div className="text-xs text-gray-500">Win%</div>
                      <div className={`font-medium ${entry.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}`}>{entry.win_rate?.toFixed(0)}%</div>
                    </div>
                    <div>
                      <div className="text-xs text-gray-500">R/R</div>
                      <div className={`font-medium ${entry.risk_reward >= 1 ? 'text-green-400' : 'text-red-400'}`}>{entry.risk_reward?.toFixed(2)}</div>
                    </div>
                    <div>
                      <div className="text-xs text-gray-500">Kurs</div>
                      <div className="text-white">{formatPrice(entry.current_price, entry.symbol)}</div>
                    </div>
                    <div>
                      <div className="text-xs text-gray-500">Rendite</div>
                      <div className="flex flex-col gap-0.5">
                        {entry.modes.map(m => {
                          const ret = formatModeReturn(m)
                          if (!ret) return null
                          const mc = MODE_COLORS[m.mode]
                          return (
                            <div key={m.mode} className="flex items-center gap-1 text-xs">
                              <span className={`${mc.text}`}>{mc.label.substring(0, 3)}</span>
                              <span className={`font-bold ${m.trade_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>{ret}</span>
                            </div>
                          )
                        })}
                        {entry.modes.every(m => m.trade_return_pct == null) && <span className="text-gray-500 text-xs">--</span>}
                      </div>
                    </div>
                    <div>
                      <div className="text-xs text-gray-500">Trades</div>
                      <div className="text-white">{entry.total_trades}</div>
                    </div>
                  </div>
                </div>
              ))}
            </div>

            {/* Desktop Table */}
            <div className="hidden md:block bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="text-left text-xs text-gray-500 border-b border-dark-600 bg-dark-900/50 whitespace-nowrap">
                      <th className="px-2 py-2 cursor-pointer hover:text-white transition-colors" onClick={() => handleSort('name')}>
                        Name <SortIcon field="name" />
                      </th>
                      <th className="px-2 py-2 cursor-pointer hover:text-white transition-colors" onClick={() => handleSort('signal')}>
                        Signal <SortIcon field="signal" />
                      </th>
                      <th className="px-2 py-2 cursor-pointer hover:text-white transition-colors" onClick={() => handleSort('mode_count')}>
                        Modi <SortIcon field="mode_count" />
                      </th>
                      <th className="px-2 py-2">Datum</th>
                      <th className="px-2 py-2 cursor-pointer hover:text-white transition-colors text-right" onClick={() => handleSort('current_price')}>
                        Kurs <SortIcon field="current_price" />
                      </th>
                      <th className="px-2 py-2 cursor-pointer hover:text-white transition-colors text-right" onClick={() => handleSort('trade_return_pct')}>
                        Rendite <SortIcon field="trade_return_pct" />
                      </th>
                      <th className="px-2 py-2 cursor-pointer hover:text-white transition-colors text-right" onClick={() => handleSort('win_rate')}>
                        Win% <SortIcon field="win_rate" />
                      </th>
                      <th className="px-2 py-2 cursor-pointer hover:text-white transition-colors text-right" onClick={() => handleSort('risk_reward')}>
                        R/R <SortIcon field="risk_reward" />
                      </th>
                      <th className="px-2 py-2 cursor-pointer hover:text-white transition-colors text-right" onClick={() => handleSort('total_return')}>
                        Gesamt <SortIcon field="total_return" />
                      </th>
                      <th className="px-2 py-2 cursor-pointer hover:text-white transition-colors text-right" onClick={() => handleSort('avg_return')}>
                        &Oslash;/T <SortIcon field="avg_return" />
                      </th>
                      <th className="px-2 py-2 cursor-pointer hover:text-white transition-colors text-right" onClick={() => handleSort('total_trades')}>
                        Trades <SortIcon field="total_trades" />
                      </th>
                      <th className="px-2 py-2 cursor-pointer hover:text-white transition-colors text-right" onClick={() => handleSort('market_cap')}>
                        MCap <SortIcon field="market_cap" />
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {sortedEntries.map((entry) => (
                      <tr
                        key={entry.symbol}
                        onClick={() => isAdmin && toggleVisibility(entry.symbol, entry.visible)}
                        className={`border-b border-dark-700/50 transition-colors ${isAdmin ? 'cursor-pointer' : ''} hover:bg-dark-700/30 ${!entry.visible ? 'opacity-50' : ''}`}
                      >
                        <td className={`px-2 py-1.5 ${!entry.visible ? 'line-through' : ''}`}>
                          <span className="font-medium text-white">{entry.name}</span>
                          <span className="text-gray-500 text-xs ml-1">({entry.symbol})</span>
                        </td>
                        <td className="px-2 py-1.5">
                          <span className={`px-1.5 py-0.5 text-xs font-bold rounded border ${getSignalStyle(entry.signal)}`}>
                            {entry.signal}
                          </span>
                        </td>
                        <td className="px-2 py-1.5">
                          <div className="flex flex-wrap gap-0.5">
                            {entry.modes.map(m => {
                              const mc = MODE_COLORS[m.mode]
                              return (
                                <span key={m.mode} className={`px-1 py-0.5 text-[10px] font-bold rounded border ${mc.bg} ${mc.text} ${mc.border}`} title={`${mc.label}: ${m.signal}`}>
                                  {mc.label.substring(0, 3)}
                                </span>
                              )
                            })}
                          </div>
                        </td>
                        <td className="px-2 py-1.5 text-xs text-gray-400 whitespace-nowrap">
                          {formatSignalDate(entry.signal_since)}
                        </td>
                        <td className="px-2 py-1.5 text-right text-white whitespace-nowrap">{formatPrice(entry.current_price, entry.symbol)}</td>
                        <td className="px-2 py-1.5 text-right">
                          <div className="flex flex-col items-end gap-0.5">
                            {entry.modes.map(m => {
                              const ret = formatModeReturn(m)
                              if (!ret) return null
                              const mc = MODE_COLORS[m.mode]
                              return (
                                <div key={m.mode} className="flex items-center gap-1 text-xs">
                                  <span className={`${mc.text} font-medium`}>{mc.label.substring(0, 3)}</span>
                                  <span className={`font-bold ${m.trade_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>{ret}</span>
                                </div>
                              )
                            })}
                            {entry.modes.every(m => m.trade_return_pct == null) && <span className="text-gray-500">--</span>}
                          </div>
                        </td>
                        <td className={`px-2 py-1.5 text-right font-medium ${entry.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                          {entry.win_rate?.toFixed(0)}%
                        </td>
                        <td className={`px-2 py-1.5 text-right font-medium ${entry.risk_reward >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                          {entry.risk_reward?.toFixed(2)}
                        </td>
                        <td className={`px-2 py-1.5 text-right font-bold ${entry.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {formatPercent(entry.total_return)}
                        </td>
                        <td className={`px-2 py-1.5 text-right font-medium ${(entry.avg_return || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {formatPercent(entry.avg_return)}
                        </td>
                        <td className="px-2 py-1.5 text-right text-white">{entry.total_trades}</td>
                        <td className="px-2 py-1.5 text-right text-gray-300 whitespace-nowrap">{formatMarketCap(entry.market_cap)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  )
}

export default SignalList
