package desmos

// nolint
// autogenerated code using github.com/haasted/alias-generator.
// based on functionality in github.com/rigelrozanski/multitool

import (
	"github.com/bitsongofficial/go-bitsong/x/ibc/desmos/keeper"
	"github.com/bitsongofficial/go-bitsong/x/ibc/desmos/types"
)

const (
	ModuleName                    = types.ModuleName
	Version                       = types.Version
	PortID                        = types.PortID
	StoreKey                      = types.StoreKey
	RouterKey                     = types.RouterKey
	PortKey                       = types.PortKey
	QuerierRoute                  = types.QuerierRoute
	DesmosBitsongSubspace         = types.DesmosBitsongSubspace
	DesmosSongIDAttribute         = types.DesmosSongIDAttribute
	DefaultPacketTimeout          = keeper.DefaultPacketTimeout
	DefaultPacketTimeoutTimestamp = keeper.DefaultPacketTimeoutTimestamp
)

var (
	// functions aliases
	NewKeeper            = keeper.NewKeeper
	RegisterCodec        = types.RegisterCodec
	DefaultGenesis       = types.DefaultGenesis
	NewMsgCreateSongPost = types.NewMsgCreateSongPost
	NewSongCreationData  = types.NewSongCreationData

	// variable aliases
	ModuleCdc = types.ModuleCdc
)

type (
	Keeper            = keeper.Keeper
	GenesisState      = types.GenesisState
	MsgCreateSongPost = types.MsgCreateSongPost
)
