package provisioner

// RemoteFileSpec represents a file that is restored on remote nodes
type RemoteFileSpec struct {
	Path string `yaml:"path"`

	// Content is the content of this file
	// Either Content or Source can be specified
	Content Content `yaml:"content,omitempty"`

	// Template is the template for the content of this file
	// that exists for backward-compatibility with CustomFile
	Template string `yaml:"template,omitempty"`

	// Permissions is the desired file mode of the file at `Path`, that looks like 0755
	//
	// kube-aws runs chmod on the file towards the desired file mode.
	//
	// This is optional. When omitted, kube-aws doesn't run chmod
	Permissions uint `yaml:"permissions"`

	// Source specifies how and from where the content of this file is loaded
	// Either Content or Source can be specified
	Source `yaml:"source"`

	// Type, when specified to binary, omits diff for this file
	Type string `yaml:"type"`

	// CachePath specifies where this file should be cached locally
	CachePath string
}

// RemoteFile is an instantiation of RemoteFileSpec
type RemoteFile struct {
	Path string `yaml:"path"`

	// Content is the content of this file
	// Either Content or Source can be specified
	Content Content `yaml:"content,omitempty"`

	// Permissions is the desired file mode of the file at `Path`, that looks like 0755
	//
	// kube-aws runs chmod on the file towards the desired file mode.
	//
	// This is optional. When omitted, kube-aws doesn't run chmod
	Permissions uint `yaml:"permissions"`

	// Type and Encrypted affects how kube-aws handles the file in transfer and decryption

	// Type, when specified to binary, omits diff for this file
	Type string `yaml:"type"`

	// Encrypted should be set to true when the content is encrypted with AWS KMS
	Encrypted bool `yaml:"encrypted"`
}

type Source struct {
	// Path is from where kube-aws loads the content of this `RemoteFileSpec`.
	Path string `yaml:"path"`

	// URL, when specified, instruct kube-aws to download the resource into `Path`.
	URL string `yaml:"url"`

	// Cert is the name of the keypair from which load the x509 cert
	Cert string `yaml:"cert"`

	// Cert is the name of the keypair from which load the x509 key
	Key string `yaml:"key"`
}

type Content struct {
	bytes []byte
	str   string
}

// TarGzArchiver is a archived bundle.
// TarGzArchiver is created, transferred, and then extracted to the etcd, controller and worker nodes to provide
// necessary files for node provisioning.
type TarGzArchiver struct {
	File RemoteFileSpec `yaml:",inline"`
	// Bundle is a set of files necessary for node provisioning, that is composed of multiple source files
	Bundle []RemoteFileSpec `yaml:"files"`
}

type TransferredFile struct {
	RemoteFileSpec
	s3DirURI string
}

type Provisioner struct {
	// Name is the name of the provisioner
	Name string `yaml:"name"`

	// EntrypointLocalPath is an executable file that is executed on the node after the bundle is transferred.
	// EntrypointLocalPath can either be one of bundled files that are transferred, or an already existing file on the remote node.
	EntrypointLocalPath string `yaml:"entrypoint"`

	// Bundle is the bundle the provisioner uses to provision nodes
	Bundle []RemoteFileSpec `yaml:"bundle,inline"`

	S3DirURI      string
	LocalCacheDir string
}
