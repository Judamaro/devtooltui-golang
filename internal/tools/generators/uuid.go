package generators

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/converters"
)

// UUID generates UUID v4/v7 and ULID, or inspects an existing one. Now and
// Rand pin the clock and randomness in tests; when nil, time.Now and
// crypto/rand are used.
type UUID struct {
	Now  func() time.Time
	Rand io.Reader
}

func (UUID) Spec() tool.Spec {
	return tool.Spec{
		ID:          "uuid",
		Name:        "UUID / ULID",
		Category:    tool.Generators,
		Description: "Generate UUID v4, v7 or ULID; with an input, inspect it",
		Keywords:    []string{"guid", "ulid", "uuidv4", "uuidv7", "id"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "UUID or ULID to inspect (empty = generate)"}},
		Options: []tool.Option{
			{Key: "version", Label: "Version", Kind: tool.Select, Choices: []string{"v4", "v7", "ulid"}, Default: "v4"},
			{Key: "count", Label: "Count", Kind: tool.Number, Min: 1, Max: 1000, Default: 1},
			{Key: "uppercase", Label: "Uppercase", Kind: tool.Toggle, Default: false},
		},
		Output: tool.PlainText,
		Live:   true,
	}
}

func (u UUID) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	opts := req.Opts.WithDefaults(u.Spec())
	now := time.Now
	if u.Now != nil {
		now = u.Now
	}
	if in := strings.TrimSpace(req.Inputs["input"]); in != "" {
		return inspectID(in, now())
	}

	r := u.Rand
	if r == nil {
		r = rand.Reader
	}
	count := min(max(opts.Int("count"), 1), 1000)
	version := opts.String("version")
	ids, err := generateIDs(ctx, version, count, now(), r)
	if err != nil {
		return tool.Result{}, err
	}
	out := strings.Join(ids, "\n")
	if opts.Bool("uppercase") {
		out = strings.ToUpper(out)
	}
	return tool.Result{
		Text: out,
		Meta: []tool.KV{{Key: "Generated", Value: fmt.Sprint(count)}, {Key: "Type", Value: version}},
	}, nil
}

func generateIDs(ctx context.Context, version string, count int, now time.Time, r io.Reader) ([]string, error) {
	ids := make([]string, 0, count)
	switch version {
	case "ulid":
		entropy := ulid.Monotonic(r, 0)
		for range count {
			id, err := ulid.New(ulid.Timestamp(now), entropy)
			if err != nil {
				return nil, err
			}
			ids = append(ids, id.String())
		}
	case "v7":
		var counter uint16
		for i := range count {
			id, err := newV7(now, r, &counter, i == 0)
			if err != nil {
				return nil, err
			}
			ids = append(ids, id.String())
		}
	default:
		for range count {
			id, err := uuid.NewRandomFromReader(r)
			if err != nil {
				return nil, err
			}
			ids = append(ids, id.String())
		}
	}
	return ids, ctx.Err()
}

// newV7 follows RFC 9562 §6.2 (method 1): rand_a is a 12-bit counter that
// starts random and increments within the batch, so UUIDs generated in the
// same millisecond stay sorted.
func newV7(now time.Time, r io.Reader, counter *uint16, first bool) (uuid.UUID, error) {
	var id uuid.UUID
	if _, err := io.ReadFull(r, id[6:]); err != nil {
		return id, err
	}
	if first {
		*counter = binary.BigEndian.Uint16(id[6:8]) & 0x07ff // leaves room to increment
	} else {
		*counter = (*counter + 1) & 0x0fff
	}
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(max(now.UnixMilli(), 0))) // low 48 bits
	copy(id[0:6], ts[2:8])
	binary.BigEndian.PutUint16(id[6:8], 0x7000|*counter) // version 7 + counter
	id[8] = 0x80 | (id[8] & 0x3f)                        // RFC 9562 variant
	return id, nil
}

var versionNames = map[uuid.Version]string{
	1: "time and MAC based",
	2: "DCE Security",
	3: "MD5 of a name",
	4: "random",
	5: "SHA-1 of a name",
	6: "reordered time",
	7: "Unix epoch, sortable",
	8: "custom",
}

var ulidRe = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Za-hjkmnp-tv-z]{26}$`)

func inspectID(s string, now time.Time) (tool.Result, error) {
	if ulidRe.MatchString(s) {
		id, err := ulid.ParseStrict(strings.ToUpper(s))
		if err == nil {
			t := ulid.Time(id.Time()).UTC()
			text := strings.Join([]string{
				row("ULID", id.String()),
				row("As UUID", uuid.UUID(id).String()),
				row("Timestamp", fmt.Sprintf("%s (%s)", t.Format("2006-01-02 15:04:05.000 UTC"), converters.Relative(t.Sub(now)))),
			}, "\n")
			return tool.Result{Text: text, Meta: []tool.KV{{Key: "Type", Value: "ULID"}}}, nil
		}
	}

	id, err := uuid.Parse(s)
	if err != nil {
		return tool.Result{}, &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: "not a valid UUID nor ULID"}
	}
	rows := []string{row("UUID", id.String())}
	switch id {
	case uuid.Nil:
		rows = append(rows, row("Type", "Nil UUID (all zeros)"))
		return tool.Result{Text: strings.Join(rows, "\n"), Meta: []tool.KV{{Key: "Type", Value: "nil"}}}, nil
	case uuid.Max:
		rows = append(rows, row("Type", "Max UUID (all ones)"))
		return tool.Result{Text: strings.Join(rows, "\n"), Meta: []tool.KV{{Key: "Type", Value: "max"}}}, nil
	}
	v := id.Version()
	name := versionNames[v]
	if name == "" {
		name = "unknown"
	}
	rows = append(rows, row("Version", fmt.Sprintf("%d (%s)", v, name)), row("Variant", id.Variant().String()))
	if v == 1 || v == 6 || v == 7 {
		sec, nsec := id.Time().UnixTime()
		t := time.Unix(sec, nsec).UTC()
		rows = append(rows, row("Timestamp", fmt.Sprintf("%s (%s)", t.Format("2006-01-02 15:04:05.000 UTC"), converters.Relative(t.Sub(now)))))
	}
	return tool.Result{Text: strings.Join(rows, "\n"), Meta: []tool.KV{{Key: "Type", Value: fmt.Sprintf("UUID v%d", v)}}}, nil
}

func row(label, value string) string { return fmt.Sprintf("%-10s %s", label+":", value) }

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Detect recognises a canonical UUID (8-4-4-4-12).
func (UUID) Detect(sample string) float64 {
	if uuidRe.MatchString(strings.TrimSpace(sample)) {
		return 0.90
	}
	return 0
}
