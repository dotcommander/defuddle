package defuddle

import (
	"reflect"
	"testing"
)

func TestExtractTables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		html string
		want []Table
	}{
		{
			name: "thead headers with tbody rows",
			html: `<table><caption>Scores</caption><thead><tr><th>Model</th><th>Elo</th></tr></thead><tbody><tr><td>GPT-5</td><td>88.0</td></tr><tr><td>Claude</td><td>90.1</td></tr></tbody></table>`,
			want: []Table{{
				Caption: "Scores",
				Headers: []string{"Model", "Elo"},
				Rows:    [][]string{{"GPT-5", "88.0"}, {"Claude", "90.1"}},
			}},
		},
		{
			name: "first th row as header when no thead",
			html: `<table><tr><th>Name</th><th>Score</th></tr><tr><td>a</td><td>1</td></tr></table>`,
			want: []Table{{
				Caption: "",
				Headers: []string{"Name", "Score"},
				Rows:    [][]string{{"a", "1"}},
			}},
		},
		{
			name: "whitespace collapsed in cells",
			html: "<table><thead><tr><th>  Col\n  One </th></tr></thead><tbody><tr><td> x\ty </td></tr></tbody></table>",
			want: []Table{{
				Caption: "",
				Headers: []string{"Col One"},
				Rows:    [][]string{{"x y"}},
			}},
		},
		{
			name: "no tables",
			html: `<p>nothing here</p>`,
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ExtractTables(tc.html)
			if err != nil {
				t.Fatalf("ExtractTables error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}
