package ds

import (
	"sync/atomic"
)

type status struct {
	readCursor  uint64
	readLen     uint64
	writeCursor uint64
	writingLen  uint64
}

type RotatedQueue[T any] struct {
	buf    []T
	len    uint64
	cursor atomic.Value
}

// NewRotatedQueue len must be pow of 2
func NewRotatedQueue[T any](size int) *RotatedQueue[T] {
	q := &RotatedQueue[T]{
		buf:    make([]T, size),
		len:    uint64(size),
		cursor: atomic.Value{},
	}
	q.cursor.Store(status{0, 0, 0, 0})
	return q
}

// Push Must be called by only one writer
func (q *RotatedQueue[T]) Push(v T) {
Alloc:
	// alloc spare space
	old := q.cursor.Load().(status)
	var n status
	if old.readLen+old.writingLen == q.len {
		// full
		if old.readLen == 0 && old.writingLen == q.len {
			// impossible in single writer
			goto Alloc
		}
		n = status{
			// eat one element
			readCursor:  (old.readCursor + 1) % q.len,
			readLen:     old.readLen - 1,
			writeCursor: old.writeCursor,
			writingLen:  old.writingLen + 1,
		}
	} else {
		// not full
		n = status{
			readCursor:  old.readCursor,
			readLen:     old.readLen,
			writeCursor: old.writeCursor,
			writingLen:  old.writingLen + 1,
		}
	}
	if !q.cursor.CompareAndSwap(old, n) {
		goto Alloc
	}

	// fill data
	q.buf[n.writeCursor] = v

LengthPlus1:
	old = q.cursor.Load().(status)
	if !q.cursor.CompareAndSwap(old, status{
		readCursor:  old.readCursor,
		readLen:     old.readLen + 1,
		writeCursor: (old.writeCursor + 1) % q.len,
		writingLen:  old.writingLen - 1,
	}) {
		goto LengthPlus1
	}
}

func (q *RotatedQueue[T]) Pop() T {
Read:
	old := q.cursor.Load().(status)
	if old.readLen == 0 {
		// empty
		goto Read
	}

	data := q.buf[old.readCursor]
	if !q.cursor.CompareAndSwap(old, status{
		readCursor:  (old.readCursor + 1) % q.len,
		readLen:     old.readLen - 1,
		writeCursor: old.writeCursor,
		writingLen:  old.writingLen,
	}) {
		goto Read
	}

	return data
}
