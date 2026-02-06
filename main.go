//go:build wasip1

package main

import (
	"fmt"
	"log/slog"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron"
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/cre-sdk-go/cre/wasm"
)

// ═══════════════════════════════════════════════════════════════
// CONFIGURATION STRUCTURES
// ═══════════════════════════════════════════════════════════════

type Config struct {
	VaultAddress   string `json:"vaultAddress"`
	VIXEndpoint    string `json:"vixEndpoint"`
	OpenAIEndpoint string `json:"openAIEndpoint"`
}

type RiskSnapshot struct {
	LastUpdate  uint64 `json:"lastUpdate"`
	RiskScore   uint64 `json:"riskScore"`
	AIRationale string `json:"aiRationale"`
}

type Result struct {
	OldRisk     uint64 `json:"oldRisk"`
	NewRisk     uint64 `json:"newRisk"`
	ActionTaken bool   `json:"actionTaken"`
	Rationale   string `json:"rationale"`
	Method      string `json:"method"`
}

// ═══════════════════════════════════════════════════════════════
// WORKFLOW INITIALIZATION (Spec Generation Phase)
// ═══════════════════════════════════════════════════════════════

func InitWorkflow(config *Config, logger *slog.Logger, secretsProvider cre.SecretsProvider) (cre.Workflow[*Config], error) {
	// Validation: Prevent "nil spec response" by validating config early
	if config.VaultAddress == "" {
		return cre.Workflow[*Config]{}, fmt.Errorf("vaultAddress is required in config.json")
	}

	// Construct Workflow with Cron Trigger
	cronTrigger := cron.Trigger(&cron.Config{Schedule: "0 * * * *"}) // Hourly

	handler := func(cfg *Config, rt cre.Runtime, trg *cron.Payload) (*Result, error) {
		return executeStrategy(cfg, rt, logger)
	}

	return cre.Workflow[*Config]{
		cre.Handler(cronTrigger, handler),
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// EXECUTION STRATEGY (Runtime Execution Phase)
// ═══════════════════════════════════════════════════════════════

func executeStrategy(config *Config, runtime cre.Runtime, logger *slog.Logger) (*Result, error) {
	logger.Info("🌀 AuraProtocol v2 Agent Starting...")

	// STEP 1: READ-YOUR-WRITES - Fetch On-Chain State
	snapshot, err := fetchOnChainState(runtime, config.VaultAddress, logger)
	if err != nil {
		return nil, err
	}
	logger.Info("📖 Context Loaded", "prev_risk", snapshot.RiskScore)

	// STEP 2: OBSERVE - Fetch Market Data
	vixValue := 22.5 // In production: fetch from VIX API
	logger.Info("📊 Market Data", "VIX", vixValue)

	// STEP 3: THINK - Calculate Risk (Deterministic Fallback)
	newRiskScore, rationale, method := calculateRisk(runtime, config, vixValue, snapshot.RiskScore, logger)
	logger.Info("🤔 Decision", "score", newRiskScore, "method", method)

	// STEP 4: DECIDE - Check Threshold
	diff := int64(newRiskScore) - int64(snapshot.RiskScore)
	if diff < 0 {
		diff = -diff
	}

	// STEP 5: ACT - Execute if threshold exceeded
	actionTaken := false
	if diff > 5 {
		logger.Info("🚀 Threshold exceeded - triggering rebalance")
		if err := executeRebalance(runtime, config.VaultAddress, newRiskScore, rationale, logger); err != nil {
			return nil, err
		}
		actionTaken = true
	} else {
		logger.Info("💤 Threshold not met - holding position")
	}

	return &Result{
		OldRisk:     snapshot.RiskScore,
		NewRisk:     newRiskScore,
		ActionTaken: actionTaken,
		Rationale:   rationale,
		Method:      method,
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// BUSINESS LOGIC FUNCTIONS
// ═══════════════════════════════════════════════════════════════

func calculateRisk(rt cre.Runtime, cfg *Config, vix float64, prevRisk uint64, logger *slog.Logger) (uint64, string, string) {
	// Deterministic Fallback Formula (as per Phase 2 requirements)
	// Risk = (VIX / 20) * 100, clamped to 100
	calc := (vix / 20.0) * 100.0
	if calc > 100 {
		calc = 100
	}

	return uint64(calc), fmt.Sprintf("FALLBACK: VIX %.2f", vix), "FALLBACK_DETERMINISTIC"
}

func fetchOnChainState(rt cre.Runtime, addr string, logger *slog.Logger) (*RiskSnapshot, error) {
	logger.Info("📡 Reading on-chain state from vault", "address", addr)
	// In production: use EVM read capability
	// rt.ReadContract(...)
	return &RiskSnapshot{
		LastUpdate:  1234567890,
		RiskScore:   50,
		AIRationale: "Initial State",
	}, nil
}

func executeRebalance(rt cre.Runtime, addr string, risk uint64, rationale string, logger *slog.Logger) error {
	logger.Info("🔗 Requesting chain write", "risk", risk, "rationale", rationale)

	// Generate report for on-chain transaction
	// In production, this would encode the rebalance(uint256 riskScore, string rationale) call
	// For simulation, we use a simple encoded payload

	// Simple ABI encoding simulation (32 bytes for uint256)
	encodedPayload := make([]byte, 32)
	// Risk score in big-endian format (simplified for simulation)
	encodedPayload[31] = byte(risk)

	reportReq := &cre.ReportRequest{
		EncoderName:    "evm",       // EVM ABI encoding
		SigningAlgo:    "ecdsa",     // Ethereum standard signing
		HashingAlgo:    "keccak256", // Ethereum standard hashing
		EncodedPayload: encodedPayload,
	}

	result, err := rt.GenerateReport(reportReq).Await()
	if err != nil {
		return err
	}

	logger.Info("✅ Chain write successful", "result", result)
	return nil
}

// ═══════════════════════════════════════════════════════════════
// WASM ENTRYPOINT (Critical for Spec Generation)
// ═══════════════════════════════════════════════════════════════

func main() {
	// CORRECT SYNTAX per SDK v1.1.3 documentation
	// wasm.NewRunner instantiates the lifecycle manager
	// cre.ParseJSON[Config] provides type-safe config parsing
	// .Run(InitWorkflow) starts the event loop
	wasm.NewRunner(cre.ParseJSON[Config]).Run(InitWorkflow)
}
