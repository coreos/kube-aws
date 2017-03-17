package config

import (
	"testing"

	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"github.com/coreos/kube-aws/model"
	"github.com/coreos/kube-aws/test/helper"
)

func genAuthTokens(t *testing.T) *RawAuthTokens {
	cluster, err := ClusterFromBytes([]byte(singleAzConfigYaml))
	if err != nil {
		t.Fatalf("failed generating config: %v", err)
	}

	return cluster.NewAuthTokens()
}

// The default token file is empty, and since encryption/compaction only
// happens if the token file is not empty, we need a way to create a sample
// token file in order to cover both scenarios
func writeSampleValidAuthTokenFile(dir string, t *testing.T) {
	err := ioutil.WriteFile(fmt.Sprintf("%s/tokens.csv", dir), []byte("token,user,1,group"), 0600)
	if err != nil {
		t.Errorf("failed to create sample valid auth tokens files: %v", err)
	}
}

func writeSampleInvalidAuthTokenFile(dir string, t *testing.T) {
	err := ioutil.WriteFile(fmt.Sprintf("%s/tokens.csv", dir), []byte("# invalid token record"), 0600)
	if err != nil {
		t.Errorf("failed to create sample invalid auth tokens files: %v", err)
	}
}

func TestAuthTokenGeneration(t *testing.T) {
	authTokens := genAuthTokens(t)

	if len(authTokens.Contents) > 0 {
		t.Errorf("expected default auth tokens to be an empty string, but was %v", authTokens.Contents)
	}
}

func TestReadOrCreateCompactEmptyAuthTokens(t *testing.T) {
	helper.WithDummyCredentials(func(dir string) {
		kmsConfig := KMSConfig{
			KMSKeyARN:      "keyarn",
			Region:         model.RegionForName("us-west-1"),
			EncryptService: &dummyEncryptService{},
		}

		// See https://github.com/coreos/kube-aws/issues/107
		t.Run("CachedToPreventUnnecessaryNodeReplacement", func(t *testing.T) {
			created, err := ReadOrCreateCompactAuthTokens(dir, kmsConfig)

			if err != nil {
				t.Errorf("failed to read or update compact auth tokens in %s : %v", dir, err)
			}

			if len(created.Contents) > 0 {
				t.Errorf("compacted auth tokens expected to be an empty string, but was %s", created.Contents)
			}

			// This depends on TestDummyEncryptService which ensures dummy encrypt service to produce different ciphertext for each encryption
			// created == read means that encrypted assets were loaded from cached files named *.enc, instead of re-encrypting token files
			// TODO Use some kind of mocking framework for tests like this
			read, err := ReadOrCreateCompactAuthTokens(dir, kmsConfig)

			if err != nil {
				t.Errorf("failed to read or update compact auth tokens in %s : %v", dir, err)
			}

			if !reflect.DeepEqual(created, read) {
				t.Errorf(`failed to cache encrypted auth tokens.
	encrypted auth tokens must not change after their first creation but they did change:
	created = %v
	read = %v`, created, read)
			}
		})
	})
}

func TestReadOrCreateEmptyUnEcryptedCompactAuthTokens(t *testing.T) {
	helper.WithDummyCredentials(func(dir string) {
		t.Run("CachedToPreventUnnecessaryNodeReplacementOnUnencrypted", func(t *testing.T) {
			created, err := ReadOrCreateUnecryptedCompactAuthTokens(dir)

			if err != nil {
				t.Errorf("failed to read or update compact auth tokens in %s : %v", dir, err)
			}

			read, err := ReadOrCreateUnecryptedCompactAuthTokens(dir)

			if err != nil {
				t.Errorf("failed to read or update compact auth tokens in %s : %v", dir, err)
			}

			if !reflect.DeepEqual(created, read) {
				t.Errorf(`failed to cache unencrypted auth tokens.
 	unencrypted auth tokens must not change after their first creation but they did change:
 	created = %v
 	read = %v`, created, read)
			}
		})
	})
}

func TestReadOrCreateCompactNonEmptyValidAuthTokens(t *testing.T) {
	helper.WithDummyCredentials(func(dir string) {
		kmsConfig := KMSConfig{
			KMSKeyARN:      "keyarn",
			Region:         model.RegionForName("us-west-1"),
			EncryptService: &dummyEncryptService{},
		}

		writeSampleValidAuthTokenFile(dir, t)

		// See https://github.com/coreos/kube-aws/issues/107
		t.Run("CachedToPreventUnnecessaryNodeReplacement", func(t *testing.T) {
			created, err := ReadOrCreateCompactAuthTokens(dir, kmsConfig)

			if err != nil {
				t.Errorf("failed to read or update compact auth tokens in %s : %v", dir, err)
			}

			// This depends on TestDummyEncryptService which ensures dummy encrypt service to produce different ciphertext for each encryption
			// created == read means that encrypted assets were loaded from cached files named *.enc, instead of re-encrypting token files
			// TODO Use some kind of mocking framework for tests like this
			read, err := ReadOrCreateCompactAuthTokens(dir, kmsConfig)

			if err != nil {
				t.Errorf("failed to read or update compact auth tokens in %s : %v", dir, err)
			}

			if !reflect.DeepEqual(created, read) {
				t.Errorf(`failed to cache encrypted auth tokens.
	encrypted auth tokens must not change after their first creation but they did change:
	created = %v
	read = %v`, created, read)
			}
		})

		t.Run("RemoveAuthTokenCacheFileToRegenerate", func(t *testing.T) {
			original, err := ReadOrCreateCompactAuthTokens(dir, kmsConfig)

			if err != nil {
				t.Errorf("failed to read the original encrypted auth tokens : %v", err)
			}

			if err := os.Remove(filepath.Join(dir, "tokens.csv.enc")); err != nil {
				t.Errorf("failed to remove tokens.csv.enc for test setup : %v", err)
				t.FailNow()
			}

			regenerated, err := ReadOrCreateCompactAuthTokens(dir, kmsConfig)

			if err != nil {
				t.Errorf("failed to read the regenerated encrypted auth tokens : %v", err)
			}

			if original.Contents == regenerated.Contents {
				t.Errorf("Auth token file contents must change but it didn't : original = %v, regenrated = %v ", original.Contents, regenerated.Contents)
			}

			if reflect.DeepEqual(original, regenerated) {
				t.Errorf(`unexpecteed data contained in (possibly) regenerated encrypted auth tokens.
	encrypted auth tokens must change after regeneration but they didn't:
	original = %v
	regenerated = %v`, original, regenerated)
			}
		})
	})
}

func TestReadOrCreateCompactNonEmptyInvalidAuthTokens(t *testing.T) {
	helper.WithDummyCredentials(func(dir string) {
		kmsConfig := KMSConfig{
			KMSKeyARN:      "keyarn",
			Region:         model.RegionForName("us-west-1"),
			EncryptService: &dummyEncryptService{},
		}

		writeSampleInvalidAuthTokenFile(dir, t)
		_, err := ReadOrCreateCompactAuthTokens(dir, kmsConfig)

		if err == nil {
			t.Errorf("expected invalid token file to return an error, but it didn't")
		}
	})
}

func TestReadOrCreateNonEmptyIncvalidUnEcryptedCompactAuthTokens(t *testing.T) {
	helper.WithDummyCredentials(func(dir string) {
		writeSampleInvalidAuthTokenFile(dir, t)
		_, err := ReadOrCreateUnecryptedCompactAuthTokens(dir)

		if err == nil {
			t.Errorf("expected invalid token file to return an error, but it didn't")
		}
	})
}
