package tx

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/histolabs/metro/app"
	"github.com/histolabs/metro/app/encoding"
	metrotx "github.com/histolabs/metro/pb"
	"github.com/histolabs/metro/pkg/builder"
	"github.com/histolabs/metro/pkg/consts"
	"github.com/histolabs/metro/testutil/testfactory"
)

const (
	appName        = "metro"
	keyringBackend = "test"
	keyringRootDir = "~/.metro"
	keyName        = "validator"
	chainID        = "private"

	DefaultGRPCEndpoint = "127.0.0.1:9090"
)

func BuildAndSendSecondaryTransaction(grpcEndpoint, secondaryChainID string, tx []byte) error {
	ecfg := encoding.MakeConfig(app.ModuleEncodingRegisters...)

	config := types.GetConfig()
	config.SetBech32PrefixForAccount(app.Bech32PrefixAccAddr, app.Bech32PrefixAccPub)

	kr, err := keyring.New(appName, keyringBackend, keyringRootDir, bytes.NewBuffer([]byte{}), ecfg.Codec)
	if err != nil {
		return err
	}

	grpcConn, err := grpc.Dial(
		grpcEndpoint,
		grpc.WithInsecure(),
	)
	if err != nil {
		return err
	}
	defer grpcConn.Close()

	signer, fromAddr, err := NewSigner(ecfg, kr, grpcConn, secondaryChainID)
	if err != nil {
		return err
	}

	err = BuildAndSendTx(signer, fromAddr, grpcConn, true, secondaryChainID, tx)
	if err != nil {
		return err
	}

	return nil
}

func NewSigner(ecfg encoding.Config, kr keyring.Keyring, conn *grpc.ClientConn, secondaryChainID string) (*builder.KeyringSigner, sdk.AccAddress, error) {
	chainID := chainID
	if secondaryChainID != "" {
		chainID = strings.Join([]string{chainID, secondaryChainID}, consts.ChainIDSeparator)
	}

	signer := builder.NewKeyringSigner(ecfg, kr, keyName, chainID)
	err := signer.UpdateAccount(context.Background(), conn)
	if err != nil {
		return nil, nil, err
	}

	info, err := signer.Key(keyName)
	if err != nil {
		return nil, nil, err
	}

	fromAddr, err := info.GetAddress()
	if err != nil {
		return nil, nil, err
	}

	return signer, fromAddr, nil
}

func BuildAndSendTx(signer *builder.KeyringSigner, fromAddr sdk.AccAddress, conn *grpc.ClientConn, isSecondary bool, secondaryChainID string, txData []byte) error {
	feeCoin := sdk.Coin{
		Denom:  consts.BondDenom,
		Amount: sdk.NewInt(1000000),
	}

	opts := []builder.TxBuilderOption{
		builder.SetFeeAmount(sdk.NewCoins(feeCoin)),
		builder.SetGasLimit(1000000000),
	}

	txBuilder := signer.NewTxBuilder(opts...)

	var msg sdk.Msg
	if isSecondary {
		msg = metrotx.NewSequencerMsg([]byte(secondaryChainID), txData, fromAddr)
	} else {
		msg = banktypes.NewMsgSend(fromAddr, testfactory.RandomAddress().(types.AccAddress), types.NewCoins(types.NewInt64Coin(consts.BondDenom, 1)))
	}

	tx, err := signer.BuildSignedTx(txBuilder, isSecondary, msg)
	if err != nil {
		return err
	}

	txBytes, err := signer.EncodeTx(tx)
	if err != nil {
		return err
	}

	txClient := txtypes.NewServiceClient(conn)
	grpcRes, err := txClient.BroadcastTx(
		context.Background(),
		&txtypes.BroadcastTxRequest{
			Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
			TxBytes: txBytes,
		},
	)
	if err != nil {
		return err
	}

	fmt.Println("exit code:", grpcRes.TxResponse.Code)
	if grpcRes.TxResponse.Code == 0 {
		fmt.Println("tx submitted successfully 🤨")
		return nil
	}

	return fmt.Errorf("tx submission failed: response: %s", grpcRes.TxResponse)
}
