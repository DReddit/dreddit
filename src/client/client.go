package main

import (
  "clerk"
	"fmt"
	"node"
	"strconv"
	"util"
  "net/rpc"
  "os"
  "bufio"
  "strings"
  "bytes"
)

type Client struct {
	clerkPort    string            // address of clerk
  nodePort     string            // address of drnode
	server       *rpc.Client       // dreddit node the client knows of

  hashMap      map[string]string // map of tx abbreviations to tx hashes
}

func MakeClient(clerkPort string, nodePort string) *Client {
	cl := new(Client)
  cl.clerkPort = clerkPort
  cl.nodePort = nodePort

  cl.hashMap = make(map[string]string)

  client, err := rpc.DialHTTPPath("tcp", "localhost:"+nodePort, "/dreddit"+nodePort)
  if err == nil {
    cl.server = client
  }

	return cl
}

//
// Periodically queries the current state of the blockchain
// and then renders a "front page" as ASCII text
func (cl *Client) QueryBlockchain() {
  args := node.BlockArgs{}
  reply := node.BlockReply{}
  cl.server.Call("DRNode.GetBlockchain", &args, &reply)
  blockchain := reply.Log
  fmt.Printf("               _____ _          ___ _         _      _         _    \n")
  fmt.Printf("              |_   _| |_  ___  | _ ) |___  __| |____| |_  __ _(_)_ _ \n")
  fmt.Printf("                | | | ' \\/ -_) | _ \\ / _ \\/ _| / / _| ' \\/ _` | | ' \\ \n")
  fmt.Printf("                |_| |_||_\\___| |___/_\\___/\\__|_\\_\\__|_||_\\__,_|_|_||_|\n")
  fmt.Printf("                                                                     \n")
  for _, block := range blockchain {
    fmt.Printf("%s\n", block.ToString())
  }
  cl.renderFrontPage(blockchain)
  fmt.Printf("\n")
}

func (cl *Client) renderFrontPage(blockchain []node.Block) {
  fmt.Printf("\n\n")
  fmt.Printf("                __  __         ___              __                    \n")
  fmt.Printf("               / /_/ /  ___   / _/______  ___  / /____  ___ ____ ____ \n")
  fmt.Printf("              / __/ _ \\/ -_) / _/ __/ _ \\/ _ \\/ __/ _ \\/ _ `/ _ `/ -_)\n")
  fmt.Printf("              \\__/_//_/\\__/ /_//_/  \\___/_//_/\\__/ .__/\\_,_/\\_, /\\__/ \n")
  fmt.Printf("                                                /_/        /___/      \n\n\n")
  txs := make([]node.Transaction, 0)
  for _, block := range blockchain {
    txs = append(txs, block.Transactions...)
  }
  for _, tx := range txs {
    if tx.Type == node.POST {
      cl.renderContent(tx, txs, 0)
      fmt.Printf("\n")
    }
  }
}

func (cl *Client) renderContent(tx node.Transaction, txs []node.Transaction, indent int) {
  fmt.Printf(strings.Repeat("      ", indent) + "                    /--------------------------------------------\n")
  fmt.Printf(strings.Repeat("      ", indent) + "                    |   %s", tx.Content)
  fmt.Printf(strings.Repeat("      ", indent) + "                    \\--------------------------------------------\n")
  fmt.Printf(strings.Repeat("      ", indent) + "                                                \\  %x\n", tx.Hash()[0:6])
  fmt.Printf(strings.Repeat("      ", indent) + "                                                 \\---------------\n")
  hash := tx.Hash()
  cl.hashMap[fmt.Sprintf("%x\n", hash[0:6])] = string(hash)

  for _, txd := range txs {
    if txd.Type == node.COMMENT && bytes.Equal(txd.Parent, hash) {
      cl.renderContent(txd, txs, indent + 1)
    }
  }
}

func main() {
  fmt.Printf("                               _           _   _ _ _   \n")
  fmt.Printf("                             _| |___ ___ _| |_| |_| |_ \n")
  fmt.Printf("                            | . |  _| -_| . | . | |  _|\n")
  fmt.Printf("                            |___|_| |___|___|___|_|_|  \n")
  fmt.Printf("                                                       \n")

  node.DIFFICULTY = 1
	node.BLOCK_SIZE_THRESHOLD = 1

  const nservers = 1
  const nclients = 1

  drNodes := make([]*node.DRNode, nservers)
  clients := make([]*clerk.Clerk, nclients)
  empty := make([]string, 0)
  serverPorts := make([]string, nservers)

  pkPairs := util.ReadPKPairs()

  for i := 0; i < nservers; i++ {
    drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+10000), pkPairs[i].PubHash, empty)
    serverPorts[i] = strconv.Itoa(i + 10000)
  }

  for i := 0; i < nclients; i++ {
    clients[i] = clerk.MakeClerk(strconv.Itoa(i+10100), pkPairs[nservers+i].Priv, serverPorts)
  }

  cl := MakeClient(strconv.Itoa(10100), strconv.Itoa(10000))

  ck := clients[0]

  reader := bufio.NewReader(os.Stdin)
  quit := false
  for {
    fmt.Printf("> ")
    text, _ := reader.ReadString('\n')
    switch text {
    case "help\n":
      fmt.Printf("Available commands:\n")
      fmt.Printf("  * help\n")
      fmt.Printf("  * post\n")
      fmt.Printf("  * comment\n")
      fmt.Printf("  * upvote\n")
      fmt.Printf("  * transfer\n")
      fmt.Printf("  * view\n")
      fmt.Printf("  * quit\n")
    case "view\n":
      cl.QueryBlockchain()
    case "post\n":
      fmt.Printf("Content: ")
      text, _ := reader.ReadString('\n')
      ok := ck.Post(text)
      if ok {
        fmt.Printf("Success!\n")
      } else {
        fmt.Printf("Failed :(\n")
      }
    case "comment\n":
      fmt.Printf("Parent: ")
      text, _ := reader.ReadString('\n')
      parent, ok := cl.hashMap[text]
      if !ok {
        fmt.Printf("Couldn't find parent\n")
      } else {
        fmt.Printf("Content: ")
        text, _ := reader.ReadString('\n')
        ok = ck.Comment(parent, text)
        if ok {
          fmt.Printf("Success!\n")
        } else {
          fmt.Printf("Failed :(\n")
        }
      }
    case "quit\n":
      quit = true
    default:
      fmt.Printf("not a recognized command\n")
    }
    if quit {
      break
    }
  }
}
