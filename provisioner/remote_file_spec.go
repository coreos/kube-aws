package provisioner

import (
	"os"
	"path/filepath"
)

func (f RemoteFileSpec) Load(loader *RemoteFileLoader) (*RemoteFile, error) {
	return loader.Load(f)
}

func (f RemoteFileSpec) BaseName() string {
	return filepath.Base(f.Source.Path)
}

func (f RemoteFileSpec) FileMode() *os.FileMode {
	if f.Permissions != 0 {
		mode := os.FileMode(f.Permissions)
		return &mode
	}
	return nil
}

func (f RemoteFileSpec) IsBinary() bool {
	return f.Type == "binary"
}
