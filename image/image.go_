package image

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	"go.etcd.io/bbolt"

	"golang.org/x/sync/semaphore"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/local"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/diff/apply"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/metadata"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/platforms"
	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/rootfs"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/containerd/snapshots/overlay"
	"github.com/containerd/log"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type store struct {
	content     content.Store
	snapshot    snapshots.Snapshotter
	image       images.Store
	root        string
	rCtx        containerd.RemoteContext
	resolverOpt docker.ResolverOptions
}

var (
	service *store
	ctx     = namespaces.WithNamespace(context.Background(), "default")
)

func newContentStore(root string) content.Store {

	cs, err := local.NewStore(root)
	if err != nil {
		log.G(context.Background()).WithError(err).Fatalln(err)
	}
	return cs

}
func newImageStore(root string, cs content.Store) images.Store {

	idb, err := bbolt.Open(filepath.Join(root, "metaimage.db"), 0644, bbolt.DefaultOptions)
	if err != nil {
		log.G(context.Background()).WithError(err).Fatalln(err)
	}

	mdb := metadata.NewDB(idb, cs, map[string]snapshots.Snapshotter{}, metadata.WithPolicyIsolated)
	if err := mdb.Init(context.Background()); err != nil {
		log.G(context.Background()).WithError(err).Fatalln(err)
	}

	return metadata.NewImageStore(mdb)

}

func newSnapshotStore(root string) snapshots.Snapshotter {
	sn, err := overlay.NewSnapshotter(root)
	if err != nil {
		log.G(context.Background()).WithError(err).Fatalln(err)
	}
	return sn

}

type Repo struct {
	Path string
}

func InitRepository(root string) Repo {
	sn := newSnapshotStore(root)
	cs := newContentStore(root)
	is := newImageStore(root, cs)

	service = &store{
		root:     root,
		image:    is,
		content:  cs,
		snapshot: sn,
		rCtx: containerd.RemoteContext{Resolver: docker.NewResolver(docker.ResolverOptions{}),
			AllMetadata:     false,
			PlatformMatcher: platforms.Default()},
		resolverOpt: docker.ResolverOptions{},
	}
	return Repo{Path: root}

}
func getImageName(ref string) string {
	r, err := refdocker.ParseDockerRef(ref)
	if err != nil {
		return ""
	}
	return r.String()
}

func Fetch(ref string, unpackBool ...bool) error {
	ctx := namespaces.WithNamespace(context.Background(), "default")
	name := getImageName(ref)
	img, err := service.image.Get(ctx, name)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return err
		}

		img, err = fetch(ctx, name)
		if err != nil {
			fmt.Println("fetch faild", err)
			return err
		}

		img, err = service.image.Create(ctx, img)
		if err != nil {
			fmt.Println("image create", err)
			return err
		}
	}
	if len(unpackBool) != 1 {
		unpackBool = []bool{false}
	}
	if unpackBool[0] {
		err := unpack(ctx, img)
		if err != nil {
			fmt.Println("image create unpack", err)
			return err
		}
	}

	return nil

}

func Unpack(ref string) error {

	name := getImageName(ref)
	img, err := service.image.Get(ctx, name)
	if err != nil {
		return nil
	}
	err = unpack(ctx, img)
	if err != nil {
		fmt.Println("unpack", err)
		return err
	}
	return nil
}

func Pull(ref string) error {

	name := getImageName(ref)

	var (
		img images.Image
		err error
	)

	img, err = service.image.Get(ctx, name)

	if err != nil {

		if !errdefs.IsNotFound(err) {

			fmt.Println("service.image.Get", err)
			return err

		}

		img, err = fetch(ctx, name)
		if err != nil {
			fmt.Println("fetch faild", err)
			return err
		}

		img, err = service.image.Create(ctx, img)
		if err != nil {
			fmt.Println("image create", err)
			return err
		}
	}

	err = unpack(ctx, img)
	if err != nil {
		return err
	}
	return nil

}

func unpack(ctx context.Context, img images.Image) error {

	manifest, err := images.Manifest(ctx, service.content, img.Target, platforms.Default())
	if err != nil {
		return err
	}
	diffIDs, err := img.RootFS(ctx, service.content, platforms.Default())
	if err != nil {
		return err
	}
	imageLayers := []ocispec.Descriptor{}
	for _, ociLayer := range manifest.Layers {
		if images.IsLayerType(ociLayer.MediaType) {
			imageLayers = append(imageLayers, ociLayer)
		}
	}

	if len(diffIDs) != len(imageLayers) {
		return fmt.Errorf("mismatched image rootfs and manifest layers")
	}

	layers := make([]rootfs.Layer, len(diffIDs))
	for i := range diffIDs {
		layers[i].Diff = ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageLayer,
			Digest:    diffIDs[i],
		}
		layers[i].Blob = imageLayers[i]
	}

	var chain []digest.Digest

	applier := apply.NewFileSystemApplier(service.content)

	for _, layer := range layers {
		_, err := rootfs.ApplyLayerWithOpts(ctx, layer, chain, service.snapshot, applier, []snapshots.Opt{}, []diff.ApplyOpt{})
		if err != nil {
			return err
		}
		chain = append(chain, layer.Diff.Digest)
	}
	//parent := identity.ChainID(chain).String()
	return nil
}

func fetch(ctx context.Context, ref string) (images.Image, error) {

	name, desc, err := service.rCtx.Resolver.Resolve(ctx, ref)
	if err != nil {
		return images.Image{}, err
	}
	fetcher, err := service.rCtx.Resolver.Fetcher(ctx, name)
	if err != nil {
		return images.Image{}, err
	}

	var handler images.Handler
	childrenHandler := images.ChildrenHandler(service.content)
	childrenHandler = images.LimitManifests(childrenHandler, service.rCtx.PlatformMatcher, 1)
	handlers := append(service.rCtx.BaseHandlers, remotes.FetchHandler(service.content, fetcher), childrenHandler)
	handler = images.Handlers(handlers...)
	var limiter *semaphore.Weighted
	err = images.Dispatch(ctx, handler, limiter, desc)
	if err != nil {
		return images.Image{}, err
	}

	return images.Image{
		Name:   name,
		Target: desc,
		Labels: service.rCtx.Labels,
	}, nil

}

func isUnpacked(ref string) error {

	name := getImageName(ref)
	img, err := service.image.Get(ctx, name)
	if err != nil {
		return nil
	}

	diffs, err := img.RootFS(ctx, service.content, platforms.Default())
	if err != nil {
		fmt.Println("isUnpacked", err)
		return err
	}
	parent := string(identity.ChainID(diffs).String())
	_, err = service.snapshot.Stat(ctx, parent)
	if err == nil {
		//c.parent = parent
		return nil
	}

	if !errdefs.IsNotFound(err) {
		return err
	}
	/*
		err = unpack()
		if err == nil {
			return nil
		}
	*/
	return nil
}

func Mount(ref string, key string, target string, readonly ...bool) error {

	name := getImageName(ref)
	img, err := service.image.Get(ctx, name)
	if err != nil {
		fmt.Println(err)
		return err
	}

	diffs, err := img.RootFS(ctx, service.content, platforms.Default())
	if err != nil {
		fmt.Println(err)
		return err
	}

	parent := string(identity.ChainID(diffs).String())

	if len(readonly) != 1 {
		readonly = []bool{false}
	}

	var mounts []mount.Mount
	if readonly[0] {
		mounts, err = service.snapshot.View(ctx, key, parent)
	} else {
		mounts, err = service.snapshot.Prepare(ctx, key, parent)
	}

	if err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return err
		}
		mounts, err = service.snapshot.Mounts(ctx, key)
		if err != nil {
			return err
		}
	}

	err = mount.All(mounts, target)
	if err != nil {
		service.snapshot.Remove(ctx, key)
		err = fmt.Errorf("mount target %s: %w", target, err)
		fmt.Println(err)
		return err
	}

	return nil

}

func Umount(target string, key ...string) error {

	err := mount.UnmountAll(target, 0)
	if err != nil {
		fmt.Println("mount.UnmountAll", err)
		return err
	}

	if len(key) == 1 {
		err = service.snapshot.Remove(ctx, key[1])
		fmt.Println("umount remove key", err)
	}

	return nil
}

func WithImageConfig(ref string) oci.SpecOpts {

	ctx := namespaces.WithNamespace(context.Background(), "default")
	name := getImageName(ref)

	img, err := service.image.Get(ctx, name)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return func(ctx context.Context, _ oci.Client, c *containers.Container, s *oci.Spec) error {

		ic, err := img.Config(ctx, service.content, platforms.Default())

		if err != nil {
			return err
		}
		var (
			imageConfigBytes []byte
			ociimage         ocispec.Image
			config           ocispec.ImageConfig
		)
		switch ic.MediaType {
		case ocispec.MediaTypeImageConfig, images.MediaTypeDockerSchema2Config:
			var err error
			imageConfigBytes, err = content.ReadBlob(ctx, service.content, ic)
			if err != nil {
				return err
			}

			if err := json.Unmarshal(imageConfigBytes, &ociimage); err != nil {
				return err
			}
			config = ociimage.Config
		default:
			return fmt.Errorf("unknown image config media type %s", ic.MediaType)
		}
		s.Process.Env = config.Env

		cmd := config.Cmd
		if s.Process.Args[0] == "" {
			cmd = append(cmd, s.Process.Args[1:]...)
			s.Process.Args = append(config.Entrypoint, cmd...)
		}

		cwd := config.WorkingDir
		if cwd == "" {
			cwd = "/"
		}
		s.Process.Cwd = cwd
		s.Annotations["stop-signal"] = config.StopSignal
		return nil

	}

}
