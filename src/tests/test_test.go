package tests

import (
  "testing"
  "fmt"
  "node"
  "clerk"
)

func TestOnePostSingleClient(t *testing.T) {
  fmt.Printf("Test: Basic setup with one miner, one user, one post ...\n")

  node.DIFFICULTY = 1
  node.BLOCK_SIZE_THRESHOLD = 1

  const nservers = 1
  const nclients = 1
  const unreliable = false
  const tag = "BasicOnePostSingleClient"
  cfg := make_config(t, tag, nservers, unreliable)
  defer cfg.cleanup()

  ck := cfg.makeClient(cfg.All())
  ok := ck.Post("First Post!!!1")

  if !ok {
    t.Fatal("Attempted Post failed")
  }

  fmt.Printf("  ... Passed\n")
}

func TestMultiplePostsSingleClient(t *testing.T) {
  fmt.Printf("Test: Basic setup with one miner, one user, ten posts ...\n")

  node.DIFFICULTY = 1
  node.BLOCK_SIZE_THRESHOLD = 1

  const nservers = 1
  const nclients = 1
  const nposts   = 10
  const unreliable = false
  const tag = "BasicMultiplePostsSingleClient"
  cfg := make_config(t, tag, nservers, unreliable)
  defer cfg.cleanup()


  ck := cfg.makeClient(cfg.All())

  for i := 1; i <= nposts; i++ {
    ok := ck.Post(fmt.Sprintf("Post #%d", i))
    if !ok {
      t.Fatal("Attempted Post failed")
    }
  }

  fmt.Printf("  ... Passed\n")
}

func TestMultiplePostsMultipleClients(t *testing.T) {
  fmt.Printf("Test: Basic setup with one miner, ten users, ten posts each ...\n")

  node.DIFFICULTY = 1
  node.BLOCK_SIZE_THRESHOLD = 1

  const nservers = 1
  const nclients = 10
  const nposts   = 10
  const unreliable = false
  const tag = "BasicMultiplePostsMultipleClients"
  cfg := make_config(t, tag, nservers, unreliable)
  defer cfg.cleanup()

  ch := make(chan int)

  for j := 1; j <= nclients; j++ {
    ck := cfg.makeClient(cfg.All())
    go func(ck *clerk.Clerk, j int) {
      for i := 1; i <= nposts; i++ {
        ok := ck.Post(fmt.Sprintf("Client #%d; Post #%d", j, i))
        if !ok {
          t.Fatal("Attempted Post failed")
        }
      }
      ch <- j
    }(ck, j)
  }

  count := 0
  for {
    <-ch
    count++
    if count == nclients {
      break
    }
  }

  fmt.Printf("  ... Passed\n")
}
