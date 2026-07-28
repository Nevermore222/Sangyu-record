package book

import (
	"bytes"
	"html/template"
)

var manuscriptTemplate = template.Must(template.New("manuscript").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<style>
@page { size: A5; margin: 18mm 16mm 20mm; }
body { color: #222; font-family: "Noto Serif CJK SC", "Source Han Serif SC", serif; font-size: 11pt; line-height: 1.8; }
.cover { min-height: 160mm; display: flex; flex-direction: column; justify-content: center; text-align: center; page-break-after: always; }
h1 { font-size: 26pt; font-weight: 600; }
h2 { font-size: 18pt; page-break-before: always; }
p { margin: 0 0 0.9em; text-align: justify; }
.evidence { page-break-before: always; font-family: sans-serif; font-size: 8pt; color: #555; }
.evidence li { overflow-wrap: anywhere; }
</style>
</head>
<body>
<section class="cover"><h1>{{.Title}}</h1>{{if .Subtitle}}<p>{{.Subtitle}}</p>{{end}}</section>
{{range .Chapters}}
<section><h2>{{.Title}}</h2>{{range .Paragraphs}}<p>{{.Text}}</p>{{end}}</section>
{{end}}
<section class="evidence"><h2>内部来源映射</h2>
{{range .Chapters}}<h3>{{.Title}}</h3><ol>{{range .Paragraphs}}<li>{{range .EvidenceRefs}}<span>{{.}}</span> {{end}}</li>{{end}}</ol>{{end}}
</section>
</body>
</html>`))

func RenderHTML(manuscript Manuscript) (string, error) {
	var output bytes.Buffer
	if err := manuscriptTemplate.Execute(&output, manuscript); err != nil {
		return "", err
	}
	return output.String(), nil
}
