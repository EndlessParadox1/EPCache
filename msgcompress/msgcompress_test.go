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
	i := 0
	for range mc.Out {
		fmt.Print(i, " ")
		i++
	}
}
