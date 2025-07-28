package bitcoin

import (
	"fmt"
	"log"
	"strings"

	"bitbridge/internal/indexer"
	"bitbridge/pkg/config"
	"bitbridge/pkg/types"
)

type Service struct {
	client      *Client
	monitor     *indexer.UTXOMonitor
	config      *config.BitcoinConfig
	depositAddresses map[string]bool
}

func NewService(cfg *config.BitcoinConfig) (*Service, error) {
	client, err := NewClient(cfg.RPCHost, cfg.RPCPort, cfg.RPCUser, cfg.RPCPassword, cfg.Network)
	if err != nil {
		return nil, fmt.Errorf("failed to create Bitcoin client: %v", err)
	}

	// Test connection
	_, err = client.GetBlockCount()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Bitcoin node: %v", err)
	}

	monitor := indexer.NewUTXOMonitor(client)
	
	service := &Service{
		client:           client,
		monitor:          monitor,
		config:           cfg,
		depositAddresses: make(map[string]bool),
	}

	// Set up UTXO callback for new deposits
	monitor.AddCallback(service.handleUTXOEvent)

	log.Printf("Bitcoin service initialized for network: %s", cfg.Network)
	return service, nil
}

func (s *Service) Start() error {
	log.Println("Starting Bitcoin service...")
	
	// Test network connectivity
	networkInfo, err := s.client.GetNetworkInfo()
	if err != nil {
		return fmt.Errorf("failed to get network info: %v", err)
	}
	
	log.Printf("Connected to Bitcoin %s network", networkInfo)
	
	// Start UTXO monitoring
	s.monitor.Start()
	
	return nil
}

func (s *Service) Stop() {
	log.Println("Stopping Bitcoin service...")
	s.monitor.Stop()
	s.client.Close()
}

func (s *Service) GenerateDepositAddress() (string, error) {
	address, err := s.client.GenerateDepositAddress()
	if err != nil {
		return "", err
	}

	// Add to watch list
	err = s.monitor.AddWatchAddress(address)
	if err != nil {
		log.Printf("Warning: failed to watch new deposit address %s: %v", address, err)
	}

	s.depositAddresses[address] = true
	log.Printf("Generated new deposit address: %s", address)
	
	return address, nil
}

func (s *Service) WatchAddress(address string) error {
	err := s.monitor.AddWatchAddress(address)
	if err != nil {
		return err
	}

	s.depositAddresses[address] = true
	return nil
}

func (s *Service) GetAddressUTXOs(address string) ([]*types.UTXO, error) {
	utxos := s.monitor.GetUTXOsByAddress(address)
	return utxos, nil
}

func (s *Service) GetUTXO(txid string, vout uint32) (*types.UTXO, error) {
	utxo, exists := s.monitor.GetUTXO(txid, vout)
	if !exists {
		return nil, fmt.Errorf("UTXO not found: %s:%d", txid, vout)
	}
	return utxo, nil
}

func (s *Service) GetAllWatchedUTXOs() []*types.UTXO {
	return s.monitor.GetAllUTXOs()
}

func (s *Service) ValidateTransaction(txid string, vout uint32, expectedAmount int64) error {
	utxo, exists := s.monitor.GetUTXO(txid, vout)
	if !exists {
		return fmt.Errorf("UTXO not found: %s:%d", txid, vout)
	}

	if utxo.Amount != expectedAmount {
		return fmt.Errorf("amount mismatch: expected %d, got %d", expectedAmount, utxo.Amount)
	}

	if utxo.Confirmations < 3 { // Require at least 3 confirmations
		return fmt.Errorf("insufficient confirmations: %d (required: 3)", utxo.Confirmations)
	}

	return nil
}

func (s *Service) SendBitcoin(toAddress string, amount float64) (string, error) {
	if s.config.Network == "mainnet" {
		return "", fmt.Errorf("Bitcoin sending disabled on mainnet for safety")
	}

	txid, err := s.client.SendBitcoin(toAddress, amount)
	if err != nil {
		return "", err
	}

	log.Printf("Sent %.8f BTC to %s, txid: %s", amount, toAddress, txid)
	return txid, nil
}

func (s *Service) GetDepositAddresses() []string {
	addresses := make([]string, 0, len(s.depositAddresses))
	for addr := range s.depositAddresses {
		addresses = append(addresses, addr)
	}
	return addresses
}

func (s *Service) IsValidBitcoinAddress(address string) bool {
	// Basic validation - check if it's a valid format for the network
	switch s.config.Network {
	case "mainnet":
		return strings.HasPrefix(address, "1") || strings.HasPrefix(address, "3") || strings.HasPrefix(address, "bc1")
	case "testnet":
		return strings.HasPrefix(address, "m") || strings.HasPrefix(address, "n") || strings.HasPrefix(address, "2") || strings.HasPrefix(address, "tb1")
	case "regtest":
		return strings.HasPrefix(address, "m") || strings.HasPrefix(address, "n") || strings.HasPrefix(address, "2") || strings.HasPrefix(address, "bcrt1")
	}
	return false
}

func (s *Service) handleUTXOEvent(utxo *types.UTXO, event string) {
	log.Printf("UTXO Event [%s]: %s:%d - %.8f BTC (%d confirmations)", 
		event, utxo.TxID, utxo.Vout, float64(utxo.Amount)/100000000, utxo.Confirmations)

	// Check if this is a deposit to one of our watched addresses
	if s.depositAddresses[utxo.Address] {
		if event == "new" {
			log.Printf("New deposit detected! UTXO: %s:%d", utxo.TxID, utxo.Vout)
			// TODO: Trigger token creation process
		} else if event == "confirmation_update" && utxo.Confirmations >= 3 {
			log.Printf("Deposit confirmed! UTXO: %s:%d (%d confirmations)", 
				utxo.TxID, utxo.Vout, utxo.Confirmations)
			// TODO: Finalize token creation if not already done
		}
	}
}

func (s *Service) GetNetworkInfo() (string, int64, error) {
	network, err := s.client.GetNetworkInfo()
	if err != nil {
		return "", 0, err
	}

	blockCount, err := s.client.GetBlockCount()
	if err != nil {
		return network, 0, err
	}

	return network, blockCount, nil
}