# 网站测绘 CLI 工具架构与 MVP 任务文档

## 1. 文档目的

本文档在《项目需求.md》和《需求分析文档.md》的基础上，进一步推演工具架构，明确核心模块、输入输出接口格式、内部数据结构、数据流转路径和 MVP 实现任务。

本文档不包含具体代码实现，目标是在开发前先确定整体流程，确保首版 MVP 可以快速打通“输入参数 -> mDNS 发现 -> 解析资产 -> 过滤结果 -> 输出结果”的闭环。

## 2. MVP 目标

MVP 版本只解决最核心的问题：

```text
用户输入 IP 网段和端口范围，程序执行 mDNS 服务发现，解析发现结果，按 IP 和端口条件过滤，输出资产服务信息和 banner 信息。
```

MVP 不追求完整资产测绘平台能力，不做复杂端口扫描，不做漏洞检测，不做 Web UI，不做长期存储。

## 3. MVP 功能边界

### 3.1 MVP 必须实现

- 命令行入口
- CIDR 参数解析
- 端口范围参数解析
- mDNS 查询任务发起
- mDNS 响应接收
- PTR/SRV/TXT/A/AAAA 记录解析
- 服务资产聚合
- IP 网段过滤
- 端口范围过滤
- 文本格式输出
- JSON 格式输出，作为可选但强烈建议
- 示例数据或 mock 数据解析能力，用于无真实 mDNS 环境时验证流程

### 3.2 MVP 暂不实现

- TCP 主动端口扫描
- HTTP 页面标题识别
- SMB/AFP 协议深度探测
- 漏洞检测
- 分布式扫描
- Web 管理后台
- 数据库存储
- 定时任务
- 资产历史对比
- 复杂设备指纹库

### 3.3 关键实现原则

由于 mDNS 的工作方式是组播发现，而不是对每个 IP 和端口进行 TCP 扫描，因此 MVP 的流程应定义为：

```text
先通过 mDNS 发现服务，再根据用户输入的 IP 网段和端口范围过滤结果。
```

这比“逐个 IP、逐个端口扫描 mDNS”更符合协议实际工作方式。

## 4. 总体架构

### 4.1 架构分层

建议采用简单分层架构：

```text
cmd
  负责 CLI 程序入口、参数接收、主流程编排

internal/config
  负责配置结构、参数校验、端口范围解析

internal/mdns
  负责 mDNS 查询、响应接收、原始记录采集

internal/parser
  负责将 mDNS 原始记录解析为中间结构

internal/asset
  负责服务资产聚合、设备信息合并、banner 归一化

internal/filter
  负责 IP 网段过滤和端口范围过滤

internal/output
  负责 text/json 输出

testdata
  存放 mDNS mock 数据或样例数据
```

### 4.2 推荐目录结构

```text
.
├── cmd/
│   └── mdnsmap/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── mdns/
│   │   ├── client.go
│   │   └── record.go
│   ├── parser/
│   │   └── parser.go
│   ├── asset/
│   │   ├── model.go
│   │   └── aggregator.go
│   ├── filter/
│   │   └── filter.go
│   └── output/
│       ├── text.go
│       └── json.go
├── testdata/
│   └── sample-mdns.json
├── go.mod
├── README.md
├── 项目需求.md
├── 需求分析文档.md
└── 架构与MVP任务文档.md
```

MVP 可以适当合并文件，但模块职责不应混在一起。

## 5. 核心模块说明

### 5.1 CLI 入口模块

模块位置：

```text
cmd/mdnsmap
```

职责：

- 接收命令行参数
- 调用配置解析
- 初始化 mDNS 客户端
- 编排发现、解析、过滤、输出流程
- 处理顶层错误
- 决定程序退出码

建议命令形式：

```bash
mdnsmap --cidr 192.168.1.0/24 --ports 1-6000 --timeout 5s --format text
```

参数最小集合：

| 参数 | 必填 | 默认值 | 说明 |
|---|---|---|---|
| `--cidr` | 是 | 无 | 目标 IP 网段 |
| `--ports` | 是 | 无 | 目标端口范围 |
| `--timeout` | 否 | `5s` | mDNS 查询等待时间 |
| `--format` | 否 | `text` | 输出格式，支持 `text`、`json` |
| `--mock` | 否 | 空 | 从 mock 文件读取记录，便于测试 |
| `--verbose` | 否 | `false` | 输出详细日志或调试信息 |

MVP 阶段建议保留 `--mock` 参数，因为真实 mDNS 环境不一定稳定，mock 输入可以保证流程可演示。

### 5.2 配置模块

模块位置：

```text
internal/config
```

职责：

- 保存程序运行配置
- 校验 CIDR
- 解析端口表达式
- 校验输出格式
- 校验 timeout

配置结构建议：

```go
type Config struct {
    CIDR    string
    Ports   PortSet
    Timeout time.Duration
    Format  string
    Mock    string
    Verbose bool
}
```

端口集合结构建议：

```go
type PortSet struct {
    Ranges []PortRange
}

type PortRange struct {
    Start int
    End   int
}
```

端口表达式支持：

```text
80
1-1024
80,443,5000
1-1024,5000,8000-9000
```

### 5.3 mDNS 发现模块

模块位置：

```text
internal/mdns
```

职责：

- 发起 mDNS 服务发现查询
- 接收响应记录
- 把第三方库或底层网络返回的数据转换成项目内部 RawRecord

MVP 查询策略：

```text
1. 查询 _services._dns-sd._udp.local，发现服务类型。
2. 对常见服务类型发起查询。
3. 收集查询窗口内返回的 PTR、SRV、TXT、A、AAAA 记录。
4. 超时后返回所有原始记录。
```

MVP 可内置常见服务类型：

```text
_http._tcp.local
_smb._tcp.local
_workstation._tcp.local
_device-info._tcp.local
_afpovertcp._tcp.local
_qdiscover._tcp.local
```

内部原始记录结构建议：

```go
type RawRecord struct {
    Type     string
    Name     string
    Value    string
    Hostname string
    Port     int
    Priority int
    Weight   int
    TTL      uint32
    IPv4     string
    IPv6     string
    TXT      []string
}
```

说明：

- 该结构不是 DNS 标准结构的完整映射，而是为 MVP 聚合流程服务。
- 如果使用 mDNS 库，适配层负责将库对象转换成此结构。

### 5.4 Parser 解析模块

模块位置：

```text
internal/parser
```

职责：

- 将 RawRecord 转成更规范的 ParsedRecord
- 从服务类型中提取 service 和 protocol
- 将 TXT 字符串解析为 banner 键值对
- 标准化 hostname、service name、record type

服务名解析规则：

```text
_http._tcp.local       -> service=http, protocol=tcp
_smb._tcp.local        -> service=smb, protocol=tcp
_afpovertcp._tcp.local -> service=afpovertcp, protocol=tcp
_qdiscover._tcp.local  -> service=qdiscover, protocol=tcp
```

TXT 解析规则：

```text
path=/ -> {"path": "/"}
model=TS-X64 -> {"model": "TS-X64"}
fwVer=5.2.9 -> {"fwVer": "5.2.9"}
```

对于没有 `=` 的 TXT 项：

```text
flag -> {"flag": "true"}
```

解析后结构建议：

```go
type ParsedRecord struct {
    RecordType string
    Service    string
    Protocol   string
    Instance   string
    Name       string
    Hostname   string
    Port       int
    TTL        uint32
    IPv4       string
    IPv6       string
    Banner     map[string]string
    RawName    string
    RawValue   string
}
```

### 5.5 资产聚合模块

模块位置：

```text
internal/asset
```

职责：

- 将多条 ParsedRecord 聚合成资产服务
- 合并同一服务实例的 IP、端口、主机名、banner
- 处理同一主机多个服务
- 保留 answers 汇总信息

聚合目标结构：

```go
type ScanResult struct {
    Services []ServiceAsset `json:"services"`
    Answers  AnswerSummary  `json:"answers,omitempty"`
}

type ServiceAsset struct {
    IP       string            `json:"ip,omitempty"`
    IPv6     string            `json:"ipv6,omitempty"`
    Port     int               `json:"port,omitempty"`
    Protocol string            `json:"protocol,omitempty"`
    Service  string            `json:"service,omitempty"`
    Name     string            `json:"name,omitempty"`
    Hostname string            `json:"hostname,omitempty"`
    TTL      uint32            `json:"ttl,omitempty"`
    Banner   map[string]string `json:"banner,omitempty"`
}

type AnswerSummary struct {
    PTR []string `json:"ptr,omitempty"`
}
```

聚合 key 建议：

```text
hostname + service + protocol + port
```

如果 hostname 为空，则使用：

```text
ip + service + protocol + port
```

如果是 `device-info` 这类无端口记录，则使用：

```text
hostname + service
```

### 5.6 过滤模块

模块位置：

```text
internal/filter
```

职责：

- 根据 CIDR 过滤资产 IP
- 根据 PortSet 过滤服务端口
- 处理无端口设备信息

IP 过滤规则：

```text
如果 asset.IP 在 cidr 内，保留。
如果 asset.IP 不在 cidr 内，过滤。
如果 asset.IP 为空但 asset.IPv6 存在，MVP 默认保留，并在输出中展示 IPv6。
```

端口过滤规则：

```text
如果 asset.Port 在用户指定端口范围内，保留。
如果 asset.Port 不在用户指定端口范围内，过滤。
如果 asset.Port 为空或为 0：
  - device-info 类型保留。
  - 其他类型默认过滤。
```

### 5.7 输出模块

模块位置：

```text
internal/output
```

职责：

- 将 ScanResult 输出为 text
- 将 ScanResult 输出为 json
- 保持字段顺序稳定，方便人工阅读和测试断言

输出格式由 `--format` 参数决定。

支持：

```text
text
json
```

## 6. 输入接口设计

### 6.1 CLI 输入

MVP 标准命令：

```bash
mdnsmap --cidr 192.168.1.0/24 --ports 1-6000
```

完整命令：

```bash
mdnsmap --cidr 192.168.1.0/24 --ports 1-6000 --timeout 5s --format text
```

JSON 输出：

```bash
mdnsmap --cidr 192.168.1.0/24 --ports 1-6000 --format json
```

Mock 输入：

```bash
mdnsmap --cidr 192.168.1.0/24 --ports 1-6000 --mock testdata/sample-mdns.json
```

### 6.2 参数格式

#### CIDR

格式：

```text
IPv4/CIDR
```

示例：

```text
192.168.1.0/24
10.0.0.0/8
172.16.0.0/16
```

MVP 只要求支持 IPv4 CIDR。

#### Ports

格式：

```text
端口
端口范围
逗号分隔的端口和端口范围
```

示例：

```text
80
1-1024
80,443,5000
1-1024,5000,8000-9000
```

#### Timeout

格式：

```text
Go duration 字符串
```

示例：

```text
3s
5s
10s
```

#### Format

允许值：

```text
text
json
```

## 7. 输出接口设计

### 7.1 Text 输出

Text 输出用于人工阅读，应贴近原始需求示例。

输出示例：

```text
services:
5000/tcp http:
Name=slw-nas
IPv4=192.168.1.20
IPv6=fe80::265e:beff:fe69:a313
Hostname=slw-nas.local
TTL=10
path=/

445/tcp smb:
Name=slw-nas
IPv4=192.168.1.20
IPv6=fe80::265e:beff:fe69:a313
Hostname=slw-nas.local
TTL=10

5000/tcp qdiscover:
Name=slw-nas
IPv4=192.168.1.20
IPv6=fe80::265e:beff:fe69:a313
Hostname=slw-nas.local
TTL=10
accessType=https,accessPort=86,model=TS-X64,displayModel=TS-464C,fwVer=5.2.9,fwBuildNum=20260214

answers:
PTR:
_http._tcp.local
_smb._tcp.local
_qdiscover._tcp.local
```

Text 输出规则：

- 第一行输出 `services:`
- 有端口服务使用 `{port}/{protocol} {service}:`
- 无端口设备信息使用 `{service}:`
- 基础字段按固定顺序输出：
  - Name
  - IPv4
  - IPv6
  - Hostname
  - TTL
- banner 字段在基础字段之后输出
- answers 汇总放在最后

### 7.2 JSON 输出

JSON 输出用于程序集成和自动化测试。

输出示例：

```json
{
  "services": [
    {
      "ip": "192.168.1.20",
      "ipv6": "fe80::265e:beff:fe69:a313",
      "port": 5000,
      "protocol": "tcp",
      "service": "http",
      "name": "slw-nas",
      "hostname": "slw-nas.local",
      "ttl": 10,
      "banner": {
        "path": "/"
      }
    },
    {
      "ip": "192.168.1.20",
      "ipv6": "fe80::265e:beff:fe69:a313",
      "port": 5000,
      "protocol": "tcp",
      "service": "qdiscover",
      "name": "slw-nas",
      "hostname": "slw-nas.local",
      "ttl": 10,
      "banner": {
        "accessType": "https",
        "accessPort": "86",
        "model": "TS-X64",
        "displayModel": "TS-464C",
        "fwVer": "5.2.9",
        "fwBuildNum": "20260214"
      }
    }
  ],
  "answers": {
    "ptr": [
      "_http._tcp.local",
      "_qdiscover._tcp.local"
    ]
  }
}
```

## 8. 数据流转路径

### 8.1 主流程

```text
用户输入 CLI 参数
        |
        v
CLI 入口解析参数
        |
        v
Config 模块校验 CIDR、Ports、Timeout、Format
        |
        v
判断是否使用 mock 输入
        |
        +--------------------------+
        |                          |
        v                          v
读取 mock RawRecord         mDNS 模块发起查询
        |                          |
        +------------+-------------+
                     |
                     v
Parser 模块解析 RawRecord
                     |
                     v
Asset 模块聚合 ParsedRecord
                     |
                     v
Filter 模块按 CIDR 和 Ports 过滤
                     |
                     v
Output 模块按 format 输出
                     |
                     v
程序退出
```

### 8.2 数据形态流转

```text
CLI Args
  -> Config
  -> []RawRecord
  -> []ParsedRecord
  -> ScanResult
  -> Filtered ScanResult
  -> text/json
```

### 8.3 错误流转

```text
参数错误
  -> CLI 输出错误
  -> exit code 2

mDNS 网络错误
  -> CLI 输出错误
  -> exit code 1

未发现资产
  -> 输出空 services
  -> exit code 0

部分记录解析失败
  -> 默认跳过
  -> verbose 模式输出原因
  -> exit code 0
```

## 9. 关键数据结构定义

### 9.1 Config

```go
type Config struct {
    CIDR    string
    Ports   PortSet
    Timeout time.Duration
    Format  string
    Mock    string
    Verbose bool
}
```

用途：

```text
承载 CLI 参数解析后的运行配置。
```

### 9.2 RawRecord

```go
type RawRecord struct {
    Type     string   `json:"type"`
    Name     string   `json:"name"`
    Value    string   `json:"value,omitempty"`
    Hostname string   `json:"hostname,omitempty"`
    Port     int      `json:"port,omitempty"`
    TTL      uint32   `json:"ttl,omitempty"`
    IPv4     string   `json:"ipv4,omitempty"`
    IPv6     string   `json:"ipv6,omitempty"`
    TXT      []string `json:"txt,omitempty"`
}
```

用途：

```text
承载 mDNS 模块收集到的原始记录或 mock 文件中的原始记录。
```

### 9.3 ParsedRecord

```go
type ParsedRecord struct {
    RecordType string
    Service    string
    Protocol   string
    Instance   string
    Name       string
    Hostname   string
    Port       int
    TTL        uint32
    IPv4       string
    IPv6       string
    Banner     map[string]string
    RawName    string
    RawValue   string
}
```

用途：

```text
承载解析后的标准化记录，为资产聚合做准备。
```

### 9.4 ServiceAsset

```go
type ServiceAsset struct {
    IP       string            `json:"ip,omitempty"`
    IPv6     string            `json:"ipv6,omitempty"`
    Port     int               `json:"port,omitempty"`
    Protocol string            `json:"protocol,omitempty"`
    Service  string            `json:"service,omitempty"`
    Name     string            `json:"name,omitempty"`
    Hostname string            `json:"hostname,omitempty"`
    TTL      uint32            `json:"ttl,omitempty"`
    Banner   map[string]string `json:"banner,omitempty"`
}
```

用途：

```text
表示最终输出中的一条服务资产。
```

### 9.5 ScanResult

```go
type ScanResult struct {
    Services []ServiceAsset `json:"services"`
    Answers  AnswerSummary  `json:"answers,omitempty"`
}
```

用途：

```text
表示最终扫描结果。
```

## 10. Mock 数据接口

### 10.1 设计目的

真实 mDNS 扫描依赖网络环境，可能出现没有设备响应、防火墙拦截、组播不可用等情况。为了保证 MVP 可以稳定演示，应支持 mock 数据输入。

### 10.2 Mock 文件格式

文件路径示例：

```text
testdata/sample-mdns.json
```

文件内容示例：

```json
[
  {
    "type": "PTR",
    "name": "_http._tcp.local",
    "value": "slw-nas._http._tcp.local",
    "ttl": 10
  },
  {
    "type": "SRV",
    "name": "slw-nas._http._tcp.local",
    "hostname": "slw-nas.local",
    "port": 5000,
    "ttl": 10
  },
  {
    "type": "TXT",
    "name": "slw-nas._http._tcp.local",
    "txt": ["path=/"],
    "ttl": 10
  },
  {
    "type": "A",
    "name": "slw-nas.local",
    "ipv4": "192.168.1.20",
    "ttl": 10
  },
  {
    "type": "AAAA",
    "name": "slw-nas.local",
    "ipv6": "fe80::265e:beff:fe69:a313",
    "ttl": 10
  }
]
```

### 10.3 Mock 执行流程

```text
传入 --mock
  -> 跳过真实 mDNS 查询
  -> 读取 mock JSON
  -> 转为 []RawRecord
  -> 进入 parser/asset/filter/output 正常流程
```

这样可以保证即使没有真实 NAS 或 mDNS 设备，MVP 也能证明解析和输出链路是通的。

## 11. MVP 任务拆解

### 11.1 任务 1：项目骨架

目标：

```text
建立 Go 项目目录结构和基础入口。
```

产出：

- `go.mod`
- `cmd/mdnsmap/main.go`
- `internal` 基础目录
- README 初稿

验收：

```bash
go run ./cmd/mdnsmap --help
```

可以看到帮助信息。

### 11.2 任务 2：参数解析与配置校验

目标：

```text
实现 CIDR、ports、timeout、format、mock 参数解析。
```

产出：

- Config 结构
- PortSet 结构
- CIDR 校验逻辑
- 端口范围解析逻辑

验收：

```bash
mdnsmap --cidr 192.168.1.0/24 --ports 1-6000
```

参数合法，程序进入下一阶段。

错误输入：

```bash
mdnsmap --cidr abc --ports 1-6000
```

应输出错误。

### 11.3 任务 3：Mock 数据链路

目标：

```text
先用 mock 数据打通完整流程，降低真实网络依赖。
```

产出：

- `testdata/sample-mdns.json`
- mock loader
- RawRecord 结构

验收：

```bash
mdnsmap --cidr 192.168.1.0/24 --ports 1-6000 --mock testdata/sample-mdns.json
```

可以读取 mock 数据并进入解析流程。

### 11.4 任务 4：Parser 解析

目标：

```text
将 RawRecord 解析为 ParsedRecord。
```

产出：

- 服务类型解析
- 协议解析
- TXT banner 解析
- hostname/name 标准化

验收：

输入：

```text
_qdiscover._tcp.local
accessType=https
model=TS-X64
```

输出 ParsedRecord 中应包含：

```text
service=qdiscover
protocol=tcp
banner.accessType=https
banner.model=TS-X64
```

### 11.5 任务 5：资产聚合

目标：

```text
将多条 ParsedRecord 合并成 ServiceAsset。
```

产出：

- Aggregator
- ScanResult
- PTR answers 汇总

验收：

同一个服务实例的 PTR/SRV/TXT/A/AAAA 记录应合并成一条资产服务。

### 11.6 任务 6：过滤器

目标：

```text
按 CIDR 和端口范围过滤资产。
```

产出：

- IP 过滤
- PortSet 匹配
- device-info 保留规则

验收：

输入：

```text
CIDR=192.168.1.0/24
Ports=1-6000
```

结果：

- `192.168.1.20:5000` 保留
- `10.0.0.10:5000` 过滤
- `192.168.1.20:8080` 过滤
- `device-info` 无端口记录保留

### 11.7 任务 7：Text/JSON 输出

目标：

```text
实现符合示例深度的输出。
```

产出：

- text formatter
- json formatter

验收：

Text 输出包含：

```text
services:
5000/tcp qdiscover:
Name=slw-nas
IPv4=192.168.1.20
Hostname=slw-nas.local
accessType=https,accessPort=86,model=TS-X64,displayModel=TS-464C,fwVer=5.2.9,fwBuildNum=20260214
```

JSON 输出包含：

```json
"service": "qdiscover",
"banner": {
  "accessType": "https",
  "model": "TS-X64"
}
```

### 11.8 任务 8：真实 mDNS 查询

目标：

```text
实现真实网络环境下的 mDNS 发现。
```

产出：

- mDNS client
- 查询常见服务
- 响应记录转换为 RawRecord

验收：

在存在 mDNS 设备的局域网中运行：

```bash
mdnsmap --cidr 192.168.1.0/24 --ports 1-6000
```

可以输出发现到的服务资产。

### 11.9 任务 9：README 与演示说明

目标：

```text
说明工具用途、运行方式、参数、示例输出和限制。
```

产出：

- README.md

验收：

README 中包含：

- 项目说明
- 参数说明
- mock 运行示例
- 真实 mDNS 运行示例
- 输出字段说明
- 已知限制

## 12. MVP 开发顺序建议

推荐顺序：

```text
1. 项目骨架
2. 参数解析
3. Mock 数据链路
4. Parser
5. Aggregator
6. Filter
7. Output
8. 真实 mDNS 查询
9. README
```

原因：

```text
先打通 mock 数据链路，可以绕开真实网络不稳定因素，快速验证核心业务流程。等解析、聚合、过滤和输出全部稳定后，再接入真实 mDNS 查询模块。
```

## 13. MVP 验收用例

### 13.1 用例 1：mock 数据 text 输出

命令：

```bash
mdnsmap --cidr 192.168.1.0/24 --ports 1-6000 --format text --mock testdata/sample-mdns.json
```

期望：

```text
输出 services。
输出 http/smb/qdiscover/device-info/afpovertcp。
qdiscover 输出 model、displayModel、fwVer、fwBuildNum。
```

### 13.2 用例 2：mock 数据 json 输出

命令：

```bash
mdnsmap --cidr 192.168.1.0/24 --ports 1-6000 --format json --mock testdata/sample-mdns.json
```

期望：

```text
输出合法 JSON。
services 为数组。
banner 为对象。
answers.ptr 包含服务类型。
```

### 13.3 用例 3：CIDR 过滤

命令：

```bash
mdnsmap --cidr 10.0.0.0/8 --ports 1-6000 --mock testdata/sample-mdns.json
```

期望：

```text
192.168.1.20 的资产被过滤。
```

### 13.4 用例 4：端口过滤

命令：

```bash
mdnsmap --cidr 192.168.1.0/24 --ports 1-100 --mock testdata/sample-mdns.json
```

期望：

```text
5000、445、548 端口服务被过滤。
device-info 可保留。
```

### 13.5 用例 5：非法参数

命令：

```bash
mdnsmap --cidr abc --ports 1-6000
```

期望：

```text
程序输出 CIDR 格式错误。
退出码非 0。
```

## 14. 后续扩展方向

MVP 完成后可以考虑：

- 支持多网卡选择
- 支持指定服务类型查询
- 支持输出 CSV
- 支持保存结果文件
- 支持 HTTP 标题和状态码识别
- 支持更丰富的设备指纹规则
- 支持主动连接验证端口是否开放
- 支持扫描结果去重和排序

## 15. 最终结论

本工具首版应以“mDNS 资产发现和 banner 解析”为核心，而不是以传统端口扫描为核心。

推荐先通过 mock 数据打通 MVP 主链路：

```text
参数解析 -> mock RawRecord -> ParsedRecord -> ServiceAsset -> Filtered ScanResult -> text/json 输出
```

主链路稳定后，再接入真实 mDNS 网络查询：

```text
mDNS response -> RawRecord -> 复用后续解析、聚合、过滤、输出流程
```

这样可以降低实现风险，也能确保在没有真实 mDNS 设备的测试环境中仍然可以完成演示和验收。
