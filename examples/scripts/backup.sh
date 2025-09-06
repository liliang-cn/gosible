#!/bin/bash
set -e

# Backup script
BACKUP_DIR="/var/backups/myapp"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/backup_${TIMESTAMP}.tar.gz"

# Create backup directory if it doesn't exist
mkdir -p "${BACKUP_DIR}"

# Backup database
echo "Backing up database..."
pg_dump myapp_db | gzip > "${BACKUP_DIR}/db_${TIMESTAMP}.sql.gz"

# Backup application files
echo "Backing up application files..."
tar -czf "${BACKUP_FILE}" \
    --exclude='node_modules' \
    --exclude='.git' \
    --exclude='logs' \
    -C /opt myapp

# Backup configuration
echo "Backing up configuration..."
tar -czf "${BACKUP_DIR}/config_${TIMESTAMP}.tar.gz" \
    -C /etc myapp

# Clean old backups (keep last 7 days)
echo "Cleaning old backups..."
find "${BACKUP_DIR}" -name "*.tar.gz" -mtime +7 -delete
find "${BACKUP_DIR}" -name "*.sql.gz" -mtime +7 -delete

echo "Backup completed: ${BACKUP_FILE}"