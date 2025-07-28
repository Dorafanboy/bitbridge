package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware handles Cross-Origin Resource Sharing
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-API-Version")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// APIVersionMiddleware handles API versioning
func APIVersionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		version := c.GetHeader("X-API-Version")
		if version == "" {
			version = c.DefaultQuery("version", "v1")
		}
		
		// Store version in context for handlers to use
		c.Set("api_version", version)
		
		// Add version to response headers
		c.Header("X-API-Version", version)
		
		c.Next()
	}
}

// RateLimitMiddleware implements basic rate limiting
func RateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
	// Simple in-memory rate limiter - for production use Redis or similar
	clients := make(map[string][]time.Time)
	
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()
		
		// Clean old requests (older than 1 minute)
		if requests, exists := clients[clientIP]; exists {
			var validRequests []time.Time
			for _, reqTime := range requests {
				if now.Sub(reqTime) < time.Minute {
					validRequests = append(validRequests, reqTime)
				}
			}
			clients[clientIP] = validRequests
		}
		
		// Check rate limit
		if len(clients[clientIP]) >= requestsPerMinute {
			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error: "Rate limit exceeded",
				Code:  "RATE_LIMIT_EXCEEDED",
				Details: map[string]interface{}{
					"limit":     requestsPerMinute,
					"window":    "1 minute",
					"retry_after": 60,
				},
			})
			c.Abort()
			return
		}
		
		// Add current request
		clients[clientIP] = append(clients[clientIP], now)
		
		c.Next()
	}
}

// RequestLoggingMiddleware logs API requests
func RequestLoggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %d %s %s\n",
			param.TimeStamp.Format("2006-01-02 15:04:05"),
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
			param.ClientIP,
		)
	})
}

// ErrorHandlingMiddleware handles panics and errors globally
func ErrorHandlingMiddleware() gin.HandlerFunc {
	return gin.Recovery()
}

// ContentTypeMiddleware ensures JSON content type for API responses
func ContentTypeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.Next()
	}
}

// ValidationMiddleware provides request validation utilities
func ValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add validation helper functions to context
		c.Set("validate_bitcoin_address", validateBitcoinAddress)
		c.Set("validate_ethereum_address", validateEthereumAddress)
		c.Set("validate_transaction_hash", validateTransactionHash)
		
		c.Next()
	}
}

// Validation helper functions
func validateBitcoinAddress(address string) bool {
	// Basic Bitcoin address validation
	if len(address) < 26 || len(address) > 62 {
		return false
	}
	
	// Check for valid prefixes
	validPrefixes := []string{"1", "3", "bc1", "tb1", "2", "m", "n", "bcrt1"}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(address, prefix) {
			return true
		}
	}
	
	return false
}

func validateEthereumAddress(address string) bool {
	// Ethereum address validation (42 characters with 0x prefix)
	if len(address) != 42 {
		return false
	}
	
	if !strings.HasPrefix(address, "0x") {
		return false
	}
	
	// Check if remaining characters are hex
	for _, char := range address[2:] {
		if !((char >= '0' && char <= '9') || 
			 (char >= 'a' && char <= 'f') || 
			 (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	
	return true
}

func validateTransactionHash(hash string) bool {
	// Transaction hash validation (64 hex characters, optionally with 0x prefix)
	cleanHash := hash
	if strings.HasPrefix(hash, "0x") {
		cleanHash = hash[2:]
	}
	
	if len(cleanHash) != 64 {
		return false
	}
	
	// Check if all characters are hex
	for _, char := range cleanHash {
		if !((char >= '0' && char <= '9') || 
			 (char >= 'a' && char <= 'f') || 
			 (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	
	return true
}

// MetricsMiddleware collects basic API metrics
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		c.Next()
		
		// Calculate request duration
		duration := time.Since(start)
		
		// Add metrics headers
		c.Header("X-Response-Time", duration.String())
		c.Header("X-Request-ID", generateRequestID())
		
		// Log metrics (in production would send to metrics system)
		if gin.IsDebugging() {
			fmt.Printf("API Metrics: %s %s - %d - %v\n", 
				c.Request.Method, 
				c.Request.URL.Path, 
				c.Writer.Status(), 
				duration)
		}
	}
}

func generateRequestID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// SecurityMiddleware adds basic security headers
func SecurityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		
		c.Next()
	}
}