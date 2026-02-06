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
// CONFIGURATION
// ═══════════════════════════════════════════════════════════════

type Config struct {
	VaultAddress      string  `json:"vaultAddress"`      // Arbitrum Sepolia AuraVault address
	VIXAPIEndpoint    string  `json:"vixAPIEndpoint"`    // VIX data source
	AIEndpoint        string  `json:"aiEndpoint"`        // OpenAI/Gemini API
	RiskThreshold     float64 `json:"riskThreshold"`     // AI risk > 0.8 triggers action
	MinRebalanceDelay uint64  `json:"minRebalanceDelay"` // Seconds between rebalances
}

// ═══════════════════════════════════════════════════════════════
// STATE TYPES (Read from Blockchain)
// ═══════════════════════════════════════════════════════════════

type OnChainState struct {
	LastRiskScore     uint64 `json:"lastRiskScore"` // From vault's latestRisk
	LastVIXValue      uint64 `json:"lastVIXValue"`
	LastRebalanceTime uint64 `json:"lastRebalanceTime"`
	TotalRebalances   uint64 `json:"totalRebalances"`
	DefensiveAlloc    uint64 `json:"defensiveAlloc"`  // BPS
	AggressiveAlloc   uint64 `json:"aggressiveAlloc"` // BPS
}

// ═══════════════════════════════════════════════════════════════
// AI RESPONSE STRUCTURE
// ═══════════════════════════════════════════════════════════════

type AIRecommendation struct {
	RiskScore  float64 `json:"riskScore"` // 0.0 to 1.0
	Rationale  string  `json:"rationale"`
	Action     string  `json:"action"`     // "DEFENSIVE", "NEUTRAL", "AGGRESSIVE"
	Confidence float64 `json:"confidence"` // LLM confidence level
}

// ═══════════════════════════════════════════════════════════════
// MARKET DATA
// ═══════════════════════════════════════════════════════════════

type VIXData struct {
	Value     float64 `json:"value"` // e.g., 25.50
	Timestamp uint64  `json:"timestamp"`
}

type MarketSentiment struct {
	Score       float64 `json:"score"`  // -1.0 (bearish) to +1.0 (bullish)
	Source      string  `json:"source"` // "NewsAPI", "Twitter", etc.
	HeadlineTop string  `json:"headlineTop"`
}

// ═══════════════════════════════════════════════════════════════
// WORKFLOW ENTRYPOINT
// ═══════════════════════════════════════════════════════════════

func InitWorkflow(config *Config, logger *slog.Logger, secretsProvider cre.SecretsProvider) (cre.Workflow[*Config], error) {
	cronTrigger := cron.Trigger(&cron.Config{Schedule: "*/15 * * * *"}) // Every 15 minutes

	handler := func(cfg *Config, rt cre.Runtime, trg *cron.Payload) (*RebalanceResult, error) {
		return onCronTrigger(cfg, rt, secretsProvider, logger)
	}

	return cre.Workflow[*Config]{
		cre.Handler(cronTrigger, handler),
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// MAIN EXECUTION LOGIC (State-Aware AI Agent)
// ═══════════════════════════════════════════════════════════════

func onCronTrigger(
	config *Config,
	runtime cre.Runtime,
	secrets cre.SecretsProvider,
	logger *slog.Logger,
) (*RebalanceResult, error) {

	logger.Info("═════════════════════════════════════════════")
	logger.Info("🧠 AuraVault AI Agent - Institutional Mode")
	logger.Info("═════════════════════════════════════════════")

	// ═══════════════════════════════════════════════════════════════
	// STEP 1: READ-YOUR-WRITES - Fetch On-Chain State
	// ═══════════════════════════════════════════════════════════════
	logger.Info("📖 STEP 1: Reading on-chain state from AuraVault...")

	onChainState, err := readVaultState(runtime, config.VaultAddress, logger)
	if err != nil {
		logger.Error("Failed to read vault state", "error", err)
		return nil, err
	}

	logger.Info("✅ On-chain state loaded",
		"lastRebalance", onChainState.LastRebalanceTime,
		"totalRebalances", onChainState.TotalRebalances,
		"defensiveAlloc", fmt.Sprintf("%d BPS", onChainState.DefensiveAlloc))

	// Check if rebalance is allowed (time-based cooling period)
	currentTime := uint64(runtime.Now().Unix())
	timeSinceLastRebalance := currentTime - onChainState.LastRebalanceTime

	if timeSinceLastRebalance < config.MinRebalanceDelay {
		logger.Warn("⏸️  Rebalance cooling period active",
			"elapsed", timeSinceLastRebalance,
			"required", config.MinRebalanceDelay)
		return &RebalanceResult{
			Status:    "SKIPPED",
			Reason:    "Cooling period active",
			Timestamp: currentTime,
		}, nil
	}

	// ═══════════════════════════════════════════════════════════════
	// STEP 2: FETCH MARKET DATA
	// ═══════════════════════════════════════════════════════════════
	logger.Info("📊 STEP 2: Fetching VIX and sentiment data...")

	vixData, err := fetchVIXData(runtime, config.VIXAPIEndpoint, logger)
	if err != nil {
		logger.Error("Failed to fetch VIX", "error", err)
		return nil, err
	}

	sentiment, err := fetchMarketSentiment(runtime, logger)
	if err != nil {
		logger.Error("Failed to fetch sentiment", "error", err)
		// Continue with neutral sentiment on failure
		sentiment = &MarketSentiment{Score: 0.0, Source: "Fallback"}
	}

	logger.Info("✅ Market data loaded",
		"VIX", vixData.Value,
		"sentiment", sentiment.Score)

	// Calculate delta from previous execution
	vixDelta := vixData.Value - float64(onChainState.LastVIXValue)/100.0
	logger.Info("📈 VIX Delta", "change", fmt.Sprintf("%.2f", vixDelta))

	// ═══════════════════════════════════════════════════════════════
	// STEP 3: AI REASONING ENGINE
	// ═══════════════════════════════════════════════════════════════
	logger.Info("🤖 STEP 3: Consulting AI Risk Agent...")

	aiKey, _ := secrets.Get("OPENAI_API_KEY")

	aiRecommendation, err := consultAIAgent(
		runtime,
		config.AIEndpoint,
		string(aiKey),
		vixData,
		sentiment,
		onChainState,
		logger,
	)
	if err != nil {
		logger.Error("AI consultation failed", "error", err)
		return nil, err
	}

	logger.Info("✅ AI Analysis Complete",
		"riskScore", aiRecommendation.RiskScore,
		"action", aiRecommendation.Action,
		"confidence", aiRecommendation.Confidence)
	logger.Info("💭 AI Rationale", "reasoning", aiRecommendation.Rationale)

	// ═══════════════════════════════════════════════════════════════
	// STEP 4: DECISION ENGINE
	// ═══════════════════════════════════════════════════════════════
	logger.Info("⚖️  STEP 4: Making rebalance decision...")

	shouldRebalance := aiRecommendation.RiskScore > config.RiskThreshold

	if !shouldRebalance {
		logger.Info("✅ Risk within acceptable bounds",
			"aiRisk", aiRecommendation.RiskScore,
			"threshold", config.RiskThreshold)
		return &RebalanceResult{
			Status:    "NO_ACTION",
			Reason:    "Risk below threshold",
			AIRisk:    aiRecommendation.RiskScore,
			Timestamp: currentTime,
		}, nil
	}

	// ═══════════════════════════════════════════════════════════════
	// STEP 5: EXECUTE ON-CHAIN REBALANCE
	// ═══════════════════════════════════════════════════════════════
	logger.Info("🚨 STEP 5: HIGH RISK DETECTED - Triggering rebalance...")
	logger.Warn("⚠️  Risk Score Exceeds Threshold",
		"current", aiRecommendation.RiskScore,
		"threshold", config.RiskThreshold)

	txResult, err := executeRebalance(
		runtime,
		config.VaultAddress,
		aiRecommendation,
		vixData,
		logger,
	)
	if err != nil {
		logger.Error("Rebalance execution failed", "error", err)
		return nil, err
	}

	logger.Info("✅ Rebalance executed successfully", "tx", txResult)

	return &RebalanceResult{
		Status:      "EXECUTED",
		Reason:      aiRecommendation.Rationale,
		AIRisk:      aiRecommendation.RiskScore,
		Action:      aiRecommendation.Action,
		VIXValue:    vixData.Value,
		Transaction: txResult,
		Timestamp:   currentTime,
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// READ-YOUR-WRITES: Fetch State from Smart Contract
// ═══════════════════════════════════════════════════════════════

func readVaultState(
	runtime cre.Runtime,
	vaultAddress string,
	logger *slog.Logger,
) (*OnChainState, error) {

	// In production, this would use the EVM read capability:
	// 1. Call vault.getLatestRiskSnapshot()
	// 2. Call vault.getRebalanceStats()
	// 3. Parse ABI-encoded responses

	// For simulation, return mock state
	logger.Info("🔍 [SIMULATED] Reading vault state via EVM read capability...")

	return &OnChainState{
		LastRiskScore:     850000000000000000,                  // 0.85 scaled to 1e18
		LastVIXValue:      2350,                                // 23.50 scaled to 1e2
		LastRebalanceTime: uint64(runtime.Now().Unix()) - 3600, // 1 hour ago
		TotalRebalances:   5,
		DefensiveAlloc:    6500, // 65% defensive
		AggressiveAlloc:   3500, // 35% aggressive
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// FETCH VIX DATA
// ═══════════════════════════════════════════════════════════════

func fetchVIXData(
	runtime cre.Runtime,
	endpoint string,
	logger *slog.Logger,
) (*VIXData, error) {

	// In production: http.SendRequest to fetch real VIX
	// For simulation, return mock data
	logger.Info("📡 [SIMULATED] Fetching VIX from CBOE...")

	return &VIXData{
		Value:     27.85, // Elevated volatility
		Timestamp: uint64(runtime.Now().Unix()),
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// FETCH MARKET SENTIMENT
// ═══════════════════════════════════════════════════════════════

func fetchMarketSentiment(
	runtime cre.Runtime,
	logger *slog.Logger,
) (*MarketSentiment, error) {

	// In production: Call NewsAPI or Twitter API
	logger.Info("📰 [SIMULATED] Analyzing market sentiment...")

	return &MarketSentiment{
		Score:       -0.35, // Slightly bearish
		Source:      "NewsAPI",
		HeadlineTop: "Fed signals higher rates, markets react negatively",
	}, nil
}

// ═══════════════════════════════════════════════════════════════
// AI AGENT CONSULTATION (LLM Reasoning)
// ═══════════════════════════════════════════════════════════════

func consultAIAgent(
	runtime cre.Runtime,
	aiEndpoint string,
	apiKey string,
	vix *VIXData,
	sentiment *MarketSentiment,
	state *OnChainState,
	logger *slog.Logger,
) (*AIRecommendation, error) {

	// Build AI prompt
	systemPrompt := `You are a DeFi Risk Manager with institutional-grade standards.
Analyze the provided market data and recommend a vault rebalancing strategy.

OUTPUT FORMAT (JSON ONLY):
{
  "riskScore": 0.85,
  "rationale": "VIX at 27.85 indicates heightened volatility...",
  "action": "DEFENSIVE",
  "confidence": 0.92
}

RULES:
- riskScore: 0.0 (no risk) to 1.0 (extreme risk)
- action: "DEFENSIVE" (reduce risk), "NEUTRAL" (maintain), "AGGRESSIVE" (increase exposure)
- rationale: Clear, concise explanation for executives
`

	userPrompt := fmt.Sprintf(`Current Market Conditions:
- VIX Index: %.2f
- Market Sentiment: %.2f (%.2f = bearish, +1.0 = bullish)
- Headline: %s
- Current Allocation: %d%% defensive, %d%% aggressive
- Last Rebalance: %d second ago
- Total Rebalances: %d

Analyze the crash risk and recommend action.`,
		vix.Value,
		sentiment.Score,
		sentiment.Score,
		sentiment.HeadlineTop,
		state.DefensiveAlloc/100,
		state.AggressiveAlloc/100,
		uint64(runtime.Now().Unix())-state.LastRebalanceTime,
		state.TotalRebalances,
	)

	logger.Info("🤖 Calling AI Agent...", "endpoint", aiEndpoint)

	// In production: Use http capability to call OpenAI/Gemini
	// httpClient := http.NewClient(runtime, ...)
	// response := httpClient.Post(aiEndpoint, payload)

	// For simulation, return realistic AI response
	logger.Info("🧠 [SIMULATED] AI Agent processing...")

	aiResponse := &AIRecommendation{
		RiskScore:  0.87, // High risk due to VIX + bearish sentiment
		Rationale:  fmt.Sprintf("VIX at %.2f (elevated) combined with bearish sentiment (%.2f) indicates increased market stress. Recommend defensive posture to protect capital. Fed policy uncertainty compounds risk.", vix.Value, sentiment.Score),
		Action:     "DEFENSIVE",
		Confidence: 0.92,
	}

	logger.Info("✅ AI Response", "reasoning", aiResponse.Rationale)

	return aiResponse, nil
}

// ═══════════════════════════════════════════════════════════════
// EXECUTE REBALANCE ON-CHAIN
// ═══════════════════════════════════════════════════════════════

func executeRebalance(
	runtime cre.Runtime,
	vaultAddress string,
	ai *AIRecommendation,
	vix *VIXData,
	logger *slog.Logger,
) (string, error) {

	logger.Info("📝 Encoding rebalance transaction...")

	// ABI encode: vault.rebalance(uint256 riskScore, uint256 vixValue, string rationale, string action)
	riskScoreScaled := uint64(ai.RiskScore * 1e18)
	vixValueScaled := uint64(vix.Value * 100)

	// Function selector: keccak256("rebalance(uint256,uint256,string,string)")[:4] = 0xc4d66de8
	calldata := fmt.Sprintf("0xc4d66de8%064x%064x...", riskScoreScaled, vixValueScaled)
	// (String encoding omitted for brevity - would use ABI encoding library)

	logger.Info("📡 Submitting transaction to vault...",
		"vault", vaultAddress,
		"riskScore", ai.RiskScore,
		"action", ai.Action)

	// In production: Use GenerateReport to trigger chain write
	reportReq := &cre.ReportRequest{}
	reportPromise := runtime.GenerateReport(reportReq)
	report, err := reportPromise.Await()

	if err != nil {
		return "", fmt.Errorf("chain write failed: %w", err)
	}

	// Extract tx hash from report
	txHash := "0x7f4b5c2a...3b8e" // Would come from report.TxHash

	logger.Info("✅ Transaction confirmed", "tx", txHash)

	return txHash, nil
}

// ═══════════════════════════════════════════════════════════════
// RESULT TYPE
// ═══════════════════════════════════════════════════════════════

type RebalanceResult struct {
	Status      string  `json:"status"` // "EXECUTED", "SKIPPED", "NO_ACTION"
	Reason      string  `json:"reason"`
	AIRisk      float64 `json:"aiRisk"`
	Action      string  `json:"action"`
	VIXValue    float64 `json:"vixValue"`
	Transaction string  `json:"transaction"`
	Timestamp   uint64  `json:"timestamp"`
}

// ═══════════════════════════════════════════════════════════════
// WASM ENTRYPOINT
// ═══════════════════════════════════════════════════════════════

func main() {
	wasm.Run(InitWorkflow)
}
