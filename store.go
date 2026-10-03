package attrition

import (
	"math"
	"sort"
)

// openEnd 表示未离职任职的无限止日。
const openEnd = Date(math.MaxInt32)

// noParent 表示根部门（无父部门）。
const noParent = int32(-1)

// maxDepth 为组织树历史任意一天的最大层数。
const maxDepth = 6

// deptInfo 保存单个部门的预计算索引。
type deptInfo struct {
	id string

	// 父级变更：changeDays 升序，changeParent[i] 自 changeDays[i] 起生效。
	changeDays   []Date
	changeParent []int32

	// 每日在岗人数的断点表：hcDays 升序；
	// hcCount[i] 为 [hcDays[i], hcDays[i+1]) 期间本部门直接在岗人数；
	// hcCum[i] 为 [0, hcDays[i]) 的累计人员日数。
	hcDays  []Date
	hcCount []int32
	hcCum   []int64
}

// Store 为一次装载、只读的历史库。
type Store struct {
	depts   []deptInfo
	byID    map[string]int
	active  []int32 // 有过在岗记录的部门下标
	depDays []Date  // 离职事件，按离职日升序
	depDept []int32 // 离职前一日的直接部门
}

// Build 由内存中的完整历史构建只读 Store。
// 输入保证完整有效，不做业务校验。
func Build(h *History) (*Store, error) {
	s := &Store{
		depts: make([]deptInfo, len(h.Departments)),
		byID:  make(map[string]int, len(h.Departments)),
	}
	for i, d := range h.Departments {
		s.depts[i].id = d.ID
		s.byID[d.ID] = i
	}

	// 父级变更：解析父部门标识并按生效日排序。
	for i, d := range h.Departments {
		info := &s.depts[i]
		pcs := make([]ParentChange, len(d.ParentChanges))
		copy(pcs, d.ParentChanges)
		sort.Slice(pcs, func(a, b int) bool { return pcs[a].Effective < pcs[b].Effective })
		info.changeDays = make([]Date, len(pcs))
		info.changeParent = make([]int32, len(pcs))
		for j, pc := range pcs {
			info.changeDays[j] = pc.Effective
			p := noParent
			if pc.Parent != "" {
				p = int32(s.byID[pc.Parent])
			}
			info.changeParent[j] = p
		}
	}

	// 在岗人数增量与离职事件。
	type delta struct {
		day Date
		d   int32
	}
	deltas := make([][]delta, len(h.Departments))
	for _, e := range h.Employees {
		for _, t := range e.Tenures {
			for _, a := range t.Assignments {
				di := int32(s.byID[a.Dept])
				deltas[di] = append(deltas[di], delta{a.Start, 1})
				if a.End < openEnd {
					deltas[di] = append(deltas[di], delta{a.End, -1})
				}
			}
			if t.End != nil {
				// 离职前一日的直接部门：覆盖最后在岗日的归属段。
				last := *t.End - 1
				for _, a := range t.Assignments {
					if a.Start <= last && last < a.End {
						s.depDays = append(s.depDays, *t.End)
						s.depDept = append(s.depDept, int32(s.byID[a.Dept]))
						break
					}
				}
			}
		}
	}

	// 合并增量，构建每部门断点表。
	for i := range deltas {
		ds := deltas[i]
		if len(ds) == 0 {
			continue
		}
		sort.Slice(ds, func(a, b int) bool { return ds[a].day < ds[b].day })
		info := &s.depts[i]
		var count int32
		var cum int64
		prev := Date(0)
		for _, d := range ds {
			if len(info.hcDays) > 0 && d.day == prev {
				count += d.d
				info.hcCount[len(info.hcCount)-1] = count
				continue
			}
			if len(info.hcDays) > 0 {
				cum += int64(count) * int64(d.day-prev)
			}
			count += d.d
			prev = d.day
			info.hcDays = append(info.hcDays, d.day)
			info.hcCount = append(info.hcCount, count)
			info.hcCum = append(info.hcCum, cum)
		}
		s.active = append(s.active, int32(i))
	}

	// 离职事件按日排序（保持日与部门配对）。
	type departure struct {
		day  Date
		dept int32
	}
	deps := make([]departure, len(s.depDays))
	for i := range deps {
		deps[i] = departure{s.depDays[i], s.depDept[i]}
	}
	sort.Slice(deps, func(a, b int) bool { return deps[a].day < deps[b].day })
	for i, d := range deps {
		s.depDays[i] = d.day
		s.depDept[i] = d.dept
	}
	return s, nil
}
