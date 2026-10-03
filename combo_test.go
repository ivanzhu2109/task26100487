package attrition

import "testing"

// 组合场景（2025-03）：
//
//	R ─┬─ A ─┬─ A1
//	   │     └─ A2
//	   └─ B ─── B1
//
// 03-10 同日：A1 从 A 转到 B；A2 从 A 转到 B1（多处子树迁移）；
//
//	E1 同日从 A1 调岗到 B1；E2 同日从 A2 离职并重新入职 B1。
//
// E3：03-09 入职 A1，03-11 离职。
func comboHistory() *History {
	mk := func(id, parent string, changes ...ParentChange) Department {
		return Department{ID: id, ParentChanges: changes}
	}
	pc := func(day, parent string) ParentChange {
		return ParentChange{Effective: d(day), Parent: parent}
	}
	return &History{
		Departments: []Department{
			mk("R", ""),
			mk("A", "", pc("2025-01-01", "R")),
			mk("B", "", pc("2025-01-01", "R")),
			mk("A1", "", pc("2025-01-01", "A"), pc("2025-03-10", "B")),
			mk("A2", "", pc("2025-01-01", "A"), pc("2025-03-10", "B1")),
			mk("B1", "", pc("2025-01-01", "B")),
		},
		Employees: []Employee{
			{ID: "E1", Tenures: []Tenure{ // 持续在岗，03-10 调岗 A1 -> B1
				{
					Start: d("2025-03-01"),
					Assignments: []Assignment{
						{Start: d("2025-03-01"), End: d("2025-03-10"), Dept: "A1"},
						{Start: d("2025-03-10"), End: d("2026-01-01"), Dept: "B1"},
					},
				},
			}},
			{ID: "E2", Tenures: []Tenure{ // 03-10 离职并同日重新入职
				{
					Start: d("2025-03-01"),
					End:   datePtr("2025-03-10"),
					Assignments: []Assignment{
						{Start: d("2025-03-01"), End: d("2025-03-10"), Dept: "A2"},
					},
				},
				{
					Start: d("2025-03-10"),
					Assignments: []Assignment{
						{Start: d("2025-03-10"), End: d("2026-01-01"), Dept: "B1"},
					},
				},
			}},
			{ID: "E3", Tenures: []Tenure{ // 03-09 入职 A1，03-11 离职
				{
					Start: d("2025-03-09"),
					End:   datePtr("2025-03-11"),
					Assignments: []Assignment{
						{Start: d("2025-03-09"), End: d("2025-03-11"), Dept: "A1"},
					},
				},
			}},
		},
	}
}

func TestComboHandComputed(t *testing.T) {
	store, err := Build(comboHistory())
	if err != nil {
		t.Fatal(err)
	}
	start, end := d("2025-03-08"), d("2025-03-13") // 5 天

	rate := func(v float64) *float64 { return &v }
	checkResult(t, "A", store.Query("A", start, end), 5, 1, rate(100))
	checkResult(t, "B", store.Query("B", start, end), 7, 1, rate(100.0/1.4))
	checkResult(t, "B1", store.Query("B1", start, end), 6, 0, rate(0))
	checkResult(t, "A2", store.Query("A2", start, end), 2, 1, rate(250))

	// 离职判定用离职前一日，即使前一日不在查询区间内：
	// E2 的离职日 03-10 落在区间内，按 03-09 的 A2（属 A）计入 A；
	// 区间内 A 无任何在岗，人员日数为零，流失率为空但离职次数为 1。
	checkResult(t, "A[03-10,03-13)", store.Query("A", d("2025-03-10"), end), 0, 1, nil)
	// 同一离职对 B 不计数（03-09 时 A2 尚属 A）。
	checkResult(t, "B[03-10,03-13)", store.Query("B", d("2025-03-10"), end), 7, 1, rate(300.0/7))
}

// 同一组合历史与独立逐日核算在多个区间上完全一致。
func TestComboVsBruteForce(t *testing.T) {
	h := comboHistory()
	store, err := Build(h)
	if err != nil {
		t.Fatal(err)
	}
	depts := []string{"R", "A", "B", "A1", "A2", "B1"}
	for _, dept := range depts {
		for s := 5; s <= 12; s++ {
			for e := s + 1; e <= 16; e++ {
				start := d("2025-03-01").Add(s - 1)
				end := d("2025-03-01").Add(e - 1)
				got := store.Query(dept, start, end)
				want := bruteResult(h, dept, start, end)
				compareResults(t, dept+" ["+start.String()+","+end.String()+")", got, want)
			}
		}
	}
}
