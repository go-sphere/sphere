package payment

import (
	"context"
	"errors"
)

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

type Provider interface {
	Initialize(config map[string]interface{}) error
	CreatePayment(ctx context.Context, req *Request) (*Response, error)
	QueryPayment(ctx context.Context, transactionID string) (*Response, error)
	RefundPayment(ctx context.Context, transactionID string, amount Amount) error
	ValidateCallback(ctx context.Context, params map[string]interface{}) (*Response, error)
}

type BaseService struct {
	providers map[string]Provider
}

func NewBaseService() *BaseService {
	return &BaseService{
		providers: make(map[string]Provider),
	}
}

func (s *BaseService) RegisterProvider(method string, provider Provider) {
	s.providers[method] = provider
}

func (s *BaseService) GetProvider(method string) (Provider, bool) {
	provider, ok := s.providers[method]
	return provider, ok
}
func (s *BaseService) ProcessPayment(ctx context.Context, req *Request) (*Response, error) {
	provider, ok := s.providers[req.Method]
	if !ok {
		return nil, ErrorPaymentNotFound
	}
	return provider.CreatePayment(ctx, req)
}

func (s *BaseService) QueryPayment(ctx context.Context, method, transactionID string) (*Response, error) {
	provider, ok := s.providers[method]
	if !ok {
		return nil, ErrorPaymentNotFound
	}
	return provider.QueryPayment(ctx, transactionID)
}

func (s *BaseService) RefundPayment(ctx context.Context, method, transactionID string, amount Amount) error {
	provider, ok := s.providers[method]
	if !ok {
		return ErrorPaymentNotFound
	}
	return provider.RefundPayment(ctx, transactionID, amount)
}
