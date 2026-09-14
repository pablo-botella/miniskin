package miniskin

import "testing"

func TestApplyMinify(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		level string
		ext   string
		want  string
	}{
		{
			name:  "level 0 is a no-op",
			in:    "body {\n  color: red;\n}\n",
			level: "0",
			ext:   ".css",
			want:  "body {\n  color: red;\n}\n",
		},
		{
			name:  "empty level is a no-op",
			in:    "body {\n  color: red;\n}\n",
			level: "",
			ext:   ".css",
			want:  "body {\n  color: red;\n}\n",
		},
		{
			name:  "css is minified",
			in:    "body {\n  color: red;\n}\n",
			level: "1",
			ext:   ".css",
			want:  "body{color:red}",
		},
		{
			name:  "level 2 css is minified",
			in:    "body {\n  color: red;\n}\n",
			level: "2",
			ext:   ".css",
			want:  "body{color:red}",
		},
		{
			name:  "level 1 js keeps variable names",
			in:    "function f() {\n  var longName = 1;\n  return longName;\n}\n",
			level: "1",
			ext:   ".js",
			want:  "function f(){var longName=1;return longName}",
		},
		{
			name:  "json is minified",
			in:    "{\n  \"a\": 1\n}\n",
			level: "1",
			ext:   ".json",
			want:  "{\"a\":1}",
		},
		{
			name:  "unsupported type passes through unchanged",
			in:    "plain   text\n\n  stays\n",
			level: "1",
			ext:   ".txt",
			want:  "plain   text\n\n  stays\n",
		},
		{
			name:  "extension matching is case-insensitive",
			in:    "body {\n  color: red;\n}\n",
			level: "1",
			ext:   ".CSS",
			want:  "body{color:red}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyMinify(tt.in, tt.level, tt.ext)
			if err != nil {
				t.Fatalf("applyMinify returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("applyMinify() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyMinifyUnknownLevel(t *testing.T) {
	in := "body {\n  color: red;\n}\n"
	got, err := applyMinify(in, "3", ".css")
	if err == nil {
		t.Fatal("expected error for unknown minify level")
	}
	if got != in {
		t.Errorf("content should be returned untouched on error: %q", got)
	}
}
