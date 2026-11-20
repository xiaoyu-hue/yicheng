package main

// 一键成城 · SVG 渲染层：把 City 数据画成一张羊皮纸风的古城图。
// 文字在 SVG 里以文本形式存在，由浏览器/看图软件用本机字体渲染，图更清晰。
// 配色由主题（皮肤）决定：默认「宣纸」与最初版本一致；春/秋/冬/夜只换色板，
// 不改任何几何与文字——同种子换皮肤，城的布局不变。
// City.Plain（素面）形态供郡图缩印：只画城本身，不带纸面、标题与城外景。

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
)

func f1(v float64) string { return fmt.Sprintf("%.1f", v) }

func ptsAttr(poly []Pt) string {
	var sb strings.Builder
	for i, p := range poly {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(f1(p.X))
		sb.WriteByte(',')
		sb.WriteString(f1(p.Y))
	}
	return sb.String()
}

// palette 一套皮肤的全部颜色。wards 为 nil 时用坊自带的 Fill（默认宣纸即如此）。
type palette struct {
	paper, spot, riverO, riverI, bank, ground, wardLine, building, road, roadEdge string
	wallD, wallL, gateText, wardText, title, sub, frame, fieldA, fieldB           string
	wards                                                                         map[string]string
}

func (p palette) fillFor(w Ward) string {
	if p.wards != nil {
		if fill, ok := p.wards[w.Kind]; ok {
			return fill
		}
	}
	return w.Fill
}

var palettes = map[string]palette{
	"": { // 宣纸（默认，与最初版本配色一致）
		paper: "#e7d9b8", spot: "#bea66e", riverO: "#9fbec2", riverI: "#b7d2d4",
		bank: "#968254", ground: "#eadfc0", wardLine: "#78643c", building: "#926646",
		road: "#dcc491", roadEdge: "#78643c", wallD: "#5d4a35", wallL: "#7d6750",
		gateText: "#43331f", wardText: "#4a3823", title: "#3d2c17", sub: "#6d5836",
		frame: "#8a734f", fieldA: "#c2cf9a", fieldB: "#dcc98f",
	},
	"spring": { // 春：嫩绿新纸
		paper: "#edf0d6", spot: "#c2c493", riverO: "#8fc3c9", riverI: "#aad6d8",
		bank: "#8a8455", ground: "#f4efdb", wardLine: "#6d6242", building: "#8a6a48",
		road: "#e3d096", roadEdge: "#6d6242", wallD: "#5d4a35", wallL: "#7d6750",
		gateText: "#43331f", wardText: "#44512e", title: "#37402a", sub: "#66704c",
		frame: "#8a8a5f", fieldA: "#a9c98a", fieldB: "#ddd08f",
		wards: map[string]string{"衙署": "#e3ac90", "市集": "#f7dd8e", "寺庙": "#f0d9a0",
			"学宫": "#cfdfae", "园林": "#9cc184", "驿站": "#eed9ac", "民居": "#f5ecc9",
			"水市": "#9fc6bd", "码头": "#cbb28a"},
	},
	"autumn": { // 秋：金红暖纸
		paper: "#f2e0ba", spot: "#cfa96e", riverO: "#94b4ba", riverI: "#b0ccd0",
		bank: "#96754a", ground: "#f6e7c2", wardLine: "#7d5f38", building: "#96684a",
		road: "#e8c87e", roadEdge: "#7d5f38", wallD: "#5d4030", wallL: "#7d5a40",
		gateText: "#453020", wardText: "#54381e", title: "#402c14", sub: "#7d5f36",
		frame: "#96754a", fieldA: "#d9b96a", fieldB: "#c98f4e",
		wards: map[string]string{"衙署": "#d98f66", "市集": "#f5c86e", "寺庙": "#eec27e",
			"学宫": "#e3d3a0", "园林": "#cfa15e", "驿站": "#ecd3a0", "民居": "#f2dfae",
			"水市": "#a8beb2", "码头": "#c4a06a"},
	},
	"winter": { // 冬：雪后素纸
		paper: "#e9eae3", spot: "#bfc2b4", riverO: "#a8c8d4", riverI: "#c6dde6",
		bank: "#8d8d7c", ground: "#f5f5ef", wardLine: "#6e6a5c", building: "#7d6a58",
		road: "#e4dfca", roadEdge: "#6e6a5c", wallD: "#54483c", wallL: "#6e5f4e",
		gateText: "#3e382c", wardText: "#4a463c", title: "#36342c", sub: "#6a685c",
		frame: "#8d8d7c", fieldA: "#dfe4da", fieldB: "#eae6d4",
		wards: map[string]string{"衙署": "#d8a68e", "市集": "#e8d9a8", "寺庙": "#e6d9ba",
			"学宫": "#dde2d2", "园林": "#c9d4c0", "驿站": "#e8dec2", "民居": "#eceade",
			"水市": "#b4ccd2", "码头": "#cfc0a0"},
	},
	"night": { // 夜：深蓝夜纸，灯火色的字
		paper: "#2c3850", spot: "#3d4c6a", riverO: "#33506e", riverI: "#40658a",
		bank: "#54607a", ground: "#39466a", wardLine: "#232c44", building: "#20283c",
		road: "#77683c", roadEdge: "#232c44", wallD: "#1d2536", wallL: "#2c3850",
		gateText: "#efe6cc", wardText: "#e8dfc8", title: "#f0e7cd", sub: "#b9c0d4",
		frame: "#77683c", fieldA: "#37455e", fieldB: "#3f4f66",
		wards: map[string]string{"衙署": "#8a5540", "市集": "#b08d34", "寺庙": "#9a7f3a",
			"学宫": "#5f7a58", "园林": "#4d6e4e", "驿站": "#7d6c40", "民居": "#4a5878",
			"水市": "#3f6a72", "码头": "#6e5c38"},
	},
}

// segInt 两线段求交点（水乡架桥、郡图架桥用）。
func segInt(a, b, c, d Pt) (Pt, bool) {
	r := b.sub(a)
	s := d.sub(c)
	den := r.X*s.Y - r.Y*s.X
	if math.Abs(den) < 1e-9 {
		return Pt{}, false
	}
	t := ((c.X-a.X)*s.Y - (c.Y-a.Y)*s.X) / den
	u := ((c.X-a.X)*r.Y - (c.Y-a.Y)*r.X) / den
	if t >= 0 && t <= 1 && u >= 0 && u <= 1 {
		return a.add(r.mul(t)), true
	}
	return Pt{}, false
}

func palFor(theme string) palette {
	if p, ok := palettes[theme]; ok {
		return p
	}
	return palettes[""]
}

// SVG 渲染整座城为一张 1280×960 的 SVG 图。
func (c *City) SVG() string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1280 960" width="1280" height="960">`)
	b.WriteString(c.bodySVG(palFor(c.Theme)))
	b.WriteString(`</svg>`)
	return b.String()
}

// bodySVG 城市的主体图元（不含 <svg> 外壳），郡图缩印时也用它。
func (c *City) bodySVG(p palette) string {
	var b strings.Builder
	if !c.Plain && !c.NoChrome {
		fmt.Fprintf(&b, `<rect width="1280" height="960" fill="%s"/>`, p.paper)
		// 纸纹斑点（独立种子源，与城市生成互不干扰）
		prng := rand.New(rand.NewSource(c.Seed ^ 0xBEEF))
		for i := 0; i < 260; i++ {
			x, y := prng.Float64()*1280, prng.Float64()*960
			r := 8 + prng.Float64()*34
			fmt.Fprintf(&b, `<ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="%s" opacity="0.05"/>`,
				f1(x), f1(y), f1(r), f1(r), p.spot)
		}
	}

	// 城南河
	if !c.Plain && len(c.River) > 1 {
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="52"/>`, ptsAttr(c.River), p.riverO)
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="36"/>`, ptsAttr(c.River), p.riverI)
		bank := make([]Pt, len(c.River))
		for i, pt := range c.River {
			bank[i] = Pt{pt.X, pt.Y - 30}
		}
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="3" opacity="0.55"/>`, ptsAttr(bank), p.bank)
	}

	// 城外田垄
	if !c.Plain {
		for _, fd := range c.Fields {
			fill := p.fieldA
			if fd.Tone == 1 {
				fill = p.fieldB
			}
			fmt.Fprintf(&b, `<polygon points="%s" fill="%s" opacity="0.55"/>`, ptsAttr(fd.Poly), fill)
		}
	}

	// 护城河：城界放大一圈成环；城门处架小桥
	if len(c.Boundary) > 2 {
		moat := make([]Pt, 0, len(c.Boundary)+1)
		for _, pt := range c.Boundary {
			moat = append(moat, c.Center.add(pt.sub(c.Center).mul(1.14)))
		}
		moat = append(moat, moat[0])
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="17" opacity="0.75"/>`, ptsAttr(moat), p.riverO)
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="10" opacity="0.75"/>`, ptsAttr(moat), p.riverI)
		for _, g := range c.Gates {
			a := c.Center.add(g.Mid.sub(c.Center).mul(1.02))
			d := c.Center.add(g.Mid.sub(c.Center).mul(1.3))
			fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="13"/>`,
				f1(a.X), f1(a.Y), f1(d.X), f1(d.Y), p.road)
		}
	}

	// 地面与坊
	fmt.Fprintf(&b, `<polygon points="%s" fill="%s"/>`, ptsAttr(c.Boundary), p.ground)
	for _, i := range c.Inner {
		w := c.Wards[i]
		fmt.Fprintf(&b, `<polygon points="%s" fill="%s" stroke="%s" stroke-opacity="0.35" stroke-width="1"/>`,
			ptsAttr(c.Cells[i]), p.fillFor(w), p.wardLine)
	}

	// 屋舍
	fmt.Fprintf(&b, `<g fill="%s" opacity="0.6">`, p.building)
	for _, bd := range c.Buildings {
		fmt.Fprintf(&b, `<g transform="translate(%s %s) rotate(%s)"><rect x="%s" y="%s" width="%s" height="%s"/></g>`,
			f1(bd.CX), f1(bd.CY), f1(bd.Ang*180/math.Pi),
			f1(-bd.HW), f1(-bd.HH), f1(2*bd.HW), f1(2*bd.HH))
	}
	b.WriteString(`</g>`)

	// 城外村落屋舍
	if !c.Plain {
		fmt.Fprintf(&b, `<g fill="%s" opacity="0.5">`, p.building)
		for _, bd := range c.Villages {
			fmt.Fprintf(&b, `<g transform="translate(%s %s) rotate(%s)"><rect x="%s" y="%s" width="%s" height="%s"/></g>`,
				f1(bd.CX), f1(bd.CY), f1(bd.Ang*180/math.Pi),
				f1(-bd.HW), f1(-bd.HH), f1(2*bd.HW), f1(2*bd.HH))
		}
		b.WriteString(`</g>`)
	}

	// 街道：环城道 + 四条入城大道
	if len(c.Ring) > 3 {
		ring := append(append([]Pt(nil), c.Ring...), c.Ring[0])
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="9"/>`, ptsAttr(ring), p.road)
	}
	for _, road := range c.Roads {
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="13"/>`, ptsAttr(road), p.road)
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="1" opacity="0.4"/>`, ptsAttr(road), p.roadEdge)
	}

	// 水乡运河 + 桥 + 船（仅水乡格局有 Canals）
	for _, canal := range c.Canals {
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="17" stroke-linejoin="round"/>`, ptsAttr(canal), p.riverO)
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="11" stroke-linejoin="round"/>`, ptsAttr(canal), p.riverI)
	}
	if len(c.Canals) > 0 {
		var roadLines [][]Pt
		roadLines = append(roadLines, c.Roads...)
		if len(c.Ring) > 3 {
			roadLines = append(roadLines, append(append([]Pt(nil), c.Ring...), c.Ring[0]))
		}
		var drawn []Pt
		for _, canal := range c.Canals {
			for ci := 0; ci+1 < len(canal); ci++ {
				for _, rl := range roadLines {
					for ri := 0; ri+1 < len(rl); ri++ {
						at, ok := segInt(canal[ci], canal[ci+1], rl[ri], rl[ri+1])
						if !ok {
							continue
						}
						dupe := false
						for _, q := range drawn {
							if dist(at, q) < 20 {
								dupe = true
							}
						}
						if dupe {
							continue
						}
						drawn = append(drawn, at)
						dirv := rl[ri+1].sub(rl[ri])
						l := math.Hypot(dirv.X, dirv.Y)
						if l == 0 {
							continue
						}
						dirv = dirv.mul(1 / l)
						a, d := at.add(dirv.mul(-16)), at.add(dirv.mul(16))
						fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="13"/>`,
							f1(a.X), f1(a.Y), f1(d.X), f1(d.Y), p.road)
						fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1" opacity="0.4"/>`,
							f1(a.X), f1(a.Y), f1(d.X), f1(d.Y), p.roadEdge)
					}
				}
			}
		}
		for _, bt := range c.Boats {
			fmt.Fprintf(&b, `<g transform="translate(%s %s) rotate(%s)"><path d="M-11,0 Q0,6 11,0 L7,-3.5 L-7,-3.5 Z" fill="#6e5136"/><rect x="-2.5" y="-6.5" width="5" height="4.5" fill="#8a6a48"/></g>`,
				f1(bt.CX), f1(bt.CY), f1(bt.Ang*180/math.Pi))
		}
	}

	// 城墙（双道线：深底 + 亮面）
	for _, pass := range []struct {
		color string
		width int
	}{{p.wallD, 11}, {p.wallL, 5}} {
		for _, e := range c.WallEdges {
			fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="%d"/>`,
				f1(e[0].X), f1(e[0].Y), f1(e[1].X), f1(e[1].Y), pass.color, pass.width)
		}
	}

	// 城门：缺口 + 门楼 + 门名（标在墙外侧，避免被墙压住；素面形态不标字）
	for _, g := range c.Gates {
		fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="9" fill="%s"/>`, f1(g.Mid.X), f1(g.Mid.Y), p.ground)
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="22" height="14" fill="none" stroke="%s" stroke-width="3"/>`,
			f1(g.Mid.X-11), f1(g.Mid.Y-7), p.wallD)
		if !c.Plain {
			fmt.Fprintf(&b, `<text x="%s" y="%s" font-family="serif" font-size="19" font-weight="bold" fill="%s" text-anchor="middle">%s</text>`,
				f1(g.Label.X), f1(g.Label.Y+6), p.gateText, g.Name)
		}
	}

	// 坊名（素面形态不标，郡图另有城名标注）
	if !c.Plain {
		for _, i := range c.Inner {
			w := c.Wards[i]
			size, weight := 17, "normal"
			if w.Kind == "衙署" {
				size, weight = 23, "bold"
			}
			fmt.Fprintf(&b, `<text x="%s" y="%s" font-family="serif" font-size="%d" font-weight="%s" fill="%s" text-anchor="middle" dominant-baseline="central">%s</text>`,
				f1(c.Cents[i].X), f1(c.Cents[i].Y), size, weight, p.wardText, w.Name)
		}
	}

	// 标题、指北针、落款、画框（素面/海报形态不带）
	if !c.Plain && !c.NoChrome {
		subtitle := fmt.Sprintf("一键成城 · 种子 %d · %s · %s", c.Seed, c.SizeLabel, c.ModeLabel)
		if c.ThemeLabel != "" {
			subtitle += " · " + c.ThemeLabel
		}
		fmt.Fprintf(&b, `<text x="56" y="86" font-family="serif" font-size="46" font-weight="bold" fill="%s">%s</text>`, p.title, c.Name)
		fmt.Fprintf(&b, `<text x="58" y="118" font-family="serif" font-size="19" fill="%s">%s</text>`, p.sub, subtitle)
		fmt.Fprintf(&b, `<text x="1222" y="66" font-family="serif" font-size="22" font-weight="bold" fill="%s" text-anchor="middle">北</text>`, p.title)
		fmt.Fprintf(&b, `<polygon points="1222,78 1214,98 1230,98" fill="%s"/>`, p.title)
		n := int64(len(captions))
		cap := captions[((c.Seed%n)+n)%n]
		fmt.Fprintf(&b, `<text x="58" y="922" font-family="serif" font-size="18" fill="%s">%s</text>`, p.sub, cap)
		fmt.Fprintf(&b, `<rect x="18" y="18" width="1244" height="924" fill="none" stroke="%s" stroke-width="3"/>`, p.frame)
		fmt.Fprintf(&b, `<rect x="28" y="28" width="1224" height="904" fill="none" stroke="%s" stroke-width="1"/>`, p.frame)
	}
	return b.String()
}
