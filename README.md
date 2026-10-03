# attrition

按历史组织口径统计人员流失率的 Go 库。统计反映每个历史日期真实的组织归属：
部门合并、整支团队转移、员工重新入职都不会让历史报表失真。

## 数据模型

历史数据保存在本地 JSON 文件，一次装载后只读：

```json
{
  "departments":   [{"id": "D1", "parent": ""}],
  "parentChanges": [{"dept": "D2", "newParent": "D1", "day": "2025-07-03"}],
  "tenures":       [{"employee": "E1", "start": "2025-07-01", "end": "2025-07-03"}],
  "assignments":   [{"employee": "E1", "dept": "D2", "start": "2025-07-01", "end": "2025-07-03"}]
}
```

- `departments`：部门永久标识与历史起点的父部门（空串为根）。
- `parentChanges`：父部门变更，自 `day` 起（含当天）生效，整棵子树随之迁移。
- `tenures`：任职区间，`start`（入职日）计为在岗，`end`（离职日）不计；
  `open: true` 表示未离职。同一员工可有多段不重叠任职，可首尾相接。
- `assignments`：直接部门归属区间 `[start, end)`，每个在岗日恰好归属一个部门。

## 用法

```go
eng, err := attrition.LoadEngine("history.json") // 装载完整历史并构建索引
res := eng.Query("D1", attrition.NewDate(2025, 7, 1), attrition.NewDate(2025, 7, 6))
// res.PersonDays   人员日数
// res.Departures   离职次数
// res.AvgHeadcount 平均每日在岗人数（未舍入）
// res.Attrition    未年化流失率(%)，*float64；人员日数为零时为 nil
```

也可以先用 `LoadHistory`/`DecodeHistory` 读入 `History`，再用 `NewEngine` 构建；
`History.Save` 可把历史写回本地文件。

## 统计口径

- 查询区间为 `[start, end)`，公历自然日；查询范围是部门自身及**当天**组织树中的全部后代。
- 人员日数 = 区间内每天范围在岗人数之和；平均每日在岗人数 = 人员日数 / 区间自然日数。
- 离职次数按离职日落在区间内的任职结束事件统计，归属按**离职前一日**的直接部门
  及当日组织树判定（即使前一日不在查询区间内）。重新入职不撤销此前离职。
- 流失率 = 离职次数 / 平均每日在岗人数 × 100%，可超过 100%；人员日数为零时流失率为空。
- 调岗与部门迁移只改变相应日期的人数，不产生离职。

## 测试

```sh
go test ./...        # 全部测试（含规模测试）
go test -short ./... # 跳过规模测试
```

- `acceptance_test.go`：验收场景（入职/离职/子部门当日迁移，A 得 2 人员日、
  1 次离职、250% 流失率，B 全零且流失率为空）。
- `brute_test.go`：同日多处子树迁移、员工调岗、离职当天重新入职的组合历史，
  以及随机历史，均与独立逐日核算参考实现完全一致。
- `scale_test.go`：5000 员工、5000 部门、25000 段任职、100000 段归属、
  20000 次父级变更、50 年跨度；记录装载 + 300 次查询总耗时与堆内存，
  并抽查部分查询与逐日核算一致。本机实测：装载约 0.12s，300 次查询约 0.24s，
  堆内存约 21MiB，远低于 30s / 256MiB 的限制。
