package mempool

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func removalTestContext() (sdk.Context, *storetypes.KVStoreKey) {
	key := storetypes.NewKVStoreKey("removal")
	ctx := testutil.DefaultContext(key, storetypes.NewTransientStoreKey("removal_transient"))
	ctx, _ = ctx.CacheContext()
	ctx = ctx.WithBlockHeader(cmtproto.Header{Height: 1, AppHash: make([]byte, common.HashLength)}).
		WithGasMeter(storetypes.NewGasMeter(1_000_000)).
		WithBlockGasMeter(storetypes.NewGasMeter(2_000_000))
	ctx.MultiStore().GetKVStore(key).Set([]byte("nonce"), []byte("4"))
	ctx.MultiStore().GetKVStore(key).Set([]byte("balance"), []byte("100"))
	ctx.EventManager().EmitEvent(sdk.NewEvent("parent"))
	ctx.GasMeter().ConsumeGas(7, "parent")
	ctx.BlockGasMeter().ConsumeGas(11, "parent")
	return ctx, key
}

func TestRemovalValidationIsolated(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		remove bool
	}{
		{name: "valid"},
		{name: "nonce gap", err: fmt.Errorf("wrapped: %w", ErrNonceGap)},
		{name: "invalid sequence", err: fmt.Errorf("wrapped: %w", sdkerrors.ErrInvalidSequence)},
		{name: "out of gas", err: fmt.Errorf("wrapped: %w", sdkerrors.ErrOutOfGas)},
		{name: "other failure", err: errors.New("invalid transaction"), remove: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, key := removalTestContext()
			pool := &ExperimentalEVMMempool{
				logger:     log.NewNopLogger(),
				blockchain: &Blockchain{latestCtx: ctx, logger: log.NewNopLogger()},
				anteHandler: func(validation sdk.Context, _ sdk.Tx, simulate bool) (sdk.Context, error) {
					require.True(t, simulate)
					require.Equal(t, ctx.GasMeter().Limit(), validation.GasMeter().Limit())
					require.Equal(t, ctx.BlockGasMeter().Limit(), validation.BlockGasMeter().Limit())
					require.Equal(t, []byte("4"), validation.KVStore(key).Get([]byte("nonce")))
					validation.KVStore(key).Set([]byte("nonce"), []byte("5"))
					validation.KVStore(key).Set([]byte("balance"), []byte("90"))
					validation.EventManager().EmitEvent(sdk.NewEvent("validation"))
					validation.GasMeter().ConsumeGas(13, "validation")
					validation.BlockGasMeter().ConsumeGas(17, "validation")
					return validation, tc.err
				},
			}
			for range 2 {
				require.Equal(t, tc.remove, pool.shouldRemoveFromEVMPool(nil))
				require.Equal(t, []byte("4"), ctx.MultiStore().GetKVStore(key).Get([]byte("nonce")))
				require.Equal(t, []byte("100"), ctx.MultiStore().GetKVStore(key).Get([]byte("balance")))
				require.Equal(t, sdk.Events{sdk.NewEvent("parent")}, ctx.EventManager().Events())
				require.Equal(t, uint64(7), ctx.GasMeter().GasConsumed())
				require.Equal(t, uint64(11), ctx.BlockGasMeter().GasConsumed())
			}
		})
	}
}

func TestRemoveConcurrentStateAt(t *testing.T) {
	ctx, key := removalTestContext()
	blockchain := &Blockchain{latestCtx: ctx, logger: log.NewNopLogger()}
	pool := &ExperimentalEVMMempool{
		logger:     log.NewNopLogger(),
		blockchain: blockchain,
		anteHandler: func(validation sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
			validation.KVStore(key).Set([]byte("nonce"), []byte("5"))
			validation.EventManager().EmitEvent(sdk.NewEvent("validation"))
			validation.BlockGasMeter().ConsumeGas(1, "validation")
			return validation, nil
		},
	}
	tx := evmtypes.NewTx(&evmtypes.EvmTxArgs{
		ChainID: big.NewInt(9001), GasLimit: 21000, GasPrice: big.NewInt(1),
	})
	const iterations = 200
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for range iterations {
			if err := pool.Remove(tx); err != nil {
				errs <- err
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := range iterations {
			blockchain.setLatestContext(ctx.WithBlockHeight(int64(i + 1)))
			if _, err := blockchain.StateAt(common.HexToHash("0x01")); err != nil {
				errs <- err
				return
			}
		}
	}()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, []byte("4"), ctx.MultiStore().GetKVStore(key).Get([]byte("nonce")))
	require.Equal(t, sdk.Events{sdk.NewEvent("parent")}, ctx.EventManager().Events())
	require.Equal(t, uint64(7), ctx.GasMeter().GasConsumed())
	require.Equal(t, uint64(11), ctx.BlockGasMeter().GasConsumed())
}
