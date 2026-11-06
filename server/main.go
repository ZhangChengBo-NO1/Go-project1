package main

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
)

type Client struct {
	Name string
	Conn net.Conn
}

// 在线用户列表
var clients = make(map[string]*Client)
var mutex sync.Mutex

// 处理客户端

func handle(client *Client) {
	defer client.Conn.Close()
	fmt.Println("新客户端连接成功")

	for {
		buf := make([]byte, 1024)
		n, err := client.Conn.Read(buf)
		if err == io.EOF {
			fmt.Println(client.Name, "退出聊天室")

			// 删除用户
			mutex.Lock()

			delete(clients, client.Name)
			fmt.Println("当前在线人数：", len(clients))

			mutex.Unlock()
			return
		}

		if err != nil {
			fmt.Println("Read发生错误:", err)
			return
		}

		// 原始信息
		msg := strings.TrimSpace(string(buf[:n]))

		// 防止用户回车导致死循环
		if msg == "" {
			continue
		}

		// =====获取列表的操作
		if msg == "/list" {

			users := getOnlineUsers()

			client.Conn.Write([]byte(users))

			continue
		}

		//====== 退出操作
		if msg == "/quit" {
			fmt.Println(client.Name, "退出聊天室")

			// 删除用户
			mutex.Lock()

			delete(clients, client.Name)

			fmt.Println("【系统】：当前在线人数：", len(clients))

			mutex.Unlock()

			// 通知其他用户

			broadcastSystem(fmt.Sprintf("【系统】: %s退出聊天室", client.Name))
			return
		}

		// 组装信息

		message := fmt.Sprintf("%s:%s", client.Name, msg)

		fmt.Println(message)

		// 广播

		broadcast(message, client.Name)
	}
}

// 把消息发送给所有在线用户
func broadcast(msg string, sender string) {
	mutex.Lock()

	for name, client := range clients {

		if name == sender {
			continue
		}
		_, err := client.Conn.Write([]byte(msg))
		if err != nil {
			fmt.Println("向", client.Name, "发送消息失败:", err)
		}
	}
	mutex.Unlock()
}

// 把系统消息（退出）发送到在线的用户
func broadcastSystem(msg string) {
	mutex.Lock()

	defer mutex.Unlock()

	for _, client := range clients {
		_, err := client.Conn.Write([]byte(msg))

		if err != nil {

			fmt.Println(
				"发送失败:",
				err,
			)
		}
	}
}

// 返回在线用户字符串

func getOnlineUsers() string {
	var result string

	result += "当前在线用户:\n"

	mutex.Lock()
	defer mutex.Unlock()

	for name := range clients {
		result += name + "\n"
	}

	return result
}

func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:9000")
	if err != nil {
		fmt.Printf("创建监听有问题, err=%v\n", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()

		if err != nil {

			continue

		}

		// 第一次读取用户名

		buf := make([]byte, 1024)

		n, err := conn.Read(buf)

		if err != nil {

			conn.Close()

			continue

		}

		name := strings.TrimSpace(string(buf[:n]))

		// 创建用户

		client := &Client{

			Name: name,

			Conn: conn,
		}

		// 保存用户

		mutex.Lock()

		clients[name] = client

		fmt.Println(
			name,
			"加入聊天室",
		)

		fmt.Println(
			"【系统】：当前在线人数:",
			len(clients),
		)

		mutex.Unlock()

		// 通知其他用户

		broadcast(
			fmt.Sprintf(
				"【系统】：%s加入聊天室",
				name,
			),
			name,
		)

		// 开启处理

		go handle(client)
	}
}