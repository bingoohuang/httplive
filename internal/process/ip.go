package process

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/bingoohuang/gg/pkg/goip"
	"github.com/bingoohuang/httplive/pkg/util"
	"github.com/gin-gonic/gin"
	"github.com/gobars/cmd"
)

// ProcessIP process ip request.
func ProcessIP(c *gin.Context, useJSON bool) {
	mainIP, ipList := goip.MainIP(c.Query("iface"))
	m := map[string]interface{}{
		"mainIP":   mainIP,
		"ipList":   ipList,
		"outbound": goip.Outbound(),
	}

	if v, err := goip.ListAllIPv4(); err != nil {
		m["v4error"] = err.Error()
	} else if len(v) > 0 {
		m["v4"] = v
	}

	if v, err := goip.ListAllIPv6(); err != nil {
		m["v6error"] = err.Error()
	} else if len(v) > 0 {
		m["v6"] = v
	}

	m["ifaces"] = listIfaces()
	m["more"] = moreInfo()

	_, status := cmd.Bash(`hostname -I`)
	m["hostname -I"] = strings.Join(append(status.Stdout, status.Stderr...), " ")
	_, status = cmd.Bash(`hostname -i`)
	m["hostname -i"] = strings.Join(append(status.Stdout, status.Stderr...), " ")

	if useJSON {
		c.PureJSON(http.StatusOK, m)
	} else {
		jb, _ := json.MarshalIndent(m, "", "  ")
		c.Data(http.StatusOK, util.ContentTypeText, jb)
	}
}

// listIfaces 根据mode 列出本机所有IP和网卡名称
func listIfaces() map[string]interface{} {
	list, err := net.Interfaces()
	m := map[string]interface{}{}

	if err != nil {
		m["error"] = err.Error()
		return m
	}

	type InterfaceInfo struct {
		Iface string   `json:"iface"`
		Error string   `json:"error,omitempty"`
		Addrs []string `json:"addrs,omitempty"`
	}

	interfaces := make([]InterfaceInfo, len(list))

	for i, iface := range list {
		interfaces[i] = InterfaceInfo{
			Iface: fmt.Sprintf("%+v", iface),
		}

		if iface.HardwareAddr == nil || iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback == 1 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			interfaces[i].Error = err.Error()
			continue
		}

		if len(addrs) == 0 {
			continue
		}

		filteredAddrs := make([]string, 0, len(addrs))

		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}

			if ipnet.IP.IsLoopback() {
				continue
			}

			filteredAddrs = append(filteredAddrs, addr.String())
		}

		interfaces[i].Addrs = filteredAddrs
	}

	m["data"] = interfaces

	return m
}

func moreInfo() map[string]interface{} {
	externalIP := goip.External()

	m := map[string]interface{}{
		"publicIP": externalIP,
	}

	if externalIP == "" {
		return m
	}

	eip := net.ParseIP(externalIP)
	if eip != nil {
		m["ipDecimal"] = goip.ToDecimal(net.ParseIP(externalIP))

		result := TabaoAPI(externalIP)
		if result != nil && result.Data.Country != "" {
			m["ipInfo"] = result
		}
	}

	return m
}

// nolint lll
// https://topic.alibabacloud.com/a/go-combat-golang-get-public-ip-view-intranet-ip-detect-ip-type-verify-ip-range-ip-address-string-and-int-conversion-judge-by-ip_1_38_10267608.html

// Info ...
type Info struct {
	Code int `json:"code"`
	Data IP  `json:"data"`
}

// IP ...
type IP struct {
	Country   string `json:"country"`
	CountryID string `json:"country_id"`
	Area      string `json:"area"`
	AreaID    string `json:"area_id"`
	Region    string `json:"region"`
	RegionID  string `json:"region_id"`
	City      string `json:"city"`
	CityID    string `json:"city_id"`
	Isp       string `json:"isp"`
}

// TabaoAPI ...
func TabaoAPI(ip string) *Info {
	ctx, cncl := context.WithTimeout(context.Background(), time.Second*5)
	defer cncl()

	addr := "http://ip.taobao.com/service/getIpInfo.php?ip=" + ip
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}

	defer resp.Body.Close()

	out, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var result Info

	if err := json.Unmarshal(out, &result); err != nil {
		return nil
	}

	return &result
}
