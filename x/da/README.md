# DA

## Specification for Zero-Knowledge Proof

### Terms and Notation

- $n$: Total number of shards
- $t$: Threshold (minimum number of shards required to prove the possession)
- $s_i$: The $i$-th shard
- $H$: The hash function

### Overview

This system verifies the possession of data shard hash `H(s_i)` without exposing `H(s_i)`.

### Zero-Knowledge Proof System

#### Public Inputs

- $\{H^2(s_i)\}_{i=1}^n$
- $t$: Threshold

#### Private Inputs

- $I$: Index set of shards
- $\{H(s_i)\}_{i \in I}$

#### ZKP Circuit Constraints

- For each $i$, verify $H^2(s_i) = H^2(s_i)_{public}$
- $t \le |I|$

## Msgs

### MsgPublishData

```protobuf
message MsgPublishData {
  option (cosmos.msg.v1.signer) = "sender";
  string sender = 1;
  string metadata_uri = 2;
  repeated bytes shard_double_hashes = 3;
}
```

## Metadata

The metadata that is preserved as a JSON in the `metadata_uri` has a schema below

```protobuf
message Metadata {
  uint64 shard_size = 1;
  uint64 shard_count = 2;
  repeated string shard_uris = 3;
}
```

## URI

Supported URIs are below

- `ipfs://`: IPFS
- `ar://`: Arweave
