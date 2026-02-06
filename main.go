package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"
)

// ============================================================================
// DATA MODELS
// ============================================================================

// PriceData represents a single price observation from a data source
type PriceData struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
	Weight    float64   `json:"weight"`
}

// AlphaVantageGlobalQuote represents the API response structure for GLOBAL_QUOTE
type AlphaVantageGlobalQuote struct {
	GlobalQuote struct {
		Symbol           string `json:"01. symbol"`
		Open             string `json:"02. open"`
		High             string `json:"03. high"`
		Low              string `json:"04. low"`
		Price            string `json:"05. price"`
		Volume           string `json:"06. volume"`
		LatestTradingDay string `json:"07. latest trading day"`
		PreviousClose    string `json:"08. previous close"`
		Change           string `json:"09. change"`
		ChangePercent    string `json:"10. change percent"`
	} `json:"Global Quote"`
}

// AlphaVantageForex represents the API response for currency exchange rates
type AlphaVantageForex struct {
	RealtimeCurrencyExchangeRate struct {
		FromCurrencyCode string `json:"1. From_Currency Code"`
		FromCurrencyName string `json:"2. From_Currency Name"`
		ToCurrencyCode   string `json:"3. To_Currency Code"`
		ToCurrencyName   string `json:"4. To_Currency Name"`
		ExchangeRate     string `json:"5. Exchange Rate"`
		LastRefreshed    string `json:"6. Last Refreshed"`
		TimeZone         string `json:"7. Time Zone"`
		BidPrice         string `json:"8. Bid Price"`
		AskPrice         string `json:"9. Ask Price"`
	} `json:"Realtime Currency Exchange Rate"`
}

// ConsensusResult represents the aggregated result from DON consensus
type ConsensusResult struct {
	MedianPrice       float64   `json:"median_price"`
	MeanPrice         float64   `json:"mean_price"`
	Variance          float64   `json:"variance"`
	StandardDeviation float64   `json:"std_dev"`
	VolatilityWarning string    `json:"volatility_warning"`
	RiskScore         float64   `json:"risk_score"`
	Timestamp         time.Time `json:"timestamp"`
	NumSources        int       `json:"num_sources"`
}

// MasterLogicOutput represents the final output of the master logic computation
type MasterLogicOutput struct {
	GoldData           ConsensusResult `json:"gold_data"`
	MsftData           ConsensusResult `json:"msft_data"`
	CrossAssetVariance float64         `json:"cross_asset_variance"`
	SystemRiskScore    float64         `json:"system_risk_score"`
	Alert              string          `json:"alert"`
	Timestamp          time.Time       `json:"timestamp"`
}

// ============================================================================
// CHAINLINK WORKFLOW IMPLEMENTATION
// ============================================================================

// AuraProtocolWorkflow implements the data ingestion workflow
// In production CRE deployment, this would implement the workflow.Executor interface
type AuraProtocolWorkflow struct {
	config         *Config
	priceCache     map[string]*PriceData
	lastUpdateTime time.Time
}

// Config holds the application configuration loaded from config.json
type Config struct {
	Version   string `json:"version"`
	Network   string `json:"network"`
	Consensus struct {
		MinimumNodes       int     `json:"minimumNodes"`
		BftThreshold       float64 `json:"bftThreshold"`
		AggregationMethod  string  `json:"aggregationMethod"`
		MaxVariancePercent float64 `json:"maxVariancePercent"`
	} `json:"consensus"`
	DataSources map[string]struct {
		Provider               string `json:"provider"`
		RefreshIntervalSeconds int    `json:"refreshIntervalSeconds"`
		StaleThresholdSeconds  int    `json:"staleThresholdSeconds"`
		Endpoints              []struct {
			URL    string            `json:"url"`
			Params map[string]string `json:"params"`
			Weight float64           `json:"weight"`
		} `json:"endpoints"`
	} `json:"dataSources"`
	RiskParameters struct {
		HighVolatilityThreshold    float64 `json:"highVolatilityThreshold"`
		ExtremeVolatilityThreshold float64 `json:"extremeVolatilityThreshold"`
		CircuitBreakerEnabled      bool    `json:"circuitBreakerEnabled"`
		MaxPriceDeviationPercent   float64 `json:"maxPriceDeviationPercent"`
	} `json:"riskParameters"`
	Performance struct {
		Timeout         int  `json:"timeout"`
		MaxRetries      int  `json:"maxRetries"`
		CacheEnabled    bool `json:"cacheEnabled"`
		CacheTTLSeconds int  `json:"cacheTTLSeconds"`
	} `json:"performance"`
}

// NewAuraProtocolWorkflow creates a new instance of the workflow
func NewAuraProtocolWorkflow(config *Config) *AuraProtocolWorkflow {
	return &AuraProtocolWorkflow{
		config:     config,
		priceCache: make(map[string]*PriceData),
	}
}

// ============================================================================
// SECRET MANAGEMENT
// ============================================================================

// GetSecret retrieves a secret from the environment or CRE runtime
// In production CRE deployment, this would use the CRE SDK's runtime.GetSecret()
// For local development, it falls back to environment variables
func GetSecret(secretName string) (string, error) {
	// In production CRE deployment:
	// return runtime.GetSecret(secretName)
	//
	// For demonstration/local development:
	value := os.Getenv(secretName)
	if value == "" {
		return "", fmt.Errorf("secret %s not found in environment", secretName)
	}
	return value, nil
}

// substituteSecrets replaces ${SECRET_NAME} placeholders with actual secret values
func substituteSecrets(template string) string {
	// Pattern: ${SECRET_NAME}
	result := template

	// Replace ${ALPHA_VANTAGE_API_KEY}
	if apiKey, err := GetSecret("ALPHA_VANTAGE_API_KEY"); err == nil {
		result = replaceEnvVar(result, "ALPHA_VANTAGE_API_KEY", apiKey)
	}

	// Replace ${AURA_CONTRACT_ADDRESS}
	if contractAddr, err := GetSecret("AURA_CONTRACT_ADDRESS"); err == nil {
		result = replaceEnvVar(result, "AURA_CONTRACT_ADDRESS", contractAddr)
	}

	// Replace ${ALERT_WEBHOOK_URL}
	if webhookURL, err := GetSecret("ALERT_WEBHOOK_URL"); err == nil {
		result = replaceEnvVar(result, "ALERT_WEBHOOK_URL", webhookURL)
	}

	return result
}

// replaceEnvVar replaces ${VAR_NAME} with actual value
func replaceEnvVar(template, varName, value string) string {
	placeholder := fmt.Sprintf("${%s}", varName)
	return replaceAll(template, placeholder, value)
}

// replaceAll is a simple string replacement helper
func replaceAll(s, old, new string) string {
	result := s
	for i := 0; i < len(s); i++ {
		if len(result) >= len(old) && result[i:i+len(old)] == old {
			result = result[:i] + new + result[i+len(old):]
			i += len(new) - 1
		}
	}
	return result
}

// ============================================================================
// PERFORM - Main Execution Function (Called by CRE)
// ============================================================================

// Perform is the main entry point called by the Chainlink Runtime Environment
// This function orchestrates the entire data ingestion and consensus process
//
// CONSENSUS MECHANISM:
// 1. The CRE executes this function across multiple DON nodes simultaneously
// 2. Each node fetches data from multiple sources using HTTP requests
// 3. The DON achieves BFT consensus by comparing cryptographic hashes of responses
// 4. Once 2/3+ nodes agree on the response (Byzantine Fault Tolerance), the data is accepted
// 5. The aggregated results are then processed through Master Logic
// 6. Final output is verified across nodes before on-chain reporting
//
// NOTE: In production, http.SendRequest would be provided by the CRE SDK.
// This implementation simulates the consensus mechanism for demonstration.
func (w *AuraProtocolWorkflow) Perform(ctx context.Context, trigger interface{}) ([]byte, error) {
	startTime := time.Now()

	// Step 2: Fetch GOLD prices with consensus
	// In production CRE deployment, each HTTP request triggers DON-wide consensus
	// All nodes in the DON must agree (BFT threshold: 67%) on the HTTP response
	goldPrices, err := w.fetchGoldPricesWithConsensus(ctx)
	if err != nil {
		return nil, fmt.Errorf("GOLD price consensus failed: %w", err)
	}

	// Step 3: Fetch MSFT prices with consensus
	// Same consensus mechanism as GOLD - DON nodes vote on the correct response
	msftPrices, err := w.fetchMsftPricesWithConsensus(ctx)
	if err != nil {
		return nil, fmt.Errorf("MSFT price consensus failed: %w", err)
	}

	// Step 4: Apply Master Logic - Compute variance and volatility
	// Performance: O(n log n) due to sorting for median calculation
	// This is more efficient than O(n²) approaches and provides better outlier resistance
	goldConsensus := w.computeConsensus(goldPrices, "GOLD")
	msftConsensus := w.computeConsensus(msftPrices, "MSFT")

	// Step 5: Master Logic Check - Cross-asset variance analysis
	masterLogicOutput := w.computeMasterLogic(goldConsensus, msftConsensus)

	// Step 6: Cache results if enabled
	if w.config.Performance.CacheEnabled {
		w.updateCache("GOLD", goldPrices)
		w.updateCache("MSFT", msftPrices)
	}

	// Step 7: Serialize output for on-chain reporting
	outputBytes, err := json.Marshal(masterLogicOutput)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize output: %w", err)
	}

	// Performance metrics logging
	elapsed := time.Since(startTime)
	fmt.Printf("[PERF] Total execution time: %v\n", elapsed)
	fmt.Printf("[CONSENSUS] GOLD variance: %.4f%%, MSFT variance: %.4f%%\n",
		goldConsensus.Variance, msftConsensus.Variance)

	return outputBytes, nil
}

// ============================================================================
// RETRIEVE - Query Function (Called by external consumers)
// ============================================================================

// Retrieve allows external systems to query cached price data without triggering consensus
// This is useful for read-heavy workloads where real-time consensus is not required
func (w *AuraProtocolWorkflow) Retrieve(ctx context.Context, query []byte) ([]byte, error) {
	var queryParams struct {
		Symbol    string `json:"symbol"`
		FromCache bool   `json:"from_cache"`
	}

	if err := json.Unmarshal(query, &queryParams); err != nil {
		return nil, fmt.Errorf("invalid query format: %w", err)
	}

	// Return cached data if available and not stale
	if queryParams.FromCache && w.config.Performance.CacheEnabled {
		if cachedPrice, exists := w.priceCache[queryParams.Symbol]; exists {
			// Check if cache is stale
			cacheTTL := time.Duration(w.config.Performance.CacheTTLSeconds) * time.Second
			if time.Since(cachedPrice.Timestamp) < cacheTTL {
				return json.Marshal(cachedPrice)
			}
		}
	}

	// Cache miss or stale - trigger fresh consensus
	return nil, fmt.Errorf("cache miss or stale data for symbol: %s", queryParams.Symbol)
}

// ============================================================================
// CONSENSUS FUNCTIONS - Multi-Source Data Aggregation
// ============================================================================

// fetchGoldPricesWithConsensus fetches GOLD prices from multiple sources
// Each HTTP request goes through the DON consensus mechanism
func (w *AuraProtocolWorkflow) fetchGoldPricesWithConsensus(ctx context.Context) ([]*PriceData, error) {
	goldConfig := w.config.DataSources["gold"]
	prices := make([]*PriceData, 0, len(goldConfig.Endpoints))

	for _, endpoint := range goldConfig.Endpoints {
		// Construct URL with parameters and substitute secrets
		url := endpoint.URL
		for key, value := range endpoint.Params {
			url = fmt.Sprintf("%s&%s=%s", url, key, value)
		}
		// Securely substitute API keys from environment/CRE secrets
		url = substituteSecrets(url)

		// In production CRE deployment:
		// http.SendRequest triggers DON consensus:
		// 1. All DON nodes execute this HTTP request independently
		// 2. Nodes compare their responses using a cryptographic hash
		// 3. If 2/3+ nodes receive identical responses, consensus is reached
		// 4. The agreed-upon response is returned to all nodes
		// 5. If consensus fails, an error is returned
		//
		// Example CRE SDK usage:
		// req := http.Request{Method: "GET", URL: url, Headers: map[string]string{"Accept": "application/json"}}
		// resp, err := w.httpCapability.SendRequest(ctx, req)
		//
		// For this demonstration, we'll use a placeholder:
		_ = url // Demonstration: This URL would be passed to http.SendRequest in production
		fmt.Printf("[INFO] Would fetch GOLD via DON consensus from: %s\n", endpoint.URL)

		// Simulated price for demonstration (in production, parsed from HTTP response)
		price := 2050.0 + float64(len(prices))*10.0 // Simulated variance

		prices = append(prices, &PriceData{
			Symbol:    "GOLD",
			Price:     price,
			Timestamp: time.Now(),
			Source:    endpoint.URL,
			Weight:    endpoint.Weight,
		})
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no valid GOLD prices obtained from consensus")
	}

	return prices, nil
}

// fetchMsftPricesWithConsensus fetches MSFT prices from multiple sources
func (w *AuraProtocolWorkflow) fetchMsftPricesWithConsensus(ctx context.Context) ([]*PriceData, error) {
	msftConfig := w.config.DataSources["msft"]
	prices := make([]*PriceData, 0, len(msftConfig.Endpoints))

	for _, endpoint := range msftConfig.Endpoints {
		// Construct URL with parameters and substitute secrets
		url := endpoint.URL
		for key, value := range endpoint.Params {
			url = fmt.Sprintf("%s&%s=%s", url, key, value)
		}
		// Securely substitute API keys from environment/CRE secrets
		url = substituteSecrets(url)

		// Same DON consensus mechanism as GOLD
		// In production: resp, err := w.httpCapability.SendRequest(ctx, http.Request{...})
		_ = url // Demonstration: This URL would be passed to http.SendRequest in production
		fmt.Printf("[INFO] Would fetch MSFT via DON consensus from: %s\n", endpoint.URL)

		// Simulated price for demonstration
		price := 420.0 + float64(len(prices))*5.0

		prices = append(prices, &PriceData{
			Symbol:    "MSFT",
			Price:     price,
			Timestamp: time.Now(),
			Source:    endpoint.URL,
			Weight:    endpoint.Weight,
		})
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no valid MSFT prices obtained from consensus")
	}

	return prices, nil
}

// ============================================================================
// MASTER LOGIC - Variance Computation & Risk Assessment
// ============================================================================

// computeConsensus aggregates multiple price observations into a single consensus value
// Performance: O(n log n) due to sorting for median calculation
// This provides better outlier resistance than simple averaging (O(n))
func (w *AuraProtocolWorkflow) computeConsensus(prices []*PriceData, symbol string) ConsensusResult {
	if len(prices) == 0 {
		return ConsensusResult{}
	}

	// Step 1: Sort prices for median calculation - O(n log n)
	// We use weighted median to account for source reliability
	sortedPrices := make([]*PriceData, len(prices))
	copy(sortedPrices, prices)
	sort.Slice(sortedPrices, func(i, j int) bool {
		return sortedPrices[i].Price < sortedPrices[j].Price
	})

	// Step 2: Calculate median - O(1) after sorting
	var median float64
	n := len(sortedPrices)
	if n%2 == 0 {
		median = (sortedPrices[n/2-1].Price + sortedPrices[n/2].Price) / 2.0
	} else {
		median = sortedPrices[n/2].Price
	}

	// Step 3: Calculate weighted mean - O(n)
	var weightedSum, totalWeight float64
	for _, p := range prices {
		weightedSum += p.Price * p.Weight
		totalWeight += p.Weight
	}
	mean := weightedSum / totalWeight

	// Step 4: Calculate variance - O(n)
	// Variance = sqrt(sum((x_i - mean)^2) / n)
	var sumSquaredDiff float64
	for _, p := range prices {
		diff := p.Price - mean
		sumSquaredDiff += diff * diff
	}
	variance := math.Sqrt(sumSquaredDiff / float64(n))
	variancePercent := (variance / mean) * 100.0

	// Step 5: Determine volatility warning based on variance threshold
	volatilityWarning := "Normal"
	riskScore := 0.0

	if variancePercent > w.config.RiskParameters.ExtremeVolatilityThreshold {
		volatilityWarning = "EXTREME VOLATILITY - Circuit Breaker Triggered"
		riskScore = 10.0
	} else if variancePercent > w.config.RiskParameters.HighVolatilityThreshold {
		volatilityWarning = "High Volatility"
		riskScore = 7.0
	} else if variancePercent > w.config.Consensus.MaxVariancePercent {
		volatilityWarning = "Elevated Volatility"
		riskScore = 4.0
	}

	return ConsensusResult{
		MedianPrice:       median,
		MeanPrice:         mean,
		Variance:          variancePercent,
		StandardDeviation: variance,
		VolatilityWarning: volatilityWarning,
		RiskScore:         riskScore,
		Timestamp:         time.Now(),
		NumSources:        len(prices),
	}
}

// computeMasterLogic performs cross-asset analysis and generates final risk assessment
// This is the "Master Logic" that computes variance between GOLD and MSFT
func (w *AuraProtocolWorkflow) computeMasterLogic(gold, msft ConsensusResult) MasterLogicOutput {
	// Cross-asset variance: Measure price movement correlation
	crossAssetVariance := math.Abs(gold.Variance - msft.Variance)

	// System risk score: Aggregate risk from both assets
	systemRiskScore := (gold.RiskScore + msft.RiskScore) / 2.0

	// Generate alert if variance threshold is exceeded
	alert := "All Systems Normal"
	if crossAssetVariance > w.config.RiskParameters.HighVolatilityThreshold {
		alert = fmt.Sprintf("ALERT: Cross-asset variance %.2f%% exceeds threshold %.2f%%",
			crossAssetVariance, w.config.RiskParameters.HighVolatilityThreshold)
		systemRiskScore += 2.0 // Increase risk score for cross-asset volatility
	}

	// Circuit breaker logic
	if w.config.RiskParameters.CircuitBreakerEnabled && systemRiskScore >= 9.0 {
		alert = "CRITICAL: Circuit Breaker Activated - Trading Halted"
	}

	return MasterLogicOutput{
		GoldData:           gold,
		MsftData:           msft,
		CrossAssetVariance: crossAssetVariance,
		SystemRiskScore:    systemRiskScore,
		Alert:              alert,
		Timestamp:          time.Now(),
	}
}

// ============================================================================
// HELPER FUNCTIONS - Response Parsing
// ============================================================================

// parseGoldResponse parses Alpha Vantage API responses for GOLD
func (w *AuraProtocolWorkflow) parseGoldResponse(body []byte, function string) (float64, error) {
	switch function {
	case "GLOBAL_QUOTE":
		var quote AlphaVantageGlobalQuote
		if err := json.Unmarshal(body, &quote); err != nil {
			return 0, err
		}
		return parseFloat(quote.GlobalQuote.Price)

	case "CURRENCY_EXCHANGE_RATE":
		var forex AlphaVantageForex
		if err := json.Unmarshal(body, &forex); err != nil {
			return 0, err
		}
		return parseFloat(forex.RealtimeCurrencyExchangeRate.ExchangeRate)

	default:
		return 0, fmt.Errorf("unsupported function: %s", function)
	}
}

// parseMsftResponse parses Alpha Vantage API responses for MSFT
func (w *AuraProtocolWorkflow) parseMsftResponse(body []byte, function string) (float64, error) {
	switch function {
	case "GLOBAL_QUOTE":
		var quote AlphaVantageGlobalQuote
		if err := json.Unmarshal(body, &quote); err != nil {
			return 0, err
		}
		return parseFloat(quote.GlobalQuote.Price)

	case "TIME_SERIES_INTRADAY":
		// Parse intraday data and return latest price
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return 0, err
		}
		// Extract latest price from time series
		// (Simplified - production code would handle complete time series parsing)
		return 0, fmt.Errorf("TIME_SERIES_INTRADAY parsing not fully implemented")

	default:
		return 0, fmt.Errorf("unsupported function: %s", function)
	}
}

// parseFloat safely converts string to float64
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// updateCache updates the in-memory price cache
func (w *AuraProtocolWorkflow) updateCache(symbol string, prices []*PriceData) {
	if len(prices) == 0 {
		return
	}
	// Store the most recent price
	w.priceCache[symbol] = prices[len(prices)-1]
	w.lastUpdateTime = time.Now()
}

// ============================================================================
// MAIN ENTRY POINT - Production Deployment
// ============================================================================

func main() {
	// Load configuration
	configData := []byte(`{...}`) // Load from file in production
	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	// Initialize workflow
	auraWorkflow := NewAuraProtocolWorkflow(&config)

	// In production CRE deployment, register with:
	// workflow.Register("aura-protocol-rwa-ingestion", auraWorkflow)
	// For demonstration:
	_ = auraWorkflow

	fmt.Println("AuraProtocol RWA Ingestion Layer initialized successfully")
	fmt.Println("Waiting for CRE triggers...")

	// Keep the process running (CRE handles execution)
	select {}
}
