# Payment Logic

## 交易流程

1. 支付服务根据支付方式(Method)选择对应的支付提供商(Provider)
2. 支付提供商验证请求参数的合法性（金额、货币等）
3. 支付提供商创建支付交易，返回支付URL或其他支付凭证
4. 用户完成支付后，支付提供商通过CallbackURL回调通知支付结果，或者用户主动访问支付URL完成支付
5. 支付服务验证回调参数，更新支付状态
6. 支付状态变更记录被保存在StatusChanges中，包含状态变更的时间和原因
7. 如需退款，通过RefundPayment接口向支付提供商发起退款请求

### 状态流转：
- 初始状态：pending（待支付）
- 支付完成：success（支付成功）
- 支付失败：failed（支付失败）
- 交易取消：canceled（已取消）
- 退款完成：refunded（已退款）


## 建议数据库结构

### 1. 支付主表
- 存储支付的基本信息
- 记录当前支付状态
- 保存支付相关的所有核心数据
```sql
CREATE TABLE payments (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    transaction_id VARCHAR(64) UNIQUE NOT NULL,    -- 外部交易ID
    amount DECIMAL(20,2) NOT NULL,                 -- 支付金额
    currency VARCHAR(10) NOT NULL,                 -- 货币类型
    method VARCHAR(32) NOT NULL,                   -- 支付方式
    description TEXT,                              -- 支付描述
    callback_url VARCHAR(255),                     -- 回调URL
    status VARCHAR(20) NOT NULL,                   -- 支付状态
    payment_url VARCHAR(255),                      -- 支付URL
    error_message TEXT,                            -- 错误信息
    metadata JSON,                                 -- 元数据
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

### 2. 状态变更记录表
- 记录支付状态的所有变更历史
- 用于追踪支付状态的完整变更链路
- 便于审计和问题排查
```sql
CREATE TABLE payment_status_changes (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    payment_id BIGINT NOT NULL,                    -- 关联支付ID
    from_status VARCHAR(20) NOT NULL,              -- 原状态
    to_status VARCHAR(20) NOT NULL,                -- 新状态
    reason TEXT,                                   -- 变更原因
    changed_at TIMESTAMP NOT NULL,                 -- 变更时间
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (payment_id) REFERENCES payments(id)
);
```

### 3. 退款记录表
- 记录退款相关的信息
- 支持部分退款的场景
- 记录每笔退款的处理状态
```sql  
CREATE TABLE payment_refunds (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    payment_id BIGINT NOT NULL,                    -- 关联支付ID
    amount DECIMAL(20,2) NOT NULL,                 -- 退款金额
    status VARCHAR(20) NOT NULL,                   -- 退款状态
    reason TEXT,                                   -- 退款原因
    metadata JSON,                                 -- 元数据
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (payment_id) REFERENCES payments(id)
);
```

### 4. 回调记录表
- 记录支付提供商的回调请求
- 用于追踪回调处理状态
- 便于重试和问题排查
```sql  
CREATE TABLE payment_callbacks (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    payment_id BIGINT NOT NULL,                    -- 关联支付ID
    callback_params JSON NOT NULL,                 -- 回调参数
    processed_at TIMESTAMP,                        -- 处理时间
    is_success BOOLEAN NOT NULL,                   -- 处理是否成功
    error_message TEXT,                            -- 错误信息
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (payment_id) REFERENCES payments(id)
);
```
