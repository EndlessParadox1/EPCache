package epcache

import (
	"testing"
)

func TestByteViewSlice(t *testing.T) {
	v := ByteView{[]byte("abc")}
	v = v.Slice(1, 2)
	if v.String() != "b" {
		t.Errorf("got %q; want %q", v.String(), "b")
	}
}

func TestByteViewEqual(t *testing.T) {
	tests := []struct {
		a    []byte
		b    []byte
		want bool
	}{
		{[]byte("x"), []byte("x"), true},
		{[]byte("x"), []byte("y"), false},
		{[]byte("x"), []byte("yy"), false},
	}
	for i, tt := range tests {
		va := ByteView{tt.a}
		if got := va.EqualBytes(tt.b); got != tt.want {
			t.Errorf("%d. EqualBytes = %v; want %v", i, got, tt.want)
		}
		vb := ByteView{tt.b}
		if got := va.Equal(vb); got != tt.want {
			t.Errorf("%d. Equal = %v; want %v", i, got, tt.want)
		}
	}
}
