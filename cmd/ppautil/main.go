package main

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	"yv35.com/ppa/ppa-utils/internal/distinfo"
	"yv35.com/ppa/ppa-utils/internal/ppa"
)


func main() {

	distInfoReader, err := distinfo.NewUbuntuRegistry()
	if err != nil {
		println("Error creating ubuntu dist registry:", err.Error())
		return
	}

	newCode := ""
	nextRelease, err := distInfoReader.FindCurrentRelease()
	if err != nil {
		if errors.Is(err, distinfo.ErrNoNextRelease) {
			println("No next release found:", err.Error())
			newCode = nextRelease.Series
		} else {
			println("Error getting current distribution info:", err.Error())
			return
		}
	} else {
		println("Next release found:", nextRelease.Series)
		newCode = nextRelease.Series
	}

	sourceFiles, err := ppa.ListSourceFiles()
	if err != nil {
		println("Error listing source files:", err.Error())
		return
	}
	for _, sf := range sourceFiles {

		// When sf.Suites contains stable skip
		isSupported := false
		if isUsingUbuntuReleaseCode(&sf) {
			isSupported = isUbuntuReleaseCodeSupported(&sf, newCode)
		} else {
			isSupported = true
			continue
		}

		check := "❌ " + sf.URIs[0]
		if isSupported {
			check = "✅"
		}
		fmt.Printf("%s - %s\n", sf.Path, check)


	}
}

var Non_UbuntuReleaseCodes = []string{
	"stable",
	"/",
	"",
	"multiverse",
}

func isUsingUbuntuReleaseCode(source *ppa.SourceFile) bool {

	if slices.Contains(source.Components, "multiverse") {
		return false
	}

	for _, suite := range source.Suites {
		if !slices.Contains(Non_UbuntuReleaseCodes, suite) {
			return true
		}
	}
	return false
}

func toInReleaseURL(source *ppa.SourceFile, code string) string {
	if code == "" {
		code = source.Suites[0]
	}

	baseURI := source.URIs[0]
	if baseURI[len(baseURI)-1] == '/' {
		baseURI = baseURI[:len(baseURI)-1]
	}

	return fmt.Sprintf("%s/dists/%s/InRelease", baseURI, code)
}

func isUbuntuReleaseCodeSupported(source *ppa.SourceFile, code string) bool {
	if code == "" {
		code = source.Suites[0]
	}

	inReleaseURL := toInReleaseURL(source, code)


	resp, err := http.Get(inReleaseURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Add logic to check if the Ubuntu release code is supported
	// For now, let's assume all codes are supported

	return resp.StatusCode == http.StatusOK
}
