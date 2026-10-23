package ppa

type Architecture string

const (
	ArchitectureAMD64 Architecture = "amd64"
	ArchitectureARM64 Architecture = "arm64"
	ArchitectureI386  Architecture = "i386"
)

func ParseArchitecture(value string) Architecture {
	switch value {
	case "amd64":
		return ArchitectureAMD64
	case "arm64":
		return ArchitectureARM64
	case "i386":
		return ArchitectureI386
	default:
		return ArchitectureAMD64
	}
}

type EnabledStatus string

const (
	EnabledStatusYes EnabledStatus = "Yes"
	EnabledStatusNo  EnabledStatus = "No"
)

func ParseEnabledStatus(value string) EnabledStatus {
	switch value {
	case "Yes", "yes":
		return EnabledStatusYes
	case "No", "no":
		return EnabledStatusNo
	default:
		return EnabledStatusYes
	}
}

// IsEnabled reports whether the source is enabled. A source without an
// Enabled field is enabled.
func (e EnabledStatus) IsEnabled() bool {
	return e != EnabledStatusNo
}

type SourceFile struct {
	// Location of the source file within the repository or filesystem.
	Path string
	// Stanza is the index of the entry within the source file, which can hold several.
	Stanza int
	// Types of the source file (e.g., "deb", "binary", "source").
	Types []string
	// URIs where the source file can be accessed.
	URIs []string
	// Suites to which the source file belongs.
	Suites []string
	// Components to which the source file belongs.
	Components []string
	// Architectures supported by the source file.
	Architectures []Architecture
	// Key used to sign the source file.
	SignedBy string
	// Indicates whether the source file is enabled. Yes or No.
	Enabled EnabledStatus
}

func (sf *SourceFile) IsEnabled() bool {
	return sf.Enabled.IsEnabled()
}
