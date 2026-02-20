import { useState, useEffect, useMemo } from 'react'
import { useCurrency } from '../context/CurrencyContext'
import ArenaChart from './ArenaChart'
import ArenaIndicatorChart from './ArenaIndicatorChart'
import ArenaBacktestPanel from './ArenaBacktestPanel'

const BASE_MODES = [
  { value: 'defensive', label: 'Defensiv (FlipperBot)' },
  { value: 'aggressive', label: 'Aggressiv (Lutz)' },
  { value: 'quant', label: 'Quant' },
  { value: 'ditz', label: 'Ditz' },
  { value: 'trader', label: 'Trader' },
]

const MONTHLY_CONDITIONS = [
  { value: 'BUY', label: 'BUY' },
  { value: 'SELL', label: 'SELL' },
  { value: 'HOLD', label: 'HOLD' },
  { value: 'WAIT', label: 'WAIT' },
  { value: 'FIRST_LIGHT_RED', label: '1. Light Red' },
  { value: 'ANY', label: 'Egal' },
]

const WEEKLY_CONDITIONS = [
  { value: 'BUY', label: 'BUY' },
  { value: 'SELL', label: 'SELL' },
  { value: 'HOLD', label: 'HOLD' },
  { value: 'WAIT', label: 'WAIT' },
  { value: 'BUY_TO_HOLD', label: 'BUY\u2192HOLD' },
  { value: 'ANY', label: 'Egal' },
]

const PRESETS = [
  {
    name: 'Monthly Light Red + Weekly BUY',
    rules: [
      { type: 'entry', monthly_condition: 'FIRST_LIGHT_RED', weekly_condition: 'BUY', operator: 'AND' },
    ],
  },
  {
    name: 'Weekly Early Entry',
    rules: [
      { type: 'entry', monthly_condition: 'SELL', weekly_condition: 'BUY_TO_HOLD', operator: 'AND' },
      { type: 'entry', monthly_condition: 'WAIT', weekly_condition: 'BUY_TO_HOLD', operator: 'AND' },
    ],
  },
  {
    name: 'Dual Confirm',
    rules: [
      { type: 'entry', monthly_condition: 'BUY', weekly_condition: 'BUY', operator: 'AND' },
    ],
  },
]

const TIME_RANGES = [
  { value: '1y', label: '1J' }, { value: '2y', label: '2J' }, { value: '3y', label: '3J' },
  { value: '5y', label: '5J' }, { value: '10y', label: '10J' }, { value: 'all', label: 'Max' },
]

// Default rules
const DEFAULT_RULES = [
  { type: 'entry', monthly_condition: 'WAIT', weekly_condition: 'BUY', operator: 'AND' },
  { type: 'exit', monthly_condition: 'SELL', weekly_condition: 'SELL', operator: 'AND' },
]

function BacktestLab({ token, isAdmin }) {
  const { formatPrice } = useCurrency()
  const [stocks, setStocks] = useState([])
  const [selectedSymbol, setSelectedSymbol] = useState('')
  const [baseMode, setBaseMode] = useState('aggressive')
  const [tsl, setTsl] = useState(0)
  const [rules, setRules] = useState(DEFAULT_RULES)
  const [loading, setLoading] = useState(false)
  const [results, setResults] = useState(null)
  const [error, setError] = useState('')

  // Batch mode
  const [batchMode, setBatchMode] = useState(true)
  const [batchResults, setBatchResults] = useState(null)
  const [batchProgress, setBatchProgress] = useState(null)
  const [timeRange, setTimeRange] = useState('3y')

  // Filters
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [filters, setFilters] = useState({
    minWinrate: '', maxWinrate: '', minRR: '', maxRR: '',
    minAvgReturn: '', maxAvgReturn: '', minMarketCap: '50',
  })

  // History
  const [history, setHistory] = useState([])
  const [showHistory, setShowHistory] = useState(false)
  const [selectedHistory, setSelectedHistory] = useState([])

  const handleFilterChange = (f, v) => setFilters(p => ({ ...p, [f]: v }))
  const clearFilters = () => setFilters({ minWinrate: '', maxWinrate: '', minRR: '', maxRR: '', minAvgReturn: '', maxAvgReturn: '', minMarketCap: '' })
  const hasActiveFilters = Object.values(filters).some(v => v !== '')

  useEffect(() => {
    const fetchStocks = async () => {
      try {
        const res = await fetch('/api/stocks', {
          headers: token ? { 'Authorization': `Bearer ${token}` } : {},
        })
        if (res.ok) {
          const data = await res.json()
          setStocks(data || [])
        }
      } catch (err) {
        console.error('Failed to fetch watchlist:', err)
      }
    }
    fetchStocks()
    fetchHistory()
  }, [token])

  const fetchHistory = async () => {
    try {
      const res = await fetch('/api/backtest-lab/history', {
        headers: { 'Authorization': `Bearer ${token}` },
      })
      if (res.ok) {
        const data = await res.json()
        setHistory(data || [])
      }
    } catch { /* ignore */ }
  }

  const deleteHistory = async (id) => {
    try {
      await fetch(`/api/backtest-lab/history/${id}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` },
      })
      setHistory(h => h.filter(i => i.id !== id))
      setSelectedHistory(s => s.filter(i => i !== id))
    } catch { /* ignore */ }
  }

  const toggleHistorySelect = (id) => {
    setSelectedHistory(prev =>
      prev.includes(id) ? prev.filter(i => i !== id) : [...prev, id]
    )
  }

  const addRule = (type) => {
    setRules([...rules, {
      type,
      monthly_condition: type === 'entry' ? 'BUY' : 'SELL',
      weekly_condition: type === 'entry' ? 'BUY' : 'SELL',
      operator: 'AND',
    }])
  }

  const updateRule = (idx, field, value) => {
    const updated = [...rules]
    updated[idx] = { ...updated[idx], [field]: value }
    setRules(updated)
  }

  const removeRule = (idx) => {
    setRules(rules.filter((_, i) => i !== idx))
  }

  const applyPreset = (preset) => {
    setRules([...preset.rules])
  }

  const runSingleBacktest = async () => {
    if (!selectedSymbol) return
    setLoading(true)
    setError('')
    setResults(null)
    setBatchResults(null)
    try {
      const res = await fetch('/api/backtest-lab', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          symbol: selectedSymbol,
          base_mode: baseMode,
          rules,
          tsl,
        }),
      })
      if (res.ok) {
        const data = await res.json()
        setResults(data)
      } else {
        const text = await res.text()
        try {
          const err = JSON.parse(text)
          setError(err.error || 'Fehler beim Backtest')
        } catch {
          setError('Server-Fehler (Status ' + res.status + ')')
        }
      }
    } catch (err) {
      setError('Netzwerkfehler: ' + err.message)
    }
    setLoading(false)
  }

  const runBatchBacktest = async () => {
    setLoading(true)
    setError('')
    setResults(null)
    setBatchResults(null)
    setBatchProgress({ current: 0, total: 0, symbol: '', status: 'Starte Batch-Backtest...' })
    try {
      const body = {
        base_mode: baseMode,
        rules,
        tsl,
        time_range: timeRange,
      }
      if (filters.minWinrate) body.min_winrate = parseFloat(filters.minWinrate)
      if (filters.maxWinrate) body.max_winrate = parseFloat(filters.maxWinrate)
      if (filters.minRR) body.min_rr = parseFloat(filters.minRR)
      if (filters.maxRR) body.max_rr = parseFloat(filters.maxRR)
      if (filters.minAvgReturn) body.min_avg_return = parseFloat(filters.minAvgReturn)
      if (filters.maxAvgReturn) body.max_avg_return = parseFloat(filters.maxAvgReturn)
      if (filters.minMarketCap) body.min_market_cap = parseFloat(filters.minMarketCap)

      const res = await fetch('/api/backtest-lab/batch', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify(body),
      })

      if (!res.ok) {
        const text = await res.text()
        try {
          const err = JSON.parse(text)
          setError(err.error || 'Fehler beim Batch-Backtest')
        } catch {
          setError('Server-Fehler (Status ' + res.status + ')')
        }
        setBatchProgress(null)
        setLoading(false)
        return
      }

      // Read SSE stream
      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })

        // Process complete SSE events (separated by double newline)
        let idx
        while ((idx = buffer.indexOf('\n\n')) !== -1) {
          const block = buffer.slice(0, idx)
          buffer = buffer.slice(idx + 2)

          let eventType = ''
          let dataStr = ''
          for (const line of block.split('\n')) {
            if (line.startsWith('event: ')) {
              eventType = line.slice(7).trim()
            } else if (line.startsWith('data: ')) {
              dataStr = line.slice(6)
            }
          }

          if (dataStr) {
            try {
              const parsed = JSON.parse(dataStr)
              if (eventType === 'progress') {
                setBatchProgress(parsed)
              } else if (eventType === 'result') {
                setBatchResults(parsed)
              }
            } catch { /* incomplete */ }
          }
        }
      }

      // Refresh history after batch completes
      fetchHistory()
    } catch (err) {
      setError('Netzwerkfehler: ' + err.message)
    }
    setBatchProgress(null)
    setLoading(false)
  }

  const entryRules = rules.filter(r => r.type === 'entry')
  const exitRules = rules.filter(r => r.type === 'exit')

  const buildIndicators = (shortData, longData, label) => {
    if (!shortData?.length) return []
    const indicators = [
      {
        type: 'histogram',
        name: `${label} Short`,
        color: '#6366f1',
        data: shortData.map(d => ({
          time: d.time,
          value: d.value,
          color: d.value >= 0 ? '#22c55e' : '#ef4444',
        })),
      },
    ]
    if (longData?.length) {
      indicators.push({
        type: 'line',
        name: `${label} Long`,
        color: '#f59e0b',
        data: longData.map(d => ({ time: d.time, value: d.value })),
      })
    }
    return indicators
  }

  // Sort batch results by total return
  const sortedBatchResults = useMemo(() => {
    if (!batchResults?.stock_results) return []
    return [...batchResults.stock_results].sort((a, b) => b.metrics.total_return - a.metrics.total_return)
  }, [batchResults])

  // Compare mode
  const comparedHistory = useMemo(() => {
    if (selectedHistory.length < 2) return []
    return history.filter(h => selectedHistory.includes(h.id))
  }, [history, selectedHistory])

  return (
    <div className="flex-1 flex flex-col p-4 max-w-7xl mx-auto w-full overflow-auto">
      <h1 className="text-2xl font-bold text-white mb-4">Backtest Lab</h1>

      {/* Mode Toggle */}
      <div className="flex gap-2 mb-4">
        <button
          onClick={() => { setBatchMode(false); setBatchResults(null) }}
          className={`px-4 py-2 rounded text-sm font-medium ${!batchMode ? 'bg-indigo-600 text-white' : 'bg-dark-700 text-gray-400 hover:text-white'}`}
        >
          Einzelaktie
        </button>
        <button
          onClick={() => { setBatchMode(true); setResults(null) }}
          className={`px-4 py-2 rounded text-sm font-medium ${batchMode ? 'bg-indigo-600 text-white' : 'bg-dark-700 text-gray-400 hover:text-white'}`}
        >
          Alle Aktien (Batch)
        </button>
        <button
          onClick={() => setShowHistory(!showHistory)}
          className={`ml-auto px-4 py-2 rounded text-sm font-medium ${showHistory ? 'bg-amber-600 text-white' : 'bg-dark-700 text-gray-400 hover:text-white'}`}
        >
          Historie {history.length > 0 && `(${history.length})`}
        </button>
      </div>

      {/* History View */}
      {showHistory && (
        <HistoryPanel
          history={history}
          selectedHistory={selectedHistory}
          comparedHistory={comparedHistory}
          onToggleSelect={toggleHistorySelect}
          onDelete={deleteHistory}
          isAdmin={isAdmin}
          formatPrice={formatPrice}
        />
      )}

      {/* Controls */}
      <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
        <div className={`grid grid-cols-1 gap-3 mb-4 ${batchMode ? 'md:grid-cols-3' : 'md:grid-cols-4'}`}>
          {!batchMode && (
            <div>
              <label className="text-xs text-gray-400 mb-1 block">Symbol</label>
              <select
                value={selectedSymbol}
                onChange={(e) => setSelectedSymbol(e.target.value)}
                className="w-full bg-dark-700 border border-dark-600 rounded px-3 py-2 text-white text-sm"
              >
                <option value="">-- Symbol w\u00E4hlen --</option>
                {stocks.map(s => (
                  <option key={s.symbol} value={s.symbol}>{s.symbol} - {s.name}</option>
                ))}
              </select>
            </div>
          )}
          <div>
            <label className="text-xs text-gray-400 mb-1 block">Base Mode</label>
            <select
              value={baseMode}
              onChange={(e) => setBaseMode(e.target.value)}
              className="w-full bg-dark-700 border border-dark-600 rounded px-3 py-2 text-white text-sm"
            >
              {BASE_MODES.map(m => (
                <option key={m.value} value={m.value}>{m.label}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="text-xs text-gray-400 mb-1 block">TSL % {tsl === 0 && <span className="text-yellow-500">(Aus)</span>}</label>
            <input
              type="number"
              value={tsl}
              onChange={(e) => setTsl(parseFloat(e.target.value) || 0)}
              className="w-full bg-dark-700 border border-dark-600 rounded px-3 py-2 text-white text-sm"
              min="0" max="50" step="1"
            />
          </div>
          <div className="flex items-end">
            <button
              onClick={batchMode ? runBatchBacktest : runSingleBacktest}
              disabled={(!batchMode && !selectedSymbol) || loading}
              className={`w-full py-2 rounded font-medium text-sm ${
                (batchMode || selectedSymbol) && !loading
                  ? 'bg-indigo-600 text-white hover:bg-indigo-700'
                  : 'bg-dark-700 text-gray-500 cursor-not-allowed'
              }`}
            >
              {loading ? 'Berechne...' : batchMode ? 'Batch starten' : 'Backtest starten'}
            </button>
          </div>
        </div>

        {/* Batch Progress Indicator */}
        {loading && batchMode && batchProgress && (
          <div className="mb-4 bg-dark-700 rounded-lg p-4 border border-dark-600">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm text-gray-300">
                {batchProgress.symbol ? (
                  <>{batchProgress.status} <span className="text-indigo-400 font-medium">{batchProgress.symbol}</span></>
                ) : batchProgress.status}
              </span>
              {batchProgress.total > 0 && (
                <span className="text-xs text-gray-500">{batchProgress.current} / {batchProgress.total}</span>
              )}
            </div>
            {batchProgress.total > 0 && (
              <div className="w-full bg-dark-600 rounded-full h-2">
                <div
                  className="bg-indigo-500 h-2 rounded-full transition-all duration-300"
                  style={{ width: `${(batchProgress.current / batchProgress.total) * 100}%` }}
                />
              </div>
            )}
          </div>
        )}

        {/* Time Range (batch mode) */}
        {batchMode && (
          <div className="mb-4">
            <div className="text-xs text-gray-400 mb-2">Zeitraum</div>
            <div className="inline-flex bg-dark-700 rounded-lg p-1 border border-dark-600">
              {TIME_RANGES.map(o => (
                <button key={o.value} onClick={() => setTimeRange(o.value)}
                  className={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${timeRange === o.value ? 'bg-indigo-600 text-white' : 'text-gray-400 hover:text-white hover:bg-dark-600'}`}>
                  {o.label}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Filters (batch mode) */}
        {batchMode && (
          <div className="mb-4">
            <button
              onClick={() => setFiltersOpen(!filtersOpen)}
              className={`text-xs px-3 py-1.5 rounded border transition-colors ${hasActiveFilters ? 'bg-indigo-600/20 text-indigo-400 border-indigo-500/50' : 'bg-dark-700 text-gray-400 border-dark-600 hover:text-white'}`}
            >
              Filter {hasActiveFilters && '(aktiv)'} {filtersOpen ? '\u25B2' : '\u25BC'}
            </button>
            {filtersOpen && (
              <div className="mt-2 grid grid-cols-2 md:grid-cols-4 gap-2">
                <FilterInput label="Min Winrate %" value={filters.minWinrate} onChange={v => handleFilterChange('minWinrate', v)} />
                <FilterInput label="Max Winrate %" value={filters.maxWinrate} onChange={v => handleFilterChange('maxWinrate', v)} />
                <FilterInput label="Min R/R" value={filters.minRR} onChange={v => handleFilterChange('minRR', v)} step="0.1" />
                <FilterInput label="Max R/R" value={filters.maxRR} onChange={v => handleFilterChange('maxRR', v)} step="0.1" />
                <FilterInput label="Min \u00D8 Rendite %" value={filters.minAvgReturn} onChange={v => handleFilterChange('minAvgReturn', v)} />
                <FilterInput label="Max \u00D8 Rendite %" value={filters.maxAvgReturn} onChange={v => handleFilterChange('maxAvgReturn', v)} />
                <FilterInput label="Min Market Cap (Mrd)" value={filters.minMarketCap} onChange={v => handleFilterChange('minMarketCap', v)} step="0.1" />
                <div className="flex items-end">
                  <button onClick={clearFilters} className="text-xs px-3 py-2 bg-dark-700 text-gray-500 rounded hover:text-white border border-dark-600 w-full">
                    Filter zur\u00FCcksetzen
                  </button>
                </div>
              </div>
            )}
          </div>
        )}

        {/* Presets */}
        <div className="mb-4">
          <div className="text-xs text-gray-400 mb-2">Presets</div>
          <div className="flex flex-wrap gap-2">
            {PRESETS.map((preset, i) => (
              <button
                key={i}
                onClick={() => applyPreset(preset)}
                className="text-xs px-3 py-1.5 bg-dark-700 text-gray-300 rounded hover:bg-dark-600 border border-dark-600 hover:border-indigo-500/50 transition-colors"
              >
                {preset.name}
              </button>
            ))}
            <button
              onClick={() => setRules([])}
              className="text-xs px-3 py-1.5 bg-dark-700 text-gray-500 rounded hover:bg-dark-600 border border-dark-600"
            >
              Zur\u00FCcksetzen
            </button>
          </div>
        </div>

        {/* Entry Rules */}
        <div className="mb-3">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-green-400">Entry-Regeln</span>
            <button
              onClick={() => addRule('entry')}
              className="text-xs px-2 py-1 bg-green-600/20 text-green-400 rounded hover:bg-green-600/30 border border-green-600/30"
            >
              + Entry
            </button>
          </div>
          {entryRules.length === 0 && (
            <div className="text-xs text-gray-600 py-2 px-3 border border-dashed border-dark-600 rounded text-center">
              Keine Entry-Regeln — Base-Mode BUY-Signal wird verwendet
            </div>
          )}
          {rules.map((rule, idx) => rule.type === 'entry' && (
            <RuleRow key={idx} rule={rule} idx={idx} onUpdate={updateRule} onRemove={removeRule} />
          ))}
        </div>

        {/* Exit Rules */}
        <div>
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-red-400">Exit-Regeln</span>
            <button
              onClick={() => addRule('exit')}
              className="text-xs px-2 py-1 bg-red-600/20 text-red-400 rounded hover:bg-red-600/30 border border-red-600/30"
            >
              + Exit
            </button>
          </div>
          {exitRules.length === 0 && (
            <div className="text-xs text-gray-600 py-2 px-3 border border-dashed border-dark-600 rounded text-center">
              Keine Exit-Regeln — Base-Mode SELL-Signal + TSL wird verwendet
            </div>
          )}
          {rules.map((rule, idx) => rule.type === 'exit' && (
            <RuleRow key={idx} rule={rule} idx={idx} onUpdate={updateRule} onRemove={removeRule} />
          ))}
        </div>
      </div>

      {/* Error */}
      {error && (
        <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-3 mb-4 text-red-400 text-sm">
          {error}
        </div>
      )}

      {/* Single Stock Results */}
      {results && !batchMode && (
        <>
          <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
            <h3 className="text-sm font-medium text-white mb-2">{selectedSymbol} — Weekly Chart</h3>
            <ArenaChart
              symbol={selectedSymbol}
              interval="1wk"
              token={token}
              markers={results.markers}
              customData={results.weekly_bars}
            />
          </div>
          <ArenaIndicatorChart
            indicators={buildIndicators(results.monthly_short, results.monthly_long, 'Monthly')}
            strategyName="Monthly BXtrender"
          />
          <ArenaIndicatorChart
            indicators={buildIndicators(results.weekly_short, results.weekly_long, 'Weekly')}
            markers={results.markers}
            strategyName="Weekly BXtrender"
          />
          <ArenaBacktestPanel
            metrics={results.metrics}
            trades={results.trades}
            formatPrice={formatPrice}
            symbol={selectedSymbol}
          />
        </>
      )}

      {/* Batch Results */}
      {batchResults && batchMode && (
        <BatchResults data={batchResults} sortedResults={sortedBatchResults} formatPrice={formatPrice} />
      )}
    </div>
  )
}

// ========== History Panel ==========

function HistoryPanel({ history, selectedHistory, comparedHistory, onToggleSelect, onDelete, isAdmin, formatPrice }) {
  const [expandedId, setExpandedId] = useState(null)
  const formatDate = (ts) => new Date(ts).toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: '2-digit', hour: '2-digit', minute: '2-digit' })
  const formatReturn = (v) => v != null ? `${v >= 0 ? '+' : ''}${v.toFixed(1)}%` : '-'

  const formatRule = (rule) => {
    const mc = rule.monthly_condition === 'FIRST_LIGHT_RED' ? '1.LightRed' : rule.monthly_condition
    const wc = rule.weekly_condition === 'BUY_TO_HOLD' ? 'BUY\u2192HOLD' : rule.weekly_condition
    return `M:${mc} ${rule.operator} W:${wc}`
  }

  const toggleExpand = (id, e) => {
    e.stopPropagation()
    setExpandedId(expandedId === id ? null : id)
  }

  return (
    <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
      <h3 className="text-sm font-medium text-white mb-3">
        Vergangene Tests
        {selectedHistory.length >= 2 && <span className="text-indigo-400 ml-2">({selectedHistory.length} zum Vergleichen ausgew\u00E4hlt)</span>}
      </h3>

      {history.length === 0 && (
        <div className="text-xs text-gray-600 text-center py-4">Noch keine gespeicherten Tests</div>
      )}

      {/* Compare Table */}
      {comparedHistory.length >= 2 && (
        <div className="mb-4 overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-left text-gray-500 border-b border-dark-600">
                <th className="pb-2 pr-3">Test</th>
                <th className="pb-2 pr-3">Modus</th>
                <th className="pb-2 pr-3">Regeln</th>
                <th className="pb-2 pr-3 text-right">Win Rate</th>
                <th className="pb-2 pr-3 text-right">R/R</th>
                <th className="pb-2 pr-3 text-right">Gesamt</th>
                <th className="pb-2 pr-3 text-right">{'\u00D8'}/Trade</th>
                <th className="pb-2 pr-3 text-right">Trades</th>
                <th className="pb-2 text-right">Aktien</th>
              </tr>
            </thead>
            <tbody>
              {comparedHistory.map(h => (
                <tr key={h.id} className="border-b border-dark-700/50">
                  <td className="py-1.5 pr-3 text-gray-300">{formatDate(h.created_at)}</td>
                  <td className="py-1.5 pr-3 text-white font-medium">{h.base_mode}</td>
                  <td className="py-1.5 pr-3">
                    {h.rules?.length > 0 ? h.rules.map((r, i) => (
                      <span key={i} className={`inline-block text-[10px] px-1.5 py-0.5 rounded mr-1 mb-0.5 ${r.type === 'entry' ? 'bg-green-500/15 text-green-400' : 'bg-red-500/15 text-red-400'}`}>
                        {formatRule(r)}
                      </span>
                    )) : <span className="text-gray-600 text-[10px]">Standard</span>}
                  </td>
                  <td className={`py-1.5 pr-3 text-right font-medium ${h.metrics.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                    {h.metrics.win_rate.toFixed(0)}%
                  </td>
                  <td className={`py-1.5 pr-3 text-right ${h.metrics.risk_reward >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                    {h.metrics.risk_reward.toFixed(2)}
                  </td>
                  <td className={`py-1.5 pr-3 text-right font-medium ${h.metrics.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {formatReturn(h.metrics.total_return)}
                  </td>
                  <td className={`py-1.5 pr-3 text-right ${h.metrics.avg_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {formatReturn(h.metrics.avg_return)}
                  </td>
                  <td className="py-1.5 pr-3 text-right text-gray-400">{h.metrics.total_trades}</td>
                  <td className="py-1.5 text-right text-gray-400">{h.tested_stocks}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* History List */}
      <div className="space-y-1 max-h-[500px] overflow-auto">
        {history.map(h => (
          <div key={h.id}>
            <div
              className={`flex items-center gap-3 px-3 py-2 rounded transition-colors cursor-pointer ${
                selectedHistory.includes(h.id)
                  ? 'bg-indigo-600/20 border border-indigo-500/50'
                  : 'bg-dark-700 border border-transparent hover:bg-dark-600'
              }`}
              onClick={() => onToggleSelect(h.id)}
            >
              <input
                type="checkbox"
                checked={selectedHistory.includes(h.id)}
                readOnly
                className="accent-indigo-500"
              />
              <span className="text-xs text-gray-500 w-28 shrink-0">{formatDate(h.created_at)}</span>
              <span className="text-xs text-white font-medium w-20 shrink-0">{h.base_mode}</span>
              <span className="text-xs text-gray-500 w-8 shrink-0">{h.time_range}</span>
              {/* Rules badges */}
              <div className="flex flex-wrap gap-0.5 flex-1 min-w-0">
                {h.rules?.length > 0 ? h.rules.map((r, i) => (
                  <span key={i} className={`text-[10px] px-1 py-0.5 rounded ${r.type === 'entry' ? 'bg-green-500/15 text-green-400' : 'bg-red-500/15 text-red-400'}`}>
                    {formatRule(r)}
                  </span>
                )) : <span className="text-gray-600 text-[10px]">Standard</span>}
              </div>
              <span className={`text-xs font-medium w-12 shrink-0 text-right ${h.metrics.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                {h.metrics.win_rate.toFixed(0)}%
              </span>
              <span className={`text-xs font-medium w-16 shrink-0 text-right ${h.metrics.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                {formatReturn(h.metrics.total_return)}
              </span>
              <span className="text-xs text-gray-500 w-16 shrink-0 text-right">{h.tested_stocks} Aktien</span>
              <span className="text-xs text-gray-600 w-16 shrink-0 text-right">{h.metrics.total_trades} Trades</span>
              <button
                onClick={(e) => toggleExpand(h.id, e)}
                className="text-gray-500 hover:text-indigo-400 text-xs px-1 shrink-0"
                title="Details anzeigen"
              >
                {expandedId === h.id ? '\u25B2' : '\u25BC'}
              </button>
              {isAdmin && (
                <button
                  onClick={(e) => { e.stopPropagation(); onDelete(h.id) }}
                  className="text-gray-600 hover:text-red-400 text-xs px-1 shrink-0"
                  title="Test l\u00F6schen"
                >
                  \u2715
                </button>
              )}
            </div>

            {/* Expanded Detail View */}
            {expandedId === h.id && (
              <div className="mt-1 ml-8 mr-1 mb-2 bg-dark-700/50 rounded-lg border border-dark-600 p-3">
                {/* Metrics */}
                <div className="grid grid-cols-4 gap-1 mb-3">
                  <MetricBox label="Win Rate" value={`${h.metrics.win_rate.toFixed(0)}%`} positive={h.metrics.win_rate >= 50} small />
                  <MetricBox label="R/R" value={h.metrics.risk_reward.toFixed(1)} positive={h.metrics.risk_reward >= 1} small />
                  <MetricBox label="Total" value={formatReturn(h.metrics.total_return)} positive={h.metrics.total_return >= 0} small />
                  <MetricBox label={'\u00D8/Trade'} value={formatReturn(h.metrics.avg_return)} positive={h.metrics.avg_return >= 0} small />
                  <MetricBox label="Trades" value={h.metrics.total_trades} neutral small />
                  <MetricBox label="Wins" value={h.metrics.wins} positive small />
                  <MetricBox label="Losses" value={h.metrics.losses} positive={false} small />
                  <MetricBox label="Max DD" value={`-${h.metrics.max_drawdown.toFixed(1)}%`} positive={false} small />
                </div>

                {/* Rules detail */}
                <div className="mb-3">
                  <div className="text-[10px] text-gray-500 uppercase mb-1">Regeln</div>
                  {h.rules?.length > 0 ? (
                    <div className="space-y-1">
                      {h.rules.map((r, i) => (
                        <div key={i} className="flex items-center gap-2 text-xs">
                          <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${r.type === 'entry' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'}`}>
                            {r.type === 'entry' ? 'ENTRY' : 'EXIT'}
                          </span>
                          <span className="text-gray-400">Monthly:</span>
                          <span className="text-white font-medium">{r.monthly_condition}</span>
                          <span className="text-indigo-400 font-medium">{r.operator}</span>
                          <span className="text-gray-400">Weekly:</span>
                          <span className="text-white font-medium">{r.weekly_condition}</span>
                        </div>
                      ))}
                    </div>
                  ) : <span className="text-xs text-gray-600">Keine Custom-Regeln (Standard-Modus)</span>}
                  {h.tsl > 0 && <div className="text-xs text-gray-500 mt-1">TSL: {h.tsl}%</div>}
                </div>

                {/* Stock Summary Table */}
                {h.stock_summary?.length > 0 && (
                  <div>
                    <div className="text-[10px] text-gray-500 uppercase mb-1">Top Aktien (nach Rendite)</div>
                    <div className="max-h-48 overflow-auto">
                      <table className="w-full text-xs">
                        <thead>
                          <tr className="text-left text-gray-500 border-b border-dark-600">
                            <th className="pb-1 pr-2">Symbol</th>
                            <th className="pb-1 pr-2">Name</th>
                            <th className="pb-1 pr-2 text-right">WR</th>
                            <th className="pb-1 pr-2 text-right">Rendite</th>
                            <th className="pb-1 pr-2 text-right">{'\u00D8'}/Trade</th>
                            <th className="pb-1 pr-2 text-right">R/R</th>
                            <th className="pb-1 text-right">Trades</th>
                          </tr>
                        </thead>
                        <tbody>
                          {[...h.stock_summary].sort((a, b) => b.total_return - a.total_return).map((s, i) => (
                            <tr key={i} className="border-b border-dark-700/50 last:border-0">
                              <td className="py-1 pr-2 text-white font-medium">{s.symbol}</td>
                              <td className="py-1 pr-2 text-gray-500 truncate max-w-[120px]">{s.name}</td>
                              <td className={`py-1 pr-2 text-right ${s.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                                {s.win_rate.toFixed(0)}%
                              </td>
                              <td className={`py-1 pr-2 text-right font-medium ${s.total_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                {formatReturn(s.total_return)}
                              </td>
                              <td className={`py-1 pr-2 text-right ${s.avg_return >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                {formatReturn(s.avg_return)}
                              </td>
                              <td className={`py-1 pr-2 text-right ${s.risk_reward >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                                {s.risk_reward.toFixed(1)}
                              </td>
                              <td className="py-1 text-right text-gray-400">{s.total_trades}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

// ========== Sub Components ==========

function FilterInput({ label, value, onChange, step = '1' }) {
  return (
    <div>
      <label className="text-xs text-gray-500 mb-1 block">{label}</label>
      <input
        type="number"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="-"
        step={step}
        className="w-full bg-dark-700 border border-dark-600 rounded px-2 py-1.5 text-white text-xs"
      />
    </div>
  )
}

function BatchResults({ data, sortedResults, formatPrice }) {
  const [expandedSymbol, setExpandedSymbol] = useState(null)

  const formatReturn = (v) => {
    if (v == null) return '-'
    return `${v >= 0 ? '+' : ''}${v.toFixed(1)}%`
  }

  const formatTime = (ts) => {
    if (!ts) return '-'
    const d = new Date(ts * 1000)
    return d.toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: '2-digit' })
  }

  return (
    <>
      {/* Summary */}
      <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
        <h3 className="text-sm font-medium text-white mb-3">
          Gesamt-Performance ({data.tested_stocks} Aktien getestet)
        </h3>
        <div className="text-xs text-gray-500 mb-3">
          {data.total_stocks} Watchlist | {data.filtered_stocks} gefiltert | {data.tested_stocks} getestet | {data.skipped_stocks?.length || 0} \u00FCbersprungen
        </div>

        {/* Total Metrics Grid */}
        <div className="grid grid-cols-4 gap-2 mb-4">
          <MetricBox label="Win Rate" value={`${data.total_metrics.win_rate.toFixed(0)}%`} positive={data.total_metrics.win_rate >= 50} />
          <MetricBox label="R/R" value={data.total_metrics.risk_reward.toFixed(1)} positive={data.total_metrics.risk_reward >= 1} />
          <MetricBox label="Total" value={formatReturn(data.total_metrics.total_return)} positive={data.total_metrics.total_return >= 0} />
          <MetricBox label={'\u00D8/Trade'} value={formatReturn(data.total_metrics.avg_return)} positive={data.total_metrics.avg_return >= 0} />
          <MetricBox label="Trades" value={data.total_metrics.total_trades} neutral />
          <MetricBox label="Wins" value={data.total_metrics.wins} positive />
          <MetricBox label="Losses" value={data.total_metrics.losses} positive={false} />
          <MetricBox label="Max DD" value={`-${data.total_metrics.max_drawdown.toFixed(1)}%`} positive={false} />
        </div>
      </div>

      {/* Per-Stock Results */}
      <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
        <h3 className="text-sm font-medium text-white mb-3">Ergebnisse pro Aktie</h3>
        <div className="space-y-1">
          {sortedResults.map(r => (
            <div key={r.symbol}>
              <button
                onClick={() => setExpandedSymbol(expandedSymbol === r.symbol ? null : r.symbol)}
                className="w-full flex items-center justify-between px-3 py-2 bg-dark-700 rounded hover:bg-dark-600 transition-colors"
              >
                <div className="flex items-center gap-3">
                  <span className="text-sm font-medium text-white w-16 text-left">{r.symbol}</span>
                  <span className="text-xs text-gray-500 truncate max-w-[150px]">{r.name}</span>
                </div>
                <div className="flex items-center gap-4 text-xs">
                  <span className={r.metrics.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}>
                    WR: {r.metrics.win_rate.toFixed(0)}%
                  </span>
                  <span className={r.metrics.total_return >= 0 ? 'text-green-400' : 'text-red-400'}>
                    {formatReturn(r.metrics.total_return)}
                  </span>
                  <span className="text-gray-500">{r.metrics.total_trades} Trades</span>
                  <span className="text-gray-600">{expandedSymbol === r.symbol ? '\u25B2' : '\u25BC'}</span>
                </div>
              </button>
              {expandedSymbol === r.symbol && (
                <div className="mt-1 ml-4 mr-1 mb-2">
                  <div className="grid grid-cols-4 gap-1 mb-2">
                    <MetricBox label="R/R" value={r.metrics.risk_reward.toFixed(1)} positive={r.metrics.risk_reward >= 1} small />
                    <MetricBox label={'\u00D8/Trade'} value={formatReturn(r.metrics.avg_return)} positive={r.metrics.avg_return >= 0} small />
                    <MetricBox label="Wins" value={r.metrics.wins} positive small />
                    <MetricBox label="Losses" value={r.metrics.losses} positive={false} small />
                  </div>
                  <div className="max-h-48 overflow-auto">
                    <table className="w-full text-xs">
                      <thead>
                        <tr className="text-left text-gray-500 border-b border-dark-600">
                          <th className="pb-1 pr-2">Entry</th>
                          <th className="pb-1 pr-2">Exit</th>
                          <th className="pb-1 text-right">Return</th>
                          <th className="pb-1 text-right">Reason</th>
                        </tr>
                      </thead>
                      <tbody>
                        {[...r.trades].filter(t => !t.is_open).reverse().map((t, i) => (
                          <tr key={i} className="border-b border-dark-700/50 last:border-0">
                            <td className="py-1 pr-2 text-gray-400">
                              {formatPrice ? formatPrice(t.entry_price, r.symbol) : `$${t.entry_price.toFixed(2)}`}
                              <span className="text-gray-600 text-[10px] ml-1">{formatTime(t.entry_time)}</span>
                            </td>
                            <td className="py-1 pr-2 text-gray-400">
                              {formatPrice ? formatPrice(t.exit_price, r.symbol) : `$${t.exit_price.toFixed(2)}`}
                              <span className="text-gray-600 text-[10px] ml-1">{formatTime(t.exit_time)}</span>
                            </td>
                            <td className={`py-1 text-right font-medium ${t.return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatReturn(t.return_pct)}
                            </td>
                            <td className="py-1 text-right">
                              <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                                t.exit_reason === 'TSL' ? 'bg-red-500/20 text-red-400' : 'bg-yellow-500/20 text-yellow-400'
                              }`}>
                                {t.exit_reason}
                              </span>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Skipped Stocks */}
      {data.skipped_stocks?.length > 0 && (
        <div className="bg-dark-800 rounded-lg border border-dark-600 p-4 mb-4">
          <h3 className="text-sm font-medium text-yellow-400 mb-3">
            \u00DCbersprungene Aktien ({data.skipped_stocks.length})
          </h3>
          <div className="space-y-1 max-h-48 overflow-auto">
            {data.skipped_stocks.map((s, i) => (
              <div key={i} className="flex items-center justify-between px-3 py-1.5 bg-dark-700 rounded text-xs">
                <div className="flex items-center gap-2">
                  <span className="text-white font-medium w-16">{s.symbol}</span>
                  <span className="text-gray-500 truncate max-w-[200px]">{s.name}</span>
                </div>
                <span className="text-yellow-500/70 truncate max-w-[300px]">{s.reason}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </>
  )
}

function MetricBox({ label, value, positive, neutral, small }) {
  const colorClass = neutral ? 'text-white' : positive ? 'text-green-400' : 'text-red-400'
  return (
    <div className={`${small ? 'bg-dark-600 p-1.5' : 'bg-dark-700 p-2'} rounded text-center`}>
      <div className={`${small ? 'text-[10px]' : 'text-xs'} text-gray-500`}>{label}</div>
      <div className={`${small ? 'text-xs' : 'text-base'} font-bold ${colorClass}`}>{value}</div>
    </div>
  )
}

function RuleRow({ rule, idx, onUpdate, onRemove }) {
  return (
    <div className="bg-dark-700 rounded p-2 mb-2 flex items-center gap-2 flex-wrap">
      <span className="text-xs text-gray-500 w-14">Monthly:</span>
      <select
        value={rule.monthly_condition}
        onChange={(e) => onUpdate(idx, 'monthly_condition', e.target.value)}
        className="bg-dark-600 border border-dark-500 rounded px-2 py-1 text-xs text-white"
      >
        {MONTHLY_CONDITIONS.map(c => (
          <option key={c.value} value={c.value}>{c.label}</option>
        ))}
      </select>

      <select
        value={rule.operator}
        onChange={(e) => onUpdate(idx, 'operator', e.target.value)}
        className="bg-dark-600 border border-dark-500 rounded px-2 py-1 text-xs text-indigo-400 font-medium"
      >
        <option value="AND">AND</option>
        <option value="OR">OR</option>
      </select>

      <span className="text-xs text-gray-500">Weekly:</span>
      <select
        value={rule.weekly_condition}
        onChange={(e) => onUpdate(idx, 'weekly_condition', e.target.value)}
        className="bg-dark-600 border border-dark-500 rounded px-2 py-1 text-xs text-white"
      >
        {WEEKLY_CONDITIONS.map(c => (
          <option key={c.value} value={c.value}>{c.label}</option>
        ))}
      </select>

      <span className="text-xs text-gray-500 mx-1">=</span>
      <span className={`text-xs font-medium ${rule.type === 'entry' ? 'text-green-400' : 'text-red-400'}`}>
        {rule.type === 'entry' ? 'BUY' : 'SELL'}
      </span>

      <button
        onClick={() => onRemove(idx)}
        className="ml-auto text-gray-500 hover:text-red-400 text-xs px-1"
      >
        \u2715
      </button>
    </div>
  )
}

export default BacktestLab
