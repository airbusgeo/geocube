package geocube

import (
	"encoding/binary"
	"fmt"
	"image"
	"unsafe"

	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/airbusgeo/godal"
)

// Bitmap decribes any image as a bitmap of bytes
type Bitmap struct {
	// Bytes is the []byte representation of the image
	Bytes []byte
	// Bands is the number of interlaced bands
	Bands int
	// Rect is the image's bounds.
	Rect image.Rectangle
	// Datatype of the pixel
	DType DType
	// For conversion between dtype and byte
	ByteOrder binary.ByteOrder
}

// NewBitmapHeader creates a new empty image (pixels are not allocated)
func NewBitmapHeader(r image.Rectangle, dtype DType, bands int) *Bitmap {
	return &Bitmap{
		Bands:     bands,
		Rect:      r,
		DType:     dtype,
		ByteOrder: nativeEndianness(),
	}
}

// NewBitmapFromDataset creates a new bitmap from the dataset, copying the memory
func NewBitmapFromDataset(ds *godal.Dataset) (*Bitmap, error) {
	xSize := ds.Structure().SizeX
	ySize := ds.Structure().SizeY
	bands := ds.Structure().NBands
	dtype := DTypeFromGDal(ds.Structure().DataType)

	if bands < 1 {
		return nil, fmt.Errorf("unsupported band count %d", bands)
	}

	r := image.Rect(0, 0, xSize, ySize)
	image := NewBitmapHeader(r, dtype, bands)
	image.Bytes = make([]byte, dtype.Size()*bands*r.Dx()*r.Dy())
	if bands == 1 {
		// Read one band
		band := ds.Bands()[0]
		if err := band.Read(0, 0, image.getPix(), xSize, ySize); err != nil {
			return nil, fmt.Errorf("band.IO: %w", err)
		}
	} else {
		bandmap := make([]int, bands)
		for i := 0; i < bands; i++ {
			bandmap[i] = i + 1
		}
		// Read severals bands
		if err := ds.Read(0, 0, image.getPix(), xSize, ySize); err != nil {
			return nil, fmt.Errorf("dataset.IO: %w", err)
		}
	}

	return image, nil
}

// SizeX returns the x size of the image
func (i *Bitmap) SizeX() int {
	return i.Rect.Dx()
}

// SizeY returns the y size of the image
func (i *Bitmap) SizeY() int {
	return i.Rect.Dy()
}

func (i *Bitmap) getPix() interface{} {
	// Convert up to a slice of the right type
	var pix interface{}
	switch i.DType {
	case DTypeUINT8:
		pix = i.Bytes
	case DTypeUINT16:
		pix = utils.SliceByteToUInt16(i.Bytes)
	case DTypeUINT32:
		pix = utils.SliceByteToUInt32(i.Bytes)
	case DTypeINT8:
		pix = utils.SliceByteToInt8(i.Bytes)
	case DTypeINT16:
		pix = utils.SliceByteToInt16(i.Bytes)
	case DTypeINT32:
		pix = utils.SliceByteToInt32(i.Bytes)
	case DTypeFLOAT32:
		pix = utils.SliceByteToFloat32(i.Bytes)
	case DTypeFLOAT64:
		pix = utils.SliceByteToFloat64(i.Bytes)
	case DTypeCOMPLEX64:
		pix = utils.SliceByteToComplex64(i.Bytes)
	}
	return pix
}

func nativeEndianness() binary.ByteOrder {
	var i int32 = 0x01020304
	u := unsafe.Pointer(&i)
	pb := (*byte)(u)
	b := *pb
	if b == 0x04 {
		return binary.LittleEndian
	}
	return binary.BigEndian
}
