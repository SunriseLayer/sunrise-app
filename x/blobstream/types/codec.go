package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	// this line is used by starport scaffolding # 1
)

func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgRegisterEvmAddress{}, "/sunrise.blobstream.v1.MsgRegisterEvmAddress", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "/sunrise.blobstream.v1.MsgUpdateParams", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgRegisterEvmAddress{},
		&MsgUpdateParams{},
	)
	// this line is used by starport scaffolding # 3

	registry.RegisterInterface(
		"AttestationRequestI",
		(*AttestationRequestI)(nil),
		&DataCommitment{},
		&Valset{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}
