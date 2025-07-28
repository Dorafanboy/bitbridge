// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title SPVVerifier
 * @dev Smart contract for verifying Bitcoin SPV (Simplified Payment Verification) proofs
 * Allows verification of Bitcoin transactions on Ethereum using block headers and Merkle proofs
 */
contract SPVVerifier {
    
    // Events
    event ProofVerified(bytes32 indexed txHash, bytes32 indexed blockHash, uint256 blockHeight);
    event BlockHeaderStored(bytes32 indexed blockHash, uint256 blockHeight);
    
    // Storage
    mapping(bytes32 => bool) public verifiedTransactions;
    mapping(bytes32 => BlockHeader) public blockHeaders;
    mapping(uint256 => bytes32) public blocksByHeight;
    
    struct BlockHeader {
        uint32 version;
        bytes32 prevBlock;
        bytes32 merkleRoot;
        uint32 timestamp;
        uint32 bits;
        uint32 nonce;
        uint256 height;
        bool exists;
    }
    
    struct MerkleProof {
        bytes32[] proof;
        uint256 index;
        bytes32 txHash;
        bytes32 merkleRoot;
    }
    
    /**
     * @dev Parse Bitcoin block header from raw bytes
     * @param headerBytes Raw block header bytes (80 bytes)
     * @return Parsed BlockHeader struct
     */
    function parseBlockHeader(bytes memory headerBytes) public pure returns (BlockHeader memory) {
        require(headerBytes.length == 80, "Invalid header length");
        
        BlockHeader memory header;
        
        // Parse version (bytes 0-3, little endian)
        header.version = uint32(bytes4(reverseBytes4(bytes4(slice(headerBytes, 0, 4)))));
        
        // Parse previous block hash (bytes 4-35, reverse byte order)
        header.prevBlock = bytes32(reverseBytes32(bytes32(slice(headerBytes, 4, 32))));
        
        // Parse merkle root (bytes 36-67, reverse byte order)
        header.merkleRoot = bytes32(reverseBytes32(bytes32(slice(headerBytes, 36, 32))));
        
        // Parse timestamp (bytes 68-71, little endian)
        header.timestamp = uint32(bytes4(reverseBytes4(bytes4(slice(headerBytes, 68, 4)))));
        
        // Parse bits (bytes 72-75, little endian)
        header.bits = uint32(bytes4(reverseBytes4(bytes4(slice(headerBytes, 72, 4)))));
        
        // Parse nonce (bytes 76-79, little endian)
        header.nonce = uint32(bytes4(reverseBytes4(bytes4(slice(headerBytes, 76, 4)))));
        
        header.exists = true;
        
        return header;
    }
    
    /**
     * @dev Verify Bitcoin SPV proof
     * @param headerBytes Raw Bitcoin block header (80 bytes)
     * @param merkleProof Merkle proof data
     * @param blockHeight Block height for this header
     * @return True if proof is valid
     */
    function verifyProof(
        bytes memory headerBytes,
        MerkleProof memory merkleProof,
        uint256 blockHeight
    ) public returns (bool) {
        
        // Parse block header
        BlockHeader memory header = parseBlockHeader(headerBytes);
        header.height = blockHeight;
        
        // Calculate block hash (double SHA256 of header)
        bytes32 blockHash = doubleSha256(headerBytes);
        
        // Verify merkle root matches header
        require(header.merkleRoot == merkleProof.merkleRoot, "Merkle root mismatch");
        
        // Verify merkle proof
        require(verifyMerkleProof(merkleProof), "Invalid merkle proof");
        
        // Store block header and mark transaction as verified
        blockHeaders[blockHash] = header;
        blocksByHeight[blockHeight] = blockHash;
        verifiedTransactions[merkleProof.txHash] = true;
        
        emit BlockHeaderStored(blockHash, blockHeight);
        emit ProofVerified(merkleProof.txHash, blockHash, blockHeight);
        
        return true;
    }
    
    /**
     * @dev Verify a Merkle proof
     * @param merkleProof The merkle proof to verify
     * @return True if the proof is valid
     */
    function verifyMerkleProof(MerkleProof memory merkleProof) public pure returns (bool) {
        bytes32 computedHash = merkleProof.txHash;
        uint256 index = merkleProof.index;
        
        for (uint256 i = 0; i < merkleProof.proof.length; i++) {
            bytes32 proofElement = merkleProof.proof[i];
            
            if (index % 2 == 0) {
                // If index is even, proof element goes on the right
                computedHash = doubleSha256(abi.encodePacked(computedHash, proofElement));
            } else {
                // If index is odd, proof element goes on the left
                computedHash = doubleSha256(abi.encodePacked(proofElement, computedHash));
            }
            
            index = index / 2;
        }
        
        return computedHash == merkleProof.merkleRoot;
    }
    
    /**
     * @dev Check if a Bitcoin transaction has been verified
     * @param txHash The transaction hash to check
     * @return True if transaction has been verified
     */
    function isTransactionVerified(bytes32 txHash) public view returns (bool) {
        return verifiedTransactions[txHash];
    }
    
    /**
     * @dev Get block header by hash
     * @param blockHash The block hash
     * @return The stored block header
     */
    function getBlockHeader(bytes32 blockHash) public view returns (BlockHeader memory) {
        require(blockHeaders[blockHash].exists, "Block header not found");
        return blockHeaders[blockHash];
    }
    
    /**
     * @dev Get block hash by height
     * @param height The block height
     * @return The block hash at that height
     */
    function getBlockHashByHeight(uint256 height) public view returns (bytes32) {
        return blocksByHeight[height];
    }
    
    /**
     * @dev Verify multiple proofs in a single transaction (batch verification)
     * @param headerBytesArray Array of block headers
     * @param merkleProofs Array of merkle proofs
     * @param blockHeights Array of block heights
     * @return Array of verification results
     */
    function batchVerifyProofs(
        bytes[] memory headerBytesArray,
        MerkleProof[] memory merkleProofs,
        uint256[] memory blockHeights
    ) public returns (bool[] memory) {
        require(
            headerBytesArray.length == merkleProofs.length &&
            merkleProofs.length == blockHeights.length,
            "Array lengths must match"
        );
        
        bool[] memory results = new bool[](headerBytesArray.length);
        
        for (uint256 i = 0; i < headerBytesArray.length; i++) {
            results[i] = verifyProof(headerBytesArray[i], merkleProofs[i], blockHeights[i]);
        }
        
        return results;
    }
    
    // Utility functions
    
    /**
     * @dev Compute double SHA256 hash (Bitcoin's hashing algorithm)
     * @param data Input data
     * @return Double SHA256 hash
     */
    function doubleSha256(bytes memory data) public pure returns (bytes32) {
        return sha256(abi.encodePacked(sha256(data)));
    }
    
    /**
     * @dev Reverse byte order of bytes32 (for Bitcoin little-endian format)
     * @param input Input bytes32
     * @return Reversed bytes32
     */
    function reverseBytes32(bytes32 input) public pure returns (bytes32) {
        bytes memory temp = new bytes(32);
        bytes32 data = input;
        
        for (uint256 i = 0; i < 32; i++) {
            temp[i] = data[31 - i];
        }
        
        return bytes32(temp);
    }
    
    /**
     * @dev Reverse byte order of bytes4 (for Bitcoin little-endian format)
     * @param input Input bytes4
     * @return Reversed bytes4
     */
    function reverseBytes4(bytes4 input) public pure returns (bytes4) {
        bytes memory temp = new bytes(4);
        bytes4 data = input;
        
        for (uint256 i = 0; i < 4; i++) {
            temp[i] = data[3 - i];
        }
        
        return bytes4(temp);
    }
    
    /**
     * @dev Extract slice from bytes array
     * @param data Source bytes array
     * @param start Start index
     * @param length Length of slice
     * @return Extracted bytes
     */
    function slice(bytes memory data, uint256 start, uint256 length) public pure returns (bytes memory) {
        require(start + length <= data.length, "Slice out of bounds");
        
        bytes memory result = new bytes(length);
        for (uint256 i = 0; i < length; i++) {
            result[i] = data[start + i];
        }
        
        return result;
    }
    
    /**
     * @dev Get contract version
     * @return Version string
     */
    function version() public pure returns (string memory) {
        return "1.0.0";
    }
    
    /**
     * @dev Get total number of verified transactions
     * @return Count of verified transactions
     */
    function getVerifiedTransactionCount() public view returns (uint256) {
        // Note: This is an approximation since we can't iterate over mapping keys
        // In production, you might want to maintain a separate counter
        return 0; // Placeholder - would need additional storage to track count
    }
}