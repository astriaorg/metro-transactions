package main

import (
	"google.golang.org/grpc"

	"github.com/astriaorg/metro-transactions/tx"
)

func main() {
	kr, err := tx.NewKeyring(tx.EncodingCfg.Codec)
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
		signer, fromAddr, err := tx.NewSigner(tx.EncodingCfg, kr, grpcConn, id)
		if err != nil {
			panic(err)
		}

		err = tx.BuildAndSendTx(signer, fromAddr, grpcConn, id != "", id, []byte("hello"))
		if err != nil {
			panic(err)
		}
	}
}
