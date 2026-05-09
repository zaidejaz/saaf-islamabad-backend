package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/zaidejaz/saaf-islamabad-backend/database"
	"github.com/zaidejaz/saaf-islamabad-backend/messaging"
	"github.com/zaidejaz/saaf-islamabad-backend/middleware"
	"github.com/zaidejaz/saaf-islamabad-backend/models"
)

// upgrader keeps defaults relaxed because the front-ends are first-party
// (web admin dashboard + mobile apps); CORS gating is handled separately at
// the gin layer for HTTP routes.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

const (
	pingPeriod = 30 * time.Second
	pongWait   = 60 * time.Second
	writeWait  = 10 * time.Second
)

// WSMessages handles real-time chat delivery for non-citizen accounts.
//
// We accept the JWT via either:
//   - the Authorization header (preferred, used by the mobile apps),
//   - a `?token=<jwt>` query string (browser WebSocket clients can't send
//     custom headers).
//
// The client purely receives events; sending happens through the regular
// REST endpoint POST /conversations/:id/messages. This keeps persistence,
// authorization and validation in one place and avoids re-implementing it
// over the socket.
func WSMessages(c *gin.Context) {
	tokenString := c.GetHeader("Authorization")
	if tokenString != "" {
		// strip "Bearer " prefix if present
		if len(tokenString) > 7 && (tokenString[:7] == "Bearer " || tokenString[:7] == "bearer ") {
			tokenString = tokenString[7:]
		}
	}
	if tokenString == "" {
		tokenString = c.Query("token")
	}
	if tokenString == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token required"})
		return
	}

	token, err := jwt.ParseWithClaims(tokenString, &middleware.Claims{}, func(t *jwt.Token) (interface{}, error) {
		return middleware.JWTSecret, nil
	})
	if err != nil || !token.Valid {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	claims, ok := token.Claims.(*middleware.Claims)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
		return
	}
	if claims.Role == models.RoleCitizen {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "citizens cannot use messaging"})
		return
	}

	// Confirm the user is still active.
	var user models.User
	if err := database.DB.First(&user, "id = ? AND is_active = true", claims.UserID).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user no longer active"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	events, cleanup := messaging.Default.Subscribe(claims.UserID)
	defer cleanup()

	// Reader goroutine: discards client frames except pong handling. We use
	// it to detect the client going away and to keep the read deadline
	// rolling.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()

	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	// Greet the client with a hello so they can confirm authenticated
	// connection succeeded.
	_ = conn.WriteJSON(gin.H{"type": "hello", "user_id": claims.UserID})

	for {
		select {
		case <-done:
			return
		case evt := <-events:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteJSON(evt); err != nil {
				return
			}
		case <-pingTicker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// _ blank-use to prevent unused imports if uuid stays just for typing
var _ = uuid.Nil
