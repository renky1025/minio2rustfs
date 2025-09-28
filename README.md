# MinIO to RustFS Migration Tool

一个高性能、可恢复的对象迁移工具，用于将对象从 MinIO 迁移到 RustFS（S3 兼容存储）。

## 特性

- 🚀 **高并发**: 支持多 worker 并发迁移
- 🔄 **断点续传**: 支持从中断点恢复迁移
- 🛡️ **可靠性**: 自动重试机制和错误处理
- 📈 **进度显示**: 实时显示迁移进度、速度和预计时间
- 📊 **监控**: Prometheus 指标和结构化日志
- 🎯 **灵活性**: 支持按 bucket、前缀或单个对象迁移
- ✅ **校验**: 自动校验对象完整性

## 快速开始

### 构建

#### 使用 Make (Linux/macOS)
```bash
# 构建二进制文件
make build

# 清理构建文件
make clean

# 运行测试
make test

# 格式化代码
make fmt

# 代码检查
make vet

# 构建并运行
make run

# 查看所有可用命令
make help
```

#### 使用批处理脚本 (Windows)
```cmd
# 构建二进制文件
build.bat build

# 清理构建文件
build.bat clean

# 运行测试
build.bat test

# 格式化代码
build.bat fmt

# 代码检查
build.bat vet

# 构建并运行
build.bat run

# 查看所有可用命令
build.bat help
```

#### 使用 PowerShell 脚本 (Windows)
```powershell
# 构建二进制文件
.\build.ps1 build

# 带版本号构建
.\build.ps1 -Command build -Version v1.0.0

# 其他命令
.\build.ps1 clean
.\build.ps1 test
.\build.ps1 run
```

#### 手动构建
```bash
go build -o minio2rustfs ./cmd
```

### Docker 构建

```bash
# 构建 Docker 镜像
make docker
# 或者
docker build -t minio2rustfs .

# 运行 Docker 容器
make docker-run
# 或者
docker run --rm -it minio2rustfs --help
```

### 基本用法

```bash
# 迁移整个 bucket
./minio2rustfs \
  --src-endpoint http://minio:9000 \
  --src-access-key AKIA... \
  --src-secret-key ... \
  --dst-endpoint https://rustfs:443 \
  --dst-access-key RU_ACCESS \
  --dst-secret-key RU_SECRET \
  --bucket my-bucket

# 按前缀迁移
./minio2rustfs \
  --src-endpoint http://minio:9000 \
  --src-access-key AKIA... \
  --src-secret-key ... \
  --dst-endpoint https://rustfs:443 \
  --dst-access-key RU_ACCESS \
  --dst-secret-key RU_SECRET \
  --bucket my-bucket \
  --prefix logs/2025/

# 迁移单个对象
./minio2rustfs \
  --src-endpoint http://minio:9000 \
  --src-access-key AKIA... \
  --src-secret-key ... \
  --dst-endpoint https://rustfs:443 \
  --dst-access-key RU_ACCESS \
  --dst-secret-key RU_SECRET \
  --bucket my-bucket \
  --object path/to/file.txt
```

### 使用配置文件

```bash
# 复制示例配置
cp config.yaml.example config.yaml

# 编辑配置文件
vim config.yaml

# 使用配置文件运行
./minio2rustfs --config config.yaml
```

## 配置选项

### 命令行参数

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `--src-endpoint` | MinIO 端点 | - |
| `--src-access-key` | MinIO 访问密钥 | - |
| `--src-secret-key` | MinIO 密钥 | - |
| `--src-secure` | 源端使用 HTTPS | false |
| `--dst-endpoint` | RustFS 端点 | - |
| `--dst-access-key` | RustFS 访问密钥 | - |
| `--dst-secret-key` | RustFS 密钥 | - |
| `--dst-secure` | 目标端使用 HTTPS | true |
| `--bucket` | 存储桶名称 | - |
| `--prefix` | 对象前缀过滤 | - |
| `--object` | 单个对象键 | - |
| `--concurrency` | 并发 worker 数量 | 16 |
| `--multipart-threshold` | 多部分上传阈值（字节） | 104857600 |
| `--part-size` | 多部分分片大小（字节） | 67108864 |
| `--retries` | 最大重试次数 | 5 |
| `--retry-backoff-ms` | 初始重试退避时间（毫秒） | 500 |
| `--dry-run` | 仅列出对象不实际迁移 | false |
| `--checkpoint` | 检查点数据库文件路径 | ./checkpoint.db |
| `--skip-existing` | 跳过已存在且匹配的对象 | true |
| `--resume` | 从检查点恢复 | false |
| `--show-progress` | 显示进度显示（dry-run模式下自动禁用） | true |
| `--log-level` | 日志级别 | info |

### 配置文件格式

```yaml
source:
  endpoint: http://minio.local:9000
  access_key: AKIA_YOUR_ACCESS_KEY
  secret_key: your_secret_key
  secure: false

target:
  endpoint: https://rustfs.local:443
  access_key: RU_ACCESS_KEY
  secret_key: ru_secret_key
  secure: true

migration:
  bucket: my-bucket
  prefix: logs/2025/
  concurrency: 32
  multipart_threshold: 104857600  # 100MB
  part_size: 67108864             # 64MB
  retries: 5
  skip_existing: true
  show_progress: true                    # 显示进度显示
  checkpoint: ./migrate_checkpoint.db

log_level: info
```

## 进度显示

程序支持实时进度显示功能，在支持ANSI转义序列的终端中会显示详细的迁移进度信息：

### 📊 进度信息包括：
- **对象进度**：已处理/总计对象数量及百分比
- **数据进度**：已传输/总计数据量及百分比
- **详细统计**：成功、失败、跳过的对象数量
- **速度信息**：当前传输速度和平均速度
- **时间信息**：已用时间、预计剩余时间、预计完成时间

### 🎛️ 进度显示控制：
```bash
# 启用进度显示（默认）
./minio2rustfs --show-progress ...

# 禁用进度显示
./minio2rustfs --show-progress=false ...

# dry-run 模式自动禁用进度显示
./minio2rustfs --dry-run ...
```

### 📱 显示效果示例：
```
🚀 对象迁移进度
==============================================
📊 对象进度: 1250/5000 (25.0%)
    [██████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░] 25.0%
💾 数据进度: 2.1GB/8.5GB (24.7%)
    [██████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░] 24.7%

📈 详细统计:
  ✅ 成功: 1200
  ❌ 失败: 5
  ⏭️  跳过: 45

⚡ 速度信息:
  当前速度: 15.2 MB/s
  平均速度: 12.8 MB/s

⏱️  时间信息:
  已用时间: 2m45s
  预计剩余: 8m20s
  预计完成: 14:23:15

⏰ 最后更新: 14:14:55
```

## 监控

程序在 `:8080/metrics` 端点暴露 Prometheus 指标：

- `migrate_objects_total{status}`: 处理的对象总数（按状态分类）
- `migrate_bytes_total`: 迁移的总字节数
- `migrate_inflight_workers`: 当前活跃的 worker 数量
- `migrate_object_duration_seconds`: 对象迁移耗时分布

## 错误处理

- **网络错误**: 自动重试，指数退避
- **权限错误**: 记录并跳过或终止
- **对象不存在**: 记录并跳过
- **数据校验失败**: 重试或标记失败

## 性能调优

### 并发设置
- 根据网络带宽和系统资源调整 `--concurrency`
- 通常设置为 CPU 核数的 2-4 倍

### 分片大小
- 大文件使用较大的 `--part-size`（64MB-256MB）
- 小文件较多时可以降低 `--multipart-threshold`

### 网络优化
- 确保源和目标之间有足够的网络带宽
- 考虑在同一数据中心或区域运行

## 故障恢复

程序支持优雅停止和恢复：

1. 使用 `Ctrl+C` 或 `SIGTERM` 停止程序
2. 程序会保存当前进度到检查点数据库
3. 使用 `--resume` 参数重新启动以继续迁移

```bash
# 恢复迁移
./minio2rustfs --config config.yaml --resume
```

## 安全注意事项

- 不要在日志中暴露访问密钥
- 使用 HTTPS 连接生产环境
- 定期轮换访问密钥
- 确保网络连接安全

## 故障排除

### 常见问题

1. **连接超时**
   - 检查网络连接
   - 验证端点地址
   - 调整重试参数

2. **权限错误**
   - 验证访问密钥和权限
   - 确保对源和目标都有必要权限

3. **内存不足**
   - 降低并发数
   - 减小分片大小
   - 增加系统内存

### 日志分析

程序输出结构化 JSON 日志，可以使用 `jq` 等工具分析：

```bash
# 查看错误日志
./minio2rustfs --config config.yaml 2>&1 | jq 'select(.level=="error")'

# 统计迁移进度
./minio2rustfs --config config.yaml 2>&1 | jq 'select(.msg=="Task completed successfully")' | wc -l
```

## 开发

### 项目结构

```
├── cmd/                    # 主程序入口
├── internal/
│   ├── app/               # 应用逻辑
│   ├── config/            # 配置管理
│   ├── storage/           # 存储抽象
│   ├── worker/            # 工作池
│   ├── checkpoint/        # 检查点存储
│   ├── metrics/           # 监控指标
│   └── logger/            # 日志
├── config.yaml.example    # 配置示例
└── README.md
```

### 运行测试

```bash
go test ./...
```

### 构建 Docker 镜像

```bash
docker build -t minio2rustfs .
```

## 许可证

MIT License

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=renky1025/minio2rustfs&type=Date)](https://star-history.com/#renky1025/minio2rustfs&Date)
