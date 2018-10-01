package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

const head = "<html><body>\n"
const input = "<div><form method=\"post\" enctype=\"multipart/form-data\"><input type=\"file\" id=\"upload\" name=\"upload\"><input type=\"submit\"></form></div>\n\n"
const tail = "</body></html>"

func genHtml(infos []os.FileInfo) string {
	sort.SliceStable(infos, func(i, j int) bool {
		return (infos[i].IsDir() && !infos[j].IsDir()) || (infos[i].Name() < infos[j].Name())
	})

	var sb strings.Builder
	sb.WriteString(head)
	sb.WriteString(input)

	sb.WriteString("<div><ul>\n")
	for _, i := range infos {
		name := i.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		mode := i.Mode()
		if mode.IsDir() {
			name = name + "/"
			sb.WriteString(fmt.Sprintf("<li><a href=\"%s\">%s</a>", name, name))
		} else if mode.IsRegular() {
			sb.WriteString(fmt.Sprintf("<li><a href=\"%s\">%s</a> %d %s", name, name, i.Size(), i.ModTime()))
		}
	}
	sb.WriteString("</ul></div>\n\n")
	sb.WriteString(tail)
	return sb.String()
}
