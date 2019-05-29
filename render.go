package main

import (
	"html"    // EscapeString
	"strconv" // Itoa
)

type Renderer interface {
	render() string
}

type TextRenderer string

type HTMLRenderer string

type AttachmentRenderer struct {
	data     []byte
	filename string
}

func (r TextRenderer) render() string {
	return `<p class="plain">` + string(r) + "</p>"
}

func (r HTMLRenderer) render() string {
	return `<iframe style="height: 100%; width: 100%;" src="data:text/html;charset=utf-8,` + html.EscapeString(string(r)) + `"></iframe>`
}

func (r AttachmentRenderer) render() string {
	return "<p>Attachment: " + r.filename + " (" + strconv.Itoa(len(r.data)) + " bytes)</p>"
}
