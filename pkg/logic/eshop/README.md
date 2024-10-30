# Eshop Logic

这是一个数字商品销售系统示例, 所以每个SKU都是唯一的, 每个SKU都对应一个Product, SKU没有库存数量, 只有状态
具体SKU的属性存储在SKU的Properties中, 例如: 软件兑换码的code

### `SKU` 状态
`available` -> `reserved` -> `sold`/`available` -> `invalid`

### `Order` 状态
`pending` -> `success`/`failed` -> `refunded`/`canceled`

### 库存查询
根据`product_id`查询`Product`下的所有状态为SKUStatusAvailable的SKU的数量

### 库存操作

#### `InventoryActionReserve`
1. 根据`product_id`查询`Product`下的所有状态为`SKUStatusAvailable`的`SKU`
2. 扣减库存，修改`SKU`的`Status`为`SKUStatusReserved`
3. 记录库存操作记录`InventoryRecord`状态为`InventoryActionReserve`
4. 创建`Order`并关联`SKU`
5. 创建`Payment`, 关联`Order`

### `InventoryActionRelease`
1. 支付失败或者创建支付失败后, 修改`Order`状态为`OrderStatusFailed`, 并更新`Error`
2. 记录库存操作记录`InventoryRecord`状态为`InventoryActionRelease`
3. 修改对应`SKU`的`Status`为`SKUStatusAvailable`,让它可以重新售卖

#### `InventoryActionSell`
1. 支付成功后, 修改`Order`状态为`OrderStatusSuccess`
2. 记录库存操作记录`InventoryRecord`状态为`InventoryActionSell`
3. 修改对应`SKU`的`Status`为`SKUStatusSold`, 用户可以查看售卖商品的`properties`

#### `InventoryActionRefund`
1. 退款后, 修改`Order`状态为`OrderStatusRefunded`
2. 记录库存操作记录`InventoryRecord`状态为`InventoryActionRefund`
3. 修改对应`SKU`的`Status`为`SKUStatusInvalid`, 退款商品不可重新售卖

#### `InventoryActionInvalid`
1. 后台作废, 修改对应`SKU`的`Status`为`SKUStatusInvalid`