package book

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestHTMLIncludesEvidenceMapAndEscapesText(t *testing.T) {
	manuscript := Manuscript{
		Title: "岁月留声 <script>alert(1)</script>",
		Chapters: []Chapter{{
			Title: "纺织厂的日子",
			Paragraphs: []Paragraph{{
				Text:         "1978年，她进入了当地纺织厂。",
				EvidenceRefs: []string{"audio-fixture#12-20"},
			}},
		}},
	}
	html, err := RenderHTML(manuscript)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "audio-fixture#12-20") {
		t.Fatal("rendered HTML omitted internal evidence map")
	}
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Fatal("rendered HTML did not escape model text")
	}
}

func TestChromiumEngineRendersPDF(t *testing.T) {
	endpoint := os.Getenv("TEST_CHROMIUM_URL")
	if endpoint == "" {
		t.Skip("TEST_CHROMIUM_URL is not set")
	}
	pdf, err := NewChromiumEngine(endpoint).Render(context.Background(), "<!doctype html><meta charset=utf-8><h1>岁月留声</h1>")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		t.Fatalf("PDF prefix = %q", pdf[:min(5, len(pdf))])
	}
}
