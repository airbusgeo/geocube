// Copyright 2021 Airbus Defence and Space
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package osio

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unicode"

	lru "github.com/hashicorp/golang-lru"
	"github.com/vburenin/nsync"
)

// KeyReaderAt is the interface that wraps the basic ReadAt method for the specified key.
//
// For performance reasons, as we expect the ReadAt method to perform network access and thus
// be relatively slow, the ReadAt method diverges somewhat from the standard io.ReaderAt interface:
//
// • ReadAt should return ENOENT in case of an error due to an inexistant file. This non-existant
// status is cached by the Adapter in order to prevent subsequent calls to the same key.
//
// • ReadAt should return the total size of the object when called with a 0 offset. This is required
// in order to implement the io.Seeker interface, and to detect out of bounds accesses without incurring
// a network access. If you do not rely on this functionality, your implementation may return math.MaxInt64

type KeyReaderAt interface {
	// ReadAt reads len(p) bytes from the resource identified by key into p
	// starting at offset off. It returns the number of bytes read (0 <= n <= len(p)) and
	// any error encountered.
	//
	// If the read fails because the object does not exist, ReadAt must return syscall.ENOENT
	// (or a wrapped error of syscall.ENOENT)
	//
	// When ReadAt returns n < len(p), it returns a non-nil error explaining why more bytes
	// were not returned. In this respect, ReadAt is stricter than io.Read.
	//
	// Even if ReadAt returns n < len(p), it may use all of p as scratch space during the call.
	// If some data is available but not len(p) bytes, ReadAt blocks until either all the data
	// is available or an error occurs. In this respect ReadAt is different from io.Read.
	//
	// If the n = len(p) bytes returned by ReadAt are at the end of the input source, ReadAt
	// may return either err == io.EOF or err == nil.
	//
	// If ReadAt is reading from an input source with a seek offset, ReadAt should not affect
	// nor be affected by the underlying seek offset.
	//
	// Clients of ReadAt can execute parallel ReadAt calls on the same input source.
	//
	// If called with off==0, ReadAt must also return the total object size in its second
	// return value
	//
	// Implementations must not retain p.
	ReadAt(key string, p []byte, off int64) (int, int64, error)
}

// BlockCacher is the interface that wraps block caching functionality
//
// Add inserts data to the cache for the given key and blockID.
//
// Get fetches the data for the given key and blockID. It returns
// the data and wether the data was found in the cache or not
type BlockCacher interface {
	Add(key string, blockID uint, data []byte)
	Get(key string, blockID uint) ([]byte, bool)
}

// NamedOnceMutex is a locker on arbitrary lock names.
type NamedOnceMutex interface {
	//Lock tries to acquire a lock on a keyed resource. If the keyed resource is not already locked,
	//Lock aquires a lock to the resource and returns true. If the keyed resource is already locked,
	//Lock waits until the resource has been unlocked and returns false
	Lock(key interface{}) bool
	//Unlock a keyed resource. Should be called by a client whose call to Lock returned true once the
	//resource is ready for consumption by other clients
	Unlock(key interface{})
}

// Adapter caches fixed-sized chunks of a KeyReaderAt, and exposes a proxy KeyReaderAt
// that feeds from its internal cache, only falling back to the provided KeyReaderAt whenever
// data could not be retrieved from its internal cache, while ensuring that concurrent requests
// only result in a single call to the source reader.
type Adapter struct {
	blockSize       int64
	blmu            NamedOnceMutex
	numCachedBlocks int
	cache           BlockCacher
	reader          KeyReaderAt
	splitRanges     bool
	sizeCache       *lru.Cache
}

func (a *Adapter) srcReadAt(key string, buf []byte, off int64) (int, error) {
	n, tot, err := a.reader.ReadAt(key, buf, off)
	if off == 0 {
		if err != nil {
			if errors.Is(err, syscall.ENOENT) {
				a.sizeCache.Add(key, int64(-1))
			}
			if errors.Is(err, io.EOF) {
				a.sizeCache.Add(key, tot)
			}
		} else {
			a.sizeCache.Add(key, tot)
		}
	}
	return n, err
}

type AdapterOption interface {
	adapterOpt(a *Adapter) error
}

type bcao struct {
	bc BlockCacher
}

func (b bcao) adapterOpt(a *Adapter) error {
	if b.bc == nil {
		return fmt.Errorf("BlockCacher must not be nil")
	}
	a.cache = b.bc
	return nil
}

// BlockCache is an option to make Adapter use the specified block cacher. If
// not provided, the Adapter will use an internal lru cache holding up to 100 blocks
// of data
func BlockCache(bc BlockCacher) AdapterOption {
	return bcao{bc}
}

type bsao struct {
	bs string
}

type ncbao struct {
	numCachedBlocks int
}

func (b ncbao) adapterOpt(a *Adapter) error {
	if b.numCachedBlocks <= 0 {
		return fmt.Errorf("NumCachedBlocks must be > 0")
	}
	a.numCachedBlocks = b.numCachedBlocks
	return nil
}

// NumCachedBlocks is an option to set the number of blocks to cache in the
// default lru implementation. It is ignored if you are passing your own cache
// implementation through BlockCache
func NumCachedBlocks(n int) interface {
	AdapterOption
} {
	return ncbao{n}
}

func (b bsao) adapterOpt(a *Adapter) error {
	const (
		BYTE = 1 << (10 * iota)
		KILOBYTE
		MEGABYTE
		//GIGABYTE
		//TERABYTE
		//PETABYTE
		//EXABYTE
	)
	s := strings.TrimSpace(b.bs)
	if len(s) == 0 {
		return fmt.Errorf("blocksize is empty")
	}
	s = strings.ToUpper(s)

	i := strings.IndexFunc(s, unicode.IsLetter)

	if i == -1 {
		ii, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("failed to parse integer from %s: %w", b.bs, err)
		}
		if ii <= 0 {
			return fmt.Errorf("blocksize %s must be strictly positive", b.bs)
		}
		a.blockSize = int64(ii)
		return nil
	}

	bytesString, multiple := s[:i], s[i:]
	bytes, err := strconv.ParseFloat(bytesString, 64)
	if err != nil {
		return fmt.Errorf("failed to parse float from %s: %w", b.bs, err)
	}
	if bytes < 0 {
		return fmt.Errorf("blocksize %s must be strictly positive", b.bs)
	}

	switch multiple {
	/*
		case "E", "EB", "EIB":
			return int(bytes * EXABYTE)
		case "P", "PB", "PIB":
			return int(bytes * PETABYTE)
		case "T", "TB", "TIB":
			return int(bytes * TERABYTE)
		case "G", "GB", "GIB":
			return int(bytes * GIGABYTE)
	*/
	case "M", "MB", "MIB":
		a.blockSize = int64(bytes * MEGABYTE)
	case "K", "KB", "KIB":
		a.blockSize = int64(bytes * KILOBYTE)
	case "B":
		a.blockSize = int64(bytes)
	default:
		return fmt.Errorf("failed to parse blocksize %s", b.bs)
	}
	return nil
}

// BlockSize is an option to set the size of the blocks that will be cached. If not
// provided, the adapter will use 128kb blocks.
//
// BlockSize will panic if the given string does not represent a strictly positive
// number of bytes
func BlockSize(blockSize string) interface {
	AdapterOption
} {
	return bsao{blockSize}
}

type srao struct {
	splitRanges bool
}

func (b srao) adapterOpt(a *Adapter) error {
	a.splitRanges = b.splitRanges
	return nil
}

//SplitRanges is an option to prevent making MultiRead try to merge
//consecutive ranges into a single block request
func SplitRanges(splitRanges bool) interface {
	AdapterOption
} {
	return srao{splitRanges}
}

type scao struct {
	numCachedSizes int
}

func (b scao) adapterOpt(a *Adapter) error {
	var err error
	a.sizeCache, err = lru.New(b.numCachedSizes)
	return err
}

// SizeCache is an option that determines how many key sizes will be cached by
// the adapter. Having a size cache speeds up the opening of files by not requiring
// that a lookup to the KeyReaderAt for the object size.
func SizeCache(numEntries int) interface {
	AdapterOption
} {
	return scao{numEntries}
}

// NewAdapter creates a caching adapter around the provided KeyReaderAt
//
// NewAdapter will only return an error if you do not provide plausible options
// (e.g. negative number of blocks or sizes, nil caches, etc...)
const (
	DefaultBlockSize       = 128 * 1024
	DefaultNumCachedBlocks = 100
)

func NewAdapter(reader KeyReaderAt, opts ...AdapterOption) (*Adapter, error) {
	bc := &Adapter{
		blockSize:       DefaultBlockSize,
		numCachedBlocks: DefaultNumCachedBlocks,
		reader:          reader,
		splitRanges:     false,
	}
	for _, o := range opts {
		if err := o.adapterOpt(bc); err != nil {
			return nil, err
		}
	}
	if bc.cache != nil && bc.numCachedBlocks != DefaultNumCachedBlocks {
		return nil, fmt.Errorf("invalid options: NumCachedBlocks may not be used alongside BlockCache")
	}
	if bc.blmu == nil {
		bc.blmu = nsync.NewNamedOnceMutex()
	}
	if bc.cache == nil {
		bc.cache, _ = NewLRUCache(bc.numCachedBlocks)
	}
	if bc.sizeCache == nil {
		bc.sizeCache, _ = lru.New(1000)
	}
	return bc, nil
}

type blockRange struct {
	start int64
	end   int64
}

func min(n1, n2 int64) int64 {
	if n1 > n2 {
		return n2
	}
	return int64(n1)
}

func (a *Adapter) getRange(key string, rng blockRange) ([][]byte, error) {
	//fmt.Printf("getrange [%d-%d]\n", rng.start, rng.end)
	blocks := make([][]byte, rng.end-rng.start+1)
	var err error
	if rng.start == rng.end {
		blocks[0], err = a.getBlock(key, int64(rng.start))
		return blocks, err
	}
	done := make(chan bool)
	defer close(done)
	for i := rng.start; i <= rng.end; i++ {
		go func(id int64) {
			blockID := a.blockKey(key, id)
			if a.blmu.Lock(blockID) {
				//unlock block once we've finished
				<-done
				a.blmu.Unlock(blockID)
			}
		}(int64(i))
	}
	buf := make([]byte, (rng.end-rng.start+1)*a.blockSize)
	n, err := a.srcReadAt(key, buf, rng.start*a.blockSize)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	left := int64(n)
	for bid := int64(0); bid <= rng.end-rng.start && left > 0; bid++ {
		ll := min(left, a.blockSize)
		blocks[bid] = make([]byte, ll)
		copy(blocks[bid], buf[bid*a.blockSize:bid*a.blockSize+ll])
		left -= ll
		a.cache.Add(key, uint(rng.start+bid), blocks[bid])
	}
	return blocks, nil
}

func (a *Adapter) applyBlock(mu *sync.Mutex, block int64, data []byte, written []int, bufs [][]byte, offsets []int64) {
	if len(data) == 0 {
		return
	}
	blockStart := block * a.blockSize
	blockEnd := blockStart + int64(len(data))
	for ibuf := 0; ibuf < len(bufs); ibuf++ {
		//fmt.Printf("maybe apply block [%d-%d] to [%d-%d]\n", blockStart, blockEnd, offsets[ibuf], offsets[ibuf]+int64(len(bufs[ibuf])))
		if blockStart < offsets[ibuf]+int64(len(bufs[ibuf])) &&
			blockEnd > offsets[ibuf] {
			bufStart := int64(0)
			dataStart := int64(0)
			dataLen := int64(len(data))
			if blockStart < offsets[ibuf] {
				dataStart = offsets[ibuf] - blockStart
				dataLen -= dataStart
			} else {
				bufStart = blockStart - offsets[ibuf]
			}
			if trimright := blockEnd - (offsets[ibuf] + int64(len(bufs[ibuf]))); trimright > 0 {
				dataLen -= trimright
			}
			if dataLen > 0 {
				//fmt.Printf("apply block [%d-%d] to [%d-%d]\n", blockStart, blockEnd, offsets[ibuf], offsets[ibuf]+int64(len(bufs[ibuf])))
				//fmt.Printf("=>[%d:] from [%d:%d]\n", bufStart+offsets[ibuf], blockStart+dataStart, blockStart+dataStart+dataLen)
				mu.Lock()
				written[ibuf] += copy(bufs[ibuf][bufStart:], data[dataStart:dataStart+dataLen])
				mu.Unlock()
			}
		}
	}
}

func (a *Adapter) ReadAtMulti(key string, bufs [][]byte, offsets []int64) ([]int, error) {
	blids := make(map[int64]bool)
	errmu := sync.Mutex{}
	for ibuf := range bufs {
		zblock := offsets[ibuf] / a.blockSize
		lblock := (offsets[ibuf] + int64(len(bufs[ibuf])) - 1) / a.blockSize
		for ib := zblock; ib <= lblock; ib++ {
			blids[ib] = true
		}
	}
	written := make([]int, len(bufs))
	mu := &sync.Mutex{}

	var err error
	if a.splitRanges {
		wg := sync.WaitGroup{}
		wg.Add(len(blids))
		for k := range blids {
			go func(bid int64) {
				defer wg.Done()
				bdata, berr := a.getBlock(key, bid)
				if berr != nil {
					errmu.Lock()
					defer errmu.Unlock()
					if err == nil {
						err = berr
					}
					return
				}
				a.applyBlock(mu, bid, bdata, written, bufs, offsets)
			}(k)
		}
		wg.Wait()
	} else {
		blocks := make([]int64, 0)
		for k := range blids {
			bdata, ok := a.cache.Get(key, uint(k))
			if ok {
				a.applyBlock(mu, k, bdata, written, bufs, offsets)
			} else {
				blocks = append(blocks, k)
			}
		}
		if len(blocks) > 0 {
			sort.Slice(blocks, func(i, j int) bool {
				return blocks[i] < blocks[j]
			})
			wg := sync.WaitGroup{}
			rng := blockRange{start: blocks[0], end: blocks[0]}
			for k := 1; k < len(blocks); k++ {
				if blocks[k] != blocks[k-1]+1 {
					rng.end = blocks[k-1]
					wg.Add(1)
					//fmt.Printf("get // range [%d,%d]\n", rng.start, rng.end)
					go func(rng blockRange) {
						defer wg.Done()
						bblocks, berr := a.getRange(key, rng)
						if berr != nil {
							errmu.Lock()
							defer errmu.Unlock()
							if err == nil {
								err = berr
							}
							return
						}
						for ib := range bblocks {
							a.applyBlock(mu, rng.start+int64(ib), bblocks[ib], written, bufs, offsets)
						}
					}(rng)
					rng.start = blocks[k]
					rng.end = blocks[k]
				} else {
					rng.end = blocks[k]
				}
			}

			//fmt.Printf("get range [%d,%d]\n", rng.start, rng.end)
			bblocks, berr := a.getRange(key, rng)
			if berr != nil {
				errmu.Lock()
				if err == nil {
					err = berr
				}
				errmu.Unlock()
			} else {
				for ib := range bblocks {
					a.applyBlock(mu, rng.start+int64(ib), bblocks[ib], written, bufs, offsets)
				}
			}

			wg.Wait()
			if err != nil {
				return written, err
			}
		}
	}
	for i, buf := range bufs {
		if written[i] != len(buf) && err == nil {
			err = io.EOF
		}
	}
	return written, err
}

func (a *Adapter) ReadAt(key string, p []byte, off int64) (int, error) {
	written, err := a.ReadAtMulti(key, [][]byte{p}, []int64{off})
	return written[0], err
}

func (a *Adapter) blockKey(key string, id int64) string {
	return fmt.Sprintf("%s-%d", key, id)
}

func (a *Adapter) getBlock(key string, id int64) ([]byte, error) {
	blockData, ok := a.cache.Get(key, uint(id))
	if ok {
		return blockData, nil
	}
	blockID := a.blockKey(key, id)
	if a.blmu.Lock(blockID) {
		buf := make([]byte, a.blockSize)
		n, err := a.srcReadAt(key, buf, int64(id)*int64(a.blockSize))
		if err != nil && !errors.Is(err, io.EOF) {
			a.blmu.Unlock(blockID)
			return nil, err
		}
		if n > 0 {
			buf = buf[0:n]
			a.cache.Add(key, uint(id), buf)
		} else {
			buf = nil
			a.cache.Add(key, uint(id), buf)
		}
		a.blmu.Unlock(blockID)
		return buf, nil
	}
	//else (lock not acquired, recheck from cache)
	return a.getBlock(key, id)
}

type Reader struct {
	a    *Adapter
	key  string
	size int64
	off  int64
}

func (r *Reader) Read(buf []byte) (int, error) {
	if r.off >= r.size {
		return 0, io.EOF
	}
	n, err := r.a.ReadAt(r.key, buf, r.off)
	r.off += int64(n)
	return n, err
}

func (r *Reader) ReadAt(buf []byte, off int64) (int, error) {
	if off >= r.size {
		return 0, io.EOF
	}
	return r.a.ReadAt(r.key, buf, off)
}

func (r *Reader) ReadAtMulti(bufs [][]byte, offs []int64) ([]int, error) {
	for _, off := range offs {
		if off >= r.size {
			return nil, io.EOF
		}
	}
	return r.a.ReadAtMulti(r.key, bufs, offs)
}

func (r *Reader) Seek(off int64, nWhence int) (int64, error) {
	coff := r.off
	switch nWhence {
	case io.SeekCurrent:
		coff += off
	case io.SeekStart:
		coff = off
	case io.SeekEnd:
		coff = r.size + off
	default:
		return 0, os.ErrInvalid
	}
	if coff < 0 {
		return r.off, os.ErrInvalid
	}
	r.off = coff
	return r.off, nil
}

func (r *Reader) Size() int64 {
	return r.size
}

func (a *Adapter) Reader(key string) (*Reader, error) {
	si, ok := a.sizeCache.Get(key)
	var err error
	if !ok {
		_, err = a.ReadAt(key, []byte{0}, 0) //ignore errors as we just want to populate the size cache
		si, ok = a.sizeCache.Get(key)
	}
	if ok {
		size := si.(int64)
		if size == -1 {
			return nil, syscall.ENOENT
		}
		return &Reader{
			a:    a,
			key:  key,
			size: size,
			off:  0,
		}, nil
	}
	if err == nil {
		err = fmt.Errorf("BUG: size cache miss")
	}
	return nil, err
}
