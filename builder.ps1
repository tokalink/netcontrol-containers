Write-Host ">>> Building NetControl Containers..."
Write-Host "-----------------------------------"

# Create output directory if it doesn't exist
New-Item -ItemType Directory -Force -Path "build" | Out-Null

# Build for Windows (amd64)
Write-Host "[Windows] Building for Windows (amd64)..."
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o build/netcontrol-container.exe .
if ($LASTEXITCODE -eq 0) {
    Write-Host "[OK] Windows build successful: build/netcontrol-container.exe"
}
else {
    Write-Host "[ERROR] Windows build failed"
    exit 1
}

# Build for Linux (amd64)
Write-Host "[Linux] Building for Linux (amd64)..."
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o build/netcontrol-container .
if ($LASTEXITCODE -eq 0) {
    Write-Host "[OK] Linux build successful: build/netcontrol-container"
}
else {
    Write-Host "[ERROR] Linux build failed"
    exit 1
}

# Copy assets
Write-Host ">>> Copying assets..."
Copy-Item -Path "templates", "static" -Destination "build" -Recurse -Force
Write-Host "[OK] Assets copied."

Write-Host "-----------------------------------"
Write-Host "Build complete. Artifacts are in build/ directory."
