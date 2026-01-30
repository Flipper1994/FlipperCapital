import { useState } from 'react'
import { Link } from 'react-router-dom'

function Help() {
  const [expandedSection, setExpandedSection] = useState(null)

  const sections = [
    {
      id: 'dashboard',
      title: 'Dashboard',
      icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6',
      content: [
        {
          subtitle: 'Watchlist',
          text: 'Die Watchlist zeigt alle verfolgten Aktien mit Echtzeit-Kursen. Kurse werden alle 30 Sekunden automatisch aktualisiert. Klicke auf eine Aktie, um den Chart und die technische Analyse zu sehen.'
        },
        {
          subtitle: 'Chart-Ansicht',
          text: 'Nach Auswahl einer Aktie siehst du den TradingView-Chart. Du kannst zwischen monatlicher (M), wöchentlicher (W) und täglicher (D) Ansicht wechseln.'
        },
        {
          subtitle: 'B-Xtrender Analyse',
          text: 'Unter dem Chart erscheint der B-Xtrender Indikator - ein technischer Oszillator, der automatisch BUY, SELL, HOLD oder WAIT Signale generiert. Die Signale basieren auf dem Momentum der Aktie.'
        },
        {
          subtitle: 'System Performance',
          text: 'Die eingeklappte Sektion "System Performance" zeigt die historischen Trades und Statistiken (Win Rate, Risk/Reward, Gesamtrendite) des B-Xtrender-Systems für die ausgewählte Aktie.'
        }
      ]
    },
    {
      id: 'tracker',
      title: 'Aktien Tracker',
      icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
      content: [
        {
          subtitle: 'Signal-Übersicht',
          text: 'Der Aktien Tracker zeigt alle Aktien mit ihren aktuellen Trading-Signalen. Die Statistik-Karten oben zeigen, wie viele Aktien aktuell ein BUY, HOLD oder SELL/WAIT Signal haben.'
        },
        {
          subtitle: 'Filtern & Sortieren',
          text: 'Klicke auf die Signal-Filter (BUY, HOLD, SELL/WAIT), um nur Aktien mit diesem Signal anzuzeigen. Die Tabelle kann nach jeder Spalte sortiert werden.'
        },
        {
          subtitle: 'Trade-Historie',
          text: 'Klicke auf eine Zeile, um die vollständige Trade-Historie dieser Aktie zu sehen - mit allen Einstiegs- und Ausstiegspunkten sowie den jeweiligen Renditen.'
        }
      ]
    },
    {
      id: 'portfolio',
      title: 'Mein Portfolio',
      icon: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01',
      content: [
        {
          subtitle: 'Position hinzufügen',
          text: 'Suche oben nach einer Aktie (z.B. "AAPL" oder "Apple"). Nach Auswahl kannst du Kaufkurs, Währung, Anzahl und Kaufdatum eingeben.'
        },
        {
          subtitle: 'Performance-Übersicht',
          text: 'Das Dashboard zeigt deinen Gesamtwert, investiertes Kapital, Gewinn/Verlust und Gesamtrendite. Der Chart visualisiert die historische Entwicklung.'
        },
        {
          subtitle: 'Währungsumrechnung',
          text: 'Aktuelle Kurse werden automatisch von USD in deine gewählte Währung umgerechnet. Du siehst immer den aktuellen Wert in deiner Währung.'
        },
        {
          subtitle: 'Bearbeiten & Löschen',
          text: 'Über die Icons rechts kannst du jede Position bearbeiten oder löschen. Änderungen werden sofort gespeichert.'
        }
      ]
    },
    {
      id: 'compare',
      title: 'Portfolio Vergleich',
      icon: 'M16 8v8m-4-5v5m-4-2v2m-2 4h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z',
      content: [
        {
          subtitle: 'Ranking',
          text: 'Sieh, wie dein Portfolio im Vergleich zu anderen Nutzern abschneidet. Die Rangliste sortiert alle Nutzer nach ihrer Gesamtrendite.'
        },
        {
          subtitle: 'Performance-Chart',
          text: 'Klicke auf einen Nutzer im Ranking, um dessen Portfolio-Verlauf im Chart zu sehen.'
        },
        {
          subtitle: 'Portfolio-Details',
          text: 'Klicke auf ein Portfolio in der Detail-Liste, um alle Positionen dieses Nutzers zu sehen - inklusive Kaufkurs und aktueller Rendite.'
        }
      ]
    },
    {
      id: 'currency',
      title: 'Währungseinstellungen',
      icon: 'M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
      content: [
        {
          subtitle: 'Währung wählen',
          text: 'Oben rechts im Header findest du die Währungsauswahl. Wähle zwischen USD, EUR, GBP oder CHF.'
        },
        {
          subtitle: 'Live-Umrechnung',
          text: 'Alle Preise werden automatisch mit dem aktuellen Wechselkurs umgerechnet. Die Umrechnung erfolgt in Echtzeit.'
        }
      ]
    },
    {
      id: 'flipperbot-lab',
      title: 'FlipperBot Lab (Beta)',
      icon: 'M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z',
      content: [
        {
          subtitle: 'Was ist FlipperBot Lab?',
          text: 'FlipperBot Lab ist ein experimentelles Feature, das automatisiert nach den B-Xtrender Signalen handelt. Der Bot simuliert Trades seit dem 01.01.2026 und kauft bei BUY-Signal jeweils 1 Aktie zum historischen Kurs des Signal-Datums.'
        },
        {
          subtitle: 'Wie funktioniert es?',
          text: 'Der Bot prüft alle Aktien im Aktien Tracker. Bei einem BUY-Signal wird 1 Aktie zum Kurs des Signal-Tages gekauft (nur an Werktagen). Bei SELL/WAIT wird die Position verkauft. HOLD-Signale bedeuten, dass die Position gehalten wird.'
        },
        {
          subtitle: 'Update & Trade',
          text: 'Klicke auf "Update & Trade", um neue Signale zu verarbeiten. Der Bot prüft dann alle Aktien und führt entsprechende Käufe oder Verkäufe aus. Das Debug-Log zeigt detailliert, welche Entscheidungen getroffen wurden.'
        },
        {
          subtitle: 'Portfolio Rendite',
          text: 'Oben siehst du die Gesamtrendite des FlipperBot-Portfolios, bestehend aus investiertem Kapital, aktuellem Wert und der prozentualen Rendite. Die Statistiken zeigen Win Rate und Anzahl der Trades.'
        },
        {
          subtitle: 'Im Portfolio Vergleich',
          text: 'Der FlipperBot erscheint auch im Portfolio Vergleich als eigener Nutzer. So kannst du sehen, wie gut die automatische Strategie im Vergleich zu anderen Nutzern abschneidet.'
        },
        {
          subtitle: 'Reset',
          text: 'Mit dem Reset-Button kannst du alle FlipperBot-Daten löschen und von vorne beginnen. Nützlich, wenn du die Strategie neu starten möchtest.'
        }
      ]
    },
    {
      id: 'flipperbot',
      title: 'FlipperBot (Premium)',
      icon: 'M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z',
      content: [
        {
          subtitle: 'Coming Soon',
          text: 'FlipperBot ist unser kommender Premium-Service für automatisierte Trading-Signale mit Push-Benachrichtigungen, Risikomanagement und mehr.'
        },
        {
          subtitle: 'Benachrichtigung',
          text: 'Trage dich auf der FlipperBot-Seite in die Warteliste ein, um benachrichtigt zu werden, sobald der Service verfügbar ist.'
        }
      ]
    }
  ]

  const toggleSection = (id) => {
    setExpandedSection(expandedSection === id ? null : id)
  }

  return (
    <div className="flex-1 p-4 md:p-6 overflow-auto">
      <div className="max-w-4xl mx-auto">
        {/* Header */}
        <div className="mb-6 md:mb-8">
          <h1 className="text-2xl md:text-3xl font-bold text-white mb-2">Hilfe & Anleitung</h1>
          <p className="text-gray-500">Lerne alle Funktionen von FlipperCapital kennen</p>
        </div>

        {/* Quick Start - Desktop */}
        <div className="hidden md:block bg-gradient-to-br from-accent-500/20 to-accent-600/10 rounded-xl border border-accent-500/30 p-6 mb-8">
          <h2 className="text-xl font-semibold text-white mb-4">Schnellstart</h2>
          <div className="grid grid-cols-3 gap-6">
            <div className="flex items-start gap-3">
              <div className="w-8 h-8 bg-accent-500 rounded-full flex items-center justify-center text-white font-bold shrink-0">1</div>
              <div>
                <h3 className="font-medium text-white">Aktie auswählen</h3>
                <p className="text-sm text-gray-400">Klicke auf eine Aktie in der Watchlist rechts</p>
              </div>
            </div>
            <div className="flex items-start gap-3">
              <div className="w-8 h-8 bg-accent-500 rounded-full flex items-center justify-center text-white font-bold shrink-0">2</div>
              <div>
                <h3 className="font-medium text-white">Signale prüfen</h3>
                <p className="text-sm text-gray-400">Der B-Xtrender zeigt BUY/SELL Signale</p>
              </div>
            </div>
            <div className="flex items-start gap-3">
              <div className="w-8 h-8 bg-accent-500 rounded-full flex items-center justify-center text-white font-bold shrink-0">3</div>
              <div>
                <h3 className="font-medium text-white">Portfolio aufbauen</h3>
                <p className="text-sm text-gray-400">Tracke deine Investments unter "Mein Portfolio"</p>
              </div>
            </div>
          </div>
        </div>

        {/* Quick Start - Mobile */}
        <div className="md:hidden bg-gradient-to-br from-accent-500/20 to-accent-600/10 rounded-xl border border-accent-500/30 p-4 mb-6">
          <h2 className="text-lg font-semibold text-white mb-3">Schnellstart</h2>
          <div className="space-y-3">
            <div className="flex items-center gap-3">
              <div className="w-7 h-7 bg-accent-500 rounded-full flex items-center justify-center text-white text-sm font-bold shrink-0">1</div>
              <p className="text-sm text-gray-300">Aktie in der Watchlist auswählen</p>
            </div>
            <div className="flex items-center gap-3">
              <div className="w-7 h-7 bg-accent-500 rounded-full flex items-center justify-center text-white text-sm font-bold shrink-0">2</div>
              <p className="text-sm text-gray-300">B-Xtrender Signale prüfen</p>
            </div>
            <div className="flex items-center gap-3">
              <div className="w-7 h-7 bg-accent-500 rounded-full flex items-center justify-center text-white text-sm font-bold shrink-0">3</div>
              <p className="text-sm text-gray-300">Portfolio unter "Mein Portfolio" aufbauen</p>
            </div>
          </div>
        </div>

        {/* Sections */}
        <div className="space-y-3">
          {sections.map((section) => (
            <div
              key={section.id}
              className="bg-dark-800 rounded-xl border border-dark-600 overflow-hidden"
            >
              <button
                onClick={() => toggleSection(section.id)}
                className="w-full flex items-center justify-between p-4 md:p-5 hover:bg-dark-700/50 transition-colors"
              >
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 bg-accent-500/20 rounded-lg flex items-center justify-center">
                    <svg className="w-5 h-5 text-accent-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d={section.icon} />
                    </svg>
                  </div>
                  <span className="text-white font-medium">{section.title}</span>
                </div>
                <svg
                  className={`w-5 h-5 text-gray-400 transition-transform duration-200 ${expandedSection === section.id ? 'rotate-180' : ''}`}
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                </svg>
              </button>

              {expandedSection === section.id && (
                <div className="px-4 pb-4 md:px-5 md:pb-5 space-y-4">
                  {section.content.map((item, idx) => (
                    <div key={idx} className="pl-13 md:pl-[52px]">
                      <h4 className="text-accent-400 font-medium text-sm mb-1">{item.subtitle}</h4>
                      <p className="text-gray-400 text-sm leading-relaxed">{item.text}</p>
                    </div>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>

        {/* Back to Dashboard */}
        <div className="mt-8 text-center">
          <Link
            to="/"
            className="inline-flex items-center gap-2 px-6 py-3 bg-accent-500 text-white rounded-lg hover:bg-accent-400 transition-colors font-medium"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
            </svg>
            Zurück zum Dashboard
          </Link>
        </div>
      </div>
    </div>
  )
}

export default Help
