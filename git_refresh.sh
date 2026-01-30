#!/bin/bash

echo "🔄 FlipperCapital Git Refresh & Deploy"
echo "======================================="

# 1. Lokale Änderungen stashen
echo ""
echo "📦 Stashing local changes..."
git stash push -m "wip"

# 2. Neueste Version ziehen
echo ""
echo "⬇️  Pulling latest changes..."
git pull

# 3. Stash wieder anwenden (falls vorhanden)
echo ""
echo "📦 Restoring local files..."
git stash pop 2>/dev/null || echo "   (keine gestashten Änderungen)"

# 4. Berechtigungen setzen
echo ""
echo "🔐 Setting permissions..."
chmod 777 -R .

# 5. Container stoppen falls laufend
echo ""
echo "🛑 Stopping old containers..."
docker compose -f docker-compose.prod.yml down 2>/dev/null

# 6. Produktion starten
echo ""
echo "🚀 Starting production..."
./run-prod.sh
