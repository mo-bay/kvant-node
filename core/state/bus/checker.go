package bus

import (
	"github.com/kvant-node/core/types"
	"math/big"
)

type Checker interface {
	AddCoin(types.CoinSymbol, *big.Int, ...string)
	AddCoinVolume(types.CoinSymbol, *big.Int)
}
