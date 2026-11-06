package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
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
		fmt.Println( string(buf[:n]))
	}
}


func sendMsg(conn net.Conn, reader *bufio.Reader,done chan bool){

	for {
		fmt.Print("请输入你的消息:")
		msg, _ := reader.ReadString('\n')
		msg = strings.TrimSpace(msg) // 去掉首尾空格和换行

		if msg == "" {
			continue
		}

		// 必须加上 \n，否则服务端无法区分消息边界
		_, err := conn.Write([]byte(msg + "\n"))
		if err != nil {
			fmt.Println("发送消息失败:", err)
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
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("请输入用户名：")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	// 发送用户名
	_, err = conn.Write([]byte(name + "\n"))
	if err != nil {
		fmt.Println("发送用户名失败:", err)
		return
	}

	fmt.Println("欢迎来到聊天室:", name)

	// 开启接收信息
	done := make(chan bool)

	go receiveMsg(conn, done)

	// 发送聊天信息

	go sendMsg(conn,reader, done)

	// 等待退出

	<- done

	fmt.Println("客户端退出")
}