#!/bin/bash
# Example deployment script

echo "Starting deployment process..."

# Check if app directory exists
if [ ! -d "/opt/example-app" ]; then
    echo "Creating application directory..."
    mkdir -p /opt/example-app
fi

# Copy application files
echo "Deploying application files..."
# In real deployment, this would copy actual files

# Set permissions
echo "Setting permissions..."
chown -R app:app /opt/example-app
chmod -R 755 /opt/example-app

# Start service
echo "Starting service..."
systemctl enable example-app
systemctl start example-app

echo "Deployment completed successfully!"