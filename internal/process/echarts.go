package process

import (
	"encoding/json"
	"text/template"

	"github.com/gin-gonic/gin"
)

const (
	HlEcharts = "echarts"
)

func init() {
	registerHlHandlers(HlEcharts, func() HlHandler { return &EchartConfig{} })
}

type EchartConfig struct {
	Title   string `json:"title"`
	Tooltip struct {
		Prefix string `json:"prefix"`
	} `json:"tooltip"`
	Series []struct {
		Name     string `json:"name"`
		X        string `json:"x"`
		Y        string `json:"y"`
		Selected bool   `json:"selected"`
		Tooltip  string `json:"tooltip"`
	} `json:"series"`
	CsvData      []string        `json:"csvData"`
	DataRows     json.RawMessage `json:"dataRows"`
	AfterCsvLoad []string        `json:"afterCsvLoad"`
}

var EchartsTemplate *template.Template

func (m EchartConfig) HlHandle(c *gin.Context, _ *APIDataModel, _ func(name string) string) error {
	c.Header("Content-Type", "text/html; charset=utf-8")
	return EchartsTemplate.Execute(c.Writer, m)
}
