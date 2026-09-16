package cmd

import "testing"

func TestParseSlackMessageURLHTTPAndHTTPS(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		channel string
		ts      string
		wantErr bool
	}{
		{name: "https", input: "https://workspace.slack.com/archives/C123/p1705312365000100", channel: "C123", ts: "1705312365.000100"},
		{name: "http", input: "http://workspace.slack.com/archives/C123/p1705312365000100?thread_ts=1705312365.000100", channel: "C123", ts: "1705312365.000100"},
		{name: "ftp rejected", input: "ftp://workspace.slack.com/archives/C123/p1705312365000100", wantErr: true},
		{name: "relative rejected", input: "//workspace.slack.com/archives/C123/p1705312365000100", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel, ts, err := ParseSlackMessageURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSlackMessageURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if channel != tt.channel || ts != tt.ts {
				t.Fatalf("ParseSlackMessageURL() = %q, %q; want %q, %q", channel, ts, tt.channel, tt.ts)
			}
		})
	}
}
