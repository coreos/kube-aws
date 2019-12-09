package api

// ExistingState describes the existing state of the etcd cluster
type EtcdExistingState struct {
	EtcdMigrationEnabled           bool
	EtcdMigrationExistingEndpoints string
}
