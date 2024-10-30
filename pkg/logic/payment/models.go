package payment

import (
	"encoding/json"
	"time"
)

type Amount = json.Number

type Metadata map[string]interface{}

type Request struct {
	Amount      Amount   `json:"amount"`       // 金额
	Currency    string   `json:"currency"`     // 货币
	Method      string   `json:"method"`       // 支付方式
	Description string   `json:"description"`  // 描述
	CallbackURL string   `json:"callback_url"` // 回调URL
	Metadata    Metadata `json:"metadata"`     // 元数据
}

type Response struct {
	TransactionID string   `json:"transaction_id"`        // 外部交易ID
	Status        Status   `json:"status"`                // 支付状态
	Error         string   `json:"error,omitempty"`       // 错误信息
	PaymentURL    string   `json:"payment_url,omitempty"` // 支付URL
	Metadata      Metadata `json:"metadata"`              // 元数据
}

type StatusChange struct {
	FromStatus Status    `json:"from_status"`      // 从状态
	ToStatus   Status    `json:"to_status"`        // 到状态
	ChangedAt  time.Time `json:"changed_at"`       // 变更时间
	Reason     string    `json:"reason,omitempty"` // 变更原因
}

type Record struct {
	Request       *Request        `json:"request"`        // 请求
	Response      *Response       `json:"response"`       // 最新响应
	StatusChanges []*StatusChange `json:"status_changes"` // 状态变更记录
}
