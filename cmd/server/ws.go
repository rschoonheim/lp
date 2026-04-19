package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// wsHub - Manages WebSocket client connections and broadcasts events.
type wsHub struct {
	mu      sync.RWMutex
	clients map[*wsConn]struct{}
}

// wsConn - A single WebSocket connection.
type wsConn struct {
	conn net.Conn
	mu   sync.Mutex
}

// wsEvent - JSON event pushed to WebSocket clients.
type wsEvent struct {
	Type     string         `json:"type"`
	Time     time.Time      `json:"time"`
	Pipeline string         `json:"pipeline,omitempty"`
	Step     string         `json:"step,omitempty"`
	Status   string         `json:"status,omitempty"`
	Error    string         `json:"error,omitempty"`
	Stdout   string         `json:"stdout,omitempty"`
	Stderr   string         `json:"stderr,omitempty"`
	Steps    int            `json:"steps,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

// newWSHub - Creates a new WebSocket hub.
func newWSHub() *wsHub {
	return &wsHub{clients: make(map[*wsConn]struct{})}
}

// handleUpgrade - HTTP handler that upgrades to a WebSocket connection.
func (h *wsHub) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "expected websocket upgrade", http.StatusBadRequest)
		return
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return
	}

	accept := wsAcceptKey(key)

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "server does not support hijacking", http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write upgrade response
	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	bufrw.WriteString(resp)
	bufrw.Flush()

	ws := &wsConn{conn: conn}
	h.addClient(ws)

	// Read loop (handles pings/close, discards data frames)
	go h.readPump(ws, bufrw.Reader)
}

// addClient - Registers a client connection.
func (h *wsHub) addClient(c *wsConn) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

// removeClient - Unregisters and closes a client connection.
func (h *wsHub) removeClient(c *wsConn) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	c.conn.Close()
}

// broadcast - Sends a wsEvent to all connected clients as JSON.
func (h *wsHub) broadcast(evt wsEvent) {
	if evt.Time.IsZero() {
		evt.Time = time.Now()
	}
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}

	h.mu.RLock()
	clients := make([]*wsConn, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		if err := c.writeText(data); err != nil {
			h.removeClient(c)
		}
	}
}

// readPump - Reads frames from the client, handles close/ping, discards data.
func (h *wsHub) readPump(c *wsConn, r *bufio.Reader) {
	defer h.removeClient(c)
	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		opcode, _, err := wsReadFrame(r)
		if err != nil {
			return
		}
		if opcode == 0x8 { // close
			return
		}
		if opcode == 0x9 { // ping → pong
			c.writeFrame(0xA, nil)
		}
	}
}

// writeText - Sends a text frame to the client.
func (c *wsConn) writeText(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return wsWriteFrame(c.conn, 0x1, data)
}

// writeFrame - Sends a frame with the given opcode.
func (c *wsConn) writeFrame(opcode byte, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return wsWriteFrame(c.conn, opcode, data)
}

// wsAcceptKey - Computes the Sec-WebSocket-Accept header value.
func wsAcceptKey(key string) string {
	const magic = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(key + magic))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// wsReadFrame - Reads a single WebSocket frame, returns opcode and payload.
func wsReadFrame(r *bufio.Reader) (opcode byte, payload []byte, err error) {
	b0, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	opcode = b0 & 0x0F

	b1, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	masked := b1&0x80 != 0
	length := uint64(b1 & 0x7F)

	switch length {
	case 126:
		var buf [2]byte
		if _, err := r.Read(buf[:]); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(buf[:]))
	case 127:
		var buf [8]byte
		if _, err := r.Read(buf[:]); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(buf[:])
	}

	var mask [4]byte
	if masked {
		if _, err := r.Read(mask[:]); err != nil {
			return 0, nil, err
		}
	}

	if length > 1<<20 { // 1MB max
		return 0, nil, fmt.Errorf("frame too large: %d", length)
	}

	payload = make([]byte, length)
	n := 0
	for n < int(length) {
		nn, err := r.Read(payload[n:])
		if err != nil {
			return 0, nil, err
		}
		n += nn
	}

	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}

	return opcode, payload, nil
}

// wsWriteFrame - Writes a WebSocket frame (server frames are unmasked).
func wsWriteFrame(conn net.Conn, opcode byte, data []byte) error {
	var header []byte
	length := len(data)

	header = append(header, 0x80|opcode) // FIN + opcode
	if length < 126 {
		header = append(header, byte(length))
	} else if length < 65536 {
		header = append(header, 126)
		header = append(header, byte(length>>8), byte(length))
	} else {
		header = append(header, 127)
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(length))
		header = append(header, buf[:]...)
	}

	if _, err := conn.Write(header); err != nil {
		return err
	}
	if len(data) > 0 {
		if _, err := conn.Write(data); err != nil {
			return err
		}
	}
	return nil
}

