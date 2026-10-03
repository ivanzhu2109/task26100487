package attrition

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"
)

// genScaleHistory 生成满足规模要求的确定性随机历史：
// 5000 员工、5000 部门、25000 段任职、100000 段归属、20000 次父级变更、50 年跨度。
func genScaleHistory(rng *rand.Rand) (*History, []string, []string, int) {
	const (
		ndept    = 5000
		nemp     = 5000
		ntenure  = 25000
		nassign  = 100000
		nchange  = 20000
		spanDays = 50*365 + 12 // 50 年（含闰日）
	)
	base := NewDate(2000, 1, 1)
	mkDate := func(day int) Date { return base.AddDays(day) }

	h := &History{}
	deptIDs := make([]string, 0, ndept)
	empIDs := make([]string, 0, nemp)
	for i := 0; i < ndept; i++ {
		deptIDs = append(deptIDs, fmt.Sprintf("D%05d", i))
	}
	for i := 0; i < nemp; i++ {
		empIDs = append(empIDs, fmt.Sprintf("E%05d", i))
	}

	// 初始树：深度不超过 5 层。
	depthOf := map[string]int{deptIDs[0]: 1}
	h.Departments = append(h.Departments, Dept{ID: deptIDs[0]})
	for i := 1; i < ndept; i++ {
		var parent string
		for {
			parent = deptIDs[rng.Intn(i)]
			if depthOf[parent] < 5 {
				break
			}
		}
		h.Departments = append(h.Departments, Dept{ID: deptIDs[i], Parent: parent})
		depthOf[deptIDs[i]] = depthOf[parent] + 1
	}

	// 父级变更：候选按日排序后依次校验应用，保证每天树有效且不超过 6 层。
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
	type cand struct{ day, dept, np int }
	cands := make([]cand, 0, nchange)
	// 候选数量留出富余，使校验后实际变更达到 20000 次。
	for len(cands) < nchange*4 {
		cands = append(cands, cand{rng.Intn(spanDays), 1 + rng.Intn(ndept-1), rng.Intn(ndept)})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].day < cands[j].day })
	for _, c := range cands {
		if len(h.ParentChanges) >= nchange {
			break
		}
		dept, np := deptIDs[c.dept], deptIDs[c.np]
		if c.dept == c.np {
			continue
		}
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
		if cyc || depthAt(np)+1+heightOf(dept) > 6 {
			continue
		}
		h.ParentChanges = append(h.ParentChanges, ParentChange{Dept: dept, NewParent: np, Day: mkDate(c.day)})
		op := curParent[dept]
		cs := childrenOf[op]
		for i, ch := range cs {
			if ch == dept {
				childrenOf[op] = append(cs[:i], cs[i+1:]...)
				break
			}
		}
		childrenOf[np] = append(childrenOf[np], dept)
		curParent[dept] = np
	}

	// 任职：每员工 5 段，可相邻；归属：每段任职 4 段，精确覆盖。
	perEmpTenure := ntenure / nemp
	perTenureAssign := nassign / ntenure
	for _, emp := range empIDs {
		day := rng.Intn(spanDays / 4)
		for seg := 0; seg < perEmpTenure; seg++ {
			length := 30 + rng.Intn(2000)
			end := day + length
			open := seg == perEmpTenure-1 && rng.Intn(4) == 0
			tn := TenureSegment{Employee: emp, Start: mkDate(day)}
			if open {
				tn.Open = true
			} else {
				tn.End = mkDate(end)
			}
			h.Tenures = append(h.Tenures, tn)
			// 归属段覆盖 [day, end)。
			cuts := make([]int, 0, perTenureAssign-1)
			for k := 0; k < perTenureAssign-1; k++ {
				cuts = append(cuts, day+1+rng.Intn(length-1))
			}
			sort.Ints(cuts)
			ad := day
			for k := 0; k < perTenureAssign; k++ {
				next := end
				if k < len(cuts) {
					next = cuts[k]
				}
				a := Assignment{Employee: emp, Dept: deptIDs[rng.Intn(ndept)], Start: mkDate(ad)}
				if open && k == perTenureAssign-1 {
					a.Open = true
				} else {
					a.End = mkDate(next)
				}
				h.Assignments = append(h.Assignments, a)
				ad = next
			}
			if open {
				break
			}
			day = end + rng.Intn(30)
		}
	}
	return h, deptIDs, empIDs, spanDays
}

// scaleBrute 是面向规模数据的分组逐日核算参考实现。
func scaleBrute(h *History, dept string, s, en int) (int64, int) {
	tenures := map[string][]TenureSegment{}
	for _, t := range h.Tenures {
		tenures[t.Employee] = append(tenures[t.Employee], t)
	}
	assigns := map[string][]Assignment{}
	for _, a := range h.Assignments {
		assigns[a.Employee] = append(assigns[a.Employee], a)
	}
	// 逐日父级快照。
	changes := append([]ParentChange{}, h.ParentChanges...)
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Day.days() < changes[j].Day.days() })
	parent := map[string]string{}
	for _, d := range h.Departments {
		parent[d.ID] = d.Parent
	}
	ci := 0
	inSubtree := func(x string) bool {
		cur := x
		for d := 0; d < 10; d++ {
			if cur == dept {
				return true
			}
			p := parent[cur]
			if p == "" {
				return false
			}
			cur = p
		}
		return false
	}
	deptCache := map[string]bool{}
	var personDays int64
	for day := s; day < en; day++ {
		for ci < len(changes) && changes[ci].Day.days() <= day {
			parent[changes[ci].Dept] = changes[ci].NewParent
			ci++
		}
		for k := range deptCache {
			delete(deptCache, k)
		}
		for emp, tns := range tenures {
			present := false
			for _, t := range tns {
				if t.Start.days() <= day && day < segEnd(t.End, t.Open) {
					present = true
					break
				}
			}
			if !present {
				continue
			}
			ad := ""
			for _, a := range assigns[emp] {
				if a.Start.days() <= day && day < segEnd(a.End, a.Open) {
					ad = a.Dept
					break
				}
			}
			if ad == "" {
				continue
			}
			mem, ok := deptCache[ad]
			if !ok {
				mem = inSubtree(ad)
				deptCache[ad] = mem
			}
			if mem {
				personDays++
			}
		}
	}
	// 离职：离职日前一日的直接部门与当日组织树。
	parent = map[string]string{}
	for _, d := range h.Departments {
		parent[d.ID] = d.Parent
	}
	deps := 0
	for emp, tns := range tenures {
		for _, t := range tns {
			if t.Open {
				continue
			}
			endDay := t.End.days()
			if endDay < s || endDay >= en {
				continue
			}
			prev := endDay - 1
			for _, d := range h.Departments {
				parent[d.ID] = d.Parent
			}
			for _, c := range changes {
				if c.Day.days() <= prev {
					parent[c.Dept] = c.NewParent
				}
			}
			ad := ""
			for _, a := range assigns[emp] {
				if a.Start.days() <= prev && prev < segEnd(a.End, a.Open) {
					ad = a.Dept
					break
				}
			}
			if ad != "" && inSubtree(ad) {
				deps++
			}
		}
	}
	return personDays, deps
}

func heapInuseMB() float64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.HeapInuse) / (1 << 20)
}

// 规模测试：装载 + 300 次查询的总耗时与峰值内存，并核对部分查询正确性。
func TestScale(t *testing.T) {
	if testing.Short() {
		t.Skip("scale test skipped in -short mode")
	}
	rng := rand.New(rand.NewSource(42))
	h, deptIDs, _, spanDays := genScaleHistory(rng)
	base := NewDate(2000, 1, 1)
	t.Logf("history: %d depts, %d changes, %d tenures, %d assignments",
		len(h.Departments), len(h.ParentChanges), len(h.Tenures), len(h.Assignments))

	// 落盘后从文件装载，覆盖完整的本地文件路径。
	path := filepath.Join(t.TempDir(), "history.json")
	if err := h.Save(path); err != nil {
		t.Fatal(err)
	}

	t0 := time.Now()
	eng, err := LoadEngine(path)
	if err != nil {
		t.Fatal(err)
	}
	loadDur := time.Since(t0)
	loadHeap := heapInuseMB()

	// 300 次不同部门与区间的查询，区间长度混合短中长期。
	t1 := time.Now()
	type qres struct {
		dept  string
		s, en int
		res   Result
	}
	var results []qres
	var totalPD int64
	for i := 0; i < 300; i++ {
		dept := deptIDs[rng.Intn(len(deptIDs))]
		var length int
		switch i % 3 {
		case 0:
			length = 30 + rng.Intn(335) // 短：约一年内
		case 1:
			length = 365 + rng.Intn(4*365) // 中：1~5 年
		default:
			length = 5*365 + rng.Intn(spanDays-5*365) // 长：5~50 年
		}
		s := rng.Intn(spanDays - length)
		res := eng.Query(dept, base.AddDays(s), base.AddDays(s+length))
		results = append(results, qres{dept, s, s + length, res})
		totalPD += res.PersonDays
	}
	queryDur := time.Since(t1)
	peakHeap := heapInuseMB()
	total := loadDur + queryDur
	t.Logf("load=%v queries=%v total=%v heapAfterLoad=%.1fMiB heapAfterQueries=%.1fMiB totalPersonDays=%d",
		loadDur, queryDur, total, loadHeap, peakHeap, totalPD)
	if total > 30*time.Second {
		t.Fatalf("total time %v exceeds 30s", total)
	}
	if peakHeap > 256 {
		t.Fatalf("peak heap %.1fMiB exceeds 256MiB", peakHeap)
	}

	// 核对部分查询：取 6 个短区间查询与逐日参考实现对比。
	checked := 0
	for _, r := range results {
		if r.en-r.s > 400 {
			continue
		}
		pd, deps := scaleBrute(h, r.dept, base.AddDays(r.s).days(), base.AddDays(r.en).days())
		if pd != r.res.PersonDays || deps != r.res.Departures {
			t.Fatalf("query %s [%d,%d): got (%d,%d), want (%d,%d)",
				r.dept, r.s, r.en, r.res.PersonDays, r.res.Departures, pd, deps)
		}
		checked++
		if checked >= 6 {
			break
		}
	}
	t.Logf("cross-checked %d queries against day-by-day reference", checked)
}
