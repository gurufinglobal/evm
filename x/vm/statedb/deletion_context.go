package statedb

import (
	"context"

	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type newContractDeletion struct {
	address common.Address
}

func withNewContractDeletion(ctx sdk.Context, address common.Address) sdk.Context {
	parent := ctx.Context()
	if parent == nil {
		parent = context.Background()
	}
	return ctx.WithContext(context.WithValue(parent, newContractDeletion{}, newContractDeletion{address: address}))
}

// IsNewContractDeletionAuthorized reports whether StateDB authorized deleting
// address as a contract created and self-destructed in the current transaction.
func IsNewContractDeletionAuthorized(ctx sdk.Context, address common.Address) bool {
	if ctx.Context() == nil {
		return false
	}
	authorization, ok := ctx.Value(newContractDeletion{}).(newContractDeletion)
	return ok && authorization.address == address
}
