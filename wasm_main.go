//go:build js && wasm

package main

// 一键成城 · 网页版入口（WebAssembly）
// 把生成核心编译进浏览器：页面调用 generateCity(种子字符串, 规模) 拿到 SVG。
// 全部计算在访客浏览器里完成，不需要任何服务器。

import (
	"strconv"
	"syscall/js"
)

func main() {
	js.Global().Set("generateCity", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) < 2 {
			return ""
		}
		seed, err := strconv.ParseInt(args[0].String(), 10, 64)
		if err != nil {
			seed = 20261120
		}
		mode := "organic"
		if len(args) > 2 {
			mode = args[2].String()
		}
		theme := ""
		if len(args) > 3 {
			theme = args[3].String()
		}
		return renderRequest(seed, args[1].String(), mode, theme)
	}))
	parse := func(args []js.Value) (int64, string, string, string, bool) {
		if len(args) < 2 {
			return 0, "", "", "", false
		}
		seed, err := strconv.ParseInt(args[0].String(), 10, 64)
		if err != nil {
			seed = 20261120
		}
		mode, theme := "organic", ""
		if len(args) > 2 {
			mode = args[2].String()
		}
		if len(args) > 3 {
			theme = args[3].String()
		}
		return seed, args[1].String(), mode, theme, true
	}
	js.Global().Set("generatePoster", js.FuncOf(func(this js.Value, args []js.Value) any {
		seed, size, mode, theme, ok := parse(args)
		if !ok {
			return ""
		}
		return renderPoster(seed, size, mode, theme)
	}))
	js.Global().Set("generateRecord", js.FuncOf(func(this js.Value, args []js.Value) any {
		seed, size, mode, theme, ok := parse(args)
		if !ok {
			return "{}"
		}
		return renderRecord(seed, size, mode, theme)
	}))
	js.Global().Set("generateScene", js.FuncOf(func(this js.Value, args []js.Value) any {
		seed, size, mode, theme, ok := parse(args)
		if !ok {
			return "{}"
		}
		return renderScene(seed, size, mode, theme)
	}))
	select {} // 常驻，等待页面调用
}
