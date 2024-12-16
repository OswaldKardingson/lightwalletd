## HTTP Endpoints

### `GET /get_merkle_root`
Fetches the Merkle root of the current block.

**Request**: None  
**Response**:
```json
{
  "merkle_root": "abc123def456"
}

### `GET /get_merkle_proof`
Generates a Merkle proof for a given transaction index.

**Request**: Query Parameter: 
tx_index 
(int): Transaction index in the block.  
**Response**:
{
  "transaction_index": 3,
  "merkle_proof": [
    "hash1",
    "hash2",
    "hash3"
  ]
}
