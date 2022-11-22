package api

import (
	hash "IS/blockchain/database_utils/hashing"
	db_types "IS/blockchain/database_utils/types"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
)

func getLoginDataFromRequest(w http.ResponseWriter, r *http.Request) (*db_types.LoginData, error) {
	requestBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error() + "\n"))
		return nil, errors.New("invalid request body")
	}

	usr := &db_types.LoginData{}

	if err = json.Unmarshal(requestBody, usr); err != nil {
		w.WriteHeader(http.StatusBadRequest)

		log.Print("Invalid authUsingSessionKey json")
		return nil, errors.New("invalid authUsingSessionKey json")
	}

	return usr, nil
}

func CalculatePublicKeyByUsername(username string) Address {
	return hash.HashUsername(username)
}
