package utils

import (
	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func ValidatorConsAddressToHex(valAddress string) common.Address {
	if common.IsHexAddress(valAddress) {
		return common.HexToAddress(valAddress)
	}

	valAddr, err := sdk.ValAddressFromBech32(valAddress)
	if err != nil || len(valAddr) != common.AddressLength {
		return common.Address{}
	}
	return common.BytesToAddress(valAddr)
}
