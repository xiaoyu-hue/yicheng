//go:build !(js && wasm)

package main

// 一键成城 · 入口（本地服务版；网页版入口在 wasm_main.go）
// 用法：
//   yicheng                 启动网页服务（默认 http://localhost:8077）
//   yicheng -seed 42 -out city.svg   把一座城导出为 SVG 文件
//   yicheng -check          自检：同一颗种子生成两遍，比对结果哈希

import (
	"crypto/sha256"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
)

//go:embed web/index.html
var indexHTML []byte

func main() {
	addr := flag.String("addr", ":8077", "网页服务监听地址")
	seed := flag.Int64("seed", 20261120, "种子（同种子同城）")
	size := flag.String("size", "city", "规模：town 小城 / city 中城 / capital 大城")
	mode := flag.String("mode", "organic", "格局：organic 山水 / grid 棋盘")
	out := flag.String("out", "", "把 SVG 写到该文件后退出")
	check := flag.Bool("check", false, "自检同种子同城后退出")
	flag.Parse()

	if *check {
		okAll := true
		for _, m := range []string{"organic", "grid"} {
			a := GenerateMode(*seed, *size, m).SVG()
			b := GenerateMode(*seed, *size, m).SVG()
			ha, hb := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
			fmt.Printf("[%s] 第 1 遍 sha256: %x\n[%s] 第 2 遍 sha256: %x\n", m, ha, m, hb)
			if ha != hb {
				okAll = false
			}
		}
		if okAll {
			fmt.Println("自检通过：两种格局下，同一颗种子两遍生成的城都一字不差。")
			return
		}
		fmt.Println("自检失败：同种子生成结果不一致！")
		os.Exit(1)
	}
	if *out != "" {
		svg := GenerateMode(*seed, *size, *mode).SVG()
		if err := os.WriteFile(*out, []byte(svg), 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("已生成 %s（种子 %d）\n", *out, *seed)
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
	mux.HandleFunc("/api/city", func(w http.ResponseWriter, r *http.Request) {
		s, err := strconv.ParseInt(r.URL.Query().Get("seed"), 10, 64)
		if err != nil {
			s = 20261120
		}
		svg := GenerateMode(s, r.URL.Query().Get("size"), r.URL.Query().Get("mode")).SVG()
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		_, _ = w.Write([]byte(svg))
	})
	url := "http://localhost" + *addr
	log.Printf("一键成城已启动：%s", url)
	go openBrowser(url) // 双击运行时自动打开浏览器；打不开也不影响服务
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// openBrowser 尽力用系统默认浏览器打开页面，失败静默（无桌面环境时常见）。
func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd, args = "open", []string{url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	_ = exec.Command(cmd, args...).Start()
}
