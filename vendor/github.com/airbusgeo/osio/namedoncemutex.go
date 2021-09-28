// The MIT License (MIT)
//
// Copyright (c) 2016 Volodymyr Burenin
// Copyright (c) 2021 Airbus Defence and Space
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package osio

import "sync"

// onceMutex is a mutex that can be locked only once.
// Lock operation returns true if mutex has been successfully locked.
// Any other concurrent attempts will block until mutex is unlocked.
// However, any other attempts to grab a lock will return false.
type onceMutex struct {
	mu   sync.Mutex
	used bool
}

// Lock tries to acquire lock.
func (om *onceMutex) Lock() bool {
	om.mu.Lock()
	if om.used {
		om.mu.Unlock()
		return false
	}
	return true
}

// Unlock tries to release a lock.
func (om *onceMutex) Unlock() {
	om.used = true
	om.mu.Unlock()
}

// NamedOnceMutex is a map of dynamically created mutexes by provided id.
// First attempt to lock by id will create a new mutex and acquire a lock.
// All other concurrent attempts will block waiting mutex to be unlocked for the same id.
// Once mutex unlocked, all other lock attempts will return false for the same instance of mutex.
// Unlocked mutex is discarded. Next attempt to acquire a lock for the same id will succeed.
// Such behaviour may be used to refresh a local cache of data identified by some key avoiding
// concurrent request to receive a refreshed value for the same key.
type namedOnceMutex struct {
	lockMap map[interface{}]*onceMutex
	mutex   sync.Mutex
}

// NewNamedOnceMutex returns an instance of NamedOnceMutex.
func newNamedOnceMutex() *namedOnceMutex {
	return &namedOnceMutex{
		lockMap: make(map[interface{}]*onceMutex),
	}
}

// Lock try to acquire a lock for provided id. If attempt is successful, true is returned
// If lock is already acquired by something else it will block until mutex is unlocked returning false.
func (nom *namedOnceMutex) Lock(useMutexKey interface{}) bool {
	nom.mutex.Lock()
	m, ok := nom.lockMap[useMutexKey]
	if ok {
		nom.mutex.Unlock()
		return m.Lock()
	}

	m = &onceMutex{}
	m.Lock()
	nom.lockMap[useMutexKey] = m
	nom.mutex.Unlock()
	return true
}

// TryLock try to acquire a lock for provided id. If attempt is successful, true is returned
// If lock is already acquired by something else it will return false.
func (nom *namedOnceMutex) TryLock(useMutexKey interface{}) bool {
	nom.mutex.Lock()
	_, ok := nom.lockMap[useMutexKey]
	if ok {
		nom.mutex.Unlock()
		return false
	}

	m := &onceMutex{}
	m.Lock()
	nom.lockMap[useMutexKey] = m
	nom.mutex.Unlock()
	return true
}

// Unlock unlocks the locked mutex. Used mutex will be discarded.
func (nom *namedOnceMutex) Unlock(useMutexKey interface{}) {
	nom.mutex.Lock()
	m, ok := nom.lockMap[useMutexKey]
	if ok {
		delete(nom.lockMap, useMutexKey)
		nom.mutex.Unlock()
		m.Unlock()
	} else {
		nom.mutex.Unlock()
	}
}
