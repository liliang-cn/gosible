# Gosinble 常用内置操作指南

## 🎯 核心模块 (Core Modules)

### 1. **文件管理 (File)**
```bash
# 创建目录
gosinble -m file -a "path=/opt/app state=directory mode=0755"

# 创建文件
gosinble -m file -a "path=/etc/app.conf state=file mode=0644"

# 创建软链接
gosinble -m file -a "src=/opt/app/bin path=/usr/local/bin/app state=link"

# 删除文件/目录
gosinble -m file -a "path=/tmp/old state=absent"

# 修改权限和所有者
gosinble -m file -a "path=/opt/app mode=0755 owner=app group=app"
```

### 2. **服务管理 (Service)**
```bash
# 启动/停止服务
gosinble -m service -a "name=nginx state=started"
gosinble -m service -a "name=apache2 state=stopped"

# 重启/重载服务
gosinble -m service -a "name=nginx state=restarted"
gosinble -m service -a "name=nginx state=reloaded"

# 开机自启
gosinble -m service -a "name=nginx enabled=true"

# systemd daemon-reload
gosinble -m service -a "name=myapp daemon_reload=true state=restarted"
```

### 3. **包管理 (Package)**
```bash
# 安装软件包
gosinble -m package -a "name=nginx state=present"

# 安装多个包
gosinble -m package -a "name=git,vim,curl state=present"

# 卸载软件包
gosinble -m package -a "name=apache2 state=absent"

# 更新到最新版本
gosinble -m package -a "name=docker state=latest update_cache=true"
```

### 4. **用户管理 (User)**
```bash
# 创建用户
gosinble -m user -a "name=appuser uid=1001 shell=/bin/bash"

# 创建系统用户
gosinble -m user -a "name=myservice system=true shell=/bin/false"

# 添加到组
gosinble -m user -a "name=john groups=sudo,docker append=true"

# 删除用户（包括家目录）
gosinble -m user -a "name=olduser state=absent remove=true"

# 设置密码
gosinble -m user -a "name=john password=secretpass"
```

### 5. **组管理 (Group)**
```bash
# 创建组
gosinble -m group -a "name=developers gid=2001"

# 创建系统组
gosinble -m group -a "name=myservice system=true"

# 删除组
gosinble -m group -a "name=oldgroup state=absent"
```

### 6. **命令执行 (Command/Shell)**
```bash
# 执行命令
gosinble -m command -a "cmd='ls -la /opt'"

# 执行Shell命令（支持管道、重定向）
gosinble -m shell -a "cmd='cat /etc/passwd | grep root'"

# 带工作目录
gosinble -m command -a "cmd=make chdir=/opt/app"

# 条件执行
gosinble -m command -a "cmd='rm -rf /tmp/cache' creates=/tmp/cache"
```

### 7. **文件复制 (Copy)**
```bash
# 复制文件到远程
gosinble -m copy -a "src=/local/file.txt dest=/remote/file.txt mode=0644"

# 备份原文件
gosinble -m copy -a "src=app.conf dest=/etc/app.conf backup=true"

# 直接写入内容
gosinble -m copy -a "content='Hello World' dest=/tmp/hello.txt"
```

### 8. **模板渲染 (Template)**
```bash
# 渲染模板文件
gosinble -m template -a "src=nginx.conf.j2 dest=/etc/nginx/nginx.conf"

# 带变量渲染
gosinble -m template -a 'src=app.yml.j2 dest=/opt/app/config.yml vars={"port":8080,"debug":false}'

# 备份原文件
gosinble -m template -a "src=config.j2 dest=/etc/app.conf backup=true"
```

### 9. **调试输出 (Debug)**
```bash
# 输出消息
gosinble -m debug -a "msg='Current status: OK'"

# 输出变量
gosinble -m debug -a "var=ansible_facts"
```

### 10. **连接测试 (Ping)**
```bash
# 测试连接
gosinble -m ping
```

### 11. **系统信息收集 (Setup)**
```bash
# 收集系统facts
gosinble -m setup

# 只收集特定信息
gosinble -m setup -a "gather_subset=network"
```

## 🔧 高级任务库 (Advanced Task Library)

### 文件任务 (FileTasks)
```go
tasks := library.NewFileTasks()

// 确保文件存在并设置内容
tasks.EnsureFile("/etc/app.conf", "config content", 0644)

// 备份文件
tasks.BackupFile("/etc/important.conf", "/backup/")

// 模板配置
tasks.TemplateConfig("nginx.conf.j2", "/etc/nginx/nginx.conf", vars)

// 同步目录
tasks.SyncDirectory("/source/", "/dest/")

// 清理旧文件
tasks.CleanOldFiles("/var/log/", 7) // 7天
```

### 服务任务 (ServiceTasks)
```go
tasks := library.NewServiceTasks()

// 安装二进制为系统服务
tasks.InstallBinaryAsService("myapp", "/opt/myapp/bin", systemdTemplate)

// 确保服务运行
tasks.EnsureServiceRunning("nginx")

// 重载服务配置
tasks.ReloadServiceConfig("nginx")
```

### 包管理任务 (PackageTasks)
```go
tasks := library.NewPackageTasks()

// 安装依赖包
tasks.InstallDependencies([]string{"git", "build-essential", "curl"})

// 确保包版本
tasks.EnsurePackageVersion("docker-ce", "20.10.0")

// 从URL安装
tasks.InstallFromURL("https://example.com/package.deb")
```

### 网络任务 (NetworkTasks)
```go
tasks := library.NewNetworkTasks()

// 配置防火墙
tasks.ConfigureFirewall([]int{80, 443}, "tcp")

// SSH加固
tasks.HardenSSH()

// 配置代理
tasks.SetupProxy("http://proxy.company.com:8080")
```

## 📦 内容分发 (Content Distribution)

### 嵌入式内容
```go
//go:embed templates/*
var templatesFS embed.FS

content := library.NewEmbeddedContent(templatesFS, "templates")

// 部署嵌入的文件
content.DeployFile("config.yml", "/etc/app/config.yml")

// 部署整个目录
content.DeployDirectory("configs", "/etc/app/")
```

### 外部文件分发
```go
dist := library.NewDistributionTasks()

// 注册文件
dist.RegisterFile("binary", "/local/app", 0755)

// 分发到远程
dist.DistributeBinary("binary", "/opt/app/bin/app", true)

// 并行分发多个文件
dist.ParallelDistribute([]string{"file1", "file2", "file3"}, 5)
```

## 🎭 Playbook 功能

### 条件执行
```yaml
- name: Install on Ubuntu
  package:
    name: nginx
  when: ansible_os_family == "Debian"
```

### 循环操作
```yaml
- name: Create users
  user:
    name: "{{ item }}"
    state: present
  loop:
    - alice
    - bob
    - charlie
```

### 处理器 (Handlers)
```yaml
handlers:
  - name: restart nginx
    service:
      name: nginx
      state: restarted
```

### 标签 (Tags)
```yaml
- name: Install web server
  package:
    name: nginx
  tags:
    - web
    - nginx
```

## 🔐 安全功能

### Ansible Vault 加密
```go
vault := vault.New("password")

// 加密敏感数据
encrypted := vault.Encrypt([]byte("secret"))

// 解密
decrypted := vault.Decrypt(encrypted)
```

## 🚀 并发执行
```go
runner := runner.NewTaskRunner()

// 设置并发数
runner.SetMaxConcurrency(10)

// 并行执行任务
runner.ExecuteTasks(ctx, tasks, inventory)
```

## 📋 模块参数速查表

### File 模块
| 参数 | 说明 | 示例 |
|------|------|------|
| path | 文件路径 | `/tmp/test` |
| state | 状态 | `file`, `directory`, `link`, `absent`, `touch` |
| mode | 权限 | `0755` |
| owner | 所有者 | `root` |
| group | 组 | `root` |
| src | 链接源 | `/opt/app` |
| recurse | 递归 | `true` |
| force | 强制 | `true` |

### Service 模块
| 参数 | 说明 | 示例 |
|------|------|------|
| name | 服务名 | `nginx` |
| state | 状态 | `started`, `stopped`, `restarted`, `reloaded` |
| enabled | 开机启动 | `true` |
| daemon_reload | 重载daemon | `true` |

### Package 模块
| 参数 | 说明 | 示例 |
|------|------|------|
| name | 包名 | `nginx,vim,git` |
| state | 状态 | `present`, `absent`, `latest` |
| update_cache | 更新缓存 | `true` |

### User 模块
| 参数 | 说明 | 示例 |
|------|------|------|
| name | 用户名 | `john` |
| state | 状态 | `present`, `absent` |
| uid | 用户ID | `1001` |
| gid | 组ID | `1001` |
| groups | 附加组 | `sudo,docker` |
| append | 追加组 | `true` |
| home | 家目录 | `/home/john` |
| shell | Shell | `/bin/bash` |
| password | 密码 | `encrypted_pass` |
| system | 系统用户 | `true` |
| remove | 删除家目录 | `true` |

### Group 模块
| 参数 | 说明 | 示例 |
|------|------|------|
| name | 组名 | `developers` |
| state | 状态 | `present`, `absent` |
| gid | 组ID | `2001` |
| system | 系统组 | `true` |

## 🎯 常见使用场景

### 1. 批量部署应用
```bash
# 创建应用目录
gosinble -i inventory.yml -m file -a "path=/opt/myapp state=directory" -hosts all

# 复制应用文件
gosinble -i inventory.yml -m copy -a "src=myapp dest=/opt/myapp/myapp mode=0755" -hosts all

# 创建服务文件
gosinble -i inventory.yml -m template -a "src=myapp.service.j2 dest=/etc/systemd/system/myapp.service" -hosts all

# 启动服务
gosinble -i inventory.yml -m service -a "name=myapp state=started enabled=true daemon_reload=true" -hosts all
```

### 2. 系统初始化
```bash
# 更新包缓存
gosinble -i inventory.yml -m package -a "update_cache=true" -hosts all

# 安装基础包
gosinble -i inventory.yml -m package -a "name=vim,git,curl,wget,htop state=present" -hosts all

# 创建管理用户
gosinble -i inventory.yml -m user -a "name=admin groups=sudo shell=/bin/bash" -hosts all

# 配置SSH
gosinble -i inventory.yml -m file -a "path=/home/admin/.ssh state=directory mode=0700" -hosts all
```

### 3. 安全加固
```bash
# 禁用root SSH登录
gosinble -i inventory.yml -m shell -a "cmd='sed -i s/PermitRootLogin yes/PermitRootLogin no/ /etc/ssh/sshd_config'" -hosts all

# 重启SSH服务
gosinble -i inventory.yml -m service -a "name=ssh state=restarted" -hosts all

# 配置防火墙
gosinble -i inventory.yml -m shell -a "cmd='ufw allow 22/tcp && ufw allow 80/tcp && ufw allow 443/tcp && ufw --force enable'" -hosts all
```

---
*这些内置操作覆盖了系统管理的绝大部分场景，从基础的文件操作到复杂的服务部署，都可以轻松处理！*