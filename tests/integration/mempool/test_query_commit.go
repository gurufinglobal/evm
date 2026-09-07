package mempool

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "cosmossdk.io/store/types"
)

// TestQuerySnapshotWhileCommitting exercises the immutable IAVL snapshot used
// by mempool state reads while the application commits newer versions.
func (s *IntegrationTestSuite) TestQuerySnapshotWhileCommitting() {
	ctx, err := s.network.App.GetBaseApp().CreateQueryContext(0, false)
	s.Require().NoError(err)
	ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	keeper := s.network.App.GetEVMKeeper()
	done := make(chan struct{})
	ready := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(ready)
		for i := uint64(1); ; i++ {
			select {
			case <-done:
				result <- nil
				return
			default:
			}
			// Vary absent keys so the query cache cannot mask IAVL reads.
			addr := common.Address{0x7f}
			binary.BigEndian.PutUint64(addr[12:], i)
			if hash := keeper.GetCodeHash(ctx, addr); hash != common.BytesToHash(evmtypes.EmptyCodeHash) {
				result <- fmt.Errorf("absent contract returned code hash %s", hash)
				return
			}
		}
	}()
	<-ready
	for range 50 {
		if err = s.network.NextBlock(); err != nil {
			break
		}
	}
	close(done)
	s.Require().NoError(<-result)
	s.Require().NoError(err)
}
