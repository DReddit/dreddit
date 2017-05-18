package main

import (
	"encoding/base64"
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"node"
)

func GeneratePKPairs() (string, string, string) {
	privKey, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		fmt.Println(err)
		return "", "", ""
	}
	pubKey := privKey.PubKey()
	privKeyB64 := base64.StdEncoding.EncodeToString(privKey.Serialize())
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey.SerializeCompressed())
	pubKeyB64Hash := base64.StdEncoding.EncodeToString(node.PKHash(pubKey.SerializeCompressed()))
	return privKeyB64, pubKeyB64, pubKeyB64Hash
}

func main() {
	for i := 0; i < 10; i++ {
		priv, pub, pubhash := GeneratePKPairs()
		fmt.Println("priv: ", priv)
		fmt.Println("pub: ", pub)
		fmt.Println("pubhash: ", pubhash)
		fmt.Println()
	}
}
