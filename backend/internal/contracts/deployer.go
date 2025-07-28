package contracts

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Deployer struct {
	client     *ethclient.Client
	privateKey *ecdsa.PrivateKey
	chainID    *big.Int
}

type DeploymentResult struct {
	ContractAddress common.Address `json:"contract_address"`
	TransactionHash common.Hash    `json:"transaction_hash"`
	BlockNumber     uint64         `json:"block_number"`
	GasUsed         uint64         `json:"gas_used"`
}

func NewDeployer(client *ethclient.Client, privateKeyHex string, chainID *big.Int) (*Deployer, error) {
	// Remove 0x prefix if present
	if strings.HasPrefix(privateKeyHex, "0x") {
		privateKeyHex = privateKeyHex[2:]
	}

	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &Deployer{
		client:     client,
		privateKey: privateKey,
		chainID:    chainID,
	}, nil
}

// DeploySPVVerifier deploys the SPV Verifier smart contract
func (d *Deployer) DeploySPVVerifier(ctx context.Context) (*DeploymentResult, error) {
	// Get the contract bytecode and ABI
	bytecode, abi, err := d.loadContractData("SPVVerifier")
	if err != nil {
		return nil, fmt.Errorf("failed to load contract data: %w", err)
	}

	// Create transactor
	auth, err := d.createTransactor(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transactor: %w", err)
	}

	// Deploy contract
	address, tx, _, err := bind.DeployContract(auth, *abi, bytecode, d.client)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy contract: %w", err)
	}

	// Wait for transaction to be mined
	receipt, err := bind.WaitMined(ctx, d.client, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for transaction: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, fmt.Errorf("contract deployment failed")
	}

	return &DeploymentResult{
		ContractAddress: address,
		TransactionHash: tx.Hash(),
		BlockNumber:     receipt.BlockNumber.Uint64(),
		GasUsed:         receipt.GasUsed,
	}, nil
}

// InteractWithSPVVerifier creates a bound contract instance for interaction
func (d *Deployer) InteractWithSPVVerifier(contractAddress common.Address) (*SPVVerifierContract, error) {
	// Load ABI
	_, abiData, err := d.loadContractData("SPVVerifier")
	if err != nil {
		return nil, fmt.Errorf("failed to load contract ABI: %w", err)
	}

	// Create bound contract
	contract := bind.NewBoundContract(contractAddress, *abiData, d.client, d.client, d.client)

	return &SPVVerifierContract{
		contract: contract,
		address:  contractAddress,
		client:   d.client,
		deployer: d,
	}, nil
}

// createTransactor creates a transactor for contract interactions
func (d *Deployer) createTransactor(ctx context.Context) (*bind.TransactOpts, error) {
	publicKey := d.privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error casting public key to ECDSA")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	
	nonce, err := d.client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := d.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(d.privateKey, d.chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to create transactor: %w", err)
	}

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(3000000) // 3M gas limit
	auth.GasPrice = gasPrice

	return auth, nil
}

// loadContractData loads bytecode and ABI for a contract
func (d *Deployer) loadContractData(contractName string) ([]byte, *abi.ABI, error) {
	// This is a simplified version - in practice, you'd compile Solidity contracts
	// and load the resulting JSON artifacts
	
	// For now, return placeholder data - you'll need to compile the Solidity contract
	// and extract the bytecode and ABI from the compilation output
	
	return nil, nil, fmt.Errorf("contract compilation not implemented - please compile %s.sol manually", contractName)
}

// SPVVerifierContract represents a bound SPV Verifier contract
type SPVVerifierContract struct {
	contract *bind.BoundContract
	address  common.Address
	client   *ethclient.Client
	deployer *Deployer
}

// VerifyProof calls the verifyProof function on the smart contract
func (c *SPVVerifierContract) VerifyProof(ctx context.Context, headerBytes []byte, proof ProofData, blockHeight *big.Int) (*types.Transaction, error) {
	auth, err := c.deployer.createTransactor(ctx)
	if err != nil {
		return nil, err
	}

	// Convert proof data to contract format
	merkleProof := MerkleProofContract{
		Proof:      proof.MerkleProof,
		Index:      big.NewInt(int64(proof.Index)),
		TxHash:     proof.TxHash,
		MerkleRoot: proof.MerkleRoot,
	}

	tx, err := c.contract.Transact(auth, "verifyProof", headerBytes, merkleProof, blockHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to call verifyProof: %w", err)
	}

	return tx, nil
}

// IsTransactionVerified checks if a transaction has been verified
func (c *SPVVerifierContract) IsTransactionVerified(ctx context.Context, txHash [32]byte) (bool, error) {
	var result []interface{}
	err := c.contract.Call(&bind.CallOpts{Context: ctx}, &result, "isTransactionVerified", txHash)
	if err != nil {
		return false, err
	}

	return result[0].(bool), nil
}

// GetBlockHeader retrieves stored block header
func (c *SPVVerifierContract) GetBlockHeader(ctx context.Context, blockHash [32]byte) (*BlockHeaderContract, error) {
	var result []interface{}
	err := c.contract.Call(&bind.CallOpts{Context: ctx}, &result, "getBlockHeader", blockHash)
	if err != nil {
		return nil, err
	}

	// Parse result into BlockHeaderContract
	// This would need to match the exact return structure from the contract
	return &BlockHeaderContract{
		Version:    result[0].(uint32),
		PrevBlock:  result[1].([32]byte),
		MerkleRoot: result[2].([32]byte),
		Timestamp:  result[3].(uint32),
		Bits:       result[4].(uint32),
		Nonce:      result[5].(uint32),
		Height:     result[6].(*big.Int),
		Exists:     result[7].(bool),
	}, nil
}

// Contract data structures matching Solidity contract

type ProofData struct {
	MerkleProof [][32]byte `json:"merkle_proof"`
	Index       uint32     `json:"index"`
	TxHash      [32]byte   `json:"tx_hash"`
	MerkleRoot  [32]byte   `json:"merkle_root"`
}

type MerkleProofContract struct {
	Proof      [][32]byte `json:"proof"`
	Index      *big.Int   `json:"index"`
	TxHash     [32]byte   `json:"txHash"`
	MerkleRoot [32]byte   `json:"merkleRoot"`
}

type BlockHeaderContract struct {
	Version    uint32     `json:"version"`
	PrevBlock  [32]byte   `json:"prevBlock"`
	MerkleRoot [32]byte   `json:"merkleRoot"`
	Timestamp  uint32     `json:"timestamp"`
	Bits       uint32     `json:"bits"`
	Nonce      uint32     `json:"nonce"`
	Height     *big.Int   `json:"height"`
	Exists     bool       `json:"exists"`
}

// EstimateDeploymentGas estimates gas needed for contract deployment
func (d *Deployer) EstimateDeploymentGas(ctx context.Context) (uint64, error) {
	bytecode, abi, err := d.loadContractData("SPVVerifier")
	if err != nil {
		return 0, fmt.Errorf("failed to load contract data: %w", err)
	}

	publicKey := d.privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return 0, fmt.Errorf("error casting public key to ECDSA")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	// Pack constructor arguments (none for SPVVerifier)
	input, err := abi.Pack("")
	if err != nil {
		return 0, fmt.Errorf("failed to pack constructor arguments: %w", err)
	}

	data := append(bytecode, input...)

	// Create call message for gas estimation
	msg := ethereum.CallMsg{
		From: fromAddress,
		Data: data,
	}

	gas, err := d.client.EstimateGas(ctx, msg)
	if err != nil {
		return 0, fmt.Errorf("failed to estimate gas: %w", err)
	}

	return gas, nil
}

// GetContractAddress returns the address of a deployed contract
func (c *SPVVerifierContract) GetAddress() common.Address {
	return c.address
}

// BatchVerifyProofs calls the batch verification function
func (c *SPVVerifierContract) BatchVerifyProofs(ctx context.Context, headerBytesArray [][]byte, proofs []ProofData, blockHeights []*big.Int) (*types.Transaction, error) {
	auth, err := c.deployer.createTransactor(ctx)
	if err != nil {
		return nil, err
	}

	// Convert proof data to contract format
	merkleProofs := make([]MerkleProofContract, len(proofs))
	for i, proof := range proofs {
		merkleProofs[i] = MerkleProofContract{
			Proof:      proof.MerkleProof,
			Index:      big.NewInt(int64(proof.Index)),
			TxHash:     proof.TxHash,
			MerkleRoot: proof.MerkleRoot,
		}
	}

	tx, err := c.contract.Transact(auth, "batchVerifyProofs", headerBytesArray, merkleProofs, blockHeights)
	if err != nil {
		return nil, fmt.Errorf("failed to call batchVerifyProofs: %w", err)
	}

	return tx, nil
}