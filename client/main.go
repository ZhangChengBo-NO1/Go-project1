package main

import (
	"fmt"
	"io"
	"net"
)

func receiveMsg(conn net.Conn, done chan bool) {
	buf := make([]byte, 1024)

	for {
		n, err := conn.Read(buf)

		if err == io.EOF {
			fmt.Println("服务器关闭")

			done <- true
			return
		}

		if err != nil {
			fmt.Println("接收消息错误:", err)

			done <- true

			return
		}
		fmt.Println("\n收到消息:", string(buf[:n]))
	}
}


func sendMsg(conn net.Conn, done chan bool){

	for {

		var msg string

		fmt.Print("请输入你的消息:")


		_,err := fmt.Scan(&msg)


		if err != nil {
			fmt.Println("输入异常")

			done <- true

			return

		}


		_,err=conn.Write(
			[]byte(msg),
		)


		if err != nil {

			done <- true

			return
		}

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
	done := make(chan bool)

	go receiveMsg(conn, done)

	// 发送聊天信息

	go sendMsg(conn, done)

	// 等待退出

	<- done

	fmt.Println("客户端退出")
}
