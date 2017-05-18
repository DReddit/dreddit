# The dreddit Protocol

## Common Standards

* dreddit uses scrypt for all hashes
* Merkle trees are binary trees of hashes.
    * Merkle trees in dreddit use scrypt.
    * If, when forming a row in the tree (other than the root of the tree), it would have an odd number of elements, the final double-hash is duplicated to ensure that the row has an even number of hashes.
* dreddit uses ECDSA to sign transactions
    * Signatures use DER encoding to pack the r and s components into a single byte stream (this is also what OpenSSL produces by default).

## Transaction Verification
In addition to the [rules](https://en.bitcoin.it/wiki/Protocol_rules#.22tx.22_messages) that the original bitcoin protocol verifies, dreddit requires additional checks for each of the additional transactions supported:

* Post: 
	* There must be exactly 1 output. The corresponding publickeyhash is defined to be the post's author

* Comment:
	* Parent Tx field must not be empty. 
	* Parent Tx must reference either a Post or Comment Tx already existing in the blockhain
	* There must be exactly 1 output. The corresponding publickeyhash is defined to be the comment's author

* Upvote:
	* Parent Tx field must not be empty
	* Parent Tx must reference either a Post or Comment Tx already existing in the blockhain
	* There must be exactly 2 outputs
		* First output's publickeyhash must be the same the parent tx's only output's pubkeyhash
