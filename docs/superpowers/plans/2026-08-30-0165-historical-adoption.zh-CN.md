# 0165 历史已执行环境采纳实施计划

1. 增加失败测试，证明历史已执行 0165 且缺控制证据时 runner 被阻断，并覆盖采纳参数与后置 schema 校验。
2. 扩展 0165 控制表和 controller，写入 `adopted_existing`、外部备份 SHA、恢复边界与采纳时间。
3. 扩展 CLI 的 `adopt-existing` 动作，要求 request ID、外部备份 SHA 和停流确认。
4. 让普通 runner 只接受唯一、完整且后置 schema 仍有效的历史采纳记录。
5. 更新受控迁移手册，明确该流程不等同于迁移前备份或 verified。
6. 在隔离 MariaDB 先跑红绿测试，再对已完成恢复校验的本地命名卷执行采纳、普通迁移和状态复验。
