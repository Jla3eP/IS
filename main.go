package main

import (
	"IS/blockchain/blockchain/api"
	"IS/blockchain/config"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
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
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()
	router.Use(gin.Recovery())
	router.Use(errorsMiddleware())
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

func handleFuncs(router *gin.Engine) {
	router.POST("/register", api.CreateUserReq)
	router.POST("/sendTx", api.SendTx)
	router.GET("/getKey", api.GetPublicKeyByUsername)
	router.GET("/getBalance", api.GetBalanceByBlockNumber)
	router.GET("/getTxsWithFilters", api.GetTransactionsWithFilters)
}

func errorsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		var nilErr *gin.Error = nil
		err := c.Errors.Last()
		if err == nilErr {
			return
		}

		errCode := err.Error()

		c.JSON(http.StatusInternalServerError, gin.H{
			"result":   "error",
			"err_code": errCode,
		})

	}
}
