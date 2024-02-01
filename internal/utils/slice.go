package utils

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

// ToSliceByte converts an unsafe.Pointer to a slice of byte
// Usage:
// f := []float64{1.0, 2.0, 3.0}
// b := ToSliceByte(unsafe.Pointer(&f[0]), len(f)*8)
func ToSliceByte(ptr unsafe.Pointer, l int) []byte {
	sl := (*[1]byte)(ptr)[:]
	setCapLen(unsafe.Pointer(&sl), l)
	return sl
}

func convertSize(b []byte, d int) int {
	l := len(b)
	if l%d != 0 {
		panic(fmt.Sprintf("len must be a multiple of %d", d))
	}
	return l / d
}

// SliceByteToGeneric converts a slice of byte to a slice of generic
func SliceByteToGeneric[T any](b []byte) []T {
	r := (*[1]T)(unsafe.Pointer(&b[0]))[:]
	size := reflect.TypeOf((*T)(nil)).Elem().Size()
	setCapLen(unsafe.Pointer(&r), convertSize(b, int(size)))
	return r
}

// Ugly function to set the capacity and the length of a slice
func setCapLen(ptr unsafe.Pointer, l int) {
	addrSize := unsafe.Sizeof(uintptr(0))
	lenPtr := unsafe.Pointer(uintptr(ptr) + addrSize)   // Capture the address where the length and cap size is stored
	capPtr := unsafe.Pointer(uintptr(ptr) + 2*addrSize) // WARNING: This is fragile, depending on a go-internal structure.
	*(*int)(lenPtr) = l
	*(*int)(capPtr) = l
}

// SliceInt64Equal returns true if the two slices contain the same elements
func SliceInt64Equal(a, b []int64) bool {
	// For big slices, it's more efficient to use ToSliceByte and bytes.Equal ?
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// SliceFloat64Equal returns true if the two slices contain the same elements
func SliceFloat64Equal(a, b []float64) bool {
	// For big slices, it's more efficient to use ToSliceByte and bytes.Equal ?
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// JoinInt64 is the int64 equivalent of strings.Join
func JoinInt64(elems []int64, sep string) string {
	strelems := make([]string, len(elems))
	for i, e := range elems {
		strelems[i] = strconv.Itoa(int(e))
	}
	return strings.Join(strelems, sep)
}

// StringSet is a set of strings (all elements are unique)
type StringSet map[string]struct{}

// Push adds the string to the set if not already exists
func (ss StringSet) Push(s string) {
	ss[s] = struct{}{}
}

// Pop removes the string from the set
func (ss StringSet) Pop(s string) {
	delete(ss, s)
}

// Slice returns a slice from the set
func (ss StringSet) Slice() []string {
	sl := make([]string, 0, len(ss))
	for k := range ss {
		sl = append(sl, k)
	}
	return sl
}

// Exists returns true if the string already exists in the Set
func (ss StringSet) Exists(s string) bool {
	_, ok := ss[s]
	return ok
}
