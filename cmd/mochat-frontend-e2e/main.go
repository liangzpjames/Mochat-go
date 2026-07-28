package main

import (
	"encoding/json"
	"log"
	"net/http"

	"jiyi/mochat-go/internal/frontend"
)

func main() {
	manifest, err := frontend.LoadMigrationManifest("web/apps/dashboard/src/migration-routes.json")
	if err != nil {
		log.Fatal(err)
	}

	apiNotFound := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	})
	handler := frontend.WrapDashboard(apiNotFound, frontend.DashboardConfig{
		DistDir: "web/apps/dashboard/dist",
	})
	handler = frontend.WrapLegacyDashboard(handler, frontend.LegacyDashboardConfig{
		DistDir:  "web/dashboard/dist",
		Manifest: manifest,
	})

	log.Fatal(http.ListenAndServe("127.0.0.1:4174", handler))
}
