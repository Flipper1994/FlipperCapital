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
  { value: 'gmma_pullback', label: 'GMMA Pullback', disabled: true, disabledReason: 'Nicht profitabel' },
  { value: 'macd_sr', label: 'MACD + S/R' },
  { value: 'trippa_trade', label: 'TrippaTrade RSO', beta: true },
  { value: 'vwap_day_trading', label: 'VWAP Day Trading', beta: true },
  { value: 'gaussian_trend', label: 'Gaussian + RSI + MACD', beta: true },
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
    { key: 'hybrid_filter', label: 'Hybrid AlgoAI?', default: 0, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'hybrid_long_thresh', label: 'Threshold Long', default: 75, min: 0, max: 100, step: 1 },
    { key: 'hybrid_short_thresh', label: 'Threshold Short', default: 25, min: 0, max: 100, step: 1 },
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
  macd_sr: [
    { key: 'macd_fast', label: 'MACD Fast', default: 12, min: 2, max: 50, step: 1 },
    { key: 'macd_slow', label: 'MACD Slow', default: 26, min: 10, max: 100, step: 1 },
    { key: 'macd_signal', label: 'MACD Signal', default: 9, min: 2, max: 30, step: 1 },
    { key: 'ema_period', label: 'EMA Period', default: 200, min: 50, max: 400, step: 10 },
    { key: 'sl_buffer', label: 'SL Buffer %', default: 1.5, min: 0, max: 5.0, step: 0.1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 1.5, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'sr_filter', label: 'S/R Filter?', default: 1, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'fractal_periods', label: 'Fractal Periods', default: 5, min: 2, max: 15, step: 1 },
    { key: 'zone_count', label: 'S/R Zonen', default: 5, min: 1, max: 10, step: 1 },
    { key: 'sr_tolerance', label: 'S/R Toleranz %', default: 1.5, min: 0.5, max: 5.0, step: 0.1 },
    { key: 'hybrid_filter', label: 'Hybrid AlgoAI?', default: 0, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'hybrid_long_thresh', label: 'Threshold Long', default: 75, min: 0, max: 100, step: 1 },
    { key: 'hybrid_short_thresh', label: 'Threshold Short', default: 25, min: 0, max: 100, step: 1 },
  ],
  trippa_trade: [
    { key: 'max_range', label: 'Max Range', default: 100, min: 20, max: 300, step: 5 },
    { key: 'min_range', label: 'Min Range', default: 10, min: 5, max: 50, step: 5 },
    { key: 'reg_step', label: 'Step', default: 5, min: 1, max: 20, step: 1 },
    { key: 'signal_len', label: 'Signal SMA', default: 7, min: 3, max: 30, step: 1 },
    { key: 'ema_fast', label: 'EMA Fast', default: 5, min: 3, max: 20, step: 1 },
    { key: 'ema_slow', label: 'EMA Slow', default: 13, min: 5, max: 50, step: 1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 2.0, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'sl_buffer', label: 'SL Buffer %', default: 0.5, min: 0, max: 3.0, step: 0.1 },
    { key: 'min_trend_bars', label: 'Min Trend Bars', default: 3, min: 1, max: 10, step: 1 },
  ],
  vwap_day_trading: [
    { key: 'band_mult_1', label: 'Band 1 (σ)', default: 2.0, min: 0.5, max: 4.0, step: 0.25 },
    { key: 'band_mult_2', label: 'Band 2 (σ)', default: 3.0, min: 1.0, max: 5.0, step: 0.25 },
    { key: 'pullback_enabled', label: 'Pullback-Modus', default: 1, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'reversal_enabled', label: 'Reversal-Modus', default: 1, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'skip_bars', label: 'Skip Bars (Reversal)', default: 12, min: 0, max: 30, step: 1 },
    { key: 'trend_bars', label: 'Trend-Fenster', default: 6, min: 3, max: 20, step: 1 },
    { key: 'body_pct', label: 'Min Body %', default: 50, min: 20, max: 80, step: 5 },
    { key: 'risk_reward', label: 'Risk/Reward (Pullback)', default: 2.0, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'sl_lookback', label: 'SL Lookback', default: 10, min: 3, max: 30, step: 1 },
    { key: 'sl_buffer', label: 'SL Buffer %', default: 0.3, min: 0, max: 3.0, step: 0.1 },
    { key: 'prev_vwap_filter', label: 'Prev VWAP S/R?', default: 0, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'hybrid_filter', label: 'Hybrid AlgoAI?', default: 0, min: 0, max: 1, step: 1, isToggle: true },
    { key: 'hybrid_long_thresh', label: 'Threshold Long', default: 75, min: 0, max: 100, step: 1 },
    { key: 'hybrid_short_thresh', label: 'Threshold Short', default: 25, min: 0, max: 100, step: 1 },
  ],
  gaussian_trend: [
    { key: 'period', label: 'Gaussian Period', default: 25, min: 5, max: 100, step: 1 },
    { key: 'poles', label: 'Poles', default: 5, min: 1, max: 9, step: 1 },
    { key: 'filter_period', label: 'STD Filter Period', default: 10, min: 5, max: 50, step: 1 },
    { key: 'filter_deviations', label: 'STD Filter Mult', default: 1.0, min: 0, max: 3.0, step: 0.1 },
    { key: 'rsi_period', label: 'RSI Period', default: 30, min: 5, max: 50, step: 1 },
    { key: 'macd_fast', label: 'MACD Fast', default: 24, min: 2, max: 50, step: 1 },
    { key: 'macd_slow', label: 'MACD Slow', default: 52, min: 10, max: 100, step: 1 },
    { key: 'macd_signal', label: 'MACD Signal', default: 9, min: 2, max: 30, step: 1 },
    { key: 'sl_buffer', label: 'SL Buffer %', default: 0.5, min: 0, max: 3.0, step: 0.1 },
    { key: 'risk_reward', label: 'Risk/Reward', default: 1.5, min: 1.0, max: 5.0, step: 0.1 },
    { key: 'sl_lookback', label: 'SL Lookback', default: 10, min: 3, max: 30, step: 1 },
  ],
}

export const STRATEGY_DEFAULT_INTERVAL = {
  regression_scalping: '5m',
  hybrid_ai_trend: '5m',
  smart_money_flow: '4h',
  hann_trend: '1h',
  gmma_pullback: '1h',
  macd_sr: '1h',
  trippa_trade: '1h',
  vwap_day_trading: '5m',
  gaussian_trend: '1h',
}

// Interval-specific recommended presets (scaling relative to 4h baseline)
// Only strategies with meaningful interval-dependent tuning are listed
export const STRATEGY_INTERVAL_PRESETS = {
  smart_money_flow: {
    '1h': {
      label: '1h (skaliert von 4h)',
      params: { trend_length: 136, basis_smooth: 12, flow_window: 96, flow_smooth: 20, atr_length: 56, dot_cooldown: 48, risk_reward: 2.0 },
    },
    '2h': {
      label: '2h (skaliert von 4h)',
      params: { trend_length: 68, basis_smooth: 6, flow_window: 48, flow_smooth: 10, atr_length: 28, dot_cooldown: 24, risk_reward: 2.0 },
    },
    '4h': {
      label: '4h (Standard)',
      params: { trend_length: 34, basis_smooth: 3, flow_window: 24, flow_smooth: 5, atr_length: 14, dot_cooldown: 12, risk_reward: 2.0 },
    },
  },
}

// Machine-readable algorithm descriptions for AI/ML export
export const STRATEGY_ALGORITHMS = {
  regression_scalping: {
    name: 'Regression Scalping',
    description: 'Polynomial regression channel with Awesome Oscillator confirmation and Heikin-Ashi candle filter.',
    indicators: {
      'Awesome Oscillator': 'AO = SMA(midpoint, 5) - SMA(midpoint, 34) where midpoint = (high + low) / 2',
    },
    overlays: {
      'Upper Band': 'polyPredict(close, degree, length)[lag=1] + multiplier * RMSE[lag=1]',
      'Prediction': 'polyPredict(close, degree, length)[lag=1]',
      'Lower Band': 'polyPredict(close, degree, length)[lag=1] - multiplier * RMSE[lag=1]',
    },
    signal_logic: {
      LONG: '1) close < Lower Band (setup), 2) AO flips green (AO[i] > AO[i-1] after AO[i-1] <= AO[i-2]), 3) HA candle bullish (HA_close > HA_open). Steps 2-3 only with confirmation_required=1.',
      SHORT: '1) close > Upper Band (setup), 2) AO flips red, 3) HA candle bearish. Reset if price returns between bands before confirmation.',
    },
    sl_tp: {
      LONG: 'SL = min(low, sl_lookback bars). TP = entry + risk_reward * (entry - SL).',
      SHORT: 'SL = max(high, sl_lookback bars). TP = entry - risk_reward * (SL - entry).',
    },
    entry: 'Next bar open after signal confirmation.',
    heikin_ashi: 'HA_close = (O+H+L+C)/4, HA_open = (prevHA_open + prevHA_close)/2, HA_high = max(H, HA_open, HA_close), HA_low = min(L, HA_open, HA_close)',
  },
  hybrid_ai_trend: {
    name: 'NW Bollinger Bands',
    description: '4-level Bollinger Bands smoothed with Nadaraya-Watson Gaussian kernel. Signal generation uses Level 1 only. Optional Hybrid EMA AlgoLearner filter.',
    indicators: {
      'Hybrid AlgoAI': 'k-NN weighted EMA oscillator: shortEMA=EMA(close,50), longEMA=EMA(close,200), k=5 nearest neighbors in lookback=100, normalized to [0,100] over 400-bar window.',
    },
    overlays: {
      'NW BB Upper/Lower 1-4': 'BB_basis = SMA(HLC3, period), BB_upper = basis + stdev_mult * stdev(HLC3, period), then Nadaraya-Watson smoothing: NW(x_i) = Σ K(x_i, x_j) * y_j / Σ K(x_i, x_j) where K = exp(-offset² / (2*h²)), h=nw_bandwidth, lookback=nw_lookback. Level 1 (period=bb1_period, σ=bb1_stdev) used for signals.',
    },
    signal_logic: {
      LONG: 'close[i] <= NW_lower1[i] AND close[i-1] > NW_lower1[i-1] (cross below). Optional: hybrid_filter requires oscillator >= hybrid_long_thresh. Optional: confirm_candle requires next candle bullish.',
      SHORT: 'close[i] >= NW_upper1[i] AND close[i-1] < NW_upper1[i-1] (cross above). Optional: hybrid_filter requires oscillator <= hybrid_short_thresh.',
    },
    sl_tp: {
      LONG: 'SL = entry * (1 - sl_buffer/100). TP = entry + risk_reward * (entry - SL).',
      SHORT: 'SL = entry * (1 + sl_buffer/100). TP = entry - risk_reward * (SL - entry).',
    },
    entry: 'Next bar open (or bar+2 with confirm_candle=1).',
  },
  smart_money_flow: {
    name: 'Smart Money Flow',
    description: 'Double-EMA basis with Chaikin-style volume flow, adaptive ATR bands, and a 3-phase regime state machine (tracking → armed → entry).',
    indicators: {
      'Smart Money Flow': 'CLV = ((C-L)-(H-C))/(H-L). mfRatio = sum(CLV*vol, flow_window) / sum(|CLV*vol|, flow_window). mfSm = EMA(mfRatio, flow_smooth) * 100. Histogram: green if >=0, red if <0.',
    },
    overlays: {
      'Basis Open': 'EMA(EMA(open, trend_length), basis_smooth)',
      'Basis Close': 'EMA(EMA(close, trend_length), basis_smooth)',
      'Upper Band': 'basisClose + ATR(atr_length) * mult, where mult = band_tightness + (band_expansion - band_tightness) * clamp(|mfSm|^flow_boost, 0, 1)',
      'Lower Band': 'basisClose - ATR(atr_length) * mult',
    },
    signal_logic: {
      regime: 'regime=+1 when close crosses above upper band, regime=-1 when close crosses below lower band, otherwise persistent.',
      LONG: 'In bull regime: 1) phTracking: track swingHigh. 2) phArmed: price dips below basisClose (retest), record pullbackLow. 3) Entry: close > swingHigh (structure break). Cooldown: dot_cooldown bars between entries.',
      SHORT: 'In bear regime: 1) phTracking: track swingLow. 2) phArmed: price rises above basisClose, record pullbackHigh. 3) Entry: close < swingLow.',
    },
    sl_tp: {
      LONG: 'SL = pullbackExtreme (low of retest dip). TP = entry + risk_reward * (entry - SL). Minimum risk > 0.1% of entry.',
      SHORT: 'SL = pullbackExtreme (high of retest bounce). TP = entry - risk_reward * (SL - entry).',
    },
    entry: 'Next bar open after structure break.',
  },
  hann_trend: {
    name: 'Hann Trend (DMH + SAR)',
    description: 'Directional Movement filtered by Hann FIR window combined with Parabolic SAR for 4-phase pullback entries.',
    indicators: {
      'DMH': 'netDM = pDM - mDM (Wilder DM). RMA(netDM, dmh_length) → Hann FIR filter: coefficients c_k = 1 - cos(k * 2π / (period+1)). Histogram: yellow if >0, blue if <=0.',
    },
    overlays: {
      'Parabolic SAR': 'Wilder SAR: AF starts at sar_start, increments by sar_increment (max sar_max). Flip on price crossing SAR.',
    },
    signal_logic: {
      LONG: '1) DMH crosses above 0 → bullish. 2) phWaitPullback: track swingHigh. SAR flips down → phInPullback, freeze swingHigh over swing_lookback bars, track pullbackLow. 3) SAR flips up → phWaitConfirm. 4) close > swingHigh → LONG entry.',
      SHORT: '1) DMH crosses below 0 → bearish. 2) SAR flips up → phInPullback, freeze swingLow, track pullbackHigh. 3) SAR flips down → phWaitConfirm. 4) close < swingLow → SHORT entry.',
    },
    sl_tp: {
      LONG: 'SL = pullbackExtreme * (1 - sl_buffer/100). TP = entry + risk_reward * (entry - SL). Minimum risk > 0.1% of entry.',
      SHORT: 'SL = pullbackExtreme * (1 + sl_buffer/100). TP = entry - risk_reward * (SL - entry).',
    },
    entry: 'Next bar open after swing break.',
    optional_hybrid: 'Hybrid EMA AlgoLearner filter (same as hybrid_ai_trend): shortEMA=50, longEMA=200, k=5, normalized 0-100.',
  },
  gmma_pullback: {
    name: 'GMMA Pullback',
    description: 'Guppy Multiple Moving Average oscillator crossover filtered by fractal Support/Resistance zones.',
    indicators: {
      'GMMA Main': 'fastEMAs=[3,5,8,10,12,15], slowEMAs=[30,35,40,45,50,60]. osc = (sum(fastEMAs) - sum(slowEMAs)) / sum(slowEMAs) * 100. mainLine = SMA(osc, smooth_len).',
      'GMMA Signal': 'sigLine = EMA(osc, signal_len).',
    },
    overlays: {
      'Support Zones': 'Fractal pivot lows (low[pi] < low[pi±j] for j=1..fractal_periods). Zone = [low, body]. Breached if close > pivot + 0.5%. Keep last zone_count unbreached.',
      'Resistance Zones': 'Fractal pivot highs (high[pi] > high[pi±j]). Zone = [body, high]. Keep last zone_count unbreached.',
    },
    signal_logic: {
      LONG: 'mainLine crosses above sigLine AND mainLine > 0 AND sigLine > 0 AND close is inside an active support zone.',
      SHORT: 'mainLine crosses below sigLine AND mainLine < 0 AND sigLine < 0 AND close is inside an active resistance zone.',
    },
    sl_tp: {
      LONG: 'SL = min(low, sl_lookback bars) * (1 - sl_buffer/100). TP = entry + risk_reward * (entry - SL).',
      SHORT: 'SL = max(high, sl_lookback bars) * (1 + sl_buffer/100). TP = entry - risk_reward * (SL - entry).',
    },
    entry: 'Next bar open after crossover inside S/R zone.',
  },
  macd_sr: {
    name: 'MACD + S/R',
    description: 'Classic MACD crossover below/above zero line with EMA(200) trend filter and optional fractal S/R zone confirmation.',
    indicators: {
      'MACD Histogram': 'MACD_line - Signal_line. Green if positive, red if negative.',
      'MACD': 'EMA(close, macd_fast) - EMA(close, macd_slow)',
      'Signal': 'EMA(MACD_line, macd_signal)',
      'Hybrid AlgoAI': 'Optional: k-NN EMA oscillator (0-100), same as hybrid_ai_trend.',
    },
    overlays: {
      'EMA(200)': 'EMA(close, ema_period). Trend filter: LONG only above, SHORT only below.',
      'S/R Zones': 'Optional (sr_filter=1): Fractal pivots with fractal_periods, zone_count zones, sr_tolerance % proximity required.',
    },
    signal_logic: {
      LONG: 'MACD crosses above Signal AND both were below 0 (cross from negative) AND close > EMA(200). Optional: sr_filter requires close within sr_tolerance% of support zone. Optional: hybrid_filter requires oscillator >= hybrid_long_thresh.',
      SHORT: 'MACD crosses below Signal AND both were above 0 AND close < EMA(200). Optional S/R and hybrid filters (inverted).',
    },
    sl_tp: {
      LONG: 'SL = EMA200 * (1 - sl_buffer/100). TP = entry + risk_reward * (entry - SL).',
      SHORT: 'SL = EMA200 * (1 + sl_buffer/100). TP = entry - risk_reward * (SL - entry).',
    },
    entry: 'Next bar open after crossover.',
  },
  trippa_trade: {
    name: 'TrippaTrade RSO',
    description: 'Multi-period log-regression slope oscillator with EMA ribbon for trend-following pullback re-entries.',
    indicators: {
      'RSO': 'For each bar, average OLS log-regression slope over windows L=[min_range, min_range+step, ..., max_range]. slope(L) = (L*Σ(X*ln(close)) - ΣX*Σln(close)) / (L*ΣX² - (ΣX)²), negated. Histogram: green if RSO>0, red if <0.',
      'Signal': 'SMA(RSO, signal_len).',
    },
    overlays: {
      'EMA Fast': 'EMA(close, ema_fast)',
      'EMA Slow': 'EMA(close, ema_slow)',
    },
    signal_logic: {
      trend: 'RSO > 0 = bullish, RSO < 0 = bearish. Trend confirmed after min_trend_bars consecutive bars.',
      LONG: 'Confirmed bullish trend → pullback detected (EMA_fast < EMA_slow AND RSO < Signal) → re-entry (EMA_fast > EMA_slow AND RSO > Signal).',
      SHORT: 'Confirmed bearish trend → pullback detected (EMA_fast > EMA_slow AND RSO > Signal) → re-entry (EMA_fast < EMA_slow AND RSO < Signal).',
    },
    sl_tp: {
      LONG: 'SL = pullbackLow * (1 - sl_buffer/100). TP = entry + risk_reward * (entry - SL).',
      SHORT: 'SL = pullbackHigh * (1 + sl_buffer/100). TP = entry - risk_reward * (SL - entry).',
    },
    entry: 'Next bar open after re-entry confirmation.',
  },
  vwap_day_trading: {
    name: 'VWAP Day Trading',
    description: 'Intraday VWAP with standard deviation bands. Two signal modes: Pullback (trend continuation at VWAP) and Reversal (mean-reversion from ±2σ band).',
    indicators: {
      'VWAP Dist (σ)': '(close - VWAP) / SD. Histogram: green if >0, red if <0.',
    },
    overlays: {
      'VWAP': 'Cumulative TP*Vol / CumVol where TP=(H+L+C)/3. Resets each trading day (gap > 4h).',
      'Upper/Lower 2σ': 'VWAP ± band_mult_1 * SD where SD = sqrt(cumTP²V/cumVol - VWAP²)',
      'Upper/Lower 3σ': 'VWAP ± band_mult_2 * SD',
      'Prev VWAP': 'Previous day closing VWAP level.',
    },
    signal_logic: {
      pullback_LONG: 'barsIntoDay >= trend_bars AND >=75% of last trend_bars bars above VWAP AND vwapSlope > 0 AND low touched VWAP (low <= VWAP*1.002) AND prevClose > prevVWAP AND bullish candle (close>open, bodyPct>=body_pct, close>VWAP).',
      pullback_SHORT: 'Mirror: >=75% below VWAP, vwapSlope < 0, high touched VWAP, bearish candle.',
      reversal_SHORT: 'barsIntoDay >= skip_bars AND sideways market (no strong trend) AND high >= Upper1 (2σ) AND bearish candle.',
      reversal_LONG: 'low <= Lower1 (2σ) AND bullish candle in sideways market.',
      cooldown: '3 bars minimum between signals.',
    },
    sl_tp: {
      pullback_LONG: 'SL = min(low, sl_lookback bars) * (1 - sl_buffer/100). TP = entry + risk_reward * (entry - SL).',
      pullback_SHORT: 'SL = max(high, sl_lookback bars) * (1 + sl_buffer/100). TP = entry - risk_reward * (SL - entry).',
      reversal_LONG: 'SL = min(candle.Low * (1-sl_buffer/100), Lower2). TP = VWAP (mean-reversion). Minimum R/R >= 0.8.',
      reversal_SHORT: 'SL = max(candle.High * (1+sl_buffer/100), Upper2). TP = VWAP.',
    },
    entry: 'Next bar open after signal.',
    optional_hybrid: 'Hybrid EMA AlgoLearner (shortEMA=8, longEMA=21, k=5, lookback=100, normLookback=50).',
  },
  gaussian_trend: {
    name: 'Gaussian + RSI + MACD',
    description: 'STD-Filtered N-Pole Gaussian Filter (Ehlers/Loxx) combined with RSI and MACD histogram confirmation. Trend direction from Gaussian color changes, confirmed by momentum.',
    indicators: {
      'RSI': 'RSI(rsi_period). Bullish > 50, bearish < 50.',
      'MACD Histogram': 'EMA(close, macd_fast) - EMA(close, macd_slow), then Signal = EMA(MACD, macd_signal). Histogram = MACD - Signal. Green > 0, red < 0.',
    },
    overlays: {
      'Gaussian Filter': 'Ehlers N-Pole recursive filter: alpha = -beta + sqrt(beta²+2*beta), beta = (1-cos(2π/period)) / (sqrt(2)^(2/poles) - 1). N-pole with binomial coefficients. STD-filter: only update when |src-filter| > deviations * stdev(src, filter_period). Green when rising, red when falling.',
    },
    signal_logic: {
      LONG: 'Gaussian turns green (was red, i.e. filter[i] > filter[i-1] after filter[i-1] <= filter[i-2]) AND RSI > 50 AND MACD histogram > 0.',
      SHORT: 'Gaussian turns red (was green) AND RSI < 50 AND MACD histogram < 0.',
    },
    sl_tp: {
      LONG: 'SL = min(Gaussian filter value, swing low over sl_lookback bars) * (1 - sl_buffer/100). TP = entry + risk_reward * (entry - SL).',
      SHORT: 'SL = max(Gaussian filter value, swing high over sl_lookback bars) * (1 + sl_buffer/100). TP = entry - risk_reward * (SL - entry).',
    },
    entry: 'Next bar open after signal.',
  },
}

export function getDefaultParams(strategy) {
  const defs = STRATEGY_PARAMS[strategy] || []
  const obj = {}
  defs.forEach(p => { obj[p.key] = p.default })
  return obj
}

export function getPresetParams(strategy, interval) {
  const presets = STRATEGY_INTERVAL_PRESETS[strategy]
  if (!presets || !presets[interval]) return null
  return presets[interval]
}
