package node

import (
  "time"
  "bytes"
  "encoding/binary"
  "fmt"
)

const DIFFICULTY = 4
const HASH_NUM_BYTES = 32
const DATA_NUM_BYTES = 80

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

func packInt(myInt uint32) []byte {
  data := make([]byte, 4)
  binary.LittleEndian.PutUint32(data, myInt)
  return data
}


// Returns a string representation of the block
func (block *Block) toString() string {
  var repr string
  repr  =             "  ------------------------------------------------------------------------------\n"
  repr += fmt.Sprintf(" / PrevBlock:  %x\n", block.PrevBlock)
  repr += fmt.Sprintf(" | MerkleRoot: %x\n", block.MerkleRoot)
  repr += fmt.Sprintf(" | Timestamp:  %d\n", block.Timestamp)
  repr += fmt.Sprintf(" | Nonce:      %d\n", block.Nonce)
  repr += fmt.Sprintf(" | Bits:       %d\n", block.Bits)
  repr += fmt.Sprintf(" \\ BlockHash:  %x\n", block.BlockHash)
  repr +=             "  ------------------------------------------------------------------------------"
  return repr
}

// Given a new block with a provided
//  MerkleRoot
//  PrevBlock
//  Timestamp
//  Bits
// computes and sets a Nonce and BlockHash such that
// the scrypt of the above data satisfies the difficulty constraint
func proofOfWork(block *Block) {
  data := make([]byte, DATA_NUM_BYTES)
  copy(data[0:4], []byte("\x01\x00\x00\x00")) // Version
  copy(data[4:4 + HASH_NUM_BYTES], block.PrevBlock) // Prev Block Hash
  copy(data[4 + HASH_NUM_BYTES : 4 + 2*HASH_NUM_BYTES], block.MerkleRoot) // Merkle Root
  copy(data[4 + 2*HASH_NUM_BYTES : 4 + 2*HASH_NUM_BYTES + 4], packInt(block.Timestamp)) // Timestamp
  copy(data[8 + 2*HASH_NUM_BYTES : 8 + 2*HASH_NUM_BYTES + 4], packInt(block.Bits)) // Bits

  nonceIndex := 12 + 2*HASH_NUM_BYTES
  var nonce uint32
  for {
    copy(data[nonceIndex:nonceIndex + 4], packInt(nonce)) // Nonce
    hash := Hash(data)
    if bytes.Equal(hash[0:1], []byte("\x00")) {
      block.Nonce = nonce
      block.BlockHash = hash
      return
    }
    nonce++
  }
}

//
// Given a list of validated transactions and the current state of the blockchain
// generates a new valid block using the transactions and returns it
func GenerateBlock(blockchain []*Block, txs []Transaction) *Block {
  newBlock := new(Block)

  // PrevBlock
  if len(blockchain) == 0 {
    // Genesis block has prev block hash set to all 0's
    newBlock.PrevBlock = make([]byte, HASH_NUM_BYTES)
  } else {
    lastBlockHash := blockchain[len(blockchain) - 1].BlockHash
    newBlock.PrevBlock = make([]byte, len(lastBlockHash))
    copy(newBlock.PrevBlock, lastBlockHash)
  }

  // MerkleRoot
  newBlock.MerkleRoot = BuildMerkleTreeStore(txs)

  // Timestamp
  newBlock.Timestamp = uint32(time.Now().Unix())

  // Nonce
  newBlock.Nonce = 0

  // Bits
  newBlock.Bits = DIFFICULTY

  // BlockHash
  proofOfWork(newBlock)

  // Transactions
  newBlock.Transactions = txs

  return newBlock
}

// // Appends a validated block to the blockchain
// func AppendBlock(blockchain *[]*Block, newBlock *Block) {
//   blockchain = append(*blockchain, newBlock)
//   return
// }
