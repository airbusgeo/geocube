package mucog

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	IDX_IMAGE int = iota // TopLevel IFD/Dataset
	IDX_LEVEL            // Full + Overviews ie 0: Full, 1:N: Overviews/Reduced image
	IDX_TILE             // Block/Chunk
	IDX_PLANE            // Bands
	KEY_IMAGE = "I"
	KEY_LEVEL = "L"
	KEY_TILE  = "T"
	KEY_PLANE = "P"
)

var Names = []string{"Image", "Level", "Tile", "Plane"}

// Iterator on integers with an Identifier
// Usage:
// var it Iterator
// var indices = []*int
// for it.Init(indices); it.Next(); {
//   fmt.Printf("It[%d] = %d", it.ID(), *pval)
// }
type Iterator interface {
	// ID returns the identifier of the iterator
	ID() int
	// Init resets the iterator and initializes indices[ID()] with the pointer on the current value (updated when Next() is called, invalid if Next()==False)
	Init(indices []*int)
	// Next updates the current value and returns True, or False if the iteration is finished.
	Next() bool
}

func InitIterators(pattern string, nbImages, nbPlanes int, levelMinMaxBlock [][4]int32) ([]*Iterators, error) {
	var iterators []*Iterators
	for _, itersS := range strings.Split(pattern, ";") {
		iters, err := NewIteratorsFromString(itersS, nbImages, nbPlanes, levelMinMaxBlock)
		if err != nil {
			return nil, err
		}
		iterators = append(iterators, iters)
	}
	return iterators, nil
}

// RangeIterator implements Iterator on a range of values from start to end (included)
type RangeIterator struct {
	id         int
	curValue   int
	Start, End int
}

// NewRangeIterator creates an Iterator on a range of values from start to end (included)
func NewRangeIterator(id, start, end int) Iterator {
	return &RangeIterator{
		id:    id,
		Start: start,
		End:   end,
	}
}

func (it *RangeIterator) Init(indices []*int) {
	it.curValue = it.Start - 1
	indices[it.id] = &it.curValue
}

func (it *RangeIterator) ID() int {
	return it.id
}

func (it *RangeIterator) Next() bool {
	if it.curValue == it.End-1 {
		return false
	}
	it.curValue++
	return true
}

// ValuesIterator implements Iterator on a slice of values
type ValuesIterator struct {
	id       int
	Values   []int
	curValue int
	curIdx   int
}

// NewValuesIterator creates an Iterator on a slice of values
func NewValuesIterator(id int, values []int) Iterator {
	return &ValuesIterator{
		id:     id,
		Values: values,
	}
}

func (it *ValuesIterator) Init(indices []*int) {
	it.curIdx = 0
	indices[it.id] = &it.curValue
}

func (it *ValuesIterator) ID() int {
	return it.id
}

func (it *ValuesIterator) Next() bool {
	if it.curIdx == len(it.Values) {
		return false
	}
	it.curValue = it.Values[it.curIdx]
	it.curIdx++
	return true
}

const (
	MIN_X int = iota
	MAX_X
	MIN_Y
	MAX_Y
)

// TileIterator creates an Iterator on the tiles of an overview level.
type TileIterator struct {
	id               int
	curValue         int
	maxX, minY, maxY int32
	curX, curY       int32
	levelMinMaxBlock [][4]int32
}

// NewTileIterator creates an Iterator on the blocks of an overview level.
func NewTileIterator(id int, levelMinMaxBlock [][4]int32) Iterator {
	return &TileIterator{
		id:               id,
		levelMinMaxBlock: levelMinMaxBlock,
	}
}

// Init returns a pointer on an encoded value of the block indices (see DecodePair to get x, y)
func (it *TileIterator) Init(indices []*int) {
	levelIdx := *indices[IDX_LEVEL]
	it.curX, it.maxX = it.levelMinMaxBlock[levelIdx][MIN_X], it.levelMinMaxBlock[levelIdx][MAX_X]
	it.minY, it.maxY = it.levelMinMaxBlock[levelIdx][MIN_Y], it.levelMinMaxBlock[levelIdx][MAX_Y]
	it.curY = it.minY
	indices[it.id] = &it.curValue
}

func (it *TileIterator) ID() int {
	return it.id
}

func (it *TileIterator) Next() bool {
	it.curValue = EncodePair(it.curX, it.curY)
	if it.curX >= it.maxX || it.curY >= it.maxY {
		return false
	}
	if it.curY < it.maxY {
		it.curY++
	}
	if it.curY >= it.maxY {
		it.curX++
		it.curY = it.minY
	}
	return true
}

// EncodePair creates an int from x, y coordinates
func EncodePair(x, y int32) int {
	return int(x)*(math.MaxUint32+1) + int(y)
}

// DecodePair retrieves x, y from an encoded pair
func DecodePair(p int) (int32, int32) {
	return int32(p / (math.MaxUint32 + 1)), int32(p % (math.MaxUint32 + 1))
}

type Iterators [4]Iterator

func NewIteratorsFromString(s string, nbImages, nbPlanes int, levelMinMaxBlock [][4]int32) (*Iterators, error) {
	its := strings.Split(s, ">")
	if len(its) != 4 {
		return nil, fmt.Errorf("%s must have four level of iterations, got %d", s, len(its))
	}

	var res Iterators
	for i, it := range its {
		itSplit := strings.SplitN(it, "=", 2)
		switch itSplit[0] {
		case KEY_TILE:
			res[i] = NewTileIterator(IDX_TILE, levelMinMaxBlock)

		case KEY_PLANE, KEY_IMAGE, KEY_LEVEL:
			var idx, maxV int
			switch itSplit[0] {
			case KEY_PLANE:
				idx, maxV = IDX_PLANE, nbPlanes
			case KEY_IMAGE:
				idx, maxV = IDX_IMAGE, nbImages
			case KEY_LEVEL:
				idx, maxV = IDX_LEVEL, len(levelMinMaxBlock)
			}
			if len(itSplit) == 1 || strings.Contains(itSplit[1], ":") {
				// Using range
				minV := 0
				if len(itSplit) == 2 {
					valuesS := strings.SplitN(itSplit[1], ":", 2)
					if valuesS[0] != "" { // Parse first value of the range
						nMinV, err := strconv.Atoi(valuesS[0])
						if err != nil {
							return nil, fmt.Errorf("cannot parse min value of range %s: %w", itSplit[1], err)
						}
						if nMinV > minV {
							minV = nMinV
						}
					}
					if valuesS[1] != "" { // Parse last value of the range
						nMaxV, err := strconv.Atoi(valuesS[1])
						if err != nil {
							return nil, fmt.Errorf("cannot parse max value of range %s: %w", itSplit[1], err)
						}
						if nMaxV < maxV {
							maxV = nMaxV
						}
					}
				}
				res[i] = NewRangeIterator(idx, minV, maxV)
			} else {
				// Using values
				valuesS := strings.Split(itSplit[1], ",")
				var values []int
				for _, v := range valuesS {
					v, err := strconv.Atoi(v)
					if err != nil {
						return nil, fmt.Errorf("cannot parse values of %s: %w", it, err)
					}
					if 0 <= v && v <= maxV {
						values = append(values, v)
					}
				}
				res[i] = NewValuesIterator(idx, values)
			}
		default:
			return nil, fmt.Errorf("unknown key %s: must be one of [%s, %s, %s, %s]", itSplit[0], KEY_PLANE, KEY_IMAGE, KEY_LEVEL, KEY_TILE)
		}
	}
	return &res, res.Check()
}

func (its Iterators) Check() error {
	defined := [4]bool{}
	for _, iter := range its {
		idx := iter.ID()
		if idx > len(defined) {
			return fmt.Errorf("Iterators.Check: unknown index %d", idx)
		}
		if defined[idx] {
			return fmt.Errorf("Iterators.Check: %s (idx=%d) is defined twice", Names[idx], idx)
		}
		if idx == IDX_TILE && !defined[IDX_LEVEL] {
			return fmt.Errorf("Iterators.Check: %s (idx=%d) cannot be defined before %s (idx=%d)", Names[IDX_TILE], IDX_TILE, Names[IDX_LEVEL], IDX_LEVEL)
		}
		defined[idx] = true
	}
	return nil
}
