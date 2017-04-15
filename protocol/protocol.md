# The DReddit Protocol

## Common Standards

* DReddit uses scrypt for all hashes
* Merkle trees are binary trees of hashes.
    * Merkle trees in DReddit use scrypt.
    * If, when forming a row in the tree (other than the root of the tree), it would have an odd number of elements, the final double-hash is duplicated to ensure that the row has an even number of hashes.
* DReddit uses ECDSA to sign transactions
    * Signatures use DER encoding to pack the r and s components into a single byte stream (this is also what OpenSSL produces by default).
    * 
