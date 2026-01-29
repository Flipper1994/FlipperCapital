function PortfolioCompare() {
  return (
    <div className="flex-1 p-6">
      <div className="max-w-4xl mx-auto">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-white">Portfolio vergleich</h1>
          <p className="text-gray-500">Vergleiche verschiedene Portfolios und Strategien</p>
        </div>

        <div className="bg-dark-800 rounded-xl border border-dark-600 p-12 text-center">
          <div className="w-20 h-20 mx-auto mb-6 bg-dark-700 rounded-full flex items-center justify-center">
            <svg className="w-10 h-10 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
            </svg>
          </div>
          <h2 className="text-xl font-semibold text-gray-400 mb-3">Demnachst verfugbar</h2>
          <p className="text-gray-600 max-w-md mx-auto">
            Hier wirst du verschiedene Portfolios miteinander vergleichen konnen,
            um die beste Strategie fur deine Investments zu finden.
          </p>
        </div>
      </div>
    </div>
  )
}

export default PortfolioCompare
