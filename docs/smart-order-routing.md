# Smart Order Routing (智能订单路由)

## 概述

智能订单路由基于 Polymarket 的互补代币机制，提供灵活的订单转换和执行功能。

**重要说明：** 由于 Polymarket CLOB 的设计，订单簿在 YES 和 NO 两个方向是**镜像的**。例如，在 YES 方向挂买单 @ 0.6 会自动在 NO 方向生成对应的卖单 @ 0.4。因此，理论上两个方向的流动性和价格应该是完全对称的。

### 核心功能

- **订单转换** - 在 YES/NO 方向之间灵活转换
- **便利接口** - 统一的 API 处理两个方向
- **订单分析** - 提供订单簿深度和流动性分析工具
- **未来扩展** - 为可能的手续费优化和特殊情况预留接口

## 核心原理

### 互补代币机制

在 Polymarket 二元市场中：
```
YES + NO = 1 USDC
Buy YES @ 0.6 = Sell NO @ 0.4
```

这意味着：
- 买入 YES @ 0.6 的经济效果 = 卖出 NO @ 0.4
- 卖出 YES @ 0.3 的经济效果 = 买入 NO @ 0.7

### 订单簿镜像特性

**关键理解：** CLOB 会自动维护镜像订单簿：
- YES 买单 @ P → NO 卖单 @ (1-P)
- YES 卖单 @ P → NO 买单 @ (1-P)

因此：
- **流动性对称** - 两边的可用流动性理论上相同
- **价格对称** - 不存在价格套利空间
- **执行等价** - 在任一方向执行结果相同

### 实际应用价值

虽然订单簿镜像，但智能路由仍有**关键的实际价值**：

#### 1. 解决持仓同步延迟问题（核心价值）

**问题场景：**
- 用户刚买入 YES 代币
- 出现止盈信号，需要立即平仓
- 但 Polymarket 服务器需要 3-5 秒同步链上余额
- **直接卖 YES 会失败**（服务器还未同步到最新持仓）

**解决方案：**
- 转换为**买 NO**！
- 买 NO 不需要持有 YES，可以**立即执行**
- 经济效果完全相同（持有 YES + NO = 持有 USDC）
- 之后可以通过 **Merge** 操作合并成 USDC

这是订单转换最重要的实际应用场景！

#### 2. 其他价值

- **便利性** - 自动处理订单转换，无需手动计算
- **灵活性** - 可以轻松在两个方向之间切换
- **分析工具** - 提供订单簿分析和可视化

## 功能设计

### 1. 订单簿深度分析

```go
type OrderbookDepth struct {
    // 可用流动性（USDC）
    AvailableLiquidity decimal.Decimal

    // 最优价格
    BestPrice decimal.Decimal

    // 订单数量
    OrderCount int

    // 价格深度（可以接受的价格范围）
    PriceDepth decimal.Decimal

    // 预期滑点（百分比）
    ExpectedSlippage decimal.Decimal
}

// AnalyzeOrderbookDepth 分析订单簿深度
func (c *Client) AnalyzeOrderbookDepth(
    ctx context.Context,
    tokenID string,
    side types.OrderSide,
    targetSize decimal.Decimal,
) (*OrderbookDepth, error)
```

**功能说明：**
- 分析指定方向的订单簿
- 计算可用流动性和预期滑点
- 评估订单成交的可行性

### 2. 双向流动性比较

```go
type SideComparison struct {
    // 原始方向
    OriginalSide struct {
        Side               types.OrderSide
        TokenID            string
        Price              decimal.Decimal
        AvailableLiquidity decimal.Decimal
        ExpectedSlippage   decimal.Decimal
        EstimatedCost      decimal.Decimal  // 预计总成本
    }

    // 互补方向
    ComplementarySide struct {
        Side               types.OrderSide
        TokenID            string
        Price              decimal.Decimal
        AvailableLiquidity decimal.Decimal
        ExpectedSlippage   decimal.Decimal
        EstimatedCost      decimal.Decimal
    }

    // 推荐方向
    RecommendedSide types.OrderSide

    // 预计节省成本（百分比）
    ExpectedSavings decimal.Decimal

    // 推荐理由
    Reason string
}

// CompareOrderSides 比较原始方向和互补方向的流动性
func (c *Client) CompareOrderSides(
    ctx context.Context,
    order *types.UserOrder,
) (*SideComparison, error)
```

**功能说明：**
- 同时分析两个方向的订单簿
- 比较流动性、滑点、成本
- 给出推荐方向和理由

### 3. 智能订单执行

```go
type SmartOrderResult struct {
    // 执行的方向
    ExecutedSide types.OrderSide

    // 执行的 Token ID
    ExecutedTokenID string

    // 执行价格
    ExecutedPrice decimal.Decimal

    // 是否使用了订单转换
    UsedConversion bool

    // 原始订单
    OriginalOrder *types.UserOrder

    // 执行的订单
    ExecutedOrder *types.UserOrder

    // 订单 ID
    OrderID string

    // 节省的成本（百分比）
    CostSavings decimal.Decimal
}

// SmartPlaceOrder 智能下单 - 自动选择最优方向执行
func (c *Client) SmartPlaceOrder(
    ctx context.Context,
    order *types.UserOrder,
) (*SmartOrderResult, error)
```

**功能说明：**
- 分析并选择最优执行方向
- 如果需要，自动转换订单
- 在最优方向下单
- 返回详细的执行结果

### 4. 最优方向查询

```go
type ExecutionSide struct {
    // 推荐的执行方向
    Side types.OrderSide

    // 推荐的 Token ID
    TokenID string

    // 推荐价格
    RecommendedPrice decimal.Decimal

    // 可用流动性
    AvailableLiquidity decimal.Decimal

    // 预期滑点
    ExpectedSlippage decimal.Decimal

    // 是否需要转换订单
    NeedsConversion bool

    // 转换后的订单（如果需要转换）
    ConvertedOrder *types.UserOrder

    // 理由
    Reason string
}

// GetBestExecutionSide 获取最佳执行方向（只分析，不执行）
func (c *Client) GetBestExecutionSide(
    ctx context.Context,
    order *types.UserOrder,
) (*ExecutionSide, error)
```

**功能说明：**
- 只做分析，不实际下单
- 返回推荐的执行方向
- 提供决策依据

## 自动管理功能

### 5. 自动赎回 (Auto-Redeem)

**核心价值：** 当市场结算后，自动将赢的头寸赎回为 USDC。

```go
// AutoRedeemConfig 自动赎回配置
type AutoRedeemConfig struct {
    // 轮询间隔（默认 60 秒）
    PollingInterval time.Duration

    // 是否启用（默认关闭）
    Enabled bool

    // 错误回调（可选）
    OnError func(error)

    // 成功回调（可选）
    OnSuccess func(tokenID string, amount decimal.Decimal)
}

// StartAutoRedeem 启动自动赎回服务
// 定期检查用户持有的头寸，如果对应市场已结算，则自动赎回
func (c *Client) StartAutoRedeem(
    ctx context.Context,
    config *AutoRedeemConfig,
) error

// StopAutoRedeem 停止自动赎回服务
func (c *Client) StopAutoRedeem() error
```

**工作流程：**
1. 定期轮询（根据配置的间隔）
2. 获取用户所有持仓的代币列表
3. 对每个代币，检查对应的市场是否已结算
4. 如果已结算，调用 Redeem() 赎回
5. 记录赎回结果，触发回调

**配置示例：**
```go
// 通过选项配置自动赎回
client, err := polymarket.NewClient(
    ethClient,
    polymarket.WithAutoRedeem(&polymarket.AutoRedeemConfig{
        PollingInterval: 30 * time.Second,  // 每 30 秒检查一次
        Enabled:         true,
        OnSuccess: func(tokenID string, amount decimal.Decimal) {
            log.Printf("成功赎回 %s: %s USDC", tokenID, amount.String())
        },
        OnError: func(err error) {
            log.Printf("赎回失败: %v", err)
        },
    }),
)
```

### 6. 自动合并 (Auto-Merge)

**核心价值：** 当用户同时持有 YES 和 NO 代币时，自动合并为 USDC。

```go
// AutoMergeConfig 自动合并配置
type AutoMergeConfig struct {
    // 轮询间隔（默认 60 秒）
    PollingInterval time.Duration

    // 是否启用（默认关闭）
    Enabled bool

    // 最小合并数量阈值（低于此值不合并，避免频繁小额合并）
    MinMergeAmount decimal.Decimal

    // 错误回调（可选）
    OnError func(error)

    // 成功回调（可选）
    OnSuccess func(conditionID string, amount decimal.Decimal)
}

// StartAutoMerge 启动自动合并服务
// 定期检查用户持有的互补代币对，自动合并为 USDC
func (c *Client) StartAutoMerge(
    ctx context.Context,
    config *AutoMergeConfig,
) error

// StopAutoMerge 停止自动合并服务
func (c *Client) StopAutoMerge() error
```

**工作流程：**
1. 定期轮询（根据配置的间隔）
2. 获取用户所有持仓的代币列表
3. 按 ConditionID 分组，找出同时持有 YES 和 NO 的市场
4. 计算可合并的数量（取两者最小值）
5. 如果数量 >= 最小阈值，调用 Merge() 合并
6. 记录合并结果，触发回调

**配置示例：**
```go
// 通过选项配置自动合并
client, err := polymarket.NewClient(
    ethClient,
    polymarket.WithAutoMerge(&polymarket.AutoMergeConfig{
        PollingInterval: 60 * time.Second,           // 每 60 秒检查一次
        Enabled:         true,
        MinMergeAmount:  decimal.NewFromFloat(1.0),  // 至少 1 USDC 才合并
        OnSuccess: func(conditionID string, amount decimal.Decimal) {
            log.Printf("成功合并 %s: %s USDC", conditionID, amount.String())
        },
        OnError: func(err error) {
            log.Printf("合并失败: %v", err)
        },
    }),
)
```

### 7. 统一自动管理接口

**便利方法：** 同时启用自动赎回和自动合并

```go
// AutoManagementConfig 自动管理配置（包含赎回和合并）
type AutoManagementConfig struct {
    Redeem *AutoRedeemConfig
    Merge  *AutoMergeConfig
}

// StartAutoManagement 启动完整的自动管理服务
func (c *Client) StartAutoManagement(
    ctx context.Context,
    config *AutoManagementConfig,
) error

// StopAutoManagement 停止所有自动管理服务
func (c *Client) StopAutoManagement() error
```

**使用示例：**
```go
// 一次性启动所有自动管理功能
err := client.StartAutoManagement(ctx, &polymarket.AutoManagementConfig{
    Redeem: &polymarket.AutoRedeemConfig{
        PollingInterval: 30 * time.Second,
        Enabled:         true,
    },
    Merge: &polymarket.AutoMergeConfig{
        PollingInterval: 60 * time.Second,
        Enabled:         true,
        MinMergeAmount:  decimal.NewFromFloat(1.0),
    },
})
```

### 实现要点

**1. 轮询机制**
- 使用 `time.Ticker` 实现定期轮询
- 支持通过 context 取消
- 每个服务独立 goroutine

**2. 错误处理**
- 单次失败不应中断整个服务
- 记录错误日志，触发回调
- 可配置重试策略

**3. 性能优化**
- 批量查询用户持仓（一次 API 调用获取所有代币）
- 缓存市场状态信息，减少重复查询
- 并发处理多个赎回/合并操作

**4. 资源管理**
- 提供明确的启动/停止接口
- 确保服务可以优雅关闭
- 避免 goroutine 泄漏

## 使用场景

### 场景 1：订单转换便利工具

**需求：**
用户想买 YES，但发现更习惯在 NO 方向操作。

**解决方案：**
```go
// 原始想法：买 YES @ 0.6
originalOrder := &types.UserOrder{
    TokenID: yesTokenID,
    Price:   decimal.NewFromFloat(0.6),
    Size:    decimal.NewFromInt(100),
    Side:    types.OrderSideBuy,
}

// 转换到 NO 方向
convertedOrder, err := client.ConvertLimitOrder(ctx, originalOrder)
// 结果：Sell NO @ 0.4 (经济效果完全相同)
```

### 场景 2：订单簿分析工具

**需求：**
分析订单簿深度，了解市场流动性情况。

**解决方案：**
```go
// 分析 YES 方向的订单簿
depth, err := client.AnalyzeOrderbookDepth(
    ctx,
    yesTokenID,
    types.OrderSideBuy,
    decimal.NewFromInt(10000),
)

fmt.Printf("可用流动性: %s USDC\n", depth.AvailableLiquidity)
fmt.Printf("预期滑点: %.2f%%\n", depth.ExpectedSlippage.InexactFloat64())
fmt.Printf("订单数量: %d\n", depth.OrderCount)
```

### 场景 3：镜像验证

**需求：**
验证 YES 和 NO 订单簿确实是镜像的（理论上应该是）。

**解决方案：**
```go
// 同时分析两个方向
comparison, err := client.CompareOrderSides(ctx, order)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("YES 方向流动性: %s\n", comparison.OriginalSide.AvailableLiquidity)
fmt.Printf("NO 方向流动性: %s\n", comparison.ComplementarySide.AvailableLiquidity)

// 理论上应该相等或非常接近
if !comparison.OriginalSide.AvailableLiquidity.Equal(
    comparison.ComplementarySide.AvailableLiquidity) {
    fmt.Println("⚠️  警告：镜像不完全对称，可能存在异常")
}
```

### 场景 4：开发和测试

**需求：**
在开发交易策略时，需要灵活地在两个方向之间切换。

**解决方案：**
```go
// 策略开发时可以轻松切换方向
func testStrategy(order *types.UserOrder, useComplementary bool) {
    finalOrder := order

    if useComplementary {
        // 转换到互补方向
        converted, err := client.ConvertLimitOrder(ctx, order)
        if err != nil {
            log.Fatal(err)
        }
        finalOrder = converted
    }

    // 执行订单
    result, err := client.PlaceOrder(ctx, finalOrder)
    // ...
}
```

### 场景 5：快速止盈 - 解决持仓同步延迟

**需求：**
买入 YES 后立即出现止盈信号，但服务器还未同步最新持仓，无法直接卖出。

**问题：**
```go
// 刚买入 YES
buyOrder := &types.UserOrder{
    TokenID: yesTokenID,
    Price:   decimal.NewFromFloat(0.6),
    Size:    decimal.NewFromInt(100),
    Side:    types.OrderSideBuy,
}
result, err := client.PlaceOrder(ctx, buyOrder)

// 立即出现止盈信号，想卖出 YES
sellOrder := &types.UserOrder{
    TokenID: yesTokenID,
    Price:   decimal.NewFromFloat(0.7),
    Size:    decimal.NewFromInt(100),
    Side:    types.OrderSideSell,  // ❌ 失败！服务器还未同步持仓
}
_, err = client.PlaceOrder(ctx, sellOrder)
// Error: insufficient balance
```

**解决方案：**
```go
// 刚买入 YES
buyOrder := &types.UserOrder{
    TokenID: yesTokenID,
    Price:   decimal.NewFromFloat(0.6),
    Size:    decimal.NewFromInt(100),
    Side:    types.OrderSideBuy,
}
result, err := client.PlaceOrder(ctx, buyOrder)

// 出现止盈信号，转换为买 NO！
sellOrder := &types.UserOrder{
    TokenID: yesTokenID,
    Price:   decimal.NewFromFloat(0.7),
    Size:    decimal.NewFromInt(100),
    Side:    types.OrderSideSell,
}

// 转换：Sell YES @ 0.7 → Buy NO @ 0.3
convertedOrder, err := client.ConvertLimitOrder(ctx, sellOrder)
if err != nil {
    log.Fatal(err)
}

// 买 NO 不需要持有 YES，可以立即执行 ✓
_, err = client.PlaceOrder(ctx, convertedOrder)
// 成功！现在持有 100 YES + 100 NO = 100 USDC（经济上已平仓）

// 之后自动合并服务会将 YES + NO 合并为 USDC
```

### 场景 6：自动管理 - 免去手动操作

**需求：**
不想手动赎回已结算的头寸，也不想手动合并互补代币。

**解决方案：**
```go
// 创建客户端时启用自动管理
client, err := polymarket.NewClient(
    ethClient,
    polymarket.WithContractInterfaceOptions(
        polymarketcontracts.WithSafeSigner(safeSigner),
    ),
    // 启用自动赎回
    polymarket.WithAutoRedeem(&polymarket.AutoRedeemConfig{
        PollingInterval: 30 * time.Second,
        Enabled:         true,
        OnSuccess: func(tokenID string, amount decimal.Decimal) {
            log.Printf("✓ 自动赎回成功: %s USDC", amount.String())
        },
    }),
    // 启用自动合并
    polymarket.WithAutoMerge(&polymarket.AutoMergeConfig{
        PollingInterval: 60 * time.Second,
        Enabled:         true,
        MinMergeAmount:  decimal.NewFromFloat(1.0),
        OnSuccess: func(conditionID string, amount decimal.Decimal) {
            log.Printf("✓ 自动合并成功: %s USDC", amount.String())
        },
    }),
)

// 之后就不用管了！
// - 市场结算后，自动赎回赢的头寸
// - 同时持有 YES + NO，自动合并为 USDC
// - 所有操作在后台自动完成
```

## 实现优先级

### Phase 1: 基础分析（优先实现）
- [x] `GetComplementaryTokenID` - 获取互补代币 ID
- [x] `ConvertLimitOrder` - 订单转换
- [x] `ConvertMarketOrder` - 市价单转换
- [ ] `AnalyzeOrderbookDepth` - 订单簿深度分析

### Phase 2: 自动管理（核心功能）
- [ ] `StartAutoRedeem` / `StopAutoRedeem` - 自动赎回服务
- [ ] `StartAutoMerge` / `StopAutoMerge` - 自动合并服务
- [ ] `StartAutoManagement` / `StopAutoManagement` - 统一自动管理
- [ ] `WithAutoRedeem` / `WithAutoMerge` - 选项配置

### Phase 3: 智能对比（可选）
- [ ] `CompareOrderSides` - 双向流动性比较
- [ ] `GetBestExecutionSide` - 获取最优执行方向

### Phase 4: 便利功能（可选）
- [ ] `PlaceOrderWithAutoConversion` - 支持自动转换的下单接口
- [ ] 批量订单转换工具

## 技术细节

### 订单簿数据获取

使用 CLOB API 获取订单簿：
```go
// 获取订单簿
orderbook, err := client.ClobClient().GetOrderbook(ctx, tokenID)

// 分析深度
depth := analyzeDepth(orderbook, side, targetSize)
```

### 滑点计算

```go
func calculateSlippage(
    orderbook *types.Orderbook,
    side types.OrderSide,
    targetSize decimal.Decimal,
) decimal.Decimal {
    // 1. 按价格排序订单
    // 2. 累计匹配订单直到满足目标数量
    // 3. 计算加权平均价格
    // 4. 与最优价格比较，得出滑点
}
```

### 成本计算

```go
type OrderCost struct {
    // 市场价格成本
    MarketCost decimal.Decimal

    // 滑点成本
    SlippageCost decimal.Decimal

    // 交易费用
    TradingFees decimal.Decimal

    // 总成本
    TotalCost decimal.Decimal
}

func calculateOrderCost(
    orderbook *types.Orderbook,
    order *types.UserOrder,
) *OrderCost
```

## 性能优化

1. **订单簿缓存**
   - 缓存最近的订单簿数据（1-5 秒）
   - 减少 API 调用次数

2. **并行查询**
   - 同时查询两个方向的订单簿
   - 使用 goroutine 提高速度

3. **增量更新**
   - 使用 WebSocket 实时更新订单簿
   - 只计算变化的部分

## 风险控制

1. **价格保护**
   - 设置最大可接受滑点
   - 超过阈值时拒绝执行

2. **流动性检查**
   - 确保有足够的流动性
   - 避免大额订单造成市场冲击

3. **超时保护**
   - 设置分析和执行的超时时间
   - 避免在过期数据上决策

## 示例代码

完整示例将在 `examples/04_orderbook_analysis/` 中提供，展示：
- 订单簿深度分析
- YES/NO 两个方向的流动性比较
- 订单转换的实际应用
- 镜像验证工具

## 总结

智能订单路由系统基于 Polymarket 的互补代币机制，提供：

1. **订单转换工具** - 便捷的 YES/NO 方向转换
2. **快速止盈方案** - 解决持仓同步延迟问题（核心价值）
3. **自动管理功能** - 自动赎回和合并，免去手动操作
4. **分析工具** - 订单簿深度和流动性分析
5. **验证工具** - 镜像对称性检查
6. **开发辅助** - 策略开发时的灵活工具

**核心应用场景：**
- 当买入后立即需要止盈，但服务器还未同步持仓时，通过订单转换（卖 YES → 买 NO）实现立即平仓
- 自动赎回已结算市场的赢利头寸
- 自动合并互补代币（YES + NO）为 USDC

**重要提醒：** 由于订单簿镜像特性，理论上两个方向的价格和流动性应该完全对称。本系统的价值主要在于：
1. 解决实际交易中的持仓同步延迟问题
2. 提供自动化的资金管理功能
3. 提供便利性和分析工具
