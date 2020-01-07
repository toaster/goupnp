package soap

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

type capturingRoundTripper struct {
	err         error
	resp        *http.Response
	capturedReq *http.Request
}

func (rt *capturingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.capturedReq = req
	return rt.resp, rt.err
}

func TestActionInputs(t *testing.T) {
	t.Parallel()
	url, err := url.Parse("http://example.com/soap")
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]struct {
		input      interface{}
		output     interface{}
		wantBody   string
		wantOutput interface{}
	}{
		"default": {
			input: &struct {
				Foo string
				Bar string `soap:"bar"`
				Baz string
			}{"foo", "bar", "quoted=\"baz\""},
			output: &struct {
				A string
				B string
			}{},
			wantBody: (soapPrefix +
				`<u:myaction xmlns:u="mynamespace">` +
				`<Foo>foo</Foo>` +
				`<bar>bar</bar>` +
				`<Baz>quoted="baz"</Baz>` +
				`</u:myaction>` +
				soapSuffix),
			wantOutput: &struct {
				A string
				B string
			}{"valueA", "valueB"},
		},
		"input is a map": {
			input: map[string]string{
				"Foo": "foo",
				"bar": "bar",
				"Baz": `quoted="baz"`,
			},
			output: &struct {
				A string
				B string
			}{},
			wantBody: (soapPrefix +
				`<u:myaction xmlns:u="mynamespace">` +
				`<Foo>foo</Foo>` +
				`<bar>bar</bar>` +
				`<Baz>quoted="baz"</Baz>` +
				`</u:myaction>` +
				soapSuffix),
			wantOutput: &struct {
				A string
				B string
			}{"valueA", "valueB"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rt := &capturingRoundTripper{
				err: nil,
				resp: &http.Response{
					StatusCode: 200,
					Body: ioutil.NopCloser(bytes.NewBufferString(`
				<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
					<s:Body>
						<u:myactionResponse xmlns:u="mynamespace">
							<A>valueA</A>
							<B>valueB</B>
						</u:myactionResponse>
					</s:Body>
				</s:Envelope>
			`)),
				},
			}
			client := SOAPClient{
				EndpointURL: *url,
				HTTPClient: http.Client{
					Transport: rt,
				},
			}

			err = client.PerformAction("mynamespace", "myaction", tt.input, tt.output)
			if err != nil {
				t.Fatal(err)
			}

			body, err := ioutil.ReadAll(rt.capturedReq.Body)
			if err != nil {
				t.Fatal(err)
			}
			gotBody := string(body)
			if tt.wantBody != gotBody {
				t.Errorf("Bad request body\nwant: %q\n got: %q", tt.wantBody, gotBody)
			}

			if !reflect.DeepEqual(tt.wantOutput, tt.output) {
				t.Errorf("Bad output\nwant: %+v\n got: %+v", tt.wantOutput, tt.output)
			}
		})
	}
}

func TestEscapeXMLText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"abc123", "abc123"},
		{"<foo>&", "&lt;foo&gt;&amp;"},
		{"\"foo'", "\"foo'"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.input, func(t *testing.T) {
			got := escapeXMLText(test.input)
			if got != test.want {
				t.Errorf("want %q, got %q", test.want, got)
			}
		})
	}
}
