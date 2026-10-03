package attrition

import (
	"math/rand"
	"runtime"
	"testing"
	"time"
)

// 规模参数（验收要求的上限）。
const (
	scaleDepts      = 5000
	scaleEmployees  = 5000
	scaleTenures    = 25000
	scaleAssigns    = 100000 // = 25000 段任职 × 4 段归属
	scaleParentChgs = 20000
	scaleSpanYears  = 50
	scaleQueries    = 300
	scaleSeed       = 42
)

// genScaleHistory 生成确定性规模历史：组织树任意一天不超过六层。
func genScaleHistory(rng *rand.Rand, base Date, spanDays int) *History {
	h := &History{}

	// 部门与初始树。
	g := &genTree{parent: make([]int, scaleDepts), children: make([][]int, scaleDepts)}
	depth := make([]int, scaleDepts)
	for i := 0; i < scaleDepts; i++ {
		h.Departments = append(h.Departments, Department{ID: "D" + itoa(i)})
		g.parent[i] = -1
	}
	for i := 1; i < scaleDepts; i++ {
		for {
			p := rng.Intn(i + 1) // 可能选到自己(i)表示挂根
			if p == i {
				break
			}
			if depth[p]+1 <= 6 {
				g.move(i, p)
				depth[i] = depth[p] + 1
				h.Departments[i].ParentChanges = append(h.Departments[i].ParentChanges,
					ParentChange{Effective: base, Parent: h.Departments[p].ID})
				break
			}
		}
	}

	// 父级变更：按时间顺序生成，逐步维护当前树，保证每天都不超过六层。
	day := 0
	for c := 0; c < scaleParentChgs; c++ {
		day += rng.Intn(3) // 允许同日多处变更
		if day >= spanDays {
			day = spanDays - 1
		}
		for attempt := 0; attempt < 50; attempt++ {
			x := rng.Intn(scaleDepts)
			p := rng.Intn(scaleDepts+1) - 1
			if p == x || (p >= 0 && g.inSubtree(p, x)) {
				continue
			}
			base2 := 0
			if p >= 0 {
				base2 = g.depthOf(p) + 1
			}
			if base2+g.maxRelDepth(x)+1 > 6 {
				continue
			}
			g.move(x, p)
			parent := ""
			if p >= 0 {
				parent = h.Departments[p].ID
			}
			h.Departments[x].ParentChanges = append(h.Departments[x].ParentChanges,
				ParentChange{Effective: base.Add(day), Parent: parent})
			break
		}
	}

	// 员工：每人若干段不重叠任职，每段 4 段直接部门归属。
	perEmp := scaleTenures / scaleEmployees
	for e := 0; e < scaleEmployees; e++ {
		emp := Employee{ID: "E" + itoa(e)}
		cursor := rng.Intn(spanDays / 4)
		for k := 0; k < perEmp; k++ {
			length := 30 + rng.Intn(1500)
			if cursor+length > spanDays {
				break
			}
			tn := Tenure{Start: base.Add(cursor)}
			ongoing := rng.Intn(10) < 2
			endDay := cursor + length
			if !ongoing {
				tn.End = datePtrOf(base.Add(endDay))
			}
			// 4 段归属切分。
			cuts := []int{cursor}
			for len(cuts) < scaleAssigns/scaleTenures {
				cuts = append(cuts, cursor+1+rng.Intn(length-1))
			}
			sortInts(cuts)
			cuts = append(cuts, endDay)
			for s := 0; s+1 < len(cuts); s++ {
				tn.Assignments = append(tn.Assignments, Assignment{
					Start: base.Add(cuts[s]),
					End:   base.Add(cuts[s+1]),
					Dept:  h.Departments[rng.Intn(scaleDepts)].ID,
				})
			}
			emp.Tenures = append(emp.Tenures, tn)
			cursor = endDay + rng.Intn(60) // 间隔可为 0：首尾相接的离职再入职
		}
		h.Employees = append(h.Employees, emp)
	}
	return h
}

func sortInts(v []int) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

func TestScale(t *testing.T) {
	base := Civil(1976, 1, 1)
	spanDays := scaleSpanYears*365 + scaleSpanYears/4 // 约 50 年（含闰年）
	rng := rand.New(rand.NewSource(scaleSeed))
	h := genScaleHistory(rng, base, spanDays)

	nTenures, nAssigns, nChanges := 0, 0, 0
	for _, e := range h.Employees {
		nTenures += len(e.Tenures)
		for _, tn := range e.Tenures {
			nAssigns += len(tn.Assignments)
		}
	}
	for _, dp := range h.Departments {
		nChanges += len(dp.ParentChanges)
	}
	t.Logf("规模: 员工=%d 部门=%d 任职=%d 归属段=%d 父级变更=%d 跨度=%d天",
		len(h.Employees), len(h.Departments), nTenures, nAssigns, nChanges, spanDays)

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)
	t0 := time.Now()

	store, err := Build(h)
	if err != nil {
		t.Fatal(err)
	}
	loadDur := time.Since(t0)

	// 连续 300 次不同部门与区间的查询。
	t1 := time.Now()
	qrng := rand.New(rand.NewSource(scaleSeed + 1))
	type queryCase struct {
		dept       string
		start, end Date
	}
	queries := make([]queryCase, 0, scaleQueries)
	for i := 0; i < scaleQueries; i++ {
		s := qrng.Intn(spanDays - 8)
		length := 7 + qrng.Intn(3650)
		if i < 8 {
			length = 7 + qrng.Intn(24) // 短区间，用于逐日核算比对
		} else if i < 10 {
			length = 180 // 中区间，同样参与比对
		}
		if s+length > spanDays {
			length = spanDays - s
		}
		queries = append(queries, queryCase{
			dept:  h.Departments[qrng.Intn(scaleDepts)].ID,
			start: base.Add(s),
			end:   base.Add(s + length),
		})
	}
	var sink int64
	for _, q := range queries {
		r := store.Query(q.dept, q.start, q.end)
		sink += r.PersonDays + int64(r.Departures)
	}
	queryDur := time.Since(t1)
	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	total := loadDur + queryDur
	heapMiB := float64(memAfter.HeapAlloc) / (1 << 20)
	totalAllocMiB := float64(memAfter.TotalAlloc-memBefore.TotalAlloc) / (1 << 20)
	t.Logf("装载耗时=%v 查询耗时=%v(300次) 总耗时=%v", loadDur, queryDur, total)
	t.Logf("装载+查询后堆内存=%.1f MiB, 期间累计分配=%.1f MiB (sink=%d)", heapMiB, totalAllocMiB, sink)
	if total > 30*time.Second {
		t.Errorf("总耗时 %v 超过 30 秒", total)
	}
	if heapMiB > 256 {
		t.Errorf("堆内存 %.1f MiB 超过 256 MiB", heapMiB)
	}

	// 抽样正确性：前 10 个查询（8 短 + 2 中）与独立逐日核算比对。
	checked := 0
	for i := 0; i < 10; i++ {
		q := queries[i]
		got := store.Query(q.dept, q.start, q.end)
		want := bruteResult(h, q.dept, q.start, q.end)
		compareResults(t, "scale["+q.dept+"]", got, want)
		checked++
	}
	t.Logf("抽样比对通过: %d 个查询与逐日核算一致", checked)
}
