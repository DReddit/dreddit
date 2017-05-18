package clerk 

import (
	"encoding/base64"
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"net/rpc"
	"node"
)

type BadClerk struct {
    Clerk
}

func MakeBadClerk(port string, privKey string, servers []string) *BadClerk {
    ck := new(BadClerk)
	ck.clerkId = int(nrand())
	ck.current = 0
	ck.ports = make([]string, 0)

	ck.port = port
	ck.servers = make([]*rpc.Client, 0)

	for _, serverPort := range servers {
		client, err := rpc.DialHTTPPath("tcp", "localhost:"+serverPort, "/dreddit"+serverPort)
		if err == nil {
			DPrintf("%d: Successfully connected to miner at port "+serverPort, ck.clerkId)
			ck.servers = append(ck.servers, client)
			ck.ports = append(ck.ports, serverPort)
		}
	}

	pkBytes, err := base64.StdEncoding.DecodeString(privKey)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	ck.privKey, _ = btcec.PrivKeyFromBytes(btcec.S256(), pkBytes)

	go ck.GossipProtocol()

	return ck
}

func (ck *BadClerk) BadPostStructure(content string) bool {
	current := ck.current
	args := node.AppendTxArgs{}

	var inputSum uint32
	//txFee := uint32(node.TX_FEE)
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
	//ptsTx := node.TxOut{inputSum - txFee, pubkeyHash}
	//txOuts := make([]node.TxOut, 1)
	//txOuts[0] = ptsTx
    
	Tx := node.Transaction{node.POST, txIns, nil, nil, []byte(content)}
	ck.SignTx(&Tx)

	args.Tx = Tx
	args.ClerkId = ck.clerkId
	DPrintf("%d: trying to post \"%s\"", ck.clerkId, content)

	for {
		reply := node.AppendTxReply{}
		err := ck.servers[current].Call("DRNode.AppendTx", &args, &reply)
		if err != nil {
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

func (ck *BadClerk) BadCommentStructure(parent string, content string) bool {
	current := ck.current
	args := node.AppendTxArgs{}

	var inputSum uint32
	//txFee := uint32(node.TX_FEE)
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

	//// give self amount of input - transaction fee
	//ptsTx := node.TxOut{inputSum - txFee, pubkeyHash}
	//txOuts := make([]node.TxOut, 1)
	//txOuts[0] = ptsTx

	Tx := node.Transaction{node.COMMENT, txIns, nil, []byte(parent), []byte(content)}
	ck.SignTx(&Tx)

	args.Tx = Tx
	args.ClerkId = ck.clerkId
	DPrintf("%d: trying to comment \"%s\" on %s", ck.clerkId, content, parent)

	for {
		reply := node.AppendTxReply{}
		err := ck.servers[current].Call("DRNode.AppendTx", &args, &reply)
		if err != nil {
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

func (ck *BadClerk) BadUpvoteStructure(parent string, destination string) bool {
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
		if inputSum > node.TX_UPVOTE+txFee {
			break
		}
	}
	if inputSum < node.TX_UPVOTE+txFee {
		DPrintf("Not enough dkarma in wallet to make upvote. Aborting Transaction.")
		return false
	}

	// give self amount of input - (transaction fee + upvote amount)
	//ptsTx := node.TxOut{inputSum - txFee - node.TX_UPVOTE, pubkeyHash}
	// give recipient upvote
	// TODO: change this method to automatically find the destination from the blockchain
	upvTx := node.TxOut{node.TX_UPVOTE, []byte(destination)}
	txOuts := make([]node.TxOut, 1)
	txOuts[0] = upvTx
	//txOuts[1] = ptsTx

	Tx := node.Transaction{node.UPVOTE, txIns, txOuts, []byte(parent), nil}
	ck.SignTx(&Tx)

	args.Tx = Tx
	args.ClerkId = ck.clerkId
	DPrintf("%d: trying to upvote %s", ck.clerkId, parent)

	for {
		reply := node.AppendTxReply{}
		err := ck.servers[current].Call("DRNode.AppendTx", &args, &reply)
		if err != nil {
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
