#!/bin/bash

echo "🚀 Building NetControl Containers..."
echo "-----------------------------------"

# Create output directory if it doesn't exist
mkdir -p build

# Build for Windows (amd64)
# Build for Windows (amd64)
echo "📦 Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -o build/netcontrol-container.exe .
if [ $? -eq 0 ]; then
    echo "✅ Windows build successful: build/netcontrol-container.exe"
else
    echo "❌ Windows build failed"
    exit 1
fi

# Build for Linux (amd64)
echo "🐧 Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o build/netcontrol-container .
if [ $? -eq 0 ]; then
    echo "✅ Linux build successful: build/netcontrol-container"
else
    echo "❌ Linux build failed"
    exit 1
fi

# Copy assets
echo "📂 Copying assets..."
cp -r templates static build/
echo "[OK] Assets copied."

echo "-----------------------------------"
echo "✨ Build complete! Artifacts are in 'build/' directory."
