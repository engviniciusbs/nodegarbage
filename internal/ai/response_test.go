package ai

import (
	"errors"
	"testing"
)

const goodJSON = `{"items":[{"path":"C:\\x","verdict":"safe","reason":"cache","confidence":0.9}],"report_markdown":"# R"}`

func TestParseAnalysis(t *testing.T) {
	cases := map[string]string{
		"pure":            goodJSON,
		"fence":           "```json\n" + goodJSON + "\n```",
		"prefix":          "Aqui está:\n" + goodJSON + "\nFim.",
		"brace in string": `Resultado: {"items":[{"path":"C:\\a{b","verdict":"keep","reason":"tem } e \" aspas {","confidence":1}],"report_markdown":"x"}`,
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			a, err := ParseAnalysis(in)
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Items) != 1 || a.ReportMarkdown == "" {
				t.Fatalf("bad analysis: %+v", a)
			}
		})
	}
	a, _ := ParseAnalysis(goodJSON)
	if a.Items[0].Path != `C:\x` || a.Items[0].Verdict != "safe" || a.Items[0].Confidence != 0.9 {
		t.Fatalf("%+v", a.Items[0])
	}
}

func TestParseAnalysisInvalid(t *testing.T) {
	for _, in := range []string{"", "sem json aqui", "{", `{"items":[],"report_markdown":""}`, "{}", `{"a":`} {
		if _, err := ParseAnalysis(in); !errors.Is(err, ErrInvalidResponse) {
			t.Errorf("%q: got %v", in, err)
		}
	}
}
