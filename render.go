package main

// 一键成城 · SVG 渲染层：把 City 数据画成一张羊皮纸风的古城图。
// 文字在 SVG 里以文本形式存在，由浏览器/看图软件用本机字体渲染，图更清晰。

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

// SVG 渲染整座城为一张 1280×960 的 SVG 图。
func (c *City) SVG() string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1280 960" width="1280" height="960">`)
	b.WriteString(`<rect width="1280" height="960" fill="#e7d9b8"/>`)

	// 纸纹斑点（独立种子源，与城市生成互不干扰）
	prng := rand.New(rand.NewSource(c.Seed ^ 0xBEEF))
	for i := 0; i < 260; i++ {
		x, y := prng.Float64()*1280, prng.Float64()*960
		r := 8 + prng.Float64()*34
		fmt.Fprintf(&b, `<ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="#bea66e" opacity="0.05"/>`,
			f1(x), f1(y), f1(r), f1(r))
	}

	// 城南河
	if len(c.River) > 1 {
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="#9fbec2" stroke-width="52"/>`, ptsAttr(c.River))
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="#b7d2d4" stroke-width="36"/>`, ptsAttr(c.River))
		bank := make([]Pt, len(c.River))
		for i, p := range c.River {
			bank[i] = Pt{p.X, p.Y - 30}
		}
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="#968254" stroke-width="3" opacity="0.55"/>`, ptsAttr(bank))
	}

	// 城外田垄
	for _, fd := range c.Fields {
		fill := "#c2cf9a"
		if fd.Tone == 1 {
			fill = "#dcc98f"
		}
		fmt.Fprintf(&b, `<polygon points="%s" fill="%s" opacity="0.55"/>`, ptsAttr(fd.Poly), fill)
	}

	// 护城河：城界放大一圈成环；城门处架小桥
	if len(c.Boundary) > 2 {
		moat := make([]Pt, 0, len(c.Boundary)+1)
		for _, p := range c.Boundary {
			moat = append(moat, c.Center.add(p.sub(c.Center).mul(1.14)))
		}
		moat = append(moat, moat[0])
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="#9fbec2" stroke-width="17" opacity="0.75"/>`, ptsAttr(moat))
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="#b7d2d4" stroke-width="10" opacity="0.75"/>`, ptsAttr(moat))
		for _, g := range c.Gates {
			a := c.Center.add(g.Mid.sub(c.Center).mul(1.02))
			d := c.Center.add(g.Mid.sub(c.Center).mul(1.3))
			fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="#dcc491" stroke-width="13"/>`,
				f1(a.X), f1(a.Y), f1(d.X), f1(d.Y))
		}
	}

	// 地面与坊
	fmt.Fprintf(&b, `<polygon points="%s" fill="#eadfc0"/>`, ptsAttr(c.Boundary))
	for _, i := range c.Inner {
		w := c.Wards[i]
		fmt.Fprintf(&b, `<polygon points="%s" fill="%s" stroke="#78643c" stroke-opacity="0.35" stroke-width="1"/>`,
			ptsAttr(c.Cells[i]), w.Fill)
	}

	// 屋舍
	b.WriteString(`<g fill="#926646" opacity="0.6">`)
	for _, bd := range c.Buildings {
		fmt.Fprintf(&b, `<g transform="translate(%s %s) rotate(%s)"><rect x="%s" y="%s" width="%s" height="%s"/></g>`,
			f1(bd.CX), f1(bd.CY), f1(bd.Ang*180/math.Pi),
			f1(-bd.HW), f1(-bd.HH), f1(2*bd.HW), f1(2*bd.HH))
	}
	b.WriteString(`</g>`)

	// 城外村落屋舍
	b.WriteString(`<g fill="#926646" opacity="0.5">`)
	for _, bd := range c.Villages {
		fmt.Fprintf(&b, `<g transform="translate(%s %s) rotate(%s)"><rect x="%s" y="%s" width="%s" height="%s"/></g>`,
			f1(bd.CX), f1(bd.CY), f1(bd.Ang*180/math.Pi),
			f1(-bd.HW), f1(-bd.HH), f1(2*bd.HW), f1(2*bd.HH))
	}
	b.WriteString(`</g>`)

	// 街道：环城道 + 四条入城大道
	if len(c.Ring) > 3 {
		ring := append(append([]Pt(nil), c.Ring...), c.Ring[0])
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="#dcc491" stroke-width="9"/>`, ptsAttr(ring))
	}
	for _, road := range c.Roads {
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="#dcc491" stroke-width="13"/>`, ptsAttr(road))
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="#78643c" stroke-width="1" opacity="0.4"/>`, ptsAttr(road))
	}

	// 城墙（双道线：深底 + 亮面）
	for _, pass := range []struct {
		color string
		width int
	}{{"#5d4a35", 11}, {"#7d6750", 5}} {
		for _, e := range c.WallEdges {
			fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="%d"/>`,
				f1(e[0].X), f1(e[0].Y), f1(e[1].X), f1(e[1].Y), pass.color, pass.width)
		}
	}

	// 城门：缺口 + 门楼 + 门名（标在墙外侧，避免被墙压住）
	for _, g := range c.Gates {
		fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="9" fill="#eadfc0"/>`, f1(g.Mid.X), f1(g.Mid.Y))
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="22" height="14" fill="none" stroke="#5d4a35" stroke-width="3"/>`,
			f1(g.Mid.X-11), f1(g.Mid.Y-7))
		fmt.Fprintf(&b, `<text x="%s" y="%s" font-family="serif" font-size="19" font-weight="bold" fill="#43331f" text-anchor="middle">%s</text>`,
			f1(g.Label.X), f1(g.Label.Y+6), g.Name)
	}

	// 坊名
	for _, i := range c.Inner {
		w := c.Wards[i]
		size, weight := 17, "normal"
		if w.Kind == "衙署" {
			size, weight = 23, "bold"
		}
		fmt.Fprintf(&b, `<text x="%s" y="%s" font-family="serif" font-size="%d" font-weight="%s" fill="#4a3823" text-anchor="middle" dominant-baseline="central">%s</text>`,
			f1(c.Cents[i].X), f1(c.Cents[i].Y), size, weight, w.Name)
	}

	// 标题、指北针、落款、画框
	fmt.Fprintf(&b, `<text x="56" y="86" font-family="serif" font-size="46" font-weight="bold" fill="#3d2c17">%s</text>`, c.Name)
	fmt.Fprintf(&b, `<text x="58" y="118" font-family="serif" font-size="19" fill="#6d5836">一键成城 · 种子 %d · %s · %s</text>`, c.Seed, c.SizeLabel, c.ModeLabel)
	b.WriteString(`<text x="1222" y="66" font-family="serif" font-size="22" font-weight="bold" fill="#3d2c17" text-anchor="middle">北</text>`)
	b.WriteString(`<polygon points="1222,78 1214,98 1230,98" fill="#3d2c17"/>`)
	n := int64(len(captions))
	cap := captions[((c.Seed%n)+n)%n]
	fmt.Fprintf(&b, `<text x="58" y="922" font-family="serif" font-size="18" fill="#6d5836">%s</text>`, cap)
	b.WriteString(`<rect x="18" y="18" width="1244" height="924" fill="none" stroke="#8a734f" stroke-width="3"/>`)
	b.WriteString(`<rect x="28" y="28" width="1224" height="904" fill="none" stroke="#8a734f" stroke-width="1"/>`)
	b.WriteString(`</svg>`)
	return b.String()
}
