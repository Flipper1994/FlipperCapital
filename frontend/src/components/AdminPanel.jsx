import { useState, useEffect, useRef, useMemo } from 'react'
import { Link, Navigate } from 'react-router-dom'
import { useTradingMode } from '../context/TradingModeContext'
import { useCurrency } from '../context/CurrencyContext'
import { processStock } from '../utils/bxtrender'
import PortfolioChart from './PortfolioChart'
import StockDetailOverlay from './StockDetailOverlay'

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
  const [editingUserId, setEditingUserId] = useState(null)
  const [editingUsername, setEditingUsername] = useState('')
  const [updatingStocks, setUpdatingStocks] = useState(false)
  const [updateProgress, setUpdateProgress] = useState(null)
  const [forceUpdate, setForceUpdate] = useState(false)
  const [lastFullUpdate, setLastFullUpdate] = useState(null)
  const [showTrackedDiff, setShowTrackedDiff] = useState(false)
  const [trackedDiff, setTrackedDiff] = useState({ defensive: [], aggressive: [] })
  const [loadingDiff, setLoadingDiff] = useState(false)
  const { mode, isAggressive } = useTradingMode()
  const { formatPrice, convertPrice, convertToUSD, currencySymbol } = useCurrency()

  // Stock detail overlay
  const [selectedPosition, setSelectedPosition] = useState(null)

  // Bots state
  const [flipperPositions, setFlipperPositions] = useState([])
  const [flipperTrades, setFlipperTrades] = useState([])
  const [lutzPositions, setLutzPositions] = useState([])
  const [lutzTrades, setLutzTrades] = useState([])
  const [quantPositions, setQuantPositions] = useState([])
  const [quantTrades, setQuantTrades] = useState([])
  const [botTab, setBotTab] = useState('flipper')
  const [editingItem, setEditingItem] = useState(null)
  const [fixingDB, setFixingDB] = useState(false)
  const [fixResult, setFixResult] = useState(null)
  const [backfillDate, setBackfillDate] = useState('')
  const [backfilling, setBackfilling] = useState(false)
  const [backfillResult, setBackfillResult] = useState(null)
  const [backfillProgress, setBackfillProgress] = useState(null)
  const [showBxConfig, setShowBxConfig] = useState(false)
  const [showBotReset, setShowBotReset] = useState(false)
  const [showBackfill, setShowBackfill] = useState(false)
  const [importProgress, setImportProgress] = useState(null)
  const [importing, setImporting] = useState(false)
  const [flipperPendingTrades, setFlipperPendingTrades] = useState([])
  const [lutzPendingTrades, setLutzPendingTrades] = useState([])
  const [quantPendingTrades, setQuantPendingTrades] = useState([])
  const [acceptingTrade, setAcceptingTrade] = useState(null)
  const [quantPrivatePortfolio, setQuantPrivatePortfolio] = useState(null)
  const [quantPrivatePerformance, setQuantPrivatePerformance] = useState(null)
  const [lastQuantRefresh, setLastQuantRefresh] = useState(null)
  const [quantRefreshing, setQuantRefreshing] = useState(false)
  const [schedulerTime, setSchedulerTime] = useState('00:00')
  const [savingSchedulerTime, setSavingSchedulerTime] = useState(false)
  const [schedulerCountdown, setSchedulerCountdown] = useState('')
  const [quantRefreshLogs, setQuantRefreshLogs] = useState([])
  const [quantSortColumn, setQuantSortColumn] = useState('symbol')
  const [quantSortDir, setQuantSortDir] = useState('asc')
  const [bxtrenderConfig, setBxtrenderConfig] = useState({
    defensive: { short_l1: 5, short_l2: 20, short_l3: 15, long_l1: 20, long_l2: 15, tsl_percent: 20.0, tsl_enabled: true },
    aggressive: { short_l1: 5, short_l2: 20, short_l3: 15, long_l1: 20, long_l2: 15, tsl_percent: 15.0, tsl_enabled: true }
  })
  const [quantConfig, setQuantConfig] = useState({
    short_l1: 5, short_l2: 20, short_l3: 15,
    long_l1: 20, long_l2: 15,
    ma_filter_on: true, ma_length: 200, ma_type: 'EMA',
    tsl_percent: 20.0, tsl_enabled: true
  })
  const [savingConfig, setSavingConfig] = useState(false)
  const [savingQuantConfig, setSavingQuantConfig] = useState(false)
  const [showManualTrade, setShowManualTrade] = useState(false)
  const [manualTrade, setManualTrade] = useState({ symbol: '', name: '', action: 'BUY', price: '', quantity: '', date: '', is_live: false })
  const [creatingManualTrade, setCreatingManualTrade] = useState(false)
  const [quantUnreadCount, setQuantUnreadCount] = useState(0)
  const [quantUnreadTrades, setQuantUnreadTrades] = useState([])

  // Flipper/Lutz SIM state
  const [flipperPrivatePortfolio, setFlipperPrivatePortfolio] = useState(null)
  const [flipperPrivatePerformance, setFlipperPrivatePerformance] = useState(null)
  const [lastFlipperRefresh, setLastFlipperRefresh] = useState(null)
  const [flipperRefreshing, setFlipperRefreshing] = useState(false)
  const [flipperRefreshLogs, setFlipperRefreshLogs] = useState([])
  const [flipperUnreadCount, setFlipperUnreadCount] = useState(0)
  const [flipperUnreadTrades, setFlipperUnreadTrades] = useState([])
  const [flipperSortColumn, setFlipperSortColumn] = useState('symbol')
  const [flipperSortDir, setFlipperSortDir] = useState('asc')
  const [lutzPrivatePortfolio, setLutzPrivatePortfolio] = useState(null)
  const [lutzPrivatePerformance, setLutzPrivatePerformance] = useState(null)
  const [lastLutzRefresh, setLastLutzRefresh] = useState(null)
  const [lutzRefreshing, setLutzRefreshing] = useState(false)
  const [lutzRefreshLogs, setLutzRefreshLogs] = useState([])
  const [lutzUnreadCount, setLutzUnreadCount] = useState(0)
  const [lutzUnreadTrades, setLutzUnreadTrades] = useState([])
  const [lutzSortColumn, setLutzSortColumn] = useState('symbol')
  const [lutzSortDir, setLutzSortDir] = useState('asc')

  // Ditz state
  const [ditzPositions, setDitzPositions] = useState([])
  const [ditzTrades, setDitzTrades] = useState([])
  const [ditzPendingTrades, setDitzPendingTrades] = useState([])
  const [ditzPrivatePortfolio, setDitzPrivatePortfolio] = useState(null)
  const [ditzPrivatePerformance, setDitzPrivatePerformance] = useState(null)
  const [lastDitzRefresh, setLastDitzRefresh] = useState(null)
  const [ditzRefreshing, setDitzRefreshing] = useState(false)
  const [ditzRefreshLogs, setDitzRefreshLogs] = useState([])
  const [ditzSortColumn, setDitzSortColumn] = useState('symbol')
  const [ditzSortDir, setDitzSortDir] = useState('asc')
  const [ditzConfig, setDitzConfig] = useState({
    short_l1: 5, short_l2: 20, short_l3: 15,
    long_l1: 20, long_l2: 15,
    ma_filter_on: true, ma_length: 200, ma_type: 'EMA',
    tsl_percent: 20.0, tsl_enabled: true
  })
  const [savingDitzConfig, setSavingDitzConfig] = useState(false)
  const [ditzUnreadCount, setDitzUnreadCount] = useState(0)
  const [ditzUnreadTrades, setDitzUnreadTrades] = useState([])
  const [ditzManualTrade, setDitzManualTrade] = useState({ symbol: '', name: '', action: 'BUY', price: '', quantity: '', date: '', is_live: false })
  const [showDitzManualTrade, setShowDitzManualTrade] = useState(false)

  // Trader state
  const [traderPositions, setTraderPositions] = useState([])
  const [traderTrades, setTraderTrades] = useState([])
  const [traderPendingTrades, setTraderPendingTrades] = useState([])
  const [traderPrivatePortfolio, setTraderPrivatePortfolio] = useState(null)
  const [traderPrivatePerformance, setTraderPrivatePerformance] = useState(null)
  const [lastTraderRefresh, setLastTraderRefresh] = useState(null)
  const [traderRefreshing, setTraderRefreshing] = useState(false)
  const [traderRefreshLogs, setTraderRefreshLogs] = useState([])
  const [traderSortColumn, setTraderSortColumn] = useState('symbol')
  const [traderSortDir, setTraderSortDir] = useState('asc')
  const [traderConfig, setTraderConfig] = useState({
    short_l1: 5, short_l2: 20, short_l3: 15,
    long_l1: 20, long_l2: 15,
    ma_filter_on: false, ma_length: 200, ma_type: 'EMA',
    tsl_percent: 20.0, tsl_enabled: true
  })
  const [savingTraderConfig, setSavingTraderConfig] = useState(false)
  const [traderUnreadCount, setTraderUnreadCount] = useState(0)
  const [traderUnreadTrades, setTraderUnreadTrades] = useState([])
  const [traderManualTrade, setTraderManualTrade] = useState({ symbol: '', name: '', action: 'BUY', price: '', quantity: '', date: '', is_live: false })
  const [showTraderManualTrade, setShowTraderManualTrade] = useState(false)

  // Completed trades state
  const [flipperCompletedTrades, setFlipperCompletedTrades] = useState([])
  const [lutzCompletedTrades, setLutzCompletedTrades] = useState([])
  const [quantCompletedTrades, setQuantCompletedTrades] = useState([])
  const [ditzCompletedTrades, setDitzCompletedTrades] = useState([])
  const [traderCompletedTrades, setTraderCompletedTrades] = useState([])
  const [showFlipperTradeHistory, setShowFlipperTradeHistory] = useState(false)
  const [showLutzTradeHistory, setShowLutzTradeHistory] = useState(false)
  const [showQuantTradeHistory, setShowQuantTradeHistory] = useState(false)
  const [showDitzTradeHistory, setShowDitzTradeHistory] = useState(false)
  const [showTraderTradeHistory, setShowTraderTradeHistory] = useState(false)

  const [manualTradeSearch, setManualTradeSearch] = useState('')
  const [manualTradeResults, setManualTradeResults] = useState([])
  const [manualTradeSearching, setManualTradeSearching] = useState(false)

  // Allowlist state
  const [allowlistData, setAllowlistData] = useState({})
  const [allowlistLoading, setAllowlistLoading] = useState(false)
  const [allowlistFilter, setAllowlistFilter] = useState('')
  const [allowlistMessage, setAllowlistMessage] = useState(null)

  // Bot filter config state
  const [botFilterConfigs, setBotFilterConfigs] = useState({})
  const [botFilterSaving, setBotFilterSaving] = useState(null)
  const [globalFilter, setGlobalFilter] = useState({})

  const fetchAllowlist = async () => {
    setAllowlistLoading(true)
    try {
      const res = await fetch('/api/admin/bot-allowlist', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      if (res.ok) {
        const data = await res.json()
        setAllowlistData(data)
      }
    } catch (err) {
      console.error('Failed to fetch allowlist:', err)
    }
    setAllowlistLoading(false)
  }

  const toggleAllBots = async (symbol) => {
    const bots = ['flipper', 'lutz', 'quant', 'ditz', 'trader']
    // Check current state: if ALL that have entries are allowed → block all, otherwise → allow all
    const entries = bots.map(bot => {
      const botEntries = allowlistData[bot] || []
      return { bot, entry: botEntries.find(e => e.symbol === symbol) }
    }).filter(b => b.entry)
    if (entries.length === 0) return
    const allAllowed = entries.every(b => b.entry.allowed)
    const newAllowed = !allAllowed

    let retroactiveScan = false
    if (allAllowed) {
      retroactiveScan = confirm(`${symbol} für ALLE Bots verbieten.\n\nSollen Trades rückwirkend gescannt und als gelöscht markiert werden?`)
    }

    const messages = []
    for (const { bot } of entries) {
      try {
        const res = await fetch('/api/admin/bot-allowlist', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
          body: JSON.stringify({ bot_name: bot, symbol, allowed: newAllowed, retroactive_scan: retroactiveScan })
        })
        if (res.ok) {
          const data = await res.json()
          if (data.retroactive_deleted > 0) messages.push(`${bot}: ${data.retroactive_deleted} Trades gelöscht`)
          if (data.closed_position) messages.push(`${bot}: Position geschlossen`)
        }
      } catch { /* ignore */ }
    }

    // Update local state for all bots at once
    setAllowlistData(prev => {
      const updated = { ...prev }
      for (const { bot } of entries) {
        if (updated[bot]) {
          updated[bot] = updated[bot].map(entry =>
            entry.symbol === symbol ? { ...entry, allowed: newAllowed } : entry
          )
        }
      }
      return updated
    })

    if (messages.length > 0) {
      setAllowlistMessage(`${symbol}: ${messages.join(', ')}`)
      setTimeout(() => setAllowlistMessage(null), 6000)
    }
  }

  const toggleAllowlist = async (botName, symbol, currentAllowed) => {
    let retroactiveScan = false
    if (currentAllowed) {
      // Banning — ask about retroactive scan
      retroactiveScan = confirm(`${symbol} für ${botName} verbieten.\n\nSollen Trades rückwirkend gescannt und als gelöscht markiert werden?`)
    }
    try {
      const res = await fetch('/api/admin/bot-allowlist', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify({ bot_name: botName, symbol, allowed: !currentAllowed, retroactive_scan: retroactiveScan })
      })
      if (res.ok) {
        const data = await res.json()
        if (data.retroactive_deleted > 0) {
          setAllowlistMessage(`${symbol} bei ${botName} verboten — ${data.retroactive_deleted} Trades rückwirkend gelöscht`)
          setTimeout(() => setAllowlistMessage(null), 6000)
        } else if (data.closed_position) {
          setAllowlistMessage(`Position ${symbol} bei ${botName} wurde geschlossen`)
          setTimeout(() => setAllowlistMessage(null), 4000)
        }
        // Update local state
        setAllowlistData(prev => {
          const updated = { ...prev }
          if (updated[botName]) {
            updated[botName] = updated[botName].map(entry =>
              entry.symbol === symbol ? { ...entry, allowed: !currentAllowed } : entry
            )
          }
          return updated
        })
      }
    } catch (err) {
      console.error('Failed to toggle allowlist:', err)
    }
  }

  useEffect(() => {
    checkAdmin()
  }, [])

  useEffect(() => {
    if (activeTab === 'allowlist' && Object.keys(allowlistData).length === 0) {
      fetchAllowlist()
    }
    if (activeTab === 'botfilter') {
      fetchBotFilterConfigs()
    }
  }, [activeTab])

  const fetchBotFilterConfigs = async () => {
    try {
      const res = await fetch('/api/admin/bot-filter-config', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      if (res.ok) {
        const data = await res.json()
        setBotFilterConfigs(data)
      }
    } catch (err) {
      console.error('Failed to fetch bot filter configs:', err)
    }
  }

  const saveBotFilterConfig = async (botName) => {
    setBotFilterSaving(botName)
    try {
      const config = botFilterConfigs[botName] || {}
      const body = {
        bot_name: botName,
        enabled: config.enabled || false,
        min_winrate: config.min_winrate !== '' && config.min_winrate != null ? parseFloat(config.min_winrate) : null,
        max_winrate: config.max_winrate !== '' && config.max_winrate != null ? parseFloat(config.max_winrate) : null,
        min_rr: config.min_rr !== '' && config.min_rr != null ? parseFloat(config.min_rr) : null,
        max_rr: config.max_rr !== '' && config.max_rr != null ? parseFloat(config.max_rr) : null,
        min_avg_return: config.min_avg_return !== '' && config.min_avg_return != null ? parseFloat(config.min_avg_return) : null,
        max_avg_return: config.max_avg_return !== '' && config.max_avg_return != null ? parseFloat(config.max_avg_return) : null,
        min_market_cap: config.min_market_cap !== '' && config.min_market_cap != null ? parseFloat(config.min_market_cap) : null,
      }
      const res = await fetch('/api/admin/bot-filter-config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
        body: JSON.stringify(body)
      })
      if (res.ok) {
        const data = await res.json()
        setBotFilterConfigs(prev => ({ ...prev, [botName]: data }))
        alert(`${botName} Filter gespeichert!`)
      }
    } catch (err) {
      console.error('Failed to save bot filter config:', err)
      alert('Fehler beim Speichern')
    }
    setBotFilterSaving(null)
  }

  const updateBotFilterValue = (botName, field, value) => {
    setBotFilterConfigs(prev => ({
      ...prev,
      [botName]: { ...(prev[botName] || { bot_name: botName }), [field]: value }
    }))
  }

  const applyGlobalFilterToAll = async () => {
    const bots = ['flipper', 'lutz', 'quant', 'ditz', 'trader']
    const fields = ['min_winrate', 'max_winrate', 'min_rr', 'max_rr', 'min_avg_return', 'max_avg_return', 'min_market_cap']
    setBotFilterSaving('global')
    try {
      for (const botName of bots) {
        const prev = botFilterConfigs[botName] || { bot_name: botName }
        const merged = { ...prev }
        for (const f of fields) {
          if (globalFilter[f] !== undefined && globalFilter[f] !== '') {
            merged[f] = globalFilter[f]
          }
        }
        if (globalFilter.enabled !== undefined) {
          merged.enabled = globalFilter.enabled
        }
        const body = {
          bot_name: botName,
          enabled: merged.enabled || false,
          min_winrate: merged.min_winrate !== '' && merged.min_winrate != null ? parseFloat(merged.min_winrate) : null,
          max_winrate: merged.max_winrate !== '' && merged.max_winrate != null ? parseFloat(merged.max_winrate) : null,
          min_rr: merged.min_rr !== '' && merged.min_rr != null ? parseFloat(merged.min_rr) : null,
          max_rr: merged.max_rr !== '' && merged.max_rr != null ? parseFloat(merged.max_rr) : null,
          min_avg_return: merged.min_avg_return !== '' && merged.min_avg_return != null ? parseFloat(merged.min_avg_return) : null,
          max_avg_return: merged.max_avg_return !== '' && merged.max_avg_return != null ? parseFloat(merged.max_avg_return) : null,
          min_market_cap: merged.min_market_cap !== '' && merged.min_market_cap != null ? parseFloat(merged.min_market_cap) : null,
        }
        const res = await fetch('/api/admin/bot-filter-config', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
          body: JSON.stringify(body)
        })
        if (res.ok) {
          const data = await res.json()
          setBotFilterConfigs(prev => ({ ...prev, [botName]: data }))
        }
      }
      alert('Filter auf alle Bots angewendet!')
    } catch (err) {
      console.error('Failed to apply global filter:', err)
      alert('Fehler beim Anwenden')
    }
    setBotFilterSaving(null)
  }

  useEffect(() => {
    if (isAdmin) {
      if (activeTab === 'dashboard') {
        fetchStats()
        fetchLastFullUpdate()
        fetchSchedulerTime()
      }
      if (activeTab === 'users') fetchUsers()
      if (activeTab === 'activity') fetchActivity()
      if (activeTab === 'traffic') fetchTraffic()
      if (activeTab === 'bots') {
        fetchBotData()
        fetchBXtrenderConfig()
        fetchFlipperSimulatedData()
        fetchLastFlipperRefresh()
        fetchFlipperUnreadCount()
        fetchLutzSimulatedData()
        fetchLastLutzRefresh()
        fetchLutzUnreadCount()
        fetchQuantConfig()
        fetchQuantSimulatedData()
        fetchLastQuantRefresh()
        fetchQuantUnreadCount()
        fetchDitzConfig()
        fetchDitzSimulatedData()
        fetchLastDitzRefresh()
        fetchDitzUnreadCount()
        fetchTraderConfig()
        fetchTraderSimulatedData()
        fetchLastTraderRefresh()
        fetchTraderUnreadCount()
      }
    }
  }, [isAdmin, activeTab, activityFilter])

  // Countdown to next scheduler run
  useEffect(() => {
    const calcCountdown = () => {
      if (!schedulerTime) return
      const [h, m] = schedulerTime.split(':').map(Number)
      const now = new Date()
      // Convert current time to Europe/Berlin
      const berlinNow = new Date(now.toLocaleString('en-US', { timeZone: 'Europe/Berlin' }))
      const next = new Date(berlinNow)
      next.setHours(h, m, 0, 0)
      if (next <= berlinNow) next.setDate(next.getDate() + 1)
      const diff = next - berlinNow
      const hours = Math.floor(diff / 3600000)
      const mins = Math.floor((diff % 3600000) / 60000)
      const secs = Math.floor((diff % 60000) / 1000)
      setSchedulerCountdown(`${String(hours).padStart(2, '0')}:${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`)
    }
    calcCountdown()
    const interval = setInterval(calcCountdown, 1000)
    return () => clearInterval(interval)
  }, [schedulerTime])

  // Enrich performance data with win rate, avg_win/loss, risk_reward from ALL trades (closed + open positions)
  const enrichPerformance = (perf, completedTrades, portfolio) => {
    if (!perf) return perf
    const allItems = [
      ...(completedTrades || []).map(t => ({ pct: t.profit_loss_pct || 0 })),
      ...((portfolio?.positions) || []).map(p => ({ pct: p.total_return_pct || 0 }))
    ]
    const allWins = allItems.filter(i => i.pct > 0)
    const allLosses = allItems.filter(i => i.pct < 0)
    const win_rate = allItems.length > 0 ? (allWins.length / allItems.length) * 100 : perf.win_rate || 0
    const wins = allWins.length
    const losses = allLosses.length
    const avg_win_pct = allWins.length > 0 ? allWins.reduce((s, i) => s + i.pct, 0) / allWins.length : 0
    const avg_loss_pct = allLosses.length > 0 ? Math.abs(allLosses.reduce((s, i) => s + i.pct, 0) / allLosses.length) : 0
    const risk_reward = avg_loss_pct > 0 ? avg_win_pct / avg_loss_pct : avg_win_pct > 0 ? Infinity : 0
    return { ...perf, win_rate, wins, losses, avg_win_pct, avg_loss_pct, risk_reward, total_trades: allItems.length }
  }

  const flipperEnrichedPerf = useMemo(() => enrichPerformance(flipperPrivatePerformance, flipperCompletedTrades, flipperPrivatePortfolio), [flipperPrivatePerformance, flipperCompletedTrades, flipperPrivatePortfolio])
  const lutzEnrichedPerf = useMemo(() => enrichPerformance(lutzPrivatePerformance, lutzCompletedTrades, lutzPrivatePortfolio), [lutzPrivatePerformance, lutzCompletedTrades, lutzPrivatePortfolio])
  const quantEnrichedPerf = useMemo(() => enrichPerformance(quantPrivatePerformance, quantCompletedTrades, quantPrivatePortfolio), [quantPrivatePerformance, quantCompletedTrades, quantPrivatePortfolio])
  const ditzEnrichedPerf = useMemo(() => enrichPerformance(ditzPrivatePerformance, ditzCompletedTrades, ditzPrivatePortfolio), [ditzPrivatePerformance, ditzCompletedTrades, ditzPrivatePortfolio])
  const traderEnrichedPerf = useMemo(() => enrichPerformance(traderPrivatePerformance, traderCompletedTrades, traderPrivatePortfolio), [traderPrivatePerformance, traderCompletedTrades, traderPrivatePortfolio])

  const fetchLastFullUpdate = async () => {
    try {
      const res = await fetch('/api/admin/last-full-update', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      setLastFullUpdate(data)
    } catch (err) {
      console.error('Failed to fetch last full update:', err)
    }
  }

  const fetchSchedulerTime = async () => {
    try {
      const res = await fetch('/api/admin/scheduler-time', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      if (data.time) setSchedulerTime(data.time)
    } catch (err) {
      console.error('Failed to fetch scheduler time:', err)
    }
  }

  const saveSchedulerTime = async () => {
    setSavingSchedulerTime(true)
    try {
      const res = await fetch('/api/admin/scheduler-time', {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ time: schedulerTime })
      })
      const data = await res.json()
      if (data.time) setSchedulerTime(data.time)
    } catch (err) {
      console.error('Failed to save scheduler time:', err)
    }
    setSavingSchedulerTime(false)
  }

  const fetchQuantSimulatedData = async () => {
    try {
      const [portfolioRes, perfRes] = await Promise.all([
        fetch('/api/quant/simulated-portfolio', {
          headers: { 'Authorization': `Bearer ${token}` }
        }),
        fetch('/api/quant/simulated-performance', {
          headers: { 'Authorization': `Bearer ${token}` }
        })
      ])
      const portfolioData = await portfolioRes.json()
      const perfData = await perfRes.json()
      setQuantPrivatePortfolio(portfolioData)
      setQuantPrivatePerformance(perfData)
    } catch (err) {
      console.error('Failed to fetch Quant simulated data:', err)
    }
  }

  const fetchLastQuantRefresh = async () => {
    try {
      const res = await fetch('/api/quant/last-refresh', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      if (data && data.updated_at) {
        setLastQuantRefresh(data)
        if (data.logs) {
          setQuantRefreshLogs(data.logs)
        }
      }
    } catch (err) {
      console.error('Failed to fetch last Quant refresh:', err)
    }
  }

  const fetchBXtrenderConfig = async () => {
    try {
      const res = await fetch('/api/admin/bxtrender-config', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      if (Array.isArray(data)) {
        const config = {}
        data.forEach(c => { config[c.mode] = c })
        setBxtrenderConfig(config)
      }
    } catch (err) {
      console.error('Failed to fetch BXtrender config:', err)
    }
  }

  const handleSaveBXtrenderConfig = async (mode) => {
    setSavingConfig(true)
    try {
      const config = bxtrenderConfig[mode]
      const res = await fetch('/api/admin/bxtrender-config', {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          mode,
          short_l1: parseInt(config.short_l1),
          short_l2: parseInt(config.short_l2),
          short_l3: parseInt(config.short_l3),
          long_l1: parseInt(config.long_l1),
          long_l2: parseInt(config.long_l2),
          tsl_percent: parseFloat(config.tsl_percent) || 20.0,
          tsl_enabled: config.tsl_enabled !== false
        })
      })
      if (res.ok) {
        // Clear config cache in bxtrender.js
        const { clearConfigCache } = await import('../utils/bxtrender')
        clearConfigCache()
        alert(`${mode === 'defensive' ? 'Defensiv' : 'Aggressiv'} Konfiguration gespeichert!`)
      } else {
        alert('Fehler beim Speichern')
      }
    } catch (err) {
      console.error('Failed to save config:', err)
      alert('Fehler beim Speichern')
    } finally {
      setSavingConfig(false)
    }
  }

  const updateConfigValue = (mode, field, value) => {
    setBxtrenderConfig(prev => ({
      ...prev,
      [mode]: {
        ...prev[mode],
        [field]: value
      }
    }))
  }

  const fetchQuantConfig = async () => {
    try {
      const res = await fetch('/api/admin/bxtrender-quant-config', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      if (data && data.id) {
        setQuantConfig(data)
      }
    } catch (err) {
      console.error('Failed to fetch Quant config:', err)
    }
  }

  const handleSaveQuantConfig = async () => {
    setSavingQuantConfig(true)
    try {
      const res = await fetch('/api/admin/bxtrender-quant-config', {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          short_l1: parseInt(quantConfig.short_l1),
          short_l2: parseInt(quantConfig.short_l2),
          short_l3: parseInt(quantConfig.short_l3),
          long_l1: parseInt(quantConfig.long_l1),
          long_l2: parseInt(quantConfig.long_l2),
          ma_filter_on: quantConfig.ma_filter_on,
          ma_length: parseInt(quantConfig.ma_length),
          ma_type: quantConfig.ma_type,
          tsl_percent: parseFloat(quantConfig.tsl_percent),
          tsl_enabled: quantConfig.tsl_enabled !== false
        })
      })
      if (res.ok) {
        const { clearQuantConfigCache } = await import('../utils/bxtrender')
        clearQuantConfigCache()
        alert('Quant Konfiguration gespeichert!')
      } else {
        alert('Fehler beim Speichern')
      }
    } catch (err) {
      console.error('Failed to save Quant config:', err)
      alert('Fehler beim Speichern')
    } finally {
      setSavingQuantConfig(false)
    }
  }

  const updateQuantConfigValue = (field, value) => {
    setQuantConfig(prev => ({ ...prev, [field]: value }))
  }

  // Ditz functions
  // Flipper/Lutz SIM fetch functions
  const fetchFlipperSimulatedData = async () => {
    try {
      const [portfolioRes, perfRes] = await Promise.all([
        fetch('/api/flipperbot/simulated-portfolio', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/flipperbot/simulated-performance', { headers: { 'Authorization': `Bearer ${token}` } })
      ])
      const portfolioData = await portfolioRes.json()
      const perfData = await perfRes.json()
      setFlipperPrivatePortfolio(portfolioData)
      setFlipperPrivatePerformance(perfData)
    } catch (err) {
      console.error('Failed to fetch Flipper simulated data:', err)
    }
  }

  const fetchLastFlipperRefresh = async () => {
    try {
      const res = await fetch('/api/flipperbot/last-refresh', { headers: { 'Authorization': `Bearer ${token}` } })
      const data = await res.json()
      if (data && data.updated_at) {
        setLastFlipperRefresh(data)
        if (data.logs) setFlipperRefreshLogs(data.logs)
      }
    } catch (err) {
      console.error('Failed to fetch last Flipper refresh:', err)
    }
  }

  const fetchFlipperUnreadCount = async () => {
    try {
      const res = await fetch('/api/flipperbot/trades/unread-count', { headers: { 'Authorization': `Bearer ${token}` } })
      const data = await res.json()
      setFlipperUnreadCount(data.count || 0)
      setFlipperUnreadTrades(data.trades || [])
    } catch (err) {
      console.error('Failed to fetch Flipper unread count:', err)
    }
  }

  const fetchLutzSimulatedData = async () => {
    try {
      const [portfolioRes, perfRes] = await Promise.all([
        fetch('/api/lutz/simulated-portfolio', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/lutz/simulated-performance', { headers: { 'Authorization': `Bearer ${token}` } })
      ])
      const portfolioData = await portfolioRes.json()
      const perfData = await perfRes.json()
      setLutzPrivatePortfolio(portfolioData)
      setLutzPrivatePerformance(perfData)
    } catch (err) {
      console.error('Failed to fetch Lutz simulated data:', err)
    }
  }

  const fetchLastLutzRefresh = async () => {
    try {
      const res = await fetch('/api/lutz/last-refresh', { headers: { 'Authorization': `Bearer ${token}` } })
      const data = await res.json()
      if (data && data.updated_at) {
        setLastLutzRefresh(data)
        if (data.logs) setLutzRefreshLogs(data.logs)
      }
    } catch (err) {
      console.error('Failed to fetch last Lutz refresh:', err)
    }
  }

  const fetchLutzUnreadCount = async () => {
    try {
      const res = await fetch('/api/lutz/trades/unread-count', { headers: { 'Authorization': `Bearer ${token}` } })
      const data = await res.json()
      setLutzUnreadCount(data.count || 0)
      setLutzUnreadTrades(data.trades || [])
    } catch (err) {
      console.error('Failed to fetch Lutz unread count:', err)
    }
  }

  const fetchDitzSimulatedData = async () => {
    try {
      const [portfolioRes, perfRes] = await Promise.all([
        fetch('/api/ditz/simulated-portfolio', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/ditz/simulated-performance', { headers: { 'Authorization': `Bearer ${token}` } })
      ])
      const portfolioData = await portfolioRes.json()
      const perfData = await perfRes.json()
      setDitzPrivatePortfolio(portfolioData)
      setDitzPrivatePerformance(perfData)
    } catch (err) {
      console.error('Failed to fetch Ditz simulated data:', err)
    }
  }

  const fetchLastDitzRefresh = async () => {
    try {
      const res = await fetch('/api/ditz/last-refresh', { headers: { 'Authorization': `Bearer ${token}` } })
      const data = await res.json()
      if (data && data.updated_at) {
        setLastDitzRefresh(data)
        if (data.logs) setDitzRefreshLogs(data.logs)
      }
    } catch (err) {
      console.error('Failed to fetch last Ditz refresh:', err)
    }
  }

  const fetchDitzConfig = async () => {
    try {
      const res = await fetch('/api/admin/bxtrender-ditz-config', { headers: { 'Authorization': `Bearer ${token}` } })
      const data = await res.json()
      if (data && data.id) setDitzConfig(data)
    } catch (err) {
      console.error('Failed to fetch Ditz config:', err)
    }
  }

  const handleSaveDitzConfig = async () => {
    setSavingDitzConfig(true)
    try {
      const res = await fetch('/api/admin/bxtrender-ditz-config', {
        method: 'PUT',
        headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          short_l1: parseInt(ditzConfig.short_l1),
          short_l2: parseInt(ditzConfig.short_l2),
          short_l3: parseInt(ditzConfig.short_l3),
          long_l1: parseInt(ditzConfig.long_l1),
          long_l2: parseInt(ditzConfig.long_l2),
          ma_filter_on: ditzConfig.ma_filter_on,
          ma_length: parseInt(ditzConfig.ma_length),
          ma_type: ditzConfig.ma_type,
          tsl_percent: parseFloat(ditzConfig.tsl_percent),
          tsl_enabled: ditzConfig.tsl_enabled !== false
        })
      })
      if (res.ok) {
        const { clearDitzConfigCache } = await import('../utils/bxtrender')
        clearDitzConfigCache()
        alert('Ditz Konfiguration gespeichert!')
      } else {
        alert('Fehler beim Speichern')
      }
    } catch (err) {
      console.error('Failed to save Ditz config:', err)
      alert('Fehler beim Speichern')
    } finally {
      setSavingDitzConfig(false)
    }
  }

  const updateDitzConfigValue = (field, value) => {
    setDitzConfig(prev => ({ ...prev, [field]: value }))
  }

  const fetchDitzUnreadCount = async () => {
    try {
      const res = await fetch('/api/ditz/trades/unread-count', { headers: { 'Authorization': `Bearer ${token}` } })
      const data = await res.json()
      if (res.ok) {
        setDitzUnreadCount(data.count || 0)
        setDitzUnreadTrades(data.trades || [])
      }
    } catch (err) { /* ignore */ }
  }

  const toggleDitzTradeRead = async (tradeId) => {
    try {
      const res = await fetch(`/api/ditz/trade/${tradeId}/read`, { method: 'PUT', headers: { 'Authorization': `Bearer ${token}` } })
      if (res.ok) {
        fetchBotData()
        fetchDitzUnreadCount()
      }
    } catch (err) { /* ignore */ }
  }

  const markAllDitzTradesRead = async () => {
    try {
      const res = await fetch('/api/ditz/trades/read-all', { method: 'PUT', headers: { 'Authorization': `Bearer ${token}` } })
      if (res.ok) {
        fetchBotData()
        fetchDitzUnreadCount()
      }
    } catch (err) { /* ignore */ }
  }

  const markAllDitzTradesUnread = async () => {
    try {
      const res = await fetch('/api/ditz/trades/unread-all', { method: 'PUT', headers: { 'Authorization': `Bearer ${token}` } })
      if (res.ok) {
        fetchBotData()
        fetchDitzUnreadCount()
      }
    } catch (err) { /* ignore */ }
  }

  const fetchTraderSimulatedData = async () => {
    try {
      const [portfolioRes, perfRes] = await Promise.all([
        fetch('/api/trader/simulated-portfolio', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/trader/simulated-performance', { headers: { 'Authorization': `Bearer ${token}` } })
      ])
      const portfolioData = await portfolioRes.json()
      const perfData = await perfRes.json()
      setTraderPrivatePortfolio(portfolioData)
      setTraderPrivatePerformance(perfData)
    } catch (err) {
      console.error('Failed to fetch Trader simulated data:', err)
    }
  }

  const fetchLastTraderRefresh = async () => {
    try {
      const res = await fetch('/api/trader/last-refresh', { headers: { 'Authorization': `Bearer ${token}` } })
      const data = await res.json()
      if (data && data.updated_at) {
        setLastTraderRefresh(data)
        if (data.logs) setTraderRefreshLogs(data.logs)
      }
    } catch (err) {
      console.error('Failed to fetch last Trader refresh:', err)
    }
  }

  const fetchTraderConfig = async () => {
    try {
      const res = await fetch('/api/admin/bxtrender-trader-config', { headers: { 'Authorization': `Bearer ${token}` } })
      const data = await res.json()
      if (data && data.id) setTraderConfig(data)
    } catch (err) {
      console.error('Failed to fetch Trader config:', err)
    }
  }

  const handleSaveTraderConfig = async () => {
    setSavingTraderConfig(true)
    try {
      const res = await fetch('/api/admin/bxtrender-trader-config', {
        method: 'PUT',
        headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          short_l1: parseInt(traderConfig.short_l1),
          short_l2: parseInt(traderConfig.short_l2),
          short_l3: parseInt(traderConfig.short_l3),
          long_l1: parseInt(traderConfig.long_l1),
          long_l2: parseInt(traderConfig.long_l2),
          ma_filter_on: traderConfig.ma_filter_on,
          ma_length: parseInt(traderConfig.ma_length),
          ma_type: traderConfig.ma_type,
          tsl_percent: parseFloat(traderConfig.tsl_percent),
          tsl_enabled: traderConfig.tsl_enabled !== false
        })
      })
      if (res.ok) {
        const { clearTraderConfigCache } = await import('../utils/bxtrender')
        clearTraderConfigCache()
        alert('Trader Konfiguration gespeichert!')
      } else {
        alert('Fehler beim Speichern')
      }
    } catch (err) {
      console.error('Failed to save Trader config:', err)
      alert('Fehler beim Speichern')
    } finally {
      setSavingTraderConfig(false)
    }
  }

  const updateTraderConfigValue = (field, value) => {
    setTraderConfig(prev => ({ ...prev, [field]: value }))
  }

  const fetchTraderUnreadCount = async () => {
    try {
      const res = await fetch('/api/trader/trades/unread-count', { headers: { 'Authorization': `Bearer ${token}` } })
      const data = await res.json()
      if (res.ok) {
        setTraderUnreadCount(data.count || 0)
        setTraderUnreadTrades(data.trades || [])
      }
    } catch (err) { /* ignore */ }
  }

  const toggleTraderTradeRead = async (tradeId) => {
    try {
      const res = await fetch(`/api/trader/trade/${tradeId}/read`, { method: 'PUT', headers: { 'Authorization': `Bearer ${token}` } })
      if (res.ok) {
        fetchBotData()
        fetchTraderUnreadCount()
      }
    } catch (err) { /* ignore */ }
  }

  const markAllTraderTradesRead = async () => {
    try {
      const res = await fetch('/api/trader/trades/read-all', { method: 'PUT', headers: { 'Authorization': `Bearer ${token}` } })
      if (res.ok) {
        fetchBotData()
        fetchTraderUnreadCount()
      }
    } catch (err) { /* ignore */ }
  }

  const markAllTraderTradesUnread = async () => {
    try {
      const res = await fetch('/api/trader/trades/unread-all', { method: 'PUT', headers: { 'Authorization': `Bearer ${token}` } })
      if (res.ok) {
        fetchBotData()
        fetchTraderUnreadCount()
      }
    } catch (err) { /* ignore */ }
  }

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
      const [fpRes, ftRes, lpRes, ltRes, fptRes, lptRes, qpRes, qtRes, qptRes, dpRes, dtRes, dptRes, fspRes, lspRes, fctRes, lctRes, qctRes, dctRes, trpRes, trtRes, trptRes, trctRes] = await Promise.all([
        fetch('/api/flipperbot/simulated-portfolio', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/flipperbot/actions-all', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/lutz/simulated-portfolio', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/lutz/actions-all', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/flipperbot/pending-trades', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/lutz/pending-trades', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/quant/simulated-portfolio', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/quant/actions-all', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/quant/pending-trades', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/ditz/simulated-portfolio', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/ditz/actions-all', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/ditz/pending-trades', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/flipperbot/simulated-performance', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/lutz/simulated-performance', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/flipperbot/completed-trades', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/lutz/completed-trades', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/quant/completed-trades', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/ditz/completed-trades', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/trader/simulated-portfolio', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/trader/actions-all', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/trader/pending-trades', { headers: { 'Authorization': `Bearer ${token}` } }),
        fetch('/api/trader/completed-trades', { headers: { 'Authorization': `Bearer ${token}` } })
      ])
      const [fp, ft, lp, lt, fpt, lpt, qp, qt, qpt, dp, dtt, dpt, fsp, lsp, fct, lct, qct, dct, trp, trt, trpt, trct] = await Promise.all([
        fpRes.json(), ftRes.json(), lpRes.json(), ltRes.json(), fptRes.json(), lptRes.json(),
        qpRes.json(), qtRes.json(), qptRes.json(),
        dpRes.json(), dtRes.json(), dptRes.json(),
        fspRes.json(), lspRes.json(),
        fctRes.json(), lctRes.json(), qctRes.json(), dctRes.json(),
        trpRes.json(), trtRes.json(), trptRes.json(), trctRes.json()
      ])
      setFlipperPositions(fp?.positions || [])
      setFlipperTrades(ft || [])
      setFlipperPrivatePortfolio(fp)
      setFlipperPrivatePerformance(fsp)
      setLutzPositions(lp?.positions || [])
      setLutzTrades(lt || [])
      setLutzPrivatePortfolio(lp)
      setLutzPrivatePerformance(lsp)
      setFlipperPendingTrades(fpt || [])
      setLutzPendingTrades(lpt || [])
      setQuantPositions(qp?.positions || [])
      setQuantTrades(qt || [])
      setQuantPendingTrades(qpt || [])
      fetchQuantUnreadCount()
      fetchFlipperUnreadCount()
      fetchLutzUnreadCount()
      setDitzPositions(dp?.positions || [])
      setDitzTrades(dtt || [])
      setDitzPendingTrades(dpt || [])
      setFlipperCompletedTrades(fct || [])
      setLutzCompletedTrades(lct || [])
      setQuantCompletedTrades(qct || [])
      setDitzCompletedTrades(dct || [])
      fetchDitzUnreadCount()
      setTraderPositions(trp?.positions || [])
      setTraderTrades(trt || [])
      setTraderPendingTrades(trpt || [])
      setTraderCompletedTrades(trct || [])
      fetchTraderUnreadCount()
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
      const body = {
        quantity: parseFloat(trade.quantity),
        price: priceInUSD,
        is_live: trade.is_live
      }
      if (trade.signal_date) {
        body.signal_date = trade.signal_date + 'T00:00:00Z'
      }
      const res = await fetch(`/api/${bot}/trade/${trade.id}`, {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(body)
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

  const handleCreateManualTrade = async () => {
    if (!manualTrade.symbol || !manualTrade.price) {
      alert('Symbol und Preis sind Pflichtfelder')
      return
    }
    setCreatingManualTrade(true)
    try {
      const priceInUSD = convertToUSD(parseFloat(manualTrade.price))
      const res = await fetch('/api/quant/manual-trade', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          symbol: manualTrade.symbol.toUpperCase(),
          name: manualTrade.name || manualTrade.symbol.toUpperCase(),
          action: manualTrade.action,
          price: priceInUSD,
          quantity: manualTrade.quantity ? parseFloat(manualTrade.quantity) : 0,
          date: manualTrade.date || '',
          is_live: manualTrade.is_live
        })
      })
      if (res.ok) {
        setShowManualTrade(false)
        setManualTrade({ symbol: '', name: '', action: 'BUY', price: '', quantity: '', date: '', is_live: false })
        fetchBotData()
        fetchQuantSimulatedData()
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Erstellen')
      }
    } catch (err) {
      console.error('Failed to create manual trade:', err)
    } finally {
      setCreatingManualTrade(false)
    }
  }

  const searchManualTradeStock = async (query) => {
    setManualTradeSearch(query)
    if (query.length < 2) {
      setManualTradeResults([])
      return
    }
    setManualTradeSearching(true)
    try {
      const res = await fetch(`/api/search?q=${encodeURIComponent(query)}`)
      if (res.ok) {
        const data = await res.json()
        setManualTradeResults(data.slice(0, 8))
      }
    } catch (err) {
      console.error('Search error:', err)
    } finally {
      setManualTradeSearching(false)
    }
  }

  const selectManualTradeStock = (stock) => {
    setManualTrade({ ...manualTrade, symbol: stock.symbol, name: stock.name || stock.symbol })
    setManualTradeSearch('')
    setManualTradeResults([])
  }

  const handleCreateDitzManualTrade = async () => {
    if (!ditzManualTrade.symbol || !ditzManualTrade.price) {
      alert('Symbol und Preis sind Pflichtfelder')
      return
    }
    setCreatingManualTrade(true)
    try {
      const priceInUSD = convertToUSD(parseFloat(ditzManualTrade.price))
      const res = await fetch('/api/ditz/manual-trade', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          symbol: ditzManualTrade.symbol.toUpperCase(),
          name: ditzManualTrade.name || ditzManualTrade.symbol.toUpperCase(),
          action: ditzManualTrade.action,
          price: priceInUSD,
          quantity: ditzManualTrade.quantity ? parseFloat(ditzManualTrade.quantity) : 0,
          date: ditzManualTrade.date || '',
          is_live: ditzManualTrade.is_live
        })
      })
      if (res.ok) {
        setShowDitzManualTrade(false)
        setDitzManualTrade({ symbol: '', name: '', action: 'BUY', price: '', quantity: '', date: '', is_live: false })
        fetchBotData()
        fetchDitzSimulatedData()
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Erstellen')
      }
    } catch (err) {
      console.error('Failed to create Ditz manual trade:', err)
    } finally {
      setCreatingManualTrade(false)
    }
  }

  const handleCreateTraderManualTrade = async () => {
    if (!traderManualTrade.symbol || !traderManualTrade.price) {
      alert('Symbol und Preis sind Pflichtfelder')
      return
    }
    setCreatingManualTrade(true)
    try {
      const priceInUSD = convertToUSD(parseFloat(traderManualTrade.price))
      const res = await fetch('/api/trader/manual-trade', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: JSON.stringify({
          symbol: traderManualTrade.symbol.toUpperCase(),
          name: traderManualTrade.name || traderManualTrade.symbol.toUpperCase(),
          action: traderManualTrade.action,
          price: priceInUSD,
          quantity: traderManualTrade.quantity ? parseFloat(traderManualTrade.quantity) : 0,
          date: traderManualTrade.date || '',
          is_live: traderManualTrade.is_live
        })
      })
      if (res.ok) {
        setShowTraderManualTrade(false)
        setTraderManualTrade({ symbol: '', name: '', action: 'BUY', price: '', quantity: '', date: '', is_live: false })
        fetchBotData()
        fetchTraderSimulatedData()
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Erstellen')
      }
    } catch (err) {
      console.error('Failed to create Trader manual trade:', err)
    } finally {
      setCreatingManualTrade(false)
    }
  }

  const handleDeleteTrade = async (bot, tradeId, symbol, action, isDeleted) => {
    // Soft-delete toggle: no confirm needed for restore
    if (!isDeleted) {
      const msg = action === 'BUY'
        ? `Trade streichen? BUY + zugehöriger SELL für ${symbol} werden gestrichen. Position wird gelöscht.`
        : `SELL streichen? Position für ${symbol} wird wieder geöffnet.`
      if (!confirm(msg)) return
    }
    try {
      const res = await fetch(`/api/${bot}/trade/${tradeId}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
      })
      if (res.ok) {
        fetchBotData()
        if (bot === 'flipperbot') fetchFlipperUnreadCount()
        if (bot === 'lutz') fetchLutzUnreadCount()
        if (bot === 'quant') fetchQuantUnreadCount()
        if (bot === 'ditz') fetchDitzUnreadCount()
        if (bot === 'trader') fetchTraderUnreadCount()
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler')
      }
    } catch (err) {
      console.error('Failed to delete trade:', err)
    }
  }

  const fetchQuantUnreadCount = async () => {
    try {
      const res = await fetch('/api/quant/trades/unread-count', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      if (res.ok) {
        const data = await res.json()
        setQuantUnreadCount(data.count || 0)
        setQuantUnreadTrades(data.trades || [])
      }
    } catch (err) {
      console.error('Failed to fetch unread count:', err)
    }
  }

  const handleToggleTradeRead = async (tradeId) => {
    try {
      const res = await fetch(`/api/quant/trade/${tradeId}/read`, {
        method: 'PUT',
        headers: { 'Authorization': `Bearer ${token}` }
      })
      if (res.ok) {
        fetchBotData()
        fetchQuantUnreadCount()
      }
    } catch (err) {
      console.error('Failed to toggle trade read:', err)
    }
  }

  const handleMarkAllTradesRead = async () => {
    try {
      const res = await fetch('/api/quant/trades/read-all', {
        method: 'PUT',
        headers: { 'Authorization': `Bearer ${token}` }
      })
      if (res.ok) {
        fetchBotData()
        fetchQuantUnreadCount()
      }
    } catch (err) {
      console.error('Failed to mark all read:', err)
    }
  }

  const handleMarkAllTradesUnread = async () => {
    try {
      const res = await fetch('/api/quant/trades/unread-all', {
        method: 'PUT',
        headers: { 'Authorization': `Bearer ${token}` }
      })
      if (res.ok) {
        fetchBotData()
        fetchQuantUnreadCount()
      }
    } catch (err) {
      console.error('Failed to mark all unread:', err)
    }
  }

  const toggleFlipperTradeRead = async (tradeId) => {
    try {
      const res = await fetch(`/api/flipperbot/trade/${tradeId}/read`, { method: 'PUT', headers: { 'Authorization': `Bearer ${token}` } })
      if (res.ok) { fetchBotData(); fetchFlipperUnreadCount() }
    } catch (err) { /* ignore */ }
  }

  const markAllFlipperTradesRead = async () => {
    try {
      const res = await fetch('/api/flipperbot/trades/read-all', { method: 'PUT', headers: { 'Authorization': `Bearer ${token}` } })
      if (res.ok) { fetchBotData(); fetchFlipperUnreadCount() }
    } catch (err) { /* ignore */ }
  }

  const markAllFlipperTradesUnread = async () => {
    try {
      const res = await fetch('/api/flipperbot/trades/unread-all', { method: 'PUT', headers: { 'Authorization': `Bearer ${token}` } })
      if (res.ok) { fetchBotData(); fetchFlipperUnreadCount() }
    } catch (err) { /* ignore */ }
  }

  const toggleLutzTradeRead = async (tradeId) => {
    try {
      const res = await fetch(`/api/lutz/trade/${tradeId}/read`, { method: 'PUT', headers: { 'Authorization': `Bearer ${token}` } })
      if (res.ok) { fetchBotData(); fetchLutzUnreadCount() }
    } catch (err) { /* ignore */ }
  }

  const markAllLutzTradesRead = async () => {
    try {
      const res = await fetch('/api/lutz/trades/read-all', { method: 'PUT', headers: { 'Authorization': `Bearer ${token}` } })
      if (res.ok) { fetchBotData(); fetchLutzUnreadCount() }
    } catch (err) { /* ignore */ }
  }

  const markAllLutzTradesUnread = async () => {
    try {
      const res = await fetch('/api/lutz/trades/unread-all', { method: 'PUT', headers: { 'Authorization': `Bearer ${token}` } })
      if (res.ok) { fetchBotData(); fetchLutzUnreadCount() }
    } catch (err) { /* ignore */ }
  }

  const handleFixDB = async () => {
    if (!confirm('Datenbank-Fix durchführen? Dies behebt kaputte Trades und Positionen.')) return
    setFixingDB(true)
    setFixResult(null)
    try {
      const res = await fetch('/api/flipperbot/fix-db', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      if (res.ok) {
        setFixResult({ success: true, data })
        fetchBotData() // Reload data
      } else {
        setFixResult({ success: false, error: data.error || 'Fehler beim Fix' })
      }
    } catch (err) {
      setFixResult({ success: false, error: err.message })
    } finally {
      setFixingDB(false)
    }
  }

  const getBotApiName = () => {
    if (botTab === 'flipper') return 'flipperbot'
    if (botTab === 'lutz') return 'lutz'
    if (botTab === 'ditz') return 'ditz'
    if (botTab === 'trader') return 'trader'
    return 'quant'
  }

  // Gruppiert SELL-Trades mit dem dazugehörigen BUY direkt darunter
  const getGroupedTrades = (trades) => {
    const result = []
    const pairedIds = new Set()

    for (let i = 0; i < trades.length; i++) {
      if (pairedIds.has(trades[i].id)) continue
      result.push(trades[i])

      if (trades[i].action === 'SELL') {
        for (let j = i + 1; j < trades.length; j++) {
          if (trades[j].action === 'BUY' && trades[j].symbol === trades[i].symbol && !pairedIds.has(trades[j].id)) {
            result.push(trades[j])
            pairedIds.add(trades[j].id)
            break
          }
        }
      }
    }
    return result
  }

  const handleBackfill = async (bot = 'flipperbot') => {
    if (!backfillDate) {
      alert('Bitte ein Datum auswählen')
      return
    }
    const botName = bot === 'flipperbot' ? 'FlipperBot' : bot === 'lutz' ? 'Lutz' : bot === 'ditz' ? 'Ditz' : bot === 'trader' ? 'Trader' : 'Quant'
    const modeInfo = bot === 'flipperbot' ? 'Defensiv' : bot === 'lutz' ? 'Aggressiv' : bot === 'ditz' ? 'Ditz' : bot === 'trader' ? 'Trader' : 'Quant'
    if (!confirm(`${botName} Backfill ab ${backfillDate} bis heute durchführen? Historische Trades für ${modeInfo}-Aktien werden erstellt.`)) return
    setBackfilling(true)
    setBackfillResult(null)
    setBackfillProgress({ current: 0, total: 0, symbol: '', message: 'Starte Backfill...' })
    try {
      const res = await fetch(`/api/${bot}/backfill`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ until_date: backfillDate })
      })
      if (!res.ok) {
        const text = await res.text()
        let error = 'Fehler beim Backfill'
        try { error = JSON.parse(text).error || error } catch {}
        setBackfillResult({ success: false, error })
        setBackfillProgress(null)
        return
      }
      const reader = res.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop()
        for (const line of lines) {
          if (!line.trim()) continue
          try {
            const event = JSON.parse(line)
            if (event.type === 'progress') {
              setBackfillProgress({ current: event.current, total: event.total, symbol: event.symbol, message: event.message })
            } else if (event.type === 'done') {
              setBackfillResult({ success: true, data: event })
              setBackfillProgress(null)
              fetchBotData()
            }
          } catch {}
        }
      }
      if (buffer.trim()) {
        try {
          const event = JSON.parse(buffer)
          if (event.type === 'done') {
            setBackfillResult({ success: true, data: event })
            setBackfillProgress(null)
            fetchBotData()
          }
        } catch {}
      }
    } catch (err) {
      setBackfillResult({ success: false, error: err.message })
      setBackfillProgress(null)
    } finally {
      setBackfilling(false)
    }
  }

  const handleBackfillAll = async () => {
    if (!backfillDate) {
      alert('Bitte ein Datum auswählen')
      return
    }
    const allBots = [
      { api: 'flipperbot', name: 'FlipperBot' },
      { api: 'lutz', name: 'Lutz' },
      { api: 'quant', name: 'Quant' },
      { api: 'ditz', name: 'Ditz' },
      { api: 'trader', name: 'Trader' },
    ]
    if (!confirm(`Backfill für ALLE 5 Bots ab ${backfillDate} bis heute durchführen?`)) return
    setBackfilling(true)
    setBackfillResult(null)

    let totalTrades = 0
    let totalPositions = 0
    const errors = []

    for (let i = 0; i < allBots.length; i++) {
      const bot = allBots[i]
      setBackfillProgress({ current: 0, total: 0, symbol: '', message: `${bot.name} (${i + 1}/5) wird gestartet...` })
      try {
        const res = await fetch(`/api/${bot.api}/backfill`, {
          method: 'POST',
          headers: { 'Authorization': `Bearer ${token}`, 'Content-Type': 'application/json' },
          body: JSON.stringify({ until_date: backfillDate })
        })
        if (!res.ok) {
          const text = await res.text()
          let error = 'Fehler'
          try { error = JSON.parse(text).error || error } catch {}
          errors.push(`${bot.name}: ${error}`)
          continue
        }
        const reader = res.body.getReader()
        const decoder = new TextDecoder()
        let buffer = ''
        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          buffer += decoder.decode(value, { stream: true })
          const lines = buffer.split('\n')
          buffer = lines.pop()
          for (const line of lines) {
            if (!line.trim()) continue
            try {
              const event = JSON.parse(line)
              if (event.type === 'progress') {
                setBackfillProgress({ current: event.current, total: event.total, symbol: event.symbol, message: `${bot.name} (${i + 1}/5): ${event.message || event.symbol}` })
              } else if (event.type === 'done') {
                totalTrades += event.trades_created || 0
                totalPositions += event.positions_created || 0
              }
            } catch {}
          }
        }
        if (buffer.trim()) {
          try {
            const event = JSON.parse(buffer)
            if (event.type === 'done') {
              totalTrades += event.trades_created || 0
              totalPositions += event.positions_created || 0
            }
          } catch {}
        }
      } catch (err) {
        errors.push(`${bot.name}: ${err.message}`)
      }
    }

    setBackfillProgress(null)
    if (errors.length > 0) {
      setBackfillResult({ success: false, error: errors.join('; ') })
    } else {
      setBackfillResult({ success: true, data: { trades_created: totalTrades, positions_created: totalPositions } })
    }
    fetchBotData()
    setBackfilling(false)
  }

  const handleAcceptTrade = async (bot, tradeId) => {
    setAcceptingTrade(tradeId)
    try {
      const res = await fetch(`/api/${bot}/trade/${tradeId}/accept`, {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` }
      })
      if (res.ok) {
        fetchBotData()
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Akzeptieren')
      }
    } catch (err) {
      console.error('Failed to accept trade:', err)
      alert('Fehler beim Akzeptieren')
    } finally {
      setAcceptingTrade(null)
    }
  }

  const handleAcceptAllTrades = async (bot) => {
    const trades = bot === 'flipperbot' ? flipperPendingTrades : bot === 'lutz' ? lutzPendingTrades : bot === 'trader' ? traderPendingTrades : quantPendingTrades
    if (trades.length === 0) return
    if (!confirm(`Alle ${trades.length} ausstehenden Trades akzeptieren?`)) return

    for (const trade of trades) {
      await handleAcceptTrade(bot, trade.id)
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

  const handleSaveUsername = async (userId) => {
    const trimmed = editingUsername.trim()
    if (!trimmed) return
    try {
      const res = await fetch(`/api/admin/users/${userId}`, {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ username: trimmed })
      })
      if (res.ok) {
        setEditingUserId(null)
        fetchUsers()
      } else {
        const data = await res.json()
        alert(data.error || 'Fehler beim Speichern')
      }
    } catch (err) {
      console.error('Failed to update username:', err)
    }
  }

  const formatPercent = (val) => {
    const v = val || 0
    return `${v >= 0 ? '+' : ''}${v.toFixed(2)}%`
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

  const formatDateShort = (dateStr) => {
    if (!dateStr) return '-'
    return new Date(dateStr).toLocaleDateString('de-DE')
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

  const fetchTrackedDiff = async () => {
    setLoadingDiff(true)
    try {
      const res = await fetch('/api/admin/tracked-diff', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      setTrackedDiff(data)
    } catch (err) {
      console.error('Failed to fetch tracked diff:', err)
    } finally {
      setLoadingDiff(false)
    }
  }

  const handleDeleteTracked = async (symbol) => {
    if (!confirm(`"${symbol}" aus den getrackten Aktien löschen?`)) return
    try {
      const res = await fetch(`/api/admin/tracked/${encodeURIComponent(symbol)}`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${token}` }
      })
      if (res.ok) {
        fetchTrackedDiff()
        fetchStats()
      }
    } catch (err) {
      console.error('Failed to delete tracked stock:', err)
    }
  }

  const handleOpenTrackedDiff = () => {
    setShowTrackedDiff(true)
    fetchTrackedDiff()
  }

  const handleExportWatchlist = async () => {
    try {
      const res = await fetch('/api/admin/export-watchlist', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      const date = new Date().toISOString().split('T')[0]
      a.download = `watchlist_export_${date}.json`
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      console.error('Export failed:', err)
      alert('Export fehlgeschlagen: ' + err.message)
    }
  }

  const handleImportWatchlist = async (e) => {
    const file = e.target.files[0]
    if (!file) return
    e.target.value = ''

    try {
      const text = await file.text()
      const entries = JSON.parse(text)
      if (!Array.isArray(entries)) {
        alert('Ungültiges Format: JSON-Array erwartet')
        return
      }
      if (!confirm(`${entries.length} Aktien importieren? Bestehende Ticker bekommen die Kategorie aus dem Import. Neue Aktien werden angelegt und verarbeitet.`)) {
        return
      }

      setImporting(true)
      setImportProgress({ current: 0, total: 0, status: 'Importiere...' })

      // Phase 1: Import to backend
      const res = await fetch('/api/admin/import-watchlist', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(entries)
      })
      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Import fehlgeschlagen')
      }

      // Phase 2: Process new stocks
      const newStocks = data.new_stocks || []
      if (newStocks.length > 0) {
        setImportProgress({ current: 0, total: newStocks.length, status: 'Verarbeite neue Aktien...' })
        let successCount = 0
        let errorCount = 0

        for (let i = 0; i < newStocks.length; i++) {
          const stock = newStocks[i]
          setImportProgress({
            current: i,
            total: newStocks.length,
            status: `Verarbeite ${stock.symbol}...`,
            currentStock: `${stock.symbol} - ${stock.name}`
          })

          try {
            await processStock(stock.symbol, stock.name)
            successCount++
          } catch (err) {
            errorCount++
            console.warn(`Failed to process ${stock.symbol}:`, err)
          }

          await new Promise(r => setTimeout(r, 1000))
        }

        setImportProgress({
          current: newStocks.length,
          total: newStocks.length,
          status: `Import fertig! ${data.updated} aktualisiert, ${data.created} neu angelegt (${successCount} verarbeitet, ${errorCount} fehlgeschlagen)`,
          currentStock: null
        })
      } else {
        setImportProgress({
          current: 0,
          total: 0,
          status: `Import fertig! ${data.updated} aktualisiert, ${data.created} neu angelegt.`
        })
      }

      fetchStats()
    } catch (err) {
      console.error('Import failed:', err)
      setImportProgress({ current: 0, total: 0, status: 'Fehler: ' + err.message })
    } finally {
      setImporting(false)
    }
  }

  const handleUpdateAllStocks = async () => {
    if (!confirm(`Alle Watchlist-Aktien aktualisieren?${forceUpdate ? ' (FORCE — alle werden aktualisiert)' : ' Bereits heute aktualisierte werden übersprungen.'}`)) {
      return
    }

    setUpdatingStocks(true)
    setUpdateProgress({ current: 0, total: 0, status: 'Lade Aktien-Liste...' })

    let total = 0
    let successCount = 0
    let errorCount = 0
    let skippedCount = 0

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
      const lastUpdates = data.last_updates || {}
      total = stocks.length
      const todayStr = new Date().toISOString().slice(0, 10)

      setUpdateProgress({ current: 0, total, status: 'Verarbeite Aktien...' })

      // Process each stock
      for (let i = 0; i < stocks.length; i++) {
        const stock = stocks[i]

        // Skip if already updated today (unless force)
        if (!forceUpdate && lastUpdates[stock.symbol]) {
          const updatedDate = lastUpdates[stock.symbol].slice(0, 10)
          if (updatedDate === todayStr) {
            skippedCount++
            setUpdateProgress({
              current: i + 1,
              total,
              status: `${stock.symbol} übersprungen (heute bereits aktualisiert)`,
              currentStock: `${stock.symbol} — skipped`
            })
            continue
          }
        }

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
        status: `Fertig! ${successCount} aktualisiert${skippedCount > 0 ? `, ${skippedCount} übersprungen` : ''}${errorCount > 0 ? `, ${errorCount} fehlgeschlagen` : ''}`,
        currentStock: null
      })
    } catch (err) {
      console.error('Failed to update stocks:', err)
      setUpdateProgress({ status: 'Fehler: ' + err.message, current: 0, total: 0 })
    } finally {
      // Always record update and refresh, even if loop was interrupted
      if (total > 0) {
        try {
          await fetch('/api/admin/record-full-update', {
            method: 'POST',
            headers: {
              'Authorization': `Bearer ${token}`,
              'Content-Type': 'application/json'
            },
            body: JSON.stringify({
              stocks_count: total,
              success: successCount,
              failed: errorCount
            })
          })
        } catch (err) {
          console.error('Failed to record full update:', err)
        }
        fetchLastFullUpdate()
      }
      fetchStats()
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
            { key: 'bots', label: 'Bots' },
            { key: 'quant', label: 'Quant' },
            { key: 'ditz', label: 'Ditz' },
            { key: 'trader', label: 'Trader' },
            { key: 'botfilter', label: 'Bot Filter' },
            { key: 'allowlist', label: 'Aktien Listen' }
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
                        Aktualisiere alle Aktien der Watchlist für alle Modi (Defensiv, Aggressiv, Quant, Ditz, Trader).
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
                    <label className="flex items-center gap-2 cursor-pointer select-none">
                      <input type="checkbox" checked={forceUpdate} onChange={e => setForceUpdate(e.target.checked)}
                        className="w-4 h-4 rounded border-dark-500 bg-dark-700 text-orange-500 focus:ring-orange-500/50" />
                      <span className={`text-sm font-medium ${forceUpdate ? 'text-orange-400' : 'text-gray-400'}`}>Force?</span>
                    </label>
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
                  {/* Last Full Update Info */}
                  {lastFullUpdate && lastFullUpdate.updated_at && (
                    <div className="mt-4 p-3 bg-dark-800 rounded-lg border border-dark-600">
                      <div className="flex items-center gap-2 text-sm">
                        <svg className="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <span className="text-gray-400">Letzte vollständige Aktualisierung:</span>
                        <span className="text-white font-medium">
                          {new Date(lastFullUpdate.updated_at).toLocaleDateString('de-DE', {
                            day: '2-digit',
                            month: '2-digit',
                            year: 'numeric',
                            hour: '2-digit',
                            minute: '2-digit'
                          })}
                        </span>
                        <span className="text-gray-500">von</span>
                        <span className={`px-2 py-0.5 text-xs font-medium rounded ${
                          lastFullUpdate.triggered_by === 'system'
                            ? 'bg-purple-500/20 text-purple-400'
                            : 'bg-accent-500/20 text-accent-400'
                        }`}>
                          {lastFullUpdate.triggered_by === 'system' ? 'System (Auto)' : lastFullUpdate.triggered_by}
                        </span>
                      </div>
                      {lastFullUpdate.stocks_count > 0 && (
                        <div className="flex items-center gap-4 mt-2 text-xs text-gray-500">
                          <span>{lastFullUpdate.stocks_count} Aktien</span>
                          <span className="text-green-400">{lastFullUpdate.success} erfolgreich</span>
                          {lastFullUpdate.failed > 0 && (
                            <span className="text-red-400">{lastFullUpdate.failed} fehlgeschlagen</span>
                          )}
                        </div>
                      )}
                    </div>
                  )}

                </div>

                {/* Import / Export */}
                <div className={`rounded-xl border p-4 ${
                  isAggressive
                    ? 'bg-orange-500/10 border-orange-500/30'
                    : 'bg-blue-500/10 border-blue-500/30'
                }`}>
                  <h2 className="text-lg font-semibold text-white">Import / Export</h2>
                  <p className="text-sm text-gray-400 mt-1">
                    Watchlist als JSON exportieren oder importieren. Neue Aktien werden automatisch verarbeitet.
                  </p>
                  <div className="flex gap-3 mt-4">
                    <button
                      onClick={handleExportWatchlist}
                      disabled={importing || updatingStocks}
                      className={`flex items-center gap-2 px-4 py-2.5 rounded-lg font-medium transition-colors ${
                        isAggressive
                          ? 'bg-orange-500 text-white hover:bg-orange-400 disabled:bg-orange-500/50'
                          : 'bg-blue-500 text-white hover:bg-blue-400 disabled:bg-blue-500/50'
                      } disabled:cursor-not-allowed`}
                    >
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                      </svg>
                      Exportieren
                    </button>
                    <label className={`flex items-center gap-2 px-4 py-2.5 rounded-lg font-medium transition-colors cursor-pointer ${
                      importing || updatingStocks
                        ? isAggressive
                          ? 'bg-orange-500/50 text-white cursor-not-allowed'
                          : 'bg-blue-500/50 text-white cursor-not-allowed'
                        : isAggressive
                          ? 'bg-orange-500 text-white hover:bg-orange-400'
                          : 'bg-blue-500 text-white hover:bg-blue-400'
                    }`}>
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                      </svg>
                      Importieren
                      <input
                        type="file"
                        accept=".json"
                        className="hidden"
                        onChange={handleImportWatchlist}
                        disabled={importing || updatingStocks}
                      />
                    </label>
                  </div>
                  {importProgress && (
                    <div className="mt-4 p-3 bg-dark-800 rounded-lg">
                      <div className="flex items-center justify-between text-sm mb-2">
                        <span className="text-gray-400">{importProgress.status}</span>
                        {importProgress.total > 0 && (
                          <span className="text-white font-medium">
                            {importProgress.current} / {importProgress.total}
                          </span>
                        )}
                      </div>
                      {importProgress.total > 0 && (
                        <div className="w-full h-2 bg-dark-700 rounded-full overflow-hidden">
                          <div
                            className={`h-full transition-all duration-300 ${
                              isAggressive ? 'bg-orange-500' : 'bg-blue-500'
                            }`}
                            style={{ width: `${(importProgress.current / importProgress.total) * 100}%` }}
                          />
                        </div>
                      )}
                      {importProgress.currentStock && (
                        <div className="text-xs text-gray-500 mt-2">
                          Aktuell: {importProgress.currentStock}
                        </div>
                      )}
                    </div>
                  )}
                </div>

                <div className={`rounded-xl border p-4 ${
                  isAggressive
                    ? 'bg-orange-500/10 border-orange-500/30'
                    : 'bg-blue-500/10 border-blue-500/30'
                }`}>
                  {/* Scheduler Time Setting */}
                  <div className="mt-4 p-4 bg-dark-800 rounded-lg border border-dark-600">
                    <div className="flex items-center justify-between">
                      <div>
                        <h3 className="text-sm font-medium text-white flex items-center gap-2">
                          <svg className="w-4 h-4 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                          </svg>
                          Täglicher Auto-Refresh
                        </h3>
                        <p className="text-xs text-gray-500 mt-1">Uhrzeit für den automatischen System-Refresh (MEZ/MESZ)</p>
                      </div>
                      <div className="flex items-center gap-3">
                        {schedulerCountdown && (
                          <div className="flex items-center gap-2 px-3 py-1.5 bg-purple-500/10 rounded-lg border border-purple-500/20">
                            <span className="w-1.5 h-1.5 bg-purple-400 rounded-full animate-pulse"></span>
                            <span className="text-sm font-mono text-purple-300">{schedulerCountdown}</span>
                          </div>
                        )}
                        <input
                          type="time"
                          value={schedulerTime}
                          onChange={(e) => setSchedulerTime(e.target.value)}
                          className="bg-dark-700 border border-dark-500 text-white rounded-lg px-3 py-1.5 text-sm focus:ring-2 focus:ring-accent-500 focus:border-transparent"
                        />
                        <button
                          onClick={saveSchedulerTime}
                          disabled={savingSchedulerTime}
                          className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                            savingSchedulerTime
                              ? 'bg-dark-600 text-gray-500 cursor-not-allowed'
                              : isAggressive
                                ? 'bg-red-500/20 text-red-400 hover:bg-red-500/30'
                                : 'bg-accent-500/20 text-accent-400 hover:bg-accent-500/30'
                          }`}
                        >
                          {savingSchedulerTime ? 'Speichert...' : 'Speichern'}
                        </button>
                      </div>
                    </div>
                  </div>
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
                  <button
                    onClick={handleOpenTrackedDiff}
                    className="bg-dark-800 rounded-xl border border-dark-600 p-4 text-left hover:border-accent-500 transition-colors w-full"
                  >
                    <div className="text-xs text-gray-500 mb-1">Getrackte Aktien</div>
                    <div className="text-3xl font-bold text-white">{stats.tracked_stocks}</div>
                    <div className="text-xs text-gray-500 mt-1">Klicken für Details</div>
                  </button>
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
                                  {(editingUserId === user.id ? editingUsername : user.username)?.charAt(0).toUpperCase()}
                                </span>
                              </div>
                              {editingUserId === user.id ? (
                                <div className="flex items-center gap-1">
                                  <input
                                    type="text"
                                    value={editingUsername}
                                    onChange={(e) => setEditingUsername(e.target.value)}
                                    onKeyDown={(e) => {
                                      if (e.key === 'Enter') handleSaveUsername(user.id)
                                      if (e.key === 'Escape') setEditingUserId(null)
                                    }}
                                    autoFocus
                                    className="bg-dark-700 border border-accent-500 rounded px-2 py-1 text-white text-sm w-32 focus:outline-none"
                                  />
                                  <button onClick={() => handleSaveUsername(user.id)} className="p-1 text-green-400 hover:text-green-300" title="Speichern">
                                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" /></svg>
                                  </button>
                                  <button onClick={() => setEditingUserId(null)} className="p-1 text-gray-400 hover:text-gray-300" title="Abbrechen">
                                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" /></svg>
                                  </button>
                                </div>
                              ) : (
                                <span
                                  className="font-medium text-white cursor-pointer hover:text-accent-400 transition-colors"
                                  onClick={() => { setEditingUserId(user.id); setEditingUsername(user.username) }}
                                  title="Klicken zum Bearbeiten"
                                >
                                  {user.username}
                                </span>
                              )}
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
                {/* Quant Notifications Banner - ganz oben */}
                {quantUnreadCount > 0 && (
                  <div className="bg-violet-500/10 border border-violet-500/30 rounded-xl p-4 flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <div className="w-8 h-8 bg-violet-500/20 rounded-full flex items-center justify-center">
                        <svg className="w-4 h-4 text-violet-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
                        </svg>
                      </div>
                      <div>
                        <p className="text-violet-300 font-medium text-sm">
                          {quantUnreadCount} neue{quantUnreadCount === 1 ? 'r' : ''} Quant Trade{quantUnreadCount === 1 ? '' : 's'}
                        </p>
                        <p className="text-xs text-gray-400">
                          {quantUnreadTrades.slice(0, 3).map(t =>
                            `${t.action} ${t.symbol}`
                          ).join(', ')}
                          {quantUnreadCount > 3 && ` und ${quantUnreadCount - 3} weitere`}
                        </p>
                      </div>
                    </div>
                    <button
                      onClick={handleMarkAllTradesRead}
                      className="px-3 py-1.5 bg-violet-500/20 text-violet-400 rounded-lg text-xs font-medium hover:bg-violet-500/30"
                    >
                      Alle als gelesen markieren
                    </button>
                  </div>
                )}

                {/* Bot Selector */}
                <div className="flex gap-2">
                  <button
                    onClick={() => setBotTab('flipper')}
                    className={`px-4 py-2 rounded-lg font-medium transition-colors relative ${
                      botTab === 'flipper'
                        ? 'bg-blue-500 text-white'
                        : 'bg-dark-700 text-gray-400 hover:text-white'
                    }`}
                  >
                    FlipperBot (Defensiv)
                    {flipperUnreadCount > 0 && (
                      <span className="absolute -top-1 -right-1 w-5 h-5 bg-blue-500 text-white text-xs rounded-full flex items-center justify-center font-bold">
                        {flipperUnreadCount > 9 ? '9+' : flipperUnreadCount}
                      </span>
                    )}
                  </button>
                  <button
                    onClick={() => setBotTab('lutz')}
                    className={`px-4 py-2 rounded-lg font-medium transition-colors relative ${
                      botTab === 'lutz'
                        ? 'bg-orange-500 text-white'
                        : 'bg-dark-700 text-gray-400 hover:text-white'
                    }`}
                  >
                    Lutz (Aggressiv)
                    {lutzUnreadCount > 0 && (
                      <span className="absolute -top-1 -right-1 w-5 h-5 bg-orange-500 text-white text-xs rounded-full flex items-center justify-center font-bold">
                        {lutzUnreadCount > 9 ? '9+' : lutzUnreadCount}
                      </span>
                    )}
                  </button>
                  <button
                    onClick={() => setBotTab('quant')}
                    className={`px-4 py-2 rounded-lg font-medium transition-colors relative ${
                      botTab === 'quant'
                        ? 'bg-violet-500 text-white'
                        : 'bg-dark-700 text-gray-400 hover:text-white'
                    }`}
                  >
                    Quant
                    {quantUnreadCount > 0 && (
                      <span className="absolute -top-1 -right-1 w-5 h-5 bg-violet-500 text-white text-xs rounded-full flex items-center justify-center font-bold">
                        {quantUnreadCount > 9 ? '9+' : quantUnreadCount}
                      </span>
                    )}
                  </button>
                  <button
                    onClick={() => setBotTab('ditz')}
                    className={`px-4 py-2 rounded-lg font-medium transition-colors relative ${
                      botTab === 'ditz'
                        ? 'bg-cyan-500 text-white'
                        : 'bg-dark-700 text-gray-400 hover:text-white'
                    }`}
                  >
                    Ditz
                    {ditzUnreadCount > 0 && (
                      <span className="absolute -top-1 -right-1 w-5 h-5 bg-cyan-500 text-white text-xs rounded-full flex items-center justify-center font-bold">
                        {ditzUnreadCount > 9 ? '9+' : ditzUnreadCount}
                      </span>
                    )}
                  </button>
                  <button
                    onClick={() => setBotTab('trader')}
                    className={`px-4 py-2 rounded-lg font-medium transition-colors relative ${
                      botTab === 'trader'
                        ? 'bg-emerald-500 text-white'
                        : 'bg-dark-700 text-gray-400 hover:text-white'
                    }`}
                  >
                    Trader
                    {traderUnreadCount > 0 && (
                      <span className="absolute -top-1 -right-1 w-5 h-5 bg-emerald-500 text-white text-xs rounded-full flex items-center justify-center font-bold">
                        {traderUnreadCount > 9 ? '9+' : traderUnreadCount}
                      </span>
                    )}
                  </button>
                </div>

                {/* Info */}
                <div className="p-4 bg-blue-500/10 border border-blue-500/30 rounded-xl text-sm text-blue-300">
                  <p>Trades bearbeiten um echte Werte einzutragen. Positionen werden automatisch aktualisiert.</p>
                </div>

                {/* BXtrender Configuration - collapsible */}
                <div className={`rounded-xl border ${
                  botTab === 'flipper'
                    ? 'bg-blue-500/10 border-blue-500/30'
                    : 'bg-orange-500/10 border-orange-500/30'
                }`}>
                  <button
                    onClick={() => setShowBxConfig(!showBxConfig)}
                    className={`w-full p-4 flex items-center justify-between text-sm font-medium ${
                      botTab === 'flipper' ? 'text-blue-300' : 'text-orange-300'
                    }`}
                  >
                    B-Xtrender Konfiguration ({botTab === 'flipper' ? 'Defensiv' : 'Aggressiv'})
                    <svg className={`w-4 h-4 transition-transform ${showBxConfig ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                  </button>
                  {showBxConfig && (
                  <div className="px-4 pb-4">
                  <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mb-4">
                    {[
                      { key: 'short_l1', label: 'Short L1', desc: 'EMA kurz' },
                      { key: 'short_l2', label: 'Short L2', desc: 'EMA lang' },
                      { key: 'short_l3', label: 'Short L3', desc: 'RSI Periode' },
                      { key: 'long_l1', label: 'Long L1', desc: 'EMA Periode' },
                      { key: 'long_l2', label: 'Long L2', desc: 'RSI Periode' }
                    ].map(({ key, label, desc }) => (
                      <div key={key} className="bg-dark-800 rounded-lg p-3">
                        <label className="text-xs text-gray-500 block mb-1">{label}</label>
                        <input
                          type="number"
                          min="1"
                          max="200"
                          value={bxtrenderConfig[botTab === 'flipper' ? 'defensive' : 'aggressive']?.[key] || ''}
                          onChange={(e) => updateConfigValue(
                            botTab === 'flipper' ? 'defensive' : 'aggressive',
                            key,
                            e.target.value
                          )}
                          className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1 text-white text-sm"
                        />
                        <span className="text-[10px] text-gray-600">{desc}</span>
                      </div>
                    ))}
                  </div>
                  <div className="grid grid-cols-2 gap-3 mb-4">
                    <div className="bg-dark-800 rounded-lg p-3">
                      <label className="text-xs text-gray-500 block mb-1">Stop Loss %</label>
                      <input
                        type="number"
                        min="1"
                        max="50"
                        step="0.5"
                        value={bxtrenderConfig[botTab === 'flipper' ? 'defensive' : 'aggressive']?.tsl_percent || 20}
                        onChange={(e) => updateConfigValue(
                          botTab === 'flipper' ? 'defensive' : 'aggressive',
                          'tsl_percent',
                          e.target.value
                        )}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1 text-white text-sm"
                      />
                      <span className="text-[10px] text-gray-600">Trailing Stop Loss</span>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-3 flex items-center gap-3">
                      <label className="text-xs text-gray-500">Stop Loss aktiv</label>
                      <button
                        onClick={() => updateConfigValue(
                          botTab === 'flipper' ? 'defensive' : 'aggressive',
                          'tsl_enabled',
                          !bxtrenderConfig[botTab === 'flipper' ? 'defensive' : 'aggressive']?.tsl_enabled
                        )}
                        className={`relative w-10 h-5 rounded-full transition-colors ${
                          bxtrenderConfig[botTab === 'flipper' ? 'defensive' : 'aggressive']?.tsl_enabled !== false
                            ? 'bg-green-500' : 'bg-gray-600'
                        }`}
                      >
                        <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${
                          bxtrenderConfig[botTab === 'flipper' ? 'defensive' : 'aggressive']?.tsl_enabled !== false
                            ? 'translate-x-5' : ''
                        }`} />
                      </button>
                    </div>
                  </div>
                  <div className="flex items-center justify-between">
                    <p className="text-xs text-gray-500">
                      Short: RSI(EMA(close, L1) - EMA(close, L2), L3) - 50<br/>
                      Long: RSI(EMA(close, L1), L2) - 50
                    </p>
                    <button
                      onClick={() => handleSaveBXtrenderConfig(botTab === 'flipper' ? 'defensive' : 'aggressive')}
                      disabled={savingConfig}
                      className={`px-4 py-2 rounded-lg font-medium text-white disabled:opacity-50 ${
                        botTab === 'flipper' ? 'bg-blue-500 hover:bg-blue-400' : 'bg-orange-500 hover:bg-orange-400'
                      }`}
                    >
                      {savingConfig ? 'Speichern...' : 'Speichern'}
                    </button>
                  </div>
                  </div>
                  )}
                </div>

                {/* FlipperBot Simulated Portfolio Section */}
                {botTab === 'flipper' && (
                  <div className="p-4 bg-blue-500/10 border border-blue-500/30 rounded-xl">
                    <div className="flex items-center justify-between mb-4">
                      <div>
                        <h3 className="text-lg font-semibold text-blue-300 flex items-center gap-2">
                          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                          </svg>
                          SIM Portfolio (Defensiv)
                        </h3>
                        {lastFlipperRefresh && (
                          <p className="text-xs text-gray-500 mt-1">
                            Letzter Refresh: {new Date(lastFlipperRefresh.updated_at).toLocaleString('de-DE')}
                            {lastFlipperRefresh.triggered_by && ` (von: ${lastFlipperRefresh.triggered_by})`}
                          </p>
                        )}
                      </div>
                      <button
                        onClick={async () => {
                          setFlipperRefreshing(true)
                          try {
                            await fetch('/api/flipperbot/update', { headers: { 'Authorization': `Bearer ${token}` } })
                            await fetchFlipperSimulatedData()
                            await fetchLastFlipperRefresh()
                            fetchBotData()
                          } catch (err) { console.error(err) }
                          setFlipperRefreshing(false)
                        }}
                        disabled={flipperRefreshing}
                        className="px-3 py-1.5 bg-blue-500/20 text-blue-300 rounded-lg hover:bg-blue-500/30 text-sm disabled:opacity-50"
                      >
                        {flipperRefreshing ? 'Aktualisiere...' : 'Refresh'}
                      </button>
                    </div>

                    {flipperPrivatePerformance && (
                      <div className="mb-4">
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mb-4">
                          <div className="bg-gradient-to-r from-blue-500/20 to-indigo-500/20 rounded-lg p-3 md:p-4 border border-blue-500/30">
                            <div className="text-xs text-gray-400 mb-1">Gesamt Rendite</div>
                            <div className={`text-xl md:text-2xl font-bold ${flipperPrivatePerformance.overall_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPercent(flipperPrivatePerformance.overall_return_pct)}
                            </div>
                            <div className={`text-xs mt-1 ${(flipperPrivatePerformance.total_gain || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(flipperPrivatePerformance.total_gain || 0)} Gewinn
                            </div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Investiert</div>
                            <div className="text-lg md:text-xl font-bold text-white">{formatPrice(flipperPrivatePerformance.invested_in_positions || 0)}</div>
                            <div className="text-xs text-gray-500 mt-1">Aktuell: {formatPrice(flipperPrivatePerformance.current_value || 0)}</div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Unrealisiert</div>
                            <div className={`text-lg md:text-xl font-bold ${(flipperPrivatePerformance.unrealized_gain || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(flipperPrivatePerformance.unrealized_gain || 0)}
                            </div>
                            <div className={`text-xs mt-1 ${(flipperPrivatePerformance.total_return_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPercent(flipperPrivatePerformance.total_return_pct)}
                            </div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Realisiert</div>
                            <div className={`text-lg md:text-xl font-bold ${(flipperPrivatePerformance.realized_profit || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(flipperPrivatePerformance.realized_profit || 0)}
                            </div>
                            <div className="text-xs text-gray-500 mt-1">{flipperPrivatePerformance.total_trades || 0} Trades</div>
                          </div>
                        </div>
                        <div className="grid grid-cols-2 md:grid-cols-5 gap-3 md:gap-4">
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Win Rate</div>
                            <div className={`text-base font-bold ${(flipperEnrichedPerf.win_rate || 0) >= 50 ? 'text-green-400' : 'text-red-400'}`}>{flipperEnrichedPerf.win_rate?.toFixed(1) || 0}%</div>
                            <div className="text-xs text-gray-500 mt-1">{flipperEnrichedPerf.wins || 0}W / {flipperEnrichedPerf.losses || 0}L</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Risiko-Rendite</div>
                            <div className={`text-base font-bold ${(flipperEnrichedPerf.risk_reward || 0) >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                              {(flipperEnrichedPerf.risk_reward || 0).toFixed(2)}
                            </div>
                            <div className="text-xs text-gray-500 mt-1">Ø Gewinn / Ø Verlust</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Ø Gewinn-Trade</div>
                            <div className="text-base font-bold text-green-400">+{(flipperEnrichedPerf.avg_win_pct || 0).toFixed(2)}%</div>
                            <div className="text-xs text-gray-500 mt-1">{flipperEnrichedPerf.wins || 0} Trades</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Ø Verlust-Trade</div>
                            <div className="text-base font-bold text-red-400">-{(flipperEnrichedPerf.avg_loss_pct || 0).toFixed(2)}%</div>
                            <div className="text-xs text-gray-500 mt-1">{flipperEnrichedPerf.losses || 0} Trades</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Offene Positionen</div>
                            <div className="text-base font-bold text-white">{flipperEnrichedPerf.open_positions || 0}</div>
                            <div className="text-xs mt-1">
                              <span className="text-gray-500">von {flipperEnrichedPerf.total_buys || 0} Käufen</span>
                              {flipperPrivatePortfolio?.positions?.filter(p => p.is_live).length > 0 && (
                                <span className="ml-1 text-green-400">{flipperPrivatePortfolio.positions.filter(p => p.is_live).length} Live</span>
                              )}
                            </div>
                          </div>
                        </div>
                      </div>
                    )}

                    {flipperPrivatePortfolio?.positions?.length > 0 ? (
                      <div className="overflow-x-auto">
                        <table className="w-full">
                          <thead>
                            <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                              {[
                                { key: 'symbol', label: 'Symbol', align: 'left' },
                                { key: 'name', label: 'Name', align: 'left' },
                                { key: 'quantity', label: 'Menge (Invest.)', align: 'right' },
                                { key: 'avg_price', label: 'Kaufpreis', align: 'right' },
                                { key: 'current_price', label: 'Aktuell', align: 'right' },
                                { key: 'total_return_pct', label: 'Rendite (Wert)', align: 'right' },
                                { key: 'stop_loss_price', label: 'SL', align: 'right' },
                                { key: 'buy_date', label: 'Kaufdatum', align: 'left' },
                                { key: 'is_live', label: 'Live', align: 'center' },
                                { key: 'market_cap', label: 'MCap', align: 'right' },
                                { key: 'chart', label: 'Chart', align: 'center' }
                              ].map(col => (
                                <th key={col.key}
                                  className={`p-2 ${col.key !== 'chart' ? 'cursor-pointer hover:text-blue-400' : ''} select-none ${col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : ''}`}
                                  onClick={() => {
                                    if (col.key === 'chart') return
                                    if (flipperSortColumn === col.key) setFlipperSortDir(flipperSortDir === 'asc' ? 'desc' : 'asc')
                                    else { setFlipperSortColumn(col.key); setFlipperSortDir('asc') }
                                  }}
                                >
                                  {col.label}
                                  {flipperSortColumn === col.key && <span className="ml-1 text-blue-400">{flipperSortDir === 'asc' ? '↑' : '↓'}</span>}
                                </th>
                              ))}
                            </tr>
                          </thead>
                          <tbody>
                            {[...flipperPrivatePortfolio.positions].sort((a, b) => {
                              let valA = a[flipperSortColumn], valB = b[flipperSortColumn]
                              if (flipperSortColumn === 'buy_date') { valA = new Date(valA).getTime(); valB = new Date(valB).getTime() }
                              if (typeof valA === 'string') return flipperSortDir === 'asc' ? valA.localeCompare(valB) : valB.localeCompare(valA)
                              return flipperSortDir === 'asc' ? valA - valB : valB - valA
                            }).map(pos => (
                              <tr key={pos.id} className="border-b border-dark-700/50 hover:bg-dark-700/30">
                                <td className="p-2 font-medium text-white cursor-pointer hover:text-blue-400" onClick={() => setSelectedPosition({ symbol: pos.symbol, name: pos.name, mode: 'defensive' })}>{pos.symbol}</td>
                                <td className="p-2 text-gray-400 text-sm cursor-pointer hover:text-blue-400" onClick={() => setSelectedPosition({ symbol: pos.symbol, name: pos.name, mode: 'defensive' })}>{pos.name}</td>
                                <td className="p-2 text-right text-gray-300">
                                  {pos.quantity?.toFixed(2)}
                                  <span className="text-gray-500 text-xs ml-1">({formatPrice(pos.invested_eur || pos.avg_price * pos.quantity)})</span>
                                </td>
                                <td className="p-2 text-right text-gray-300">{formatPrice(pos.avg_price)}</td>
                                <td className="p-2 text-right text-white">{formatPrice(pos.current_price)}</td>
                                <td className={`p-2 text-right font-medium ${pos.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                  {pos.total_return_pct >= 0 ? '+' : ''}{pos.total_return_pct?.toFixed(2)}%
                                  <span className="text-gray-400 text-xs ml-1">({formatPrice(pos.current_price * pos.quantity)})</span>
                                </td>
                                <td className="p-2 text-right">
                                  {pos.stop_loss_price > 0 ? (
                                    <div>
                                      <span className={`font-medium ${
                                        pos.current_price > 0 && pos.stop_loss_price > 0
                                          ? ((pos.current_price - pos.stop_loss_price) / pos.current_price * 100) > 10
                                            ? 'text-green-400'
                                            : ((pos.current_price - pos.stop_loss_price) / pos.current_price * 100) > 5
                                              ? 'text-orange-400'
                                              : 'text-red-400'
                                          : 'text-gray-400'
                                      }`}>{formatPrice(pos.stop_loss_price)}</span>
                                      <div className="text-[10px] text-gray-500">
                                        {pos.stop_loss_percent ? `${pos.stop_loss_percent}%` : 'default'} {pos.stop_loss_type || 'trailing'}
                                      </div>
                                    </div>
                                  ) : (
                                    <span className="text-gray-600">-</span>
                                  )}
                                </td>
                                <td className="p-2 text-gray-500 text-sm">{new Date(pos.buy_date).toLocaleDateString('de-DE')}</td>
                                <td className="p-2 text-center">
                                  {pos.is_live && <span className="px-1.5 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded">LIVE</span>}
                                </td>
                                <td className="p-2 text-right text-gray-400 text-xs">
                                  {pos.market_cap > 0 ? (pos.market_cap >= 1e12 ? `${(pos.market_cap / 1e12).toFixed(1)}T` : pos.market_cap >= 1e9 ? `${(pos.market_cap / 1e9).toFixed(1)}B` : pos.market_cap >= 1e6 ? `${(pos.market_cap / 1e6).toFixed(0)}M` : pos.market_cap.toLocaleString()) : '-'}
                                </td>
                                <td className="p-2 text-center">
                                  <a href={`https://www.tradingview.com/chart/?symbol=${pos.symbol}`} target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:text-blue-300" title="TradingView Chart">
                                    <svg className="w-4 h-4 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
                                  </a>
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                        <div className="mt-3 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Gesamt investiert (inkl. geschlossen):</span>
                          <span className="text-white font-medium">{formatPrice(flipperPrivatePortfolio.overall_invested || flipperPrivatePortfolio.total_invested)}</span>
                        </div>
                        <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Aktueller Wert (offen):</span>
                          <span className="text-white font-medium">{formatPrice(flipperPrivatePortfolio.total_value)}</span>
                        </div>
                        {(flipperPrivatePortfolio.realized_pl !== undefined && flipperPrivatePortfolio.realized_pl !== 0) && (
                          <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                            <span className="text-gray-400">Realisiert (geschlossene Trades):</span>
                            <span className={`font-medium ${flipperPrivatePortfolio.realized_pl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {flipperPrivatePortfolio.realized_pl >= 0 ? '+' : ''}{formatPrice(flipperPrivatePortfolio.realized_pl)}
                            </span>
                          </div>
                        )}
                        <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Gesamt Rendite:</span>
                          <span className={`font-medium ${(flipperPrivatePortfolio.overall_return || flipperPrivatePortfolio.total_return) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {formatPrice(flipperPrivatePortfolio.overall_return || flipperPrivatePortfolio.total_return)}
                            ({flipperPrivatePortfolio.total_return_pct >= 0 ? '+' : ''}{flipperPrivatePortfolio.total_return_pct?.toFixed(2)}%)
                          </span>
                        </div>
                      </div>
                    ) : (
                      <div className="text-center py-8 text-gray-500">
                        <p>Keine Positionen vorhanden</p>
                        <p className="text-xs mt-1">Klicke "Refresh" um Signale zu verarbeiten</p>
                      </div>
                    )}

                    {/* Flipper Trade History */}
                    <div className="mt-4 rounded-xl border border-dark-600 overflow-hidden">
                      <button
                        onClick={() => setShowFlipperTradeHistory(!showFlipperTradeHistory)}
                        className="w-full p-4 flex items-center justify-between hover:bg-dark-700/50 transition-colors"
                      >
                        <h3 className="text-sm font-semibold text-white">Trade History ({flipperCompletedTrades.length})</h3>
                        <svg className={`w-5 h-5 text-gray-400 transition-transform ${showFlipperTradeHistory ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                      </button>
                      {showFlipperTradeHistory && (
                        <div className="border-t border-dark-600">
                          {flipperCompletedTrades.length === 0 ? (
                            <div className="p-8 text-center text-gray-500">Noch keine abgeschlossenen Trades</div>
                          ) : (
                            <div className="overflow-x-auto">
                              <table className="w-full text-sm">
                                <thead>
                                  <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                                    <th className="pt-4 pb-3 px-4">Symbol</th>
                                    <th className="pt-4 pb-3 px-4">Kauf</th>
                                    <th className="pt-4 pb-3 px-4">Verkauf</th>
                                    <th className="pt-4 pb-3 px-4 text-right">Rendite</th>
                                  </tr>
                                </thead>
                                <tbody>
                                  {flipperCompletedTrades.map((trade) => (
                                    <tr key={trade.id} className={`border-b border-dark-700/50 last:border-0 ${trade.is_live ? 'bg-green-500/5' : ''}`}>
                                      <td className="py-3 px-4">
                                        <div className="flex items-center gap-2">
                                          <div className="font-medium text-white">{trade.symbol}</div>
                                          {trade.is_live && <span className="px-1.5 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded">LIVE</span>}
                                        </div>
                                      </td>
                                      <td className="py-3 px-4">
                                        <div className="text-gray-300">{formatPrice(trade.buy_price)}</div>
                                        <div className="text-xs text-gray-500">{formatDateShort(trade.buy_date)}</div>
                                      </td>
                                      <td className="py-3 px-4">
                                        <div className="text-gray-300">{formatPrice(trade.sell_price)}</div>
                                        <div className="text-xs text-gray-500">{formatDateShort(trade.sell_date)}</div>
                                      </td>
                                      <td className={`py-3 px-4 text-right font-medium ${trade.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                        <div>{formatPercent(trade.profit_loss_pct)}</div>
                                        <div className="text-xs">{trade.profit_loss >= 0 ? '+' : ''}{formatPrice(trade.profit_loss)}</div>
                                      </td>
                                    </tr>
                                  ))}
                                </tbody>
                              </table>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                )}

                {/* Lutz Simulated Portfolio Section */}
                {botTab === 'lutz' && (
                  <div className="p-4 bg-orange-500/10 border border-orange-500/30 rounded-xl">
                    <div className="flex items-center justify-between mb-4">
                      <div>
                        <h3 className="text-lg font-semibold text-orange-300 flex items-center gap-2">
                          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                          </svg>
                          SIM Portfolio (Aggressiv)
                        </h3>
                        {lastLutzRefresh && (
                          <p className="text-xs text-gray-500 mt-1">
                            Letzter Refresh: {new Date(lastLutzRefresh.updated_at).toLocaleString('de-DE')}
                            {lastLutzRefresh.triggered_by && ` (von: ${lastLutzRefresh.triggered_by})`}
                          </p>
                        )}
                      </div>
                      <button
                        onClick={async () => {
                          setLutzRefreshing(true)
                          try {
                            await fetch('/api/lutz/update', { headers: { 'Authorization': `Bearer ${token}` } })
                            await fetchLutzSimulatedData()
                            await fetchLastLutzRefresh()
                            fetchBotData()
                          } catch (err) { console.error(err) }
                          setLutzRefreshing(false)
                        }}
                        disabled={lutzRefreshing}
                        className="px-3 py-1.5 bg-orange-500/20 text-orange-300 rounded-lg hover:bg-orange-500/30 text-sm disabled:opacity-50"
                      >
                        {lutzRefreshing ? 'Aktualisiere...' : 'Refresh'}
                      </button>
                    </div>

                    {lutzPrivatePerformance && (
                      <div className="mb-4">
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mb-4">
                          <div className="bg-gradient-to-r from-orange-500/20 to-amber-500/20 rounded-lg p-3 md:p-4 border border-orange-500/30">
                            <div className="text-xs text-gray-400 mb-1">Gesamt Rendite</div>
                            <div className={`text-xl md:text-2xl font-bold ${lutzPrivatePerformance.overall_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPercent(lutzPrivatePerformance.overall_return_pct)}
                            </div>
                            <div className={`text-xs mt-1 ${(lutzPrivatePerformance.total_gain || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(lutzPrivatePerformance.total_gain || 0)} Gewinn
                            </div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Investiert</div>
                            <div className="text-lg md:text-xl font-bold text-white">{formatPrice(lutzPrivatePerformance.invested_in_positions || 0)}</div>
                            <div className="text-xs text-gray-500 mt-1">Aktuell: {formatPrice(lutzPrivatePerformance.current_value || 0)}</div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Unrealisiert</div>
                            <div className={`text-lg md:text-xl font-bold ${(lutzPrivatePerformance.unrealized_gain || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(lutzPrivatePerformance.unrealized_gain || 0)}
                            </div>
                            <div className={`text-xs mt-1 ${(lutzPrivatePerformance.total_return_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPercent(lutzPrivatePerformance.total_return_pct)}
                            </div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Realisiert</div>
                            <div className={`text-lg md:text-xl font-bold ${(lutzPrivatePerformance.realized_profit || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(lutzPrivatePerformance.realized_profit || 0)}
                            </div>
                            <div className="text-xs text-gray-500 mt-1">{lutzPrivatePerformance.total_trades || 0} Trades</div>
                          </div>
                        </div>
                        <div className="grid grid-cols-2 md:grid-cols-5 gap-3 md:gap-4">
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Win Rate</div>
                            <div className={`text-base font-bold ${(lutzEnrichedPerf.win_rate || 0) >= 50 ? 'text-green-400' : 'text-red-400'}`}>{lutzEnrichedPerf.win_rate?.toFixed(1) || 0}%</div>
                            <div className="text-xs text-gray-500 mt-1">{lutzEnrichedPerf.wins || 0}W / {lutzEnrichedPerf.losses || 0}L</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Risiko-Rendite</div>
                            <div className={`text-base font-bold ${(lutzEnrichedPerf.risk_reward || 0) >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                              {(lutzEnrichedPerf.risk_reward || 0).toFixed(2)}
                            </div>
                            <div className="text-xs text-gray-500 mt-1">Ø Gewinn / Ø Verlust</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Ø Gewinn-Trade</div>
                            <div className="text-base font-bold text-green-400">+{(lutzEnrichedPerf.avg_win_pct || 0).toFixed(2)}%</div>
                            <div className="text-xs text-gray-500 mt-1">{lutzEnrichedPerf.wins || 0} Trades</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Ø Verlust-Trade</div>
                            <div className="text-base font-bold text-red-400">-{(lutzEnrichedPerf.avg_loss_pct || 0).toFixed(2)}%</div>
                            <div className="text-xs text-gray-500 mt-1">{lutzEnrichedPerf.losses || 0} Trades</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Offene Positionen</div>
                            <div className="text-base font-bold text-white">{lutzEnrichedPerf.open_positions || 0}</div>
                            <div className="text-xs mt-1">
                              <span className="text-gray-500">von {lutzEnrichedPerf.total_buys || 0} Käufen</span>
                              {lutzPrivatePortfolio?.positions?.filter(p => p.is_live).length > 0 && (
                                <span className="ml-1 text-green-400">{lutzPrivatePortfolio.positions.filter(p => p.is_live).length} Live</span>
                              )}
                            </div>
                          </div>
                        </div>
                      </div>
                    )}

                    {lutzPrivatePortfolio?.positions?.length > 0 ? (
                      <div className="overflow-x-auto">
                        <table className="w-full">
                          <thead>
                            <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                              {[
                                { key: 'symbol', label: 'Symbol', align: 'left' },
                                { key: 'name', label: 'Name', align: 'left' },
                                { key: 'quantity', label: 'Menge (Invest.)', align: 'right' },
                                { key: 'avg_price', label: 'Kaufpreis', align: 'right' },
                                { key: 'current_price', label: 'Aktuell', align: 'right' },
                                { key: 'total_return_pct', label: 'Rendite (Wert)', align: 'right' },
                                { key: 'stop_loss_price', label: 'SL', align: 'right' },
                                { key: 'buy_date', label: 'Kaufdatum', align: 'left' },
                                { key: 'is_live', label: 'Live', align: 'center' },
                                { key: 'market_cap', label: 'MCap', align: 'right' },
                                { key: 'chart', label: 'Chart', align: 'center' }
                              ].map(col => (
                                <th key={col.key}
                                  className={`p-2 ${col.key !== 'chart' ? 'cursor-pointer hover:text-orange-400' : ''} select-none ${col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : ''}`}
                                  onClick={() => {
                                    if (col.key === 'chart') return
                                    if (lutzSortColumn === col.key) setLutzSortDir(lutzSortDir === 'asc' ? 'desc' : 'asc')
                                    else { setLutzSortColumn(col.key); setLutzSortDir('asc') }
                                  }}
                                >
                                  {col.label}
                                  {lutzSortColumn === col.key && <span className="ml-1 text-orange-400">{lutzSortDir === 'asc' ? '↑' : '↓'}</span>}
                                </th>
                              ))}
                            </tr>
                          </thead>
                          <tbody>
                            {[...lutzPrivatePortfolio.positions].sort((a, b) => {
                              let valA = a[lutzSortColumn], valB = b[lutzSortColumn]
                              if (lutzSortColumn === 'buy_date') { valA = new Date(valA).getTime(); valB = new Date(valB).getTime() }
                              if (typeof valA === 'string') return lutzSortDir === 'asc' ? valA.localeCompare(valB) : valB.localeCompare(valA)
                              return lutzSortDir === 'asc' ? valA - valB : valB - valA
                            }).map(pos => (
                              <tr key={pos.id} className="border-b border-dark-700/50 hover:bg-dark-700/30">
                                <td className="p-2 font-medium text-white cursor-pointer hover:text-orange-400" onClick={() => setSelectedPosition({ symbol: pos.symbol, name: pos.name, mode: 'aggressive' })}>{pos.symbol}</td>
                                <td className="p-2 text-gray-400 text-sm cursor-pointer hover:text-orange-400" onClick={() => setSelectedPosition({ symbol: pos.symbol, name: pos.name, mode: 'aggressive' })}>{pos.name}</td>
                                <td className="p-2 text-right text-gray-300">
                                  {pos.quantity?.toFixed(2)}
                                  <span className="text-gray-500 text-xs ml-1">({formatPrice(pos.invested_eur || pos.avg_price * pos.quantity)})</span>
                                </td>
                                <td className="p-2 text-right text-gray-300">{formatPrice(pos.avg_price)}</td>
                                <td className="p-2 text-right text-white">{formatPrice(pos.current_price)}</td>
                                <td className={`p-2 text-right font-medium ${pos.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                  {pos.total_return_pct >= 0 ? '+' : ''}{pos.total_return_pct?.toFixed(2)}%
                                  <span className="text-gray-400 text-xs ml-1">({formatPrice(pos.current_price * pos.quantity)})</span>
                                </td>
                                <td className="p-2 text-right">
                                  {pos.stop_loss_price > 0 ? (
                                    <div>
                                      <span className={`font-medium ${
                                        pos.current_price > 0 && pos.stop_loss_price > 0
                                          ? ((pos.current_price - pos.stop_loss_price) / pos.current_price * 100) > 10
                                            ? 'text-green-400'
                                            : ((pos.current_price - pos.stop_loss_price) / pos.current_price * 100) > 5
                                              ? 'text-orange-400'
                                              : 'text-red-400'
                                          : 'text-gray-400'
                                      }`}>{formatPrice(pos.stop_loss_price)}</span>
                                      <div className="text-[10px] text-gray-500">
                                        {pos.stop_loss_percent ? `${pos.stop_loss_percent}%` : 'default'} {pos.stop_loss_type || 'trailing'}
                                      </div>
                                    </div>
                                  ) : (
                                    <span className="text-gray-600">-</span>
                                  )}
                                </td>
                                <td className="p-2 text-gray-500 text-sm">{new Date(pos.buy_date).toLocaleDateString('de-DE')}</td>
                                <td className="p-2 text-center">
                                  {pos.is_live && <span className="px-1.5 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded">LIVE</span>}
                                </td>
                                <td className="p-2 text-right text-gray-400 text-xs">
                                  {pos.market_cap > 0 ? (pos.market_cap >= 1e12 ? `${(pos.market_cap / 1e12).toFixed(1)}T` : pos.market_cap >= 1e9 ? `${(pos.market_cap / 1e9).toFixed(1)}B` : pos.market_cap >= 1e6 ? `${(pos.market_cap / 1e6).toFixed(0)}M` : pos.market_cap.toLocaleString()) : '-'}
                                </td>
                                <td className="p-2 text-center">
                                  <a href={`https://www.tradingview.com/chart/?symbol=${pos.symbol}`} target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:text-blue-300" title="TradingView Chart">
                                    <svg className="w-4 h-4 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
                                  </a>
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                        <div className="mt-3 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Gesamt investiert (inkl. geschlossen):</span>
                          <span className="text-white font-medium">{formatPrice(lutzPrivatePortfolio.overall_invested || lutzPrivatePortfolio.total_invested)}</span>
                        </div>
                        <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Aktueller Wert (offen):</span>
                          <span className="text-white font-medium">{formatPrice(lutzPrivatePortfolio.total_value)}</span>
                        </div>
                        {(lutzPrivatePortfolio.realized_pl !== undefined && lutzPrivatePortfolio.realized_pl !== 0) && (
                          <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                            <span className="text-gray-400">Realisiert (geschlossene Trades):</span>
                            <span className={`font-medium ${lutzPrivatePortfolio.realized_pl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {lutzPrivatePortfolio.realized_pl >= 0 ? '+' : ''}{formatPrice(lutzPrivatePortfolio.realized_pl)}
                            </span>
                          </div>
                        )}
                        <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Gesamt Rendite:</span>
                          <span className={`font-medium ${(lutzPrivatePortfolio.overall_return || lutzPrivatePortfolio.total_return) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {formatPrice(lutzPrivatePortfolio.overall_return || lutzPrivatePortfolio.total_return)}
                            ({lutzPrivatePortfolio.total_return_pct >= 0 ? '+' : ''}{lutzPrivatePortfolio.total_return_pct?.toFixed(2)}%)
                          </span>
                        </div>
                      </div>
                    ) : (
                      <div className="text-center py-8 text-gray-500">
                        <p>Keine Positionen vorhanden</p>
                        <p className="text-xs mt-1">Klicke "Refresh" um Signale zu verarbeiten</p>
                      </div>
                    )}

                    {/* Lutz Trade History */}
                    <div className="mt-4 rounded-xl border border-dark-600 overflow-hidden">
                      <button
                        onClick={() => setShowLutzTradeHistory(!showLutzTradeHistory)}
                        className="w-full p-4 flex items-center justify-between hover:bg-dark-700/50 transition-colors"
                      >
                        <h3 className="text-sm font-semibold text-white">Trade History ({lutzCompletedTrades.length})</h3>
                        <svg className={`w-5 h-5 text-gray-400 transition-transform ${showLutzTradeHistory ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                      </button>
                      {showLutzTradeHistory && (
                        <div className="border-t border-dark-600">
                          {lutzCompletedTrades.length === 0 ? (
                            <div className="p-8 text-center text-gray-500">Noch keine abgeschlossenen Trades</div>
                          ) : (
                            <div className="overflow-x-auto">
                              <table className="w-full text-sm">
                                <thead>
                                  <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                                    <th className="pt-4 pb-3 px-4">Symbol</th>
                                    <th className="pt-4 pb-3 px-4">Kauf</th>
                                    <th className="pt-4 pb-3 px-4">Verkauf</th>
                                    <th className="pt-4 pb-3 px-4 text-right">Rendite</th>
                                  </tr>
                                </thead>
                                <tbody>
                                  {lutzCompletedTrades.map((trade) => (
                                    <tr key={trade.id} className={`border-b border-dark-700/50 last:border-0 ${trade.is_live ? 'bg-green-500/5' : ''}`}>
                                      <td className="py-3 px-4">
                                        <div className="flex items-center gap-2">
                                          <div className="font-medium text-white">{trade.symbol}</div>
                                          {trade.is_live && <span className="px-1.5 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded">LIVE</span>}
                                        </div>
                                      </td>
                                      <td className="py-3 px-4">
                                        <div className="text-gray-300">{formatPrice(trade.buy_price)}</div>
                                        <div className="text-xs text-gray-500">{formatDateShort(trade.buy_date)}</div>
                                      </td>
                                      <td className="py-3 px-4">
                                        <div className="text-gray-300">{formatPrice(trade.sell_price)}</div>
                                        <div className="text-xs text-gray-500">{formatDateShort(trade.sell_date)}</div>
                                      </td>
                                      <td className={`py-3 px-4 text-right font-medium ${trade.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                        <div>{formatPercent(trade.profit_loss_pct)}</div>
                                        <div className="text-xs">{trade.profit_loss >= 0 ? '+' : ''}{formatPrice(trade.profit_loss)}</div>
                                      </td>
                                    </tr>
                                  ))}
                                </tbody>
                              </table>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                )}

                {/* Quant Simulated Portfolio Section (shown in Admin, is_live = false) */}
                {botTab === 'quant' && (
                  <div className="p-4 bg-violet-500/10 border border-violet-500/30 rounded-xl">
                    <div className="flex items-center justify-between mb-4">
                      <div>
                        <h3 className="text-lg font-semibold text-violet-300 flex items-center gap-2">
                          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                          </svg>
                          Simuliertes Portfolio (Backtest)
                        </h3>
                        {lastQuantRefresh && (
                          <p className="text-xs text-gray-500 mt-1">
                            Letzter Refresh: {new Date(lastQuantRefresh.updated_at).toLocaleString('de-DE')} von {lastQuantRefresh.triggered_by || 'unbekannt'}
                          </p>
                        )}
                      </div>
                      <div className="flex gap-2">
                        <button
                          onClick={async () => {
                            if (!confirm('Quant Bot Refresh - Alle Signale prüfen und Trades ausführen?')) return
                            setQuantRefreshing(true)
                            setQuantRefreshLogs([{ level: 'info', message: 'Refresh gestartet...' }])
                            try {
                              const res = await fetch('/api/quant/update', {
                                method: 'GET',
                                headers: { 'Authorization': `Bearer ${token}` }
                              })
                              const data = await res.json()
                              if (res.ok) {
                                setQuantRefreshLogs(data.logs || [{ level: 'success', message: 'Update abgeschlossen' }])
                                fetchBotData()
                                fetchQuantSimulatedData()
                                fetchLastQuantRefresh()
                              } else {
                                setQuantRefreshLogs([{ level: 'error', message: data.error || 'Unbekannter Fehler' }])
                              }
                            } catch (err) {
                              setQuantRefreshLogs([{ level: 'error', message: err.message }])
                            } finally {
                              setQuantRefreshing(false)
                            }
                          }}
                          disabled={quantRefreshing}
                          className="px-3 py-1.5 bg-violet-500 text-white rounded-lg font-medium hover:bg-violet-400 text-sm flex items-center gap-2 disabled:opacity-50"
                        >
                          {quantRefreshing ? (
                            <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                          ) : (
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                            </svg>
                          )}
                          {quantRefreshing ? 'Läuft...' : 'Refresh'}
                        </button>
                        <button
                          onClick={async () => {
                            if (!confirm('Alle ausstehenden Trades und Todos löschen?')) return
                            setQuantRefreshLogs([{ level: 'info', message: 'Cleanup gestartet...' }])
                            try {
                              const res = await fetch('/api/quant/cleanup-pending', {
                                method: 'POST',
                                headers: { 'Authorization': `Bearer ${token}` }
                              })
                              const data = await res.json()
                              if (res.ok) {
                                setQuantRefreshLogs([
                                  { level: 'success', message: `Cleanup abgeschlossen` },
                                  { level: 'info', message: `${data.deleted_trades} Trades gelöscht` },
                                  { level: 'info', message: `${data.deleted_positions} Positionen gelöscht` },
                                  { level: 'info', message: `${data.deleted_todos} Todos gelöscht` }
                                ])
                                fetchBotData()
                                fetchQuantSimulatedData()
                              } else {
                                setQuantRefreshLogs([{ level: 'error', message: data.error || 'Unbekannter Fehler' }])
                              }
                            } catch (err) {
                              setQuantRefreshLogs([{ level: 'error', message: err.message }])
                            }
                          }}
                          className="px-3 py-1.5 bg-red-500/20 text-red-400 rounded-lg font-medium hover:bg-red-500/30 text-sm flex items-center gap-2"
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                          </svg>
                          Cleanup
                        </button>
                      </div>
                    </div>

                    {/* Log Area */}
                    {quantRefreshLogs.length > 0 && (
                      <div className="mb-4 p-3 bg-dark-800 rounded-lg border border-dark-600 max-h-[200px] overflow-y-auto">
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-xs font-medium text-gray-400">Logs</span>
                          <button
                            onClick={() => setQuantRefreshLogs([])}
                            className="text-xs text-gray-500 hover:text-gray-400"
                          >
                            Schließen
                          </button>
                        </div>
                        <div className="space-y-1 font-mono text-xs">
                          {quantRefreshLogs.map((log, idx) => (
                            <div key={idx} className={`${
                              log.level === 'error' ? 'text-red-400' :
                              log.level === 'success' ? 'text-green-400' :
                              log.level === 'warning' ? 'text-yellow-400' :
                              'text-gray-400'
                            }`}>
                              [{log.level?.toUpperCase() || 'INFO'}] {log.message}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    {/* Sim Performance Chart */}
                    <div className="mb-4">
                      <PortfolioChart
                        token={token}
                        botType="quant"
                        extraParams="live=false"
                        height={200}
                        title="Simulierte Performance"
                      />
                    </div>

                    {/* Performance Stats */}
                    {quantPrivatePerformance && (
                      <div className="mb-4">
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mb-4">
                          <div className="bg-gradient-to-r from-violet-500/20 to-purple-500/20 rounded-lg p-3 md:p-4 border border-violet-500/30">
                            <div className="text-xs text-gray-400 mb-1">Gesamt Rendite</div>
                            <div className={`text-xl md:text-2xl font-bold ${quantPrivatePerformance.overall_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPercent(quantPrivatePerformance.overall_return_pct)}
                            </div>
                            <div className={`text-xs mt-1 ${(quantPrivatePerformance.total_gain || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(quantPrivatePerformance.total_gain || 0)} Gewinn
                            </div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Investiert</div>
                            <div className="text-lg md:text-xl font-bold text-white">{formatPrice(quantPrivatePerformance.invested_in_positions || 0)}</div>
                            <div className="text-xs text-gray-500 mt-1">Aktuell: {formatPrice(quantPrivatePerformance.current_value || 0)}</div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Unrealisiert</div>
                            <div className={`text-lg md:text-xl font-bold ${(quantPrivatePerformance.unrealized_gain || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(quantPrivatePerformance.unrealized_gain || 0)}
                            </div>
                            <div className={`text-xs mt-1 ${(quantPrivatePerformance.total_return_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPercent(quantPrivatePerformance.total_return_pct)}
                            </div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Realisiert</div>
                            <div className={`text-lg md:text-xl font-bold ${(quantPrivatePerformance.realized_profit || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(quantPrivatePerformance.realized_profit || 0)}
                            </div>
                            <div className="text-xs text-gray-500 mt-1">{quantPrivatePerformance.total_trades || 0} Trades</div>
                          </div>
                        </div>
                        <div className="grid grid-cols-2 md:grid-cols-5 gap-3 md:gap-4">
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Win Rate</div>
                            <div className={`text-base font-bold ${(quantEnrichedPerf.win_rate || 0) >= 50 ? 'text-green-400' : 'text-red-400'}`}>{quantEnrichedPerf.win_rate?.toFixed(1) || 0}%</div>
                            <div className="text-xs text-gray-500 mt-1">{quantEnrichedPerf.wins || 0}W / {quantEnrichedPerf.losses || 0}L</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Risiko-Rendite</div>
                            <div className={`text-base font-bold ${(quantEnrichedPerf.risk_reward || 0) >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                              {(quantEnrichedPerf.risk_reward || 0).toFixed(2)}
                            </div>
                            <div className="text-xs text-gray-500 mt-1">Ø Gewinn / Ø Verlust</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Ø Gewinn-Trade</div>
                            <div className="text-base font-bold text-green-400">+{(quantEnrichedPerf.avg_win_pct || 0).toFixed(2)}%</div>
                            <div className="text-xs text-gray-500 mt-1">{quantEnrichedPerf.wins || 0} Trades</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Ø Verlust-Trade</div>
                            <div className="text-base font-bold text-red-400">-{(quantEnrichedPerf.avg_loss_pct || 0).toFixed(2)}%</div>
                            <div className="text-xs text-gray-500 mt-1">{quantEnrichedPerf.losses || 0} Trades</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Offene Positionen</div>
                            <div className="text-base font-bold text-white">{quantEnrichedPerf.open_positions || 0}</div>
                            <div className="text-xs mt-1">
                              <span className="text-gray-500">von {quantEnrichedPerf.total_buys || 0} Käufen</span>
                              {quantPrivatePortfolio?.positions?.filter(p => p.is_live).length > 0 && (
                                <span className="ml-1 text-green-400">{quantPrivatePortfolio.positions.filter(p => p.is_live).length} Live</span>
                              )}
                            </div>
                          </div>
                        </div>
                      </div>
                    )}

                    {/* Private Positions Table */}
                    {quantPrivatePortfolio?.positions?.length > 0 ? (
                      <div className="overflow-x-auto">
                        <table className="w-full">
                          <thead>
                            <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                              {[
                                { key: 'symbol', label: 'Symbol', align: 'left' },
                                { key: 'name', label: 'Name', align: 'left' },
                                { key: 'quantity', label: 'Menge (Invest.)', align: 'right' },
                                { key: 'avg_price', label: 'Kaufpreis', align: 'right' },
                                { key: 'current_price', label: 'Aktuell', align: 'right' },
                                { key: 'tsl', label: 'SL', align: 'right' },
                                { key: 'total_return_pct', label: 'Rendite (Wert)', align: 'right' },
                                { key: 'buy_date', label: 'Kaufdatum', align: 'left' },
                                { key: 'is_live', label: 'Live', align: 'center' },
                                { key: 'market_cap', label: 'MCap', align: 'right' },
                                { key: 'chart', label: 'Chart', align: 'center' }
                              ].map(col => (
                                <th
                                  key={col.key}
                                  className={`p-2 ${col.key !== 'chart' ? 'cursor-pointer hover:text-violet-400' : ''} select-none ${col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : ''}`}
                                  onClick={() => {
                                    if (col.key === 'chart') return
                                    if (quantSortColumn === col.key) {
                                      setQuantSortDir(quantSortDir === 'asc' ? 'desc' : 'asc')
                                    } else {
                                      setQuantSortColumn(col.key)
                                      setQuantSortDir('asc')
                                    }
                                  }}
                                >
                                  {col.label}
                                  {quantSortColumn === col.key && (
                                    <span className="ml-1 text-violet-400">{quantSortDir === 'asc' ? '↑' : '↓'}</span>
                                  )}
                                </th>
                              ))}
                            </tr>
                          </thead>
                          <tbody>
                            {[...quantPrivatePortfolio.positions]
                              .sort((a, b) => {
                                let valA = a[quantSortColumn]
                                let valB = b[quantSortColumn]
                                if (quantSortColumn === 'buy_date') {
                                  valA = new Date(valA).getTime()
                                  valB = new Date(valB).getTime()
                                }
                                if (typeof valA === 'string') {
                                  return quantSortDir === 'asc' ? valA.localeCompare(valB) : valB.localeCompare(valA)
                                }
                                return quantSortDir === 'asc' ? valA - valB : valB - valA
                              })
                              .map((pos) => {
                                const currentValue = (pos.current_price || 0) * (pos.quantity || 0)
                                const stopPrice = pos.stop_loss_price || 0
                                const isNearStop = pos.current_price && stopPrice && (pos.current_price - stopPrice) / pos.current_price < 0.05
                                return (
                                  <tr key={pos.id} className="border-b border-dark-700/50 hover:bg-dark-700/30">
                                    <td className="p-2 font-medium text-white cursor-pointer hover:text-violet-400" onClick={() => setSelectedPosition({ symbol: pos.symbol, name: pos.name, mode: 'quant' })}>{pos.symbol}</td>
                                    <td className="p-2 text-gray-400 text-sm cursor-pointer hover:text-violet-400" onClick={() => setSelectedPosition({ symbol: pos.symbol, name: pos.name, mode: 'quant' })}>{pos.name}</td>
                                    <td className="p-2 text-right text-gray-300">
                                      {pos.quantity?.toFixed(2)}
                                      <span className="text-gray-500 text-xs ml-1">({formatPrice(pos.invested_eur || pos.avg_price * pos.quantity)})</span>
                                    </td>
                                    <td className="p-2 text-right text-gray-300">{formatPrice(pos.avg_price)}</td>
                                    <td className="p-2 text-right text-white">{formatPrice(pos.current_price)}</td>
                                    <td className={`p-2 text-right ${isNearStop ? 'text-yellow-400' : 'text-red-400'}`}>
                                      {formatPrice(stopPrice)}
                                    </td>
                                    <td className={`p-2 text-right font-medium ${
                                      pos.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                                    }`}>
                                      {pos.total_return_pct >= 0 ? '+' : ''}{pos.total_return_pct?.toFixed(2)}%
                                      <span className="text-gray-400 text-xs ml-1">({formatPrice(currentValue)})</span>
                                    </td>
                                    <td className="p-2 text-gray-500 text-sm">
                                      {new Date(pos.buy_date).toLocaleDateString('de-DE')}
                                    </td>
                                    <td className="p-2 text-center">
                                      {pos.is_live && <span className="px-1.5 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded">LIVE</span>}
                                    </td>
                                    <td className="p-2 text-center">
                                      <a href={`https://www.tradingview.com/chart/?symbol=${pos.symbol}`} target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:text-blue-300" title="TradingView Chart">
                                        <svg className="w-4 h-4 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
                                      </a>
                                    </td>
                                  </tr>
                                )
                              })}
                          </tbody>
                        </table>
                        <div className="mt-3 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Gesamt investiert (inkl. geschlossen):</span>
                          <span className="text-white font-medium">{formatPrice(quantPrivatePortfolio.overall_invested || quantPrivatePortfolio.total_invested)}</span>
                        </div>
                        <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Aktueller Wert (offen):</span>
                          <span className="text-white font-medium">{formatPrice(quantPrivatePortfolio.total_value)}</span>
                        </div>
                        {(quantPrivatePortfolio.realized_pl !== undefined && quantPrivatePortfolio.realized_pl !== 0) && (
                          <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                            <span className="text-gray-400">Realisiert (geschlossene Trades):</span>
                            <span className={`font-medium ${quantPrivatePortfolio.realized_pl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {quantPrivatePortfolio.realized_pl >= 0 ? '+' : ''}{formatPrice(quantPrivatePortfolio.realized_pl)}
                            </span>
                          </div>
                        )}
                        <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Gesamt Rendite:</span>
                          <span className={`font-medium ${
                            (quantPrivatePortfolio.overall_return || quantPrivatePortfolio.total_return) >= 0 ? 'text-green-400' : 'text-red-400'
                          }`}>
                            {formatPrice(quantPrivatePortfolio.overall_return || quantPrivatePortfolio.total_return)}
                            ({quantPrivatePortfolio.total_return_pct >= 0 ? '+' : ''}{quantPrivatePortfolio.total_return_pct?.toFixed(2)}%)
                          </span>
                        </div>
                      </div>
                    ) : (
                      <div className="text-center py-8 text-gray-500">
                        <svg className="w-12 h-12 mx-auto mb-3 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                        </svg>
                        <p>Keine simulierten Positionen (is_live = false)</p>
                        <p className="text-xs mt-1">Backtest-Trades ohne "Live" Flag erscheinen hier</p>
                      </div>
                    )}

                    {/* Quant Trade History */}
                    <div className="mt-4 rounded-xl border border-dark-600 overflow-hidden">
                      <button
                        onClick={() => setShowQuantTradeHistory(!showQuantTradeHistory)}
                        className="w-full p-4 flex items-center justify-between hover:bg-dark-700/50 transition-colors"
                      >
                        <h3 className="text-sm font-semibold text-white">Trade History ({quantCompletedTrades.length})</h3>
                        <svg className={`w-5 h-5 text-gray-400 transition-transform ${showQuantTradeHistory ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                      </button>
                      {showQuantTradeHistory && (
                        <div className="border-t border-dark-600">
                          {quantCompletedTrades.length === 0 ? (
                            <div className="p-8 text-center text-gray-500">Noch keine abgeschlossenen Trades</div>
                          ) : (
                            <div className="overflow-x-auto">
                              <table className="w-full text-sm">
                                <thead>
                                  <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                                    <th className="pt-4 pb-3 px-4">Symbol</th>
                                    <th className="pt-4 pb-3 px-4">Kauf</th>
                                    <th className="pt-4 pb-3 px-4">Verkauf</th>
                                    <th className="pt-4 pb-3 px-4 text-right">Rendite</th>
                                  </tr>
                                </thead>
                                <tbody>
                                  {quantCompletedTrades.map((trade) => (
                                    <tr key={trade.id} className={`border-b border-dark-700/50 last:border-0 ${trade.is_live ? 'bg-green-500/5' : ''}`}>
                                      <td className="py-3 px-4">
                                        <div className="flex items-center gap-2">
                                          <div className="font-medium text-white">{trade.symbol}</div>
                                          {trade.is_live && <span className="px-1.5 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded">LIVE</span>}
                                        </div>
                                      </td>
                                      <td className="py-3 px-4">
                                        <div className="text-gray-300">{formatPrice(trade.buy_price)}</div>
                                        <div className="text-xs text-gray-500">{formatDateShort(trade.buy_date)}</div>
                                      </td>
                                      <td className="py-3 px-4">
                                        <div className="text-gray-300">{formatPrice(trade.sell_price)}</div>
                                        <div className="text-xs text-gray-500">{formatDateShort(trade.sell_date)}</div>
                                      </td>
                                      <td className={`py-3 px-4 text-right font-medium ${trade.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                        <div>{formatPercent(trade.profit_loss_pct)}</div>
                                        <div className="text-xs">{trade.profit_loss >= 0 ? '+' : ''}{formatPrice(trade.profit_loss)}</div>
                                      </td>
                                    </tr>
                                  ))}
                                </tbody>
                              </table>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                )}

                {/* Ditz Simulated Portfolio Section (shown in Admin, is_live = false) */}
                {botTab === 'ditz' && (
                  <div className="p-4 bg-cyan-500/10 border border-cyan-500/30 rounded-xl">
                    <div className="flex items-center justify-between mb-4">
                      <div>
                        <h3 className="text-lg font-semibold text-cyan-300 flex items-center gap-2">
                          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                          </svg>
                          Simuliertes Portfolio (Backtest)
                        </h3>
                        {lastDitzRefresh && (
                          <p className="text-xs text-gray-500 mt-1">
                            Letzter Refresh: {new Date(lastDitzRefresh.updated_at).toLocaleString('de-DE')} von {lastDitzRefresh.triggered_by || 'unbekannt'}
                          </p>
                        )}
                      </div>
                      <div className="flex gap-2">
                        <button
                          onClick={async () => {
                            if (!confirm('Ditz Bot Refresh - Alle Signale prüfen und Trades ausführen?')) return
                            setDitzRefreshing(true)
                            setDitzRefreshLogs([{ level: 'info', message: 'Refresh gestartet...' }])
                            try {
                              const res = await fetch('/api/ditz/update', {
                                method: 'GET',
                                headers: { 'Authorization': `Bearer ${token}` }
                              })
                              const data = await res.json()
                              if (res.ok) {
                                setDitzRefreshLogs(data.logs || [{ level: 'success', message: 'Update abgeschlossen' }])
                                fetchBotData()
                                fetchDitzSimulatedData()
                                fetchLastDitzRefresh()
                              } else {
                                setDitzRefreshLogs([{ level: 'error', message: data.error || 'Unbekannter Fehler' }])
                              }
                            } catch (err) {
                              setDitzRefreshLogs([{ level: 'error', message: err.message }])
                            } finally {
                              setDitzRefreshing(false)
                            }
                          }}
                          disabled={ditzRefreshing}
                          className="px-3 py-1.5 bg-cyan-500 text-white rounded-lg font-medium hover:bg-cyan-400 text-sm flex items-center gap-2 disabled:opacity-50"
                        >
                          {ditzRefreshing ? (
                            <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                          ) : (
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                            </svg>
                          )}
                          {ditzRefreshing ? 'Läuft...' : 'Refresh'}
                        </button>
                        <button
                          onClick={async () => {
                            if (!confirm('Alle ausstehenden Trades und Todos löschen?')) return
                            setDitzRefreshLogs([{ level: 'info', message: 'Cleanup gestartet...' }])
                            try {
                              const res = await fetch('/api/ditz/cleanup-pending', {
                                method: 'POST',
                                headers: { 'Authorization': `Bearer ${token}` }
                              })
                              const data = await res.json()
                              if (res.ok) {
                                setDitzRefreshLogs([
                                  { level: 'success', message: `Cleanup abgeschlossen` },
                                  { level: 'info', message: `${data.deleted_trades} Trades gelöscht` },
                                  { level: 'info', message: `${data.deleted_positions} Positionen gelöscht` },
                                  { level: 'info', message: `${data.deleted_todos} Todos gelöscht` }
                                ])
                                fetchBotData()
                                fetchDitzSimulatedData()
                              } else {
                                setDitzRefreshLogs([{ level: 'error', message: data.error || 'Unbekannter Fehler' }])
                              }
                            } catch (err) {
                              setDitzRefreshLogs([{ level: 'error', message: err.message }])
                            }
                          }}
                          className="px-3 py-1.5 bg-red-500/20 text-red-400 rounded-lg font-medium hover:bg-red-500/30 text-sm flex items-center gap-2"
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                          </svg>
                          Cleanup
                        </button>
                      </div>
                    </div>

                    {/* Log Area */}
                    {ditzRefreshLogs.length > 0 && (
                      <div className="mb-4 p-3 bg-dark-800 rounded-lg border border-dark-600 max-h-[200px] overflow-y-auto">
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-xs font-medium text-gray-400">Logs</span>
                          <button
                            onClick={() => setDitzRefreshLogs([])}
                            className="text-xs text-gray-500 hover:text-gray-400"
                          >
                            Schließen
                          </button>
                        </div>
                        <div className="space-y-1 font-mono text-xs">
                          {ditzRefreshLogs.map((log, idx) => (
                            <div key={idx} className={`${
                              log.level === 'error' ? 'text-red-400' :
                              log.level === 'success' ? 'text-green-400' :
                              log.level === 'warning' ? 'text-yellow-400' :
                              'text-gray-400'
                            }`}>
                              [{log.level?.toUpperCase() || 'INFO'}] {log.message}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    {/* Sim Performance Chart */}
                    <div className="mb-4">
                      <PortfolioChart
                        token={token}
                        botType="ditz"
                        extraParams="live=false"
                        height={200}
                        title="Simulierte Performance"
                      />
                    </div>

                    {/* Performance Stats */}
                    {ditzPrivatePerformance && (
                      <div className="mb-4">
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mb-4">
                          <div className="bg-gradient-to-r from-cyan-500/20 to-teal-500/20 rounded-lg p-3 md:p-4 border border-cyan-500/30">
                            <div className="text-xs text-gray-400 mb-1">Gesamt Rendite</div>
                            <div className={`text-xl md:text-2xl font-bold ${ditzPrivatePerformance.overall_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPercent(ditzPrivatePerformance.overall_return_pct)}
                            </div>
                            <div className={`text-xs mt-1 ${(ditzPrivatePerformance.total_gain || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(ditzPrivatePerformance.total_gain || 0)} Gewinn
                            </div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Investiert</div>
                            <div className="text-lg md:text-xl font-bold text-white">{formatPrice(ditzPrivatePerformance.invested_in_positions || 0)}</div>
                            <div className="text-xs text-gray-500 mt-1">Aktuell: {formatPrice(ditzPrivatePerformance.current_value || 0)}</div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Unrealisiert</div>
                            <div className={`text-lg md:text-xl font-bold ${(ditzPrivatePerformance.unrealized_gain || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(ditzPrivatePerformance.unrealized_gain || 0)}
                            </div>
                            <div className={`text-xs mt-1 ${(ditzPrivatePerformance.total_return_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPercent(ditzPrivatePerformance.total_return_pct)}
                            </div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Realisiert</div>
                            <div className={`text-lg md:text-xl font-bold ${(ditzPrivatePerformance.realized_profit || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(ditzPrivatePerformance.realized_profit || 0)}
                            </div>
                            <div className="text-xs text-gray-500 mt-1">{ditzPrivatePerformance.total_trades || 0} Trades</div>
                          </div>
                        </div>
                        <div className="grid grid-cols-2 md:grid-cols-5 gap-3 md:gap-4">
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Win Rate</div>
                            <div className={`text-base font-bold ${(ditzEnrichedPerf.win_rate || 0) >= 50 ? 'text-green-400' : 'text-red-400'}`}>{ditzEnrichedPerf.win_rate?.toFixed(1) || 0}%</div>
                            <div className="text-xs text-gray-500 mt-1">{ditzEnrichedPerf.wins || 0}W / {ditzEnrichedPerf.losses || 0}L</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Risiko-Rendite</div>
                            <div className={`text-base font-bold ${(ditzEnrichedPerf.risk_reward || 0) >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                              {(ditzEnrichedPerf.risk_reward || 0).toFixed(2)}
                            </div>
                            <div className="text-xs text-gray-500 mt-1">Ø Gewinn / Ø Verlust</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Ø Gewinn-Trade</div>
                            <div className="text-base font-bold text-green-400">+{(ditzEnrichedPerf.avg_win_pct || 0).toFixed(2)}%</div>
                            <div className="text-xs text-gray-500 mt-1">{ditzEnrichedPerf.wins || 0} Trades</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Ø Verlust-Trade</div>
                            <div className="text-base font-bold text-red-400">-{(ditzEnrichedPerf.avg_loss_pct || 0).toFixed(2)}%</div>
                            <div className="text-xs text-gray-500 mt-1">{ditzEnrichedPerf.losses || 0} Trades</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Offene Positionen</div>
                            <div className="text-base font-bold text-white">{ditzEnrichedPerf.open_positions || 0}</div>
                            <div className="text-xs mt-1">
                              <span className="text-gray-500">von {ditzEnrichedPerf.total_buys || 0} Käufen</span>
                              {ditzPrivatePortfolio?.positions?.filter(p => p.is_live).length > 0 && (
                                <span className="ml-1 text-green-400">{ditzPrivatePortfolio.positions.filter(p => p.is_live).length} Live</span>
                              )}
                            </div>
                          </div>
                        </div>
                      </div>
                    )}

                    {/* Private Positions Table */}
                    {ditzPrivatePortfolio?.positions?.length > 0 ? (
                      <div className="overflow-x-auto">
                        <table className="w-full">
                          <thead>
                            <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                              {[
                                { key: 'symbol', label: 'Symbol', align: 'left' },
                                { key: 'name', label: 'Name', align: 'left' },
                                { key: 'quantity', label: 'Menge (Invest.)', align: 'right' },
                                { key: 'avg_price', label: 'Kaufpreis', align: 'right' },
                                { key: 'current_price', label: 'Aktuell', align: 'right' },
                                { key: 'tsl', label: 'SL', align: 'right' },
                                { key: 'total_return_pct', label: 'Rendite (Wert)', align: 'right' },
                                { key: 'buy_date', label: 'Kaufdatum', align: 'left' },
                                { key: 'is_live', label: 'Live', align: 'center' },
                                { key: 'market_cap', label: 'MCap', align: 'right' },
                                { key: 'chart', label: 'Chart', align: 'center' }
                              ].map(col => (
                                <th
                                  key={col.key}
                                  className={`p-2 ${col.key !== 'chart' ? 'cursor-pointer hover:text-cyan-400' : ''} select-none ${col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : ''}`}
                                  onClick={() => {
                                    if (col.key === 'chart') return
                                    if (ditzSortColumn === col.key) {
                                      setDitzSortDir(ditzSortDir === 'asc' ? 'desc' : 'asc')
                                    } else {
                                      setDitzSortColumn(col.key)
                                      setDitzSortDir('asc')
                                    }
                                  }}
                                >
                                  {col.label}
                                  {ditzSortColumn === col.key && (
                                    <span className="ml-1 text-cyan-400">{ditzSortDir === 'asc' ? '↑' : '↓'}</span>
                                  )}
                                </th>
                              ))}
                            </tr>
                          </thead>
                          <tbody>
                            {[...ditzPrivatePortfolio.positions]
                              .sort((a, b) => {
                                let valA = a[ditzSortColumn]
                                let valB = b[ditzSortColumn]
                                if (ditzSortColumn === 'buy_date') {
                                  valA = new Date(valA).getTime()
                                  valB = new Date(valB).getTime()
                                }
                                if (typeof valA === 'string') {
                                  return ditzSortDir === 'asc' ? valA.localeCompare(valB) : valB.localeCompare(valA)
                                }
                                return ditzSortDir === 'asc' ? valA - valB : valB - valA
                              })
                              .map((pos) => {
                                const currentValue = (pos.current_price || 0) * (pos.quantity || 0)
                                const stopPrice = pos.stop_loss_price || 0
                                const isNearStop = pos.current_price && stopPrice && (pos.current_price - stopPrice) / pos.current_price < 0.05
                                return (
                                  <tr key={pos.id} className="border-b border-dark-700/50 hover:bg-dark-700/30">
                                    <td className="p-2 font-medium text-white cursor-pointer hover:text-cyan-400" onClick={() => setSelectedPosition({ symbol: pos.symbol, name: pos.name, mode: 'ditz' })}>{pos.symbol}</td>
                                    <td className="p-2 text-gray-400 text-sm cursor-pointer hover:text-cyan-400" onClick={() => setSelectedPosition({ symbol: pos.symbol, name: pos.name, mode: 'ditz' })}>{pos.name}</td>
                                    <td className="p-2 text-right text-gray-300">
                                      {pos.quantity?.toFixed(2)}
                                      <span className="text-gray-500 text-xs ml-1">({formatPrice(pos.invested_eur || pos.avg_price * pos.quantity)})</span>
                                    </td>
                                    <td className="p-2 text-right text-gray-300">{formatPrice(pos.avg_price)}</td>
                                    <td className="p-2 text-right text-white">{formatPrice(pos.current_price)}</td>
                                    <td className={`p-2 text-right ${isNearStop ? 'text-yellow-400' : 'text-red-400'}`}>
                                      {formatPrice(stopPrice)}
                                    </td>
                                    <td className={`p-2 text-right font-medium ${
                                      pos.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                                    }`}>
                                      {pos.total_return_pct >= 0 ? '+' : ''}{pos.total_return_pct?.toFixed(2)}%
                                      <span className="text-gray-400 text-xs ml-1">({formatPrice(currentValue)})</span>
                                    </td>
                                    <td className="p-2 text-gray-500 text-sm">
                                      {new Date(pos.buy_date).toLocaleDateString('de-DE')}
                                    </td>
                                    <td className="p-2 text-center">
                                      {pos.is_live && <span className="px-1.5 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded">LIVE</span>}
                                    </td>
                                    <td className="p-2 text-center">
                                      <a href={`https://www.tradingview.com/chart/?symbol=${pos.symbol}`} target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:text-blue-300" title="TradingView Chart">
                                        <svg className="w-4 h-4 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
                                      </a>
                                    </td>
                                  </tr>
                                )
                              })}
                          </tbody>
                        </table>
                        <div className="mt-3 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Gesamt investiert (inkl. geschlossen):</span>
                          <span className="text-white font-medium">{formatPrice(ditzPrivatePortfolio.overall_invested || ditzPrivatePortfolio.total_invested)}</span>
                        </div>
                        <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Aktueller Wert (offen):</span>
                          <span className="text-white font-medium">{formatPrice(ditzPrivatePortfolio.total_value)}</span>
                        </div>
                        {(ditzPrivatePortfolio.realized_pl !== undefined && ditzPrivatePortfolio.realized_pl !== 0) && (
                          <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                            <span className="text-gray-400">Realisiert (geschlossene Trades):</span>
                            <span className={`font-medium ${ditzPrivatePortfolio.realized_pl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {ditzPrivatePortfolio.realized_pl >= 0 ? '+' : ''}{formatPrice(ditzPrivatePortfolio.realized_pl)}
                            </span>
                          </div>
                        )}
                        <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Gesamt Rendite:</span>
                          <span className={`font-medium ${
                            (ditzPrivatePortfolio.overall_return || ditzPrivatePortfolio.total_return) >= 0 ? 'text-green-400' : 'text-red-400'
                          }`}>
                            {formatPrice(ditzPrivatePortfolio.overall_return || ditzPrivatePortfolio.total_return)}
                            ({ditzPrivatePortfolio.total_return_pct >= 0 ? '+' : ''}{ditzPrivatePortfolio.total_return_pct?.toFixed(2)}%)
                          </span>
                        </div>
                      </div>
                    ) : (
                      <div className="text-center py-8 text-gray-500">
                        <svg className="w-12 h-12 mx-auto mb-3 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                        </svg>
                        <p>Keine simulierten Positionen (is_live = false)</p>
                        <p className="text-xs mt-1">Backtest-Trades ohne "Live" Flag erscheinen hier</p>
                      </div>
                    )}

                    {/* Ditz Trade History */}
                    <div className="mt-4 rounded-xl border border-dark-600 overflow-hidden">
                      <button
                        onClick={() => setShowDitzTradeHistory(!showDitzTradeHistory)}
                        className="w-full p-4 flex items-center justify-between hover:bg-dark-700/50 transition-colors"
                      >
                        <h3 className="text-sm font-semibold text-white">Trade History ({ditzCompletedTrades.length})</h3>
                        <svg className={`w-5 h-5 text-gray-400 transition-transform ${showDitzTradeHistory ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                      </button>
                      {showDitzTradeHistory && (
                        <div className="border-t border-dark-600">
                          {ditzCompletedTrades.length === 0 ? (
                            <div className="p-8 text-center text-gray-500">Noch keine abgeschlossenen Trades</div>
                          ) : (
                            <div className="overflow-x-auto">
                              <table className="w-full text-sm">
                                <thead>
                                  <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                                    <th className="pt-4 pb-3 px-4">Symbol</th>
                                    <th className="pt-4 pb-3 px-4">Kauf</th>
                                    <th className="pt-4 pb-3 px-4">Verkauf</th>
                                    <th className="pt-4 pb-3 px-4 text-right">Rendite</th>
                                  </tr>
                                </thead>
                                <tbody>
                                  {ditzCompletedTrades.map((trade) => (
                                    <tr key={trade.id} className={`border-b border-dark-700/50 last:border-0 ${trade.is_live ? 'bg-green-500/5' : ''}`}>
                                      <td className="py-3 px-4">
                                        <div className="flex items-center gap-2">
                                          <div className="font-medium text-white">{trade.symbol}</div>
                                          {trade.is_live && <span className="px-1.5 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded">LIVE</span>}
                                        </div>
                                      </td>
                                      <td className="py-3 px-4">
                                        <div className="text-gray-300">{formatPrice(trade.buy_price)}</div>
                                        <div className="text-xs text-gray-500">{formatDateShort(trade.buy_date)}</div>
                                      </td>
                                      <td className="py-3 px-4">
                                        <div className="text-gray-300">{formatPrice(trade.sell_price)}</div>
                                        <div className="text-xs text-gray-500">{formatDateShort(trade.sell_date)}</div>
                                      </td>
                                      <td className={`py-3 px-4 text-right font-medium ${trade.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                        <div>{formatPercent(trade.profit_loss_pct)}</div>
                                        <div className="text-xs">{trade.profit_loss >= 0 ? '+' : ''}{formatPrice(trade.profit_loss)}</div>
                                      </td>
                                    </tr>
                                  ))}
                                </tbody>
                              </table>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                )}

                {botTab === 'trader' && (
                  <div className="p-4 bg-emerald-500/10 border border-emerald-500/30 rounded-xl">
                    <div className="flex items-center justify-between mb-4">
                      <div>
                        <h3 className="text-lg font-semibold text-emerald-300 flex items-center gap-2">
                          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                          </svg>
                          Simuliertes Portfolio (Backtest)
                        </h3>
                        {lastTraderRefresh && (
                          <p className="text-xs text-gray-500 mt-1">
                            Letzter Refresh: {new Date(lastTraderRefresh.updated_at).toLocaleString('de-DE')} von {lastTraderRefresh.triggered_by || 'unbekannt'}
                          </p>
                        )}
                      </div>
                      <div className="flex gap-2">
                        <button
                          onClick={async () => {
                            if (!confirm('Trader Bot Refresh - Alle Signale prüfen und Trades ausführen?')) return
                            setTraderRefreshing(true)
                            setTraderRefreshLogs([{ level: 'info', message: 'Refresh gestartet...' }])
                            try {
                              const res = await fetch('/api/trader/update', {
                                method: 'GET',
                                headers: { 'Authorization': `Bearer ${token}` }
                              })
                              const data = await res.json()
                              if (res.ok) {
                                setTraderRefreshLogs(data.logs || [{ level: 'success', message: 'Update abgeschlossen' }])
                                fetchBotData()
                                fetchTraderSimulatedData()
                                fetchLastTraderRefresh()
                              } else {
                                setTraderRefreshLogs([{ level: 'error', message: data.error || 'Unbekannter Fehler' }])
                              }
                            } catch (err) {
                              setTraderRefreshLogs([{ level: 'error', message: err.message }])
                            } finally {
                              setTraderRefreshing(false)
                            }
                          }}
                          disabled={traderRefreshing}
                          className="px-3 py-1.5 bg-emerald-500 text-white rounded-lg font-medium hover:bg-emerald-400 text-sm flex items-center gap-2 disabled:opacity-50"
                        >
                          {traderRefreshing ? (
                            <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                          ) : (
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                            </svg>
                          )}
                          {traderRefreshing ? 'Läuft...' : 'Refresh'}
                        </button>
                        <button
                          onClick={async () => {
                            if (!confirm('Alle ausstehenden Trades und Todos löschen?')) return
                            setTraderRefreshLogs([{ level: 'info', message: 'Cleanup gestartet...' }])
                            try {
                              const res = await fetch('/api/trader/cleanup-pending', {
                                method: 'POST',
                                headers: { 'Authorization': `Bearer ${token}` }
                              })
                              const data = await res.json()
                              if (res.ok) {
                                setTraderRefreshLogs([
                                  { level: 'success', message: `Cleanup abgeschlossen` },
                                  { level: 'info', message: `${data.deleted_trades} Trades gelöscht` },
                                  { level: 'info', message: `${data.deleted_positions} Positionen gelöscht` },
                                  { level: 'info', message: `${data.deleted_todos} Todos gelöscht` }
                                ])
                                fetchBotData()
                                fetchTraderSimulatedData()
                              } else {
                                setTraderRefreshLogs([{ level: 'error', message: data.error || 'Unbekannter Fehler' }])
                              }
                            } catch (err) {
                              setTraderRefreshLogs([{ level: 'error', message: err.message }])
                            }
                          }}
                          className="px-3 py-1.5 bg-red-500/20 text-red-400 rounded-lg font-medium hover:bg-red-500/30 text-sm flex items-center gap-2"
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                          </svg>
                          Cleanup
                        </button>
                      </div>
                    </div>

                    {/* Log Area */}
                    {traderRefreshLogs.length > 0 && (
                      <div className="mb-4 p-3 bg-dark-800 rounded-lg border border-dark-600 max-h-[200px] overflow-y-auto">
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-xs font-medium text-gray-400">Logs</span>
                          <button
                            onClick={() => setTraderRefreshLogs([])}
                            className="text-xs text-gray-500 hover:text-gray-400"
                          >
                            Schließen
                          </button>
                        </div>
                        <div className="space-y-1 font-mono text-xs">
                          {traderRefreshLogs.map((log, idx) => (
                            <div key={idx} className={`${
                              log.level === 'error' ? 'text-red-400' :
                              log.level === 'success' ? 'text-green-400' :
                              log.level === 'warning' ? 'text-yellow-400' :
                              'text-gray-400'
                            }`}>
                              [{log.level?.toUpperCase() || 'INFO'}] {log.message}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    {/* Sim Performance Chart */}
                    <div className="mb-4">
                      <PortfolioChart
                        token={token}
                        botType="trader"
                        extraParams="live=false"
                        height={200}
                        title="Simulierte Performance"
                      />
                    </div>

                    {/* Performance Stats */}
                    {traderPrivatePerformance && (
                      <div className="mb-4">
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mb-4">
                          <div className="bg-gradient-to-r from-emerald-500/20 to-teal-500/20 rounded-lg p-3 md:p-4 border border-emerald-500/30">
                            <div className="text-xs text-gray-400 mb-1">Gesamt Rendite</div>
                            <div className={`text-xl md:text-2xl font-bold ${traderPrivatePerformance.overall_return_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPercent(traderPrivatePerformance.overall_return_pct)}
                            </div>
                            <div className={`text-xs mt-1 ${(traderPrivatePerformance.total_gain || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(traderPrivatePerformance.total_gain || 0)} Gewinn
                            </div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Investiert</div>
                            <div className="text-lg md:text-xl font-bold text-white">{formatPrice(traderPrivatePerformance.invested_in_positions || 0)}</div>
                            <div className="text-xs text-gray-500 mt-1">Aktuell: {formatPrice(traderPrivatePerformance.current_value || 0)}</div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Unrealisiert</div>
                            <div className={`text-lg md:text-xl font-bold ${(traderPrivatePerformance.unrealized_gain || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(traderPrivatePerformance.unrealized_gain || 0)}
                            </div>
                            <div className={`text-xs mt-1 ${(traderPrivatePerformance.total_return_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPercent(traderPrivatePerformance.total_return_pct)}
                            </div>
                          </div>
                          <div className="bg-dark-700 rounded-lg p-3 md:p-4">
                            <div className="text-xs text-gray-500 mb-1">Realisiert</div>
                            <div className={`text-lg md:text-xl font-bold ${(traderPrivatePerformance.realized_profit || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatPrice(traderPrivatePerformance.realized_profit || 0)}
                            </div>
                            <div className="text-xs text-gray-500 mt-1">{traderPrivatePerformance.total_trades || 0} Trades</div>
                          </div>
                        </div>
                        <div className="grid grid-cols-2 md:grid-cols-5 gap-3 md:gap-4">
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Win Rate</div>
                            <div className={`text-base font-bold ${(traderEnrichedPerf.win_rate || 0) >= 50 ? 'text-green-400' : 'text-red-400'}`}>{traderEnrichedPerf.win_rate?.toFixed(1) || 0}%</div>
                            <div className="text-xs text-gray-500 mt-1">{traderEnrichedPerf.wins || 0}W / {traderEnrichedPerf.losses || 0}L</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Risiko-Rendite</div>
                            <div className={`text-base font-bold ${(traderEnrichedPerf.risk_reward || 0) >= 1 ? 'text-green-400' : 'text-red-400'}`}>
                              {(traderEnrichedPerf.risk_reward || 0).toFixed(2)}
                            </div>
                            <div className="text-xs text-gray-500 mt-1">Ø Gewinn / Ø Verlust</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Ø Gewinn-Trade</div>
                            <div className="text-base font-bold text-green-400">+{(traderEnrichedPerf.avg_win_pct || 0).toFixed(2)}%</div>
                            <div className="text-xs text-gray-500 mt-1">{traderEnrichedPerf.wins || 0} Trades</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Ø Verlust-Trade</div>
                            <div className="text-base font-bold text-red-400">-{(traderEnrichedPerf.avg_loss_pct || 0).toFixed(2)}%</div>
                            <div className="text-xs text-gray-500 mt-1">{traderEnrichedPerf.losses || 0} Trades</div>
                          </div>
                          <div className="bg-dark-700/50 rounded-lg p-3">
                            <div className="text-xs text-gray-500 mb-1">Offene Positionen</div>
                            <div className="text-base font-bold text-white">{traderEnrichedPerf.open_positions || 0}</div>
                            <div className="text-xs mt-1">
                              <span className="text-gray-500">von {traderEnrichedPerf.total_buys || 0} Käufen</span>
                              {traderPrivatePortfolio?.positions?.filter(p => p.is_live).length > 0 && (
                                <span className="ml-1 text-green-400">{traderPrivatePortfolio.positions.filter(p => p.is_live).length} Live</span>
                              )}
                            </div>
                          </div>
                        </div>
                      </div>
                    )}

                    {/* Private Positions Table */}
                    {traderPrivatePortfolio?.positions?.length > 0 ? (
                      <div className="overflow-x-auto">
                        <table className="w-full">
                          <thead>
                            <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                              {[
                                { key: 'symbol', label: 'Symbol', align: 'left' },
                                { key: 'name', label: 'Name', align: 'left' },
                                { key: 'quantity', label: 'Menge (Invest.)', align: 'right' },
                                { key: 'avg_price', label: 'Kaufpreis', align: 'right' },
                                { key: 'current_price', label: 'Aktuell', align: 'right' },
                                { key: 'tsl', label: 'SL', align: 'right' },
                                { key: 'total_return_pct', label: 'Rendite (Wert)', align: 'right' },
                                { key: 'buy_date', label: 'Kaufdatum', align: 'left' },
                                { key: 'is_live', label: 'Live', align: 'center' },
                                { key: 'market_cap', label: 'MCap', align: 'right' },
                                { key: 'chart', label: 'Chart', align: 'center' }
                              ].map(col => (
                                <th
                                  key={col.key}
                                  className={`p-2 ${col.key !== 'chart' ? 'cursor-pointer hover:text-emerald-400' : ''} select-none ${col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : ''}`}
                                  onClick={() => {
                                    if (col.key === 'chart') return
                                    if (traderSortColumn === col.key) {
                                      setTraderSortDir(traderSortDir === 'asc' ? 'desc' : 'asc')
                                    } else {
                                      setTraderSortColumn(col.key)
                                      setTraderSortDir('asc')
                                    }
                                  }}
                                >
                                  {col.label}
                                  {traderSortColumn === col.key && (
                                    <span className="ml-1 text-emerald-400">{traderSortDir === 'asc' ? '↑' : '↓'}</span>
                                  )}
                                </th>
                              ))}
                            </tr>
                          </thead>
                          <tbody>
                            {[...traderPrivatePortfolio.positions]
                              .sort((a, b) => {
                                let valA = a[traderSortColumn]
                                let valB = b[traderSortColumn]
                                if (traderSortColumn === 'buy_date') {
                                  valA = new Date(valA).getTime()
                                  valB = new Date(valB).getTime()
                                }
                                if (typeof valA === 'string') {
                                  return traderSortDir === 'asc' ? valA.localeCompare(valB) : valB.localeCompare(valA)
                                }
                                return traderSortDir === 'asc' ? valA - valB : valB - valA
                              })
                              .map((pos) => {
                                const currentValue = (pos.current_price || 0) * (pos.quantity || 0)
                                const stopPrice = pos.stop_loss_price || 0
                                const isNearStop = pos.current_price && stopPrice && (pos.current_price - stopPrice) / pos.current_price < 0.05
                                return (
                                  <tr key={pos.id} className="border-b border-dark-700/50 hover:bg-dark-700/30">
                                    <td className="p-2 font-medium text-white cursor-pointer hover:text-emerald-400" onClick={() => setSelectedPosition({ symbol: pos.symbol, name: pos.name, mode: 'trader' })}>{pos.symbol}</td>
                                    <td className="p-2 text-gray-400 text-sm cursor-pointer hover:text-emerald-400" onClick={() => setSelectedPosition({ symbol: pos.symbol, name: pos.name, mode: 'trader' })}>{pos.name}</td>
                                    <td className="p-2 text-right text-gray-300">
                                      {pos.quantity?.toFixed(2)}
                                      <span className="text-gray-500 text-xs ml-1">({formatPrice(pos.invested_eur || pos.avg_price * pos.quantity)})</span>
                                    </td>
                                    <td className="p-2 text-right text-gray-300">{formatPrice(pos.avg_price)}</td>
                                    <td className="p-2 text-right text-white">{formatPrice(pos.current_price)}</td>
                                    <td className={`p-2 text-right ${isNearStop ? 'text-yellow-400' : 'text-red-400'}`}>
                                      {formatPrice(stopPrice)}
                                    </td>
                                    <td className={`p-2 text-right font-medium ${
                                      pos.total_return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                                    }`}>
                                      {pos.total_return_pct >= 0 ? '+' : ''}{pos.total_return_pct?.toFixed(2)}%
                                      <span className="text-gray-400 text-xs ml-1">({formatPrice(currentValue)})</span>
                                    </td>
                                    <td className="p-2 text-gray-500 text-sm">
                                      {new Date(pos.buy_date).toLocaleDateString('de-DE')}
                                    </td>
                                    <td className="p-2 text-center">
                                      {pos.is_live && <span className="px-1.5 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded">LIVE</span>}
                                    </td>
                                    <td className="p-2 text-center">
                                      <a href={`https://www.tradingview.com/chart/?symbol=${pos.symbol}`} target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:text-blue-300" title="TradingView Chart">
                                        <svg className="w-4 h-4 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
                                      </a>
                                    </td>
                                  </tr>
                                )
                              })}
                          </tbody>
                        </table>
                        <div className="mt-3 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Gesamt investiert (inkl. geschlossen):</span>
                          <span className="text-white font-medium">{formatPrice(traderPrivatePortfolio.overall_invested || traderPrivatePortfolio.total_invested)}</span>
                        </div>
                        <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Aktueller Wert (offen):</span>
                          <span className="text-white font-medium">{formatPrice(traderPrivatePortfolio.total_value)}</span>
                        </div>
                        {(traderPrivatePortfolio.realized_pl !== undefined && traderPrivatePortfolio.realized_pl !== 0) && (
                          <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                            <span className="text-gray-400">Realisiert (geschlossene Trades):</span>
                            <span className={`font-medium ${traderPrivatePortfolio.realized_pl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {traderPrivatePortfolio.realized_pl >= 0 ? '+' : ''}{formatPrice(traderPrivatePortfolio.realized_pl)}
                            </span>
                          </div>
                        )}
                        <div className="mt-2 p-3 bg-dark-800 rounded-lg flex justify-between text-sm">
                          <span className="text-gray-400">Gesamt Rendite:</span>
                          <span className={`font-medium ${
                            (traderPrivatePortfolio.overall_return || traderPrivatePortfolio.total_return) >= 0 ? 'text-green-400' : 'text-red-400'
                          }`}>
                            {formatPrice(traderPrivatePortfolio.overall_return || traderPrivatePortfolio.total_return)}
                            ({traderPrivatePortfolio.total_return_pct >= 0 ? '+' : ''}{traderPrivatePortfolio.total_return_pct?.toFixed(2)}%)
                          </span>
                        </div>
                      </div>
                    ) : (
                      <div className="text-center py-8 text-gray-500">
                        <svg className="w-12 h-12 mx-auto mb-3 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                        </svg>
                        <p>Keine simulierten Positionen (is_live = false)</p>
                        <p className="text-xs mt-1">Backtest-Trades ohne "Live" Flag erscheinen hier</p>
                      </div>
                    )}

                    {/* Trader Trade History */}
                    <div className="mt-4 rounded-xl border border-dark-600 overflow-hidden">
                      <button
                        onClick={() => setShowTraderTradeHistory(!showTraderTradeHistory)}
                        className="w-full p-4 flex items-center justify-between hover:bg-dark-700/50 transition-colors"
                      >
                        <h3 className="text-sm font-semibold text-white">Trade History ({traderCompletedTrades.length})</h3>
                        <svg className={`w-5 h-5 text-gray-400 transition-transform ${showTraderTradeHistory ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                      </button>
                      {showTraderTradeHistory && (
                        <div className="border-t border-dark-600">
                          {traderCompletedTrades.length === 0 ? (
                            <div className="p-8 text-center text-gray-500">Noch keine abgeschlossenen Trades</div>
                          ) : (
                            <div className="overflow-x-auto">
                              <table className="w-full text-sm">
                                <thead>
                                  <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                                    <th className="pt-4 pb-3 px-4">Symbol</th>
                                    <th className="pt-4 pb-3 px-4">Kauf</th>
                                    <th className="pt-4 pb-3 px-4">Verkauf</th>
                                    <th className="pt-4 pb-3 px-4 text-right">Rendite</th>
                                  </tr>
                                </thead>
                                <tbody>
                                  {traderCompletedTrades.map((trade) => (
                                    <tr key={trade.id} className={`border-b border-dark-700/50 last:border-0 ${trade.is_live ? 'bg-green-500/5' : ''}`}>
                                      <td className="py-3 px-4">
                                        <div className="flex items-center gap-2">
                                          <div className="font-medium text-white">{trade.symbol}</div>
                                          {trade.is_live && <span className="px-1.5 py-0.5 bg-green-500 text-white text-[10px] font-bold rounded">LIVE</span>}
                                        </div>
                                      </td>
                                      <td className="py-3 px-4">
                                        <div className="text-gray-300">{formatPrice(trade.buy_price)}</div>
                                        <div className="text-xs text-gray-500">{formatDateShort(trade.buy_date)}</div>
                                      </td>
                                      <td className="py-3 px-4">
                                        <div className="text-gray-300">{formatPrice(trade.sell_price)}</div>
                                        <div className="text-xs text-gray-500">{formatDateShort(trade.sell_date)}</div>
                                      </td>
                                      <td className={`py-3 px-4 text-right font-medium ${trade.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                        <div>{formatPercent(trade.profit_loss_pct)}</div>
                                        <div className="text-xs">{trade.profit_loss >= 0 ? '+' : ''}{formatPrice(trade.profit_loss)}</div>
                                      </td>
                                    </tr>
                                  ))}
                                </tbody>
                              </table>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                )}

                {/* Pending Trades Section - nur für Flipper und Lutz, Quant führt direkt aus */}
                {((botTab === 'flipper' && flipperPendingTrades.length > 0) || (botTab === 'lutz' && lutzPendingTrades.length > 0)) && (
                  <div className={`p-4 rounded-xl ${
                    botTab === 'flipper'
                      ? 'bg-yellow-500/10 border border-yellow-500/30'
                      : 'bg-yellow-500/10 border border-yellow-500/30'
                  }`}>
                    <div className="flex items-center justify-between mb-4">
                      <div>
                        <h3 className="text-sm font-medium text-yellow-300 flex items-center gap-2">
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                          </svg>
                          Ausstehende Trades ({(botTab === 'flipper' ? flipperPendingTrades : lutzPendingTrades).length})
                        </h3>
                        <p className="text-xs text-gray-500 mt-1">
                          Trades warten auf Freigabe. Akzeptierte Trades werden öffentlich sichtbar.
                        </p>
                      </div>
                      <button
                        onClick={() => handleAcceptAllTrades(getBotApiName())}
                        className="px-4 py-2 bg-green-500 text-white rounded-lg font-medium hover:bg-green-400 text-sm flex items-center gap-2"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                        </svg>
                        Alle akzeptieren
                      </button>
                    </div>
                    <div className="overflow-x-auto max-h-[300px] overflow-y-auto">
                      <table className="w-full">
                        <thead className="sticky top-0 bg-dark-900/80">
                          <tr className="text-left text-xs text-gray-500 border-b border-dark-600">
                            <th className="p-2">Datum</th>
                            <th className="p-2">Symbol</th>
                            <th className="p-2">Typ</th>
                            <th className="p-2 text-right">Anzahl</th>
                            <th className="p-2 text-right">Preis</th>
                            <th className="p-2 text-right">P/L</th>
                            <th className="p-2"></th>
                          </tr>
                        </thead>
                        <tbody>
                          {(botTab === 'flipper' ? flipperPendingTrades : lutzPendingTrades).map((trade) => (
                            <tr key={trade.id} className="border-b border-dark-700 hover:bg-dark-700/50">
                              <td className="p-2 text-sm text-gray-400">
                                {new Date(trade.signal_date).toLocaleDateString('de-DE')}
                              </td>
                              <td className="p-2 text-sm font-medium text-white">{trade.symbol}</td>
                              <td className="p-2">
                                <span className={`text-xs px-2 py-1 rounded ${
                                  trade.action === 'BUY' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
                                }`}>
                                  {trade.action}
                                </span>
                                {trade.is_stop_loss && <span className="ml-1 px-1.5 py-0.5 bg-red-500/20 text-red-400 text-[10px] rounded font-medium">SL</span>}
                                {trade.is_filter_blocked && (
                                  <span className="ml-1 px-1.5 py-0.5 bg-yellow-500/20 text-yellow-400 text-[10px] rounded font-medium cursor-help" title={trade.filter_block_reason}>
                                    FILTER
                                  </span>
                                )}
                              </td>
                              <td className="p-2 text-sm text-gray-300 text-right">{trade.quantity?.toFixed(4)}</td>
                              <td className="p-2 text-sm text-gray-300 text-right">{formatPrice(trade.price)}</td>
                              <td className="p-2 text-sm text-right">
                                {trade.profit_loss_pct != null ? (
                                  <span className={trade.profit_loss_pct >= 0 ? 'text-green-400' : 'text-red-400'}>
                                    {trade.profit_loss_pct >= 0 ? '+' : ''}{trade.profit_loss_pct.toFixed(2)}%
                                  </span>
                                ) : '-'}
                              </td>
                              <td className="p-2 text-right">
                                <button
                                  onClick={() => handleAcceptTrade(getBotApiName(), trade.id)}
                                  disabled={acceptingTrade === trade.id}
                                  className="px-3 py-1 bg-green-500 text-white rounded-lg text-xs font-medium hover:bg-green-400 disabled:opacity-50 flex items-center gap-1 ml-auto"
                                >
                                  {acceptingTrade === trade.id ? (
                                    <div className="w-3 h-3 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                                  ) : (
                                    <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                                    </svg>
                                  )}
                                  Accept
                                </button>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                )}

                {/* Manual Trade Form (Quant only) */}
                {botTab === 'quant' && showManualTrade && (
                  <div className="bg-dark-800 rounded-xl border border-violet-500/30 overflow-hidden p-4 mb-4">
                    <h3 className="text-sm font-semibold text-violet-300 mb-3">Manuellen Trade erstellen</h3>

                    {/* Stock Search */}
                    {!manualTrade.symbol ? (
                      <div className="relative">
                        <input
                          type="text"
                          placeholder="Aktie suchen..."
                          value={manualTradeSearch}
                          onChange={(e) => searchManualTradeStock(e.target.value)}
                          className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white placeholder-gray-600 focus:outline-none focus:border-violet-500"
                          autoFocus
                        />
                        {manualTradeSearching && (
                          <div className="absolute right-3 top-2.5">
                            <div className="w-4 h-4 border-2 border-violet-500 border-t-transparent rounded-full animate-spin"></div>
                          </div>
                        )}
                        {manualTradeResults.length > 0 && (
                          <div className="absolute z-10 w-full mt-1 bg-dark-700 border border-dark-500 rounded-lg max-h-[200px] overflow-y-auto">
                            {manualTradeResults.map((stock) => (
                              <button
                                key={stock.symbol}
                                onClick={() => selectManualTradeStock(stock)}
                                className="w-full px-3 py-2 text-left hover:bg-dark-600 flex items-center justify-between"
                              >
                                <div>
                                  <span className="text-white font-medium">{stock.symbol}</span>
                                  <span className="text-gray-400 text-sm ml-2">{stock.name}</span>
                                </div>
                                <span className="text-gray-500 text-xs">{stock.exchange}</span>
                              </button>
                            ))}
                          </div>
                        )}
                        <div className="flex justify-end mt-2">
                          <button
                            onClick={() => { setShowManualTrade(false); setManualTradeSearch(''); setManualTradeResults([]) }}
                            className="px-3 py-1.5 bg-dark-600 text-gray-400 rounded text-sm hover:bg-dark-500"
                          >
                            Abbrechen
                          </button>
                        </div>
                      </div>
                    ) : (
                      <>
                        {/* Selected stock header */}
                        <div className="flex items-center gap-2 mb-4">
                          <span className="font-semibold text-white">{manualTrade.symbol}</span>
                          <span className="text-gray-500 text-sm truncate">{manualTrade.name}</span>
                          <button
                            onClick={() => setManualTrade({ ...manualTrade, symbol: '', name: '' })}
                            className="text-gray-500 hover:text-gray-300 ml-auto text-xs"
                          >
                            Andere Aktie
                          </button>
                        </div>

                        {/* Trade form fields */}
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-3">
                          <div>
                            <label className="block text-xs text-gray-500 mb-1">Aktion</label>
                            <select
                              value={manualTrade.action}
                              onChange={(e) => setManualTrade({...manualTrade, action: e.target.value})}
                              className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white focus:outline-none focus:border-violet-500"
                            >
                              <option value="BUY">BUY</option>
                              <option value="SELL">SELL</option>
                            </select>
                          </div>
                          <div>
                            <label className="block text-xs text-gray-500 mb-1">Kaufkurs ({currencySymbol}) *</label>
                            <input
                              type="number"
                              step="0.01"
                              required
                              placeholder="0.00"
                              value={manualTrade.price}
                              onChange={(e) => setManualTrade({...manualTrade, price: e.target.value})}
                              className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white placeholder-gray-600 focus:outline-none focus:border-violet-500"
                            />
                          </div>
                          <div>
                            <label className="block text-xs text-gray-500 mb-1">Anzahl</label>
                            <input
                              type="number"
                              step="0.0001"
                              placeholder="auto (100 EUR)"
                              value={manualTrade.quantity}
                              onChange={(e) => setManualTrade({...manualTrade, quantity: e.target.value})}
                              className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white placeholder-gray-600 focus:outline-none focus:border-violet-500"
                            />
                          </div>
                          <div>
                            <label className="block text-xs text-gray-500 mb-1">Datum</label>
                            <input
                              type="date"
                              value={manualTrade.date}
                              onChange={(e) => setManualTrade({...manualTrade, date: e.target.value})}
                              className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white focus:outline-none focus:border-violet-500"
                            />
                          </div>
                        </div>

                        {/* Actions */}
                        <div className="flex items-center gap-3">
                          <button
                            onClick={() => setManualTrade({...manualTrade, is_live: !manualTrade.is_live})}
                            className={`px-4 py-2 rounded-lg text-sm font-bold transition-colors ${
                              manualTrade.is_live
                                ? 'bg-green-500 text-white'
                                : 'bg-dark-600 text-gray-400'
                            }`}
                          >
                            {manualTrade.is_live ? 'LIVE' : 'SIM'}
                          </button>
                          <button
                            onClick={handleCreateManualTrade}
                            disabled={creatingManualTrade || !manualTrade.price}
                            className="px-4 py-2 bg-violet-500 text-white rounded-lg text-sm font-medium hover:bg-violet-600 disabled:opacity-50"
                          >
                            {creatingManualTrade ? 'Erstelle...' : 'Hinzufügen'}
                          </button>
                          <button
                            onClick={() => { setShowManualTrade(false); setManualTrade({ symbol: '', name: '', action: 'BUY', price: '', quantity: '', date: '', is_live: false }) }}
                            className="px-4 py-2 bg-dark-600 text-gray-300 rounded-lg hover:bg-dark-500 text-sm"
                          >
                            Abbrechen
                          </button>
                        </div>
                      </>
                    )}
                  </div>
                )}

                {/* Manual Trade Form (Ditz only) */}
                {botTab === 'ditz' && showDitzManualTrade && (
                  <div className="bg-dark-800 rounded-xl border border-cyan-500/30 overflow-hidden p-4 mb-4">
                    <h3 className="text-sm font-semibold text-cyan-300 mb-3">Manuellen Trade erstellen</h3>

                    {/* Stock Search */}
                    {!manualTrade.symbol ? (
                      <div className="relative">
                        <input
                          type="text"
                          placeholder="Aktie suchen..."
                          value={manualTradeSearch}
                          onChange={(e) => searchManualTradeStock(e.target.value)}
                          className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white placeholder-gray-600 focus:outline-none focus:border-cyan-500"
                          autoFocus
                        />
                        {manualTradeSearching && (
                          <div className="absolute right-3 top-2.5">
                            <div className="w-4 h-4 border-2 border-cyan-500 border-t-transparent rounded-full animate-spin"></div>
                          </div>
                        )}
                        {manualTradeResults.length > 0 && (
                          <div className="absolute z-10 w-full mt-1 bg-dark-700 border border-dark-500 rounded-lg max-h-[200px] overflow-y-auto">
                            {manualTradeResults.map((stock) => (
                              <button
                                key={stock.symbol}
                                onClick={() => selectManualTradeStock(stock)}
                                className="w-full px-3 py-2 text-left hover:bg-dark-600 flex items-center justify-between"
                              >
                                <div>
                                  <span className="text-white font-medium">{stock.symbol}</span>
                                  <span className="text-gray-400 text-sm ml-2">{stock.name}</span>
                                </div>
                                <span className="text-gray-500 text-xs">{stock.exchange}</span>
                              </button>
                            ))}
                          </div>
                        )}
                        <div className="flex justify-end mt-2">
                          <button
                            onClick={() => { setShowDitzManualTrade(false); setManualTradeSearch(''); setManualTradeResults([]) }}
                            className="px-3 py-1.5 bg-dark-600 text-gray-400 rounded text-sm hover:bg-dark-500"
                          >
                            Abbrechen
                          </button>
                        </div>
                      </div>
                    ) : (
                      <>
                        {/* Selected stock header */}
                        <div className="flex items-center gap-2 mb-4">
                          <span className="font-semibold text-white">{manualTrade.symbol}</span>
                          <span className="text-gray-500 text-sm truncate">{manualTrade.name}</span>
                          <button
                            onClick={() => setManualTrade({ ...manualTrade, symbol: '', name: '' })}
                            className="text-gray-500 hover:text-gray-300 ml-auto text-xs"
                          >
                            Andere Aktie
                          </button>
                        </div>

                        {/* Trade form fields */}
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-3">
                          <div>
                            <label className="block text-xs text-gray-500 mb-1">Aktion</label>
                            <select
                              value={manualTrade.action}
                              onChange={(e) => setManualTrade({...manualTrade, action: e.target.value})}
                              className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white focus:outline-none focus:border-cyan-500"
                            >
                              <option value="BUY">BUY</option>
                              <option value="SELL">SELL</option>
                            </select>
                          </div>
                          <div>
                            <label className="block text-xs text-gray-500 mb-1">Kaufkurs ({currencySymbol}) *</label>
                            <input
                              type="number"
                              step="0.01"
                              required
                              placeholder="0.00"
                              value={manualTrade.price}
                              onChange={(e) => setManualTrade({...manualTrade, price: e.target.value})}
                              className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white placeholder-gray-600 focus:outline-none focus:border-cyan-500"
                            />
                          </div>
                          <div>
                            <label className="block text-xs text-gray-500 mb-1">Anzahl</label>
                            <input
                              type="number"
                              step="0.0001"
                              placeholder="auto (100 EUR)"
                              value={manualTrade.quantity}
                              onChange={(e) => setManualTrade({...manualTrade, quantity: e.target.value})}
                              className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white placeholder-gray-600 focus:outline-none focus:border-cyan-500"
                            />
                          </div>
                          <div>
                            <label className="block text-xs text-gray-500 mb-1">Datum</label>
                            <input
                              type="date"
                              value={manualTrade.date}
                              onChange={(e) => setManualTrade({...manualTrade, date: e.target.value})}
                              className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white focus:outline-none focus:border-cyan-500"
                            />
                          </div>
                        </div>

                        {/* Actions */}
                        <div className="flex items-center gap-3">
                          <button
                            onClick={() => setManualTrade({...manualTrade, is_live: !manualTrade.is_live})}
                            className={`px-4 py-2 rounded-lg text-sm font-bold transition-colors ${
                              manualTrade.is_live
                                ? 'bg-green-500 text-white'
                                : 'bg-dark-600 text-gray-400'
                            }`}
                          >
                            {manualTrade.is_live ? 'LIVE' : 'SIM'}
                          </button>
                          <button
                            onClick={handleCreateDitzManualTrade}
                            disabled={creatingManualTrade || !manualTrade.price}
                            className="px-4 py-2 bg-cyan-500 text-white rounded-lg text-sm font-medium hover:bg-cyan-600 disabled:opacity-50"
                          >
                            {creatingManualTrade ? 'Erstelle...' : 'Hinzufügen'}
                          </button>
                          <button
                            onClick={() => { setShowDitzManualTrade(false); setManualTrade({ symbol: '', name: '', action: 'BUY', price: '', quantity: '', date: '', is_live: false }) }}
                            className="px-4 py-2 bg-dark-600 text-gray-300 rounded-lg hover:bg-dark-500 text-sm"
                          >
                            Abbrechen
                          </button>
                        </div>
                      </>
                    )}
                  </div>
                )}

                {/* Manual Trade Form (Trader only) */}
                {botTab === 'trader' && showTraderManualTrade && (
                  <div className="bg-dark-800 rounded-xl border border-emerald-500/30 overflow-hidden p-4 mb-4">
                    <h3 className="text-sm font-semibold text-emerald-300 mb-3">Manuellen Trade erstellen</h3>

                    {/* Stock Search */}
                    {!manualTrade.symbol ? (
                      <div className="relative">
                        <input
                          type="text"
                          placeholder="Aktie suchen..."
                          value={manualTradeSearch}
                          onChange={(e) => searchManualTradeStock(e.target.value)}
                          className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white placeholder-gray-600 focus:outline-none focus:border-emerald-500"
                          autoFocus
                        />
                        {manualTradeSearching && (
                          <div className="absolute right-3 top-2.5">
                            <div className="w-4 h-4 border-2 border-emerald-500 border-t-transparent rounded-full animate-spin"></div>
                          </div>
                        )}
                        {manualTradeResults.length > 0 && (
                          <div className="absolute z-10 w-full mt-1 bg-dark-700 border border-dark-500 rounded-lg max-h-[200px] overflow-y-auto">
                            {manualTradeResults.map((stock) => (
                              <button
                                key={stock.symbol}
                                onClick={() => selectManualTradeStock(stock)}
                                className="w-full px-3 py-2 text-left hover:bg-dark-600 flex items-center justify-between"
                              >
                                <div>
                                  <span className="text-white font-medium">{stock.symbol}</span>
                                  <span className="text-gray-400 text-sm ml-2">{stock.name}</span>
                                </div>
                                <span className="text-gray-500 text-xs">{stock.exchange}</span>
                              </button>
                            ))}
                          </div>
                        )}
                        <div className="flex justify-end mt-2">
                          <button
                            onClick={() => { setShowTraderManualTrade(false); setManualTradeSearch(''); setManualTradeResults([]) }}
                            className="px-3 py-1.5 bg-dark-600 text-gray-400 rounded text-sm hover:bg-dark-500"
                          >
                            Abbrechen
                          </button>
                        </div>
                      </div>
                    ) : (
                      <>
                        {/* Selected stock header */}
                        <div className="flex items-center gap-2 mb-4">
                          <span className="font-semibold text-white">{manualTrade.symbol}</span>
                          <span className="text-gray-500 text-sm truncate">{manualTrade.name}</span>
                          <button
                            onClick={() => setManualTrade({ ...manualTrade, symbol: '', name: '' })}
                            className="text-gray-500 hover:text-gray-300 ml-auto text-xs"
                          >
                            Andere Aktie
                          </button>
                        </div>

                        {/* Trade form fields */}
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-3">
                          <div>
                            <label className="block text-xs text-gray-500 mb-1">Aktion</label>
                            <select
                              value={manualTrade.action}
                              onChange={(e) => setManualTrade({...manualTrade, action: e.target.value})}
                              className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white focus:outline-none focus:border-emerald-500"
                            >
                              <option value="BUY">BUY</option>
                              <option value="SELL">SELL</option>
                            </select>
                          </div>
                          <div>
                            <label className="block text-xs text-gray-500 mb-1">Kaufkurs ({currencySymbol}) *</label>
                            <input
                              type="number"
                              step="0.01"
                              required
                              placeholder="0.00"
                              value={manualTrade.price}
                              onChange={(e) => setManualTrade({...manualTrade, price: e.target.value})}
                              className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white placeholder-gray-600 focus:outline-none focus:border-emerald-500"
                            />
                          </div>
                          <div>
                            <label className="block text-xs text-gray-500 mb-1">Anzahl</label>
                            <input
                              type="number"
                              step="0.0001"
                              placeholder="auto (100 EUR)"
                              value={manualTrade.quantity}
                              onChange={(e) => setManualTrade({...manualTrade, quantity: e.target.value})}
                              className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white placeholder-gray-600 focus:outline-none focus:border-emerald-500"
                            />
                          </div>
                          <div>
                            <label className="block text-xs text-gray-500 mb-1">Datum</label>
                            <input
                              type="date"
                              value={manualTrade.date}
                              onChange={(e) => setManualTrade({...manualTrade, date: e.target.value})}
                              className="w-full px-3 py-2 bg-dark-700 border border-dark-500 rounded-lg text-white focus:outline-none focus:border-emerald-500"
                            />
                          </div>
                        </div>

                        {/* Actions */}
                        <div className="flex items-center gap-3">
                          <button
                            onClick={() => setManualTrade({...manualTrade, is_live: !manualTrade.is_live})}
                            className={`px-4 py-2 rounded-lg text-sm font-bold transition-colors ${
                              manualTrade.is_live
                                ? 'bg-green-500 text-white'
                                : 'bg-dark-600 text-gray-400'
                            }`}
                          >
                            {manualTrade.is_live ? 'LIVE' : 'SIM'}
                          </button>
                          <button
                            onClick={handleCreateTraderManualTrade}
                            disabled={creatingManualTrade || !manualTrade.price}
                            className="px-4 py-2 bg-emerald-500 text-white rounded-lg text-sm font-medium hover:bg-emerald-600 disabled:opacity-50"
                          >
                            {creatingManualTrade ? 'Erstelle...' : 'Hinzufügen'}
                          </button>
                          <button
                            onClick={() => { setShowTraderManualTrade(false); setManualTrade({ symbol: '', name: '', action: 'BUY', price: '', quantity: '', date: '', is_live: false }) }}
                            className="px-4 py-2 bg-dark-600 text-gray-300 rounded-lg hover:bg-dark-500 text-sm"
                          >
                            Abbrechen
                          </button>
                        </div>
                      </>
                    )}
                  </div>
                )}

                {/* Trades Table */}
                <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
                  <div className="p-4 border-b border-dark-600 flex items-center justify-between">
                    <div>
                      <h2 className="text-lg font-semibold text-white">
                        {botTab === 'flipper' ? 'FlipperBot' : botTab === 'lutz' ? 'Lutz' : botTab === 'ditz' ? 'Ditz' : botTab === 'trader' ? 'Trader' : 'Quant'} Trades
                        <span className="text-sm font-normal text-gray-500 ml-2">
                          ({(botTab === 'flipper' ? flipperTrades : botTab === 'lutz' ? lutzTrades : botTab === 'ditz' ? ditzTrades : botTab === 'trader' ? traderTrades : quantTrades).length})
                        </span>
                      </h2>
                      <p className="text-xs text-gray-500">BUY-Trades bearbeiten um Position zu aktualisieren</p>
                    </div>
                    <div className="flex items-center gap-2">
                      {botTab === 'flipper' && (
                        <div className="flex items-center gap-1">
                          <button
                            onClick={markAllFlipperTradesRead}
                            className="px-2 py-1 text-xs text-gray-400 hover:text-blue-400 transition-colors"
                            title="Alle als gelesen markieren"
                          >
                            Alle gelesen
                          </button>
                          <span className="text-dark-600">|</span>
                          <button
                            onClick={markAllFlipperTradesUnread}
                            className="px-2 py-1 text-xs text-gray-400 hover:text-blue-400 transition-colors"
                            title="Alle als ungelesen markieren"
                          >
                            Alle ungelesen
                          </button>
                        </div>
                      )}
                      {botTab === 'lutz' && (
                        <div className="flex items-center gap-1">
                          <button
                            onClick={markAllLutzTradesRead}
                            className="px-2 py-1 text-xs text-gray-400 hover:text-orange-400 transition-colors"
                            title="Alle als gelesen markieren"
                          >
                            Alle gelesen
                          </button>
                          <span className="text-dark-600">|</span>
                          <button
                            onClick={markAllLutzTradesUnread}
                            className="px-2 py-1 text-xs text-gray-400 hover:text-orange-400 transition-colors"
                            title="Alle als ungelesen markieren"
                          >
                            Alle ungelesen
                          </button>
                        </div>
                      )}
                      {botTab === 'quant' && (
                        <div className="flex items-center gap-1">
                          <button
                            onClick={handleMarkAllTradesRead}
                            className="px-2 py-1 text-xs text-gray-400 hover:text-violet-400 transition-colors"
                            title="Alle als gelesen markieren"
                          >
                            Alle gelesen
                          </button>
                          <span className="text-dark-600">|</span>
                          <button
                            onClick={handleMarkAllTradesUnread}
                            className="px-2 py-1 text-xs text-gray-400 hover:text-violet-400 transition-colors"
                            title="Alle als ungelesen markieren"
                          >
                            Alle ungelesen
                          </button>
                        </div>
                      )}
                      {botTab === 'ditz' && (
                        <div className="flex items-center gap-1">
                          <button
                            onClick={markAllDitzTradesRead}
                            className="px-2 py-1 text-xs text-gray-400 hover:text-cyan-400 transition-colors"
                            title="Alle als gelesen markieren"
                          >
                            Alle gelesen
                          </button>
                          <span className="text-dark-600">|</span>
                          <button
                            onClick={markAllDitzTradesUnread}
                            className="px-2 py-1 text-xs text-gray-400 hover:text-cyan-400 transition-colors"
                            title="Alle als ungelesen markieren"
                          >
                            Alle ungelesen
                          </button>
                        </div>
                      )}
                      {botTab === 'quant' && !showManualTrade && (
                        <button
                          onClick={() => setShowManualTrade(true)}
                          className="px-3 py-1.5 bg-violet-500/20 text-violet-400 rounded-lg text-sm font-medium hover:bg-violet-500/30 flex items-center gap-1"
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                          </svg>
                          Manueller Trade
                        </button>
                      )}
                      {botTab === 'ditz' && !showDitzManualTrade && (
                        <button
                          onClick={() => setShowDitzManualTrade(true)}
                          className="px-3 py-1.5 bg-cyan-500/20 text-cyan-400 rounded-lg text-sm font-medium hover:bg-cyan-500/30 flex items-center gap-1"
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                          </svg>
                          Manueller Trade
                        </button>
                      )}
                      {botTab === 'trader' && (
                        <div className="flex items-center gap-1">
                          <button
                            onClick={markAllTraderTradesRead}
                            className="px-2 py-1 text-xs text-gray-400 hover:text-emerald-400 transition-colors"
                            title="Alle als gelesen markieren"
                          >
                            Alle gelesen
                          </button>
                          <span className="text-dark-600">|</span>
                          <button
                            onClick={markAllTraderTradesUnread}
                            className="px-2 py-1 text-xs text-gray-400 hover:text-emerald-400 transition-colors"
                            title="Alle als ungelesen markieren"
                          >
                            Alle ungelesen
                          </button>
                        </div>
                      )}
                      {botTab === 'trader' && !showTraderManualTrade && (
                        <button
                          onClick={() => setShowTraderManualTrade(true)}
                          className="px-3 py-1.5 bg-emerald-500/20 text-emerald-400 rounded-lg text-sm font-medium hover:bg-emerald-500/30 flex items-center gap-1"
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                          </svg>
                          Manueller Trade
                        </button>
                      )}
                    </div>
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
                        {(() => {
                          const trades = botTab === 'flipper' ? flipperTrades : botTab === 'lutz' ? lutzTrades : botTab === 'ditz' ? ditzTrades : botTab === 'trader' ? traderTrades : quantTrades
                          return ((botTab === 'quant' || botTab === 'ditz' || botTab === 'trader') ? getGroupedTrades(trades) : trades).slice(0, 50)
                        })().map((trade) => (
                          <tr key={trade.id} className={`border-b border-dark-700/50 hover:bg-dark-700/30 ${
                            trade.is_deleted ? 'opacity-50' : ''
                          } ${trade.is_filter_blocked ? 'opacity-60 bg-yellow-500/5' : ''
                          } ${trade.is_live && !trade.is_deleted ? 'bg-green-500/5' : ''} ${
                            botTab === 'flipper' && !trade.is_read && !trade.is_deleted ? 'bg-blue-500/5 border-l-2 border-l-blue-500' : ''
                          } ${
                            botTab === 'lutz' && !trade.is_read && !trade.is_deleted ? 'bg-orange-500/5 border-l-2 border-l-orange-500' : ''
                          } ${
                            botTab === 'quant' && !trade.is_read && !trade.is_deleted ? 'bg-violet-500/5 border-l-2 border-l-violet-500' : ''
                          } ${
                            botTab === 'ditz' && !trade.is_read && !trade.is_deleted ? 'bg-cyan-500/5 border-l-2 border-l-cyan-500' : ''
                          } ${
                            botTab === 'trader' && !trade.is_read && !trade.is_deleted ? 'bg-emerald-500/5 border-l-2 border-l-emerald-500' : ''
                          }`}>
                            {editingItem?.type === 'trade' && editingItem?.id === trade.id ? (
                              <>
                                <td className="p-3">
                                  <input
                                    type="date"
                                    value={editingItem.signal_date || ''}
                                    onChange={(e) => setEditingItem({...editingItem, signal_date: e.target.value})}
                                    className="w-32 bg-dark-700 border border-dark-500 rounded px-2 py-1 text-white text-sm"
                                  />
                                </td>
                                <td className="p-3 font-medium text-white">{trade.symbol}</td>
                                <td className="p-3">
                                  <span className={`px-2 py-1 text-xs rounded ${
                                    trade.action === 'BUY' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
                                  }`}>
                                    {trade.action}
                                  </span>
                                  {trade.is_stop_loss && <span className="ml-1 px-1.5 py-0.5 bg-red-500/20 text-red-400 text-[10px] rounded font-medium">SL</span>}
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
                                      onClick={() => handleUpdateTrade(getBotApiName(), editingItem)}
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
                                <td className={`p-3 text-gray-400 text-sm ${trade.is_deleted ? 'line-through' : ''}`}>
                                  {new Date(trade.signal_date || trade.created_at).toLocaleDateString('de-DE')}
                                </td>
                                <td className={`p-3 font-medium ${trade.is_deleted ? 'text-gray-500 line-through' : 'text-white'}`}>{trade.symbol}</td>
                                <td className="p-3">
                                  <span className={`px-2 py-1 text-xs rounded ${trade.is_deleted ? 'line-through opacity-60' : ''} ${
                                    trade.action === 'BUY' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
                                  }`}>
                                    {trade.action}
                                  </span>
                                  {trade.is_stop_loss && <span className="ml-1 px-1.5 py-0.5 bg-red-500/20 text-red-400 text-[10px] rounded font-medium">SL</span>}
                                  {trade.is_filter_blocked && (
                                    <span className="ml-1 px-1.5 py-0.5 bg-yellow-500/20 text-yellow-400 text-[10px] rounded font-medium cursor-help" title={trade.filter_block_reason}>
                                      FILTER
                                    </span>
                                  )}
                                </td>
                                <td className={`p-3 text-right ${trade.is_deleted ? 'text-gray-500 line-through' : 'text-white'}`}>{trade.quantity}</td>
                                <td className={`p-3 text-right ${trade.is_deleted ? 'text-gray-500 line-through' : 'text-white'}`}>{formatPrice(trade.price, trade.symbol)}</td>
                                <td className="p-3 text-center">
                                  {trade.is_live && (
                                    <span className={`px-2 py-1 bg-green-500/20 text-green-400 rounded text-xs font-bold ${trade.is_deleted ? 'opacity-60' : ''}`}>
                                      LIVE
                                    </span>
                                  )}
                                </td>
                                <td className="p-3">
                                  <div className="flex items-center gap-1">
                                    {/* Read/Unread toggle - for all bots */}
                                    {botTab === 'flipper' && (
                                      <button
                                        onClick={() => toggleFlipperTradeRead(trade.id)}
                                        className={`p-1.5 transition-colors ${
                                          trade.is_read
                                            ? 'text-gray-600 hover:text-blue-400'
                                            : 'text-blue-400 hover:text-blue-300'
                                        }`}
                                        title={trade.is_read ? 'Als ungelesen markieren' : 'Als gelesen markieren'}
                                      >
                                        {trade.is_read ? (
                                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 19v-8.93a2 2 0 01.89-1.664l7-4.666a2 2 0 012.22 0l7 4.666A2 2 0 0121 10.07V19M3 19a2 2 0 002 2h14a2 2 0 002-2M3 19l6.75-4.5M21 19l-6.75-4.5M3 10l6.75 4.5M21 10l-6.75 4.5m0 0l-1.14.76a2 2 0 01-2.22 0l-1.14-.76" />
                                          </svg>
                                        ) : (
                                          <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                                            <path d="M3 3l18 18M20.94 11c.04.33.06.66.06 1 0 5.52-4.48 10-10 10S1 17.52 1 12 5.48 2 11 2c2.04 0 3.93.61 5.51 1.66L20.94 11zM12 20c4.41 0 8-3.59 8-8 0-.05 0-.1 0-.15L12.15 4H12c-4.41 0-8 3.59-8 8s3.59 8 8 8z" />
                                            <circle cx="12" cy="12" r="3" />
                                          </svg>
                                        )}
                                      </button>
                                    )}
                                    {botTab === 'lutz' && (
                                      <button
                                        onClick={() => toggleLutzTradeRead(trade.id)}
                                        className={`p-1.5 transition-colors ${
                                          trade.is_read
                                            ? 'text-gray-600 hover:text-orange-400'
                                            : 'text-orange-400 hover:text-orange-300'
                                        }`}
                                        title={trade.is_read ? 'Als ungelesen markieren' : 'Als gelesen markieren'}
                                      >
                                        {trade.is_read ? (
                                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 19v-8.93a2 2 0 01.89-1.664l7-4.666a2 2 0 012.22 0l7 4.666A2 2 0 0121 10.07V19M3 19a2 2 0 002 2h14a2 2 0 002-2M3 19l6.75-4.5M21 19l-6.75-4.5M3 10l6.75 4.5M21 10l-6.75 4.5m0 0l-1.14.76a2 2 0 01-2.22 0l-1.14-.76" />
                                          </svg>
                                        ) : (
                                          <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                                            <path d="M3 3l18 18M20.94 11c.04.33.06.66.06 1 0 5.52-4.48 10-10 10S1 17.52 1 12 5.48 2 11 2c2.04 0 3.93.61 5.51 1.66L20.94 11zM12 20c4.41 0 8-3.59 8-8 0-.05 0-.1 0-.15L12.15 4H12c-4.41 0-8 3.59-8 8s3.59 8 8 8z" />
                                            <circle cx="12" cy="12" r="3" />
                                          </svg>
                                        )}
                                      </button>
                                    )}
                                    {botTab === 'quant' && (
                                      <button
                                        onClick={() => handleToggleTradeRead(trade.id)}
                                        className={`p-1.5 transition-colors ${
                                          trade.is_read
                                            ? 'text-gray-600 hover:text-violet-400'
                                            : 'text-violet-400 hover:text-violet-300'
                                        }`}
                                        title={trade.is_read ? 'Als ungelesen markieren' : 'Als gelesen markieren'}
                                      >
                                        {trade.is_read ? (
                                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 19v-8.93a2 2 0 01.89-1.664l7-4.666a2 2 0 012.22 0l7 4.666A2 2 0 0121 10.07V19M3 19a2 2 0 002 2h14a2 2 0 002-2M3 19l6.75-4.5M21 19l-6.75-4.5M3 10l6.75 4.5M21 10l-6.75 4.5m0 0l-1.14.76a2 2 0 01-2.22 0l-1.14-.76" />
                                          </svg>
                                        ) : (
                                          <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                                            <path d="M3 3l18 18M20.94 11c.04.33.06.66.06 1 0 5.52-4.48 10-10 10S1 17.52 1 12 5.48 2 11 2c2.04 0 3.93.61 5.51 1.66L20.94 11zM12 20c4.41 0 8-3.59 8-8 0-.05 0-.1 0-.15L12.15 4H12c-4.41 0-8 3.59-8 8s3.59 8 8 8z" />
                                            <circle cx="12" cy="12" r="3" />
                                          </svg>
                                        )}
                                      </button>
                                    )}
                                    {botTab === 'ditz' && (
                                      <button
                                        onClick={() => toggleDitzTradeRead(trade.id)}
                                        className={`p-1.5 transition-colors ${
                                          trade.is_read
                                            ? 'text-gray-600 hover:text-cyan-400'
                                            : 'text-cyan-400 hover:text-cyan-300'
                                        }`}
                                        title={trade.is_read ? 'Als ungelesen markieren' : 'Als gelesen markieren'}
                                      >
                                        {trade.is_read ? (
                                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 19v-8.93a2 2 0 01.89-1.664l7-4.666a2 2 0 012.22 0l7 4.666A2 2 0 0121 10.07V19M3 19a2 2 0 002 2h14a2 2 0 002-2M3 19l6.75-4.5M21 19l-6.75-4.5M3 10l6.75 4.5M21 10l-6.75 4.5m0 0l-1.14.76a2 2 0 01-2.22 0l-1.14-.76" />
                                          </svg>
                                        ) : (
                                          <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                                            <path d="M3 3l18 18M20.94 11c.04.33.06.66.06 1 0 5.52-4.48 10-10 10S1 17.52 1 12 5.48 2 11 2c2.04 0 3.93.61 5.51 1.66L20.94 11zM12 20c4.41 0 8-3.59 8-8 0-.05 0-.1 0-.15L12.15 4H12c-4.41 0-8 3.59-8 8s3.59 8 8 8z" />
                                            <circle cx="12" cy="12" r="3" />
                                          </svg>
                                        )}
                                      </button>
                                    )}
                                    {botTab === 'trader' && (
                                      <button
                                        onClick={() => toggleTraderTradeRead(trade.id)}
                                        className={`p-1.5 transition-colors ${
                                          trade.is_read
                                            ? 'text-gray-600 hover:text-emerald-400'
                                            : 'text-emerald-400 hover:text-emerald-300'
                                        }`}
                                        title={trade.is_read ? 'Als ungelesen markieren' : 'Als gelesen markieren'}
                                      >
                                        {trade.is_read ? (
                                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 19v-8.93a2 2 0 01.89-1.664l7-4.666a2 2 0 012.22 0l7 4.666A2 2 0 0121 10.07V19M3 19a2 2 0 002 2h14a2 2 0 002-2M3 19l6.75-4.5M21 19l-6.75-4.5M3 10l6.75 4.5M21 10l-6.75 4.5m0 0l-1.14.76a2 2 0 01-2.22 0l-1.14-.76" />
                                          </svg>
                                        ) : (
                                          <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                                            <path d="M3 3l18 18M20.94 11c.04.33.06.66.06 1 0 5.52-4.48 10-10 10S1 17.52 1 12 5.48 2 11 2c2.04 0 3.93.61 5.51 1.66L20.94 11zM12 20c4.41 0 8-3.59 8-8 0-.05 0-.1 0-.15L12.15 4H12c-4.41 0-8 3.59-8 8s3.59 8 8 8z" />
                                            <circle cx="12" cy="12" r="3" />
                                          </svg>
                                        )}
                                      </button>
                                    )}
                                    {trade.is_stop_loss && !trade.is_deleted && (
                                      <button
                                        onClick={() => {
                                          if (confirm(`Stop Loss für ${trade.symbol} rückgängig machen? Position wird wieder geöffnet.`)) {
                                            handleDeleteTrade(getBotApiName(), trade.id, trade.symbol, trade.action, trade.is_deleted)
                                          }
                                        }}
                                        className="px-2 py-1 bg-orange-500/20 text-orange-400 hover:bg-orange-500/30 text-[10px] rounded font-medium transition-colors"
                                        title="Stop Loss rückgängig machen"
                                      >
                                        Undo SL
                                      </button>
                                    )}
                                    {!trade.is_deleted && (
                                      <button
                                        onClick={() => {
                                          const d = new Date(trade.signal_date || trade.executed_at || trade.created_at)
                                          const dateStr = d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0')
                                          setEditingItem({
                                            type: 'trade',
                                            id: trade.id,
                                            quantity: trade.quantity,
                                            price: convertPrice(trade.price)?.toFixed(2) || trade.price,
                                            is_live: trade.is_live || false,
                                            signal_date: dateStr
                                          })
                                        }}
                                        className="p-1.5 text-gray-400 hover:text-accent-400 transition-colors"
                                        title="Bearbeiten"
                                      >
                                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                                        </svg>
                                      </button>
                                    )}
                                    <button
                                      onClick={() => handleDeleteTrade(
                                        getBotApiName(),
                                        trade.id,
                                        trade.symbol,
                                        trade.action,
                                        trade.is_deleted
                                      )}
                                      className={`p-1.5 transition-colors ${
                                        trade.is_deleted
                                          ? 'text-green-600 hover:text-green-400'
                                          : 'text-gray-400 hover:text-red-400'
                                      }`}
                                      title={trade.is_deleted ? 'Wiederherstellen' : 'Streichen'}
                                    >
                                      {trade.is_deleted ? (
                                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                        </svg>
                                      ) : (
                                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                        </svg>
                                      )}
                                    </button>
                                  </div>
                                </td>
                              </>
                            )}
                          </tr>
                        ))}
                        {(botTab === 'flipper' ? flipperTrades : botTab === 'lutz' ? lutzTrades : botTab === 'ditz' ? ditzTrades : botTab === 'trader' ? traderTrades : quantTrades).length === 0 && (
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

                {/* Bot Reset - collapsible */}
                <div className="bg-dark-800 rounded-xl border border-dark-600">
                  <button
                    onClick={() => setShowBotReset(!showBotReset)}
                    className="w-full p-4 flex items-center justify-between text-sm font-medium text-white"
                  >
                    Bot Reset
                    <svg className={`w-4 h-4 transition-transform ${showBotReset ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                  </button>
                  {showBotReset && (
                    <div className="px-4 pb-4 flex flex-col gap-3">
                      <button
                        onClick={async () => {
                          if (!confirm('ALLE 5 Bots komplett zurücksetzen? Sämtliche Trades, Positionen und Logs werden gelöscht!')) return
                          try {
                            const res = await fetch('/api/bots/reset-all', { method: 'POST', headers: { 'Authorization': `Bearer ${token}` } })
                            if (!res.ok) throw new Error('Reset fehlgeschlagen')
                            alert('Alle Bots wurden zurückgesetzt')
                            window.location.reload()
                          } catch (err) { alert('Fehler beim Zurücksetzen') }
                        }}
                        className="w-full px-4 py-2 bg-red-500/20 text-red-400 rounded-lg hover:bg-red-500/30 transition-colors font-medium text-sm flex items-center justify-center gap-2 border border-red-500/30"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                        Alle Bots zurücksetzen
                      </button>
                      <div className="flex gap-3">
                      <button
                        onClick={async () => {
                          if (!confirm('FlipperBot komplett zurücksetzen? Alle Trades und Positionen werden gelöscht!')) return
                          try {
                            const res = await fetch('/api/flipperbot/reset', { method: 'POST', headers: { 'Authorization': `Bearer ${token}` } })
                            if (!res.ok) throw new Error('Reset fehlgeschlagen')
                            alert('FlipperBot wurde zurückgesetzt')
                            window.location.reload()
                          } catch (err) { alert('Fehler beim Zurücksetzen') }
                        }}
                        className="px-4 py-2 bg-purple-500/20 text-purple-400 rounded-lg hover:bg-purple-500/30 transition-colors font-medium text-sm flex items-center gap-2"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                        FlipperBot Reset
                      </button>
                      <button
                        onClick={async () => {
                          if (!confirm('Lutz komplett zurücksetzen? Alle Trades und Positionen werden gelöscht!')) return
                          try {
                            const res = await fetch('/api/lutz/reset', { method: 'POST', headers: { 'Authorization': `Bearer ${token}` } })
                            if (!res.ok) throw new Error('Reset fehlgeschlagen')
                            alert('Lutz wurde zurückgesetzt')
                            window.location.reload()
                          } catch (err) { alert('Fehler beim Zurücksetzen') }
                        }}
                        className="px-4 py-2 bg-orange-500/20 text-orange-400 rounded-lg hover:bg-orange-500/30 transition-colors font-medium text-sm flex items-center gap-2"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                        Lutz Reset
                      </button>
                      <button
                        onClick={async () => {
                          if (!confirm('Quant komplett zurücksetzen? Alle Trades und Positionen werden gelöscht!')) return
                          try {
                            const res = await fetch('/api/quant/reset', { method: 'POST', headers: { 'Authorization': `Bearer ${token}` } })
                            if (!res.ok) throw new Error('Reset fehlgeschlagen')
                            alert('Quant wurde zurückgesetzt')
                            window.location.reload()
                          } catch (err) { alert('Fehler beim Zurücksetzen') }
                        }}
                        className="px-4 py-2 bg-violet-500/20 text-violet-400 rounded-lg hover:bg-violet-500/30 transition-colors font-medium text-sm flex items-center gap-2"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                        Quant Reset
                      </button>
                      <button
                        onClick={async () => {
                          if (!confirm('Ditz komplett zurücksetzen? Alle Trades und Positionen werden gelöscht!')) return
                          try {
                            const res = await fetch('/api/ditz/reset', { method: 'POST', headers: { 'Authorization': `Bearer ${token}` } })
                            if (!res.ok) throw new Error('Reset fehlgeschlagen')
                            alert('Ditz wurde zurückgesetzt')
                            window.location.reload()
                          } catch (err) { alert('Fehler beim Zurücksetzen') }
                        }}
                        className="px-4 py-2 bg-cyan-500/20 text-cyan-400 rounded-lg hover:bg-cyan-500/30 transition-colors font-medium text-sm flex items-center gap-2"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                        Ditz Reset
                      </button>
                      <button
                        onClick={async () => {
                          if (!confirm('Trader komplett zurücksetzen? Alle Trades und Positionen werden gelöscht!')) return
                          try {
                            const res = await fetch('/api/trader/reset', { method: 'POST', headers: { 'Authorization': `Bearer ${token}` } })
                            if (!res.ok) throw new Error('Reset fehlgeschlagen')
                            alert('Trader wurde zurückgesetzt')
                            window.location.reload()
                          } catch (err) { alert('Fehler beim Zurücksetzen') }
                        }}
                        className="px-4 py-2 bg-emerald-500/20 text-emerald-400 rounded-lg hover:bg-emerald-500/30 transition-colors font-medium text-sm flex items-center gap-2"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                        Trader Reset
                      </button>
                      </div>
                    </div>
                  )}
                </div>

                {/* Rückwirkende Trades - collapsible */}
                <div className={`rounded-xl ${
                  botTab === 'flipper'
                    ? 'bg-purple-500/10 border border-purple-500/30'
                    : botTab === 'lutz'
                    ? 'bg-orange-500/10 border border-orange-500/30'
                    : botTab === 'ditz'
                    ? 'bg-cyan-500/10 border border-cyan-500/30'
                    : botTab === 'trader'
                    ? 'bg-emerald-500/10 border border-emerald-500/30'
                    : 'bg-violet-500/10 border border-violet-500/30'
                }`}>
                  <button
                    onClick={() => setShowBackfill(!showBackfill)}
                    className={`w-full p-4 flex items-center justify-between text-sm font-medium ${
                      botTab === 'flipper' ? 'text-purple-300' : botTab === 'lutz' ? 'text-orange-300' : botTab === 'ditz' ? 'text-cyan-300' : botTab === 'trader' ? 'text-emerald-300' : 'text-violet-300'
                    }`}
                  >
                    Rückwirkende Trades
                    <svg className={`w-4 h-4 transition-transform ${showBackfill ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                    </svg>
                  </button>
                  {showBackfill && (
                    <div className="px-4 pb-4">
                      <p className="text-xs text-gray-500 mb-3">
                        Erstellt historische Trades ab dem gewählten Datum bis heute (100€ pro Position, nicht live)
                      </p>
                      <div className="flex items-center gap-2">
                        <input
                          type="date"
                          value={backfillDate}
                          onChange={(e) => setBackfillDate(e.target.value)}
                          className="bg-dark-700 border border-dark-500 rounded-lg px-3 py-2 text-white text-sm"
                        />
                        <button
                          onClick={() => handleBackfill(getBotApiName())}
                          disabled={backfilling || !backfillDate}
                          className={`px-4 py-2 text-white rounded-lg font-medium disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2 whitespace-nowrap ${
                            botTab === 'flipper'
                              ? 'bg-purple-500 hover:bg-purple-400'
                              : botTab === 'lutz'
                              ? 'bg-orange-500 hover:bg-orange-400'
                              : botTab === 'ditz'
                              ? 'bg-cyan-500 hover:bg-cyan-400'
                              : botTab === 'trader'
                              ? 'bg-emerald-500 hover:bg-emerald-400'
                              : 'bg-violet-500 hover:bg-violet-400'
                          }`}
                        >
                          {backfilling ? (
                            <>
                              <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                              Backfill...
                            </>
                          ) : (
                            <>
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                              </svg>
                              Backfill
                            </>
                          )}
                        </button>
                        <button
                          onClick={handleBackfillAll}
                          disabled={backfilling || !backfillDate}
                          className="px-4 py-2 text-white rounded-lg font-medium disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2 whitespace-nowrap bg-gradient-to-r from-purple-500 via-violet-500 to-emerald-500 hover:from-purple-400 hover:via-violet-400 hover:to-emerald-400"
                        >
                          {backfilling ? (
                            <>
                              <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                              Alle...
                            </>
                          ) : (
                            <>
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                              </svg>
                              Alle Bots
                            </>
                          )}
                        </button>
                      </div>
                      {backfillProgress && (
                        <div className="mt-3 p-3 bg-dark-800 rounded-lg">
                          <div className="flex items-center justify-between text-sm mb-2">
                            <span className="text-gray-400">{backfillProgress.message || 'Starte Backfill...'}</span>
                            {backfillProgress.total > 0 && (
                              <span className="text-white font-medium">
                                {backfillProgress.current} / {backfillProgress.total}
                              </span>
                            )}
                          </div>
                          {backfillProgress.total > 0 && (
                            <div className="w-full h-2 bg-dark-700 rounded-full overflow-hidden">
                              <div
                                className={`h-full transition-all duration-300 ${
                                  botTab === 'flipper' ? 'bg-purple-500'
                                    : botTab === 'lutz' ? 'bg-orange-500'
                                    : botTab === 'ditz' ? 'bg-cyan-500'
                                    : botTab === 'trader' ? 'bg-emerald-500'
                                    : 'bg-violet-500'
                                }`}
                                style={{ width: `${(backfillProgress.current / backfillProgress.total) * 100}%` }}
                              />
                            </div>
                          )}
                          {backfillProgress.symbol && (
                            <div className="text-xs text-gray-500 mt-2">
                              Aktuell: {backfillProgress.symbol}
                            </div>
                          )}
                        </div>
                      )}
                      {backfillResult && (
                        <div className={`mt-3 p-3 rounded-lg text-sm ${backfillResult.success ? 'bg-green-500/20 text-green-300' : 'bg-red-500/20 text-red-300'}`}>
                          {backfillResult.success ? (
                            <div>
                              <p className="font-medium mb-1">Backfill abgeschlossen:</p>
                              <p className="text-xs">
                                {backfillResult.data.trades_created} Trades erstellt, {backfillResult.data.positions_created} Positionen erstellt
                              </p>
                            </div>
                          ) : (
                            <p>Fehler: {backfillResult.error}</p>
                          )}
                        </div>
                      )}
                    </div>
                  )}
                </div>

                {/* Fix DB Button - only for FlipperBot */}
                {botTab === 'flipper' && (
                  <div className="p-4 bg-red-500/10 border border-red-500/30 rounded-xl">
                    <div className="flex items-center justify-between">
                      <div>
                        <h3 className="text-sm font-medium text-red-300">Datenbank-Fix</h3>
                        <p className="text-xs text-gray-500 mt-1">Behebt kaputte Trades (invalid qty/price), doppelte BUYs, verwaiste Positionen</p>
                      </div>
                      <button
                        onClick={handleFixDB}
                        disabled={fixingDB}
                        className="px-4 py-2 bg-red-500 text-white rounded-lg font-medium hover:bg-red-400 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
                      >
                        {fixingDB ? (
                          <>
                            <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                            Fixe...
                          </>
                        ) : (
                          <>
                            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                            </svg>
                            Fix DB
                          </>
                        )}
                      </button>
                    </div>
                    {fixResult && (
                      <div className={`mt-3 p-3 rounded-lg text-sm ${fixResult.success ? 'bg-green-500/20 text-green-300' : 'bg-red-500/20 text-red-300'}`}>
                        {fixResult.success ? (
                          <div>
                            <p className="font-medium mb-2">Fix abgeschlossen:</p>
                            <ul className="text-xs space-y-1">
                              <li>Trades mit ungültiger Menge gelöscht: {fixResult.data.deleted_invalid_qty_trades || 0}</li>
                              <li>Positionen mit ungültiger Menge gelöscht: {fixResult.data.deleted_invalid_qty_positions || 0}</li>
                              <li>Positionen mit ungültigem Preis gelöscht: {fixResult.data.deleted_invalid_price_positions || 0}</li>
                              <li>Doppelte BUY-Trades gelöscht: {fixResult.data.deleted_duplicate_buys || 0}</li>
                              <li>Verwaiste Positionen gelöscht: {fixResult.data.deleted_orphan_positions || 0}</li>
                              <li>Positionen aus Trades rekonstruiert: {fixResult.data.rebuilt_positions || 0}</li>
                            </ul>
                          </div>
                        ) : (
                          <p>Fehler: {fixResult.error}</p>
                        )}
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}

            {/* Quant Tab */}
            {/* Ditz Config Tab */}
            {activeTab === 'ditz' && (
              <div className="space-y-6">
                <div className="p-4 bg-cyan-500/10 border border-cyan-500/30 rounded-xl">
                  <h3 className="text-lg font-medium text-cyan-300 mb-2 flex items-center gap-2">
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                    </svg>
                    B-Xtrender Ditz Konfiguration
                  </h3>
                  <p className="text-sm text-gray-400 mb-6">
                    Basiert auf dem QuantTherapy Backtest Edition Algorithmus. Eigene Trade-Regeln (in Entwicklung).
                  </p>

                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-cyan-400 font-medium block mb-2">Short L1</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={ditzConfig.short_l1 || 5}
                        onChange={(e) => updateDitzConfigValue('short_l1', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">Schnelle EMA-Periode. Höher = glatter, weniger rauschig. Niedriger = reaktionsschneller.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-cyan-400 font-medium block mb-2">Short L2</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={ditzConfig.short_l2 || 20}
                        onChange={(e) => updateDitzConfigValue('short_l2', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">Langsame EMA-Periode. Höher = stabilere Signale. Niedriger = mehr Trades.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-cyan-400 font-medium block mb-2">Short L3</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={ditzConfig.short_l3 || 15}
                        onChange={(e) => updateDitzConfigValue('short_l3', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">RSI-Periode für kurzfristigen Indikator. Höher = weniger Schwankungen.</p>
                    </div>

                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-cyan-400 font-medium block mb-2">Long L1</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={ditzConfig.long_l1 || 20}
                        onChange={(e) => updateDitzConfigValue('long_l1', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">EMA-Periode für langfristigen Indikator. Höher = langsamere Trenderfassung.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-cyan-400 font-medium block mb-2">Long L2</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={ditzConfig.long_l2 || 15}
                        onChange={(e) => updateDitzConfigValue('long_l2', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">RSI-Periode für langfristigen Indikator. Höher = stabilere Signale.</p>
                    </div>

                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-cyan-400 font-medium block mb-2">MA Filter</label>
                      <button
                        onClick={() => updateDitzConfigValue('ma_filter_on', !ditzConfig.ma_filter_on)}
                        className={`px-4 py-2 rounded font-bold transition-colors mb-2 ${
                          ditzConfig.ma_filter_on
                            ? 'bg-cyan-500 text-white'
                            : 'bg-dark-600 text-gray-400'
                        }`}
                      >
                        {ditzConfig.ma_filter_on ? 'AN' : 'AUS'}
                      </button>
                      <p className="text-xs text-gray-500">Kauft nur wenn Preis über MA liegt. AN = konservativer, weniger Trades in Bärenmärkten.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-cyan-400 font-medium block mb-2">MA Länge</label>
                      <input
                        type="number"
                        min="10"
                        max="500"
                        value={ditzConfig.ma_length || 200}
                        onChange={(e) => updateDitzConfigValue('ma_length', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">Perioden für MA-Filter. 200 = klassisch. Höher = langfristigerer Trend.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-cyan-400 font-medium block mb-2">MA Typ</label>
                      <select
                        value={ditzConfig.ma_type || 'EMA'}
                        onChange={(e) => updateDitzConfigValue('ma_type', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      >
                        <option value="SMA">SMA (Simple)</option>
                        <option value="EMA">EMA (Exponential)</option>
                      </select>
                      <p className="text-xs text-gray-500">SMA = gleichmäßig gewichtet. EMA = reagiert schneller auf aktuelle Preise.</p>
                    </div>

                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-cyan-400 font-medium block mb-2">TSL Prozent</label>
                      <input
                        type="number"
                        min="1"
                        max="100"
                        step="0.5"
                        value={ditzConfig.tsl_percent || 20}
                        onChange={(e) => updateDitzConfigValue('tsl_percent', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">Trailing Stop Loss in %. Höher = mehr Spielraum. Niedriger = schnellerer Ausstieg bei Verlust.</p>
                    </div>

                    <div className="bg-dark-800 rounded-lg p-3 flex items-center gap-3">
                      <label className="text-xs text-gray-500">SL aktiv</label>
                      <button
                        onClick={() => updateDitzConfigValue('tsl_enabled', !ditzConfig.tsl_enabled)}
                        className={`relative w-10 h-5 rounded-full transition-colors ${
                          ditzConfig.tsl_enabled !== false ? 'bg-green-500' : 'bg-gray-600'
                        }`}
                      >
                        <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${
                          ditzConfig.tsl_enabled !== false ? 'translate-x-5' : ''
                        }`} />
                      </button>
                    </div>
                  </div>

                  <div className="flex items-center justify-between pt-4 border-t border-cyan-500/20">
                    <div className="text-sm text-gray-400">
                      <strong className="text-cyan-400">Entry:</strong> Short &gt; 0 UND Long &gt; 0 UND (Preis &gt; MA oder MA-Filter aus)<br/>
                      <strong className="text-cyan-400">Exit:</strong> Short &lt; 0 ODER Long &lt; 0
                    </div>
                    <button
                      onClick={handleSaveDitzConfig}
                      disabled={savingDitzConfig}
                      className="px-6 py-2 bg-cyan-500 hover:bg-cyan-400 text-white rounded-lg font-medium disabled:opacity-50"
                    >
                      {savingDitzConfig ? 'Speichern...' : 'Speichern'}
                    </button>
                  </div>
                </div>
              </div>
            )}

            {activeTab === 'trader' && (
              <div className="space-y-6">
                <div className="p-4 bg-emerald-500/10 border border-emerald-500/30 rounded-xl">
                  <h3 className="text-lg font-medium text-emerald-300 mb-2 flex items-center gap-2">
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                    </svg>
                    B-Xtrender Trader Konfiguration
                  </h3>
                  <p className="text-sm text-gray-400 mb-6">
                    Basiert auf dem QuantTherapy Backtest Edition Algorithmus. Eigene Trade-Regeln (in Entwicklung).
                  </p>

                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-emerald-400 font-medium block mb-2">Short L1</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={traderConfig.short_l1 || 5}
                        onChange={(e) => updateTraderConfigValue('short_l1', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">Schnelle EMA-Periode. Höher = glatter, weniger rauschig. Niedriger = reaktionsschneller.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-emerald-400 font-medium block mb-2">Short L2</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={traderConfig.short_l2 || 20}
                        onChange={(e) => updateTraderConfigValue('short_l2', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">Langsame EMA-Periode. Höher = stabilere Signale. Niedriger = mehr Trades.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-emerald-400 font-medium block mb-2">Short L3</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={traderConfig.short_l3 || 15}
                        onChange={(e) => updateTraderConfigValue('short_l3', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">RSI-Periode für kurzfristigen Indikator. Höher = weniger Schwankungen.</p>
                    </div>

                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-emerald-400 font-medium block mb-2">Long L1</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={traderConfig.long_l1 || 20}
                        onChange={(e) => updateTraderConfigValue('long_l1', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">EMA-Periode für langfristigen Indikator. Höher = langsamere Trenderfassung.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-emerald-400 font-medium block mb-2">Long L2</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={traderConfig.long_l2 || 15}
                        onChange={(e) => updateTraderConfigValue('long_l2', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">RSI-Periode für langfristigen Indikator. Höher = stabilere Signale.</p>
                    </div>

                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-emerald-400 font-medium block mb-2">MA Filter</label>
                      <button
                        onClick={() => updateTraderConfigValue('ma_filter_on', !traderConfig.ma_filter_on)}
                        className={`px-4 py-2 rounded font-bold transition-colors mb-2 ${
                          traderConfig.ma_filter_on
                            ? 'bg-emerald-500 text-white'
                            : 'bg-dark-600 text-gray-400'
                        }`}
                      >
                        {traderConfig.ma_filter_on ? 'AN' : 'AUS'}
                      </button>
                      <p className="text-xs text-gray-500">Kauft nur wenn Preis über MA liegt. AN = konservativer, weniger Trades in Bärenmärkten.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-emerald-400 font-medium block mb-2">MA Länge</label>
                      <input
                        type="number"
                        min="10"
                        max="500"
                        value={traderConfig.ma_length || 200}
                        onChange={(e) => updateTraderConfigValue('ma_length', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">Perioden für MA-Filter. 200 = klassisch. Höher = langfristigerer Trend.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-emerald-400 font-medium block mb-2">MA Typ</label>
                      <select
                        value={traderConfig.ma_type || 'EMA'}
                        onChange={(e) => updateTraderConfigValue('ma_type', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      >
                        <option value="SMA">SMA (Simple)</option>
                        <option value="EMA">EMA (Exponential)</option>
                      </select>
                      <p className="text-xs text-gray-500">SMA = gleichmäßig gewichtet. EMA = reagiert schneller auf aktuelle Preise.</p>
                    </div>

                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-emerald-400 font-medium block mb-2">TSL Prozent</label>
                      <input
                        type="number"
                        min="1"
                        max="100"
                        step="0.5"
                        value={traderConfig.tsl_percent || 20}
                        onChange={(e) => updateTraderConfigValue('tsl_percent', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">Trailing Stop Loss in %. Höher = mehr Spielraum. Niedriger = schnellerer Ausstieg bei Verlust.</p>
                    </div>

                    <div className="bg-dark-800 rounded-lg p-3 flex items-center gap-3">
                      <label className="text-xs text-gray-500">SL aktiv</label>
                      <button
                        onClick={() => updateTraderConfigValue('tsl_enabled', !traderConfig.tsl_enabled)}
                        className={`relative w-10 h-5 rounded-full transition-colors ${
                          traderConfig.tsl_enabled !== false ? 'bg-green-500' : 'bg-gray-600'
                        }`}
                      >
                        <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${
                          traderConfig.tsl_enabled !== false ? 'translate-x-5' : ''
                        }`} />
                      </button>
                    </div>
                  </div>

                  <div className="flex items-center justify-between pt-4 border-t border-emerald-500/20">
                    <div className="text-sm text-gray-400">
                      <strong className="text-emerald-400">Entry:</strong> Short &gt; 0 UND Long &gt; 0 UND (Preis &gt; MA oder MA-Filter aus)<br/>
                      <strong className="text-emerald-400">Exit:</strong> Short &lt; 0 ODER Long &lt; 0
                    </div>
                    <button
                      onClick={handleSaveTraderConfig}
                      disabled={savingTraderConfig}
                      className="px-6 py-2 bg-emerald-500 hover:bg-emerald-400 text-white rounded-lg font-medium disabled:opacity-50"
                    >
                      {savingTraderConfig ? 'Speichern...' : 'Speichern'}
                    </button>
                  </div>
                </div>
              </div>
            )}

            {activeTab === 'quant' && (
              <div className="space-y-6">
                <div className="p-4 bg-violet-500/10 border border-violet-500/30 rounded-xl">
                  <h3 className="text-lg font-medium text-violet-300 mb-2 flex items-center gap-2">
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                    </svg>
                    B-Xtrender Quant Konfiguration
                  </h3>
                  <p className="text-sm text-gray-400 mb-6">
                    Basiert auf dem QuantTherapy Backtest Edition Algorithmus. Kauft wenn BEIDE Indikatoren positiv sind, verkauft wenn EINER negativ wird.
                  </p>

                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
                    {/* Short-Term Settings */}
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-violet-400 font-medium block mb-2">Short L1</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={quantConfig.short_l1 || 5}
                        onChange={(e) => updateQuantConfigValue('short_l1', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">Schnelle EMA-Periode. Höher = glatter, weniger rauschig. Niedriger = reaktionsschneller.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-violet-400 font-medium block mb-2">Short L2</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={quantConfig.short_l2 || 20}
                        onChange={(e) => updateQuantConfigValue('short_l2', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">Langsame EMA-Periode. Höher = stabilere Signale. Niedriger = mehr Trades.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-violet-400 font-medium block mb-2">Short L3</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={quantConfig.short_l3 || 15}
                        onChange={(e) => updateQuantConfigValue('short_l3', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">RSI-Periode für kurzfristigen Indikator. Höher = weniger Schwankungen.</p>
                    </div>

                    {/* Long-Term Settings */}
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-violet-400 font-medium block mb-2">Long L1</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={quantConfig.long_l1 || 20}
                        onChange={(e) => updateQuantConfigValue('long_l1', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">EMA-Periode für langfristigen Indikator. Höher = langsamere Trenderfassung.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-violet-400 font-medium block mb-2">Long L2</label>
                      <input
                        type="number"
                        min="1"
                        max="200"
                        value={quantConfig.long_l2 || 15}
                        onChange={(e) => updateQuantConfigValue('long_l2', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">RSI-Periode für langfristigen Indikator. Höher = stabilere Signale.</p>
                    </div>

                    {/* MA Filter Settings */}
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-violet-400 font-medium block mb-2">MA Filter</label>
                      <button
                        onClick={() => updateQuantConfigValue('ma_filter_on', !quantConfig.ma_filter_on)}
                        className={`px-4 py-2 rounded font-bold transition-colors mb-2 ${
                          quantConfig.ma_filter_on
                            ? 'bg-violet-500 text-white'
                            : 'bg-dark-600 text-gray-400'
                        }`}
                      >
                        {quantConfig.ma_filter_on ? 'AN' : 'AUS'}
                      </button>
                      <p className="text-xs text-gray-500">Kauft nur wenn Preis über MA liegt. AN = konservativer, weniger Trades in Bärenmärkten.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-violet-400 font-medium block mb-2">MA Länge</label>
                      <input
                        type="number"
                        min="10"
                        max="500"
                        value={quantConfig.ma_length || 200}
                        onChange={(e) => updateQuantConfigValue('ma_length', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">Perioden für MA-Filter. 200 = klassisch. Höher = langfristigerer Trend.</p>
                    </div>
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-violet-400 font-medium block mb-2">MA Typ</label>
                      <select
                        value={quantConfig.ma_type || 'EMA'}
                        onChange={(e) => updateQuantConfigValue('ma_type', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      >
                        <option value="SMA">SMA (Simple)</option>
                        <option value="EMA">EMA (Exponential)</option>
                      </select>
                      <p className="text-xs text-gray-500">SMA = gleichmäßig gewichtet. EMA = reagiert schneller auf aktuelle Preise.</p>
                    </div>

                    {/* TSL Percent */}
                    <div className="bg-dark-800 rounded-lg p-4">
                      <label className="text-sm text-violet-400 font-medium block mb-2">TSL Prozent</label>
                      <input
                        type="number"
                        min="1"
                        max="100"
                        step="0.5"
                        value={quantConfig.tsl_percent || 20}
                        onChange={(e) => updateQuantConfigValue('tsl_percent', e.target.value)}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-3 py-2 text-white mb-2"
                      />
                      <p className="text-xs text-gray-500">Trailing Stop Loss in %. Höher = mehr Spielraum. Niedriger = schnellerer Ausstieg bei Verlust.</p>
                    </div>

                    <div className="bg-dark-800 rounded-lg p-3 flex items-center gap-3">
                      <label className="text-xs text-gray-500">SL aktiv</label>
                      <button
                        onClick={() => updateQuantConfigValue('tsl_enabled', !quantConfig.tsl_enabled)}
                        className={`relative w-10 h-5 rounded-full transition-colors ${
                          quantConfig.tsl_enabled !== false ? 'bg-green-500' : 'bg-gray-600'
                        }`}
                      >
                        <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${
                          quantConfig.tsl_enabled !== false ? 'translate-x-5' : ''
                        }`} />
                      </button>
                    </div>
                  </div>

                  <div className="flex items-center justify-between pt-4 border-t border-violet-500/20">
                    <div className="text-sm text-gray-400">
                      <strong className="text-violet-400">Entry:</strong> Short &gt; 0 UND Long &gt; 0 UND (Preis &gt; MA oder MA-Filter aus)<br/>
                      <strong className="text-violet-400">Exit:</strong> Short &lt; 0 ODER Long &lt; 0
                    </div>
                    <button
                      onClick={handleSaveQuantConfig}
                      disabled={savingQuantConfig}
                      className="px-6 py-2 bg-violet-500 hover:bg-violet-400 text-white rounded-lg font-medium disabled:opacity-50"
                    >
                      {savingQuantConfig ? 'Speichern...' : 'Speichern'}
                    </button>
                  </div>
                </div>
              </div>
            )}

            {/* Bot Filter Tab */}
            {activeTab === 'botfilter' && (
              <div className="space-y-6">
                <div>
                  <h3 className="text-lg font-medium text-white mb-1">Bot Performance Filter</h3>
                  <p className="text-sm text-gray-400">
                    Konfiguriere Filter pro Bot. Wenn aktiv, werden BUY-Signale blockiert wenn die Aktie die Kriterien nicht erfüllt. Blockierte Trades werden trotzdem aufgezeichnet.
                  </p>
                </div>

                {/* Global Filter - apply to all bots */}
                <div className="p-4 bg-dark-700 border border-yellow-500/30 rounded-xl">
                  <div className="flex items-center justify-between mb-4">
                    <h4 className="text-md font-medium text-yellow-400">Für alle Bots</h4>
                    <button
                      onClick={() => setGlobalFilter(prev => ({ ...prev, enabled: !prev.enabled }))}
                      className={`px-4 py-1.5 rounded font-bold text-sm transition-colors ${
                        globalFilter.enabled ? 'bg-green-500 text-white' : 'bg-dark-600 text-gray-400'
                      }`}
                    >
                      {globalFilter.enabled ? 'AKTIV' : 'INAKTIV'}
                    </button>
                  </div>

                  <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
                    <div className="bg-dark-800 rounded-lg p-3">
                      <label className="text-xs text-gray-400 block mb-1">Min WinRate (%)</label>
                      <input type="number" step="1" placeholder="-"
                        value={globalFilter.min_winrate ?? ''}
                        onChange={e => setGlobalFilter(prev => ({ ...prev, min_winrate: e.target.value === '' ? null : e.target.value }))}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                      />
                    </div>
                    <div className="bg-dark-800 rounded-lg p-3">
                      <label className="text-xs text-gray-400 block mb-1">Max WinRate (%)</label>
                      <input type="number" step="1" placeholder="-"
                        value={globalFilter.max_winrate ?? ''}
                        onChange={e => setGlobalFilter(prev => ({ ...prev, max_winrate: e.target.value === '' ? null : e.target.value }))}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                      />
                    </div>
                    <div className="bg-dark-800 rounded-lg p-3">
                      <label className="text-xs text-gray-400 block mb-1">Min R/R</label>
                      <input type="number" step="0.1" placeholder="-"
                        value={globalFilter.min_rr ?? ''}
                        onChange={e => setGlobalFilter(prev => ({ ...prev, min_rr: e.target.value === '' ? null : e.target.value }))}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                      />
                    </div>
                    <div className="bg-dark-800 rounded-lg p-3">
                      <label className="text-xs text-gray-400 block mb-1">Max R/R</label>
                      <input type="number" step="0.1" placeholder="-"
                        value={globalFilter.max_rr ?? ''}
                        onChange={e => setGlobalFilter(prev => ({ ...prev, max_rr: e.target.value === '' ? null : e.target.value }))}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                      />
                    </div>
                    <div className="bg-dark-800 rounded-lg p-3">
                      <label className="text-xs text-gray-400 block mb-1">Min Ø Rendite (%)</label>
                      <input type="number" step="0.1" placeholder="-"
                        value={globalFilter.min_avg_return ?? ''}
                        onChange={e => setGlobalFilter(prev => ({ ...prev, min_avg_return: e.target.value === '' ? null : e.target.value }))}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                      />
                    </div>
                    <div className="bg-dark-800 rounded-lg p-3">
                      <label className="text-xs text-gray-400 block mb-1">Max Ø Rendite (%)</label>
                      <input type="number" step="0.1" placeholder="-"
                        value={globalFilter.max_avg_return ?? ''}
                        onChange={e => setGlobalFilter(prev => ({ ...prev, max_avg_return: e.target.value === '' ? null : e.target.value }))}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                      />
                    </div>
                    <div className="bg-dark-800 rounded-lg p-3 col-span-2">
                      <label className="text-xs text-gray-400 block mb-1">Min MarketCap (Mrd)</label>
                      <input type="number" step="0.1" placeholder="-"
                        value={globalFilter.min_market_cap ?? ''}
                        onChange={e => setGlobalFilter(prev => ({ ...prev, min_market_cap: e.target.value === '' ? null : e.target.value }))}
                        className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                      />
                    </div>
                  </div>

                  <div className="flex justify-end">
                    <button
                      onClick={applyGlobalFilterToAll}
                      disabled={botFilterSaving === 'global'}
                      className="px-5 py-2 bg-yellow-500 hover:bg-yellow-400 text-black rounded-lg font-medium text-sm disabled:opacity-50"
                    >
                      {botFilterSaving === 'global' ? 'Wird angewendet...' : 'Auf alle Bots anwenden'}
                    </button>
                  </div>
                </div>

                {[
                  { name: 'flipper', label: 'FlipperBot (Defensiv)', color: 'blue' },
                  { name: 'lutz', label: 'Lutz (Aggressiv)', color: 'orange' },
                  { name: 'quant', label: 'Quant', color: 'violet' },
                  { name: 'ditz', label: 'Ditz', color: 'cyan' },
                  { name: 'trader', label: 'Trader', color: 'emerald' }
                ].map(bot => {
                  const config = botFilterConfigs[bot.name] || {}
                  return (
                    <div key={bot.name} className="p-4 bg-dark-700 border border-dark-500 rounded-xl">
                      <div className="flex items-center justify-between mb-4">
                        <h4 className="text-md font-medium text-white">{bot.label}</h4>
                        <button
                          onClick={() => updateBotFilterValue(bot.name, 'enabled', !config.enabled)}
                          className={`px-4 py-1.5 rounded font-bold text-sm transition-colors ${
                            config.enabled ? 'bg-green-500 text-white' : 'bg-dark-600 text-gray-400'
                          }`}
                        >
                          {config.enabled ? 'AKTIV' : 'INAKTIV'}
                        </button>
                      </div>

                      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
                        <div className="bg-dark-800 rounded-lg p-3">
                          <label className="text-xs text-gray-400 block mb-1">Min WinRate (%)</label>
                          <input type="number" step="1" placeholder="-"
                            value={config.min_winrate ?? ''}
                            onChange={e => updateBotFilterValue(bot.name, 'min_winrate', e.target.value === '' ? null : e.target.value)}
                            className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                          />
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <label className="text-xs text-gray-400 block mb-1">Max WinRate (%)</label>
                          <input type="number" step="1" placeholder="-"
                            value={config.max_winrate ?? ''}
                            onChange={e => updateBotFilterValue(bot.name, 'max_winrate', e.target.value === '' ? null : e.target.value)}
                            className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                          />
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <label className="text-xs text-gray-400 block mb-1">Min R/R</label>
                          <input type="number" step="0.1" placeholder="-"
                            value={config.min_rr ?? ''}
                            onChange={e => updateBotFilterValue(bot.name, 'min_rr', e.target.value === '' ? null : e.target.value)}
                            className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                          />
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <label className="text-xs text-gray-400 block mb-1">Max R/R</label>
                          <input type="number" step="0.1" placeholder="-"
                            value={config.max_rr ?? ''}
                            onChange={e => updateBotFilterValue(bot.name, 'max_rr', e.target.value === '' ? null : e.target.value)}
                            className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                          />
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <label className="text-xs text-gray-400 block mb-1">Min Ø Rendite (%)</label>
                          <input type="number" step="0.1" placeholder="-"
                            value={config.min_avg_return ?? ''}
                            onChange={e => updateBotFilterValue(bot.name, 'min_avg_return', e.target.value === '' ? null : e.target.value)}
                            className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                          />
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <label className="text-xs text-gray-400 block mb-1">Max Ø Rendite (%)</label>
                          <input type="number" step="0.1" placeholder="-"
                            value={config.max_avg_return ?? ''}
                            onChange={e => updateBotFilterValue(bot.name, 'max_avg_return', e.target.value === '' ? null : e.target.value)}
                            className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                          />
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3 col-span-2">
                          <label className="text-xs text-gray-400 block mb-1">Min MarketCap (Mrd)</label>
                          <input type="number" step="0.1" placeholder="-"
                            value={config.min_market_cap ?? ''}
                            onChange={e => updateBotFilterValue(bot.name, 'min_market_cap', e.target.value === '' ? null : e.target.value)}
                            className="w-full bg-dark-700 border border-dark-500 rounded px-2 py-1.5 text-white text-sm"
                          />
                        </div>
                      </div>

                      <div className="flex justify-end">
                        <button
                          onClick={() => saveBotFilterConfig(bot.name)}
                          disabled={botFilterSaving === bot.name}
                          className="px-5 py-2 bg-blue-500 hover:bg-blue-400 text-white rounded-lg font-medium text-sm disabled:opacity-50"
                        >
                          {botFilterSaving === bot.name ? 'Speichern...' : 'Speichern'}
                        </button>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}

            {/* Aktien Listen Tab */}
            {activeTab === 'allowlist' && (
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <h3 className="text-lg font-medium text-white">Bot Aktien Listen</h3>
                  <button
                    onClick={fetchAllowlist}
                    className="px-3 py-1.5 bg-dark-700 text-gray-300 rounded-lg text-sm hover:bg-dark-600"
                  >
                    Aktualisieren
                  </button>
                </div>

                {allowlistMessage && (
                  <div className="p-3 bg-yellow-500/10 border border-yellow-500/30 rounded-lg text-yellow-300 text-sm">
                    {allowlistMessage}
                  </div>
                )}

                <input
                  type="text"
                  placeholder="Aktie suchen..."
                  value={allowlistFilter}
                  onChange={(e) => setAllowlistFilter(e.target.value)}
                  className="w-full md:w-64 px-3 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white text-sm placeholder-gray-500 focus:outline-none focus:border-accent-500"
                />

                {allowlistLoading ? (
                  <div className="text-center py-12">
                    <div className="w-8 h-8 border-2 border-accent-500 border-t-transparent rounded-full animate-spin mx-auto"></div>
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b border-dark-600">
                          <th className="text-left py-2 px-3 text-gray-400 font-medium">Symbol</th>
                          {['flipper', 'lutz', 'quant', 'ditz', 'trader'].map(bot => (
                            <th key={bot} className="text-center py-2 px-3 text-gray-400 font-medium capitalize">{bot}</th>
                          ))}
                          <th className="text-center py-2 px-3 text-gray-400 font-medium">Alle</th>
                        </tr>
                      </thead>
                      <tbody>
                        {(() => {
                          const allSymbols = new Set()
                          Object.values(allowlistData).forEach(entries => {
                            if (entries) entries.forEach(e => allSymbols.add(e.symbol))
                          })
                          const symbolList = [...allSymbols].sort().filter(s =>
                            !allowlistFilter || s.toLowerCase().includes(allowlistFilter.toLowerCase())
                          )

                          return symbolList.map(symbol => (
                            <tr key={symbol} className="border-b border-dark-700 hover:bg-dark-700/50">
                              <td className="py-2 px-3 text-white font-mono">{symbol}</td>
                              {['flipper', 'lutz', 'quant', 'ditz', 'trader'].map(bot => {
                                const botEntries = allowlistData[bot] || []
                                const entry = botEntries.find(e => e.symbol === symbol)
                                const hasStock = !!entry
                                const isAllowed = entry ? entry.allowed : true

                                return (
                                  <td key={bot} className="text-center py-2 px-3">
                                    {hasStock ? (
                                      <button
                                        onClick={() => toggleAllowlist(bot, symbol, isAllowed)}
                                        className={`w-8 h-8 rounded-lg flex items-center justify-center mx-auto transition-colors ${
                                          isAllowed
                                            ? 'bg-green-500/20 text-green-400 hover:bg-green-500/30'
                                            : 'bg-red-500/20 text-red-400 hover:bg-red-500/30'
                                        }`}
                                        title={isAllowed ? 'Erlaubt - Klicken zum Deaktivieren' : 'Blockiert - Klicken zum Aktivieren'}
                                      >
                                        {isAllowed ? (
                                          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                                          </svg>
                                        ) : (
                                          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                          </svg>
                                        )}
                                      </button>
                                    ) : (
                                      <span className="text-gray-600">-</span>
                                    )}
                                  </td>
                                )
                              })}
                              {(() => {
                                const bots = ['flipper', 'lutz', 'quant', 'ditz', 'trader']
                                const entries = bots.map(bot => {
                                  const botEntries = allowlistData[bot] || []
                                  return botEntries.find(e => e.symbol === symbol)
                                }).filter(Boolean)
                                if (entries.length === 0) return <td className="text-center py-2 px-3"><span className="text-gray-600">-</span></td>
                                const allAllowed = entries.every(e => e.allowed)
                                const allBlocked = entries.every(e => !e.allowed)
                                return (
                                  <td className="text-center py-2 px-3">
                                    <button
                                      onClick={() => toggleAllBots(symbol)}
                                      className={`w-8 h-8 rounded-lg flex items-center justify-center mx-auto transition-colors ${
                                        allAllowed
                                          ? 'bg-green-500/20 text-green-400 hover:bg-red-500/20 hover:text-red-400'
                                          : allBlocked
                                            ? 'bg-red-500/20 text-red-400 hover:bg-green-500/20 hover:text-green-400'
                                            : 'bg-yellow-500/20 text-yellow-400 hover:bg-green-500/20 hover:text-green-400'
                                      }`}
                                      title={allAllowed ? 'Alle Bots deaktivieren' : 'Alle Bots aktivieren'}
                                    >
                                      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={
                                          allAllowed
                                            ? 'M5 13l4 4L19 7'
                                            : allBlocked
                                              ? 'M6 18L18 6M6 6l12 12'
                                              : 'M20 12H4'
                                        } />
                                      </svg>
                                    </button>
                                  </td>
                                )
                              })()}
                            </tr>
                          ))
                        })()}
                      </tbody>
                    </table>
                    {Object.keys(allowlistData).length === 0 && (
                      <div className="text-center py-8 text-gray-500">Keine Daten geladen</div>
                    )}
                  </div>
                )}
              </div>
            )}
          </>
        )}
      </div>

      {/* Tracked Diff Modal */}
      {showTrackedDiff && (
        <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
          <div className="bg-dark-800 rounded-xl border border-dark-600 w-full max-w-2xl max-h-[80vh] overflow-hidden">
            <div className="flex items-center justify-between p-4 border-b border-dark-600">
              <div>
                <h2 className="text-lg font-semibold text-white">Getrackte Aktien - Differenz</h2>
                <p className="text-xs text-gray-500">Aktien die getrackt werden, aber nicht mehr in der Watchlist sind</p>
              </div>
              <button
                onClick={() => setShowTrackedDiff(false)}
                className="p-2 text-gray-400 hover:text-white transition-colors"
              >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div className="p-4 overflow-y-auto max-h-[60vh]">
              {loadingDiff ? (
                <div className="text-center py-8">
                  <div className="w-6 h-6 border-2 border-accent-500 border-t-transparent rounded-full animate-spin mx-auto"></div>
                </div>
              ) : (
                <div className="space-y-4">
                  {/* Defensive */}
                  <div>
                    <h3 className="text-sm font-medium text-blue-400 mb-2">Defensiv ({trackedDiff.defensive?.length || 0})</h3>
                    {trackedDiff.defensive?.length > 0 ? (
                      <div className="space-y-1">
                        {trackedDiff.defensive.map((s) => (
                          <div key={s.symbol} className="flex items-center justify-between bg-dark-700 rounded-lg px-3 py-2">
                            <div>
                              <span className="font-medium text-white">{s.symbol}</span>
                              <span className="text-gray-500 text-sm ml-2">{s.name}</span>
                            </div>
                            <button
                              onClick={() => handleDeleteTracked(s.symbol)}
                              className="p-1.5 text-gray-400 hover:text-red-400 transition-colors"
                              title="Löschen"
                            >
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                              </svg>
                            </button>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-gray-500 text-sm">Keine Differenz</p>
                    )}
                  </div>
                  {/* Aggressive */}
                  <div>
                    <h3 className="text-sm font-medium text-orange-400 mb-2">Aggressiv ({trackedDiff.aggressive?.length || 0})</h3>
                    {trackedDiff.aggressive?.length > 0 ? (
                      <div className="space-y-1">
                        {trackedDiff.aggressive.map((s) => (
                          <div key={s.symbol} className="flex items-center justify-between bg-dark-700 rounded-lg px-3 py-2">
                            <div>
                              <span className="font-medium text-white">{s.symbol}</span>
                              <span className="text-gray-500 text-sm ml-2">{s.name}</span>
                            </div>
                            <button
                              onClick={() => handleDeleteTracked(s.symbol)}
                              className="p-1.5 text-gray-400 hover:text-red-400 transition-colors"
                              title="Löschen"
                            >
                              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                              </svg>
                            </button>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-gray-500 text-sm">Keine Differenz</p>
                    )}
                  </div>
                  {(trackedDiff.defensive?.length === 0 && trackedDiff.aggressive?.length === 0) && (
                    <div className="text-center py-8 text-gray-500">
                      Alle getrackten Aktien sind auch in der Watchlist
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {selectedPosition && (
        <StockDetailOverlay
          symbol={selectedPosition.symbol}
          name={selectedPosition.name}
          mode={selectedPosition.mode}
          onClose={() => setSelectedPosition(null)}
        />
      )}
    </div>
  )
}

export default AdminPanel
