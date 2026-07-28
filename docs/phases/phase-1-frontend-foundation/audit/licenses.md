# Legacy frontend license audit

The repository root [LICENSE](../../../../LICENSE) is GPL-3.0. The React administrative source also carries [NUWA-APACHE-2.0.txt](../../../../web/apps/saas-admin/NUWA-APACHE-2.0.txt), the Apache-2.0 license text. Neither file establishes provenance for assets restored under `web/legacy`.

For every dependency in the restored `dashboard/package.json`, `sidebar/package.json`, and `operation/package.json`, verify the resolved package license before a replacement is shipped: run `npm view <package>@<resolved-version> license repository.url`, retain the package-lock/yarn-lock evidence if present, and compare its notice requirements against the destination application's distribution license. The dependency matrix records the migration decision; it is not a substitute for a package-specific license review.

The source and provenance of all files recorded in `assets.csv` is not established by the backup branch: dashboard `src/assets/{1..14}.jpeg`, `*.png`, `*.jpg`, the `systemHomePage/` image set, `logo.svg`, `avatar-default.svg`, and `avatar-room-default.svg`; sidebar `src/assets/{check,excel,file,pdf,word,zip}.png`; operation `src/static/images/*`; and any font loaded by legacy CSS or a transitive package. These assets are **blocked** and must not be copied into a React application until their original source and license are documented.

The PHP `api-server` tree may be read only through `git show origin/backup/pre-phase0-main-20260723:api-server/<path>` to compare request and response behavior. It is not restored as a runtime dependency and never appears in `go_evidence`; `-` in that column means that no matching Go handler/store/test path has been found yet.
