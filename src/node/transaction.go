package node

import (
	"encoding/base64"
    "errors"
    "bytes"
)

const (
	COINBASE = iota
	TRANSFER = iota
	POST     = iota
	COMMENT  = iota
	UPVOTE   = iota
)

const TX_FEE      = 1
const TX_UPVOTE   = 10
const TX_COINBASE = 100

type Transaction struct {
	Type    uint32    // the type of transaction
	TxIns   []TxIn    // list of input transactions
	TxOuts  []TxOut   // list of output transactions
	Parent  []byte    // hash of referenced transaction (for comment and upvote)
	Content []byte    // content of post / comment
}

type TxIn struct {
	PrevTxHash      []byte     // reference to corresponding previous Transaction
	PrevTxOutIndex  uint32     // index of corresponding TxOut in prev transaction
	Sig             []byte     // ECDSA signature of entire Transaction (except other sigs and pks)
	PubKey          []byte     // Public key of spender
}

type TxOut struct {
	Value      uint32     // amount of DKarma transferred
	PubKeyHash []byte     // hash of public key of the eventual spender
}

func (txIn *TxIn) pack() []byte {
	data := make([]byte, len(txIn.PrevTxHash) + 4 + len(txIn.Sig) + len(txIn.PubKey))
	copy(data[0:HASH_NUM_BYTES], txIn.PrevTxHash) // PrevTxHash
	copy(data[HASH_NUM_BYTES:4 + HASH_NUM_BYTES], packInt(txIn.PrevTxOutIndex)) // PrevTxOut Index
	copy(data[4 + HASH_NUM_BYTES : 4 + HASH_NUM_BYTES + len(txIn.Sig)], txIn.Sig) // Sig
	copy(data[4 + HASH_NUM_BYTES + len(txIn.Sig): 4 + HASH_NUM_BYTES + len(txIn.Sig) + len(txIn.PubKey)],
		txIn.PubKey) // PubKey
	return data
}

func (txIn *TxIn) packNoSig() []byte {
	data := make([]byte, len(txIn.PrevTxHash) + 4 + len(txIn.PubKey))
	copy(data[0:HASH_NUM_BYTES], txIn.PrevTxHash) // PrevTxHash
	copy(data[HASH_NUM_BYTES:4 + HASH_NUM_BYTES], packInt(txIn.PrevTxOutIndex)) // PrevTxOut Index
	copy(data[4 + HASH_NUM_BYTES: 4 + HASH_NUM_BYTES + len(txIn.PubKey)],
		txIn.PubKey) // PubKey
	return data
}


func (txOut *TxOut) pack() []byte {
	data := make([]byte, 4 + len(txOut.PubKeyHash))
	copy(data[0:4], packInt(txOut.Value)) // Value
	copy(data[4:4 + PKHASH_NUM_BYTES], txOut.PubKeyHash) // PubKeyHash
	return data
}

// The transaction ID is the base64 repr. of its hash
func (tx *Transaction) Id() string {
	return base64.StdEncoding.EncodeToString(tx.Hash())
}

// Returns the hash of the complete transaction
// Used for:
//  - Merkle Tree
//  - UTXO Database
func (tx *Transaction) Hash() []byte {
	data := make([]byte, 0)
	data = append(data, packInt(tx.Type)...)
	for _, txIn := range tx.TxIns {
		data = append(data, txIn.pack()...)
	}
	for _, txOut := range tx.TxOuts {
		data = append(data, txOut.pack()...)
	}
	data = append(data, tx.Parent...)
	data = append(data, tx.Content...)
	return Hash(data)
}

// Returns the hash of the transaction not including signatures
// Used for:
//  - transaction signatures
func (tx *Transaction) HashNoSig() []byte {
	data := make([]byte, 0)
	data = append(data, packInt(tx.Type)...)
	for _, txIn := range tx.TxIns {
		data = append(data, txIn.packNoSig()...)
	}
	for _, txOut := range tx.TxOuts {
		data = append(data, txOut.pack()...)
	}
	data = append(data, tx.Parent...)
	data = append(data, tx.Content...)
	return Hash(data)
}

func (tx *Transaction) ValidateStructure() (bool, error) {
    // For Post and Comment TX, there should only be one output (to oneself)
    if tx.Type == POST || tx.Type == COMMENT {
        if len(tx.TxOuts) != 1 {
            return false, errors.New("Transaction should only have one output")
        }
        // validate that the rest of the money goes back to original account 
        // NOTE: problematic if we ever want to support an account to have multiple public keys
        if !(bytes.Equal(PKHash(tx.TxIns[0].PubKey), tx.TxOuts[0].PubKeyHash)) {
            return false, errors.New("Output PubKeyHash not consistent with input")
        }
    }
    // Upvote Tx should have 2 outputs
    if tx.Type == UPVOTE {
        if len(tx.TxOuts) != 2 {
            return false, errors.New("Upvote Tx should only have two outputs")
        }

        // validate that second output goes back to original account 
        // NOTE: problematic if we ever want to support an account to have multiple public keys
        if !(bytes.Equal(PKHash(tx.TxIns[0].PubKey), tx.TxOuts[1].PubKeyHash)) {
            return false, errors.New("Output PubKeyHash not consistent with input")
        }

    }

    if tx.Type == COMMENT || tx.Type == UPVOTE {
        if tx.Parent == nil {
            return false, errors.New("Comment/Upvote Tx must have parent")
        }
    } else { 
        if tx.Parent != nil {
            return false, errors.New("Transfer/Post Tx should not parent")
        }
    }
    return true, nil

}
