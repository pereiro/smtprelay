package main

import (
	"net"
	"reflect"
	"testing"
)

func TestMailServersFromMX(t *testing.T) {
	tests := []struct {
		name string
		mx   []*net.MX
		want []string
	}{
		{
			name: "preserve resolver preference order",
			mx: []*net.MX{
				{Host: "mx1.example.com.", Pref: 10},
				{Host: "mx2.example.com.", Pref: 20},
				{Host: "mx3.example.com.", Pref: 30},
			},
			want: []string{"mx1.example.com:25", "mx2.example.com:25", "mx3.example.com:25"},
		},
		{
			name: "equal preferences retain resolver order",
			mx: []*net.MX{
				{Host: "mx2.example.com.", Pref: 10},
				{Host: "mx1.example.com.", Pref: 10},
			},
			want: []string{"mx2.example.com:25", "mx1.example.com:25"},
		},
		{
			name: "normalize names and skip duplicates and unusable records",
			mx: []*net.MX{
				nil, {Host: "."}, {Host: ""}, {Host: "LOCALHOST."},
				{Host: "127.0.0.1"}, {Host: "::1"},
				{Host: "MX.EXAMPLE.COM.", Pref: 10},
				{Host: "mx.example.com", Pref: 20},
				{Host: "backup.example.com", Pref: 30},
			},
			want: []string{"mx.example.com:25", "backup.example.com:25"},
		},
		{name: "no records"},
		{name: "null MX", mx: []*net.MX{{Host: "."}}},
		{name: "only unusable records", mx: []*net.MX{nil, {Host: ""}, {Host: "localhost."}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mailServersFromMX("example.com", tt.mx)
			if tt.want == nil {
				if err == nil {
					t.Fatal("expected an error for unusable MX records")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("servers = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLookupMailServersRejectsLocalDomain(t *testing.T) {
	for _, domain := range []string{"", "localhost", "LOCALHOST.", "127.0.0.1", "127.0.0.2", "::1"} {
		if _, err := lookupMailServers(domain); err == nil {
			t.Errorf("expected an error for domain %q", domain)
		}
	}
}

func TestLookupDeliveryServersInRelayMode(t *testing.T) {
	previous := conf
	t.Cleanup(func() { conf = previous })
	conf = &Conf{RelayModeEnabled: true, RelayServer: "relay.example.com:2525"}
	servers, err := lookupDeliveryServers("localhost")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(servers, []string{conf.RelayServer}) {
		t.Fatalf("servers = %v, want the configured relay", servers)
	}
}
