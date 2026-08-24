package websockets

import (
	"log"
	"net/http"
	"pdm-backend/internal/config"
	"pdm-backend/repositories"
	"pdm-backend/services"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// sendBuffer is how many broadcasts may queue up for a single connection before
// it is treated as too slow and dropped.
const sendBuffer = 16

// client is one open connection. Every write to conn goes through send so that a
// single goroutine (writePump) owns the connection: gorilla/websocket allows
// only one concurrent writer, and concurrent WriteJSON calls panic.
type client struct {
	conn      *websocket.Conn
	userId    uint
	financeId uint
	send      chan any
	done      chan struct{}
	closeOnce sync.Once
}

// close is safe to call more than once and from any goroutine.
func (cl *client) close() {
	cl.closeOnce.Do(func() {
		close(cl.done)
		cl.conn.Close()
	})
}

// financeClients holds the live connections per finance. A user may have several
// (phone and tablet, or a reconnect racing a dead socket), so connections are
// keyed by client, never by user id.
var financeClients = make(map[uint]map[*client]struct{})
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

func addClient(cl *client) {
	mu.Lock()
	defer mu.Unlock()

	if financeClients[cl.financeId] == nil {
		financeClients[cl.financeId] = make(map[*client]struct{})
	}

	financeClients[cl.financeId][cl] = struct{}{}
}

// removeClient deregisters a single connection and closes it.
func removeClient(cl *client) {
	mu.Lock()

	if clients := financeClients[cl.financeId]; clients != nil {
		delete(clients, cl)

		if len(clients) == 0 {
			delete(financeClients, cl.financeId)
		}
	}
	mu.Unlock()

	cl.close()
}

// DisconnectUser drops every connection a user holds on a finance. Call it when
// a membership is revoked so the socket does not outlive the access it was
// granted under.
func DisconnectUser(financeId, userId uint) {
	mu.Lock()
	var dropped []*client

	if clients := financeClients[financeId]; clients != nil {
		for cl := range clients {
			if cl.userId == userId {
				dropped = append(dropped, cl)
				delete(clients, cl)
			}
		}

		if len(clients) == 0 {
			delete(financeClients, financeId)
		}
	}
	mu.Unlock()

	for _, cl := range dropped {
		cl.close()
	}
}

type SharedFinanceWS struct {
	FinanceRepo *repositories.SharedFinanceRepository
}

func NewSharedFinanceWS(financeRepo *repositories.SharedFinanceRepository) *SharedFinanceWS {
	return &SharedFinanceWS{
		FinanceRepo: financeRepo,
	}
}

func (sfws *SharedFinanceWS) HandleConnection(c *gin.Context) {

	idParam := c.Param("id")

	idUint, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid finance id"})
		return
	}
	financeId := uint(idUint)

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	financeExists := sfws.FinanceRepo.DoesSharedFinanceExists(financeId)
	if !financeExists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Shared finance doesn't exist or you don't have access to it"})
		return
	}

	userBelongs := sfws.FinanceRepo.UserBelongsToSharedFinance(userClaims.UserID, financeId)
	if !userBelongs {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "You don't have access to this shared finance"})
		return
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Could not upgrade the connection to a websocket")
		return
	}

	cl := &client{
		conn:      ws,
		userId:    userClaims.UserID,
		financeId: financeId,
		send:      make(chan any, sendBuffer),
		done:      make(chan struct{}),
	}

	addClient(cl)
	defer removeClient(cl)

	go cl.writePump()

	for {
		var msg any
		if err := ws.ReadJSON(&msg); err != nil {
			break
		}
	}
}

// writePump is the only goroutine that writes to the connection.
func (cl *client) writePump() {
	for {
		select {
		case msg := <-cl.send:
			if err := cl.conn.WriteJSON(msg); err != nil {
				log.Println("Error sending message: ", err)
				removeClient(cl)
				return
			}
		case <-cl.done:
			return
		}
	}
}

func (sfws *SharedFinanceWS) HandleBroadCast() {
	for msg := range BroadcastMessages {
		sfws.dispatch(msg)
	}
}

// dispatch fans one message out to the finance's connections, skipping any whose
// membership has been revoked since they connected. It runs on the broadcast
// goroutine so messages keep their order.
func (sfws *SharedFinanceWS) dispatch(msg repositories.BroadCastMessage) {
	mu.RLock()
	clients := make([]*client, 0, len(financeClients[msg.FinanceID]))

	for cl := range financeClients[msg.FinanceID] {
		clients = append(clients, cl)
	}
	mu.RUnlock()

	// One membership lookup per user, not per connection.
	membership := make(map[uint]bool)

	for _, cl := range clients {
		belongs, checked := membership[cl.userId]
		if !checked {
			belongs = sfws.FinanceRepo.UserBelongsToSharedFinance(cl.userId, msg.FinanceID)
			membership[cl.userId] = belongs
		}

		if !belongs {
			removeClient(cl)
			continue
		}

		select {
		case cl.send <- msg.EventInfo:
		default:
			log.Println("Dropping websocket client that is too slow, user: ", cl.userId)
			removeClient(cl)
		}
	}
}
