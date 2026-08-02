# Phase 3.2 查询性能与隔离证据

## 验证范围

验证 Overview 趋势查询按企业和日期范围过滤，Global Conversation 查询按企业、员工和会话对象过滤，并确认跨企业详情不会返回数据。

## 自动化验证

```text
go test ./internal/store -run 'CorpData|Conversation|WorkMessage' -count=1  PASS
go test ./internal/dashboard -run 'CorpData|Conversation|WorkMessage|AutoTag' -count=1  PASS
```

现有测试覆盖：企业范围、日期边界、全局会话员工权限、跨企业详情 404、消息分页和空结果处理。

## MariaDB EXPLAIN

执行环境：`mochat-go-desktop-mysql-1`，MariaDB 10.6。

Overview 趋势查询使用：

```sql
WHERE corp_id = ?
  AND date >= ?
  AND date < DATE_ADD(?, INTERVAL 1 DAY)
ORDER BY date ASC
LIMIT 31
```

结果：使用 `idx_mc_corp_day_data_corp_date (corp_id, date)`，访问类型为 `range`，Extra 为 `Using index condition`，不再出现全表扫描或 filesort。

Global Conversation 详情查询使用 `idx_mc_work_message_1_corp_seq`，访问类型为 `range`，并按企业和消息序号范围读取；十张分表沿用相同查询结构。

## 结论

本阶段未新增跨企业数据出口。Overview 趋势日期条件已改为可索引的半开区间，并通过迁移 `0103_phase3_2_query_indexes` 在桌面保留数据卷上应用。
