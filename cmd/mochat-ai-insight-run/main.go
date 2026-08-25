// Command mochat-ai-insight-run is intentionally disabled. A process-level
// backfill cannot establish an authenticated tenant/corp principal and must
// not construct a global environment-backed model provider.
package main

import (
	"log"
)

func main() {
	log.Fatal("mochat-ai-insight-run 已禁用：请通过已认证的 AI 洞察工作区按租户和企业范围执行")
}
