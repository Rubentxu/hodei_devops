package artifact

import "encoding/xml"

type MavenMetadata struct {
	XMLName    xml.Name       `xml:"metadata"`
	GroupID    string         `xml:"groupId"`
	ArtifactID string         `xml:"artifactId"`
	Versioning VersioningInfo `xml:"versioning"`
	Path       string
}

type VersioningInfo struct {
	Latest      string   `xml:"latest,omitempty"`
	Release     string   `xml:"release,omitempty"`
	Versions    []string `xml:"versions>version,omitempty"`
	LastUpdated string   `xml:"lastUpdated,omitempty"`
}
