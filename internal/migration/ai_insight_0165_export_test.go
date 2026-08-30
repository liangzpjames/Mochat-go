package migration

// AIInsight0165BodyForServerForTest exposes only the immutable 0165
// compatibility transformation to the external-package integration contract.
func AIInsight0165BodyForServerForTest(body, serverVersion string) string {
	return aiInsight0165BodyForServer(body, serverVersion)
}
