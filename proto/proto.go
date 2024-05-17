package proto

import (
	"errors"
)

const (
	// reply right now
	FlagPing = 1 << iota
	// no reply
	FlagKeepalive
)

var ErrFormat = errors.New("invalid format")

type Layer struct {
	Flags uint8
	// Len is the length of payload
	Len uint8
	// seq, encrypted at beginning of payload
	Payload []byte
}

func (l *Layer) Unmarshal(b []byte) error {
	if len(b) < 2 {
		return ErrFormat
	}
	l.Flags = b[0]
	l.Len = b[1]
	if len(b) != int(l.Len)+2 {
		return ErrFormat
	}
	l.Payload = make([]byte, l.Len)
	copy(l.Payload, b[2:])
	return nil
}

// Marshal also fill Len field
func (l *Layer) Marshal() ([]byte, error) {
	buf := make([]byte, 2+len(l.Payload))
	l.Len = uint8(len(l.Payload))
	buf[0] = l.Flags
	buf[1] = l.Len
	copy(buf[2:], l.Payload)
	return buf, nil
}
