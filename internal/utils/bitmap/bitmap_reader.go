package bitmap

// FIFOBuffer optimizes the allocated memory by copying the remaining buffer at the begining before each write
type FIFOBuffer struct {
	buffer []byte
	pos    int
}

func (rb *FIFOBuffer) Reset() {
	rb.buffer = rb.buffer[:0]
	rb.pos = 0
}

// Push allocates at least n bytes (if necessary) and returns the free byte array, ready to be written.
func (rb *FIFOBuffer) Push(n int) []byte {
	l := len(rb.buffer[rb.pos:])
	oldBuf := rb.buffer
	if cap(rb.buffer) < n+l {
		rb.buffer = make([]byte, n+l)
	}
	if rb.pos > 0 || &oldBuf != &rb.buffer {
		copy(rb.buffer, oldBuf[rb.pos:])
		rb.pos = 0
	}
	rb.buffer = rb.buffer[:n+l]
	return rb.buffer[l:]
}

func (rb *FIFOBuffer) Pop(n int) []byte {
	if n > rb.Len() {
		n = rb.Len()
	}
	ret := rb.buffer[rb.pos : rb.pos+n]
	rb.pos += n
	return ret
}

func (rb *FIFOBuffer) Len() int {
	return len(rb.buffer) - rb.pos
}
