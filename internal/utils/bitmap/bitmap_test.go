package bitmap

import (
	"bytes"
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/airbusgeo/godal"
)

func testInternalBuffer(t *testing.T, rb *FIFOBuffer, value []byte, pos int) {
	if !reflect.DeepEqual(rb.buffer, value) {
		t.Errorf("Buffer : want %v, got %v", rb.buffer, value)
	}
	if rb.Len() != len(value)-pos {
		t.Errorf("Len() : want %d, got %d", len(value)-pos, rb.Len())
	}
	if rb.pos != pos {
		t.Errorf("Pos: want %d, got %d", pos, rb.pos)
	}
}

func TestRotatingBuffer(t *testing.T) {
	rb := FIFOBuffer{}

	copy(rb.Push(5), []byte{1, 2, 3, 4, 5})

	testInternalBuffer(t, &rb, []byte{1, 2, 3, 4, 5}, 0)

	res, exp := rb.Pop(2), []byte{1, 2}
	if !reflect.DeepEqual(res, exp) {
		t.Errorf("Read(2) : want %v, got %v", exp, res)
	}
	testInternalBuffer(t, &rb, []byte{1, 2, 3, 4, 5}, 2)

	copy(rb.Push(5), []byte{6, 7, 8, 9, 10})
	testInternalBuffer(t, &rb, []byte{3, 4, 5, 6, 7, 8, 9, 10}, 0)
	res, exp = rb.Pop(5), []byte{3, 4, 5, 6, 7}
	if !reflect.DeepEqual(res, exp) {
		t.Errorf("Read(2) : want %v, got %v", exp, res)
	}
	testInternalBuffer(t, &rb, []byte{3, 4, 5, 6, 7, 8, 9, 10}, 5)

	copy(rb.Push(5), []byte{11, 12, 13, 14, 15})
	testInternalBuffer(t, &rb, []byte{8, 9, 10, 11, 12, 13, 14, 15}, 0)
	res, exp = rb.Pop(10), []byte{8, 9, 10, 11, 12, 13, 14, 15}
	if !reflect.DeepEqual(res, exp) {
		t.Errorf("Read(2) : want %v, got %v", exp, res)
	}
	testInternalBuffer(t, &rb, []byte{8, 9, 10, 11, 12, 13, 14, 15}, 8)

	copy(rb.Push(4), []byte{16, 17, 18, 19})
	testInternalBuffer(t, &rb, []byte{16, 17, 18, 19}, 0)
	res, exp = rb.Pop(2), []byte{16, 17}
	if !reflect.DeepEqual(res, exp) {
		t.Errorf("Read(2) : want %v, got %v", exp, res)
	}
	testInternalBuffer(t, &rb, []byte{16, 17, 18, 19}, 2)
	res, exp = rb.Pop(1), []byte{18}
	if !reflect.DeepEqual(res, exp) {
		t.Errorf("Read(1) : want %v, got %v", exp, res)
	}
	testInternalBuffer(t, &rb, []byte{16, 17, 18, 19}, 3)
	res, exp = rb.Pop(3), []byte{19}
	if !reflect.DeepEqual(res, exp) {
		t.Errorf("Read(3) : want %v, got %v", exp, res)
	}
	testInternalBuffer(t, &rb, []byte{16, 17, 18, 19}, 4)
}

var _ = Describe("Test Streamable Bitmap", func() {
	var path string
	var chunkSize int
	var expected, returned []byte

	paths := []string{"../../image/test_data/image_cast0.tif"}

	var (
		itShouldReturnTheBitmap = func() {
			It("it should return the same bitmap", func() {
				Expect(returned).To(Equal(expected))
			})
		}
	)
	BeforeEach(func() {
		godal.RegisterAll()
	})

	JustBeforeEach(func() {
		ds, err := godal.Open(path)
		Expect(err).To(BeNil())
		bitmap, err := NewBitmapFromDataset(ds)
		Expect(err).To(BeNil())
		expected, err = bitmap.ReadAllBytes()
		Expect(err).To(BeNil())
		bitmap, err = NewStreamableBitmapFromDataset(ds)
		Expect(err).To(BeNil())
		var buffer bytes.Buffer
		for i := 0; i < bitmap.Chunks.Len(); i += chunkSize {
			b, err := bitmap.Chunks.Next(chunkSize)
			Expect(err).To(BeNil())
			buffer.Write(b)
		}
		returned = buffer.Bytes()
	})

	Context("with chunkSize=5", func() {
		BeforeEach(func() {
			path = paths[0]
			chunkSize = 5
		})
		itShouldReturnTheBitmap()
	})
	Context("with chunkSize=100000", func() {
		BeforeEach(func() {
			path = paths[0]
			chunkSize = 10000
		})
		itShouldReturnTheBitmap()
	})
	Context("with chunkSize=600000", func() {
		BeforeEach(func() {
			path = paths[0]
			chunkSize = 600000
		})
		itShouldReturnTheBitmap()
	})
})
