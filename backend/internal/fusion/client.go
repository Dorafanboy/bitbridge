package fusion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	chainID    int64
	httpClient *http.Client
}

type Config struct {
	BaseURL string
	APIKey  string
	ChainID int64
}

// 1inch API response structures
type QuoteResponse struct {
	DstAmount          string                 `json:"dstAmount"`
	SrcAmount          string                 `json:"srcAmount"`
	EstimatedGas       string                 `json:"estimatedGas"`
	Protocols          [][]ProtocolSelection `json:"protocols"`
	EstimateGasError   *string               `json:"estimateGasError"`
}

type SwapResponse struct {
	DstAmount string    `json:"dstAmount"`
	SrcAmount string    `json:"srcAmount"`
	Tx        TxData    `json:"tx"`
	Protocols [][]ProtocolSelection `json:"protocols"`
}

type TxData struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Data     string `json:"data"`
	Value    string `json:"value"`
	GasPrice string `json:"gasPrice"`
	Gas      string `json:"gas"`
}

type ProtocolSelection struct {
	Name string  `json:"name"`
	Part float64 `json:"part"`
	FromTokenAddress string `json:"fromTokenAddress"`
	ToTokenAddress   string `json:"toTokenAddress"`
}

// Fusion+ specific structures
type FusionOrder struct {
	Maker        string `json:"maker"`
	MakerAsset   string `json:"makerAsset"`
	TakerAsset   string `json:"takerAsset"`
	MakingAmount string `json:"makingAmount"`
	TakingAmount string `json:"takingAmount"`
	Salt         string `json:"salt"`
	Receiver     string `json:"receiver"`
	Interactions string `json:"interactions"`
}

type FusionQuote struct {
	FromTokenAddress string `json:"fromTokenAddress"`
	ToTokenAddress   string `json:"toTokenAddress"`
	Amount           string `json:"amount"`
	FromAddress      string `json:"fromAddress"`
	Slippage         string `json:"slippage"`
	DisableEstimate  bool   `json:"disableEstimate"`
	AllowPartialFill bool   `json:"allowPartialFill"`
}

func NewClient(config Config) *Client {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.1inch.dev"
	}

	return &Client{
		baseURL: config.BaseURL,
		apiKey:  config.APIKey,
		chainID: config.ChainID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) GetQuote(ctx context.Context, srcToken, dstToken, amount, fromAddress string) (*QuoteResponse, error) {
	params := url.Values{}
	params.Set("src", srcToken)
	params.Set("dst", dstToken)
	params.Set("amount", amount)
	params.Set("from", fromAddress)

	endpoint := fmt.Sprintf("/swap/v6.0/%d/quote", c.chainID)
	
	var quote QuoteResponse
	err := c.makeRequest(ctx, "GET", endpoint, params, nil, &quote)
	if err != nil {
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}

	return &quote, nil
}

func (c *Client) GetSwap(ctx context.Context, srcToken, dstToken, amount, fromAddress string, slippage float64) (*SwapResponse, error) {
	params := url.Values{}
	params.Set("src", srcToken)
	params.Set("dst", dstToken)
	params.Set("amount", amount)
	params.Set("from", fromAddress)
	params.Set("slippage", fmt.Sprintf("%.1f", slippage))

	endpoint := fmt.Sprintf("/swap/v6.0/%d/swap", c.chainID)
	
	var swap SwapResponse
	err := c.makeRequest(ctx, "GET", endpoint, params, nil, &swap)
	if err != nil {
		return nil, fmt.Errorf("failed to get swap: %w", err)
	}

	return &swap, nil
}

// Fusion+ methods
func (c *Client) GetFusionQuote(ctx context.Context, quote FusionQuote) (*QuoteResponse, error) {
	endpoint := fmt.Sprintf("/fusion/v1.0/%d/quote", c.chainID)
	
	var response QuoteResponse
	err := c.makeRequest(ctx, "POST", endpoint, nil, quote, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get fusion quote: %w", err)
	}

	return &response, nil
}

func (c *Client) CreateFusionOrder(ctx context.Context, order FusionOrder) (*FusionOrder, error) {
	endpoint := fmt.Sprintf("/fusion/v1.0/%d/order", c.chainID)
	
	var response FusionOrder
	err := c.makeRequest(ctx, "POST", endpoint, nil, order, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to create fusion order: %w", err)
	}

	return &response, nil
}

func (c *Client) makeRequest(ctx context.Context, method, endpoint string, params url.Values, body interface{}, result interface{}) error {
	var reqBody io.Reader
	
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	urlStr := c.baseURL + endpoint
	if params != nil {
		urlStr += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}