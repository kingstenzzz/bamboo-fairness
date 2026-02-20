package HyperG

import (
	"github.com/gitferry/bamboo/crypto"
	"github.com/gitferry/bamboo/identity"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestHyperGCompleteWorkflow(t *testing.T) {
	// 创建测试交易
	tx1 := crypto.MakeID("transaction_1")
	tx2 := crypto.MakeID("transaction_2")
	tx3 := crypto.MakeID("transaction_3")
	tx4 := crypto.MakeID("transaction_4")

	// 模拟4个节点的不同排序偏好
	node1Order := NewOrderedList([]crypto.Identifier{tx1, tx2, tx3, tx4}, []int64{10, 20, 30, 40})
	node2Order := NewOrderedList([]crypto.Identifier{tx2, tx1, tx4, tx3}, []int64{15, 25, 35, 45})
	node3Order := NewOrderedList([]crypto.Identifier{tx1, tx3, tx2, tx4}, []int64{5, 15, 25, 35})
	node4Order := NewOrderedList([]crypto.Identifier{tx3, tx1, tx4, tx2}, []int64{12, 22, 32, 42})

	proposals := map[identity.NodeID]*OrderedList{
		"node1": node1Order,
		"node2": node2Order,
		"node3": node3Order,
		"node4": node4Order,
	}

	// 创建HyperG排序器 (4节点，1个Byzantine，gamma=0.8, delta=5)
	sorter := NewHyperGSorter(4, 1, 0.8, 5)

	// 记录开始时间
	startTime := time.Now()

	// 执行完整排序流程
	result, err := sorter.ComputeFairOrder(proposals, 4, 1)

	// 验证执行时间和结果
	executionTime := time.Since(startTime)
	t.Logf("Execution time: %v", executionTime)
	t.Logf("Result length: %d", len(result))

	// 基本验证
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 验证统计信息
	stats := sorter.GetStats()
	assert.NotNil(t, stats)
	assert.True(t, stats.TotalTime > 0)
	assert.True(t, stats.PrefMatrixTime > 0)
	assert.True(t, stats.ClusteringTime > 0)
	assert.True(t, stats.HypergraphTime > 0)
	assert.True(t, stats.ExtractionTime > 0)

	// 验证参数正确性
	assert.True(t, stats.NHyperedges >= 1) // 至少应该有一个超边
	assert.True(t, stats.AvgHyperedgeSize > 0)
	assert.Equal(t, 0.8, stats.Gamma)
	assert.Equal(t, 5, stats.Delta)
	assert.Equal(t, 2, stats.Threshold) // 4*(1-0.8) + 1 + 1 = 2

	t.Logf("HyperG Stats: %+v", stats)
}

func TestHyperGEdgeCases(t *testing.T) {
	// 测试空输入
	sorter := NewHyperGSorter(3, 1, 0.5, 3)
	emptyProposals := make(map[identity.NodeID]*OrderedList)

	result, err := sorter.ComputeFairOrder(emptyProposals, 3, 1)
	assert.NoError(t, err)
	assert.Empty(t, result)

	// 测试单个交易
	tx1 := crypto.MakeID("single_tx")
	singleOrder := NewOrderedList([]crypto.Identifier{tx1}, []int64{100})
	singleProposal := map[identity.NodeID]*OrderedList{
		"node1": singleOrder,
	}

	result, err = sorter.ComputeFairOrder(singleProposal, 3, 1)
	assert.NoError(t, err)
	assert.Len(t, result, 1)

	// 测试极端gamma值
	extremeSorter := NewHyperGSorter(4, 1, 0.0, 5) // gamma=0.0
	result, err = extremeSorter.ComputeFairOrder(singleProposal, 4, 1)
	assert.NoError(t, err)

	extremeSorter2 := NewHyperGSorter(4, 1, 1.0, 5) // gamma=1.0
	result, err = extremeSorter2.ComputeFairOrder(singleProposal, 4, 1)
	assert.NoError(t, err)
}

func TestHyperGPerformanceBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// 创建大量测试数据
	numTxs := 50
	numNodes := 4

	// 生成测试交易
	transactions := make([]crypto.Identifier, numTxs)
	for i := 0; i < numTxs; i++ {
		transactions[i] = crypto.MakeID("tx_" + string(rune(i)))
	}

	// 生成节点提案
	proposals := make(map[identity.NodeID]*OrderedList)
	for n := 0; n < numNodes; n++ {
		// 每个节点都有略微不同的排序
		cmds := make([]crypto.Identifier, numTxs)
		timestamps := make([]int64, numTxs)

		for i := 0; i < numTxs; i++ {
			cmds[i] = transactions[i]
			// 添加一些随机性
			timestamps[i] = int64(i*10 + n)
		}

		proposals[identity.NodeID("node"+string(rune(n)))] = NewOrderedList(cmds, timestamps)
	}

	// 执行性能测试
	sorter := NewHyperGSorter(numNodes, 1, 0.8, 5)

	startTime := time.Now()
	result, err := sorter.ComputeFairOrder(proposals, numNodes, 1)
	executionTime := time.Since(startTime)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	t.Logf("Performance Test Results:")
	t.Logf("  Transactions: %d", numTxs)
	t.Logf("  Nodes: %d", numNodes)
	t.Logf("  Execution Time: %v", executionTime)
	t.Logf("  Result Length: %d", len(result))

	stats := sorter.GetStats()
	t.Logf("  Pref Matrix Time: %v", time.Duration(int64(stats.PrefMatrixTime*1000000000)))
	t.Logf("  Clustering Time: %v", time.Duration(int64(stats.ClusteringTime*1000000000)))
	t.Logf("  Hypergraph Time: %v", time.Duration(int64(stats.HypergraphTime*1000000000)))
	t.Logf("  Extraction Time: %v", time.Duration(int64(stats.ExtractionTime*1000000000)))
}
