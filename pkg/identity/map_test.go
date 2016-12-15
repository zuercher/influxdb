package identity_test

import (
	"bytes"
	"testing"

	"github.com/influxdata/influxdb/pkg/identity"
)

func TestMap_Get(t *testing.T) {
	m := identity.NewMap()

	id, ok := m.Set([]byte("test"))
	if !ok {
		t.Fatalf("Set ok mismatch: exp true, got %v", ok)
	}

	id, ok = m.Set([]byte("test"))
	if ok {
		t.Fatalf("Set ok mismatch: exp false, got %v", ok)
	}

	if exp, got := []byte("test"), m.Get(id); !bytes.Equal(exp, got) {
		t.Fatalf("Get mismatch: exp %v, got %v", exp, got)
	}
}

func TestMap_Get_Invalid(t *testing.T) {
	m := identity.NewMap()

	if exp, got := []byte(""), m.Get(0); !bytes.Equal(exp, got) {
		t.Fatalf("Get mismatch: exp %v, got %v", exp, got)
	}

	if exp, got := []byte(""), m.Get(-10); !bytes.Equal(exp, got) {
		t.Fatalf("Get mismatch: exp %v, got %v", exp, got)
	}
}

func TestMap_Remove(t *testing.T) {
	m := identity.NewMap()

	id, _ := m.Set([]byte("test"))

	if exp, got := []byte("test"), m.Get(id); !bytes.Equal(exp, got) {
		t.Fatalf("Get mismatch: exp %v, got %v", exp, got)
	}

	if _, ok := m.Remove([]byte("test")); !ok {
		t.Fatalf("Remove mismatch: exp true, got %v", ok)
	}

	if exp, got := []byte(""), m.Get(id); !bytes.Equal(exp, got) {
		t.Fatalf("Get mismatch: exp %v, got %v", exp, got)
	}

	if _, ok := m.Remove([]byte("test")); ok {
		t.Fatalf("Remove mismatch: exp false, got %v", ok)
	}
}
