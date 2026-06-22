package api

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"agent-base/services/agent-service/internal/runtime"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type websocketEnvelope struct {
	Type       string            `json:"type"`
	Connection string            `json:"connection_id,omitempty"`
	Delivery   *runtime.Delivery `json:"delivery,omitempty"`
	Message    string            `json:"message,omitempty"`
}

type websocketAck struct {
	Type       string `json:"type"`
	DeliveryID string `json:"delivery_id"`
	Status     string `json:"status"`
}

func (h *Handler) connectionWebSocket(w http.ResponseWriter, r *http.Request, connectionID string) {
	if !isWebSocketUpgrade(r) {
		writeError(w, http.StatusBadRequest, "websocket upgrade required")
		return
	}
	if !h.requireConnectionToken(w, r, connectionID) {
		return
	}
	conn, rw, err := upgradeWebSocket(w, r)
	if err != nil {
		return
	}
	defer conn.Close()

	if err := writeTextFrame(conn, websocketEnvelope{Type: "gateway.connected", Connection: connectionID}); err != nil {
		return
	}

	for {
		delivery, err := h.store.NextDelivery(connectionID)
		if errors.Is(err, runtime.ErrNoTurnAvailable) {
			if err := writeTextFrame(conn, websocketEnvelope{Type: "gateway.idle", Connection: connectionID}); err != nil {
				return
			}
			return
		}
		if err != nil {
			_ = writeTextFrame(conn, websocketEnvelope{Type: "gateway.error", Connection: connectionID, Message: err.Error()})
			return
		}
		if err := writeTextFrame(conn, websocketEnvelope{Type: "delivery", Connection: connectionID, Delivery: &delivery}); err != nil {
			return
		}
		if err := readAckFrame(conn, rw, connectionID, delivery.ID, h.store); err != nil {
			_ = writeTextFrame(conn, websocketEnvelope{Type: "gateway.error", Connection: connectionID, Message: err.Error()})
			return
		}
	}
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") &&
		r.Header.Get("Sec-WebSocket-Key") != ""
}

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		writeError(w, http.StatusInternalServerError, "websocket hijacking unsupported")
		return nil, nil, errors.New("websocket hijacking unsupported")
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, err
	}
	accept := websocketAccept(r.Header.Get("Sec-WebSocket-Key"))
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n" +
		"\r\n"
	if _, err := rw.WriteString(response); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return conn, rw, nil
}

func websocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func writeTextFrame(w io.Writer, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	header := []byte{0x81}
	switch {
	case len(payload) < 126:
		header = append(header, byte(len(payload)))
	case len(payload) <= 65535:
		header = append(header, 126, byte(len(payload)>>8), byte(len(payload)))
	default:
		header = append(header, 127)
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(payload)))
		header = append(header, size[:]...)
	}
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err = w.Write(payload)
	return err
}

func readAckFrame(conn net.Conn, rw *bufio.ReadWriter, connectionID, deliveryID string, store runtime.Store) error {
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	payload, err := readTextFrame(rw.Reader)
	if err != nil {
		return err
	}
	var ack websocketAck
	if err := json.Unmarshal(payload, &ack); err != nil {
		return err
	}
	if ack.Type != "delivery.ack" {
		return errors.New("expected delivery.ack")
	}
	if ack.DeliveryID != deliveryID {
		return errors.New("delivery ack id mismatch")
	}
	_, err = store.AckDelivery(runtime.AckDeliveryInput{
		ConnectionID: connectionID,
		DeliveryID:   ack.DeliveryID,
		Status:       ack.Status,
	})
	return err
}

func readTextFrame(r *bufio.Reader) ([]byte, error) {
	first, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	opcode := first & 0x0f
	if opcode == 0x8 {
		return nil, io.EOF
	}
	if opcode != 0x1 {
		return nil, errors.New("expected text frame")
	}
	second, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	masked := second&0x80 != 0
	length := uint64(second & 0x7f)
	switch length {
	case 126:
		var size [2]byte
		if _, err := io.ReadFull(r, size[:]); err != nil {
			return nil, err
		}
		length = uint64(binary.BigEndian.Uint16(size[:]))
	case 127:
		var size [8]byte
		if _, err := io.ReadFull(r, size[:]); err != nil {
			return nil, err
		}
		length = binary.BigEndian.Uint64(size[:])
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(r, mask[:]); err != nil {
			return nil, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return payload, nil
}
