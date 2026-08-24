package statedb

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestNewContractDeletionAuthorization(t *testing.T) {
	address := common.HexToAddress("0x1")
	otherAddress := common.HexToAddress("0x2")
	ctx := sdk.Context{}.WithContext(context.Background())

	require.False(t, IsNewContractDeletionAuthorized(ctx, address))
	require.False(t, IsNewContractDeletionAuthorized(sdk.Context{}, address))

	authorizedCtx := withNewContractDeletion(ctx, address)
	require.True(t, IsNewContractDeletionAuthorized(authorizedCtx, address))
	require.False(t, IsNewContractDeletionAuthorized(authorizedCtx, otherAddress))
	require.False(t, IsNewContractDeletionAuthorized(ctx, address))
}
