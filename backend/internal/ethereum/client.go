package ethereum

import (
	"context"
	"crypto/ecdsa"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Client struct {
	client     *ethclient.Client
	privateKey *ecdsa.PrivateKey
	address    common.Address
	chainID    *big.Int
}

type Config struct {
	RpcURL     string
	PrivateKey string
	ChainID    int64
}

func NewClient(config Config) (*Client, error) {
	client, err := ethclient.Dial(config.RpcURL)
	if err != nil {
		return nil, err
	}

	privateKey, err := crypto.HexToECDSA(config.PrivateKey)
	if err != nil {
		return nil, err
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("error casting public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	chainID := big.NewInt(config.ChainID)

	return &Client{
		client:     client,
		privateKey: privateKey,
		address:    address,
		chainID:    chainID,
	}, nil
}

func (c *Client) GetBalance(ctx context.Context, address common.Address) (*big.Int, error) {
	return c.client.BalanceAt(ctx, address, nil)
}

func (c *Client) GetNonce(ctx context.Context) (uint64, error) {
	return c.client.PendingNonceAt(ctx, c.address)
}

func (c *Client) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	return c.client.SendTransaction(ctx, tx)
}

func (c *Client) GetTransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return c.client.TransactionReceipt(ctx, txHash)
}

func (c *Client) GetBlockNumber(ctx context.Context) (uint64, error) {
	return c.client.BlockNumber(ctx)
}

func (c *Client) Close() {
	c.client.Close()
}

func (c *Client) GetAddress() common.Address {
	return c.address
}

func (c *Client) GetPrivateKey() *ecdsa.PrivateKey {
	return c.privateKey
}

func (c *Client) GetChainID() *big.Int {
	return c.chainID
}