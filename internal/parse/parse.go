package parse

import "io"

// Parse reads all input and returns a placeholder AST.
func Parse(r io.Reader) (*File, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &File{Text: string(data)}, nil
}
