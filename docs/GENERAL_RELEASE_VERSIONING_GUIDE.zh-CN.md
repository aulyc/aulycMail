# 通用应用版本与发版流程规范

> 本文是一套可复用于桌面应用、移动应用、Web 服务、CLI 和插件项目的参考规范。
> 示例默认采用标准 SemVer、独立构建号、专用发布提交、不带 `v` 的不可移动 Git 标签。

## 一、目标

这套流程需要保证：

1. 每个安装包都能唯一对应版本号、构建号、Git Commit 和发布标签。
2. 测试包与正式包有清晰边界，不能把测试包改名后当作正式包。
3. 功能代码先提交，版本更新由发布流程自动完成。
4. 已发布版本、标签和安装包不可覆盖或复用。
5. 正式包完成平台要求的签名、公证、安全扫描或商店校验。
6. 安装后验证实际运行版本，而不是只检查源码中的版本字段。
7. 发布失败可以安全重试，不会无意义地消耗版本号或移动标签。

## 二、发布类型

建议明确区分三类构建。

| 类型 | 是否属于发版 | 版本示例 | 构建号 | Git 发布提交/标签 | 签名与公证 | 用途 |
|---|---:|---|---:|---|---|---|
| 本地开发安装 | 否 | `1.4.0-dev+a1b2c3d.dirty` | `0` | 不需要 | 本地 ad-hoc 或不签名 | 开发调试 |
| 测试发版 | 是 | `1.4.0-beta.1` | 正数递增 | 必须 | 自签名/ad-hoc，不公证 | 内部测试、验收 |
| 正式发版 | 是 | `1.4.0` | 正数递增 | 必须 | 平台正式签名、公证或商店签名 | 对外发布 |

### 2.1 本地开发安装

本地开发安装不是测试发版，允许：

- 使用 `-dev` 版本；
- 使用构建号 `0`；
- 显示 Commit SHA；
- 工作区有未提交修改时增加 `.dirty`；
- 不创建发布提交和标签；
- 不上传公证或应用商店。

示例：

```text
1.4.0-dev+a1b2c3d
1.4.0-dev+a1b2c3d.dirty
```

### 2.2 测试发版

测试发版属于正式的版本生命周期，必须具备独立身份：

- 使用 `alpha.N`、`beta.N` 或 `rc.N`；
- 每次发包都增加预发布序号或进入下一阶段；
- 分配新的全局构建号；
- 创建专用发布提交；
- 创建与版本完全一致的不可移动标签；
- 生成版本化安装包和 manifest；
- 允许使用自签名或 macOS ad-hoc 签名；
- 不得声称已通过平台公证或正式信任校验。

### 2.3 正式发版

正式发版必须：

- 使用无预发布后缀的稳定版本；
- 使用新的全局构建号；
- 从对应正式标签重新构建；
- 使用平台正式签名；
- 完成公证、商店验证、安全扫描或平台要求的其他步骤；
- 安装后核验实际运行版本；
- 发布 DMG/EXE/PKG/APK/IPA/镜像等产物及其 manifest。

## 三、版本号规则

正式版本采用标准 SemVer：

```text
MAJOR.MINOR.PATCH
主版本.次版本.修订版本
```

| 变更类型 | 更新位置 | 示例 |
|---|---:|---|
| 不兼容改动、重大架构变化 | MAJOR | `2.4.3 → 3.0.0` |
| 向后兼容的新功能 | MINOR | `2.4.3 → 2.5.0` |
| Bug、安全或性能修复 | PATCH | `2.4.3 → 2.4.4` |

版本段允许超过 9：

```text
2.0.9 → 2.0.10
```

不能使用十进制式进位：

```text
2.0.9 → 2.1.0  # 错误，除非确实包含 MINOR 级新功能
```

## 四、预发布版本规则

支持以下生命周期：

```text
MAJOR.MINOR.PATCH-dev
MAJOR.MINOR.PATCH-alpha.N
MAJOR.MINOR.PATCH-beta.N
MAJOR.MINOR.PATCH-rc.N
MAJOR.MINOR.PATCH
```

建议语义：

- `dev`：日常开发状态，不发包；
- `alpha`：功能尚未完整的内部验证；
- `beta`：功能基本完成，用于测试和验收；
- `rc`：发布候选，只接受阻断性问题修复；
- 无后缀：正式稳定版。

典型顺序：

```text
1.4.0-dev
1.4.0-beta.1
1.4.0-beta.2
1.4.0-rc.1
1.4.0
```

同一预发布版本不得覆盖：

```text
1.4.0-beta.1  # 第一次测试包
1.4.0-beta.2  # 修复后重新发包
```

## 五、自动版本升级规则

发布工具可以根据当前版本和提交内容自动选择目标版本。

### 5.1 生命周期优先

当项目已经处于 `dev`、`beta` 或 `rc` 阶段时，优先完成当前版本生命周期：

| 当前版本 | 测试发版 | 正式发版 |
|---|---|---|
| `1.4.0-dev` | `1.4.0-beta.1` | `1.4.0` |
| `1.4.0-beta.1` | `1.4.0-beta.2` | `1.4.0` |
| `1.4.0-rc.1` | `1.4.0-rc.2` | `1.4.0` |

不能把已测试的 `1.4.0-beta.1` 正式发版为 `1.4.1`；正式版应为同一基线的 `1.4.0`。

### 5.2 稳定版之后按提交判断

从最近稳定标签之后的提交中判断最高影响级别：

- `BREAKING CHANGE:` 或 `type!:` → MAJOR；
- `feat:` → MINOR；
- `fix:`、`perf:`、安全修复 → PATCH；
- 无法识别的已提交改动默认 PATCH；
- `docs`、`test`、`chore`、纯重构通常不单独触发发版。

同一次发布包含多种变更时采用最高级别。

对于历史提交不规范的项目，发布命令应允许显式覆盖：

```bash
make release-test RELEASE_BUMP=patch
make release-test RELEASE_BUMP=minor
make release-formal RELEASE_BUMP=major
```

## 六、独立构建号

应用版本和构建号分开维护：

```text
显示版本：1.4.0-beta.2
构建号：128
```

构建号规则：

1. 使用持续递增的整数。
2. 测试发版和正式发版都要分配新构建号。
3. 构建号不能重复或倒退。
4. 本地开发构建可使用 `0`，且不消耗公共构建号。
5. 构建号不混入正式 SemVer。
6. 未创建标签、未发布的失败版本可以安全重试并复用原构建号。
7. 已创建并公开的标签或产物必须使用新版本和新构建号修复。

## 七、唯一版本来源

每个项目必须指定一个唯一版本源，例如：

```json
{
  "version": "1.4.0-dev",
  "build": 127
}
```

其他文件全部由脚本同步：

- `package.json` / lock 文件；
- Wails、Electron、Tauri 或原生项目配置；
- macOS `Info.plist`；
- Windows manifest/resource；
- Android `versionName` / `versionCode`；
- iOS `MARKETING_VERSION` / `CURRENT_PROJECT_VERSION`；
- 插件 manifest；
- Docker image label；
- Web 前端运行时版本常量。

版本校验命令应在构建前执行，并在发现漂移时失败：

```bash
make version-check
make version-test
```

## 八、Commit 规则

普通开发提交不修改发布版本：

```text
feat: add task filtering
fix: correct keyboard focus
refactor: simplify task store
test: add release regression tests
```

发版前先提交所有功能代码，工作区必须干净。发布工具随后自动创建专用发布提交：

```text
chore: release 1.4.0-beta.1
chore: release 1.4.0
```

发布提交只包含：

- 唯一版本源更新；
- 派生版本文件；
- Changelog 版本标题和日期；
- 必须纳入版本控制的发布元数据。

功能实现不应混入发布提交。

## 九、Git 标签规则

建议所有项目统一采用不带 `v` 的 annotated tag：

```text
1.4.0-beta.1
1.4.0-rc.1
1.4.0
```

必须满足：

- 标签名称与应用版本完全一致；
- 标签指向对应发布提交；
- 标签在全部质量检查通过后创建；
- 测试版和正式版都使用独立标签；
- 已发布标签禁止移动、覆盖、删除后复用；
- 重跑发版时只允许验证并复用指向同一提交的现有标签；
- 标签不正确时直接失败，不能自动强制覆盖。

## 十、签名与公证矩阵

| 平台/产物 | 测试发版 | 正式发版 |
|---|---|---|
| macOS App/DMG | ad-hoc 或内部自签名；不公证 | Developer ID + Hardened Runtime + Notarization + Staple + Gatekeeper |
| Windows EXE/MSI | 内部测试证书或不签名 | 组织代码签名证书；必要时时间戳和 SmartScreen 信誉 |
| iOS | Development/Ad Hoc/TestFlight | Distribution/App Store 签名 |
| Android | 独立测试 keystore | 正式 release keystore/Play App Signing |
| Web 服务 | 测试环境镜像和部署身份 | 生产镜像签名、制品证明、生产发布审批 |
| CLI/插件 | 测试渠道包，可使用预发布版本 | 正式 registry/repository 包和校验和/签名 |

### macOS 测试发版

ad-hoc 签名示例：

```bash
codesign --force --deep --sign - Example.app
codesign --force --sign - Example.dmg
```

注意：

- ad-hoc 签名只能证明文件内部一致性，不能证明发布者身份；
- ad-hoc 包不会通过正式 Gatekeeper 信任评估；
- manifest 必须明确写入 `signatureType: "adhoc"`；
- 正式安装器默认应拒绝 ad-hoc 包；
- 只有显式测试入口可以允许安装。

### macOS 正式发版

正式流程至少包括：

```text
Developer ID 签名
→ codesign 校验
→ 生成 DMG/PKG
→ DMG/PKG 签名
→ notarytool 上传并等待 Accepted
→ stapler staple
→ stapler validate
→ spctl Gatekeeper 校验
```

签名证书名可以通过参数传入，但 Apple 凭据必须保存在 Keychain 或安全的 CI Secret 中，禁止写入仓库和 manifest。

## 十一、测试发版流程

推荐提供一个可恢复的一键入口：

```bash
make release-test
```

内部顺序：

```text
确认功能代码已 commit、工作区干净
→ 自动选择 beta.N/rc.N
→ 递增构建号
→ 将 Changelog 的 Unreleased 内容归档到新版本
→ 同步所有版本文件
→ 创建 chore: release <version>
→ 运行完整质量检查和生产构建
→ 创建或验证 annotated tag
→ 从标签重新构建
→ ad-hoc/自签名 App 和安装包
→ 生成 manifest
→ 安装测试包
→ 验证实际运行版本、构建号、Commit 和签名类型
```

测试发版不能上传 Apple 公证，也不能在报告中写成“已公证正式包”。

## 十二、正式发版流程

推荐入口：

```bash
make release-formal \
  SIGN_IDENTITY="Developer ID Application: ..." \
  NOTARY_PROFILE=project-notary
```

内部顺序：

```text
确认正式签名和公证凭据可用
→ 确认功能代码已 commit、工作区干净
→ 将 dev/beta/rc 提升为同一基线的稳定版
→ 递增构建号
→ 更新 Changelog 和派生版本文件
→ 创建正式发布提交
→ 完整质量检查和生产构建
→ 创建或验证正式标签
→ 从正式标签重新构建
→ Developer ID/平台正式签名
→ 公证、商店校验或安全扫描
→ 生成 manifest
→ 安装正式包
→ 验证真实运行版本和平台信任状态
```

正式版本不能直接复用或重命名测试安装包。

## 十三、质量门禁

测试版和正式版都应执行：

- 唯一版本源及派生文件一致性检查；
- SemVer 和构建号格式检查；
- Changelog 精确版本标题检查；
- Git 工作区干净检查；
- 发布提交检查；
- formatter/lint；
- typecheck；
- 单元测试；
- 集成测试；
- 前端生产构建；
- 后端/原生生产构建；
- tag 类型、名称和目标提交检查；
- 安装包元数据检查；
- 安装后运行版本检查。

正式版额外执行：

- 正式代码签名检查；
- 公证或应用商店校验；
- Gatekeeper/SmartScreen/平台安全验证；
- 必要的 SAST、依赖漏洞和恶意软件扫描。

## 十四、发布产物和 manifest

建议产物命名：

```text
example-1.4.0-beta.2-build.128.dmg
example-1.4.0-beta.2-build.128.manifest.json
```

manifest 示例：

```json
{
  "application": "example",
  "version": "1.4.0-beta.2",
  "buildNumber": 128,
  "tag": "1.4.0-beta.2",
  "signatureType": "adhoc",
  "commit": "0123456789abcdef0123456789abcdef01234567",
  "sha256": "...",
  "architecture": "arm64",
  "notarizationSubmissionId": null,
  "builtAt": "2026-07-14T01:20:46Z",
  "artifact": "example-1.4.0-beta.2-build.128.dmg"
}
```

正式 macOS 包应使用：

```json
{
  "signatureType": "developer-id",
  "notarizationSubmissionId": "Apple submission UUID"
}
```

manifest 中不得包含：

- 密码；
- 私钥；
- API Token；
- Keychain 凭据；
- 用户数据和配置内容。

## 十五、安装验证

安装完成后至少验证：

1. 安装包 SHA-256 与 manifest 一致。
2. 安装包文件名与 manifest 中的 artifact 一致。
3. App/二进制签名符合目标渠道。
4. Bundle/manifest/运行时版本完全一致。
5. 构建号一致。
6. Commit SHA 一致。
7. CPU 架构符合发布目标。
8. 正式包通过平台信任校验。
9. 实际运行的二进制来自预期安装目录。

macOS 可检查：

```bash
codesign --verify --deep --strict --verbose=2 Example.app
codesign -dv --verbose=4 Example.app
spctl -a -vvv -t exec Example.app
xcrun stapler validate Example.dmg
Example.app/Contents/MacOS/example --version
```

## 十六、失败与重试

### 标签创建前失败

如果质量检查失败且标签尚未创建：

- 修复问题并提交；
- 复用当前尚未发布的版本号和构建号；
- 将新增 Unreleased 说明合并进当前版本；
- 创建新的发布提交；
- 重新运行检查。

### 标签创建后失败

如果标签或安装包已经公开：

- 禁止移动标签；
- 禁止覆盖旧安装包；
- 发布新的 `beta.N`、`rc.N` 或 PATCH 版本；
- 分配新构建号。

### 重跑同一已验证产物

如果标签已存在且仍指向当前发布提交：

- 可以重新验证并从同一标签构建相同内容；
- 不允许让标签指向其他提交；
- 重新生成的公开产物必须与既有内容一致，否则使用新版本。

## 十七、安全要求

1. 发布脚本不得打印签名密码、Token 或私钥。
2. 公证和商店凭据存入 Keychain、CI Secret 或专用凭据服务。
3. 正式签名身份必须可配置，但不能被测试渠道偷偷替换。
4. 安装器必须显式区分 ad-hoc 与正式签名。
5. 正式安装路径不能接受通过修改 manifest 伪装的测试包。
6. 用户数据库、设置、邮件、凭据等不得包含在应用覆盖列表中。
7. 发布工具默认 fail closed：无法确认时拒绝发版。

## 十八、推荐命令接口

不同项目可以统一提供以下命令：

```text
make build                    本地生产模式构建，不发版
make install                  本地开发安装，不发版
make version-check            校验版本文件一致性
make version-test             测试版本与发版逻辑
make prepare-test-release     只准备测试版元数据和发布提交
make prepare-formal-release   只准备正式版元数据和发布提交
make release-check            执行全部公共质量门禁
make release-tag              创建或验证不可移动标签
make release-test             一键测试发版、安装和验证
make release-formal           一键正式发版、安装和验证
```

高级项目还可以拆分：

```text
make test-release-package
make formal-release-package
make install-test-release
make install-formal-release
```

## 十九、迁移现有项目

迁移步骤：

1. 盘点所有版本字段和构建号字段。
2. 确定当前最后一个真实正式版本。
3. 新建唯一版本源。
4. 编写同步和漂移检查脚本。
5. 把当前开发版本设置为下一个目标的 `-dev`。
6. 初始化全局构建号，不得低于已发布平台记录。
7. 统一测试版和正式版标签格式。
8. 接入自动 Changelog 和发布提交。
9. 拆分测试签名和正式签名路径。
10. 增加 manifest 与安装后版本验证。
11. 对版本选择、重试、标签和签名分支编写自动化测试。
12. 完成一次真实测试发版演练。
13. 完成一次真实正式签名/公证演练。

已有公开标签不要重命名或移动。新规范可以从下一个版本开始执行。

## 二十、禁止事项

- 不要手工逐个修改多个版本文件。
- 不要把 `2.0.9` 按十进制思维升级为 `2.1.0`。
- 不要复用同一个测试版本覆盖不同内容。
- 不要在工作区未提交时发版。
- 不要把功能代码混入发布提交。
- 不要创建轻量标签替代 annotated release tag。
- 不要强制移动已发布标签。
- 不要将测试包改名为正式包。
- 不要把 ad-hoc 签名描述为平台正式签名。
- 不要跳过安装后的真实版本核验。
- 不要在发布日志和 manifest 中泄露凭据。

## 二十一、最终检查清单

### 所有发版

- [ ] 功能代码已经 commit。
- [ ] Git 工作区干净。
- [ ] 自动版本级别正确。
- [ ] 版本文件同步一致。
- [ ] 构建号已增加且未重复。
- [ ] Changelog 包含精确版本标题。
- [ ] 发布提交只包含发布元数据。
- [ ] 所有质量门禁通过。
- [ ] annotated tag 与版本一致并指向发布提交。
- [ ] 产物文件名包含版本和构建号。
- [ ] manifest 信息完整、SHA-256 正确。
- [ ] 安装后运行版本、构建号和 Commit 正确。
- [ ] 发布结束后工作区干净。

### 测试发版

- [ ] 使用独立预发布版本。
- [ ] manifest 明确标记 `adhoc`/测试签名。
- [ ] 没有上传正式公证或商店渠道。
- [ ] 没有对外宣称通过平台正式信任校验。

### 正式发版

- [ ] 使用稳定版本。
- [ ] 从正式标签重新构建。
- [ ] 使用正式平台签名身份。
- [ ] 公证、商店校验或安全扫描成功。
- [ ] Gatekeeper/平台信任验证通过。
- [ ] 正式产物和 manifest 一同发布。

---

采用这套规范后，任何一个公开安装包都应能反向回答四个问题：

```text
它是什么版本？
它是第几个构建？
它来自哪个 Commit 和标签？
它通过了哪一级签名与平台验证？
```
