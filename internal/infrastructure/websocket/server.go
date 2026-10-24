package websocket

import (
	"net/http"

	"notification-service/config"
	"notification-service/internal/domain"

	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
	"github.com/whagency/core-common/logger"
)

type WsHandler struct {
	Room *Room
}

// Room represents a single chat room
type Room struct {
	Forward  chan []byte
	Join     chan *Chatter
	Leave    chan *Chatter
	Chatters map[*Chatter]bool
	Topic    string
}

// removeChatter removes chatter from room and closes its outbound channel once.
func (r *Room) removeChatter(chatter *Chatter) bool {
	if _, ok := r.Chatters[chatter]; !ok {
		return false
	}

	delete(r.Chatters, chatter)
	close(chatter.Send)
	return true
}

// Chatter represents a single Chatter
type Chatter struct {
	Socket  *websocket.Conn
	Send    chan []byte
	Room    *Room
	Uid     uint64
	CanSend bool
}

// NewWsRoom creates a new chat room
func NewWsRoom(topic string) *Room {
	return &Room{
		Forward:  make(chan []byte),
		Join:     make(chan *Chatter),
		Leave:    make(chan *Chatter),
		Chatters: make(map[*Chatter]bool),
		Topic:    topic,
	}
}

// Configure the upgrader
var _upgrader = &websocket.Upgrader{
	ReadBufferSize:  config.Websocket().SocketBufferSize,
	WriteBufferSize: config.Websocket().SocketBufferSize,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ServeHTTP handles WebSocket requests from clients
func (h *WsHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	socket, err := _upgrader.Upgrade(w, req, nil)
	if err != nil {
		logger.Log().Errorf("[Room:ServeHTTP][err: %s]", err)
		return
	}

	uid, _ := req.Context().Value(domain.ContextWebsocketUserIDKey).(uint64)
	canSend, _ := req.Context().Value(domain.ContextWebsocketCanSendKey).(bool)

	chatter := &Chatter{
		Socket:  socket,
		Send:    make(chan []byte, config.Websocket().MessageBufferSize),
		Room:    h.Room,
		Uid:     uid,
		CanSend: canSend,
	}

	h.Room.Join <- chatter
	defer func() {
		h.Room.Leave <- chatter
	}()
	go chatter.write()
	chatter.read()
}

// Run initializes a chat room
func (r *Room) Run(fn func([]byte, *Room)) {
	logger.Log().Debugf("[Room:Run][msg: ws server is starting on port: %s, room %s]", config.Websocket().Port, r.Topic)
	for {
		select {
		case chatter := <-r.Join:
			logger.Log().Debugf("[Room:Run][msg: new userID %d in room: %s]", chatter.Uid, r.Topic)
			//chatter.socket.Close() // Block user
			r.Chatters[chatter] = true
		case chatter := <-r.Leave:
			logger.Log().Debugf("[Room:Run][msg: userID %d leaved room: %s]", chatter.Uid, r.Topic)
			r.removeChatter(chatter)
		case msg := <-r.Forward:
			fn(msg, r)
		}
	}
}

// MessageWsHandler receive all websocket messages and process them.
func MessageWsHandler(msg []byte, room *Room) {
	logger.Log().Debugf("[Room:MessageWsHandler][room: %s][message: %s]", room.Topic, string(msg))

	var toRemove []*Chatter

	var inputMessage domain.WsInputMessage
	if err := jsoniter.Unmarshal(msg, &inputMessage); err != nil {
		logger.Log().Errorf("[Room:MessageWsHandler][err: %s]", err)
		return
	}

	var receivers []uint64
	for chatter := range room.Chatters {
		if inputMessage.Uid == chatter.Uid || inputMessage.Uid == 0 {
			receivers = append(receivers, chatter.Uid)
			select {
			case chatter.Send <- msg:
			default:
				toRemove = append(toRemove, chatter)
			}
		}
	}

	for _, chatter := range toRemove {
		room.removeChatter(chatter)
	}

	logger.Log().Debugf("[Room:Receive][msg: message resent to %v]", receivers)
}

// read reads messages from the WebSocket connection and forwards them to the room
func (c *Chatter) read() {
	for {
		_, msg, err := c.Socket.ReadMessage()
		if err != nil {
			break
		}

		if !c.CanSend {
			logger.Log().Warnf("[Chatter:read][msg: unauthorized write attempt from UID %d]", c.Uid)
			continue
		}

		c.Room.Forward <- msg
	}

	if err := c.Socket.Close(); err != nil {
		logger.Log().Errorf("[Chatter:read][err: %s]", err)
	}
}

// write writes messages from the Send channel to the WebSocket connection
func (c *Chatter) write() {
	for msg := range c.Send {
		if err := c.Socket.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
}
