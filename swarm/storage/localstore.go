// Copyright 2016 The BXMP Authors
// This file is part of the BXMP library.
//
// The BXMP library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The BXMP library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the BXMP library. If not, see <http://www.gnu.org/licenses/>.

package storage

import (
	"encoding/binary"
)

// LocalStore is a combination of inmemory db over a disk persisted db
// implements a Get/Put with fallback (caching) logic using any 2 ChunkStores
type LocalStore struct {
	memStore ChunkStore
	DbStore  ChunkStore
}

// This constructor uses MemStore and DbStore as components
func NewLocalStore(hash SwarmHasher, params *StoreParams) (*LocalStore, error) {
	dbStore, err := NewDbStore(params.ChunkDbPath, hash, params.DbCapacity, params.Radius)
	if err != nil {
		return nil, err
	}
	return &LocalStore{
		memStore: NewMemStore(dbStore, params.CacheCapacity),
		DbStore:  dbStore,
	}, nil
}

// LocalStore is itself a chunk store
// unsafe, in that the data is not integrity checked
func (self *LocalStore) Put(chunk *Chunk) {
	chunk.dbStored = make(chan bool)
	self.memStore.Put(chunk)
	if chunk.wg != nil {
		chunk.wg.Add(1)
	}
	go func() {
		self.DbStore.Put(chunk)
		if chunk.wg != nil {
			chunk.wg.Done()
		}
	}()
}

// Get(chunk *Chunk) looks up a chunk in the local stores
// This method is blocking until the chunk is retrieved
// so additional timeout may be needed to wrap this call if
// ChunkStores are remote and can have long latency
func (self *LocalStore) Get(key Key) (chunk *Chunk, err error) {
	chunk, err = self.memStore.Get(key)
	if err == nil {
		return
	}
	chunk, err = self.DbStore.Get(key)
	if err != nil {
		return
	}
	chunk.Size = int64(binary.LittleEndian.Uint64(chunk.SData[0:8]))
	self.memStore.Put(chunk)
	return
}

// Close local store
func (self *LocalStore) Close() {}
