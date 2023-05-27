package api

import (
	"IS/utils"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	lru "github.com/hashicorp/golang-lru"
	"sort"
	"time"
)

var (
	SendTxCh            = make(chan SendTxBcRequest, 1)
	SaveFutureTxCh      = make(chan SendTxBcRequest, 1)
	GetBalanceCh        = make(chan GetBalanceRequest, 1)
	GetTxsWithFiltersCh = make(chan GetTransactionsWithFiltersRequest, 1)
)

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
	bc.txQueue.transactions = append(bc.txQueue.transactions, &tx)
}

func (bc *BlockChain) processEpoch() {
	if bc.txQueue.IsEmpty() && len(bc.futureTransactions) == 0 {
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

	block.Transactions = append(append(block.Transactions, bc.txQueue.GetMaxCountAndRemove()...), bc.getTransactionsToFinalize()...)
	if len(block.Transactions) == 0 {
		log.Info("No transactions to finalize")
		return
	}
	bc.finalizeBlock(block)
}

func (bc *BlockChain) getTransactionsToFinalize() (res Transactions) {
	now := time.Now().Unix()
	for i := 0; i < len(bc.futureTransactions); i++ {
		tx := bc.futureTransactions[i]
		if tx.Condition.SendAfterBlock != nil && *tx.Condition.SendAfterBlock != bc.lastFinalizedNumber {
			continue
		}
		if tx.Condition.SendAfterTimestamp != nil && *tx.Condition.SendAfterTimestamp > now {
			continue
		}
		switch tx.Condition.Type {
		case TimeCond:
			res = append(res, tx)
			bc.futureTransactions = append(bc.futureTransactions[:i], bc.futureTransactions[i+1:]...)
			i--
		case AccountBalanceMoreThen:
			if val, err := bc.getBalance(bc.lastFinalizedNumber, tx.Condition.CondAccount.Address); err == nil &&
				val.GreeterThen(tx.Condition.CondValue) {
				res = append(res, tx)
				bc.futureTransactions = append(bc.futureTransactions[:i], bc.futureTransactions[i+1:]...)
				i--
			}
		case AccountBalanceLessThen:
			if val, err := bc.getBalance(bc.lastFinalizedNumber, tx.Condition.CondAccount.Address); err == nil &&
				val.LessThen(tx.Condition.CondValue) {
				res = append(res, tx)
				bc.futureTransactions = append(bc.futureTransactions[:i], bc.futureTransactions[i+1:]...)
				i--
			}
		case AccountSentTransaction:
			for _, finTx := range bc.lastFinalizedBlock.Transactions {
				if finTx.From.Address == tx.Condition.CondAccount.Address {
					res = append(res, tx)
					bc.futureTransactions = append(bc.futureTransactions[:i], bc.futureTransactions[i+1:]...)
					i--
				}
			}
		}
	}

	return res
}

func (bc *BlockChain) finalizeBlock(block *Block) {
	err := WriteBlock(block)

	bc.lastFinalizedNumber = block.Number
	bc.lastFinalizedBlock = block

	if state, ok := bc.stateCache.Get(block.Number - 1); ok {
		bc.writeStateUsingLastState(state.(State), *block)
	}

	if err != nil {
		log.Error("can't finalize block")
		return
	}

	for _, tx := range block.Transactions {
		_ = DeleteTransactionByID(tx.ID)
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
	if bc.lastFinalizedNumber < block.Number {
		bc.lastFinalizedBlock = &block
		bc.lastFinalizedNumber = block.Number
	}
}

func (bc *BlockChain) processBalanceRequest(req GetBalanceRequest) {
	defer close(req.ResponseCh)
	respCh := req.ResponseCh

	res, err := bc.getBalance(req.BlockNumber, req.Address)
	if err != nil {
		respCh <- GetBalanceBCResponse{Err: err}
		return
	}

	respCh <- GetBalanceBCResponse{Balance: *res}
}

func (bc *BlockChain) getBalance(blockNumber utils.BlockNumber, addr Address) (*Value_, error) {
	stateInter, exists := bc.stateCache.Get(blockNumber)
	if !exists || stateInter == nil {
		return nil, FutureBlockError
	}

	state := stateInter.(State)
	if balance, exists := state.Balances[addr]; !exists {
		return nil, UnknownAddressError
	} else {
		return &balance, nil
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

func (bc *BlockChain) getTransactionsUsingFilters(req GetTransactionsWithFiltersRequest) {
	result := make(Transactions, 0)

	for num := utils.BlockNumber(1); num <= bc.lastFinalizedNumber; num++ {
		block := bc.GetBlockByNumber(num)

		exit := false
		for _, tx := range block.Transactions {
			if req.TxTypes != nil {
				if ok, _ := utils.Contains(Any, req.TxTypes); ok {
					req.TxTypes = nil
				} else {
					if ok, _ := utils.Contains(tx.TxType, req.TxTypes); !ok {
						continue
					}
				}
			}
			if req.From != nil {
				if tx.From != nil && tx.From.Address != req.From.Address {
					continue
				}
			}
			if req.To != nil {
				if tx.To != nil && tx.To.Address != req.To.Address {
					continue
				}
			}
			if req.TimeStampFrom != nil {
				if *tx.Timestamp < *req.TimeStampFrom {
					continue
				}
			}
			if req.TimeStampTo != nil {
				if *tx.Timestamp > *req.TimeStampTo {
					exit = true
					continue
				}
			}
			result = append(result, tx)
		}
		if exit {
			break
		}
	}
	req.ResponseCh <- result
	close(req.ResponseCh)
}

func (bc *BlockChain) processFutureTxRequest(req SendTxBcRequest) {
	if req.Tx.Condition == nil {
		req.ResponseCh <- errors.New("missing required parameter: condition")
		return
	}

	switch req.Tx.Condition.Type {
	case AccountBalanceMoreThen, AccountBalanceLessThen:
		if req.Tx.Condition.CondAccount == nil {
			req.ResponseCh <- errors.New("missing required parameter: CondAccount")
			return
		} else if req.Tx.Condition.CondValue == nil {
			req.ResponseCh <- errors.New("missing required parameter: CondValue")
			return
		}
	case AccountSentTransaction:
		if req.Tx.Condition.CondAccount == nil {
			req.ResponseCh <- errors.New("missing required parameter: CondAccount")
			return
		}
	case TimeCond:
		//nothing
	default:
		req.ResponseCh <- errors.New("unknown cond type")
	}

	req.Tx.ID = bc.globalTxID
	bc.globalTxID++
	bc.futureTransactions = append(bc.futureTransactions, &req.Tx)
	err := WriteFutureTx(&req.Tx)
	if err != nil {
		log.Error("can't write future tx")
	}

	req.ResponseCh <- nil
}

func Start() {
	go func() {
		stateCache, _ := lru.New(512)
		bc := &BlockChain{
			stateCache:            stateCache,
			sendTxCh:              SendTxCh,
			saveFutureTransaction: SaveFutureTxCh,
			getBalanceCh:          GetBalanceCh,
			getTxsWithFiltersCh:   GetTxsWithFiltersCh,
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
		bc.futureTransactions = GetFutureTxs()

		epochTicker := time.NewTicker(EpochDuration)
		for {
			select {
			case req := <-bc.sendTxCh:
				bc.processTx(req)
			case req := <-bc.getBalanceCh:
				bc.processBalanceRequest(req)
			case req := <-bc.getTxsWithFiltersCh:
				bc.getTransactionsUsingFilters(req)
			case req := <-bc.saveFutureTransaction:
				bc.processFutureTxRequest(req)
			case <-epochTicker.C:
				bc.processEpoch()
			}
		}
	}()
}
