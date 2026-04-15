package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	log "xiaozhi-esp32-server-golang/logger"
)

const (
	// DefaultRequestTimeout defaultrequesttimeouttime
	DefaultRequestTimeout = 30 * time.Second
	// DefaultCloseTimeout defaultclosetimeouttime
	DefaultCloseTimeout = 5 * time.Second
)

/**
// Interface for the transport layer.
type Interface interface {
	// Start the connection. Start should only be called once.
	Start(ctx context.Context) error

	// SendRequest sends a json RPC request and returns the response synchronously.
	SendRequest(ctx context.Context, request JSONRPCRequest) (*JSONRPCResponse, error)

	// SendNotification sends a json RPC Notification to the server.
	SendNotification(ctx context.Context, notification mcp.JSONRPCNotification) error

	// SetNotificationHandler sets the handler for notifications.
	// Any notification before the handler is set will be discarded.
	SetNotificationHandler(handler func(notification mcp.JSONRPCNotification))

	// Close the connection.
	Close() error
}
*/

type WebsocketTransport struct {
	url  string
	conn *websocket.Conn

	notifyHandler func(notification mcp.JSONRPCNotification)
	// addclosecallback
	onCloseHandler func(reason string)

	// respondchannelmanage
	respChans    map[string]chan *transport.JSONRPCResponse
	respChansMux sync.RWMutex

	// messagelistencontrol
	readDone chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc

	// joinstate
	closed    bool
	closedMux sync.RWMutex

	// WebSocketwritelock，preventconcurrentwrite
	writeMux sync.Mutex

	// timeoutconfig
	requestTimeout time.Duration
	closeTimeout   time.Duration
}

func (t *WebsocketTransport) Send(ctx context.Context, msg []byte) error {
	// inspectjoinstate
	t.closedMux.RLock()
	if t.closed {
		t.closedMux.RUnlock()
		return fmt.Errorf("connection is closed")
	}
	t.closedMux.RUnlock()

	// sendmessage（usemutexlockprotectedwrite操as）
	t.writeMux.Lock()
	err := t.conn.WriteMessage(websocket.TextMessage, msg)
	t.writeMux.Unlock()
	return err
}

func NewWebsocketTransport(conn *websocket.Conn) (*WebsocketTransport, error) {
	ctx, cancel := context.WithCancel(context.Background())

	wst := &WebsocketTransport{
		conn:           conn,
		respChans:      make(map[string]chan *transport.JSONRPCResponse),
		readDone:       make(chan struct{}),
		ctx:            ctx,
		cancel:         cancel,
		requestTimeout: DefaultRequestTimeout,
		closeTimeout:   DefaultCloseTimeout,
	}
	// startmessagelistengoroutine
	go wst.readMessages()

	return wst, nil
}

// implement Interface interface
func (t *WebsocketTransport) Start(ctx context.Context) error {
	return nil
}

// readMessages 持continuelisten WebSocket message
func (t *WebsocketTransport) readMessages() {
	defer close(t.readDone)

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// use Go language级别oftimeoutcontrol
			_, message, err := t.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Errorf("WebSocket read error: %v", err)
				}

				// joinclosewhennotifyclientlayer
				if t.onCloseHandler != nil {
					reason := "connection_closed"
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						reason = "normal_closure"
					} else if websocket.IsUnexpectedCloseError(err) {
						reason = "unexpected_closure"
					}
					t.onCloseHandler(reason)
				}

				return
			}

			// processreceivetoofmessage
			t.handleMessage(message)
		}
	}
}

// handleMessage processreceivetoofmessage
func (t *WebsocketTransport) handleMessage(message []byte) {
	// tryparseis JSON-RPC respond
	var response transport.JSONRPCResponse
	if err := json.Unmarshal(message, &response); err == nil {
		// 这yesa JSON-RPC respond
		t.handleResponse(&response)
		return
	}

	// tryparseis JSON-RPC notify
	var notification mcp.JSONRPCNotification
	if err := json.Unmarshal(message, &notification); err == nil && notification.Method != "" {
		// 这yesa JSON-RPC notify
		t.handleNotification(&notification)
		return
	}

	// no法recognizeofmessageformat
	log.Warnf("Received unrecognized message: %s", string(message))
}

// handleResponse process JSON-RPC respond
func (t *WebsocketTransport) handleResponse(response *transport.JSONRPCResponse) {
	respByte, _ := json.Marshal(response)
	// will ID convertischarstringasiskey
	idStr := response.ID.String()

	t.respChansMux.RLock()
	respChan, exists := t.respChans[idStr]
	t.respChansMux.RUnlock()

	if exists {
		// sendrespondtocorrespondingchannel
		select {
		case respChan <- response:
			// respondalreadysend，cleanupchannel
			t.respChansMux.Lock()
			delete(t.respChans, idStr)
			t.respChansMux.Unlock()
			close(respChan)
		case <-time.After(t.requestTimeout):
			log.Warnf("websocket mcp handleResponse timeout for ID: %s, response: %+v", idStr, string(respByte))
		}
	} else {
		log.Warnf("No response channel found for ID: %s, response: %+v", idStr, string(respByte))
	}
}

// handleNotification process JSON-RPC notify
func (t *WebsocketTransport) handleNotification(notification *mcp.JSONRPCNotification) {
	if t.notifyHandler != nil {
		t.notifyHandler(*notification)
	}
}

func (t *WebsocketTransport) SendRequest(ctx context.Context, request transport.JSONRPCRequest) (*transport.JSONRPCResponse, error) {
	// inspectjoinstate
	t.closedMux.RLock()
	if t.closed {
		t.closedMux.RUnlock()
		return nil, fmt.Errorf("connection is closed")
	}
	t.closedMux.RUnlock()

	// createrespondchannel
	idStr := request.ID.String()

	respChan := make(chan *transport.JSONRPCResponse, 1)

	// registerrespondchannel
	t.respChansMux.Lock()
	t.respChans[idStr] = respChan
	t.respChansMux.Unlock()

	// sendrequest（usemutexlockprotectedwrite操as）
	t.writeMux.Lock()
	err := t.conn.WriteJSON(request)
	t.writeMux.Unlock()
	if err != nil {
		// sendfailed，cleanupchannel
		t.respChansMux.Lock()
		delete(t.respChans, idStr)
		t.respChansMux.Unlock()
		close(respChan)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// use Go language级别oftimeoutcontrolwaitrespond
	select {
	case response := <-respChan:
		return response, nil
	case <-ctx.Done():
		// contextcancel，cleanupchannel
		t.respChansMux.Lock()
		delete(t.respChans, idStr)
		t.respChansMux.Unlock()
		close(respChan)
		return nil, ctx.Err()
	case <-time.After(t.requestTimeout):
		// Go language级别oftimeoutcontrol
		t.respChansMux.Lock()
		delete(t.respChans, idStr)
		t.respChansMux.Unlock()
		close(respChan)
		return nil, fmt.Errorf("request timeout")
	}
}

func (t *WebsocketTransport) SendNotification(ctx context.Context, notification mcp.JSONRPCNotification) error {
	// inspectjoinstate
	t.closedMux.RLock()
	if t.closed {
		t.closedMux.RUnlock()
		return fmt.Errorf("connection is closed")
	}
	t.closedMux.RUnlock()

	// sendnotifymessage（usemutexlockprotectedwrite操as）
	t.writeMux.Lock()
	err := t.conn.WriteJSON(notification)
	t.writeMux.Unlock()
	return err
}

func (t *WebsocketTransport) SetNotificationHandler(handler func(notification mcp.JSONRPCNotification)) {
	t.notifyHandler = handler
}

// SetOnCloseHandler setjoinclosecallback
func (t *WebsocketTransport) SetOnCloseHandler(handler func(reason string)) {
	t.onCloseHandler = handler
}

func (t *WebsocketTransport) Close() error {
	// markjoinalreadyclose
	t.closedMux.Lock()
	t.closed = true
	t.closedMux.Unlock()

	// notifyclientlayerjoin即willclose
	if t.onCloseHandler != nil {
		t.onCloseHandler("manual_close")
	}

	// cancelcontext
	t.cancel()

	// waitreadgoroutineend
	select {
	case <-t.readDone:
	case <-time.After(t.closeTimeout):
		log.Warnf("Timeout waiting for read goroutine to finish")
	}

	// cleanupallrespondchannel
	t.respChansMux.Lock()
	for idStr, respChan := range t.respChans {
		close(respChan)
		delete(t.respChans, idStr)
	}
	t.respChansMux.Unlock()

	// close WebSocket join
	return t.conn.Close()
}

func (t *WebsocketTransport) GetSessionId() string {
	return t.conn.RemoteAddr().String()
}

// IsClosed inspectjoinwhetheralreadyclose
func (t *WebsocketTransport) IsClosed() bool {
	t.closedMux.RLock()
	defer t.closedMux.RUnlock()
	return t.closed
}

// GetActiveRequests get current活跃ofrequestcount
func (t *WebsocketTransport) GetActiveRequests() int {
	t.respChansMux.RLock()
	defer t.respChansMux.RUnlock()
	return len(t.respChans)
}

// SetRequestTimeout setrequesttimeouttime
func (t *WebsocketTransport) SetRequestTimeout(timeout time.Duration) {
	t.requestTimeout = timeout
}

// SetCloseTimeout setclosetimeouttime
func (t *WebsocketTransport) SetCloseTimeout(timeout time.Duration) {
	t.closeTimeout = timeout
}

// GetRequestTimeout get currentrequesttimeouttime
func (t *WebsocketTransport) GetRequestTimeout() time.Duration {
	return t.requestTimeout
}

// GetCloseTimeout get currentclosetimeouttime
func (t *WebsocketTransport) GetCloseTimeout() time.Duration {
	return t.closeTimeout
}
