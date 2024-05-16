package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	// reply right now
	FlagPing = 1 << iota
	// no reply
	FlagKeepalive
)

type Layer struct {
	Flags uint8
	// seq, encrypted at beginning of payload
	Payload []byte
}

func (l *Layer) Unmarshal(b []byte) error {
	if len(b) < 1 {
		return fmt.Errorf("short buffer")
	}
	err := binary.Read(bytes.NewReader(b), binary.BigEndian, &l.Flags)
	if err != nil {
		return fmt.Errorf("read flags fail: %v", err)
	}
	l.Payload = make([]byte, len(b)-1)
	copy(l.Payload, b[1:])
	return nil
}

func (l *Layer) Marshal() ([]byte, error) {
	buf := make([]byte, 1+len(l.Payload))
	buf[0] = l.Flags
	copy(buf[1:], l.Payload)
	return buf, nil
}
