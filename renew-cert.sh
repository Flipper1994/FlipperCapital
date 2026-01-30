#!/bin/bash

# Zertifikat erneuern (alle 90 Tage nötig, am besten als Cronjob)
# Beispiel Cronjob: 0 3 1 * * /pfad/zu/FlipperCapital/renew-cert.sh

if [ ! -f ".env.prod" ]; then
    echo "❌ .env.prod nicht gefunden!"
    exit 1
fi

export $(cat .env.prod | grep -v '^#' | xargs)

echo "🔄 Erneuere Let's Encrypt Zertifikat..."

docker compose -f docker-compose.prod.yml run --rm certbot renew

# Nginx neu starten um neues Zertifikat zu laden
docker compose -f docker-compose.prod.yml restart nginx

echo "✅ Zertifikat erneuert und Nginx neu gestartet"
