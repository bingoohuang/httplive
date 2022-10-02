package process

import (
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/bingoohuang/gg/pkg/emb"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

func init() {
	registerHlHandlers("websocket", func() HlHandler { return &websocketHandler{} })
}

type websocketHandler struct{}

func (w websocketHandler) HlHandle(c *gin.Context, _ *APIDataModel, _ func(name string) string) error {
	if c.Query("websocket") != "" {
		handleConnections(c.Writer, c.Request)
		return nil
	}

	if c.Request.Method == "GET" {
		base := filepath.Base(c.Request.URL.Path)
		switch {
		case strings.HasSuffix(base, ".js"):
			emb.ServeFile(subStatic, base, c.Writer, c.Request)
		default:
			emb.ServeFile(subStatic, "websocket.html", c.Writer, c.Request)
		}
	}

	return nil
}

type Message struct {
	Message string `json:"message"`
}

// 注册成为 websocket
func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	msg := Message{Message: "First blood at " + time.Now().Format("2006-01-02 15:04:05")}
	if err := ws.WriteJSON(msg); err != nil {
		log.Printf("error: %v", err)
		return
	}

	for {
		time.Sleep(3 * time.Second)
		// 接收数据
		var msgRecv Message
		if err := ws.ReadJSON(&msgRecv); err != nil {
			log.Printf("error: %v", err)
			break
		}
		log.Println(msgRecv)

		msg := Message{Message: msgRecv.Message + " at " + time.Now().Format("2006-01-02 15:04:05")}
		if err := ws.WriteJSON(msg); err != nil {
			log.Printf("error: %v", err)
			break
		}
	}
}

/*
Nginx configuration:

http {
    map $http_upgrade $connection_upgrade {
        default upgrade;
        '' close;
    }

    upstream websocket {
        server 192.168.100.10:8010;
    }

    server {
        listen 8020;
        location / {
            proxy_pass http://websocket;
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection $connection_upgrade;
            proxy_set_header Host $host;
        }
    }
}
*/
