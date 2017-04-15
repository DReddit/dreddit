package node

import (
  "time"
)

const DIFFICULTY = 4

type Block struct {
  PrevBlock     []byte         // hash of previous block
  MerkleRoot    []byte         // Merkle Tree root of transactions
  Timestamp     uint32         // Unix timestamp of when the block was generated
  Nonce         uint32         // The nonce used to generate this block
  Bits          uint32         // The calculated difficulty target being used for this block
  BlockHash     []byte         // hash of all of the above values in this block
  Transactions  []Transaction  // list of transactions associated with this block
}

// TODO: we're probably going to need a "Blockchain" struct eventually

//
// Given a list of validated transactions and the current state of the blockchain
// generates a new valid block using the transactions and returns it
func GenerateBlock(blockchain *[]Block, txNodes *[]TxNode) Block {
  newBlock := new(Block)

  // PrevBlock
  lastBlockHash := blockchain[len(blockchain) - 1].BlockHash
  newBlock.PrevBlock = make([]byte, len(lastBlockHash))
  copy(newBlock.PrevBlock, lastBlockHash)

  // MerkleRoot
  // TODO: get the MerkleRoot working
  newBlock.MerkleRoot = nil

  // Timestamp
  newBlock.Timestamp = time.Now().Unix()

  // Nonce
  newBlock.Nonce = 0

  // Bits
  newBlock.Bits = DIFFICULTY

  // BlockHash
  newBlock.BlockHash = make([]byte, len(lastBlockHash))

  // Transactions
  transactions = make([]Transaction, len(txNodes))
  for txNode, i := range txNodes {
    transactions[i] = txNode.Tx
  }
  newBlock.Transactions = transactions

  // TODO: actually do the proof of work part
  return newBlock
}

// Appends a validated block to the blockchain
func AppendBlock(blockchain *[]Block, newBlock *Block) {
  append(blockChain, newBlock)
  return
}
