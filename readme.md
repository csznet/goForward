使用 golang 实现的 tcp udp 端口转发

目前已实现：

- 规则热加载
- web 管理面板
- 流量统计

支持：Linux、Windows、MacOS（MacOS 需要自行编译）

**截图**

![image](https://github.com/csznet/goForward/assets/127601663/2f7840ff-9b34-4f69-a7c1-41feb35e726b)

**使用**

Linux 下载

```
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/csznet/goForward/main/get.sh)"
```

运行

```
./goForward
```

**参数**

TCP 无传输超时关闭
默认 60，单位秒

```
./goForward -tt 18000
```

自定义 web 管理端口

```
./goForward -port 8899
```

指定 IP 绑定

```
./goForward -ip 1.1.1.1
```

设置 web 管理访问密码

```
./goForward -pass 666
```

当 24H 内同一 IP 密码试错超过 3 次将会 ban 掉

## 开机自启

**创建 Systemd 服务**

```
sudo nano /etc/systemd/system/goForward.service
```

**输入内容**

```
[Unit]
Description=Start goForward on boot

[Service]
ExecStart=/full/path/to/your/goForward -pass 666

[Install]
WantedBy=default.target
```

其中的`/full/path/to/your/goForward`改为二进制文件地址，后面可接参数

**重新加载 Systemd 配置**

```
sudo systemctl daemon-reload
```

**启用服务**

```
sudo systemctl enable goForward
```

**启动服务**

```
sudo systemctl start goForward
```

**检查状态**

```
sudo systemctl status goForward.service
```
