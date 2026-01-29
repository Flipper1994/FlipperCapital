import { useState, useEffect, useCallback } from 'react'
import ChartArea from './ChartArea'
import Watchlist from './Watchlist'
import BacktestPanel from './BacktestPanel'

function Dashboard({ isAdmin, token }) {
  const [stocks, setStocks] = useState([])
  const [loading, setLoading] = useState(true)
  const [selectedStock, setSelectedStock] = useState(null)
  const [backtestData, setBacktestData] = useState({ trades: [], metrics: null })

  // Mobile collapsible state for watchlist (default collapsed)
  const [watchlistExpanded, setWatchlistExpanded] = useState(false)

  const fetchStocks = useCallback(async (showLoading = false) => {
    if (showLoading) setLoading(true)
    try {
      const res = await fetch('/api/stocks')
      const data = await res.json()
      setStocks(data)

      if (selectedStock) {
        const updated = data.find(s => s.id === selectedStock.id)
        if (updated) setSelectedStock(updated)
      }
    } catch (err) {
      console.error('Failed to fetch stocks:', err)
    } finally {
      setLoading(false)
    }
  }, [selectedStock])

  useEffect(() => {
    fetchStocks(true)
  }, [])

  useEffect(() => {
    const interval = setInterval(() => {
      fetchStocks(false)
    }, 30000)

    return () => clearInterval(interval)
  }, [fetchStocks])

  const addStock = async (stock) => {
    try {
      const res = await fetch('/api/stocks', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify(stock)
      })
      if (res.ok) {
        await fetchStocks(false)
        return true
      }
      return false
    } catch {
      return false
    }
  }

  const deleteStock = async (id) => {
    try {
      const res = await fetch(`/api/stocks/${id}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${token}`
        }
      })
      if (res.ok) {
        if (selectedStock?.id === id) {
          setSelectedStock(null)
        }
        fetchStocks(false)
      }
    } catch (err) {
      console.error('Failed to delete stock:', err)
    }
  }

  const handleBacktestUpdate = useCallback((data) => {
    setBacktestData(data)
  }, [])

  // Chevron icon component for collapsible sections
  const ChevronIcon = ({ expanded }) => (
    <svg
      className={`w-5 h-5 transition-transform duration-200 ${expanded ? 'rotate-180' : ''}`}
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
    >
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
    </svg>
  )

  return (
    <main className="flex flex-col md:flex-row flex-1 overflow-hidden">
      {/* Chart Area */}
      <div className="flex-1 p-4 md:p-6 overflow-auto">
        <ChartArea
          stock={selectedStock}
          stocks={stocks}
          onBacktestUpdate={handleBacktestUpdate}
          onSelectStock={setSelectedStock}
          backtestData={backtestData}
        />
      </div>

      {/* Right Sidebar - Desktop: fixed sidebar, Mobile: collapsible sections */}
      <aside className="md:w-80 bg-dark-800 border-t md:border-t-0 md:border-l border-dark-600 flex flex-col">
        {/* Watchlist Section */}
        <div className="flex-1 md:flex-1 overflow-hidden flex flex-col">
          {/* Collapsible Header - Mobile only */}
          <button
            onClick={() => setWatchlistExpanded(!watchlistExpanded)}
            className="md:hidden flex items-center justify-between w-full px-4 py-3 bg-dark-700 border-b border-dark-600"
          >
            <div className="flex items-center gap-2">
              <svg className="w-5 h-5 text-accent-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
              </svg>
              <span className="text-white font-medium">Watchlist</span>
              <span className="text-xs text-gray-500">({stocks.length})</span>
            </div>
            <ChevronIcon expanded={watchlistExpanded} />
          </button>

          {/* Watchlist Content */}
          <div className={`
            overflow-hidden transition-all duration-300 ease-in-out
            ${watchlistExpanded ? 'max-h-[400px]' : 'max-h-0'}
            md:max-h-none md:flex-1
          `}>
            <Watchlist
              stocks={stocks}
              loading={loading}
              isAdmin={isAdmin}
              onAdd={addStock}
              onDelete={deleteStock}
              onSelectStock={setSelectedStock}
            />
          </div>
        </div>

        {/* Backtest Panel Section - Desktop only (mobile version is in ChartArea) */}
        <div className="hidden md:flex border-t border-dark-600 md:max-h-[50%] overflow-hidden flex-col">
          <div className="overflow-auto flex-1">
            <BacktestPanel
              trades={backtestData.trades}
              metrics={backtestData.metrics}
            />
          </div>
        </div>
      </aside>
    </main>
  )
}

export default Dashboard
