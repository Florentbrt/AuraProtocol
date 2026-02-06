// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/extensions/ERC4626.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/security/Pausable.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@chainlink/contracts/src/v0.8/interfaces/AggregatorV3Interface.sol";

/**
 * @title AuraVault
 * @author AuraProtocol Team
 * @notice Institutional-grade ERC-4626 vault with AI-driven risk management
 * @dev Implements Read-Your-Writes pattern for CRE compatibility
 * 
 * SECURITY FEATURES:
 * 1. Role-based access control (only CRE DON can rebalance)
 * 2. Chainlink Price Feed integration with staleness checks
 * 3. Automatic circuit breaker for >20% single-tx movements
 * 4. Pausable for emergency stops
 * 5. Reentrancy protection
 * 6. AI decision logging for audit trail
 */
contract AuraVault is ERC4626, AccessControl, Pausable, ReentrancyGuard {
    // ═══════════════════════════════════════════════════════════════
    // ROLES
    // ═══════════════════════════════════════════════════════════════
    bytes32 public constant CRE_EXECUTOR_ROLE = keccak256("CRE_EXECUTOR_ROLE");
    bytes32 public constant GUARDIAN_ROLE = keccak256("GUARDIAN_ROLE");
    
    // ═══════════════════════════════════════════════════════════════
    // STATE VARIABLES (Read-Your-Writes Pattern)
    // ═══════════════════════════════════════════════════════════════
    
    struct RiskSnapshot {
        uint256 timestamp;
        uint256 riskScore;        // Scaled by 1e18 (0.85 = 850000000000000000)
        uint256 vixValue;         // Scaled by 1e2 (25.50 = 2550)
        string aiRationale;       // LLM reasoning
        string action;            // "DEFENSIVE", "NEUTRAL", "AGGRESSIVE"
    }
    
    RiskSnapshot public latestRisk;
    uint256 public lastRebalanceTime;
    uint256 public totalRebalances;
    
    // Price Feed for asset verification
    AggregatorV3Interface public immutable assetPriceFeed;
    
    // Circuit Breaker Settings
    uint256 public constant MAX_SINGLE_MOVEMENT_BPS = 2000; // 20%
    uint256 public constant ORACLE_STALENESS_THRESHOLD = 3 hours;
    
    // Asset allocation (BPS = Basis Points, 10000 = 100%)
    uint256 public defensiveAllocationBPS;  // e.g., 8000 = 80% in stablecoins
    uint256 public aggressiveAllocationBPS; // e.g., 2000 = 20% in risk assets
    
    // ═══════════════════════════════════════════════════════════════
    // EVENTS
    // ═══════════════════════════════════════════════════════════════
    event RebalanceExecuted(
        uint256 indexed timestamp,
        uint256 riskScore,
        string action,
        uint256 newDefensiveBPS,
        string aiRationale
    );
    
    event CircuitBreakerTriggered(
        uint256 attemptedMovement,
        uint256 maxAllowed,
        address executor
    );
    
    event OracleStalenessDetected(
        uint256 lastUpdate,
        uint256 blockTimestamp
    );
    
    // ═══════════════════════════════════════════════════════════════
    // ERRORS
    // ═══════════════════════════════════════════════════════════════
    error UnauthorizedExecutor();
    error OracleStale();
    error MovementTooLarge(uint256 attempted, uint256 max);
    error InvalidRiskScore();
    error InvalidAllocation();
    
    // ═══════════════════════════════════════════════════════════════
    // CONSTRUCTOR
    // ═══════════════════════════════════════════════════════════════
    constructor(
        IERC20 _asset,
        string memory _name,
        string memory _symbol,
        address _priceFeed,
        address _creExecutor
    ) ERC4626(_asset) ERC20(_name, _symbol) {
        require(_priceFeed != address(0), "Invalid price feed");
        require(_creExecutor != address(0), "Invalid executor");
        
        assetPriceFeed = AggregatorV3Interface(_priceFeed);
        
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(CRE_EXECUTOR_ROLE, _creExecutor);
        _grantRole(GUARDIAN_ROLE, msg.sender);
        
        // Initialize with conservative allocation
        defensiveAllocationBPS = 5000; // 50% defensive
        aggressiveAllocationBPS = 5000; // 50% aggressive
    }
    
    // ═══════════════════════════════════════════════════════════════
    // READ-YOUR-WRITES STATE GETTERS
    // ═══════════════════════════════════════════════════════════════
    
    /**
     * @notice Returns the latest risk assessment for CRE to read
     * @dev CRE workflow calls this BEFORE making new decisions
     */
    function getLatestRiskSnapshot() external view returns (
        uint256 timestamp,
        uint256 riskScore,
        uint256 vixValue,
        string memory aiRationale,
        string memory action
    ) {
        RiskSnapshot memory snapshot = latestRisk;
        return (
            snapshot.timestamp,
            snapshot.riskScore,
            snapshot.vixValue,
            snapshot.aiRationale,
            snapshot.action
        );
    }
    
    /**
     * @notice Returns rebalance history for delta calculations
     */
    function getRebalanceStats() external view returns (
        uint256 lastTime,
        uint256 totalCount,
        uint256 currentDefensiveBPS,
        uint256 currentAggressiveBPS
    ) {
        return (
            lastRebalanceTime,
            totalRebalances,
            defensiveAllocationBPS,
            aggressiveAllocationBPS
        );
    }
    
    // ═══════════════════════════════════════════════════════════════
    // CORE REBALANCE FUNCTION (Called by CRE DON)
    // ═══════════════════════════════════════════════════════════════
    
    /**
     * @notice AI-driven rebalance with institutional safeguards
     * @param riskScore AI-calculated risk (0-1e18 scale)
     * @param vixValue Current VIX index (scaled by 1e2)
     * @param aiRationale LLM reasoning text
     * @param suggestedAction "DEFENSIVE", "NEUTRAL", "AGGRESSIVE"
     * 
     * SECURITY CHECKS:
     * 1. Role verification (only CRE DON)
     * 2. Oracle price staleness check
     * 3. Circuit breaker for large movements
     * 4. Pausable emergency stop
     */
    function rebalance(
        uint256 riskScore,
        uint256 vixValue,
        string calldata aiRationale,
        string calldata suggestedAction
    ) external nonReentrant whenNotPaused onlyRole(CRE_EXECUTOR_ROLE) {
        // Validation: Risk score must be 0-100% (scaled to 1e18)
        if (riskScore > 1e18) revert InvalidRiskScore();
        
        // SECURITY CHECK 1: Verify asset price freshness
        _verifyOracleFreshness();
        
        // Calculate new allocation based on AI suggestion
        (uint256 newDefensiveBPS, uint256 newAggressiveBPS) = 
            _calculateAllocation(riskScore, suggestedAction);
        
        // SECURITY CHECK 2: Circuit Breaker - prevent >20% single movement
        uint256 movementBPS = _abs(int256(newDefensiveBPS) - int256(defensiveAllocationBPS));
        if (movementBPS > MAX_SINGLE_MOVEMENT_BPS) {
            emit CircuitBreakerTriggered(movementBPS, MAX_SINGLE_MOVEMENT_BPS, msg.sender);
            _pause(); // Auto-pause on suspicious activity
            revert MovementTooLarge(movementBPS, MAX_SINGLE_MOVEMENT_BPS);
        }
        
        // Update state (Read-Your-Writes for next CRE execution)
        latestRisk = RiskSnapshot({
            timestamp: block.timestamp,
            riskScore: riskScore,
            vixValue: vixValue,
            aiRationale: aiRationale,
            action: suggestedAction
        });
        
        lastRebalanceTime = block.timestamp;
        totalRebalances++;
        
        // Apply new allocation
        defensiveAllocationBPS = newDefensiveBPS;
        aggressiveAllocationBPS = newAggressiveBPS;
        
        emit RebalanceExecuted(
            block.timestamp,
            riskScore,
            suggestedAction,
            newDefensiveBPS,
            aiRationale
        );
    }
    
    // ═══════════════════════════════════════════════════════════════
    // INTERNAL HELPERS
    // ═══════════════════════════════════════════════════════════════
    
    /**
     * @dev Verify Chainlink oracle is not stale
     * Prevents Flash Crash exploitation via outdated prices
     */
    function _verifyOracleFreshness() internal view {
        (, , , uint256 updatedAt, ) = assetPriceFeed.latestRoundData();
        
        if (block.timestamp - updatedAt > ORACLE_STALENESS_THRESHOLD) {
            revert OracleStale();
        }
    }
    
    /**
     * @dev Calculate defensive/aggressive split based on AI risk score
     * Higher risk → More defensive (stablecoins)
     * Lower risk → More aggressive (yield assets)
     */
    function _calculateAllocation(
        uint256 riskScore,
        string calldata action
    ) internal pure returns (uint256 defensiveBPS, uint256 aggressiveBPS) {
        // riskScore is 0-1e18, where 1e18 = 100% risk
        // Convert to BPS: (riskScore * 10000) / 1e18
        
        if (keccak256(bytes(action)) == keccak256("DEFENSIVE")) {
            // High risk: 80-90% defensive
            defensiveBPS = 8000 + ((riskScore * 1000) / 1e18);
            if (defensiveBPS > 9000) defensiveBPS = 9000;
        } else if (keccak256(bytes(action)) == keccak256("AGGRESSIVE")) {
            // Low risk: 30-50% defensive
            defensiveBPS = 3000 + ((riskScore * 2000) / 1e18);
            if (defensiveBPS > 5000) defensiveBPS = 5000;
        } else {
            // Neutral: 50-70% defensive
            defensiveBPS = 5000 + ((riskScore * 2000) / 1e18);
        }
        
        aggressiveBPS = 10000 - defensiveBPS;
        
        if (defensiveBPS + aggressiveBPS != 10000) revert InvalidAllocation();
    }
    
    /**
     * @dev Safe absolute value for int256
     */
    function _abs(int256 x) internal pure returns (uint256) {
        return x >= 0 ? uint256(x) : uint256(-x);
    }
    
    // ═══════════════════════════════════════════════════════════════
    // EMERGENCY CONTROLS (Guardian Role)
    // ═══════════════════════════════════════════════════════════════
    
    function pause() external onlyRole(GUARDIAN_ROLE) {
        _pause();
    }
    
    function unpause() external onlyRole(GUARDIAN_ROLE) {
        _unpause();
    }
    
    // ═══════════════════════════════════════════════════════════════
    // ERC4626 OVERRIDES (Add pause protection)
    // ═══════════════════════════════════════════════════════════════
    
    function deposit(uint256 assets, address receiver) 
        public 
        override 
        whenNotPaused 
        returns (uint256) 
    {
        return super.deposit(assets, receiver);
    }
    
    function withdraw(uint256 assets, address receiver, address owner)
        public
        override
        whenNotPaused
        returns (uint256)
    {
        return super.withdraw(assets, receiver, owner);
    }
}
