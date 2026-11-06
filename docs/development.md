# 维护导览

本项目为 Go/MyGo 原生 Windows 应用。根包为可执行程序；较小的文件按领域拆分，共享明确的模型，而不是按文件创建无意义的公共 API。第三方 XLS 兼容副本的版权和原始注释保留，变更记录在 third_party/xls/QUESORA_CHANGES.md。

## 入口与职责

| 领域 | 主要文件 | 约束 |
| --- | --- | --- |
| 启动/原生视图 | main.go、view.go、quiz.go、native_dialogs.go | 快照启动不打开用户数据库；模态窗口隔离练习快捷键 |
| 题库导入 | importer.go、import_format.go、import_schema.go、import_xls.go | 先识别容器，再识别表头说明；未知或冲突行报告来源，不伪造答案 |
| 练习 | model.go、practice_session.go、practice_repeat.go | 最近提交与本次访问分开，答案始终使用原始选项索引 |
| 考试/PDF | exam.go、exam_setup*.go、exam_view.go、exam_pdf.go | 固定题序和映射，独立于练习；重要题失误综合分归零但保留实际正确数 |
| 数据/设置 | storage.go、practice_store.go、data_*.go、preferences.go | 导入原子提交、单连接、串行任务、一致性备份、失败保留原文件 |
| 原生资源 | fonts.go、icons.go、important_animation.go、frosted.go | 字体/标志内嵌；动画仅在活动时请求下一帧 |
| 打包/发布 | build-preview.py、qa/、packaging/、.github/workflows/windows.yml | 发布白名单隔离用户库；固定安装 AppId；先测试后发布 |

## 模型和线程

question.Answer 和 response.Choice 的第 i 位表示原始 Options[i]。OptionOrder 是显示位置到原始索引的排列；历史/PDF 使用保存的映射。不要为了选项打乱直接重排题目内容。

app 字段在 UI 线程修改。后台先捕获不可变数据或持久服务，磁盘操作交给 libraryJobs，结果经 publish 返回。不要捕获跨帧 ui.Element；构建可能在同一帧重复执行，因此渲染函数不得无条件发起 I/O。数据库单连接使用时，查询游标必须关闭再发起下一次查询。

## 解析规则扩展

先添加最小 CSV/XLSX 回归样例，说明原始单元格、期望标准题型/选项/答案和警告行。表头别名在 headerField，顶部说明在 schemaAt，答案解释在 normalizedChoice。增加规则前检查“选项序号”和“选项内容”同时合法但含义不同的情况，优先采用用户表格的显式说明。保持导入源文件只读。

## 数据保护

questions/practice 通过外键关联；settings 保存版本化偏好和历史快照。导入全部工作表使用一个事务，同名拒绝并回滚整个批次。快照使用 VACUUM INTO，包括已提交 WAL，完整性检查成功后发布目标文件；不要用普通复制替代活动 SQLite 备份。

安装版使用用户 LocalAppData，便携版保留 EXE 旁 data。安装器 AppId 固定，卸载不删除用户数据。修改迁移/恢复逻辑时必须覆盖已有目标、损坏源、WAL、失败回滚和保留原数据的测试。

## 验证与提交

在 Windows、CGO_ENABLED=0 下运行 go test ./... 和 go tool mygo vet .；python build-preview.py 会在隔离目录执行完整构建、资源和离屏检查。仅注释批次应保持去注释后的 Go AST 不变，不混入业务改动。

UI 回归使用 ui.NewTester 覆盖焦点、按键、重复帧和最小窗口；截图用于查看布局。实际窗口、Mica、输入法和真实题库验收由用户执行，不能用离屏通过替代。

构建安装器用 python build-preview.py --installer。qa/installer_smoke.py 仅允许在临时 GitHub Windows runner 执行，禁止用测试安装器操作开发者机器上的真实安装/数据库。已发布标签不可改写；修复应新建版本。

## 注释风格

类型与关键函数注释以名称开头，说明职责、输入含义、返回/失败约束及副作用。复杂算法说明选择原因和歧义边界，异步回调说明数据所有权。不要逐行复述语法、给简单绘制辅助函数堆砌注释，或为第三方未支持的功能写“已支持”。新增代码同步更新注释，过时说明随实现一并修改。

## 新增维护边界

数据恢复在 data_restore.go/restore_ui.go：先生成只读快照验证，再保留恢复前备份，最后事务替换四张业务表，不在活动连接存在时直接替换 .db 文件。

更新在 update.go/update_download.go/update_ui.go/update_runtime.go：公开元数据只决定候选，固定仓库/产物名/HTTPS 重定向名单和 SHA-256 均通过才允许安装；测试注入下载/进程替身。实际安装在用户确认后交给可见安装器，不在自动化测试中启动真实安装。版本、来源 URL、取消、限流和缓存被修改均需回归。

resources/icon-design.json 记录图标导出参数。qa/icons 使用用户提供的高分辨率原图，所有尺寸和标题栏均保留完整题卡、A/B/C/D、Q 气泡、颜色和布局，只等比缩放并裁切外部圆角，不重绘或按尺寸更换标志。执行 JSON 中的 export.command 同时重建 icon.png、app-icon.png 和 icon-frames/，不要手动缩小已有 PNG。build-preview.py 在隔离目录重新生成全部帧，qa/winres 使用 NewIconFromImages 直接嵌入，不进行二次缩放；qa/verify_executable.py 对实际 PE 帧与导出 PNG 逐字节比较。

AI 维护边界：ai_provider.go 绑定五厂商官方 HTTPS 来源，禁止认证请求重定向，不记录服务原始错误或密钥；ai_config.go/ai_secret_windows.go 独立加密配置。ai_state.go 用任务世代拒绝取消后的迟到响应，短暂的配置原子写入不能取消。ai_tasks.go 先完整预览再事务入库，ai_grading.go 只更新练习记录，考试保持本地评分。Beta 默认关闭并在每个网络任务入口检查。

appearance.go 将持久化外观选择与实时 dark 配色分开，MyGo OnUpdated 在 UI 线程同步系统变化，窗口关闭解除监听；学习设置版本 2 保留版本 1 的浅/深色选择。MyGo 固定 v0.7.4，升级前阅读其迁移指南并运行 migrate-ui 预览、mygo vet 和全部测试。

Windows 图标缓存：安装器给桌面/开始菜单快捷方式指定 icons/quesora-<version>.ico 并通知 Shell 刷新，不能只覆盖同一路径 EXE 后期待缓存失效。运行窗口创建后用 SetIcon 设置完整题卡 PNG；不修改用户原图或用简化 Q 替代小尺寸。qa/installer_smoke.py 在临时 runner 验证真实快捷方式的 IconLocation 和 ICO 内容，不在开发机安装/重装。

预览通道更新由 update_feed.go 优先读取固定仓库的 releases.atom，仅接受本仓库合法 SemVer 标签链接，再用 HEAD 确认固定命名资产长度。订阅源或资产探测失败才使用 update.go 中的发布列表 API；不重试 403/429。UI 对成功检查缓存一分钟，保留候选和已校验文件；所有下载仍经官方 HTTPS 主机白名单与 SHA-256 检查。
正式通道仍通过 API 验证 release.prerelease 标记，因为 Atom 不携带该字段，不能只根据标签格式区分正式与预发布。
