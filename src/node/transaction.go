package node

import (
)

const (
  TRANSFER = iota
  POST = iota
  COMMENT = iota
  UPVOTE = iota
)

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

func (txOut *TxOut) pack() []byte {
  data := make([]byte, 4 + len(txOut.PubKeyHash))
  copy(data[0:4], packInt(txOut.Value)) // Value
  copy(data[4:4 + HASH_NUM_BYTES], txOut.PubKeyHash) // PubKeyHash
  return data
}

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
  return hash(data)
}
