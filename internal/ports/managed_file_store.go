package ports

type ManagedFileStore interface {
	SaveMarkdown(profileID string, documentID string, contents []byte) (string, error)
	SaveDOCX(profileID string, documentID string, contents []byte) (string, error)
	Read(string) ([]byte, error)
	ExportContents(contents []byte, destination string) error
	Delete(string) error
}
