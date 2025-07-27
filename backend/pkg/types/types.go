package types

import (
	"time"
)

// UTXO represents a Bitcoin unspent transaction output
type UTXO struct {
	TxID         string    `json:"txid"`
	Vout         uint32    `json:"vout"`
	Amount       int64     `json:"amount"` // satoshis
	ScriptPubKey string    `json:"script_pubkey"`
	Address      string    `json:"address"`
	Confirmations int      `json:"confirmations"`
	BlockHeight  int       `json:"block_height"`
	CreatedAt    time.Time `json:"created_at"`
}

// UTXOToken represents the ERC-20 token created from a UTXO
type UTXOToken struct {
	ID              string `json:"id"`
	TokenAddress    string `json:"token_address"`
	UTXO            UTXO   `json:"utxo"`
	TokenName       string `json:"token_name"`       // e.g., "UTXO_123abc"
	TokenSymbol     string `json:"token_symbol"`     // e.g., "UTXO123"
	TotalSupply     int64  `json:"total_supply"`     // equals UTXO amount
	OwnerAddress    string `json:"owner_address"`    // Ethereum address
	Status          string `json:"status"`           // created, active, burned
	CreatedAt       time.Time `json:"created_at"`
	BurnedAt        *time.Time `json:"burned_at,omitempty"`
}

// Transaction represents a cross-chain transaction
type Transaction struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`         // deposit, withdrawal, swap
	Status          string    `json:"status"`       // pending, confirmed, failed
	BitcoinTxID     string    `json:"bitcoin_txid,omitempty"`
	EthereumTxHash  string    `json:"ethereum_tx_hash,omitempty"`
	Amount          int64     `json:"amount"`       // satoshis
	FromAddress     string    `json:"from_address"`
	ToAddress       string    `json:"to_address"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Confirmations   int       `json:"confirmations"`
	RequiredConfirms int      `json:"required_confirms"`
}

// SwapRequest represents a request to swap tokens via 1inch
type SwapRequest struct {
	TokenAddress string `json:"token_address"`
	Amount       string `json:"amount"`
	ToToken      string `json:"to_token"`      // USDC, ETH, etc.
	Slippage     string `json:"slippage"`      // percentage
	FromAddress  string `json:"from_address"`  // user's Ethereum address
}

// SwapResponse represents the response from 1inch API
type SwapResponse struct {
	ToAmount     string      `json:"to_amount"`
	Tx           SwapTxData  `json:"tx"`
	Protocols    [][]Protocol `json:"protocols"`
}

type SwapTxData struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Data     string `json:"data"`
	Value    string `json:"value"`
	GasPrice string `json:"gas_price"`
	Gas      string `json:"gas"`
}

type Protocol struct {
	Name string `json:"name"`
	Part string `json:"part"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}