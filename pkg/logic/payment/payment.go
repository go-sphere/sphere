package payment

import (
	"context"
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
