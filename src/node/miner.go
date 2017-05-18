package node

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/rpc"
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
	// Represents state for a dreddit mining node
	me      int    // id of this node
	pubHash string // hash of public key of this node

	// locks (must always be acquired in this order!)
	mu     sync.Mutex // lock on DRNode's state
	bcmu   sync.Mutex // lock on the blockchain
	utxoMu sync.Mutex // lock on DRNode's UTXO

	// transactions
	numPending int                // number of pending transactions
	PendingTxs map[string]*TxNode // map from pending tx hash to tx metadata
	SeenTxs    map[string]bool    // stores if we've seen a Tx before

	// blockchain
	Blockchain   []*Block     // current view of the blockchain
	SideChains   []*SideChain // non-main chains rooted somewhere on blockchain
	OrphanChains []*SideChain // non-main chains whoes earliest block is parent-less
	Log          []Block      // log of blocks for replay purposes

	// UTXO
	Utxo *UtxoDb // UTXO

	// networking
	port     string        // port of this server
	ports    []string      // ports of this server's peers
	servers  []*rpc.Client // list of this server's peers
	peermu   sync.Mutex    // lock on the peer list
	listener net.Listener  // rpc listener

	// channels
	chNewTx    chan bool        // channel to inform of new tx
	chNewBlock chan bool        // channel to inform of new block
	gossip     chan GossipReply // the gossip we get back
	quit       chan int         // channel to send kill message
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
	Hash    string      // hash of the transaction
}

// AppendTx
type AppendTxArgs struct {
	Tx      Transaction // the transaction we want to append
	ClerkId int         // the id of the clerk who sent the request
}

type AppendTxReply struct {
	Success bool // whether or not the request was successful
}

// Dummy set of arguments for non-rpc calls from tests
type DummyArgs struct {
}

type DummyReply struct {
	RetVal int
}

// Kills this node
func (node *DRNode) Kill(args *DummyArgs, reply *DummyReply) error {
	close(node.quit)
	node.listener.Close()

	for i := 0; i < len(node.servers); i++ {
		node.servers[i].Close()
	}
	return nil
}

// Starts dreddit node
func StartDRNode(me int, port string, pubHash string, servers []string) *DRNode {
	DPrintf("Started new DRNode with id %d", me)
	node := new(DRNode)
	node.me = me
	node.pubHash = pubHash

	// transactions
	node.PendingTxs = make(map[string]*TxNode)
	node.SeenTxs = make(map[string]bool)
	node.numPending = 0

	// blockchain
	node.Blockchain = make([]*Block, 0)
	node.Log = make([]Block, 0)

	// networking
	node.port = port
	node.ports = make([]string, 0)
	node.servers = make([]*rpc.Client, 0)

	// channels
	node.quit = make(chan int)

	for _, serverPort := range servers {
		client, err := rpc.DialHTTPPath("tcp", "localhost:"+serverPort, "/dreddit"+serverPort)
		if err == nil {
			DPrintf("%d: Successfully connected to miner at port "+serverPort, me)
			node.servers = append(node.servers, client)
			node.ports = append(node.ports, serverPort)

			if len(node.Blockchain) == 0 {
				args := BlockArgs{}
				reply := BlockReply{}
				// we don't put this in a goroutine because we want to wait
				err := client.Call("DRNode.GetBlockChain", &args, &reply)

				if err == nil {
					// When a new node connects to the dreddit network, it gets the blockchain
					// from another node as well as a list of all blocks that node received in order
					// it then replays this block log to produce an identical block tree complete with
					// sidechains and orphan blocks
					node.Log = reply.Log
					node.replayLog()
				}
			}
		}
	}

	DPrintf(rpc.DefaultRPCPath)
	DPrintf(rpc.DefaultDebugPath)

	server := rpc.NewServer()
	server.RegisterName("DRNode", node)
	server.HandleHTTP("/dreddit"+port, "/debug/dreddit"+port)
	l, _ := net.Listen("tcp", "localhost:"+port)
	node.listener = l
	go http.Serve(l, nil)

	go node.gossipProtocol()

	// If our blockchain is still empty, we must be the first-ish node in the network!
	// We'll set up the canonical dreddit genesis block
	if len(node.Blockchain) == 0 {
		node.bootstrap()
		for _, block := range node.Blockchain {
			DPrintf("Genesis Block:\n%v", block.toString())
		}
	}
	go node.mine()

	return node
}

// used purely for testing
func (node *DRNode) GetPeerSize(args *DummyArgs, reply *DummyReply) error {
	reply.RetVal = len(node.ports)
	return nil
}

type GossipArgs struct {
	Port  string
	Peers []string
}

type GossipReply struct {
	Port  string
	Peers []string
}

func (node *DRNode) Gossip(args *GossipArgs, reply *GossipReply) error {
	node.peermu.Lock()
	reply.Port = node.port
	reply.Peers = node.ports
	if args.Port != "-1" {
		node.merge(append(args.Peers, args.Port))
	} else {
		node.merge(args.Peers)
	}
	node.peermu.Unlock()
	return nil
}

type BlockArgs struct{}

type BlockReply struct {
	Log []Block
}

type SendBlockArgs struct {
	SentBlock Block
}

type SendBlockReply struct{}

type SideChain struct {
	Parent      *SideChain // nil if attached to mainchain
	ParentIndex int        // -1 if orphan block
	Block       *Block
	Depth       int
	Children    []*SideChain
}

// given a sidechain, finds the sidechain block which has hash prevHash
// returns nil if no block found. Returns a special sidechain of index -2
// if we find the block in the sidechain
func (sc *SideChain) FindParent(prevHash, curHash []byte) *SideChain {
	if bytes.Compare(sc.Block.BlockHash, curHash) == 0 {
		return &SideChain{ParentIndex: -2}
	}

	if bytes.Compare(sc.Block.BlockHash, prevHash) == 0 {
		return sc
	}

	if len(sc.Children) == 0 {
		return nil
	}

	for _, subSC := range sc.Children {
		result := subSC.FindParent(prevHash, curHash)
		if result != nil {
			return result
		}
	}

	return nil
}

// re-computes depth and parents using supplied value
func (sc *SideChain) Recompute(depth, parentIndex int) {
	sc.Depth = depth + 1
	sc.ParentIndex = parentIndex

	if len(sc.Children) == 0 {
		return
	}

	for _, subSC := range sc.Children {
		subSC.Recompute(depth+1, parentIndex)
	}
}

func (node *DRNode) replayLog() {
	node.bcmu.Lock()
	// first we reset the current blockchain.
	node.Blockchain = make([]*Block, 0)
	node.SideChains = make([]*SideChain, 0)
	node.OrphanChains = make([]*SideChain, 0)
	node.bcmu.Unlock()

	// now we replay the whole log
	for _, newBlock := range node.Log {
		fakeArgs := SendBlockArgs{newBlock}
		fakeReply := SendBlockReply{}
		node.SendBlock(&fakeArgs, &fakeReply)
	}
}

// This function handles our logic on updating the blockchain upon receiving a block
// We roughly follow this guide: https://en.bitcoin.it/wiki/Protocol_rules#.22block.22_messages
func (node *DRNode) SendBlock(args *SendBlockArgs, reply *SendBlockReply) error {
	DPrintf("%d received a block with hash %x", node.me, args.SentBlock.BlockHash)

	newBlock := &args.SentBlock
	prevHash := newBlock.PrevBlock
	curHash := newBlock.BlockHash
	node.bcmu.Lock()
	defer node.bcmu.Unlock()

	node.Log = append(node.Log, *newBlock)

	succ, err := ValidateBlock(newBlock)
	if !succ {
		return err
	}

	if len(node.Blockchain) == 0 ||
		bytes.Compare(prevHash, node.Blockchain[len(node.Blockchain)-1].BlockHash) == 0 { // adds to main chain
		succ, err := node.validateBlockTxs(newBlock)
		if !succ {
			return err
		}
		// append to main chain
		node.Blockchain = append(node.Blockchain, newBlock)

		// Finally, mark all the successful transactions as valid
		node.mu.Lock()
		node.utxoMu.Lock()
		for _, tx := range newBlock.Transactions {
			if tx.Type != COINBASE {
				node.PendingTxs[string((&tx).Hash())].Status = SUCCESS
			}
			node.updateUtxoDb(tx)
		}
		node.numPending -= (len(newBlock.Transactions) - 1)
		node.utxoMu.Unlock()
		node.mu.Unlock()
		return nil
	}

	var curSideChain *SideChain
	status := 0 // 0 for orphan, 1 for new sidechain, 2 for expanding old sidechain/orphan chain

	// now we scan through our current blocks to see where our block falls

	for index, block := range node.Blockchain {
		if bytes.Compare(curHash, block.BlockHash) == 0 { // we've seen this block before
			return nil
		}
		if bytes.Compare(prevHash, block.BlockHash) == 0 { // this is the parent, create sidechain
			newSideChain := SideChain{nil, index, newBlock, 0, make([]*SideChain, 0)}
			curSideChain = &newSideChain
			status = 1
		}
	}

	for _, sc := range node.SideChains {
		result := sc.FindParent(prevHash, curHash)
		if result != nil {
			if result.ParentIndex == -2 {
				return nil
			}

			newSideChain := SideChain{result, result.ParentIndex, newBlock, result.Depth + 1, make([]*SideChain, 0)}
			curSideChain = &newSideChain
			result.Children = append(result.Children, curSideChain)
			status = 2
		}
	}

	for _, oc := range node.OrphanChains {
		result := oc.FindParent(prevHash, curHash)
		if result != nil {
			if result.ParentIndex == -2 {
				return nil
			}
			newSideChain := SideChain{result, result.ParentIndex, newBlock, result.Depth + 1, make([]*SideChain, 0)}
			curSideChain = &newSideChain
			result.Children = append(result.Children, curSideChain)
			status = 2
		}
	}

	if status == 0 { // did not find parent of any kind
		newSideChain := SideChain{nil, -1, newBlock, 0, make([]*SideChain, 0)}
		curSideChain = &newSideChain
	}

	// now we search to see if we can attach orphan chains to our current block

	deleted := 0
	for i := range node.OrphanChains {
		j := i - deleted
		oc := node.OrphanChains[j]
		if bytes.Compare(curHash, oc.Block.PrevBlock) == 0 { // found a match
			curSideChain.Children = append(curSideChain.Children, oc)
			oc.Parent = curSideChain
			oc.Recompute(curSideChain.Depth, curSideChain.ParentIndex)

			// magic trick to do deletion in O(1) time
			node.OrphanChains[j] = node.OrphanChains[len(node.OrphanChains)-1]
			node.OrphanChains = node.OrphanChains[0 : len(node.OrphanChains)-1]
		}
	}

	// now that the block has been added to the system, we need to see if the main chain
	// has changed. In particular, only curSideChain could have become the new main chain

	if curSideChain.ParentIndex == -1 { // if orphan chain, dont do anything
		return nil
	}

	newLength := curSideChain.Depth + curSideChain.ParentIndex + 2
	if newLength > len(node.Blockchain) { // breaks ties in favor of current mainchain so >
		// TODO: gotta validate the side chain before doing this
		breakPoint := curSideChain.ParentIndex
		breakMainChain := node.Blockchain[curSideChain.ParentIndex+1 : len(node.Blockchain)]

		// we construct sidechain nodes for the nodes originally on the mainchain
		newSideNodes := make([]*SideChain, len(breakMainChain))
		for i := range newSideNodes {
			newSCNode := SideChain{nil, breakPoint, breakMainChain[i], i, make([]*SideChain, 0)}
			if i != 0 {
				newSCNode.Parent = newSideNodes[i-1]
				newSideNodes[i-1].Children = append(newSideNodes[i-1].Children, &newSCNode)
			}
			newSideNodes[i] = &newSCNode
		}

		// we look for sidechains off the mainchain after breakpoint and adjust them
		for i := range node.SideChains {
			j := i - deleted
			sc := node.SideChains[j]
			if sc.ParentIndex > breakPoint {
				newParent := newSideNodes[sc.ParentIndex-breakPoint-1]
				sc.Parent = newParent
				sc.Recompute(newParent.Depth, breakPoint)

				// magic trick to do deletion in O(1) time
				node.SideChains[j] = node.SideChains[len(node.SideChains)-1]
				node.SideChains = node.SideChains[0 : len(node.SideChains)-1]
			}
		}

		newMainChain := make([]*Block, curSideChain.Depth+1)

		// heal the mainchain and construct new sidechains in the process
		for curSideChain.Parent != nil {
			depth := curSideChain.Depth
			newMainChain[depth] = curSideChain.Block
			parent := curSideChain.Parent

			for _, sc := range parent.Children {
				if bytes.Compare(curSideChain.Block.BlockHash, sc.Block.BlockHash) == 0 {
					continue
				}
				sc.Parent = nil
				sc.Recompute(0, breakPoint+depth+1) // because depth = 0 corresponds to index breakpoint + 1
				node.SideChains = append(node.SideChains, sc)
			}
			curSideChain = curSideChain.Parent
		}

		node.Blockchain = append(node.Blockchain, newMainChain...)
	}

	return nil
}

func (node *DRNode) GetBlockChain(args *BlockArgs, reply *BlockReply) error {
	node.bcmu.Lock()
	reply.Log = node.Log
	node.bcmu.Unlock()
	return nil
}

func (node *DRNode) gossipProtocol() {
	gossipTimeout := time.NewTimer(time.Duration(100+rand.Intn(100)) * time.Millisecond) // probably should randomize this

	for {
		select {
		case <-node.quit:
			return
		case reply := <-node.gossip: // an optimization would be only run merge when a digest of the peer list has changed
			node.peermu.Lock()
			node.merge(append(reply.Peers, reply.Port))
			node.peermu.Unlock()

		case <-gossipTimeout.C:
			DPrintf("Node %d gossiping", node.me)
			node.peermu.Lock()
			args := GossipArgs{Port: node.port, Peers: node.ports}
			for _, client := range node.servers {
				go func(c *rpc.Client) {
					reply := GossipReply{}
					err := c.Call("DRNode.Gossip", &args, &reply)
					if err == nil {
						node.gossip <- reply
					}
				}(client)
			}
			node.peermu.Unlock()
			gossipTimeout.Reset(time.Duration(100+rand.Intn(100)) * time.Millisecond)
		}
	}
}

// Updates the node's UTXO with the new tranasction tx
func (node *DRNode) updateUtxoDb(tx Transaction) {
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
		DPrintf("%d successfully Updated UTXO: %v %v", node.me, base64.StdEncoding.EncodeToString(txOut.PubKeyHash), txOut.Value)
	}
}

// Bootstrapping mining node
// - give initial genesis block
// - fill UTXO accordingly
func (node *DRNode) bootstrap() {
	DPrintf("DRNode %d: boostrapping blockchain", node.me)
	node.Blockchain = make([]*Block, 0)
	genesisBlock := GenerateGenesisBlock()
	node.Blockchain = append(node.Blockchain, genesisBlock)

	node.Utxo = new(UtxoDb)
	node.Utxo.Entries = make(map[string]*UtxoEntry)
	for _, block := range node.Blockchain {
		for _, tx := range block.Transactions {
			node.updateUtxoDb(tx)
		}
	}
}

// Merge list of peers from a peer with our own list
func (node *DRNode) merge(newPeers []string) {
	// dont need to lock peers, this function is always called with such a lock being held
	// very inefficient implementation, ideally should use hash functions
	for _, port := range newPeers {
		if port == node.port {
			continue
		}
		found := false
		for _, knownport := range node.ports {
			if knownport == port {
				found = true
				break
			}
		}
		if !found {
			client, err := rpc.DialHTTPPath("tcp", "localhost:"+port, "/dreddit"+port)
			if err == nil {
				DPrintf("%d: Successfully connected to miner at port "+port, node.me)
				node.servers = append(node.servers, client)
				node.ports = append(node.ports, port)
			}
		}
	}
}

func (node *DRNode) AppendTx(args *AppendTxArgs, reply *AppendTxReply) error {
	DPrintf("%d received AppendTx request from client %d", node.me, args.ClerkId)
	node.mu.Lock()

	s := string(args.Tx.Hash())
	_, ok := node.SeenTxs[s]
	if ok {
		DPrintf("%d: already seen AppendTx request with hash %x", node.me, s)
		node.mu.Unlock()
		reply.Success = false
		return nil
	} else {
		node.SeenTxs[s] = true
	}

	outStr := ""
	for _, port := range node.ports {
		outStr = outStr + " " + port
	}
	DPrintf("%d has ports "+outStr, node.me)

	// Else, add this to pending txs
	// TODO: maybe validate the transction first?
	// Important to send different error if the tx is already in the blockchain
	node.PendingTxs[s] = &TxNode{args.Tx, args.ClerkId, PENDING, s}
	node.numPending++
	node.mu.Unlock()
	DPrintf("%d successfully started AppendTx request for client %d", node.me, args.ClerkId)

	// flood the transaction to our peers
	node.peermu.Lock()
	for _, client := range node.servers {
		go func(c *rpc.Client) {
			fakereply := AppendTxReply{}
			c.Call("DRNode.AppendTx", args, &fakereply)
		}(client)
	}
	node.peermu.Unlock()

	for {
		node.mu.Lock()
		if node.PendingTxs[s].Status == SUCCESS {
			reply.Success = true
			node.PendingTxs[s].Status = HANDLED
			node.mu.Unlock()
			DPrintf("%d successfully completed AppendTx request for client %d", node.me, args.ClerkId)
			return nil
		}
		if node.PendingTxs[s].Status == INVALID {
			reply.Success = false
			node.PendingTxs[s].Status = HANDLED
			node.mu.Unlock()
			DPrintf("%d's AppendTx request from client %d failed", node.me, args.ClerkId)
			return nil
		}
		node.mu.Unlock()

		// TODO: figure out how long to sleep / maybe a way to do this w/o spinning
		// condition variables seem like the right way to accomplish this:
		// http://stackoverflow.com/questions/19802037/long-polling-global-button-broadcast-to-everyone
		time.Sleep(time.Millisecond * 10)
	}
}

// Infinite loop which collects pending transactions
// and attempts to mine a block
func (node *DRNode) mine() {
	for {
		select {
		case <-node.quit:
			return
		default:
		}

		node.mu.Lock()

		// Check if we have enough pending transactions to make a block
		if node.numPending >= BLOCK_SIZE_THRESHOLD {
			txs := make([]Transaction, 0)
			txNodes := make([]*TxNode, 0)

			// First, validate all pending transactions
			for s, txNode := range node.PendingTxs {
				if txNode.Status == PENDING {
					// TODO actually validate the transaction here
					// remember to validate against the entire blockchain PLUS
					// all transactions currently in txNodes!

					succ, err := node.validateTransaction(&txNode.Tx)

					// If the transaction isn't valid, mark it as such here
					if !succ {
						DPrintf("Error while validating transaction: %v", err)
						node.PendingTxs[s].Status = INVALID
					} else {
						txs = append(txs, txNode.Tx)
						txNodes = append(txNodes, txNode)
					}
				}
			}

			// Grab prevblock hash
			node.bcmu.Lock()
			prevBlockHash := make([]byte, HASH_NUM_BYTES)
			if len(node.Blockchain) != 0 {
				// Genesis block has prev block hash set to all 0's
				lastBlockHash := node.Blockchain[len(node.Blockchain)-1].BlockHash
				copy(prevBlockHash, lastBlockHash)
			}
			node.bcmu.Unlock()

			// Next generate a block that includes all these transactions
			newBlock := GenerateBlock(prevBlockHash, txs, node.pubHash)

			node.bcmu.Lock()

			if len(node.Blockchain) == 0 ||
				bytes.Compare(newBlock.PrevBlock, node.Blockchain[len(node.Blockchain)-1].BlockHash) == 0 { // if the block we just mined still adds to mainchain
				args := SendBlockArgs{*newBlock}

				// Then, advertise our new block to other miners...
				for _, client := range node.servers {
					go func(c *rpc.Client) {
						reply := SendBlockReply{}
						c.Call("DRNode.SendBlock", &args, &reply)
					}(client)
				}

				// ...and append our block to the blockchain
				node.Blockchain = append(node.Blockchain, newBlock)
				node.Log = append(node.Log, *newBlock)
				DPrintf("%d appended a new block to the blockchain:\n%s", node.me, newBlock.toString())

				// Finally, mark all the successful transactions as valid
				node.utxoMu.Lock()
				for _, txNode := range txNodes {
					node.PendingTxs[txNode.Hash].Status = SUCCESS
					node.updateUtxoDb(txNode.Tx)
				}
				// update UTXO with coinbase tx
				node.updateUtxoDb(newBlock.Transactions[len(newBlock.Transactions)-1])
				node.numPending -= len(txNodes)
				node.utxoMu.Unlock()
			}

			node.bcmu.Unlock()
		}
		node.mu.Unlock()

		// TODO: again, use channels or condition variables here too
		time.Sleep(time.Millisecond * 100)
	}
}

// Verify the signature of a TxIn, given the input tx's signature-free hash
func (node *DRNode) verifySignature(txIn *TxIn, txHashNoSig []byte) bool {
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

func (node *DRNode) validateTransaction(tx *Transaction) (bool, error) {
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
			if !(node.verifySignature(&txIn, tx.HashNoSig())) {
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
		parentTx := node.findTxInBlockChain(tx.Parent)
		if parentTx == nil || !(parentTx.Type == POST || parentTx.Type == COMMENT) {
			return false, errors.New("Invalid parent Tx Hash provided")
		}
		if tx.Type == UPVOTE && !(bytes.Equal(tx.TxOuts[0].PubKeyHash, parentTx.TxOuts[0].PubKeyHash)) {
			return false, errors.New("Parent PubKeyHash not matching")
		}
	}
	return true, nil
}

//
// Given a validated (structurally) block; check that:
//   - each non-coinbase transaction is valid against the current blockchain
//   - the coinbase transaction's value is equal to the sum of the
//     transaction fees plus the mining reward
func (node *DRNode) validateBlockTxs(newBlock *Block) (bool, error) {
	for _, tx := range newBlock.Transactions {
		if tx.Type != COINBASE {
			succ, err := tx.ValidateStructure()
			if !succ {
				return false, err
			}
			succ, err = node.validateTransaction(&tx)
			if !succ {
				return false, err
			}
		} else {
			if tx.TxOuts[0].Value != uint32((len(newBlock.Transactions)-1)*TX_FEE+TX_COINBASE) {
				return false, errors.New("Coinbase value is wrong")
			}
		}
		// TODO: tx's need to be validated against *each other* within this new block
		// not sure where or how btc does this?
	}
	return true, nil
}

func (node *DRNode) findTxInBlockChain(txHash []byte) *Transaction {
	for _, block := range node.Blockchain {
		for _, tx := range block.Transactions {
			if bytes.Equal(tx.Hash(), txHash) {
				return &tx
			}
		}
	}
	return nil
}
