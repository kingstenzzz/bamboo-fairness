package themis

import (
	"github.com/gitferry/bamboo/crypto"
)

type TopologyGraph struct {
	nodes          []crypto.Identifier
	idMap          map[crypto.Identifier]int
	adj            [][]bool
	dfn            []int
	low            []int
	stack          []int
	inStack        []bool
	timer          int
	sccs           [][]int
	bel            []int
	inDegree       []int
	sccAdj         [][]int
}

func NewTopologyGraph(nodes []crypto.Identifier) *TopologyGraph {
	n := len(nodes)
	idMap := make(map[crypto.Identifier]int)
	for i, node := range nodes {
		idMap[node] = i
	}
	return &TopologyGraph{
		nodes:   nodes,
		idMap:   idMap,
		adj:     make([][]bool, n),
		dfn:     make([]int, n),
		low:     make([]int, n),
		inStack: make([]bool, n),
		bel:     make([]int, n),
	}
}

func (g *TopologyGraph) AddEdge(from, to crypto.Identifier) {
	u, ok1 := g.idMap[from]
	v, ok2 := g.idMap[to]
	if ok1 && ok2 {
		if g.adj[u] == nil {
			g.adj[u] = make([]bool, len(g.nodes))
		}
		g.adj[u][v] = true
	}
}

func (g *TopologyGraph) FindSCC() {
	n := len(g.nodes)
	for i := 0; i < n; i++ {
		if g.dfn[i] == 0 {
			g.tarjan(i)
		}
	}
}

func (g *TopologyGraph) tarjan(u int) {
	g.timer++
	g.dfn[u] = g.timer
	g.low[u] = g.timer
	g.stack = append(g.stack, u)
	g.inStack[u] = true

	for v := 0; v < len(g.nodes); v++ {
		if g.adj[u] == nil || !g.adj[u][v] {
			continue
		}
		if g.dfn[v] == 0 {
			g.tarjan(v)
			if g.low[v] < g.low[u] {
				g.low[u] = g.low[v]
			}
		} else if g.inStack[v] {
			if g.dfn[v] < g.low[u] {
				g.low[u] = g.dfn[v]
			}
		}
	}

	if g.low[u] == g.dfn[u] {
		var scc []int
		for {
			v := g.stack[len(g.stack)-1]
			g.stack = g.stack[:len(g.stack)-1]
			g.inStack[v] = false
			g.bel[v] = len(g.sccs)
			scc = append(scc, v)
			if u == v {
				break
			}
		}
		g.sccs = append(g.sccs, scc)
	}
}

func (g *TopologyGraph) BuildSCCGraph() {
	numSCC := len(g.sccs)
	g.sccAdj = make([][]int, numSCC)
	g.inDegree = make([]int, numSCC)
	for u := 0; u < len(g.nodes); u++ {
		for v := 0; v < len(g.nodes); v++ {
			if g.adj[u] != nil && g.adj[u][v] {
				uSCC := g.bel[u]
				vSCC := g.bel[v]
				if uSCC != vSCC {
					found := false
					for _, neighbor := range g.sccAdj[uSCC] {
						if neighbor == vSCC {
							found = true
							break
						}
					}
					if !found {
						g.sccAdj[uSCC] = append(g.sccAdj[uSCC], vSCC)
						g.inDegree[vSCC]++
					}
				}
			}
		}
	}
}

func (g *TopologyGraph) FindHamiltonPath(sccIdx int) []int {
	sccNodes := g.sccs[sccIdx]
	if len(sccNodes) <= 1 {
		return sccNodes
	}
	
	// 这里使用简单的回溯搜索哈密尔顿路径
	// 在 Themis 中，SCC 通常很小，或者具有竞赛图性质，保证了哈密尔顿路径的存在
	var path []int
	visited := make(map[int]bool)
	
	var backtrack func(curr int) bool
	backtrack = func(curr int) bool {
		path = append(path, curr)
		visited[curr] = true
		if len(path) == len(sccNodes) {
			return true
		}
		for _, next := range sccNodes {
			if !visited[next] && g.adj[curr] != nil && g.adj[curr][next] {
				if backtrack(next) {
					return true
				}
			}
		}
		delete(visited, curr)
		path = path[:len(path)-1]
		return false
	}
	
	for _, start := range sccNodes {
		if backtrack(start) {
			return path
		}
	}
	
	return sccNodes // Fallback
}

func (g *TopologyGraph) Finalize() []crypto.Identifier {
	g.FindSCC()
	g.BuildSCCGraph()
	
	var result []crypto.Identifier
	var queue []int
	for i, deg := range g.inDegree {
		if deg == 0 {
			queue = append(queue, i)
		}
	}
	
	for len(queue) > 0 {
		uSCC := queue[0]
		queue = queue[1:]
		
		// 对每个 SCC 内部寻找哈密尔顿路径
		path := g.FindHamiltonPath(uSCC)
		for _, nodeIdx := range path {
			result = append(result, g.nodes[nodeIdx])
		}
		
		for _, vSCC := range g.sccAdj[uSCC] {
			g.inDegree[vSCC]--
			if g.inDegree[vSCC] == 0 {
				queue = append(queue, vSCC)
			}
		}
	}
	
	return result
}
