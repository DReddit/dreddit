package clerk

import (
  "crypto/rand"
  "math/big"
  "net/rpc"
  "node"
  "log"
)


const Debug = 1

type Clerk struct {
  port    string
  ports   []string
  servers []*rpc.Client
  clerkId int  // the unique id of this clerk
  current int  // the current server that this clerk will talk to
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
  ck.ports = servers
  ck.clerkId = int(nrand())
  ck.current = 0

  ck.port = port
  ck.servers = make([]*rpc.Client, 0)

  for _, serverPort := range servers {
    client, err := rpc.DialHTTP("tcp", "localhost:" + serverPort)
    if err == nil {
      DPrintf("%d: Successfully connected to miner at port %d", ck.clerkId, port)
      ck.servers = append(ck.servers, client)
    }
  }
  
  return ck
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
