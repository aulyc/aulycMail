# 双发布源接入

本项目显式采用中央可选策略 `aulyc-dual-mirror-v1` `1.1.0`，Release Profile
仍是 `macos-arm64-app`。私有 GitHub 仓库 `aulyc/aulycMail` 是唯一源码权威；
不得向 Gitee 推送源码。

中央渠道映射使用公开 `aulyc/aulycMail-releases` GitHub/Gitee 仓库。GitHub
说明为简体中文在前、English 在后，Gitee 仅简体中文；两端必须发布同一 DMG、
DMG checksum、最终 provenance、provenance checksum 和 `latest.json`。

当前应用没有应用内更新检查或下载器，`updater: N/A`。未来新增前必须先实现
GitHub manifest 优先、Gitee fallback，并对任一来源执行相同的版本、build、
Commit、Bundle ID、arm64、SHA-256、provenance、Developer ID、公证、安装身份
和 installed-runtime 验证。

项目现有流程仍负责 DMG、最终 provenance、Changelog、签名、公证和安装验证。
源码 branch/tag 已推送并回写最终 provenance 后：

```bash
bash tools/dual-mirror-release.sh prepare \
  --provenance dist/<final>.release-provenance.json \
  --notes-zh-cn dist/release-notes.zh-CN.md \
  --notes-en dist/release-notes.en.md \
  --output-dir dist/dual-mirror-<version>

bash tools/dual-mirror-release.sh preflight \
  --plan dist/dual-mirror-<version>/dual-mirror-plan.json
bash tools/dual-mirror-release.sh publish \
  --plan dist/dual-mirror-<version>/dual-mirror-plan.json \
  --state dist/dual-mirror-<version>/dual-mirror-state.json
bash tools/dual-mirror-release.sh verify \
  --plan dist/dual-mirror-<version>/dual-mirror-plan.json \
  --state dist/dual-mirror-<version>/dual-mirror-state.json
```

`prepare` 只生成项目内 staging；`preflight` 只读；只有明确授权的 `publish`
写远端。任一端失败都保留无凭据状态并只允许同计划向前重试。公开镜像仓库未创建
时预检必须失败，不得绕过或临时改用源码仓库。
