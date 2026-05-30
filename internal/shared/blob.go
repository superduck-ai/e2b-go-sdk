package shared

import "bytes"

// Blob is the Go-side stand-in for the JS SDK's Blob payload family.
type Blob []byte

func (b Blob) Bytes() []byte {
	return []byte(b)
}

func (b Blob) Text() string {
	return string(b)
}

func (b Blob) String() string {
	return string(b)
}

func (b Blob) Reader() *bytes.Reader {
	return bytes.NewReader([]byte(b))
}
