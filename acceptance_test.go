package attrition

import (
	"math"
	"testing"
)

// 验收场景：员工 2025-07-01 入职、07-03 离职；其子部门 07-03 从 A 转到 B。
// 查询 07-01 至 07-06：A 应有 2 个人员日、1 次离职、250% 流失率；B 全零、流失率为空。
func TestAcceptance(t *testing.T) {
	h := &History{
		Departments: []Dept{
			{ID: "A", Parent: ""},
			{ID: "B", Parent: ""},
			{ID: "S", Parent: "A"},
		},
		ParentChanges: []ParentChange{
			{Dept: "S", NewParent: "B", Day: NewDate(2025, 7, 3)},
		},
		Tenures: []TenureSegment{
			{Employee: "E1", Start: NewDate(2025, 7, 1), End: NewDate(2025, 7, 3)},
		},
		Assignments: []Assignment{
			{Employee: "E1", Dept: "S", Start: NewDate(2025, 7, 1), End: NewDate(2025, 7, 3)},
		},
	}
	e := NewEngine(h)
	start, end := NewDate(2025, 7, 1), NewDate(2025, 7, 6)

	a := e.Query("A", start, end)
	if a.PersonDays != 2 {
		t.Fatalf("A person-days = %d, want 2", a.PersonDays)
	}
	if a.Departures != 1 {
		t.Fatalf("A departures = %d, want 1", a.Departures)
	}
	if a.Attrition == nil {
		t.Fatal("A attrition should not be nil")
	}
	if math.Abs(*a.Attrition-250.0) > 1e-9 {
		t.Fatalf("A attrition = %v, want 250", *a.Attrition)
	}
	if math.Abs(a.AvgHeadcount-0.4) > 1e-12 {
		t.Fatalf("A avg headcount = %v, want 0.4", a.AvgHeadcount)
	}

	b := e.Query("B", start, end)
	if b.PersonDays != 0 || b.Departures != 0 {
		t.Fatalf("B = %+v, want zero person-days and departures", b)
	}
	if b.Attrition != nil {
		t.Fatalf("B attrition = %v, want nil", *b.Attrition)
	}
}
