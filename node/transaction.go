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
  Type    int       // the type of transaction
  TxIns   []TxIn    // list of input transactions
  TxOuts  []TxOut   // list of output transactions
  Parent  []byte    // hash of referenced transaction (for comment and upvote)
  Content []byte    // content of post / comment
}

type TxIn struct {
  PrevOutput []Output   // reference to corresponding TxOut
  Sig        []byte     // ECDSA signature of entire Transaction (except other sigs and pks)
  PubKey     []byte     // Public key of spender
}

type TxOut struct {
  Value      []int64    // amount of DKarma transferred
  PubKeyHash []byte     // hash of public key of the eventual spender
}
