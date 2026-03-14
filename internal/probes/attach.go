package probes

import (
    "errors"
    "fmt"

    "github.com/cilium/ebpf"
    "github.com/cilium/ebpf/link"
    "github.com/cilium/ebpf/rlimit"
)

type AttachSpec struct {
    BPFObjectPath string
    Uprobes       []UprobeSpec
}

type UprobeSpec struct {
    BinaryPath string
    Symbol     string
    Handler    string
}

type Handle struct {
    collection *ebpf.Collection
    links      []link.Link
}

func (h *Handle) Close() error {
    if h == nil {
        return nil
    }
    for _, l := range h.links {
        _ = l.Close()
    }
    if h.collection != nil {
        h.collection.Close()
    }
    return nil
}

func Attach(spec AttachSpec) (*Handle, error) {
    if spec.BPFObjectPath == "" {
        return nil, errors.New("missing BPFObjectPath")
    }
    if err := rlimit.RemoveMemlock(); err != nil {
        return nil, fmt.Errorf("set memlock rlimit: %w", err)
    }

    collSpec, err := ebpf.LoadCollectionSpec(spec.BPFObjectPath)
    if err != nil {
        return nil, fmt.Errorf("load bpf object: %w", err)
    }

    coll, err := ebpf.NewCollection(collSpec)
    if err != nil {
        return nil, fmt.Errorf("create bpf collection: %w", err)
    }

    handle := &Handle{collection: coll}
    for _, up := range spec.Uprobes {
        if up.BinaryPath == "" {
            handle.Close()
            return nil, errors.New("missing BinaryPath for uprobe")
        }
        if up.Symbol == "" {
            handle.Close()
            return nil, errors.New("missing Symbol for uprobe")
        }
        prog := coll.Programs[up.Handler]
        if prog == nil {
            handle.Close()
            return nil, fmt.Errorf("missing program handler %q in bpf object", up.Handler)
        }
        exe, err := link.OpenExecutable(up.BinaryPath)
        if err != nil {
            handle.Close()
            return nil, fmt.Errorf("open executable %q: %w", up.BinaryPath, err)
        }
        ln, err := exe.Uprobe(up.Symbol, prog, nil)
        if err != nil {
            handle.Close()
            return nil, fmt.Errorf("attach uprobe %s:%s -> %s: %w", up.BinaryPath, up.Symbol, up.Handler, err)
        }
        handle.links = append(handle.links, ln)
    }

    return handle, nil
}
