# Themis 公平排序整合方案 (Go 模块化)

## 1. 概述
本方案旨在将 Themis 公平交易排序算法整合进现有的 Bamboo/Phalanx 共识系统中。为了保证系统的可维护性和 Go 代码的纯净性，我们将 Themis 的核心图论排序算法从 C++ 重写为 Go，并以模块化的方式集成到 `replica` 和 `safety` 流程中。

## 2. 核心模块设计 (`themis` package)

新创建 `themis` 模块，负责核心公平排序逻辑。

### 2.1 数据结构
- **`OrderedList`**: 副本提议的本地排序列表，包含交易哈希及其对应的时间戳。
- **`TopologyGraph`**: 核心算法载体。
    - `Edges`: 邻接矩阵/邻接表。
    - `TarjanSCC()`: 寻找强连通分量。
    - `HamiltonPath()`: 在 SCC 内部解决循环依赖。
    - `Finalize()`: 输出最终确定的公平排序。

### 2.2 接口定义
```go
type FairOrderer interface {
    // ProposeLocalOrder 生成本地副本的交易排序提议
    ProposeLocalOrder(txs []*message.Transaction) *OrderedList
    
    // ComputeFairOrder 主节点汇总各副本提议并计算全局公平排序
    ComputeFairOrder(proposals map[identity.NodeID]*OrderedList, n int, f int) ([]*message.Transaction, error)
}
```

## 3. 算法移植细节 (C++ to Go)
- **Tarjan 算法**: 保持 O(N+E) 复杂度，用于识别冲突环路（SCC）。
- **哈密尔顿路径**: 在 SCC 内部进行路径搜索。由于 Themis 限制了 SCC 的规模或通过竞赛图性质简化，Go 实现将采用回溯或特定启发式算法确保性能。
- **并发安全**: 内部计算过程采用函数式风格，不持有全局状态，确保在多视图并发环境下的安全。

## 4. 流程集成方案

### 4.1 副本端 (Follower)
1. **记录时间戳**: 当副本接收到客户端交易时，在 `message.Transaction.Timestamp` 中记录本地到达时间。
2. **发送提议**: 在 `NewView` 消息或新增的 `OrderPropose` 消息中，将本地排序的 `OrderedList` 发送给下一任主节点。

### 4.2 主节点端 (Leader)
1. **收集提议**: 主节点在准备提议区块前，等待并收集至少 $2f+1$ (或 $f+1$ 根据具体安全需求) 个副本的 `OrderedList`。
2. **计算排序**: 调用 `themis.ComputeFairOrder()`。
    - 统计所有出现的交易。
    - 根据 "超过 $f+1$ 个副本认为 A 在 B 之前" 等规则构建有向图。
    - 运行 `Finalize()` 得到排序。
3. **生成区块**: 使用公平排序后的交易序列调用 `Safety.MakeProposal()` 生成区块。

### 4.3 验证阶段
- 副本在 `ProcessBlock` 时，可以根据区块中附带的证明信息（或者利用已知的其他副本提议）重新运行排序算法验证公平性。

## 5. 目录结构变更
```text
themis/
├── libhotstuff/      # 原始 C++ 代码 (保留参考)
├── graph.go          # Go 实现的图论算法 (Tarjan, SCC, Hamilton)
├── orderer.go        # FairOrderer 接口与顶层逻辑
└── types.go          # OrderedList 等基础类型
```

## 6. 后续步骤
1. 实现 `themis` 包下的核心图算法。
2. 扩展 `blockchain.Block` 和消息协议以支持 `OrderedList` 传递。
3. 在 `replica/replica.go` 的 `proposeBlock` 逻辑中接入排序计算。
