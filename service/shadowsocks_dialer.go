package service

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/shadowsocks/go-shadowsocks2/core"
	"github.com/shadowsocks/go-shadowsocks2/socks"
)

// shadowsocksDialer 建立到 Shadowsocks 服务器的加密 TCP 隧道，并在隧道内完成
// SOCKS5 CONNECT 握手。目标地址以域名原样发送，由 SS 服务端解析（远端 DNS）。
type shadowsocksDialer struct {
	cipher    core.Cipher
	ssAddress string
	forward   *net.Dialer
}

func newShadowsocksDialer(method, password string, host string, port int, forwardDialer *net.Dialer) (*shadowsocksDialer, error) {
	cipher, err := core.PickCipher(method, nil, password)
	if err != nil {
		return nil, fmt.Errorf("shadowsocks: %w", err)
	}
	return &shadowsocksDialer{
		cipher:    cipher,
		ssAddress: net.JoinHostPort(host, strconv.Itoa(port)),
		forward:   forwardDialer,
	}, nil
}

func (d *shadowsocksDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("shadowsocks: unsupported network %q", network)
	}
	target := socks.ParseAddr(address)
	if target == nil {
		return nil, fmt.Errorf("shadowsocks: invalid target address %q", address)
	}
	rawConn, err := d.forward.DialContext(ctx, "tcp", d.ssAddress)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = rawConn.SetDeadline(deadline)
	}
	conn := d.cipher.StreamConn(rawConn)
	if err := shadowsocksConnect(conn, target); err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

// shadowsocksConnect 将目标地址以 socks.Addr 字节序写入加密流。SS 协议
// 借用 SOCKS5 的地址编码（ATYP/ADDR/PORT）但无握手头，服务端读地址后
// 直接建立连接，且不返回任何确认，首个数据包即应用层流量。
func shadowsocksConnect(conn net.Conn, target socks.Addr) error {
	if _, err := conn.Write(target); err != nil {
		return fmt.Errorf("shadowsocks: send target address: %w", err)
	}
	return nil
}