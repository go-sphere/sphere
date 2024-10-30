package payment

import (
	"encoding/json"
	"time"
)

type Amount = json.Number

type Metadata map[string]interface{}

type Request struct {
	ID          int64    `json:"id"`           // 内部交易ID
	OrderID     int64    `json:"order_id"`     // 关联订单ID
	Amount      Amount   `json:"amount"`       // 金额
	Currency    string   `json:"currency"`     // 货币
	Method      string   `json:"method"`       // 支付方式
	Description string   `json:"description"`  // 描述
	CallbackURL string   `json:"callback_url"` // 回调URL
	Metadata    Metadata `json:"metadata"`     // 元数据
}

type Status string

const (
	StatusPending  Status = "pending"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
	StatusRefunded Status = "refunded"
)

var (
	// 客户端状态转换权限: [from] -> [to]
	DefaultStatusTransitionPermission = map[Status]map[Status]struct{}{
		StatusPending: {
			StatusSuccess:  {}, // 支付成功
			StatusFailed:   {}, // 支付失败
			StatusCanceled: {}, // 支付取消
		},
		StatusSuccess: {
			StatusRefunded: {}, // 支付成功后退款
		},
		StatusFailed:   {}, // 支付失败不可转换
		StatusCanceled: {}, // 支付取消不可转换
		StatusRefunded: {}, // 退款不可转换
	}
	// 管理员状态转换权限,可以将支付失败的订单重置为待支付
	RecoveryStatusTransitionPermission = map[Status]map[Status]struct{}{
		StatusPending: {
			StatusSuccess:  {},
			StatusFailed:   {},
			StatusCanceled: {},
		},
		StatusSuccess: {
			StatusRefunded: {},
		},
		StatusFailed: {
			StatusPending: {}, // 支付失败后重置订单
		},
		StatusCanceled: {
			StatusPending: {}, // 支付取消后重置订单
		},
		StatusRefunded: {
			StatusPending: {}, // 退款后重置订单
		},
	}
)

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
