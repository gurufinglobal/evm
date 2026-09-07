package network

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	cmttypes "github.com/cometbft/cometbft/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func TestInitGenFilesReplacesSupplyAndPreservesMetadata(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NumValidators = 1
	var bankGenesis banktypes.GenesisState
	cfg.Codec.MustUnmarshalJSON(cfg.GenesisState[banktypes.ModuleName], &bankGenesis)
	metadata := bankGenesis.DenomMetadata
	require.NotEmpty(t, metadata)
	address := sdk.AccAddress(make([]byte, 20)).String()
	bankGenesis.Balances = []banktypes.Balance{{Address: address, Coins: sdk.NewCoins(sdk.NewInt64Coin(cfg.BondDenom, 100))}}
	bankGenesis.Supply = bankGenesis.Balances[0].Coins
	cfg.GenesisState[banktypes.ModuleName] = cfg.Codec.MustMarshalJSON(&bankGenesis)

	balances := []banktypes.Balance{{Address: address, Coins: sdk.NewCoins(sdk.NewInt64Coin(cfg.BondDenom, 5))}}
	genesisFile := filepath.Join(t.TempDir(), "genesis.json")
	require.NoError(t, initGenFiles(cfg, nil, balances, []string{genesisFile}))
	doc, err := cmttypes.GenesisDocFromFile(genesisFile)
	require.NoError(t, err)
	var state map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(doc.AppState, &state))
	cfg.Codec.MustUnmarshalJSON(state[banktypes.ModuleName], &bankGenesis)
	require.Equal(t, balances, bankGenesis.Balances)
	require.Empty(t, bankGenesis.Supply)
	require.Equal(t, metadata, bankGenesis.DenomMetadata)
	require.NoError(t, bankGenesis.Validate())
}
