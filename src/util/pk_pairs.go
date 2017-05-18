package util

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"node"
	"os"
	"strings"
)

type PKPair struct {
	Priv    string
	Pub     string
	PubHash string
}

func GeneratePKPairs() PKPair {
	privKey, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		fmt.Println(err)
		return PKPair{}
	}
	pubKey := privKey.PubKey()
	privKeyB64 := base64.StdEncoding.EncodeToString(privKey.Serialize())
	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey.SerializeCompressed())
	pubKeyB64Hash := base64.StdEncoding.EncodeToString(node.PKHash(pubKey.SerializeCompressed()))
	return PKPair{privKeyB64, pubKeyB64, pubKeyB64Hash}
}

func ReadPKPairs() []PKPair {
	f, _ := os.Open("genesis_pkpairs")
	defer f.Close()

	pkPairs := make([]PKPair, 10)

	scanner := bufio.NewScanner(f)
	for i := 0; i < 10; i++ {
		scanner.Scan()
		priv := strings.Split(scanner.Text(), "  ")[1]
		scanner.Scan()
		pub := strings.Split(scanner.Text(), "  ")[1]
		scanner.Scan()
		pubHash := strings.Split(scanner.Text(), "  ")[1]
		scanner.Scan()
		pkPairs[i] = PKPair{priv, pub, pubHash}
	}

	return pkPairs
}

func main() {
	for i := 0; i < 10; i++ {
		pkPair := GeneratePKPairs()
		fmt.Println("priv: ", pkPair.Priv)
		fmt.Println("pub: ", pkPair.Pub)
		fmt.Println("pubhash: ", pkPair.PubHash)
		fmt.Println()
	}
}
