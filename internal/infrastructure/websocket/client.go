package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/whagency/core-common/logger"
)

type Sender struct {
	connectionUrl   url.URL
	connectionError error
	connection      *websocket.Conn
	connectionMutex sync.Mutex
}

// NewSender creates a new instance of Sender.
func NewSender(connectionUrl url.URL) *Sender {
	return &Sender{
		connectionUrl: connectionUrl,
	}
}

// Send it sends the websocket message.
func (s *Sender) Send(ctx context.Context, message any) error {
	logger.LogCtx(ctx).Debugf("[websocket][Sender:Send][message: %+v]", message)

	s.connectionMutex.Lock()
	defer s.connectionMutex.Unlock()

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("[websocket][Sender:Send][err: %w]", err)
	}

	conn, connErr := s.getConnection()
	if connErr != nil {
		return fmt.Errorf("[websocket][Sender:Send][getConnection][err: %w]", connErr)
	}

	if err = conn.WriteMessage(websocket.TextMessage, body); err != nil {
		_ = conn.Close()
		s.connection = nil

		conn, err = s.getConnection()
		if err != nil {
			return fmt.Errorf("[websocket][Sender:Send][reconnect][err: %w]", err)
		}

		if err = conn.WriteMessage(websocket.TextMessage, body); err != nil {
			return fmt.Errorf("[websocket][Sender:Send][writeAfterReconnect][err: %w]", err)
		}
	}

	return nil
}

// getConnection it returns the websocket connection.
func (s *Sender) getConnection() (*websocket.Conn, error) {
	if s.connection == nil {
		s.connection, _, s.connectionError = websocket.DefaultDialer.Dial(s.connectionUrl.String(), nil)
	}
	return s.connection, s.connectionError
}
