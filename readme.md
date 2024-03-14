使用golang实现的tcp udp端口转发

目前已实现：

 - 规则热加载
 - web管理面板
 - 流量统计

**截图**

![preview](https://www.csz.net/images/goForward.png)

**使用**

Linux下载
```
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/csznet/goForward/main/get.sh)"
```
运行
```
./goForward
```

**参数**  
TCP无传输超时关闭
默认3600，单位秒
```
./goForward -tt 18000
```


自定义web管理端口

```
./goForward -port 8899
```

设置web管理访问密码

```
./goForward -pass 666
```

当24H内同一IP密码试错超过3次将会ban掉

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
ExecStart=/full/path/to/your/goForward

[Install]
WantedBy=default.target
```

其中的```/full/path/to/your/goForward```改为二进制文件地址，后面可接参数

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