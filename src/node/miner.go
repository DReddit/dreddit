package node

import (
  "log"
  "sync"
  "time"
  "net/rpc"
  "net"
  "net/http"
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
  mu          sync.Mutex     // lock on DRNode's state
  me          int            // id of this node
  numPending  int            // number of pending transactions
  PendingTxs  map[int]*TxNode // map from ClerkId to last pending transaction
  Blockchain  []*Block        // current view of the blockchain
  port        string
  ports       []string
  servers     []*rpc.Client
  peermu      sync.Mutex     // lock on the peer list

  // channels
  chNewTx     chan bool      // channel to inform of new tx
  chQuit      chan bool      // channel to send Kill message
  chNewBlock  chan bool      // channel to inform of new block
  gossip      chan GossipReply // the gossip we get back
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
func StartDRNode(me int, port string, servers []string) *DRNode {
  DPrintf("Started new DRNode with id %d", me)
  node := new(DRNode)
  node.me = me
  node.PendingTxs = make(map[int]*TxNode)
  node.numPending = 0
  node.Blockchain = make([]*Block, 0)
  node.port = port
  node.ports = make([]string, 0)
  node.servers = make([]*rpc.Client, 0)

  for _, serverPort := range servers {
    client, err := rpc.DialHTTPPath("tcp", "localhost:" + serverPort, "/" + serverPort)
    if err == nil {
      DPrintf("%d: Successfully connected to miner at port " + serverPort, me)
      node.servers = append(node.servers, client)
      node.ports = append(node.ports, serverPort)
    }
  }

  DPrintf(rpc.DefaultRPCPath)
  DPrintf(rpc.DefaultDebugPath)

  server := rpc.NewServer()
  server.RegisterName("DRNode", node)
  server.HandleHTTP("/" + port, "/debug/" + port)
  l, _ := net.Listen("tcp", "localhost:" + port)
  go http.Serve(l, nil)
  go node.GossipProtocol()
  go node.Mine()
  
  return node
}

// used purely for testing
func (node *DRNode) GetPeerSize() int{ 
  return len(node.ports)
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
  node.Merge(append(args.Peers, args.Port))
  node.peermu.Unlock()
  return nil
}

func (node *DRNode) GossipProtocol() {
  gossipTimeout := time.NewTimer(time.Duration(200) * time.Millisecond) // probably should randomize this
  
  for {
    select {
    case <- gossipTimeout.C:
      DPrintf("Node %d gossiping", node.me)
      node.peermu.Lock()    
      args := GossipArgs{Port:node.port, Peers:node.ports}
      for _, client := range node.servers {
        go func() {
          reply := GossipReply{}
          err := client.Call("DRNode.Gossip", &args, &reply)
          if err == nil {
            node.gossip <- reply
          }
        }()
      }
      node.peermu.Unlock()
      gossipTimeout.Reset(time.Duration(200) * time.Millisecond)
      
    case reply := <- node.gossip: // an optimization would be only run merge when a digest of the peer list has changed
      node.peermu.Lock()
      node.Merge(append(reply.Peers, reply.Port))
      node.peermu.Unlock()
    }
  }
}

func (node *DRNode) Merge(newPeers []string) {
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
      client, err := rpc.DialHTTPPath("tcp", "localhost:" + port, "/" + port)
      if err == nil {
        DPrintf("%d: Successfully connected to miner at port " + port, node.me)
        node.servers = append(node.servers, client)
        node.ports = append(node.ports, port)
      }      
    }
  }
}

func (node *DRNode) AppendTx(args *AppendTxArgs, reply *AppendTxReply) error {
  DPrintf("%d received AppendTx request from client %d", node.me, args.ClerkId)
  node.mu.Lock()
  // Check that this clerk has no pending transaction already
  txNode, ok := node.PendingTxs[args.ClerkId]
  if ok && txNode.Status != HANDLED {
    DPrintf("%d: client %d already has a pending request", node.me, args.ClerkId)
    reply.Success = false
    node.mu.Unlock()
    return nil
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
      return nil
    }
    if node.PendingTxs[args.ClerkId].Status == INVALID {
      reply.Success = false
      node.PendingTxs[args.ClerkId].Status = HANDLED
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
          txs = append(txs, txNode.Tx)
          txNodes = append(txNodes, txNode)

          // If the transaction isn't valid, mark it as such here
        }
      }

      // Next generate a block that includes all these transactions
      newBlock := GenerateBlock(node.Blockchain, txs)

      // Then, advertise our new block to other miners (later)
      // and append our block to the blockchain
      node.Blockchain = append(node.Blockchain, newBlock)

      DPrintf("%d appended a new block to the blockchain:\n%s", node.me, newBlock.toString())

      // Finally, mark all the successful transactions as valid
      for _, txNode := range txNodes {
        node.PendingTxs[txNode.ClerkId].Status = SUCCESS
      }
      node.numPending -= len(txNodes)
    }
    node.mu.Unlock()

    // TODO: again, use channels or condition variables here too
    time.Sleep(time.Millisecond * 100)
  }
}
