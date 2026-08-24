package keeper_test

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/mock"

	evmaddress "github.com/cosmos/evm/encoding/address"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (suite *KeeperTestSuite) TestGetCoinbaseAddress() {
	proposerConsAddr := sdk.ConsAddress([]byte("proposer"))
	headerProposerConsAddr := sdk.ConsAddress([]byte("header_proposer"))
	valAddr := sdk.ValAddress([]byte("test_validator_addr1"))
	validatorOperator := valAddr.String()
	validatorAddressCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ValidatorAddrPrefix())
	suite.stakingKeeper.On("ValidatorAddressCodec").Return(validatorAddressCodec)

	header := suite.ctx.BlockHeader()
	header.ProposerAddress = headerProposerConsAddr
	ctxWithHeaderProposer := suite.ctx.WithBlockHeader(header)

	testCases := []struct {
		name         string
		ctx          sdk.Context
		proposerAddr sdk.ConsAddress
		malleate     func()
		expectedAddr common.Address
		expectedErr  string
	}{
		{
			name:         "explicit proposer takes precedence",
			ctx:          ctxWithHeaderProposer,
			proposerAddr: proposerConsAddr,
			malleate: func() {
				validator := stakingtypes.Validator{OperatorAddress: validatorOperator}
				suite.stakingKeeper.On("GetValidatorByConsAddr", mock.Anything, proposerConsAddr).
					Return(validator, nil).Once()
			},
			expectedAddr: common.BytesToAddress(valAddr.Bytes()),
		},
		{
			name: "empty argument falls back to header proposer",
			ctx:  ctxWithHeaderProposer,
			malleate: func() {
				validator := stakingtypes.Validator{OperatorAddress: validatorOperator}
				suite.stakingKeeper.On("GetValidatorByConsAddr", mock.Anything, headerProposerConsAddr).
					Return(validator, nil).Once()
			},
			expectedAddr: common.BytesToAddress(valAddr.Bytes()),
		},
		{
			name:         "hex validator operator is supported",
			ctx:          suite.ctx,
			proposerAddr: proposerConsAddr,
			malleate: func() {
				validator := stakingtypes.Validator{OperatorAddress: common.BytesToAddress(valAddr).Hex()}
				suite.stakingKeeper.On("GetValidatorByConsAddr", mock.Anything, proposerConsAddr).
					Return(validator, nil).Once()
			},
			expectedAddr: common.BytesToAddress(valAddr.Bytes()),
		},
		{
			name:         "validator not found returns error",
			ctx:          suite.ctx,
			proposerAddr: proposerConsAddr,
			malleate: func() {
				suite.stakingKeeper.On("GetValidatorByConsAddr", mock.Anything, proposerConsAddr).
					Return(stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound).Once()
			},
			expectedErr: "failed to retrieve validator",
		},
		{
			name:         "malformed validator operator returns error",
			ctx:          suite.ctx,
			proposerAddr: proposerConsAddr,
			malleate: func() {
				validator := stakingtypes.Validator{OperatorAddress: "not-bech32"}
				suite.stakingKeeper.On("GetValidatorByConsAddr", mock.Anything, proposerConsAddr).
					Return(validator, nil).Once()
			},
			expectedErr: "failed to convert validator operator address",
		},
		{
			name:         "wrong prefix validator operator returns error",
			ctx:          suite.ctx,
			proposerAddr: proposerConsAddr,
			malleate: func() {
				validator := stakingtypes.Validator{OperatorAddress: sdk.AccAddress(valAddr).String()}
				suite.stakingKeeper.On("GetValidatorByConsAddr", mock.Anything, proposerConsAddr).
					Return(validator, nil).Once()
			},
			expectedErr: "failed to convert validator operator address",
		},
		{
			name:         "short validator operator returns error",
			ctx:          suite.ctx,
			proposerAddr: proposerConsAddr,
			malleate: func() {
				validator := stakingtypes.Validator{OperatorAddress: sdk.ValAddress(make([]byte, common.AddressLength-1)).String()}
				suite.stakingKeeper.On("GetValidatorByConsAddr", mock.Anything, proposerConsAddr).
					Return(validator, nil).Once()
			},
			expectedErr: "must decode to 20 bytes",
		},
		{
			name:         "long validator operator returns error",
			ctx:          suite.ctx,
			proposerAddr: proposerConsAddr,
			malleate: func() {
				validator := stakingtypes.Validator{OperatorAddress: sdk.ValAddress(make([]byte, common.AddressLength+1)).String()}
				suite.stakingKeeper.On("GetValidatorByConsAddr", mock.Anything, proposerConsAddr).
					Return(validator, nil).Once()
			},
			expectedErr: "must decode to 20 bytes",
		},
		{
			name:         "empty proposer returns zero address",
			ctx:          suite.ctx,
			malleate:     func() {},
			expectedAddr: common.Address{},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.malleate()
			addr, err := suite.vmKeeper.GetCoinbaseAddress(tc.ctx, tc.proposerAddr)
			if tc.expectedErr != "" {
				suite.Require().ErrorContains(err, tc.expectedErr)
				suite.Require().Equal(common.Address{}, addr)
				return
			}

			suite.Require().NoError(err)
			suite.Require().Equal(tc.expectedAddr, addr)
		})
	}
}
