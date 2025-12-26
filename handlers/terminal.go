package handlers

import (
	"net/http"

	"netcontrol-containers/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var terminalUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func TerminalWS(c *gin.Context) {
	conn, err := terminalUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Get or create session ID
	sessionID := c.Query("session")
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Get terminal size
	rows := uint16(24)
	cols := uint16(80)

	// Create PTY session
	ptyManager := services.GetPTYManager()
	session, err := ptyManager.CreateSession(sessionID, rows, cols)
	if err != nil {
		conn.WriteJSON(gin.H{"error": err.Error()})
		return
	}

	// Send session ID to client
	conn.WriteJSON(gin.H{"session": sessionID})

	// Handle PTY output -> WebSocket
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := session.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					return
				}
			}
		}
	}()

	// Handle WebSocket input -> PTY
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		switch messageType {
		case websocket.TextMessage:
			// Handle control messages (resize, etc.)
			var msg map[string]interface{}
			if err := conn.ReadJSON(&msg); err == nil {
				if msg["type"] == "resize" {
					if r, ok := msg["rows"].(float64); ok {
						if c, ok := msg["cols"].(float64); ok {
							session.Resize(uint16(r), uint16(c))
						}
					}
				}
			}
		case websocket.BinaryMessage:
			// Write to PTY
			session.Write(data)
		}
	}

	// Cleanup
	session.Close()
}

func TerminalResize(c *gin.Context) {
	sessionID := c.Param("session")

	var req struct {
		Rows uint16 `json:"rows" binding:"required"`
		Cols uint16 `json:"cols" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	ptyManager := services.GetPTYManager()
	session := ptyManager.GetSession(sessionID)
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if err := session.Resize(req.Rows, req.Cols); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Terminal resized"})
}

func ListTerminalSessions(c *gin.Context) {
	ptyManager := services.GetPTYManager()
	sessions := ptyManager.ListSessions()
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

func CloseTerminalSession(c *gin.Context) {
	sessionID := c.Param("session")

	ptyManager := services.GetPTYManager()
	if err := ptyManager.CloseSession(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session closed"})
}
