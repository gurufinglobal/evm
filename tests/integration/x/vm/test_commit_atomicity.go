package vm

import (
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/x/vm/statedb"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// commitErrorKeeper injects a late failure while retaining the real application
// stores, including auth account allocation, bank supply and emitted events.
type commitErrorKeeper struct {
	statedb.Keeper
	failAddress common.Address
	failure     error
}

func (k commitErrorKeeper) SetAccount(ctx sdk.Context, addr common.Address, account statedb.Account) error {
	if addr == k.failAddress {
		return k.failure
	}
	return k.Keeper.SetAccount(ctx, addr, account)
}

func (s *KeeperTestSuite) TestCommitAtomicityWithSameTxSelfDestruct() {
	for _, useCache := range []bool{false, true} {
		for _, emptyRuntime := range []bool{false, true} {
			for _, failCommit := range []bool{false, true} {
				s.Run(fmt.Sprintf("cache=%t/empty-runtime=%t/failure=%t", useCache, emptyRuntime, failCommit), func() {
					s.SetupTest()
					ctx := s.Network.GetContext()
					evmKeeper := s.Network.App.GetEVMKeeper()
					accountKeeper := s.Network.App.GetAccountKeeper()
					bankKeeper := s.Network.App.GetBankKeeper()

					// Commit order is deletion, beneficiary credit, then the error.
					contract := common.BigToAddress(big.NewInt(100))
					beneficiary := common.BigToAddress(big.NewInt(150))
					blocked := common.BigToAddress(big.NewInt(200))
					code := []byte{0x60, 0x42}
					if emptyRuntime {
						code = nil
					}
					codeHash := crypto.Keccak256Hash(code)
					slot := common.HexToHash("0x01")
					value := common.HexToHash("0x02")

					// A pre-funded account exists before the creation transaction.
					seed := s.StateDB()
					seed.AddBalance(contract, uint256.NewInt(50), tracing.BalanceChangeUnspecified)
					s.Require().NoError(seed.Commit())
					accountBefore := evmKeeper.GetAccount(ctx, contract)
					supplyBefore := bankKeeper.GetSupply(ctx, s.EvmDenom())
					numberBefore, err := accountKeeper.AccountNumber.Peek(ctx)
					s.Require().NoError(err)
					eventsBefore := slices.Clone(ctx.EventManager().Events())

					failure := errors.New("injected late account write failure")
					keeper := commitErrorKeeper{Keeper: evmKeeper, failure: failure}
					if failCommit {
						keeper.failAddress = blocked
					}
					db := statedb.New(ctx, keeper, statedb.NewEmptyTxConfig())
					db.SetCode(contract, code)
					db.SetState(contract, slot, value)
					db.CreateContract(contract)
					if useCache {
						cacheCtx, err := db.GetCacheContext()
						s.Require().NoError(err)
						s.Require().NoError(db.FlushToCacheCtx())
						s.Require().Equal(value, evmKeeper.GetState(cacheCtx, contract, slot))
						s.Require().Equal(code, evmKeeper.GetCode(cacheCtx, codeHash))
					}
					balance, destructed := db.SelfDestruct6780(contract)
					s.Require().True(destructed)
					db.AddBalance(beneficiary, &balance, tracing.BalanceChangeUnspecified)
					db.SetNonce(blocked, 1, tracing.NonceChangeUnspecified)

					err = db.Commit()
					if failCommit {
						s.Require().ErrorIs(err, failure)
						s.Require().Equal(accountBefore, evmKeeper.GetAccount(ctx, contract))
						s.Require().Nil(evmKeeper.GetAccount(ctx, beneficiary))
						s.Require().Nil(evmKeeper.GetAccount(ctx, blocked))
						numberAfter, err := accountKeeper.AccountNumber.Peek(ctx)
						s.Require().NoError(err)
						s.Require().Equal(numberBefore, numberAfter)
						s.Require().Equal(eventsBefore, ctx.EventManager().Events())
					} else {
						s.Require().NoError(err)
						s.Require().Nil(evmKeeper.GetAccount(ctx, contract))
						s.Require().Equal(uint256.NewInt(50), evmKeeper.GetAccount(ctx, beneficiary).Balance)
						s.Require().Equal(uint64(1), evmKeeper.GetAccount(ctx, blocked).Nonce)
					}
					s.Require().Equal(supplyBefore, bankKeeper.GetSupply(ctx, s.EvmDenom()))
					s.Require().Equal(common.Hash{}, evmKeeper.GetState(ctx, contract, slot))
					s.Require().Empty(evmKeeper.GetCode(ctx, codeHash))
				})
			}
		}
	}
}
