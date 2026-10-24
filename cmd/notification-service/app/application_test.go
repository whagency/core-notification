package app

import (
	"errors"
	"net/http"
	"testing"

	"github.com/whagency/core-common/queue"
)

func TestNewApplicationInitializesQueueConnections(t *testing.T) {
	t.Parallel()

	app := NewApplication()
	if app == nil {
		t.Fatal("expected application")
	}
	if app.queueConnections == nil {
		t.Fatal("expected queue connections map")
	}
}

func TestApplicationAccessors(t *testing.T) {
	t.Parallel()

	httpServer := &http.Server{Addr: ":8000"}
	websocketServer := &http.Server{Addr: ":8100"}
	core := &Core{}
	queueManager := fakeQueueManager{}
	app := &Application{
		httpServer:      httpServer,
		websocketServer: websocketServer,
		core:            core,
		queueManager:    queueManager,
	}

	if app.HTTPServer() != httpServer {
		t.Fatal("unexpected HTTP server")
	}
	if app.WebsocketServer() != websocketServer {
		t.Fatal("unexpected websocket server")
	}
	if app.Core() != core {
		t.Fatal("unexpected core")
	}
	if app.QueueManager() == nil {
		t.Fatal("expected queue manager")
	}
	if app.DB() != nil {
		t.Fatal("expected nil DB")
	}
}

func TestServicesGRPCClose_ClosesInReverseOrderAndReturnsFirstError(t *testing.T) {
	t.Parallel()

	var order []string
	firstErr := errors.New("first close failure")
	services := servicesGRPC{}
	services.registerCloser(fakeCloser{name: "first", order: &order, err: firstErr})
	services.registerCloser(fakeCloser{name: "second", order: &order})
	services.registerCloser(fakeCloser{name: "third", order: &order, err: errors.New("third close failure")})

	err := services.Close()
	if err == nil || err.Error() != "third close failure" {
		t.Fatalf("error = %v; want third close failure", err)
	}
	if len(order) != 3 || order[0] != "third" || order[1] != "second" || order[2] != "first" {
		t.Fatalf("close order = %#v", order)
	}
}

func TestCoreCloseGRPC(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("close failed")
	core := &Core{}
	core.servicesGRPC.registerCloser(fakeCloser{err: closeErr})

	if err := core.CloseGRPC(); !errors.Is(err, closeErr) {
		t.Fatalf("CloseGRPC error = %v; want %v", err, closeErr)
	}
}

func TestCoreInitControllers(t *testing.T) {
	t.Parallel()

	core := &Core{
		notificationSenders: notificationSenders{
			Mailer:    nil,
			Websocket: nil,
		},
	}
	if err := core.initControllers(); err != nil {
		t.Fatalf("initControllers returned error: %v", err)
	}
	if core.WebsocketCtrl == nil {
		t.Fatal("expected websocket controller")
	}
	if core.NotificationCtrl == nil {
		t.Fatal("expected notification controller")
	}
}

type fakeCloser struct {
	name  string
	order *[]string
	err   error
}

func (f fakeCloser) Close() error {
	if f.order != nil {
		*f.order = append(*f.order, f.name)
	}
	return f.err
}

type fakeQueueManager struct{}

func (fakeQueueManager) AddConsumer(queue.Consumer) error   { return nil }
func (fakeQueueManager) AddPublisher(queue.Publisher) error { return nil }
func (fakeQueueManager) GetPublisher(queue.PublisherType) (queue.Publisher, error) {
	return nil, nil
}
func (fakeQueueManager) RunConsumers() error    { return nil }
func (fakeQueueManager) CloseConsumers() error  { return nil }
func (fakeQueueManager) ClosePublishers() error { return nil }
