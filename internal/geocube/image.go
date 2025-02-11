package geocube

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"unsafe"

	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/airbusgeo/godal"
)

type ChunkReader interface {
	Next(chunkSize int) ([]byte, error) // may return io.EOF
	Len() int
	Reset() error
}

// Bitmap decribes any image as a bitmap of bytes
type Bitmap struct {
	// Chunks is a reader of the image by chunks as []byte
	Chunks ChunkReader
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
	var err error
	xSize := ds.Structure().SizeX
	ySize := ds.Structure().SizeY
	bands := ds.Structure().NBands
	dtype := DTypeFromGDal(ds.Structure().DataType)

	if bands < 1 {
		return nil, fmt.Errorf("unsupported band count %d", bands)
	}

	r := image.Rect(0, 0, xSize, ySize)
	image := NewBitmapHeader(r, dtype, bands)
	image.Chunks, err = NewByteArrayFromDataset(ds, bands, xSize, ySize, dtype)

	return image, err
}

// NewBitmapFromBand creates a new bitmap from the band, copying the memory
func NewBitmapFromBand(band *godal.Band) (*Bitmap, error) {
	var err error
	xSize := band.Structure().SizeX
	ySize := band.Structure().SizeY
	dtype := DTypeFromGDal(band.Structure().DataType)

	r := image.Rect(0, 0, xSize, ySize)
	image := NewBitmapHeader(r, dtype, 1)
	image.Chunks, err = NewByteArrayFromBand(band, xSize, ySize, dtype)

	return image, err
}

// SizeX returns the x size of the image
func (i *Bitmap) SizeX() int {
	return i.Rect.Dx()
}

// SizeY returns the y size of the image
func (i *Bitmap) SizeY() int {
	return i.Rect.Dy()
}
func (i *Bitmap) Len() int {
	if i.Chunks != nil {
		return i.Chunks.Len()
	}
	return 0
}
func (i *Bitmap) ReadAllBytes() ([]byte, error) {
	if i.Chunks == nil {
		return nil, nil
	}
	bytes, err := i.Chunks.Next(i.Chunks.Len())
	if err != nil {
		return nil, err
	}
	i.Chunks.Reset()
	return bytes, nil
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

// IsValid returns true if <minValidPix> pixels != nodata are found in the image
func (i *Bitmap) IsValid(nodata float64, minValidPix int) bool {
	defer i.Chunks.Reset()
	minValidPix *= i.Bands
	chunkSize := max(minValidPix, 1024*1024)
	for {
		if minValidPix < 0 {
			return true
		}
		bytes, err := i.Chunks.Next(chunkSize)
		if err == io.EOF {
			return false
		}
		switch i.DType {
		case DTypeUINT8:
			minValidPix = decreaseValid(bytes, uint8(nodata), minValidPix)
		case DTypeUINT16:
			pix := utils.SliceByteToGeneric[uint16](bytes)
			minValidPix = decreaseValid(pix, uint16(nodata), minValidPix)
		case DTypeUINT32:
			pix := utils.SliceByteToGeneric[uint32](bytes)
			minValidPix = decreaseValid(pix, uint32(nodata), minValidPix)
		case DTypeINT8:
			pix := utils.SliceByteToGeneric[int8](bytes)
			minValidPix = decreaseValid(pix, int8(nodata), minValidPix)
		case DTypeINT16:
			pix := utils.SliceByteToGeneric[int16](bytes)
			minValidPix = decreaseValid(pix, int16(nodata), minValidPix)
		case DTypeINT32:
			pix := utils.SliceByteToGeneric[int32](bytes)
			minValidPix = decreaseValid(pix, int32(nodata), minValidPix)
		case DTypeFLOAT32:
			pix := utils.SliceByteToGeneric[float32](bytes)
			minValidPix = decreaseValid(pix, float32(nodata), minValidPix)
		case DTypeFLOAT64, DTypeCOMPLEX64:
			pix := utils.SliceByteToGeneric[float64](bytes)
			minValidPix = decreaseValid(pix, nodata, minValidPix)
		default:
			return false
		}
	}
}

func decreaseValid[T comparable](pix []T, nodata T, minValidPix int) int {
	for _, p := range pix {
		if p != nodata {
			minValidPix--
			if minValidPix < 0 {
				break
			}
		}
	}
	return minValidPix
}

// ByteArray is an array of Byte implementing ChunkReader
type ByteArray struct {
	Bytes  []byte
	offset int
}

func NewByteArrayFromDataset(ds *godal.Dataset, bands, xSize, ySize int, dtype DType) (*ByteArray, error) {
	if bands == 1 {
		return NewByteArrayFromBand(&ds.Bands()[0], xSize, ySize, dtype)
	}

	bytes := ByteArray{Bytes: make([]byte, bands*xSize*ySize*dtype.Size())}
	if err := ds.Read(0, 0, bytes.getPix(dtype), xSize, ySize); err != nil {
		return nil, fmt.Errorf("dataset.IO: %w", err)
	}
	return &bytes, nil
}

func NewByteArrayFromBand(band *godal.Band, xSize, ySize int, dtype DType) (*ByteArray, error) {
	bytes := ByteArray{Bytes: make([]byte, dtype.Size()*xSize*ySize)}
	if err := band.Read(0, 0, bytes.getPix(dtype), xSize, ySize); err != nil {
		return nil, fmt.Errorf("band.IO: %w", err)
	}
	return &bytes, nil
}

func (ba *ByteArray) Next(chunkSize int) ([]byte, error) {
	if ba.offset >= ba.Len() {
		return nil, io.EOF
	}
	o := ba.offset
	ba.offset = min(ba.offset+chunkSize, ba.Len())
	return ba.Bytes[o:ba.offset], nil
}

func (ba *ByteArray) Len() int {
	return len(ba.Bytes)
}

func (ba *ByteArray) Reset() error {
	ba.offset = 0
	return nil
}

func (ba ByteArray) getPix(dtype DType) interface{} {
	// Convert up to a slice of the right type
	var pix interface{}
	switch dtype {
	case DTypeUINT8:
		pix = ba.Bytes
	case DTypeUINT16:
		pix = utils.SliceByteToGeneric[uint16](ba.Bytes)
	case DTypeUINT32:
		pix = utils.SliceByteToGeneric[uint32](ba.Bytes)
	case DTypeINT8:
		pix = utils.SliceByteToGeneric[int8](ba.Bytes)
	case DTypeINT16:
		pix = utils.SliceByteToGeneric[int16](ba.Bytes)
	case DTypeINT32:
		pix = utils.SliceByteToGeneric[int32](ba.Bytes)
	case DTypeFLOAT32:
		pix = utils.SliceByteToGeneric[float32](ba.Bytes)
	case DTypeFLOAT64:
		pix = utils.SliceByteToGeneric[float64](ba.Bytes)
	case DTypeCOMPLEX64:
		pix = utils.SliceByteToGeneric[complex64](ba.Bytes)
	}
	return pix
}
