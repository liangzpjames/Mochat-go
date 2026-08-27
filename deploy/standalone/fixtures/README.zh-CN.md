# 本地会话存档模拟文件

老大可将 PNG、WAV、MP4、PDF 等不含真实业务数据的文件放在此目录。`archive-simulator` 容器以只读方式挂载本目录到 `/fixtures/input`；模拟器只接受不超过 20 MiB 的普通文件。

所有模拟数据集名称必须以 `MOCHAT-LOCAL-SIM-` 开头，避免与真实业务数据混淆。
