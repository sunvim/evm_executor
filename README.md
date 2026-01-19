# EVM Executor Service

A lightweight distributed EVM block execution engine that processes blockchain transactions, maintains world state with a sliding window mechanism, and generates transaction receipts.

## Features

- **EVM Integration**: Full support for EVM execution using go-ethereum
- **State Management**: Maintains a 1024-block sliding window with automatic TTL-based pruning
- **High Performance**: LRU caching and batch write optimizations
- **Monitoring**: Prometheus metrics and health checks
- **Distributed**: Designed for horizontal scaling with Kubernetes
- **Chain Support**: BSC mainnet (extensible to other EVM chains)

## Architecture

```
┌─────────────────┐
│  Block Syncer   │ (writes blocks to Pika)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│      Pika       │ (Redis-compatible storage)
│                 │
│ • Blocks        │
│ • State         │
│ • Receipts      │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  EVM Executor   │ (this service)
│                 │
│ ┌─────────────┐ │
│ │  StateDB    │ │ ← LRU Cache
│ │  (Pika)     │ │
│ └─────────────┘ │
│        │        │
│        ▼        │
│ ┌─────────────┐ │
│ │ EVM Engine  │ │
│ └─────────────┘ │
│        │        │
│        ▼        │
│ ┌─────────────┐ │
│ │  Receipts   │ │
│ └─────────────┘ │
└─────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.21+
- Pika (Redis-compatible database)
- Docker (optional)

### Build

```bash
make build
```

### Configuration

Edit `config/config.yaml`:

```yaml
chain:
  name: "bsc"
  network_id: 56

storage:
  pika:
    addr: "127.0.0.1:9221"
    
execution:
  state_cache_size: 10000
  storage_cache_size: 50000
```

### Run

```bash
./bin/executor -config config/config.yaml
```

## State Management

### Sliding Window Mechanism

The executor maintains a 1024-block sliding window:

- **Historical State**: Stored with 60-minute TTL
  - `st:{blockNum}:acc:{address}` → Account
  - `st:{blockNum}:stor:{address}:{key}` → Value

- **Latest State**: Permanently stored
  - `st:latest:acc:{address}` → Account
  - `st:latest:stor:{address}:{key}` → Value

- **Contract Code**: Permanently stored (deduplicated by hash)
  - `st:code:{codeHash}` → Bytecode

### Cache Strategy

Three-level cache hierarchy:

1. **L1**: In-memory modifications (current block)
2. **L2**: LRU cache (hot data)
3. **L3**: Pika storage

## Monitoring

### Metrics

Exposed on `http://localhost:9091/metrics`:

#### Execution Metrics
- `evm_executor_execution_height` - Current execution height
- `evm_executor_execution_lag` - Blocks behind sync
- `evm_executor_txs_executed_total` - Total transactions executed
- `evm_executor_gas_used_total` - Total gas consumed

#### State Metrics
- `evm_executor_state_accounts_cached` - Cached accounts
- `evm_executor_state_cache_hit_rate` - Cache hit rate
- `evm_executor_state_commit_latency_seconds` - Commit latency

#### Worker Pool Metrics
- `evm_executor_worker_pool_active_workers{pool}` - Active workers
- `evm_executor_worker_pool_queue_length{pool}` - Queue depth

### Grafana Dashboard

Import the dashboard from `deployments/grafana/dashboard.json` to visualize:
- Execution progress vs sync height
- Transaction throughput (TPS)
- Gas usage trends
- Cache performance
- Worker pool status

## Docker Deployment

### Build Image

```bash
make docker-build
```

### Run with Docker Compose

```bash
docker-compose -f deployments/docker/docker-compose.yaml up
```

This starts:
- Pika database
- EVM Executor
- Prometheus
- Grafana (http://localhost:3000, admin/admin)

## Kubernetes Deployment

```bash
kubectl apply -f deployments/kubernetes/deployment.yaml
```

Resources:
- **CPU**: 2-4 cores per pod
- **Memory**: 8-16GB per pod
- **Replicas**: 2-4 (with distributed lock coordination)

## Development

### Build

```bash
make build
```

### Test

```bash
make test
```

### Lint

```bash
make lint
```

### Format

```bash
make fmt
```

## Configuration Reference

| Parameter | Description | Default |
|-----------|-------------|---------|
| `chain.name` | Chain name (bsc) | bsc |
| `chain.network_id` | Network ID | 56 |
| `storage.pika.addr` | Pika address | 127.0.0.1:9221 |
| `execution.state_cache_size` | Account cache size | 10000 |
| `execution.storage_cache_size` | Storage cache size | 50000 |
| `execution.code_cache_size` | Code cache size | 1000 |
| `evm.enable_debug` | Enable EVM debug | false |
| `metrics.enabled` | Enable metrics | true |
| `metrics.listen_addr` | Metrics listen address | 0.0.0.0:9091 |

## Environment Variables

All config parameters can be overridden via environment variables:

```bash
export EVM_EXECUTOR_STORAGE_PIKA_ADDR="pika:9221"
export EVM_EXECUTOR_LOGGING_LEVEL="debug"
```

## Performance Tuning

### Cache Sizes

Adjust based on available memory:

```yaml
execution:
  state_cache_size: 20000      # Increase for more memory
  storage_cache_size: 100000   # Storage slots are small
  code_cache_size: 2000        # Code is large
```

### Worker Pools

Balance based on CPU cores:

```yaml
worker_pools:
  tx_executor:
    worker_count: 8            # CPU-bound, match cores
    queue_size: 200
```

### Batch Size

Trade latency for throughput:

```yaml
execution:
  batch_size: 20               # Process more blocks per iteration
```

## Storage Layout

### Pika Keys

```
# Progress
idx:sync:progress              → Sync progress JSON
idx:exec:progress              → Execution progress JSON

# Blocks
blk:hdr:{blockNum}             → Block header (RLP)
blk:txs:{blockNum}             → Transactions (RLP)
blk:rcpt:{blockNum}            → Receipts (RLP)

# State (historical, TTL=60min)
st:{blockNum}:acc:{address}    → Account JSON
st:{blockNum}:stor:{addr}:{key}→ Storage value

# State (latest, permanent)
st:latest:acc:{address}        → Account JSON
st:latest:stor:{addr}:{key}    → Storage value

# Code (permanent)
st:code:{codeHash}             → Bytecode
```

## Troubleshooting

### Execution lag increasing

Check:
1. Pika performance (use `INFO stats`)
2. Cache hit rates (metrics)
3. Worker pool queue lengths

### High memory usage

Reduce cache sizes:
```yaml
execution:
  state_cache_size: 5000
  storage_cache_size: 25000
```

### Transaction execution failures

Enable debug logging:
```yaml
logging:
  level: "debug"
evm:
  enable_debug: true
```

## Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## License

MIT License - see LICENSE file for details

## Support

- GitHub Issues: https://github.com/sunvim/evm_executor/issues
- Documentation: https://github.com/sunvim/evm_executor/wiki
