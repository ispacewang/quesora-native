# Windows 安装包

Inno Setup 6.7.1 或更新版本；固定 AppId 为 com.ispace.quesora.native，不能随版本修改。当前用户安装，升级沿用注册表中的安装目录；卸载只删除安装器管理的程序文件，不删除 LocalAppData 中的题库数据。便携 ZIP 不包含 installed.marker。

ChineseSimplified.isl 来自 jrsoftware/issrc 的 is-6_7_1 标签 Files/Languages/Unofficial/ChineseSimplified.isl，保留原始译者署名，参见 INNO-LICENSE.txt。

快捷方式与卸载图标使用安装器携带的完整多尺寸 ICO，安装到 icons/quesora-<version>.ico；版本变化会改变图标路径，避免 Explorer 复用早期 Q 图标缓存。安装结束通过 ChangesAssociations 通知 Shell 刷新；运行窗口使用 MyGo SetIcon 显式设置当前完整题卡图。不会重启 Explorer 或删除系统图标缓存。CI 在临时 Windows runner 核对真实桌面/开始菜单快捷方式、目标程序和独立 ICO 内容。
