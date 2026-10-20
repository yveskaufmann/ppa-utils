package ppa

import (
	"path/filepath"
	"reflect"
	"testing"
)

// testdataPath returns the path of a fixture file in the testdata directory.
// Go runs tests with the package directory as working directory, so the
// relative path resolves to internal/ppa/testdata/<name>.
func testdataPath(name string) string {
	return filepath.Join("testdata", name)
}

func TestReadSourceFile(t *testing.T) {
	tests := []struct {
		name string
		// file is the fixture file name within testdata/.
		file    string
		want    *SourceFile
		wantErr bool
	}{
		{
			name: "valid source file",
			file: "valid.sources",
			want: &SourceFile{
				Types:         []string{"deb"},
				URIs:          []string{"https://ppa.launchpadcontent.net/example/ppa/ubuntu/"},
				Suites:        []string{"noble"},
				Components:    []string{"main"},
				Architectures: []Architecture{ArchitectureAMD64},
				SignedBy:      "/etc/apt/keyrings/example.gpg",
				Enabled:       EnabledStatusYes,
			},
		},
		{
			name: "multiple values per field",
			file: "multiple-values.sources",
			want: &SourceFile{
				Types:         []string{"deb", "deb-src"},
				URIs:          []string{"https://ppa.launchpadcontent.net/example/ppa/ubuntu/"},
				Suites:        []string{"noble", "noble-updates"},
				Components:    []string{"main", "universe"},
				Architectures: []Architecture{ArchitectureAMD64, ArchitectureARM64},
				SignedBy:      "/etc/apt/keyrings/example.gpg",
			},
		},
		{
			name: "disabled source file",
			file: "disabled.sources",
			want: &SourceFile{
				Types:      []string{"deb"},
				URIs:       []string{"https://ppa.launchpadcontent.net/example/ppa/ubuntu/"},
				Suites:     []string{"noble"},
				Components: []string{"main"},
				Enabled:    EnabledStatusNo,
			},
		},
		{
			name: "signed-by with inline pgp public key",
			file: "signed-by-inline-key.sources",
			want: &SourceFile{
				Types:      []string{"deb"},
				URIs:       []string{"https://ppa.launchpadcontent.net/example/ppa/ubuntu/"},
				Suites:     []string{"noble"},
				Components: []string{"main"},
				SignedBy: `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBGRkPdMBEADGnmHdNTOdmAiDvLl3DmQTXNSWBHqPWn7jPVAcwW5ZJJeVhVGQ
xsmSJvz4V0lvb3D6WbNcIJ9qJrCw7gZwJFELCLbDXHRV8MJAxPvP7nqJzVyAHRB2
=abcd
-----END PGP PUBLIC KEY BLOCK-----`,
				Enabled: EnabledStatusYes,
			},
		},
		{
			name:    "file does not exist",
			file:    "does-not-exist.sources",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := testdataPath(tt.file)
			if tt.want != nil {
				tt.want.Path = path
			}

			got, err := ReadSourceFile(path)

			if (err != nil) != tt.wantErr {
				t.Fatalf("readSourceFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readSourceFile() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
