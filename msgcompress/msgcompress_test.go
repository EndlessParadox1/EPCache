package msgcompress

import (
	"fmt"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	mc := New(2 * time.Second)
	go func() {
		for range 100 {
			mc.In <- struct{}{}
			time.Sleep(100 * time.Millisecond)
		}
	}()
	go func() {
		i := 0
		for range mc.Out {
			fmt.Println(i)
			i++
		}
	}()
	time.Sleep(12 * time.Second)
}
