import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useCurrency } from '../context/CurrencyContext'
import { useTradingMode } from '../context/TradingModeContext'

function Performance({ token }) {
  const navigate = useNavigate()
  const { mode, isDefensive, isAggressive, isQuant } = useTradingMode()
  const [trades, setTrades] = useState([])
  const [loading, setLoading] = useState(true)
  const [itemsPerPage, setItemsPerPage] = useState(10)
  const [currentPage, setCurrentPage] = useState(1)
  const [currentSort, setCurrentSort] = useState({ field: 'entry_date', dir: 'desc' })
  const [timeRange, setTimeRange] = useState('1y') // 1m, 1y, 2y, all
  const { formatPrice } = useCurrency()

  // Filter state
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [filters, setFilters] = useState({
    minWinrate: '',
    maxWinrate: '',
    minRR: '',
    maxRR: '',
    minAvgReturn: '',
    maxAvgReturn: '',
    minMarketCap: ''
  })

  // Redirect to home if not logged in
  useEffect(() => {
    if (!token) {
      navigate('/')
    }
  }, [token, navigate])

  // Calculate items per page based on screen height
  useEffect(() => {
    const calculateItemsPerPage = () => {
      const headerHeight = 220
      const rowHeight = 56
      const paginatorHeight = 60
      const availableHeight = window.innerHeight - headerHeight - paginatorHeight
      const items = Math.floor(availableHeight / rowHeight)
      setItemsPerPage(Math.max(5, items))
    }

    calculateItemsPerPage()
    window.addEventListener('resize', calculateItemsPerPage)
    return () => window.removeEventListener('resize', calculateItemsPerPage)
  }, [])

  useEffect(() => {
    fetchTrades()
  }, [token])

  const fetchTrades = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/performance/history', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      setTrades(Array.isArray(data) ? data : [])
    } catch (err) {
      console.error('Failed to fetch performance data:', err)
    } finally {
      setLoading(false)
    }
  }

  // Calculate cutoff date based on time range
  const getCutoffDate = () => {
    const now = Math.floor(Date.now() / 1000) // Current time in seconds
    switch (timeRange) {
      case '1m': return now - (30 * 24 * 60 * 60)
      case '1y': return now - (365 * 24 * 60 * 60)
      case '2y': return now - (2 * 365 * 24 * 60 * 60)
      case 'all': return 0
      default: return now - (365 * 24 * 60 * 60)
    }
  }
  const cutoffDate = getCutoffDate()

  // Get current mode string
  const currentMode = isQuant ? 'quant' : isAggressive ? 'aggressive' : 'defensive'

  // Filter trades by current mode, time range, and custom filters
  const applyFilters = (tradeList) => {
    return tradeList.filter(t => {
      // Filter by stock-level metrics (from backend)
      if (filters.minWinrate && t.win_rate < parseFloat(filters.minWinrate)) return false
      if (filters.maxWinrate && t.win_rate > parseFloat(filters.maxWinrate)) return false
      if (filters.minRR && t.risk_reward < parseFloat(filters.minRR)) return false
      if (filters.maxRR && t.risk_reward > parseFloat(filters.maxRR)) return false
      if (filters.minAvgReturn && t.avg_return < parseFloat(filters.minAvgReturn)) return false
      if (filters.maxAvgReturn && t.avg_return > parseFloat(filters.maxAvgReturn)) return false
      if (filters.minMarketCap && t.market_cap < parseFloat(filters.minMarketCap) * 1e9) return false
      return true
    })
  }

  // Filter trades by mode and time range first
  const modeFilteredTrades = trades.filter(t => t.mode === currentMode && t.entry_date >= cutoffDate)
  // Then apply custom filters
  const filteredTrades = applyFilters(modeFilteredTrades)

  // Calculate statistics
  const calcStats = (tradeList) => {
    const closed = tradeList.filter(t => t.status === 'CLOSED')
    const wins = closed.filter(t => t.return_pct > 0)
    const losses = closed.filter(t => t.return_pct <= 0)
    const winRate = closed.length > 0 ? (wins.length / closed.length) * 100 : 0

    const avgWin = wins.length > 0 ? wins.reduce((sum, t) => sum + t.return_pct, 0) / wins.length : 0
    const avgLoss = losses.length > 0 ? Math.abs(losses.reduce((sum, t) => sum + t.return_pct, 0) / losses.length) : 1
    const riskReward = avgLoss > 0 ? avgWin / avgLoss : 0

    const totalReturn = tradeList.reduce((sum, t) => sum + (t.return_pct || 0), 0)
    const avgReturn = tradeList.length > 0 ? totalReturn / tradeList.length : 0

    return { winRate, riskReward, totalReturn, avgReturn, tradeCount: tradeList.length, wins: wins.length, losses: losses.length }
  }

  const stats = calcStats(filteredTrades)

  // Sort function
  const sortTrades = (tradeList, sort) => {
    return [...tradeList].sort((a, b) => {
      let aVal = a[sort.field]
      let bVal = b[sort.field]
      if (sort.dir === 'asc') {
        return aVal - bVal
      }
      return bVal - aVal
    })
  }

  const sortedTrades = sortTrades(filteredTrades, currentSort)

  // Pagination
  const totalPages = Math.ceil(sortedTrades.length / itemsPerPage) || 1

  const paginatedTrades = sortedTrades.slice(
    (currentPage - 1) * itemsPerPage,
    currentPage * itemsPerPage
  )

  // Reset page when mode or filters change
  useEffect(() => {
    setCurrentPage(1)
  }, [mode, filters, timeRange])

  // Get mode display info
  const getModeInfo = () => {
    if (isQuant) return { title: 'Quant', color: 'from-violet-500/20 to-transparent' }
    if (isAggressive) return { title: 'Aggressiv', color: 'from-orange-500/20 to-transparent' }
    return { title: 'Defensiv', color: 'from-blue-500/20 to-transparent' }
  }
  const modeInfo = getModeInfo()

  const handleFilterChange = (field, value) => {
    setFilters(prev => ({ ...prev, [field]: value }))
  }

  const clearFilters = () => {
    setFilters({
      minWinrate: '',
      maxWinrate: '',
      minRR: '',
      maxRR: '',
      minAvgReturn: '',
      maxAvgReturn: '',
      minMarketCap: ''
    })
  }

  const hasActiveFilters = Object.values(filters).some(v => v !== '')

  const formatDate = (timestamp) => {
    if (!timestamp) return '-'
    // Timestamp is in seconds, convert to milliseconds for JavaScript Date
    return new Date(timestamp * 1000).toLocaleDateString('de-DE', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric'
    })
  }

  const formatPercent = (value) => {
    if (value === undefined || value === null) return '-'
    return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
  }

  const SortButton = ({ field, children }) => {
    const isActive = currentSort.field === field
    const handleClick = () => {
      if (isActive) {
        setCurrentSort({ field, dir: currentSort.dir === 'asc' ? 'desc' : 'asc' })
      } else {
        setCurrentSort({ field, dir: 'desc' })
      }
    }
    return (
      <button onClick={handleClick} className="flex items-center gap-1 hover:text-white transition-colors">
        {children}
        {isActive && (
          <span className="text-accent-400">{currentSort.dir === 'asc' ? '↑' : '↓'}</span>
        )}
      </button>
    )
  }

  const TradeTable = ({ data, page, setPage, totalPages, title, color, stats, paginatedData }) => (
    <div className="flex-1 min-w-0">
      <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden h-full flex flex-col">
        <div className={`p-4 border-b border-dark-600 bg-gradient-to-r ${color}`}>
          <h2 className="text-lg font-semibold text-white">{title}</h2>
          <div className="flex flex-wrap gap-4 mt-2 text-sm">
            <div>
              <span className="text-gray-400">Trades: </span>
              <span className="text-white font-medium">{data.length}</span>
            </div>
            <div>
              <span className="text-gray-400">Winrate: </span>
              <span className={`font-medium ${stats.winRate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                {stats.winRate.toFixed(1)}%
              </span>
              <span className="text-gray-500 text-xs ml-1">({stats.wins}W/{stats.losses}L)</span>
            </div>
            <div>
              <span className="text-gray-400">R/R: </span>
              <span className={`font-medium ${stats.riskReward >= 1 ? 'text-green-400' : 'text-orange-400'}`}>
                {stats.riskReward.toFixed(2)}
              </span>
            </div>
            <div>
              <span className="text-gray-400">Kumuliert: </span>
              <span className={`font-medium ${stats.totalReturn >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                {stats.totalReturn >= 0 ? '+' : ''}{stats.totalReturn.toFixed(1)}%
              </span>
            </div>
            <div>
              <span className="text-gray-400">Ø/Trade: </span>
              <span className={`font-medium ${stats.avgReturn >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                {stats.avgReturn >= 0 ? '+' : ''}{stats.avgReturn.toFixed(2)}%
              </span>
            </div>
          </div>
        </div>

        {loading ? (
          <div className="flex-1 flex items-center justify-center p-8">
            <div className="animate-spin w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full"></div>
          </div>
        ) : data.length === 0 ? (
          <div className="flex-1 flex items-center justify-center p-8 text-gray-500">
            Keine Trades vorhanden
          </div>
        ) : (
          <>
            {/* Desktop Table */}
            <div className="hidden md:block flex-1 overflow-auto">
              <table className="w-full">
                <thead className="bg-dark-700 sticky top-0">
                  <tr>
                    <th className="px-3 py-3 text-left text-xs font-medium text-gray-400 uppercase">Symbol</th>
                    <th className="px-3 py-3 text-center text-xs font-medium text-gray-400 uppercase">
                      <SortButton field="entry_date">BUY</SortButton>
                    </th>
                    <th className="px-3 py-3 text-right text-xs font-medium text-gray-400 uppercase">Einstieg</th>
                    <th className="px-3 py-3 text-center text-xs font-medium text-gray-400 uppercase">SELL / OPEN</th>
                    <th className="px-3 py-3 text-right text-xs font-medium text-gray-400 uppercase">Ausstieg</th>
                    <th className="px-3 py-3 text-right text-xs font-medium text-gray-400 uppercase">
                      <SortButton field="return_pct">Rendite</SortButton>
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-dark-700">
                  {paginatedData.map((trade) => (
                    <tr key={`${trade.mode}-${trade.id}`} className="hover:bg-dark-700/50 transition-colors">
                      <td className="px-3 py-3">
                        <div className="font-medium text-white">{trade.symbol}</div>
                        <div className="text-xs text-gray-500 truncate max-w-[100px]">{trade.name}</div>
                      </td>
                      <td className="px-3 py-3 text-center">
                        <span className="px-2 py-1 text-xs rounded bg-green-500/20 text-green-400 font-medium">BUY</span>
                        <div className="text-xs text-gray-400 mt-1">{formatDate(trade.entry_date)}</div>
                      </td>
                      <td className="px-3 py-3 text-right text-sm text-gray-300">{formatPrice(trade.entry_price)}</td>
                      <td className="px-3 py-3 text-center">
                        {trade.status === 'OPEN' ? (
                          <span className="px-2 py-1 text-xs rounded bg-blue-500/20 text-blue-400 font-medium">OPEN</span>
                        ) : (
                          <>
                            <span className="px-2 py-1 text-xs rounded bg-red-500/20 text-red-400 font-medium">SELL</span>
                            <div className="text-xs text-gray-400 mt-1">{formatDate(trade.exit_date)}</div>
                          </>
                        )}
                      </td>
                      <td className="px-3 py-3 text-right text-sm text-gray-300">
                        {trade.status === 'OPEN' ? formatPrice(trade.current_price) : formatPrice(trade.exit_price)}
                      </td>
                      <td className={`px-3 py-3 text-right text-sm font-bold ${
                        trade.return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                      }`}>
                        {formatPercent(trade.return_pct)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Mobile Cards */}
            <div className="md:hidden flex-1 overflow-auto p-3 space-y-2">
              {paginatedData.map((trade) => (
                <div
                  key={`${trade.mode}-${trade.id}`}
                  className={`p-3 rounded-lg border ${
                    trade.return_pct >= 0
                      ? 'bg-green-500/5 border-green-500/20'
                      : 'bg-red-500/5 border-red-500/20'
                  }`}
                >
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-white">{trade.symbol}</span>
                      <span className={`text-sm font-bold ${
                        trade.return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                      }`}>
                        {formatPercent(trade.return_pct)}
                      </span>
                    </div>
                  </div>
                  <div className="flex items-center gap-3 text-xs mb-1">
                    <span className="px-1.5 py-0.5 rounded bg-green-500/20 text-green-400 font-medium">BUY</span>
                    <span className="text-gray-400">{formatDate(trade.entry_date)}</span>
                    <span className="text-gray-300">{formatPrice(trade.entry_price)}</span>
                  </div>
                  <div className="flex items-center gap-3 text-xs">
                    {trade.status === 'OPEN' ? (
                      <>
                        <span className="px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-400 font-medium">OPEN</span>
                        <span className="text-gray-400">aktuell</span>
                        <span className="text-gray-300">{formatPrice(trade.current_price)}</span>
                      </>
                    ) : (
                      <>
                        <span className="px-1.5 py-0.5 rounded bg-red-500/20 text-red-400 font-medium">SELL</span>
                        <span className="text-gray-400">{formatDate(trade.exit_date)}</span>
                        <span className="text-gray-300">{formatPrice(trade.exit_price)}</span>
                      </>
                    )}
                  </div>
                </div>
              ))}
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="p-3 border-t border-dark-600 flex items-center justify-between bg-dark-800">
                <button
                  onClick={() => setPage(p => Math.max(1, p - 1))}
                  disabled={page === 1}
                  className="px-3 py-1.5 text-sm bg-dark-700 text-gray-300 rounded-lg hover:bg-dark-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  Zurück
                </button>
                <span className="text-sm text-gray-400">
                  Seite {page} von {totalPages}
                </span>
                <button
                  onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                  disabled={page === totalPages}
                  className="px-3 py-1.5 text-sm bg-dark-700 text-gray-300 rounded-lg hover:bg-dark-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  Weiter
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )

  const timeRangeOptions = [
    { value: '1m', label: '1 Monat' },
    { value: '1y', label: '1 Jahr' },
    { value: '2y', label: '2 Jahre' },
    { value: 'all', label: 'Alle' },
  ]

  return (
    <div className="p-4 md:p-6 h-full flex flex-col max-w-4xl mx-auto w-full">
      <div className="mb-4 text-center">
        <h1 className="text-2xl font-bold text-white">Performance</h1>
        <p className="text-gray-400 mt-1">Trade-Historie der Watchlist - {modeInfo.title} Modus</p>
      </div>

      {/* Time Range Selection */}
      <div className="flex justify-center mb-4">
        <div className="inline-flex bg-dark-800 rounded-lg p-1 border border-dark-600">
          {timeRangeOptions.map((option) => (
            <button
              key={option.value}
              onClick={() => setTimeRange(option.value)}
              className={`px-4 py-2 text-sm font-medium rounded-md transition-all ${
                timeRange === option.value
                  ? 'bg-accent-500 text-white'
                  : 'text-gray-400 hover:text-white hover:bg-dark-700'
              }`}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>

      {/* Collapsible Filter Panel */}
      <div className="mb-4 bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
        <button
          onClick={() => setFiltersOpen(!filtersOpen)}
          className="w-full px-4 py-3 flex items-center justify-between text-left hover:bg-dark-700/50 transition-colors"
        >
          <div className="flex items-center gap-2">
            <svg className="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
            </svg>
            <span className="text-white font-medium">Filter</span>
            {hasActiveFilters && (
              <span className="px-2 py-0.5 text-xs bg-accent-500 text-white rounded-full">Aktiv</span>
            )}
          </div>
          <svg className={`w-5 h-5 text-gray-400 transition-transform ${filtersOpen ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </button>

        {filtersOpen && (
          <div className="px-4 pb-4 border-t border-dark-600">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mt-4">
              {/* Winrate Filter */}
              <div>
                <label className="block text-xs text-gray-400 mb-1">Winrate (%)</label>
                <div className="flex gap-2">
                  <input
                    type="number"
                    placeholder="Min"
                    value={filters.minWinrate}
                    onChange={(e) => handleFilterChange('minWinrate', e.target.value)}
                    className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500"
                  />
                  <input
                    type="number"
                    placeholder="Max"
                    value={filters.maxWinrate}
                    onChange={(e) => handleFilterChange('maxWinrate', e.target.value)}
                    className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500"
                  />
                </div>
              </div>

              {/* Risk/Reward Filter */}
              <div>
                <label className="block text-xs text-gray-400 mb-1">Risk/Reward</label>
                <div className="flex gap-2">
                  <input
                    type="number"
                    step="0.1"
                    placeholder="Min"
                    value={filters.minRR}
                    onChange={(e) => handleFilterChange('minRR', e.target.value)}
                    className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500"
                  />
                  <input
                    type="number"
                    step="0.1"
                    placeholder="Max"
                    value={filters.maxRR}
                    onChange={(e) => handleFilterChange('maxRR', e.target.value)}
                    className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500"
                  />
                </div>
              </div>

              {/* Avg Return Filter */}
              <div>
                <label className="block text-xs text-gray-400 mb-1">Ø Rendite (%)</label>
                <div className="flex gap-2">
                  <input
                    type="number"
                    step="0.1"
                    placeholder="Min"
                    value={filters.minAvgReturn}
                    onChange={(e) => handleFilterChange('minAvgReturn', e.target.value)}
                    className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500"
                  />
                  <input
                    type="number"
                    step="0.1"
                    placeholder="Max"
                    value={filters.maxAvgReturn}
                    onChange={(e) => handleFilterChange('maxAvgReturn', e.target.value)}
                    className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500"
                  />
                </div>
              </div>

              {/* Market Cap Filter */}
              <div>
                <label className="block text-xs text-gray-400 mb-1">Min Market Cap (Mrd)</label>
                <input
                  type="number"
                  step="0.1"
                  placeholder="z.B. 10"
                  value={filters.minMarketCap}
                  onChange={(e) => handleFilterChange('minMarketCap', e.target.value)}
                  className="w-full px-2 py-1.5 text-sm bg-dark-700 border border-dark-600 rounded text-white placeholder-gray-500"
                />
              </div>
            </div>

            {hasActiveFilters && (
              <div className="mt-3 flex justify-end">
                <button
                  onClick={clearFilters}
                  className="px-3 py-1.5 text-sm text-gray-400 hover:text-white transition-colors"
                >
                  Filter zurücksetzen
                </button>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Single Mode Table */}
      <div className="flex-1 min-h-0">
        <TradeTable
          data={sortedTrades}
          paginatedData={paginatedTrades}
          page={currentPage}
          setPage={setCurrentPage}
          totalPages={totalPages}
          title={modeInfo.title}
          color={modeInfo.color}
          stats={stats}
        />
      </div>
    </div>
  )
}

export default Performance
