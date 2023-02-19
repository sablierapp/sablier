package caddy_test

import (
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/acouvreur/sablier/plugins/caddy"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
)

var fals bool = false
var tru bool = true
var oneMinute = 1 * time.Minute

func TestConfig_BuildRequest(t *testing.T) {
	tests := []struct {
		name    string
		fields  caddy.Config
		want    *http.Request
		wantErr bool
	}{
		{
			name: "dynamic session with required values",
			fields: caddy.Config{

				SablierURL: "http://sablier:10000",
				Names:      []string{"nginx", "apache"},
				Dynamic:    &caddy.DynamicConfiguration{},
			},
			want:    createRequest("GET", "http://sablier:10000/api/strategies/dynamic?names=nginx&names=apache", nil),
			wantErr: false,
		},
		{
			name: "dynamic session with default values",
			fields: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Names:           []string{"nginx", "apache"},
				SessionDuration: &oneMinute,
				Dynamic:         &caddy.DynamicConfiguration{},
			},
			want:    createRequest("GET", "http://sablier:10000/api/strategies/dynamic?names=nginx&names=apache&session_duration=1m", nil),
			wantErr: false,
		},
		{
			name: "dynamic session with group",
			fields: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Group:           "default",
				SessionDuration: &oneMinute,
				Dynamic:         &caddy.DynamicConfiguration{},
			},
			want:    createRequest("GET", "http://sablier:10000/api/strategies/dynamic?group=default&session_duration=1m", nil),
			wantErr: false,
		},
		{
			name: "dynamic session with theme values",
			fields: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Names:           []string{"nginx", "apache"},
				SessionDuration: &oneMinute,
				Dynamic: &caddy.DynamicConfiguration{
					Theme: "hacker-terminal",
				},
			},
			want:    createRequest("GET", "http://sablier:10000/api/strategies/dynamic?names=nginx&names=apache&session_duration=1m&theme=hacker-terminal", nil),
			wantErr: false,
		},
		{
			name: "dynamic session with theme and display name values",
			fields: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Names:           []string{"nginx", "apache"},
				SessionDuration: &oneMinute,
				Dynamic: &caddy.DynamicConfiguration{
					Theme:       "hacker-terminal",
					DisplayName: "Hello World!",
				},
			},
			want:    createRequest("GET", "http://sablier:10000/api/strategies/dynamic?display_name=Hello+World%21&names=nginx&names=apache&session_duration=1m&theme=hacker-terminal", nil),
			wantErr: false,
		},
		{
			name: "dynamic session with refresh frequency",
			fields: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Names:           []string{"nginx", "apache"},
				SessionDuration: &oneMinute,
				Dynamic: &caddy.DynamicConfiguration{
					RefreshFrequency: &oneMinute,
				},
			},
			want:    createRequest("GET", "http://sablier:10000/api/strategies/dynamic?names=nginx&names=apache&refresh_frequency=1m&session_duration=1m", nil),
			wantErr: false,
		},
		{
			name: "dynamic session with show details to true",
			fields: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Names:           []string{"nginx", "apache"},
				SessionDuration: &oneMinute,
				Dynamic: &caddy.DynamicConfiguration{
					ShowDetails:      &tru,
					RefreshFrequency: &oneMinute,
				},
			},
			want:    createRequest("GET", "http://sablier:10000/api/strategies/dynamic?names=nginx&names=apache&refresh_frequency=1m&session_duration=1m&show_details=true", nil),
			wantErr: false,
		},
		{
			name: "dynamic session with show details to false",
			fields: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Names:           []string{"nginx", "apache"},
				SessionDuration: &oneMinute,
				Dynamic: &caddy.DynamicConfiguration{
					ShowDetails:      &fals,
					RefreshFrequency: &oneMinute,
				},
			},
			want:    createRequest("GET", "http://sablier:10000/api/strategies/dynamic?names=nginx&names=apache&refresh_frequency=1m&session_duration=1m&show_details=false", nil),
			wantErr: false,
		},
		{
			name: "dynamic session without show details set",
			fields: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Names:           []string{"nginx", "apache"},
				SessionDuration: &oneMinute,
				Dynamic: &caddy.DynamicConfiguration{
					ShowDetails:      nil,
					RefreshFrequency: &oneMinute,
				},
			},
			want:    createRequest("GET", "http://sablier:10000/api/strategies/dynamic?names=nginx&names=apache&refresh_frequency=1m&session_duration=1m", nil),
			wantErr: false,
		},
		{
			name: "blocking session with required values",
			fields: caddy.Config{
				SablierURL: "http://sablier:10000",
				Names:      []string{"nginx", "apache"},
				Blocking:   &caddy.BlockingConfiguration{},
			},
			want:    createRequest("GET", "http://sablier:10000/api/strategies/blocking?names=nginx&names=apache", nil),
			wantErr: false,
		},
		{
			name: "blocking session with default values",
			fields: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Names:           []string{"nginx", "apache"},
				SessionDuration: &oneMinute,
				Blocking:        &caddy.BlockingConfiguration{},
			},
			want:    createRequest("GET", "http://sablier:10000/api/strategies/blocking?names=nginx&names=apache&session_duration=1m", nil),
			wantErr: false,
		},
		{
			name: "blocking session with group",
			fields: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Group:           "default",
				SessionDuration: &oneMinute,
				Blocking:        &caddy.BlockingConfiguration{},
			},
			want:    createRequest("GET", "http://sablier:10000/api/strategies/blocking?group=default&session_duration=1m", nil),
			wantErr: false,
		},
		{
			name: "blocking session with timeout value",
			fields: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Names:           []string{"nginx", "apache"},
				SessionDuration: &oneMinute,
				Blocking: &caddy.BlockingConfiguration{
					Timeout: nil,
				},
			},
			want:    createRequest("GET", "http://sablier:10000/api/strategies/blocking?names=nginx&names=apache&session_duration=1m&timeout=5m", nil),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &caddy.Config{
				SablierURL:      tt.fields.SablierURL,
				Names:           tt.fields.Names,
				Group:           tt.fields.Group,
				SessionDuration: tt.fields.SessionDuration,
				Dynamic:         tt.fields.Dynamic,
				Blocking:        tt.fields.Blocking,
			}

			got, err := c.BuildRequest()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.BuildRequest() error = %v, wantErr %v", err, tt.wantErr)
			} else if got.RequestURI != tt.want.RequestURI {
				t.Errorf("Config.BuildRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_UnmarshalCaddyfile(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		want         caddy.Config
		wantErr      bool
		wantErrValue string
	}{
		{
			name: "default sablier URL",
			input: `sablier {
				group mygroup
				dynamic
			}`,
			want: caddy.Config{
				SablierURL: "http://sablier:10000",
				Group:      "mygroup",
				Dynamic:    &caddy.DynamicConfiguration{},
			},
			wantErr: false,
		},
		{
			name: "specific sablier URL",
			input: `sablier http://mysablier:3000 {
				names container1 container2 container3
				blocking
			}`,
			want: caddy.Config{
				SablierURL: "http://mysablier:3000",
				Names:      []string{"container1", "container2", "container3"},
				Blocking:   &caddy.BlockingConfiguration{},
			},
			wantErr: false,
		},
		{
			name: "parse valid names dynamic",
			input: `sablier {
				names container1 container2 container3
				session_duration 1m
				dynamic {
					display_name This is a display name!
					show_details on
					theme hacker-terminal
					refresh_frequency 1m
				}
			}`,
			want: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Names:           []string{"container1", "container2", "container3"},
				SessionDuration: &oneMinute,
				Dynamic: &caddy.DynamicConfiguration{
					DisplayName:      "This is a display name!",
					ShowDetails:      &tru,
					Theme:            "hacker-terminal",
					RefreshFrequency: &oneMinute,
				},
			},
			wantErr: false,
		},
		{
			name: "parse valid group dynamic",
			input: `sablier {
				group mygroup
				session_duration 1m
				dynamic {
					display_name This is a display name!
					show_details on
					theme hacker-terminal
					refresh_frequency 1m
				}
			}`,
			want: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Group:           "mygroup",
				SessionDuration: &oneMinute,
				Dynamic: &caddy.DynamicConfiguration{
					DisplayName:      "This is a display name!",
					ShowDetails:      &tru,
					Theme:            "hacker-terminal",
					RefreshFrequency: &oneMinute,
				},
			},
			wantErr: false,
		},
		{
			name: "parse valid names blocking",
			input: `sablier {
				group mygroup
				session_duration 1m
				blocking {
					timeout 1m
				}
			}`,
			want: caddy.Config{
				SablierURL:      "http://sablier:10000",
				Group:           "mygroup",
				SessionDuration: &oneMinute,
				Blocking: &caddy.BlockingConfiguration{
					Timeout: &oneMinute,
				},
			},
			wantErr: false,
		},
		{
			name:  "parse invalid no strategies",
			input: `sablier`,
			want: caddy.Config{
				SablierURL: "http://sablier:10000",
			},
			wantErr:      true,
			wantErrValue: "you must specify one strategy (dynamic or blocking)",
		},
		{
			name: "parse invalid two strategies",
			input: `sablier {
				blocking 
				dynamic
			}`,
			want: caddy.Config{
				SablierURL: "http://sablier:10000",
			},
			wantErr:      true,
			wantErrValue: "you must specify only one strategy",
		},
		{
			name: "parse invalid no names or group",
			input: `sablier {
				blocking
			}`,
			want: caddy.Config{
				SablierURL: "http://sablier:10000",
			},
			wantErr:      true,
			wantErrValue: "you must specify names or group",
		},
		{
			name: "parse empty names",
			input: `sablier {
				names
				blocking
			}`,
			want: caddy.Config{
				SablierURL: "http://sablier:10000",
			},
			wantErr:      true,
			wantErrValue: "you must specify names or group",
		},
		{
			name: "parse empty group",
			input: `sablier {
				group
				blocking
			}`,
			want: caddy.Config{
				SablierURL: "http://sablier:10000",
			},
			wantErr:      true,
			wantErrValue: "you must specify names or group",
		},
		{
			name: "parse invalid names and group",
			input: `sablier {
				names container1 container2
				group mygroup
				blocking
			}`,
			want: caddy.Config{
				SablierURL: "http://sablier:10000",
			},
			wantErr:      true,
			wantErrValue: "you must specify either names or group",
		},
		{
			name: "parse invalid session_duration",
			input: `sablier {
				group mygroup
				session_duration invalid
				blocking
			}`,
			want: caddy.Config{
				SablierURL: "http://sablier:10000",
			},
			wantErr:      true,
			wantErrValue: "time: invalid duration \"invalid\"",
		},
		{
			name: "parse invalid refresh_frequency",
			input: `sablier {
				group mygroup
				dynamic {
					refresh_frequency invalid
				}
			}`,
			want: caddy.Config{
				SablierURL: "http://sablier:10000",
			},
			wantErr:      true,
			wantErrValue: "time: invalid duration \"invalid\"",
		},
		{
			name: "parse invalid timeout",
			input: `sablier {
				group mygroup
				blocking {
					timeout invalid
				}
			}`,
			want: caddy.Config{
				SablierURL: "http://sablier:10000",
			},
			wantErr:      true,
			wantErrValue: "time: invalid duration \"invalid\"",
		},
	}

	for _, tt := range tests {
		h := httpcaddyfile.Helper{
			Dispenser: caddyfile.NewTestDispenser(tt.input),
		}
		got := caddy.Config{}
		err := got.UnmarshalCaddyfile(h.Dispenser)

		if tt.wantErr {
			if (err != nil) != tt.wantErr {
				t.Errorf("%s: UnmarshalCaddyfile() error = %v, wantErr = %v", tt.name, err, tt.wantErr)
			}
			if err.Error() != tt.wantErrValue {
				t.Errorf("%s: UnmarshalCaddyfile() error = %v, wantErrValue = %v", tt.name, err.Error(), tt.wantErrValue)
			}
		} else if !reflect.DeepEqual(tt.want, got) {
			t.Errorf("%s: UnmarshalCaddyfile() = %v, want %v", tt.name, got, tt.want)
		}

	}
}

func createRequest(method string, url string, body io.Reader) *http.Request {
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}
	return request
}
