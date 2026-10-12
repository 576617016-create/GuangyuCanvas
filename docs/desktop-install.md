# 安装 光宇无限画布

安装包不使用付费代码签名证书。Windows 可能显示“未知发布者”或 SmartScreen 提示；macOS 可能要求在“系统设置 → 隐私与安全性”中允许打开。受组织策略或 Smart App Control 限制的设备可能无法运行。请只从 光宇无限画布 官网或官方 GitHub Releases 下载。

- Windows：运行 `光宇无限画布-vX.Y.Z-windows-amd64-setup.exe`，安装到当前用户目录，无需管理员权限。开始菜单有 光宇无限画布 和卸载入口，可以选择创建桌面快捷方式。WebView2 是运行依赖，缺失时应用会提示下载安装。
- macOS：打开对应芯片的 `.dmg`，将 `光宇无限画布.app` 拖到“Applications”，退出磁盘映像后从“应用程序”启动。已有版本时先退出 光宇无限画布，再替换应用。
- Ubuntu 24.04 x64：下载 `.deb` 后运行 `sudo apt install ./光宇无限画布-vX.Y.Z-linux-amd64.deb`，从应用菜单启动。下载新版 DEB 并再次安装即可升级；DEB 不使用应用内自动更新。

ZIP 仍供便携使用和应用内更新。Windows/macOS 安装后可继续使用应用内更新。旧 ZIP 用户先退出应用，安装新包后用同一系统账户启动；确认旧项目和素材正常，再删除旧程序目录。

项目、素材、草稿和设置默认存放在独立的用户目录：Windows `%AppData%\光宇无限画布`；macOS `~/Library/Application Support/光宇无限画布`；Linux `~/.config/光宇无限画布`（遵循 `XDG_CONFIG_HOME`）。安装、覆盖安装和卸载不删除这些目录。Windows 的 WebView2 缓存 `%AppData%\光宇无限画布.exe` 也会保留。

如果曾通过 `CANVAS_DESKTOP_DATA_DIR` 或 `--data-dir` 指定目录，安装后仍需使用同一目录；不要把工作数据放进程序安装目录。安装包不会搬迁自定义目录，也不会自动找回已经丢失的文件。
