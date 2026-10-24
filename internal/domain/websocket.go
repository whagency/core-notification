package domain

import "context"

const (
	ContextWebsocketUserIDKey  string = "uid"
	ContextWebsocketCanSendKey string = "canSend"
)

type WsMessageType uint8

const (
	WsMessageTypeSuccessfulDeposit WsMessageType = iota + 1
	WsMessageTypeRequiredVerificationDeposit
	WsMessageTypeSuccessfulWithdrawal
	WsMessageTypeSuccessfulIncomingTransfer
)

var wsMessageTypeToString = map[WsMessageType]string{
	WsMessageTypeSuccessfulDeposit:           "sd",
	WsMessageTypeRequiredVerificationDeposit: "rvd",
	WsMessageTypeSuccessfulWithdrawal:        "sw",
	WsMessageTypeSuccessfulIncomingTransfer:  "st",
}

// String returns the string representation of the WsMessageType.
func (s WsMessageType) String() string {
	if val, ok := wsMessageTypeToString[s]; ok {
		return val
	}
	return "unknown"
}

type WsMessage struct {
	To     uint64
	Params TemplateData
	Kind   MessageKind
	Locale Locale
}

// WsInputMessage represents a input message from a client for detecting credentials
type WsInputMessage struct {
	Uid  uint64 `json:"u"`
	Type string `json:"t"`
}

// WsSuccessfulDepositMessage represents a websocket message for a successful deposit
type WsSuccessfulDepositMessage struct {
	Type      string  `json:"t"`
	UserID    uint64  `json:"u"`
	AssetCode string  `json:"as"`
	Amount    float64 `json:"am"`
}

// WsRequiredVerificationDepositMessage represents a websocket message for a required verification deposit
type WsRequiredVerificationDepositMessage struct {
	Type      string  `json:"t"`
	UserID    uint64  `json:"u"`
	AssetCode string  `json:"as"`
	Amount    float64 `json:"am"`
}

// WsSuccessfulWithdrawalMessage represents a websocket message for a successful withdrawal
type WsSuccessfulWithdrawalMessage struct {
	Type      string  `json:"t"`
	UserID    uint64  `json:"u"`
	AssetCode string  `json:"as"`
	Amount    float64 `json:"am"`
}

// WsSuccessfulIncomingTransfer represents a websocket message for a successful incoming transfer
type WsSuccessfulIncomingTransfer struct {
	Type      string  `json:"t"`
	UserID    uint64  `json:"u"`
	AssetCode string  `json:"as"`
	Amount    float64 `json:"am"`
}

// WebsocketController provides processing received and sending websocket messages.
type WebsocketController interface {
	Send(context.Context, WsMessage) error
}

// WebsocketManager defines the interface for sending WS messages.
type WebsocketManager interface {
	Send(context.Context, any) error
}
