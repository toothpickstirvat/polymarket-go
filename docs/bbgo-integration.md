# BBGO 集成可行性分析

## 概述

本文档分析了在 Polymarket Go 客户端暂时无法实现 KLine（K线/蜡烛图）功能的情况下，集成 BBGO 交易框架的可行性。

**结论：可以集成，但有限制**

## BBGO 简介

[BBGO](https://github.com/c9s/bbgo) 是一个用 Go 语言编写的现代加密货币交易机器人框架，特点包括：
- 支持多个交易所（Binance、MAX、FTX、OKEx 等）
- 内置技术指标（ATR、Bollinger Bands、RSI、MACD 等）
- 支持实时交易和历史回测
- 提供 WebSocket 实时数据流
- 灵活的策略开发框架

## 核心发现：QueryKLines 是可选的

根据 [BBGO 官方文档](https://github.com/c9s/bbgo/blob/main/doc/development/adding-new-exchange.md)，实现一个新交易所接口的**最小必需方法**不包括 `QueryKLines`。

### 必需接口（Minimum Requirements for Spot Trading）

```go
type Exchange interface {
    // 市场信息
    Name() types.ExchangeName
    QueryMarkets(ctx context.Context) (types.MarketMap, error)

    // 行情数据
    QueryTickers(ctx context.Context, symbols ...string) (map[string]types.Ticker, error)

    // 订单管理
    QueryOpenOrders(ctx context.Context, symbol string) ([]types.Order, error)
    SubmitOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error)
    CancelOrders(ctx context.Context, orders ...types.Order) error

    // 数据流
    NewStream() types.Stream
}
```

### 可选接口

```go
// QueryKLines - 仅回测功能需要
QueryKLines(ctx context.Context, symbol string, interval types.Interval,
    options types.KLineQueryOptions) ([]types.KLine, error)

// 其他可选方法
QueryAccountBalances(ctx context.Context) (types.BalanceMap, error)
QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error)
QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) ([]types.Order, error)
IsSupportedInterval(interval types.Interval) bool
```

**关键点：** `QueryKLines` 在文档中标记为 "optional, required by backtesting"，意味着：
- ✅ 不是核心必需接口
- ⚠️ 只有在需要**回测功能**时才必需
- ✅ 可以在未来 Polymarket 提供历史数据 API 时再实现

## Polymarket 现有能力映射

### 已具备的能力

| BBGO 必需接口 | Polymarket 对应实现 | 状态 |
|--------------|-------------------|------|
| `QueryMarkets` | `GammaClient.GetMarkets()`, `GetMarket()` | ✅ 已实现 |
| `QueryTickers` | `DataClient.GetMarket()`, `RealtimeDataClient` | ✅ 已实现 |
| `QueryOpenOrders` | `ClobClient.GetOpenOrders()` | ✅ 已实现 |
| `SubmitOrder` | `ClobClient.PostOrder()` | ✅ 已实现 |
| `CancelOrders` | `ClobClient.CancelOrder()`, `CancelOrders()` | ✅ 已实现 |
| `NewStream` | `RealtimeDataClient` (WebSocket) | ✅ 已实现 |

### 可选但已具备的能力

| BBGO 可选接口 | Polymarket 对应实现 | 状态 |
|--------------|-------------------|------|
| `QueryAccountBalances` | `ClobClient.GetBalanceAllowance()` | ✅ 已实现 |
| `QueryTrades` | `DataClient.GetTrades()` (仅最近交易) | ⚠️ 部分实现 |
| `QueryClosedOrders` | `ClobClient.GetOrderByID()`, `GetOrder()` | ✅ 已实现 |

### 无法实现的功能

| BBGO 接口 | 原因 | 影响 |
|----------|------|------|
| `QueryKLines` | Polymarket API 不支持历史交易数据查询 | ❌ 无法回测 |

## Polymarket API 限制说明

### KLine 功能被禁用的原因

1. **CLOB API (`GetTrades`)**
   - 限制：只能查询用户自己的交易，无法查询市场全部交易
   - 影响：无法获取市场级别的交易数据

2. **Data API (`GetTrades`)**
   - 限制：支持市场级别的交易查询，但**没有时间范围参数**
   - 影响：只能获取最近的交易数据（通常是最近几小时/几天）
   - 最大查询量：10,000 条交易记录

### 测试结果

在测试中，对一个高交易量的市场（$1.8M 24小时交易量）查询 30 天的数据，仅返回了当天的 3 条交易记录，证明了 API 的时间范围限制。

详见 `kline.go` 文件头部的详细说明。

## 集成限制与影响

### ❌ 无法使用的功能

1. **策略回测 (Backtesting)**
   - BBGO 的回测引擎是基于 KLine 的
   - 没有历史 K 线数据，无法进行回测
   - 命令如 `bbgo backtest --sync --sync-from 2020-01-01` 将无法工作

2. **基于历史数据的技术指标**
   - ATR (Average True Range) - 需要历史 K 线
   - Bollinger Bands - 需要历史价格数据
   - RSI (Relative Strength Index) - 需要历史价格序列
   - MACD - 需要历史价格序列
   - 移动平均线 (MA, EMA) - 需要历史数据

3. **历史数据分析**
   - 无法分析历史波动率
   - 无法进行历史 PnL 计算（除非从交易记录重建）
   - 无法做长期趋势分析

### ✅ 仍然可用的功能

1. **实时交易执行**
   - 订单提交、取消、查询
   - 实时订单簿访问
   - 实时价格监控

2. **基于实时数据的策略**
   - **Order Book Based Strategies** - 基于订单簿深度的策略
   - **Market Making** - 做市策略（使用当前买卖价差）
   - **Grid Trading** - 网格交易（基于当前价格区间）
   - **Arbitrage** - 套利策略（实时价差监控）
   - **Event-Driven Trading** - 事件驱动交易（响应 Polymarket 市场事件）

3. **实时风险管理**
   - 持仓管理
   - 实时 PnL 跟踪
   - 仓位调整
   - 止损/止盈（基于实时价格）

4. **数据流和监控**
   - WebSocket 实时数据推送
   - 市场事件监听
   - 订单状态更新

## 适用的交易策略

对于 Polymarket 做市和交易场景，以下策略**不依赖历史 KLine**，可以直接使用：

### 1. Pure Market Making（纯做市策略）

```go
// 基于实时订单簿深度和库存调整报价
type MarketMakingStrategy struct {
    targetSpread      decimal.Decimal  // 目标价差
    inventorySkew     decimal.Decimal  // 库存偏差调整
    maxPosition       decimal.Decimal  // 最大持仓
}

// 只需要实时数据：
// - 当前订单簿深度 (OrderBook)
// - 当前持仓 (QueryAccountBalances)
// - 最新成交价 (QueryTickers)
```

### 2. Spread Monitoring & Capture（价差监控与捕获）

```go
// 已实现的 spread scanner 可以直接集成
type SpreadStrategy struct {
    minSpread     decimal.Decimal  // 最小可获利价差
    maxOrderSize  decimal.Decimal  // 单笔最大订单
}

// 监控互补代币之间的价差
// 当价差超过阈值时执行套利
```

### 3. Inventory Management（库存管理）

```go
// 基于当前持仓和目标仓位进行调整
type InventoryStrategy struct {
    targetRatio    decimal.Decimal  // 目标 YES/NO 持仓比例
    rebalanceThreshold decimal.Decimal  // 触发再平衡的偏差阈值
}

// 使用实时数据：
// - 当前持仓 (QueryAccountBalances)
// - 当前价格 (QueryTickers)
// - 计算偏差并调整
```

### 4. Event-Driven Trading（事件驱动交易）

```go
// 响应 Polymarket 特定事件
type EventDrivenStrategy struct {
    eventThreshold  decimal.Decimal  // 事件触发阈值
    quickExitTime   time.Duration    // 快速退出时间窗口
}

// 监控市场事件：
// - 交易量突然增加
// - 价格快速变动
// - 订单簿失衡
```

### 5. Grid Trading（网格交易）

```go
// 在价格区间内设置买卖网格
type GridStrategy struct {
    gridLevels    int              // 网格层数
    gridSpacing   decimal.Decimal  // 网格间距
    basePrice     decimal.Decimal  // 基准价格（从当前价格开始）
}

// 不需要历史数据，基于当前价格创建网格
```

## 集成实现方案

### Phase 1: 基础集成（无 KLine）- 当前可行

```go
package bbgoexchange

import (
    "context"
    "github.com/c9s/bbgo/pkg/types"
    "github.com/ivanzzeth/polymarket-go"
)

// PolymarketExchange 实现 BBGO 的 Exchange 接口
type PolymarketExchange struct {
    client *polymarket.Client
}

// 必需方法实现
func (e *PolymarketExchange) Name() types.ExchangeName {
    return types.ExchangeName("polymarket")
}

func (e *PolymarketExchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
    // 使用 GammaClient.GetMarkets()
    markets, err := e.client.GammaClient().GetMarkets(ctx, nil)
    if err != nil {
        return nil, err
    }

    // 转换为 BBGO MarketMap
    return convertToMarketMap(markets), nil
}

func (e *PolymarketExchange) QueryTickers(ctx context.Context, symbols ...string) (map[string]types.Ticker, error) {
    // 使用 DataClient.GetMarket() 或 RealtimeDataClient
    // 实现略...
}

func (e *PolymarketExchange) QueryOpenOrders(ctx context.Context, symbol string) ([]types.Order, error) {
    // 使用 ClobClient.GetOpenOrders()
    // 实现略...
}

func (e *PolymarketExchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
    // 使用 ClobClient.PostOrder()
    // 实现略...
}

func (e *PolymarketExchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
    // 使用 ClobClient.CancelOrders()
    // 实现略...
}

func (e *PolymarketExchange) NewStream() types.Stream {
    // 使用 RealtimeDataClient (WebSocket)
    return NewPolymarketStream(e.client.RealtimeDataClient())
}

// QueryKLines - 暂时返回 not supported error
func (e *PolymarketExchange) QueryKLines(ctx context.Context, symbol string,
    interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
    return nil, fmt.Errorf("KLines not supported: Polymarket API does not provide historical trade data with time range parameters. See docs/bbgo-integration.md for details")
}

// 可选方法实现
func (e *PolymarketExchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
    // 使用 ClobClient.GetBalanceAllowance()
    // 实现略...
}

func (e *PolymarketExchange) QueryTrades(ctx context.Context, symbol string,
    options *types.TradeQueryOptions) ([]types.Trade, error) {
    // 使用 DataClient.GetTrades() - 仅返回最近的交易
    // 注意：不支持时间范围查询
    // 实现略...
}
```

### Phase 2: 未来增强（当 Polymarket 支持历史数据时）

```go
// 当以下任一条件满足时，可以启用 KLine 功能：
// 1. Polymarket 提供带时间范围参数的历史交易数据 API
// 2. 实现基于链上事件的历史数据扫描和存储

func (e *PolymarketExchange) QueryKLines(ctx context.Context, symbol string,
    interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
    // 将 kline.go 中的 getKLines 改为 GetKLines（导出）
    // 使用 client.GetKLines()
    tokenID := extractTokenID(symbol)
    klines, err := e.client.GetKLines(ctx, tokenID,
        intervalToDuration(interval),
        options.StartTime,
        options.EndTime)

    if err != nil {
        return nil, err
    }

    // 转换为 BBGO KLine 格式
    return convertToTypes(klines), nil
}
```

## 实现建议

### 1. 项目结构

```
polymarket-go/
├── bbgo/                          # BBGO 集成包
│   ├── exchange.go                # Exchange 接口实现
│   ├── stream.go                  # Stream 实现（WebSocket）
│   ├── converter.go               # 类型转换工具
│   └── strategies/                # 示例策略
│       ├── market_making.go
│       ├── grid.go
│       └── spread_capture.go
├── examples/
│   └── bbgo-integration/
│       └── main.go                # BBGO 集成示例
└── docs/
    └── bbgo-integration.md        # 本文档
```

### 2. 开发步骤

1. **实现核心 Exchange 接口**（必需方法）
   - 从简单的方法开始：`Name()`, `PlatformFeeCurrency()`
   - 实现查询方法：`QueryMarkets`, `QueryTickers`, `QueryOpenOrders`
   - 实现交易方法：`SubmitOrder`, `CancelOrders`
   - 实现流方法：`NewStream`

2. **实现 Stream（WebSocket）**
   - 包装 `RealtimeDataClient`
   - 实现订单簿更新推送
   - 实现交易数据推送
   - 实现用户数据推送（订单、余额）

3. **类型转换**
   - Polymarket types → BBGO types
   - 市场格式转换
   - 订单格式转换
   - 时间戳格式转换

4. **错误处理**
   - API 限流处理
   - 网络错误重试
   - WebSocket 重连逻辑

5. **测试**
   - 单元测试（类型转换、错误处理）
   - 集成测试（与 Polymarket API 交互）
   - 策略测试（简单的做市策略）

### 3. 注意事项

**Polymarket 特殊性：**
- 二元市场结构（YES/NO 代币对）
- 互补代币机制
- NegRisk 市场类型
- 条件代币框架（CTF）

**BBGO 适配要点：**
- Symbol 格式：建议使用 `MARKET_SLUG-YES` / `MARKET_SLUG-NO`
- 价格范围：Polymarket 价格在 [0, 1] 区间
- 最小订单量：注意 Polymarket 的最小交易金额要求
- 手续费模型：Polymarket 无交易手续费（仅有 gas 费）

## 参考资料

### BBGO 官方文档
- [BBGO GitHub Repository](https://github.com/c9s/bbgo)
- [Adding New Exchange Guide](https://github.com/c9s/bbgo/blob/main/doc/development/adding-new-exchange.md)
- [Developing Strategy Guide](https://github.com/c9s/bbgo/blob/main/doc/topics/developing-strategy.md)
- [BBGO Official Website](https://bbgo.finance/)

### Polymarket Go 客户端文档
- [KLine 功能说明](../kline.go) - KLine 被禁用的详细原因
- [Order Converter](../order_converter.go) - 互补订单转换机制
- [Auto Management](../auto_management.go) - 自动赎回和合并功能

### 相关决策
- [DECISION.md](./DECISION.md) - 架构决策记录
- [Arbitrage Scanner](./arbitrage-scanner.md) - 套利扫描器设计
- [Smart Order Routing](./smart-order-routing.md) - 智能订单路由

## 总结

**可以立即开始 BBGO 集成，专注于实时交易功能。**

### 优势
✅ BBGO 不强制要求 KLine 功能
✅ Polymarket 已具备所有必需的实时交易接口
✅ 可以实现多种不依赖历史数据的交易策略
✅ 为未来的 KLine 功能预留了扩展空间

### 限制
⚠️ 无法使用回测功能
⚠️ 无法使用基于历史数据的技术指标
⚠️ 历史数据分析受限

### 未来路线
当以下任一条件满足时，可以启用完整的 KLine 功能：
1. Polymarket 提供历史交易数据 API（支持时间范围查询）
2. 实现基于以太坊链上事件的历史数据扫描和存储系统

---

**最后更新：** 2025-11-25
**状态：** 可行 - Phase 1 (无 KLine) 可立即实施
