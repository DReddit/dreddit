package tests

import (
	"clerk"
	"fmt"
	"node"
	"strconv"
	"testing"
	"time"
	"util"
)

func cleanUp(drNodes []*node.DRNode) {
	for i := 0; i < len(drNodes); i++ {
		drNodes[i].Kill(nil, nil)
	}
}

func TestTxValidationPostStructure(t *testing.T) {
	fmt.Printf("Test: Transaction Validation: Post Structure ...\n")
	node.DIFFICULTY = 1
	node.BLOCK_SIZE_THRESHOLD = 1
    PORT_INIT := 50000
	const nservers = 1
	const nclients = 1
	const tag = "tx validation - post"

	drNodes := make([]*node.DRNode, nservers)
	clients := make([]*clerk.Clerk, nclients)
	empty := make([]string, 0)
	serverPorts := make([]string, nservers)

	pkPairs := util.ReadPKPairs()
    
	for i := 0; i < nservers; i++ {
		drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+PORT_INIT), pkPairs[i].PubHash, empty)
		serverPorts[i] = strconv.Itoa(i + PORT_INIT)
	}

	for i := 0; i < nclients; i++ {
		clients[i] = clerk.MakeClerk(strconv.Itoa(i+2100), pkPairs[nservers+i].Priv, serverPorts)
	}

	defer cleanUp(drNodes)

    // steal valid privkey
    x := 5
    ck := clerk.MakeBadClerk(strconv.Itoa(x+2100), pkPairs[nservers+x].Priv, serverPorts)

	for i := 1; i <= 10; i++ {
		ok := ck.BadPostStructure(fmt.Sprintf("Post #%d", i))
		if ok {
			t.Fatal("Post structure validation failed")
		}
	}

	fmt.Printf("  ... Passed\n")
}

func TestOnePost(t *testing.T) {
	fmt.Printf("Test: Basic setup with one miner, one user, one post ...\n")
	node.DIFFICULTY = 1
	node.BLOCK_SIZE_THRESHOLD = 1

	const nservers = 1
	const nclients = 1
	const tag = "basic_one_post"

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

	defer cleanUp(drNodes)

	ck := clients[0]
	ok := ck.Post("First Post!!!1")

	if !ok {
		t.Fatal("Attempted Post failed")
	}

	fmt.Printf("  ... Passed\n")
}

func TestValidComment(t *testing.T) {
	fmt.Printf("Test: Valid Comment ...\n")
	node.DIFFICULTY = 1
	node.BLOCK_SIZE_THRESHOLD = 1
    PORT_INIT := 50100
	const nservers = 1
	const nclients = 1
	const tag = "tx validation - post"

	drNodes := make([]*node.DRNode, nservers)
	clients := make([]*clerk.Clerk, nclients)
	empty := make([]string, 0)
	serverPorts := make([]string, nservers)

	pkPairs := util.ReadPKPairs()
    
	for i := 0; i < nservers; i++ {
		drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+PORT_INIT), pkPairs[i].PubHash, empty)
		serverPorts[i] = strconv.Itoa(i + PORT_INIT)
	}

	for i := 0; i < nclients; i++ {
		clients[i] = clerk.MakeClerk(strconv.Itoa(i+2100), pkPairs[nservers+i].Priv, serverPorts)
	}

	defer cleanUp(drNodes)

    var ok bool
    ck := clients[0]
    ok = ck.Post("Post 1")
    if !ok {
        t.Fatal("Initial Post failed")
    }
    
    refNode := drNodes[0]
    txHash := refNode.Blockchain[1].Transactions[0].Hash()
    ok = ck.Comment(string(txHash), "Comment 1")
    if !ok {
        t.Fatal("Attempted Comment failed")
    }

	fmt.Printf("  ... Passed\n")
}

func TestTxValidationCommentStructure(t *testing.T) {
	fmt.Printf("Test: Transaction Validation: Comment - Invalid Structure ...\n")
	node.DIFFICULTY = 1
	node.BLOCK_SIZE_THRESHOLD = 1
    PORT_INIT := 50300
	const nservers = 1
	const nclients = 1
	const tag = "tx validation - post"

	drNodes := make([]*node.DRNode, nservers)
	clients := make([]*clerk.Clerk, nclients)
	empty := make([]string, 0)
	serverPorts := make([]string, nservers)

	pkPairs := util.ReadPKPairs()
    
	for i := 0; i < nservers; i++ {
		drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+PORT_INIT), pkPairs[i].PubHash, empty)
		serverPorts[i] = strconv.Itoa(i + PORT_INIT)
	}

	for i := 0; i < nclients; i++ {
		clients[i] = clerk.MakeClerk(strconv.Itoa(i+2100), pkPairs[nservers+i].Priv, serverPorts)
	}

	defer cleanUp(drNodes)

    var ok bool
    ck := clients[0]
    ok = ck.Post("Post 1")
    if !ok {
        t.Fatal("Initial Post failed")
    }
   
    x := 5
    bck := clerk.MakeBadClerk(strconv.Itoa(x+2100), pkPairs[nservers+x].Priv, serverPorts)
    ok = bck.BadCommentStructure("fakeTxHash", "Comment 1")
    if ok {
        t.Fatal("Comment should not have been validated")
    }

	fmt.Printf("  ... Passed\n")
}

func TestTxValidationCommentParent(t *testing.T) {
	fmt.Printf("Test: Transaction Validation: Comment - Invalid Parent ...\n")
	node.DIFFICULTY = 1
	node.BLOCK_SIZE_THRESHOLD = 1
    PORT_INIT := 50200
	const nservers = 1
	const nclients = 1
	const tag = "tx validation - post"

	drNodes := make([]*node.DRNode, nservers)
	clients := make([]*clerk.Clerk, nclients)
	empty := make([]string, 0)
	serverPorts := make([]string, nservers)

	pkPairs := util.ReadPKPairs()
    
	for i := 0; i < nservers; i++ {
		drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+PORT_INIT), pkPairs[i].PubHash, empty)
		serverPorts[i] = strconv.Itoa(i + PORT_INIT)
	}

	for i := 0; i < nclients; i++ {
		clients[i] = clerk.MakeClerk(strconv.Itoa(i+2100), pkPairs[nservers+i].Priv, serverPorts)
	}

	defer cleanUp(drNodes)

    var ok bool
    ck := clients[0]
    ok = ck.Post("Post 1")
    if !ok {
        t.Fatal("Initial Post failed")
    }
    
    fakeTxHash := string(node.Hash([]byte("asdfb")))
    ok = ck.Comment(string(fakeTxHash), "Comment 1")
    if ok {
        t.Fatal("Comment should not have been validated")
    }

	fmt.Printf("  ... Passed\n")
}

func TestTxValidationUpvoteStructure(t *testing.T) {
	fmt.Printf("Test: Transaction Validation: Upvote - Invalid Structure ...\n")
	node.DIFFICULTY = 1
	node.BLOCK_SIZE_THRESHOLD = 1
    PORT_INIT := 50400
	const nservers = 1
	const nclients = 1
	const tag = "tx validation - post"

	drNodes := make([]*node.DRNode, nservers)
	clients := make([]*clerk.Clerk, nclients)
	empty := make([]string, 0)
	serverPorts := make([]string, nservers)

	pkPairs := util.ReadPKPairs()
    
	for i := 0; i < nservers; i++ {
		drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+PORT_INIT), pkPairs[i].PubHash, empty)
		serverPorts[i] = strconv.Itoa(i + PORT_INIT)
	}

	for i := 0; i < nclients; i++ {
		clients[i] = clerk.MakeClerk(strconv.Itoa(i+2100), pkPairs[nservers+i].Priv, serverPorts)
	}

	defer cleanUp(drNodes)

    var ok bool
    ck := clients[0]
    ok = ck.Post("Post 1")
    if !ok {
        t.Fatal("Initial Post failed")
    }
   
    x := 5
    bck := clerk.MakeBadClerk(strconv.Itoa(x+2100), pkPairs[nservers+x].Priv, serverPorts)
    ok = bck.BadUpvoteStructure("fakeTxHash", "fakePubKeyHash")
    if ok {
        t.Fatal("Comment should not have been validated")
    }

	fmt.Printf("  ... Passed\n")
}


func TestMultiplePosts(t *testing.T) {
	fmt.Printf("Test: Basic setup with one miner, one user, ten posts ...\n")
	node.DIFFICULTY = 1
	node.BLOCK_SIZE_THRESHOLD = 1

	const nservers = 1
	const nclients = 1
	const tag = "basic_multiple_posts"

	drNodes := make([]*node.DRNode, nservers)
	clients := make([]*clerk.Clerk, nclients)
	empty := make([]string, 0)
	serverPorts := make([]string, nservers)

	pkPairs := util.ReadPKPairs()

	for i := 0; i < nservers; i++ {
		drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+2000), pkPairs[i].PubHash, empty)
		serverPorts[i] = strconv.Itoa(i + 2000)
	}

	for i := 0; i < nclients; i++ {
		clients[i] = clerk.MakeClerk(strconv.Itoa(i+2100), pkPairs[nservers+i].Priv, serverPorts)
	}

	defer cleanUp(drNodes)

	ck := clients[0]

	for i := 1; i <= 10; i++ {
		ok := ck.Post(fmt.Sprintf("Post #%d", i))
		if !ok {
			t.Fatal("Attempted Post failed")
		}
	}

	fmt.Printf("  ... Passed\n")
}

func TestGossip(t *testing.T) {
	node.DIFFICULTY = 1
	node.BLOCK_SIZE_THRESHOLD = 1

	fmt.Printf("Test: Setup with five miners, one user, one post ...\n")
	const nservers = 5
	const nclients = 1
	const tag = "basic_one_post"

	drNodes := make([]*node.DRNode, nservers)
	clients := make([]*clerk.Clerk, nclients)
	empty := make([]string, 0)
	serverPorts := make([]string, nservers)

	pkPairs := util.ReadPKPairs()

	drNodes[0] = node.StartDRNode(0, "13000", pkPairs[0].PubHash, empty)
	serverPorts[0] = "13000"

	for i := 1; i < nservers; i++ {
		knownPorts := make([]string, 1)
		knownPorts[0] = strconv.Itoa(i + 12999)
		drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+13000), pkPairs[i].PubHash, knownPorts)
		serverPorts[i] = strconv.Itoa(i + 13000)
	}

	defer cleanUp(drNodes)

	time.Sleep(time.Duration(3000) * time.Millisecond)

	for i := 0; i < nservers; i++ {
		dummyReply := node.DummyReply{}
		drNodes[i].GetPeerSize(nil, &dummyReply)
		size := dummyReply.RetVal
		if size != nservers-1 {
			t.Fatal("Did not learn group of peers quickly enough, miner %d only knows %d peers", i, size)
		}
	}

	for i := 0; i < nclients; i++ {
		clients[i] = clerk.MakeClerk(strconv.Itoa(i+3100), pkPairs[nservers+i].Priv, serverPorts)
	}

	ck := clients[0]
	ok := ck.Post("First Post!!!1")

	if !ok {
		t.Fatal("Attempted Post failed")
	}

	time.Sleep(time.Duration(2000) * time.Millisecond)

	fmt.Printf("  ... Passed\n")
}

func TestChainResolution(t *testing.T) {
	node.DIFFICULTY = 1
	node.BLOCK_SIZE_THRESHOLD = 1

	fmt.Printf("Test: Setup with five miners, five users, one post each ...\n")
	const nservers = 5
	const nclients = 5
	const tag = "basic_one_post"

	drNodes := make([]*node.DRNode, nservers)
	clients := make([]*clerk.Clerk, nclients)
	empty := make([]string, 0)
	serverPorts := make([]string, nservers)

	pkPairs := util.ReadPKPairs()

	drNodes[0] = node.StartDRNode(0, "14000", pkPairs[0].PubHash, empty)
	serverPorts[0] = "14000"

	for i := 1; i < nservers; i++ {
		knownPorts := make([]string, 1)
		knownPorts[0] = strconv.Itoa(i + 13999)
		drNodes[i] = node.StartDRNode(i, strconv.Itoa(i+14000), pkPairs[i].PubHash, knownPorts)
		serverPorts[i] = strconv.Itoa(i + 14000)
	}

	defer cleanUp(drNodes)

	time.Sleep(time.Duration(3000) * time.Millisecond)

	for i := 0; i < nservers; i++ {
		dummyReply := node.DummyReply{}
		drNodes[i].GetPeerSize(nil, &dummyReply)
		size := dummyReply.RetVal
		if size != nservers-1 {
			t.Fatal("Did not learn group of peers quickly enough, miner %d only knows %d peers", i, size)
		}
	}

	for i := 0; i < nclients; i++ {
		clients[i] = clerk.MakeClerk(strconv.Itoa(i+3100), pkPairs[nservers+i].Priv, serverPorts)
	}

	for i := 0; i < 5; i++ {
		go func(index int) {
			ck := clients[index]
			ok := ck.Post(strconv.Itoa(index) + " Post!!!1")

			if !ok {
				t.Fatal("Attempted Post failed")
			}
		}(i)
	}

	time.Sleep(time.Duration(10000) * time.Millisecond)

	fmt.Printf("  ... Passed\n")
}


