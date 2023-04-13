# MPTCP-PROXY

用 go 实现的用户态多路径 tcp 代理，内部对单个 TCP 进行分解，通过多个路径传输后，再进行排序重组。可突破单线路的带宽限制。
<!-- 
```plantuml
@startuml
(User)
(Client)
(Server)
(Target)
User -> Client: raw tcp(300Mbps)
Client -> Server: chunk 1(100Mbps)
Client -> Server: chunk 2(100Mbps)
Client -> Server: chunk 3(100Mbps)
Server -> Target: raw tcp(300Mbps)
@enduml
``` -->
![plantuml](p.svg)

## 使用

### 服务端

`mptcp-proxy -s :12345 -r 127.0.0.1:5001`

以上命令在本地监听 12345 端口，对数据进行排序后转发到 5001 端口。

### 客户端

`mptcp-proxy -c :5001 -p "10.0.0.1:12345,192.168.0.1:12345"`

以上命令在本地监听 5001 端口，将 tcp 数据打散后，通过 10.0.0.1 和 192.168.0.1 两个路径传输到服务端，

最终实现单个 tcp 连接的传输速率是两个路径之和（转发会存在部分损耗）

## 依赖

<https://github.com/getlantern/multipath>
