package node

import (
  "labrpc"
  "log"
  "sync"
)

const Debug = 0
const BLOCK_SIZE_THRESHOLD = 10

func DPrintf(format string, a ...interface{}) (n int, err error) {
  if Debug > 0 {
    log.Printf(format, a...)
  }
  return
}

type DRNode struct {
  // Represents state for a DReddit mining node
  mu          sync.Mutex     // lock on DRNode's state
  me          int            // id of this node
  numPending  int            // number of pending transactions
  PendingTxs  map[int]TxNode // map from ClerkId to last pending transaction
  Blockchain  []Block        // current view of the blockchain

  // channels
  chNewTx     chan bool      // channel to inform of new tx
  chQuit      chan bool      // channel to send Kill message
  chNewBlock  chan bool      // channel to inform of new block
}

const (
  PENDING = iota             // transaction is pending append
  INVALID = iota             // transaction is invalid and will not be processed
  SUCCESS = iota             // transaction was successfully appended to the chain
  HANDLED = iota             // responded to clerk
)

type TxNode struct {
  // The transaction bundled with some metadata for mining purposes
  Tx          Transaction    // the transaction itself
  ClerkId     int            // the id of the clerk who sent the tx
  Status      int            // current status of the transaction
}

// AppendTx
type AppendTxArgs struct {
  Tx          Transaction    // the transaction we want to append
  ClerkId     int            // the id of the clerk who sent the request
}

type AppendTxReply struct {
  Success     bool           // whether or not the request was successful
}

// Kills this node
func (node *DRNode) Kill() {

}

// Starts DReddit node
func StartDRNode(me int) *DRNode {
  node := new(DRNode)
  node.me = me
  node.PendingTxs = make(map[int]TxNode)

  go node.Mine()

  return node
}

func (node *DRNode) AppendTx(args *AppendTxArgs, reply *AppendTxReply) {
  DPrintf("%d received AppendTx request from client %d", node.me, args.ClerkId)
  node.mu.Lock()
  // Check that this clerk has no pending transaction already
  txNode, err := node.PendingTxs[args.ClerkId]
  if !err && txNode.Status != HANDLED {
    DPrintf("%d: client %d already has a pending request", node.me, args.ClerkId)
    reply.Success = false
    node.mu.Unlock()
    return
  }
  // Else, add this to pending txs
  // TODO: maybe validate the transction first?
  node.PendingTxs[args.ClerkId] = TxNode{args.Tx, args.ClerkId, PENDING}
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

    // TODO: figure out how long to sleep / maybe a way to do this w/o spinning
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
      txNodes = make([]TxNode)

      // First, validate all pending transactions
      for clerkId, txNode := range node.pendingTxs {
        if txNode.Status == PENDING {
          // TODO actually validate the transaction here
          // remember to validate against the entire blockchain PLUS
          // all transactions currently in txNodes!
          txNodes.append(txNode)

          // If the transaction isn't valid, mark it as such here
        }
      }

      // Next generate a block that includes all these transactions
      newBlock = GenerateBlock(node.Blockchain, txNodes)

      // Then, advertise our new block to other miners (later)
      // and append our block to the blockchain
      AppendBlock(node.Blockchain, newBlock)

      // Finally, mark all the successful transactions as valid
      for txNode := range txNodes {
        node.PendingTxs[txNode.ClerkId].Status = SUCCESS
      }
    }
    node.mu.Unlock()

    // TODO: again, use channels or condition variables here too
    time.Sleep(time.Millisecond * 100)
  }
}
