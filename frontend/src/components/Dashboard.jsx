import { useState, useEffect, useCallback } from 'react'
import ChartArea from './ChartArea'
import Watchlist from './Watchlist'
import BacktestPanel from './BacktestPanel'

function Dashboard({ isAdmin, token }) {
  const [stocks, setStocks] = useState([])
  const [loading, setLoading] = useState(true)
  const [selectedStock, setSelectedStock] = useState(null)
  const [backtestData, setBacktestData] = useState({ trades: [], metrics: null })

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

  return (
    <main className="flex flex-1 overflow-hidden">
      <div className="flex-1 p-6 overflow-auto">
        <ChartArea
          stock={selectedStock}
          stocks={stocks}
          onBacktestUpdate={handleBacktestUpdate}
          onSelectStock={setSelectedStock}
        />
      </div>
      <aside className="w-80 bg-dark-800 border-l border-dark-600 flex flex-col">
        {/* Watchlist - Top */}
        <div className="flex-1 overflow-hidden">
          <Watchlist
            stocks={stocks}
            loading={loading}
            isAdmin={isAdmin}
            onAdd={addStock}
            onDelete={deleteStock}
            onSelectStock={setSelectedStock}
          />
        </div>

        {/* Backtest Panel - Bottom */}
        <div className="border-t border-dark-600 max-h-[50%] overflow-auto">
          <BacktestPanel
            trades={backtestData.trades}
            metrics={backtestData.metrics}
          />
        </div>
      </aside>
    </main>
  )
}

export default Dashboard
