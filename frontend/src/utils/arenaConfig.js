// Shared configuration for Trading Arena v1 and v2

export const INTERVALS = ['5m', '15m', '1h', '2h', '4h', '1D', '1W']

export const INTERVAL_MAP = {
  '5m': '5m', '15m': '15m', '1h': '60m',
  '2h': '2h', '4h': '4h', '1D': '1d', '1W': '1wk',
}

export const TV_INTERVAL_MAP = {
  '5m': '5', '15m': '15', '1h': '60',
  '2h': '120', '4h': '240', '1D': 'D', '1W': 'W',
}

export const STRATEGIES = [
  { value: 'regression_scalping', label: 'Regression Scalping', beta: true },
  { value: 'hybrid_ai_trend', label: 'NW Bollinger Bands' },
  { value: 'smart_money_flow', label: 'Smart Money Flow', beta: true },
  { value: 'hann_trend', label: 'Hann Trend (DMH + SAR)' },
  { value: 'gmma_pullback', label: 'GMMA Pullback' },
]

export const STRATEGY_PARAMS = {
  regression_scalping: [
    { key: 'degree', label: 'Degree', default: 2, min: 1, max: 5, step: 1 },
    { key: 'length', label: 'LinReg Length', default: 100, min: 20, max: 300, step: 10 },
    { key: 'multiplier', label: 'LinReg Multiplier', default: 3.0, min: 0.5, max: 5.0, step: 0.1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 1.5, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'sl_lookback', label: 'SL Lookback', default: 30, min: 5, max: 100, step: 5 },
    { key: 'confirmation_required', label: 'Confirmation', default: 1, min: 0, max: 1, step: 1, isToggle: true },
  ],
  hybrid_ai_trend: [
    { key: 'bb1_period', label: 'BB1 Period', default: 20, min: 5, max: 50, step: 1 },
    { key: 'bb1_stdev', label: 'BB1 StDev', default: 3.0, min: 1.0, max: 6.0, step: 0.25 },
    { key: 'bb2_period', label: 'BB2 Period', default: 75, min: 20, max: 200, step: 5 },
    { key: 'bb2_stdev', label: 'BB2 StDev', default: 3.0, min: 1.0, max: 6.0, step: 0.25 },
    { key: 'bb3_period', label: 'BB3 Period', default: 100, min: 50, max: 300, step: 5 },
    { key: 'bb3_stdev', label: 'BB3 StDev', default: 4.0, min: 1.0, max: 6.0, step: 0.25 },
    { key: 'bb4_period', label: 'BB4 Period', default: 100, min: 50, max: 300, step: 5 },
    { key: 'bb4_stdev', label: 'BB4 StDev', default: 4.25, min: 1.0, max: 6.0, step: 0.25 },
    { key: 'nw_bandwidth', label: 'NW Smoothing', default: 6.0, min: 1.0, max: 15.0, step: 0.5 },
    { key: 'nw_lookback', label: 'NW Lookback', default: 499, min: 50, max: 999, step: 10 },
    { key: 'sl_buffer', label: 'SL Buffer %', default: 1.5, min: 0, max: 5.0, step: 0.1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 2.0, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'hybrid_filter', label: 'mit Hybrid AlgoAI?', default: 1, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'hybrid_long_thresh', label: 'Threshold Long', default: 75, min: 0, max: 100, step: 1 },
    { key: 'hybrid_short_thresh', label: 'Threshold Short', default: 25, min: 0, max: 100, step: 1 },
    { key: 'confirm_candle', label: 'Bestaetigungskerze?', default: 0, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'min_band_dist', label: 'Min Band-Abstand %', default: 0, min: 0, max: 3.0, step: 0.1 },
  ],
  // diamond_signals: entfernt — unprofitabel/unnütz

  smart_money_flow: [
    { key: 'trend_length', label: 'Trend Length', default: 34, min: 10, max: 100, step: 1 },
    { key: 'basis_smooth', label: 'Trend Smoothing', default: 3, min: 1, max: 10, step: 1 },
    { key: 'flow_window', label: 'Flow Window', default: 24, min: 5, max: 60, step: 1 },
    { key: 'flow_smooth', label: 'Flow Smoothing', default: 5, min: 1, max: 15, step: 1 },
    { key: 'flow_boost', label: 'Flow Boost', default: 1.2, min: 0.5, max: 3.0, step: 0.1 },
    { key: 'atr_length', label: 'ATR Length', default: 14, min: 5, max: 50, step: 1 },
    { key: 'band_tightness', label: 'Band Tightness', default: 0.9, min: 0.1, max: 2.0, step: 0.1 },
    { key: 'band_expansion', label: 'Band Expansion', default: 2.2, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'dot_cooldown', label: 'Retest Cooldown', default: 12, min: 0, max: 30, step: 1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 2.0, min: 1.0, max: 5.0, step: 0.1 },
  ],
  hann_trend: [
    { key: 'dmh_length', label: 'DMH Length', default: 30, min: 5, max: 80, step: 1 },
    { key: 'sar_start', label: 'SAR Start', default: 0.02, min: 0.005, max: 0.1, step: 0.005 },
    { key: 'sar_increment', label: 'SAR Increment', default: 0.03, min: 0.005, max: 0.1, step: 0.005 },
    { key: 'sar_max', label: 'SAR Max', default: 0.3, min: 0.1, max: 0.5, step: 0.01 },
    { key: 'swing_lookback', label: 'Swing Lookback', default: 5, min: 2, max: 20, step: 1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 2.0, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'sl_buffer', label: 'SL Buffer %', default: 0.3, min: 0, max: 3.0, step: 0.1 },
  ],
  gmma_pullback: [
    { key: 'signal_len', label: 'Signal EMA', default: 9, min: 3, max: 30, step: 1 },
    { key: 'smooth_len', label: 'Smoothing SMA', default: 3, min: 1, max: 10, step: 1 },
    { key: 'fractal_periods', label: 'Fractal Periods', default: 5, min: 2, max: 15, step: 1 },
    { key: 'zone_count', label: 'Aktive Zonen', default: 5, min: 1, max: 10, step: 1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 2.0, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'sl_lookback', label: 'SL Lookback', default: 10, min: 3, max: 30, step: 1 },
    { key: 'sl_buffer', label: 'SL Buffer %', default: 0.3, min: 0, max: 3.0, step: 0.1 },
  ],
}

export const STRATEGY_DEFAULT_INTERVAL = {
  regression_scalping: '5m',
  hybrid_ai_trend: '5m',
  smart_money_flow: '4h',
  hann_trend: '1h',
  gmma_pullback: '1h',
}

export function getDefaultParams(strategy) {
  const defs = STRATEGY_PARAMS[strategy] || []
  const obj = {}
  defs.forEach(p => { obj[p.key] = p.default })
  return obj
}
