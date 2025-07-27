// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";

/**
 * @title UTXOToken
 * @dev ERC20 token representing a Bitcoin UTXO
 * Each token is backed by a specific Bitcoin UTXO and can be burned to redeem the original BTC
 */
contract UTXOToken is ERC20, Ownable, ERC20Burnable {
    
    // Bitcoin UTXO information
    string public bitcoinTxId;
    uint32 public bitcoinVout;
    uint256 public bitcoinAmount; // in satoshis
    string public bitcoinAddress;
    
    // Status tracking
    bool public isRedeemed;
    address public registry;
    
    event UTXORedeemed(string bitcoinTxId, uint32 vout, address redeemer, string bitcoinDestination);
    
    modifier onlyRegistry() {
        require(msg.sender == registry, "Only registry can call this");
        _;
    }
    
    constructor(
        string memory _name,
        string memory _symbol,
        string memory _bitcoinTxId,
        uint32 _bitcoinVout,
        uint256 _bitcoinAmount,
        string memory _bitcoinAddress,
        address _initialOwner,
        address _registry
    ) ERC20(_name, _symbol) Ownable(_initialOwner) {
        bitcoinTxId = _bitcoinTxId;
        bitcoinVout = _bitcoinVout;
        bitcoinAmount = _bitcoinAmount;
        bitcoinAddress = _bitcoinAddress;
        registry = _registry;
        
        // Mint tokens equivalent to Bitcoin amount (in satoshis)
        // Using 18 decimals, so 1 satoshi = 1e10 wei tokens
        _mint(_initialOwner, _bitcoinAmount * 1e10);
    }
    
    /**
     * @dev Burns tokens and initiates Bitcoin redemption process
     * @param _bitcoinDestination Bitcoin address to receive the redeemed BTC
     */
    function redeemForBitcoin(string memory _bitcoinDestination) external {
        require(!isRedeemed, "UTXO already redeemed");
        require(balanceOf(msg.sender) == totalSupply(), "Must own all tokens to redeem");
        
        // Burn all tokens
        _burn(msg.sender, totalSupply());
        
        // Mark as redeemed
        isRedeemed = true;
        
        emit UTXORedeemed(bitcoinTxId, bitcoinVout, msg.sender, _bitcoinDestination);
    }
    
    /**
     * @dev Returns UTXO information
     */
    function getUTXOInfo() external view returns (
        string memory txId,
        uint32 vout,
        uint256 amount,
        string memory btcAddress,
        bool redeemed
    ) {
        return (bitcoinTxId, bitcoinVout, bitcoinAmount, bitcoinAddress, isRedeemed);
    }
}