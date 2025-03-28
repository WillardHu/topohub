# 交换机 ZTP 配置

## 功能

ZTP（Zero Touch Provisioning）是交换机的零触碰配置，通过 DHCP server 的 `dhcp-option=67` 参数来实现。

## 配置

1. 在安装 topohub 的 subnet 实例时， 确保为交换机子网提供服务的实例中，打开 `spec.feature.enableZtp = true` 

2. 准备交换机的 ZTP 配置文件，命名为 ztp.json 

3. 通过 topohub 的 file browser 服务，上传 ZTP 配置文件到 http/ztp/ztp.json

4. 交换机在第一次接入该子网后，会自动尝试通过 DHCP 服务来获取 IP 地址和 ZTP 配置

