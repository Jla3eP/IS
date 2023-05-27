package api

import (
	db_types "IS/blockchain/database_utils/types"
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
	"unicode/utf8"
)

func CreateUserReq(c *gin.Context) {
	usr := &db_types.LoginData{}
	err := c.ShouldBindJSON(usr)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
	}

	if usr.Username == "" || utf8.RuneCountInString(usr.Username) < 4 {
		c.JSON(http.StatusBadRequest, "nickname is so short")
		return
	}

	if usr.Password == "" {
		c.JSON(http.StatusBadRequest, "")
		return
	} else if utf8.RuneCountInString(usr.Password) < 8 {
		c.JSON(http.StatusBadRequest, "password is too short")
		return
	}

	if err = CreateUser(context.Background(), db_types.User{Username: usr.Username}, usr.Password); err != nil {
		c.JSON(http.StatusConflict, err.Error())
		return
	}

	c.JSON(http.StatusOK, fmt.Sprintf("%s, your account was created\n", usr.Username))
}

func GetPublicKeyByUsername(c *gin.Context) {
	RequestData := GetBPKByUsernameReq{}
	err := c.ShouldBindJSON(&RequestData)
	if err != nil {
		c.JSON(http.StatusBadRequest, "invalid body")
	}

	if !UserExists(context.Background(), RequestData.Username) {
		c.JSON(http.StatusNoContent, "unknown username")
		return
	}

	response := GetBPKByUsernameResp{}
	response.Address = CalculatePublicKeyByUsername(RequestData.Username)

	c.JSON(http.StatusOK, response)
}

func SendTx(c *gin.Context) {
	timestamp := time.Now().Unix()

	Request := SendTxRequest{}
	err := c.ShouldBindJSON(&Request)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}

	if ok, err := VerifyPassword(context.TODO(), db_types.User{Username: Request.Username}, Request.Password); err != nil || !ok {
		c.JSON(http.StatusUnauthorized, "")
		return
	}

	if Request.Tx.TxType == Transfer || Request.Tx.TxType == Spending {
		Request.Tx.From = &Account{Address: CalculatePublicKeyByUsername(Request.Username)}
	}
	Request.Tx.Timestamp = &timestamp

	errChan := make(chan error)
	if Request.Tx.Condition != nil {
		SaveFutureTxCh <- SendTxBcRequest{Tx: Request.Tx, ResponseCh: errChan}
	} else {
		SendTxCh <- SendTxBcRequest{Tx: Request.Tx, ResponseCh: errChan}
	}

	select {
	case err := <-errChan:
		if err != nil {
			c.JSON(http.StatusNotAcceptable, err)
			return
		}
		c.JSON(http.StatusOK, "ok")
	case <-time.After(10 * time.Second):
		c.JSON(http.StatusProcessing, "")
		return
	}
}

func GetBalanceByBlockNumber(c *gin.Context) {
	Request := GetBalanceRequest{
		ResponseCh: make(chan GetBalanceBCResponse),
	}
	err := c.ShouldBindJSON(&Request)

	if err != nil {
		c.JSON(http.StatusBadRequest, "bad request")
		return
	}

	GetBalanceCh <- Request
	Response := <-Request.ResponseCh
	if errors.Is(Response.Err, FutureBlockError) {
		c.JSON(http.StatusBadRequest, "")
		return
	}

	c.JSON(http.StatusOK, Response.Balance)
}

func GetTransactionsWithFilters(c *gin.Context) {
	Request := GetTransactionsWithFiltersRequest{ResponseCh: make(chan Transactions)}
	err := c.ShouldBindJSON(&Request)
	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusBadRequest, "")
		return
	}

	GetTxsWithFiltersCh <- Request

	resp := <-Request.ResponseCh

	c.JSON(http.StatusOK, resp)
}
