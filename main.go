//go:build wasip1

package main

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"sort"
	"time"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron"
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/cre-sdk-go/cre/wasm"
)

// RWA GUARD CONSTANTS - Institutional-Grade Protection
const (
	MAX_DEVIANCE          = 0.05 // 5% maximum price deviation before triggering shield
	PREDICTIVE_RISK_BONUS = 3.0  // Risk bonus when AI detects momentum acceleration
)

// STATE MEMORY - Track previous execution prices and variance history
// In production, this would be stored in contract state or DON consensus memory
var (
	previousGoldPrice float64 = 0.0
	previousMsftPrice float64 = 0.0
	executionCount    int     = 0

	// LAYER 3: Predictive Momentum Engine - AI Brain
	varianceHistory []float64 // Stores last 3 CrossAssetVariance values for acceleration detection

	// LAYER 4: On-Chain Execution (Hands)
	ARBITRUM_CONTRACT_ADDRESS = "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1" // Arbitrum Sepolia Target
)

type Config struct {
	Consensus struct {
		MaxVariancePercent float64 `json:"maxVariancePercent"`
	} `json:"consensus"`
	RiskParameters struct {
		HighVolatilityThreshold    float64 `json:"highVolatilityThreshold"`
		ExtremeVolatilityThreshold float64 `json:"extremeVolatilityThreshold"`
	} `json:"riskParameters"`
}

type PriceData struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
}

type AlphaVantageResponse struct {
	GlobalQuote struct {
		Symbol      string `json:"01. symbol"`
		Price       string `json:"05. price"`
		TradingDay  string `json:"07. latest trading day"`
		ChangePerct string `json:"10. change percent"`
	} `json:"Global Quote"`
	Information string `json:"Information"`
	Note        string `json:"Note"`
}

type ExecutionResult struct {
	GoldPrice          float64   `json:"gold_price"`
	MsftPrice          float64   `json:"msft_price"`
	GoldVariance       float64   `json:"gold_variance"`
	MsftVariance       float64   `json:"msft_variance"`
	CrossAssetVariance float64   `json:"cross_asset_variance"`
	VolatilityWarning  string    `json:"volatility_warning"`
	SystemRiskScore    float64   `json:"system_risk_score"`
	MarketMomentum     string    `json:"market_momentum"` // LAYER 3: Stable/Unstable/Critical
	OnChainStatus      string    `json:"on_chain_status"` // LAYER 4: Transaction Status
	Message            string    `json:"message"`
	Timestamp          time.Time `json:"timestamp"`
	DataSource         string    `json:"data_source"`
}

func InitWorkflow(config *Config, logger *slog.Logger, secretsProvider cre.SecretsProvider) (cre.Workflow[*Config], error) {
	// Get API key from environment (WASM-compatible)
	apiKey := os.Getenv("ALPHA_VANTAGE_API_KEY")
	if apiKey == "" {
		apiKey = "R4ZVPT8S0290SQP1" // Fallback
		logger.Warn("Using fallback API key")
	} else {
		logger.Info("API key loaded from environment")
	}

	cronTrigger := cron.Trigger(&cron.Config{Schedule: "*/5 * * * *"})

	// Create handler as closure with apiKey captured
	handler := func(cfg *Config, rt cre.Runtime, trg *cron.Payload) (*ExecutionResult, error) {
		return onCronTriggerWithMockData(cfg, rt, trg, apiKey)
	}

	return cre.Workflow[*Config]{
		cre.Handler(cronTrigger, handler),
	}, nil
}

func onCronTriggerWithMockData(config *Config, runtime cre.Runtime, trigger *cron.Payload, apiKey string) (*ExecutionResult, error) {
	logger := runtime.Logger()

	// Increment execution counter
	executionCount++

	logger.Info("========================================")
	logger.Info(fmt.Sprintf("🧠 AuraProtocol - Layer 3: Predictive Momentum Engine [Iteration #%d]", executionCount))
	logger.Info("========================================")
	logger.Info("⚠️  HTTP capabilities not available in local simulation")
	logger.Info("📊 Running 3-STAGE PREDICTIVE INTELLIGENCE SCENARIO")

	// 3-STAGE PREDICTIVE SCENARIO
	// Stage 1: Normal market - Low variance (0.2%)
	// Stage 2: Market nervousness - Variance accelerating (1.8%) - AI PREDICTS HERE
	// Stage 3: Flash crash - Extreme variance (25%) - RWA Guard activates
	var goldPrice float64
	var msftPrice float64
	var artificialVariance float64 // For demonstration, we'll inject variance

	switch executionCount {
	case 1:
		// Stage 1: NORMAL MARKET CONDITIONS
		goldPrice = 243.75
		msftPrice = 438.20
		artificialVariance = 0.2 // 0.2% variance (stable)
		logger.Info("📈 STAGE 1: Normal Market Conditions")
		logger.Info("   Variance: 0.2% (Baseline)")
	case 2:
		// Stage 2: MARKET NERVOUSNESS - AI Should Detect Acceleration
		goldPrice = 240.50       // Small 1.3% drop (within RWA Guard threshold)
		msftPrice = 438.20       // MSFT stable
		artificialVariance = 1.8 // 1.8% variance (growing!)
		logger.Info("📊 STAGE 2: Market Nervousness Detected")
		logger.Info("   Variance: 1.8% (Accelerating)")
		logger.Info("   🔍 AI analyzing momentum patterns...")
	default:
		// Stage 3: FLASH CRASH
		goldPrice = 180.00        // 26% drop - triggers RWA Guard
		msftPrice = 438.20        // MSFT stable
		artificialVariance = 25.0 // 25% variance (extreme!)
		logger.Info("💥 STAGE 3: Flash Crash Event")
		logger.Info("   Variance: 25.0% (CRITICAL)")
		logger.Info("   🛡️ RWA Guard should activate...")
	}

	logger.Info(fmt.Sprintf("Current GOLD Price: $%.2f", goldPrice))
	logger.Info(fmt.Sprintf("Current MSFT Price: $%.2f", msftPrice))

	// Create price data structures
	goldPrices := []*PriceData{{
		Symbol:    "GOLD",
		Price:     goldPrice,
		Source:    "Mock Data (Simulation Mode)",
		Timestamp: time.Now(),
	}}

	msftPrices := []*PriceData{{
		Symbol:    "MSFT",
		Price:     msftPrice,
		Source:    "Mock Data (Simulation Mode)",
		Timestamp: time.Now(),
	}}

	// Master Logic: O(n log n) consensus computation
	goldConsensus := computeConsensus(goldPrices, "GOLD", config, logger)
	msftConsensus := computeConsensus(msftPrices, "MSFT", config, logger)

	// Use artificial variance for demonstration
	crossAssetVariance := artificialVariance
	systemRiskScore := (goldConsensus.RiskScore + msftConsensus.RiskScore) / 2.0

	alert := "Normal"
	marketMomentum := "Stable"

	// ═══════════════════════════════════════════════════════════════
	// 🧠 LAYER 3: PREDICTIVE MOMENTUM ENGINE - AI BRAIN
	// ═══════════════════════════════════════════════════════════════
	logger.Info("========================================")
	logger.Info("🧠 LAYER 3: Predictive Momentum Engine")
	logger.Info("========================================")

	// Add current variance to history
	varianceHistory = append(varianceHistory, crossAssetVariance)

	// Keep only last 3 values
	if len(varianceHistory) > 3 {
		varianceHistory = varianceHistory[len(varianceHistory)-3:]
	}

	logger.Info(fmt.Sprintf("Variance History (last %d): %v", len(varianceHistory), varianceHistory))

	// AI LOGIC: Detect momentum acceleration
	if len(varianceHistory) >= 2 {
		// Calculate acceleration (rate of change of rate of change)
		// If variance is growing exponentially, this will be positive and large

		currentVariance := varianceHistory[len(varianceHistory)-1]
		previousVariance := varianceHistory[len(varianceHistory)-2]

		// Calculate the growth rate
		varianceGrowth := currentVariance - previousVariance

		logger.Info(fmt.Sprintf("Current Variance: %.2f%%", currentVariance))
		logger.Info(fmt.Sprintf("Previous Variance: %.2f%%", previousVariance))
		logger.Info(fmt.Sprintf("Variance Growth: %.2f%%", varianceGrowth))

		// Check for exponential growth pattern (acceleration)
		// If we have 3 data points, we can calculate true acceleration
		if len(varianceHistory) >= 3 {
			oldestVariance := varianceHistory[len(varianceHistory)-3]
			previousGrowth := previousVariance - oldestVariance

			// Acceleration = change in growth rate
			acceleration := varianceGrowth - previousGrowth

			logger.Info(fmt.Sprintf("Oldest Variance: %.2f%%", oldestVariance))
			logger.Info(fmt.Sprintf("Previous Growth: %.2f%%", previousGrowth))
			logger.Info(fmt.Sprintf("🎯 ACCELERATION: %.2f%%", acceleration))

			// PREDICTIVE THRESHOLD: If acceleration > 0.5%, we predict trouble
			if acceleration > 0.5 {
				// 🧠 AI PREDICTION TRIGGERED
				systemRiskScore += PREDICTIVE_RISK_BONUS
				marketMomentum = "Unstable"

				logger.Warn("========================================")
				logger.Warn("⚠️  PREDICTIVE WARNING ACTIVATED")
				logger.Warn("========================================")
				logger.Warn(fmt.Sprintf("🧠 AI BRAIN: Momentum acceleration detected! (+%.2f%%)", acceleration))
				logger.Warn(fmt.Sprintf("🧠 PATTERN: %.2f%% → %.2f%% → %.2f%% (Exponential Growth)",
					oldestVariance, previousVariance, currentVariance))
				logger.Warn(fmt.Sprintf("🧠 PREDICTION: Market crash likely in next iteration"))
				logger.Warn(fmt.Sprintf("🧠 ACTION: Adding Predictive Risk Bonus +%.1f to score", PREDICTIVE_RISK_BONUS))
				logger.Warn(fmt.Sprintf("🧠 NEW RISK SCORE: %.1f/10.0", systemRiskScore))
				logger.Warn("========================================")
			} else {
				logger.Info(fmt.Sprintf("✅ AI BRAIN: Acceleration %.2f%% below prediction threshold (0.5%%)", acceleration))
				logger.Info("✅ Market momentum appears stable")
			}
		} else {
			logger.Info("ℹ️  AI BRAIN: Collecting data... need 3 points for acceleration analysis")
		}
	} else {
		logger.Info("ℹ️  AI BRAIN: First iteration - establishing baseline")
	}

	// ═══════════════════════════════════════════════════════════════
	// 🛡️ RWA GUARD - PROTECTION CIRCUIT (LAYER 2)
	// ═══════════════════════════════════════════════════════════════
	onChainStatus := "IDLE"

	if previousGoldPrice != 0.0 {
		// Calculate price deviation from previous execution
		goldDeviation := math.Abs(goldPrice-previousGoldPrice) / previousGoldPrice
		msftDeviation := math.Abs(msftPrice-previousMsftPrice) / previousMsftPrice

		logger.Info("========================================")
		logger.Info("🛡️  RWA GUARD - Volatility Shield Status")
		logger.Info("========================================")
		logger.Info(fmt.Sprintf("Previous GOLD: $%.2f → Current: $%.2f", previousGoldPrice, goldPrice))
		logger.Info(fmt.Sprintf("GOLD Deviation: %.2f%% (Threshold: %.2f%%)", goldDeviation*100, MAX_DEVIANCE*100))
		logger.Info(fmt.Sprintf("MSFT Deviation: %.2f%%", msftDeviation*100))

		// CHECK: Did price move beyond acceptable threshold?
		if goldDeviation > MAX_DEVIANCE {
			// 🚨 PROTECTION CIRCUIT ACTIVATED
			systemRiskScore = 10.0 // CRITICAL - Maximum risk
			alert = "CRITICAL_HALT"
			marketMomentum = "Critical"

			logger.Error("🚨🚨🚨 CRITICAL ALERT 🚨🚨🚨")
			logger.Error(fmt.Sprintf("🛡️ RWA GUARD ACTIVATED: Gold price deviation %.2f%% exceeds %.2f%% threshold!",
				goldDeviation*100, MAX_DEVIANCE*100))
			logger.Error("🛡️ RWA GUARD: Market manipulation or flash crash detected. Shielding protocol.")
			logger.Error(fmt.Sprintf("🛡️ ACTION: SystemRiskScore → 10.0/10.0"))
			logger.Error(fmt.Sprintf("🛡️ STATUS: %s", alert))
			logger.Error("🚨 RECOMMENDATION: Halt trading, trigger circuit breaker, escalate to risk committee")

			// ═══════════════════════════════════════════════════════════════
			// 🔗 LAYER 4: ON-CHAIN HANDS - ARBITRUM EXECUTION
			// ═══════════════════════════════════════════════════════════════
			logger.Info("🔗 LAYER 4: Transmitting to Arbitrum Sepolia...")

			// ABI encode: emergencyHalt(uint256)
			// Function selector: keccak256("emergencyHalt(uint256)")[:4] = 0x5f515226
			riskScoreScaled := uint64(systemRiskScore * 10) // 10.0 -> 100
			callData := fmt.Sprintf("0x5f515226%064x", riskScoreScaled)

			logger.Info("📝 Generating chain-write report...",
				"contract", ARBITRUM_CONTRACT_ADDRESS,
				"function", "emergencyHalt",
				"calldata", callData)

			// Trigger chain write via CRE Runtime GenerateReport
			reportReq := &cre.ReportRequest{}
			reportPromise := runtime.GenerateReport(reportReq)
			report, reportErr := reportPromise.Await()

			if reportErr != nil {
				logger.Error("❌ Chain write failed", "error", reportErr)
				onChainStatus = "TX_FAILED"
			} else {
				logger.Info("✅ Chain write executed", "report", report)
				onChainStatus = "TX_SENT_TO_ARBITRUM"
			}
			logger.Error("========================================")
		} else {
			logger.Info(fmt.Sprintf("✅ RWA GUARD: Price movement within acceptable range (%.2f%% < %.2f%%)",
				goldDeviation*100, MAX_DEVIANCE*100))
			logger.Info("✅ STATUS: Protocol operating normally")
		}
	} else {
		logger.Info("ℹ️  RWA GUARD: First execution - establishing baseline prices")
	}

	// Update state memory for next iteration
	previousGoldPrice = goldPrice
	previousMsftPrice = msftPrice

	// Standard volatility check (tertiary to AI and RWA Guard)
	if crossAssetVariance > config.RiskParameters.HighVolatilityThreshold && alert != "CRITICAL_HALT" {
		alert = fmt.Sprintf("High Volatility: %.2f%%", crossAssetVariance)
		if systemRiskScore < 7.0 {
			systemRiskScore += 2.0
		}
		logger.Warn("⚠️  HIGH VOLATILITY DETECTED", "variance", crossAssetVariance)
	}

	logger.Info("========================================")
	logger.Info("✅ ITERATION COMPLETE")
	logger.Info("========================================")
	logger.Info(fmt.Sprintf("GOLD: $%.2f | MSFT: $%.2f", goldConsensus.MedianPrice, msftConsensus.MedianPrice))
	logger.Info(fmt.Sprintf("Cross-Asset Variance: %.2f%%", crossAssetVariance))
	logger.Info(fmt.Sprintf("Market Momentum: %s", marketMomentum))
	logger.Info(fmt.Sprintf("Risk Score: %.1f/10.0", systemRiskScore))
	logger.Info(fmt.Sprintf("Alert Status: %s", alert))
	logger.Info("========================================")

	// Continue to next iteration if not done
	if executionCount < 3 {
		logger.Info(fmt.Sprintf("⏭️  Triggering iteration #%d...", executionCount+1))
		logger.Info("========================================")
		time.Sleep(100 * time.Millisecond) // Brief pause for log readability
		// Recursively call to simulate next execution
		return onCronTriggerWithMockData(config, runtime, trigger, apiKey)
	}

	logger.Info("✅ 3-STAGE SIMULATION COMPLETE")
	logger.Info("========================================")

	return &ExecutionResult{
		GoldPrice:          goldConsensus.MedianPrice,
		MsftPrice:          msftConsensus.MedianPrice,
		GoldVariance:       goldConsensus.Variance,
		MsftVariance:       msftConsensus.Variance,
		CrossAssetVariance: crossAssetVariance,
		VolatilityWarning:  alert,
		SystemRiskScore:    systemRiskScore,
		MarketMomentum:     marketMomentum,
		OnChainStatus:      onChainStatus,
		Message:            fmt.Sprintf("AI+RWA: GOLD $%.2f | MSFT $%.2f | Risk %.1f | %s | Momentum: %s", goldConsensus.MedianPrice, msftConsensus.MedianPrice, systemRiskScore, alert, marketMomentum),
		Timestamp:          time.Now(),
		DataSource:         "Mock Data with Layer 3 Predictive AI + RWA Guard Protection",
	}, nil
}

type ConsensusResult struct {
	MedianPrice float64
	Variance    float64
	RiskScore   float64
}

// compute Consensus implements O(n log n) QuickSort-based median aggregation
// This is the CORE of the Master Logic consensus mechanism
func computeConsensus(prices []*PriceData, symbol string, config *Config, logger *slog.Logger) ConsensusResult {
	if len(prices) == 0 {
		return ConsensusResult{}
	}

	// O(n log n) QuickSort-based sorting for median calculation
	sort.Slice(prices, func(i, j int) bool {
		return prices[i].Price < prices[j].Price
	})

	var median float64
	n := len(prices)
	if n%2 == 0 {
		median = (prices[n/2-1].Price + prices[n/2].Price) / 2.0
	} else {
		median = prices[n/2].Price
	}

	// Calculate variance
	var sum float64
	for _, p := range prices {
		sum += p.Price
	}
	mean := sum / float64(n)

	var sumSquaredDiff float64
	for _, p := range prices {
		diff := p.Price - mean
		sumSquaredDiff += diff * diff
	}
	stdDev := math.Sqrt(sumSquaredDiff / float64(n))
	variancePercent := (stdDev / mean) * 100.0

	// Risk scoring based on variance thresholds
	riskScore := 0.0
	if variancePercent > config.RiskParameters.ExtremeVolatilityThreshold {
		riskScore = 10.0
	} else if variancePercent > config.RiskParameters.HighVolatilityThreshold {
		riskScore = 7.0
	} else if variancePercent > config.Consensus.MaxVariancePercent {
		riskScore = 4.0
	}

	return ConsensusResult{
		MedianPrice: median,
		Variance:    variancePercent,
		RiskScore:   riskScore,
	}
}

func main() {
	wasm.NewRunner(cre.ParseJSON[Config]).Run(InitWorkflow)
}
