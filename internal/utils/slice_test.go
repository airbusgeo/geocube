package utils

import (
	"testing"
	"unsafe"
)

func TestSlice(t *testing.T) {
	b := make([]byte, 5, 10)
	for i := 0; i < 5; i++ {
		b[i] = byte(i)
	}

	newLen := 51515151561
	setCapLen(unsafe.Pointer(&b), newLen)
	if len(b) != newLen {
		t.Errorf("len(b) : want %d, got %d", newLen, len(b))
	}
	if cap(b) != newLen {
		t.Errorf("cap(b) : want %d, got %d", newLen, cap(b))
	}
	newLen = 10
	setCapLen(unsafe.Pointer(&b), newLen)

	b2 := ToSliceByte(unsafe.Pointer(&b[0]), len(b))

	if len(b2) != len(b) {
		t.Errorf("len(b2) : want %d, got %d", len(b), len(b2))
	}
	if unsafe.Pointer(&b2[0]) != unsafe.Pointer(&b[0]) {
		t.Errorf("different pointers")
	}

	f := []float32{1.0, 2.0, 3.0, 4.0}
	b2 = ToSliceByte(unsafe.Pointer(&f[0]), len(f)*4)
	f2 := SliceByteToGeneric[float32](b2)
	if len(f2) != len(f) {
		t.Errorf("len(f2) want:%d get:%d", len(f), len(f2))
	}
	for i := range f {
		if f2[i] != f[i] {
			t.Errorf("want:%v get:%v", f[i], f2[i])
		}
	}
}

func equals(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := range s1 {
		if s1[i] != s2[i] {
			return false
		}
	}
	return true
}

func TestStringSet(t *testing.T) {
	s := StringSet{}
	s.Push("10")
	s.Push("10")
	if !equals(s.Slice(), []string{"10"}) {
		t.Errorf("want:%v get:%v", []string{"10"}, s.Slice())
	}
	s.Push("11")
	slice := s.Slice()
	if !equals(slice, []string{"10", "11"}) && !equals(slice, []string{"11", "10"}) {
		t.Errorf("want:%v get:%v", []string{"10", "11"}, slice)
	}
	s.Pop("10")
	if !equals(s.Slice(), []string{"11"}) {
		t.Errorf("want:%v get:%v", []string{"11"}, s.Slice())
	}

	if !s.Exists("11") {
		t.Errorf("expecting 11")
	}
}
