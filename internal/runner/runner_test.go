package runner

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestPrimaryFingerprints(t *testing.T) {
	// Saída real (resumida) de "gpg --with-colons --show-keys" do keyring do
	// GitHub CLI: duas chaves primárias, cada uma com uma subchave.
	saida := `pub:e:4096:1:23F3D4EA75716059:1631745044:1726353044::-:::sc::::::23::0:
fpr:::::::::2C6106201985B60E6C7AC87323F3D4EA75716059:
uid:e::::1631745044::5C1FD3A3B4CD1A12::GitHub CLI <opensource+cli@github.com>::::::::::0:
sub:e:4096:1:E5FAF19590714157:1631745044:1726353044:::::e::::::23:
fpr:::::::::5700BAB26C8DE75F3EE323FEE5FAF19590714157:
pub:-:4096:1:5612B36462313325:1725380000:1882000000::-:::sc::::::23::0:
fpr:::::::::7F38BBB59D064DBCB3D84D725612B36462313325:
sub:-:4096:1:F4CAB2C46C97E579:1725380000:1882000000:::::e::::::23:
fpr:::::::::B84252FAAA164D9EBEA2E2C1F4CAB2C46C97E579:
`
	got := primaryFingerprints(saida)
	want := []string{"2C6106201985B60E6C7AC87323F3D4EA75716059", "7F38BBB59D064DBCB3D84D725612B36462313325"}
	if !slices.Equal(got, want) {
		t.Errorf("primaryFingerprints = %v, esperado %v", got, want)
	}
}

func TestVerifySHA256(t *testing.T) {
	f := filepath.Join(t.TempDir(), "x")
	if err := os.WriteFile(f, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	const abc = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if err := VerifySHA256(f, abc); err != nil {
		t.Errorf("hash correto recusado: %v", err)
	}
	if err := VerifySHA256(f, "00"+abc[2:]); err == nil {
		t.Error("hash errado aceito")
	}
}
