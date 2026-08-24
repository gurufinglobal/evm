package keeper

import (
	"github.com/ethereum/go-ethereum/common"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// GetCoinbaseAddress converts the block proposer's validator operator address to an Ethereum address
// for use as block.coinbase in the EVM.
func (k Keeper) GetCoinbaseAddress(ctx sdk.Context, proposerAddress sdk.ConsAddress) (common.Address, error) {
	proposerAddress = GetProposerAddress(ctx, proposerAddress)
	if len(proposerAddress) == 0 {
		// The proposer can be absent in contexts such as CheckTx.
		return common.Address{}, nil
	}

	validator, err := k.stakingKeeper.GetValidatorByConsAddr(ctx, proposerAddress)
	if err != nil {
		return common.Address{}, errorsmod.Wrapf(
			stakingtypes.ErrNoValidatorFound,
			"failed to retrieve validator from block proposer address %s. Error: %s",
			proposerAddress.String(),
			err.Error(),
		)
	}

	operatorAddress := validator.GetOperator()
	operatorAddressBytes, err := k.stakingKeeper.ValidatorAddressCodec().StringToBytes(operatorAddress)
	if err != nil {
		return common.Address{}, errorsmod.Wrapf(
			err,
			"failed to convert validator operator address %s to bytes",
			operatorAddress,
		)
	}
	if len(operatorAddressBytes) != common.AddressLength {
		return common.Address{}, errorsmod.Wrapf(
			sdkerrors.ErrInvalidAddress,
			"validator operator address %s must decode to %d bytes, got %d",
			operatorAddress,
			common.AddressLength,
			len(operatorAddressBytes),
		)
	}

	return common.BytesToAddress(operatorAddressBytes), nil
}

// GetProposerAddress returns current block proposer's address when provided proposer address is empty.
func GetProposerAddress(ctx sdk.Context, proposerAddress sdk.ConsAddress) sdk.ConsAddress {
	if len(proposerAddress) == 0 {
		proposerAddress = ctx.BlockHeader().ProposerAddress
	}
	return proposerAddress
}
