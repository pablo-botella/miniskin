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
