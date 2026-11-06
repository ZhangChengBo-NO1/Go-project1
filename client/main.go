package main

import (
	"bufio"          // 【新增】按行读取，解决截断/粘包
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

// 【修正】接收消息改用 bufio.Reader，避免：
// 1) 一次 Read 超过 1024 字节被截断
// 2) 连续多条消息粘在一起
func receiveMsg(conn net.Conn, reader *bufio.Reader, done chan<- bool) {
	for {
		line, err := reader.ReadString('\n')
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
		fmt.Print(line)
	}
}

// 【修正】原代码忽略 ReadString 的错误，用户按 Ctrl+D (stdin EOF) 后
// 会得到空字符串，进 continue 无限空转刷满一个 CPU 核
func sendMsg(conn net.Conn, reader *bufio.Reader, done chan<- bool) {
	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			// stdin 关闭，主动通知服务端退出
			conn.Write([]byte("/quit\n"))
			done <- true
			return
		}
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}
		if _, err := conn.Write([]byte(msg + "\n")); err != nil {
			fmt.Println("发送消息失败:", err)
			done <- true
			return
		}
	}
}

func main() {
	// 【修正】原来地址硬编码 127.0.0.1:9000，客户端进容器后
	// 127.0.0.1 指向的是客户端容器自己，连不上服务端
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = "127.0.0.1:9001"
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fmt.Println("连接服务器出错", err)
		return
	}
	defer conn.Close()

	// 【修正】用一个 bufio.Reader 包住 conn，认证和后续接收都复用它，
	// 避免残留数据丢失
	connReader := bufio.NewReader(conn)

	// 【修正】stdin 用独立的 reader，避免和 conn 的 reader 混用
	stdinReader := bufio.NewReader(os.Stdin)

	fmt.Print("请输入用户名：")
	name, _ := stdinReader.ReadString('\n')
	name = strings.TrimSpace(name)

	fmt.Print("请输入密码：")
	password, _ := stdinReader.ReadString('\n')
	password = strings.TrimSpace(password)

	fmt.Print("登录(login)还是注册(register)？：")
	mode, _ := stdinReader.ReadString('\n')
	mode = strings.TrimSpace(mode)

	// 发送认证包
	authMsg := fmt.Sprintf("AUTH %s %s %s\n", name, password, mode)
	if _, err = conn.Write([]byte(authMsg)); err != nil {
		fmt.Println("发送认证失败:", err)
		return
	}

	// 【修正】按行读认证结果。原来一次 Read 若只收到 "AUTH_O"，
	// 判不出 AUTH_FAIL，会误进聊天流程
	replyLine, err := connReader.ReadString('\n')
	if err != nil {
		// 【修改】即使报错，也看看有没有收到服务端的错误提示
    if replyLine != "" {
        fmt.Print("服务器返回: ", replyLine)
    }
    fmt.Println("读取认证结果失败:", err)
    return
	}
	reply := strings.TrimSpace(replyLine)
	if strings.HasPrefix(reply, "AUTH_FAIL") {
		fmt.Println("认证失败:", reply)
		return
	}

	fmt.Println("认证成功！欢迎来到聊天室，", name)
	fmt.Println("输入消息开始聊天，输入 /list 查看在线用户，输入 /quit 退出")

	// 【修正】buffered channel：receiveMsg 和 sendMsg 都可能向 done 写入，
	// 而 main 只读一次，未缓冲时另一个 goroutine 会永久阻塞
	done := make(chan bool, 2)
	go receiveMsg(conn, connReader, done)
	go sendMsg(conn, stdinReader, done)
	<-done
}