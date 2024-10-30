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
库存操作顺序, OrderStatus -> InventoryAction -> SKUStatus
状态变化根据`OrderStatusInventoryActionMap`和`InventoryActionTransition`决定
