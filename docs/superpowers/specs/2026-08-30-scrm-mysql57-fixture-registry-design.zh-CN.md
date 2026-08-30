# SCRM MySQL 5.7 集成夹具统一设计

## 问题

SCRM 客户标签集成测试先通过 `mysqlIntegrationDB` 和
`testharness.ApplyLatest` 建立最新生产 schema，随后又读取并直接执行
`0109_scrm_customer_tag_parity.up.sql`。这条第二迁移路径绕开了生产
`migration.Runner` 的 MySQL 5.7 兼容转换。

MariaDB 接受重复的 `ADD ... IF NOT EXISTS`，因此历史门禁没有暴露问题；
MySQL 5.7 不支持该语法，导致同一份测试在目标兼容数据库失败。

## 决策

1. 客户标签集成测试只使用共享的 `integrationRepository`；
   该入口已经通过随机隔离库、生产迁移注册表和受控迁移证据建立最新 schema。
2. 删除测试内读取、拆分和执行 0109 原始 SQL 的旁路。
3. 将客户标签场景纳入 Phase 3.2 固定集成测试清单，并增加静态断言，
   禁止夹具重新引入原始迁移文件执行。
4. 将 CorpData fixture 的静态断言同步到当前带业务时区的
   `newCurrentStoreIntegrationDBWithLocation` 正式入口，避免门禁继续校验已退役调用。

## 验收

- 新门禁测试先因当前 9 个场景和原始 SQL 旁路失败。
- 修复后门禁测试通过。
- MySQL 5.7/amd64 与 MariaDB 10.6 的客户标签真实集成测试均通过。
- 全量 SCRM MySQL 集成包通过，且测试库仍由正式注册表建立。
