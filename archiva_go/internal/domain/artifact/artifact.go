package artifact

type MavenArtifact struct {
	RepoId     string
	GroupID    string
	ArtifactID string
	Version    string
	Packaging  string
	Filename   string
	Path       string
	Checksums  Checksums
}

type Checksums struct {
	SHA1 string
	MD5  string
}
