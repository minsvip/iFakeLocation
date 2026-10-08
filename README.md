# iFakeLocation (Go Version)

[English](#english) | [中文说明](#中文说明)

---

<a name="中文说明"></a>
## 🇨🇳 中文说明

**iFakeLocation (Go)** 是使用纯 Go 语言完全重构的 iOS 模拟定位工具。无需安装 .NET Runtime、Mono、iTunes 或任何 C/C++ 动态链接库（如 `libimobiledevice`），单二进制文件开箱即用。

本项目在完全兼容原版 Leaflet 地图 Web 前端交互的基础上，针对 **iOS 17+ / iOS 18+（包括最新的 iPhone 16 系列 / A18 Pro 芯片）** 进行了深度适配，支持基于 Userspace Tunnel 与 RSD（Remote Service Discovery）的 CoreDevice 通信协议，无需 `sudo` / Root 权限即可在 macOS、Linux、Windows 上稳定运行。

### ✨ 特性亮点

- **纯 Go 实现，零外部依赖**：单可执行文件，无需安装 .NET 6/7/8、iTunes 或繁琐配置 `DYLD_LIBRARY_PATH`。
- **全面支持 iOS 全版本**：
  - **iOS 9 ~ iOS 16**：自动挂载对应版本的 DeveloperDiskImage，通过经典 lockdown 协议模拟定位。
  - **iOS 17 ~ iOS 18+**：支持 Apple 最新通用个性化镜像（Personalized DDI），内置 Userspace TUN（基于 gVisor 协议栈）和 RSD 握手，走 `com.apple.instruments.dtservicehub` 的 DTX 位置模拟服务。
  - **实机验证**：已在 iPhone 16 Pro Max（A18 Pro 芯片，`ApChipId 0x8140`）实机上完整验证。
- **自动化开发者镜像管理**：
  - 自动检测并下载对应的开发者磁盘镜像（DDI）。
  - 支持 macOS 本地 Xcode 开发者镜像自动检索与无缝挂载。
- **内嵌 Web UI**：静态资源全部通过 Go `embed.FS` 内嵌，启动即自动拉起浏览器，支持搜索定位、双击选点、实时设置与恢复真实定位。
- **跨平台支持**：原生支持 macOS（Apple Silicon M1/M2/M3/M4 & Intel）、Linux、Windows。

---

### 🚀 快速开始

#### 1. 运行预编译程序
直接运行编译好的可执行文件：
```bash
# macOS / Linux
chmod +x ./bin/ifakelocation
./bin/ifakelocation

# Windows
ifakelocation.exe
```
程序启动后会自动绑定空闲端口并在默认浏览器中打开页面（如 `http://localhost:49215/`）。

#### 2. 自行编译构建
环境要求：**Go 1.21 或更高版本**

```bash
# 克隆仓库
git clone https://github.com/minsvip/iFakeLocation.git
cd iFakeLocation

# 使用 Makefile 构建 (推荐)
make build

# 或直接使用 go 构建
go build -o bin/ifakelocation ./cmd/ifakelocation

# 运行
./bin/ifakelocation
```

#### 交叉编译（Cross-Compilation）
```bash
# 一键交叉编译所有主流平台 (产物输出至 bin/ 目录)
make cross-compile

# 或者单独编译：
# macOS Apple Silicon (arm64)
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/ifakelocation-darwin-arm64 ./cmd/ifakelocation

# macOS Intel (amd64)
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/ifakelocation-darwin-amd64 ./cmd/ifakelocation

# Windows (amd64)
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/ifakelocation-windows-amd64.exe ./cmd/ifakelocation

# Linux (amd64)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/ifakelocation-linux-amd64 ./cmd/ifakelocation
```

---

### 📖 使用指南

1. **连接手机**：使用数据线将 iPhone 连接至电脑，在手机弹出的提示中点击 **“信任此电脑”** 并输入锁屏密码。
2. **iOS 16+ 开启开发者模式**：
   - 首次使用需在手机上进入：`设置` -> `隐私与安全性` -> `开发者模式` 并开启（开启后手机会自动重启一次）。
3. **设置虚拟定位**：
   - 在网页顶部下拉菜单中选择你的设备。
   - 在地图上方输入地点（如“故宫博物院”）搜索，或直接在地图上任意位置**双击鼠标**放置图钉。
   - 点击 **“Set Fake Location”**，等待提示定位成功。
4. **停止虚拟定位**：
   - 点击网页上的 **“Stop Fake Location”** 按钮即可还原真实 GPS。

---

<a name="english"></a>
## 🌐 English

**iFakeLocation (Go)** is a complete rewrite of the popular iOS location spoofing tool in pure Go. It eliminates all dependencies on .NET Runtime, Mono, iTunes, and native C libraries like `libimobiledevice`.

In addition to maintaining 100% compatibility with the original Leaflet-based web UI, this version brings full, native support for **iOS 17+ and iOS 18+ (tested on iPhone 16 series with A18 Pro)** using a userspace CoreDevice tunnel and Remote Service Discovery (RSD) over DTX (`dtservicehub`), without requiring `sudo` or root privileges.

### ✨ Key Features

- **Pure Go & Zero Runtime Dependencies**: Single self-contained binary. No .NET runtime, no iTunes, no `libimobiledevice` C dynamic library headaches.
- **Full iOS Version Support**:
  - **iOS 9 – 16**: Automatic DDI mounting and classic lockdown location simulation.
  - **iOS 17 – 18+**: Personalized DDI mounting, Userspace TUN (powered by gVisor stack), RSD handshake, and Instruments `LocationSimulationService`.
  - **Hardware Verified**: Tested and verified on iPhone 16 Pro Max (`iPhone17,2`, A18 Pro chip `0x8140`).
- **Automated DDI Management**: Automated downloading and caching of developer disk images, with fallback to local Xcode images.
- **Embedded Web UI**: Static assets embedded via Go's `embed.FS`. Automatically launches your default browser.
- **Cross-Platform**: Native binaries for macOS (Intel & Apple Silicon), Linux, and Windows.

---

### 📂 项目工程结构 / Project Structure

```
├── cmd/
│   └── ifakelocation/        # 应用程序主入口 (main.go)
├── internal/                 # 核心内部实现包
│   ├── imagehelper/          # 开发者镜像查找、自动下载与有效性校验
│   ├── location/             # 模拟定位、Userspace 隧道、RSD 握手与会话管理
│   ├── models/               # 数据模型定义、iPhone/iPad 设备型号映射
│   └── server/               # REST API 处理与嵌入静态文件服务
├── resources/                # Web 前端资源与嵌入包 (resources.go, HTML/CSS/JS)
├── scripts/                  # 辅助验证脚本 (test_tunnel, verify_api)
├── DeveloperImages/          # 开发者磁盘镜像本地缓存目录
├── updates.json              # 开发者镜像版本源索引
├── Makefile                  # 标准化构建、测试与交叉编译任务
├── go.mod / go.sum           # Go 模块依赖定义
└── README.md                 # 项目中英文文档
```

---

### 💡 常见问题 (FAQ)

**Q: 开发者磁盘镜像（DeveloperImages）会自动下载吗？**  
A: **会自动下载并缓存。**
- **iOS 9 ~ 16**：程序会根据连接的手机版本（如 15.4、16.2），自动从配置的镜像源检索并下载对应的 `DeveloperDiskImage.dmg` 及签名文件，并在前端展示下载进度条。
- **iOS 17 ~ 18+**：Apple 改用了“通用个性化镜像”（Personalized DDI）。如果宿主机装有 Xcode，程序会自动引用本地镜像；若无 Xcode，则会在初次挂载时自动下载通用个性化镜像包并缓存至 `DeveloperImages/` 目录中。

**Q: 提示 `Please unlock your iOS device screen and try again`？**  
A: iOS 挂载开发者镜像和建立安全调试隧道时，设备屏幕必须处于解锁状态。解锁手机屏幕后再次点击即可。

**Q: 我的系统是 iOS 18 / 27.x，镜像只有 17.0 / 27.0 会有影响吗？**  
A: 完全没有影响。iOS 17 起 Apple 采用通用的“个性化镜像”机制，挂载时校验的是芯片架构（如 A18 Pro）和主板标识，同一大版本下的所有系统版本均通用。

**Q: 模拟定位生效后，如何彻底恢复真实 GPS？**  
A: 点击网页上的 **Stop Fake Location** 即可。如果关闭了程序，直接重启手机或在手机设置中开关一次“定位服务”也可以立即恢复真实 GPS。

---

### 🙏 致谢与特别感谢 (Acknowledgments & Credits)

本项目由衷感谢以下开源项目与作者的卓越贡献：

1. **[iFakeLocation by master131](https://github.com/master131/iFakeLocation)**  
   *原工程作者。提供了极其出色的产品概念、易用的 Web 前端设计和算法原型。本工程基于其思路使用 Go 进行了全量现代化重构。*
2. **[go-ios by danielpaulus](https://github.com/danielpaulus/go-ios)**  
   *提供了强大的纯 Go iOS 通信协议实现、Userspace TUN 隧道、CoreDevice 适配与 DTX Instruments 调试能力。*
3. **[idevicelocation by JonGabilondoAngulo](https://github.com/JonGabilondoAngulo/idevicelocation)**  
   *为 iOS 坐标模拟提供了最早的逆向分析与实现参考。*
4. **[haikieu/xcode-developer-disk-image-all-platforms](https://github.com/haikieu/xcode-developer-disk-image-all-platforms)** 与 **[xushuduo/Xcode-iOS-Developer-Disk-Image](https://github.com/xushuduo/Xcode-iOS-Developer-Disk-Image/)**  
   *提供了丰富的全平台 iOS 开发者镜像资源镜像与归档。*

---

### 📄 开源许可证 (License)

本项目继承原项目的开源协议，采用 [GNU General Public License v3.0 (GPL-3.0)](LICENSE) 授权开源。
