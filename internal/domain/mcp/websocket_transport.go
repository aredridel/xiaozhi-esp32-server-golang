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
	// DefaultRequestTimeout default request timeout
	DefaultRequestTimeout = 30 * time.Second
	// DefaultCloseTimeout default close timeout
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
	// add close callback
	onCloseHandler func(reason string)

	// response channel management
	respChans    map[string]chan *transport.JSONRPCResponse
	respChansMux sync.RWMutex

	// message listen control
	readDone chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc

	// connection state
	closed    bool
	closedMux sync.RWMutex

	// WebSocket write lock, prevent concurrent write
	writeMux sync.Mutex

	// timeout config
	requestTimeout time.Duration
	closeTimeout   time.Duration
}

func (t *WebsocketTransport) Send(ctx context.Context, msg []byte) error {
	// check connection state
	t.closedMux.RLock()
	if t.closed {
		t.closedMux.RUnlock()
		return fmt.Errorf("connection is closed")
	}
	t.closedMux.RUnlock()

	// send message (use mutex lock protected write operation)
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
	// start message listen goroutine
	go wst.readMessages()

	return wst, nil
}

// implement Interface interface
func (t *WebsocketTransport) Start(ctx context.Context) error {
	return nil
}

// readMessages continuously listen WebSocket message
func (t *WebsocketTransport) readMessages() {
	defer close(t.readDone)

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// use Go language level timeout control
			_, message, err := t.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Errorf("WebSocket read error: %v", err)
				}

				// notify client layer when connection closes
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

			// process received message
			t.handleMessage(message)
		}
	}
}

// handleMessage process received message
func (t *WebsocketTransport) handleMessage(message []byte) {
	// try parse as JSON-RPC response
	var response transport.JSONRPCResponse
	if err := json.Unmarshal(message, &response); err == nil {
		// this is a JSON-RPC response
		t.handleResponse(&response)
		return
	}

	// try parse as JSON-RPC notification
	var notification mcp.JSONRPCNotification
	if err := json.Unmarshal(message, &notification); err == nil && notification.Method != "" {
		// this is a JSON-RPC notification
		t.handleNotification(&notification)
		return
	}

	// unrecognized message format
	log.Warnf("Received unrecognized message: %s", string(message))
}

// handleResponse process JSON-RPC response
func (t *WebsocketTransport) handleResponse(response *transport.JSONRPCResponse) {
	respByte, _ := json.Marshal(response)
	// convert ID to string as key
	idStr := response.ID.String()

	t.respChansMux.RLock()
	respChan, exists := t.respChans[idStr]
	t.respChansMux.RUnlock()

	if exists {
		// send response to corresponding channel
		select {
		case respChan <- response:
			// response already sent, cleanup channel
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

// handleNotification process JSON-RPC notification
func (t *WebsocketTransport) handleNotification(notification *mcp.JSONRPCNotification) {
	if t.notifyHandler != nil {
		t.notifyHandler(*notification)
	}
}

func (t *WebsocketTransport) SendRequest(ctx context.Context, request transport.JSONRPCRequest) (*transport.JSONRPCResponse, error) {
	// check connection state
	t.closedMux.RLock()
	if t.closed {
		t.closedMux.RUnlock()
		return nil, fmt.Errorf("connection is closed")
	}
	t.closedMux.RUnlock()

	// create response channel
	idStr := request.ID.String()

	respChan := make(chan *transport.JSONRPCResponse, 1)

	// register response channel
	t.respChansMux.Lock()
	t.respChans[idStr] = respChan
	t.respChansMux.Unlock()

	// send request (use mutex lock protected write operation)
	t.writeMux.Lock()
	err := t.conn.WriteJSON(request)
	t.writeMux.Unlock()
	if err != nil {
		// send failed, cleanup channel
		t.respChansMux.Lock()
		delete(t.respChans, idStr)
		t.respChansMux.Unlock()
		close(respChan)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// use Go language level timeout control to wait for response
	select {
	case response := <-respChan:
		return response, nil
	case <-ctx.Done():
		// context cancelled, cleanup channel
		t.respChansMux.Lock()
		delete(t.respChans, idStr)
		t.respChansMux.Unlock()
		close(respChan)
		return nil, ctx.Err()
	case <-time.After(t.requestTimeout):
		// Go language level timeout control
		t.respChansMux.Lock()
		delete(t.respChans, idStr)
		t.respChansMux.Unlock()
		close(respChan)
		return nil, fmt.Errorf("request timeout")
	}
}

func (t *WebsocketTransport) SendNotification(ctx context.Context, notification mcp.JSONRPCNotification) error {
	// check connection state
	t.closedMux.RLock()
	if t.closed {
		t.closedMux.RUnlock()
		return fmt.Errorf("connection is closed")
	}
	t.closedMux.RUnlock()

	// send notification message (use mutex lock protected write operation)
	t.writeMux.Lock()
	err := t.conn.WriteJSON(notification)
	t.writeMux.Unlock()
	return err
}

func (t *WebsocketTransport) SetNotificationHandler(handler func(notification mcp.JSONRPCNotification)) {
	t.notifyHandler = handler
}

// SetOnCloseHandler set connection close callback
func (t *WebsocketTransport) SetOnCloseHandler(handler func(reason string)) {
	t.onCloseHandler = handler
}

func (t *WebsocketTransport) Close() error {
	// mark connection already closed
	t.closedMux.Lock()
	t.closed = true
	t.closedMux.Unlock()

	// notify client layer connection will close
	if t.onCloseHandler != nil {
		t.onCloseHandler("manual_close")
	}

	// cancel context
	t.cancel()

	// wait for read goroutine to end
	select {
	case <-t.readDone:
	case <-time.After(t.closeTimeout):
		log.Warnf("Timeout waiting for read goroutine to finish")
	}

	// cleanup all response channels
	t.respChansMux.Lock()
	for idStr, respChan := range t.respChans {
		close(respChan)
		delete(t.respChans, idStr)
	}
	t.respChansMux.Unlock()

	// close WebSocket connection
	return t.conn.Close()
}

func (t *WebsocketTransport) GetSessionId() string {
	return t.conn.RemoteAddr().String()
}

// IsClosed check if connection is already closed
func (t *WebsocketTransport) IsClosed() bool {
	t.closedMux.RLock()
	defer t.closedMux.RUnlock()
	return t.closed
}

// GetActiveRequests get current active request count
func (t *WebsocketTransport) GetActiveRequests() int {
	t.respChansMux.RLock()
	defer t.respChansMux.RUnlock()
	return len(t.respChans)
}

// SetRequestTimeout set request timeout
func (t *WebsocketTransport) SetRequestTimeout(timeout time.Duration) {
	t.requestTimeout = timeout
}

// SetCloseTimeout set close timeout
func (t *WebsocketTransport) SetCloseTimeout(timeout time.Duration) {
	t.closeTimeout = timeout
}

// GetRequestTimeout get current request timeout
func (t *WebsocketTransport) GetRequestTimeout() time.Duration {
	return t.requestTimeout
}

// GetCloseTimeout get current close timeout
func (t *WebsocketTransport) GetCloseTimeout() time.Duration {
	return t.closeTimeout
}
