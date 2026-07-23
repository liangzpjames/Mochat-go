#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  cat <<'EOF'
Usage: ./scripts/init_production_evidence_pack.sh

初始化生产证据模板文件，默认输出到 docs/evidence/production/。

环境变量：
  MOCHAT_PRODUCTION_EVIDENCE_DIR
      输出目录，默认 docs/evidence/production。
  MOCHAT_PRODUCTION_EVIDENCE_FORCE
      设为 1 时覆盖已有模板目标文件。

生成的文件带有 MOCHAT_EVIDENCE_TEMPLATE_DO_NOT_USE 标记。
真实联调完成后必须删除该标记并填写实际证据，否则 production_evidence_check 会拒绝。
EOF
  exit 0
fi

OUT_DIR="${MOCHAT_PRODUCTION_EVIDENCE_DIR:-docs/evidence/production}"
FORCE="${MOCHAT_PRODUCTION_EVIDENCE_FORCE:-0}"
MARKER="MOCHAT_EVIDENCE_TEMPLATE_DO_NOT_USE"

case "$FORCE" in
  0|1)
    ;;
  *)
    echo "unknown MOCHAT_PRODUCTION_EVIDENCE_FORCE=$FORCE" >&2
    echo "valid values: 0, 1" >&2
    exit 2
    ;;
esac

mkdir -p "$OUT_DIR"

write_file() {
  local path="$1"
  if [ -e "$path" ] && [ "$FORCE" != "1" ]; then
    printf 'skip existing %s\n' "$path"
    return 0
  fi
  mkdir -p "$(dirname "$path")"
  cat >"$path"
  printf 'write %s\n' "$path"
}

write_file "$OUT_DIR/mysql57-amd64.log" <<EOF
$MARKER

# MySQL 5.7 amd64 真实容器门禁证据

状态：待验收
源码指纹：

请在 amd64/x86_64 CI 或等价环境执行：

env -u GOROOT ./scripts/ci_mysql57_amd64.sh 2>&1 | tee docs/evidence/production/mysql57-amd64.log

真实有效日志必须包含 mysql57 amd64 CI gate passed 或 mysql 5.7 schema migration smoke passed。
真实有效日志还必须包含当前 scripts/source_fingerprint.py 生成的 源码指纹。
不要使用本机 arm64 skip 日志。
EOF

write_file "$OUT_DIR/real-wecom.md" <<EOF
# 真实企业微信账号联调证据

$MARKER

- 状态：待验收
- 环境：
- 账号：
- 执行人：
- 执行时间：
- 源码指纹：
- 结论：

## 必填核验项

- 授权：
- 回调解密：
- 通讯录：
- 客户：
- 客户群：
- 标签：
- 素材：
- 企微应用消息链路：

## 证据记录

- 请求或操作路径：
- 关键响应摘要：
- 失败重试或异常路径：
- 截图 / 日志 / 工单引用：
EOF

write_file "$OUT_DIR/real-wechat-open.md" <<EOF
# 真实微信开放平台联调证据

$MARKER

- 状态：待验收
- 环境：
- 第三方平台：
- 执行人：
- 执行时间：
- 源码指纹：
- 结论：

## 必填核验项

- ticket：
- 预授权：
- 授权回跳：
- 资料回填：
- 取消授权：
- 消息回调：

## 证据记录

- 请求或操作路径：
- 关键响应摘要：
- 失败重试或异常路径：
- 截图 / 日志 / 工单引用：
EOF

write_file "$OUT_DIR/real-saas-tenants.md" <<EOF
# 真实 SaaS 多租户数据回归证据

$MARKER

- 状态：待验收
- 环境：
- 租户 A：
- 租户 B：
- 执行人：
- 执行时间：
- 源码指纹：
- 结论：

## 必填核验项

- 租户：
- 菜单权限：
- 企业归属：
- 资源额度：
- 上传账本：
- 异步任务：
- 告警：

## 证据记录

- 跨租户访问隔离：
- 用量刷新和额度拦截：
- 上传账本与回收：
- 异步任务执行记录：
- 告警通知和处置：
EOF

write_file "$OUT_DIR/prod-frontend.md" <<EOF
# 生产前端浏览器回归证据

$MARKER

- 状态：待验收
- 环境：
- 构建版本：
- 执行人：
- 执行时间：
- 源码指纹：
- 结论：

## 必填核验项

- dashboard：
- sidebar：
- operation：
- 生产：
- 异常：

## 证据记录

- 主要业务路径：
- 异常路径：
- 控制台错误：
- 网络请求错误：
- 截图 / 录像 / 报告引用：
EOF

write_file "$OUT_DIR/stability.md" <<EOF
# 短稳/外部监控稳定性记录

$MARKER

- 状态：待验收
- 环境：
- 时间范围：
- 执行人：
- 源码指纹：
- 结论：

## 必填核验项

- 短稳回归：
- 结论：

## 证据记录

- 健康检查：
- 路由覆盖：
- 错误率：
- 资源使用：
- 日志 / 监控引用：
EOF

write_file "$OUT_DIR/env.production-evidence.example" <<EOF
MOCHAT_EVIDENCE_MYSQL57_AMD64=$OUT_DIR/mysql57-amd64.log
MOCHAT_EVIDENCE_REAL_WECOM=$OUT_DIR/real-wecom.md
MOCHAT_EVIDENCE_REAL_WECHAT_OPEN=$OUT_DIR/real-wechat-open.md
MOCHAT_EVIDENCE_REAL_SAAS_TENANTS=$OUT_DIR/real-saas-tenants.md
MOCHAT_EVIDENCE_PROD_FRONTEND=$OUT_DIR/prod-frontend.md
MOCHAT_EVIDENCE_STABILITY=$OUT_DIR/stability.md
EOF

cat <<EOF

生产证据模板已初始化到 $OUT_DIR

下一步：
1. 用真实联调和 CI 结果替换模板内容。
2. 删除每个证据文件里的 $MARKER 标记。
3. 执行：

   set -a
   . $OUT_DIR/env.production-evidence.example
   set +a
   ./scripts/production_candidate_gate.sh
EOF
