package attrition

import (
	"sort"
)

// 内部统一使用 int 天数（相对 1970-01-01）。openEnd 表示开放区间端点。
const openEnd = int(1) << 30

type pcEntry struct {
	day    int
	parent int // 部门索引，-1 表示根
}

type pcEvent struct {
	day       int
	dept      int
	newParent int
}

type hcEvent struct {
	day   int
	dept  int
	delta int32
}

type depEvent struct {
	day  int
	dept int
}

// Engine 持有装载后的只读历史索引，支持按组织和日期区间查询。
type Engine struct {
	deptIdx       map[string]int
	deptIDs       []string
	initialParent []int
	parentTL      [][]pcEntry // 每个部门的父级变更时间线，按 day 升序
	changes       []pcEvent   // 全局父级变更，按 day 升序
	hcEvents      []hcEvent   // 全局人数增减事件，按 day 升序
	departures    []depEvent  // 全局离职事件，按 day 升序
}

// NewEngine 由完整历史构建查询引擎。构建后历史视为只读。
func NewEngine(h *History) *Engine {
	e := &Engine{
		deptIdx: make(map[string]int, len(h.Departments)),
	}
	for _, d := range h.Departments {
		if _, ok := e.deptIdx[d.ID]; !ok {
			e.deptIdx[d.ID] = len(e.deptIDs)
			e.deptIDs = append(e.deptIDs, d.ID)
		}
	}
	n := len(e.deptIDs)
	e.initialParent = make([]int, n)
	e.parentTL = make([][]pcEntry, n)
	for i := range e.initialParent {
		e.initialParent[i] = -1
	}
	for _, d := range h.Departments {
		i := e.deptIdx[d.ID]
		if d.Parent != "" {
			e.initialParent[i] = e.deptIdx[d.Parent]
		}
	}
	for _, c := range h.ParentChanges {
		di := e.deptIdx[c.Dept]
		p := -1
		if c.NewParent != "" {
			p = e.deptIdx[c.NewParent]
		}
		day := c.Day.days()
		e.parentTL[di] = append(e.parentTL[di], pcEntry{day: day, parent: p})
		e.changes = append(e.changes, pcEvent{day: day, dept: di, newParent: p})
	}
	for i := range e.parentTL {
		tl := e.parentTL[i]
		// 稳定排序：同一部门同一天多次变更时，以输入顺序最后一条为准，
		// 与查询扫描的应用顺序保持一致。
		sort.SliceStable(tl, func(a, b int) bool { return tl[a].day < tl[b].day })
	}
	sort.SliceStable(e.changes, func(a, b int) bool { return e.changes[a].day < e.changes[b].day })

	e.buildHeadcount(h)
	return e
}

// buildHeadcount 由任职与归属区间生成人数增减事件和离职事件。
func (e *Engine) buildHeadcount(h *History) {
	tenures := make(map[string][]TenureSegment, 1024)
	for _, t := range h.Tenures {
		tenures[t.Employee] = append(tenures[t.Employee], t)
	}
	assigns := make(map[string][]Assignment, 1024)
	for _, a := range h.Assignments {
		assigns[a.Employee] = append(assigns[a.Employee], a)
	}
	for emp, ts := range tenures {
		as := assigns[emp]
		sort.Slice(ts, func(i, j int) bool { return ts[i].Start.days() < ts[j].Start.days() })
		sort.Slice(as, func(i, j int) bool { return as[i].Start.days() < as[j].Start.days() })
		// 双指针求任职区间与归属区间的交集，生成人数增减事件。
		i, j := 0, 0
		for i < len(ts) && j < len(as) {
			ts0, te0 := ts[i].Start.days(), segEnd(ts[i].End, ts[i].Open)
			as0, ae0 := as[j].Start.days(), segEnd(as[j].End, as[j].Open)
			s := max(ts0, as0)
			en := min(te0, ae0)
			if s < en {
				di := e.deptIdx[as[j].Dept]
				e.hcEvents = append(e.hcEvents, hcEvent{day: s, dept: di, delta: 1})
				e.hcEvents = append(e.hcEvents, hcEvent{day: en, dept: di, delta: -1})
			}
			if te0 < ae0 {
				i++
			} else {
				j++
			}
		}
		// 离职事件：离职日前一日的直接部门。
		for _, t := range ts {
			if t.Open {
				continue
			}
			prev := t.End.days() - 1
			for _, a := range as {
				if a.Start.days() <= prev && prev < segEnd(a.End, a.Open) {
					e.departures = append(e.departures, depEvent{day: t.End.days(), dept: e.deptIdx[a.Dept]})
					break
				}
			}
		}
	}
	sort.Slice(e.hcEvents, func(a, b int) bool { return e.hcEvents[a].day < e.hcEvents[b].day })
	sort.Slice(e.departures, func(a, b int) bool { return e.departures[a].day < e.departures[b].day })
}

func segEnd(end Date, open bool) int {
	if open {
		return openEnd
	}
	return end.days()
}

// parentAt 返回部门 dept 在日期 day 的父部门索引，-1 表示根。
func (e *Engine) parentAt(dept, day int) int {
	tl := e.parentTL[dept]
	lo, hi := 0, len(tl)
	for lo < hi {
		mid := (lo + hi) / 2
		if tl[mid].day <= day {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == 0 {
		return e.initialParent[dept]
	}
	return tl[lo-1].parent
}

// memberAt 报告部门 x 在日期 day 是否属于 root 的当日子树（含 root 自身）。
func (e *Engine) memberAt(root, x, day int) bool {
	cur := x
	for depth := 0; depth <= len(e.deptIDs); depth++ {
		if cur == root {
			return true
		}
		p := e.parentAt(cur, day)
		if p < 0 {
			return false
		}
		cur = p
	}
	return false
}

// LoadEngine 从本地 JSON 文件装载历史并构建查询引擎。
func LoadEngine(path string) (*Engine, error) {
	h, err := LoadHistory(path)
	if err != nil {
		return nil, err
	}
	return NewEngine(h), nil
}
