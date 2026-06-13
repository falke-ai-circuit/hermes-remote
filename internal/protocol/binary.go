package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
)

const (
	MaxSmallPayloadSize = 1 << 20 // 1 MB - base64 for smaller
)

// EncodeBinary encodes a binary payload with a JSON header + raw bytes.
func EncodeBinary(env Envelope) ([]byte, error) {
	headerBytes, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("marshal header: %w", err)
	}
	headerLen := uint32(len(headerBytes))
	buf := make([]byte, 4+len(headerBytes))
	binary.BigEndian.PutUint32(buf[:4], headerLen)
	copy(buf[4:], headerBytes)
	return buf, nil
}

// DecodeBinaryHeader decodes the header portion of a binary frame, returning
// the JSON header length and the raw header bytes.
func DecodeBinaryHeader(raw []byte) (uint32, []byte, error) {
	if len(raw) < 4 {
		return 0, nil, fmt.Errorf("binary frame too short: %d bytes", len(raw))
	}
	headerLen := binary.BigEndian.Uint32(raw[:4])
	if len(raw) < int(4+headerLen) {
		return 0, nil, fmt.Errorf("binary frame truncated: expected %d header bytes, got %d", 4+headerLen, len(raw))
	}
	return headerLen, raw[4 : 4+headerLen], nil
}
