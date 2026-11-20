package main

// 一键成城 · 城谱与海报（4.1）。
// 城谱：从生成数据里数出来的真数字（坊数、城周、水道……），城与郡各一份。
// 海报：竖版 900×1350 分享图——图在上、谱在下、落款印章，配色随皮肤。

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
)

type statKV struct{ K, V string }

func polyLen(line []Pt) float64 {
	total := 0.0
	for i := 0; i+1 < len(line); i++ {
		total += dist(line[i], line[i+1])
	}
	return total
}

// liOf 图上长度换算成「里」（示意比例：200 图距 = 1 里）。
func liOf(v float64) string { return fmt.Sprintf("%.1f 里", v/200) }

func (c *City) countKind(kind string) int {
	n := 0
	for _, w := range c.Wards {
		if w.Kind == kind {
			n++
		}
	}
	return n
}

// Stats 单城的城谱（六项）。
func (c *City) Stats() []statKV {
	wall := 0.0
	for _, e := range c.WallEdges {
		wall += dist(e[0], e[1])
	}
	water := polyLen(c.River)
	for _, canal := range c.Canals {
		water += polyLen(canal)
	}
	return []statKV{
		{"坊", fmt.Sprintf("%d", len(c.Inner))},
		{"市集", fmt.Sprintf("%d", c.countKind("市集"))},
		{"城周", liOf(wall)},
		{"屋舍", fmt.Sprintf("%d", len(c.Buildings))},
		{"城门", fmt.Sprintf("%d", len(c.Gates))},
		{"水道", liOf(water)},
	}
}

// Line 单城的一句档案语。
func (c *City) Line() string {
	wall := 0.0
	for _, e := range c.WallEdges {
		wall += dist(e[0], e[1])
	}
	return fmt.Sprintf("城周%s，%d 坊环列、四门洞开，%d 市鼎沸，屋舍 %d 间。",
		liOf(wall), len(c.Inner), c.countKind("市集"), len(c.Buildings))
}

// Stats 郡的城谱（六项）。
func (r *Region) Stats() []statKV {
	roads := 0.0
	for _, rd := range r.Roads {
		roads += polyLen(rd)
	}
	river := 0.0
	if len(r.Rivers) > 0 {
		river = polyLen(r.Rivers[0])
	}
	return []statKV{
		{"城池", fmt.Sprintf("%d", len(r.Towns))},
		{"辖县", fmt.Sprintf("%d", len(r.Towns)-1)},
		{"村镇", fmt.Sprintf("%d", len(r.Villages))},
		{"官道", liOf(roads)},
		{"大河", liOf(river)},
		{"田野", fmt.Sprintf("%d", len(r.Fields))},
	}
}

// Line 郡的一句档案语。
func (r *Region) Line() string {
	roads := 0.0
	for _, rd := range r.Roads {
		roads += polyLen(rd)
	}
	river := 0.0
	if len(r.Rivers) > 0 {
		river = polyLen(r.Rivers[0])
	}
	return fmt.Sprintf("一郡之地：%d 城相望、%d 镇星罗，官道%s、大河%s。",
		len(r.Towns), len(r.Villages), liOf(roads), liOf(river))
}

type recordJSON struct {
	Name  string      `json:"name"`
	Line  string      `json:"line"`
	Stats [][2]string `json:"stats"`
}

func toRecord(name, line string, stats []statKV) string {
	rec := recordJSON{Name: name, Line: line}
	for _, s := range stats {
		rec.Stats = append(rec.Stats, [2]string{s.K, s.V})
	}
	data, _ := json.Marshal(rec)
	return string(data)
}

// renderRecord 城谱 JSON（页面展示用）。
func renderRecord(seed int64, size, mode, theme string) string {
	if mode == "region" {
		r := GenerateRegion(seed, size, theme)
		return toRecord(r.Name, r.Line(), r.Stats())
	}
	c := GenerateFull(seed, size, mode, theme)
	return toRecord(c.Name, c.Line(), c.Stats())
}

// posterShell 海报版式：标题、图（body 已去边框）、城谱、落款与印章。
func posterShell(p palette, seed int64, title, subtitle string, stats []statKV, line, body string) string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 900 1350" width="900" height="1350">`)
	fmt.Fprintf(&b, `<rect width="900" height="1350" fill="%s"/>`, p.paper)
	prng := rand.New(rand.NewSource(seed ^ 0xBEEF))
	for i := 0; i < 200; i++ {
		x, y := prng.Float64()*900, prng.Float64()*1350
		rr := 8 + prng.Float64()*34
		fmt.Fprintf(&b, `<ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="%s" opacity="0.05"/>`,
			f1(x), f1(y), f1(rr), f1(rr), p.spot)
	}
	fmt.Fprintf(&b, `<text x="60" y="118" font-family="serif" font-size="64" font-weight="bold" fill="%s">%s</text>`, p.title, title)
	fmt.Fprintf(&b, `<text x="62" y="162" font-family="serif" font-size="23" fill="%s">%s</text>`, p.sub, subtitle)
	b.WriteString(`<g transform="translate(40 196) scale(0.64)">`)
	b.WriteString(body)
	b.WriteString(`</g>`)
	// 城谱：两列三行
	for i, s := range stats {
		x := 60 + (i%2)*410
		y := 906 + (i/2)*96
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-family="serif" font-size="20" fill="%s">%s</text>`, x, y, p.sub, s.K)
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-family="serif" font-size="34" font-weight="bold" fill="%s">%s</text>`, x, y+42, p.title, s.V)
	}
	fmt.Fprintf(&b, `<text x="60" y="1224" font-family="serif" font-size="25" fill="%s">%s</text>`, p.wardText, line)
	fmt.Fprintf(&b, `<text x="60" y="1284" font-family="serif" font-size="19" fill="%s">一键成城 · 一砖一瓦皆由算法排布</text>`, p.sub)
	// 印章
	b.WriteString(`<g transform="translate(772 1158) rotate(-4)"><rect width="76" height="76" rx="9" fill="#b03a2e"/>`)
	fmt.Fprintf(&b, `<text x="38" y="36" font-family="serif" font-size="27" font-weight="bold" fill="#f7ead8" text-anchor="middle">成</text>`)
	fmt.Fprintf(&b, `<text x="38" y="66" font-family="serif" font-size="27" font-weight="bold" fill="#f7ead8" text-anchor="middle">城</text></g>`)
	fmt.Fprintf(&b, `<rect x="16" y="16" width="868" height="1318" fill="none" stroke="%s" stroke-width="3"/>`, p.frame)
	fmt.Fprintf(&b, `<rect x="25" y="25" width="850" height="1300" fill="none" stroke="%s" stroke-width="1"/>`, p.frame)
	b.WriteString(`</svg>`)
	return b.String()
}

// Poster 单城海报（调用前应已置 NoChrome）。
func (c *City) Poster() string {
	p := palFor(c.Theme)
	subtitle := fmt.Sprintf("一键成城 · 种子 %d · %s · %s", c.Seed, c.SizeLabel, c.ModeLabel)
	if c.ThemeLabel != "" {
		subtitle += " · " + c.ThemeLabel
	}
	return posterShell(p, c.Seed, c.Name, subtitle, c.Stats(), c.Line(), c.bodySVG(p))
}

// Poster 郡图海报（调用前应已置 NoChrome）。
func (r *Region) Poster() string {
	p := palFor(r.Theme)
	subtitle := fmt.Sprintf("一键成城 · 种子 %d · %s · 一郡之地", r.Seed, r.SizeLabel)
	if r.ThemeLabel != "" {
		subtitle += " · " + r.ThemeLabel
	}
	return posterShell(p, r.Seed, r.Name, subtitle, r.Stats(), r.Line(), r.bodySVG(p))
}

// renderPoster 海报统一入口。
func renderPoster(seed int64, size, mode, theme string) string {
	if mode == "region" {
		r := GenerateRegion(seed, size, theme)
		r.NoChrome = true
		return r.Poster()
	}
	c := GenerateFull(seed, size, mode, theme)
	c.NoChrome = true
	return c.Poster()
}
