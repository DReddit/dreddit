package node

import (
	"bytes"
)

type UtxoOutput struct {
	Spent				bool
	PubKeyHash 	[]byte
	Value 			uint32
}

type UtxoEntry struct {
	outputs map[uint32]*UtxoOutput // outputIndex => output values
} 

// TODO: update this once hash function output type is changed
type UtxoDb struct {
  Entries  map[string]*UtxoEntry  // string hash => utxoentry (contains)
}

type UnspentTx struct {
	TxHash []byte
	TxOutIndex uint32
	Value uint32	
}

func (utxodb *UtxoDb) GetUnspentTxs(pubkeyHash []byte) ([]UnspentTx, bool){
	var out []UnspentTx	
	for txHash, utxoEntry := range utxodb.Entries {
		for txOutIndex, utxoOut := range utxoEntry.outputs {
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
  ClerkId     int            // the id of the clerk who sent the request
  PubKeyHash  []byte      // the transaction we want to append
}

type GetUtxoReply struct {
  Success     bool           // whether or not the request was successful
	UnspentTxs  []UnspentTx
}

func (node *DRNode) GetUtxo(args *GetUtxoArgs, reply *GetUtxoReply) {
  DPrintf("%d received GetUtxo request from client %d", node.me, args.ClerkId)
  node.utxo_mu.Lock()
	utx, succ := node.Utxo.GetUnspentTxs(args.PubKeyHash)
	DPrintf("%v %v", utx, succ)

	reply.UnspentTxs, reply.Success = node.Utxo.GetUnspentTxs(args.PubKeyHash)
  node.utxo_mu.Unlock()
}
