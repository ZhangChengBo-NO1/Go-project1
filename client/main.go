package main

import (
	"fmt"
	"io"
	"net"
)

func receiveMsg(conn net.Conn) {
	buf := make([]byte, 1024)

	for {
		n, err := conn.Read(buf)

		if err == io.EOF {
			fmt.Println("服务器关闭")
		}
		if err != nil {
			fmt.Println("接收消息错误:", err)
			return
		}
		fmt.Println("\n收到消息:", string(buf[:n]))
	}
}

func main() {
	conn, err := net.Dial(
		"tcp",
		"127.0.0.1:9000",
	)

	if err != nil {
		fmt.Println("连接服务器出错", err)
		return
	}
	defer conn.Close()
	fmt.Println("连接服务器成功")


	// 第一步：输入用户名
	var name string
	fmt.Print("请输入用户名：")
	fmt.Scan(&name)

	// 发送用户名
	_, err = conn.Write([]byte(name))
	if err != nil {
		fmt.Println("发送用户名失败:", err)
		return
	}

	fmt.Println("欢迎来到聊天室:", name)


	// 开启接收信息

	go receiveMsg(conn)


	// 发送聊天信息

	for {
		var msg string
		fmt.Print("请输入你的消息: ")
		_, err := fmt.Scan(&msg)
		if err != nil {
			fmt.Println("输入异常")
			break
		}
		_, err = conn.Write([]byte(msg))

		if err != nil {
			fmt.Println("发送失败", err)
			return
		}
	}
}
