// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "./UTXOToken.sol";

/**
 * @title UTXORegistry
 * @dev Registry that tracks Bitcoin UTXOs and their corresponding ERC20 tokens
 */
contract UTXORegistry is Ownable, ReentrancyGuard {
    
    struct UTXORecord {
        string bitcoinTxId;
        uint32 bitcoinVout;
        uint256 bitcoinAmount;
        string bitcoinAddress;
        address tokenAddress;
        address tokenOwner;
        bool isActive;
        uint256 createdAt;
    }
    
    // Mapping from UTXO ID (txid:vout) to UTXO record
    mapping(bytes32 => UTXORecord) public utxos;
    
    // Mapping from token address to UTXO ID
    mapping(address => bytes32) public tokenToUtxo;
    
    // Array of all UTXO IDs for enumeration
    bytes32[] public utxoIds;
    
    // Authorized operators (backend services)
    mapping(address => bool) public operators;
    
    event UTXORegistered(
        bytes32 indexed utxoId,
        string bitcoinTxId,
        uint32 bitcoinVout,
        uint256 bitcoinAmount,
        address tokenAddress,
        address tokenOwner
    );
    
    event UTXORedeemed(
        bytes32 indexed utxoId,
        address redeemer,
        string bitcoinDestination
    );
    
    event OperatorAdded(address operator);
    event OperatorRemoved(address operator);
    
    modifier onlyOperator() {
        require(operators[msg.sender] || msg.sender == owner(), "Not authorized operator");
        _;
    }
    
    constructor() Ownable(msg.sender) {}
    
    /**
     * @dev Adds an authorized operator
     */
    function addOperator(address _operator) external onlyOwner {
        operators[_operator] = true;
        emit OperatorAdded(_operator);
    }
    
    /**
     * @dev Removes an authorized operator
     */
    function removeOperator(address _operator) external onlyOwner {
        operators[_operator] = false;
        emit OperatorRemoved(_operator);
    }
    
    /**
     * @dev Registers a new UTXO and creates corresponding ERC20 token
     */
    function registerUTXO(
        string memory _bitcoinTxId,
        uint32 _bitcoinVout,
        uint256 _bitcoinAmount,
        string memory _bitcoinAddress,
        address _tokenOwner
    ) external onlyOperator nonReentrant returns (address tokenAddress) {
        
        bytes32 utxoId = keccak256(abi.encodePacked(_bitcoinTxId, _bitcoinVout));
        require(!utxos[utxoId].isActive, "UTXO already registered");
        
        // Create token name and symbol
        string memory tokenName = string(abi.encodePacked("UTXO_", _bitcoinTxId));
        string memory tokenSymbol = string(abi.encodePacked("UTXO", uint2str(_bitcoinVout)));
        
        // Deploy new UTXO token
        UTXOToken token = new UTXOToken(
            tokenName,
            tokenSymbol,
            _bitcoinTxId,
            _bitcoinVout,
            _bitcoinAmount,
            _bitcoinAddress,
            _tokenOwner,
            address(this)
        );
        
        tokenAddress = address(token);
        
        // Store UTXO record
        utxos[utxoId] = UTXORecord({
            bitcoinTxId: _bitcoinTxId,
            bitcoinVout: _bitcoinVout,
            bitcoinAmount: _bitcoinAmount,
            bitcoinAddress: _bitcoinAddress,
            tokenAddress: tokenAddress,
            tokenOwner: _tokenOwner,
            isActive: true,
            createdAt: block.timestamp
        });
        
        // Update mappings
        tokenToUtxo[tokenAddress] = utxoId;
        utxoIds.push(utxoId);
        
        emit UTXORegistered(
            utxoId,
            _bitcoinTxId,
            _bitcoinVout,
            _bitcoinAmount,
            tokenAddress,
            _tokenOwner
        );
        
        return tokenAddress;
    }
    
    /**
     * @dev Marks a UTXO as redeemed (called when tokens are burned)
     */
    function markUTXORedeemed(
        bytes32 _utxoId,
        address _redeemer,
        string memory _bitcoinDestination
    ) external onlyOperator {
        require(utxos[_utxoId].isActive, "UTXO not active");
        
        utxos[_utxoId].isActive = false;
        
        emit UTXORedeemed(_utxoId, _redeemer, _bitcoinDestination);
    }
    
    /**
     * @dev Returns UTXO information by ID
     */
    function getUTXO(bytes32 _utxoId) external view returns (UTXORecord memory) {
        return utxos[_utxoId];
    }
    
    /**
     * @dev Returns UTXO ID from bitcoin transaction details
     */
    function getUTXOId(string memory _bitcoinTxId, uint32 _bitcoinVout) external pure returns (bytes32) {
        return keccak256(abi.encodePacked(_bitcoinTxId, _bitcoinVout));
    }
    
    /**
     * @dev Returns total number of registered UTXOs
     */
    function getUTXOCount() external view returns (uint256) {
        return utxoIds.length;
    }
    
    /**
     * @dev Helper function to convert uint to string
     */
    function uint2str(uint256 _i) internal pure returns (string memory) {
        if (_i == 0) {
            return "0";
        }
        uint256 j = _i;
        uint256 len;
        while (j != 0) {
            len++;
            j /= 10;
        }
        bytes memory bstr = new bytes(len);
        uint256 k = len;
        while (_i != 0) {
            k = k - 1;
            uint8 temp = (48 + uint8(_i - _i / 10 * 10));
            bytes1 b1 = bytes1(temp);
            bstr[k] = b1;
            _i /= 10;
        }
        return string(bstr);
    }
}