package api

import (
	"testing"
)

func TestEtcd(t *testing.T) {
	etcdTest := Etcd{
		EC2Instance: EC2Instance{
			Count:        1,
			InstanceType: "t2.medium",
			RootVolume: RootVolume{
				Size: 30,
				Type: "gp2",
				IOPS: 0,
			},
			Tenancy: "default",
		},
		DataVolume: DataVolume{
			Size: 30,
			Type: "gp2",
			IOPS: 0,
		},
		StackExists: false,
		UserSuppliedArgs: UserSuppliedArgs{
			QuotaBackendBytes:       100000000,
			AutoCompactionRetention: 1,
		},
	}

	if etcdTest.LogicalName() != "Etcd" {
		t.Errorf("logical name incorrect, expected: Etcd, got: %s", etcdTest.LogicalName())
	}

	if etcdTest.NameTagKey() != "kube-aws:etcd:name" {
		t.Errorf("name tag key incorrect, expected: kube-aws:etcd:name, got: %s", etcdTest.NameTagKey())
	}

	if etcdTest.Version() != "3.2.13" {
		t.Errorf("etcd version incorrect, epxected: 3.2.13, got: %s", etcdTest.Version())
	}

	if !etcdTest.NodeShouldHaveEIP() {
		t.Error("expected: true, got: false")
	}

	if etcdTest.SystemdUnitName() != "etcd-member.service" {
		t.Errorf("etcd systemd unit name incorrect, expected: etcd-member.service, got %s", etcdTest.SystemdUnitName())
	}

	if etcdTest.SecurityGroupRefs()[0] != `{"Fn::ImportValue" : {"Fn::Sub" : "${NetworkStackName}-EtcdSecurityGroup"}}` {
		t.Errorf("etcd security group refs incorrect, expected: `{'Fn::ImportValue' : {'Fn::Sub' : '${NetworkStackName}-EtcdSecurityGroup'}}`, got: %s", etcdTest.SecurityGroupRefs()[0])
	}

	if err := etcdTest.Validate(); err != nil {
		t.Error(err)
	}

	if etcdTest.FormatOpts() != "--quota-backend-bytes=100000000 --auto-compaction-retention=1" {
		t.Errorf("etcd optional args incorrect, expected `--quota-backend-bytes=100000000 --auto-compaction-retention=1`, got: `%s`", etcdTest.FormatOpts())
	}
}
