package epcache

import "bytes"

// ByteView holds an immutable view of bytes,
// it should be used as a value type, not a pointer type.
type ByteView struct {
	b []byte
}

func (bv ByteView) Len() int {
	return len(bv.b)
}

// ByteSlice returns a copy of the data as a byte slice.
func (bv ByteView) ByteSlice() []byte {
	return cloneBytes(bv.b)
}

// Slice slices the view between from and to.
func (bv ByteView) Slice(from, to int) ByteView {
	return ByteView{bv.b[from:to]}
}

// Equal returns whether the bytes in bv are the same as the bytes in bv2.
func (bv ByteView) Equal(bv2 ByteView) bool {
	return bytes.Equal(bv.b, bv2.b)
}

// EqualBytes returns whether the bytes in bv are the same as the bytes b2.
func (bv ByteView) EqualBytes(b2 []byte) bool {
	return bytes.Equal(bv.b, b2)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
