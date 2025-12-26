# NetControl Containers

## Overview
NetControl Containers is a premium VPS management panel focused on Docker and Kubernetes container orchestration. The backend is built with Go and SQLite, and the frontend features a modern, responsive design.

## Features Implemented
- **Authentication**: Secure login with JWT tokens. Default credentials: `admin` / `admin123`.
- **Dashboard**: System overview showing CPU, RAM, Disk usage, and general system info.
- **Docker Management**: List, start, stop, restart, remove containers. View logs and stats.
- **Kubernetes Management**: Manage Pods, Deployments, and Services. View logs and scale deployments.
- **Installer**: Built-in installer for Docker and Kubernetes with real-time progress.
- **File Explorer**: Browse file system, upload/download files, create/delete directories.
- **Web Terminal**: Fully functional xterm.js terminal connected via WebSocket.
- **Settings**: Change admin password and view panel info.

## How to Run

1.  **Build the Project**:
    ```bash
    go mod tidy
    go build -o server.exe .
    ```

    **Using Build Scripts (Recommended)**:
    We provide handy scripts to build for both Windows and Linux at once. Artifacts will be placed in the `build/` directory.

    PowerShell:
    ```powershell
    ./builder.ps1
    ```
    *Note: If you run into script execution errors, you may need to run `Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass` first.*

    Bash:
    ```bash
    chmod +x builder.sh
    ./builder.sh
    ```

    **Manual Build**:
    PowerShell:
    ```powershell
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    go build -o netcontrol-container .
    ```
    
    Bash (WSL/Linux):
    ```bash
    GOOS=linux GOARCH=amd64 go build -o netcontrol-container .
    ```

2.  **Start the Server**:
    ```bash
    ./netcontrol-container.exe
    # Or with custom port:
    # ./netcontrol-container.exe --port=8080
    ```
    The server will start on port **7002** (default).
    Access it at: [http://localhost:7002](http://localhost:7002)

3.  **Login**:
    - Username: `admin`
    - Password: `admin123`

## Implementation Details

### Backend Structure
- `main.go`: Application entry point and router setup.
- `services/`: Core logic for System, Docker, Kubernetes, Installer, and Terminal.
- `handlers/`: HTTP request handlers and WebSocket endpoints.
- `models/`: Database models (User, Settings).
- `middleware/`: Authentication middleware.
- `database/`: SQLite database initialization (using pure-Go driver).

### Frontend
- `templates/`: HTML templates for all pages.
- `static/css/style.css`: Premium dark theme with glassmorphism.
- `static/js/app.js`: Shared JavaScript logic and WebSocket manager.

## Troubleshooting
- **Build Errors**: If you encounter dependency issues, ensure `go.mod` has the correct `replace` directives (added during build fix).
- **Docker/K8s Connection**: Ensure Docker Desktop or Docker Engine is running locally. Kubernetes requires a configured `~/.kube/config`.
- **Database**: The `netcontrol.db` file is created in the same directory. To reset, simply delete this file and restart the server.
