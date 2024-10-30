package eshop

import (
	"encoding/json"
	"time"
)

type Amount = json.Number

type ProductStatus string // 商品状态

const (
	ProductStatusAvailable ProductStatus = "available" // 可用
	ProductStatusInvalid   ProductStatus = "invalid"   // 无效
)

type Product struct {
	ID          int64          `json:"id"`          // 商品ID
	Name        string         `json:"name"`        // 商品名称
	Description string         `json:"description"` // 商品描述
	Type        string         `json:"type"`        // 商品类型
	Attributes  map[string]any `json:"attributes"`  // 商品属性
	Price       Amount         `json:"price"`       // 商品价格
	CreatedAt   time.Time      `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time      `json:"updated_at"`  // 更新时间
}

type SKUStatus string // SKU状态

const (
	SKUStatusAvailable SKUStatus = "available" // 可用
	SKUStatusReserved  SKUStatus = "reserved"  // 预留
	SKUStatusSold      SKUStatus = "sold"      // 已售
	SKUStatusInvalid   SKUStatus = "invalid"   // 无效
)

type SKU struct {
	ID         int64          `json:"id"`         // 库存单位ID
	ProductID  int64          `json:"product_id"` // 商品ID
	Status     SKUStatus      `json:"status"`     // 状态
	Properties map[string]any `json:"properties"` // SKU具体属性,例如：软件兑换码
	Price      Amount         `json:"price"`      // 价格
	CreatedAt  time.Time      `json:"created_at"` // 创建时间
	UpdatedAt  time.Time      `json:"updated_at"` // 更新时间
}

type OrderStatus string // 订单状态

const (
	OrderStatusPending  OrderStatus = "pending"  // 待处理
	OrderStatusSuccess  OrderStatus = "success"  // 成功
	OrderStatusFailed   OrderStatus = "failed"   // 失败
	OrderStatusCanceled OrderStatus = "canceled" // 取消
	OrderStatusRefunded OrderStatus = "refunded" // 退款
)

type Order struct {
	ID        int64       `json:"id"`         // 订单ID
	UserID    int64       `json:"user_id"`    // 用户ID
	SKUs      []int64     `json:"skus"`       // 商品ID列表
	Status    OrderStatus `json:"status"`     // 订单状态
	Error     string      `json:"error"`      // 错误信息
	Total     Amount      `json:"total"`      // 订单总价
	CreatedAt time.Time   `json:"created_at"` // 创建时间
	UpdatedAt time.Time   `json:"updated_at"` // 更新时间
}

type InventoryAction string // 库存操作类型

const (
	InventoryActionReserve InventoryAction = "reserve" // 预留
	InventoryActionRelease InventoryAction = "release" // 释放预留
	InventoryActionSell    InventoryAction = "sell"    // 售出
	InventoryActionRefund  InventoryAction = "refund"  // 退款/退货
	InventoryActionInvalid InventoryAction = "invalid" // 作废
)

type InventoryRecord struct {
	ID         int64           `json:"id"`          // 记录ID
	SKUID      int64           `json:"sku_id"`      // SKU ID
	ProductID  int64           `json:"product_id"`  // 商品 ID
	OrderID    int64           `json:"order_id"`    // 关联订单 ID
	Action     InventoryAction `json:"action"`      // 操作类型
	FromStatus SKUStatus       `json:"from_status"` // 操作前状态
	ToStatus   SKUStatus       `json:"to_status"`   // 操作后状态
	OperatorID int64           `json:"operator_id"` // 操作人ID
	Remark     string          `json:"remark"`      // 备注信息
	CreatedAt  time.Time       `json:"created_at"`  // 创建时间
}

var (
	// OrderStatusInventoryActionMap 订单状态转换会触发对应的库存操作
	OrderStatusInventoryActionMap = map[OrderStatus]InventoryAction{
		// 下单，未创建支付前锁定库存
		OrderStatusPending: InventoryActionReserve,
		// 支付成功，售出库存
		OrderStatusSuccess: InventoryActionSell,
		// 支付失败，释放库存
		OrderStatusFailed: InventoryActionRelease,
		// 取消订单，释放库存
		OrderStatusCanceled: InventoryActionRelease,
		// 退款，作废库存
		OrderStatusRefunded: InventoryActionRefund,
	}
)

var (
	// InventoryActionTransition 库存状态转换: action -> [from, to]
	InventoryActionTransition = map[InventoryAction][]SKUStatus{
		// 锁定库存, 将库存状态从可用变为预留
		InventoryActionReserve: {SKUStatusAvailable, SKUStatusReserved},
		// 释放预留, 将库存状态从预留变为可用
		InventoryActionRelease: {SKUStatusReserved, SKUStatusAvailable},
		// 售出, 将库存状态从预留变为已售
		InventoryActionSell: {SKUStatusReserved, SKUStatusSold},
		// 退款: 退款后修改库存状态为作废
		InventoryActionRefund: {SKUStatusSold, SKUStatusInvalid},
		// 作废: 将库存状态从可用变为无效
		InventoryActionInvalid: {SKUStatusAvailable, SKUStatusInvalid},
	}
)
