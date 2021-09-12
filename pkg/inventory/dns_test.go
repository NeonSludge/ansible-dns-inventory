package inventory

import "testing"

func TestDNSDatasource_makeFQDN(t *testing.T) {
	type args struct {
		host string
		zone string
	}
	tests := []struct {
		name string
		d    *DNSDatasource
		args args
		want string
	}{
		{
			name: "empty",
			args: args{
				host: "",
				zone: "",
			},
			want: ".",
		},
		{
			name: "host-zone-1",
			args: args{
				host: "test",
				zone: "rnd.local",
			},
			want: "test.rnd.local.",
		},
		{
			name: "host-zone-2",
			args: args{
				host: "test",
				zone: ".rnd.local",
			},
			want: "test.rnd.local.",
		},
		{
			name: "host-zone-3",
			args: args{
				host: "test",
				zone: "rnd.local.",
			},
			want: "test.rnd.local.",
		},
		{
			name: "host-zone-4",
			args: args{
				host: "test",
				zone: ".rnd.local.",
			},
			want: "test.rnd.local.",
		},
		{
			name: "host-1",
			args: args{
				host: "test.rnd.local",
				zone: "",
			},
			want: "test.rnd.local.",
		},
		{
			name: "host-2",
			args: args{
				host: ".test.rnd.local",
				zone: "",
			},
			want: "test.rnd.local.",
		},
		{
			name: "host-3",
			args: args{
				host: "test.rnd.local.",
				zone: "",
			},
			want: "test.rnd.local.",
		},
		{
			name: "host-4",
			args: args{
				host: ".test.rnd.local.",
				zone: "",
			},
			want: "test.rnd.local.",
		},
		{
			name: "zone-1",
			args: args{
				host: "",
				zone: "rnd.local",
			},
			want: "rnd.local.",
		},
		{
			name: "zone-2",
			args: args{
				host: "",
				zone: ".rnd.local",
			},
			want: "rnd.local.",
		},
		{
			name: "zone-3",
			args: args{
				host: "",
				zone: "rnd.local.",
			},
			want: "rnd.local.",
		},
		{
			name: "zone-4",
			args: args{
				host: "",
				zone: ".rnd.local.",
			},
			want: "rnd.local.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.d.makeFQDN(tt.args.host, tt.args.zone); got != tt.want {
				t.Errorf("DNSDatasource.makeFQDN() = %v, want %v", got, tt.want)
			}
		})
	}
}
