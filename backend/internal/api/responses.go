package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// StandardResponse represents the standard API response format
type StandardResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *ErrorInfo  `json:"error,omitempty"`
	Meta      *MetaInfo   `json:"meta,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Code    string                 `json:"code"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ErrorInfo contains detailed error information
type ErrorInfo struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// MetaInfo contains response metadata
type MetaInfo struct {
	RequestID     string `json:"request_id"`
	Version       string `json:"version"`
	ResponseTime  string `json:"response_time"`
	RateLimit     *RateLimitInfo `json:"rate_limit,omitempty"`
}

// RateLimitInfo contains rate limiting information
type RateLimitInfo struct {
	Limit     int `json:"limit"`
	Remaining int `json:"remaining"`
	Reset     int64 `json:"reset"`
}

// PaginationInfo contains pagination metadata
type PaginationInfo struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// SuccessResponse sends a successful response
func SuccessResponse(c *gin.Context, data interface{}) {
	response := StandardResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
		Meta:      buildMetaInfo(c),
	}
	
	c.JSON(http.StatusOK, response)
}

// CreatedResponse sends a resource created response
func CreatedResponse(c *gin.Context, data interface{}) {
	response := StandardResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
		Meta:      buildMetaInfo(c),
	}
	
	c.JSON(http.StatusCreated, response)
}

// ErrorResponseWithCode sends an error response with specific HTTP status code
func ErrorResponseWithCode(c *gin.Context, statusCode int, errorCode, message string, details map[string]interface{}) {
	response := StandardResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    errorCode,
			Message: message,
			Details: details,
		},
		Timestamp: time.Now(),
		Meta:      buildMetaInfo(c),
	}
	
	c.JSON(statusCode, response)
}

// BadRequestError sends a 400 Bad Request error
func BadRequestError(c *gin.Context, message string, details map[string]interface{}) {
	ErrorResponseWithCode(c, http.StatusBadRequest, "BAD_REQUEST", message, details)
}

// UnauthorizedError sends a 401 Unauthorized error
func UnauthorizedError(c *gin.Context, message string) {
	ErrorResponseWithCode(c, http.StatusUnauthorized, "UNAUTHORIZED", message, nil)
}

// ForbiddenError sends a 403 Forbidden error
func ForbiddenError(c *gin.Context, message string) {
	ErrorResponseWithCode(c, http.StatusForbidden, "FORBIDDEN", message, nil)
}

// NotFoundError sends a 404 Not Found error
func NotFoundError(c *gin.Context, message string) {
	ErrorResponseWithCode(c, http.StatusNotFound, "NOT_FOUND", message, nil)
}

// ConflictError sends a 409 Conflict error
func ConflictError(c *gin.Context, message string, details map[string]interface{}) {
	ErrorResponseWithCode(c, http.StatusConflict, "CONFLICT", message, details)
}

// InternalServerError sends a 500 Internal Server Error
func InternalServerError(c *gin.Context, message string, details map[string]interface{}) {
	ErrorResponseWithCode(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", message, details)
}

// ServiceUnavailableError sends a 503 Service Unavailable error
func ServiceUnavailableError(c *gin.Context, message string) {
	ErrorResponseWithCode(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", message, nil)
}

// ValidationError sends a validation error response
func ValidationError(c *gin.Context, validationErrors map[string]string) {
	details := map[string]interface{}{
		"validation_errors": validationErrors,
	}
	ErrorResponseWithCode(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Validation failed", details)
}

// PaginatedResponse sends a paginated response
func PaginatedResponse(c *gin.Context, data interface{}, pagination PaginationInfo) {
	response := StandardResponse{
		Success:   true,
		Data:      data,
		Timestamp: time.Now(),
		Meta:      buildMetaInfoWithPagination(c, pagination),
	}
	
	c.JSON(http.StatusOK, response)
}

// buildMetaInfo creates meta information for responses
func buildMetaInfo(c *gin.Context) *MetaInfo {
	requestID, exists := c.Get("request_id")
	if !exists {
		requestID = strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	
	version, exists := c.Get("api_version")
	if !exists {
		version = "v1"
	}
	
	responseTime := ""
	if startTime, exists := c.Get("start_time"); exists {
		if start, ok := startTime.(time.Time); ok {
			responseTime = time.Since(start).String()
		}
	}
	
	return &MetaInfo{
		RequestID:    requestID.(string),
		Version:      version.(string),
		ResponseTime: responseTime,
	}
}

// buildMetaInfoWithPagination creates meta information with pagination
func buildMetaInfoWithPagination(c *gin.Context, pagination PaginationInfo) *MetaInfo {
	meta := buildMetaInfo(c)
	
	// Add pagination info to response headers
	c.Header("X-Pagination-Page", fmt.Sprintf("%d", pagination.Page))
	c.Header("X-Pagination-Per-Page", fmt.Sprintf("%d", pagination.PerPage))
	c.Header("X-Pagination-Total", fmt.Sprintf("%d", pagination.Total))
	c.Header("X-Pagination-Total-Pages", fmt.Sprintf("%d", pagination.TotalPages))
	
	return meta
}

// HealthCheckResponse represents health check response format
type HealthCheckResponse struct {
	Status    string                 `json:"status"`
	Service   string                 `json:"service"`
	Version   string                 `json:"version"`
	Timestamp time.Time              `json:"timestamp"`
	Uptime    string                 `json:"uptime"`
	Services  map[string]ServiceStatus `json:"services"`
}

// ServiceStatus represents individual service status
type ServiceStatus struct {
	Status  string                 `json:"status"`
	Healthy bool                   `json:"healthy"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthCheck sends a standardized health check response
func HealthCheck(c *gin.Context, services map[string]ServiceStatus, uptime time.Duration, version string) {
	overallStatus := "healthy"
	for _, service := range services {
		if !service.Healthy {
			overallStatus = "degraded"
			break
		}
	}
	
	response := HealthCheckResponse{
		Status:    overallStatus,
		Service:   "utxo-evm-gateway",
		Version:   version,
		Timestamp: time.Now(),
		Uptime:    uptime.String(),
		Services:  services,
	}
	
	statusCode := http.StatusOK
	if overallStatus != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}
	
	c.JSON(statusCode, response)
}

// WebSocketResponse represents WebSocket message format
type WebSocketResponse struct {
	Type      string      `json:"type"`
	Event     string      `json:"event"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
}

// NewWebSocketResponse creates a new WebSocket response
func NewWebSocketResponse(eventType, event string, data interface{}) *WebSocketResponse {
	return &WebSocketResponse{
		Type:      eventType,
		Event:     event,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// APIInfoResponse represents API information response
type APIInfoResponse struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Endpoints   map[string]string `json:"endpoints"`
	Features    []string          `json:"features"`
}

// APIInfo sends API information response
func APIInfo(c *gin.Context) {
	response := APIInfoResponse{
		Name:        "UTXO-EVM Gateway API",
		Version:     "1.0.0",
		Description: "Cross-chain bridge API for Bitcoin UTXO to Ethereum ERC-20 tokens",
		Endpoints: map[string]string{
			"health":         "/health",
			"bitcoin":        "/bitcoin/*",
			"ethereum":       "/ethereum/*",
			"fusion":         "/fusion/*",
			"proof":          "/proof/*",
			"contracts":      "/contracts/*",
			"websocket":      "/ws",
			"documentation":  "/docs",
		},
		Features: []string{
			"Bitcoin UTXO monitoring",
			"Ethereum smart contract integration", 
			"1inch Fusion+ protocol support",
			"SPV proof generation and verification",
			"Real-time WebSocket updates",
			"Batch operation support",
			"Rate limiting and security",
		},
	}
	
	SuccessResponse(c, response)
}