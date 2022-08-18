package main

import (
	"log"
	"os"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"

	"github.com/rustyoz/svg"
)

func main() {
	c := canvas.New(1000, 500)
	ctx := canvas.NewContext(c)

	ctx.SetStrokeWidth(10.0)

	scale := 0.4
	xmin := 400.0
	ymin := 1300.0

	ctx.SetView(canvas.Identity.Translate(0.0, 0.0).Scale(scale, -scale).Translate(-xmin, -ymin))

	reader, err := os.Open("../data/mapa.svg")
	doc, err := svg.ParseSvgFromReader(reader, "test", 1)

	if err != nil {
		panic(err)
	}

	di, _ := doc.ParseDrawingInstructions()

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
			log.Println(ins.M)
			switch ins.Kind {
			case svg.MoveInstruction:
				{
					ctx.MoveTo(ins.M[0], ins.M[1])
				}
			case svg.CircleInstruction:
				{
					ctx.MoveTo(ins.M[0], ins.M[1])
					ctx.Arc(*ins.Radius, *ins.Radius, 0, 0, 360)
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
				}
			case svg.LineInstruction:
				{
					ctx.LineTo(ins.M[0], ins.M[1])
				}
			case svg.PaintInstruction:
				{
					ctx.SetFillColor(canvas.Mediumseagreen)
					ctx.SetStrokeColor(canvas.Mediumseagreen)
					ctx.Stroke()
					ctx.Fill()
				}
			default:
				log.Println("wtf")
				panic(ins.Kind)
			}
		}
	}

	renderers.Write("mapa.png", c)
}
