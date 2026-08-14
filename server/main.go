package main

import (
	"fmt"
	"io"
	"net"
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

			delete(clients,client.Name)
			fmt.Println("当前在线人数：",len(clients))

			mutex.Unlock()
			return
		} 

		 if err != nil {
			fmt.Println("Read发生错误:", err)
			return
		}
		
		// 组装信息

		message := fmt.Sprintf("%s:%s", client.Name, string(buf[:n]))

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


func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:9000")
	if err != nil {
		fmt.Printf("创建监听有问题, err=%v\n", err)
		return
	}
	defer listener.Close()

	for {
		conn,err := listener.Accept()


		if err != nil {

			continue

		}



		// 第一次读取用户名

		buf := make([]byte,1024)


		n,err := conn.Read(buf)


		if err != nil {

			conn.Close()

			continue

		}

		name := string(buf[:n])

		// 创建用户

		client := &Client{

			Name:name,

			Conn:conn,

		}

		// 保存用户

		mutex.Lock()


		clients[name] = client



		fmt.Println(
			name,
			"加入聊天室",
		)


		fmt.Println(
			"当前在线人数:",
			len(clients),
		)


		mutex.Unlock()



		// 通知其他用户

		broadcast(
			fmt.Sprintf(
				"%s加入聊天室",
				name,
			),
			name,
		)

		// 开启处理

		go handle(client)
	}
}
