package assets

import (
	"bytes"
	"strings"
	"testing"
)

func TestStaticAssetVersion(t *testing.T) {
	previousVersion := assetVersion
	SetVersion("3.58-deadbeef")
	t.Cleanup(func() { SetVersion(previousVersion) })

	tests := []struct {
		template string
		data     map[string]interface{}
		assets   []string
	}{
		{
			template: "index.html",
			data: map[string]interface{}{
				"settings":      map[string]interface{}{},
				"authenticated": false,
			},
			assets: []string{
				"stylesheets/bootstrap.min.css",
				"stylesheets/app.css",
				"javascripts/vue.min.js",
				"javascripts/api.js",
				"javascripts/app.js",
				"javascripts/key.js",
			},
		},
		{
			template: "login.html",
			data:     map[string]interface{}{},
			assets: []string{
				"stylesheets/bootstrap.min.css",
				"stylesheets/app.css",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.template, func(t *testing.T) {
			var output bytes.Buffer
			Render(test.template, &output, test.data)
			for _, asset := range test.assets {
				want := asset + "?v=3.58-deadbeef"
				if !strings.Contains(output.String(), want) {
					t.Errorf("missing %q", want)
				}
			}
			if strings.Contains(output.String(), "v=ranking-mode") {
				t.Error("found stale hard-coded asset version")
			}
		})
	}
}
