// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title AuraShield
 * @author AuraProtocol Team
 * @notice Layer 4 Security Contract for AuraProtocol
 * @dev Receives risk scores from Aura AI and executes emergency halts
 */
contract AuraShield {
    // STATE VARIABLES
    bool public isProtocolActive = true;
    address public immutable auraOracle;
    uint256 public lastRiskScore;
    uint256 public lastUpdateTimestamp;

    // EVENTS
    event ProtocolShielded(uint256 riskScore, string reason, uint256 timestamp);
    event ProtocolResumed(address admin, uint256 timestamp);

    // ERRORS
    error Unauthorized();
    error ProtocolAlreadyShielded();

    constructor(address _oracle) {
        auraOracle = _oracle;
    }

    modifier onlyOracle() {
        if (msg.sender != auraOracle) revert Unauthorized();
        _;
    }

    /**
     * @notice Trigger emergency halt if risk is critical
     * @param riskScore Scaled risk score (e.g., 100 = 10.0)
     */
    function emergencyHalt(uint256 riskScore) external onlyOracle {
        if (!isProtocolActive) revert ProtocolAlreadyShielded();

        lastRiskScore = riskScore;
        lastUpdateTimestamp = block.timestamp;

        // Threshold logic: If Risk > 8.0 (80), halt protocol
        // In the simulation, we hit 10.0 (100)
        if (riskScore > 80) {
            isProtocolActive = false;
            emit ProtocolShielded(riskScore, "CRITICAL_RISK_THRESHOLD_EXCEEDED", block.timestamp);
        }
    }

    /**
     * @notice View function to check if asset deposits should be allowed
     */
    function canDeposit() external view returns (bool) {
        return isProtocolActive;
    }
}
