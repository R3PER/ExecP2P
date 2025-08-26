package ui

import webview "github.com/webview/webview_go"

// WebviewAdapter dostosowuje oryginalny webview.WebView do naszego interfejsu webView
type WebviewAdapter struct {
	original webview.WebView
}

// NewWebviewAdapter tworzy nowy adapter dla webview.WebView
func NewWebviewAdapter(w webview.WebView) *WebviewAdapter {
	return &WebviewAdapter{original: w}
}

func (a *WebviewAdapter) Destroy() {
	a.original.Destroy()
}

func (a *WebviewAdapter) Dispatch(fn func()) {
	a.original.Dispatch(fn)
}

func (a *WebviewAdapter) Eval(js string) {
	a.original.Eval(js)
}

func (a *WebviewAdapter) SetHtml(html string) {
	a.original.SetHtml(html)
}

// SetSize konwertuje int na webview.Hint
func (a *WebviewAdapter) SetSize(width int, height int, hint int) {
	a.original.SetSize(width, height, webview.Hint(hint))
}

func (a *WebviewAdapter) SetTitle(title string) {
	a.original.SetTitle(title)
}

func (a *WebviewAdapter) Terminate() {
	a.original.Terminate()
}

func (a *WebviewAdapter) Bind(name string, fn interface{}) error {
	return a.original.Bind(name, fn)
}

func (a *WebviewAdapter) Run() {
	a.original.Run()
}
