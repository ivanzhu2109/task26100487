package attrition

import (
	"math"
	"path/filepath"
	"testing"
)

func d(s string) Date {
	v, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return v
}

func datePtr(s string) *Date {
	v := d(s)
	return &v
}

// 验收场景：一名员工 2025-07-01 入职、07-03 离职；
// 其子部门恰在 07-03 从 A 转到 B。查询 07-01 至 07-06。
func acceptanceHistory() *History {
	return &History{
		Departments: []Department{
			{ID: "A"},
			{ID: "B"},
			{ID: "X", ParentChanges: []ParentChange{
				{Effective: d("2025-01-01"), Parent: "A"},
				{Effective: d("2025-07-03"), Parent: "B"},
			}},
		},
		Employees: []Employee{
			{ID: "E1", Tenures: []Tenure{
				{
					Start: d("2025-07-01"),
					End:   datePtr("2025-07-03"),
					Assignments: []Assignment{
						{Start: d("2025-07-01"), End: d("2025-07-03"), Dept: "X"},
					},
				},
			}},
		},
	}
}

func checkResult(t *testing.T, name string, got Result, personDays int64, departures int, rate *float64) {
	t.Helper()
	if got.PersonDays != personDays {
		t.Errorf("%s: PersonDays = %d, 期望 %d", name, got.PersonDays, personDays)
	}
	if got.Departures != departures {
		t.Errorf("%s: Departures = %d, 期望 %d", name, got.Departures, departures)
	}
	if rate == nil {
		if got.AttritionRate != nil {
			t.Errorf("%s: AttritionRate = %v, 期望为空", name, *got.AttritionRate)
		}
	} else {
		if got.AttritionRate == nil {
			t.Fatalf("%s: AttritionRate 为空, 期望 %v", name, *rate)
		}
		if math.Abs(*got.AttritionRate-*rate) > 1e-9 {
			t.Errorf("%s: AttritionRate = %v, 期望 %v", name, *got.AttritionRate, *rate)
		}
	}
}

func TestAcceptance(t *testing.T) {
	store, err := Build(acceptanceHistory())
	if err != nil {
		t.Fatal(err)
	}
	start, end := d("2025-07-01"), d("2025-07-06")

	rateA := 250.0
	checkResult(t, "A", store.Query("A", start, end), 2, 1, &rateA)
	checkResult(t, "B", store.Query("B", start, end), 0, 0, nil)
}

// 同一场景经由本地文件装载，结果须一致。
func TestAcceptanceViaFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := SaveFile(acceptanceHistory(), path); err != nil {
		t.Fatal(err)
	}
	store, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	start, end := d("2025-07-01"), d("2025-07-06")

	rateA := 250.0
	checkResult(t, "A(file)", store.Query("A", start, end), 2, 1, &rateA)
	checkResult(t, "B(file)", store.Query("B", start, end), 0, 0, nil)
}
