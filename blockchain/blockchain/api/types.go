package api

import (
	db_types "IS/blockchain/database_utils/types"
	"IS/utils"
	"encoding/json"
	"fmt"
	lru "github.com/hashicorp/golang-lru"
	"go.mongodb.org/mongo-driver/bson"
	"sync"
	"sync/atomic"
	"time"
)

const ( // categories
	Unknown = iota
	Transfer
	Spending
	Obtaining
	Any // api only
)

const (
	TimeCond = iota
	AccountBalanceMoreThen
	AccountBalanceLessThen
	AccountSentTransaction
)

const ( // ETH based
	HashLen       = 32
	AddressLen    = 20
	EpochDuration = 4 * time.Second
	BlockTxsLimit = 128
)

var (
	FutureBlockError    = fmt.Errorf("unknown block")
	UnknownAddressError = fmt.Errorf("unknown address")
)

type ( // primitive types, containers
	Hash   [HashLen]byte
	Hashes []*Hash

	Address [AddressLen]byte
)

type (
	Account struct {
		Address Address `bson:"address"`
		balance Value_
	}

	Value_ struct {
		Integer    int64 `bson:"integer" json:"integer"`
		Fractional int32 `bson:"fractional" json:"fractional"` // 0-99
	}

	Transaction struct {
		ID          int64    `bson:"ID"`
		Timestamp   *int64   `bson:"timestamp"` // unix
		From        *Account `bson:"from"`
		To          *Account `bson:"to" json:"to"`
		Value       Value_   `bson:"value" json:"value"`
		Description string   `bson:"description" json:"description"`
		TxType      uint32   `bson:"txType" json:"txType"`
		Condition   *Filter  `json:"condition,omitempty" bson:"condition"`
	}
	Transactions []*Transaction

	Filter struct {
		SendAfterBlock     *utils.BlockNumber `json:"send_after"`
		SendAfterTimestamp *int64             `json:"send_after_timestamp"`

		Type        int      `json:"type"`
		CondAccount *Account `json:"cond_account"`
		CondValue   *Value_  `json:"cond_value"`
	}

	Block struct {
		Number       utils.BlockNumber `bson:"number" json:"number"`
		Transactions Transactions      `bson:"transactions" json:"transactions"`
		ParentHash   Hash              `bson:"parentHash" json:"parentHash"`
		TimeStamp    *int64            `bson:"timeStamp,omitempty" json:"timeStamp,omitempty"` // unix
		Hash_        atomic.Value
	}
	Blocks        []*Block
	BlockByNumber map[utils.BlockNumber]*Block
	BlockByHash   map[Hash]*Block

	State struct {
		Balances            map[Address]Value_
		LastFinalizedNumber utils.BlockNumber
	}

	BlockChain struct {
		sendTxCh              chan SendTxBcRequest
		getBalanceCh          chan GetBalanceRequest
		getTxsWithFiltersCh   chan GetTransactionsWithFiltersRequest
		saveFutureTransaction chan SendTxBcRequest

		lastFinalizedBlock  *Block
		lastFinalizedNumber utils.BlockNumber
		stateCache          *lru.Cache
		txQueue             TransactionQueue
		futureTransactions  Transactions

		globalTxID int64

		mu sync.RWMutex
	}

	SendTxBcRequest struct {
		Tx         Transaction
		ResponseCh chan error
	}

	TransactionQueue struct {
		transactions Transactions
	}

	GetBalanceRequest struct {
		Address     Address           `json:"address"`
		BlockNumber utils.BlockNumber `json:"blockNumber"`
		ResponseCh  chan GetBalanceBCResponse
	}

	GetBalanceBCResponse struct {
		Balance Value_ `json:"balance"`
		Err     error  `json:"err"`
	}

	GetBPKByUsernameReq struct {
		Username string `json:"username"`
	}
	GetBPKByUsernameResp struct {
		Address Address `json:"address"`
	}

	SendTxRequest struct {
		db_types.LoginData
		Tx Transaction `json:"tx"`
	}

	GetTransactionsWithFiltersRequest struct {
		TxTypes       []uint32 `json:"txTypes,omitempty"`
		From          *Account `json:"from,omitempty"`
		To            *Account `json:"to,omitempty"`
		TimeStampFrom *int64   `json:"timeStampFrom,omitempty"`
		TimeStampTo   *int64   `json:"timeStampTo,omitempty"`
		ResponseCh    chan Transactions
	}
)

func (tq *TransactionQueue) Pop() Transaction {
	tx := tq.transactions[0]
	tq.transactions = append(tq.transactions[:0], tq.transactions[1:]...)
	return *tx
}

func (tq *TransactionQueue) GetMaxCountAndRemove() Transactions {
	if len(tq.transactions) <= BlockTxsLimit {
		res := make(Transactions, len(tq.transactions))

		copy(res, tq.transactions)
		tq.transactions = make(Transactions, 0, 1)
		return res
	}

	res := make(Transactions, BlockTxsLimit)
	copy(res, tq.transactions)

	tq.transactions = append(tq.transactions[:0], tq.transactions[BlockTxsLimit:]...)
	return res
}

func (tq *TransactionQueue) IsEmpty() bool {
	return tq == nil || len(tq.transactions) == 0
}

func (v *Value_) LessThen(v2 *Value_) bool {
	if v.Integer < v2.Integer {
		return true
	} else if v.Integer == v2.Integer {
		return v.Fractional < v2.Fractional
	}
	return false
}

func (v *Value_) GreeterThen(v2 *Value_) bool {
	if v.Integer > v2.Integer {
		return true
	} else if v.Integer == v2.Integer {
		return v.Fractional > v2.Fractional
	}
	return false
}

func (v *Value_) Equal(v2 *Value_) bool {
	return v.Integer == v2.Integer && v.Fractional == v2.Fractional
}

func (v *Value_) LessEqualThen(v2 *Value_) bool {
	return v.LessThen(v2) || v.Equal(v2)
}

func (v *Value_) GreeterEqualThen(v2 *Value_) bool {
	return v.GreeterThen(v2) || v.Equal(v2)
}

func (v *Value_) Plus(v2 *Value_) Value_ {
	newValue := Value_{
		Integer:    v.Integer + v2.Integer,
		Fractional: v.Fractional + v2.Fractional,
	}

	if newValue.Fractional >= 100 {
		newValue.Integer += int64(newValue.Fractional / 100)
		newValue.Fractional %= 100
	}

	return newValue
}

func (v *Value_) Minus(v2 *Value_) Value_ {
	newValue := Value_{
		Integer:    v.Integer - v2.Integer,
		Fractional: v.Fractional - v2.Fractional,
	}

	for newValue.Fractional < 0 {
		newValue.Integer -= 1
		newValue.Fractional += 100
	}

	return newValue
}

func (v *Value_) IsFractionalValid() bool {
	return v.Fractional < 100
}

func (acc *Account) GetHeadStateBalance(bc *BlockChain) (*Value_, error) {
	//LFN := bc.lastFinalizedNumber
	//if LFN == 0 {
	//	return &Value_{0, 0}, nil
	//}
	//numbers := bc.GetLoadedStatesNumbers()
	//
	//index := utils.GetClosest(LFN, numbers)
	//closestNumber := numbers[index]
	//if closestNumber == LFN {
	//	stateInter, _ := bc.stateCache.Get(closestNumber)
	//	state := stateInter.(State)
	//
	//	balance, ok := state.Balances[acc.Address]
	//	if !ok {
	//		return &Value_{0, 0}, nil
	//	}
	//
	//	return &balance, nil
	//}
	//
	//closestStateInter, _ := bc.stateCache.Get(closestNumber)
	//closestState := closestStateInter.(State)
	//closestStateBalance := closestState.Balances[acc.Address]
	//for closestNumber < LFN {
	//	block := bc.GetBlockByNumber(closestNumber + 1)
	//	for _, tx := range block.Transactions {
	//		if tx.To.Address == acc.Address {
	//			closestStateBalance.Plus(&tx.Value)
	//		}
	//		if tx.From.Address == acc.Address {
	//			closestStateBalance.Minus(&tx.Value)
	//		}
	//	}
	//	closestNumber++
	//}
	//
	//return &closestStateBalance, nil

	stateinter, _ := bc.stateCache.Get(bc.lastFinalizedNumber)

	state := stateinter.(State)
	res := state.Balances[acc.Address]
	return &res, nil
}

func (tx *Transaction) IsValid(bc *BlockChain) bool {
	if tx.Value.Fractional < 0 || tx.Value.Fractional > 99 {
		return false
	}
	switch tx.TxType {
	case Unknown:
		return false
	case Transfer:
		balance, err := tx.From.GetHeadStateBalance(bc)
		if err != nil {
			return false
		}
		return tx.From != nil && tx.To != nil && balance.GreeterEqualThen(&tx.Value)
	case Spending:
		balance, err := tx.From.GetHeadStateBalance(bc)
		if err != nil {
			return false
		}
		return tx.From != nil && balance.GreeterEqualThen(&tx.Value)
	case Obtaining:
		return tx.To != nil
	default:
		fmt.Println("unknown tx type")
		return false
	}
}

func (b *Block) MarshalBSON() ([]byte, error) {
	type block struct {
		Number       utils.BlockNumber `bson:"number" json:"number"`
		Transactions Transactions      `bson:"transactions" json:"transactions"`
		ParentHash   Hash              `bson:"parentHash" json:"parentHash"`
		TimeStamp    *int64            `bson:"timeStamp" json:"timeStamp"` // unix
		Hash         Hash              `bson:"hash" json:"hash"`
	}
	return bson.Marshal(block{
		Number:       b.Number,
		Transactions: b.Transactions,
		ParentHash:   b.ParentHash,
		TimeStamp:    b.TimeStamp,
		Hash:         b.GetHash(),
	})
}

func (b *Block) MarshalJSON() ([]byte, error) {
	type block struct {
		Number       utils.BlockNumber `bson:"number" json:"number"`
		Transactions Transactions      `bson:"transactions" json:"transactions"`
		ParentHash   Hash              `bson:"parentHash" json:"parentHash"`
		TimeStamp    *int64            `bson:"timeStamp" json:"timeStamp"` // unix
	}
	return json.Marshal(block{
		Number:       b.Number,
		Transactions: b.Transactions,
		ParentHash:   b.ParentHash,
		TimeStamp:    b.TimeStamp,
	})
}

func (tx *Transaction) GetBalanceDelta() map[Address]Value_ {
	res := make(map[Address]Value_)

	switch tx.TxType {
	case Transfer:
		fromDelta := tx.Value
		fromDelta.Integer *= -1
		res[tx.From.Address] = fromDelta
		res[tx.To.Address] = tx.Value
	case Spending:
		fromDelta := tx.Value
		fromDelta.Integer *= -1
		res[tx.From.Address] = fromDelta
	case Obtaining:
		res[tx.To.Address] = tx.Value
	default:
		//lol
	}

	return res
}
