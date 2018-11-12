package provisioner

import (
	"fmt"
)

func NewTarballingProvisioner(name string, bundle []RemoteFileSpec, entrypointLocalFilePath, s3DirURI string, pkgCacheDir string) *Provisioner {
	prov := &Provisioner{
		Name:                name,
		EntrypointLocalPath: entrypointLocalFilePath,
		Bundle:              bundle,
		S3DirURI:            s3DirURI,
		LocalCacheDir:       pkgCacheDir,
	}

	return prov
}

func (p *Provisioner) GetTransferredFile() TransferredFile {
	pkgFileName := fmt.Sprintf("%s.tgz", p.Name)

	pkgLocalPath := fmt.Sprintf("%s/%s", p.LocalCacheDir, pkgFileName)

	archive := RemoteFileSpec{
		Path:   fmt.Sprintf("/var/run/coreos/%s", pkgFileName),
		Source: Source{Path: pkgLocalPath},
	}

	return TransferredFile{
		archive,
		p.S3DirURI,
	}
}

func (p *Provisioner) EntrypointRemotePath() string {
	return "/opt/bin/entrypoint"
}

func (p *Provisioner) CreateTransferredFile(loader *RemoteFileLoader) (*TransferredFile, error) {
	transferredFile := p.GetTransferredFile()

	files := []RemoteFileSpec{}

	files = append(files, p.Bundle...)
	if p.EntrypointLocalPath != "" {
		entry := RemoteFileSpec{
			Path: p.EntrypointRemotePath(),
			Source: Source{
				Path: p.EntrypointLocalPath,
			},
			Permissions: 755,
		}
		files = append(files, entry)
	}

	err := CreateTarGzArchive(transferredFile.RemoteFileSpec, files, loader)
	if err != nil {
		return nil, fmt.Errorf("failed creating package: %v", err)
	}

	return &transferredFile, nil
}

func (p *Provisioner) Send(s3Client S3ObjectPutter) error {
	trans := p.GetTransferredFile()
	if err := trans.Send(s3Client); err != nil {
		return fmt.Errorf("failed sending package: %v", err)
	}
	return nil
}

func (p *Provisioner) RemoteCommand() (string, error) {
	// Download the archive when and only when there are one or more files to transfer
	if p != nil && len(p.Bundle) > 0 {
		trans := p.GetTransferredFile()
		cmd := fmt.Sprintf(`run bash -c "%s" && tar zxvf %s -C /`, trans.ReceiveCommand(), trans.Path)
		// Run the entrypoint command when and only when it is specified
		if p.EntrypointLocalPath != "" {
			cmd = cmd + " && " + p.EntrypointRemotePath()
		}
		return cmd, nil
	}
	return "", nil
}
