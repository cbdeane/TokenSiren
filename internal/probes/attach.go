package probes

import "errors"

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
    // TODO: hold bpf object, links, and map handles.
}

func (h *Handle) Close() error {
    return nil
}

func Attach(spec AttachSpec) (*Handle, error) {
    if spec.BPFObjectPath == "" {
        return nil, errors.New("missing BPFObjectPath")
    }
    // TODO: load BPF object and attach uprobes.
    return &Handle{}, nil
}
