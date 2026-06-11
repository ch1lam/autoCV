package ports

type ManagedFileStore interface {
	SaveMarkdown(profileID string, documentID string, contents []byte) (string, error)
	Read(string) ([]byte, error)
	Delete(string) error
}
