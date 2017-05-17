package node

import (
	"golang.org/x/crypto/scrypt"
)

// TODO: create custom hash object, []byte has to be converted to string for UTXO

// scrypt hash for merkle roots and block hashes
func Hash(data []byte) []byte {
	dk, _ := scrypt.Key(data, data, 1024, 1, 1, 32)
	return dk
}

// scrypt hash for dreddit addresses which are 20 bytes long
// and the hash of the account's public key
func PKHash(data []byte) []byte {
	dk, _ := scrypt.Key(data, data, 1024, 1, 1, 20)
	return dk
}
