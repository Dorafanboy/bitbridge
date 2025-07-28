package bitcoin

import (
	"fmt"
	"log"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
)

type Client struct {
	rpcClient *rpcclient.Client
	network   *chaincfg.Params
}

type UTXOInfo struct {
	TxID          string  `json:"txid"`
	Vout          uint32  `json:"vout"`
	Amount        float64 `json:"amount"`
	Confirmations int64   `json:"confirmations"`
	Address       string  `json:"address"`
	ScriptPubKey  string  `json:"scriptPubKey"`
	BlockHeight   int64   `json:"blockHeight"`
}

func NewClient(host string, port int, user, password, network string) (*Client, error) {
	connCfg := &rpcclient.ConnConfig{
		Host:         fmt.Sprintf("%s:%d", host, port),
		User:         user,
		Pass:         password,
		HTTPPostMode: true,
		DisableTLS:   true,
	}

	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create RPC client: %v", err)
	}

	var netParams *chaincfg.Params
	switch network {
	case "mainnet":
		netParams = &chaincfg.MainNetParams
	case "testnet":
		netParams = &chaincfg.TestNet3Params
	case "regtest":
		netParams = &chaincfg.RegressionNetParams
	default:
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	return &Client{
		rpcClient: client,
		network:   netParams,
	}, nil
}

func (c *Client) GetBlockCount() (int64, error) {
	return c.rpcClient.GetBlockCount()
}

func (c *Client) GetNewAddress() (string, error) {
	address, err := c.rpcClient.GetNewAddress("")
	if err != nil {
		return "", err
	}
	return address.String(), nil
}

func (c *Client) GetAddressUTXOs(address string) ([]UTXOInfo, error) {
	addr, err := btcutil.DecodeAddress(address, c.network)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %v", err)
	}

	unspent, err := c.rpcClient.ListUnspentMinMaxAddresses(1, 9999999, []btcutil.Address{addr})
	if err != nil {
		return nil, fmt.Errorf("failed to get UTXOs: %v", err)
	}

	var utxos []UTXOInfo
	for _, utxo := range unspent {
		utxos = append(utxos, UTXOInfo{
			TxID:          utxo.TxID,
			Vout:          utxo.Vout,
			Amount:        utxo.Amount,
			Confirmations: utxo.Confirmations,
			Address:       utxo.Address,
			ScriptPubKey:  utxo.ScriptPubKey,
		})
	}

	return utxos, nil
}

func (c *Client) GetTransaction(txid string) (*rpcclient.GetTransactionResult, error) {
	return c.rpcClient.GetTransaction(txid)
}

func (c *Client) GetRawTransaction(txid string) (*btcutil.Tx, error) {
	hash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		return nil, err
	}

	return c.rpcClient.GetRawTransaction(hash)
}

func (c *Client) GetBlockHash(height int64) (string, error) {
	hash, err := c.rpcClient.GetBlockHash(height)
	if err != nil {
		return "", err
	}
	return hash.String(), nil
}

func (c *Client) WatchAddress(address string) error {
	addr, err := btcutil.DecodeAddress(address, c.network)
	if err != nil {
		return fmt.Errorf("invalid address: %v", err)
	}

	err = c.rpcClient.ImportAddress(addr.String())
	if err != nil {
		log.Printf("Warning: failed to import address %s: %v", address, err)
	}

	return nil
}

func (c *Client) GenerateDepositAddress() (string, error) {
	address, err := c.rpcClient.GetNewAddress("utxo-bridge")
	if err != nil {
		return "", fmt.Errorf("failed to generate address: %v", err)
	}

	err = c.WatchAddress(address.String())
	if err != nil {
		log.Printf("Warning: failed to watch address %s: %v", address.String(), err)
	}

	return address.String(), nil
}

func (c *Client) SendBitcoin(toAddress string, amount float64) (string, error) {
	addr, err := btcutil.DecodeAddress(toAddress, c.network)
	if err != nil {
		return "", fmt.Errorf("invalid address: %v", err)
	}

	amountBTC, err := btcutil.NewAmount(amount)
	if err != nil {
		return "", fmt.Errorf("invalid amount: %v", err)
	}

	txHash, err := c.rpcClient.SendToAddress(addr, amountBTC)
	if err != nil {
		return "", fmt.Errorf("failed to send bitcoin: %v", err)
	}

	return txHash.String(), nil
}

func (c *Client) GetNetworkInfo() (string, error) {
	info, err := c.rpcClient.GetBlockChainInfo()
	if err != nil {
		return "", err
	}
	return info.Chain, nil
}

func (c *Client) Close() {
	if c.rpcClient != nil {
		c.rpcClient.Shutdown()
	}
}