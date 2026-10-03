# attrition

按历史组织口径统计人员流失率的 Go 库。统计严格反映每个历史日期真实的组织归属：
部门合并、整支团队转移、员工重新入职都不会让历史报表被“当前”组织树污染。

## 装载

```go
store, err := attrition.LoadFile("history.json") // 本地 JSON 文件，一次装载后只读
// 或由内存数据构建：
store, err := attrition.Build(&attrition.History{...})
```

JSON 格式与 `History` 结构一致，日期为 `"YYYY-MM-DD"` 字符串：

```json
{
  "departments": [
    {"id": "A"},
    {"id": "X", "parentChanges": [
      {"effective": "2025-01-01", "parent": "A"},
      {"effective": "2025-07-03", "parent": "B"}
    ]}
  ],
  "employees": [
    {"id": "E1", "tenures": [
      {"start": "2025-07-01", "end": "2025-07-03",
       "assignments": [{"start": "2025-07-01", "end": "2025-07-03", "dept": "X"}]}
    ]}
  ]
}
```

- `parent` 为空字符串表示成为根部门；首次变更生效前视为根部门。
- 任职 `end` 省略表示未离职、持续有效；入职日计在岗，离职日不计。
- 输入保证完整有效，装载不做业务校验。

## 查询

```go
res := store.Query("A", attrition.Civil(2025, 7, 1), attrition.Civil(2025, 7, 6))
// res.PersonDays         人员日数（区间每天范围在岗人数之和）
// res.Departures         离职次数（离职日落在区间内的任职结束事件）
// res.AvgDailyHeadcount  平均每日在岗人数 = PersonDays / 区间自然日数
// res.AttritionRate      未年化流失率(%) = Departures / Avg * 100；人员日数为零时为 nil
```

- 区间为起日包含、止日不包含的公历自然日。
- 查询范围 = 该部门及每天组织树中的全部后代部门，逐日判定。
- 离职按离职前一日的直接部门与当天组织树判定归属，即使前一日不在查询区间内。
- 调岗与部门迁移只改变相应日期的人数，不产生离职。

## 测试

```sh
go test ./...          # 全部测试
go test -run TestScale -v .  # 规模测试（打印装载/查询耗时与内存）
```

规模测试以固定种子生成 5000 员工、5000 部门、25000 段任职、100000 段归属、
20000 次父级变更、50 年跨度的历史，连续 300 次查询，并抽样与独立逐日核算比对。
参考结果（Apple M 系列）：装载约 18ms，300 次查询约 190ms，堆内存约 11 MiB。
