package walk

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Clause-Eight/ato/v2/internal/domain"
)

// ErrNotImage is returned when a file is not a saved container image.
var ErrNotImage = errors.New("not a saved container image")

// Image is a container image saved with `docker save`, in the classic layout (manifest.json) or the OCI layout
// (index.json). It is read in place: each layer is a section of the file, so layers can be read in parallel.
type Image struct {
	Rel     string
	Layers  []Layer
	Configs []string
	file    *os.File
	members map[string]member
	orders  [][]int
	opts    Options
	inc     []glob
	exc     []glob
	budget  *budget
}

// Layer is one filesystem layer. ID is the short digest used in paths.
type Layer struct {
	ID      string
	name    string
	files   map[string]bool
	deleted []string
	opaque  []string
}

type member struct{ off, size int64 }

// OpenImage indexes a saved image. The caller closes it.
func OpenImage(p string, opts Options) (*Image, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %w", p, err)
	}
	img := &Image{Rel: filepath.ToSlash(filepath.Clean(p)), file: f, members: map[string]member{}, opts: opts, budget: newBudget(opts.Limits)}
	if err := img.index(); err != nil {
		f.Close()
		return nil, err
	}
	if img.inc, err = compileAll(opts.Include); err == nil {
		img.exc, err = compileAll(append(append([]string{}, opts.Exclude...), opts.ImageExclude...))
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return img, nil
}

// Close releases the file.
func (img *Image) Close() error { return img.file.Close() }

// index records where every member of the outer tar starts, then reads the manifests.
func (img *Image) index() error {
	head := make([]byte, 2)
	if _, err := io.ReadFull(img.file, head); err == nil && head[0] == 0x1f && head[1] == 0x8b {
		return fmt.Errorf("%w: %s is compressed; decompress it first (gunzip)", ErrNotImage, img.Rel)
	}
	if _, err := img.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	tr := tar.NewReader(img.file)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: %s: %v", ErrNotImage, img.Rel, err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		off, err := img.file.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		img.members[cleanName(hdr.Name)] = member{off: off, size: hdr.Size}
	}
	switch {
	case img.has("manifest.json"):
		return img.dockerManifest()
	case img.has("index.json"):
		return img.ociIndex()
	}
	return fmt.Errorf("%w: %s has no manifest.json or index.json", ErrNotImage, img.Rel)
}

func (img *Image) has(name string) bool { _, ok := img.members[name]; return ok }

func (img *Image) section(name string) io.Reader {
	m := img.members[name]
	return io.NewSectionReader(img.file, m.off, m.size)
}

func (img *Image) readJSON(name string, v any) error {
	m, ok := img.members[name]
	if !ok {
		return fmt.Errorf("%w: %s is missing %s", ErrNotImage, img.Rel, name)
	}
	if m.size > 16<<20 {
		return fmt.Errorf("%w: %s in %s is too large", ErrNotImage, name, img.Rel)
	}
	if err := json.NewDecoder(img.section(name)).Decode(v); err != nil {
		return fmt.Errorf("%w: %s in %s: %v", ErrNotImage, name, img.Rel, err)
	}
	return nil
}

func (img *Image) dockerManifest() error {
	var manifests []struct {
		Config string
		Layers []string
	}
	if err := img.readJSON("manifest.json", &manifests); err != nil {
		return err
	}
	for _, m := range manifests {
		img.addConfig(cleanName(m.Config))
		names := make([]string, len(m.Layers))
		for i, l := range m.Layers {
			names[i] = cleanName(l)
		}
		if err := img.addOrder(names); err != nil {
			return err
		}
	}
	return nil
}

type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
}

func blobName(digest string) string {
	alg, hex, _ := strings.Cut(digest, ":")
	return "blobs/" + alg + "/" + hex
}

func (img *Image) ociIndex() error {
	return img.ociManifests("index.json", 0)
}

// ociManifests follows an image index to its manifests. Manifests for platforms that were not saved are
// skipped.
func (img *Image) ociManifests(name string, depth int) error {
	var doc struct {
		MediaType string          `json:"mediaType"`
		Manifests []ociDescriptor `json:"manifests"`
		Config    ociDescriptor   `json:"config"`
		Layers    []ociDescriptor `json:"layers"`
	}
	if err := img.readJSON(name, &doc); err != nil {
		return err
	}
	if len(doc.Manifests) == 0 {
		img.addConfig(blobName(doc.Config.Digest))
		names := make([]string, len(doc.Layers))
		for i, l := range doc.Layers {
			names[i] = blobName(l.Digest)
		}
		return img.addOrder(names)
	}
	if depth > 4 {
		return fmt.Errorf("%w: %s nests image indexes too deep", ErrNotImage, img.Rel)
	}
	for _, m := range doc.Manifests {
		if child := blobName(m.Digest); img.has(child) {
			if err := img.ociManifests(child, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (img *Image) addConfig(name string) {
	if !img.has(name) {
		return
	}
	for _, c := range img.Configs {
		if c == name {
			return
		}
	}
	img.Configs = append(img.Configs, name)
}

var hexDigest = regexp.MustCompile(`[0-9a-f]{64}`)

// addOrder records one image's layers, bottom first. A layer shared by several images is read once.
func (img *Image) addOrder(names []string) error {
	order := make([]int, 0, len(names))
	for _, name := range names {
		if !img.has(name) {
			return fmt.Errorf("%w: %s is missing layer %s", ErrNotImage, img.Rel, name)
		}
		idx := -1
		for i, l := range img.Layers {
			if l.name == name {
				idx = i
			}
		}
		if idx < 0 {
			id := hexDigest.FindString(name)
			if len(id) >= 12 {
				id = id[:12]
			} else {
				id = fmt.Sprintf("layer%d", len(img.Layers)+1)
			}
			img.Layers = append(img.Layers, Layer{ID: id, name: name})
			idx = len(img.Layers) - 1
		}
		order = append(order, idx)
	}
	img.orders = append(img.orders, order)
	return nil
}

// VisitConfig calls fn for an image configuration, which holds the environment, labels and build history.
func (img *Image) VisitConfig(name string, fn VisitFunc) error {
	m := img.members[name]
	return fn(Entry{Rel: img.Rel + "!/" + name, Name: name, Size: m.size}, img.section(name))
}

// VisitLayer calls fn for every selected regular file in layer i, opening archives inside it. It also records
// the layer's files and whiteouts for Shadowed. Different layers may be visited at the same time.
func (img *Image) VisitLayer(i int, fn VisitFunc) Outcome {
	l := &img.Layers[i]
	l.files = map[string]bool{}
	rel := img.Rel + "!/" + l.ID
	v := &visitor{fn: fn, exc: img.exc, maxFile: img.opts.MaxFileSize, budget: img.budget}
	if err := img.layer(v, l, rel); err != nil {
		v.out.Unreadable = append(v.out.Unreadable, domain.Skipped{Path: rel, Reason: err.Error()})
	}
	return v.out
}

func (img *Image) layer(v *visitor, l *Layer, rel string) error {
	br := bufio.NewReaderSize(img.section(l.name), 64<<10)
	var r io.Reader = br
	head, _ := br.Peek(4)
	switch {
	case bytes.HasPrefix(head, []byte{0x1f, 0x8b}):
		zr, err := gzip.NewReader(br)
		if err != nil {
			return fmt.Errorf("not a valid gzip layer: %w", err)
		}
		defer zr.Close()
		r = counted{zr, img.budget}
	case bytes.HasPrefix(head, []byte{0x28, 0xb5, 0x2f, 0xfd}):
		return errors.New("zstd-compressed layers are not supported")
	}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			if IsLimit(err) {
				return err
			}
			return fmt.Errorf("not a valid layer: %w", err)
		}
		if err := img.budget.entry(); err != nil {
			return err
		}
		name := cleanName(hdr.Name)
		dir, base := path.Split(name)
		if base == ".wh..wh..opq" {
			l.opaque = append(l.opaque, strings.TrimSuffix(dir, "/"))
			continue
		}
		if strings.HasPrefix(base, ".wh.") {
			l.deleted = append(l.deleted, dir+strings.TrimPrefix(base, ".wh."))
			continue
		}
		if hdr.Typeflag != tar.TypeReg || name == "" {
			continue
		}
		l.files[name] = true
		if !selected(name, img.inc, img.exc) {
			continue
		}
		if err := v.entry(rel+"/"+name, name, tr, hdr.Size, 1); err != nil {
			return err
		}
	}
}

// Shadowed says why a file in layer i is not visible in a running container: deleted or replaced by a later
// layer. It is "" when the file is visible. Call it after every layer was visited.
func (img *Image) Shadowed(i int, name string) string {
	for _, order := range img.orders {
		pos := -1
		for k, idx := range order {
			if idx == i {
				pos = k
			}
		}
		if pos < 0 {
			continue
		}
		for _, j := range order[pos+1:] {
			later := img.Layers[j]
			for _, d := range later.deleted {
				if within(name, d) {
					return fmt.Sprintf("deleted in later layer %s, but still stored in layer %s of the image", later.ID, img.Layers[i].ID)
				}
			}
			for _, d := range later.opaque {
				if within(name, d) {
					return fmt.Sprintf("hidden by later layer %s, but still stored in layer %s of the image", later.ID, img.Layers[i].ID)
				}
			}
			if later.files[name] {
				return fmt.Sprintf("replaced in later layer %s, but this copy is still stored in layer %s of the image", later.ID, img.Layers[i].ID)
			}
		}
	}
	return ""
}

// within reports whether name is dir or lies under it; "" is the root.
func within(name, dir string) bool {
	return dir == "" || name == dir || strings.HasPrefix(name, dir+"/")
}
