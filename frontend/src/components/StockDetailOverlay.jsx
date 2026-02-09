import { useState, useEffect } from 'react'
import { useCurrency } from '../context/CurrencyContext'

const MODE_API_MAP = {
  defensive: '/api/performance/',
  aggressive: '/api/performance/aggressive/',
  quant: '/api/performance/quant/',
  ditz: '/api/performance/ditz/',
  trader: '/api/performance/trader/'
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
    case 'NO_DATA':
      return 'bg-gray-500/20 text-gray-400 border-gray-500/30'
    default:
      return 'bg-gray-500/20 text-gray-400 border-gray-500/30'
  }
}

const formatTradeDate = (timestamp) => {
  if (!timestamp) return '-'
  const date = new Date(timestamp * 1000)
  return date.toLocaleDateString('de-DE', { month: 'short', year: 'numeric' })
}

const formatPercent = (value) => {
  if (value === undefined || value === null || isNaN(value)) return '--'
  const sign = value >= 0 ? '+' : ''
  return `${sign}${value.toFixed(1)}%`
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

export default function StockDetailOverlay({ symbol, name, mode, onClose }) {
  const [data, setData] = useState(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const { formatPrice } = useCurrency()

  useEffect(() => {
    if (!symbol || !mode) return
    setLoading(true)
    setError(null)
    const base = MODE_API_MAP[mode] || MODE_API_MAP.defensive
    fetch(`${base}${encodeURIComponent(symbol)}`)
      .then(res => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json()
      })
      .then(d => {
        // API returns { performance: {...}, trades: [...] } — flatten for display
        const perf = d.performance || {}
        setData({
          ...perf,
          trades: d.trades || [],
          monthSignal: perf.signal,
        })
        setLoading(false)
      })
      .catch(err => { setError(err.message); setLoading(false) })
  }, [symbol, mode])

  return (
    <div className="fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4" onClick={onClose}>
      <div
        className="bg-dark-800 rounded-xl border border-dark-600 max-w-3xl w-full max-h-[80vh] overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="p-4 border-b border-dark-600 flex items-center justify-between">
          <div>
            <h2 className="text-xl font-bold text-white flex items-center gap-3">
              {symbol}
              {data && (
                <span className={`px-2 py-1 text-xs font-bold rounded border ${getSignalStyle(data.monthSignal || data.signal)}`}>
                  {data.monthSignal || data.signal}
                </span>
              )}
            </h2>
            <p className="text-gray-500 text-sm">{name}</p>
          </div>
          <button
            onClick={onClose}
            className="p-2 text-gray-400 hover:text-white transition-colors"
          >
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {loading ? (
          <div className="p-12 flex items-center justify-center">
            <div className="w-8 h-8 border-2 border-gray-600 border-t-white rounded-full animate-spin"></div>
          </div>
        ) : error ? (
          <div className="p-8 text-center text-red-400">Fehler: {error}</div>
        ) : data ? (
          <>
            {/* Stats */}
            <div className="p-4 border-b border-dark-600 grid grid-cols-4 md:grid-cols-7 gap-3">
              <div className="bg-dark-700 rounded-lg p-3 text-center">
                <div className="text-xs text-gray-500">Kurs</div>
                <div className="text-lg font-bold text-white">{formatPrice(data.current_price, symbol)}</div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 text-center">
                <div className="text-xs text-gray-500">Win Rate</div>
                <div className={`text-lg font-bold ${data.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                  {data.win_rate?.toFixed(0)}%
                </div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 text-center">
                <div className="text-xs text-gray-500">R/R</div>
                <div className={`text-lg font-bold ${data.risk_reward >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                  {data.risk_reward?.toFixed(2)}
                </div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 text-center">
                <div className="text-xs text-gray-500">Total</div>
                <div className={`text-lg font-bold ${data.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {formatPercent(data.total_return)}
                </div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 text-center">
                <div className="text-xs text-gray-500">Ø/Trade</div>
                <div className={`text-lg font-bold ${(data.avg_return || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {formatPercent(data.avg_return)}
                </div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 text-center">
                <div className="text-xs text-gray-500">Trades</div>
                <div className="text-lg font-bold text-white">{data.total_trades}</div>
              </div>
              <div className="bg-dark-700 rounded-lg p-3 text-center">
                <div className="text-xs text-gray-500">W / L</div>
                <div className="text-lg font-bold">
                  <span className="text-green-400">{data.wins}</span>
                  <span className="text-gray-500"> / </span>
                  <span className="text-red-400">{data.losses}</span>
                </div>
              </div>
            </div>

            {/* Trade History */}
            <div className="p-4 overflow-auto max-h-[400px]">
              <h3 className="text-sm font-medium text-gray-400 mb-3">TRADE HISTORY</h3>
              {data.trades && data.trades.length > 0 ? (
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
                    {[...data.trades].reverse().map((trade, idx) => (
                      <tr key={idx} className="border-b border-dark-700/50">
                        <td className="py-2 pr-2 text-gray-500">{data.trades.length - idx}</td>
                        <td className="py-2 pr-2">
                          <div className="text-gray-400">{formatTradeDate(trade.entryDate)}</div>
                          <div className="text-gray-300 font-medium">{formatPrice(trade.entryPrice, symbol)}</div>
                        </td>
                        <td className="py-2 pr-2">
                          {trade.isOpen ? (
                            <div>
                              <span className="px-2 py-0.5 rounded bg-blue-500/20 text-blue-400 text-xs">OPEN</span>
                              <div className="text-gray-500 text-xs mt-1">{formatPrice(trade.currentPrice, symbol)}</div>
                            </div>
                          ) : (
                            <div>
                              <div className="text-gray-400">{formatTradeDate(trade.exitDate)}</div>
                              <div className="text-gray-300 font-medium">{formatPrice(trade.exitPrice, symbol)}</div>
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
              Zuletzt aktualisiert: {formatDate(data.updated_at)}
            </div>
          </>
        ) : null}
      </div>
    </div>
  )
}
