package api

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
	"sync"
)

var hasherPool = sync.Pool{
	New: func() interface{} { return sha3.NewLegacyKeccak256() },
}

func (b *Block) GetHash() (h Hash) {
	if b == nil {
		return
	}
	if hash := b.Hash_.Load(); hash != nil {
		return hash.(Hash)
	}
	blockCopy := b.Copy()
	blockCopy.TimeStamp = nil

	JSON, _ := json.Marshal(blockCopy)

	b.Hash_.Store(rlpHash(JSON))
	return b.Hash_.Load().(Hash)
}

func (b *Block) Copy() *Block {
	cpy := &Block{}

	cpy.Transactions = b.Transactions
	cpy.Number = b.Number
	cpy.ParentHash = b.ParentHash
	cpy.Hash_ = b.Hash_
	cpy.TimeStamp = new(int64)
	if b.TimeStamp != nil {
		*cpy.TimeStamp = *b.TimeStamp
	}

	return cpy
}

func rlpHash(x interface{}) (h Hash) {
	sha := hasherPool.Get().(crypto.KeccakState)
	defer hasherPool.Put(sha)
	sha.Reset()
	rlp.Encode(sha, x)
	sha.Read(h[:])
	return h
}
