# aict

[English](README_en.md)

> 正在开发中

Another Internet Control Message Tunnel But Featuring Firewall Bypass

## 简介

AICT是一个使用 ICMP 包绕过防火墙和 NAT 的工具，他最初作为紧急运维一些位于学校网络中的服务器的一个 fallback 策略开发的。

早先他是一个[gost](https://github.com/go-gost/gost)插件，但是感觉写着不太爽，所以现在是一个独立的工具。

## 原理

一些防火墙对 ICMP 有比较严格的限制，如果你发了一个 ICMP 出去，那么只有回来的第一个 ICMP 包才能通过防火墙，这就导致了现成的 ICMP 隧道工具都无法使用。

AICT 为了在这个限制下建立 icmp 隧道，它在服务器端维护了一个 seq 的队列，会存储来自客户端的 seq 编号以备使用。

而在客户端，他需要保证对面队列里的 seq 数量，当猜测到对面的 seq 队列比较空的时候，会发送 keepalive 补充。

## 使用

服务端这边需要先关闭 linux 的 icmp reply

```bash
echo 1 > /proc/sys/net/ipv4/icmp_echo_ignore_all
```

### client
```bash
./aict -c -r remote_ip
```

### server
```bash
./aict -s
```

如果在 windows 上使用，且开启了tun模式，需要 wintun.dll，可以在[这里](https://www.wintun.net/)下载，放在同个文件夹下。