package keeper

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/ibc/callbacks/types"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	callbacktypes "github.com/cosmos/ibc-go/v10/modules/apps/callbacks/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type gasMeteringEVMKeeper struct {
	types.EVMKeeper
	gasUsed uint64
}

func (gasMeteringEVMKeeper) IsContract(sdk.Context, common.Address) bool {
	return true
}

func (k gasMeteringEVMKeeper) CallEVM(
	ctx sdk.Context,
	_ *statedb.StateDB,
	_ abi.ABI,
	_, _ common.Address,
	_, _ bool,
	_ *big.Int,
	_ string,
	_ ...interface{},
) (*evmtypes.MsgEthereumTxResponse, error) {
	ctx.GasMeter().ConsumeGas(k.gasUsed, "test EVM execution")
	return &evmtypes.MsgEthereumTxResponse{GasUsed: k.gasUsed}, nil
}

func ensureBech32Config(t *testing.T) {
	t.Helper()
	cfg := sdk.GetConfig()
	if cfg.GetBech32AccountAddrPrefix() != "" {
		return
	}
	cfg.SetBech32PrefixForAccount("cosmos", "cosmospub")
	cfg.SetBech32PrefixForValidator("cosmosvaloper", "cosmosvaloperpub")
	cfg.SetBech32PrefixForConsensusNode("cosmosvalcons", "cosmosvalconspub")
	cfg.Seal()
}

func assertRejectsMismatchedContractSender(
	t *testing.T,
	senderEth common.Address,
	contractEth common.Address,
	seq uint64,
	invoke func(ctx sdk.Context, packet channeltypes.Packet, contractHex string, senderBech32 string) error,
) {
	t.Helper()
	ensureBech32Config(t)
	storeKey := storetypes.NewKVStoreKey("test")
	tKey := storetypes.NewTransientStoreKey("test_t")
	ctx := sdktestutil.DefaultContext(storeKey, tKey)
	ctx = ctx.WithLogger(log.NewNopLogger())
	ctx = ctx.WithGasMeter(storetypes.NewGasMeter(10_000_000))
	port := "transfer"
	srcChan := "channel-0"
	destChan := "channel-1"
	gasLimit := "1000000"
	timeoutNs := uint64(0)
	senderBech32 := sdk.AccAddress(senderEth.Bytes()).String()
	memoBz, err := json.Marshal(map[string]any{
		callbacktypes.SourceCallbackKey: map[string]string{
			"address":   contractEth.Hex(),
			"gas_limit": gasLimit,
		},
	})
	require.NoError(t, err)
	memo := string(memoBz)
	packetData := transfertypes.NewFungibleTokenPacketData(
		"stake",
		"1",
		senderBech32,
		senderBech32,
		memo,
	)
	packetDataBz, err := transfertypes.MarshalPacketData(packetData, transfertypes.V1, transfertypes.EncodingJSON)
	require.NoError(t, err)

	packet := channeltypes.NewPacket(
		packetDataBz,
		seq,
		port,
		srcChan,
		port,
		destChan,
		clienttypes.NewHeight(0, 100),
		timeoutNs,
	)

	err = invoke(ctx, packet, contractEth.Hex(), senderBech32)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrCallbackFailed)
	require.ErrorContains(t, err, "source callback contract must match packet sender")
}

func TestIBCOnAcknowledgementPacketCallback_RejectsMismatchedContractSender(t *testing.T) {
	senderEth := common.HexToAddress("0x2222222222222222222222222222222222222222")
	contractEth := common.HexToAddress("0x1111111111111111111111111111111111111111")
	assertRejectsMismatchedContractSender(t, senderEth, contractEth, 1, func(ctx sdk.Context, packet channeltypes.Packet, contractHex string, senderBech32 string) error {
		k := ContractKeeper{}
		return k.IBCOnAcknowledgementPacketCallback(
			ctx,
			packet,
			[]byte("ack"),
			sdk.AccAddress{},
			contractHex,
			senderBech32,
			transfertypes.V1,
		)
	})
}

func TestIBCOnTimeoutPacketCallback_RejectsMismatchedContractSender(t *testing.T) {
	senderEth := common.HexToAddress("0x3333333333333333333333333333333333333333")
	contractEth := common.HexToAddress("0x4444444444444444444444444444444444444444")
	assertRejectsMismatchedContractSender(t, senderEth, contractEth, 7, func(ctx sdk.Context, packet channeltypes.Packet, contractHex string, senderBech32 string) error {
		k := ContractKeeper{}
		return k.IBCOnTimeoutPacketCallback(
			ctx,
			packet,
			sdk.AccAddress{},
			contractHex,
			senderBech32,
			transfertypes.V1,
		)
	})
}

func TestIBCOnTimeoutPacketCallback_ChargesEVMGasOnce(t *testing.T) {
	ensureBech32Config(t)
	storeKey := storetypes.NewKVStoreKey("test")
	tKey := storetypes.NewTransientStoreKey("test_t")
	ctx := sdktestutil.DefaultContext(storeKey, tKey)
	ctx = ctx.WithLogger(log.NewNopLogger())
	ctx = ctx.WithGasMeter(storetypes.NewGasMeter(10_000_000))

	const evmGasUsed uint64 = 25_000
	sender := common.HexToAddress("0x5555555555555555555555555555555555555555")
	senderBech32 := sdk.AccAddress(sender.Bytes()).String()
	memoBz, err := json.Marshal(map[string]any{
		callbacktypes.SourceCallbackKey: map[string]string{
			"address":   sender.Hex(),
			"gas_limit": "1000000",
		},
	})
	require.NoError(t, err)

	packetData := transfertypes.NewFungibleTokenPacketData(
		"stake",
		"1",
		senderBech32,
		senderBech32,
		string(memoBz),
	)
	packetDataBz, err := transfertypes.MarshalPacketData(packetData, transfertypes.V1, transfertypes.EncodingJSON)
	require.NoError(t, err)
	packet := channeltypes.NewPacket(
		packetDataBz,
		1,
		transfertypes.PortID,
		"channel-0",
		transfertypes.PortID,
		"channel-1",
		clienttypes.NewHeight(0, 100),
		0,
	)

	k := NewKeeper(nil, gasMeteringEVMKeeper{gasUsed: evmGasUsed}, nil)
	gasBefore := ctx.GasMeter().GasConsumed()
	err = k.IBCOnTimeoutPacketCallback(
		ctx,
		packet,
		sdk.AccAddress{},
		sender.Hex(),
		senderBech32,
		transfertypes.V1,
	)
	require.NoError(t, err)
	require.Equal(t, evmGasUsed, ctx.GasMeter().GasConsumed()-gasBefore)
}
