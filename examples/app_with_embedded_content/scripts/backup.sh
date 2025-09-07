#!/bin/bash

# Backup script for application deployment
# This script creates a backup of the current deployment before updating

APP_NAME="$1"
BACKUP_DIR="/var/backups/${APP_NAME}"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create backup directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

echo "Creating pre-deployment backup for $APP_NAME at $TIMESTAMP"

# Backup application code if it exists
if [ -d "/opt/$APP_NAME" ]; then
    echo "Backing up application code..."
    tar -czf "$BACKUP_DIR/app_${TIMESTAMP}.tar.gz" -C /opt "$APP_NAME"
fi

# Backup configuration if it exists  
if [ -d "/etc/$APP_NAME" ]; then
    echo "Backing up configuration..."
    tar -czf "$BACKUP_DIR/config_${TIMESTAMP}.tar.gz" -C /etc "$APP_NAME"
fi

# Backup logs if they exist
if [ -d "/var/log/$APP_NAME" ]; then
    echo "Backing up logs..."
    tar -czf "$BACKUP_DIR/logs_${TIMESTAMP}.tar.gz" -C /var/log "$APP_NAME"
fi

# Create backup manifest
cat > "$BACKUP_DIR/backup_${TIMESTAMP}.manifest" << EOF
Backup created: $(date)
Application: $APP_NAME
Files:
$(ls -la "$BACKUP_DIR"/*${TIMESTAMP}* 2>/dev/null || echo "No backup files created")
EOF

echo "Backup completed successfully"
echo "Backup location: $BACKUP_DIR"
echo "Backup timestamp: $TIMESTAMP"