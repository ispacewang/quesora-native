// Quesora Native 是以 Go/MyGo 实现的离线 Windows 题库、练习与考试应用。
//
// # 数据流
//
// 文件读取器将 XLSX/XLS/CSV 转为 worksheet，questionSchema 解释表头和说明，
// 再将可确认的行归一化为 question。questionStore 事务保存整个导入批次；
// app 使用题目主键维护最近练习记录，examSession 则保存独立试卷快照。
//
// # 线程与生命周期
//
// app 的可变字段归 UI 线程所有。磁盘/数据库工作通过 libraryJobs 串行提交；
// 后台完成后用 publish/Window.Update 回写状态。不得在 app 或 goroutine 中保存
// ui.Element、构建 Context 或绘制回调的临时部件，跨帧身份使用 ui.Handle。
// 关闭时先停止窗口更新并排空任务队列，再关闭数据库。
//
// # 兼容与数据保护
//
// 答案位掩码基于原始选项索引，显示打乱只改变映射。日常练习、考试和已保存
// 错题本分别维护，重置练习不删除其他记录。SQLite 备份使用一致性快照，
// 不直接复制可能有 WAL 的活动主文件；安装目录和用户数据目录彼此独立。
//
// # 开发入口
//
// main.go 管理启动；view.go/quiz.go 是原生视图入口；import_schema.go 是解析规则；
// exam.go 是考试状态机；storage.go 管理存储；build-preview.py 与 packaging/
// 负责可复现分发。维护导览及验证命令见 docs/development.md。
package main
