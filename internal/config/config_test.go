package config

import (
	"testing"
)

func TestConfig_Active(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   string // host of the active profile
		wantOk bool
	}{
		{
			name:   "empty",
			config: &Config{Profiles: []Profile{}},
			wantOk: false,
		},
		{
			name: "default_profile",
			config: &Config{
				Default: "nas2",
				Profiles: []Profile{
					{Name: "nas1", Host: "192.168.1.1"},
					{Name: "nas2", Host: "192.168.1.2"},
				},
			},
			want:   "192.168.1.2",
			wantOk: true,
		},
		{
			name: "first_if_no_default",
			config: &Config{
				Profiles: []Profile{
					{Name: "nas1", Host: "192.168.1.1"},
				},
			},
			want:   "192.168.1.1",
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.config.Active()
			if ok != tt.wantOk {
				t.Errorf("Active() ok = %v, want %v", ok, tt.wantOk)
			}
			if ok && got.Host != tt.want {
				t.Errorf("Active() host = %v, want %v", got.Host, tt.want)
			}
		})
	}
}

func TestConfig_UpsertAndRemove(t *testing.T) {
	c := &Config{}

	c.Upsert(Profile{Name: "test", Host: "127.0.0.1"})
	if len(c.Profiles) != 1 {
		t.Errorf("Upsert() failed to add")
	}

	c.Upsert(Profile{Name: "test", Host: "192.168.0.1"})
	if len(c.Profiles) != 1 || c.Profiles[0].Host != "192.168.0.1" {
		t.Errorf("Upsert() failed to update")
	}

	if !c.Remove("test") {
		t.Errorf("Remove() failed to remove")
	}
	if len(c.Profiles) != 0 {
		t.Errorf("Remove() didn't reduce size")
	}
}
