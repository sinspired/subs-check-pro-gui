// Package main: instance.go
//
// 单实例检测 + 跨实例窗口唤醒。
package main

import (
	"log/slog"
	"net"
	"os"
	"time"
)

// ipcPort 固定本地 IPC 端口，专用于单实例检测与窗口唤醒。
// 选取不常见端口，降低与其他程序冲突的概率。
const ipcPort = "29741"

// ipcMagic 握手魔术字节，防止误响应其他程序的连接。
const ipcMagic = "SCP-SHOW-v1"

// showSignalCh 接收来自第二个实例的"唤醒主窗口"信号。
// main() 在创建窗口后启动监听协程，确保 win 指针已就绪。
var showSignalCh = make(chan struct{}, 1)

// ensureSingleInstance 确保只有一个程序实例运行。
// 必须在 main() 最开始（Wails 初始化之前）调用。
func ensureSingleInstance() {
	ln, err := net.Listen("tcp", "127.0.0.1:"+ipcPort)
	if err != nil {
		// 端口被占用 → 判断是否是本程序的另一个实例
		if trySignalExistingInstance() {
			slog.Debug("检测到已有实例正在运行，已发送唤醒信号，本实例退出")
			os.Exit(0)
		}
		// 无法握手 → 端口被其他进程占用，优雅降级允许多实例
		slog.Warn("单实例 IPC 端口已被其他程序占用，降级运行（允许多实例）",
			"port", ipcPort)
		return
	}
	slog.Debug("单实例锁已获取", "port", ipcPort)
	go listenInstanceSignals(ln)
}

// trySignalExistingInstance 向已有实例发送 SHOW 握手包。
// 返回 true 表示对方是本程序（握手成功）。
func trySignalExistingInstance() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+ipcPort, 800*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(800 * time.Millisecond))
	_, err = conn.Write([]byte(ipcMagic))
	return err == nil
}

// listenInstanceSignals 后台监听其他实例发来的 IPC 信号。
// 收到合法握手包后向 showSignalCh 发送信号，由 main() 侧协程唤醒窗口。
func listenInstanceSignals(ln net.Listener) {
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			// 监听器被关闭（应用退出），正常退出
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(800 * time.Millisecond))

			buf := make([]byte, len(ipcMagic)+4) // +4 字节冗余
			n, _ := c.Read(buf)
			if n >= len(ipcMagic) && string(buf[:len(ipcMagic)]) == ipcMagic {
				// 非阻塞投递信号，避免阻塞监听循环
				select {
				case showSignalCh <- struct{}{}:
				default:
				}
			}
		}(conn)
	}
}
