/**
 * @Author: Hongker
 * @Description:
 * @File:  epollServer
 * @Version: 1.0.0
 * @Date: 2021/3/23 20:07
 */

package websocket

import (
	"log"
	"net/http"
	"sync"
)


// epollServer implement of Server
type epollServer struct {
	// 路由引擎
	engine *Engine

	// 连接回调
	connectCallback func(conn Connection)
	// 注销回调
	disconnectCallback func(conn Connection)

	epoller *epoll
}
// HandleRequest implement of Server
func (srv *epollServer) HandleRequest(w http.ResponseWriter, r *http.Request) {
	// 获取socket连接
	conn, err := newConnection(w, r)
	if err != nil {
		// do something..
		return
	}

	srv.registerConn(conn)
}

// registerConn 注册连接
func (srv *epollServer) registerConn(conn Connection) {
	if err := srv.epoller.Add(conn); err != nil {
		log.Printf("Failed to add connection")
		conn.close()
		return
	}
	// 注册回调
	if srv.connectCallback != nil {
		srv.connectCallback(conn)
	}
}
// HandleConnect implement of Server
func (srv *epollServer) HandleConnect(callback func(conn Connection)) {
	srv.connectCallback = callback
}
// HandleDisconnect implement of Server
func (srv *epollServer) HandleDisconnect(callback func(conn Connection)) {
	srv.disconnectCallback = callback
}
// Route implement of Server
func (srv *epollServer) Route(uri string, handler Handler) {
	srv.engine.route(uri, handler)
}

// Broadcast implement of Server
func (srv *epollServer) Broadcast(response Response, ignores ...string) {
	for _, conn := range srv.epoller.connections {
		// 跳过指定连接
		var skip bool
		for _, ignore := range ignores {
			if ignore == conn.ID() {
				skip = true
				break
			}
		}
		if !skip {
			if err := conn.write(response.Byte()); err != nil {
				log.Printf("write to [%s]: %v", conn.ID(), err)
			}
		}
	}
}


// Close implement of Server
func (srv *epollServer) Close(conn Connection)  {
	if err := srv.epoller.Remove(conn); err != nil {
		log.Printf("Failed to remove %v", err)
	}
	// 关闭socket
	conn.close()
	// 注销回调
	if srv.disconnectCallback != nil {
		srv.disconnectCallback(conn)
	}
}

// Start implement of Server
func (srv *epollServer) Start() {
	// 设置默认的404路由
	if srv.engine.noRoute == nil {
		srv.engine.NoRoute(notFoundHandler)
	}
	// epoll模式
	go func() {
		for {
			connections, err := srv.epoller.Wait()
			if err != nil {
				log.Printf("Failed to epoll wait %v", err)
				continue
			}
			for _, conn := range connections {
				ctx, err := conn.context()
				if err != nil {
					srv.Close(conn)
					continue
				}
				srv.engine.handle(ctx)
			}
		}
	}()
}

func EpollServer() Server {
	epoller, err := MkEpoll()
	if err != nil {
		log.Fatalf("create epoll:%v\n", err)
	}
	return &epollServer{
		engine:             &Engine{
			rmw:     sync.RWMutex{},
			routers: map[string]Handler{},
		},
		connectCallback:    nil,
		disconnectCallback: nil,
		epoller:            epoller,
	}
}