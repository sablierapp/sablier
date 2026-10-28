package sablier

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestParseActivityPorts(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []Port
		wantErr bool
	}{
		{name: "bare number is tcp", in: "32400", want: []Port{{Number: 32400, Proto: "tcp"}}},
		{
			name: "list with mixed case protocol",
			in:   "32400/TCP, 1900/udp",
			want: []Port{{Number: 32400, Proto: "tcp"}, {Number: 1900, Proto: "udp"}},
		},
		{name: "zero", in: "0", wantErr: true},
		{name: "above 65535", in: "70000", wantErr: true},
		{name: "unknown protocol", in: "80/sctp", wantErr: true},
		{name: "not a number", in: "abc", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseActivityPorts(tt.in)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err)
			assert.DeepEqual(t, tt.want, got)
		})
	}
}
