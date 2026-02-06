# AuraProtocol - Technical Architecture Document

## Chainlink Convergence 2026 - Risk & Compliance Track

### Author: Senior Go & Blockchain Architect
### Date: 2026-02-06
### Version: 1.0.0

---

## Executive Summary

AuraProtocol implements a production-grade, Byzantine Fault Tolerant (BFT) data ingestion layer for Real-World Assets (RWA) using the Chainlink Runtime Environment (CRE). The system aggregates price data for GOLD and MICROSOFT (MSFT as OpenAI proxy) from multiple sources and achieves consensus through Decentralized Oracle Networks (DON).

## Technical Stack

- **Language**: Go 1.25.3
- **Runtime**: WASM (WebAssembly System Interface v1)
- **Framework**: Chainlink CRE SDK v1.1.3
- **APIs**: Alpha Vantage (Multi-endpoint)
- **Consensus**: Byzantine Fault Tolerance (67% quorum)

## Consensus Mechanism: Deep Dive

### How http.SendRequest Achieves BFT Consensus

The `http.SendRequest` function is the cornerstone of AuraProtocol's consensus mechanism. Here's the detailed flow:

#### Phase 1: Request Broadcast
```
Node 1 → Internet → API Endpoint
Node 2 → Internet → API Endpoint  
Node 3 → Internet → API Endpoint
Node 4 → Internet → API Endpoint
```

Each DON node independently fetches data from the same HTTP endpoint. This prevents any single node from manipulating the data source.

#### Phase 2: Response Hashing
```go
// Pseudocode representation
responseHash := sha256(httpResponse.Body)
```

Each node computes a cryptographic hash (SHA-256) of the HTTP response body. This ensures:
- Tamper detection
- Efficient comparison (32 bytes vs N kilobytes)
- Deterministic validation

#### Phase 3: Consensus Round
```
DON Network Voting:
Node 1: hash=0x1a2b3c... → Broadcast to all nodes
Node 2: hash=0x1a2b3c... → Broadcast to all nodes
Node 3: hash=0x1a2b3c... → Broadcast to all nodes
Node 4: hash=0xdeadbe... → Broadcast to all nodes (Byzantine node)
```

Nodes broadcast their hash values to all other nodes in the DON.

#### Phase 4: BFT Agreement
```
Hash Tally:
0x1a2b3c... → 3 votes (75%)
0xdeadbe... → 1 vote (25%)

BFT Threshold: 67% required
Result: CONSENSUS ACHIEVED ✓
```

If ≥67% of nodes have matching hashes, consensus is reached. The Byzantine node (Node 4) is automatically ignored.

#### Phase 5: Response Delivery
```go
// All honest nodes receive the agreed-upon data
consensusResponse := Response{
    Body: originalResponseBody,
    Status: 200,
    ConsensusReached: true,
}
```

### Handling Consensus Failures

If consensus cannot be reached (<67% agreement):

```go
// Error handling in main.go
resp, err := w.httpCapability.SendRequest(ctx, req)
if err != nil {
    // Log warning and continue with other sources
    fmt.Printf("[WARN] Failed to fetch: %v\n", err)
    continue
}
```

The system gracefully degrades by:
1. Logging the failure
2. Attempting other configured endpoints
3. Continuing if sufficient redundant sources exist
4. Failing only if NO sources achieve consensus

## Master Logic Algorithm

### Variance Computation

The Master Logic performs multi-layered variance analysis:

#### Level 1: Single-Asset Variance
```
For each asset (GOLD or MSFT):
1. Sort prices: O(n log n)
2. Calculate median: O(1)
3. Calculate weighted mean: O(n)
4. Compute variance: O(n)

Total: O(n log n)
```

**Why Median Over Mean?**
- Outlier resistance: Byzantine nodes may return extreme values
- Robust estimation: Median is the 50th percentile, unaffected by outliers
- Consensus alignment: Matches the BFT voting mechanism

#### Level 2: Cross-Asset Variance
```go
crossAssetVariance := abs(goldVariance - msftVariance)
```

This detects correlated volatility events:
- Market-wide crashes
- Sector-specific shocks
- Systemic risk events

#### Level 3: Risk Scoring
```go
systemRiskScore := (goldRiskScore + msftRiskScore) / 2.0
if crossAssetVariance > threshold {
    systemRiskScore += 2.0
}
```

## Performance Optimization

### Algorithm Complexity Analysis

| Operation | Complexity | Justification |
|-----------|-----------|---------------|
| Price sorting | O(n log n) | Quicksort for median calculation |
| Median calculation | O(1) | Direct array access after sort |
| Mean calculation | O(n) | Single pass accumulation |
| Variance computation | O(n) | Single pass after mean |
| **Total** | **O(n log n)** | Dominated by sorting |

### Memory Efficiency

```go
// Streaming aggregation - O(n) space
prices := make([]*PriceData, 0, len(endpoints))
```

- No duplicate storage
- Minimal heap allocations
- GC-friendly (short-lived objects)

### WASM Constraints

The code respects WASM limitations:

| Prohibited | AuraProtocol Approach |
|------------|----------------------|
| Filesystem I/O | ✅ Uses in-memory cache |
| Raw sockets | ✅ Uses CRE HTTP capability |
| Process spawning | ✅ Single-threaded execution |
| Non-determinism | ✅ Deterministic JSON parsing |

## Risk & Compliance Features

### 1. Multi-Source Verification

Each asset has ≥2 independent data sources:

**GOLD Sources:**
- GLD ETF quotes (Alpha Vantage)
- XAU/USD forex rates (Alpha Vantage)

**MSFT Sources:**
- Global quotes (Alpha Vantage)
- Intraday time series (Alpha Vantage)

### 2. Byzantine Fault Tolerance

Up to `(n-1)/3` nodes can be:
- Malicious (returning fake data)
- Offline (not responding)
- Compromised (under attack)

System remains secure as long as >67% nodes are honest.

### 3. Volatility Detection Tiers

| Tier | Variance | Risk Score | Action |
|------|----------|-----------|--------|
| Normal | 0-2% | 0.0 | None |
| Elevated | 2-5% | 4.0 | Log warning |
| High | 5-10% | 7.0 | Trigger alert |
| Extreme | >10% | 10.0 | Circuit breaker |

### 4. Circuit Breaker Logic

```go
if systemRiskScore >= 9.0 && circuitBreakerEnabled {
    alert = "CRITICAL: Circuit Breaker Activated - Trading Halted"
    // Prevent on-chain price updates
    // Notify compliance team
    // Pause all trading operations
}
```

### 5. Audit Trail

Every price observation includes:
- Timestamp (RFC3339 format)
- Source URL
- Response hash
- Node ID
- Consensus status

## Data Flow Diagram

```
┌─────────────────┐
│   CRE Trigger   │ (Cron: */5 * * * *)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Perform()       │
│ Entry Point     │
└────────┬────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌─────┐   ┌─────┐
│GOLD │   │MSFT │
└──┬──┘   └──┬──┘
   │         │
   │    ┌────┴────────┬─────────┐
   │    ▼             ▼         ▼
   │  Endpoint1   Endpoint2  Endpoint3
   │    │             │         │
   │    └─────┬───────┴─────┬───┘
   │          ▼             ▼
   │     ┌─────────┐   ┌─────────┐
   │     │  Node 1 │   │  Node 2 │
   │     └────┬────┘   └────┬────┘
   │          │             │
   │          └──────┬──────┘
   │                 ▼
   │           ┌──────────┐
   │           │ Consensus │ (BFT: ≥67%)
   │           └─────┬─────┘
   │                 │
   └─────────┬───────┘
             ▼
      ┌─────────────┐
      │ Master Logic │ O(n log n)
      │  - Median    │
      │  - Variance  │
      │  - Risk Score│
      └──────┬───────┘
             │
             ▼
      ┌─────────────┐
      │  Alert Check│ (Variance > 2%)
      └──────┬───────┘
             │
        ┌────┴────┐
        ▼         ▼
    ┌──────┐  ┌────────┐
    │On-Chain│ │Webhook │
    │Report │  │Alert   │
    └───────┘  └────────┘
```

## Error Handling Strategy

### 1. Graceful Degradation
```go
// Continue if some sources fail
if len(prices) == 0 {
    return nil, fmt.Errorf("no valid prices obtained")
}
```

### 2. Retry with Backoff
```go
http.Config{
    MaxRetries: 3,  // Exponential backoff
    Timeout: 30000, // 30 seconds
}
```

### 3. Circuit Breaker Pattern
```go
if failureRate > 0.5 {
    // Stop making requests
    // Return cached data
    // Alert operators
}
```

## Deployment Architecture

### Local Development
```bash
go run main.go
```

### WASM Production Build
```bash
GOOS=wasip1 GOARCH=wasm go build -o aura-protocol.wasm main.go
```

### CRE Deployment
```bash
chainlink-cre deploy \
  --workflow=workflow.yaml \
  --binary=aura-protocol.wasm \
  --network=testnet
```

## Testing Strategy

### Unit Tests
- Individual function testing
- Mock HTTP responses
- Variance calculation validation

### Integration Tests
- Multi-node DON simulation
- Byzantine node injection
- Network partition scenarios

### Performance Tests
- Latency benchmarks
- Memory profiling
- Consensus time measurement

## Security Considerations

### 1. API Key Management
```json
// config.json uses environment variable substitution
"apikey": "${ALPHA_VANTAGE_API_KEY}"
```

Never hardcode secrets in configuration files.

### 2. Input Validation
```go
// Validate all external data
if price <= 0 || price > maxReasonablePrice {
    return fmt.Errorf("invalid price: %f", price)
}
```

### 3. Rate Limiting
```
Alpha Vantage Free Tier: 5 requests/minute
Solution: 5-minute cron trigger + caching
```

### 4. HTTPS Enforcement
All HTTP requests use HTTPS to prevent MITM attacks.

## Future Enhancements

1. **Machine Learning Integration**
   - LSTM price prediction
   - Anomaly detection
   - Sentiment analysis

2. **Advanced Consensus**
   - Weighted voting by node reputation
   - Dynamic threshold adjustment
   - Multi-layer consensus (L1 + L2)

3. **Multi-Chain Support**
   - Ethereum mainnet
   - Polygon
   - Arbitrum

4. **Real-Time Streaming**
   - WebSocket connections
   - Event-driven updates
   - Sub-second latency

## Conclusion

AuraProtocol demonstrates production-grade engineering for the Risk & Compliance track:

✅ **Byzantine Fault Tolerance**: 67% BFT consensus  
✅ **Performance**: O(n log n) optimized algorithms  
✅ **WASM Compatible**: No prohibited syscalls  
✅ **Multi-Source**: Redundant data endpoints  
✅ **Risk Aware**: 4-tier volatility detection  
✅ **Production Ready**: Comprehensive error handling  

---

**End of Technical Architecture Document**
