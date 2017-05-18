package node

import (
	"bytes"
)

type UtxoOutput struct {
	Spent      bool   // true if the tx has been spent
	PubKeyHash []byte // address that tx is sent to
	Value      uint32 // transaction amount
}

type UtxoEntry struct {
	outputs map[uint32]*UtxoOutput // maps outputIndex to UTXO Output data
}

// UTXO stands for Unspent Transaction Output; it's a database derived from
// the blockchain that each node computes and maintains in order to
// efficiently validate incoming transactions
// TODO: update this once hash function output type is changed
type UtxoDb struct {
	Entries map[string]*UtxoEntry // string hash => utxoentry (contains)
}

type UnspentTx struct {
	TxHash     []byte
	TxOutIndex uint32
	Value      uint32
}

func (utxodb *UtxoDb) GetUnspentTxs(pubkeyHash []byte) ([]UnspentTx, bool) {
	var out []UnspentTx
	for txHash, utxoEntry := range utxodb.Entries {
		for txOutIndex, utxoOut := range utxoEntry.outputs {
			// DPrintf("txHash: %v, txOutIndex: %v, pubkeyHash: %v", base64.StdEncoding.EncodeToString([]byte(txHash)), txOutIndex, base64.StdEncoding.EncodeToString(utxoOut.PubKeyHash))
			if utxoOut.Spent == false && bytes.Equal(utxoOut.PubKeyHash, pubkeyHash) {
				out = append(out, UnspentTx{[]byte(txHash), txOutIndex, utxoOut.Value})
			}
		}
	}
	succ := false
	if len(out) != 0 {
		succ = true
	}
	return out, succ
}

// LookupTx
type GetUtxoArgs struct {
	ClerkId    int    // the id of the clerk who sent the request
	PubKeyHash []byte // the transaction we want to append
}

type GetUtxoReply struct {
	Success    bool        // whether or not the request was successful
	UnspentTxs []UnspentTx // slice of unspent transactions
}

// Returns current set of UTXOs to a clerk, who needs it to compute
// current balance in wallet
func (node *DRNode) GetUtxo(args *GetUtxoArgs, reply *GetUtxoReply) {
	DPrintf("%d received GetUtxo request from client %d", node.me, args.ClerkId)
	node.utxoMu.Lock()
	reply.UnspentTxs, reply.Success = node.Utxo.GetUnspentTxs(args.PubKeyHash)
	node.utxoMu.Unlock()
}
