package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// 聊天室本质就是给在的client转发消息
// 用哈希表来存储用户信息(用切片好像也可以)
// 客户端的信息统一用结构体保存
// 专门设置一个协程来传递信息(利用管道)

// 全局map同时读写会有冲突，可采取sync.map 或者加锁自己封装一个map结构体
// eg:
type RWMap struct { // 一个读写锁保护的线程安全的map
	sync.RWMutex // 读写锁保护下面的map字段
	m            map[int]int
}

// 新建一个RWMap
func NewRWMap(n int) *RWMap {
	return &RWMap{
		m: make(map[int]int, n),
	}
}
func (m *RWMap) Get(k int) (int, bool) { //从map中读取一个值
	m.RLock()
	defer m.RUnlock()
	v, existed := m.m[k] // 在锁的保护下从map中读取
	return v, existed
}

func (m *RWMap) Set(k int, v int) { // 设置一个键值对
	m.Lock() // 锁保护
	defer m.Unlock()
	m.m[k] = v
}

func (m *RWMap) Delete(k int) { //删除一个键
	m.Lock() // 锁保护
	defer m.Unlock()
	delete(m.m, k)
}

func (m *RWMap) Len() int { // map的长度
	m.RLock() // 锁保护
	defer m.RUnlock()
	return len(m.m)
}

func (m *RWMap) Each(f func(k, v int) bool) { // 遍历map
	m.RLock() //遍历期间一直持有读锁
	defer m.RUnlock()

	for k, v := range m.m {
		if !f(k, v) {
			return
		}
	}
}

type Client struct {
	C    chan string // 用于发送数据的管道 自己配一个管道在结构体里
	Name string      // 用户名
	Addr string      // 网络地址
}

var onlinemap = make(map[string]Client)
var message = make(chan string) // 换成main函数里局部的可以吗??? 一定要全局变量吗

func HandleConn(conn net.Conn) {
	defer conn.Close()
	// 处理用户连接
	cliAddr := conn.RemoteAddr().String()

	// 创建结构体成员
	cli := Client{make(chan string), cliAddr, cliAddr}
	onlinemap[cliAddr] = cli

	// 新开一个协程专门给客户端发送消息
	go WriteToClient(cli, conn)

	// 只告诉用户自己 他在哪里
	cli.C <- MakeMsg(cli, "I am here\n")

	// 广播某个人在线
	message <- MakeMsg(cli, "login\n")

	// 对方是否主动退出
	isQuit := make(chan struct{})
	// 对方是否主动发送消息(超时太久的可以踢掉他)
	hasData := make(chan bool)

	// 新开一个协程用于用户发言以及离线处理
	go func() {
		buf := make([]byte, 2048)
		for {
			n, _ := conn.Read(buf) // 没发送东西的时候会阻塞, 如果退出也会有返回值的
			if n == 0 {            // 对方断开或者出问题了
				isQuit <- struct{}{}
				return
			}
			// 添加功能 查询在线用户
			msg := string(buf[:n-1]) // 去掉额外的换行符(有能力的可以用正则表达式去掉)
			if msg == "who" {
				// 给当前用户发送所有在线的成员
				for _, tmp := range onlinemap {
					msg = tmp.Addr + ":" + tmp.Name + "\n"
					conn.Write([]byte(msg))
				}
			} else if len(msg) >= 8 && msg[:6] == "rename" {
				// 添加重命名功能 eg:rename|mike
				name := strings.Split(msg, "|")[1]
				cli.Name = name
				onlinemap[cliAddr] = cli
				conn.Write([]byte("rename ok\n"))
			} else {
				// 转发消息
				message <- MakeMsg(cli, string(buf[:n]))
			}
			hasData <- true // 代表用户发送过数据
		}
	}()

	// 防止主进程结束
	for {
		select {
		case <-isQuit:
			msg := MakeMsg(cli, "exit")
			delete(onlinemap, cliAddr)
			fmt.Println(msg)
			message <- msg
			return
		case <-hasData: // 不用处理
		case <-time.After(10 * time.Second): // 60s后超时
			msg := MakeMsg(cli, "time out leave \n")
			delete(onlinemap, cliAddr)
			fmt.Println(msg)
			message <- msg
			return
		}
	}
}
func MakeMsg(cli Client, msg string) (buf string) {
	// 代码复用
	buf = "[" + cli.Addr + "]" + cli.Name + ": " + msg
	return
}

func WriteToClient(cli Client, conn net.Conn) {
	for msg := range cli.C { // range管道的用法???  (一直读管道内容 永远不会结束)
		conn.Write([]byte(msg))
	}
}

func Manager() {
	// 新开一个协程,转发消息,有消息来了后给 map
	for {
		msg := <-message // 没收到消息会阻塞
		for _, cli := range onlinemap {
			cli.C <- msg // 遍历map给每个成员发送刚才的消息
		}
	}
}

func main() {
	// 监听
	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(listener)

	// 新开一个协程,转发消息,有消息来了后给map
	go Manager()

	// 主协程,循环阻塞等待用户连接
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go HandleConn(conn)
	}
}
