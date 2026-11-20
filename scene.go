package main

// 一键成城 · 立体沙盘的场景数据（5.0）。
// 核心只负责把一座城的几何与配色导成 JSON；三维搭建在页面层（Three.js）。
// 生成逻辑不因此改动，2D 输出不受影响。郡图暂不提供沙盘（返回 region 标记）。

import "encoding/json"

type sceneWard struct {
	Poly []Pt   `json:"poly"`
	Kind string `json:"kind"`
	Name string `json:"name"`
	Fill string `json:"fill"`
}

type sceneGate struct {
	Mid  Pt     `json:"mid"`
	Name string `json:"name"`
}

type sceneData struct {
	Region    bool              `json:"region,omitempty"`
	Name      string            `json:"name"`
	Theme     string            `json:"theme"`
	Colors    map[string]string `json:"colors"`
	Boundary  []Pt              `json:"boundary"`
	Wards     []sceneWard       `json:"wards"`
	WallEdges [][2]Pt           `json:"wallEdges"`
	Gates     []sceneGate       `json:"gates"`
	Roads     [][]Pt            `json:"roads"`
	Ring      []Pt              `json:"ring"`
	River     []Pt              `json:"river"`
	Canals    [][]Pt            `json:"canals"`
	Buildings []Building        `json:"buildings"`
	Fields    []Field           `json:"fields"`
}

// renderScene 场景 JSON 统一入口。
func renderScene(seed int64, size, mode, theme string) string {
	if mode == "region" {
		return `{"region":true}`
	}
	c := GenerateFull(seed, size, mode, theme)
	p := palFor(c.Theme)
	d := sceneData{
		Name: c.Name, Theme: c.Theme, Boundary: c.Boundary, WallEdges: c.WallEdges,
		Roads: c.Roads, Ring: c.Ring, River: c.River, Canals: c.Canals,
		Buildings: c.Buildings, Fields: c.Fields,
		Colors: map[string]string{
			"paper": p.paper, "ground": p.ground, "water": p.riverO,
			"wall": p.wallD, "wallTop": p.wallL, "road": p.road, "building": p.building,
			"fieldA": p.fieldA, "fieldB": p.fieldB,
			"roof": "#8a4a38", "roofBig": "#74382a",
		},
	}
	for _, i := range c.Inner {
		w := c.Wards[i]
		d.Wards = append(d.Wards, sceneWard{c.Cells[i], w.Kind, w.Name, p.fillFor(w)})
	}
	for _, g := range c.Gates {
		d.Gates = append(d.Gates, sceneGate{g.Mid, g.Name})
	}
	data, _ := json.Marshal(d)
	return string(data)
}
