# go-chat-server
go 语言开发聊天服务器
项目描述：使用Go以及net包实现并发聊天服务器。
项目内容： 利用net包、goroutine实现多用户并行聊天服务器，用户可以连接到服务器实现聊天室功能，支持传输文件、发送消息、上线通知、修改用户名、查询在线用户、超时退出等。通过全局map来保存用户数据，使用读写锁控制并发修改访问全局map，通过管道进行消息的收发，设置定时器实现长时间不在线自动退出等功能。
涉及技术：网络通信、并发编程、管道通信。
