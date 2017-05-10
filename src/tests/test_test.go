package tests

import (
  "testing"
  "fmt"
)

func TestBasic(t *testing.T) {
  fmt.Printf("Test: Basic setup with one miner and one user ...\n")
  const nservers = 1
  const nclients = 1
  const unreliable = false
  const tag = "basic"
  cfg := make_config(t, tag, nservers, unreliable)
  defer cfg.cleanup()

  ck := cfg.makeClient(cfg.All())
  ok := ck.Post("First Post!!!1")

  if !ok {
    t.Fatal("Attempted Post failed")
  }

  fmt.Printf("  ... Passed\n")
}
