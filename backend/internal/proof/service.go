package proof

import (
	"context"
	"fmt"
	"sync"
	"time"

	"bitbridge/internal/bitcoin"

	"github.com/btcsuite/btcd/rpcclient"
)

// Service manages SPV proof generation and caching
type Service struct {
	generator         *Generator
	cache             *ProofCache
	minConfirmations  int32
	maxCacheSize      int
	cacheExpiration   time.Duration
}

// ServiceConfig for proof service
type ServiceConfig struct {
	BitcoinClient     *bitcoin.Client
	RPCClient         *rpcclient.Client
	MinConfirmations  int32
	MaxCacheSize      int
	CacheExpiration   time.Duration
}

// ProofCache implements a simple LRU cache for proofs
type ProofCache struct {
	mutex    sync.RWMutex
	proofs   map[string]*CachedProof
	maxSize  int
	expiry   time.Duration
}

// CachedProof wraps an SPV proof with metadata
type CachedProof struct {
	Proof     *SPVProof
	CreatedAt time.Time
	AccessedAt time.Time
	UseCount  int
}

// ProofRequest represents a request for SPV proof generation
type ProofRequest struct {
	TxHash      string `json:"tx_hash" binding:"required"`
	OutputIndex uint32 `json:"output_index"`
	RequiredConfirmations int32 `json:"required_confirmations"`
}

// ProofResponse represents the response containing SPV proof
type ProofResponse struct {
	Proof      *SPVProof `json:"proof"`
	Verified   bool      `json:"verified"`
	ProofSize  int       `json:"proof_size"`
	Cached     bool      `json:"cached"`
	GeneratedAt time.Time `json:"generated_at"`
}

func NewService(config ServiceConfig) *Service {
	if config.MinConfirmations == 0 {
		config.MinConfirmations = 6 // Bitcoin standard
	}
	if config.MaxCacheSize == 0 {
		config.MaxCacheSize = 1000
	}
	if config.CacheExpiration == 0 {
		config.CacheExpiration = 24 * time.Hour
	}

	generator := NewGenerator(Config{
		BitcoinClient: config.BitcoinClient,
		RPCClient:     config.RPCClient,
	})

	cache := &ProofCache{
		proofs:  make(map[string]*CachedProof),
		maxSize: config.MaxCacheSize,
		expiry:  config.CacheExpiration,
	}

	service := &Service{
		generator:        generator,
		cache:           cache,
		minConfirmations: config.MinConfirmations,
		maxCacheSize:     config.MaxCacheSize,
		cacheExpiration:  config.CacheExpiration,
	}

	// Start cache cleanup goroutine
	go service.startCacheCleanup()

	return service
}

// GenerateProof generates or retrieves cached SPV proof
func (s *Service) GenerateProof(ctx context.Context, req *ProofRequest) (*ProofResponse, error) {
	cacheKey := fmt.Sprintf("%s:%d", req.TxHash, req.OutputIndex)
	
	// Check cache first
	if cached := s.cache.Get(cacheKey); cached != nil {
		// Verify cached proof still meets confirmation requirements
		if req.RequiredConfirmations == 0 || cached.Proof.Confirmations >= req.RequiredConfirmations {
			return &ProofResponse{
				Proof:       cached.Proof,
				Verified:    true,
				ProofSize:   s.generator.GetProofSize(cached.Proof),
				Cached:      true,
				GeneratedAt: cached.CreatedAt,
			}, nil
		}
	}

	// Generate new proof
	proof, err := s.generator.GetProofForUTXO(req.TxHash, req.OutputIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to generate proof: %w", err)
	}

	// Check minimum confirmations
	minConf := req.RequiredConfirmations
	if minConf == 0 {
		minConf = s.minConfirmations
	}
	
	if err := s.generator.ValidateMinimumConfirmations(proof, minConf); err != nil {
		return nil, fmt.Errorf("confirmation validation failed: %w", err)
	}

	// Verify the proof
	if err := s.generator.VerifyProof(proof); err != nil {
		return nil, fmt.Errorf("proof verification failed: %w", err)
	}

	// Cache the proof
	s.cache.Set(cacheKey, proof)

	return &ProofResponse{
		Proof:       proof,
		Verified:    true,
		ProofSize:   s.generator.GetProofSize(proof),
		Cached:      false,
		GeneratedAt: time.Now(),
	}, nil
}

// VerifyProof verifies an existing SPV proof
func (s *Service) VerifyProof(ctx context.Context, proof *SPVProof) error {
	return s.generator.VerifyProof(proof)
}

// GetProofForContract formats proof for smart contract consumption
func (s *Service) GetProofForContract(ctx context.Context, req *ProofRequest) (map[string]interface{}, error) {
	proofResp, err := s.GenerateProof(ctx, req)
	if err != nil {
		return nil, err
	}

	return s.generator.FormatProofForContract(proofResp.Proof), nil
}

// GetCacheStats returns cache statistics
func (s *Service) GetCacheStats() map[string]interface{} {
	s.cache.mutex.RLock()
	defer s.cache.mutex.RUnlock()

	totalHits := 0
	oldestAccess := time.Now()
	newestAccess := time.Time{}

	for _, cached := range s.cache.proofs {
		totalHits += cached.UseCount
		if cached.AccessedAt.Before(oldestAccess) {
			oldestAccess = cached.AccessedAt
		}
		if cached.AccessedAt.After(newestAccess) {
			newestAccess = cached.AccessedAt
		}
	}

	return map[string]interface{}{
		"cache_size":     len(s.cache.proofs),
		"max_cache_size": s.maxCacheSize,
		"total_hits":     totalHits,
		"oldest_access":  oldestAccess,
		"newest_access":  newestAccess,
		"expiry_duration": s.cacheExpiration.String(),
	}
}

// ClearCache clears all cached proofs
func (s *Service) ClearCache() {
	s.cache.mutex.Lock()
	defer s.cache.mutex.Unlock()
	s.cache.proofs = make(map[string]*CachedProof)
}

// ProofCache methods

// Get retrieves a proof from cache
func (pc *ProofCache) Get(key string) *CachedProof {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	cached, exists := pc.proofs[key]
	if !exists {
		return nil
	}

	// Check if expired
	if time.Since(cached.CreatedAt) > pc.expiry {
		return nil
	}

	// Update access stats
	cached.AccessedAt = time.Now()
	cached.UseCount++

	return cached
}

// Set stores a proof in cache
func (pc *ProofCache) Set(key string, proof *SPVProof) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	// Check if we need to evict old entries
	if len(pc.proofs) >= pc.maxSize {
		pc.evictOldest()
	}

	pc.proofs[key] = &CachedProof{
		Proof:      proof,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		UseCount:   1,
	}
}

// evictOldest removes the oldest entry from cache
func (pc *ProofCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, cached := range pc.proofs {
		if oldestKey == "" || cached.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = cached.AccessedAt
		}
	}

	if oldestKey != "" {
		delete(pc.proofs, oldestKey)
	}
}

// cleanup removes expired entries
func (pc *ProofCache) cleanup() {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	now := time.Now()
	for key, cached := range pc.proofs {
		if now.Sub(cached.CreatedAt) > pc.expiry {
			delete(pc.proofs, key)
		}
	}
}

// startCacheCleanup starts a goroutine to periodically clean expired cache entries
func (s *Service) startCacheCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.cache.cleanup()
	}
}

// BatchGenerateProofs generates proofs for multiple transactions
func (s *Service) BatchGenerateProofs(ctx context.Context, requests []*ProofRequest) ([]*ProofResponse, error) {
	responses := make([]*ProofResponse, len(requests))
	errors := make([]error, len(requests))

	// Use goroutines for parallel processing
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // Limit concurrent requests

	for i, req := range requests {
		wg.Add(1)
		go func(index int, request *ProofRequest) {
			defer wg.Done()
			
			sem <- struct{}{} // Acquire semaphore
			defer func() { <-sem }() // Release semaphore

			resp, err := s.GenerateProof(ctx, request)
			responses[index] = resp
			errors[index] = err
		}(i, req)
	}

	wg.Wait()

	// Check for errors
	var combinedError error
	for i, err := range errors {
		if err != nil {
			if combinedError == nil {
				combinedError = fmt.Errorf("batch error at index %d: %w", i, err)
			}
		}
	}

	return responses, combinedError
}

// GetMinimumConfirmations returns the service's minimum confirmation requirement
func (s *Service) GetMinimumConfirmations() int32 {
	return s.minConfirmations
}

// SetMinimumConfirmations updates the minimum confirmation requirement
func (s *Service) SetMinimumConfirmations(confirmations int32) {
	s.minConfirmations = confirmations
}