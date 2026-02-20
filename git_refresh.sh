#!/bin/bash

echo "🔄 FlipperCapital Git Refresh & Deploy"
echo "======================================="

# 1. Abgebrochenen Merge/Rebase aufräumen
if [ -f .git/MERGE_HEAD ]; then
    echo ""
    echo "⚠️  Unfinished merge detected, aborting..."
    git merge --abort
fi
if [ -d .git/rebase-merge ] || [ -d .git/rebase-apply ]; then
    echo ""
    echo "⚠️  Unfinished rebase detected, aborting..."
    git rebase --abort
fi

# 2. Alle lokalen Änderungen verwerfen (temp files, untracked etc.)
echo ""
echo "🧹 Resetting local changes..."
git checkout -- .
git clean -fd

# 3. Neueste Version ziehen
echo ""
echo "⬇️  Pulling latest changes..."
git pull --ff-only || {
    echo "⚠️  Fast-forward failed, hard resetting to origin/master..."
    git fetch origin
    git reset --hard origin/master
}

# 4. Berechtigungen setzen
echo ""
echo "🔐 Setting permissions..."
chmod 777 -R .

# 5. Container stoppen & alte Images entfernen
echo ""
echo "🛑 Stopping old containers..."
docker compose -f docker-compose.prod.yml down --rmi local 2>/dev/null

# 6. Build-Cache leeren (stellt sicher, dass Frontend/Backend komplett neu gebaut werden)
echo ""
echo "🧹 Clearing build cache..."
docker builder prune -f 2>/dev/null

# 7. Produktion starten
echo ""
echo "🚀 Starting production..."
./run-prod.sh
