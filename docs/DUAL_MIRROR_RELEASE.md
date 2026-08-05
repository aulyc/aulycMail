# 双发布源接入

本项目显式采用中央可选策略 `aulyc-dual-mirror-v1` `1.7.0`，Release Profile
仍是 `macos-arm64-app`。公开 GitHub 仓库 `aulyc/aulycMail` 是唯一源码与标签
权威，同时承担 GitHub Release 和更新清单分发；不得向 Gitee 推送源码。

中央渠道映射在 GitHub、Gitee 两端都使用公开的 `aulyc/aulycMail` 仓库身份，并
固定为 `macos-compact` / `dual-manifest`。GitHub 说明为简体中文在前、English
在后，Gitee 仅简体中文。两端 Release 都只允许同一份经 Profile 验证的正式 DMG；
DMG checksum 与 provenance checksum 仅作为本地 `verificationEvidence`，不上传
到 Release 或更新分支。

中央工具生成 Schema `dual-mirror-latest:2` 的同一份 `latest.json`。GitHub 清单
写入专用 `release-channel` 分支，不能推进源码 `main`；Gitee 清单写入 `main`。
最终 provenance 以同一字节写入两端的
`updates/<version>/<provenance-file>`，一经发布不可覆盖。DMR-009 的公共 GitHub
主仓拓扑适用于本项目；DMR-010 的 Obsidian Community 三文件附件合同因 Profile
为 `macos-arm64-app` 而不适用。旧 Release、Schema v1 清单和不可变发布计划保持
原样，新合同从下一个未发布稳定版本开始。

应用内更新器位于 `internal/updater`，Wails 桥接位于 `app/update.go`。它固定先读取
GitHub `latest.json`，失败后才读取 Gitee。它解析 Schema v2 中两端 raw
provenance URL，同时保留对历史 Schema v1 Release provenance URL 的只读兼容；
无论使用哪个来源，都执行相同的版本、build、Commit、Bundle ID、arm64、SHA-256、
provenance、Developer ID、Hardened Runtime、公证票据和 Gatekeeper 验证。只有
完整验证通过后，才会将新 App 暂存到 `/Applications` 同目录，由隔离 helper 在
当前进程正常退出后原子替换并重新启动；文件系统替换失败会回滚旧 App。

自动检查默认开启，在启动延迟后、应用重新激活、网络恢复以及周期到期时触发。
成功检查 24 小时内不重复，失败检查采用 6 小时退避；手动“检查更新”不受开关和
节流限制。自动检查只发现新版本，不会自动下载、安装或重启。设置修改继续遵循
该开关修改后自动保存，只有保存成功后才影响后台检查。

更新清单地址由中央渠道映射固定为：

- GitHub：`https://raw.githubusercontent.com/aulyc/aulycMail/release-channel/latest.json`
- Gitee：`https://gitee.com/aulyc/aulycMail/raw/main/latest.json`

首份 `latest.json` 必须由一次完整、明确授权的正式双镜像发布生成。发布源尚未
引导完成或两端均不可用时，更新器必须结束为可重试失败状态，不得无限 loading、
接受测试版产物或降低验证要求。

项目现有流程仍负责 DMG、最终 provenance、Changelog、签名、公证和发布产物
验证。
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

正式发布顺序固定为：两端 Release DMG、Gitee 版本化 provenance、Gitee
`latest.json`、GitHub 版本化 provenance、GitHub `latest.json`，最后独立回读
两端 DMG、版本化 provenance 与清单。任一步只完成一端时必须报告 `partial`，
不得把单端成功当作正式双镜像发布完成。

纯正式发版不写入 `/Applications`，完成时报告
`installationStatus: not-requested`。只有“正式发版安装”才在双端发布完整成功
后执行 `make install-release-dmg`。
