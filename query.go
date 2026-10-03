package attrition

import "sort"

// parentAt 返回部门 di 在日 t 的父部门（无父部门返回 noParent），
// 以及 t 之后最近一次父级变更的生效日（无则返回 openEnd）。
func (s *Store) parentAt(di int32, t Date) (parent int32, nextChange Date) {
	info := &s.depts[di]
	// 最后一个 Effective <= t 的变更生效。
	i := sort.Search(len(info.changeDays), func(j int) bool { return info.changeDays[j] > t }) - 1
	parent = noParent
	if i >= 0 {
		parent = info.changeParent[i]
	}
	nextChange = openEnd
	if i+1 < len(info.changeDays) {
		nextChange = info.changeDays[i+1]
	}
	return parent, nextChange
}

// personDays 返回部门 di 在 [a, b) 内的人员日数。
func (s *Store) personDays(di int32, a, b Date) int64 {
	info := &s.depts[di]
	at := func(t Date) int64 {
		i := sort.Search(len(info.hcDays), func(j int) bool { return info.hcDays[j] > t }) - 1
		if i < 0 {
			return 0
		}
		return info.hcCum[i] + int64(info.hcCount[i])*int64(t-info.hcDays[i])
	}
	return at(b) - at(a)
}

// underAt 报告部门 di 在日 t 是否等于 target 或为 target 的后代。
func (s *Store) underAt(di, target int32, t Date) bool {
	cur := di
	for depth := 0; depth < maxDepth; depth++ {
		if cur == target {
			return true
		}
		p, _ := s.parentAt(cur, t)
		if p == noParent {
			return false
		}
		cur = p
	}
	return false
}

// Query 统计部门 deptID 在区间 [start, end) 内的流失指标。
// 范围包含该部门及每天组织树中的全部后代部门；起日包含、止日不包含。
// 部门不存在或区间为空时返回零值结果（流失率为空）。
func (s *Store) Query(deptID string, start, end Date) Result {
	var res Result
	target, ok := s.byID[deptID]
	if !ok || end <= start {
		return res
	}
	days := int64(end - start)

	// 人员日数：对每个有过在岗记录的部门，沿其祖先链在区间内扫描，
	// 祖先链在相邻父级变更之间保持不变，逐段累加处于查询范围的子区间。
	for _, di := range s.active {
		for t := start; t < end; {
			cur := di
			under := false
			next := end
			for depth := 0; depth < maxDepth; depth++ {
				if cur == int32(target) {
					under = true
				}
				p, chg := s.parentAt(cur, t)
				if chg < next {
					next = chg
				}
				if p == noParent {
					break
				}
				cur = p
			}
			if under {
				res.PersonDays += s.personDays(di, t, next)
			}
			t = next
		}
	}

	// 离职次数：离职日落在区间内，按离职前一日的直接部门与当天组织树判定。
	lo := sort.Search(len(s.depDays), func(i int) bool { return s.depDays[i] >= start })
	hi := sort.Search(len(s.depDays), func(i int) bool { return s.depDays[i] >= end })
	for i := lo; i < hi; i++ {
		if s.underAt(s.depDept[i], int32(target), s.depDays[i]-1) {
			res.Departures++
		}
	}

	res.AvgDailyHeadcount = float64(res.PersonDays) / float64(days)
	if res.PersonDays > 0 {
		// 不先舍入平均人数，直接由原始计数计算。
		rate := float64(res.Departures) * float64(days) / float64(res.PersonDays) * 100
		res.AttritionRate = &rate
	}
	return res
}
