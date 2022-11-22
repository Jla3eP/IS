package main

import (
	"IS/blockchain/blockchain/api"
	"IS/blockchain/config"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"time"
)

func main() {
	cfg, err := config.ArgsToConfig(os.Args)
	if err != nil {
		fmt.Println(err)
		return
	}
	JSON, _ := json.Marshal(cfg)
	fmt.Println("Config: ", string(JSON))

	api.InitDB(cfg)
	runServer(cfg)
}

func runServer(cfg *config.Config) {
	router := mux.NewRouter()
	handleFuncs(router)

	server := &http.Server{
		Addr:    cfg.HttpAddress + ":" + cfg.HttpPort,
		Handler: router,
	}
	stopCh := make(chan struct{})
	go func(stopCh chan struct{}) {
		time.Sleep(time.Second)
		select {
		case <-stopCh:
			fmt.Println("failed to run server")
		default:
			fmt.Println("server is running")
		}
	}(stopCh)
	api.Start()
	err := server.ListenAndServe()
	stopCh <- struct{}{}
	if err != nil {
		fmt.Println(err)
	}
}

func handleFuncs(router *mux.Router) {
	router.HandleFunc("/register", api.CreateUserReq).Methods(http.MethodPost)
	router.HandleFunc("/sendTx", api.SendTx).Methods(http.MethodPost)
	router.HandleFunc("/getKey", api.GetPublicKeyByUsername).Methods(http.MethodGet)
	router.HandleFunc("/getBalance", api.GetBalanceByBlockNumber).Methods(http.MethodGet)
	router.HandleFunc("/getTxsWithFilters", api.GetTransactionsWithFilters).Methods(http.MethodGet)
}
