package node

import (
  "golang.org/x/crypto/scrypt"
)

// TODO: create custom hash object, []byte has to be converted to string for utxo
// Uses scrypt to hash data
func Hash(data []byte) []byte {
  dk, _ := scrypt.Key(data, data, 1024, 1, 1, 32)
  return dk
}
