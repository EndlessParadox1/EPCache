package bloomfilter

import (
	"strconv"
	"testing"
)

func TestMightContain(t *testing.T) {
	bf := New(100*20, 13)
	for i := range 100 {
		if i%2 == 0 {
			bf.Add(strconv.Itoa(i))
		}
	}
	for i := range 100 {
		ok := bf.MightContain(strconv.Itoa(i))
		if ok && i%2 != 0 {
			t.Errorf("should return %v for odd numbers, but get %v", !ok, ok)
		} else if !ok && i%2 == 0 {
			t.Errorf("should return %v for even numbers, but get %v", !ok, ok)
		}
	}
}
