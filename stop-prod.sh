#!/bin/bash
echo "🛑 Stopping FlipperCapital (Production)..."
docker compose -f docker-compose.prod.yml down
echo "✅ FlipperCapital stopped"
