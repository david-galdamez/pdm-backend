package websockets

import (
	"log"
	"net/http"
	"pdm-backend/repositories"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var financeClients = make(map[uint]map[*websocket.Conn]bool)
var mu sync.RWMutex
var BroadcastMessages = make(chan repositories.BroadCastMessage, 100)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
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
