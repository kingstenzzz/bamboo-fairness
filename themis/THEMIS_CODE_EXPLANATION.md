# Themis 公平交易排序代码解析文档

## 目录
1. [核心思想](#核心思想)
2. [项目结构](#项目结构)
3. [数据结构](#数据结构)
4. [核心算法](#核心算法)
5. [代码实现流程](#代码实现流程)
6. [关键文件说明](#关键文件说明)
7. [与实验的连接](#与实验的连接)

---

## 核心思想

### Themis 的设计目标

Themis（中文：天平）是一个区块链共识系统中的**公平交易排序**（Fair Ordering）解决方案。其核心目标是：

1. **防止主节点操纵**：防止主节点（Leader/Proposer）通过操控交易顺序获利（如抢跑、插队等）
2. **多副本参与排序**：让系统中的所有节点（副本）都参与交易排序过程
3. **全局一致性**：确保所有节点对最终的交易顺序达成共识
4. **性能最优**：在保证公平性的前提下，最小化共识开销

### 设计思路

```
交易来自多个客户端
         ↓
每个副本维护本地交易池，生成本地排序
         ↓
主节点收集所有副本的排序列表（OrderedList）
         ↓
构建交易间的依赖图（TopologyGraph）
         ↓
通过图排序算法（强连通分量 + 哈密尔顿路径）得出公平排序
         ↓
将排序结果打包为区块
         ↓
参与 HotStuff BFT 共识，达成全网一致
```

---

## 项目结构

### Aequitas-hotstuff 目录结构

```
Aequitas-hotstuff/libhotstuff/
├── include/hotstuff/        # 头文件目录
│   ├── aequitas.h           # 公平排序图论算法实现
│   ├── consensus.h          # HotStuff 共识协议头
│   ├── entity.h             # 交易和区块数据结构
│   ├── hotstuff.h           # HotStuff 主类定义
│   └── ...
├── src/                      # 源文件目录
│   ├── aequitas.cpp         # 图排序算法实现（可能被 hotstuff.cpp 包含）
│   ├── consensus.cpp        # HotStuff 共识逻辑
│   ├── entity.cpp           # 交易/区块序列化逻辑
│   ├── hotstuff.cpp         # HotStuff 主逻辑及公平排序集成
│   └── ...
└── ...
```

### 关键文件职责

| 文件 | 职责 |
|------|------|
| `aequitas.h` | 定义 `TopologyGraph` 类，实现图的强连通分量、拓扑排序、哈密尔顿路径等算法 |
| `entity.h` | 定义 `OrderedList` 和 `LeaderProposedOrderedList` 等数据结构 |
| `hotstuff.cpp` | 核心：实现 `aequitas_order()` 函数，集成公平排序与 HotStuff 共识 |
| `consensus.cpp` | HotStuff 共识协议的基础实现 |

---

## 数据结构

### 1. OrderedList（副本的本地排序）

```cpp
struct OrderedList {
    std::vector<uint256_t> cmds;          // 交易哈希列表
    std::vector<uint64_t> timestamps;     // 对应的时间戳
    
    void sort_cmds();  // 按时间戳排序交易
};
```

**说明**：
- 每个副本都维护一个 `OrderedList`，记录本地接收到的交易及其顺序。
- `sort_cmds()` 使用快速排序，按时间戳对交易进行排序，确保时间戳早的交易优先。

### 2. TopologyGraph（交易依赖图）

```cpp
template <int max_number_cmds>
class TopologyGraph {
    // 邻接矩阵：edge[i][j] = 1 表示从交易 i 有边指向交易 j
    std::vector< std::vector<int> > edge;
    
    // 强连通分量相关
    std::vector<int> scc_have[max_number_cmds];  // scc_have[i] 存储第 i 个 SCC 中的节点
    int scc;                                      // 强连通分量个数
    
    // 图的基本信息
    int num_of_edges;      // 边数
    int num_solid;         // 被足够多副本确认的交易数
    int distinct_cmd;      // 该图中最小交易编号
    int distinct_cmd_r;    // 该图中最大交易编号
    
    // 公开方法
    void addedge(int i, int j);                    // 添加有向边 i → j
    void find_scc();                                // 找强连通分量（Tarjan 算法）
    void find_hamilton(int u);                      // 找哈密尔顿路径
    std::vector<uint256_t> finalize(...);          // 排序输出
    bool is_tournament();                           // 检查是否是竞赛图
};
```

**说明**：
- **邻接矩阵 edge**：核心数据结构，`edge[i][j] = 1` 表示交易 i 应该在交易 j 前面。
- **强连通分量**：如果多个交易形成循环依赖，无法单纯按拓扑排序，需要用 Tarjan 算法找 SCC，然后在 SCC 内找哈密尔顿路径。
- **is_tournament()**：检查图是否是完全图（竞赛图），即任意两个节点之间都有边。

### 3. LeaderProposedOrderedList（最终排序结果）

```cpp
class LeaderProposedOrderedList {
    std::vector<uint256_t> cmds;  // 最终排序后的交易列表
};
```

**说明**：这是 `aequitas_order()` 的输出，代表了公平排序后的最终交易顺序。

---

## 核心算法

### 1. 多副本排序汇总

**输入**：
- `proposed_orderlist`：所有副本的本地排序（`OrderedList` 列表）
- `n_f`：系统可容错的拜占庭节点数

**步骤**：
1. 对每个副本的交易列表调用 `sort_cmds()`，按时间戳排序。
2. 统计所有出现过的交易，用唯一的编号 ID 表示。
3. 记录每个交易在多少个副本中出现（`appear` 计数）。
4. 标记"solid"交易：在至少 $n_f + 1$ 个副本中出现的交易被认为是"确认的"。

**代码片段**（来自 hotstuff.cpp, 第 430-500 行）：

```cpp
for (int i = 0; i < n_replica; i++) 
    proposed_orderlist[i].sort_cmds();  // 第一步：按时间戳排序

// 第二、三步：统计交易，映射到唯一编号
for(int pr = 0; pr < n_replica; pr++) {
    for (int i = 0; i < n_cmds; i++) {
        uint256_t cmd_i = proposed_orderlist[pr].cmds[i];
        
        if (HotStuffCore::map_cmd.find(cmd_i) == HotStuffCore::map_cmd.end()) {
            // 新交易，分配 ID
            HotStuffCore::num_of_all_cmds++;
            HotStuffCore::map_cmd[cmd_i].id = HotStuffCore::num_of_all_cmds;
            HotStuffCore::map_cmd[cmd_i].appear = 1;
            HotStuffCore::cmd_content.push_back(cmd_i);
        } else {
            // 已有交易，增加计数
            HotStuffCore::map_cmd[cmd_i].appear++;
        }
        
        // 标记 solid 交易
        if(HotStuffCore::map_cmd[cmd_i].appear >= 2 * n_f + 1)
            HotStuffCore::map_cmd[cmd_i].is_solid = 3;  // 被 2f+1 个副本确认
        else if(HotStuffCore::map_cmd[cmd_i].appear >= n_f + 1)
            HotStuffCore::map_cmd[cmd_i].is_solid = 2;  // 被 f+1 个副本确认
    }
}
```

### 2. 构建图的拓扑约束

**目标**：根据各副本提议的交易顺序，构建一个有向图，边表示"排序约束"。

**逻辑**：
- 如果交易 i 和 j 在多个副本中出现，且至少在 $n_f + 1$ 个副本中 i 都在 j 前面，则添加有向边 `i → j`。

**代码片段**（来自 hotstuff.cpp, 第 670-680 行）：

```cpp
// 为简化实现，这里对所有 distinct 交易添加边
// 实际可用 matric_before 优化边的条件
for(int i = distinct_cmd; i <= distinct_cmd_r; i++) {
    for(int j = i + 1; j <= distinct_cmd_r; j++) {
        G.addedge(i, j);  // 添加边 i → j
    }
}
```

**注意**：代码中有大量注释掉的条件检查，这些是在实际部署时应启用的优化，但在模拟/测试中被简化为添加所有边。

### 3. 图排序算法（finalize）

**算法步骤**：

#### 步骤 1：强连通分量（SCC）分析
使用 **Tarjan 算法**找强连通分量，处理可能的循环依赖。

```cpp
void find_scc() {
    for (int i = distinct_cmd; i <= distinct_cmd_r; i++)
        if (!dfn[i])
            tarjan(i);  // Tarjan DFS
}
```

#### 步骤 2：SCC 间的拓扑排序

```cpp
void topology_sort() {
    // 构建 SCC 间的邻接表
    for (int i = distinct_cmd; i <= distinct_cmd_r; i++) {
        for (int j = distinct_cmd; j <= distinct_cmd_r; j++) {
            if(!edge[i][j]) continue;
            int ii = bel[i], jj = bel[j];
            if(ii != jj) {
                edge_with_scc[ii].push_back(jj);
                ++inDegree[jj];  // 入度统计
            }
        }
    }
}
```

#### 步骤 3：哈密尔顿路径与排序

对每个 SCC，如果包含超过 2 个节点，需要找到哈密尔顿路径（访问每个节点恰好一次的路径）。

```cpp
void find_hamilton(int u) {
    if(scc_have[u].size() <= 2) return;  // 2 个节点以下无需处理
    
    int sum_this_scc = scc_have[u].size();
    
    // 对 SCC 中每个节点尝试找哈密尔顿路径
    for(int j = 0; j < sum_this_scc; j++) {
        scc_have_after_hami.clear();
        int cur = scc_have[u][j];
        
        if(hami(cur, u, sum_this_scc) 
         && edge[scc_have_after_hami[sum_this_scc - 1]][scc_have_after_hami[0]]) {
            // 找到哈密尔顿圈
            for(int i = 0; i < sum_this_scc; i++)
                scc_have[u][i] = scc_have_after_hami[i];
            success = 1;
            break;
        }
    }
}
```

#### 步骤 4：队列拓扑排序

使用入度队列，按照 SCC 的依赖关系，逐个输出各 SCC 中的交易。

```cpp
std::vector<uint256_t> finalize(std::vector<uint256_t> &cmd_content) {
    find_scc();        // 找 SCC
    topology_sort();   // SCC 间拓扑排序
    
    std::vector<uint256_t> final_ordered_cmds;
    std::queue<int> que;
    
    // 初始化：入度为 0 的 SCC 入队
    for (int i = 1; i <= scc; i++) {
        if (inDegree[i] == 0) que.push(i);
    }
    
    // 拓扑排序
    while(!que.empty()) {
        while(!que.empty()) {
            int u = que.front();
            que.pop();
            find_hamilton(u);  // 找 u 对应 SCC 的哈密尔顿路径
            
            // 输出该 SCC 中的交易
            for (int i = 0; i < scc_have[u].size(); i++) {
                final_ordered_cmds.push_back(cmd_content[scc_have[u][i] - 1]);
            }
        }
        
        // 处理下一层
        for (auto succ : edge_with_scc[u]) {
            --inDegree[succ];
            if (inDegree[succ] == 0) que.push(succ);
        }
    }
    
    return final_ordered_cmds;
}
```

---

## 代码实现流程

### 完整执行流程

```
1. HotStuffBase::aequitas_order() 被调用
   │
   ├─ 输入参数：
   │  ├─ G: TopologyGraph（引用，将被填充）
   │  ├─ proposed_orderlist: 各副本的本地排序
   │  └─ n_f: 容错节点数
   │
   ├─ 第 1 阶段：初始化与排序
   │  ├─ 对每个副本的 OrderedList 调用 sort_cmds()
   │  └─ 初始化 map_cmd（交易 ID 映射表）
   │
   ├─ 第 2 阶段：交易统计与标记
   │  ├─ 遍历所有副本的交易
   │  ├─ 统计每个交易出现次数（appear 计数）
   │  ├─ 标记 solid 交易（被足够多副本确认）
   │  └─ 记录交易在 cmd_content 中的索引
   │
   ├─ 第 3 阶段：构建约束图
   │  ├─ 遍历 [distinct_cmd, distinct_cmd_r] 范围的交易
   │  ├─ 根据副本的排序信息添加有向边
   │  └─ distinct_cmd, distinct_cmd_r 标记该图的节点范围
   │
   ├─ 第 4 阶段：图排序（finalize）
   │  ├─ 使用 Tarjan 算法找强连通分量
   │  ├─ 对 SCC 进行拓扑排序
   │  ├─ 对每个 SCC 找哈密尔顿路径
   │  └─ 按拓扑序输出最终排序的交易
   │
   └─ 输出：final_ordered_cmds（排序后的交易列表）
```

### 关键函数调用链

```
HotStuffBase::aequitas_order()
    ├─ for loop: proposed_orderlist[i].sort_cmds()  【排序汇总】
    ├─ map_cmd 填充                                 【交易映射】
    ├─ G.addedge()                                  【边添加】
    └─ G.finalize(cmd_content)                      【图排序】
         ├─ G.find_scc()                             【SCC 分析】
         │   └─ tarjan()                             【Tarjan DFS】
         ├─ G.topology_sort()                        【拓扑排序】
         ├─ for loop: G.find_hamilton()              【哈密尔顿路径】
         │   └─ hami()                               【路径查找】
         └─ return final_ordered_cmds
```

---

## 关键文件说明

### include/hotstuff/aequitas.h

**大小**：332 行

**核心类**：`TopologyGraph<max_number_cmds>`

**关键方法**：

| 方法 | 行号 | 功能 |
|------|------|------|
| `addedge(i, j)` | ~115 | 添加有向边，防止重复 |
| `find_scc()` | ~127 | 启动 Tarjan 算法找强连通分量 |
| `tarjan(u)` | ~47 | Tarjan DFS 实现 |
| `topology_sort()` | ~134 | 构建 SCC 间邻接表并计算入度 |
| `hami(cur, u, sum)` | ~149 | 哈密尔顿路径查找 |
| `find_hamilton(u)` | ~207 | 为 SCC 中的交易找哈密尔顿圈 |
| `is_tournament()` | ~239 | 判断是否是竞赛图 |
| `finalize(cmd_content)` | ~249 | **核心排序输出函数** |

### include/hotstuff/entity.h

**大小**：419 行

**核心结构**：

```cpp
struct OrderedList {
    std::vector<uint256_t> cmds;
    std::vector<uint64_t> timestamps;
    
    void sort_id_according_to_timestamp(...);  // 快速排序实现
    void sort_cmds();                          // 按时间戳排序
};

class LeaderProposedOrderedList {
    std::vector<uint256_t> cmds;               // 最终排序结果
};
```

### src/hotstuff.cpp

**大小**：837 行

**核心函数**：`void HotStuffBase::aequitas_order(...)`

| 部分 | 行号 | 说明 |
|------|------|------|
| 函数声明 | 430-432 | 函数签名 |
| 初始化 | 436-452 | 副本数检查、矩阵初始化 |
| 排序汇总 | 441-442 | `sort_cmds()` 调用 |
| 交易映射 | 453-486 | map_cmd 填充、solid 标记 |
| 图边添加 | 670-680 | `G.addedge()` 调用 |
| 最终输出 | 685-700 | `G.finalize()` 调用、结果返回 |

**关键代码块**：

```cpp
// 第 1 阶段：排序
for (int i = 0; i < n_replica; i++) 
    proposed_orderlist[i].sort_cmds();

// 第 2 阶段：统计
for(int pr = 0; pr < n_replica; pr++) {
    for (int i = 0; i < n_cmds; i++) {
        // ... map_cmd 更新逻辑 ...
    }
}

// 第 3 阶段：建图
for(int i = distinct_cmd; i <= distinct_cmd_r; i++)
    for(int j = i + 1; j <= distinct_cmd_r; j++)
        G.addedge(i, j);

// 第 4 阶段：排序输出
std::vector<uint256_t> cur_ordered_cmds = G.finalize(HotStuffCore::cmd_content);
for(int i = 0; i < cur_ordered_cmds.size(); i++)
    final_ordered_cmds.push_back(cur_ordered_cmds[i]);

hotstuff::LeaderProposedOrderedList final_ordered_vector(final_ordered_cmds);
```

---

## 性能与复杂度分析

### 时间复杂度

| 操作 | 复杂度 | 说明 |
|------|--------|------|
| `sort_cmds()` | O(m log m) | m 为副本的交易数 |
| 交易统计 | O(n × m) | n 个副本，每个有 m 个交易 |
| 图边添加 | O(N²) | N 为不同交易总数 |
| 强连通分量 (Tarjan) | O(N + E) | N 个节点，E 条边 |
| 哈密尔顿路径 | O(N²) | SCC 内的查找 |
| **总体** | **O(N² + n×m)** | 取决于交易数量 |

### 空间复杂度

- **邻接矩阵**：O(N²)，其中 N = max_num_all_txn = 405
- **SCC 存储**：O(N)
- **队列和临时变量**：O(N)

### 约束与优化

**max_num_all_txn = 405**：系统一次共识轮次中最多支持 405 个不同的交易。这是一个硬限制，超过时会导致数组溢出。根据 README，调整区块大小时需要同步修改此值：

```cpp
#define max_num_all_txn 405  // 需要调整为 block_size + 5
```

---

## 与实验的连接

本文档中描述的代码实现通过以下实验进行验证：

### 参考实验指南：[THEMIS_EXPERIMENTS_GUIDE.md](THEMIS_EXPERIMENTS_GUIDE.md)

| 实验 | 验证内容 | 关键结果 |
|------|---------|---------|
| **性能基准测试** (`benchmark_sorting.py`) | `aequitas_order()` 的时间复杂度 | O(N²)，N=1000 时耗时 10.6 秒 |
| **算法对比** (`themisVsHypergraph.py`) | Themis vs Hypergraph 性能 | Hypergraph 加速 10-20 倍 |
| **攻击评估** (`adv_reorder.py`) | 抗主节点反序攻击能力 | 高共识交易对几乎无法被反转 |
| **参数权衡** (`gamma_tradeoffs.py`) | γ 参数对公平性的影响 | 低 γ 提供更强保护，高 γ 提高性能 |

---

## 总结

### Themis 的创新点

1. **多副本参与排序**：不是单独由主节点决定交易顺序，而是所有副本共同参与，通过汇总多个排序提议来形成最终顺序。

2. **图论方法**：利用有向图的拓扑排序和强连通分量分析，优雅地处理了多副本排序中的冲突和循环依赖。

3. **哈密尔顿路径**：对循环依赖的 SCC，通过找哈密尔顿圈保证完全的交易顺序确定性。

4. **与 BFT 共识集成**：将公平排序结果作为区块内容，参与后续的 HotStuff 共识，确保排序结果全网一致且不可篡改。

### 核心优势

- **抗主节点作恶**：即使主节点提议了不公平的排序，最终结果也是所有副本共同影响的。
- **公平性保证**：时间戳早、被多个副本认可的交易更优先。
- **完全确定性**：图排序算法输出唯一的排序结果，无歧义。

### 代码阅读建议

1. 从 `OrderedList::sort_cmds()` 开始，理解本地排序逻辑。
2. 学习 `TopologyGraph` 的数据结构和 `addedge()` 方法。
3. 深入理解 `aequitas_order()` 中的交易统计和图边添加逻辑。
4. 最后研究 `finalize()` 中的 Tarjan + 哈密尔顿路径实现。
5. 参考 [THEMIS_EXPERIMENTS_GUIDE.md](THEMIS_EXPERIMENTS_GUIDE.md) 了解实验验证。

---

**文档版本**：1.0  
**最后更新**：2026 年 2 月 5 日
