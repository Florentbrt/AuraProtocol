# AuraProtocol v2: Institutional RWA Orchestration Layer

> **Submitted to the Chainlink Convergence 2026 Hackathon**
> *Unifying global liquidity by orchestrating verifiable, compliant cross-chain workflows via the Chainlink Runtime Environment (CRE).*

![Status](https://img.shields.io/badge/Status-Production%20Ready-success?style=flat-square)
![Architecture](https://img.shields.io/badge/Architecture-Institutional--Grade-375BD2?style=flat-square)
![Chainlink](https://img.shields.io/badge/Powered%20By-Chainlink%20CRE-375BD2?style=flat-square)

## 🌐 The Convergence Thesis (Problem Statement)
Global finance faces a critical bottleneck: **Capital Fragmentation**.
Trillions of dollars in tokenized Real-World Assets (RWAs) are currently isolated on private bank chains ("gardens"), unable to access the deep liquidity of public DeFi markets. Manual settlement between these environments introduces T+2 latency, counterparty risk, and operational opacity.

**The Solution:**
AuraProtocol v2 is not merely a bridge; it is a **Cross-Chain Orchestration Layer**. It leverages the **Chainlink Runtime Environment (CRE)** to execute atomic, verifiable, and regulatory-compliant workflows that synchronize state between legacy institutional systems and public blockchains.

---

## 🏗️ Technical Architecture

AuraProtocol v2 implements a "defense-in-depth" architecture where off-chain computation is cryptographically verified before any on-chain state change occurs.

```mermaid
graph TD
    subgraph "The Orchestration Layer (Chainlink CRE)"
        A[Workflow DON] -->|1. Trigger (Cron)| B(Verification Engine / WASM)
        B -->|2. Capability: Market Data| C[Fetch VIX/Volatility]
        B -->|3. Capability: Compute| D[Risk Model Consensus]
        D -->|4. Output| E[Signed OCR Report]
    end
    
    subgraph "The Settlement Layer (Arbitrum Sepolia)"
        E -->|5. Verify & Actuate| F{AuraVault / ERC-4626}
        F -->|6. Rebalance| G[Asset Allocation]
    end
```

### 1. The Orchestration Engine (CRE)
We utilize the CRE as secure middleware to abstract complexity:
- **Workflow**: A unified Go (WASM) binary manages the entire risk lifecycle.
- **Capabilities**: Modular functions fetch external market data (VIX) and read EVM state without relying on centralized servers.
- **Consensus**: Risk calculations are performed off-chain but verified by a committee of nodes (DON) via BFT consensus.

### 2. The Settlement Layer (Smart Contracts)
- **Standard**: **ERC-4626** Institutional Vault (Solidity 0.8.24).
- **Zero-Trust**: The contract strictly validates signed reports from the CRE DON.
- **Safety Modules**:
  - **Circuit Breakers**: Reverts if rebalance moves funds > 20% in one tx.
  - **Oracle Freshness**: Enforces < 3h staleness check on Chainlink Price Feeds.

---

## 🏆 Innovation & Chainlink Integration

This project demonstrates the **"Effective Use of CRE"** by orchestrating a complex, multi-step workflow rather than simply consuming a data feed.

| Technology | Implementation in AuraProtocol v2 | Prize Track Relevance |
|------------|-----------------------------------|-----------------------|
| **Chainlink Runtime (CRE)** | Orchestrates the end-to-end risk & settlement workflow using discrete Capabilities. | **Core Prize / CRE** |
| **Workflow DON** | Monitors off-chain risk parameters (VIX) and triggers preemptive on-chain rebalances. | **DeFi Track** |
| **Chainlink Data Feeds** | On-chain validation of asset prices via `AggregatorV3Interface`. | **DeFi Track** |
| **Stateless Architecture** | Implements "Read-Your-Writes" pattern to maintain consistency without local databases. | **Technical Excellence** |

---

## 🛠️ Challenges & Accomplishments

### Challenges We Overcame
1.  **Asynchronous State Synchronization**: Coordinating the CRE's asynchronous "Promise" based architecture with the synchronous nature of EVM transactions required implementing a robust "Read-Your-Writes" pattern. The agent explicitly reads the on-chain state it wrote in the previous run to ensure continuity.
2.  **Deterministic Compilation (WASM)**: Ensuring the Go binary compiled correctly for `wasip1` while managing complex dependencies (HTTP/Cron capabilities) required strict version management of the CRE SDK (v1.1.3).

### Accomplishments
- **Production-Grade Security**: Unlike many hackathon projects, this protocol includes role-based access control (RBAC), circuit breakers, and pausable guarding logic from day one.
- **Fail-Safe Design**: We implemented a deterministic fallback model. If the primary AI risk API becomes unreachable, the CRE agent autonomously switches to a volatility-based mathematical model to preserve asset safety.

---

## 👨‍⚖️ Judge's Evaluation Guide

We have provided a simulation scripts to verify the workflow logic without needing to provision a full DON.

### 1. Repository Structure
- `contracts/`: Solidity smart contracts (AuraVault, AuraShield).
- `main.go`: The CRE Workflow logic (Go/WASM).
- `cre.yaml` & `workflow.yaml`: Workflow configuration and Don definitions.

### 2. Verification Steps (Simulation)

To verify the orchestration logic (Trigger -> Data Fetch -> Logic -> Chain Write):

```bash
# 1. Install Dependencies
forge install

# 2. Run CRE Simulation
# This mimics the execution of the workflow on a decentralized node
cre workflow simulate . -T local --env .env
```

**Success Criteria:**
The logs will demonstrate the agent reading the simulated contract state, calculating the risk score, and generating a valid transaction report (Exit Code 0).

---

## 📜 License
This project is licensed under the MIT License.
