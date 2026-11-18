package client

import (
	"strings"
	"testing"
)

func TestMaskBody(t *testing.T) {
	cases := []struct {
		name string
		in   string
		// substrings that must NOT survive the redaction
		mustHide []string
		// substrings that MUST appear (e.g. the redaction marker, structural keys)
		mustHave []string
	}{
		{
			name:     "private key",
			in:       `{"privateKey":"-----BEGIN PRIVATE KEY-----\nABCDEFG\n-----END PRIVATE KEY-----"}`,
			mustHide: []string{"BEGIN PRIVATE KEY", "ABCDEFG"},
			mustHave: []string{`"privateKey":"<<REDACTED>>"`},
		},
		{
			name:     "client secret camelCase",
			in:       `{"clientSecret":"shh-this-is-secret"}`,
			mustHide: []string{"shh-this-is-secret"},
			mustHave: []string{`"clientSecret":"<<REDACTED>>"`},
		},
		{
			name:     "client secret snake_case",
			in:       `{"client_secret":"snake-secret"}`,
			mustHide: []string{"snake-secret"},
			mustHave: []string{`"client_secret":"<<REDACTED>>"`},
		},
		{
			name:     "embed url",
			in:       `{"embedUrl":"https://example.sigma.com/embed?signature=ABCDEF"}`,
			mustHide: []string{"signature=ABCDEF", "https://example"},
			mustHave: []string{`"embedUrl":"<<REDACTED>>"`},
		},
		{
			name:     "user attribute value envelope (string)",
			in:       `{"value":{"type":"string","val":"us-east"}}`,
			mustHide: []string{"us-east"},
			mustHave: []string{`"value":"<<REDACTED>>"`},
		},
		{
			name:     "user attribute value envelope inside list entry",
			in:       `{"entries":[{"teamId":"abc","value":{"type":"string","val":"tenant-42"},"priority":1}]}`,
			mustHide: []string{"tenant-42"},
			mustHave: []string{`"value":"<<REDACTED>>"`, `"teamId":"abc"`, `"priority":1`},
		},
		{
			name:     "user attribute defaultValue envelope (UA resource)",
			in:       `{"userAttributeId":"abc","name":"region","defaultValue":{"type":"string","val":"global"}}`,
			mustHide: []string{`"global"`},
			mustHave: []string{`"defaultValue":"<<REDACTED>>"`, `"userAttributeId":"abc"`, `"name":"region"`},
		},
		{
			name:     "non-secret fields untouched",
			in:       `{"name":"my-thing","color":"violet","description":"hello"}`,
			mustHide: []string{"<<REDACTED>>"},
			mustHave: []string{`"name":"my-thing"`, `"color":"violet"`, `"description":"hello"`},
		},
		{
			name:     "service account JSON nested in a string",
			in:       `{"type":"bigquery","serviceAccount":"{\"type\":\"service_account\",\"private_key\":\"-----BEGIN PRIVATE KEY-----\\nXYZ\"}"}`,
			mustHide: []string{"BEGIN PRIVATE KEY", "XYZ", "service_account"},
			mustHave: []string{`"serviceAccount":"<<REDACTED>>"`, `"type":"bigquery"`},
		},
		{
			name:     "password as object",
			in:       `{"password":{"type":"plain","value":"hunter2"}}`,
			mustHide: []string{"hunter2"},
			mustHave: []string{`"password":"<<REDACTED>>"`},
		},
		{
			name:     "bearer and aws credentials",
			in:       `{"bearer":{"token":"tok-123"},"awsSigV4":{"accessKeyId":"AKIA","secretAccessKey":"sak-456","sessionToken":"st-789"}}`,
			mustHide: []string{"tok-123", "sak-456", "st-789"},
			mustHave: []string{`"accessKeyId":"AKIA"`},
		},
		{
			name:     "api key object",
			in:       `{"apiKey":{"key":"X-Api-Key","value":"k-999","isQueryParam":false}}`,
			mustHide: []string{"k-999"},
			mustHave: []string{`"apiKey":"<<REDACTED>>"`},
		},
		{
			name:     "secret inside JSON string field",
			in:       `{"details_json":"{\"password\":\"pw-1\"}"}`,
			mustHide: []string{"pw-1"},
		},
		{
			name:     "non-JSON body falls back to regex with escaped quotes",
			in:       `not json {"password":"a\"b-secret"}`,
			mustHide: []string{"b-secret"},
			mustHave: []string{`"password":"<<REDACTED>>"`},
		},
		{
			name:     "next page token kept",
			in:       `{"entries":[],"nextPageToken":"abc"}`,
			mustHave: []string{`"nextPageToken":"abc"`},
		},
		{
			name:     "empty body",
			in:       ``,
			mustHide: nil,
			mustHave: []string{""},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := maskBody([]byte(tc.in))
			for _, hidden := range tc.mustHide {
				if hidden != "" && strings.Contains(got, hidden) {
					t.Errorf("expected %q to be redacted, got %q", hidden, got)
				}
			}
			for _, want := range tc.mustHave {
				if !strings.Contains(got, want) {
					t.Errorf("expected output to contain %q, got %q", want, got)
				}
			}
		})
	}
}

func TestMaskBody_ConnectorParamValue(t *testing.T) {
	in := `{"params":{"headers":[{"key":"X-Api-Key","mode":"static","value":"sk-live-123"}],"queryParams":[{"key":"token","mode":"static","value":"q-secret"}]}}`
	out := maskBody([]byte(in))
	for _, s := range []string{"sk-live-123", "q-secret"} {
		if strings.Contains(out, s) {
			t.Errorf("leaked %q: %s", s, out)
		}
	}
	if !strings.Contains(out, "X-Api-Key") {
		t.Errorf("header name should stay visible: %s", out)
	}
}
