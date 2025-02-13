package bitmap

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"math"
	"runtime"
	"sync"
	"unsafe"

	"github.com/airbusgeo/geocube/internal/utils"
	"github.com/airbusgeo/godal"
)

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

// NewStreamableBitmapFromDataset creates a new bitmap from the dataset, takes the ownership of the dataset and streams the memory
func NewStreamableBitmapFromDataset(ds *godal.Dataset) (*Bitmap, error) {
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

	dr := &datasetReader{ds: ds}
	runtime.SetFinalizer(dr, _datasetReader_Close)

	image.Chunks = &ImageReader{image: dr, xSize: xSize, ySize: ySize, bands: bands, dtype: dtype}
	return image, err
}

// NewStreamableBitmapFromBand creates a new bitmap from the band and streams the memory
func NewStreamableBitmapFromBand(band *godal.Band) (*Bitmap, error) {
	xSize := band.Structure().SizeX
	ySize := band.Structure().SizeY
	bands := 1
	dtype := DTypeFromGDal(band.Structure().DataType)

	if bands < 1 {
		return nil, fmt.Errorf("unsupported band count %d", bands)
	}

	r := image.Rect(0, 0, xSize, ySize)
	image := NewBitmapHeader(r, dtype, bands)
	image.Chunks = &ImageReader{image: &bandReader{band: band}, xSize: xSize, ySize: ySize, bands: 1, dtype: dtype}

	return image, nil
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
	defer i.Chunks.Restart()
	if i.Chunks == nil {
		return nil, nil
	}
	bytes, err := i.Chunks.Next(i.Chunks.Len())
	if err != nil {
		return nil, err
	}
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
	defer i.Chunks.Restart()
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

// //////////////////////////////////////////////////////////////////////
// ChunkReader to read bytes by chunk
type ChunkReader interface {
	Next(chunkSize int) ([]byte, error) // Return the next <chunkSize> bytes. Returns err=io.EOF if there is no chunk anymore
	Len() int                           // Return the total number of bytes
	Restart() error                     // Restart at the first chunk
}

// /////////////////////////////////////////////////////////////////////
// imageScanner to read <lineCount> lines of an image, starting with <line>
type imageScanner interface {
	ReadLines(buffer any, line, lineCount int) error
}

// datasetReader owns a dataset and implements imageScanner for a dataset
type datasetReader struct {
	ds         *godal.Dataset
	closeMutex sync.Mutex
}

func _datasetReader_Close(ba *datasetReader) {
	ba.closeMutex.Lock()
	if ba.ds != nil {
		ba.ds.Close()
		ba.ds = nil
	}
	ba.closeMutex.Unlock()
}

func (dr *datasetReader) ReadLines(buffer any, line, lineCount int) error {
	return dr.ds.Read(0, line, buffer, dr.ds.Structure().SizeX, lineCount)
}

// bandReader implements imageScanner for a dataset
type bandReader struct {
	band *godal.Band
}

func (br *bandReader) ReadLines(buffer any, line, lineCount int) error {
	return br.band.Read(0, line, buffer, br.band.Structure().SizeX, lineCount)
}

// /////////////////////////////////////////////////////////////////////
// ImageReader implements ChunkReader for an imageScanner
type ImageReader struct {
	image               imageScanner
	buffer              FIFOBuffer
	xSize, ySize, bands int
	dtype               DType
	yCur                int
}

func (dr *ImageReader) Next(chunkSize int) ([]byte, error) {
	oldBufSize := dr.buffer.Len()
	if dr.yCur >= dr.ySize {
		if oldBufSize == 0 {
			return nil, io.EOF
		}
		return dr.buffer.Pop(chunkSize), nil
	}
	stride := dr.bands * dr.xSize * dr.dtype.Size()

	lineCount := int(math.Ceil(float64(chunkSize-oldBufSize) / float64(stride)))
	if dr.yCur+lineCount > dr.ySize {
		lineCount = dr.ySize - dr.yCur
	}
	if lineCount > 0 {
		buf := dr.buffer.Push(lineCount * stride)
		if err := dr.image.ReadLines(getPix(buf, dr.dtype), dr.yCur, lineCount); err != nil {
			return nil, fmt.Errorf("dataset.IO: %w", err)
		}
		dr.yCur += lineCount
	}
	return dr.buffer.Pop(chunkSize), nil
}

func (dr *ImageReader) Len() int {
	return dr.bands * dr.xSize * dr.ySize * dr.dtype.Size()
}

func (dr *ImageReader) Restart() error {
	dr.yCur = 0
	dr.buffer.Reset()
	return nil
}

// /////////////////////////////////////////////////////////////////////
// ByteArray implements ChunkReader for an array of Byte
type ByteArray struct {
	Bytes  []byte
	offset int
}

func NewByteArrayFromDataset(ds *godal.Dataset, bands, xSize, ySize int, dtype DType) (*ByteArray, error) {
	if bands == 1 {
		return NewByteArrayFromBand(&ds.Bands()[0], xSize, ySize, dtype)
	}

	ba := ByteArray{Bytes: make([]byte, bands*xSize*ySize*dtype.Size())}
	if err := ds.Read(0, 0, getPix(ba.Bytes, dtype), xSize, ySize); err != nil {
		return nil, fmt.Errorf("dataset.IO: %w", err)
	}
	return &ba, nil
}

func NewByteArrayFromBand(band *godal.Band, xSize, ySize int, dtype DType) (*ByteArray, error) {
	ba := ByteArray{Bytes: make([]byte, dtype.Size()*xSize*ySize)}
	if err := band.Read(0, 0, getPix(ba.Bytes, dtype), xSize, ySize); err != nil {
		return nil, fmt.Errorf("band.IO: %w", err)
	}
	return &ba, nil
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

func (ba *ByteArray) Restart() error {
	ba.offset = 0
	return nil
}

func getPix(bytes []byte, dtype DType) interface{} {
	// Convert up to a slice of the right type
	var pix interface{}
	switch dtype {
	case DTypeUINT8:
		pix = bytes
	case DTypeUINT16:
		pix = utils.SliceByteToGeneric[uint16](bytes)
	case DTypeUINT32:
		pix = utils.SliceByteToGeneric[uint32](bytes)
	case DTypeINT8:
		pix = utils.SliceByteToGeneric[int8](bytes)
	case DTypeINT16:
		pix = utils.SliceByteToGeneric[int16](bytes)
	case DTypeINT32:
		pix = utils.SliceByteToGeneric[int32](bytes)
	case DTypeFLOAT32:
		pix = utils.SliceByteToGeneric[float32](bytes)
	case DTypeFLOAT64:
		pix = utils.SliceByteToGeneric[float64](bytes)
	case DTypeCOMPLEX64:
		pix = utils.SliceByteToGeneric[complex64](bytes)
	}
	return pix
}
