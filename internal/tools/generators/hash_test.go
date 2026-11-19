package generators_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Judamaro/devtooltui-golang/internal/testutil"
	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/generators"
)

func runHash(t testing.TB, in string, opts tool.Values) (tool.Result, error) {
	t.Helper()
	return generators.Hash{}.Run(context.Background(), tool.Request{
		Inputs: map[string]string{"input": in},
		Opts:   opts,
	})
}

func TestHashGolden(t *testing.T) {
	in := testutil.ReadInput(t, "hash", "text")
	var all []string
	for _, algo := range []string{"md5", "sha1", "sha256", "sha512"} {
		for _, key := range []string{"", "secreto"} {
			for _, enc := range []string{"hex", "base64"} {
				res, err := runHash(t, in, tool.Values{"algo": algo, "key": key, "encoding": enc})
				if err != nil {
					t.Fatal(err)
				}
				all = append(all, algo+" key="+key+" "+enc+": "+res.Text)
			}
		}
	}
	testutil.Golden(t, "hash", "text-all", strings.Join(all, "\n"))
}

// FIPS 180 and RFC 4231 (HMAC-SHA256, test case 2) vectors.
func TestHashVectors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		opts tool.Values
		want string
	}{
		{"sha256 empty", "", nil, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"md5 abc", "abc", tool.Values{"algo": "md5"}, "900150983cd24fb0d6963f7d28e17f72"},
		{"sha1 abc", "abc", tool.Values{"algo": "sha1"}, "a9993e364706816aba3e25717850c26c9cd0d89d"},
		{"sha256 abc", "abc", nil, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{"sha512 abc", "abc", tool.Values{"algo": "sha512"},
			"ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"},
		{"hmac rfc4231", "what do ya want for nothing?", tool.Values{"key": "Jefe"},
			"5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843"},
		{"uppercase", "abc", tool.Values{"algo": "md5", "uppercase": true}, "900150983CD24FB0D6963F7D28E17F72"},
		{"base64", "abc", tool.Values{"algo": "md5", "encoding": "base64"}, "kAFQmDzST7DWlj99KOF/cg=="},
		{"base64 ignores uppercase", "abc", tool.Values{"algo": "md5", "encoding": "base64", "uppercase": true}, "kAFQmDzST7DWlj99KOF/cg=="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := runHash(t, tt.in, tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Text != tt.want {
				t.Errorf("Text = %s, want %s", res.Text, tt.want)
			}
		})
	}
}

func TestHashMeta(t *testing.T) {
	tests := []struct {
		opts       tool.Values
		algo, bits string
	}{
		{nil, "SHA256", "256"},
		{tool.Values{"algo": "sha512", "key": "k"}, "HMAC-SHA512", "512"},
		{tool.Values{"algo": "md5"}, "MD5", "128"},
	}
	for _, tt := range tests {
		t.Run(tt.algo, func(t *testing.T) {
			res, err := runHash(t, "x", tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Meta[0].Value != tt.algo || res.Meta[1].Value != tt.bits {
				t.Errorf("Meta = %v", res.Meta)
			}
		})
	}
}

func TestHashUnknownAlgo(t *testing.T) {
	if _, err := runHash(t, "x", tool.Values{"algo": "crc32"}); err == nil {
		t.Error("an algorithm outside Choices returned no error")
	}
}

func BenchmarkHashSHA256_1MB(b *testing.B) {
	in := strings.Repeat("a", 1<<20)
	b.SetBytes(int64(len(in)))
	for b.Loop() {
		if _, err := runHash(b, in, nil); err != nil {
			b.Fatal(err)
		}
	}
}
