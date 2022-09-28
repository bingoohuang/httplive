package process

import (
	"runtime"
	"strings"

	"github.com/gobars/cmd"

	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/gin-gonic/gin"
)

func TrimContextPath(c *gin.Context) string {
	p := c.Request.URL.Path
	if Envs.ContextPath != "/" {
		p = strings.TrimPrefix(p, Envs.ContextPath)
	}

	return util.Or(p, "/")
}

func GetHostIps() []string {
	if runtime.GOOS == "linux" {
		_, out := cmd.Bash(`hostname -I`)
		if len(out.Stdout) > 0 {
			return strings.Fields(out.Stdout[0])
		}
	}

	return nil
}

func GetHostInfo() map[string]string {
	if runtime.GOOS == "linux" {
		_, out := cmd.Bash(`hostnamectl`)
		return ParseKvLines(out.Stdout)
	}

	return nil
}

func ParseKvLines(kvs []string) map[string]string {
	m := make(map[string]string)
	for _, kv := range kvs {
		kk := strings.SplitN(kv, ":", 2)
		if len(kk) == 2 {
			k := kk[0]
			v := kk[1]
			m[strings.TrimSpace(k)] = strings.TrimSpace(v)
		} else if len(kk) == 1 {
			k := kk[0]
			m[strings.TrimSpace(k)] = ""
		}
	}

	return m
}
