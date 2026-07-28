package book

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type ChromiumEngine struct {
	endpoint string
}

func NewChromiumEngine(endpoint string) *ChromiumEngine {
	return &ChromiumEngine{endpoint: endpoint}
}

func (e *ChromiumEngine) Render(ctx context.Context, html string) ([]byte, error) {
	allocatorContext, cancelAllocator := chromedp.NewRemoteAllocator(ctx, e.endpoint)
	defer cancelAllocator()
	tabContext, cancelTab := chromedp.NewContext(allocatorContext)
	defer cancelTab()
	tabContext, cancelTimeout := context.WithTimeout(tabContext, 30*time.Second)
	defer cancelTimeout()

	dataURL := "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(html))
	var pdf []byte
	err := chromedp.Run(tabContext,
		chromedp.Navigate(dataURL),
		chromedp.ActionFunc(func(actionContext context.Context) error {
			var err error
			pdf, _, err = page.PrintToPDF().WithPrintBackground(true).Do(actionContext)
			return err
		}),
	)
	return pdf, err
}
