package clerk

import (
  "labrpc"
  "crypto/rand"
  "math/big"
  "node"
)

type Clerk struct {
  servers []*labrpc.ClientEnd
  clerkId int  // the unique id of this clerk
  current int  // the current server that this clerk will talk to
}

func nrand() int64 {
  max := big.NewInt(int64(1) << 62)
  bigx, _ := rand.Int(rand.Reader, max)
  x := bigx.Int64()
  return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
  ck := new(Clerk)
  ck.servers = servers
  ck.clerkId = int(nrand())
  ck.current = 0
  return ck
}

func Post(content string) bool {
  current = ck.current
  args := node.AppendTxArgs{}

  args.Tx = Transaction{POST, nil, nil, nil, []byte(content)}
  args.ClerkId = ck.clerkId

  DPrintf("%d: trying to post %s", ck.clerkId, content)

  for {
    reply := node.AppendTxReply{}
    ok := ck.servers[current].Call("DRNode.AppendTx", &args, &reply)
    if !ok {
      current = (current + 1) % len(ck.servers)
    } else {
      break
    }
  }

  if reply.Success {
    DPrintf("%d: Successful post", ck.me)
  } else {
    DPrintf("%d: Transaction invalid", ck.me)
  }

  return reply.Success
}

// TODO make functions for transfer, comment, and upvote
