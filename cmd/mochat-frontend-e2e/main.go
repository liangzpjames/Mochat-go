package main

import (
	"encoding/json"
	"log"
	"net/http"

	"jiyi/mochat-go/internal/frontend"
)

func main() {
	apiNotFound := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	})
	handler := frontend.WrapDashboard(apiNotFound, frontend.DashboardConfig{
		DistDir: "web/apps/dashboard/dist",
	})
	handler = frontend.WrapApp(handler, frontend.AppConfig{
		DistDir:   "web/apps/operation/dist",
		MountPath: "/operation-app/",
	})
	handler = frontend.WrapApp(handler, frontend.AppConfig{
		DistDir:   "web/apps/sidebar/dist",
		MountPath: "/sidebar-app/",
	})

	log.Fatal(http.ListenAndServe("127.0.0.1:4174", handler))
}
