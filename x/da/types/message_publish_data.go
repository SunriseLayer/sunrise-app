package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgPublishData{}

func NewMsgPublishData(sender string) *MsgPublishData {
  return &MsgPublishData{
		Sender: sender,
	}
}

func (msg *MsgPublishData) ValidateBasic() error {
  _, err := sdk.AccAddressFromBech32(msg.Sender)
  	if err != nil {
  		return errorsmod.Wrapf(sdkerrors.ErrInvalidAddress, "invalid sender address (%s)", err)
  	}
  return nil
}

