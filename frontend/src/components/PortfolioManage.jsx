function PortfolioManage() {
  return (
    <div className="flex-1 p-6">
      <div className="max-w-4xl mx-auto">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-white">Portfolio pflegen</h1>
          <p className="text-gray-500">Verwalte deine Aktien und Investitionen</p>
        </div>

        <div className="bg-dark-800 rounded-xl border border-dark-600 p-12 text-center">
          <div className="w-20 h-20 mx-auto mb-6 bg-dark-700 rounded-full flex items-center justify-center">
            <svg className="w-10 h-10 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
            </svg>
          </div>
          <h2 className="text-xl font-semibold text-gray-400 mb-3">Demnachst verfugbar</h2>
          <p className="text-gray-600 max-w-md mx-auto">
            Hier wirst du deine Aktien-Positionen hinzufugen, bearbeiten und verwalten konnen.
            Diese Funktion wird in Kurze freigeschaltet.
          </p>
        </div>
      </div>
    </div>
  )
}

export default PortfolioManage
