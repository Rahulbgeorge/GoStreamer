#!/bin/bash
# Exit on error
set -e

echo "============================================="
echo "Starting Production Build Pipeline"
echo "============================================="

# 1. Build Frontend using the Node.js production configuration
echo "Step 1: Building Frontend Assets (baking API base url)..."
cd frontend
npm install
npm run build:prod
cd ..

# 2. Build Go Backend Server for Production (Linux amd64 target)
echo ""
echo "Step 2: Building Go Backend Binary (myserver) for target GOOS=linux GOARCH=amd64..."
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o myserver ./cmd/server
cd ..

echo ""
echo "============================================="
echo "SUCCESS: Production Build completed!"
echo "============================================="
echo "Artifacts ready for deployment:"
echo "1. Frontend Built Directory: frontend/dist"
echo "2. Backend Server Binary: backend/myserver (built for Linux production environments)"
echo "============================================="
