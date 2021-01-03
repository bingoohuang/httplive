package process

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/bingoohuang/ip"
	"github.com/hexops/valast"
)

// SimplifyValAst simplifies the v's ast representation.
func SimplifyValAst(v interface{}) string {
	// WARNING: valast.String is very slow, only for debugging/testing or limited ui access.
	s := valast.StringWithOptions(v, &valast.Options{Unqualify: true})
	p := regexp.MustCompile(`: "|\["|",|"}|"]|[\r\n\t]`)
	return p.ReplaceAllStringFunc(s, func(r string) string {
		if strings.HasPrefix(r, `"`) {
			return r[1:]
		} else if strings.HasSuffix(r, `"`) {
			return r[:len(r)-1]
		}

		return ""
	})
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
			Iface: SimplifyValAst(iface),
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
	externalIP := ip.External()

	m := map[string]interface{}{
		"publicIP": externalIP,
	}

	if externalIP == "" {
		return m
	}

	eip := net.ParseIP(externalIP)
	if eip != nil {
		result := TabaoAPI(externalIP)
		if result != nil && result.Data.Country != "" {
			m["ipInfo"] = result
		}
	}

	m["ipDecimal"] = ip.ToDecimal(net.ParseIP(externalIP))

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
