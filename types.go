// Package attrition 按历史组织口径统计人员流失率。
//
// 历史数据一次装载后只读。统计严格按每个历史日期的真实组织归属计算，
// 不受装载后任何组织调整影响（本库不保存“当前”组织概念，只有带生效日的历史）。
package attrition

// Date 为公历自然日（自 1970-01-01 起的天数），不涉及时间与时区。
type Date int32

// ParentChange 表示某部门自 Effective 当天起父部门变为 Parent。
// Parent 为空字符串表示该部门成为根（无父部门）。
// 整棵子树随部门一起改变归属。
type ParentChange struct {
	Effective Date   `json:"effective"`
	Parent    string `json:"parent"`
}

// Department 使用永久标识；ParentChanges 记录其全部父级变更历史。
// 首次变更生效之前该部门视为根部门。
type Department struct {
	ID            string         `json:"id"`
	ParentChanges []ParentChange `json:"parentChanges,omitempty"`
}

// Assignment 表示任职期间某段直接部门归属，[Start, End) 半开区间。
type Assignment struct {
	Start Date   `json:"start"`
	End   Date   `json:"end"`
	Dept  string `json:"dept"`
}

// Tenure 为一段任职区间 [Start, End)。End 为 nil 表示未离职、持续有效。
// 入职日计为在岗，离职日不计为在岗。
type Tenure struct {
	Start       Date         `json:"start"`
	End         *Date        `json:"end,omitempty"`
	Assignments []Assignment `json:"assignments"`
}

// Employee 使用永久身份标识，可有多段不重叠的任职区间。
type Employee struct {
	ID      string   `json:"id"`
	Tenures []Tenure `json:"tenures"`
}

// History 为完整历史输入。输入保证完整有效，装载时不做业务校验。
type History struct {
	Departments []Department `json:"departments"`
	Employees   []Employee   `json:"employees"`
}

// Result 为一次查询的统计结果。
type Result struct {
	// PersonDays 人员日数：区间内每天查询范围在岗人数之和。
	PersonDays int64
	// Departures 离职次数：离职日落在区间内的任职结束事件数。
	Departures int
	// AvgDailyHeadcount 平均每日在岗人数 = PersonDays / 区间自然日数。
	AvgDailyHeadcount float64
	// AttritionRate 未年化流失率（百分数）= Departures / AvgDailyHeadcount * 100。
	// PersonDays 为零时为空（nil）。
	AttritionRate *float64
}
