package evmd_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/evmd"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/server"
	simutils "github.com/cosmos/cosmos-sdk/testutil/sims"
)

func TestCloseWithoutEVMMempool(t *testing.T) {
	app := evmd.NewExampleApp(log.NewNopLogger(), dbm.NewMemDB(), nil, true, simutils.AppOptionsMap{
		flags.FlagHome:           t.TempDir(),
		server.FlagMempoolMaxTxs: -1,
	})
	require.Nil(t, app.EVMMempool)
	require.NoError(t, app.Close())
}
