package model

import (
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"reflect"
	"testing"
)

func TestEtcdClusterDNSNames(t *testing.T) {
	usEast1 := api.RegionForName("us-east-1")
	usWest1 := api.RegionForName("us-west-1")
	etcdNet := NewNetwork([]api.Subnet{}, []api.NATGateway{})
	etcdCount := 1

	t.Run("WithENI", func(t *testing.T) {
		t.Run("WithoutCustomDomain", func(t *testing.T) {
			config := api.EtcdCluster{
				MemberIdentityProvider: "eni",
			}
			t.Run("us-east-1", func(t *testing.T) {
				cluster := NewEtcdCluster(config, usEast1, etcdNet, etcdCount)
				actual := cluster.DNSNames()
				expected := []string{"*.ec2.internal"}
				if !reflect.DeepEqual(actual, expected) {
					t.Errorf("invalid dns names: expecetd=%v, got=%v", expected, actual)
				}
			})
			t.Run("us-west-1", func(t *testing.T) {
				cluster := NewEtcdCluster(config, usWest1, etcdNet, etcdCount)
				actual := cluster.DNSNames()
				expected := []string{"*.us-west-1.compute.internal"}
				if !reflect.DeepEqual(actual, expected) {
					t.Errorf("invalid dns names: expecetd=%v, got=%v", expected, actual)
				}
			})
		})
		t.Run("WithCustomDomain", func(t *testing.T) {
			config := api.EtcdCluster{
				MemberIdentityProvider: "eni",
				InternalDomainName:     "internal.example.com",
			}
			t.Run("us-east-1", func(t *testing.T) {
				cluster := NewEtcdCluster(config, usEast1, etcdNet, etcdCount)
				actual := cluster.DNSNames()
				expected := []string{"*.internal.example.com"}
				if !reflect.DeepEqual(actual, expected) {
					t.Errorf("invalid dns names: expecetd=%v, got=%v", expected, actual)
				}
			})
			t.Run("us-west-1", func(t *testing.T) {
				cluster := NewEtcdCluster(config, usWest1, etcdNet, etcdCount)
				actual := cluster.DNSNames()
				expected := []string{"*.internal.example.com"}
				if !reflect.DeepEqual(actual, expected) {
					t.Errorf("invalid dns names: expecetd=%v, got=%v", expected, actual)
				}
			})
		})
	})

	t.Run("WithEIP", func(t *testing.T) {
		config := api.EtcdCluster{
			MemberIdentityProvider: "eip",
		}
		t.Run("us-east-1", func(t *testing.T) {
			cluster := NewEtcdCluster(config, usEast1, etcdNet, etcdCount)
			actual := cluster.DNSNames()
			expected := []string{"*.compute-1.amazonaws.com"}
			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("invalid dns names: expecetd=%v, got=%v", expected, actual)
			}
		})
		t.Run("us-west-1", func(t *testing.T) {
			cluster := NewEtcdCluster(config, usWest1, etcdNet, etcdCount)
			actual := cluster.DNSNames()
			expected := []string{"*.us-west-1.compute.amazonaws.com"}
			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("invalid dns names: expecetd=%v, got=%v", expected, actual)
			}
		})
	})
}
