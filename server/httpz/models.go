package httpz

// DataResponse is the success envelope written by WithJson.
type DataResponse[T any] struct {
	Success bool `json:"success" default:"true"`
	Code    int  `json:"code,omitempty" default:"0"`
	Data    T    `json:"data"`
}

// ErrorResponse is the error envelope written by AbortWithJsonError.
// Code is an application code (0 if unclassified). Message is user-facing.
// Error is err.Error() only when DebugMode is on.
type ErrorResponse struct {
	Success bool `json:"success" default:"false"`
	// Code is an application-specific error code and is 0 for unclassified
	// errors. HTTP-level semantics are carried by the response status code,
	// while human-readable details live in Message (and Error in debug mode).
	Code    int    `json:"code" default:"0"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}
