package websockets

import (
	"log"
	"net/http"
	"pdm-backend/internal/config"
	"pdm-backend/repositories"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var financeClients = make(map[uint]map[*websocket.Conn]bool)
var mu sync.RWMutex
var BroadcastMessages = make(chan repositories.BroadCastMessage, 100)

// allowedOrigins is the ALLOWED_ORIGINS allowlist as a set, built on first use.
var allowedOrigins = sync.OnceValue(func() map[string]bool {
	set := make(map[string]bool)

	for _, origin := range config.Get().ALLOWED_ORIGINS {
		set[origin] = true
	}

	return set
})

var upgrader = websocket.Upgrader{
	// The handshake is a plain GET that CORS never guards, so the origin has to
	// be checked here. Native mobile clients send no Origin header and are not
	// subject to browser same-origin rules, so an absent Origin is let through.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}

		if !allowedOrigins()[origin] {
			log.Println("Rejected websocket connection from origin: ", origin)
			return false
		}

		return true
	},
}

func HandleConnection(c *gin.Context) {

	idParam := c.Param("id")

	idUint, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		log.Println("Could not parse the finance id")
		return
	}
	financeId := uint(idUint)

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Could not upgrade the connection to a websocket")
		return
	}

	defer ws.Close()

	mu.Lock()
	if financeClients[financeId] == nil {
		financeClients[financeId] = make(map[*websocket.Conn]bool)
	}

	financeClients[financeId][ws] = true
	mu.Unlock()

	for {
		var msg interface{}
		err := ws.ReadJSON(&msg)
		if err != nil {
			mu.Lock()
			delete(financeClients[financeId], ws)

			if len(financeClients[financeId]) == 0 {
				delete(financeClients, financeId)
			}
			mu.Unlock()
			break
		}
	}
}

func HandleBroadCast() {
	for {
		msg := <-BroadcastMessages

		mu.RLock()
		clients := financeClients[msg.FinanceID]
		mu.RUnlock()
		for client := range clients {
			go func(c *websocket.Conn) {
				if err := c.WriteJSON(msg.EventInfo); err != nil {
					log.Println("Error sending message")
					c.Close()
					mu.Lock()
					delete(clients, c)
					mu.Unlock()
				}
			}(client)
		}
	}
}
