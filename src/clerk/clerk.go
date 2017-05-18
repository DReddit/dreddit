package clerk

import (
	"labrpc"
	"crypto/rand"
	"math/big"
	"github.com/btcsuite/btcd/btcec"
	"node"
	"log"
	"fmt"
	"encoding/base64"
)


const Debug = 1

type Clerk struct {
	servers []*labrpc.ClientEnd // the list of dreddit nodes the clerk knows of
	clerkId int                 // the unique id of this clerk
	current int                 // the current server that this clerk will talk to
	privKey *btcec.PrivateKey   // private key used by clerk
}

// Generates a random 64-bit identifying id for the clerk
func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	ck.clerkId = int(nrand())
	ck.current = 0
	// Decode a hardcoded b64-encoded private key (TODO: allow specifying key on startup)
	// This is privKey #7
	pkBytes, err := base64.StdEncoding.DecodeString("p4BuPO9r2e4Wl/45lVCEzDaFnP2k3IfALlTxF4vwMHg=")
	if err != nil {
			fmt.Println(err)
			return nil
	}

	ck.privKey, _ = btcec.PrivKeyFromBytes(btcec.S256(), pkBytes)
	return ck
}

func (ck *Clerk) SignTx(tx *node.Transaction) {
	privKey := ck.privKey
	sig, err := privKey.Sign(tx.HashNoSig())
	if err != nil {
		fmt.Println(err)
		return
	}
	for i, txIn := range tx.TxIns {
		txIn.Sig = sig.Serialize()
		tx.TxIns[i] = txIn
	}
}

func (ck *Clerk) QueryUtxo(pubKeyHash []byte) ([]node.UnspentTx, bool) {
	current := ck.current
	success := false
	args := node.GetUtxoArgs{}
	args.ClerkId = ck.clerkId
	args.PubKeyHash = pubKeyHash
	for {
		reply := node.GetUtxoReply{}
		ok := ck.servers[current].Call("DRNode.GetUtxo", &args, &reply)
		if !ok {
			current = (current + 1) % len(ck.servers)
		} else {
			if reply.Success {
				DPrintf("%d: Query to UTXO successful", ck.clerkId)
				if len(reply.UnspentTxs) != 0 {
					DPrintf("%d: UTXOs Found", ck.clerkId)
					success = true
				} else {
					DPrintf("%d: UTXOs Not Found", ck.clerkId)
				}
			} else {
				DPrintf("%d: Query to UTXO not successful", ck.clerkId)
			}

			return reply.UnspentTxs, success
		}
	}
}

func (ck *Clerk) Transfer(destination string, value uint32) bool {
	current := ck.current
	args := node.AppendTxArgs{}

	txFee := uint32(node.TX_FEE)
	priv := ck.privKey
	pubkeyHash := node.PKHash(priv.PubKey().SerializeCompressed())
	DPrintf("%d: PubKeyHash: %v", ck.clerkId, base64.StdEncoding.EncodeToString(pubkeyHash))
	unspentTxs, succ := ck.QueryUtxo(pubkeyHash)
	if succ == false {
		DPrintf("No valid output transactions to spend. Aborting Transaction")
		return false
	}

	if value == 0 {
		DPrintf("Can only transfer a positive value. Aborting Transaction")
		return false
	}

	txIns := make([]node.TxIn, 0)
	inputSum := uint32(0)

	for _, utx := range unspentTxs {
		inputSum += utx.Value
		txIn := node.TxIn{utx.TxHash, utx.TxOutIndex, nil, priv.PubKey().SerializeCompressed()}
		txIns = append(txIns, txIn)
		if inputSum > value + txFee {
			break
		}
	}
	if inputSum < value + txFee {
		DPrintf("Not enough dkarma in wallet to make transfer. Aborting Transaction.")
		return false
	}

	// give self amount of input - (transaction fee + value)
	ptsTx := node.TxOut{inputSum - txFee - value, pubkeyHash}
	// give recipient value
	recTx := node.TxOut{value, []byte(destination)}
	txOuts := make([]node.TxOut, 2)
	txOuts[0] = recTx
	txOuts[1] = ptsTx

	Tx := node.Transaction{node.TRANSFER, txIns, txOuts, nil, nil}
	ck.SignTx(&Tx)

	args.Tx = Tx
	args.ClerkId = ck.clerkId
	DPrintf("%d: trying to transfer %d to %s", ck.clerkId, value, destination)

	for {
		reply := node.AppendTxReply{}
		ok := ck.servers[current].Call("DRNode.AppendTx", &args, &reply)
		if !ok {
			current = (current + 1) % len(ck.servers)
		} else {
			if reply.Success {
				DPrintf("%d: Successful post", ck.clerkId)
			} else {
				DPrintf("%d: Transaction invalid", ck.clerkId)
			}

			return reply.Success
		}
	}

	return false
}

func (ck *Clerk) Post(content string) bool {
	current := ck.current
	args := node.AppendTxArgs{}

	var inputSum uint32
	txFee := uint32(node.TX_FEE)
	priv := ck.privKey
	pubkeyHash := node.PKHash(priv.PubKey().SerializeCompressed())
	DPrintf("%d: PubKeyHash: %v", ck.clerkId, base64.StdEncoding.EncodeToString(pubkeyHash))
	unspentTxs, succ := ck.QueryUtxo(pubkeyHash)
	if succ == false {
		DPrintf("No valid output transactions to spend. Aborting Transaction")
		return false
	}
	// since the TX_FEE is set at 1 (lowest denomination of dkarma), any utxo will do
	utx := unspentTxs[0]

	txIns := make([]node.TxIn, 1)
	txIns[0].PrevTxHash = utx.TxHash
	txIns[0].PrevTxOutIndex = utx.TxOutIndex
	txIns[0].PubKey = priv.PubKey().SerializeCompressed()
	inputSum += utx.Value

	// give self amount of input - transaction fee
	ptsTx := node.TxOut{inputSum-txFee, pubkeyHash}
	txOuts := make([]node.TxOut, 1)
	txOuts[0] = ptsTx

	Tx := node.Transaction{node.POST, txIns, txOuts, nil, []byte(content)}
	ck.SignTx(&Tx)

	args.Tx = Tx
	args.ClerkId = ck.clerkId
	DPrintf("%d: trying to post \"%s\"", ck.clerkId, content)

	for {
		reply := node.AppendTxReply{}
		ok := ck.servers[current].Call("DRNode.AppendTx", &args, &reply)
		if !ok {
			current = (current + 1) % len(ck.servers)
		} else {
			if reply.Success {
				DPrintf("%d: Successful post", ck.clerkId)
			} else {
				DPrintf("%d: Transaction invalid", ck.clerkId)
			}

			return reply.Success
		}
	}

	return false
}

func (ck *Clerk) Comment(parent string, content string) bool {
	current := ck.current
	args := node.AppendTxArgs{}

	var inputSum uint32
	txFee := uint32(node.TX_FEE)
	priv := ck.privKey
	pubkeyHash := node.PKHash(priv.PubKey().SerializeCompressed())
	DPrintf("%d: PubKeyHash: %v", ck.clerkId, base64.StdEncoding.EncodeToString(pubkeyHash))
	unspentTxs, succ := ck.QueryUtxo(pubkeyHash)
	if succ == false {
		DPrintf("No valid output transactions to spend. Aborting Transaction")
		return false
	}
	// since the TX_FEE is set at 1 (lowest denomination of dkarma), any utxo will do
	utx := unspentTxs[0]

	txIns := make([]node.TxIn, 1)
	txIns[0].PrevTxHash = utx.TxHash
	txIns[0].PrevTxOutIndex = utx.TxOutIndex
	txIns[0].PubKey = priv.PubKey().SerializeCompressed()
	inputSum += utx.Value

	// give self amount of input - transaction fee
	ptsTx := node.TxOut{inputSum-txFee, pubkeyHash}
	txOuts := make([]node.TxOut, 1)
	txOuts[0] = ptsTx

	Tx := node.Transaction{node.COMMENT, txIns, txOuts, []byte(parent), []byte(content)}
	ck.SignTx(&Tx)

	args.Tx = Tx
	args.ClerkId = ck.clerkId
	DPrintf("%d: trying to comment \"%s\" on %s", ck.clerkId, content, parent)

	for {
		reply := node.AppendTxReply{}
		ok := ck.servers[current].Call("DRNode.AppendTx", &args, &reply)
		if !ok {
			current = (current + 1) % len(ck.servers)
		} else {
			if reply.Success {
				DPrintf("%d: Successful post", ck.clerkId)
			} else {
				DPrintf("%d: Transaction invalid", ck.clerkId)
			}

			return reply.Success
		}
	}

	return false
}

func (ck *Clerk) Upvote(parent string, destination string) bool {
	current := ck.current
	args := node.AppendTxArgs{}

	txFee := uint32(node.TX_FEE)
	priv := ck.privKey
	pubkeyHash := node.PKHash(priv.PubKey().SerializeCompressed())
	DPrintf("%d: PubKeyHash: %v", ck.clerkId, base64.StdEncoding.EncodeToString(pubkeyHash))
	unspentTxs, succ := ck.QueryUtxo(pubkeyHash)
	if succ == false {
		DPrintf("No valid output transactions to spend. Aborting Transaction")
		return false
	}

	txIns := make([]node.TxIn, 0)
	inputSum := uint32(0)

	for _, utx := range unspentTxs {
		inputSum += utx.Value
		txIn := node.TxIn{utx.TxHash, utx.TxOutIndex, nil, priv.PubKey().SerializeCompressed()}
		txIns = append(txIns, txIn)
		if inputSum > node.TX_UPVOTE + txFee {
			break
		}
	}
	if inputSum < node.TX_UPVOTE + txFee {
		DPrintf("Not enough dkarma in wallet to make upvote. Aborting Transaction.")
		return false
	}

	// give self amount of input - (transaction fee + upvote amount)
	ptsTx := node.TxOut{inputSum - txFee - node.TX_UPVOTE, pubkeyHash}
	// give recipient upvote
	// TODO: change this method to automatically find the destination from the blockchain
	upvTx := node.TxOut{node.TX_UPVOTE, []byte(destination)}
	txOuts := make([]node.TxOut, 2)
	txOuts[0] = upvTx
	txOuts[1] = ptsTx

	Tx := node.Transaction{node.UPVOTE, txIns, txOuts, []byte(parent), nil}
	ck.SignTx(&Tx)

	args.Tx = Tx
	args.ClerkId = ck.clerkId
	DPrintf("%d: trying to upvote %s", ck.clerkId, parent)

	for {
		reply := node.AppendTxReply{}
		ok := ck.servers[current].Call("DRNode.AppendTx", &args, &reply)
		if !ok {
			current = (current + 1) % len(ck.servers)
		} else {
			if reply.Success {
				DPrintf("%d: Successful post", ck.clerkId)
			} else {
				DPrintf("%d: Transaction invalid", ck.clerkId)
			}

			return reply.Success
		}
	}

	return false
}
