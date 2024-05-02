package epcache

import (
	"bytes"
	"io"
)

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

// String returns the data as a string, making a copy if necessary.
func (bv ByteView) String() string {
	return string(bv.b)
}

// At returns the byte at index i.
func (bv ByteView) At(i int) byte {
	return bv.b[i]
}

// Slice slices the view between the provided from and to indices.
func (bv ByteView) Slice(from, to int) ByteView {
	return ByteView{b: bv.b[from:to]}
}

// Copy copies b into dest and returns the number of bytes copied.
func (bv ByteView) Copy(dest []byte) int {
	return copy(dest, bv.b)
}

// Equal returns whether the bytes in b are the same as the bytes in b2.
func (bv ByteView) Equal(b2 ByteView) bool {
	return bv.EqualBytes(b2.b)
}

// EqualBytes returns whether the bytes in b are the same as the bytes in b2.
func (bv ByteView) EqualBytes(b2 []byte) bool {
	return bytes.Equal(bv.b, b2)
}

// Reader returns an io.ReadSeeker for the bytes in v.
func (bv ByteView) Reader() io.ReadSeeker {
	return bytes.NewReader(bv.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
