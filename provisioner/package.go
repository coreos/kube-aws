package provisioner

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (p TarGzArchiver) Name() string {
	return filepath.Base(p.File.Path)
}

func CreateTarGzArchive(archive RemoteFileSpec, bundle []RemoteFileSpec, loader *RemoteFileLoader) error {
	files := []*RemoteFile{}
	for _, s := range bundle {
		f, err := loader.Load(s)
		if err != nil {
			return fmt.Errorf("failed materializing %s: %v", s.Path, err)
		}
		files = append(files, f)
	}

	localpath := archive.Source.Path
	dir := filepath.Dir(localpath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed creating dir %s: %v", dir, err)
	}

	file, err := os.Create(localpath)
	if err != nil {
		return fmt.Errorf("failed creating %s: %v", localpath, err)
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)

	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for i := range bundle {
		if err := tarWriteFile(tarWriter, bundle[i]); err != nil {
			return fmt.Errorf("failed writing tar: %v", err)
		}
	}

	return nil
}

func tarWriteFile(tarWriter *tar.Writer, file RemoteFileSpec) error {
	var reader io.Reader
	var header *tar.Header

	srcpath := file.Source.Path
	if srcpath != "" {
		srcfile, err := os.Open(srcpath)
		if err != nil {
			return err
		}
		defer srcfile.Close()

		stat, err := srcfile.Stat()
		if err != nil {
			return err
		}

		header, err = tar.FileInfoHeader(stat, "")
		if err != nil {
			return err
		}
		reader = srcfile
	} else {
		header = new(tar.Header)
		header.Size = int64(len(file.Content.bytes))
		header.Mode = int64(*file.FileMode())
		header.ModTime = time.Now()
		reader = bytes.NewBuffer(file.Content.bytes)
	}

	dstpath := file.Path
	intarpath := strings.TrimPrefix(dstpath, "/")

	fmt.Fprintf(os.Stderr, "archiving %s as %s\n", srcpath, intarpath)

	header.Name = intarpath

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	if _, err := io.Copy(tarWriter, reader); err != nil {
		return err
	}
	return nil
}
