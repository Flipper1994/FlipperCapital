import { useState, useEffect, useMemo } from 'react'
import { formatPrice } from '../utils/currency'
import { useTradingMode } from '../context/TradingModeContext'

// Generate month options from current month going back
function generateMonthOptions() {
  const options = []
  const now = new Date()
  for (let i = 0; i < 24; i++) {
    const date = new Date(now.getFullYear(), now.getMonth() - i, 1)
    options.push({
      value: `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`,
      label: date.toLocaleDateString('de-DE', { month: 'long', year: 'numeric' })
    })
  }
  return options
}

// Calculate signal for a specific month based on trades
function calculateSignalForMonth(trades, targetYear, targetMonth, isAggressive) {
  if (!trades || trades.length === 0) return { signal: 'WAIT', bars: 0 }

  const targetDate = new Date(targetYear, targetMonth, 1)
  const nextMonth = new Date(targetYear, targetMonth + 1, 1)

  // Find trades that affect this month
  let hasOpenPosition = false
  let recentBuy = false
  let recentSell = false

  for (const trade of trades) {
    const entryDate = trade.entryDate ? new Date(trade.entryDate * 1000) : null
    const exitDate = trade.exitDate ? new Date(trade.exitDate * 1000) : null

    // Check if position was open during this month
    if (entryDate && entryDate < nextMonth) {
      if (!exitDate || exitDate >= targetDate) {
        hasOpenPosition = true
      }
      // Check for recent buy (this month or last month)
      const entryMonth = entryDate.getMonth()
      const entryYear = entryDate.getFullYear()
      if ((entryMonth === targetMonth && entryYear === targetYear) ||
          (entryMonth === (targetMonth === 0 ? 11 : targetMonth - 1) &&
           entryYear === (targetMonth === 0 ? targetYear - 1 : targetYear))) {
        if (!exitDate || exitDate >= targetDate) recentBuy = true
      }
    }

    // Check for recent sell
    if (exitDate) {
      const exitMonth = exitDate.getMonth()
      const exitYear = exitDate.getFullYear()
      if ((exitMonth === targetMonth && exitYear === targetYear) ||
          (exitMonth === (targetMonth === 0 ? 11 : targetMonth - 1) &&
           exitYear === (targetMonth === 0 ? targetYear - 1 : targetYear))) {
        recentSell = true
      }
    }
  }

  if (recentBuy && hasOpenPosition) return { signal: 'BUY', bars: 1 }
  if (recentSell && !hasOpenPosition) return { signal: 'SELL', bars: 1 }
  if (hasOpenPosition) return { signal: 'HOLD', bars: 1 }
  return { signal: 'WAIT', bars: 1 }
}

function StockTracker() {
  const [stocks, setStocks] = useState([])
  const [loading, setLoading] = useState(true)
  const [sortField, setSortField] = useState('updated_at')
  const [sortDir, setSortDir] = useState('desc')
  const [selectedStock, setSelectedStock] = useState(null)
  const [signalFilter, setSignalFilter] = useState(null)
  const [selectedMonth, setSelectedMonth] = useState(() => {
    const now = new Date()
    return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
  })
  const [, forceUpdate] = useState(0)
  const { mode, isAggressive } = useTradingMode()

  const monthOptions = useMemo(() => generateMonthOptions(), [])

  useEffect(() => {
    const handleCurrencyChange = () => forceUpdate(n => n + 1)
    window.addEventListener('currencyChanged', handleCurrencyChange)
    return () => window.removeEventListener('currencyChanged', handleCurrencyChange)
  }, [])

  useEffect(() => {
    fetchStocks()
  }, [isAggressive])

  const fetchStocks = async () => {
    setLoading(true)
    try {
      const endpoint = isAggressive ? '/api/performance/aggressive' : '/api/performance'
      const res = await fetch(endpoint)
      const data = await res.json()
      setStocks(data || [])
    } catch (err) {
      console.error('Failed to fetch tracked stocks:', err)
      setStocks([])
    } finally {
      setLoading(false)
    }
  }

  // Calculate signals for selected month and previous month
  const stocksWithMonthSignals = useMemo(() => {
    const [year, month] = selectedMonth.split('-').map(Number)
    const prevMonth = month === 1 ? 12 : month - 1
    const prevYear = month === 1 ? year - 1 : year

    return stocks.map(stock => {
      const trades = stock.trades || []
      const currentSignal = calculateSignalForMonth(trades, year, month - 1, isAggressive)
      const prevSignal = calculateSignalForMonth(trades, prevYear, prevMonth - 1, isAggressive)

      const signalChanged = currentSignal.signal !== prevSignal.signal

      return {
        ...stock,
        monthSignal: currentSignal.signal,
        prevMonthSignal: prevSignal.signal,
        signalChanged,
        displaySignal: signalChanged
          ? `${prevSignal.signal} → ${currentSignal.signal}`
          : currentSignal.signal
      }
    })
  }, [stocks, selectedMonth, isAggressive])

  // Get all signal changes for the panel
  const signalChanges = useMemo(() => {
    return stocksWithMonthSignals
      .filter(s => s.signalChanged)
      .map(s => {
        // If it was a SELL signal, find the related trade
        let tradeInfo = null
        if (s.monthSignal === 'SELL' && s.trades) {
          const [year, month] = selectedMonth.split('-').map(Number)
          const trade = s.trades.find(t => {
            if (!t.exitDate) return false
            const exitDate = new Date(t.exitDate * 1000)
            return exitDate.getMonth() === month - 1 && exitDate.getFullYear() === year
          })
          if (trade) {
            tradeInfo = {
              buyPrice: trade.entryPrice,
              sellPrice: trade.exitPrice,
              returnPct: trade.returnPct
            }
          }
        }
        return { ...s, tradeInfo }
      })
  }, [stocksWithMonthSignals, selectedMonth])

  const handleSort = (field) => {
    if (sortField === field) {
      setSortDir(sortDir === 'asc' ? 'desc' : 'asc')
    } else {
      setSortField(field)
      setSortDir('desc')
    }
  }

  // Filter stocks by signal if filter is active
  const filteredStocks = signalFilter
    ? stocksWithMonthSignals.filter(s => {
        if (signalFilter === 'SELL_WAIT') {
          return s.monthSignal === 'SELL' || s.monthSignal === 'WAIT'
        }
        return s.monthSignal === signalFilter
      })
    : stocksWithMonthSignals

  const sortedStocks = [...filteredStocks].sort((a, b) => {
    let aVal = a[sortField]
    let bVal = b[sortField]

    if (sortField === 'updated_at') {
      aVal = new Date(aVal).getTime()
      bVal = new Date(bVal).getTime()
    }

    if (sortDir === 'asc') {
      return aVal > bVal ? 1 : -1
    }
    return aVal < bVal ? 1 : -1
  })

  const selectedMonthLabel = monthOptions.find(m => m.value === selectedMonth)?.label || selectedMonth

  const toggleFilter = (filter) => {
    setSignalFilter(signalFilter === filter ? null : filter)
  }

  const getSignalStyle = (signal) => {
    switch (signal) {
      case 'BUY':
        return 'bg-green-500/20 text-green-400 border-green-500/30'
      case 'HOLD':
        return 'bg-blue-500/20 text-blue-400 border-blue-500/30'
      case 'SELL':
        return 'bg-red-500/20 text-red-400 border-red-500/30'
      case 'WAIT':
        return 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30'
      default:
        return 'bg-gray-500/20 text-gray-400 border-gray-500/30'
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

  const formatTradeDate = (timestamp) => {
    if (!timestamp) return '-'
    const date = new Date(timestamp * 1000)
    return date.toLocaleDateString('de-DE', { day: '2-digit', month: 'short', year: '2-digit' })
  }

  const formatPercent = (value) => {
    if (value === undefined || value === null || isNaN(value)) return '--'
    const sign = value >= 0 ? '+' : ''
    return `${sign}${value.toFixed(1)}%`
  }

  const SortIcon = ({ field }) => {
    if (sortField !== field) {
      return (
        <svg className="w-3 h-3 inline ml-1 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4" />
        </svg>
      )
    }
    return (
      <svg className="w-3 h-3 inline ml-1 text-accent-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        {sortDir === 'asc' ? (
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
        ) : (
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        )}
      </svg>
    )
  }

  return (
    <div className="flex-1 p-4 md:p-6 overflow-auto">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-4 md:mb-6">
          <div className="flex items-center justify-between flex-wrap gap-3">
            <div className="flex items-center gap-3 flex-wrap">
              <h1 className="text-xl md:text-2xl font-bold text-white">Aktien Tracker</h1>
              <span className="text-accent-400 font-medium">- {selectedMonthLabel}</span>
              <span className={`px-2 py-1 text-xs font-bold rounded flex items-center gap-1.5 ${
                isAggressive
                  ? 'bg-orange-500/20 text-orange-400 border border-orange-500/30'
                  : 'bg-blue-500/20 text-blue-400 border border-blue-500/30'
              }`}>
                {isAggressive ? (
                  <>
                    <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17.657 18.657A8 8 0 016.343 7.343S7 9 9 10c0-2 .5-5 2.986-7C14 5 16.09 5.777 17.656 7.343A7.975 7.975 0 0120 13a7.975 7.975 0 01-2.343 5.657z" />
                    </svg>
                    Aggressiv
                  </>
                ) : (
                  <>
                    <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
                    </svg>
                    Defensiv
                  </>
                )}
              </span>
            </div>

            {/* Month Selector */}
            <select
              value={selectedMonth}
              onChange={(e) => setSelectedMonth(e.target.value)}
              className="px-3 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white text-sm focus:outline-none focus:border-accent-500"
            >
              {monthOptions.map(opt => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </select>
          </div>
          <p className="text-gray-500 text-sm mt-1">
            BX Trender Signale für {selectedMonthLabel}
            {isAggressive && <span className="text-orange-400"> (Aggressive Signale)</span>}
          </p>
        </div>

        {loading ? (
          <div className="text-center py-12">
            <div className="w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full animate-spin mx-auto"></div>
            <p className="text-gray-500 mt-4">Lade Aktien...</p>
          </div>
        ) : stocks.length === 0 ? (
          <div className="bg-dark-800 rounded-xl border border-dark-600 p-12 text-center">
            <div className="w-20 h-20 mx-auto mb-6 bg-dark-700 rounded-full flex items-center justify-center">
              <svg className="w-10 h-10 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              </svg>
            </div>
            <h2 className="text-xl font-semibold text-gray-400 mb-3">Noch keine Aktien getrackt</h2>
            <p className="text-gray-600 max-w-md mx-auto">
              Sobald du Aktien aus der Watchlist aufrufst, werden ihre Performance-Daten hier angezeigt.
            </p>
          </div>
        ) : (
          <>
            {/* Stats Overview - Clickable for filtering */}
            <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mb-4 md:mb-6">
              <div
                onClick={() => setSignalFilter(null)}
                className={`bg-dark-800 rounded-xl border p-4 cursor-pointer transition-all ${
                  signalFilter === null ? 'border-accent-500 ring-1 ring-accent-500' : 'border-dark-600 hover:border-gray-500'
                }`}
              >
                <div className="text-xs text-gray-500 mb-1">Getrackte Aktien</div>
                <div className="text-2xl font-bold text-white">{stocksWithMonthSignals.length}</div>
              </div>
              <div
                onClick={() => toggleFilter('BUY')}
                className={`bg-dark-800 rounded-xl border p-4 cursor-pointer transition-all ${
                  signalFilter === 'BUY' ? 'border-green-500 ring-1 ring-green-500' : 'border-dark-600 hover:border-green-500/50'
                }`}
              >
                <div className="text-xs text-gray-500 mb-1">BUY Signale</div>
                <div className="text-2xl font-bold text-green-400">
                  {stocksWithMonthSignals.filter(s => s.monthSignal === 'BUY').length}
                </div>
              </div>
              <div
                onClick={() => toggleFilter('HOLD')}
                className={`bg-dark-800 rounded-xl border p-4 cursor-pointer transition-all ${
                  signalFilter === 'HOLD' ? 'border-blue-500 ring-1 ring-blue-500' : 'border-dark-600 hover:border-blue-500/50'
                }`}
              >
                <div className="text-xs text-gray-500 mb-1">HOLD Signale</div>
                <div className="text-2xl font-bold text-blue-400">
                  {stocksWithMonthSignals.filter(s => s.monthSignal === 'HOLD').length}
                </div>
              </div>
              <div
                onClick={() => toggleFilter('SELL_WAIT')}
                className={`bg-dark-800 rounded-xl border p-4 cursor-pointer transition-all ${
                  signalFilter === 'SELL_WAIT' ? 'border-red-500 ring-1 ring-red-500' : 'border-dark-600 hover:border-red-500/50'
                }`}
              >
                <div className="text-xs text-gray-500 mb-1">SELL/WAIT</div>
                <div className="text-2xl font-bold text-red-400">
                  {stocksWithMonthSignals.filter(s => s.monthSignal === 'SELL' || s.monthSignal === 'WAIT').length}
                </div>
              </div>
              <div className="bg-dark-800 rounded-xl border border-purple-500/30 p-4">
                <div className="text-xs text-gray-500 mb-1">Änderungen</div>
                <div className="text-2xl font-bold text-purple-400">
                  {signalChanges.length}
                </div>
              </div>
            </div>

            {/* Active Filter Indicator */}
            {signalFilter && (
              <div className="mb-4 flex items-center gap-2 p-3 bg-dark-800 rounded-lg border border-dark-600">
                <span className="text-gray-400 text-sm">Filter aktiv:</span>
                <span className={`px-2 py-1 text-xs font-bold rounded ${
                  signalFilter === 'BUY' ? 'bg-green-500/20 text-green-400' :
                  signalFilter === 'HOLD' ? 'bg-blue-500/20 text-blue-400' :
                  'bg-red-500/20 text-red-400'
                }`}>
                  {signalFilter === 'SELL_WAIT' ? 'SELL / WAIT' : signalFilter}
                </span>
                <span className="text-gray-500 text-sm">({filteredStocks.length} Aktien)</span>
                <button
                  onClick={() => setSignalFilter(null)}
                  className="ml-auto text-gray-400 hover:text-white transition-colors flex items-center gap-1 text-sm"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                  Filter entfernen
                </button>
              </div>
            )}

            {/* Mobile Card View */}
            <div className="md:hidden space-y-3">
              {sortedStocks.map((stock) => (
                <div
                  key={stock.id}
                  onClick={() => setSelectedStock(stock)}
                  className="bg-dark-800 rounded-xl border border-dark-600 p-4 cursor-pointer hover:border-accent-500 transition-colors"
                >
                  <div className="flex justify-between items-start mb-3">
                    <div>
                      <div className="font-semibold text-white">{stock.symbol}</div>
                      <div className="text-xs text-gray-500 truncate max-w-[180px]">{stock.name}</div>
                    </div>
                    {stock.signalChanged ? (
                      <span className="px-2 py-1 text-xs font-bold rounded border border-purple-500/50 bg-purple-500/10 text-purple-300">
                        {stock.prevMonthSignal} → {stock.monthSignal}
                      </span>
                    ) : (
                      <span className={`px-2 py-1 text-xs font-bold rounded border ${getSignalStyle(stock.monthSignal)}`}>
                        {stock.monthSignal}
                      </span>
                    )}
                  </div>
                  <div className="grid grid-cols-3 gap-2 text-sm">
                    <div>
                      <div className="text-xs text-gray-500">Win Rate</div>
                      <div className={`font-medium ${stock.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                        {stock.win_rate?.toFixed(0)}%
                      </div>
                    </div>
                    <div>
                      <div className="text-xs text-gray-500">Total Return</div>
                      <div className={`font-medium ${stock.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {formatPercent(stock.total_return)}
                      </div>
                    </div>
                    <div>
                      <div className="text-xs text-gray-500">Trades</div>
                      <div className="text-white">{stock.total_trades}</div>
                    </div>
                  </div>
                  <div className="mt-2 pt-2 border-t border-dark-700 text-xs text-gray-500">
                    Aktualisiert: {formatDate(stock.updated_at)}
                  </div>
                </div>
              ))}
            </div>

            {/* Desktop Table View */}
            <div className="hidden md:block bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="text-left text-xs text-gray-500 border-b border-dark-600 bg-dark-900/50">
                      <th
                        className="p-4 cursor-pointer hover:text-white transition-colors"
                        onClick={() => handleSort('symbol')}
                      >
                        Symbol <SortIcon field="symbol" />
                      </th>
                      <th className="p-4">Name</th>
                      <th
                        className="p-4 cursor-pointer hover:text-white transition-colors"
                        onClick={() => handleSort('signal')}
                      >
                        Signal <SortIcon field="signal" />
                      </th>
                      <th
                        className="p-4 cursor-pointer hover:text-white transition-colors text-right"
                        onClick={() => handleSort('current_price')}
                      >
                        Kurs <SortIcon field="current_price" />
                      </th>
                      <th
                        className="p-4 cursor-pointer hover:text-white transition-colors text-right"
                        onClick={() => handleSort('win_rate')}
                      >
                        Win Rate <SortIcon field="win_rate" />
                      </th>
                      <th
                        className="p-4 cursor-pointer hover:text-white transition-colors text-right"
                        onClick={() => handleSort('risk_reward')}
                      >
                        R/R <SortIcon field="risk_reward" />
                      </th>
                      <th
                        className="p-4 cursor-pointer hover:text-white transition-colors text-right"
                        onClick={() => handleSort('total_return')}
                      >
                        Total Return <SortIcon field="total_return" />
                      </th>
                      <th
                        className="p-4 cursor-pointer hover:text-white transition-colors text-right"
                        onClick={() => handleSort('total_trades')}
                      >
                        Trades <SortIcon field="total_trades" />
                      </th>
                      <th
                        className="p-4 cursor-pointer hover:text-white transition-colors"
                        onClick={() => handleSort('updated_at')}
                      >
                        Aktualisiert <SortIcon field="updated_at" />
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {sortedStocks.map((stock) => (
                      <tr
                        key={stock.id}
                        onClick={() => setSelectedStock(stock)}
                        className="border-b border-dark-700/50 hover:bg-dark-700/30 transition-colors cursor-pointer"
                      >
                        <td className="p-4 font-medium text-white">{stock.symbol}</td>
                        <td className="p-4 text-gray-400 text-sm truncate max-w-[200px]">{stock.name}</td>
                        <td className="p-4">
                          {stock.signalChanged ? (
                            <span className="px-2 py-1 text-xs font-bold rounded border border-purple-500/50 bg-purple-500/10 text-purple-300">
                              {stock.prevMonthSignal} → {stock.monthSignal}
                            </span>
                          ) : (
                            <span className={`px-2 py-1 text-xs font-bold rounded border ${getSignalStyle(stock.monthSignal)}`}>
                              {stock.monthSignal}
                            </span>
                          )}
                        </td>
                        <td className="p-4 text-right text-white">{formatPrice(stock.current_price)}</td>
                        <td className={`p-4 text-right font-medium ${stock.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                          {stock.win_rate?.toFixed(0)}%
                        </td>
                        <td className={`p-4 text-right font-medium ${stock.risk_reward >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                          {stock.risk_reward?.toFixed(2)}
                        </td>
                        <td className={`p-4 text-right font-bold ${stock.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {formatPercent(stock.total_return)}
                        </td>
                        <td className="p-4 text-right">
                          <span className="text-white">{stock.total_trades}</span>
                          <span className="text-gray-500 text-xs ml-1">
                            ({stock.wins}W/{stock.losses}L)
                          </span>
                        </td>
                        <td className="p-4 text-gray-500 text-sm">{formatDate(stock.updated_at)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>

            {/* Signal Changes Panel */}
            {signalChanges.length > 0 && (
              <div className="mt-6 bg-dark-800 rounded-xl border border-purple-500/30 overflow-hidden">
                <div className="p-4 border-b border-dark-600 bg-purple-500/5">
                  <h3 className="text-lg font-semibold text-purple-300 flex items-center gap-2">
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
                    </svg>
                    Signal-Änderungen für {selectedMonthLabel}
                  </h3>
                </div>
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                        <th className="p-3">Symbol</th>
                        <th className="p-3">Änderung</th>
                        <th className="p-3">Trade Details</th>
                        <th className="p-3 text-right">Rendite</th>
                      </tr>
                    </thead>
                    <tbody>
                      {signalChanges.map((stock) => (
                        <tr key={stock.id} className="border-b border-dark-700/50 hover:bg-dark-700/20">
                          <td className="p-3">
                            <div className="font-medium text-white">{stock.symbol}</div>
                            <div className="text-xs text-gray-500 truncate max-w-[120px]">{stock.name}</div>
                          </td>
                          <td className="p-3">
                            <span className={`px-2 py-1 text-xs font-bold rounded ${getSignalStyle(stock.prevMonthSignal)}`}>
                              {stock.prevMonthSignal}
                            </span>
                            <span className="mx-2 text-gray-500">→</span>
                            <span className={`px-2 py-1 text-xs font-bold rounded ${getSignalStyle(stock.monthSignal)}`}>
                              {stock.monthSignal}
                            </span>
                          </td>
                          <td className="p-3">
                            {stock.tradeInfo ? (
                              <div className="text-xs">
                                <span className="text-green-400">Kauf: ${stock.tradeInfo.buyPrice?.toFixed(2)}</span>
                                <span className="mx-1 text-gray-500">→</span>
                                <span className="text-red-400">Verkauf: ${stock.tradeInfo.sellPrice?.toFixed(2)}</span>
                              </div>
                            ) : (
                              <span className="text-gray-500 text-xs">-</span>
                            )}
                          </td>
                          <td className="p-3 text-right">
                            {stock.tradeInfo ? (
                              <span className={`font-bold ${stock.tradeInfo.returnPct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                {formatPercent(stock.tradeInfo.returnPct)}
                              </span>
                            ) : (
                              <span className="text-gray-500">-</span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </>
        )}

        {/* Trade History Overlay */}
        {selectedStock && (
          <div className="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4" onClick={() => setSelectedStock(null)}>
            <div
              className="bg-dark-800 rounded-xl border border-dark-600 max-w-3xl w-full max-h-[80vh] overflow-hidden"
              onClick={e => e.stopPropagation()}
            >
              {/* Header */}
              <div className="p-4 border-b border-dark-600 flex items-center justify-between">
                <div>
                  <h2 className="text-xl font-bold text-white flex items-center gap-3">
                    {selectedStock.symbol}
                    <span className={`px-2 py-1 text-xs font-bold rounded border ${getSignalStyle(selectedStock.signal)}`}>
                      {selectedStock.signal}
                    </span>
                  </h2>
                  <p className="text-gray-500 text-sm">{selectedStock.name}</p>
                </div>
                <button
                  onClick={() => setSelectedStock(null)}
                  className="p-2 text-gray-400 hover:text-white transition-colors"
                >
                  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              {/* Stats */}
              <div className="p-4 border-b border-dark-600 grid grid-cols-3 md:grid-cols-6 gap-3">
                <div className="bg-dark-700 rounded-lg p-3 text-center">
                  <div className="text-xs text-gray-500">Kurs</div>
                  <div className="text-lg font-bold text-white">{formatPrice(selectedStock.current_price)}</div>
                </div>
                <div className="bg-dark-700 rounded-lg p-3 text-center">
                  <div className="text-xs text-gray-500">Win Rate</div>
                  <div className={`text-lg font-bold ${selectedStock.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                    {selectedStock.win_rate?.toFixed(0)}%
                  </div>
                </div>
                <div className="bg-dark-700 rounded-lg p-3 text-center">
                  <div className="text-xs text-gray-500">R/R</div>
                  <div className={`text-lg font-bold ${selectedStock.risk_reward >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                    {selectedStock.risk_reward?.toFixed(2)}
                  </div>
                </div>
                <div className="bg-dark-700 rounded-lg p-3 text-center">
                  <div className="text-xs text-gray-500">Total Return</div>
                  <div className={`text-lg font-bold ${selectedStock.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {formatPercent(selectedStock.total_return)}
                  </div>
                </div>
                <div className="bg-dark-700 rounded-lg p-3 text-center">
                  <div className="text-xs text-gray-500">Trades</div>
                  <div className="text-lg font-bold text-white">{selectedStock.total_trades}</div>
                </div>
                <div className="bg-dark-700 rounded-lg p-3 text-center">
                  <div className="text-xs text-gray-500">W / L</div>
                  <div className="text-lg font-bold">
                    <span className="text-green-400">{selectedStock.wins}</span>
                    <span className="text-gray-500"> / </span>
                    <span className="text-red-400">{selectedStock.losses}</span>
                  </div>
                </div>
              </div>

              {/* Trade History */}
              <div className="p-4 overflow-auto max-h-[400px]">
                <h3 className="text-sm font-medium text-gray-400 mb-3">TRADE HISTORY</h3>
                {selectedStock.trades && selectedStock.trades.length > 0 ? (
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="text-left text-xs text-gray-500 border-b border-dark-700">
                        <th className="pb-2 pr-2">#</th>
                        <th className="pb-2 pr-2">BUY</th>
                        <th className="pb-2 pr-2">SELL</th>
                        <th className="pb-2 text-right">Return</th>
                      </tr>
                    </thead>
                    <tbody>
                      {selectedStock.trades.map((trade, idx) => (
                        <tr key={idx} className="border-b border-dark-700/50">
                          <td className="py-2 pr-2 text-gray-500">{idx + 1}</td>
                          <td className="py-2 pr-2">
                            <div className="text-gray-400">{formatTradeDate(trade.entryDate)}</div>
                            <div className="text-green-400 font-medium">${trade.entryPrice?.toFixed(2)}</div>
                          </td>
                          <td className="py-2 pr-2">
                            {trade.isOpen ? (
                              <div>
                                <span className="px-2 py-0.5 rounded bg-blue-500/20 text-blue-400 text-xs">OPEN</span>
                                <div className="text-gray-500 text-xs mt-1">${trade.currentPrice?.toFixed(2)}</div>
                              </div>
                            ) : (
                              <div>
                                <div className="text-gray-400">{formatTradeDate(trade.exitDate)}</div>
                                <div className="text-red-400 font-medium">${trade.exitPrice?.toFixed(2)}</div>
                              </div>
                            )}
                          </td>
                          <td className={`py-2 text-right font-bold ${trade.returnPct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {formatPercent(trade.returnPct)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <p className="text-gray-500 text-center py-4">Keine Trades vorhanden</p>
                )}
              </div>

              {/* Footer */}
              <div className="p-4 border-t border-dark-600 text-xs text-gray-500">
                Zuletzt aktualisiert: {formatDate(selectedStock.updated_at)}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

export default StockTracker
