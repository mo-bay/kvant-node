package api

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"minter/core/types"
	"net/http"
)

type TransactionCountResponse uint64

func GetTransactionCount(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	address := types.HexToAddress(vars["address"])

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(Response{
		Code:   0,
		Result: blockchain.CurrentState().GetNonce(address),
	})
}