package main

import (
	"bytes"
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/histolabs/metro/app"
	"github.com/histolabs/metro/app/encoding"
	"github.com/histolabs/metro/pkg/builder"
	"github.com/histolabs/metro/pkg/consts"
	"github.com/histolabs/metro/testutil/testfactory"
)

func main() {
	ecfg := encoding.MakeConfig(app.ModuleEncodingRegisters...)

	feeCoin := sdk.Coin{
		Denom:  consts.BondDenom,
		Amount: sdk.NewInt(210),
	}

	opts := []builder.TxBuilderOption{
		builder.SetFeeAmount(sdk.NewCoins(feeCoin)),
		builder.SetGasLimit(1000000000),
	}

	config := types.GetConfig()
	config.SetBech32PrefixForAccount(app.Bech32PrefixAccAddr, app.Bech32PrefixAccPub)

	kr, err := keyring.New("metro", "test", "/home/e/.metro", bytes.NewBuffer([]byte{}), ecfg.Codec)
	if err != nil {
		panic(err)
	}

	grpcConn, err := grpc.Dial(
		"127.0.0.1:9090",
		grpc.WithInsecure(),
	)
	if err != nil {
		panic(err)
	}
	defer grpcConn.Close()

	signer := builder.NewKeyringSigner(ecfg, kr, "validator", "private")
	err = signer.UpdateAccount(context.Background(), grpcConn)
	if err != nil {
		panic(err)
	}

	txBuilder := signer.NewTxBuilder(opts...)

	info, err := signer.Key("validator")
	if err != nil {
		panic(err)
	}

	fromAddr, err := info.GetAddress()
	if err != nil {
		panic(err)
	}

	msg := banktypes.NewMsgSend(fromAddr, testfactory.RandomAddress().(types.AccAddress), types.NewCoins(types.NewInt64Coin(consts.BondDenom, 12)))
	tx, err := signer.BuildSignedTx(txBuilder, false, msg)
	if err != nil {
		panic(err)
	}

	txBytes, err := signer.EncodeTx(tx)
	if err != nil {
		panic(err)
	}

	_ = txBytes
	fmt.Println("code ran successfully 🤨")
}
