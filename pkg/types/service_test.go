package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeBaseService() *Service {
	return &Service{
		ID:        "svc-1",
		Name:      "test-svc",
		Namespace: "default",
		Image:     "repo/app:1.0",
		Command:   "run",
		Args:      []string{"--foo", "bar"},
		Scale:     1,
		Runtime:   RuntimeTypeContainer,
		Env:       map[string]string{"A": "1", "B": "2"},
	}
}

func TestServiceHash_EnvOrderInsensitive(t *testing.T) {
	s1 := makeBaseService()
	// Insert in one order
	s1.Env = map[string]string{"A": "1", "B": "2"}
	h1 := s1.CalculateHash()

	// Same keys, different insertion order
	s2 := makeBaseService()
	s2.Env = map[string]string{"B": "2", "A": "1"}
	h2 := s2.CalculateHash()

	assert.Equal(t, h1, h2, "hash should be stable regardless of env map insertion order")
}

func TestServiceHash_ChangesWhenCoreFieldsChange(t *testing.T) {
	s := makeBaseService()
	h1 := s.CalculateHash()

	s.Image = "repo/app:2.0"
	h2 := s.CalculateHash()
	assert.NotEqual(t, h1, h2, "changing image should change hash")

	s.Image = "repo/app:1.0"
	s.Scale = 2
	h3 := s.CalculateHash()
	assert.NotEqual(t, h1, h3, "changing scale should change hash")
}

func TestServiceHash_SecretMountsOrderInsensitive(t *testing.T) {
	// Unsorted items and mounts
	s1 := makeBaseService()
	s1.SecretMounts = []SecretMount{
		{
			Name:       "b-mount",
			MountPath:  "/secrets/b",
			SecretName: "sec-b",
			Items:      []KeyToPath{{Key: "y", Path: "y.txt"}, {Key: "x", Path: "x.txt"}},
		},
		{
			Name:       "a-mount",
			MountPath:  "/secrets/a",
			SecretName: "sec-a",
			Items:      []KeyToPath{{Key: "b", Path: "b.txt"}, {Key: "a", Path: "a.txt"}},
		},
	}
	h1 := s1.CalculateHash()

	// Same data, different order
	s2 := makeBaseService()
	s2.SecretMounts = []SecretMount{
		{
			Name:       "a-mount",
			MountPath:  "/secrets/a",
			SecretName: "sec-a",
			Items:      []KeyToPath{{Key: "a", Path: "a.txt"}, {Key: "b", Path: "b.txt"}},
		},
		{
			Name:       "b-mount",
			MountPath:  "/secrets/b",
			SecretName: "sec-b",
			Items:      []KeyToPath{{Key: "x", Path: "x.txt"}, {Key: "y", Path: "y.txt"}},
		},
	}
	h2 := s2.CalculateHash()

	assert.Equal(t, h1, h2, "hash should not depend on order of secret mounts or their items")

	// Changing an item should change the hash
	s2.SecretMounts[0].Items[0].Path = "a2.txt"
	h3 := s2.CalculateHash()
	assert.NotEqual(t, h1, h3, "modifying a secret mount item should change hash")
}

func TestServiceHash_ConfigmapMountsOrderInsensitive(t *testing.T) {
	s1 := makeBaseService()
	s1.ConfigmapMounts = []ConfigmapMount{
		{
			Name:       "cm2",
			MountPath:  "/config/2",
			ConfigName: "cfg-2",
			Items:      []KeyToPath{{Key: "k2", Path: "p2"}, {Key: "k1", Path: "p1"}},
		},
		{
			Name:       "cm1",
			MountPath:  "/config/1",
			ConfigName: "cfg-1",
			Items:      []KeyToPath{{Key: "a", Path: "pa"}, {Key: "b", Path: "pb"}},
		},
	}
	h1 := s1.CalculateHash()

	s2 := makeBaseService()
	s2.ConfigmapMounts = []ConfigmapMount{
		{
			Name:       "cm1",
			MountPath:  "/config/1",
			ConfigName: "cfg-1",
			Items:      []KeyToPath{{Key: "b", Path: "pb"}, {Key: "a", Path: "pa"}},
		},
		{
			Name:       "cm2",
			MountPath:  "/config/2",
			ConfigName: "cfg-2",
			Items:      []KeyToPath{{Key: "k1", Path: "p1"}, {Key: "k2", Path: "p2"}},
		},
	}
	h2 := s2.CalculateHash()

	assert.Equal(t, h1, h2, "hash should not depend on order of configmap mounts or their items")
}
