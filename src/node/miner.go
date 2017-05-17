package node

import (
  "log"
  "sync"
  "time"
  "net/rpc"
  "net"
  "net/http"
  "math/rand"
  "bytes"
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
  bcmu        sync.Mutex     // lock on the blockchain
  me          int            // id of this node
  numPending  int            // number of pending transactions
  PendingTxs  map[int]*TxNode // map from ClerkId to last pending transaction
  SeenTxs     map[string]bool  // stores if we've seen a Tx before
  Blockchain  []*Block        // current view of the blockchain
  SideChains  []*SideChain
  OrphanChains []*SideChain
  port        string
  ports       []string
  servers     []*rpc.Client
  peermu      sync.Mutex     // lock on the peer list
  listener    net.Listener

  // channels
  chNewTx     chan bool      // channel to inform of new tx
  chQuit      chan bool      // channel to send Kill message
  chNewBlock  chan bool      // channel to inform of new block
  gossip      chan GossipReply // the gossip we get back
  quit        chan int
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
  close(node.quit)
  node.listener.Close()

  for i := 0; i < len(node.servers); i++ {
    node.servers[i].Close()
  }
}

// Starts DReddit node
func StartDRNode(me int, port string, servers []string) *DRNode {
  DPrintf("Started new DRNode with id %d", me)
  node := new(DRNode)
  node.me = me
  node.PendingTxs = make(map[int]*TxNode)
  node.SeenTxs = make(map[string]bool)
  node.numPending = 0
  node.Blockchain = make([]*Block, 0)
  node.port = port
  node.ports = make([]string, 0)
  node.servers = make([]*rpc.Client, 0)

  node.quit = make(chan int)

  for _, serverPort := range servers {
    client, err := rpc.DialHTTPPath("tcp", "localhost:" + serverPort, "/dreddit" + serverPort)
    if err == nil {
      DPrintf("%d: Successfully connected to miner at port " + serverPort, me)
      node.servers = append(node.servers, client)
      node.ports = append(node.ports, serverPort)

      args := BlockArgs{}
      reply := BlockReply{}
      err := client.Call("DRNode.GetBlockChain", &args, &reply) // we don't put this in a goroutine because we want to wait
      
      if err == nil {
        if len(reply.Blockchain) > len(node.Blockchain) {
          node.Blockchain = reply.Blockchain
          node.SideChains = reply.SideChains
          node.OrphanChains = reply.OrphanChains
        }
      }
    }
  }

  DPrintf(rpc.DefaultRPCPath)
  DPrintf(rpc.DefaultDebugPath)

  server := rpc.NewServer()
  server.RegisterName("DRNode", node)
  server.HandleHTTP("/dreddit" + port, "/debug/dreddit" + port)
  l, _ := net.Listen("tcp", "localhost:" + port)
  node.listener = l
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
  if args.Port != "-1" {
    node.Merge(append(args.Peers, args.Port))
  } else {
    node.Merge(args.Peers)
  }
  node.peermu.Unlock()
  return nil
}

type BlockArgs struct {}

type BlockReply struct {
  Blockchain []*Block
  SideChains []*SideChain
  OrphanChains []*SideChain
}

type SendBlockArgs struct {
  SentBlock *Block
}

type SendBlockReply struct {}

type SideChain struct {
  Parent  *SideChain // nil if attached to mainchain
  ParentIndex  int // -1 if orphanblock
  Block  *Block
  Depth  int
  Children  []*SideChain
}

// given a sidechain, finds the sidechain block which has hash prevHash
// returns nil if no block found. Returns a special sidechain of index -2
// if we find the block in the sidechain
func (sc *SideChain) FindParent(prevHash, curHash []byte) *SideChain { 
  if bytes.Compare(sc.Block.BlockHash, curHash) == 0 {
    return &SideChain{ParentIndex:-2}
  }

  if bytes.Compare(sc.Block.BlockHash, prevHash) == 0 {
    return sc
  }

  if len(sc.Children) == 0 { return nil }

  for _, subSC := range sc.Children {
    result := subSC.FindParent(prevHash, curHash)
    if result != nil { return result }
  }

  return nil
}

// re-computes depth and parents using supplied value
func (sc *SideChain) Recompute(depth, parentIndex int) {
  sc.Depth = depth + 1
  sc.ParentIndex = parentIndex

  if len(sc.Children) == 0 { return }
  
  for _, subSC := range sc.Children {
    subSC.Recompute(depth + 1, parentIndex)
  }
}

// This function handles our logic on updating the blockchain upon receiving a block
func (node *DRNode) SendBlock(args *SendBlockArgs, reply *SendBlockReply) error {
  DPrintf("%d received a block with hash %x", node.me, args.SentBlock.BlockHash)

  newBlock := args.SentBlock
  prevHash := newBlock.PrevBlock
  curHash := newBlock.BlockHash
  node.bcmu.Lock()
  defer node.bcmu.Unlock()

  if len(node.Blockchain) == 0 || 
    bytes.Compare(prevHash, node.Blockchain[len(node.Blockchain) - 1].BlockHash) == 0 { // adds to main chain
    node.Blockchain = append(node.Blockchain, newBlock)
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
      node.OrphanChains[j] = node.OrphanChains[len(node.OrphanChains) - 1]
      node.OrphanChains = node.OrphanChains[0:len(node.OrphanChains) - 1]
    }
  }

  // now that the block has been added to the system, we need to see if the main chain
  // has changed. In particular, only curSideChain could have become the new main chain

  if curSideChain.ParentIndex == -1 { // if orphan chain, dont do anything
    return nil
  }

  newLength := curSideChain.Depth + curSideChain.ParentIndex + 2
  if newLength > len(node.Blockchain) { // breaks ties in favor of current mainchain so >
    breakPoint := curSideChain.ParentIndex
    breakMainChain := node.Blockchain[curSideChain.ParentIndex + 1:len(node.Blockchain)]
    
    // we construct sidechain nodes for the nodes originally on the mainchain
    newSideNodes := make([]*SideChain, len(breakMainChain))
    for i := range newSideNodes {
      newSCNode := SideChain{nil, breakPoint, breakMainChain[i], i, make([]*SideChain, 0)}
      if i != 0 {
        newSCNode.Parent = newSideNodes[i - 1]
        newSideNodes[i-1].Children = append(newSideNodes[i-1].Children, &newSCNode)
      }
      newSideNodes[i] = &newSCNode
    }

    // we look for sidechains off the mainchain after breakpoint and adjust them
    for i := range node.SideChains {
      j := i - deleted
      sc := node.SideChains[j]
      if sc.ParentIndex > breakPoint { 
        newParent := newSideNodes[sc.ParentIndex - breakPoint - 1]
        sc.Parent = newParent
        sc.Recompute(newParent.Depth, breakPoint)

        // magic trick to do deletion in O(1) time
        node.SideChains[j] = node.SideChains[len(node.SideChains) - 1]
        node.SideChains = node.SideChains[0:len(node.SideChains) - 1]
      }
    }
    
    newMainChain := make([]*Block, curSideChain.Depth + 1)
  
    // heal the mainchain and construct new sidechains in the process
    for curSideChain.Parent != nil {
      depth := curSideChain.Depth
      newMainChain[depth] = curSideChain.Block
      parent := curSideChain.Parent

      for _, sc := range parent.Children {
        if bytes.Compare(curSideChain.Block.BlockHash, sc.Block.BlockHash) == 0 { continue }
        sc.Parent = nil
        sc.Recompute(0, breakPoint + depth + 1) // because depth = 0 corresponds to index breakpoint + 1
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
  reply.Blockchain = node.Blockchain
  reply.SideChains = node.SideChains
  reply.OrphanChains = node.OrphanChains
  node.bcmu.Unlock()
  return nil
}

func (node *DRNode) GossipProtocol() {
  gossipTimeout := time.NewTimer(time.Duration(100 + rand.Intn(100)) * time.Millisecond) // probably should randomize this
  
  for {
    select {
    case <- node.quit:
      return
    case reply := <- node.gossip: // an optimization would be only run merge when a digest of the peer list has changed
      node.peermu.Lock()
      node.Merge(append(reply.Peers, reply.Port))
      node.peermu.Unlock()
      
    case <- gossipTimeout.C:
      DPrintf("Node %d gossiping", node.me)
      node.peermu.Lock()    
      args := GossipArgs{Port:node.port, Peers:node.ports}
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
      gossipTimeout.Reset(time.Duration(100 + rand.Intn(100)) * time.Millisecond)
      
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
      client, err := rpc.DialHTTPPath("tcp", "localhost:" + port, "/dreddit" + port)
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

  s := string(args.Tx.Hash())
  _, ok = node.SeenTxs[s]
  if ok {
    DPrintf("%d: already seen AppendTx request with hash %x", node.me, s)
    node.mu.Unlock()
    reply.Success = false
    return nil
  } else { node.SeenTxs[s] = true }

  outStr := ""
  for _, port := range node.ports {
    outStr = outStr + " " + port
  }
  DPrintf("%d has ports " + outStr, node.me)

  // Else, add this to pending txs
  // TODO: maybe validate the transction first?
  // Important to send different error if the tx is already in the blockchain
  node.PendingTxs[args.ClerkId] = &TxNode{args.Tx, args.ClerkId, PENDING}
  node.numPending++
  node.mu.Unlock()
  DPrintf("%d successfully started AppendTx request for client %d", node.me, args.ClerkId)

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
    select {
    case <- node.quit:
      return
    default:
    }

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

      node.bcmu.Lock()

      if len(node.Blockchain) == 0 || 
        bytes.Compare(newBlock.PrevBlock, node.Blockchain[len(node.Blockchain) - 1].BlockHash) == 0 { // if the block we just mined still adds to mainchain 
        args := SendBlockArgs{newBlock}
        
        for _, client := range node.servers {
          go func(c *rpc.Client) {
            reply := SendBlockReply{}
            c.Call("DRNode.SendBlock", &args, &reply)
          }(client)
        }
        
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

      node.bcmu.Unlock()
    }
    node.mu.Unlock()

    // TODO: again, use channels or condition variables here too
    time.Sleep(time.Millisecond * 100)
  }
}
