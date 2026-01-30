#!/bin/bash

# Prüfe ob .env.prod existiert
if [ ! -f ".env.prod" ]; then
    echo "❌ Fehler: .env.prod nicht gefunden!"
    echo ""
    echo "Erstelle die Datei mit deinem DuckDNS Token:"
    echo "  cp .env.prod.example .env.prod"
    echo "  nano .env.prod  (oder anderer Editor)"
    echo ""
    echo "Du findest deinen Token auf https://www.duckdns.org nach dem Login"
    exit 1
fi

# Lade Umgebungsvariablen
export $(cat .env.prod | grep -v '^#' | xargs)

if [ "$DUCKDNS_TOKEN" = "dein-token-hier" ] || [ -z "$DUCKDNS_TOKEN" ]; then
    echo "❌ Fehler: Bitte trage deinen DuckDNS Token in .env.prod ein!"
    exit 1
fi

# Prüfe ob Zertifikat bereits existiert
CERT_PATH="certbot/certs/live/flippercapital.duckdns.org/fullchain.pem"

if [ ! -f "$CERT_PATH" ]; then
    echo "🔐 Hole Let's Encrypt Zertifikat (kann 2-3 Minuten dauern)..."

    # Bis zu 3 Versuche
    for i in 1 2 3; do
        echo "   Versuch $i von 3..."
        docker compose -f docker-compose.prod.yml run --rm certbot

        if [ -f "$CERT_PATH" ]; then
            echo "✅ Zertifikat erfolgreich geholt!"
            break
        fi

        if [ $i -lt 3 ]; then
            echo "   Fehlgeschlagen, warte 30 Sekunden vor erneutem Versuch..."
            sleep 30
        fi
    done

    if [ ! -f "$CERT_PATH" ]; then
        echo "❌ Zertifikat konnte nach 3 Versuchen nicht geholt werden!"
        echo "   Prüfe die Logs: cat certbot/logs/letsencrypt.log"
        echo ""
        echo "   Mögliche Ursachen:"
        echo "   - Netzwerkprobleme (Server kann Let's Encrypt nicht erreichen)"
        echo "   - DuckDNS Token falsch"
        echo "   - Let's Encrypt Rate Limit erreicht (max 5 Zertifikate pro Woche)"
        exit 1
    fi
fi

echo "🚀 Starting FlipperCapital (Production with SSL)..."
docker compose -f docker-compose.prod.yml up --build -d frontend backend nginx

echo "✅ FlipperCapital is running at https://flippercapital.duckdns.org:54321"
