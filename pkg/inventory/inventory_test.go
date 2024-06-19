package inventory

import (
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/go-playground/validator/v10/non-standard/validators"
)

func TestInventory_ParseAttributes(t *testing.T) {
	cfg := &Config{}
	cfg.Txt.Kv.Separator = ";"
	cfg.Txt.Kv.Equalsign = "="
	cfg.Txt.Keys.Os = "OS"
	cfg.Txt.Keys.Env = "ENV"
	cfg.Txt.Keys.Role = "ROLE"
	cfg.Txt.Keys.Srv = "SRV"
	cfg.Txt.Keys.Vars = "VARS"

	validator := validator.New()
	validator.RegisterValidation("notblank", validators.NotBlank)
	validator.RegisterValidation("safelist", isSafeList)
	validator.RegisterValidation("safelistsep", isSafeListWithSeparator)

	testInventory := &Inventory{
		Validator: validator,
		Config:    cfg,
	}

	type args struct {
		raw string
	}
	tests := []struct {
		name    string
		i       *Inventory
		args    args
		want    *HostAttributes
		wantErr bool
	}{
		{
			name: "valid",
			i:    testInventory,
			args: args{
				raw: "OS=linux;ENV=dev;ROLE=app;SRV=wildfly_public;VARS=test=123456,test2=654321",
			},
			want: &HostAttributes{
				OS:   "linux",
				Env:  "dev",
				Role: "app",
				Srv:  "wildfly_public",
				Vars: "test=123456,test2=654321",
			},
			wantErr: false,
		},
		{
			name: "valid-role-list",
			i:    testInventory,
			args: args{
				raw: "OS=linux;ENV=dev;ROLE=app,storage;SRV=wildfly_public;VARS=test=123456,test2=654321",
			},
			want: &HostAttributes{
				OS:   "linux",
				Env:  "dev",
				Role: "app,storage",
				Srv:  "wildfly_public",
				Vars: "test=123456,test2=654321",
			},
			wantErr: false,
		},
		{
			name: "valid-srv-list",
			i:    testInventory,
			args: args{
				raw: "OS=linux;ENV=dev;ROLE=app;SRV=wildfly_public,wildfly_private;VARS=test=123456,test2=654321",
			},
			want: &HostAttributes{
				OS:   "linux",
				Env:  "dev",
				Role: "app",
				Srv:  "wildfly_public,wildfly_private",
				Vars: "test=123456,test2=654321",
			},
			wantErr: false,
		},
		{
			name: "valid-alt-sep",
			i:    testInventory,
			args: args{
				raw: "OS=linux;ENV=dev;ROLE=app;SRV=wildfly-public;VARS=test=123456,test2=654321",
			},
			want: &HostAttributes{
				OS:   "linux",
				Env:  "dev",
				Role: "app",
				Srv:  "wildfly-public",
				Vars: "test=123456,test2=654321",
			},
			wantErr: false,
		},
		{
			name: "valid-empty-srv",
			i:    testInventory,
			args: args{
				raw: "OS=linux;ENV=dev;ROLE=app;SRV=;VARS=test=123456,test2=654321",
			},
			want: &HostAttributes{
				OS:   "linux",
				Env:  "dev",
				Role: "app",
				Srv:  "",
				Vars: "test=123456,test2=654321",
			},
			wantErr: false,
		},
		{
			name: "valid-empty-vars",
			i:    testInventory,
			args: args{
				raw: "OS=linux;ENV=dev;ROLE=app;SRV=wildfly_public;VARS=",
			},
			want: &HostAttributes{
				OS:   "linux",
				Env:  "dev",
				Role: "app",
				Srv:  "wildfly_public",
				Vars: "",
			},
			wantErr: false,
		},
		{
			name: "valid-no-srv",
			i:    testInventory,
			args: args{
				raw: "OS=linux;ENV=dev;ROLE=app;VARS=test=123456,test2=654321",
			},
			want: &HostAttributes{
				OS:   "linux",
				Env:  "dev",
				Role: "app",
				Srv:  "",
				Vars: "test=123456,test2=654321",
			},
			wantErr: false,
		},
		{
			name: "valid-no-vars",
			i:    testInventory,
			args: args{
				raw: "OS=linux;ENV=dev;ROLE=app;SRV=wildfly_public",
			},
			want: &HostAttributes{
				OS:   "linux",
				Env:  "dev",
				Role: "app",
				Srv:  "wildfly_public",
				Vars: "",
			},
			wantErr: false,
		},
		{
			name: "invalid-empty-env",
			i:    testInventory,
			args: args{
				raw: "OS=linux;ENV=;ROLE=app;SRV=wildfly_public;VARS=test=123456,test2=654321",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid-blank-env",
			i:    testInventory,
			args: args{
				raw: "OS=linux;ENV= ;ROLE=app;SRV=wildfly_public;VARS=test=123456,test2=654321",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid-no-env",
			i:    testInventory,
			args: args{
				raw: "OS=linux;ROLE=app;SRV=wildfly_public;VARS=test=123456,test2=654321",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid-env",
			i:    testInventory,
			args: args{
				raw: "OS=linux;ENV=!@#$%^;ROLE=app;SRV=wildfly_public;VARS=test=123456,test2=654321",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.i.ParseAttributes(tt.args.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("Inventory.ParseAttributes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Inventory.ParseAttributes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInventory_RenderAttributes(t *testing.T) {
	cfg := &Config{}
	cfg.Txt.Kv.Separator = ";"
	cfg.Txt.Kv.Equalsign = "="
	cfg.Txt.Keys.Os = "OS"
	cfg.Txt.Keys.Env = "ENV"
	cfg.Txt.Keys.Role = "ROLE"
	cfg.Txt.Keys.Srv = "SRV"
	cfg.Txt.Keys.Vars = "VARS"

	validator := validator.New()
	validator.RegisterValidation("notblank", validators.NotBlank)
	validator.RegisterValidation("safelist", isSafeList)
	validator.RegisterValidation("safelistsep", isSafeListWithSeparator)

	testInventory := &Inventory{
		Validator: validator,
		Config:    cfg,
	}

	type args struct {
		attributes *HostAttributes
	}
	tests := []struct {
		name    string
		i       *Inventory
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "valid",
			i:    testInventory,
			args: args{
				attributes: &HostAttributes{
					OS:   "testos",
					Env:  "testenv",
					Role: "testrole",
					Srv:  "testsrv",
					Vars: "testvar=testvalue",
				},
			},
			want:    "OS=testos;ENV=testenv;ROLE=testrole;SRV=testsrv;VARS=testvar=testvalue",
			wantErr: false,
		},
		{
			name: "valid-no-vars",
			i:    testInventory,
			args: args{
				attributes: &HostAttributes{
					OS:   "testos",
					Env:  "testenv",
					Role: "testrole",
					Srv:  "testsrv",
				},
			},
			want:    "OS=testos;ENV=testenv;ROLE=testrole;SRV=testsrv;VARS=",
			wantErr: false,
		},
		{
			name: "valid-no-vars-no-srv",
			i:    testInventory,
			args: args{
				attributes: &HostAttributes{
					OS:   "testos",
					Env:  "testenv",
					Role: "testrole",
				},
			},
			want:    "OS=testos;ENV=testenv;ROLE=testrole;SRV=;VARS=",
			wantErr: false,
		},
		{
			name: "invalid-attribute",
			i:    testInventory,
			args: args{
				attributes: &HostAttributes{
					OS:   "testos",
					Env:  "testenv",
					Role: "testrole",
					Srv:  "%",
				},
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.i.RenderAttributes(tt.args.attributes)
			if (err != nil) != tt.wantErr {
				t.Errorf("Inventory.RenderAttributes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Inventory.RenderAttributes() = %v, want %v", got, tt.want)
			}
		})
	}
}
