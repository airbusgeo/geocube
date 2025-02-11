package utils

import (
	"bytes"
	"compress/flate"
	"context"
	"io"
)

// Buffer is a bytes.Buffer, but Reset creates a new buffer with same capacity instead of reuse the memory
type Buffer struct {
	bytes.Buffer
}

func (b *Buffer) Reset() {
	cap := b.Buffer.Cap()
	b.Buffer = bytes.Buffer{}
	b.Buffer.Grow(cap)
}

type Reader interface {
	Next(int) ([]byte, error) // Return io.EOF at the end
}

type ChunkElem struct {
	Chunk []byte
	Err   error
}

type CompressionStreamer struct {
	deflater  *flate.Writer
	chunkSize int
}

// NewCompressionStreamer creates a new deflater that compress a Reader by chunk and streams it on a channel.
// chunkSize is the size of the chunk read at each step. The size of the written chunk is from 0 to 2*chunkSize
//
// If level is in the range [-2, 9] then the error returned will be nil.
// Otherwise the error returned will be non-nil.
func NewCompressionStreamer(level int, chunkSize int) (*CompressionStreamer, error) {
	deflater, err := flate.NewWriter(nil, level)
	if err != nil {
		return nil, err
	}
	return &CompressionStreamer{
		deflater:  deflater,
		chunkSize: chunkSize,
	}, nil
}

func (cs *CompressionStreamer) Compress(ctx context.Context, r Reader) <-chan ChunkElem {
	var compressed Buffer
	cs.deflater.Reset(&compressed)

	compressedChunksChan := make(chan ChunkElem, 2)

	// Compression routine
	go func() {
		defer close(compressedChunksChan)
		defer cs.deflater.Close()
		var chunk []byte
		for err := error(nil); err != io.EOF; {
			var chunkElem ChunkElem
			// Compress the chunk
			chunkElem.Err = func() error {
				if chunk == nil { // Always true the first time or if reader returned an error
					return err // might be the error of r.Next() (see below)
				}
				if _, err := cs.deflater.Write(chunk); err != nil {
					return err
				}
				return cs.deflater.Flush()
			}()

			// Get the next chunk, to test if there is more.
			chunk, err = r.Next(cs.chunkSize)
			if err == io.EOF {
				// No more chunk => close the deflater (this step updates "compressed")
				if err := cs.deflater.Close(); err != nil {
					chunkElem.Err = err
				}
			}

			// Send to the channel
			if compressed.Len() >= cs.chunkSize || err == io.EOF {
				chunkElem.Chunk = compressed.Bytes()
				compressed.Reset()
			}
			if chunkElem.Chunk != nil || chunkElem.Err != nil { // Always false the first time (except when chunks is empty)
				select {
				case <-ctx.Done():
					return
				case compressedChunksChan <- chunkElem:
					chunkElem.Chunk = nil
				}
			}
		}
	}()

	return compressedChunksChan
}
