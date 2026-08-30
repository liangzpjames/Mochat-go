FROM node:24.20.0-alpine@sha256:e67514e5d0f6c46656005e1b693b2ec9d52e80b641307de684d4a015ba7a4eaf AS frontend-build

ARG NPM_REGISTRY=https://registry.npmmirror.com
ENV COREPACK_NPM_REGISTRY=${NPM_REGISTRY}

WORKDIR /src

RUN corepack enable \
	&& corepack prepare pnpm@11.17.0 --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web ./web
RUN pnpm config set registry "${NPM_REGISTRY}" \
	&& pnpm install --frozen-lockfile \
	&& pnpm --filter @mochat/dashboard build \
	&& pnpm --filter @mochat/sidebar build \
	&& pnpm --filter @mochat/operation build \
	&& pnpm --filter @mochat/saas-admin build

FROM golang:1.26.7-alpine@sha256:28d89ee9cc0ff9fec75c82ca201e6bf7fdf9a679d4b7b24dfa04f2bb766bb468 AS build

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

WORKDIR /src

RUN sed -i 's#https://dl-cdn.alpinelinux.org#https://mirrors.aliyun.com#g; s#http://dl-cdn.alpinelinux.org#https://mirrors.aliyun.com#g' /etc/apk/repositories \
	&& apk add --no-cache python3

COPY go.mod go.sum ./
RUN go mod download

COPY .dockerignore Dockerfile ./
COPY .github ./.github
COPY cmd ./cmd
COPY internal ./internal
COPY scripts ./scripts
COPY web ./web
COPY deploy ./deploy
COPY LICENSE NOTICE.md SOURCE_OFFER.md MODIFICATIONS.md THIRD_PARTY_NOTICES.md ./

RUN SOURCE_FINGERPRINT="$(python3 scripts/source_fingerprint.py | python3 -c 'import json,sys; print(json.load(sys.stdin)["fingerprint"])')" \
	&& test "${#SOURCE_FINGERPRINT}" -eq 64 \
	&& BUILD_LDFLAGS="-X jiyi/mochat-go/internal/buildinfo.SourceFingerprint=${SOURCE_FINGERPRINT}" \
	&& CGO_ENABLED=0 GOOS=linux go build -ldflags "$BUILD_LDFLAGS" -o /out/mochat-go ./cmd/mochat-go \
	&& CGO_ENABLED=0 GOOS=linux go build -ldflags "$BUILD_LDFLAGS" -o /out/mochat-migrate ./cmd/mochat-migrate \
	&& CGO_ENABLED=0 GOOS=linux go build -ldflags "$BUILD_LDFLAGS" -o /out/mochat-callback-legacy-cutover ./cmd/mochat-callback-legacy-cutover \
	&& CGO_ENABLED=0 GOOS=linux go build -ldflags "$BUILD_LDFLAGS" -o /out/mochat-bootstrap ./cmd/mochat-bootstrap \
	&& CGO_ENABLED=0 GOOS=linux go build -ldflags "$BUILD_LDFLAGS" -o /out/mochat-saas-maintenance ./cmd/mochat-saas-maintenance \
	&& CGO_ENABLED=0 GOOS=linux go build -ldflags "$BUILD_LDFLAGS" -o /out/mochat-identity-preflight ./cmd/mochat-identity-preflight \
	&& CGO_ENABLED=0 GOOS=linux go build -ldflags "$BUILD_LDFLAGS" -o /out/mochat-identity-migrate ./cmd/mochat-identity-migrate \
	&& CGO_ENABLED=0 GOOS=linux go build -ldflags "$BUILD_LDFLAGS" -o /out/mochat-archive-simulator ./cmd/mochat-archive-simulator \
	&& CGO_ENABLED=0 GOOS=linux go build -ldflags "$BUILD_LDFLAGS" -o /out/mochat-archive-bridge ./cmd/mochat-archive-bridge \
	&& CGO_ENABLED=0 GOOS=linux go build -ldflags "$BUILD_LDFLAGS" -o /out/mochat-archive-acceptance ./cmd/mochat-archive-acceptance \
	&& CGO_ENABLED=0 GOOS=linux go build -ldflags "$BUILD_LDFLAGS" -o /out/mochat-ai-insight-0165 ./cmd/mochat-ai-insight-0165

FROM alpine:3.22.5@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce

RUN sed -i 's#https\?://dl-cdn.alpinelinux.org#https://mirrors.aliyun.com#g' /etc/apk/repositories \
	&& apk add --no-cache mariadb-client tzdata \
	&& addgroup -S mochat \
	&& adduser -S -G mochat -u 10001 mochat

WORKDIR /app

COPY --from=build /out/mochat-go /usr/local/bin/mochat-go
COPY --from=build /out/mochat-migrate /usr/local/bin/mochat-migrate
COPY --from=build /out/mochat-callback-legacy-cutover /usr/local/bin/mochat-callback-legacy-cutover
COPY --from=build /out/mochat-bootstrap /usr/local/bin/mochat-bootstrap
COPY --from=build /out/mochat-saas-maintenance /usr/local/bin/mochat-saas-maintenance
COPY --from=build /out/mochat-identity-preflight /usr/local/bin/mochat-identity-preflight
COPY --from=build /out/mochat-identity-migrate /usr/local/bin/mochat-identity-migrate
COPY --from=build /out/mochat-archive-simulator /usr/local/bin/mochat-archive-simulator
COPY --from=build /out/mochat-archive-bridge /usr/local/bin/mochat-archive-bridge
COPY --from=build /out/mochat-archive-acceptance /usr/local/bin/mochat-archive-acceptance
COPY --from=build /out/mochat-ai-insight-0165 /usr/local/bin/mochat-ai-insight-0165
COPY --from=build /src/web ./web
COPY --from=frontend-build /src/web/apps/dashboard/dist ./web/apps/dashboard/dist
COPY --from=frontend-build /src/web/apps/sidebar/dist ./web/apps/sidebar/dist
COPY --from=frontend-build /src/web/apps/operation/dist ./web/apps/operation/dist
COPY --from=frontend-build /src/web/apps/saas-admin/dist ./web/apps/saas-admin/dist
COPY --from=build /src/deploy/standalone ./deploy/standalone
COPY --from=build /src/LICENSE /src/NOTICE.md /src/SOURCE_OFFER.md /src/MODIFICATIONS.md /src/THIRD_PARTY_NOTICES.md ./

RUN mkdir -p /app/storage/upload/static /app/storage/backups /app/audit-anchors \
	&& chown -R mochat:mochat /app/storage /app/audit-anchors

USER mochat

ENV MOCHAT_GO_STANDALONE=1 \
	MOCHAT_GO_ENABLE_FRONTEND_SERVERS=1 \
	MOCHAT_GO_ADDR=0.0.0.0:8080 \
	TZ=Asia/Shanghai \
	MOCHAT_FILE_STORAGE_ROOT=/app/storage/upload/static

EXPOSE 8080 8081 8082

ENTRYPOINT ["mochat-go"]
