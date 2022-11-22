package api

import (
	"IS/utils"
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	lru "github.com/hashicorp/golang-lru"
	"sort"
	"time"
)

var SendTxChan = make(chan SendTxBcRequest)
var GetBalanceChan = make(chan GetBalanceRequest)

func (bc *BlockChain) GetBlockByHash(hash *Hash) *Block {
	block, err := GetBlockByHash(*hash)
	if err != nil || block == nil {
		log.Error("unknown block hash", "err", err)
		return nil
	}
	return block
}

func (bc *BlockChain) GetBlockByNumber(nr utils.BlockNumber) *Block {
	block, err := GetBlockByNumber(nr)
	if err != nil || block == nil {
		log.Error("unknown block number", "err", err)
		return nil
	}
	return block
}

func (bc *BlockChain) GetBlocksByHash(hashes *Hashes) BlockByHash {
	res := make(BlockByHash)
	for _, hash := range *hashes {
		res[*hash] = bc.GetBlockByHash(hash)
	}

	return res
}

func (bc *BlockChain) GetBlocksByNumber(nrs []utils.BlockNumber) BlockByNumber {
	res := make(BlockByNumber)
	for _, nr := range nrs {
		res[nr] = bc.GetBlockByNumber(nr)
	}

	return res
}

func (bc *BlockChain) GetLoadedStatesNumbers() []utils.BlockNumber {
	keysInter := bc.stateCache.Keys()
	keys := make([]utils.BlockNumber, 0, len(keysInter))
	for _, key := range keysInter {
		keys = append(keys, key.(utils.BlockNumber))
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})
	return keys
}

func (bc *BlockChain) IsLoaded(number utils.BlockNumber) bool {
	keysInter := bc.stateCache.Keys()
	keys := make([]utils.BlockNumber, 0, len(keysInter))
	for _, key := range keysInter {
		keys = append(keys, key.(utils.BlockNumber))
	}

	contains, _ := utils.Contains(number, keys)
	return contains
}

func (bc *BlockChain) processTx(req SendTxBcRequest) {
	defer close(req.ResponseCh)

	tx := req.Tx
	if !tx.IsValid(bc) {
		req.ResponseCh <- fmt.Errorf("invalid transaction")
		return
	}

	req.ResponseCh <- nil
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.txQueue.transactions = append(bc.txQueue.transactions, &tx)
}

func (bc *BlockChain) processEpoch() {
	if bc.txQueue.IsEmpty() {
		log.Info("No transactions to finalize")
		return
	}

	now := time.Now().Unix()
	block := &Block{
		TimeStamp:    &now,
		Number:       bc.lastFinalizedNumber + 1,
		ParentHash:   bc.lastFinalizedBlock.GetHash(),
		Transactions: make(Transactions, 0, BlockTxsLimit),
	}
	block.GetHash()

	block.Transactions = append(block.Transactions, bc.txQueue.GetMaxCountAndRemove()...)
	bc.finalizeBlock(block)
}

func (bc *BlockChain) finalizeBlock(block *Block) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	err := WriteBlock(block)

	bc.lastFinalizedNumber = block.Number
	bc.lastFinalizedBlock = block

	if state, ok := bc.stateCache.Get(block.Number - 1); ok {
		bc.writeStateUsingLastState(state.(State), *block)
	}

	if err != nil {
		log.Error("can't finalize block")
	}
}

func (bc *BlockChain) writeStateUsingLastState(lastState State, block Block) {
	if lastState.LastFinalizedNumber+1 != block.Number {
		return
	}
	newState := State{
		LastFinalizedNumber: lastState.LastFinalizedNumber + 1,
		Balances:            utils.Copy(lastState.Balances),
	}
	for _, tx := range block.Transactions {
		deltas := tx.GetBalanceDelta()
		for addr, delta := range deltas {
			if _, ok := newState.Balances[addr]; !ok {
				newState.Balances[addr] = delta
				continue
			}

			value_ := newState.Balances[addr]
			if delta.Integer < 0 {
				delta.Integer *= -1
				value_ = value_.Minus(&delta)
			} else {
				value_ = value_.Plus(&delta)
			}

			newState.Balances[addr] = value_
		}
	}
	bc.stateCache.Add(newState.LastFinalizedNumber, newState)
}

func (bc *BlockChain) processBalanceRequest(req GetBalanceRequest) {
	defer close(req.ResponseCh)
	respCh := req.ResponseCh
	stateInter, exists := bc.stateCache.Get(req.BlockNumber)
	if !exists || stateInter == nil {
		respCh <- GetBalanceBCResponse{Err: FutureBlockError}
		return
	}

	state := stateInter.(State)
	if balance, exists := state.Balances[req.Address]; !exists {
		respCh <- GetBalanceBCResponse{Err: UnknownAddressError}
	} else {
		respCh <- GetBalanceBCResponse{Balance: balance}
	}
}

func (bc *BlockChain) loadStates() {
	blocks := GetBlocks()
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Number < blocks[j].Number
	})

	if len(blocks) == 0 {
		return
	}

	bc.writeStateUsingLastState(State{Balances: make(map[Address]Value_), LastFinalizedNumber: -1}, *blocks[0])

	for i, block := range blocks {
		if i == 0 {
			continue
		}
		stateInter, _ := bc.stateCache.Get(block.Number - 1)
		state := stateInter.(State)
		bc.writeStateUsingLastState(state, *block)
	}
}

func Start() {
	go func() {
		stateCache, _ := lru.New(512)
		bc := &BlockChain{
			stateCache:   stateCache,
			sendTxCh:     SendTxChan,
			getBalanceCh: GetBalanceChan,
		}

		if !BlocksExist() {
			block := StateBlock()
			bc.finalizeBlock(&block)
			bc.writeStateUsingLastState(State{Balances: make(map[Address]Value_), LastFinalizedNumber: -1}, block)
		} else {
			LFB, _ := GetLastBlock()
			bc.lastFinalizedBlock = LFB
			bc.lastFinalizedNumber = LFB.Number
		}
		bc.loadStates()

		epochTicker := time.NewTicker(EpochDuration)
		for {
			select {
			case req := <-bc.sendTxCh:
				bc.processTx(req)
			case req := <-bc.getBalanceCh:
				bc.processBalanceRequest(req)
			case <-epochTicker.C:
				bc.processEpoch()
			}
		}
	}()
}
