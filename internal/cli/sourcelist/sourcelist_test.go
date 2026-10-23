package sourcelist

import (
	"reflect"
	"testing"

	"yv35.com/ppa-utils/internal/ppa"
)

func testSources() []ppa.SourceFile {
	return []ppa.SourceFile{
		{
			Path:          "/etc/apt/sources.list.d/docker.sources",
			URIs:          []string{"https://download.docker.com/linux/ubuntu/"},
			Suites:        []string{"resolute"},
			Components:    []string{"stable"},
			Architectures: []ppa.Architecture{ppa.ArchitectureAMD64},
			SignedBy:      "/etc/apt/keyrings/docker.asc",
			Enabled:       ppa.EnabledStatusYes,
		},
		{
			Path:       "/etc/apt/sources.list.d/kisak-mesa.sources",
			URIs:       []string{"https://ppa.launchpadcontent.net/kisak/kisak-mesa/ubuntu/"},
			Suites:     []string{"noble", "noble-updates"},
			Components: []string{"main", "universe"},
			SignedBy:   "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nabc\n-----END PGP PUBLIC KEY BLOCK-----",
			Enabled:    ppa.EnabledStatusNo,
		},
		{
			Path:   "/etc/apt/sources.list.d/ubuntu.sources",
			URIs:   []string{"http://archive.ubuntu.com/ubuntu"},
			Suites: []string{"noble"},
		},
	}
}

func TestToRows(t *testing.T) {
	got := ToRows(testSources())
	want := []Row{
		{
			Name:          "docker",
			Path:          "/etc/apt/sources.list.d/docker.sources",
			URIs:          []string{"https://download.docker.com/linux/ubuntu/"},
			Suites:        []string{"resolute"},
			Components:    []string{"stable"},
			Architectures: []string{"amd64"},
			SignedBy:      "/etc/apt/keyrings/docker.asc",
			Status:        StatusEnabled,
		},
		{
			Name:          "kisak-mesa",
			Path:          "/etc/apt/sources.list.d/kisak-mesa.sources",
			URIs:          []string{"https://ppa.launchpadcontent.net/kisak/kisak-mesa/ubuntu/"},
			Suites:        []string{"noble", "noble-updates"},
			Components:    []string{"main", "universe"},
			Architectures: []string{},
			SignedBy:      InlineKey,
			Status:        StatusDisabled,
		},
		{
			Name:          "ubuntu",
			Path:          "/etc/apt/sources.list.d/ubuntu.sources",
			URIs:          []string{"http://archive.ubuntu.com/ubuntu"},
			Suites:        []string{"noble"},
			Components:    []string{},
			Architectures: []string{},
			Status:        StatusEnabled,
			Official:      true,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToRows() =\n%+v\nwant\n%+v", got, want)
	}
}

func TestSourceName(t *testing.T) {
	tests := map[string]string{
		"/etc/apt/sources.list.d/docker.sources":           "docker",
		"/etc/apt/sources.list.d/home:codestation.sources": "home:codestation",
		"plain": "plain",
	}
	for path, want := range tests {
		if got := SourceName(path); got != want {
			t.Errorf("SourceName(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestFilter(t *testing.T) {
	rows := ToRows(testSources())

	tests := []struct {
		name   string
		filter Filter
		want   []string
	}{
		{"no filter", Filter{}, []string{"docker", "kisak-mesa", "ubuntu"}},
		{"name", Filter{Name: "KISAK"}, []string{"kisak-mesa"}},
		{"uri", Filter{URI: "launchpadcontent"}, []string{"kisak-mesa"}},
		{"suite", Filter{Suite: "resolute"}, []string{"docker"}},
		{"suite matches any value", Filter{Suite: "noble-updates"}, []string{"kisak-mesa"}},
		{"component", Filter{Component: "universe"}, []string{"kisak-mesa"}},
		{"arch", Filter{Arch: "amd64"}, []string{"docker"}},
		{"signed by path", Filter{SignedBy: "keyrings"}, []string{"docker"}},
		{"signed by inline key", Filter{SignedBy: "inline-key"}, []string{"kisak-mesa"}},
		{"status disabled", Filter{Status: "disabled"}, []string{"kisak-mesa"}},
		{"status enabled", Filter{Status: "enabled"}, []string{"docker", "ubuntu"}},
		{"filters are combined", Filter{Suite: "noble", Component: "stable"}, []string{}},
		{"no match", Filter{Name: "nope"}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := []string{}
			for _, row := range tt.filter.Apply(rows) {
				got = append(got, row.Name)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
