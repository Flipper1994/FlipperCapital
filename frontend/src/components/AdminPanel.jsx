import { useState, useEffect, useRef } from 'react'
import { Link, Navigate } from 'react-router-dom'
import { useTradingMode } from '../context/TradingModeContext'
import { useCurrency } from '../context/CurrencyContext'
import { processStock } from '../utils/bxtrender'
import PortfolioChart from './PortfolioChart'

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
  const [updatingStocks, setUpdatingStocks] = useState(false)
  const [updateProgress, setUpdateProgress] = useState(null)
  const [lastFullUpdate, setLastFullUpdate] = useState(null)
  const [showTrackedDiff, setShowTrackedDiff] = useState(false)
  const [trackedDiff, setTrackedDiff] = useState({ defensive: [], aggressive: [] })
  const [loadingDiff, setLoadingDiff] = useState(false)
  const { mode, isAggressive } = useTradingMode()
  const { formatPrice, convertPrice, convertToUSD, currencySymbol } = useCurrency()

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
  const [showBxConfig, setShowBxConfig] = useState(false)
  const [showBotReset, setShowBotReset] = useState(false)
  const [showBackfill, setShowBackfill] = useState(false)
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
    defensive: { short_l1: 5, short_l2: 20, short_l3: 15, long_l1: 20, long_l2: 15 },
    aggressive: { short_l1: 5, short_l2: 20, short_l3: 15, long_l1: 20, long_l2: 15 }
  })
  const [quantConfig, setQuantConfig] = useState({
    short_l1: 5, short_l2: 20, short_l3: 15,
    long_l1: 20, long_l2: 15,
    ma_filter_on: true, ma_length: 200, ma_type: 'EMA',
    tsl_percent: 20.0
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
    tsl_percent: 20.0
  })
  const [savingDitzConfig, setSavingDitzConfig] = useState(false)
  const [ditzUnreadCount, setDitzUnreadCount] = useState(0)
  const [ditzUnreadTrades, setDitzUnreadTrades] = useState([])
  const [ditzManualTrade, setDitzManualTrade] = useState({ symbol: '', name: '', action: 'BUY', price: '', quantity: '', date: '', is_live: false })
  const [showDitzManualTrade, setShowDitzManualTrade] = useState(false)

  const [manualTradeSearch, setManualTradeSearch] = useState('')
  const [manualTradeResults, setManualTradeResults] = useState([])
  const [manualTradeSearching, setManualTradeSearching] = useState(false)

  useEffect(() => {
    checkAdmin()
  }, [])

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
          long_l2: parseInt(config.long_l2)
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
          tsl_percent: parseFloat(quantConfig.tsl_percent)
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
          tsl_percent: parseFloat(ditzConfig.tsl_percent)
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
      const [fpRes, ftRes, lpRes, ltRes, fptRes, lptRes, qpRes, qtRes, qptRes, dpRes, dtRes, dptRes, fspRes, lspRes] = await Promise.all([
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
        fetch('/api/lutz/simulated-performance', { headers: { 'Authorization': `Bearer ${token}` } })
      ])
      const [fp, ft, lp, lt, fpt, lpt, qp, qt, qpt, dp, dtt, dpt, fsp, lsp] = await Promise.all([
        fpRes.json(), ftRes.json(), lpRes.json(), ltRes.json(), fptRes.json(), lptRes.json(),
        qpRes.json(), qtRes.json(), qptRes.json(),
        dpRes.json(), dtRes.json(), dptRes.json(),
        fspRes.json(), lspRes.json()
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
      fetchDitzUnreadCount()
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

  const handleDeleteTrade = async (bot, tradeId, symbol, action, isDeleted) => {
    // For quant/ditz bot: soft-delete toggle (no confirm needed for restore)
    if (bot === 'quant' || bot === 'ditz') {
      if (!isDeleted) {
        const msg = action === 'BUY'
          ? `Trade streichen? Die Position für ${symbol} wird geschlossen.`
          : `Trade streichen? Die Position für ${symbol} wird wieder geöffnet.`
        if (!confirm(msg)) return
      }
    } else {
      const msg = action === 'BUY'
        ? `Trade löschen? Die Position für ${symbol} wird ebenfalls gelöscht.`
        : `Trade für ${symbol} löschen?`
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
    const botName = bot === 'flipperbot' ? 'FlipperBot' : bot === 'lutz' ? 'Lutz' : bot === 'ditz' ? 'Ditz' : 'Quant'
    const modeInfo = bot === 'flipperbot' ? 'Defensiv' : bot === 'lutz' ? 'Aggressiv' : bot === 'ditz' ? 'Ditz' : 'Quant'
    if (!confirm(`${botName} Backfill ab ${backfillDate} bis heute durchführen? Historische Trades für ${modeInfo}-Aktien werden erstellt.`)) return
    setBackfilling(true)
    setBackfillResult(null)
    try {
      const res = await fetch(`/api/${bot}/backfill`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ until_date: backfillDate })
      })
      const data = await res.json()
      if (res.ok) {
        setBackfillResult({ success: true, data })
        fetchBotData()
      } else {
        setBackfillResult({ success: false, error: data.error || 'Fehler beim Backfill' })
      }
    } catch (err) {
      setBackfillResult({ success: false, error: err.message })
    } finally {
      setBackfilling(false)
    }
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
    const trades = bot === 'flipperbot' ? flipperPendingTrades : bot === 'lutz' ? lutzPendingTrades : quantPendingTrades
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

  const handleUpdateAllStocks = async () => {
    if (!confirm(`Alle Watchlist-Aktien aktualisieren? Das speichert BX-Trender Daten für ALLE Modi (Defensiv, Aggressiv, Quant & Ditz). Das kann mehrere Minuten dauern.`)) {
      return
    }

    setUpdatingStocks(true)
    setUpdateProgress({ current: 0, total: 0, status: 'Lade Aktien-Liste...' })

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
      const total = stocks.length
      let successCount = 0
      let errorCount = 0

      setUpdateProgress({ current: 0, total, status: 'Verarbeite Aktien...' })

      // Process each stock
      for (let i = 0; i < stocks.length; i++) {
        const stock = stocks[i]
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
        status: `Fertig! ${successCount} erfolgreich, ${errorCount} fehlgeschlagen`,
        currentStock: null
      })

      // Record the full update to backend
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
        fetchLastFullUpdate()
      } catch (err) {
        console.error('Failed to record full update:', err)
      }

      // Refresh stats
      fetchStats()
    } catch (err) {
      console.error('Failed to update stocks:', err)
      setUpdateProgress({ status: 'Fehler: ' + err.message, current: 0, total: 0 })
    } finally {
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
            { key: 'ditz', label: 'Ditz' }
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
                        Aktualisiere alle Aktien der Watchlist für alle vier Modi (Defensiv, Aggressiv, Quant, Ditz).
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
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Gesamt Rendite</div>
                          <div className={`text-xl font-bold ${(flipperPrivatePerformance.overall_return_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {(flipperPrivatePerformance.overall_return_pct || 0) >= 0 ? '+' : ''}{flipperPrivatePerformance.overall_return_pct?.toFixed(2) || '0.00'}%
                          </div>
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Win Rate</div>
                          <div className="text-xl font-bold text-blue-400">{flipperPrivatePerformance.win_rate?.toFixed(1) || '0'}%</div>
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Offene Positionen</div>
                          <div className="text-xl font-bold text-blue-400">{flipperPrivatePerformance.open_positions || 0}</div>
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Unrealisiert</div>
                          <div className={`text-xl font-bold ${(flipperPrivatePerformance.unrealized_pl_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {(flipperPrivatePerformance.unrealized_pl_pct || 0) >= 0 ? '+' : ''}{flipperPrivatePerformance.unrealized_pl_pct?.toFixed(2) || '0.00'}%
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
                                { key: 'buy_date', label: 'Kaufdatum', align: 'left' }
                              ].map(col => (
                                <th key={col.key}
                                  className={`p-2 cursor-pointer hover:text-blue-400 select-none ${col.align === 'right' ? 'text-right' : ''}`}
                                  onClick={() => {
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
                                <td className="p-2 font-medium text-white">{pos.symbol}</td>
                                <td className="p-2 text-gray-400 text-sm">{pos.name}</td>
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
                                <td className="p-2 text-gray-500 text-sm">{new Date(pos.buy_date).toLocaleDateString('de-DE')}</td>
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
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Gesamt Rendite</div>
                          <div className={`text-xl font-bold ${(lutzPrivatePerformance.overall_return_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {(lutzPrivatePerformance.overall_return_pct || 0) >= 0 ? '+' : ''}{lutzPrivatePerformance.overall_return_pct?.toFixed(2) || '0.00'}%
                          </div>
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Win Rate</div>
                          <div className="text-xl font-bold text-orange-400">{lutzPrivatePerformance.win_rate?.toFixed(1) || '0'}%</div>
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Offene Positionen</div>
                          <div className="text-xl font-bold text-orange-400">{lutzPrivatePerformance.open_positions || 0}</div>
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Unrealisiert</div>
                          <div className={`text-xl font-bold ${(lutzPrivatePerformance.unrealized_pl_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {(lutzPrivatePerformance.unrealized_pl_pct || 0) >= 0 ? '+' : ''}{lutzPrivatePerformance.unrealized_pl_pct?.toFixed(2) || '0.00'}%
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
                                { key: 'buy_date', label: 'Kaufdatum', align: 'left' }
                              ].map(col => (
                                <th key={col.key}
                                  className={`p-2 cursor-pointer hover:text-orange-400 select-none ${col.align === 'right' ? 'text-right' : ''}`}
                                  onClick={() => {
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
                                <td className="p-2 font-medium text-white">{pos.symbol}</td>
                                <td className="p-2 text-gray-400 text-sm">{pos.name}</td>
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
                                <td className="p-2 text-gray-500 text-sm">{new Date(pos.buy_date).toLocaleDateString('de-DE')}</td>
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
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Gesamt Rendite</div>
                          <div className={`text-xl font-bold ${
                            quantPrivatePerformance.overall_return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                          }`}>
                            {quantPrivatePerformance.overall_return_pct >= 0 ? '+' : ''}
                            {quantPrivatePerformance.overall_return_pct?.toFixed(2) || '0.00'}%
                          </div>
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Win Rate</div>
                          <div className="text-xl font-bold text-white">
                            {quantPrivatePerformance.win_rate?.toFixed(1) || '0.0'}%
                          </div>
                          <div className="text-xs text-gray-500">
                            {quantPrivatePerformance.wins || 0}W / {quantPrivatePerformance.losses || 0}L
                          </div>
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Offene Positionen</div>
                          <div className="text-xl font-bold text-violet-400">
                            {quantPrivatePerformance.open_positions || 0}
                          </div>
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Unrealisiert</div>
                          <div className={`text-xl font-bold ${
                            (quantPrivatePerformance.unrealized_pl_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'
                          }`}>
                            {(quantPrivatePerformance.unrealized_pl_pct || 0) >= 0 ? '+' : ''}
                            {quantPrivatePerformance.unrealized_pl_pct?.toFixed(2) || '0.00'}%
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
                                { key: 'tsl', label: `TSL (${quantConfig.tsl_percent || 20}%)`, align: 'right' },
                                { key: 'total_return_pct', label: 'Rendite (Wert)', align: 'right' },
                                { key: 'buy_date', label: 'Kaufdatum', align: 'left' }
                              ].map(col => (
                                <th
                                  key={col.key}
                                  className={`p-2 cursor-pointer hover:text-violet-400 select-none ${col.align === 'right' ? 'text-right' : ''}`}
                                  onClick={() => {
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
                                const tslPercent = quantConfig.tsl_percent || 20
                                const stopPrice = (pos.current_price || 0) * (1 - tslPercent / 100)
                                const isNearStop = pos.current_price && stopPrice && (pos.current_price - stopPrice) / pos.current_price < 0.05
                                return (
                                  <tr key={pos.id} className="border-b border-dark-700/50 hover:bg-dark-700/30">
                                    <td className="p-2 font-medium text-white">{pos.symbol}</td>
                                    <td className="p-2 text-gray-400 text-sm">{pos.name}</td>
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
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Gesamt Rendite</div>
                          <div className={`text-xl font-bold ${
                            ditzPrivatePerformance.overall_return_pct >= 0 ? 'text-green-400' : 'text-red-400'
                          }`}>
                            {ditzPrivatePerformance.overall_return_pct >= 0 ? '+' : ''}
                            {ditzPrivatePerformance.overall_return_pct?.toFixed(2) || '0.00'}%
                          </div>
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Win Rate</div>
                          <div className="text-xl font-bold text-white">
                            {ditzPrivatePerformance.win_rate?.toFixed(1) || '0.0'}%
                          </div>
                          <div className="text-xs text-gray-500">
                            {ditzPrivatePerformance.wins || 0}W / {ditzPrivatePerformance.losses || 0}L
                          </div>
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Offene Positionen</div>
                          <div className="text-xl font-bold text-cyan-400">
                            {ditzPrivatePerformance.open_positions || 0}
                          </div>
                        </div>
                        <div className="bg-dark-800 rounded-lg p-3">
                          <div className="text-xs text-gray-500 mb-1">Unrealisiert</div>
                          <div className={`text-xl font-bold ${
                            (ditzPrivatePerformance.unrealized_pl_pct || 0) >= 0 ? 'text-green-400' : 'text-red-400'
                          }`}>
                            {(ditzPrivatePerformance.unrealized_pl_pct || 0) >= 0 ? '+' : ''}
                            {ditzPrivatePerformance.unrealized_pl_pct?.toFixed(2) || '0.00'}%
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
                                { key: 'tsl', label: `TSL (${ditzConfig.tsl_percent || 20}%)`, align: 'right' },
                                { key: 'total_return_pct', label: 'Rendite (Wert)', align: 'right' },
                                { key: 'buy_date', label: 'Kaufdatum', align: 'left' }
                              ].map(col => (
                                <th
                                  key={col.key}
                                  className={`p-2 cursor-pointer hover:text-cyan-400 select-none ${col.align === 'right' ? 'text-right' : ''}`}
                                  onClick={() => {
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
                                const tslPercent = ditzConfig.tsl_percent || 20
                                const stopPrice = (pos.current_price || 0) * (1 - tslPercent / 100)
                                const isNearStop = pos.current_price && stopPrice && (pos.current_price - stopPrice) / pos.current_price < 0.05
                                return (
                                  <tr key={pos.id} className="border-b border-dark-700/50 hover:bg-dark-700/30">
                                    <td className="p-2 font-medium text-white">{pos.symbol}</td>
                                    <td className="p-2 text-gray-400 text-sm">{pos.name}</td>
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

                {/* Trades Table */}
                <div className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden">
                  <div className="p-4 border-b border-dark-600 flex items-center justify-between">
                    <div>
                      <h2 className="text-lg font-semibold text-white">
                        {botTab === 'flipper' ? 'FlipperBot' : botTab === 'lutz' ? 'Lutz' : botTab === 'ditz' ? 'Ditz' : 'Quant'} Trades
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
                          const trades = botTab === 'flipper' ? flipperTrades : botTab === 'lutz' ? lutzTrades : botTab === 'ditz' ? ditzTrades : quantTrades
                          return ((botTab === 'quant' || botTab === 'ditz') ? getGroupedTrades(trades) : trades).slice(0, 50)
                        })().map((trade) => (
                          <tr key={trade.id} className={`border-b border-dark-700/50 hover:bg-dark-700/30 ${
                            trade.is_deleted ? 'opacity-50' : ''
                          } ${trade.is_live && !trade.is_deleted ? 'bg-green-500/5' : ''} ${
                            botTab === 'flipper' && !trade.is_read && !trade.is_deleted ? 'bg-blue-500/5 border-l-2 border-l-blue-500' : ''
                          } ${
                            botTab === 'lutz' && !trade.is_read && !trade.is_deleted ? 'bg-orange-500/5 border-l-2 border-l-orange-500' : ''
                          } ${
                            botTab === 'quant' && !trade.is_read && !trade.is_deleted ? 'bg-violet-500/5 border-l-2 border-l-violet-500' : ''
                          } ${
                            botTab === 'ditz' && !trade.is_read && !trade.is_deleted ? 'bg-cyan-500/5 border-l-2 border-l-cyan-500' : ''
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
                        {(botTab === 'flipper' ? flipperTrades : botTab === 'lutz' ? lutzTrades : botTab === 'ditz' ? ditzTrades : quantTrades).length === 0 && (
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
                    <div className="px-4 pb-4 flex gap-3">
                      <button
                        onClick={async () => {
                          if (!confirm('FlipperBot komplett zurücksetzen? Alle Trades und Positionen werden gelöscht!')) return
                          try {
                            await fetch('/api/flipperbot/reset', { method: 'POST', headers: { 'Authorization': `Bearer ${token}` } })
                            fetchBotData()
                            alert('FlipperBot wurde zurückgesetzt')
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
                            await fetch('/api/lutz/reset', { method: 'POST', headers: { 'Authorization': `Bearer ${token}` } })
                            fetchBotData()
                            alert('Lutz wurde zurückgesetzt')
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
                            await fetch('/api/quant/reset', { method: 'POST', headers: { 'Authorization': `Bearer ${token}` } })
                            fetchBotData()
                            alert('Quant wurde zurückgesetzt')
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
                            await fetch('/api/ditz/reset', { method: 'POST', headers: { 'Authorization': `Bearer ${token}` } })
                            fetchBotData()
                            alert('Ditz wurde zurückgesetzt')
                          } catch (err) { alert('Fehler beim Zurücksetzen') }
                        }}
                        className="px-4 py-2 bg-cyan-500/20 text-cyan-400 rounded-lg hover:bg-cyan-500/30 transition-colors font-medium text-sm flex items-center gap-2"
                      >
                        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                        Ditz Reset
                      </button>
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
                    : 'bg-violet-500/10 border border-violet-500/30'
                }`}>
                  <button
                    onClick={() => setShowBackfill(!showBackfill)}
                    className={`w-full p-4 flex items-center justify-between text-sm font-medium ${
                      botTab === 'flipper' ? 'text-purple-300' : botTab === 'lutz' ? 'text-orange-300' : botTab === 'ditz' ? 'text-cyan-300' : 'text-violet-300'
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
                      </div>
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
    </div>
  )
}

export default AdminPanel
