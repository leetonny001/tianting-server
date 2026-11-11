package moderation

import (
	"slices"
	"sort"
)

// automaton 是按码点的 Aho-Corasick 自动机（规则 4）。构建之后只读：多个 goroutine 同时匹配不加锁。
// 转移存成压缩的数组（每个节点的子节点按码点排序，二分查找），10 万个词也不需要每个节点一个 map。
type automaton struct {
	first []int32 // 节点 i 的子节点在 edges 里的区间 [first[i], first[i+1])
	label []rune  // 边的码点（按节点分段，段内升序）
	child []int32 // 边指向的节点
	fail  []int32
	hit   []int32 // 节点（含经失败链接到达的后缀）对应的第一个词的下标，-1 表示没有
}

// buildAutomaton 用 patterns（已经标准化的词）建自动机。空串被忽略（空词会匹配一切，管理接口拒绝它）。
func buildAutomaton(patterns []string) *automaton {
	type edge struct {
		r     rune
		child int32
	}
	children := [][]edge{nil}
	terminal := []int32{-1}
	index := map[uint64]int32{} // 构建时的转移表（节点 << 32 | 码点）；根节点可能有上万个子节点，不能线性查找
	for i, p := range patterns {
		if p == "" {
			continue
		}
		node := int32(0)
		for _, r := range p {
			key := uint64(node)<<32 | uint64(uint32(r))
			next, ok := index[key]
			if !ok {
				next = int32(len(children))
				children = append(children, nil)
				terminal = append(terminal, -1)
				children[node] = append(children[node], edge{r, next})
				index[key] = next
			}
			node = next
		}
		if terminal[node] < 0 {
			terminal[node] = int32(i)
		}
	}
	n := len(children)
	a := &automaton{first: make([]int32, n+1), fail: make([]int32, n), hit: make([]int32, n)}
	total := 0
	for i, es := range children {
		slices.SortFunc(es, func(x, y edge) int { return int(x.r) - int(y.r) })
		a.first[i] = int32(total)
		total += len(es)
	}
	a.first[n] = int32(total)
	a.label, a.child = make([]rune, 0, total), make([]int32, 0, total)
	for _, es := range children {
		for _, e := range es {
			a.label = append(a.label, e.r)
			a.child = append(a.child, e.child)
		}
	}
	// 按层（BFS）算失败链接；hit 沿失败链接继承，匹配时每个位置只看一次。
	a.hit[0] = terminal[0]
	queue := make([]int32, 0, n)
	for k := a.first[0]; k < a.first[1]; k++ {
		c := a.child[k]
		a.fail[c] = 0
		a.hit[c] = terminal[c]
		queue = append(queue, c)
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for k := a.first[u]; k < a.first[u+1]; k++ {
			r, v := a.label[k], a.child[k]
			f := a.fail[u]
			for {
				if w := a.next(f, r); w >= 0 {
					a.fail[v] = w
					break
				}
				if f == 0 {
					a.fail[v] = 0
					break
				}
				f = a.fail[f]
			}
			a.hit[v] = terminal[v]
			if a.hit[v] < 0 {
				a.hit[v] = a.hit[a.fail[v]]
			}
			queue = append(queue, v)
		}
	}
	return a
}

// next 是节点 u 经码点 r 的转移，没有返回 -1。
func (a *automaton) next(u int32, r rune) int32 {
	lo, hi := int(a.first[u]), int(a.first[u+1])
	k := lo + sort.Search(hi-lo, func(i int) bool { return a.label[lo+i] >= r })
	if k < hi && a.label[k] == r {
		return a.child[k]
	}
	return -1
}

// find 返回 text 里第一个命中的词的下标（按结束位置最早），没有返回 -1。
func (a *automaton) find(text string) int32 {
	if a == nil || len(a.label) == 0 {
		return -1
	}
	node := int32(0)
	for _, r := range text {
		for {
			if w := a.next(node, r); w >= 0 {
				node = w
				break
			}
			if node == 0 {
				break
			}
			node = a.fail[node]
		}
		if h := a.hit[node]; h >= 0 {
			return h
		}
	}
	return -1
}
