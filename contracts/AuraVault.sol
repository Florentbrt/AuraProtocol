// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/extensions/ERC4626.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/utils/Pausable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@chainlink/contracts/src/v0.8/shared/interfaces/AggregatorV3Interface.sol";

/**
 * @title AuraVault V2
 * @author AuraProtocol Team
 * @notice Institutional-Grade ERC-4626 Vault for Chainlink Convergence 2026
 * @dev Implements "Read-Your-Writes" support via internal RiskSnapshot storage.
 */
contract AuraVault is ERC4626, AccessControl, Pausable, ReentrancyGuard {
    // ═══════════════════════════════════════════════════════════════
    // ROLES
    // ═══════════════════════════════════════════════════════════════
    bytes32 public constant CRE_ROLE = keccak256("CRE_ROLE");           // Role for the Automated Workflow
    bytes32 public constant GUARDIAN_ROLE = keccak256("GUARDIAN_ROLE"); // Role for Emergency Pause

    // ═══════════════════════════════════════════════════════════════
    // STATE: "Read-Your-Writes" Support
    // ═══════════════════════════════════════════════════════════════
    struct RiskSnapshot {
        uint256 lastUpdate;
        uint256 riskScore;    // 0-100
        string aiRationale;
    }

    RiskSnapshot public latestSnapshot;

    // ═══════════════════════════════════════════════════════════════
    // CONFIGURATION & IMMUTABLES
    // ═══════════════════════════════════════════════════════════════
    AggregatorV3Interface public immutable assetPriceFeed;
    uint256 public constant ORACLE_HEARTBEAT = 3600; // 1 hour staleness check
    uint256 public constant MAX_DEVIATION_BPS = 2000; // 20% circuit breaker

    // Allocation in Basis Points (0-10000)
    uint256 public currentRiskAllocation; // 0 means 100% defensive, 10000 means 100% risk

    // ═══════════════════════════════════════════════════════════════
    // EVENTS
    // ═══════════════════════════════════════════════════════════════
    event RebalanceExecuted(uint256 indexed timestamp, uint256 oldRisk, uint256 newRisk, string rationale);
    event CircuitBreakerTriggered(uint256 tried, uint256 current, string msg);

    // ═══════════════════════════════════════════════════════════════
    // ERRORS
    // ═══════════════════════════════════════════════════════════════
    error OracleStale();
    error DeviationTooHigh(uint256 deviation);

    constructor(
        IERC20 _asset,
        string memory _name,
        string memory _symbol,
        address _priceFeed,
        address _creExecutor
    ) ERC4626(_asset) ERC20(_name, _symbol) {
        assetPriceFeed = AggregatorV3Interface(_priceFeed);
        
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(CRE_ROLE, _creExecutor);
        _grantRole(GUARDIAN_ROLE, msg.sender);

        // Initial State
        latestSnapshot = RiskSnapshot(block.timestamp, 50, "Initial deployment");
        currentRiskAllocation = 5000; // 50%
    }

    // ═══════════════════════════════════════════════════════════════
    // CORE LOGIC
    // ═══════════════════════════════════════════════════════════════

    /**
     * @notice Rebalances the vault based on AI risk assessment
     * @dev Only callable by CRE_ROLE.
     */
    function rebalance(uint256 newRiskScore, string calldata rationale) 
        external 
        onlyRole(CRE_ROLE) 
        whenNotPaused 
        nonReentrant 
    {
        // 1. SECURITY: Staleness Check
        _checkOracleFreshness();

        // Target allocation based on risk score (0-100 map to 0-10000 BPS)
        // If Risk is 100 (Safe), Allocation is 0 (Risk Assets).
        // If Risk is 0 (Safe), Allocation is 10000 (Risk Assets)? 
        // Let's assume RiskScore 100 = High Risk Market = Go Defensive.
        // RiskScore 0 = Low Risk Market = Go Aggressive.
        
        // Mapping: RiskScore (0-100) -> RiskExposure (10000 - 0)
        // High Risk Score (100) -> 0% Risk Exposure (100% Stable)
        uint256 targetAllocation = (100 - newRiskScore) * 100;

        // 2. SECURITY: Circuit Breaker
        _enforceCircuitBreaker(targetAllocation);

        // 3. UPDATE STATE
        uint256 oldRisk = latestSnapshot.riskScore;
        latestSnapshot = RiskSnapshot({
            lastUpdate: block.timestamp,
            riskScore: newRiskScore,
            aiRationale: rationale
        });

        currentRiskAllocation = targetAllocation;

        emit RebalanceExecuted(block.timestamp, oldRisk, newRiskScore, rationale);
    }

    function _checkOracleFreshness() internal view {
        (, , , uint256 updatedAt, ) = assetPriceFeed.latestRoundData();
        if (block.timestamp - updatedAt > ORACLE_HEARTBEAT) {
            revert OracleStale();
        }
    }

    function _enforceCircuitBreaker(uint256 targetAlloc) internal {
        // Calculate deviations in Basis Points relative to TOTAL capacity (10000)
        // Simple logic: Is |target - current| > 2000?
        uint256 diff;
        if (targetAlloc > currentRiskAllocation) {
            diff = targetAlloc - currentRiskAllocation;
        } else {
            diff = currentRiskAllocation - targetAlloc;
        }

        if (diff > MAX_DEVIATION_BPS) {
            // Check if Guardian mode? Or just revert?
            // "Add a modifier that reverts"
            revert DeviationTooHigh(diff);
        }
    }

    // ═══════════════════════════════════════════════════════════════
    // READ-YOUR-WRITES HELPERS
    // ═══════════════════════════════════════════════════════════════
    function getLatestSnapshot() external view returns (RiskSnapshot memory) {
        return latestSnapshot;
    }

    // ═══════════════════════════════════════════════════════════════
    // GUARDIAN CONTROLS
    // ═══════════════════════════════════════════════════════════════
    function pause() external onlyRole(GUARDIAN_ROLE) {
        _pause();
    }

    function unpause() external onlyRole(GUARDIAN_ROLE) {
        _unpause();
    }
}
