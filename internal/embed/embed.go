//go:generate go run cmd/generator/main.go

package embed

// box contains a map of filenames to the contents of the files
// as a slice of bytes.
type box struct {
	files map[string][]byte
}

func newBox() *box {
	return &box{files: make(map[string][]byte)}
}

func (e *box) Add(file string, content []byte) {
	e.files[file] = content
}

func (e *box) Get(file string) []byte {
	if f, ok := e.files[file]; ok {
		return f
	}
	return []byte{}
}

func (e *box) Has(file string) bool {
	if _, ok := e.files[file]; ok {
		return true
	}
	return false
}

var b = newBox()

// Add embed a file in the box.
func Add(file string, content []byte) {
	b.Add(file, content)
}

// Get a file from box
func Get(file string) []byte {
	return b.Get(file)
}

// Has a file in box
func Has(file string) bool {
	return b.Has(file)
}
