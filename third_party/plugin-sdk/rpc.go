package pluginsdk

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const MaxMessageBytes = 16 << 20

var (
	ErrDisconnected           = errors.New("plugin disconnected; interrupted mutations must be reconciled, not replayed")
	ErrCancelUnacknowledged   = errors.New("plugin did not acknowledge operation cancellation")
	operationCancelAckTimeout = 30 * time.Second
)

type Handler func(context.Context, string, json.RawMessage) (any, error)
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string { return e.Message }

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	headerTerminated := false
	for n := 0; n < 32; n++ {
		rawLine, err := r.ReadSlice('\n')
		line := string(rawLine)
		if err != nil {
			return nil, err
		}
		if len(line) > 8192 {
			return nil, errors.New("oversized frame header")
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			headerTerminated = true
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, errors.New("invalid frame header")
		}
		if strings.EqualFold(parts[0], "Content-Length") {
			if length != -1 {
				return nil, errors.New("duplicate content length")
			}
			length, err = strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, err
			}
		}
	}
	if !headerTerminated {
		return nil, errors.New("unterminated frame header")
	}
	if length < 0 || length > MaxMessageBytes {
		return nil, errors.New("invalid frame length")
	}
	b := make([]byte, length)
	_, err := io.ReadFull(r, b)
	return b, err
}
func writeFrame(w io.Writer, m message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(b) > MaxMessageBytes {
		return errors.New("message too large")
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(b))
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

type Client struct {
	writer       io.Writer
	mu           sync.Mutex
	pending      map[uint64]chan message
	operations   map[string]context.CancelFunc
	next         atomic.Uint64
	done         chan struct{}
	once         sync.Once
	writeMu      sync.Mutex
	Notification func(string, json.RawMessage)
	handler      Handler
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	readErr      error
}

func NewClient(r io.Reader, w io.Writer) *Client { return NewClientWithNotifications(r, w, nil) }
func NewClientWithNotifications(r io.Reader, w io.Writer, n func(string, json.RawMessage)) *Client {
	return NewClientWithHandler(r, w, nil, n)
}

// NewClientWithHandler is a bidirectional peer. Incoming requests run concurrently
// so a handler may call the opposite peer without blocking the frame reader.
func NewClientWithHandler(r io.Reader, w io.Writer, h Handler, n func(string, json.RawMessage)) *Client {
	return newPeer(context.Background(), r, w, h, n)
}
func newPeer(ctx context.Context, r io.Reader, w io.Writer, h Handler, n func(string, json.RawMessage)) *Client {
	lifetime, cancel := context.WithCancel(ctx)
	c := &Client{writer: w, pending: map[uint64]chan message{}, operations: map[string]context.CancelFunc{}, done: make(chan struct{}), Notification: n, handler: h, ctx: lifetime, cancel: cancel}
	go c.read(r)
	go func() {
		select {
		case <-ctx.Done():
			_ = c.Close()
		case <-c.done:
		}
	}()
	return c
}
func (c *Client) Done() <-chan struct{} { return c.done }
func (c *Client) Close() error {
	c.once.Do(func() { c.cancel(); c.mu.Lock(); close(c.done); c.mu.Unlock() })
	return nil
}
func (c *Client) fail(err error) { c.mu.Lock(); c.readErr = err; c.mu.Unlock(); _ = c.Close() }
func (c *Client) read(r io.Reader) {
	br := bufio.NewReader(r)
	for {
		b, err := readFrame(br)
		if err != nil {
			c.fail(err)
			return
		}
		var m message
		if err = json.Unmarshal(b, &m); err != nil || m.JSONRPC != "2.0" {
			if err == nil {
				err = errors.New("invalid JSON-RPC version")
			}
			c.fail(err)
			return
		}
		if m.Method != "" {
			c.receiveRequest(m)
			continue
		}
		c.mu.Lock()
		ch := c.pending[m.ID]
		c.mu.Unlock()
		if ch != nil {
			select {
			case ch <- m:
			default:
			}
		}
	}
}

type peerKey struct{}

// CallCore calls a core service from a plugin handler on the existing transport.
// The host permits only its explicit core-service allowlist, not arbitrary daemon APIs.
func CallCore(ctx context.Context, method string, input, result any) error {
	peer, _ := ctx.Value(peerKey{}).(*Client)
	if peer == nil {
		return errors.New("no core peer in context")
	}
	return peer.Call(ctx, method, input, result)
}
func (c *Client) receiveRequest(m message) {
	c.mu.Lock()
	select {
	case <-c.done:
		c.mu.Unlock()
		return
	default:
	}
	c.wg.Add(1)
	c.mu.Unlock()
	dispatched := false
	defer func() {
		if !dispatched {
			c.wg.Done()
		}
	}()
	var op struct {
		OperationID string `json:"operationId"`
	}
	_ = json.Unmarshal(m.Params, &op)
	if m.Method == "cancel" && m.ID == 0 {
		c.mu.Lock()
		if cancel := c.operations[op.OperationID]; cancel != nil {
			cancel()
		}
		c.mu.Unlock()
		return
	}
	if m.ID == 0 && c.Notification != nil {
		c.Notification(m.Method, m.Params)
		return
	}
	ctx, cancel := context.WithCancel(c.ctx)
	ctx = context.WithValue(ctx, peerKey{}, c)
	ctx = context.WithValue(ctx, progressKey{}, func(p Progress) error { return c.Notify("progress", p) })
	if op.OperationID != "" {
		c.mu.Lock()
		if _, exists := c.operations[op.OperationID]; exists {
			c.mu.Unlock()
			cancel()
			if m.ID != 0 {
				_ = c.send(message{JSONRPC: "2.0", ID: m.ID, Error: &RPCError{-32602, "duplicate active operation ID"}})
			}
			return
		}
		c.operations[op.OperationID] = cancel
		c.mu.Unlock()
	}
	dispatched = true
	go func() {
		defer c.wg.Done()
		defer cancel()
		defer func() { c.mu.Lock(); delete(c.operations, op.OperationID); c.mu.Unlock() }()
		var value any
		var err error
		if c.handler == nil {
			err = &RPCError{-32601, "method not found"}
		} else {
			value, err = c.handler(ctx, m.Method, m.Params)
		}
		if m.ID == 0 {
			return
		}
		out := message{JSONRPC: "2.0", ID: m.ID}
		if err != nil {
			var typed *RPCError
			if errors.As(err, &typed) {
				out.Error = typed
			} else {
				out.Error = &RPCError{-32000, err.Error()}
			}
		} else {
			out.Result, err = json.Marshal(value)
			if err != nil {
				out.Error = &RPCError{-32603, err.Error()}
			}
		}
		if err = c.send(out); err != nil {
			c.fail(err)
		}
	}()
}
func (c *Client) send(m message) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return writeFrame(c.writer, m)
}
func (c *Client) Notify(method string, input any) error {
	p, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return c.send(message{JSONRPC: "2.0", Method: method, Params: p})
}
func (c *Client) Call(ctx context.Context, method string, input, result any) error {
	p, err := json.Marshal(input)
	if err != nil {
		return err
	}
	id := c.next.Add(1)
	ch := make(chan message, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, id); c.mu.Unlock() }()
	select {
	case <-c.done:
		return ErrDisconnected
	default:
	}
	if err := c.send(message{JSONRPC: "2.0", ID: id, Method: method, Params: p}); err != nil {
		return fmt.Errorf("%w: %v", ErrDisconnected, err)
	}
	select {
	case <-ctx.Done():
		var op struct {
			OperationID string `json:"operationId"`
		}
		_ = json.Unmarshal(p, &op)
		if op.OperationID == "" {
			return ctx.Err()
		}
		if err := c.Notify("cancel", map[string]string{"operationId": op.OperationID}); err != nil {
			return fmt.Errorf("%w: cancellation notification could not be sent", ErrCancelUnacknowledged)
		}
		timer := time.NewTimer(operationCancelAckTimeout)
		defer timer.Stop()
		select {
		case <-ch:
			return ctx.Err()
		case <-c.done:
			return ErrDisconnected
		case <-timer.C:
			return ErrCancelUnacknowledged
		}
	case <-c.done:
		return ErrDisconnected
	case m := <-ch:
		if m.Error != nil {
			return m.Error
		}
		if result != nil {
			return json.Unmarshal(m.Result, result)
		}
		return nil
	}
}

type progressKey struct{}

func ReportProgress(ctx context.Context, p Progress) error {
	fn, _ := ctx.Value(progressKey{}).(func(Progress) error)
	if fn == nil {
		return errors.New("no progress channel")
	}
	return fn(p)
}
func Serve(ctx context.Context, r io.Reader, w io.Writer, h Handler) error {
	c := newPeer(ctx, r, w, h, nil)
	<-c.Done()
	// Wait for handler cleanup after EOF/cancellation. Handlers must obey context.
	c.wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	err := c.readErr
	c.mu.Unlock()
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
