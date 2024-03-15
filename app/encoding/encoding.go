package encoding

import (
	"cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/gogoproto/proto"
)

type ModuleRegister interface {
	RegisterLegacyAminoCodec(*codec.LegacyAmino)
	RegisterInterfaces(codectypes.InterfaceRegistry)
}

// Config specifies the concrete encoding types to use for a given app.
// This is provided for compatibility between protobuf and amino implementations.
type Config struct {
	InterfaceRegistry codectypes.InterfaceRegistry
	Codec             codec.Codec
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}

// MakeConfig creates an encoding config for the app.
func MakeConfig(regs ...ModuleRegister) Config {
	// create the codec
	amino := codec.NewLegacyAmino()
	interfaceRegistry, _ := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec:          address.NewBech32Codec("sunrise"),
			ValidatorAddressCodec: address.NewBech32Codec("sunrise" + "valoper"),
		},
	})

	// register the standard types from the sdk
	std.RegisterLegacyAminoCodec(amino)
	std.RegisterInterfaces(interfaceRegistry)

	// register specific modules
	for _, reg := range regs {
		reg.RegisterInterfaces(interfaceRegistry)
	}

	// create the final configuration
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txCfg := tx.NewTxConfig(marshaler, tx.DefaultSignModes)

	dec := txCfg.TxDecoder()
	dec = IndexWrapperDecoder(dec)

	txCfg, _ = tx.NewTxConfigWithOptions(marshaler, tx.ConfigOptions{
		ProtoDecoder: dec,
	})

	return Config{
		InterfaceRegistry: interfaceRegistry,
		Codec:             marshaler,
		TxConfig:          txCfg,
		Amino:             amino,
	}
}
