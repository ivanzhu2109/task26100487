package attrition

import (
	"math"
	"math/rand"
	"sort"
	"testing"
)

// bruteQuery 是独立的逐日核算参考实现，用于交叉验证。
func bruteQuery(h *History, dept string, start, end Date) Result {
	s, en := start.days(), end.days()
	if en <= s {
		return Result{}
	}
	// 每日组织树：parent[deptID]。
	parentOf := func(day int) map[string]string {
		p := make(map[string]string, len(h.Departments))
		for _, d := range h.Departments {
			p[d.ID] = d.Parent
		}
		var cs []ParentChange
		cs = append(cs, h.ParentChanges...)
		sort.SliceStable(cs, func(i, j int) bool { return cs[i].Day.days() < cs[j].Day.days() })
		for _, c := range cs {
			if c.Day.days() <= day {
				p[c.Dept] = c.NewParent
			}
		}
		return p
	}
	inSubtree := func(p map[string]string, x string) bool {
		cur := x
		for i := 0; i < 64; i++ {
			if cur == dept {
				return true
			}
			par, ok := p[cur]
			if !ok || par == "" {
				return false
			}
			cur = par
		}
		return false
	}
	deptAt := func(emp string, day int) string {
		for _, a := range h.Assignments {
			if a.Employee != emp {
				continue
			}
			lo, hi := a.Start.days(), segEnd(a.End, a.Open)
			if lo <= day && day < hi {
				return a.Dept
			}
		}
		return ""
	}
	present := func(emp string, day int) bool {
		for _, t := range h.Tenures {
			if t.Employee != emp {
				continue
			}
			if t.Start.days() <= day && day < segEnd(t.End, t.Open) {
				return true
			}
		}
		return false
	}
	emps := map[string]bool{}
	for _, t := range h.Tenures {
		emps[t.Employee] = true
	}

	var personDays int64
	for d := s; d < en; d++ {
		p := parentOf(d)
		for emp := range emps {
			if present(emp, d) {
				if ad := deptAt(emp, d); ad != "" && inSubtree(p, ad) {
					personDays++
				}
			}
		}
	}
	deps := 0
	for _, t := range h.Tenures {
		if t.Open {
			continue
		}
		endDay := t.End.days()
		if endDay < s || endDay >= en {
			continue
		}
		ad := deptAt(t.Employee, endDay-1)
		if ad == "" {
			continue
		}
		if inSubtree(parentOf(endDay-1), ad) {
			deps++
		}
	}
	days := en - s
	res := Result{PersonDays: personDays, Departures: deps, AvgHeadcount: float64(personDays) / float64(days)}
	if personDays > 0 {
		rate := float64(deps) / res.AvgHeadcount * 100
		res.Attrition = &rate
	}
	return res
}

func compareResult(t *testing.T, label string, got, want Result) {
	t.Helper()
	if got.PersonDays != want.PersonDays || got.Departures != want.Departures {
		t.Fatalf("%s: got (%d, %d), want (%d, %d)", label, got.PersonDays, got.Departures, want.PersonDays, want.Departures)
	}
	if math.Abs(got.AvgHeadcount-want.AvgHeadcount) > 1e-9 {
		t.Fatalf("%s: avg got %v, want %v", label, got.AvgHeadcount, want.AvgHeadcount)
	}
	if (got.Attrition == nil) != (want.Attrition == nil) {
		t.Fatalf("%s: attrition nil mismatch", label)
	}
	if got.Attrition != nil && math.Abs(*got.Attrition-*want.Attrition) > 1e-9 {
		t.Fatalf("%s: attrition got %v, want %v", label, *got.Attrition, *want.Attrition)
	}
}

// 确定性组合历史：同日多处子树迁移 + 员工调岗 + 离职当天重新入职。
func comboHistory() *History {
	d := func(m, day int) Date { return NewDate(2025, m, day) }
	return &History{
		Departments: []Dept{
			{ID: "R", Parent: ""},
			{ID: "A", Parent: "R"},
			{ID: "B", Parent: "R"},
			{ID: "S1", Parent: "A"},
			{ID: "S2", Parent: "A"},
			{ID: "T1", Parent: "B"},
		},
		ParentChanges: []ParentChange{
			// 同日两处子树迁移。
			{Dept: "S1", NewParent: "B", Day: d(3, 10)},
			{Dept: "T1", NewParent: "A", Day: d(3, 10)},
			// 子树嵌套迁移：S2 先挂到 T1，再随 T1 走。
			{Dept: "S2", NewParent: "T1", Day: d(3, 12)},
			{Dept: "S1", NewParent: "S2", Day: d(3, 15)},
		},
		Tenures: []TenureSegment{
			// E1：离职当天重新入职（首尾相接）。
			{Employee: "E1", Start: d(3, 1), End: d(3, 10)},
			{Employee: "E1", Start: d(3, 10), End: d(3, 20)},
			// E2：任职中段调岗。
			{Employee: "E2", Start: d(3, 1), End: d(3, 31)},
			// E3：一直在职。
			{Employee: "E3", Start: d(3, 5), Open: true},
			// E4：两次离职。
			{Employee: "E4", Start: d(3, 2), End: d(3, 8)},
			{Employee: "E4", Start: d(3, 12), End: d(3, 18)},
		},
		Assignments: []Assignment{
			{Employee: "E1", Dept: "S1", Start: d(3, 1), End: d(3, 10)},
			{Employee: "E1", Dept: "T1", Start: d(3, 10), End: d(3, 20)},
			{Employee: "E2", Dept: "S1", Start: d(3, 1), End: d(3, 10)},
			{Employee: "E2", Dept: "S2", Start: d(3, 10), End: d(3, 31)},
			{Employee: "E3", Dept: "T1", Start: d(3, 5), Open: true},
			{Employee: "E4", Dept: "S2", Start: d(3, 2), End: d(3, 8)},
			{Employee: "E4", Dept: "S1", Start: d(3, 12), End: d(3, 18)},
		},
	}
}

func TestComboAgainstBruteForce(t *testing.T) {
	h := comboHistory()
	e := NewEngine(h)
	depts := []string{"R", "A", "B", "S1", "S2", "T1"}
	for _, dep := range depts {
		for s := 1; s <= 28; s += 3 {
			for en := s + 1; en <= 31; en += 4 {
				start, end := NewDate(2025, 3, s), NewDate(2025, 3, en)
				got := e.Query(dep, start, end)
				want := bruteQuery(h, dep, start, end)
				compareResult(t, dep+" "+start.String()+"~"+end.String(), got, want)
			}
		}
	}
}

// 随机历史交叉验证：随机组织树、同日多处迁移、调岗、相邻任职段。
func TestRandomizedAgainstBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(20251004))
	for trial := 0; trial < 30; trial++ {
		h, ids, window, mkDate := genHistoryForTest(rng)
		eng := NewEngine(h)
		for q := 0; q < 40; q++ {
			dep := ids[rng.Intn(len(ids))]
			s := rng.Intn(window)
			en := s + 1 + rng.Intn(window-s)
			start, end := mkDate(s), mkDate(en)
			got := eng.Query(dep, start, end)
			want := bruteQuery(h, dep, start, end)
			compareResult(t, dep+" "+start.String()+"~"+end.String(), got, want)
		}
	}
}

// genHistoryForTest 生成一个随机但保证有效的历史：组织树始终不超过 6 层，
// 任职段不重叠且可相邻，归属段精确覆盖任职段。
func genHistoryForTest(rng *rand.Rand) (*History, []string, int, func(int) Date) {
	const base = 20000 // 任意起点的内部天数
	mkDate := func(day int) Date { return dateFromDays(base + day) }
	const ndept, nemp, window = 12, 15, 90
	h := &History{}
	ids := []string{"D0"}
	depthOf := map[string]int{"D0": 1}
	h.Departments = append(h.Departments, Dept{ID: "D0", Parent: ""})
	for i := 1; i < ndept; i++ {
		id := "D" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		// 初始树深度不超过 5 层。
		var parent string
		for {
			parent = ids[rng.Intn(len(ids))]
			if depthOf[parent] < 5 {
				break
			}
		}
		h.Departments = append(h.Departments, Dept{ID: id, Parent: parent})
		depthOf[id] = depthOf[parent] + 1
		ids = append(ids, id)
	}
	// 随机父级变更，逐条应用并拒绝成环与超深，允许多个变更落在同一天。
	curParent := map[string]string{}
	childrenOf := map[string][]string{}
	for _, d := range h.Departments {
		curParent[d.ID] = d.Parent
		if d.Parent != "" {
			childrenOf[d.Parent] = append(childrenOf[d.Parent], d.ID)
		}
	}
	depthAt := func(id string) int {
		d := 1
		for curParent[id] != "" {
			id = curParent[id]
			d++
		}
		return d
	}
	var heightOf func(id string) int
	heightOf = func(id string) int {
		hgt := 0
		for _, c := range childrenOf[id] {
			if ch := heightOf(c) + 1; ch > hgt {
				hgt = ch
			}
		}
		return hgt
	}
	// 先生成候选变更，再按日期排序后依次校验应用，
	// 保证按日期回放时每一天都是有效树。
	type cand struct {
		day      int
		dept, np string
	}
	var cands []cand
	for k := 0; k < 25; k++ {
		dept := ids[1+rng.Intn(ndept-1)]
		np := ids[rng.Intn(ndept)]
		if np == dept {
			continue
		}
		cands = append(cands, cand{day: rng.Intn(window), dept: dept, np: np})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].day < cands[j].day })
	for _, c := range cands {
		dept, np := c.dept, c.np
		// 拒绝成环：新父不能在自己的子树里。
		cur, cyc := np, false
		for depth := 0; depth < 10; depth++ {
			if cur == dept {
				cyc = true
				break
			}
			p := curParent[cur]
			if p == "" {
				break
			}
			cur = p
		}
		if cyc {
			continue
		}
		// 保持整棵树不超过 6 层。
		if depthAt(np)+1+heightOf(dept) > 6 {
			continue
		}
		h.ParentChanges = append(h.ParentChanges, ParentChange{Dept: dept, NewParent: np, Day: mkDate(c.day)})
		op := curParent[dept]
		cs := childrenOf[op]
		for i, c := range cs {
			if c == dept {
				childrenOf[op] = append(cs[:i], cs[i+1:]...)
				break
			}
		}
		childrenOf[np] = append(childrenOf[np], dept)
		curParent[dept] = np
	}
	// 随机任职与归属：任职段不重叠、可相邻；归属段精确覆盖任职段。
	for i := 0; i < nemp; i++ {
		emp := "E" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		day := rng.Intn(10)
		nt := 1 + rng.Intn(3)
		for seg := 0; seg < nt && day < window-2; seg++ {
			length := 1 + rng.Intn(20)
			end := day + length
			open := false
			if seg == nt-1 && rng.Intn(3) == 0 || end >= window {
				open = true
				end = window + 100 // 开放段
			}
			tn := TenureSegment{Employee: emp, Start: mkDate(day)}
			if open {
				tn.Open = true
			} else {
				tn.End = mkDate(end)
			}
			h.Tenures = append(h.Tenures, tn)
			// 归属段覆盖 [day, end)。
			ad := day
			for {
				var next int
				if open {
					next = end
				} else if rng.Intn(2) == 0 || ad+5 >= end {
					next = end
				} else {
					next = ad + 1 + rng.Intn(min(5, end-ad-1))
				}
				a := Assignment{Employee: emp, Dept: ids[rng.Intn(ndept)], Start: mkDate(ad)}
				if open && next == end {
					a.Open = true
				} else {
					a.End = mkDate(next)
				}
				h.Assignments = append(h.Assignments, a)
				ad = next
				if ad >= end {
					break
				}
			}
			if open {
				break
			}
			// 下一段：可相邻（离职当天重新入职）或留空档。
			day = end + rng.Intn(3)
		}
	}
	return h, ids, window, mkDate
}
