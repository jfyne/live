package examples

import (
	"github.com/jfyne/live/page"
)

type Button struct {
	Count int

	page.BaseComponent
}

func NewButton(start int) *Button {
	return &Button{Count: start}
}

func (b Button) Render() page.RenderFunc {
	return page.HTML(`
        {{.Count}}
    `, b)
}
