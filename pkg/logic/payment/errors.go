package payment

import "errors"

var (
	ErrorPaymentNotFound        = errors.New("payment not found")
	ErrorInvalidAmount          = errors.New("invalid payment amount")
	ErrorInvalidCurrency        = errors.New("invalid currency")
	ErrorProviderNotInitialized = errors.New("payment provider not initialized")

	ErrorCreateUnsupported = errors.New("create payment not supported")
	ErrorQueryUnsupported  = errors.New("query payment not supported")
	ErrorRefundUnsupported = errors.New("refund payment not supported")

	ErrorInvalidCallbackURL   = errors.New("invalid callback URL")
	ErrorInvalidMetadata      = errors.New("invalid metadata")
	ErrorDuplicateTransaction = errors.New("duplicate transaction ID")
	ErrorRefundExceedsPayment = errors.New("refund amount exceeds payment amount")
	ErrorInvalidPaymentStatus = errors.New("invalid payment status for operation")
)
