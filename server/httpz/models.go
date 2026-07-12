package httpz

// DataResponse represents a successful API response containing typed data.
// It follows a standard structure for consistent API responses across the application.
type DataResponse[T any] struct {
	Success bool `json:"success" default:"true"`
	Code    int  `json:"code,omitempty" default:"0"`
	Data    T    `json:"data"`
}

// ErrorResponse represents an API error response with error details.
// It provides both error and message fields for different levels of error information.
type ErrorResponse struct {
	Success bool `json:"success" default:"false"`
	// Code is an application-specific error code and is 0 for unclassified
	// errors. HTTP-level semantics are carried by the response status code,
	// while human-readable details live in Message (and Error in debug mode).
	Code    int    `json:"code" default:"0"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}
