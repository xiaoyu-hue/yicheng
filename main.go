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

//go:embed web/three.min.js
var threeJS []byte

//go:embed web/orbit-controls.js
var orbitJS []byte

//go:embed web/scene3d.js
var sceneJS []byte

func main() {
	addr := flag.String("addr", ":8077", "网页服务监听地址")
	seed := flag.Int64("seed", 20261120, "种子（同种子同城）")
	size := flag.String("size", "city", "规模：town 小城 / city 中城 / capital 大城")
	mode := flag.String("mode", "organic", "格局：organic 山水 / grid 棋盘 / water 水乡 / region 一郡之地")
	theme := flag.String("theme", "", "皮肤：空 宣纸 / spring 春 / autumn 秋 / winter 冬 / night 夜")
	poster := flag.Bool("poster", false, "配合 -out：导出竖版海报（含城谱）而非地图")
	scene := flag.Bool("scene", false, "配合 -out：导出立体沙盘的场景 JSON 而非地图")
	out := flag.String("out", "", "把 SVG 写到该文件后退出")
	check := flag.Bool("check", false, "自检同种子同城后退出")
	flag.Parse()

	if *check {
		okAll := true
		for _, m := range []string{"organic", "grid", "water"} {
			a := GenerateMode(*seed, *size, m).SVG()
			b := GenerateMode(*seed, *size, m).SVG()
			ha, hb := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
			fmt.Printf("[%s] 第 1 遍 sha256: %x\n[%s] 第 2 遍 sha256: %x\n", m, ha, m, hb)
			if ha != hb {
				okAll = false
			}
		}
		for _, sd := range []int64{42, 20261120} {
			a := GenerateRegion(sd, "city", "").SVG()
			b := GenerateRegion(sd, "city", "").SVG()
			if a != b {
				fmt.Printf("自检失败：郡图（种子 %d）两遍生成不一致！\n", sd)
				okAll = false
			}
		}
		if okAll {
			fmt.Println("自检通过：三种格局与郡图下，同一颗种子两遍生成都一字不差。")
			return
		}
		fmt.Println("自检失败：同种子生成结果不一致！")
		os.Exit(1)
	}
	if *out != "" {
		svg := renderRequest(*seed, *size, *mode, *theme)
		if *poster {
			svg = renderPoster(*seed, *size, *mode, *theme)
		}
		if *scene {
			svg = renderScene(*seed, *size, *mode, *theme)
		}
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
	for path, blob := range map[string][]byte{
		"/three.min.js": threeJS, "/orbit-controls.js": orbitJS, "/scene3d.js": sceneJS,
	} {
		blob := blob
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			_, _ = w.Write(blob)
		})
	}
	mux.HandleFunc("/api/scene", func(w http.ResponseWriter, r *http.Request) {
		s, _ := strconv.ParseInt(r.URL.Query().Get("seed"), 10, 64)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, renderScene(s, r.URL.Query().Get("size"), r.URL.Query().Get("mode"), r.URL.Query().Get("theme")))
	})
	mux.HandleFunc("/api/poster", func(w http.ResponseWriter, r *http.Request) {
		s, _ := strconv.ParseInt(r.URL.Query().Get("seed"), 10, 64)
		svg := renderPoster(s, r.URL.Query().Get("size"), r.URL.Query().Get("mode"), r.URL.Query().Get("theme"))
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		fmt.Fprint(w, svg)
	})
	mux.HandleFunc("/api/record", func(w http.ResponseWriter, r *http.Request) {
		s, _ := strconv.ParseInt(r.URL.Query().Get("seed"), 10, 64)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, renderRecord(s, r.URL.Query().Get("size"), r.URL.Query().Get("mode"), r.URL.Query().Get("theme")))
	})
	mux.HandleFunc("/api/city", func(w http.ResponseWriter, r *http.Request) {
		s, err := strconv.ParseInt(r.URL.Query().Get("seed"), 10, 64)
		if err != nil {
			s = 20261120
		}
		svg := renderRequest(s, r.URL.Query().Get("size"), r.URL.Query().Get("mode"), r.URL.Query().Get("theme"))
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
