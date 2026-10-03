package attrition

import "sort"

// Result 是一次查询的统计结果。
type Result struct {
	PersonDays   int64    // 人员日数：区间内每天范围内在岗人数之和
	Departures   int      // 离职次数：离职日落在区间内的任职结束事件
	AvgHeadcount float64  // 平均每日在岗人数：人员日数 / 区间自然日数
	Attrition    *float64 // 未年化流失率(%)；人员日数为零时为 nil
}

// Query 统计部门 dept（含当天组织树中的全部后代）在 [start, end) 区间内的流失指标。
func (e *Engine) Query(dept string, start, end Date) Result {
	root, ok := e.deptIdx[dept]
	s, en := start.days(), end.days()
	if !ok || en <= s {
		return Result{}
	}
	n := len(e.deptIDs)

	// 区间起日的父级快照与子女表。
	parent := make([]int, n)
	children := make([][]int, n)
	for i := 0; i < n; i++ {
		parent[i] = e.parentAt(i, s)
		if parent[i] >= 0 {
			children[parent[i]] = append(children[parent[i]], i)
		}
	}

	// 区间起日的成员标记：沿父链向上找 root。
	mem := make([]bool, n)
	for i := 0; i < n; i++ {
		cur := i
		for depth := 0; depth <= n; depth++ {
			if cur == root {
				mem[i] = true
				break
			}
			p := parent[cur]
			if p < 0 {
				break
			}
			cur = p
		}
	}

	// 区间起日的各部门在岗人数与范围内合计。
	hc := make([]int32, n)
	hcIdx := sort.Search(len(e.hcEvents), func(i int) bool { return e.hcEvents[i].day >= s })
	for _, ev := range e.hcEvents[:hcIdx] {
		hc[ev.dept] += ev.delta
	}
	var totalHC int64
	for i := 0; i < n; i++ {
		if mem[i] {
			totalHC += int64(hc[i])
		}
	}

	chgIdx := sort.Search(len(e.changes), func(i int) bool { return e.changes[i].day >= s })

	var personDays int64
	cur := s
	for cur < en {
		next := en
		if hcIdx < len(e.hcEvents) && e.hcEvents[hcIdx].day < next {
			next = e.hcEvents[hcIdx].day
		}
		if chgIdx < len(e.changes) && e.changes[chgIdx].day < next {
			next = e.changes[chgIdx].day
		}
		personDays += totalHC * int64(next-cur)
		cur = next
		if cur >= en {
			break
		}
		for hcIdx < len(e.hcEvents) && e.hcEvents[hcIdx].day == cur {
			ev := e.hcEvents[hcIdx]
			hc[ev.dept] += ev.delta
			if mem[ev.dept] {
				totalHC += int64(ev.delta)
			}
			hcIdx++
		}
		for chgIdx < len(e.changes) && e.changes[chgIdx].day == cur {
			ev := e.changes[chgIdx]
			totalHC += e.applyChange(root, ev.dept, ev.newParent, parent, children, mem, hc)
			chgIdx++
		}
	}

	// 离职事件：按离职日前一日的直接部门与当日组织树判定归属。
	deps := 0
	lo := sort.Search(len(e.departures), func(i int) bool { return e.departures[i].day >= s })
	hi := sort.Search(len(e.departures), func(i int) bool { return e.departures[i].day >= en })
	for _, ev := range e.departures[lo:hi] {
		if e.memberAt(root, ev.dept, ev.day-1) {
			deps++
		}
	}

	days := en - s
	res := Result{
		PersonDays:   personDays,
		Departures:   deps,
		AvgHeadcount: float64(personDays) / float64(days),
	}
	if personDays > 0 {
		rate := float64(deps) / res.AvgHeadcount * 100
		res.Attrition = &rate
	}
	return res
}

// applyChange 应用一次父级变更，返回范围内在岗合计的增量。
func (e *Engine) applyChange(root, p, newParent int, parent []int, children [][]int, mem []bool, hc []int32) int64 {
	old := parent[p]
	if old >= 0 {
		cs := children[old]
		for i, c := range cs {
			if c == p {
				children[old] = append(cs[:i], cs[i+1:]...)
				break
			}
		}
	}
	parent[p] = newParent
	if newParent >= 0 {
		children[newParent] = append(children[newParent], p)
	}
	if p == root {
		// 查询根自身换父不改变其子树集合。
		return 0
	}
	oldMem := mem[p]
	newMem := newParent >= 0 && mem[newParent]
	if oldMem == newMem {
		// 成员身份不变，但仍需保持子树 mem 一致（子树内不含 root 时 mem 与 p 相同，无需更新）。
		return 0
	}
	// 遍历 p 的子树（跳过 root 子树，其成员身份恒为真），更新 mem 并累计人数增量。
	var delta int64
	stack := []int{p}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if x == root {
			continue
		}
		mem[x] = newMem
		if newMem {
			delta += int64(hc[x])
		} else {
			delta -= int64(hc[x])
		}
		stack = append(stack, children[x]...)
	}
	return delta
}
