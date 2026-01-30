#!/bin/bash
echo "🚀 Starting FlipperCapital (Production with SSL)..."
docker compose -f docker-compose.prod.yml up --build -d
echo "✅ FlipperCapital is running at https://flippercapital.duckdns.org:54321"
