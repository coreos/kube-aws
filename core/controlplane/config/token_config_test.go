package config

import (
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/test/helper"
)

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
	authTokens := NewAuthTokens()

	if len(authTokens.Contents) > 0 {
		t.Errorf("expected default auth tokens to be an empty string, but was %v", authTokens.Contents)
	}
}

func TestRandomKubeletBootstrapTokenString(t *testing.T) {
	bytelen := 2048

	randomToken, err := RandomKubeletBootstrapTokenString(bytelen)
	if err != nil {
		t.Errorf("failed to generate a Kubelet bootstrap token: %v", err)
	}
	if strings.Index(randomToken, ",") >= 0 {
		t.Errorf("random token not expect to contain a comma: %v", randomToken)
	}

	b, err := base64.URLEncoding.DecodeString(randomToken)
	if err != nil {
		t.Errorf("failed to decode base64 token string: %v", err)
	}
	if len(b) != bytelen {
		t.Errorf("expected token to be %d bits long, but was %d", bytelen, len(b))
	}
}

func TestAuthTokensFileExists(t *testing.T) {
	t.Run("NoAuthTokenFile", func(t *testing.T) {
		helper.WithTempDir(func(dir string) {
			exists := AuthTokensFileExists(dir)
			if exists {
				t.Errorf("Expected auth token file not to exist, but it does")
			}
		})
	})

	t.Run("EmptyAuthTokenFile", func(t *testing.T) {
		helper.WithTempDir(func(dir string) {
			authTokensFile := fmt.Sprintf("%s/tokens.csv", dir)
			if err := ioutil.WriteFile(authTokensFile, []byte(""), 0600); err != nil {
				t.Errorf("Error writing the empty auth token file: %v", err)
			}

			exists := AuthTokensFileExists(dir)
			if exists {
				t.Errorf("Expected auth token file not to exist, but it does")
			}
		})
	})

	t.Run("NonEmptyAuthTokenFile", func(t *testing.T) {
		helper.WithTempDir(func(dir string) {
			authTokensFile := fmt.Sprintf("%s/tokens.csv", dir)
			if err := ioutil.WriteFile(authTokensFile, []byte("dummy-token"), 0600); err != nil {
				t.Errorf("Error writing the empty auth token file: %v", err)
			}

			exists := AuthTokensFileExists(dir)
			if !exists {
				t.Errorf("Expected auth token file to exist, but it does not")
			}
		})
	})
}

func TestRandomBootstrapTokenRecord(t *testing.T) {
	record, err := RandomBootstrapTokenRecord()
	if err != nil {
		t.Errorf("failed to generate a Kubelet bootstrap token record: %v", err)
	}

	csvReader := csv.NewReader(strings.NewReader(record))
	readRecords, err := csvReader.ReadAll()
	if err != nil {
		t.Errorf("failed to read the Kubelet bootstrap token record as a CSV file: %v", err)
	}

	if len(readRecords) != 1 {
		t.Errorf("expected CSV to have 1 line, but has %d", len(readRecords))
	}

	if len(readRecords[0]) != 4 {
		t.Errorf("expected CSV to have 4 columns, but has %d", len(readRecords[0]))
	}
}

func TestCreateRawAuthTokens(t *testing.T) {
	t.Run("EmptyAuthTokenFile", func(t *testing.T) {
		helper.WithTempDir(func(dir string) {
			filename := fmt.Sprintf("%s/tokens.csv", dir)
			created, err := CreateRawAuthTokens(false, dir)
			if err != nil {
				t.Errorf("expected error to be nil, but was: %v", err)
			}

			// Do not create the auth token file if there's no bootstrap token to be added
			if created {
				t.Errorf("expected auth token file not to be created, but it was")
			}

			if _, err := os.Stat(filename); err == nil {
				t.Errorf("expected file not to exist, but it does: %v", filename)
			}
		})
	})

	t.Run("TokenFileWithBootstrapToken", func(t *testing.T) {
		helper.WithTempDir(func(dir string) {
			filename := fmt.Sprintf("%s/tokens.csv", dir)
			created, err := CreateRawAuthTokens(true, dir)
			if err != nil {
				t.Errorf("expected err to be nil, but was: %v", err)
			}

			// Create auth token file with random bootstrap token in it
			if !created {
				t.Errorf("expected auth token file to be created, but it was not")
			}

			contents, err := ioutil.ReadFile(filename)
			if err != nil {
				t.Errorf("expected err to be nil, but was: %v", err)
			}
			if len(contents) == 0 {
				t.Error("expected auth token file not to be empty, but it is")
			}
		})
	})
}

func TestKubeletBootstrapTokenFromRecord(t *testing.T) {
	testCases := []struct {
		groups   string
		expected bool
	}{
		{
			// No groups
			groups:   "  ",
			expected: false,
		},
		{
			// Only the bootstrap group
			groups:   "  system:kubelet-bootstrap  ",
			expected: true,
		},
		{
			// Only some regular  group
			groups:   "  some-group  ",
			expected: false,
		},
		{
			// Bootstrap group after regular group
			groups:   "  some-group   ,  system:kubelet-bootstrap  ",
			expected: true,
		},
		{
			// Bootstrap group before regular group
			groups:   "   system:kubelet-bootstrap  ,  some-group  ",
			expected: true,
		},
		{
			// Invalid bootstrap group
			groups:   "   system:kubelet-bootstraps  ",
			expected: false,
		},
		{
			// Invalid bootstrap group
			groups:   "   ssystem:kubelet-bootstrap  ",
			expected: false,
		},
		{
			// Invalid group syntax
			groups:   "   system:kubelet-bootstrap  ,  some-group  ,  ",
			expected: false,
		},
		{
			// Invalid group syntax
			groups:   "  ,  system:kubelet-bootstrap  ,  some-group  ",
			expected: false,
		},
	}

	for _, test := range testCases {
		record := []string{"token", "user-name", "user-id", test.groups}

		token, err := KubeletBootstrapTokenFromRecord(record)
		if err != nil {
			t.Errorf("expected error to be nil, but was %v", err)
		}

		if !test.expected && (token == "token") {
			t.Errorf("expected kubelet token not to be found in the record %v, but it was", record)
		}

		if test.expected && (token != "token") {
			t.Errorf("expected kubelet token to be found in the record %v, but it was not", record)
		}
	}
}

func TestReadOrCreateCompactEmptyAuthTokens(t *testing.T) {
	helper.WithDummyCredentials(func(dir string) {
		kmsConfig := KMSConfig{
			KMSKeyARN:      "keyarn",
			Region:         model.RegionForName("us-west-1"),
			EncryptService: &dummyEncryptService{},
		}

		// See https://github.com/kubernetes-incubator/kube-aws/issues/107
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
			created, err := ReadOrCreateUnencryptedCompactAuthTokens(dir)

			if err != nil {
				t.Errorf("failed to read or update compact auth tokens in %s : %v", dir, err)
			}

			read, err := ReadOrCreateUnencryptedCompactAuthTokens(dir)

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

		// See https://github.com/kubernetes-incubator/kube-aws/issues/107
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

func TestReadOrCreateNonEmptyIncvalidUnEncryptedCompactAuthTokens(t *testing.T) {
	helper.WithDummyCredentials(func(dir string) {
		writeSampleInvalidAuthTokenFile(dir, t)
		_, err := ReadOrCreateUnencryptedCompactAuthTokens(dir)

		if err == nil {
			t.Errorf("expected invalid token file to return an error, but it didn't")
		}
	})
}
