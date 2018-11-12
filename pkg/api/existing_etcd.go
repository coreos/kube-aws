package api

// ExistingState describes the existing state of the etcd cluster
type EtcdExistingState struct {
	StackExists                    bool
	EtcdMigrationEnabled           bool
	EtcdMigrationExistingEndpoints string
}
