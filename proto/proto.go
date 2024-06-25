package proto

import (
	"errors"
	"gvisor.dev/gvisor/pkg/binary"
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
	Len uint16
	// seq, encrypted at beginning of payload
	Payload []byte
}

func (l *Layer) Unmarshal(b []byte) error {
	if len(b) < 3 {
		return ErrFormat
	}
	l.Flags = b[0]
	l.Len = binary.LittleEndian.Uint16(b[1:3])
	if len(b) != int(l.Len)+3 {
		return ErrFormat
	}
	l.Payload = make([]byte, l.Len)
	copy(l.Payload, b[3:])
	return nil
}

// Marshal also fill Len field
func (l *Layer) Marshal() ([]byte, error) {
	buf := make([]byte, 3+len(l.Payload))
	l.Len = uint16(len(l.Payload))
	buf[0] = l.Flags
	binary.LittleEndian.PutUint16(buf[1:3], l.Len)
	copy(buf[3:], l.Payload)
	return buf, nil
}

type IdSeqPair struct {
	Id  uint16
	Seq uint16
}
