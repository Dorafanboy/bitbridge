package indexer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"bitbridge/internal/bitcoin"
	"bitbridge/pkg/types"
)

type UTXOMonitor struct {
	btcClient      *bitcoin.Client
	watchAddresses map[string]bool
	utxoStore      map[string]*types.UTXO
	callbacks      []UTXOCallback
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
}

type UTXOCallback func(utxo *types.UTXO, event string)

func NewUTXOMonitor(btcClient *bitcoin.Client) *UTXOMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &UTXOMonitor{
		btcClient:      btcClient,
		watchAddresses: make(map[string]bool),
		utxoStore:      make(map[string]*types.UTXO),
		callbacks:      make([]UTXOCallback, 0),
		ctx:            ctx,
		cancel:         cancel,
	}
}

func (m *UTXOMonitor) AddWatchAddress(address string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := m.btcClient.WatchAddress(address)
	if err != nil {
		return fmt.Errorf("failed to watch address: %v", err)
	}

	m.watchAddresses[address] = true
	log.Printf("Now watching Bitcoin address: %s", address)
	
	return nil
}

func (m *UTXOMonitor) RemoveWatchAddress(address string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.watchAddresses, address)
	log.Printf("Stopped watching Bitcoin address: %s", address)
}

func (m *UTXOMonitor) AddCallback(callback UTXOCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callbacks = append(m.callbacks, callback)
}

func (m *UTXOMonitor) Start() {
	log.Println("Starting UTXO monitor...")
	
	go m.monitorLoop()
}

func (m *UTXOMonitor) Stop() {
	log.Println("Stopping UTXO monitor...")
	m.cancel()
}

func (m *UTXOMonitor) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			log.Println("UTXO monitor stopped")
			return
		case <-ticker.C:
			m.checkForNewUTXOs()
		}
	}
}

func (m *UTXOMonitor) checkForNewUTXOs() {
	m.mu.RLock()
	addresses := make([]string, 0, len(m.watchAddresses))
	for addr := range m.watchAddresses {
		addresses = append(addresses, addr)
	}
	m.mu.RUnlock()

	for _, address := range addresses {
		utxos, err := m.btcClient.GetAddressUTXOs(address)
		if err != nil {
			log.Printf("Error checking UTXOs for address %s: %v", address, err)
			continue
		}

		for _, utxo := range utxos {
			utxoKey := fmt.Sprintf("%s:%d", utxo.TxID, utxo.Vout)
			
			m.mu.Lock()
			existingUTXO, exists := m.utxoStore[utxoKey]
			
			if !exists {
				// New UTXO found
				newUTXO := &types.UTXO{
					TxID:         utxo.TxID,
					Vout:         utxo.Vout,
					Amount:       int64(utxo.Amount * 100000000), // Convert to satoshis
					Address:      utxo.Address,
					ScriptPubKey: utxo.ScriptPubKey,
					Confirmations: int(utxo.Confirmations),
					CreatedAt:    time.Now(),
				}
				
				m.utxoStore[utxoKey] = newUTXO
				m.mu.Unlock()
				
				log.Printf("New UTXO detected: %s:%d (%.8f BTC, %d confirmations)", 
					utxo.TxID, utxo.Vout, utxo.Amount, utxo.Confirmations)
				
				// Notify callbacks
				m.notifyCallbacks(newUTXO, "new")
				
			} else {
				// Update existing UTXO confirmations
				if existingUTXO.Confirmations != int(utxo.Confirmations) {
					existingUTXO.Confirmations = int(utxo.Confirmations)
					m.mu.Unlock()
					
					log.Printf("UTXO confirmation updated: %s:%d (%d confirmations)", 
						utxo.TxID, utxo.Vout, utxo.Confirmations)
					
					// Notify callbacks about confirmation update
					m.notifyCallbacks(existingUTXO, "confirmation_update")
				} else {
					m.mu.Unlock()
				}
			}
		}
	}
}

func (m *UTXOMonitor) notifyCallbacks(utxo *types.UTXO, event string) {
	m.mu.RLock()
	callbacks := make([]UTXOCallback, len(m.callbacks))
	copy(callbacks, m.callbacks)
	m.mu.RUnlock()

	for _, callback := range callbacks {
		go func(cb UTXOCallback) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("UTXO callback panic: %v", r)
				}
			}()
			cb(utxo, event)
		}(callback)
	}
}

func (m *UTXOMonitor) GetUTXO(txid string, vout uint32) (*types.UTXO, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	utxoKey := fmt.Sprintf("%s:%d", txid, vout)
	utxo, exists := m.utxoStore[utxoKey]
	return utxo, exists
}

func (m *UTXOMonitor) GetAllUTXOs() []*types.UTXO {
	m.mu.RLock()
	defer m.mu.RUnlock()

	utxos := make([]*types.UTXO, 0, len(m.utxoStore))
	for _, utxo := range m.utxoStore {
		utxos = append(utxos, utxo)
	}
	return utxos
}

func (m *UTXOMonitor) GetUTXOsByAddress(address string) []*types.UTXO {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var utxos []*types.UTXO
	for _, utxo := range m.utxoStore {
		if utxo.Address == address {
			utxos = append(utxos, utxo)
		}
	}
	return utxos
}