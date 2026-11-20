package main

// 一键成城 · 入口
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
	"strconv"
)

//go:embed web/index.html
var indexHTML []byte

func main() {
	addr := flag.String("addr", ":8077", "网页服务监听地址")
	seed := flag.Int64("seed", 20261120, "种子（同种子同城）")
	size := flag.String("size", "city", "规模：town 小城 / city 中城 / capital 大城")
	out := flag.String("out", "", "把 SVG 写到该文件后退出")
	check := flag.Bool("check", false, "自检同种子同城后退出")
	flag.Parse()

	if *check {
		a := Generate(*seed, *size).SVG()
		b := Generate(*seed, *size).SVG()
		ha := sha256.Sum256([]byte(a))
		hb := sha256.Sum256([]byte(b))
		fmt.Printf("第 1 遍 sha256: %x\n第 2 遍 sha256: %x\n", ha, hb)
		if ha == hb {
			fmt.Println("自检通过：同一颗种子，两遍生成的城一字不差。")
			return
		}
		fmt.Println("自检失败：同种子生成结果不一致！")
		os.Exit(1)
	}
	if *out != "" {
		svg := Generate(*seed, *size).SVG()
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
		svg := Generate(s, r.URL.Query().Get("size")).SVG()
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		_, _ = w.Write([]byte(svg))
	})
	log.Printf("一键成城已启动：http://localhost%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
