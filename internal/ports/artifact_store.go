package ports

type ArtifactStore interface {
	SaveArtifact(
		runID string,
		artifactID string,
		extension string,
		contents []byte,
	) (string, error)
	ReadArtifact(relativePath string) ([]byte, error)
	ExportArtifact(relativePath string, destination string) error
	ExportContents(contents []byte, destination string) error
}
