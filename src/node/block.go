package node

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const DIFFICULTY = 4
const HASH_NUM_BYTES = 32
const PKHASH_NUM_BYTES = 20
const DATA_NUM_BYTES = 80

type Block struct {
	PrevBlock    []byte        // hash of previous block
	MerkleRoot   []byte        // Merkle Tree root of transactions
	Timestamp    uint32        // Unix timestamp of when the block was generated
	Nonce        uint32        // The nonce used to generate this block
	Bits         uint32        // The calculated difficulty target being used for this block
	BlockHash    []byte        // hash of all of the above values in this block
	Transactions []Transaction // list of transactions associated with this block
}

// TODO: we're probably going to need a "Blockchain" struct eventually

func packInt(myInt uint32) []byte {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, myInt)
	return data
}

// Packs the hashed components of a block into an 80-byte slice
func pack(block *Block) []byte {
	data := make([]byte, DATA_NUM_BYTES)
	copy(data[0:4], []byte("\x01\x00\x00\x00"))                                   // Version
	copy(data[4:4+HASH_NUM_BYTES], block.PrevBlock)                               // Prev Block Hash
	copy(data[4+HASH_NUM_BYTES:4+2*HASH_NUM_BYTES], block.MerkleRoot)             // Merkle Root
	copy(data[4+2*HASH_NUM_BYTES:4+2*HASH_NUM_BYTES+4], packInt(block.Timestamp)) // Timestamp
	copy(data[8+2*HASH_NUM_BYTES:8+2*HASH_NUM_BYTES+4], packInt(block.Bits))      // Bits
	copy(data[12+2*HASH_NUM_BYTES:12+2*HASH_NUM_BYTES+4], packInt(block.Nonce))   // Nonce
	return data
}

// Returns a string representation of the block
func (block *Block) toString() string {
	var repr string
	repr = "  ------------------------------------------------------------------------------\n"
	repr += fmt.Sprintf(" / PrevBlock:  %x\n", block.PrevBlock)
	repr += fmt.Sprintf(" | MerkleRoot: %x\n", block.MerkleRoot)
	repr += fmt.Sprintf(" | Timestamp:  %d\n", block.Timestamp)
	repr += fmt.Sprintf(" | Nonce:      %d\n", block.Nonce)
	repr += fmt.Sprintf(" | Bits:       %d\n", block.Bits)
	repr += fmt.Sprintf(" \\ BlockHash:  %x\n", block.BlockHash)
	repr += "  ------------------------------------------------------------------------------"
	return repr
}

// Given a list of transactions
// compute a coinbase tranasction which
//  - consumes each of the tx fees
//  - includes the mining reward
func MakeCoinbaseTx(txs []Transaction) Transaction {
	// TODO: make this customizable
	// This is pubKey #1
	pkHash, _ := base64.StdEncoding.DecodeString("ekZtvHY9XwiGbnzyVOvvMhCEDSE=")
	value := uint32(len(txs)*TX_FEE + TX_COINBASE)
	txOut := TxOut{value, pkHash}
	txOuts := make([]TxOut, 1)
	txOuts[0] = txOut

	return Transaction{COINBASE, nil, txOuts, nil, nil}
}

func VerifyCoinbaseTx(tx Transaction) (bool, error) {
	if tx.Type != COINBASE {
		return false, errors.New("last transaction is not coinbase")
	}

	if tx.TxIns != nil {
		return false, errors.New("coinbase transaction has non-empty txIn")
	}

	if len(tx.TxOuts) != 1 {
		return false, errors.New("coinbase transaction has more or less than one txOut")
	}

	if tx.Parent != nil {
		return false, errors.New("coinbase transaction has non-empty parent")
	}

	if tx.Content != nil {
		return false, errors.New("coinbase transaction has non-empty content")
	}

	return true, nil
}

// Given a new block with a provided
//  MerkleRoot
//  PrevBlock
//  Timestamp
//  Bits
// computes and sets a Nonce and BlockHash such that
// the scrypt of the above data satisfies the difficulty constraint
func proofOfWork(block *Block) {
	data := pack(block)
	nonceIndex := 12 + 2*HASH_NUM_BYTES
	var nonce uint32
	leadingZeros := make([]byte, block.Bits)
	for {
		copy(data[nonceIndex:nonceIndex+4], packInt(nonce)) // Nonce
		hash := Hash(data)
		if bytes.Equal(hash[0:block.Bits], leadingZeros) {
			block.Nonce = nonce
			block.BlockHash = hash
			return
		}
		nonce++
	}
}

//
// Generates the Genesis Block for the dreddit blockchain
// The block consists of 10 transfers of 100 dkarma each to
// 10 hardcoded dreddit addresses
func GenerateGenesisBlock() *Block {
	txOuts := make([]TxOut, 10)
	pubKeyHashB64 := make([]string, 10)
	pubKeyHashB64[0] = "ekZtvHY9XwiGbnzyVOvvMhCEDSE="
	pubKeyHashB64[1] = "2WTu40XZmDEeVXplZMMbRLcp0Aw="
	pubKeyHashB64[2] = "xR48QqPrOIR+aEoAigBVXIXfvrI="
	pubKeyHashB64[3] = "Fag0WPJefAQmrE1tiiKQOrzkwJ0="
	pubKeyHashB64[4] = "gy0QaOI+3nI16oz6lnmuA8zGzGk="
	pubKeyHashB64[5] = "cPeHP2/uyu7tuzOptqgL9Y3R3/I="
	pubKeyHashB64[6] = "uhVWws4xgu/N9lba+5pg6v3XKGY="
	pubKeyHashB64[7] = "uSAfThFpURu+7MN7Dl4YOQWgO/8="
	pubKeyHashB64[8] = "qyfnaoRKUnFTdgVLp+20YC1KTVk="
	pubKeyHashB64[9] = "IJMO7/8wIeyS1/gVDEiZvXoLt4E="

	for i, _ := range txOuts {
		pkHash, _ := base64.StdEncoding.DecodeString(pubKeyHashB64[i])
		txOuts[i] = TxOut{100, pkHash}
	}

	tx := Transaction{COINBASE, nil, txOuts, nil, nil}
	txs := []Transaction{tx}
	genesisBlock := new(Block)
	genesisBlock.PrevBlock = make([]byte, HASH_NUM_BYTES)

	// MerkleRoot
	genesisBlock.MerkleRoot = BuildMerkleTreeStore(txs)

	// Timestamp
	genesisBlock.Timestamp = uint32(1494688622)
	DPrintf("%v", genesisBlock.Timestamp)

	// Nonce
	genesisBlock.Nonce = 0

	// Bits
	genesisBlock.Bits = DIFFICULTY

	// BlockHash
	proofOfWork(genesisBlock)

	// Transactions
	genesisBlock.Transactions = txs

	return genesisBlock
}

//
// Given a list of validated transactions and the previous blockhash
// generate a new valid block using the transactions and returns it
func GenerateBlock(prevBlockHash []byte, txs []Transaction) *Block {
	newBlock := new(Block)

	// PrevBlock
	newBlock.PrevBlock = prevBlockHash

	// Coinbase
	txs = append(txs, MakeCoinbaseTx(txs))

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

//
// Given a block, validates:
//  - difficulty
//  - merkle root
//  - proof of work
//  - coinbase
//  - misc.
func ValidateBlock(block *Block) (bool, error) {
	// transaction list is non-empty:
	if len(block.Transactions) == 0 {
		return false, errors.New("Transaction list should be non-empty")
	}

	// bits must be set to the correct difficulty
	if block.Bits != DIFFICULTY {
		return false, errors.New("Block difficulty set incorrectly")
	}

	// block hash must include Bits leading 0's
	leadingZeros := make([]byte, block.Bits)
	if !bytes.Equal(block.BlockHash[0:block.Bits], leadingZeros) {
		return false, errors.New("Block hash does not match difficulty")
	}

	// block.BlockHash must match hash of block
	data := pack(block)
	hash := Hash(data)
	if !bytes.Equal(block.BlockHash, hash) {
		return false, errors.New("Block hash is incorrect")
	}

	// Merkle Root must be computed correctly
	merkle := BuildMerkleTreeStore(block.Transactions)
	if !bytes.Equal(block.MerkleRoot, merkle) {
		return false, errors.New("Merkle root is incorrect")
	}

	// Coinbase transaction must be last
	txCoinbase := block.Transactions[len(block.Transactions)-1]
	succ, err := VerifyCoinbaseTx(txCoinbase)
	if !succ {
		return false, err
	}

	// None of the other transactions can be coinbase:
	for i, tx := range block.Transactions {
		if i != len(block.Transactions)-1 {
			if tx.Type == COINBASE {
				return false, errors.New("coinbase transaction found in non-last spot")
			}
		}
	}

	return true, nil
}
