package api

import (
	db_types "IS/blockchain/database_utils/types"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
	"unicode/utf8"
)

func CreateUserReq(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	usr, err := getLoginDataFromRequest(w, r)

	if usr.Username == "" || utf8.RuneCountInString(usr.Username) < 4 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("nickname is so short\n"))
		return
	}

	if usr.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	} else if utf8.RuneCountInString(usr.Password) < 8 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("password is too short\n"))
		return
	}

	if err = CreateUser(context.Background(), db_types.User{Username: usr.Username}, usr.Password); err != nil {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(err.Error() + "\n"))
		return
	}
	w.WriteHeader(200)
	w.Write([]byte(fmt.Sprintf("%s, your account was created\n", usr.Username)))
}

func GetPublicKeyByUsername(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	requestBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid body"))
		return
	}

	RequestData := GetBPKByUsernameReq{}
	err = json.Unmarshal(requestBody, &RequestData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid body"))
		return
	}

	if !UserExists(context.Background(), RequestData.Username) {
		w.WriteHeader(http.StatusNoContent)
		w.Write([]byte("unknown username"))
		return
	}

	response := GetBPKByUsernameResp{}
	response.Address = CalculatePublicKeyByUsername(RequestData.Username)

	responseJson, err := json.Marshal(response)
	w.Write(responseJson)
}

func SendTx(w http.ResponseWriter, r *http.Request) {
	timestamp := time.Now().Unix()

	requestBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	Request := SendTxRequest{}
	err = json.Unmarshal(requestBody, &Request)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if ok, err := VerifyPassword(context.TODO(), db_types.User{Username: Request.Username}, Request.Password); err != nil || !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if Request.Tx.TxType == Transfer || Request.Tx.TxType == Spending {
		Request.Tx.From = &Account{Address: CalculatePublicKeyByUsername(Request.Username)}
	}
	Request.Tx.Timestamp = &timestamp

	errChan := make(chan error)

	SendTxChan <- SendTxBcRequest{Tx: Request.Tx, ResponseCh: errChan}

	select {
	case err := <-errChan:
		if err != nil {
			w.WriteHeader(http.StatusNotAcceptable)
			w.Write([]byte(err.Error()))
			return
		}
		w.Write([]byte("ok"))
	case <-time.After(10 * time.Second):
		w.WriteHeader(http.StatusProcessing)
		return
	}
}

func GetBalanceByBlockNumber(w http.ResponseWriter, r *http.Request) {
	requestBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	Request := GetBalanceRequest{
		ResponseCh: make(chan GetBalanceBCResponse),
	}
	err = json.Unmarshal(requestBody, &Request)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	GetBalanceChan <- Request
	Response := <-Request.ResponseCh
	if errors.Is(Response.Err, FutureBlockError) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	JSON, _ := json.Marshal(Response.Balance)

	w.Write(JSON)
}

func GetTransactionsWithFilters(w http.ResponseWriter, r *http.Request) {
	requestBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	Request := GetTransactionsWithFiltersRequest{ResponseCh: make(chan Transactions)}
	err = json.Unmarshal(requestBody, &Request)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	GetTxsWithFiltersCh <- Request

	resp := <-Request.ResponseCh
	JSON, _ := json.Marshal(resp)

	w.Write(JSON)
}
