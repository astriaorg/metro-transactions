package main

import (
	"bytes"

	"google.golang.org/grpc"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/histolabs/metro/app"
	"github.com/histolabs/metro/app/encoding"

	"github.com/astriaorg/metro-transactions/tx"
)

const (
	appName        = "metro"
	keyringBackend = "test"
	keyringRootDir = "~/.metro"
)

func main() {
	ecfg := encoding.MakeConfig(app.ModuleEncodingRegisters...)

	config := types.GetConfig()
	config.SetBech32PrefixForAccount(app.Bech32PrefixAccAddr, app.Bech32PrefixAccPub)

	kr, err := keyring.New(appName, keyringBackend, keyringRootDir, bytes.NewBuffer([]byte{}), ecfg.Codec)
	if err != nil {
		panic(err)
	}

	grpcConn, err := grpc.Dial(
		tx.DefaultGRPCEndpoint,
		grpc.WithInsecure(),
	)
	if err != nil {
		panic(err)
	}
	defer grpcConn.Close()

	secondaryChainIDs := []string{
		"",    // primary
		"aaa", // secondary
		"bbb", // secondary
	}

	for _, id := range secondaryChainIDs {
		signer, fromAddr, err := tx.NewSigner(ecfg, kr, grpcConn, id)
		if err != nil {
			panic(err)
		}

		err = tx.BuildAndSendTx(signer, fromAddr, grpcConn, id != "", id, []byte("hello"))
		if err != nil {
			panic(err)
		}
	}
}
