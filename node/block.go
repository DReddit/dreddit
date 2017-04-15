package node

import (

)

type Block struct {
  PrevBlock     []byte         // hash of previous block
  MerkleRoot    []byte         // Merkle Tree root of transactions
  Timestamp     uint32         // Unix timestamp of when the block was generated
  Nonce         uint32         // The nonce used to generate this block
  Bits          uint32         // The calculated difficulty target being used for this block
  BlockHash     []byte         // hash of all of the above values in this block
  Transactions  []Transaction  // list of transactions associated with this block
}
