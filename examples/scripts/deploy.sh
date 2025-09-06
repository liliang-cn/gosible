#!/bin/bash
set -e

# Deployment script for application
echo "Starting deployment at $(date)"

# Check if running as correct user
if [ "$(whoami)" != "deploy" ]; then
    echo "This script must be run as 'deploy' user"
    exit 1
fi

# Navigate to application directory
cd /opt/myapp

# Pull latest code
echo "Pulling latest code..."
git pull origin main

# Install dependencies
echo "Installing dependencies..."
npm ci --production

# Run database migrations
echo "Running database migrations..."
npm run migrate

# Build assets
echo "Building assets..."
npm run build

# Restart application
echo "Restarting application..."
pm2 restart myapp --update-env

# Health check
echo "Performing health check..."
sleep 5
curl -f http://localhost:3000/health || exit 1

echo "Deployment completed successfully at $(date)"