import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useCurrency } from '../context/CurrencyContext'

function Performance({ token }) {
  const navigate = useNavigate()
  const [trades, setTrades] = useState([])
  const [loading, setLoading] = useState(true)
  const [itemsPerPage, setItemsPerPage] = useState(10)
  const [defensivePage, setDefensivePage] = useState(1)
  const [aggressivePage, setAggressivePage] = useState(1)
  const [defensiveSort, setDefensiveSort] = useState({ field: 'entry_date', dir: 'desc' })
  const [aggressiveSort, setAggressiveSort] = useState({ field: 'entry_date', dir: 'desc' })
  const [timeRange, setTimeRange] = useState('1y') // 1m, 1y, 2y, all
  const { formatPrice } = useCurrency()

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

  // Filter trades by mode and time range
  const defensiveTrades = trades.filter(t => t.mode === 'defensive' && t.entry_date >= cutoffDate)
  const aggressiveTrades = trades.filter(t => t.mode === 'aggressive' && t.entry_date >= cutoffDate)

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

  const defensiveStats = calcStats(defensiveTrades)
  const aggressiveStats = calcStats(aggressiveTrades)

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

  const sortedDefensive = sortTrades(defensiveTrades, defensiveSort)
  const sortedAggressive = sortTrades(aggressiveTrades, aggressiveSort)

  // Pagination
  const defensivePages = Math.ceil(sortedDefensive.length / itemsPerPage) || 1
  const aggressivePages = Math.ceil(sortedAggressive.length / itemsPerPage) || 1

  const paginatedDefensive = sortedDefensive.slice(
    (defensivePage - 1) * itemsPerPage,
    defensivePage * itemsPerPage
  )
  const paginatedAggressive = sortedAggressive.slice(
    (aggressivePage - 1) * itemsPerPage,
    aggressivePage * itemsPerPage
  )

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

  const SortButton = ({ field, sort, setSort, children }) => {
    const isActive = sort.field === field
    const handleClick = () => {
      if (isActive) {
        setSort({ field, dir: sort.dir === 'asc' ? 'desc' : 'asc' })
      } else {
        setSort({ field, dir: 'desc' })
      }
    }
    return (
      <button onClick={handleClick} className="flex items-center gap-1 hover:text-white transition-colors">
        {children}
        {isActive && (
          <span className="text-accent-400">{sort.dir === 'asc' ? '↑' : '↓'}</span>
        )}
      </button>
    )
  }

  const TradeTable = ({ data, page, setPage, totalPages, title, color, stats, sort, setSort, paginatedData }) => (
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
                      <SortButton field="entry_date" sort={sort} setSort={setSort}>BUY</SortButton>
                    </th>
                    <th className="px-3 py-3 text-right text-xs font-medium text-gray-400 uppercase">Einstieg</th>
                    <th className="px-3 py-3 text-center text-xs font-medium text-gray-400 uppercase">SELL / OPEN</th>
                    <th className="px-3 py-3 text-right text-xs font-medium text-gray-400 uppercase">Ausstieg</th>
                    <th className="px-3 py-3 text-right text-xs font-medium text-gray-400 uppercase">
                      <SortButton field="return_pct" sort={sort} setSort={setSort}>Rendite</SortButton>
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
    <div className="p-4 md:p-6 h-full flex flex-col max-w-7xl mx-auto w-full">
      <div className="mb-4 text-center">
        <h1 className="text-2xl font-bold text-white">Performance</h1>
        <p className="text-gray-400 mt-1">Trade-Historie der Watchlist</p>
      </div>

      {/* Time Range Selection */}
      <div className="flex justify-center mb-4">
        <div className="inline-flex bg-dark-800 rounded-lg p-1 border border-dark-600">
          {timeRangeOptions.map((option) => (
            <button
              key={option.value}
              onClick={() => {
                setTimeRange(option.value)
                setDefensivePage(1)
                setAggressivePage(1)
              }}
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

      {/* Two Column Layout */}
      <div className="flex-1 flex flex-col md:flex-row gap-4 min-h-0">
        <TradeTable
          data={sortedDefensive}
          paginatedData={paginatedDefensive}
          page={defensivePage}
          setPage={setDefensivePage}
          totalPages={defensivePages}
          title="Defensiv"
          color="from-blue-500/20 to-transparent"
          stats={defensiveStats}
          sort={defensiveSort}
          setSort={setDefensiveSort}
        />
        <TradeTable
          data={sortedAggressive}
          paginatedData={paginatedAggressive}
          page={aggressivePage}
          setPage={setAggressivePage}
          totalPages={aggressivePages}
          title="Aggressiv"
          color="from-orange-500/20 to-transparent"
          stats={aggressiveStats}
          sort={aggressiveSort}
          setSort={setAggressiveSort}
        />
      </div>
    </div>
  )
}

export default Performance
