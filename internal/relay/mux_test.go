package relay

import (
	"encoding/binary"
	"testing"
)

func TestParseFrame(t *testing.T) {
	// Valid frame: magic=0x7B, chanID=42, payload="hello"
	frame := make([]byte, 5+5)
	frame[0] = 0x7B
	binary.BigEndian.PutUint32(frame[1:5], 42)
	copy(frame[5:], []byte("hello"))

	magic, chanID, payload, err := ParseFrame(frame)
	if err != nil {
		t.Fatalf("ParseFrame error: %v", err)
	}
	if magic != 0x7B {
		t.Errorf("magic = 0x%02X, want 0x7B", magic)
	}
	if chanID != 42 {
		t.Errorf("chanID = %d, want 42", chanID)
	}
	if string(payload) != "hello" {
		t.Errorf("payload = %q, want 'hello'", string(payload))
	}
}

func TestParseFrame_TooShort(t *testing.T) {
	_, _, _, err := ParseFrame([]byte{0x01, 0x02, 0x03})
	if err == nil {
		t.Error("expected error for short frame, got nil")
	}
}

func TestParseFrame_EmptyPayload(t *testing.T) {
	frame := make([]byte, 5)
	frame[0] = 0x7B
	binary.BigEndian.PutUint32(frame[1:5], 0)

	magic, chanID, payload, err := ParseFrame(frame)
	if err != nil {
		t.Fatalf("ParseFrame error: %v", err)
	}
	if magic != 0x7B {
		t.Errorf("magic = 0x%02X, want 0x7B", magic)
	}
	if chanID != 0 {
		t.Errorf("chanID = %d, want 0", chanID)
	}
	if len(payload) != 0 {
		t.Errorf("payload length = %d, want 0", len(payload))
	}
}

func TestMakeFrame(t *testing.T) {
	payload := []byte("test data")
	frame := MakeFrame(0x42, 100, payload)

	if len(frame) != 5+len(payload) {
		t.Fatalf("frame length = %d, want %d", len(frame), 5+len(payload))
	}
	if frame[0] != 0x42 {
		t.Errorf("magic = 0x%02X, want 0x42", frame[0])
	}
	chanID := binary.BigEndian.Uint32(frame[1:5])
	if chanID != 100 {
		t.Errorf("chanID = %d, want 100", chanID)
	}
	if string(frame[5:]) != "test data" {
		t.Errorf("payload = %q, want 'test data'", string(frame[5:]))
	}
}

func TestMakeControlFrame(t *testing.T) {
	msg := ControlMessage{
		Type:      "channel_open",
		ChannelID: 5,
		RelayID:   "test-relay",
	}
	frame, err := MakeControlFrame(0x7B, msg)
	if err != nil {
		t.Fatalf("MakeControlFrame error: %v", err)
	}

	magic, chanID, payload, err := ParseFrame(frame)
	if err != nil {
		t.Fatalf("ParseFrame error: %v", err)
	}
	if magic != 0x7B {
		t.Errorf("magic = 0x%02X, want 0x7B", magic)
	}
	if chanID != 0 {
		t.Errorf("control chanID = %d, want 0", chanID)
	}
	if len(payload) == 0 {
		t.Error("control payload should not be empty")
	}
}

func TestChannelMap_Alloc(t *testing.T) {
	cm := NewChannelMap()
	ch1 := cm.Alloc(nil, nil)
	ch2 := cm.Alloc(nil, nil)
	ch3 := cm.Alloc(nil, nil)

	if ch1.ID == ch2.ID || ch2.ID == ch3.ID || ch1.ID == ch3.ID {
		t.Error("channel IDs should be unique")
	}
	if ch1.ID == 0 {
		t.Error("channel ID should never be 0 (reserved for control)")
	}
}

func TestChannelMap_Get(t *testing.T) {
	cm := NewChannelMap()
	ch := cm.Alloc(nil, nil)

	got := cm.Get(ch.ID)
	if got == nil {
		t.Error("Get returned nil for existing channel")
	}
	if got.ID != ch.ID {
		t.Errorf("Get returned ID %d, want %d", got.ID, ch.ID)
	}

	missing := cm.Get(999999)
	if missing != nil {
		t.Error("Get should return nil for non-existent channel")
	}
}

func TestChannelMap_Close(t *testing.T) {
	cm := NewChannelMap()
	ch := cm.Alloc(nil, nil)

	cm.Close(ch.ID)

	if cm.Get(ch.ID) != nil {
		t.Error("channel should be removed after Close")
	}
	if !ch.IsClosed() {
		t.Error("channel should be marked closed")
	}
}

func TestChannelMap_Count(t *testing.T) {
	cm := NewChannelMap()
	if cm.Count() != 0 {
		t.Errorf("Count = %d, want 0", cm.Count())
	}

	cm.Alloc(nil, nil)
	cm.Alloc(nil, nil)
	if cm.Count() != 2 {
		t.Errorf("Count = %d, want 2", cm.Count())
	}

	ch := cm.Alloc(nil, nil)
	if cm.Count() != 3 {
		t.Errorf("Count = %d, want 3", cm.Count())
	}

	cm.Close(ch.ID)
	if cm.Count() != 2 {
		t.Errorf("Count = %d, want 2 after close", cm.Count())
	}
}

func TestChannelMap_All(t *testing.T) {
	cm := NewChannelMap()
	cm.Alloc(nil, nil)
	cm.Alloc(nil, nil)
	cm.Alloc(nil, nil)

	all := cm.All()
	if len(all) != 3 {
		t.Errorf("All returned %d channels, want 3", len(all))
	}
}

func TestChannel_QueueMessage(t *testing.T) {
	cm := NewChannelMap()
	ch := cm.Alloc(nil, nil)

	// Fill buffer
	for i := 0; i < defaultBufferSize; i++ {
		if !ch.QueueMessage([]byte("x")) {
			t.Fatalf("QueueMessage returned false at message %d (buffer should have room)", i)
		}
	}

	// Buffer should be full
	if ch.QueueMessage([]byte("overflow")) {
		t.Error("QueueMessage should return false when buffer is full")
	}
}

func TestChannel_OnClose(t *testing.T) {
	called := false
	cm := NewChannelMap()
	ch := cm.Alloc(nil, func() { called = true })

	cm.Close(ch.ID)
	if !called {
		t.Error("OnClose callback should be called on Close")
	}
}