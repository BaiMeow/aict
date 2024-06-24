package ds

import (
	"context"
	"fmt"
	"testing"
)

func TestQueue(t *testing.T) {
	q := NewRotatedQueue[int](4)
	for i := 0; i < 10; i++ {
		q.Push(i)
	}
	for i := 0; i < 2; i++ {
		fmt.Println(q.buf, q.cursor)
		fmt.Println(q.Pop())
	}
	for i := 10; i < 20; i++ {
		q.Push(i)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		i := 20
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			q.Push(i)
			i++
		}
	}()
	for i := 0; i < 600; i++ {
		fmt.Println(q.buf, q.cursor)
		fmt.Println(q.Pop())
	}
	cancel()
}
