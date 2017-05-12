package clerk

import (
  "labrpc"
  "crypto/rand"
	"math/big"
  "github.com/btcsuite/btcd/btcec"
  "node"
  "log"
	"fmt"
	"encoding/hex"
)


const Debug = 1

type Clerk struct {
  servers []*labrpc.ClientEnd
  clerkId int  // the unique id of this clerk
  current int  // the current server that this clerk will talk too
  privKey *btcec.PrivateKey // private key used by clerk
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

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
  ck := new(Clerk)
  ck.servers = servers
  ck.clerkId = int(nrand())
  ck.current = 0
	// Decode a hex-encoded private key.
	pkBytes, err := hex.DecodeString("22a47fa09a223f2aa079edf85a7c2d4f87" +
			"20ee63e502ee2869afab7de234b80c")
	if err != nil {
			fmt.Println(err)
			return nil
	}

	ck.privKey, _ = btcec.PrivKeyFromBytes(btcec.S256(), pkBytes)
  //ck.privKey, err := GenerateKey(c, rand.Reader)
  return ck
}

func (ck *Clerk) SignTx(tx *node.Transaction) {
  privKey := ck.privKey
	sig, err := privKey.Sign(tx.HashNoSig())
	if err != nil {
		fmt.Println(err)
		return
	}
  for _, txIn := range tx.TxIns {
    txIn.Sig = sig.Serialize()
    txIn.PubKey = privKey.PubKey().SerializeUncompressed()
  }
}

func (ck *Clerk) QueryUtxo(pubkeyHash []byte) ([]node.UnspentTx, bool) {
  current := ck.current
	success := false
	args := node.GetUtxoArgs{}
	args.ClerkId = ck.clerkId
  for {
    reply := node.GetUtxoReply{}
    ok := ck.servers[current].Call("DRNode.GetUtxo", &args, &reply)
    if !ok {
      current = (current + 1) % len(ck.servers)
    } else {
      if reply.Success {
				DPrintf("%d: Query to UTXO successful", ck.clerkId)
				if len(reply.UnspentTxs) != 0 {
        	DPrintf("%d: UTXOs Found", ck.clerkId)
					success = true 
				} else {
					DPrintf("%d: UTXOs Not Found", ck.clerkId)
				}
      } else {
        DPrintf("%d: Query to UTXO not successful", ck.clerkId)
      }

      return reply.UnspentTxs, success
    }
  }
}

func (ck *Clerk) Post(content string) bool {
  current := ck.current
  args := node.AppendTxArgs{}
	
	var inputSum uint32
	var txFee uint32
	priv := ck.privKey
 	pubkeyHash := node.Hash(priv.PubKey().SerializeUncompressed())
	unspentTxs, succ := ck.QueryUtxo(pubkeyHash)	
	if succ == false {
		DPrintf("No valid output transactions to spend. Aborting Transaction")	
		return false
	}
	fmt.Println(unspentTxs)
	utx := unspentTxs[0]
	
	txIns := make([]node.TxIn, 1)
	txIns[0].PrevTxHash = utx.TxHash
	txIns[0].PrevTxOutIndex = utx.TxOutIndex
	txIns[0].PubKey = priv.PubKey().SerializeUncompressed()
	inputSum += utx.Value

	// give self amount of input - transaction fee		
	ptsTx := node.TxOut{inputSum-txFee, pubkeyHash}
	txOuts := make([]node.TxOut, 1)
	txOuts[0] = ptsTx

  Tx := node.Transaction{node.POST, txIns, txOuts, nil, []byte(content)}
	ck.SignTx(&Tx)

  args.Tx = Tx 
  args.ClerkId = ck.clerkId

  DPrintf("%d: trying to post \"%s\"", ck.clerkId, content)

  for {
    reply := node.AppendTxReply{}
    ok := ck.servers[current].Call("DRNode.AppendTx", &args, &reply)
    if !ok {
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
// func (ck *Clerk) Comment(content string, parent []byte)
