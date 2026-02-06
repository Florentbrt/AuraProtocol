//go:build wasip1

package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sort"
	"time"

	"github.com/smartcontractkit/cre-sdk-go/capabilities/networking/http"
	"github.com/smartcontractkit/cre-sdk-go/capabilities/scheduler/cron"
	"github.com/smartcontractkit/cre-sdk-go/cre"
	"github.com/smartcontractkit/cre-sdk-go/cre/wasm"
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
		return onCronTriggerWithCapability(cfg, rt, trg, apiKey)
	}

	return cre.Workflow[*Config]{
		cre.Handler(cronTrigger, handler),
	}, nil
}

func onCronTriggerWithCapability(config *Config, runtime cre.Runtime, trigger *cron.Payload, apiKey string) (*ExecutionResult, error) {
	logger := runtime.Logger()

	logger.Info("========================================")
	logger.Info("🚀 AuraProtocol - LIVE RWA Data Ingestion Via CRE HTTP Capability")
	logger.Info("========================================")
	logger.Info("🔑 Using Alpha Vantage API")

	// Create HTTP client
	httpClient := &http.Client{}

	// Fetch GOLD price using CRE HTTP capability
	goldPrices, err := fetchLiveGoldPrice(runtime, httpClient, apiKey, logger)
	if err != nil {
		logger.Error("Failed to fetch GOLD price", "error", err)
		return nil, err
	}

	// Fetch MSFT price using CRE HTTP capability
	msftPrices, err := fetchLiveMsftPrice(runtime, httpClient, apiKey, logger)
	if err != nil {
		logger.Error("Failed to fetch MSFT price", "error", err)
		return nil, err
	}

	// Master Logic: O(n log n) consensus computation
	goldConsensus := computeConsensus(goldPrices, "GOLD", config, logger)
	msftConsensus := computeConsensus(msftPrices, "MSFT", config, logger)

	crossAssetVariance := math.Abs(goldConsensus.Variance - msftConsensus.Variance)
	systemRiskScore := (goldConsensus.RiskScore + msftConsensus.RiskScore) / 2.0

	alert := "Normal"
	if crossAssetVariance > config.RiskParameters.HighVolatilityThreshold {
		alert = fmt.Sprintf("High Volatility: %.2f%%", crossAssetVariance)
		systemRiskScore += 2.0
		logger.Warn("⚠️  HIGH VOLATILITY DETECTED", "variance", crossAssetVariance)
	}

	logger.Info("========================================")
	logger.Info("✅ LIVE DATA SUCCESSFULLY RETRIEVED")
	logger.Info("========================================")
	logger.Info(fmt.Sprintf("GOLD: $%.2f | MSFT: $%.2f", goldConsensus.MedianPrice, msftConsensus.MedianPrice))
	logger.Info(fmt.Sprintf("Risk Score: %.1f/10.0", systemRiskScore))

	return &ExecutionResult{
		GoldPrice:          goldConsensus.MedianPrice,
		MsftPrice:          msftConsensus.MedianPrice,
		GoldVariance:       goldConsensus.Variance,
		MsftVariance:       msftConsensus.Variance,
		CrossAssetVariance: crossAssetVariance,
		VolatilityWarning:  alert,
		SystemRiskScore:    systemRiskScore,
		Message:            fmt.Sprintf("LIVE: GOLD $%.2f | MSFT $%.2f | Risk %.1f", goldConsensus.MedianPrice, msftConsensus.MedianPrice, systemRiskScore),
		Timestamp:          time.Now(),
		DataSource:         "Alpha Vantage Live API via CRE HTTP Capability",
	}, nil
}

// fetchLiveGoldPrice uses CRE HTTP capability (v1.8.7 pattern)
func fetchLiveGoldPrice(runtime cre.Runtime, httpClient *http.Client, apiKey string, logger *slog.Logger) ([]*PriceData, error) {
	logger.Info("📊 Fetching LIVE GOLD price via CRE HTTP Capability...")

	url := fmt.Sprintf("https://www.alphavantage.co/query?function=GLOBAL_QUOTE&symbol=GLD&apikey=%s", apiKey)

	// CRE HTTP Capability request
	req := &http.Request{
		Method: "GET",
		Url:    url, // Note: field is "Url" not "URL"
		Headers: map[string]string{
			"Accept": "application/json",
		},
	}

	// Execute request via CRE capability - SendRequest returns Promise[*Response]
	respPromise := httpClient.SendRequest(runtime.(cre.NodeRuntime), req)
	resp, err := respPromise.Await()
	if err != nil {
		logger.Error("HTTP capability request failed", "error", err)
		return nil, fmt.Errorf("failed to fetch GOLD price: %w", err)
	}

	// Parse Alpha Vantage response
	var avResp AlphaVantageResponse
	if err := json.Unmarshal(resp.Body, &avResp); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}

	if avResp.Note != "" {
		return nil, fmt.Errorf("API limit: %s", avResp.Note)
	}

	if avResp.GlobalQuote.Price == "" {
		return nil, fmt.Errorf("no price data received")
	}

	var price float64
	fmt.Sscanf(avResp.GlobalQuote.Price, "%f", &price)

	logger.Info("✅ GOLD Price Retrieved", "price", fmt.Sprintf("$%.2f", price), "symbol", "GLD")

	return []*PriceData{{
		Symbol:    "GOLD",
		Price:     price,
		Source:    "Alpha Vantage GLD",
		Timestamp: time.Now(),
	}}, nil
}

// fetchLiveMsftPrice uses CRE HTTP capability (v1.8.7 pattern)
func fetchLiveMsftPrice(runtime cre.Runtime, httpClient *http.Client, apiKey string, logger *slog.Logger) ([]*PriceData, error) {
	logger.Info("📊 Fetching LIVE MSFT price via CRE HTTP Capability...")

	url := fmt.Sprintf("https://www.alphavantage.co/query?function=GLOBAL_QUOTE&symbol=MSFT&apikey=%s", apiKey)

	// CRE HTTP Capability request
	req := &http.Request{
		Method: "GET",
		Url:    url, // Note: field is "Url" not "URL"
		Headers: map[string]string{
			"Accept": "application/json",
		},
	}

	// Execute request via CRE capability - SendRequest returns Promise[*Response]
	respPromise := httpClient.SendRequest(runtime.(cre.NodeRuntime), req)
	resp, err := respPromise.Await()
	if err != nil {
		logger.Error("HTTP capability request failed", "error", err)
		return nil, fmt.Errorf("failed to fetch MSFT price: %w", err)
	}

	// Parse Alpha Vantage response
	var avResp AlphaVantageResponse
	if err := json.Unmarshal(resp.Body, &avResp); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}

	if avResp.Note != "" {
		return nil, fmt.Errorf("API limit: %s", avResp.Note)
	}

	if avResp.GlobalQuote.Price == "" {
		return nil, fmt.Errorf("no price data received")
	}

	var price float64
	fmt.Sscanf(avResp.GlobalQuote.Price, "%f", &price)

	logger.Info("✅ MSFT Price Retrieved", "price", fmt.Sprintf("$%.2f", price), "symbol", "MSFT")

	return []*PriceData{{
		Symbol:    "MSFT",
		Price:     price,
		Source:    "Alpha Vantage MSFT",
		Timestamp: time.Now(),
	}}, nil
}

type ConsensusResult struct {
	MedianPrice float64
	Variance    float64
	RiskScore   float64
}

// computeConsensus implements O(n log n) QuickSort-based median aggregation
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
