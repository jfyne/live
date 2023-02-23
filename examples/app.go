package examples

import "github.com/jfyne/live/page"

type App struct {
	Button *Button

	page.BaseComponent
}

func NewApp() *App {
	return &App{
		Button: NewButton(10),
	}
}

func (a App) Render() page.RenderFunc {
	return page.HTML(`
        <!doctype html>
        <html lang="en">
        <head>
          <meta charset="utf-8">
          <title>Live examples</title>
        </head>
        <body>
          {{ .Button }}
          <script src="/live.js"></script>
        </body>
        </html>
        `, a)
}
