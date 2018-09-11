# api proxy

## 介绍
简单api的请求转发，支持所有请求方式、header传递、cookie传递

## 环境变量
> MAX_REQUEST 最大并发请求数
> LISTEN_PORT 监听端口
> REQ_TIMEOUT 请求超时时间（单位：毫秒）

## 使用简介
> curl http://10.16.111.202:8080?url=https%3a%2f%2ftuke.so.com%2fapi%2fhome

url为真正请求的链接需要urlencode
其他没有任何变化，按照正常get、post传输数据即可

