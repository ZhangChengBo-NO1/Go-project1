package main

import (
	"bufio"          // 【新增】按行读取，解决 TCP 粘包/拆包
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"           // 【新增】用于认证超时、连接池生命周期

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

type Client struct {
	Name string
	Conn net.Conn
}

var clients = make(map[string]*Client)
var mutex sync.Mutex
var db *sql.DB

// 1. 初始化数据库并建表
func initDB(dsn string) {
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("连接数据库失败: ", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal("数据库 PING 失败: ", err)
	}

	// 【新增】连接池参数：避免连接泄漏和 MySQL 主动断开后残留死连接
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	createTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INT AUTO_INCREMENT PRIMARY KEY,
		username VARCHAR(50) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`
	if _, err = db.Exec(createTable); err != nil {
		log.Fatal("建表失败: ", err)
	}
	fmt.Println("数据库连接成功，用户表已就绪！")
}

// 2. 【新增】认证逻辑
func authenticate(username, password, mode string) (bool, string) {
	fmt.Println("--> [1] 开始查询用户:", username) // 【新增】
	var storedHash string
	err := db.QueryRow("SELECT password_hash FROM users WHERE username = ?", username).Scan(&storedHash)
	fmt.Println("--> [2] 查询结束，err:", err) // 【新增】

	if mode == "register" {
		if err == nil {
			return false, "用户名已存在"
		}
		if err != sql.ErrNoRows {
			return false, "数据库错误"
		}
		
		fmt.Println("--> [3] 开始计算密码哈希（bcrypt）") // 【新增】
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return false, "密码加密失败"
		}
		fmt.Println("--> [4] 哈希计算完成，准备插入数据库") // 【新增】
		
		_, err = db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, string(hash))
		fmt.Println("--> [5] 插入完成，err:", err) // 【新增】
		
		if err != nil {
			return false, "注册失败"
		}
		return true, "注册成功"
	}

	// login 模式
	if err == sql.ErrNoRows {
		return false, "用户不存在"
	}
	// 【新增】原来是裸的 err != ErrNoRows 没处理，真正的数据库异常会被当成“密码错误”
	if err != nil {
		return false, "数据库错误"
	}
	if bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) != nil {
		return false, "密码错误"
	}
	return true, "登录成功"
}

// 【修正】handle 增加 reader 参数，复用 main 中已创建的 bufio.Reader，
// 否则认证时残留在缓冲区的数据会被丢失
func handle(client *Client, reader *bufio.Reader) {
	defer client.Conn.Close()
	fmt.Println(client.Name, "进入聊天室")
	defer func() {
		mutex.Lock()
		delete(clients, client.Name)
		count := len(clients)              // 【建议】锁内读取，锁外打印
		mutex.Unlock()
		fmt.Println(client.Name, "退出聊天室，当前在线人数：", count)
		broadcastSystem(fmt.Sprintf("【系统】: %s退出聊天室\n", client.Name))
	}()

	// 【修正】原代码用 conn.Read(buf) 一次读，超过 1024 字节会截断、多条消息会粘连
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			return
		}
		if err != nil {
			fmt.Println("Read发生错误:", err)
			return
		}

		msg := strings.TrimSpace(line)
		if msg == "" {
			continue
		}

		if msg == "/list" {
			client.Conn.Write([]byte(getOnlineUsers() + "\n"))
			continue
		}
		if msg == "/quit" {
			return
		}

		message := fmt.Sprintf("%s: %s\n", client.Name, msg)
		broadcast(message, client.Name)
	}
}

// 【修正】broadcast 收集写失败的用户，锁外统一清理，
// 避免死连接一直占着在线列表（注意 sync.Mutex 不可重入，清理必须放在解锁之后）
func broadcast(msg string, sender string) {
	mutex.Lock()
	var dead []string
	for name, client := range clients {
		if name == sender {
			continue
		}
		if _, err := client.Conn.Write([]byte(msg)); err != nil {
			dead = append(dead, name)
		}
	}
	mutex.Unlock()

	if len(dead) > 0 {
		mutex.Lock()
		for _, name := range dead {
			if c, ok := clients[name]; ok {
				delete(clients, name)
				c.Conn.Close()
				fmt.Println(name, "写入失败，已从在线列表移除")
			}
		}
		mutex.Unlock()
	}
}

// 【修正】broadcastSystem 同样处理写失败
func broadcastSystem(msg string) {
	mutex.Lock()
	var dead []string
	for name, client := range clients {
		if _, err := client.Conn.Write([]byte(msg)); err != nil {
			dead = append(dead, name)
		}
	}
	mutex.Unlock()

	if len(dead) > 0 {
		mutex.Lock()
		for _, name := range dead {
			if c, ok := clients[name]; ok {
				delete(clients, name)
				c.Conn.Close()
			}
		}
		mutex.Unlock()
	}
}

func getOnlineUsers() string {
	var result string = "当前在线用户:\n"
	mutex.Lock()
	defer mutex.Unlock()
	for name := range clients {
		result += name + "\n"
	}
	return result
}

func main() {

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("🚨 服务端发生崩溃 panic:", r)
		}
	}()

	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = "chatuser:chatpass123@tcp(127.0.0.1:3307)/chatroom?charset=utf8mb4&parseTime=True&loc=Local"
	}
	initDB(dsn)
	// 【建议】db 是包级变量，Close 放在 main 中 defer 更合理
	defer db.Close()

	listener, err := net.Listen("tcp", "0.0.0.0:9001")
	if err != nil {
		fmt.Printf("创建监听有问题, err=%v\n", err)
		return
	}
	defer listener.Close()
	fmt.Println("聊天室服务端启动，监听 9001 端口...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		// 【修正】认证包用 bufio.Reader + 10s 超时读取，避免
		// 1) AUTH 包分片到达导致 strings.Fields 解析失败
		// 2) 恶意连接不发数据一直占着 goroutine
		connReader := bufio.NewReader(conn)
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		line, err := connReader.ReadString('\n')
		_ = conn.SetReadDeadline(time.Time{}) // 清除超时，聊天阶段不限时
		if err != nil {
			fmt.Println("❌ 读取认证包失败:", err) // 【新增】打印报错
			conn.Close()
			continue
		}

		parts := strings.Fields(strings.TrimSpace(line))
		if len(parts) != 4 || parts[0] != "AUTH" {
			fmt.Println("❌ 收到格式错误的认证包:", line) // 【新增】打印出来看看收到了啥
			conn.Write([]byte("AUTH_FAIL 无效的认证请求格式\n"))
			time.Sleep(100 * time.Millisecond) // 【新增】防止 RST 覆盖数据
			conn.Close()
			continue
		}

		username, password, mode := parts[1], parts[2], parts[3]
		ok, msg := authenticate(username, password, mode)
		if !ok {
			conn.Write([]byte("AUTH_FAIL " + msg + "\n"))
			time.Sleep(100 * time.Millisecond)
			conn.Close()
			continue
		}

		// 查重
		mutex.Lock()
		if _, exists := clients[username]; exists {
			mutex.Unlock()
			conn.Write([]byte("AUTH_FAIL 该用户已在聊天室中\n"))
			time.Sleep(100 * time.Millisecond) // 【新增】防止 RST
			conn.Close()
			continue
		}
		client := &Client{Name: username, Conn: conn}
		clients[username] = client
		mutex.Unlock()

		conn.Write([]byte("AUTH_OK\n"))
		broadcast(fmt.Sprintf("【系统】：%s加入聊天室\n", username), username)
		go handle(client, connReader) // 【修正】把 reader 传进去
	}
}