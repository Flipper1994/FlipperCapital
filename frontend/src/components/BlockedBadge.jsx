export default function BlockedBadge({ symbol, blockedStocks }) {
  if (!blockedStocks || !blockedStocks[symbol]) return null
  const bots = blockedStocks[symbol]
  return (
    <span
      className="inline-flex items-center text-[9px] px-1 py-0.5 bg-red-500/20 text-red-400 rounded font-medium ml-1"
      title={`Gesperrt für: ${bots.join(', ')}`}
    >
      <svg className="w-2.5 h-2.5 mr-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636" />
      </svg>
      GESPERRT
    </span>
  )
}
