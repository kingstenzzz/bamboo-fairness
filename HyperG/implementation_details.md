# Themis与HyperOrder-X协议技术实现详解

## 1. 系统架构概述

本系统实现了两种互补的公平交易排序协议：**Themis**（基于有向图）和**HyperOrder-X**（基于超图）。两种协议共享统一的γ-fairness参数化模型，但采用不同的算法策略来处理拜占庭环境下的交易排序问题。

### 1.1 统一接口设计

两种协议均实现了统一的Sorter接口，支持以下核心方法：
- `sort(gammas, node_orderings)`: 批量处理多个γ值的排序请求
- `sort_single_gamma(gamma, node_orderings)`: 单γ值排序
- `generate_test_data()`: 测试数据生成器
- `build_dependency_graph()`: 依赖图构建（兼容性方法）

这种设计模式使得上层应用可以无缝切换不同的排序算法，同时保持一致的调用接口。

## 2. Themis协议实现细节

### 2.1 核心数据结构

Themis协议的核心数据结构包括：

**Pairwise Preference Matrix**: 使用NumPy数组存储节点对交易对的投票结果，形状为`(n_transactions, n_transactions)`，其中`pref[i][j]`表示有多少节点认为交易i应该排在交易j之前。

**Directed Graph**: 使用NetworkX的`DiGraph`类构建有向图，节点代表交易，边代表排序关系。

**Strongly Connected Components (SCC)**: 使用Tarjan算法检测强连通分量，将相互循环依赖的交易分组。

### 2.2 关键算法实现

#### 2.2.1 动态阈值计算

Themis协议采用优化的阈值公式：
```
threshold = n(1-γ) + γ·f_γ + 1
```
其中`f_γ = floor(n(2γ-1) / (2(γ+1)))`。

Python实现如下：
```python
def compute_f_gamma(n: int, gamma: float) -> int:
    return math.floor((0.5 * n * (2 * gamma - 1)) / (gamma + 1) - 0.01)
```

这种动态阈值设计解决了原始论文中γ较小时阈值过高的问题，使算法在不同γ值下都能有效工作。

#### 2.2.2 图构建算法

图构建过程包含以下步骤：

1. **投票统计**: 对每对交易`(i, j)`，统计有多少节点认为`i < j`
2. **边添加决策**: 
   - 如果`count(i→j) ≥ threshold`且`count(j→i) < threshold`，添加边`i→j`
   - 如果`count(j→i) ≥ threshold`且`count(i→j) < threshold`，添加边`j→i`
   - 如果两者都≥threshold，选择票数更高的方向
   - 如果两者都<threshold，不添加边（发出警告）

3. **SCC分解**: 使用NetworkX的`strongly_connected_components()`函数

4. **拓扑排序**: 对SCC的凝聚图进行拓扑排序

### 2.3 性能优化

- **向量化操作**: 使用NumPy的广播机制批量计算投票结果
- **内存效率**: 避免存储完整的三维数组，而是直接累加计数
- **并行处理**: 支持多γ值并行处理

时间复杂度：`O(n·m² + m²·log m)`，其中n为节点数，m为交易数。

## 3. HyperOrder-X协议实现细节

### 3.1 四阶段流水线架构

HyperOrder-X采用四阶段流水线设计：

1. **Preference Matrix Computation**: 偏好矩阵计算
2. **Canonical Clustering**: 规范聚类
3. **Hypergraph Construction**: 超图构建
4. **Topological Extraction**: 拓扑提取

### 3.2 核心数据结构

**Hyperedges**: 超边表示交易簇，每个超边包含一组高度相关的交易。

**Hypergraph Dependencies**: 超边间的依赖关系，使用三元组`(source_hyperedge, target_hyperedge, weight)`表示。

**Internal Orders**: 每个超边内部的交易排序。

### 3.3 关键算法实现

#### 3.3.1 Canonical Position Embedding

每个交易被分配一个规范位置：
```python
tx_canonical_pos[tx] = count_before
```
其中`count_before`是确定排在当前交易前面的交易数量（投票数≥threshold）。

#### 3.3.2 Geometric Clustering

使用几何距离阈值δ进行聚类：
```python
dist = tx_canonical_pos[tx_curr] - tx_canonical_pos[tx_prev]
if dist < delta:
    current_hyperedge.append(tx_curr)
else:
    hyperedges.append(current_hyperedge)
    current_hyperedge = [tx_curr]
```

δ的动态计算：
```python
def calculate_delta(gen_param, network_param, phi=0.1):
    delta_opt = phi * network_param / gen_param
    return max(1, int(round(delta_opt)))
```

#### 3.3.3 Hypergraph Dependency Resolution

使用1.2倍边际规则建立超边依赖：
```python
pij = pref_matrix[np.ix_(hi, hj)].sum()
pji = pref_matrix[np.ix_(hj, hi)].sum()

if pij > 1.2 * pji:
    deps.append((i, j, pij))
elif pji > 1.2 * pij:
    deps.append((j, i, pji))
```

#### 3.3.4 Internal Ordering within Hyperedges

使用净偏好分数进行内部排序：
```python
net = (sub - sub.T).sum(axis=1)  # Net preference score
order = he[np.argsort(-net)]
```

### 3.4 性能优化

- **Chunked Processing**: 内存分块处理大规模数据
- **Vectorized Operations**: 充分利用NumPy的向量化操作
- **Efficient Clustering**: 线性时间复杂度的聚类算法

时间复杂度：`O(n·m² + m·log m)`，比Themis在大规模数据上更具优势。

## 4. 安全性实现特性

### 4.1 Condorcet Cycle Resistance

- **Themis**: 通过SCC检测识别循环依赖，但可能将无关交易分组到同一SCC
- **HyperOrder-X**: 天然免疫Condorcet循环，因为超图结构防止了任意循环的形成

### 4.2 Front-Running Resistance

- **Themis**: 对front-running攻击具有极强抵抗力（平均前移概率1.57%）
- **HyperOrder-X**: 虽然免疫Condorcet攻击，但对front-running较为脆弱（平均前移概率74.40%）

### 4.3 Byzantine Tolerance

两种协议都支持`f < n/3`的拜占庭容错，通过投票阈值机制过滤恶意节点的影响。

#