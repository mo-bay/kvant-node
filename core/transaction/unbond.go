package transaction

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/kvant-node/core/code"
	"github.com/kvant-node/core/commissions"
	"github.com/kvant-node/core/state"
	"github.com/kvant-node/core/types"
	"github.com/kvant-node/formula"
	"github.com/kvant-node/hexutil"
	"github.com/tendermint/tendermint/libs/kv"
	"math/big"
)

const unbondPeriod = 518400

type UnbondData struct {
	PubKey types.Pubkey
	Coin   types.CoinSymbol
	Value  *big.Int
}

func (data UnbondData) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		PubKey string `json:"pub_key"`
		Coin   string `json:"coin"`
		Value  string `json:"value"`
	}{
		PubKey: data.PubKey.String(),
		Coin:   data.Coin.String(),
		Value:  data.Value.String(),
	})
}

func (data UnbondData) TotalSpend(tx *Transaction, context *state.State) (TotalSpends, []Conversion, *big.Int, *Response) {
	panic("implement me")
}

func (data UnbondData) BasicCheck(tx *Transaction, context *state.State) *Response {
	if data.Value == nil {
		return &Response{
			Code: code.DecodeError,
			Log:  "Incorrect tx data"}
	}

	if !context.Coins.Exists(data.Coin) {
		return &Response{
			Code: code.CoinNotExists,
			Log:  fmt.Sprintf("Coin %s not exists", data.Coin),
			Info: EncodeError(map[string]string{
				"coin": fmt.Sprintf("%s", data.Coin),
			}),
		}
	}

	if !context.Candidates.Exists(data.PubKey) {
		return &Response{
			Code: code.CandidateNotFound,
			Log:  fmt.Sprintf("Candidate with such public key not found"),
			Info: EncodeError(map[string]string{
				"pub_key": data.PubKey.String(),
			}),
		}
	}

	sender, _ := tx.Sender()
	stake := context.Candidates.GetStakeValueOfAddress(data.PubKey, sender, data.Coin)

	if stake == nil {
		return &Response{
			Code: code.StakeNotFound,
			Log:  fmt.Sprintf("Stake of current user not found")}
	}

	if stake.Cmp(data.Value) < 0 {
		return &Response{
			Code: code.InsufficientStake,
			Log:  fmt.Sprintf("Insufficient stake for sender account"),
			Info: EncodeError(map[string]string{
				"pub_key": data.PubKey.String(),
			})}
	}

	return nil
}

func (data UnbondData) String() string {
	return fmt.Sprintf("UNBOND pubkey:%s",
		hexutil.Encode(data.PubKey[:]))
}

func (data UnbondData) Gas() int64 {
	return commissions.UnbondTx
}

func (data UnbondData) Run(tx *Transaction, context *state.State, isCheck bool, rewardPool *big.Int, currentBlock uint64) Response {
	sender, _ := tx.Sender()

	response := data.BasicCheck(tx, context)
	if response != nil {
		return *response
	}

	commissionInBaseCoin := tx.CommissionInBaseCoin()
	commission := big.NewInt(0).Set(commissionInBaseCoin)

	if !tx.GasCoin.IsBaseCoin() {
		coin := context.Coins.GetCoin(tx.GasCoin)

		errResp := CheckReserveUnderflow(coin, commissionInBaseCoin)
		if errResp != nil {
			return *errResp
		}

		if coin.Reserve().Cmp(commissionInBaseCoin) < 0 {
			return Response{
				Code: code.CoinReserveNotSufficient,
				Log:  fmt.Sprintf("Coin reserve balance is not sufficient for transaction. Has: %s, required %s", coin.Reserve().String(), commissionInBaseCoin.String()),
				Info: EncodeError(map[string]string{
					"has_reserve": coin.Reserve().String(),
					"commission":  commissionInBaseCoin.String(),
					"gas_coin":    coin.CName,
				}),
			}
		}

		commission = formula.CalculateSaleAmount(coin.Volume(), coin.Reserve(), coin.Crr(), commissionInBaseCoin)
	}

	if context.Accounts.GetBalance(sender, tx.GasCoin).Cmp(commission) < 0 {
		return Response{
			Code: code.InsufficientFunds,
			Log:  fmt.Sprintf("Insufficient funds for sender account: %s. Wanted %s %s", sender.String(), commission, tx.GasCoin),
			Info: EncodeError(map[string]string{
				"sender":       sender.String(),
				"needed_value": commission.String(),
				"gas_coin":     fmt.Sprintf("%s", tx.GasCoin),
			}),
		}
	}

	if !isCheck {
		// now + 30 days
		unbondAtBlock := currentBlock + unbondPeriod

		rewardPool.Add(rewardPool, commissionInBaseCoin)

		context.Coins.SubReserve(tx.GasCoin, commissionInBaseCoin)
		context.Coins.SubVolume(tx.GasCoin, commission)

		context.Accounts.SubBalance(sender, tx.GasCoin, commission)
		context.Candidates.SubStake(sender, data.PubKey, data.Coin, data.Value)
		context.FrozenFunds.AddFund(unbondAtBlock, sender, data.PubKey, data.Coin, data.Value)
		context.Accounts.SetNonce(sender, tx.Nonce)
	}

	tags := kv.Pairs{
		kv.Pair{Key: []byte("tx.type"), Value: []byte(hex.EncodeToString([]byte{byte(TypeUnbond)}))},
		kv.Pair{Key: []byte("tx.from"), Value: []byte(hex.EncodeToString(sender[:]))},
	}

	return Response{
		Code:      code.OK,
		GasUsed:   tx.Gas(),
		GasWanted: tx.Gas(),
		Tags:      tags,
	}
}
