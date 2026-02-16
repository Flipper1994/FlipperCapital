import { useState, useEffect } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { CurrencyProvider } from './context/CurrencyContext'
import { TradingModeProvider } from './context/TradingModeContext'
import Header from './components/Header'
import Sidebar from './components/Sidebar'
import Dashboard from './components/Dashboard'
import Login from './components/Login'
import Register from './components/Register'
import PortfolioManage from './components/PortfolioManage'
import PortfolioCompare from './components/PortfolioCompare'
import StockTracker from './components/StockTracker'
import AdminPanel from './components/AdminPanel'
import FlipperBot from './components/FlipperBot'
import FlipperBotLab from './components/FlipperBotLab'
import LutzLab from './components/LutzLab'
import QuantLab from './components/QuantLab'
import DitzLab from './components/DitzLab'
import TraderLab from './components/TraderLab'
import Performance from './components/Performance'
import SignalList from './components/SignalList'
import Profile from './components/Profile'
import Help from './components/Help'
import TradingArena from './components/TradingArena'
import LiveTrading from './components/LiveTrading'
import DaytradingStats from './components/DaytradingStats'
import BacktestLab from './components/BacktestLab'

function UnderConstruction() {
  return (
    <div className="flex-1 flex items-center justify-center p-8">
      <div className="text-center max-w-md">
        <div className="text-6xl mb-6">
          🚧🏗️🚧
        </div>
        <h1 className="text-2xl font-bold text-white mb-3">Im Aufbau</h1>
        <p className="text-gray-400 mb-6">
          Dieser Bereich ist derzeit noch in der Entwicklung und nur für Administratoren zugänglich.
        </p>
        <div className="flex justify-center gap-2 text-4xl">
          🦺⚠️🔧
        </div>
      </div>
    </div>
  )
}

function App() {
  const [isAdmin, setIsAdmin] = useState(false)
  const [isLoggedIn, setIsLoggedIn] = useState(false)
  const [authLoading, setAuthLoading] = useState(!!localStorage.getItem('authToken'))
  const [user, setUser] = useState(null)
  const [token, setToken] = useState(localStorage.getItem('authToken') || '')
  const [sidebarOpen, setSidebarOpen] = useState(false)

  useEffect(() => {
    if (token) {
      verifyToken()
    }
  }, [])

  const verifyToken = async () => {
    try {
      const res = await fetch('/api/verify', {
        headers: { 'Authorization': `Bearer ${token}` }
      })
      const data = await res.json()
      if (data.valid) {
        setIsLoggedIn(true)
        setIsAdmin(data.user?.is_admin || false)
        setUser(data.user || null)
      } else {
        clearAuth()
      }
    } catch {
      clearAuth()
    } finally {
      setAuthLoading(false)
    }
  }

  const clearAuth = () => {
    localStorage.removeItem('authToken')
    setToken('')
    setIsLoggedIn(false)
    setIsAdmin(false)
    setUser(null)
  }

  const handleLogin = (newToken, userData) => {
    setToken(newToken)
    localStorage.setItem('authToken', newToken)
    setIsLoggedIn(true)
    setIsAdmin(userData?.is_admin || false)
    setUser(userData)
  }

  const handleLogout = async () => {
    try {
      await fetch('/api/logout', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${token}` }
      })
    } catch {
      // Ignore errors, just clear local state
    }
    clearAuth()
  }

  return (
    <CurrencyProvider>
      <TradingModeProvider>
        <div className="min-h-screen bg-dark-900 flex flex-col">
          <Header
            isLoggedIn={isLoggedIn}
            isAdmin={isAdmin}
            user={user}
            onLogout={handleLogout}
            sidebarOpen={sidebarOpen}
            setSidebarOpen={setSidebarOpen}
          />
          <div className="flex flex-1 overflow-x-hidden">
            <Sidebar
              isLoggedIn={isLoggedIn}
              isAdmin={isAdmin}
              user={user}
              isOpen={sidebarOpen}
              onClose={() => setSidebarOpen(false)}
            />
            <Routes>
              <Route
                path="/"
                element={
                  <Dashboard
                    isAdmin={isAdmin}
                    token={token}
                  />
                }
              />
              <Route
                path="/login"
                element={<Login onLogin={handleLogin} />}
              />
              <Route
                path="/register"
                element={<Register onLogin={handleLogin} />}
              />
              <Route
                path="/portfolio"
                element={authLoading ? null : isLoggedIn ? <PortfolioManage /> : <Navigate to="/login" />}
              />
              <Route
                path="/profile"
                element={authLoading ? null : isLoggedIn ? <Profile /> : <Navigate to="/login" />}
              />
              <Route
                path="/compare"
                element={authLoading ? null : isLoggedIn ? <PortfolioCompare user={user} isAdmin={isAdmin} /> : <Navigate to="/login" />}
              />
              <Route
                path="/tracker"
                element={<StockTracker />}
              />
              <Route
                path="/performance"
                element={<Performance token={token} />}
              />
              <Route
                path="/signal-liste"
                element={<SignalList token={token} isAdmin={isAdmin} />}
              />
              <Route
                path="/admin"
                element={<AdminPanel />}
              />
              <Route
                path="/flipperbot"
                element={<FlipperBot />}
              />
              <Route
                path="/help"
                element={<Help />}
              />
              <Route
                path="/flipperbot-lab"
                element={<FlipperBotLab isAdmin={isAdmin} isLoggedIn={isLoggedIn} token={token} />}
              />
              <Route
                path="/lutz-lab"
                element={<LutzLab isAdmin={isAdmin} isLoggedIn={isLoggedIn} token={token} />}
              />
              <Route
                path="/quant-lab"
                element={<QuantLab isAdmin={isAdmin} isLoggedIn={isLoggedIn} token={token} />}
              />
              <Route
                path="/ditz-lab"
                element={<DitzLab isAdmin={isAdmin} isLoggedIn={isLoggedIn} token={token} />}
              />
              <Route
                path="/trader-lab"
                element={<TraderLab isAdmin={isAdmin} isLoggedIn={isLoggedIn} token={token} />}
              />
              <Route
                path="/backtest-lab"
                element={authLoading ? null : isLoggedIn ?
                  <BacktestLab token={token} isAdmin={isAdmin} /> : <Navigate to="/login" />}
              />
              <Route
                path="/trading-arena"
                element={authLoading ? null : isLoggedIn ?
                  <TradingArena isAdmin={isAdmin} token={token} /> : <Navigate to="/login" />}
              />
              <Route
                path="/live-trading"
                element={authLoading ? null : isLoggedIn ?
                  <LiveTrading isAdmin={isAdmin} token={token} /> : <Navigate to="/login" />}
              />
              <Route
                path="/daytrading-stats"
                element={authLoading ? null : isLoggedIn ?
                  <DaytradingStats token={token} isAdmin={isAdmin} /> : <Navigate to="/login" />}
              />
            </Routes>
          </div>
        </div>
      </TradingModeProvider>
    </CurrencyProvider>
  )
}

export default App
