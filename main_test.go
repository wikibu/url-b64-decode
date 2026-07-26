package main

import "testing"

func TestDecodeBase64UTF8(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"standard base64", "aGVsbG8gd29ybGQ=", "hello world", false},
		{"url-safe base64", "PDw_Pz4-", "<<??>>", false},
		{"utf8 chinese", "5L2g5aW9", "你好", false},
		{"surrounding whitespace", "  aGVsbG8=\n", "hello", false},
		{"invalid base64", "!!!not base64!!!", "", true},
		{"decoded not utf8", "gIE=", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeBase64UTF8(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
