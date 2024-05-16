package ds

import (
	"fmt"
	"testing"
	"time"
)

func TestQueue(t *testing.T) {
	q := NewRotatedQueue[int](4)
	for i := 0; i < 10; i++ {
		q.Push(i)
	}
	for i := 0; i < 2; i++ {
		fmt.Println(q.buf, q.writeCursor, q.readCursor)
		fmt.Println(q.Pop())
	}
	for i := 10; i < 20; i++ {
		q.Push(i)
	}
	go func() {
		time.Sleep(time.Millisecond * 1000)
		q.Push(20)
		q.Push(21)
		q.Push(22)
	}()
	go func() {
		for {
			i := 1
			i++
		}
	}()
	for i := 0; i < 6; i++ {
		fmt.Println(q.buf, q.writeCursor, q.readCursor)
		fmt.Println(q.Pop())
	}
}
