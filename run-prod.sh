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
    echo "🔐 Hole Let's Encrypt Zertifikat (kann 1-2 Minuten dauern)..."
    docker compose -f docker-compose.prod.yml run --rm certbot

    if [ ! -f "$CERT_PATH" ]; then
        echo "❌ Zertifikat konnte nicht geholt werden!"
        echo "   Prüfe die Logs: docker compose -f docker-compose.prod.yml logs certbot"
        exit 1
    fi
    echo "✅ Zertifikat erfolgreich geholt!"
fi

echo "🚀 Starting FlipperCapital (Production with SSL)..."
docker compose -f docker-compose.prod.yml up --build -d frontend backend nginx

echo "✅ FlipperCapital is running at https://flippercapital.duckdns.org:54321"
