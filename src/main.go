package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"

	"github.com/rustyoz/svg"

	"github.com/asim/quadtree"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/tdewolff/canvas/renderers/opengl"

	_ "embed"
)

const (
	Nada       = 0
	Paredes    = 1
	Divisiones = 2
	Aulas      = 3
	Puntitos   = 4
)

//go:embed embed/comic.ttf
var font []byte

type Edge struct {
	u, v int
}

var scale float64

var fontFamily *canvas.FontFamily

const fontSize = 100.0

const MAX_PUNTITOS = 1000

var aulaIds []string
var aulaIndex map[string]int
var aulaID map[int]string
var esAula map[int]bool

var t time.Time
var ts time.Time
var tf time.Duration

var dist [MAX_PUNTITOS][MAX_PUNTITOS]float64
var next [MAX_PUNTITOS][MAX_PUNTITOS]int

var doc *svg.Svg
var qtree *quadtree.QuadTree
var ps []*quadtree.Point

var tk int
var docTk int

var doRandom bool = false

var mx, my float64

// TODO: que puedan ser puntitos cualquiera
var gMouse, gSrc, gDst string

// TODO: ajustar el rendering de texto para que esto se pueda cambiar sin romper todo
const winW, winH = 1000, 500
const canvasW, canvasH = 1000, 500

func tLog(s string) {
	n := time.Now()
	if s == "" {
		tf = n.Sub(ts)
		tk = 0
		ts = n
	} else {
		ms := n.Sub(t).Microseconds()
		fmt.Printf("%-25s %7d             \n", s, ms)
	}
	t = n
	tk++
}

func render(ctx *canvas.Context, di chan *svg.DrawingInstruction, label string) *quadtree.Point {
	ps := []*quadtree.Point{}

	drawLabel := false
	done := false
	for !done {
		select {
		case ins, ok := <-di:
			if !ok {
				done = true
				break
			}
			if ins == nil {
				continue
			}
			switch ins.Kind {
			case svg.MoveInstruction:
				{
					ctx.MoveTo(ins.M[0], ins.M[1])
					ps = append(ps, quadtree.NewPoint(ins.M[0], ins.M[1], nil))
				}
			case svg.CircleInstruction:
				{
					r := *ins.Radius
					ctx.MoveTo(ins.M[0]+r, ins.M[1])
					ctx.Arc(r, r, 0, 0, 360)
				}
			case svg.CurveInstruction:
				{
					ctx.CubeTo(
						ins.CurvePoints.C1[0],
						ins.CurvePoints.C1[1],
						ins.CurvePoints.C2[0],
						ins.CurvePoints.C2[1],
						ins.CurvePoints.T[0],
						ins.CurvePoints.T[1],
					)
					ps = append(ps, quadtree.NewPoint(ins.CurvePoints.T[0], ins.CurvePoints.T[1], nil))
				}
			case svg.LineInstruction:
				{
					ctx.LineTo(ins.M[0], ins.M[1])
					ps = append(ps, quadtree.NewPoint(ins.M[0], ins.M[1], nil))
				}
			case svg.CloseInstruction:
				{
					ctx.Close()
					x, y := ctx.Pos()
					ps = append(ps, quadtree.NewPoint(x, y, nil))
				}

			case svg.PaintInstruction:
				{
					if label != "" {
						drawLabel = true
					}
					ctx.Stroke()
				}
			default:
				log.Println("wtf")
				panic(ins.Kind)
			}
		}
	}

	cx := 0.0
	cy := 0.0
	l := float64(len(ps))
	for _, p := range ps {
		x, y := p.Coordinates()
		cx += x
		cy += y
	}

	mx, my := cx/l, cy/l
	if drawLabel {
		face := fontFamily.Face(fontSize, ctx.Style.StrokeColor, canvas.FontRegular, canvas.FontNormal)
		text := canvas.NewTextBox(face, label, 0.0, 0.0, canvas.Center, canvas.Center, 0.0, 0.0)

		coordView := canvas.Identity
		coordView = coordView.ReflectYAbout(ctx.Height() * 1.45)
		coord := coordView.Mul(ctx.CoordView()).Dot(canvas.Point{X: mx + 10.0, Y: my + 25.0})
		m := ctx.View().Translate(coord.X, coord.Y)
		ctx.RenderText(text, m)
	}

	return quadtree.NewPoint(mx, my, nil)
}

func renderAula(ctx *canvas.Context, p *svg.Path) *quadtree.Point {
	di, _ := p.ParseDrawingInstructions()
	return render(ctx, di, p.ID)
}

func path(u, v int) []int {
	if next[u][v] == -1 {
		return []int{}
	}
	path := []int{u}

	for u != v {
		u = next[u][v]
		path = append(path, u)
	}
	return path
}

func renderMapa(ctx *canvas.Context, src string, dst string) {

	tLog("render begin")

	ctx.SetFillColor(color.RGBA{0xfd, 0xfd, 0xfd, 0xff})
	ctx.DrawPath(0, 0, canvas.Rectangle(ctx.Width(), ctx.Height()))
	tLog("fill")

	scale := 0.4
	xmin := 330.0
	ymin := 90.0
	ctx.SetView(canvas.Identity.Translate(0.0, 0.0).Scale(scale, scale).Translate(-xmin, -ymin))
	tLog("setview")

	fmt.Println()
	fmt.Println("(cached)")
	fmt.Println()

	if doc == nil {
		docTk = tk
		tLog("parsing svg")
		reader, err := os.Open("../data/mapa.svg")
		if err != nil {
			panic(err)
		}
		doc, err = svg.ParseSvgFromReader(reader, "mapa", 1)
		tLog("parsed svg")
		if err != nil {
			panic(err)
		}

		centerPoint := quadtree.NewPoint(0.0, 0.0, nil)
		halfPoint := quadtree.NewPoint(3000.0, 3000.0, nil)
		bb := quadtree.NewAABB(centerPoint, halfPoint)
		qtree = quadtree.New(bb, 0, nil)

		i := 0
		ps = make([]*quadtree.Point, 0)
		aulaIndex = make(map[string]int)
		esAula = make(map[int]bool)
		aulaID = make(map[int]string)

		tLog("puntitos quadtree")
		for _, e := range doc.Groups[Puntitos].Elements {
			c, ok := e.(*svg.Circle)
			if !ok {
				log.Println("Expected a circle and wasn't")
				p, wasPath := e.(*svg.Path)
				if wasPath {
					log.Println("-- path with ID:", p.ID)
				}
			}
			p := quadtree.NewPoint(c.Cx, c.Cy, i)
			ok = qtree.Insert(p)
			if !ok {
				panic("out of bounds")
			}
			ps = append(ps, p)
			i++
		}
		tLog("puntitos quadtree done")
		// log.Println(len(ps), "puntitos")

		if doc.Groups[Aulas].ID != "aulas" {
			panic(doc.Groups[Aulas].ID)
		}

		ctx.SetStrokeColor(canvas.Transparent)
		tLog("aulas quadtree")
		for _, e := range doc.Groups[Aulas].Elements {
			var p *svg.Path
			p = e.(*svg.Path)

			c := renderAula(ctx, p)

			x, y := c.Coordinates()
			qp := quadtree.NewPoint(x, y, i)
			ok := qtree.Insert(qp)
			if !ok {
				panic("out of bounds")
			}
			ps = append(ps, c)
			aulaIds = append(aulaIds, p.ID)
			aulaIndex[p.ID] = i
			aulaID[i] = p.ID
			esAula[i] = true
			i++
		}
		tLog("aulas quadtree done")

		if len(ps) > MAX_PUNTITOS {
			panic("muchos puntitos")
		}

		es := make([]*Edge, 0)

		tLog("quadtree knearest")
		for i, p := range ps {
			dist := 60.0
			knc := p
			knd := quadtree.NewPoint(dist, dist, nil)
			knbb := quadtree.NewAABB(knc, knd)

			maxPoints := 10
			for _, point := range qtree.KNearest(knbb, maxPoints, nil) {
				var j int
				j = point.Data().(int)

				// TODO: no construir ejes entre puntos vecinos de aulas distintas

				es = append(es, &Edge{i, j})
				es = append(es, &Edge{j, i})
			}
		}
		tLog("quadtree knearest done")

		tLog("floyd")
		for u := range ps {
			for v := range ps {
				dist[u][v] = 100000000000
				next[u][v] = -1
			}
		}
		tLog("floyd init")
		for _, e := range es {
			ux, uy := ps[e.u].Coordinates()
			vx, vy := ps[e.v].Coordinates()
			dist[e.u][e.v] = math.Sqrt((ux-vx)*(ux-vx) + (uy-vy)*(uy-vy)) // The weight of the edge (u, v)
			next[e.u][e.v] = e.v
		}
		tLog("floyd edges initial done")

		for i := range ps {
			dist[i][i] = 0
			next[i][i] = i
		}
		tLog("floyd self init done")

		tLog("floyd triple for ...")
		for k := range ps {
			for i := range ps {
				for j := range ps {
					if dist[i][j] > dist[i][k]+dist[k][j] {
						dist[i][j] = dist[i][k] + dist[k][j]
						next[i][j] = next[i][k]
					}
				}
			}
		}
		tLog("floyd triple for done")
		docTk = tk - docTk
	} else {
		for i := 0; i < docTk; i++ {
			fmt.Println()
		}
	}

	fmt.Println()
	fmt.Println("(cached)")
	fmt.Println()

	var di chan *svg.DrawingInstruction

	if doc.Groups[Divisiones].ID != "divisiones" {
		panic(doc.Groups[Divisiones].ID)
	}
	tLog("divisiones pdi")
	di, _ = doc.Groups[Divisiones].ParseDrawingInstructions()
	ctx.SetStrokeWidth(3.0) // divisiones finitas
	ctx.SetStrokeColor(canvas.Lightgray)
	tLog("divisiones render")
	render(ctx, di, "")
	tLog("divisiones done")

	if doc.Groups[Paredes].ID != "paredes" {
		panic(doc.Groups[Paredes].ID)
	}
	tLog("paredes pdi")
	di, _ = doc.Groups[Paredes].ParseDrawingInstructions()
	ctx.SetStrokeWidth(6.0) // paredes gruesas
	ctx.SetStrokeColor(canvas.Black)
	tLog("paredes render")
	render(ctx, di, "")
	tLog("paredes done")

	if doc.Groups[Puntitos].ID != "puntitos" {
		panic(doc.Groups[Puntitos].ID)
	}

	tLog("path find ...")
	camino := path(aulaIndex[src], aulaIndex[dst])
	tLog("path find done")

	px, py := ps[aulaIndex[src]].Coordinates()

	ctx.SetStrokeJoiner(canvas.RoundJoin)
	ctx.SetStrokeCapper(canvas.RoundCap)
	ctx.SetStrokeColor(canvas.Gold)
	ctx.SetStrokeWidth(20.0) // camino bien visible
	tLog("path stroke ...")
	ctx.MoveTo(px, py)
	for _, u := range camino {
		x, y := ps[u].Coordinates()
		ctx.LineTo(x, y)
	}
	ctx.Stroke()
	tLog("path stroke done")

	// tLog("drawing edges")
	// drawn := map[string]bool{}
	// ctx.SetStrokeWidth(4.0) // puntitos intermedios
	// ctx.SetStrokeColor(color.RGBA{0, 0, 0, 10})
	// for _, e := range es {
	// 	if e.u >= e.v {
	// 		continue
	// 	}
	// 	arc := "" + strconv.Itoa(e.u) + ":" + strconv.Itoa(e.v)
	// 	if drawn[arc] {
	// 		continue
	// 	}

	// 	x, y := ps[e.u].Coordinates()
	// 	px, py := ps[e.v].Coordinates()
	// 	ctx.MoveTo(px, py)
	// 	ctx.LineTo(x, y)
	// 	ctx.Stroke()

	// 	drawn[arc] = true
	// }
	// tLog("drawing edges done")

	// di, _ = doc.Groups[Puntitos].ParseDrawingInstructions()
	// tLog("pdi puntitos done")

	// render(ctx, di, "")
	// tLog("drawing puntitos done")

	tLog("render aulas")
	ctx.SetStrokeWidth(8.0) // aulas gruesas
	ctx.SetStrokeColor(color.RGBA{28, 28, 28, 128})
	for _, e := range doc.Groups[Aulas].Elements {
		var p *svg.Path
		p = e.(*svg.Path)

		if p.ID == src {
			ctx.SetStrokeColor(color.RGBA{160, 151, 28, 255})
		} else if p.ID == dst {
			ctx.SetStrokeColor(color.RGBA{28, 151, 160, 255})
		} else {
			ctx.SetStrokeColor(color.RGBA{28, 28, 28, 128})
		}
		renderAula(ctx, p)
	}
	tLog("render aulas done")

	mp := ctx.View().Inv().Dot(canvas.Point{mx, my})

	dist := 100.0
	knc := quadtree.NewPoint(mp.X, mp.Y, nil)
	knd := quadtree.NewPoint(dist, dist, nil)
	knbb := quadtree.NewAABB(knc, knd)
	maxPoints := 200
	var closest *quadtree.Point
	var closestD float64
	var closestA *quadtree.Point
	var closestAD float64
	for _, point := range qtree.KNearest(knbb, maxPoints, nil) {
		x, y := point.Coordinates()
		d := math.Sqrt((mp.X-x)*(mp.X-x) + (mp.Y-y)*(mp.Y-y))
		if closest == nil || d < closestD {
			closest = point
			closestD = d
		}
		if (closestA == nil || d < closestAD) && aulaID[point.Data().(int)] != "" {
			closestA = point
			closestAD = d
		}
	}

	if closest != nil {
		if closestA != nil {
			gMouse = aulaID[closestA.Data().(int)]
		} else {
			x, y := closest.Coordinates()
			r := 20.0
			ctx.SetFillColor(canvas.Gold)
			ctx.MoveTo(x+r, y)
			ctx.Arc(r, r, 0, 0, 360)
			ctx.Fill()
		}
	}

	tLog("render mouse done")
}

func genPNG(path string, src string, dst string) {
	c := canvas.New(canvasW, canvasH)
	ctx := canvas.NewContext(c)
	ctx.SetCoordSystem(canvas.CartesianIV)

	renderMapa(ctx, src, dst)

	tLog("writing png")
	renderers.Write(path, c)
	tLog("png done")
}

func onKey(w *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	if action == glfw.Press && (key == glfw.KeyEscape || key == glfw.KeyQ) {
		w.SetShouldClose(true)
	}
	if action == glfw.Press && (key == glfw.KeyR) {
		doRandom = !doRandom
	}
}

var mouseDown bool

func onMouseButton(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
	if action == glfw.Press {
		mouseDown = true
		gSrc = gMouse
	} else {
		mouseDown = false
		gDst = gMouse
	}
}

func onCursorPos(w *glfw.Window, xpos float64, ypos float64) {
	mx, my = xpos, ypos
	if mouseDown {
		gDst = gMouse
	}
}

func regenOpenGL(src string, dst string) *opengl.OpenGL {
	tLog("regenOpenGL")
	ogl := opengl.New(canvasW, canvasH, canvas.DPMM(1.0))
	tLog("new ogl")
	ctx := canvas.NewContext(ogl)
	tLog("new ctx")
	ctx.SetCoordSystem(canvas.CartesianIV)
	// Compile canvas for OpenGl
	renderMapa(ctx, src, dst)
	ogl.Compile()
	tLog("ogl compile")

	return ogl
}

func mainOpenGL() {
	runtime.LockOSThread()

	// Set up window
	if err := glfw.Init(); err != nil {
		panic(err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 2)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	width, height := winW, winH
	window, err := glfw.CreateWindow(width, height, "mapita", nil, nil)
	if err != nil {
		panic(err)
	}
	window.MakeContextCurrent()
	window.SetKeyCallback(onKey)
	window.SetMouseButtonCallback(onMouseButton)
	window.SetCursorPosCallback(onCursorPos)

	if err := gl.Init(); err != nil {
		panic(err)
	}

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	gl.ClearColor(1, 1, 1, 1)

	tLog("")
	for !window.ShouldClose() {
		gl.Clear(gl.COLOR_BUFFER_BIT)
		fmt.Print("\033[H")

		tLog("frame start")

		src := "pab1"
		dst := "kiosko"
		if aulaIds != nil {
			if doRandom {
				src = aulaIds[rand.Intn(len(aulaIds))]
				dst = aulaIds[rand.Intn(len(aulaIds))]
				for src == dst {
					dst = aulaIds[rand.Intn(len(aulaIds))]
				}
			} else {
				if gSrc != "" {
					src = gSrc
				}
				if gDst != "" {
					dst = gDst
				}
			}
		}
		// Draw compiled canvas to OpenGL
		ogl := regenOpenGL(src, dst)
		tLog("rendered")
		ogl.Draw()
		tLog("ogl.Draw()")

		glfw.PollEvents()
		tLog("PollEvents()")
		window.SwapBuffers()
		tLog("SwapBuffers()")

		tLog("")
		fmt.Println("\n frame time:", tf, "    ")
	}
	// adios
}

func main() {
	fontFamily = canvas.NewFontFamily("comic")
	if err := fontFamily.LoadFont(font, 0, canvas.FontRegular); err != nil {
		panic(err)
	}

	tLog("")
	// genPNG("mapa.png", "pab1", "pab2")
	mainOpenGL()
}
