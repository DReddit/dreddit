package node

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"log"
	"sync"
	"time"
)

const Debug = 1
const BLOCK_SIZE_THRESHOLD = 1

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

type DRNode struct {
	// Represents state for a DReddit mining node
	mu         sync.Mutex      // lock on DRNode's state
	utxoMu     sync.Mutex      // lock on DRNode's UTXO
	me         int             // id of this node
	numPending int             // number of pending transactions
	PendingTxs map[int]*TxNode // map from ClerkId to last pending transaction
	Blockchain []*Block        // current view of the blockchain
	Utxo       *UtxoDb         // UTXO

	// channels
	chNewTx    chan bool // channel to inform of new tx
	chQuit     chan bool // channel to send Kill message
	chNewBlock chan bool // channel to inform of new block
}

const (
	PENDING = iota // transaction is pending append
	INVALID = iota // transaction is invalid and will not be processed
	SUCCESS = iota // transaction was successfully appended to the chain
	HANDLED = iota // responded to clerk
)

type TxNode struct {
	// The transaction bundled with some metadata for mining purposes
	Tx      Transaction // the transaction itself
	ClerkId int         // the id of the clerk who sent the tx
	Status  int         // current status of the transaction
}

// AppendTx
type AppendTxArgs struct {
	Tx      Transaction // the transaction we want to append
	ClerkId int         // the id of the clerk who sent the request
}

type AppendTxReply struct {
	Success bool // whether or not the request was successful
}

// Kills this node
func (node *DRNode) Kill() {

}

// Starts DReddit node
func StartDRNode(me int) *DRNode {
	DPrintf("Started new DRNode with id %d", me)
	node := new(DRNode)
	node.me = me
	node.PendingTxs = make(map[int]*TxNode)
	node.numPending = 0
	node.Blockchain = make([]*Block, 0)

	node.Bootstrap()
	for _, block := range node.Blockchain {
		DPrintf("Genesis Block:\n%v", block.toString())
	}
	go node.Mine()

	return node
}

// Updates the node's UTXO with the new tranasction tx
func (node *DRNode) UpdateUtxoDb(tx Transaction) {
	for _, txIn := range tx.TxIns {
		txHash := string(txIn.PrevTxHash)
		uidx := txIn.PrevTxOutIndex
		utxoEntry, ok := node.Utxo.Entries[txHash]
		if ok {
			utxoEntry.outputs[uidx].Spent = true
			// if remaining outputs of this transaction are all spent, remove tx from UTXO
			removeTx := true
			for _, utxoOutput := range utxoEntry.outputs {
				if utxoOutput.Spent == false {
					removeTx = false
					break
				}
			}
			if removeTx {
				delete(node.Utxo.Entries, txHash)
			}
		} else {
			// This shouldn't happen if validation is successful
			DPrintf("Detected Invalid Transaction while adding %v to UTXO", tx)
			panic(errors.New("invalid transaction detected"))
		}
	}

	txHash := string(tx.Hash())
	for idx, txOut := range tx.TxOuts {
		uidx := uint32(idx)
		if _, ok := node.Utxo.Entries[txHash]; !ok {
			node.Utxo.Entries[txHash] = new(UtxoEntry)
			node.Utxo.Entries[txHash].outputs = make(map[uint32]*UtxoOutput)
		}
		node.Utxo.Entries[txHash].outputs[uidx] = &UtxoOutput{false, txOut.PubKeyHash, txOut.Value}
		DPrintf("Successfully Updated UTXO: %v %v", base64.StdEncoding.EncodeToString(txOut.PubKeyHash), txOut.Value)
	}

}

// Bootstrapping mining node
// - give initial genesis block
// - fill UTXO accordingly
func (node *DRNode) Bootstrap() {
	DPrintf("DRNode %d: boostrapping blockchain", node.me)
	node.Blockchain = make([]*Block, 0)
	genesisBlock := GenerateGenesisBlock()
	node.Blockchain = append(node.Blockchain, genesisBlock)

	node.Utxo = new(UtxoDb)
	node.Utxo.Entries = make(map[string]*UtxoEntry)
	for _, block := range node.Blockchain {
		for _, tx := range block.Transactions {
			node.UpdateUtxoDb(tx)
		}
	}
}

func (node *DRNode) AppendTx(args *AppendTxArgs, reply *AppendTxReply) {
	DPrintf("%d received AppendTx request from client %d", node.me, args.ClerkId)
	node.mu.Lock()
	// Check that this clerk has no pending transaction already
	txNode, ok := node.PendingTxs[args.ClerkId]
	if ok && txNode.Status != HANDLED {
		DPrintf("%d: client %d already has a pending request", node.me, args.ClerkId)
		reply.Success = false
		node.mu.Unlock()
		return
	}
	// Else, add this to pending txs
	// TODO: maybe validate the transction first?
	node.PendingTxs[args.ClerkId] = &TxNode{args.Tx, args.ClerkId, PENDING}
	node.numPending++
	node.mu.Unlock()
	DPrintf("%d successfully started AppendTx request for client %d", node.me, args.ClerkId)

	for {
		node.mu.Lock()
		if node.PendingTxs[args.ClerkId].Status == SUCCESS {
			reply.Success = true
			node.PendingTxs[args.ClerkId].Status = HANDLED
			node.mu.Unlock()
			DPrintf("%d successfully completed AppendTx request for client %d", node.me, args.ClerkId)
			return
		}
		if node.PendingTxs[args.ClerkId].Status == INVALID {
			reply.Success = false
			node.PendingTxs[args.ClerkId].Status = HANDLED
			node.mu.Unlock()
			DPrintf("%d's AppendTx request from client %d failed", node.me, args.ClerkId)
			return
		}
		node.mu.Unlock()

		// TODO: figure out how long to sleep / maybe a way to do this w/o spinning;
		// condition variables seem like the right way to accomplish this:
		// http://stackoverflow.com/questions/19802037/long-polling-global-button-broadcast-to-everyone
		time.Sleep(time.Millisecond * 10)
	}
}

// Infinite loop which collects pending transactions
// and attempts to mine a block
func (node *DRNode) Mine() {
	for {
		node.mu.Lock()

		// Check if we have enough pending transactions to make a block
		if node.numPending >= BLOCK_SIZE_THRESHOLD {
			txs := make([]Transaction, 0)
			txNodes := make([]*TxNode, 0)

			// First, validate all pending transactions
			for _, txNode := range node.PendingTxs {
				if txNode.Status == PENDING {
					// TODO actually validate the transaction here
					// remember to validate against the entire blockchain PLUS
					// all transactions currently in txNodes!

					succ, err := node.ValidateTransaction(&txNode.Tx)

					// If the transaction isn't valid, mark it as such here
					if !succ {
						DPrintf("Error while validating transaction: %v", err)
						node.PendingTxs[txNode.ClerkId].Status = INVALID
					} else {
						txs = append(txs, txNode.Tx)
						txNodes = append(txNodes, txNode)
					}
				}
			}

			// Next generate a block that includes all these transactions
			newBlock := GenerateBlock(node.Blockchain, txs)

			// Then, advertise our new block to other miners (later TODO)
			// and append our block to the blockchain
			node.Blockchain = append(node.Blockchain, newBlock)

			DPrintf("%d appended a new block to the blockchain:\n%s", node.me, newBlock.toString())

			// Finally, mark all the successful transactions as valid
			node.utxoMu.Lock()
			for _, txNode := range txNodes {
				node.PendingTxs[txNode.ClerkId].Status = SUCCESS
				node.UpdateUtxoDb(txNode.Tx)
			}
			// update UTXO with coinbase tx
			node.UpdateUtxoDb(newBlock.Transactions[len(newBlock.Transactions)-1])
			node.utxoMu.Unlock()
			node.numPending -= len(txNodes)
		}
		node.mu.Unlock()

		// TODO: again, use channels or condition variables here too
		time.Sleep(time.Millisecond * 100)
	}
}

// Verify the signature of a TxIn, given the input tx's signature-free hash
func (node *DRNode) VerifySignature(txIn *TxIn, txHashNoSig []byte) bool {
	signature, err := btcec.ParseSignature(txIn.Sig, btcec.S256())
	if err != nil {
		fmt.Println(err)
		return false
	}
	pubKey, err := btcec.ParsePubKey(txIn.PubKey, btcec.S256())
	if err != nil {
		fmt.Println(err)
		return false
	}
	return signature.Verify(txHashNoSig, pubKey)
}

func (node *DRNode) ValidateTransaction(tx *Transaction) (bool, error) {
	DPrintf("%d validating transaction %v", node.me, tx.Id())

	// TxIn & TxOut are not empty
	if len(tx.TxIns) == 0 || len(tx.TxOuts) == 0 {
		return false, errors.New("TxIns and TxOuts must not be empty")
	}

	// Sum of Inputs is greater than sum of outputs
	var inputSum uint32
	var outputSum uint32
	for _, txIn := range tx.TxIns {
		utxoEntry, ok := node.Utxo.Entries[string(txIn.PrevTxHash)]
		if ok && utxoEntry.outputs[txIn.PrevTxOutIndex].Spent == false {

			// if valid txIn, validate signature
			if !(node.VerifySignature(&txIn, tx.HashNoSig())) {
				return false, errors.New("Invalid transaction signature")
			}

			// validate hash(pubkey) == pubkeyhash of output
			if !(bytes.Equal(utxoEntry.outputs[txIn.PrevTxOutIndex].PubKeyHash, PKHash(txIn.PubKey))) {
				return false, errors.New("PubKeyHashes don't match")
			}

			// TODO: validate that txIn is spendable.
			// Need to traverse last 10 blocks of blockchain and validate that transaction does not exist in those blocks

			inputSum += utxoEntry.outputs[txIn.PrevTxOutIndex].Value

		} else {
			return false, errors.New("Invalid input transaction provided: missing or spent")
		}
	}
	for _, txOut := range tx.TxOuts {
		outputSum += txOut.Value
	}

	if inputSum+TX_FEE < outputSum {
		return false, errors.New("Input sum (plus fee) is less than output sum")
	}

	// validate that transaction has right structure (number of outputs)
	if ok, err := tx.ValidateStructure(); !ok {
		return false, err
	}

	// At this point Tx are structurally valid
	if tx.Type == COMMENT || tx.Type == UPVOTE {
		parentTx := node.FindTxInBlockChain(tx.Parent)
		if parentTx == nil || !(parentTx.Type == POST || parentTx.Type == COMMENT) {
			return false, errors.New("Invalid parent Tx Hash provided")
		}
		if tx.Type == UPVOTE && !(bytes.Equal(tx.TxOuts[0].PubKeyHash, parentTx.TxOuts[0].PubKeyHash)) {
			return false, errors.New("Parent PubKeyHash not matching")
		}
	}

	return true, nil
}

func (node *DRNode) FindTxInBlockChain(txHash []byte) *Transaction {
	for _, block := range node.Blockchain {
		for _, tx := range block.Transactions {
			if bytes.Equal(tx.Hash(), txHash) {
				return &tx
			}
		}
	}
	return nil
}
