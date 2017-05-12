package clerk

import (
  "crypto/rand"
  "math/big"
  "net/rpc"
  "node"
  "log"
  "sync"
  "time"
)


const Debug = 1

type Clerk struct {
  peermu  sync.Mutex
  port    string
  ports   []string
  servers []*rpc.Client
  clerkId int  // the unique id of this clerk
  current int  // the current server that this clerk will talk to

  gossip  chan node.GossipReply
}

func nrand() int64 {
  max := big.NewInt(int64(1) << 62)
  bigx, _ := rand.Int(rand.Reader, max)
  x := bigx.Int64()
  return x
}

func DPrintf(format string, a ...interface{}) (n int, err error) {
  if Debug > 0 {
    log.Printf(format, a...)
  }
  return
}

func MakeClerk(port string, servers []string) *Clerk {
  ck := new(Clerk)
  ck.clerkId = int(nrand())
  ck.current = 0
  ck.ports = make([]string, 0)

  ck.port = port
  ck.servers = make([]*rpc.Client, 0)

  for _, serverPort := range servers {
    client, err := rpc.DialHTTPPath("tcp", "localhost:" + serverPort, "/" + serverPort)
    if err == nil {
      DPrintf("%d: Successfully connected to miner at port " + serverPort, ck.clerkId)
      ck.servers = append(ck.servers, client)
      ck.ports = append(ck.ports, serverPort)
    }
  }

  go ck.GossipProtocol()
  
  return ck
}

func (ck *Clerk) GossipProtocol() {
  gossipTimeout := time.NewTimer(time.Duration(200) * time.Millisecond) // probably should randomize this

  for {
    select {
    case <- gossipTimeout.C:
      DPrintf("Clerk %d gossiping", ck.clerkId)
      ck.peermu.Lock()
      args := node.GossipArgs{Port:"-1", Peers:ck.ports}
      for _, client := range ck.servers {
        go func(c *rpc.Client) {
          reply := node.GossipReply{}
          err := c.Call("DRNode.Gossip", &args, &reply)
          if err == nil {
            ck.gossip <- reply
          }
        }(client)
      }
      ck.peermu.Unlock()
      gossipTimeout.Reset(time.Duration(200) * time.Millisecond)

    case reply := <- ck.gossip: // an optimization would be only run merge when a digest of the peer list has changed
      ck.peermu.Lock()
      ck.Merge(append(reply.Peers, reply.Port))
      ck.peermu.Unlock()
    }
  }
}

func (ck *Clerk) Merge(newPeers []string) {
  // dont need to lock peers, this function is always called with such a lock being held
  // very inefficient implementation, ideally should use hash functions
  for _, port := range newPeers {
    found := false
    for _, knownport := range ck.ports {
      if knownport == port {
        found = true
        break
      }
    }
    if !found {
      client, err := rpc.DialHTTPPath("tcp", "localhost:" + port, "/" + port)
      if err == nil {
        DPrintf("%d: Successfully connected to miner at port " + port, ck.clerkId)
        ck.servers = append(ck.servers, client)
        ck.ports = append(ck.ports, port)
      }
    }
  }
}

func (ck *Clerk) Post(content string) bool {
  current := ck.current
  args := node.AppendTxArgs{}

  args.Tx = node.Transaction{node.POST, nil, nil, nil, []byte(content)}
  args.ClerkId = ck.clerkId

  DPrintf("%d: trying to post \"%s\"", ck.clerkId, content)

  for {
    reply := node.AppendTxReply{}
    err := ck.servers[current].Call("DRNode.AppendTx", &args, &reply)
    if err != nil {
      current = (current + 1) % len(ck.servers)
    } else {
      if reply.Success {
        DPrintf("%d: Successful post", ck.clerkId)
      } else {
        DPrintf("%d: Transaction invalid", ck.clerkId)
      }

      return reply.Success
    }
  }

  return false
}

// TODO make functions for transfer, comment, and upvote
