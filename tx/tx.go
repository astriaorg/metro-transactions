package tx

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"strings"

	"google.golang.org/grpc"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
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
	defaultKeyName = "validator"
	chainID        = "private"

	DefaultGRPCEndpoint = "127.0.0.1:9090"
)

var EncodingCfg = encoding.MakeConfig(app.ModuleEncodingRegisters...)

// BuildAndSendSecondaryTransaction builds and sends a secondary transaction to Metro with the given chain ID and transaction bytes.
func BuildAndSendSecondaryTransaction(grpcEndpoint, secondaryChainID string, tx []byte) error {
	kr, err := NewKeyring(EncodingCfg.Codec)
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

	signer, fromAddr, err := NewSigner(EncodingCfg, kr, grpcConn, secondaryChainID)
	if err != nil {
		return err
	}

	err = BuildAndSendTx(signer, fromAddr, grpcConn, true, secondaryChainID, tx)
	if err != nil {
		return err
	}

	return nil
}

// NewKeyring returns a new keyring with the given codec, using the default Metro app name, keyring backend, and root directory.
func NewKeyring(cdc codec.Codec) (keyring.Keyring, error) {
	config := types.GetConfig()
	config.SetBech32PrefixForAccount(app.Bech32PrefixAccAddr, app.Bech32PrefixAccPub)

	kr, err := keyring.New(appName, keyringBackend, keyringRootDir, bytes.NewBuffer([]byte{}), cdc)
	if err != nil {
		return nil, err
	}

	return kr, nil
}

// NewSigner returns a new KeyringSigner with the given keyring, connection, and secondary chNewSignerain ID.
// If the secondary chain ID is empty, the signer will be for the primary chain.
// If the secondary chain ID is set, the resulting signer's chain ID will be for `primaryChainID|secondaryChainID`.
func NewSigner(ecfg encoding.Config, kr keyring.Keyring, conn *grpc.ClientConn, secondaryChainID string) (*builder.KeyringSigner, sdk.AccAddress, error) {
	chainID := chainID
	if secondaryChainID != "" {
		chainID = strings.Join([]string{chainID, secondaryChainID}, consts.ChainIDSeparator)
	}

	// generate new account
	buf := make([]byte, 8)
	rand.Read(buf)
	keyName := fmt.Sprintf("key-%x", buf)
	record, _, err := kr.NewMnemonic(keyName, keyring.English, hd.CreateHDPath(118, 0, 0).String(), keyring.DefaultBIP39Passphrase, hd.Secp256k1)
	if err != nil {
		return nil, nil, err
	}

	fmt.Println("generated new account:", record.Name)

	list, err := kr.List()
	if err != nil {
		return nil, nil, err
	}

	for _, r := range list {
		fmt.Println(r.Name)
	}

	signer := builder.NewKeyringSigner(ecfg, kr, keyName, chainID)
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

// BuildAndSendTx builds and sends a primary or secondary transaction to Metro.
// If isSecondary is true, the transaction will be a secondary transaction with the given secondary chain ID and transaction bytes.
// Otherwise, it will be a primary transaction and the secondaryChainID and txData arguments will be ignored.
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

	if grpcRes.TxResponse.Code == 0 {
		fmt.Println("metro-transactions: tx submitted successfully 😌")
		return nil
	}

	return fmt.Errorf("metro-transactions: tx submission failed 🤨: response: %s", grpcRes.TxResponse)
}
