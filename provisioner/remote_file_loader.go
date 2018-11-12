package provisioner

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
)

type RemoteFileLoader struct {
}

func (loader *RemoteFileLoader) Load(f RemoteFileSpec) (*RemoteFile, error) {
	loaded := NewRemoteFile(f)

	path := f.Source.Path

	// TODO
	cachePath := path

	if path != "" {
		if f.Type == "credential" {
			path = path + ".enc"
		} else {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				if f.URL != "" {
					fmt.Fprintf(os.Stderr, "downloading %s\n", f.URL)
					err := download(f.URL, cachePath)
					if err != nil {
						return nil, fmt.Errorf("failed downloading %s: %v", f.URL, err)
					}
					mode := f.FileMode()
					if mode != nil {
						if err := os.Chmod(cachePath, *mode); err != nil {
							return nil, fmt.Errorf("failed to chmod %s: %v", path, err)
						}
					}
				} else if len(f.Content.String()) > 0 {
					err := ioutil.WriteFile(cachePath, f.Content.bytes, *f.FileMode())
					if err != nil {
						return nil, fmt.Errorf("failed to write %s: %v", cachePath, err)
					}
				} else {
					return nil, fmt.Errorf("%s not found", path)
				}
			}
		}

		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed loading %s: %v", path, err)
		}

		loaded.Content = NewBinaryContent(data)
	} else {
		loaded.Content = f.Content
	}

	return loaded, nil
}

func download(url string, dest string) error {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed creating dir %s: %v", dir, err)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
