package attrition

import (
	"math"
	"math/rand"
	"testing"
)

// ---------- 独立逐日核算（参照实现，刻意与库内实现不同） ----------

func bruteParentAt(h *History, deptIdx int, day Date) int {
	pcs := h.Departments[deptIdx].ParentChanges
	parent := ""
	best := Date(-1 << 30)
	for _, pc := range pcs {
		if pc.Effective <= day && pc.Effective >= best {
			best = pc.Effective
			parent = pc.Parent
		}
	}
	if parent == "" {
		return -1
	}
	for i, dp := range h.Departments {
		if dp.ID == parent {
			return i
		}
	}
	return -1
}

func bruteUnder(h *History, deptIdx, target int, day Date) bool {
	cur := deptIdx
	for depth := 0; depth < 6; depth++ {
		if cur == target {
			return true
		}
		p := bruteParentAt(h, cur, day)
		if p < 0 {
			return false
		}
		cur = p
	}
	return false
}

func bruteResult(h *History, deptID string, start, end Date) Result {
	var res Result
	target := -1
	deptIdx := map[string]int{}
	for i, dp := range h.Departments {
		deptIdx[dp.ID] = i
		if dp.ID == deptID {
			target = i
		}
	}
	if target < 0 || end <= start {
		return res
	}
	for day := start; day < end; day++ {
		for _, e := range h.Employees {
			for _, tn := range e.Tenures {
				tEnd := openEnd
				if tn.End != nil {
					tEnd = *tn.End
				}
				if day < tn.Start || day >= tEnd {
					continue
				}
				for _, a := range tn.Assignments {
					if a.Start <= day && day < a.End {
						if bruteUnder(h, deptIdx[a.Dept], target, day) {
							res.PersonDays++
						}
						break
					}
				}
			}
		}
	}
	for _, e := range h.Employees {
		for _, tn := range e.Tenures {
			if tn.End == nil || *tn.End < start || *tn.End >= end {
				continue
			}
			last := *tn.End - 1
			for _, a := range tn.Assignments {
				if a.Start <= last && last < a.End {
					if bruteUnder(h, deptIdx[a.Dept], target, last) {
						res.Departures++
					}
					break
				}
			}
		}
	}
	days := int64(end - start)
	res.AvgDailyHeadcount = float64(res.PersonDays) / float64(days)
	if res.PersonDays > 0 {
		rate := float64(res.Departures) / res.AvgDailyHeadcount * 100
		res.AttritionRate = &rate
	}
	return res
}

func compareResults(t *testing.T, label string, got, want Result) {
	t.Helper()
	if got.PersonDays != want.PersonDays || got.Departures != want.Departures {
		t.Errorf("%s: 得到 (人日=%d, 离职=%d), 期望 (人日=%d, 离职=%d)",
			label, got.PersonDays, got.Departures, want.PersonDays, want.Departures)
	}
	if math.Abs(got.AvgDailyHeadcount-want.AvgDailyHeadcount) > 1e-12 {
		t.Errorf("%s: AvgDailyHeadcount = %v, 期望 %v", label, got.AvgDailyHeadcount, want.AvgDailyHeadcount)
	}
	if (got.AttritionRate == nil) != (want.AttritionRate == nil) {
		t.Errorf("%s: AttritionRate 空值不一致: got=%v want=%v", label, got.AttritionRate, want.AttritionRate)
	} else if got.AttritionRate != nil && math.Abs(*got.AttritionRate-*want.AttritionRate) > 1e-9 {
		t.Errorf("%s: AttritionRate = %v, 期望 %v", label, *got.AttritionRate, *want.AttritionRate)
	}
}

// ---------- 随机历史生成 ----------

// genTree 跟踪生成过程中的“当前”组织树，保证每天都不超过六层且无环。
type genTree struct {
	parent   []int
	children [][]int
}

func (g *genTree) depthOf(x int) int {
	d := 0
	for g.parent[x] >= 0 {
		x = g.parent[x]
		d++
	}
	return d
}

func (g *genTree) maxRelDepth(x int) int {
	m := 0
	for _, c := range g.children[x] {
		if v := g.maxRelDepth(c) + 1; v > m {
			m = v
		}
	}
	return m
}

func (g *genTree) inSubtree(x, anc int) bool {
	for x >= 0 {
		if x == anc {
			return true
		}
		x = g.parent[x]
	}
	return false
}

// canMove 判断 x 能否挂到 newParent 下（-1 表示成为根）。
func (g *genTree) canMove(x, newParent int) bool {
	if newParent == x {
		return false
	}
	if newParent >= 0 && g.inSubtree(newParent, x) {
		return false // 会形成环
	}
	base := 0
	if newParent >= 0 {
		base = g.depthOf(newParent) + 1
	}
	return base+g.maxRelDepth(x)+1 <= 6
}

func (g *genTree) move(x, newParent int) {
	if old := g.parent[x]; old >= 0 {
		cs := g.children[old]
		for i, c := range cs {
			if c == x {
				g.children[old] = append(cs[:i], cs[i+1:]...)
				break
			}
		}
	}
	g.parent[x] = newParent
	if newParent >= 0 {
		g.children[newParent] = append(g.children[newParent], x)
	}
}

func randomHistory(rng *rand.Rand, nDept, nEmp, nChanges, spanDays int) *History {
	base := d("2020-01-01")
	h := &History{}
	g := &genTree{parent: make([]int, nDept), children: make([][]int, nDept)}
	for i := range g.parent {
		g.parent[i] = -1
	}
	for i := 0; i < nDept; i++ {
		h.Departments = append(h.Departments, Department{ID: string(rune('A'+i)) + "_" + itoa(i)})
	}
	// 初始随机挂一批，再随机安排变更（允许同日多处变更）。
	for i := 0; i < nDept; i++ {
		tryAttach := func(day Date) {
			for attempt := 0; attempt < 20; attempt++ {
				p := rng.Intn(nDept+1) - 1
				if g.canMove(i, p) {
					g.move(i, p)
					parent := ""
					if p >= 0 {
						parent = h.Departments[p].ID
					}
					h.Departments[i].ParentChanges = append(h.Departments[i].ParentChanges,
						ParentChange{Effective: day, Parent: parent})
					return
				}
			}
		}
		tryAttach(base)
	}
	for c := 0; c < nChanges; c++ {
		day := base.Add(rng.Intn(spanDays))
		i := rng.Intn(nDept)
		for attempt := 0; attempt < 20; attempt++ {
			p := rng.Intn(nDept+1) - 1
			if g.canMove(i, p) {
				g.move(i, p)
				parent := ""
				if p >= 0 {
					parent = h.Departments[p].ID
				}
				h.Departments[i].ParentChanges = append(h.Departments[i].ParentChanges,
					ParentChange{Effective: day, Parent: parent})
				break
			}
		}
	}
	for e := 0; e < nEmp; e++ {
		emp := Employee{ID: "E" + itoa(e)}
		cursor := rng.Intn(spanDays / 3)
		nTenure := 1 + rng.Intn(3)
		for k := 0; k < nTenure; k++ {
			start := cursor + rng.Intn(3) // 间隔可为 0：首尾相接的离职再入职
			length := 1 + rng.Intn(8)
			if start+length > spanDays {
				break
			}
			tn := Tenure{Start: base.Add(start)}
			if rng.Intn(10) < 7 {
				tn.End = datePtrOf(base.Add(start + length))
			}
			// 归属段切分：1-3 段，模拟调岗。
			nSeg := 1 + rng.Intn(3)
			segStart := start
			tEnd := start + length
			if tn.End == nil {
				tEnd = spanDays
			}
			for s := 1; s < nSeg && segStart < tEnd-1; s++ {
				cut := segStart + 1 + rng.Intn(tEnd-segStart-1)
				tn.Assignments = append(tn.Assignments, Assignment{
					Start: base.Add(segStart), End: base.Add(cut),
					Dept: h.Departments[rng.Intn(nDept)].ID,
				})
				segStart = cut
			}
			tn.Assignments = append(tn.Assignments, Assignment{
				Start: base.Add(segStart), End: base.Add(tEnd),
				Dept: h.Departments[rng.Intn(nDept)].ID,
			})
			emp.Tenures = append(emp.Tenures, tn)
			if tn.End == nil {
				break
			}
			cursor = start + length
		}
		h.Employees = append(h.Employees, emp)
	}
	return h
}

func datePtrOf(v Date) *Date { return &v }

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [8]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	return string(b[p:])
}

func TestCrossCheckRandom(t *testing.T) {
	base := d("2020-01-01")
	for seed := int64(0); seed < 40; seed++ {
		rng := rand.New(rand.NewSource(seed))
		span := 60
		h := randomHistory(rng, 12, 10, 30, span)
		store, err := Build(h)
		if err != nil {
			t.Fatalf("seed=%d: %v", seed, err)
		}
		for q := 0; q < 30; q++ {
			dept := h.Departments[rng.Intn(len(h.Departments))].ID
			s := rng.Intn(span)
			e := s + 1 + rng.Intn(span-s)
			start, end := base.Add(s), base.Add(e)
			got := store.Query(dept, start, end)
			want := bruteResult(h, dept, start, end)
			compareResults(t, deptLabel(seed, dept, start, end), got, want)
		}
	}
}

func deptLabel(seed int64, dept string, start, end Date) string {
	return "seed=" + itoa(int(seed)) + " dept=" + dept + " [" + start.String() + "," + end.String() + ")"
}
