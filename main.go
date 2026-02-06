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
	MAX_DEVIANCE = 0.05 // 5% maximum price deviation before triggering shield
)

// STATE MEMORY - Track previous execution prices
// In production, this would be stored in contract state or DON consensus memory
var (
	previousGoldPrice float64 = 0.0
	previousMsftPrice float64 = 0.0
	executionCount    int     = 0
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
	logger.Info(fmt.Sprintf("🚀 AuraProtocol - RWA Guard Demonstration [Iteration #%d]", executionCount))
	logger.Info("========================================")
	logger.Info("⚠️  HTTP capabilities not available in local simulation")
	logger.Info("📊 Running MULTI-ITERATION FLASH CRASH SCENARIO")

	// FLASH CRASH SCENARIO SIMULATION
	// Iteration 1: Normal market conditions
	// Iteration 2: Flash crash - Gold drops by 26% to trigger RWA Guard
	var goldPrice float64
	var msftPrice float64

	if executionCount == 1 {
		// First iteration: NORMAL MARKET CONDITIONS
		goldPrice = 243.75
		msftPrice = 438.20
		logger.Info("📈 SCENARIO: Normal Market Conditions")
	} else {
		// Second+ iterations: FLASH CRASH DETECTED
		goldPrice = 180.00 // 26% drop - way beyond 5% threshold
		msftPrice = 438.20 // MSFT remains stable
		logger.Info("💥 SCENARIO: Flash Crash Injected (Gold -26%)")
		logger.Info("🔍 Testing RWA Guard activation...")
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

	crossAssetVariance := math.Abs(goldConsensus.Variance - msftConsensus.Variance)
	systemRiskScore := (goldConsensus.RiskScore + msftConsensus.RiskScore) / 2.0

	alert := "Normal"

	// ═══════════════════════════════════════════════════════════════
	// 🛡️ RWA GUARD - PROTECTION CIRCUIT
	// ═══════════════════════════════════════════════════════════════
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

			logger.Error("🚨🚨🚨 CRITICAL ALERT 🚨🚨🚨")
			logger.Error(fmt.Sprintf("🛡️ RWA GUARD ACTIVATED: Gold price deviation %.2f%% exceeds %.2f%% threshold!",
				goldDeviation*100, MAX_DEVIANCE*100))
			logger.Error("🛡️ RWA GUARD: Market manipulation or flash crash detected. Shielding protocol.")
			logger.Error(fmt.Sprintf("🛡️ ACTION: SystemRiskScore → 10.0/10.0"))
			logger.Error(fmt.Sprintf("🛡️ STATUS: %s", alert))
			logger.Error("🚨 RECOMMENDATION: Halt trading, trigger circuit breaker, escalate to risk committee")
			logger.Error("========================================")
		} else {
			logger.Info(fmt.Sprintf("✅ RWA GUARD: Price movement within acceptable range (%.2f%% < %.2f%%)",
				goldDeviation*100, MAX_DEVIANCE*100))
			logger.Info("✅ STATUS: Protocol operating normally")
		}
	} else {
		logger.Info("ℹ️  First execution - establishing baseline prices")
	}

	// Update state memory for next iteration
	previousGoldPrice = goldPrice
	previousMsftPrice = msftPrice

	// Standard volatility check (secondary to RWA Guard)
	if crossAssetVariance > config.RiskParameters.HighVolatilityThreshold && alert != "CRITICAL_HALT" {
		alert = fmt.Sprintf("High Volatility: %.2f%%", crossAssetVariance)
		systemRiskScore += 2.0
		logger.Warn("⚠️  HIGH VOLATILITY DETECTED", "variance", crossAssetVariance)
	}

	logger.Info("========================================")
	logger.Info("✅ SIMULATION DATA PROCESSED")
	logger.Info("========================================")
	logger.Info(fmt.Sprintf("GOLD: $%.2f | MSFT: $%.2f", goldConsensus.MedianPrice, msftConsensus.MedianPrice))
	logger.Info(fmt.Sprintf("Risk Score: %.1f/10.0", systemRiskScore))
	logger.Info(fmt.Sprintf("Alert Status: %s", alert))
	logger.Info("========================================")

	// Trigger second iteration to demonstrate flash crash protection
	if executionCount == 1 {
		logger.Info("⏭️  Triggering second iteration to demonstrate flash crash scenario...")
		logger.Info("========================================")
		time.Sleep(100 * time.Millisecond) // Brief pause for log readability
		// Recursively call to simulate second execution
		return onCronTriggerWithMockData(config, runtime, trigger, apiKey)
	}

	return &ExecutionResult{
		GoldPrice:          goldConsensus.MedianPrice,
		MsftPrice:          msftConsensus.MedianPrice,
		GoldVariance:       goldConsensus.Variance,
		MsftVariance:       msftConsensus.Variance,
		CrossAssetVariance: crossAssetVariance,
		VolatilityWarning:  alert,
		SystemRiskScore:    systemRiskScore,
		Message:            fmt.Sprintf("SIMULATION: GOLD $%.2f | MSFT $%.2f | Risk %.1f | %s", goldConsensus.MedianPrice, msftConsensus.MedianPrice, systemRiskScore, alert),
		Timestamp:          time.Now(),
		DataSource:         "Mock Data with RWA Guard Protection (Flash Crash Scenario)",
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
