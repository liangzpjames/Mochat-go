# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e
FROM debian:bookworm-slim@sha256:88200866dfff7ea7f5cbcb6ec7c8a701889efe6fe859fe64d6990e4b07ea4171

ARG WECOM_SDK_URL=https://wwcdn.weixin.qq.com/node/wwcomm/sdk_x86_v3_20250205.tgz
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates libssl3 libstdc++6 \
    && rm -rf /var/lib/apt/lists/*
ADD --checksum=sha256:afa8c017da2994ad2215933f2fcc6042d40d935663ad42d6e1e9d7716652f0d8 ${WECOM_SDK_URL} /tmp/wecom-sdk.tgz
RUN mkdir -p /opt/wecom-sdk \
    && tar -xzf /tmp/wecom-sdk.tgz -C /opt/wecom-sdk --strip-components=1 C_sdk \
    && test "$(sha256sum /tmp/wecom-sdk.tgz | cut -d' ' -f1)" = "afa8c017da2994ad2215933f2fcc6042d40d935663ad42d6e1e9d7716652f0d8" \
    && rm -f /tmp/wecom-sdk.tgz

COPY .tmp-wecom-archive-demo-build/wecom-archive-demo /usr/local/bin/wecom-archive-demo
COPY deploy/wecom-archive-demo/entrypoint.sh /usr/local/bin/wecom-archive-demo-entrypoint
RUN sed -i 's/\r$//' /usr/local/bin/wecom-archive-demo-entrypoint \
    && chmod 0755 /usr/local/bin/wecom-archive-demo /usr/local/bin/wecom-archive-demo-entrypoint
ENV WECOM_FINANCE_SDK_PATH=/opt/wecom-sdk/libWeWorkFinanceSdk_C.so
USER 65532:65532
EXPOSE 8080 9091
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 CMD ["/usr/local/bin/wecom-archive-demo", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/wecom-archive-demo-entrypoint"]
CMD ["serve"]
