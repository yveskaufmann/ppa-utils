package distinfo

import (
	"bufio"
	"os"
	"strings"
)

type DistributionInfo struct {
	Version  string
	Codename string
	Name     string
}

// GetCurrentDistributionInfo retrieves the current distribution information from /etc/os-release.
func GetCurrentDistributionInfo() (DistributionInfo, error) {

	dist := DistributionInfo{}

	file, err := os.Open("/etc/os-release")
	if err != nil {
		return dist, err
	}
	defer file.Close()

	reader := bufio.NewScanner(file)

	for reader.Scan() {
		line := strings.TrimSpace(reader.Text())

		if len(line) == 0 || line == "=" || line[0] == '#' {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := strings.Trim(parts[1], `"`)

		switch key {
		case "VERSION_ID":
			dist.Version = value
		case "VERSION_CODENAME":
			dist.Codename = value
		case "ID":
			dist.Name = value
		}
	}

	if err := reader.Err(); err != nil {
		return dist, err
	}

	return dist, nil
}
